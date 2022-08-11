package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type PrefixSIDTLVType uint8

const (
	AttributeTypePrefixSID        AttributeTypeCode = 40
	PrefixSIDTLVTypeSRv6L3Service PrefixSIDTLVType  = 5
)

type SRv6ServiceTLV struct {
	Type    PrefixSIDTLVType
	SubTLVs []SRv6ServiceSubTLV
}

func PrefixSIDFromPathAttribute(a PathAttribute) (SRv6ServiceTLV, error) {
	if a.TypeCode != AttributeTypePrefixSID {
		return SRv6ServiceTLV{}, fmt.Errorf("invalid type code: %d", a.TypeCode)
	}
	if len(a.Value) < 4 {
		return SRv6ServiceTLV{}, fmt.Errorf("invalid attribute length: %d", len(a.Value))
	}

	v := SRv6ServiceTLV{
		Type: PrefixSIDTLVType(a.Value[0]),
	}
	if v.Type != PrefixSIDTLVTypeSRv6L3Service {
		return SRv6ServiceTLV{}, fmt.Errorf("unexpected TLV type: %d", v.Type)
	}

	r := bytes.NewReader(a.Value[4:]) // First 4 byte: type(1) + length(2) + reserved(1)

	for r.Len() > 0 {
		sub, err := ReadSRv6ServiceSubTLV(r)
		if err != nil {
			return SRv6ServiceTLV{}, fmt.Errorf("read SRv6 sub tlv: %w", err)
		}
		v.SubTLVs = append(v.SubTLVs, sub)
	}

	return v, nil
}

func (v SRv6ServiceTLV) ToPathAttribute() PathAttribute {
	var size int
	for _, s := range v.SubTLVs {
		size += s.SRv6ServiceSubTLVLen()
	}
	b := make([]byte, 4, 4+size)
	b[0] = byte(v.Type)
	binary.BigEndian.PutUint16(b[1:3], uint16(size))
	w := bytes.NewBuffer(b)
	for _, s := range v.SubTLVs {
		s.WriteSRv6ServiceSubTLV(w)
	}
	if w.Len() != (size + 4) { // TODO: デバッグ目的なので後で消す
		panic(fmt.Errorf("invalid built tlv size: expected=%d got=%d (%x)", size+4, w.Len(), w.Bytes()))
	}
	return PathAttribute{
		Flags:    0b11000000, // optional, transitive
		TypeCode: AttributeTypePrefixSID,
		Value:    w.Bytes(),
	}
}

type SRv6ServiceSubTLV interface {
	WriteSRv6ServiceSubTLV(w *bytes.Buffer)
	SRv6ServiceSubTLVLen() int
}

func ReadSRv6ServiceSubTLV(r *bytes.Reader) (SRv6ServiceSubTLV, error) {
	t, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	var length uint16
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	b := make([]byte, length)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}

	switch t {
	case 1:
		return ParseSRv6SIDInforSubTLV(b)
	default:
		return nil, fmt.Errorf("unknown SRv6 Service Sub TLV Type: %d", t)
	}
}

type SRv6SIDInfoSubTLV struct {
	SID              net.IP
	Flags            byte
	EndpointBehavior uint16
	SubSubTLV        []SRv6ServiceDataSubSubTLV
}

func ParseSRv6SIDInforSubTLV(b []byte) (SRv6SIDInfoSubTLV, error) {
	if len(b) < 21 {
		return SRv6SIDInfoSubTLV{}, fmt.Errorf("too short SID information sub TLV: %x", b)
	}

	v := SRv6SIDInfoSubTLV{
		SID:              net.IP(b[1:17]),
		Flags:            b[17],
		EndpointBehavior: binary.BigEndian.Uint16(b[18:20]),
	}

	r := bytes.NewReader(b[21:])
	for r.Len() > 0 {
		subsub, err := ReadSRv6ServiceDataSubSubTLV(r)
		if err != nil {
			return SRv6SIDInfoSubTLV{}, fmt.Errorf("read SRv6 sub sub tlv: %w", err)
		}
		v.SubSubTLV = append(v.SubSubTLV, subsub)
	}
	return v, nil
}

func (v SRv6SIDInfoSubTLV) WriteSRv6ServiceSubTLV(w *bytes.Buffer) {
	w.WriteByte(1)                                                        // Type: SID Information
	binary.Write(w, binary.BigEndian, uint16(v.SRv6ServiceSubTLVLen()-3)) // Length (Len で作っているのは Type と Length の 3 バイトも含んでいるので減らす)
	w.WriteByte(0x00)                                                     // RESERVED
	w.Write(v.SID)
	w.WriteByte(v.Flags)
	binary.Write(w, binary.BigEndian, v.EndpointBehavior)
	w.WriteByte(0x00) // RESERVED
	for _, s := range v.SubSubTLV {
		s.WriteSRv6ServiceDataSubSubTLV(w)
	}
}

func (v SRv6SIDInfoSubTLV) SRv6ServiceSubTLVLen() int {
	size := 24
	for _, s := range v.SubSubTLV {
		size += s.SRv6ServiceDataSubSubTLVLen()
	}
	return size
}

type SRv6ServiceDataSubSubTLV interface {
	WriteSRv6ServiceDataSubSubTLV(w *bytes.Buffer)
	SRv6ServiceDataSubSubTLVLen() int
}

func ReadSRv6ServiceDataSubSubTLV(r *bytes.Reader) (SRv6ServiceDataSubSubTLV, error) {
	t, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	var length uint16
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	b := make([]byte, length)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}

	switch t {
	case 1:
		return ParseSRv6SIDStructureSubSubTLV(b)
	default:
		return nil, fmt.Errorf("unknown SRv6 Service Data Sub-Sub TLV Type: %d", t)
	}

}

type SRv6SIDStructureSubSubTLV struct {
	LocatorBlockLength  uint8
	LocatorNodeLength   uint8
	FunctionLength      uint8
	ArgumentLength      uint8
	TranspositionLength uint8
	TranspositionOffset uint8
}

func ParseSRv6SIDStructureSubSubTLV(b []byte) (SRv6SIDStructureSubSubTLV, error) {
	if len(b) < 6 {
		return SRv6SIDStructureSubSubTLV{}, fmt.Errorf("invalid SID Structure Sub-Sub TLV length: %d (%x)", len(b), b)
	}
	return SRv6SIDStructureSubSubTLV{
		LocatorBlockLength:  b[0],
		LocatorNodeLength:   b[1],
		FunctionLength:      b[2],
		ArgumentLength:      b[3],
		TranspositionLength: b[4],
		TranspositionOffset: b[5],
	}, nil
}

func (v SRv6SIDStructureSubSubTLV) WriteSRv6ServiceDataSubSubTLV(w *bytes.Buffer) {
	w.Write([]byte{1})                           // Type: SID Structure
	binary.Write(w, binary.BigEndian, uint16(6)) // Length
	w.Write([]byte{
		v.LocatorBlockLength,
		v.LocatorNodeLength,
		v.FunctionLength,
		v.ArgumentLength,
		v.TranspositionLength,
		v.TranspositionOffset,
	})
}

func (v SRv6SIDStructureSubSubTLV) SRv6ServiceDataSubSubTLVLen() int {
	return 9 // type (1) + length (2) + values (6)
}

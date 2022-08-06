package main

import (
	"encoding/binary"
	"fmt"
)

type AttributeTypeCode uint8

const (
	AttributeTypeOrigin AttributeTypeCode = iota + 1
	AttributeTypeASPath
	AttributeTypeNextHop
	// TODO: Other attributes
)

type OriginAttribute uint8

const (
	OriginAttributeIGP OriginAttribute = iota
	OriginAttributeEGP
	OriginAttributeIncomplete
)

func OriginFromPathAttribute(a PathAttribute) (OriginAttribute, error) {
	if a.TypeCode != AttributeTypeOrigin {
		return 0, fmt.Errorf("invalid type code: %d", a.TypeCode)
	}
	if len(a.Value) != 1 {
		return 0, fmt.Errorf("invalid attribute value length: %d", len(a.Value))
	}
	origin := OriginAttribute(a.Value[0])
	if origin > 3 {
		return 0, fmt.Errorf("invalid origin value: %v", origin)
	}
	return origin, nil
}

func (a OriginAttribute) ToPathAttribute() PathAttribute {
	return PathAttribute{
		Flags:    0b01000000, // well-known transitive
		TypeCode: AttributeTypeOrigin,
		Value:    []byte{byte(a)},
	}
}

type ASPathAttribute struct {
	Sequence bool
	Segments []uint16
}

func ASPathFromPathAttribute(a PathAttribute) (ASPathAttribute, error) {
	if a.TypeCode != AttributeTypeASPath {
		return ASPathAttribute{}, fmt.Errorf("invalid type code: %d", a.TypeCode)
	}
	if len(a.Value) < 2 {
		return ASPathAttribute{}, fmt.Errorf("too short AS_PATH attribute: %d", len(a.Value))
	}
	length := int(a.Value[1])
	segments := make([]uint16, length)
	for i := 0; i < length; i++ {
		offset := 2 + i*2
		segments[i] = binary.BigEndian.Uint16(a.Value[offset : offset+2])
	}
	return ASPathAttribute{
		Sequence: a.Value[0] == 2,
		Segments: segments,
	}, nil
}

func (a ASPathAttribute) ToPathAttribute() PathAttribute {
	b := make([]byte, 2+len(a.Segments)*2)
	if a.Sequence {
		b[0] = 2
	} else {
		b[0] = 1
	}
	b[1] = uint8(len(a.Segments))
	for i, s := range a.Segments {
		offset := 2 + i*2
		binary.BigEndian.PutUint16(b[offset:offset+2], s)
	}
	return PathAttribute{
		Flags:    0b01000000, // well-known transitive
		TypeCode: AttributeTypeASPath,
		Value:    b,
	}
}

type NextHopAttribute []byte

func NextHopFromPathAttribute(a PathAttribute) (NextHopAttribute, error) {
	if a.TypeCode != AttributeTypeNextHop {
		return nil, fmt.Errorf("invalid type code: %d", a.TypeCode)
	}
	if len(a.Value) != 4 {
		return nil, fmt.Errorf("invalid next hop length: %d", len(a.Value))
	}
	return NextHopAttribute(a.Value), nil
}

func (a NextHopAttribute) ToPathAttribute() PathAttribute {
	return PathAttribute{
		Flags:    0b01000000, // well-known transitive
		TypeCode: AttributeTypeNextHop,
		Value:    []byte(a),
	}
}

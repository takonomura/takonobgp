package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
)

const (
	AttributeTypeMPReachNLRI   AttributeTypeCode = 14
	AttributeTypeMPUnreachNLRI AttributeTypeCode = 15
)

type MPReachNLRI struct {
	AF      AddressFamily
	NextHop []net.IP
	NLRI    []*net.IPNet
}

func MPReachNLRIFromPathAttribute(a PathAttribute) (MPReachNLRI, error) {
	if a.TypeCode != AttributeTypeMPReachNLRI {
		return MPReachNLRI{}, fmt.Errorf("invalid type code: %d", a.TypeCode)
	}
	if len(a.Value) < 4 {
		return MPReachNLRI{}, fmt.Errorf("invalid length: %d", len(a.Value))
	}

	v := MPReachNLRI{
		AF: AddressFamily{
			AFI:  AFI(binary.BigEndian.Uint16(a.Value[0:2])),
			SAFI: SAFI(a.Value[2]),
		},
	}

	v.NextHop = make([]net.IP, a.Value[3]/byte(v.AF.NextHopSize()))
	for i := 0; i < len(v.NextHop); i++ {
		offset := 4 + v.AF.NextHopSize()*i
		v.NextHop[i] = net.IP(a.Value[offset : offset+v.AF.NextHopSize()])
	}

	r := bytes.NewReader(a.Value[5+a.Value[3]:])

	for r.Len() > 0 {
		route, err := readIPNet(r, v.AF.AddressBits())
		if err != nil {
			return MPReachNLRI{}, fmt.Errorf("nlri: %w", err)
		}
		v.NLRI = append(v.NLRI, route)
	}

	return v, nil
}

func (a MPReachNLRI) ToPathAttribute() PathAttribute {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint16(a.AF.AFI))
	buf.Write([]byte{uint8(a.AF.SAFI), uint8(len(a.NextHop) * a.AF.NextHopSize())})
	for _, a := range a.NextHop {
		buf.Write([]byte(a))
	}
	for _, r := range a.NLRI {
		writeIPNet(buf, r)
	}
	buf.Write([]byte{0}) // Reserved

	return PathAttribute{
		Flags:    0b10000000, // optional non-transitive
		TypeCode: AttributeTypeMPReachNLRI,
		Value:    buf.Bytes(),
	}
}

type MPUnreachNLRI struct {
	AF              AddressFamily
	WithdrawnRoutes []*net.IPNet
}

func MPUnreachNLRIFromPathAttribute(a PathAttribute) (MPUnreachNLRI, error) {
	if a.TypeCode != AttributeTypeMPUnreachNLRI {
		return MPUnreachNLRI{}, fmt.Errorf("invalid type code: %d", a.TypeCode)
	}
	if len(a.Value) < 3 {
		return MPUnreachNLRI{}, fmt.Errorf("invalid length: %d", len(a.Value))
	}
	v := MPUnreachNLRI{
		AF: AddressFamily{
			AFI:  AFI(binary.BigEndian.Uint16(a.Value[0:2])),
			SAFI: SAFI(a.Value[2]),
		},
	}
	r := bytes.NewReader(a.Value[3:])

	for r.Len() > 0 {
		route, err := readIPNet(r, v.AF.AddressBits())
		if err != nil {
			return MPUnreachNLRI{}, fmt.Errorf("withdrawn: %w", err)
		}
		v.WithdrawnRoutes = append(v.WithdrawnRoutes, route)
	}

	return v, nil
}

func (a MPUnreachNLRI) ToPathAttribute() PathAttribute {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint16(a.AF.AFI))
	buf.Write([]byte{uint8(a.AF.SAFI)})
	for _, r := range a.WithdrawnRoutes {
		writeIPNet(buf, r)
	}

	return PathAttribute{
		Flags:    0b10000000, // optional non-transitive
		TypeCode: AttributeTypeMPUnreachNLRI,
		Value:    buf.Bytes(),
	}
}

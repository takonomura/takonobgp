package main

import "fmt"

const AttributeTypeExtendedCommunities AttributeTypeCode = 16

type ExtendedCommunities []ExtendedCommunity

func ExtendedCommunitiesFromPathAttribute(a PathAttribute) (ExtendedCommunities, error) {
	if a.TypeCode != AttributeTypeExtendedCommunities {
		return nil, fmt.Errorf("unexpected attribute type code: %d", a.TypeCode)
	}
	if (len(a.Value) % 8) != 0 {
		return nil, fmt.Errorf("invalid extended communities length: %d (%x)", len(a.Value), a.Value)
	}
	s := make(ExtendedCommunities, len(a.Value)/8)
	for i := range s {
		offset := i * 8
		s[i].UnmarshalBinary(a.Value[offset : offset+8])
	}
	return s, nil
}

func (c ExtendedCommunities) ToPathAttribute() PathAttribute {
	b := make([]byte, 0, 8*len(c))
	for _, v := range c {
		cb, _ := v.MarshalBinary()
		b = append(b, cb...)
	}
	return PathAttribute{
		Flags:    0b11000000, // optional transitive
		TypeCode: AttributeTypeExtendedCommunities,
		Value:    b,
	}
}

type ExtendedCommunityType struct {
	Type    uint8
	Subtype uint8
}

func (t ExtendedCommunityType) Bytes() []byte {
	return []byte{t.Type, t.Subtype} // TODO: Subtype なし対応
}

type ExtendedCommunity struct {
	Type      ExtendedCommunityType
	Community []byte
}

func (c ExtendedCommunity) MarshalBinary() ([]byte, error) {
	return append(c.Type.Bytes(), c.Community...), nil
}

func (c *ExtendedCommunity) UnmarshalBinary(data []byte) error {
	if len(data) != 8 {
		return fmt.Errorf("invalid data length for extended community: %x", data)
	}
	c.Type.Type = data[0]
	c.Type.Subtype = data[1] // TODO: Subtype なし対応
	copy(c.Community, data[2:8])
	return nil
}

package main

import "encoding/binary"

type MultiprotocolExtensionCapability struct {
	AddressFamily AddressFamily
}

func (c MultiprotocolExtensionCapability) ToOptionalParameter() []byte {
	b := []byte{
		2, // 0: Parameter Type: 2 = Capability
		6, // 1: Parameter Length
		1, // 2: Capability Code: 1 = Multiprotocol Extensions
		4, // 3: Capability Length
		0, // 4: AFI
		0, // 5: AFI
		0, // 6: Reserved
		0, // 7: SAFI
	}
	binary.BigEndian.PutUint16(b[4:6], uint16(c.AddressFamily.AFI))
	b[7] = uint8(c.AddressFamily.SAFI)
	return b
}

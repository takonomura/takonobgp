package main

import (
	"fmt"
)

type (
	AFI  uint16
	SAFI uint8
)

const (
	AFIIPv4               AFI  = 1
	AFIIPv6               AFI  = 2
	SAFIUnicast           SAFI = 1
	SAFILabeledVPNUnicast SAFI = 128
)

var (
	IPv4Unicast = AddressFamily{
		AFI:  AFIIPv4,
		SAFI: SAFIUnicast,
	}
	IPv6Unicast = AddressFamily{
		AFI:  AFIIPv6,
		SAFI: SAFIUnicast,
	}
	IPv6VPN = AddressFamily{
		AFI:  AFIIPv6,
		SAFI: SAFILabeledVPNUnicast,
	}
)

func (afi AFI) Size() int {
	switch afi {
	case AFIIPv4:
		return 4
	case AFIIPv6:
		return 16
	default:
		return 0 // TODO
	}
}

func (afi AFI) Bits() int {
	return afi.Size() * 8
}

type AddressFamily struct {
	AFI  AFI
	SAFI SAFI
}

func (f AddressFamily) AddressBits() int {
	switch {
	case f.AFI == AFIIPv4 && f.SAFI == SAFIUnicast:
		return 32
	case f.AFI == AFIIPv6 && f.SAFI == SAFIUnicast:
		return 128
	case f.AFI == AFIIPv6 && f.SAFI == SAFILabeledVPNUnicast:
		return 128 + 64
	default:
		panic(fmt.Errorf("unknown address family: AFI=%d SAFI=%d", f.AFI, f.SAFI))
	}
}

func (safi SAFI) NextHopIgnorableSize() int {
	switch safi {
	case SAFILabeledVPNUnicast:
		return 8
	default:
		return 0
	}
}

func (f AddressFamily) NextHopSize() int {
	return f.AFI.Size() + f.SAFI.NextHopIgnorableSize()
}

func (f AddressFamily) String() string {
	switch f {
	case IPv4Unicast:
		return "ipv4-unicast"
	case IPv6Unicast:
		return "ipv6-unicast"
	case IPv6VPN:
		return "ipv6-vpn"
	default:
		return fmt.Sprintf("address-family-%d-%d", f.AFI, f.SAFI)
	}
}

func AddressFamilyFromString(name string) (AddressFamily, bool) {
	switch name {
	case "ipv4-unicast":
		return IPv4Unicast, true
	case "ipv6-unicast":
		return IPv6Unicast, true
	case "ipv6-vpn":
		return IPv6VPN, true
	default:
		return AddressFamily{}, false
	}
}

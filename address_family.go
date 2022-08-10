package main

import (
	"fmt"
)

type (
	AFI  uint16
	SAFI uint8
)

const (
	AFIIPv4     AFI  = 1
	AFIIPv6     AFI  = 2
	SAFIUnicast SAFI = 1
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
)

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
	default:
		panic(fmt.Errorf("unknown address family: AFI=%d SAFI=%d", f.AFI, f.SAFI))
	}
}

func (f AddressFamily) NextHopSize() int {
	return f.AddressBits() / 8
}

func (f AddressFamily) String() string {
	switch f {
	case IPv4Unicast:
		return "ipv4-unicast"
	case IPv6Unicast:
		return "ipv6-unicast"
	default:
		return fmt.Sprint("address-family-%d-%d", f.AFI, f.SAFI)
	}
}

func AddressFamilyFromString(name string) (AddressFamily, bool) {
	switch name {
	case "ipv4-unicast":
		return IPv4Unicast, true
	case "ipv6-unicast":
		return IPv6Unicast, true
	default:
		return AddressFamily{}, false
	}
}

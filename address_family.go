package main

import "fmt"

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

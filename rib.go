package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
)

type RIBEntry struct {
	Prefix  *net.IPNet
	Origin  Origin
	ASPath  ASPath
	NextHop NextHop

	OtherAttributes []PathAttribute
}

type RIB struct {
	// TODO: うまく lock 取るなりして複数 goroutine からアクセスしても安全にしたい
	// 現時点では Peer のメイン goroutine からしかアクセスされない
	Entries       map[*RIBEntry]struct{}
	OnRemoveFuncs []func(*RIBEntry) error
	OnUpdateFuncs []func(prev, curr *RIBEntry) error
}

func (rib *RIB) Find(prefix *net.IPNet) *RIBEntry {
	for e := range rib.Entries {
		if e.Prefix.IP.Equal(prefix.IP) && bytes.Equal([]byte(prefix.Mask), []byte(e.Prefix.Mask)) {
			return e
		}
	}
	return nil
}

func (rib *RIB) Remove(prefix *net.IPNet) error {
	e := rib.Find(prefix)
	if e == nil {
		return fmt.Errorf("no entry to remove: %v", prefix)
	}
	delete(rib.Entries, e)
	for _, onRemove := range rib.OnRemoveFuncs {
		if err := onRemove(e); err != nil {
			return err
		}
	}
	return nil
}

func (rib *RIB) Update(e *RIBEntry) error {
	prev := rib.Find(e.Prefix)
	if prev != nil {
		delete(rib.Entries, prev)
	}
	rib.Entries[e] = struct{}{}
	for _, onUpdate := range rib.OnUpdateFuncs {
		if err := onUpdate(prev, e); err != nil {
			return err
		}
	}
	return nil
}

func UpdateMessageToRIBEntries(m UpdateMessage) ([]*RIBEntry, error) {
	var (
		origin  Origin
		asPath  ASPath
		nextHop NextHop
		others  []PathAttribute

		err error
	)

	for _, a := range m.PathAttributes {
		switch a.TypeCode {
		case AttributeTypeOrigin:
			origin, err = OriginFromPathAttribute(a)
		case AttributeTypeASPath:
			asPath, err = ASPathFromPathAttribute(a)
		case AttributeTypeNextHop:
			nextHop, err = NextHopFromPathAttribute(a)
		default:
			others = append(others, a)
		}
		if err != nil {
			return nil, err
		}
	}

	entries := make([]*RIBEntry, len(m.NLRI))
	for i, r := range m.NLRI {
		entries[i] = &RIBEntry{
			Prefix:          r,
			Origin:          origin,
			ASPath:          asPath,
			NextHop:         nextHop,
			OtherAttributes: others, // TODO: Copy other attributes?
		}
	}
	return entries, nil
}

func (rib *RIB) Print(w io.Writer) {
	for e := range rib.Entries {
		fmt.Fprintf(w,
			"- %v (ORIGIN: %v, AS_PATH: %v, NEXTHOP: %v)\n",
			e.Prefix, e.Origin, e.ASPath.Segments, net.IP(e.NextHop),
		)
	}
}

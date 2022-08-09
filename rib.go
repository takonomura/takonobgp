package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"sync"
)

type RIBEntry struct {
	Prefix  *net.IPNet
	Origin  Origin
	ASPath  ASPath
	NextHop NextHop

	OtherAttributes []PathAttribute
}

type RIB struct {
	mutex         *sync.RWMutex
	entries       map[*RIBEntry]struct{}
	onRemoveFuncs []func(*RIBEntry) error
	onUpdateFuncs []func(prev, curr *RIBEntry) error
}

func NewRIB() *RIB {
	return &RIB{
		mutex:   new(sync.RWMutex),
		entries: make(map[*RIBEntry]struct{}),
	}
}

func (rib *RIB) OnRemove(fn func(*RIBEntry) error) int {
	rib.mutex.Lock()
	defer rib.mutex.Unlock()

	rib.onRemoveFuncs = append(rib.onRemoveFuncs, fn)
	return len(rib.onRemoveFuncs) - 1
}

func (rib *RIB) OnUpdate(fn func(prev, curr *RIBEntry) error) int {
	rib.mutex.Lock()
	defer rib.mutex.Unlock()

	rib.onUpdateFuncs = append(rib.onUpdateFuncs, fn)
	return len(rib.onUpdateFuncs) - 1
}

func (rib *RIB) Find(prefix *net.IPNet) *RIBEntry {
	rib.mutex.RLock()
	defer rib.mutex.RUnlock()
	return rib.find(prefix)
}

func (rib *RIB) find(prefix *net.IPNet) *RIBEntry {
	for e := range rib.entries {
		if e.Prefix.IP.Equal(prefix.IP) && bytes.Equal([]byte(prefix.Mask), []byte(e.Prefix.Mask)) {
			return e
		}
	}
	return nil
}

func (rib *RIB) Remove(prefix *net.IPNet) error {
	rib.mutex.Lock()

	e := rib.find(prefix)
	if e == nil {
		return fmt.Errorf("no entry to remove: %v", prefix)
	}
	delete(rib.entries, e)

	rib.mutex.Unlock()

	for _, onRemove := range rib.onRemoveFuncs {
		if err := onRemove(e); err != nil {
			return err
		}
	}
	return nil
}

func (rib *RIB) Update(e *RIBEntry) error {
	rib.mutex.Lock()

	prev := rib.find(e.Prefix)
	if prev != nil {
		delete(rib.entries, prev)
	}
	rib.entries[e] = struct{}{}

	rib.mutex.Unlock()

	for _, onUpdate := range rib.onUpdateFuncs {
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

func (rib *RIB) Entries() []*RIBEntry {
	rib.mutex.RLock()
	defer rib.mutex.RUnlock()

	s := make([]*RIBEntry, 0, len(rib.entries))
	for e := range rib.entries {
		s = append(s, e)
	}
	return s
}

func (rib *RIB) Print(w io.Writer) {
	rib.mutex.RLock()
	defer rib.mutex.RUnlock()

	for e := range rib.entries {
		fmt.Fprintf(w,
			"- %v (ORIGIN: %v, AS_PATH: %v, NEXTHOP: %v)\n",
			e.Prefix, e.Origin, e.ASPath.Segments, net.IP(e.NextHop),
		)
	}
}

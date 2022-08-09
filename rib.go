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
	NextHop net.IP

	OtherAttributes []PathAttribute

	Source *Peer
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

	for i, f := range rib.onRemoveFuncs {
		if f == nil {
			rib.onRemoveFuncs[i] = fn
			return i
		}
	}

	rib.onRemoveFuncs = append(rib.onRemoveFuncs, fn)
	return len(rib.onRemoveFuncs) - 1
}

func (rib *RIB) UnregisterOnRemove(id int) {
	rib.mutex.Lock()
	defer rib.mutex.Unlock()

	rib.onRemoveFuncs[id] = nil
}

func (rib *RIB) OnUpdate(fn func(prev, curr *RIBEntry) error) int {
	rib.mutex.Lock()
	defer rib.mutex.Unlock()

	for i, f := range rib.onUpdateFuncs {
		if f == nil {
			rib.onUpdateFuncs[i] = fn
			return i
		}
	}

	rib.onUpdateFuncs = append(rib.onUpdateFuncs, fn)
	return len(rib.onUpdateFuncs) - 1
}

func (rib *RIB) UnregisterOnUpdate(id int) {
	rib.mutex.Lock()
	defer rib.mutex.Unlock()

	rib.onUpdateFuncs[id] = nil
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

func (rib *RIB) Remove(e *RIBEntry) error {
	rib.mutex.Lock()
	defer rib.mutex.Unlock()

	delete(rib.entries, e)

	// XXX: ロック取った状態で呼ぶので、こいつらが更に RIB 操作しようとするとデッドロックする
	for _, onRemove := range rib.onRemoveFuncs {
		if onRemove == nil {
			continue
		}
		if err := onRemove(e); err != nil {
			return err
		}
	}
	return nil
}

func (rib *RIB) Update(e *RIBEntry) error {
	rib.mutex.Lock()
	defer rib.mutex.Unlock()

	prev := rib.find(e.Prefix)
	if prev != nil {
		delete(rib.entries, prev)
	}
	rib.entries[e] = struct{}{}

	// XXX: ロック取った状態で呼ぶので、こいつらが更に RIB 操作しようとするとデッドロックする
	for _, onUpdate := range rib.onUpdateFuncs {
		if onUpdate == nil {
			continue
		}
		if err := onUpdate(prev, e); err != nil {
			return err
		}
	}
	return nil
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

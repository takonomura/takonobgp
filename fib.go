package main

import (
	"log"
	"net"
)

type FIBSyncer struct {
	RIB *RIB
}

func (s *FIBSyncer) Register() {
	s.RIB.OnRemove(s.onRemove)
	s.RIB.OnUpdate(s.onUpdate)
}

func (s *FIBSyncer) onRemove(e *RIBEntry) error {
	log.Printf("RIB removed: %v", e)
	if len(e.Prefix.IP) != 4 { // TODO: Non IPv4
		return nil
	}
	if e.NextHop == nil {
		return nil
	}
	return ipRoute("del", e.Prefix.String())
}

func (s *FIBSyncer) onUpdate(prev, curr *RIBEntry) error {
	log.Printf("RIB updated: %v -> %v", prev, curr)
	if len(curr.Prefix.IP) != 4 { // TODO: Non IPv4
		return nil
	}
	if prev != nil && prev.NextHop != nil {
		if err := ipRoute("del", prev.Prefix.String()); err != nil {
			return err
		}
	}
	if curr.NextHop == nil {
		return nil
	}
	return ipRoute("add", curr.Prefix.String(), "via", net.IP(curr.NextHop).String())
}

func (s *FIBSyncer) Cleanup() {
	for _, e := range s.RIB.Entries() {
		if e.NextHop == nil {
			continue
		}
		if err := ipRoute("del", e.Prefix.String()); err != nil {
			log.Printf("cleaning FIB: %v", err)
		}
	}
}

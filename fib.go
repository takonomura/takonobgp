package main

import (
	"log"
	"net"
)

type FIBSyncer struct {
	RIB *RIB

	managed map[string]struct{}
}

func (s *FIBSyncer) Register() {
	s.RIB.OnRemove(s.onRemove)
	s.RIB.OnUpdate(s.onUpdate)
}

func (s *FIBSyncer) delete(e *RIBEntry) error {
	if _, ok := s.managed[e.Prefix.String()]; !ok {
		// not managed by us
		return nil
	}
	delete(s.managed, e.Prefix.String())
	return ipRoute("del", e.Prefix.String())
}

func (s *FIBSyncer) add(e *RIBEntry) error {
	err := ipRoute("add", e.Prefix.String(), "via", net.IP(e.NextHop).String())
	if err == nil {
		s.managed[e.Prefix.String()] = struct{}{}
	}
	return err
}

func (s *FIBSyncer) onRemove(e *RIBEntry) error {
	log.Printf("RIB removed: %v", e)
	if err := s.delete(e); err != nil {
		log.Printf("ERROR: %v", err)
	}
	return nil
}

func (s *FIBSyncer) onUpdate(prev, curr *RIBEntry) error {
	log.Printf("RIB updated: %v -> %v", prev, curr)
	if prev != nil {
		if err := s.delete(prev); err != nil {
			log.Printf("ERROR: %v", err)
		}
	}
	if curr.NextHop == nil {
		return nil
	}
	if err := s.add(curr); err != nil {
		log.Printf("ERROR: %v", err)
	}
	return nil
}

func (s *FIBSyncer) Cleanup() {
	for _, e := range s.RIB.Entries() {
		if e.NextHop == nil {
			continue
		}
		if err := s.delete(e); err != nil {
			log.Printf("cleaning FIB: %v", err)
		}
	}
}

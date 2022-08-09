package main

import (
	"log"
	"net"
	"os"
)

type FIBSyncer struct {
	RIB *RIB
}

func (s *FIBSyncer) Register() {
	s.RIB.OnRemoveFuncs = append(s.RIB.OnRemoveFuncs, s.onRemove)
	s.RIB.OnUpdateFuncs = append(s.RIB.OnUpdateFuncs, s.onUpdate)
}

func (s *FIBSyncer) onRemove(e *RIBEntry) error {
	log.Printf("RIB removed: %v", e)
	s.RIB.Print(os.Stderr)
	if e.NextHop == nil {
		return nil
	}
	return ipRoute("del", e.Prefix.String())
}

func (s *FIBSyncer) onUpdate(prev, curr *RIBEntry) error {
	log.Printf("RIB updated: %v -> %v", prev, curr)
	s.RIB.Print(os.Stderr)
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
	for e := range s.RIB.Entries {
		if e.NextHop == nil {
			continue
		}
		if err := ipRoute("del", e.Prefix.String()); err != nil {
			log.Printf("cleaning FIB: %v", err)
		}
	}
}

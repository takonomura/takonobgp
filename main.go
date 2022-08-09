package main

import (
	"context"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

func getenvOrDefault(name, def string) string {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	return v
}

func main() {
	myAS, err := strconv.ParseUint(getenvOrDefault("MY_ASN", "65001"), 10, 16)
	if err != nil {
		log.Fatalf("parsing MY_ASN: %v", err)
	}

	routerID := net.ParseIP(getenvOrDefault("ROUTER_ID", "10.0.0.1")).To4()
	if routerID == nil || len(routerID) != 4 {
		log.Fatalf("invalid ROUTER_ID")
	}
	var id [4]byte
	copy(id[:], routerID)

	for {
		p := &Peer{
			MyAS:            uint16(myAS),
			ID:              id,
			NeighborAddress: getenvOrDefault("NEIGHBOR_ADDR", "10.0.0.2"),

			LocalRIB: &RIB{
				Entries: make(map[*RIBEntry]struct{}),
			},

			HoldTime: 180,
			State:    StateIdle,

			eventChan: make(chan Event, 10),
			stopChan:  make(chan struct{}),
		}

		_, route, _ := net.ParseCIDR("10.1.0.0/24") // TODO: Configurable
		p.LocalRIB.Update(&RIBEntry{
			Prefix:  route,
			Origin:  OriginAttributeIGP,
			ASPath:  ASPath{Sequence: true, Segments: []uint16{}},
			NextHop: nil,
		})
		p.LocalRIB.OnRemoveFuncs = append(p.LocalRIB.OnRemoveFuncs, func(e *RIBEntry) error {
			log.Printf("RIB removed: %v", e)
			p.LocalRIB.Print(os.Stderr)
			if e.NextHop == nil {
				return nil
			}
			return ipRoute("del", e.Prefix.String())
		})
		p.LocalRIB.OnUpdateFuncs = append(p.LocalRIB.OnUpdateFuncs, func(prev, curr *RIBEntry) error {
			log.Printf("RIB updated: %v -> %v", prev, curr)
			p.LocalRIB.Print(os.Stderr)
			if prev != nil && prev.NextHop != nil {
				if err := ipRoute("del", prev.Prefix.String()); err != nil {
					return err
				}
			}
			if curr.NextHop == nil {
				return nil
			}
			return ipRoute("add", curr.Prefix.String(), "via", net.IP(curr.NextHop).String())
		})

		p.eventChan <- ManualStartEvent{}
		if err := p.Run(context.TODO()); err != nil {
			log.Printf("error: %v", err)
		}

		for e := range p.LocalRIB.Entries {
			if e.NextHop == nil {
				continue
			}
			if err := ipRoute("del", e.Prefix.String()); err != nil {
				log.Printf("cleaning FIB: %v", err)
			}
		}

		time.Sleep(time.Second)
	}
}

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
			ASPath:  ASPath{Sequence: true, Segments: []uint16{p.MyAS}},
			NextHop: NextHop(p.ID[:]),
		})

		p.eventChan <- ManualStartEvent{}
		if err := p.Run(context.TODO()); err != nil {
			log.Printf("error: %v", err)
		}

		time.Sleep(time.Second)
	}
}

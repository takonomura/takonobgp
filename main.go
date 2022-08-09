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
	rib := NewRIB()
	syncer := FIBSyncer{RIB: rib}
	syncer.Register()
	defer syncer.Cleanup() // XXX: Unreachable

	_, route, _ := net.ParseCIDR("10.1.0.0/24") // TODO: Configurable
	rib.Update(&RIBEntry{
		Prefix:  route,
		Origin:  OriginAttributeIGP,
		ASPath:  ASPath{Sequence: true, Segments: []uint16{}},
		NextHop: nil,
	})

	httpServer := &HTTPServer{RIB: rib}
	go httpServer.ListenAndServe("127.0.0.1:8080")

	for {
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

		p := NewPeer(PeerConfig{
			MyAS:            uint16(myAS),
			RouterID:        id,
			NeighborAddress: getenvOrDefault("NEIGHBOR_ADDR", "10.0.0.2"),
			LocalRIB:        rib,
			HoldTime:        180,
		})

		if err := p.Run(context.TODO()); err != nil {
			log.Printf("error: %v", err)
		}

		time.Sleep(time.Second)
	}
}

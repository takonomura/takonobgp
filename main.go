package main

import (
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

	p := &Peer{
		MyAS:     uint16(myAS),
		ID:       id,
		HoldTime: 180,
	}
	for {
		log.Printf("connect: %v", p.connect("tcp", getenvOrDefault("NEIGHBOR_ADDR", "10.0.0.2")))
		time.Sleep(time.Second)
	}
}

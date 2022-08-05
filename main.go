package main

import (
	"log"
	"net"
	"os"
	"strconv"
)

func getenv(name string) string {
	v := os.Getenv(name)
	if v == "" {
		log.Fatalf("%s is not specified", name)
	}
	return v
}

func main() {
	myAS, err := strconv.ParseUint(getenv("MY_ASN"), 10, 16)
	if err != nil {
		log.Fatalf("parsing MY_ASN: %v", err)
	}

	routerID := net.ParseIP(getenv("ROUTER_ID"))
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
	log.Fatal(p.connect("tcp", getenv("NEIGHBOR_ADDR")))
}

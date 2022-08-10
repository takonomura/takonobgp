package main

import (
	"context"
	"log"
	"os"
	"time"
)

func getenvOrDefault(name, def string) string {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	return v
}

func loadConfigFile(ribs map[AddressFamily]*RIB) (Config, error) {
	f, err := os.Open("config.json")
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	return LoadConfig(f, ribs)
}

func main() {
	ribs := map[AddressFamily]*RIB{
		IPv4Unicast: NewRIB(),
		IPv6Unicast: NewRIB(),
	}
	for _, rib := range ribs {
		syncer := FIBSyncer{RIB: rib, managed: make(map[string]struct{})}
		syncer.Register()
		defer syncer.Cleanup() // XXX: Unreachable
	}

	cfg, err := loadConfigFile(ribs)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	for _, r := range cfg.Networks {
		var af AddressFamily
		switch len(r.IP) {
		case 4:
			af = IPv4Unicast
		case 16:
			af = IPv6Unicast
		default:
			log.Fatalf("invalid network: %v", r)
		}
		ribs[af].Update(&RIBEntry{
			AF:      af,
			Prefix:  r,
			Origin:  OriginAttributeIGP,
			ASPath:  ASPath{Sequence: true, Segments: []uint16{}},
			NextHop: nil,
		})
	}

	go (&HTTPServer{
		AF:  IPv4Unicast,
		RIB: ribs[IPv4Unicast],
	}).ListenAndServe("127.0.0.1:8080")
	go (&HTTPServer{
		AF:  IPv6Unicast,
		RIB: ribs[IPv6Unicast],
	}).ListenAndServe("127.0.0.1:8686")

	for {
		p := NewPeer(cfg.Peer)

		if err := p.Run(context.TODO()); err != nil {
			log.Printf("error: %v", err)
		}

		time.Sleep(time.Second)
	}
}

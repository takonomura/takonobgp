package main

import (
	"context"
	"log"
	"net"
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

func mustIPNet(cidr string) *net.IPNet {
	_, n, _ := net.ParseCIDR(cidr)
	return n
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
		IPv6VPN: NewRIB(),
	}

	cfg, err := loadConfigFile(ribs)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	for {
		p := NewPeer(cfg.Peer)

		if err := p.Run(context.TODO()); err != nil {
			log.Printf("error: %v", err)
		}

		time.Sleep(time.Second)
	}
}

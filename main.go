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

func loadConfigFile() (Config, error) {
	f, err := os.Open("config.json")
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	return LoadConfig(f)
}

func main() {
	rib := NewRIB()
	syncer := FIBSyncer{RIB: rib}
	syncer.Register()
	defer syncer.Cleanup() // XXX: Unreachable

	cfg, err := loadConfigFile()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	cfg.Peer.LocalRIB = rib

	for _, r := range cfg.Networks {
		rib.Update(&RIBEntry{
			Prefix:  r,
			Origin:  OriginAttributeIGP,
			ASPath:  ASPath{Sequence: true, Segments: []uint16{}},
			NextHop: nil,
		})
	}

	httpServer := &HTTPServer{RIB: rib}
	go httpServer.ListenAndServe("127.0.0.1:8080")

	for {
		p := NewPeer(cfg.Peer)

		if err := p.Run(context.TODO()); err != nil {
			log.Printf("error: %v", err)
		}

		time.Sleep(time.Second)
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
)

type Config struct {
	Networks []*net.IPNet
	Peer     PeerConfig
}

func LoadConfig(r io.Reader, ribs map[AddressFamily]*RIB) (Config, error) {
	var aux struct {
		Networks []string `json:"networks"`
		Peer     struct {
			MyAS     uint16 `json:"as"`
			RouterID string `json:"router_id"`
			Neighbor string `json:"neighbor"`

			AddressFamilies map[string]struct {
				NextHop string `json:"next_hop"`
			} `json:"address_families"`
		} `json:"peer"`
	}
	if err := json.NewDecoder(r).Decode(&aux); err != nil {
		return Config{}, err
	}
	cfg := Config{
		Networks: make([]*net.IPNet, len(aux.Networks)),
		Peer: PeerConfig{
			MyAS:            aux.Peer.MyAS,
			NeighborAddress: aux.Peer.Neighbor,
			HoldTime:        180,
		},
	}
	for i, s := range aux.Networks {
		_, r, err := net.ParseCIDR(s)
		if err != nil {
			return Config{}, fmt.Errorf("network cidr: %w", err)
		}
		cfg.Networks[i] = r
	}

	id := net.ParseIP(aux.Peer.RouterID).To4()
	if id == nil || len(id) != 4 {
		return Config{}, fmt.Errorf("invalid router id: %q", aux.Peer.RouterID)
	}
	copy(cfg.Peer.RouterID[:], id)

	cfg.Peer.AddressFamilies = make(map[AddressFamily]AddressFamilyConfig, len(aux.Peer.AddressFamilies))
	for name, v := range aux.Peer.AddressFamilies {
		af, ok := AddressFamilyFromString(name)
		if !ok {
			return Config{}, fmt.Errorf("invalid address family name: %q", name)
		}

		nextHop := net.ParseIP(v.NextHop)
		if af.NextHopSize() == 4 {
			nextHop = nextHop.To4()
		}
		if nextHop == nil {
			return Config{}, fmt.Errorf("invalid next hop: %q", v.NextHop)
		}
		if len(nextHop) != af.NextHopSize() {
			return Config{}, fmt.Errorf("invalid next hop length: %q (%d)", nextHop, len(nextHop))
		}

		cfg.Peer.AddressFamilies[af] = AddressFamilyConfig{
			SelfNextHop: nextHop,
			LocalRIB:    ribs[af],
		}
	}

	return cfg, nil
}

package main

import (
	"fmt"
	"log"
	"net"
	"sync"
)

type Peer struct {
	MyAS uint16
	ID   [4]byte

	HoldTime uint16

	conn net.Conn
}

func (p *Peer) sendMessage(m Message) error {
	_, err := m.WriteTo(p.conn)
	return err
}

func (p *Peer) connect(network, address string) error {
	log.Printf("connecting to %q", address)
	var err error
	if p.conn, err = net.Dial(network, address); err != nil {
		return err
	}
	defer p.conn.Close()

	log.Printf("sending open message")

	if err := p.sendMessage(OpenMessage{
		Version:  4,
		MyAS:     p.MyAS,
		HoldTime: p.HoldTime,
		BGPID:    p.ID,
	}); err != nil {
		return fmt.Errorf("send open message: %w", err)
	}

	log.Printf("receiving message")

	m, err := ReadPacket(p.conn)
	if err != nil {
		return err
	}

	switch m := m.(type) {
	case OpenMessage:
		log.Printf("received open message: %+v", m)
	default:
		return fmt.Errorf("unexpected message: %T (%+v)", m, m)
	}

	log.Printf("sending keepalive message")
	if err := p.sendMessage(KeepaliveMessage{}); err != nil {
		return fmt.Errorf("send keepalive message: %w", err)
	}

	var sendUpdateOnce sync.Once
	for {
		log.Printf("receiving message")

		m, err = ReadPacket(p.conn)
		if err != nil {
			return err
		}

		switch m := m.(type) {
		case KeepaliveMessage:
			log.Printf("received keepalive message")
			if err := p.sendMessage(KeepaliveMessage{}); err != nil {
				return fmt.Errorf("send pong: %w", err)
			}
			// 相手から keepalive が帰ってきて established になるはずなので、初回の keepalive 後に UPDATE を送る
			sendUpdateOnce.Do(func() {
				log.Printf("sending update message")
				_, route, _ := net.ParseCIDR("10.1.0.0/24") // TODO: Configurable
				err = p.sendMessage(UpdateMessage{
					PathAttributes: []PathAttribute{
						OriginAttributeIGP.ToPathAttribute(),
						ASPathAttribute{Sequence: true, Segments: []uint16{p.MyAS}}.ToPathAttribute(),
						NextHopAttribute(p.ID[:]).ToPathAttribute(),
					},
					NLRI: []*net.IPNet{
						route,
					},
				})
			})
			if err != nil {
				return fmt.Errorf("send update message: %w", err)
			}
		default:
			log.Printf("received message: %T (%+v)", m, m)
		}
	}
}

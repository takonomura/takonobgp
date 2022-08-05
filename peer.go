package main

import (
	"fmt"
	"log"
	"net"
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

	for {
		log.Printf("receiving message")

		m, err = ReadPacket(p.conn)
		if err != nil {
			return err
		}

		switch m := m.(type) {
		case KeepaliveMessage:
			log.Printf("received keepalive message: %+v", m)
		default:
			log.Printf("received message: %T (%+v)", m, m)
		}
	}
}

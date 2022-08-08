package main

import (
	"context"
	"log"
	"net"
	"time"
)

type State string

const (
	StateIdle        State = "idle"
	StateConnect     State = "connect"
	StateActive      State = "active"
	StateOpenSent    State = "open_sent"
	StateOpenConfirm State = "open_confirm"
	StateEstablished State = "established"
)

type Peer struct {
	MyAS            uint16
	ID              [4]byte
	NeighborAddress string

	HoldTime uint16

	State     State
	conn      net.Conn
	stopChan  chan struct{}
	eventChan chan Event

	holdTimer      *time.Ticker
	keepaliveTimer *time.Ticker
}

func (p *Peer) Run(ctx context.Context) error {
	defer func() {
		if p.conn != nil {
			p.conn.Close()
		}
		close(p.stopChan)
		// TODO: Wait all related goroutines
	}()
	for {
		select {
		case e := <-p.eventChan:
			log.Printf("event: %+v", e)
			if err := e.Do(p); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (p *Peer) setState(s State) {
	log.Printf("peer state changed: %v -> %v", p.State, s)
	p.State = s
}

func (p *Peer) sendMessage(m Message) error {
	_, err := m.WriteTo(p.conn)
	return err
}

func (p *Peer) receiveMessages() error {
	log.Printf("receiving messages")
	for {
		m, err := ReadPacket(p.conn)
		if err != nil {
			return err
		}
		log.Printf("received message: %T (%+v)", m, m)

		switch m := m.(type) { // TODO: switch なくしたい
		case OpenMessage:
			p.eventChan <- OpenMessageEvent{m}
		case UpdateMessage:
			p.eventChan <- UpdateMessageEvent{m}
		case NotificationMessage:
			p.eventChan <- NotificationMessageEvent{m}
		case KeepaliveMessage:
			p.eventChan <- KeepaliveMessageEvent{}
		default:
		}
	}

}

func (p *Peer) startHoldTimer() {
	p.holdTimer = time.NewTicker(time.Duration(p.HoldTime) * time.Second)
	go func() {
		defer p.holdTimer.Stop()
		for {
			select {
			case <-p.holdTimer.C:
				// TODO: Hold timer exceeded
			case <-p.stopChan:
				return
			}
		}
	}()
}

func (p *Peer) startKeepaliveTimer() {
	// TODO: Configurable
	p.keepaliveTimer = time.NewTicker(time.Duration(p.HoldTime/3) * time.Second)
	go func() {
		defer p.keepaliveTimer.Stop()
		for {
			select {
			case <-p.keepaliveTimer.C:
				p.eventChan <- KeepaliveTimerExpireEvent{}
			case <-p.stopChan:
				return
			}
		}
	}()
}

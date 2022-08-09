package main

import (
	"context"
	"log"
	"net"
	"sync"
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

type PeerConfig struct {
	MyAS     uint16
	RouterID [4]byte

	NeighborAddress string

	LocalRIB *RIB

	HoldTime uint16
}

type Peer struct {
	MyAS            uint16
	RouterID        [4]byte
	NeighborAddress string

	LocalRIB *RIB

	HoldTime uint16

	State State
	conn  net.Conn
	wg    *sync.WaitGroup

	stopChan  chan struct{}
	eventChan chan Event

	holdTimer      *time.Ticker
	keepaliveTimer *time.Ticker

	ribOnRemoveID int
	ribOnUpdateID int
}

func NewPeer(cfg PeerConfig) *Peer {
	return &Peer{
		MyAS:            cfg.MyAS,
		RouterID:        cfg.RouterID,
		NeighborAddress: cfg.NeighborAddress,
		LocalRIB:        cfg.LocalRIB,
		HoldTime:        cfg.HoldTime,
		State:           StateIdle,
		wg:              new(sync.WaitGroup),
		stopChan:        make(chan struct{}),
		eventChan:       make(chan Event, 10),
		ribOnRemoveID:   -1,
		ribOnUpdateID:   -1,
	}
}

func (p *Peer) Run(ctx context.Context) error {
	defer func() {
		if p.ribOnRemoveID != -1 {
			p.LocalRIB.UnregisterOnRemove(p.ribOnRemoveID)
		}
		if p.ribOnUpdateID != -1 {
			p.LocalRIB.UnregisterOnUpdate(p.ribOnUpdateID)
		}

		if p.conn != nil {
			p.conn.Close()
		}
		close(p.stopChan)

		for _, e := range p.LocalRIB.Entries() {
			if e.Source == p {
				p.LocalRIB.Remove(e)
			}
		}

		p.wg.Wait()
	}()

	p.eventChan <- ManualStartEvent{}
	for {
		select {
		case e := <-p.eventChan:
			log.Printf("event: %T (%+v)", e, e)
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

func (p *Peer) startTimers() {
	p.holdTimer = time.NewTicker(time.Duration(p.HoldTime) * time.Second)
	p.keepaliveTimer = time.NewTicker(time.Duration(p.HoldTime/3) * time.Second)
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer p.holdTimer.Stop()
		defer p.keepaliveTimer.Stop()
		for {
			select {
			case <-p.holdTimer.C:
				p.eventChan <- HoldTimerExpireEvent{}
			case <-p.keepaliveTimer.C:
				p.eventChan <- KeepaliveTimerExpireEvent{}
			case <-p.stopChan:
				return
			}
		}
	}()
}

func (p *Peer) onLocalRIBRemove(e *RIBEntry) error {
	p.eventChan <- LocalRIBUpdateEvent{
		Removed: []*net.IPNet{e.Prefix},
	}
	return nil
}

func (p *Peer) onLocalRIBUpdate(prev, curr *RIBEntry) error {
	p.eventChan <- LocalRIBUpdateEvent{
		Updated: []*RIBEntry{curr},
	}
	return nil
}

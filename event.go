package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

type (
	Event interface {
		Do(*Peer) error
	}

	ManualStartEvent struct{}
	TcpCRAckedEvent  struct{}

	OpenMessageEvent struct {
		Message OpenMessage
	}
	UpdateMessageEvent struct {
		Message UpdateMessage
	}
	NotificationMessageEvent struct {
		Message NotificationMessage
	}
	KeepaliveMessageEvent struct{}

	HoldTimerExpireEvent      struct{}
	KeepaliveTimerExpireEvent struct{}

	LocalRIBUpdateEvent struct {
		Removed []*net.IPNet
		Updated []*RIBEntry
	}
)

func (e ManualStartEvent) Do(p *Peer) error {
	if p.State != StateIdle {
		return fmt.Errorf("unexpected state: %v", p.State)
	}
	p.setState(StateConnect)
	var err error
	if p.conn, err = net.Dial("tcp", net.JoinHostPort(p.NeighborAddress, "179")); err != nil {
		return err
	}
	p.eventChan <- TcpCRAckedEvent{}
	return nil
}

func (e TcpCRAckedEvent) Do(p *Peer) error {
	if p.State != StateConnect {
		return fmt.Errorf("unexpected state: %v", p.State)
	}
	if err := p.sendMessage(OpenMessage{
		Version:  4,
		MyAS:     p.MyAS,
		HoldTime: p.HoldTime,
		BGPID:    p.RouterID,
		OptionalParameters: []byte{ // TODO: もっといい感じに...
			2,    // Parameter Type: 2 = Capability
			6,    // Parameter Length
			1,    // Capability Code: 1 = Multiprotocol Extensions
			4,    // Capability Length
			0, 1, // AFI: 1 = IPv4
			0,    // Reserved
			1,    // SAFI: 1 = Unicast
			2,    // Parameter Type: 2 = Capability
			6,    // Parameter Length
			1,    // Capability Code: 1 = Multiprotocol Extensions
			4,    // Capability Length
			0, 2, // AFI: 2 = IPv6
			0, // Reserved
			1, // SAFI: 1 = Unicast
		},
	}); err != nil {
		return fmt.Errorf("send open message: %w", err)
	}
	p.setState(StateOpenSent)

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.receiveMessages()
	}()
	return nil
}

func (e OpenMessageEvent) Do(p *Peer) error {
	if p.State != StateOpenSent {
		return fmt.Errorf("unexpected state: %v", p.State)
	}
	// TODO: 中身ちゃんと見る
	p.setState(StateOpenConfirm)
	if err := p.sendMessage(KeepaliveMessage{}); err != nil {
		return fmt.Errorf("send keepalive message: %w", err)
	}
	return nil
}

func (e UpdateMessageEvent) Do(p *Peer) error {
	if p.State != StateEstablished {
		return fmt.Errorf("unexpected state: %v", p.State)
	}
	ws, es, err := UpdateMessageToRIBEntries(e.Message, p)
	if err != nil {
		return err
	}
	for _, r := range ws {
		e := p.LocalRIB.Find(r)
		if e == nil || e.Source != p {
			continue
		}
		if err := p.LocalRIB.Remove(e); err != nil {
			return err
		}
	}
	for _, e := range es {
		curr := p.LocalRIB.Find(e.Prefix)
		if curr != nil {
			if len(curr.ASPath.Segments) < len(e.ASPath.Segments) {
				log.Printf("ignore update for %v (entry in RIB has priority)", e.Prefix)
				continue
			}
		}
		if err := p.LocalRIB.Update(e); err != nil {
			return err
		}
	}
	return nil
}

func (e NotificationMessageEvent) Do(p *Peer) error {
	return fmt.Errorf("notification received: %+v", p)
}

func (e KeepaliveMessageEvent) Do(p *Peer) error {
	switch p.State {
	case StateOpenConfirm:
		p.setState(StateEstablished)
		p.startTimers()
		p.LocalRIB.OnRemove(p.onLocalRIBRemove)
		p.LocalRIB.OnUpdate(p.onLocalRIBUpdate)

		log.Printf("sending initial update messages")
		for _, e := range p.LocalRIB.Entries() {
			if err := p.sendMessage(CreateUpdateMessage(e, p.MyAS, net.IP(p.RouterID[:]))); err != nil {
				return fmt.Errorf("send update message: %w", err)
			}
		}
		return nil
	case StateEstablished:
		p.holdTimer.Reset(time.Duration(p.HoldTime) * time.Second)
		return nil
	default:
		return fmt.Errorf("unexpected state: %v", p.State)
	}
}

func (e HoldTimerExpireEvent) Do(p *Peer) error {
	// TODO: Send notification?
	return fmt.Errorf("hold timer expired")
}

func (e KeepaliveTimerExpireEvent) Do(p *Peer) error {
	if err := p.sendMessage(KeepaliveMessage{}); err != nil {
		return fmt.Errorf("send keepalive message: %w", err)
	}
	return nil
}

func (e LocalRIBUpdateEvent) Do(p *Peer) error {
	// Withdrawn
	if len(e.Removed) > 0 {
		for _, r := range e.Removed {
			if err := p.sendMessage(CreateWithdrawnMessage(r)); err != nil {
				return fmt.Errorf("send withdrawn update message: %w", err)
			}
		}
	}
	// Update
	for _, e := range e.Updated {
		if err := p.sendMessage(CreateUpdateMessage(e, p.MyAS, net.IP(p.RouterID[:]))); err != nil {
			return fmt.Errorf("send update message: %w", err)
		}
	}
	return nil
}

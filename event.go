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
		BGPID:    p.ID,
	}); err != nil {
		return fmt.Errorf("send open message: %w", err)
	}
	p.setState(StateOpenSent)
	go p.receiveMessages()
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
	for _, r := range e.Message.WirhdrawnRoutes {
		// TODO: Check the entry is from the peer
		if err := p.LocalRIB.Remove(r); err != nil {
			return err
		}
	}
	es, err := UpdateMessageToRIBEntries(e.Message)
	if err != nil {
		return err
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
		go p.startHoldTimer()
		go p.startKeepaliveTimer()
		p.LocalRIB.OnRemove(p.onLocalRIBRemove)
		p.LocalRIB.OnUpdate(p.onLocalRIBUpdate)

		log.Printf("sending initial update messages")
		for _, e := range p.LocalRIB.Entries() {
			pathAttributes := []PathAttribute{
				e.Origin.ToPathAttribute(),
				ASPath{
					Sequence: e.ASPath.Sequence,
					Segments: append([]uint16{p.MyAS}, e.ASPath.Segments...),
				}.ToPathAttribute(),
				e.NextHop.ToPathAttribute(),
			}
			if e.NextHop == nil {
				pathAttributes[2] = NextHop(p.ID[:]).ToPathAttribute()
			}
			if err := p.sendMessage(UpdateMessage{
				PathAttributes: append(pathAttributes, e.OtherAttributes...),
				NLRI:           []*net.IPNet{e.Prefix},
			}); err != nil {
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
		if err := p.sendMessage(UpdateMessage{
			WirhdrawnRoutes: e.Removed,
		}); err != nil {
			return fmt.Errorf("send withdrawn update message: %w", err)
		}
	}
	// Update
	for _, e := range e.Updated {
		pathAttributes := []PathAttribute{
			e.Origin.ToPathAttribute(),
			ASPath{
				Sequence: e.ASPath.Sequence,
				Segments: append([]uint16{p.MyAS}, e.ASPath.Segments...),
			}.ToPathAttribute(),
			e.NextHop.ToPathAttribute(),
		}
		if e.NextHop == nil {
			pathAttributes[2] = NextHop(p.ID[:]).ToPathAttribute()
		}
		if err := p.sendMessage(UpdateMessage{
			PathAttributes: append(pathAttributes, e.OtherAttributes...),
			NLRI:           []*net.IPNet{e.Prefix},
		}); err != nil {
			return fmt.Errorf("send update message: %w", err)
		}
	}
	return nil
}

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
	// TODO: RIB に書き込む
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

		log.Printf("sending update message")
		_, route, _ := net.ParseCIDR("10.1.0.0/24") // TODO: Configurable
		if err := p.sendMessage(UpdateMessage{
			PathAttributes: []PathAttribute{
				OriginAttributeIGP.ToPathAttribute(),
				ASPath{Sequence: true, Segments: []uint16{p.MyAS}}.ToPathAttribute(),
				NextHop(p.ID[:]).ToPathAttribute(),
			},
			NLRI: []*net.IPNet{
				route,
			},
		}); err != nil {
			return fmt.Errorf("send update message: %w", err)
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

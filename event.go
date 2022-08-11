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
	if p.conn, err = net.DialTimeout("tcp", net.JoinHostPort(p.NeighborAddress, "179"), 5*time.Second); err != nil {
		return err
	}
	p.eventChan <- TcpCRAckedEvent{}
	return nil
}

func (e TcpCRAckedEvent) Do(p *Peer) error {
	if p.State != StateConnect {
		return fmt.Errorf("unexpected state: %v", p.State)
	}
	var opts []byte
	for af := range p.AddressFamilies {
		opts = append(opts, MultiprotocolExtensionCapability{af}.ToOptionalParameter()...)
	}
	if err := p.sendMessage(OpenMessage{
		Version:            4,
		MyAS:               p.MyAS,
		HoldTime:           p.HoldTime,
		BGPID:              p.RouterID,
		OptionalParameters: opts,
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
		rib := p.AddressFamilies[r.AF].LocalRIB
		e := rib.Find(r.Prefix)
		if e == nil || e.Source != p {
			continue
		}
		if err := rib.Remove(e); err != nil {
			return err
		}
	}
	for _, e := range es {
		rib := p.AddressFamilies[e.AF].LocalRIB
		curr := rib.Find(e.Prefix)
		if curr != nil {
			if len(curr.ASPath.Segments) < len(e.ASPath.Segments) {
				log.Printf("ignore update for %v (entry in RIB has priority)", e.Prefix)
				continue
			}
		}
		if err := rib.Update(e); err != nil {
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
		p.registerLocalRIBHandlers()

		log.Printf("sending initial update messages")
		for af, f := range p.AddressFamilies {
			for _, e := range f.LocalRIB.Entries() {
				if e.AF != af {
					continue // 違う AF が RIB に混ざってても広報しない (共有 RIB になった時対策)
				}
				if err := p.sendMessage(CreateUpdateMessage(e, p.MyAS, p.AddressFamilies[e.AF].SelfNextHop)); err != nil {
					return fmt.Errorf("send update message: %w", err)
				}
			}
		}

		p.sendMessage(UpdateMessage{
			PathAttributes: []PathAttribute{
				MPReachNLRI{
					AF:      IPv6VPN,
					NextHop: []net.IP{p.AddressFamilies[IPv6VPN].SelfNextHop},
					NLRI: []NLRI{
						LabeledVPNNLRI{
							Labels: []Label{NewLabel(0x0100_0, 3)},
							RD:     NewRD(1, 100),
							IPNet:  mustIPNet("2001:bb11::/64"),
						},
					},
				}.ToPathAttribute(),
				OriginAttributeIncomplete.ToPathAttribute(),
				ASPath{Sequence: true, Segments: []uint16{1}}.ToPathAttribute(),
				ExtendedCommunities{
					ExtendedCommunity{
						Type:      ExtendedCommunityType{0x00, 0x02},
						Community: []byte{00, 99, 00, 00, 00, 99},
					},
				}.ToPathAttribute(),
				SRv6ServiceTLV{
					Type: PrefixSIDTLVTypeSRv6L3Service,
					SubTLVs: []SRv6ServiceSubTLV{SRv6SIDInfoSubTLV{
						SID:              net.ParseIP("2001:1111::"),
						Flags:            0x00,
						EndpointBehavior: 0xFFFF,
						SubSubTLV: []SRv6ServiceDataSubSubTLV{SRv6SIDStructureSubSubTLV{
							LocatorBlockLength:  40,
							LocatorNodeLength:   24,
							FunctionLength:      16,
							ArgumentLength:      0,
							TranspositionLength: 16,
							TranspositionOffset: 64,
						}},
					},
					},
				}.ToPathAttribute(),
			},
		})

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
		if err := p.sendMessage(CreateUpdateMessage(e, p.MyAS, p.AddressFamilies[e.AF].SelfNextHop)); err != nil {
			return fmt.Errorf("send update message: %w", err)
		}
	}
	return nil
}

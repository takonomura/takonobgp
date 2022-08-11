package main

import (
	"fmt"
	"net"
)

type WithdrawnRoute struct {
	AF     AddressFamily
	Prefix *net.IPNet
}

func UpdateMessageToRIBEntries(m UpdateMessage, source *Peer) ([]WithdrawnRoute, []*RIBEntry, error) {
	var (
		origin  Origin
		asPath  ASPath
		nextHop NextHop
		others  []PathAttribute

		mpReach   MPReachNLRI
		mpUnreach MPUnreachNLRI

		err error
	)

	for _, a := range m.PathAttributes {
		switch a.TypeCode {
		case AttributeTypeOrigin:
			origin, err = OriginFromPathAttribute(a)
		case AttributeTypeASPath:
			asPath, err = ASPathFromPathAttribute(a)
		case AttributeTypeNextHop:
			nextHop, err = NextHopFromPathAttribute(a)
		case AttributeTypeMPReachNLRI:
			mpReach, err = MPReachNLRIFromPathAttribute(a)
		case AttributeTypeMPUnreachNLRI:
			mpUnreach, err = MPUnreachNLRIFromPathAttribute(a)
		default:
			others = append(others, a)
		}
		if err != nil {
			return nil, nil, err
		}
	}

	withdrawns := make([]WithdrawnRoute, 0, len(m.WirhdrawnRoutes)+len(mpUnreach.WithdrawnRoutes))
	for _, r := range m.WirhdrawnRoutes {
		withdrawns = append(withdrawns, WithdrawnRoute{
			AF:     IPv4Unicast,
			Prefix: r,
		})
	}
	for _, r := range mpUnreach.WithdrawnRoutes {
		withdrawns = append(withdrawns, WithdrawnRoute{
			AF:     mpUnreach.AF,
			Prefix: r,
		})
	}

	entries := make([]*RIBEntry, 0, len(mpReach.NLRI)+len(m.NLRI))
	for _, r := range mpReach.NLRI {
		entries = append(entries, &RIBEntry{
			AF:              mpReach.AF,
			Prefix:          r.Prefix(),
			Origin:          origin,
			ASPath:          asPath,
			NextHop:         mpReach.NextHop[0], // TODO: Select best
			OtherAttributes: others,
			Source:          source,
		})
	}
	for _, r := range m.NLRI {
		entries = append(entries, &RIBEntry{
			AF:              IPv4Unicast,
			Prefix:          r,
			Origin:          origin,
			ASPath:          asPath,
			NextHop:         net.IP(nextHop),
			OtherAttributes: others, // TODO: Copy other attributes?
			Source:          source,
		})
	}

	return withdrawns, entries, nil
}

func CreateWithdrawnMessage(r *net.IPNet) UpdateMessage {
	switch len(r.IP) {
	case 4: // IPv4
		return UpdateMessage{
			WirhdrawnRoutes: []*net.IPNet{r},
		}
	case 16: // IPv6
		return UpdateMessage{
			PathAttributes: []PathAttribute{
				MPUnreachNLRI{
					AF:              IPv6Unicast,
					WithdrawnRoutes: []*net.IPNet{r},
				}.ToPathAttribute(),
			},
		}
	default:
		panic(fmt.Errorf("unexpected withdrawn prefix: %v", r))
	}
}

func CreateUpdateMessage(e *RIBEntry, prependAS uint16, selfNextHop net.IP) UpdateMessage {
	nextHop := e.NextHop
	if nextHop == nil {
		nextHop = selfNextHop
	}
	switch len(e.Prefix.IP) {
	case 4:
		pathAttributes := []PathAttribute{
			e.Origin.ToPathAttribute(),
			ASPath{
				Sequence: e.ASPath.Sequence,
				Segments: append([]uint16{prependAS}, e.ASPath.Segments...),
			}.ToPathAttribute(),
			NextHop(nextHop).ToPathAttribute(),
		}
		return UpdateMessage{
			PathAttributes: append(pathAttributes, e.OtherAttributes...),
			NLRI:           []*net.IPNet{e.Prefix},
		}
	case 16:
		pathAttributes := []PathAttribute{
			e.Origin.ToPathAttribute(),
			ASPath{
				Sequence: e.ASPath.Sequence,
				Segments: append([]uint16{prependAS}, e.ASPath.Segments...),
			}.ToPathAttribute(),
			MPReachNLRI{
				AF:      IPv6Unicast,
				NextHop: []net.IP{nextHop},
				NLRI:    []NLRI{UnicastNLRI{e.Prefix}},
			}.ToPathAttribute(),
		}
		return UpdateMessage{
			PathAttributes: append(pathAttributes, e.OtherAttributes...),
		}
	default:
		panic(fmt.Errorf("unexpected rib entry: %v", e))
	}
}

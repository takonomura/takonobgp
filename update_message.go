package main

import "net"

func UpdateMessageToRIBEntries(m UpdateMessage, source *Peer) ([]*RIBEntry, error) {
	var (
		origin  Origin
		asPath  ASPath
		nextHop NextHop
		others  []PathAttribute

		mpReach MPReachNLRI
		//mpUnreach MPUnreachNLRI

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
		//case AttributeTypeMPUnreachNLRI:
		//        mpUnreach, err = MPUnreachNLRIFromPathAttribute(a)
		default:
			others = append(others, a)
		}
		if err != nil {
			return nil, err
		}
	}

	entries := make([]*RIBEntry, 0, len(mpReach.NLRI)+len(m.NLRI))

	for _, r := range mpReach.NLRI {
		entries = append(entries, &RIBEntry{
			Prefix:          r,
			Origin:          origin,
			ASPath:          asPath,
			NextHop:         mpReach.NextHop[0],
			OtherAttributes: others,
			Source:          source,
		})
	}
	for _, r := range m.NLRI {
		entries = append(entries, &RIBEntry{
			Prefix:          r,
			Origin:          origin,
			ASPath:          asPath,
			NextHop:         net.IP(nextHop),
			OtherAttributes: others, // TODO: Copy other attributes?
			Source:          source,
		})
	}
	return entries, nil
}

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

const (
	markerSize = 16
	headerSize = 19 // marker (16) + length (2) + type (1)
)

type MessageType uint8

const (
	MessageTypeOpen MessageType = iota + 1
	MessageTypeUpdate
	MessageTypeNotification
	MessageTypeKeepalive
)

type Message interface {
	io.WriterTo
}

func createHeader(l uint16, t MessageType) [headerSize]byte {
	var b [headerSize]byte
	for i := 0; i < markerSize; i++ {
		b[i] = 0xFF
	}
	binary.BigEndian.PutUint16(b[markerSize:markerSize+2], l)
	b[headerSize-1] = uint8(t)
	return b
}

type UnknownMessage struct {
	Type    MessageType
	Payload []byte
}

func (m UnknownMessage) WriteTo(w io.Writer) (int64, error) {
	buf := bytes.NewBuffer(make([]byte, 0, headerSize+len(m.Payload)))
	header := createHeader(uint16(headerSize+len(m.Payload)), m.Type)
	buf.Write(header[:])
	buf.Write(m.Payload)
	return buf.WriteTo(w)
}

func ReadPacket(r io.Reader) (Message, error) {
	var header [headerSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}

	for i := 0; i < markerSize; i++ {
		if header[i] != 0xFF {
			return nil, fmt.Errorf("invalid message marker: %x", header[:markerSize])
		}
	}

	size := binary.BigEndian.Uint16(header[markerSize : markerSize+2])
	if size < headerSize || size > 4096 {
		return nil, fmt.Errorf("invalid message length: %d", size)
	}

	buf := make([]byte, size-headerSize) // TODO: Pool
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}

	t := MessageType(header[headerSize-1])
	switch t {
	case MessageTypeOpen:
		return ParseOpenMessage(buf)
	case MessageTypeUpdate:
		return ParseUpdateMessage(buf)
	case MessageTypeNotification:
		return ParseNotificationMessage(buf)
	case MessageTypeKeepalive:
		return ParseKeepaliveMessage(buf)
	default:
		payload := make([]byte, len(buf))
		copy(payload, buf)
		return &UnknownMessage{
			Type:    t,
			Payload: payload,
		}, nil
	}
}

type OpenMessage struct {
	Version            uint8
	MyAS               uint16
	HoldTime           uint16
	BGPID              [4]byte
	OptionalParameters []byte // TODO: Parse
}

func ParseOpenMessage(buf []byte) (Message, error) {
	if len(buf) < 10 {
		return nil, fmt.Errorf("too short open message: len = %d", len(buf))
	}
	if len(buf) != 10+int(buf[9]) {
		return nil, fmt.Errorf("invalid open message length: expected %d; got %d", 10+buf[10], len(buf))
	}
	var id [4]byte
	copy(id[:], buf[5:9])
	opt := make([]byte, buf[10])
	copy(opt, buf[10:10+buf[10]])
	return OpenMessage{
		Version:            buf[0],
		MyAS:               binary.BigEndian.Uint16(buf[1:3]),
		HoldTime:           binary.BigEndian.Uint16(buf[3:5]),
		BGPID:              id,
		OptionalParameters: opt,
	}, nil
}

func (m OpenMessage) WriteTo(w io.Writer) (int64, error) {
	size := headerSize + 10 + len(m.OptionalParameters)
	buf := bytes.NewBuffer(make([]byte, 0, size))

	header := createHeader(uint16(size), MessageTypeOpen)
	buf.Write(header[:])

	buf.Write([]byte{m.Version})
	binary.Write(buf, binary.BigEndian, m.MyAS)
	binary.Write(buf, binary.BigEndian, m.HoldTime)
	buf.Write(m.BGPID[:])
	buf.Write([]byte{uint8(len(m.OptionalParameters))})
	buf.Write(m.OptionalParameters)

	return buf.WriteTo(w)
}

type AttributeFlags byte

func (b AttributeFlags) Optional() bool {
	return b&0b10000000 != 0
}

func (b AttributeFlags) Transitive() bool {
	return b&0b01000000 != 0
}

func (b AttributeFlags) Partial() bool {
	return b&0b00100000 != 0
}

func (b AttributeFlags) ExtendedLength() bool {
	return b&0b00010000 != 0
}

type PathAttribute struct {
	Flags    AttributeFlags
	TypeCode uint8
	Value    []byte
}

func (a *PathAttribute) ReadFrom(r io.Reader) (int64, error) {
	var b [2]byte
	if n, err := io.ReadFull(r, b[:]); err != nil {
		return int64(n), fmt.Errorf("path attribute: %w", err)
	}
	a.Flags = AttributeFlags(b[0])
	a.TypeCode = b[1]
	total := 2
	var length uint16
	if !a.Flags.ExtendedLength() {
		if n, err := io.ReadFull(r, b[:1]); err != nil {
			return 2 + int64(n), fmt.Errorf("path attribute length: %w", err)
		}
		total += 1
		length = uint16(b[0])
	} else {
		if n, err := io.ReadFull(r, b[:]); err != nil {
			return 2 + int64(n), fmt.Errorf("path attribute length: %w", err)
		}
		total += 2
		length = binary.BigEndian.Uint16(b[:])
	}
	a.Value = make([]byte, length)
	if n, err := io.ReadFull(r, a.Value); err != nil {
		return int64(n + total), fmt.Errorf("path attribute value: %w", err)
	}
	return int64(total) + int64(length), nil
}

func (a *PathAttribute) WriteTo(w io.Writer) (int64, error) {
	if n, err := w.Write([]byte{byte(a.Flags), a.TypeCode}); err != nil {
		return int64(n), err
	}
	total := 2

	if !a.Flags.ExtendedLength() {
		if n, err := w.Write([]byte{uint8(len(a.Value))}); err != nil {
			return int64(total + n), fmt.Errorf("path attribute length: %w", err)
		}
		total += 1
	} else {
		var b [2]byte
		binary.BigEndian.PutUint16(b[:], uint16(len(a.Value)))
		if n, err := w.Write(b[:]); err != nil {
			return int64(total + n), fmt.Errorf("path attribute length: %w", err)
		}
		total += 2
	}

	if n, err := w.Write(a.Value); err != nil {
		return int64(total + n), fmt.Errorf("path attribute value: %w", err)
	}

	return int64(total + len(a.Value)), nil
}

func (a PathAttribute) Len() int {
	total := 3 + len(a.Value)
	if a.Flags.ExtendedLength() {
		total += 1
	}
	return total
}

func prefixByteLength(maskLength int) int {
	// 収まる最小のバイト数
	// len = 0       : 0 byte
	// len = 1 ~ 8   : 1 byte
	// len = 9 ~ 16  : 2 byte
	// len = 17 ~ 24 : 3 byte
	// len = 25 ~ 32 : 4 byte
	return (maskLength + 7) / 8
}

func readIPNet(r *bytes.Reader) (*net.IPNet, error) {
	var length int
	if b, err := r.ReadByte(); err != nil {
		return nil, fmt.Errorf("prefix length: %w", err)
	} else {
		length = int(b)
	}
	// TODO: Support IPv6?
	mask := net.CIDRMask(length, 32)
	prefix := make([]byte, 4)
	if _, err := io.ReadFull(r, prefix[:prefixByteLength(length)]); err != nil {
		return nil, fmt.Errorf("prefix: %w", err)
	}

	return &net.IPNet{IP: net.IP(prefix), Mask: mask}, nil
}

func writeIPNet(w io.Writer, n *net.IPNet) (int, error) {
	length, _ := n.Mask.Size()
	b := make([]byte, 1+prefixByteLength(length))
	b[0] = uint8(length)
	copy(b[1:], n.IP)
	return w.Write(b)
}

func ipNetLen(n *net.IPNet) int {
	length, _ := n.Mask.Size()
	return 1 + prefixByteLength(length)
}

type UpdateMessage struct {
	WirhdrawnRoutes []*net.IPNet
	PathAttributes  []PathAttribute
	NLRI            []*net.IPNet
}

func ParseUpdateMessage(buf []byte) (Message, error) {
	r := bytes.NewReader(buf)
	var b [2]byte
	var m UpdateMessage

	// Withdrawn Routes
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return nil, fmt.Errorf("too short update message: %d", len(buf))
	}
	// バイト数であって件数ではないし、かつ variable length なので読んでいかないと何件あるかわからない
	// r.Len() が残りバイト数なので、これの差分で何バイト読んだかわかる
	stop := r.Len() - int(binary.BigEndian.Uint16(b[:]))
	for stop < r.Len() {
		route, err := readIPNet(r)
		if err != nil {
			return nil, fmt.Errorf("withdrawn route: %w", err)
		}
		m.WirhdrawnRoutes = append(m.WirhdrawnRoutes, route)
	}

	// Path Attributes
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return nil, fmt.Errorf("too short update message (no path attributes): %d", len(buf))
	}
	stop = r.Len() - int(binary.BigEndian.Uint16(b[:]))
	for stop < r.Len() {
		var a PathAttribute
		if _, err := a.ReadFrom(r); err != nil {
			return nil, fmt.Errorf("path attribute: %w", err)
		}
		m.PathAttributes = append(m.PathAttributes, a)
	}

	// NLRI
	for r.Len() > 0 {
		route, err := readIPNet(r)
		if err != nil {
			return nil, fmt.Errorf("nlri: %w", err)
		}
		m.NLRI = append(m.NLRI, route)
	}

	return m, nil
}

func (m UpdateMessage) WriteTo(w io.Writer) (int64, error) {
	var withdrawnLength int
	for _, r := range m.WirhdrawnRoutes {
		withdrawnLength += ipNetLen(r)
	}
	var pathAttributesLength int
	for _, a := range m.PathAttributes {
		pathAttributesLength += a.Len()
	}
	var nlriLength int
	for _, r := range m.NLRI {
		nlriLength += ipNetLen(r)
	}

	// 4 byte = Withdrawn Routes Length (2 byte) + Path Attributes Length (2 byte)
	size := headerSize + 4 + withdrawnLength + pathAttributesLength + nlriLength
	buf := bytes.NewBuffer(make([]byte, 0, size))

	header := createHeader(uint16(size), MessageTypeUpdate)
	buf.Write(header[:])

	binary.Write(buf, binary.BigEndian, uint16(withdrawnLength))
	for _, r := range m.WirhdrawnRoutes {
		writeIPNet(buf, r)
	}

	binary.Write(buf, binary.BigEndian, uint16(pathAttributesLength))
	for _, a := range m.PathAttributes {
		a.WriteTo(buf)
	}

	for _, r := range m.NLRI {
		writeIPNet(buf, r)
	}

	return buf.WriteTo(w)
}

type NotificationMessage struct {
	ErrorCode    uint8
	ErrorSubcode uint8
	Data         []byte
}

func ParseNotificationMessage(buf []byte) (Message, error) {
	if len(buf) < 2 {
		return nil, fmt.Errorf("too short notification message: %d", len(buf))
	}
	data := make([]byte, len(buf)-2)
	copy(data, buf[2:])
	return NotificationMessage{
		ErrorCode:    buf[0],
		ErrorSubcode: buf[1],
		Data:         data,
	}, nil
}

func (m NotificationMessage) WriteTo(w io.Writer) (int64, error) {
	size := headerSize + 2 + len(m.Data)
	buf := bytes.NewBuffer(make([]byte, 0, size))

	header := createHeader(uint16(size), MessageTypeNotification)
	buf.Write(header[:])
	buf.Write([]byte{m.ErrorCode, m.ErrorSubcode})
	buf.Write(m.Data)

	return buf.WriteTo(w)
}

type KeepaliveMessage struct {
}

func ParseKeepaliveMessage(buf []byte) (Message, error) {
	if len(buf) != 0 {
		return nil, fmt.Errorf("invalid keepalive message length: %d", len(buf))
	}
	return KeepaliveMessage{}, nil
}

func (m KeepaliveMessage) WriteTo(w io.Writer) (int64, error) {
	header := createHeader(headerSize, MessageTypeKeepalive)
	n, err := w.Write(header[:])
	return int64(n), err
}

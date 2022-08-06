package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
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

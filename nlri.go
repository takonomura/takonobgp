package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type NLRI interface {
	Prefix() *net.IPNet

	WriteTo(w io.Writer) (int64, error)
	Len() int
}

type UnicastNLRI struct {
	IPNet *net.IPNet
}

func (i UnicastNLRI) Prefix() *net.IPNet {
	return i.IPNet
}

func (i UnicastNLRI) WriteTo(w io.Writer) (int64, error) {
	n, err := writeIPNet(w, i.IPNet)
	return int64(n), err
}

func (i UnicastNLRI) Len() int {
	return ipNetLen(i.IPNet)
}

type Label [3]byte

func (l Label) Label() uint32 {
	b := append([]byte{0}, l[:]...)
	i := binary.BigEndian.Uint32(b)
	return i >> 4
}

func (l Label) Bottom() bool {
	return (l[2] & 0x01) == 0x01
}

func NewLabel(l uint32, bottom uint8) Label {
	l = (l << 4) | uint32(bottom&0x0F)
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, l)
	var label Label
	copy(label[:], b[1:])
	return label
}

type RD [8]byte

func NewRD(high, low uint32) RD {
	var rd RD
	binary.BigEndian.PutUint32(rd[0:4], high)
	binary.BigEndian.PutUint32(rd[4:8], low)
	return rd
}

func (rd RD) String() string {
	h := binary.BigEndian.Uint32(rd[0:4])
	l := binary.BigEndian.Uint32(rd[4:8])
	return fmt.Sprintf("%d:%d", h, l)
}

type LabeledVPNNLRI struct {
	Labels []Label
	RD     RD
	IPNet  *net.IPNet
}

func (i LabeledVPNNLRI) Prefix() *net.IPNet {
	return i.IPNet
}

func readLabeledVPNNLRI(r *bytes.Reader, afi AFI) (NLRI, error) {
	var length int
	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	length = int(b)

	var nlri LabeledVPNNLRI

	for {
		var l Label
		if _, err := io.ReadFull(r, l[:]); err != nil {
			return nil, err
		}
		nlri.Labels = append(nlri.Labels, l)
		if l.Bottom() {
			break
		}
	}

	var rd RD
	if _, err := io.ReadFull(r, rd[:]); err != nil {
		return nil, err
	}

	length -= len(nlri.Labels)*3 + 8
	prefix := make(net.IP, afi.Size())
	if _, err := io.ReadFull(r, prefix[:prefixByteLength(length)]); err != nil {
		return nil, err
	}
	nlri.IPNet = &net.IPNet{
		IP:   prefix,
		Mask: net.CIDRMask(length, afi.Bits()),
	}

	return nlri, nil
}

func (i LabeledVPNNLRI) WriteTo(w io.Writer) (int64, error) {
	var total int64

	length, _ := i.IPNet.Mask.Size()
	n, err := w.Write([]byte{uint8(len(i.Labels)*24 + 64 + length)})
	total += int64(n)
	if err != nil {
		return total, err
	}

	for _, l := range i.Labels {
		n, err := w.Write(l[:])
		total += int64(n)
		if err != nil {
			return total, err
		}
	}

	n, err = w.Write(i.RD[:])
	total += int64(n)
	if err != nil {
		return total, err
	}

	n, err = w.Write(i.IPNet.IP[:prefixByteLength(length)])
	total += int64(n)
	return total, err
}

func (i LabeledVPNNLRI) Len() int {
	return len(i.Labels)*3 + 8 + ipNetLen(i.IPNet)
}

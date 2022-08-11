package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
)

func prefixByteLength(maskLength int) int {
	// 収まる最小のバイト数
	// len = 0       : 0 byte
	// len = 1 ~ 8   : 1 byte
	// len = 9 ~ 16  : 2 byte
	// len = 17 ~ 24 : 3 byte
	// len = 25 ~ 32 : 4 byte
	return (maskLength + 7) / 8
}

func readIPNet(r *bytes.Reader, bits int) (*net.IPNet, error) {
	var length int
	if b, err := r.ReadByte(); err != nil {
		return nil, fmt.Errorf("prefix length: %w", err)
	} else {
		length = int(b)
	}
	mask := net.CIDRMask(length, bits)
	prefix := make([]byte, bits/8)
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

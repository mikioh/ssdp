// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import (
	"errors"
	"net"

	"code.google.com/p/go.net/ipv4"
	"code.google.com/p/go.net/ipv6"
)

type transport interface {
	Close() error

	JoinGroup(*net.Interface, net.Addr) error
	LeaveGroup(*net.Interface, net.Addr) error

	SetMulticastInterface(*net.Interface) error
	SetMulticastLoopback(bool) error
}

func setControlFlags(tr transport) error {
	switch c := tr.(type) {
	case *ipv4.PacketConn:
		return c.SetControlMessage(ipv4.FlagDst|ipv4.FlagInterface, true)
	case *ipv6.PacketConn:
		return c.SetControlMessage(ipv6.FlagDst|ipv6.FlagInterface, true)
	default:
		return errors.New("invalid packet conn")
	}
}

func writeTo(tr transport, b []byte, peer net.Addr) (int, error) {
	if len(b) == 0 { // to prevent writing malformed packets on some platforms
		return 0, nil
	}
	switch c := tr.(type) {
	case *ipv4.PacketConn:
		return c.WriteTo(b, nil, peer)
	case *ipv6.PacketConn:
		return c.WriteTo(b, nil, peer)
	default:
		return 0, errors.New("invalid packet conn")
	}
}

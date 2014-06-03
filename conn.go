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

type conn interface {
	Close() error

	JoinGroup(*net.Interface, net.Addr) error
	LeaveGroup(*net.Interface, net.Addr) error

	SetMulticastInterface(*net.Interface) error
	SetMulticastLoopback(bool) error

	setControlFlags() error
	readFrom([]byte) (int, *path, error)
	writeTo([]byte, *net.UDPAddr) (int, error)
	writeToMulti([]byte, []net.Interface, *net.UDPAddr) (int, error)
}

// a path represents a reverse path.
type path struct {
	src     *net.UDPAddr
	dst     *net.UDPAddr
	ifIndex int
}

type udp4Conn struct {
	*ipv4.PacketConn
}

func (c *udp4Conn) setControlFlags() error {
	return c.SetControlMessage(ipv4.FlagDst|ipv4.FlagInterface, true)
}

func (c *udp4Conn) readFrom(b []byte) (int, *path, error) {
	n, cm, src, err := c.ReadFrom(b)
	if err != nil {
		return 0, nil, err
	}
	return n, &path{src: src.(*net.UDPAddr), dst: &net.UDPAddr{IP: cm.Dst}, ifIndex: cm.IfIndex}, err
}

func (c *udp4Conn) writeTo(b []byte, peer *net.UDPAddr) (int, error) {
	if len(b) == 0 { // to prevent writing malformed packets on some platforms
		return 0, nil
	}
	return c.WriteTo(b, nil, peer)
}

func (c *udp4Conn) writeToMulti(b []byte, mifs []net.Interface, grp *net.UDPAddr) (int, error) {
	if len(b) == 0 { // to prevent writing malformed packets on some platforms
		return 0, nil
	}
	var n, oks int
	var lastErr error
	for _, ifi := range mifs {
		c.SetMulticastInterface(&ifi)
		nn, err := c.writeTo(b, grp)
		if err != nil {
			lastErr = err
			continue
		}
		n = nn
		oks++
	}
	if oks == 0 {
		return 0, lastErr
	}
	return n, nil
}

func newUDP4Conn(c *ipv4.PacketConn) *udp4Conn {
	return &udp4Conn{PacketConn: c}
}

type udp6Conn struct {
	*ipv6.PacketConn
}

func (c *udp6Conn) setControlFlags() error {
	return c.SetControlMessage(ipv6.FlagDst|ipv6.FlagInterface, true)
}

func (c *udp6Conn) readFrom(b []byte) (int, *path, error) {
	n, cm, src, err := c.ReadFrom(b)
	if err != nil {
		return 0, nil, err
	}
	return n, &path{src: src.(*net.UDPAddr), dst: &net.UDPAddr{IP: cm.Dst}, ifIndex: cm.IfIndex}, err
}

func (c *udp6Conn) writeTo(b []byte, peer *net.UDPAddr) (int, error) {
	if len(b) == 0 { // to prevent writing malformed packets on some platforms
		return 0, nil
	}
	return c.WriteTo(b, nil, peer)
}

func (c *udp6Conn) writeToMulti(b []byte, mifs []net.Interface, grp *net.UDPAddr) (int, error) {
	if len(b) == 0 { // to prevent writing malformed packets on some platforms
		return 0, nil
	}
	var n, oks int
	var lastErr error
	wrgrp := *grp
	for _, ifi := range mifs {
		c.SetMulticastInterface(&ifi)
		if ipv6LinkLocal(wrgrp.IP) {
			wrgrp.Zone = ifi.Name
		}
		nn, err := c.writeTo(b, &wrgrp)
		if err != nil {
			lastErr = err
			continue
		}
		n = nn
		oks++
	}
	if oks == 0 {
		return 0, lastErr
	}
	return n, nil
}

func newUDP6Conn(c *ipv6.PacketConn) *udp6Conn {
	return &udp6Conn{PacketConn: c}
}

func joinGroup(c conn, mifs []net.Interface, unicast func(net.IP) bool, grp *net.UDPAddr) ([]net.Interface, error) {
	mifs, err := interfaces(mifs, unicast)
	if err != nil {
		return nil, err
	}
	var rmifs []net.Interface
	for _, ifi := range mifs {
		if err := c.JoinGroup(&ifi, grp); err != nil {
			continue
		}
		rmifs = append(rmifs, ifi)
	}
	if len(rmifs) == 0 {
		return nil, errors.New("no such multicast network interface")
	}
	if err := c.setControlFlags(); err != nil {
		return nil, err
	}
	return rmifs, nil
}

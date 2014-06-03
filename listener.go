// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import (
	"net"

	"code.google.com/p/go.net/ipv4"
	"code.google.com/p/go.net/ipv6"
)

// A Listener represents a UDP multicast listener.
type Listener struct {
	// Group specifies a group IP address of the multicast UDP
	// HTTP message exchange. if it is empty, DefaultIPv4Group
	// will be used.
	Group string

	// Port specifies a service port of the unicast and multicast
	// UDP HTTP message exchanges. If it is empty, DefaultPort
	// will be used.
	Port string

	// Port specifies a local listening port of the unicast and
	// multicast UDP HTTP message exchanges. If it is not empty,
	// the listener prefers LocalPort than Port.
	LocalPort string

	// Loopback sets whether transmitted multicast packets should
	// be copied and send back to the originator.
	MulticastLoopback bool
}

func (ln *Listener) listen() (conn, *net.UDPAddr, error) {
	if ln.Group == "" {
		ln.Group = DefaultIPv4Group
	}
	if ln.Port == "" {
		ln.Port = DefaultPort
	}
	if ln.LocalPort == "" {
		ln.LocalPort = DefaultPort
	}
	grp, err := net.ResolveUDPAddr("udp", net.JoinHostPort(ln.Group, ln.Port))
	if err != nil {
		return nil, nil, err
	}
	if grp.IP.To4() != nil {
		c, err := net.ListenPacket("udp4", net.JoinHostPort(ln.Group, ln.LocalPort))
		if err != nil {
			return nil, nil, err
		}
		p := newUDP4Conn(ipv4.NewPacketConn(c))
		p.SetMulticastTTL(2)
		p.SetMulticastLoopback(ln.MulticastLoopback)
		return p, grp, nil
	}
	c, err := net.ListenPacket("udp6", net.JoinHostPort(ln.Group, ln.LocalPort))
	if err != nil {
		return nil, nil, err
	}
	p := newUDP6Conn(ipv6.NewPacketConn(c))
	if grp.IP.IsLinkLocalMulticast() || grp.IP.IsLinkLocalMulticast() {
		p.SetMulticastHopLimit(1)
	} else {
		p.SetMulticastHopLimit(5)
	}
	p.SetMulticastLoopback(ln.MulticastLoopback)
	return p, grp, nil
}

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

// A Listener represents a UDP multicast listener.
type Listener struct {
	// Group specifies a group IP address of the multicast UDP
	// HTTP message exchange. if it is empty, DefaultIPv4Group
	// will be used.
	Group string

	// Port specifies a service port of the unicast and multicast
	// UDP HTTP message exchange. If it is empty, DefaultPort will
	// be used.
	Port string

	// Port specifies a local listening port of the unicast and
	// multicast UDP HTTP message exchange. If it is not empty,
	// the listener listens on LocalPort. Otherwise listens on
	// Port.
	LocalPort string

	// Loopback sets whether transmitted multicast packets should
	// be copied and send back to the originator.
	MulticastLoopback bool

	group               *net.UDPAddr
	multicastInterfaces []net.Interface
}

func (ln *Listener) listen() (tr transport, unicast func(net.IP) bool, err error) {
	if ln.Group == "" {
		ln.Group = DefaultIPv4Group
	}
	if ln.Port == "" {
		ln.Port = DefaultPort
	}
	if ln.LocalPort == "" {
		ln.LocalPort = DefaultPort
	}
	ln.group, err = net.ResolveUDPAddr("udp", net.JoinHostPort(ln.Group, ln.Port))
	if err != nil {
		return nil, nil, err
	}
	if ln.group.IP.To4() != nil {
		c, err := net.ListenPacket("udp4", net.JoinHostPort(ln.Group, ln.LocalPort))
		if err != nil {
			return nil, nil, err
		}
		tr = ipv4.NewPacketConn(c)
		unicast = ipv4Unicast
	} else {
		c, err := net.ListenPacket("udp6", net.JoinHostPort(ln.Group, ln.LocalPort))
		if err != nil {
			return nil, nil, err
		}
		tr = ipv6.NewPacketConn(c)
		unicast = ipv6Unicast
	}
	tr.SetMulticastLoopback(ln.MulticastLoopback)
	return tr, unicast, nil
}

func (ln *Listener) joinGroup(tr transport, mifs []net.Interface, unicast func(net.IP) bool) (err error) {
	mifs, err = multicastInterfaces(mifs, unicast)
	if err != nil {
		return err
	}
	for _, ifi := range mifs {
		switch c := tr.(type) {
		case *ipv4.PacketConn:
			err = c.JoinGroup(&ifi, ln.group)
		case *ipv6.PacketConn:
			err = c.JoinGroup(&ifi, ln.group)
		default:
			return errors.New("invalid packet conn")
		}
		if err != nil {
			continue
		}
		ln.multicastInterfaces = append(ln.multicastInterfaces, ifi)
	}
	if len(ln.multicastInterfaces) == 0 {
		return errors.New("no such multicast interface")
	}
	if err := setControlFlags(tr); err != nil {
		return err
	}
	return nil
}

type path struct {
	src *net.UDPAddr
	dst net.IP
	ifi *net.Interface
}

func (ln *Listener) readFrom(tr transport, b []byte) (int, *path, error) {
	switch c := tr.(type) {
	case *ipv4.PacketConn:
		n, cm, src, err := c.ReadFrom(b)
		if err != nil {
			return 0, nil, err
		}
		return n, &path{src: src.(*net.UDPAddr), dst: cm.Dst, ifi: ln.interfaceByIndex(cm.IfIndex)}, err
	case *ipv6.PacketConn:
		n, cm, src, err := c.ReadFrom(b)
		if err != nil {
			return 0, nil, err
		}
		return n, &path{src: src.(*net.UDPAddr), dst: cm.Dst, ifi: ln.interfaceByIndex(cm.IfIndex)}, err
	default:
		return 0, nil, errors.New("invalid packet conn")
	}
}

func (ln *Listener) interfaceByIndex(index int) *net.Interface {
	for _, ifi := range ln.multicastInterfaces {
		if index == ifi.Index {
			return &ifi
		}
	}
	return nil
}

func (ln *Listener) interfaces() []net.Interface {
	mifs := make([]net.Interface, len(ln.multicastInterfaces))
	copy(mifs, ln.multicastInterfaces)
	return mifs
}

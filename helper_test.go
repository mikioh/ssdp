// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import "net"

var (
	supportsIPv4 bool
	supportsIPv6 bool
)

func init() {
	if c, err := net.ListenPacket("udp4", "127.0.0.1:0"); err == nil {
		c.Close()
		supportsIPv4 = true
	}
	if c, err := net.ListenPacket("udp6", "[::1]:0"); err == nil {
		c.Close()
		supportsIPv6 = true
	}
}

func loopbackInterface() *net.Interface {
	ift, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, ifi := range ift {
		if ifi.Flags&net.FlagLoopback == 0 || ifi.Flags&net.FlagMulticast == 0 || ifi.Flags&net.FlagUp == 0 {
			continue
		}
		return &ifi
	}
	return nil
}

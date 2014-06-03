// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import "net"

func ipv4Unicast(ip net.IP) bool {
	return ip.To4() != nil && (ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsGlobalUnicast())
}

func ipv6Unicast(ip net.IP) bool {
	return ip.To16() != nil && ip.To4() == nil && (ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsGlobalUnicast())
}

func ipv6LinkLocal(ip net.IP) bool {
	return ip.To16() != nil && ip.To4() == nil && (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast())
}

func interfaces(ift []net.Interface, unicast func(net.IP) bool) ([]net.Interface, error) {
	var err error
	if len(ift) == 0 {
		ift, err = net.Interfaces()
		if err != nil {
			return nil, err
		}
	}
	var mifs []net.Interface
	for _, ifi := range ift {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagMulticast == 0 {
			continue
		}
		ifat, err := ifi.Addrs()
		if err != nil {
			continue
		}
	loop:
		for _, ifa := range ifat {
			switch ifa := ifa.(type) {
			case *net.IPAddr:
				if unicast(ifa.IP) {
					mifs = append(mifs, ifi)
					break loop
				}
			case *net.IPNet:
				if unicast(ifa.IP) {
					mifs = append(mifs, ifi)
					break loop
				}
			}
		}
	}
	return mifs, nil
}

func interfaceByIndex(mifs []net.Interface, index int) *net.Interface {
	for _, ifi := range mifs {
		if index == ifi.Index {
			return &ifi
		}
	}
	return nil
}

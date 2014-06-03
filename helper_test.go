// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp_test

import "net"

func multicastLoopbackInterface() *net.Interface {
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

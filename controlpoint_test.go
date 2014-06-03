// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import (
	"net"
	"net/http"
	"testing"
)

func TestControlPoint(t *testing.T) {
	ifi := loopbackInterface()
	if ifi == nil {
		t.Skip("no available multicast network interface found")
	}
	mifs := []net.Interface{*ifi}
	var grps []string
	if supportsIPv4 {
		grps = append(grps, DefaultIPv4Group)
	}
	if supportsIPv6 {
		grps = append(grps, DefaultIPv6LinkLocalGroup)
	}

	for _, grp := range grps {
		ln := Listener{Group: grp}
		cp, err := ln.ListenControlPoint(mifs)
		if err != nil {
			t.Fatal(err)
		}
		hdlr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Write(nil)
		})
		go cp.Serve(hdlr)

		for _, ifi := range cp.Interfaces() {
			t.Logf("%v on %v", cp.GroupAddr(), ifi)
		}
		cp.Close()
	}
}

// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import (
	"net"
	"testing"
)

type proxy struct {
	*Redirector
}

func (p *proxy) RedirectAdvert(rdr *AdvertRedirector) {}

func (p *proxy) RedirectResponse(rdr *ResponseRedirector) {}

func TestRedirector(t *testing.T) {
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
		rdr, err := ln.ListenRedirector(mifs)
		if err != nil {
			t.Fatal(err)
		}
		p := proxy{rdr}
		go rdr.Serve(&p)

		for _, ifi := range p.Redirector.Interfaces() {
			t.Logf("%v on %v", p.Redirector.GroupAddr(), ifi)
		}
		p.Redirector.Close()
	}
}

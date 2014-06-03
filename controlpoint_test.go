// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp_test

import (
	"net"
	"net/http"
	"testing"

	"github.com/mikioh/ssdp"
)

func TestControlPoint(t *testing.T) {
	ifi := multicastLoopbackInterface()
	if ifi == nil {
		t.Skip("no available multicast interface found")
	}
	mifs := []net.Interface{*ifi}

	cpln := ssdp.Listener{}
	cp, err := cpln.ListenControlPoint(mifs)
	if err != nil {
		t.Fatal(err)
	}
	cphdlr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write(nil)
	})
	go cp.Serve(cphdlr)

	for _, ifi := range cp.Interfaces() {
		t.Logf("%v", ifi)
	}
	cp.Close()
}

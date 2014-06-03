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

func TestDevice(t *testing.T) {
	ifi := multicastLoopbackInterface()
	if ifi == nil {
		t.Skip("no available multicast interface found")
	}
	mifs := []net.Interface{*ifi}

	devln := ssdp.Listener{}
	dev, err := devln.ListenDevice(mifs)
	if err != nil {
		t.Fatal(err)
	}
	devhdlr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "max-age=1")
		h.Set("Location", "here")
		h.Set("Server", "it's me")
		w.Write(nil)
	})
	go dev.Serve(devhdlr)

	for _, ifi := range dev.Interfaces() {
		t.Logf("%v", ifi)
	}
	dev.Close()
}

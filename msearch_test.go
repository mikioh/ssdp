// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestMSearch(t *testing.T) {
	devln := Listener{}
	dev, err := devln.ListenDevice(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	devhdlr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hdr := w.Header()
		hdr.Set("Cache-Control", "max-age=1800")
		hdr.Set("Location", "http://127.0.0.1:5963/dd.xml")
		hdr.Set("BOOTID.UPNP.ORG", "1")
		hdr.Set("Server", "Go ssdp package")
		hdr.Set("USN", "uuid:--::run--")
		w.Write(nil)
	})
	go dev.Serve(devhdlr)

	cpln := Listener{LocalPort: "1901", MulticastLoopback: true}
	cp, err := cpln.ListenControlPoint(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	cphdlr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		//t.Logf("CP: %+v, %+v", w, req)
	})
	go cp.Serve(cphdlr)

	var wg sync.WaitGroup
	const N = 3
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			hdr := make(http.Header)
			hdr.Set("Test1", "oops:ouch-oops")
			hdr.Set("Test2", "ouch:oops-ouch")
			resps, err := cp.MSearch(hdr, nil, 300*time.Millisecond)
			if err != nil {
				t.Error(err)
			}
			t.Logf("%v responses received", len(resps))
			for _, resp := range resps {
				resp.Body.Close()
			}
		}()
	}
	wg.Wait()
}

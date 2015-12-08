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

func TestNotify(t *testing.T) {
	devln := Listener{LocalPort: "1901", MulticastLoopback: true}
	dev, err := devln.ListenDevice(nil)
	if err != nil {
		t.Skip(err)
	}
	defer dev.Close()
	devhdlr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		//t.Logf("DEV: %+v, %+v", w, req)
	})
	go dev.Serve(devhdlr)

	cpln := Listener{}
	cp, err := cpln.ListenControlPoint(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cp.Close()
	var rcvd = struct {
		sync.RWMutex
		reqs []*http.Request
	}{}
	cphdlr := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		//t.Logf("CP: %+v, %+v", w, req)
		rcvd.Lock()
		rcvd.reqs = append(rcvd.reqs, req)
		rcvd.Unlock()
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
			if err := dev.Notify(hdr, nil); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	time.Sleep(2 * time.Second)
	rcvd.RLock()
	t.Logf("%v requests received", len(rcvd.reqs))
	rcvd.RUnlock()
}

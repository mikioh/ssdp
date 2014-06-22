// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import (
	"bytes"
	"errors"
	"log"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// A ControlPoint represents a SSDP control point.
type ControlPoint struct {
	// ErrorLog specified an optional logger for errors. If it is
	// nil, logging goes to os.Stderr via the log package's
	// standard logger.
	ErrorLog *log.Logger

	conn                      // network connection endpoint
	group   *net.UDPAddr      // group address
	unicast func(net.IP) bool // unicast address filter
	mifs    []net.Interface   // multicast network interfaces

	muxmu sync.RWMutex
	mux   map[*http.Request]chan *http.Response // unicast message mux
}

// ListenControlPoint listens on the UDP network Listener.Group and
// Listener.Port, and returns a control point. If mifs is nil, it
// tries to listen on all available multicast network interfaces.
func (ln *Listener) ListenControlPoint(mifs []net.Interface) (*ControlPoint, error) {
	var err error
	cp := &ControlPoint{mux: make(map[*http.Request]chan *http.Response)}
	if cp.conn, cp.group, err = ln.listen(); err != nil {
		return nil, err
	}
	if cp.group.IP.To4() != nil {
		cp.unicast = ipv4Unicast
	} else {
		cp.unicast = ipv6Unicast
	}
	if cp.mifs, err = joinGroup(cp.conn, cp.group, mifs, cp.unicast); err != nil {
		cp.Close()
		return nil, err
	}
	return cp, nil
}

// Serve starts to handle incoming SSDP messages from SSDP
// devices. The handler must not be nil.
func (cp *ControlPoint) Serve(hdlr http.Handler) error {
	if hdlr == nil {
		return errors.New("invalid http handler")
	}
	b := make([]byte, 1280)
	for {
		n, path, err := cp.readFrom(b)
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
				cp.logf("read failed: %v", err)
				continue
			}
			return err
		}
		if !path.dst.IP.IsMulticast() {
			resp, err := parseResponse(b[:n])
			if err != nil {
				cp.logf("parse response failed: %v", err)
				continue
			}
			cp.muxmu.RLock()
			for _, ch := range cp.mux {
				ch <- resp
			}
			cp.muxmu.RUnlock()
			continue
		}
		if !path.dst.IP.Equal(cp.group.IP) {
			cp.logf("unknown destination address: %v on %v", path.dst, interfaceByIndex(cp.mifs, path.ifIndex).Name)
			continue
		}
		req, err := parseAdvert(b[:n])
		if err != nil {
			cp.logf("parse advert failed: %v", err)
			continue
		}
		if req.Method != notifyMethod {
			continue
		}
		resp := newResponseWriter(cp.conn, cp.mifs, cp.group, path, req)
		go func() {
			defer func() {
				if err := recover(); err != nil {
					const size = 64 << 10
					b := make([]byte, size)
					b = b[:runtime.Stack(b, false)]
					cp.logf("panic serving %v: %v\n%s", resp.path.src, err, b)
				}
			}()
			hdlr.ServeHTTP(resp, req)
		}()
	}
}

// GroupAddr returns the joined group network address.
func (cp *ControlPoint) GroupAddr() *net.UDPAddr {
	return cp.group
}

// Close closes the control point.
func (cp *ControlPoint) Close() error {
	for _, ifi := range cp.mifs {
		cp.LeaveGroup(&ifi, cp.group)
	}
	return cp.conn.Close()
}

// Interfaces returns a list of the joined multicast network
// interfaces.
func (cp *ControlPoint) Interfaces() []net.Interface {
	return cp.mifs
}

// MSearch issues a M-SEARCH SSDP message, takes a timeout and returns
// a list of responses. Callers should close each http.Response.Body
// when done reading from it. If mifs is nil, it tries to use all
// available multicast network interfaces.
func (cp *ControlPoint) MSearch(hdr http.Header, mifs []net.Interface, tmo time.Duration) ([]*http.Response, error) {
	req := newAdvert(msearchMethod, cp.group.String(), hdr)
	var buf bytes.Buffer
	if err := marshalAdvert(&buf, req); err != nil {
		return nil, err
	}
	mifs, err := interfaces(mifs, cp.unicast)
	if err != nil {
		return nil, err
	}
	if _, err := cp.writeToMulti(buf.Bytes(), cp.group, mifs); err != nil {
		return nil, err
	}
	respCh := cp.register(req)
	defer cp.deregister(req)
	t := time.NewTimer(tmo)
	defer t.Stop()
	var resps []*http.Response
loop:
	for {
		select {
		case <-t.C:
			break loop
		case resp := <-respCh:
			resps = append(resps, resp)
		}
	}
	return resps, nil
}

func (cp *ControlPoint) register(req *http.Request) chan *http.Response {
	cp.muxmu.Lock()
	defer cp.muxmu.Unlock()
	if ch, ok := cp.mux[req]; ok {
		return ch
	}
	ch := make(chan *http.Response, 1)
	cp.mux[req] = ch
	return ch
}

func (cp *ControlPoint) deregister(req *http.Request) {
	cp.muxmu.Lock()
	if ch, ok := cp.mux[req]; ok {
		close(ch)
	}
	delete(cp.mux, req)
	cp.muxmu.Unlock()
}

func (cp ControlPoint) logf(format string, args ...interface{}) {
	if cp.ErrorLog != nil {
		cp.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

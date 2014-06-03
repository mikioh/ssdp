// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import (
	"errors"
	"log"
	"net"
	"net/http"
	"runtime"
)

// A ControlPoint represents a SSDP control point.
type ControlPoint struct {
	// ErrorLog specified an optional logger for errors. If it is
	// nil, logging goes to os.Stderr via the log package's
	// standard logger.
	ErrorLog *log.Logger

	listener        *Listener
	transport       transport
	unicastResponse chan *http.Response
}

// ListenControlPoint listens on the UDP network Listener.Group and
// Listener.Port, returns a control point. If mifs is nil, it tries to
// listen on all availbale multicast network interfaces.
func (ln *Listener) ListenControlPoint(mifs []net.Interface) (*ControlPoint, error) {
	tr, unicast, err := ln.listen()
	if err != nil {
		return nil, err
	}
	if err := ln.joinGroup(tr, mifs, unicast); err != nil {
		tr.Close()
		return nil, err
	}
	return &ControlPoint{listener: ln, transport: tr, unicastResponse: make(chan *http.Response, 1)}, nil
}

// Serve starts to handle incoming UDP HTTP DIAL service discovery
// messages from the SSDP devices. The handler must not be nil.
func (cp *ControlPoint) Serve(hdlr http.Handler) error {
	defer func() {
		for _, ifi := range cp.listener.multicastInterfaces {
			cp.transport.LeaveGroup(&ifi, cp.listener.group)
		}
		cp.transport.Close()
		close(cp.unicastResponse)
	}()
	if hdlr == nil {
		return errors.New("invalid http handler")
	}
	b := make([]byte, 1280)
	for {
		n, path, err := cp.listener.readFrom(cp.transport, b)
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
				cp.logf("read failed: %v", err)
				continue
			}
			return err
		}
		if !path.dst.IsMulticast() {
			resp, err := parseResponse(b[:n], nil)
			if err != nil {
				cp.logf("parse response failed: %v", err)
				continue
			}
			cp.unicastResponse <- resp
		} else {
			if !path.dst.Equal(cp.listener.group.IP) {
				cp.logf("invalid destination address: %v on %v", path.dst, path.ifi.Name)
				continue
			}
			req, err := parseRequest(b[:n])
			if err != nil {
				cp.logf("parse request failed: %v", err)
				continue
			}
			if req.Method != "NOTIFY" {
				continue
			}
			resp := newResponse(cp.listener, cp.transport, path, req)
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
}

// GroupAddr returns the joined group network address.
func (cp *ControlPoint) GroupAddr() net.Addr {
	return cp.listener.group
}

// Close closes the control point.
func (cp *ControlPoint) Close() error {
	return cp.transport.Close()
}

// Interfaces returns a list of the joined multicast network
// interfaces.
func (cp *ControlPoint) Interfaces() []net.Interface {
	return cp.listener.interfaces()
}

func (cp ControlPoint) logf(format string, args ...interface{}) {
	if cp.ErrorLog != nil {
		cp.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

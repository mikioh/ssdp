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

// A Device represents a SSDP device.
type Device struct {
	// ErrorLog specified an optional logger for errors. If it is
	// nil, logging goes to os.Stderr via the log package's
	// standard logger.
	ErrorLog *log.Logger

	listener  *Listener
	transport transport
}

// ListenDevices listens on the UDP network Listener.Group and
// Listener.Port, returns a device. If mifs is nil, it tries to listen
// on all availbale multicast network interfaces.
func (ln *Listener) ListenDevice(mifs []net.Interface) (*Device, error) {
	tr, unicast, err := ln.listen()
	if err != nil {
		return nil, err
	}
	if err := ln.joinGroup(tr, mifs, unicast); err != nil {
		tr.Close()
		return nil, err
	}
	return &Device{listener: ln, transport: tr}, nil
}

// Serve starts to handle incoming UDP HTTP DIAL service discovery
// messages from the SSDP control points. The handler must not be nil.
func (dev *Device) Serve(hdlr http.Handler) error {
	defer func() {
		for _, ifi := range dev.listener.multicastInterfaces {
			dev.transport.LeaveGroup(&ifi, dev.listener.group)
		}
		dev.transport.Close()
	}()
	if hdlr == nil {
		return errors.New("invalid http handler")
	}
	b := make([]byte, 1280)
	for {
		n, path, err := dev.listener.readFrom(dev.transport, b)
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
				dev.logf("read failed: %v", err)
				continue
			}
			return err
		}
		if !path.dst.IsMulticast() || !path.dst.Equal(dev.listener.group.IP) {
			continue
		}
		req, err := parseRequest(b[:n])
		if err != nil {
			dev.logf("parse request failed: %v", err)
			continue
		}
		if req.Method != "M-SEARCH" {
			continue
		}
		resp := newResponse(dev.listener, dev.transport, path, req)
		go func() {
			defer func() {
				if err := recover(); err != nil {
					const size = 64 << 10
					b := make([]byte, size)
					b = b[:runtime.Stack(b, false)]
					dev.logf("panic serving %v: %v\n%s", resp.path.src, err, b)
				}
			}()
			hdlr.ServeHTTP(resp, req)
		}()
	}
}

// GroupAddr returns the joined group network address.
func (dev *Device) GroupAddr() net.Addr {
	return dev.listener.group
}

// Close closes the device.
func (dev *Device) Close() error {
	return dev.transport.Close()
}

// Interfaces returns a list of the joined multicast network
// interfaces.
func (dev *Device) Interfaces() []net.Interface {
	return dev.listener.interfaces()
}

func (dev *Device) logf(format string, args ...interface{}) {
	if dev.ErrorLog != nil {
		dev.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

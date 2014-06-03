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
)

// A Device represents a SSDP device.
type Device struct {
	// ErrorLog specified an optional logger for errors. If it is
	// nil, logging goes to os.Stderr via the log package's
	// standard logger.
	ErrorLog *log.Logger

	conn                      // network connection endpoint
	group   *net.UDPAddr      // group address
	unicast func(net.IP) bool // unicast address filter
	mifs    []net.Interface   // multicast network interfaces
}

// ListenDevices listens on the UDP network Listener.Group and
// Listener.Port, and returns a device. If mifs is nil, it tries to
// listen on all available multicast network interfaces.
func (ln *Listener) ListenDevice(mifs []net.Interface) (*Device, error) {
	var err error
	dev := &Device{}
	if dev.conn, dev.group, err = ln.listen(); err != nil {
		return nil, err
	}
	if dev.group.IP.To4() != nil {
		dev.unicast = ipv4Unicast
	} else {
		dev.unicast = ipv6Unicast
	}
	if dev.mifs, err = joinGroup(dev.conn, mifs, dev.unicast, dev.group); err != nil {
		dev.Close()
		return nil, err
	}
	return dev, nil
}

// Serve starts to handle incoming SSDP messages from SSDP control
// points. The handler must not be nil.
func (dev *Device) Serve(hdlr http.Handler) error {
	if hdlr == nil {
		return errors.New("invalid http handler")
	}
	b := make([]byte, 1280)
	for {
		n, path, err := dev.readFrom(b)
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
				dev.logf("read failed: %v", err)
				continue
			}
			return err
		}
		if !path.dst.IP.IsMulticast() {
			continue
		}
		if !path.dst.IP.Equal(dev.group.IP) {
			dev.logf("unknown destination address: %v on %v", path.dst, interfaceByIndex(dev.mifs, path.ifIndex).Name)
			continue
		}
		req, err := parseAdvert(b[:n])
		if err != nil {
			dev.logf("parse advert failed: %v", err)
			continue
		}
		if req.Method != msearchMethod {
			continue
		}
		resp := newResponseWriter(dev.conn, dev.mifs, dev.group, path, req)
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
func (dev *Device) GroupAddr() *net.UDPAddr {
	return dev.group
}

// Close closes the device.
func (dev *Device) Close() error {
	for _, ifi := range dev.mifs {
		dev.LeaveGroup(&ifi, dev.group)
	}
	return dev.conn.Close()
}

// Interfaces returns a list of the joined multicast network
// interfaces.
func (dev *Device) Interfaces() []net.Interface {
	return dev.mifs
}

// Notify issues a NOTIFY SSDP message. If mifs is nil, it tries to
// use all available multicast network interfaces.
func (dev *Device) Notify(hdr http.Header, mifs []net.Interface) error {
	req := newAdvert(notifyMethod, dev.group.String(), hdr)
	var buf bytes.Buffer
	if err := req.Write(&buf); err != nil {
		return err
	}
	mifs, err := interfaces(mifs, dev.unicast)
	if err != nil {
		return err
	}
	if _, err := dev.writeToMulti(buf.Bytes(), mifs, dev.group); err != nil {
		return err
	}
	return nil
}

func (dev *Device) logf(format string, args ...interface{}) {
	if dev.ErrorLog != nil {
		dev.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

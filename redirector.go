// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import (
	"errors"
	"log"
	"net"
	"runtime"
)

// A Redirector represents a back-to-back SSDP entity.
type Redirector struct {
	// ErrorLog specified an optional logger for errors. If it is
	// nil, logging goes to os.Stderr via the log package's
	// standard logger.
	ErrorLog *log.Logger

	conn                      // network connection endpoint
	group   *net.UDPAddr      // group address
	unicast func(net.IP) bool // unicast address filter
	mifs    []net.Interface   // multicast network interfaces
}

// ListenRedirector listens on the UDP network Listener.Group and
// Listener.Port, and returns a redirector. If mifs is nil, it tries
// to listen on all available multicast network interfaces.
func (ln *Listener) ListenRedirector(mifs []net.Interface) (*Redirector, error) {
	var err error
	rdr := &Redirector{}
	if rdr.conn, rdr.group, err = ln.listen(); err != nil {
		return nil, err
	}
	if rdr.group.IP.To4() != nil {
		rdr.unicast = ipv4Unicast
	} else {
		rdr.unicast = ipv6Unicast
	}
	if rdr.mifs, err = joinGroup(rdr.conn, mifs, rdr.unicast, rdr.group); err != nil {
		rdr.Close()
		return nil, err
	}
	return rdr, nil
}

// A RedirectHandler represens a generic redirect handler.
type RedirectHandler interface {
	// RediretAdvert handles an inbound SSDP advertisement
	// message.
	RedirectAdvert(*AdvertRedirector)

	// RediretResponse handles an inbound SSDP response message.
	RedirectResponse(*ResponseRedirector)
}

// Serve starts to handle incoming SSDP messages from either SSDP
// control points or SSDP devices. The handler must not be nil.
func (rdr *Redirector) Serve(hdlr RedirectHandler) error {
	if rdr == nil {
		return errors.New("invalid http handler")
	}
	b := make([]byte, 1280)
	for {
		n, path, err := rdr.readFrom(b)
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
				rdr.logf("read failed: %v", err)
				continue
			}
			return err
		}
		if !path.dst.IP.IsMulticast() {
			resp, err := parseResponse(b[:n])
			if err != nil {
				rdr.logf("parse response failed: %v", err)
				continue
			}
			resprdr := newResponseRedirector(rdr.conn, rdr.mifs, rdr.group, path, resp)
			go func() {
				defer func() {
					if err := recover(); err != nil {
						const size = 64 << 10
						b := make([]byte, size)
						b = b[:runtime.Stack(b, false)]
						rdr.logf("panic serving %v: %v\n%s", resprdr.path.src, err, b)
					}
				}()
				hdlr.RedirectResponse(resprdr)
			}()
			continue
		}
		if !path.dst.IP.Equal(rdr.group.IP) {
			rdr.logf("unknown destination address: %v on %v", path.dst, interfaceByIndex(rdr.mifs, path.ifIndex).Name)
			continue
		}
		req, err := parseAdvert(b[:n])
		if err != nil {
			rdr.logf("parse advert failed: %v", err)
			continue
		}
		advrdr := newAdvertRedirector(rdr.conn, rdr.mifs, rdr.group, path, req)
		go func() {
			defer func() {
				if err := recover(); err != nil {
					const size = 64 << 10
					b := make([]byte, size)
					b = b[:runtime.Stack(b, false)]
					rdr.logf("panic serving %v: %v\n%s", advrdr.path.src, err, b)
				}
			}()
			hdlr.RedirectAdvert(advrdr)
		}()
	}
}

// GroupAddr returns the joined group network address.
func (rdr *Redirector) GroupAddr() *net.UDPAddr {
	return rdr.group
}

// Close closes the redirector.
func (rdr *Redirector) Close() error {
	for _, ifi := range rdr.mifs {
		rdr.LeaveGroup(&ifi, rdr.group)
	}
	return rdr.conn.Close()
}

// Interfaces returns a list of the joined multicast network
// interfaces.
func (rdr *Redirector) Interfaces() []net.Interface {
	return rdr.mifs
}

func (rdr *Redirector) logf(format string, args ...interface{}) {
	if rdr.ErrorLog != nil {
		rdr.ErrorLog.Printf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

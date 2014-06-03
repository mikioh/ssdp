// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
)

func newAdvert(method, host string, hdr http.Header) *http.Request {
	return &http.Request{
		Method:     method,
		URL:        &url.URL{Path: "*"},
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     hdr,
		Host:       host,
	}
}

// We cannot use http.Request.Write due to golang.org/issue/5684.
func marshalAdvert(buf *bytes.Buffer, req *http.Request) error {
	fmt.Fprintf(buf, "%s %s %s\r\n", req.Method, "*", req.Proto)
	fmt.Fprintf(buf, "Host: %s\r\n", req.Host)
	if err := req.Header.WriteSubset(buf, nil); err != nil {
		return err
	}
	if _, err := buf.WriteString("\r\n"); err != nil {
		return err
	}
	return nil
}

// We cannot use http.ReadRequest due to golang.org/issue/5684.
func parseAdvert(b []byte) (*http.Request, error) {
	br := bufio.NewReader(bytes.NewBuffer(b))
	tp := textproto.NewReader(br)
	l, err := tp.ReadLine()
	if err != nil {
		return nil, err
	}
	var ok bool
	req := &http.Request{}
	req.Method, req.RequestURI, req.Proto, ok = parseRequestLine(l)
	if !ok {
		return nil, fmt.Errorf("malformed request line: %v", l)
	}
	if req.ProtoMajor, req.ProtoMinor, ok = http.ParseHTTPVersion(req.Proto); !ok {
		return nil, fmt.Errorf("malformed request line: %v", l)
	}
	ruri, err := url.QueryUnescape(req.RequestURI)
	if err != nil {
		return nil, fmt.Errorf("malformed request line: %v", l)
	}
	req.URL, err = url.ParseRequestURI(ruri)
	if err != nil {
		return nil, err
	}
	mimeh, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	req.Header = http.Header(mimeh)
	req.Host = req.URL.Host
	if req.Host == "" {
		req.Host = req.Header.Get("Host")
	}
	req.Header.Del("Host")
	switch req.Method {
	case notifyMethod, msearchMethod:
	default:
		return nil, fmt.Errorf("unknown method: %v", req.Method)
	}
	if req.Proto != "HTTP/1.1" {
		return nil, fmt.Errorf("unknown version: %v", req.Proto)
	}
	return req, nil
}

func parseRequestLine(l string) (method, requestURI, proto string, ok bool) {
	ss := strings.SplitN(l, " ", 3)
	if len(ss) < 3 {
		return
	}
	return ss[0], ss[1], ss[2], true
}

// An AdvertRedirector represents a SSDP advertisement message
// redirector.
type AdvertRedirector struct {
	conn // network connection endpoint
	mifs []net.Interface
	path *path // reverse path
	req  *http.Request
}

// Header returns the HTTP header map that will be sent by WriteTo
// method.
func (rdr *AdvertRedirector) Header() http.Header {
	return rdr.req.Header
}

// WriteTo writes the SSDP advertisement message. The outbound network
// interface ifi is used for sending multicast message. It uses the
// system assigned multicast network interface when ifi is nil.
func (rdr *AdvertRedirector) WriteTo(dst *net.UDPAddr, ifi *net.Interface) (int, error) {
	if ifi != nil {
		rdr.SetMulticastInterface(ifi)
	}
	var buf bytes.Buffer
	if err := marshalAdvert(&buf, rdr.req); err != nil {
		return 0, err
	}
	return rdr.writeTo(buf.Bytes(), dst)
}

// ForwardPath returns the destination address of the SSDP
// advertisement message.
func (rdr *AdvertRedirector) ForwardPath() *net.UDPAddr {
	return rdr.path.dst
}

// ReversePath returns the source address and inbound interface of the
// SSDP advertisement message.
func (rdr *AdvertRedirector) ReversePath() (*net.UDPAddr, *net.Interface) {
	return rdr.path.src, interfaceByIndex(rdr.mifs, rdr.path.ifIndex)
}

func newAdvertRedirector(conn conn, mifs []net.Interface, grp *net.UDPAddr, path *path, req *http.Request) *AdvertRedirector {
	rdr := &AdvertRedirector{
		conn: conn,
		mifs: mifs,
		path: path,
		req:  req,
	}
	path.dst.Port = grp.Port
	if ipv6LinkLocal(path.src.IP) {
		path.src.Zone = interfaceByIndex(mifs, path.ifIndex).Name
	}
	return rdr
}

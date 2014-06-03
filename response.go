// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
)

func parseResponse(b []byte) (*http.Response, error) {
	br := bufio.NewReader(bytes.NewBuffer(b))
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

type response struct {
	conn // network connection endpoint
	mifs []net.Interface
	path *path // reverse path
}

type responseWriter struct {
	response
	hdr    http.Header // response header
	wrthdr bool        // whether the header has been written
	buf    bytes.Buffer
	req    *http.Request
}

// Header implements the Header method of http.ResponseWriter
// interface.
func (resp *responseWriter) Header() http.Header {
	return resp.hdr
}

// Write implements the Write method of http.ResponseWriter interface.
func (resp *responseWriter) Write(b []byte) (int, error) {
	if !resp.wrthdr {
		resp.WriteHeader(http.StatusOK)
	}
	return resp.writeTo(b, resp.path.src)
}

// WriteHeader implements the WriteHeader method of
// http.ResponseWriter interface.
func (resp *responseWriter) WriteHeader(code int) {
	resp.wrthdr = true
	fmt.Fprintf(&resp.buf, "%s %d %s\r\n", resp.req.Proto, code, http.StatusText(code))
	resp.hdr.Write(&resp.buf)
	resp.buf.WriteString("\r\n")
	resp.writeTo(resp.buf.Bytes(), resp.path.src)
}

func newResponseWriter(conn conn, mifs []net.Interface, grp *net.UDPAddr, path *path, req *http.Request) *responseWriter {
	resp := &responseWriter{
		response: response{
			conn: conn,
			mifs: mifs,
			path: path,
		},
		hdr: make(http.Header),
		req: req,
	}
	resp.path.dst.Port = grp.Port
	if ipv6LinkLocal(path.src.IP) {
		path.src.Zone = interfaceByIndex(resp.mifs, resp.path.ifIndex).Name
	}
	return resp
}

// A ResponseRedirector represents a SSDP response message redirector.
type ResponseRedirector struct {
	response
	resp *http.Response
	buf  bytes.Buffer
}

// Header returns the HTTP header map that will be sent by WriteTo
// method.
func (rdr *ResponseRedirector) Header() http.Header {
	return rdr.resp.Header
}

// WriteTo writes the SSDP response message. The outbound network
// interface ifi is used for sending multicast messages. It uses the
// system assigned multicast network interface when ifi is nil.
func (rdr *ResponseRedirector) WriteTo(dst *net.UDPAddr, ifi *net.Interface) (int, error) {
	if ifi != nil {
		rdr.SetMulticastInterface(ifi)
	}
	fmt.Fprintf(&rdr.buf, "%s %s\r\n", rdr.resp.Proto, rdr.resp.Status)
	rdr.resp.Header.Write(&rdr.buf)
	rdr.buf.WriteString("\r\n")
	io.Copy(&rdr.buf, rdr.resp.Body)
	rdr.resp.Body.Close()
	return rdr.writeTo(rdr.buf.Bytes(), dst)
}

// ForwardPath returns the destination address of the SSDP response
// message.
func (rdr *ResponseRedirector) ForwardPath() *net.UDPAddr {
	return rdr.path.dst
}

// ReversePath returns the source address and inbound interface of the
// SSDP response message.
func (rdr *ResponseRedirector) ReversePath() (*net.UDPAddr, *net.Interface) {
	return rdr.path.src, interfaceByIndex(rdr.mifs, rdr.path.ifIndex)
}

func newResponseRedirector(conn conn, mifs []net.Interface, grp *net.UDPAddr, path *path, resp *http.Response) *ResponseRedirector {
	rdr := &ResponseRedirector{
		response: response{
			conn: conn,
			mifs: mifs,
			path: path,
		},
		resp: resp,
	}
	rdr.path.dst.Port = grp.Port
	if ipv6LinkLocal(path.src.IP) {
		path.src.Zone = interfaceByIndex(mifs, path.ifIndex).Name
	}
	return rdr
}

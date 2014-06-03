// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
)

var _ http.ResponseWriter = &response{}

type response struct {
	listener    *Listener
	transport   transport
	path        *path
	request     *http.Request
	header      http.Header
	wroteHeader bool
	buf         *bytes.Buffer
	bwr         *bufio.Writer
}

func (resp *response) Header() http.Header {
	return resp.header
}

func (resp *response) Write(b []byte) (int, error) {
	if !resp.wroteHeader {
		resp.WriteHeader(http.StatusOK)
	}
	return writeTo(resp.transport, b, resp.path.src)
}

func (resp *response) WriteHeader(code int) {
	resp.wroteHeader = true
	resp.request.Body.Close()
	fmt.Fprintf(resp.bwr, "%s %d %s\r\n", resp.request.Proto, code, http.StatusText(code))
	resp.header.Write(resp.bwr)
	resp.bwr.WriteString("\r\n")
	resp.bwr.Flush()
	writeTo(resp.transport, resp.buf.Bytes(), resp.path.src)
}

func newResponse(ln *Listener, tr transport, path *path, req *http.Request) *response {
	resp := &response{
		listener:  ln,
		transport: tr,
		path:      path,
		request:   req,
		header:    make(http.Header),
	}
	resp.buf = new(bytes.Buffer)
	resp.bwr = bufio.NewWriter(resp.buf)
	if ipv6LinkLocal(path.src.IP) {
		path.src.Zone = path.ifi.Name
	}
	return resp
}

func parseResponse(b []byte, req *http.Request) (*http.Response, error) {
	br := bufio.NewReader(bytes.NewBuffer(b))
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

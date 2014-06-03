// Copyright 2014 Mikio Hara. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssdp

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"net/url"
)

// NewRequest returns a http.Request given a method and host.
func NewRequest(method, host string) *http.Request {
	return &http.Request{
		Method:     method,
		URL:        &url.URL{},
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Host:       host,
	}
}

func parseRequest(b []byte) (*http.Request, error) {
	br := bufio.NewReader(bytes.NewBuffer(b))
	req, err := http.ReadRequest(br)
	if err != nil {
		return nil, err
	}
	switch req.Method {
	case "NOTIFY", "M-SEARCH":
	default:
		return nil, fmt.Errorf("unknown method: %v", req.Method)
	}
	if req.Proto != "HTTP/1.1" {
		return nil, fmt.Errorf("unknown version: %v", req.Proto)
	}
	return req, nil
}

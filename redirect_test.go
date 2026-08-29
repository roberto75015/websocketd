// Copyright 2026 Joe Walnes and the websocketd team.
// All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import "testing"

func TestRedirectAddress(t *testing.T) {
	cases := []struct {
		addr     string
		redirTo  int
		want     string
		wantErr  bool
	}{
		{addr: "127.0.0.1:8090", redirTo: 8098, want: "127.0.0.1:8098"},
		{addr: ":8090", redirTo: 8098, want: ":8098"},
		// Bracketed IPv6 literals: the first colon is inside the brackets, so
		// naive first-colon splitting produced a malformed address that failed
		// to bind (and killed every listener).
		{addr: "[::1]:8090", redirTo: 8098, want: "[::1]:8098"},
		{addr: "example.com:80", redirTo: 443, want: "example.com:443"},
		{addr: "no-port-here", redirTo: 8098, wantErr: true},
	}
	for _, c := range cases {
		got, err := redirectAddress(c.addr, c.redirTo)
		if c.wantErr {
			if err == nil {
				t.Errorf("redirectAddress(%q) = %q, want error", c.addr, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("redirectAddress(%q) unexpected error: %v", c.addr, err)
			continue
		}
		if got != c.want {
			t.Errorf("redirectAddress(%q, %d) = %q, want %q", c.addr, c.redirTo, got, c.want)
		}
	}
}

func TestRedirectLocation(t *testing.T) {
	cases := []struct {
		clientHost string
		listenAddr string
		ssl        bool
		want       string
	}{
		{clientHost: "example.com", listenAddr: "127.0.0.1:8090", want: "http://example.com:8090/"},
		// client's own port is discarded, canonical server port wins
		{clientHost: "example.com:9999", listenAddr: "127.0.0.1:8090", want: "http://example.com:8090/"},
		{clientHost: "example.com", listenAddr: "127.0.0.1:8090", ssl: true, want: "https://example.com:8090/"},
		// IPv6 client host must stay bracketed in the Location header
		{clientHost: "[::1]:9999", listenAddr: "[::1]:8090", want: "http://[::1]:8090/"},
		{clientHost: "[::1]", listenAddr: "[::1]:8090", want: "http://[::1]:8090/"},
	}
	for _, c := range cases {
		if got := redirectLocation(c.clientHost, c.listenAddr, c.ssl); got != c.want {
			t.Errorf("redirectLocation(%q, %q, %v) = %q, want %q", c.clientHost, c.listenAddr, c.ssl, got, c.want)
		}
	}
}

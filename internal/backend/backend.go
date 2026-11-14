// Package backend represents a single upstream RPC node the proxy can forward to.
package backend

import (
	"fmt"
	"net/http/httputil"
	"net/url"
)

// Backend is one upstream node. It owns the parsed target URL and its own
// ReverseProxy, so handlers can serve a request with b.Proxy.ServeHTTP.
type Backend struct {
	Name   string
	Target *url.URL
	Proxy  *httputil.ReverseProxy
}

// New constructs a Backend from a human name and a raw URL string.
// Returns an error if the URL does not parse or is missing scheme or host.
func New(name, rawURL string) (*Backend, error) {
	if name == "" {
		return nil, fmt.Errorf("backend: name is empty")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("backend %q: parse url: %w", name, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("backend %q: url scheme must be http or https, got %q", name, u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("backend %q: url host is empty", name)
	}
	return &Backend{
		Name:   name,
		Target: u,
		Proxy:  httputil.NewSingleHostReverseProxy(u),
	}, nil
}

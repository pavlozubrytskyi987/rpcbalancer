// Package backend models one upstream RPC node.
package backend

import (
	"fmt"
	"net/http/httputil"
	"net/url"
)

// Backend owns a parsed target URL and its own ReverseProxy.
type Backend struct {
	Name   string
	Target *url.URL
	Proxy  *httputil.ReverseProxy
}

// New parses rawURL and returns a ready Backend, or an error if the URL is unusable.
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

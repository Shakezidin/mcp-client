package transport

import (
	"net/http"
)

// NewHeaderRoundTripper returns an http.RoundTripper that injects configured headers into every outgoing request.
func NewHeaderRoundTripper(base http.RoundTripper, headers http.Header) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return &headerRoundTripper{
		base:    base,
		headers: headers,
	}
}

// headerRoundTripper injects configured headers into every outgoing request.
type headerRoundTripper struct {
	base    http.RoundTripper
	headers http.Header
}

func (rt *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.headers != nil {
		for name, values := range rt.headers {
			req.Header.Del(name)
			for _, value := range values {
				req.Header.Add(name, value)
			}
		}
	}
	return rt.base.RoundTrip(req)
}

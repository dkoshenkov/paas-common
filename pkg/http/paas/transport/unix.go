package transport

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

const (
	unixHost     = "paas.internal"
	BaseURL      = "http://" + unixHost
	RemoteSocket = "/run/paas/daemon/api.sock"
)

type unixTransport struct{ base *http.Transport }

func (t unixTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "http" || r.URL.Host != unixHost || r.URL.User != nil || (r.Host != "" && r.Host != unixHost) {
		return nil, fmt.Errorf("API transport only accepts its fixed Unix endpoint")
	}
	return t.base.RoundTrip(r)
}
func (t unixTransport) CloseIdleConnections() { t.base.CloseIdleConnections() }

func UnixClient(socket string) (*http.Client, error) {
	if !filepath.IsAbs(socket) || len(socket) > 103 || strings.ContainsRune(socket, 0) {
		return nil, fmt.Errorf("invalid Unix socket path")
	}
	dialer := &net.Dialer{Timeout: time.Second}
	tr := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, "unix", socket)
		},
		ResponseHeaderTimeout: 2 * time.Second, MaxResponseHeaderBytes: 16 << 10, IdleConnTimeout: 30 * time.Second,
	}
	return &http.Client{Transport: unixTransport{tr}, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}

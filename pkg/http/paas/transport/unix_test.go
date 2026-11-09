package transport

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	sdk "github.com/dkoshenkov/paas-common/pkg/http/client"
)

func TestUnixOnlyTransport(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	t.Setenv("ALL_PROXY", "http://127.0.0.1:1")
	dir, err := os.MkdirTemp("/tmp", "paas-unix-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(dir, "api.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "http://127.0.0.1:1/private", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})}
	go func() { _ = server.Serve(ln) }()
	defer func() { _ = server.Close() }()
	client, err := UnixClient(path)
	if err != nil {
		t.Fatal(err)
	}
	defer client.CloseIdleConnections()
	health, err := sdk.NewHealthClient(BaseURL, client)
	if err != nil {
		t.Fatal(err)
	}
	if response, err := health.Health(context.Background()); err != nil || response.Status != "ok" {
		t.Fatalf("health: %v %v", response, err)
	}
	response, err := client.Get(BaseURL + "/redirect")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 302 || requests.Load() != 2 {
		t.Fatal("redirect followed")
	}
	before := requests.Load()
	for _, url := range []string{"http://elsewhere/health", "https://paas.internal/health", "http://paas.internal:80/health", "http://u:p@paas.internal/health"} {
		if r, err := client.Get(url); err == nil {
			_ = r.Body.Close()
			t.Fatalf("accepted %s", url)
		}
	}
	req, _ := http.NewRequest(http.MethodGet, BaseURL+"/health", nil)
	req.Host = "elsewhere"
	if r, err := client.Do(req); err == nil {
		_ = r.Body.Close()
		t.Fatal("accepted foreign Host")
	}
	if requests.Load() != before {
		t.Fatal("unexpected endpoint reached listener")
	}
	// Prove rejection happens before dial even if a dialer could reach a server.
	var dials atomic.Int32
	client.Transport.(unixTransport).base.DialContext = func(context.Context, string, string) (net.Conn, error) {
		dials.Add(1)
		return nil, fmt.Errorf("unexpected dial")
	}
	if _, err := client.Get("http://foreign/health"); err == nil || dials.Load() != 0 {
		t.Fatal("foreign endpoint dialed")
	}
	for _, bad := range []string{"relative", "/tmp/x\x00", "/" + strings.Repeat("x", 104)} {
		if _, err := UnixClient(bad); err == nil {
			t.Fatal("accepted invalid socket path")
		}
	}
}

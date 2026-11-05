package gate

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// This intentionally fails until the actual parser contract is satisfied.
// It must not be weakened to a post-tree production validation.
func TestRequiredEarlyDepth64(t *testing.T) {
	n, err := load(block(32, flow(33)))
	if err == nil {
		t.Fatalf("BLOCKED: parser returned a complete tree of depth %d despite depth limit64", depth(&n))
	}
}

func TestHTTPWireLimit(t *testing.T) {
	const limit = 8 << 20
	for _, size := range []int{limit, limit + 1} {
		body := io.NopCloser(strings.NewReader(strings.Repeat("x", size)))
		reader := http.MaxBytesReader(httptest.NewRecorder(), body, limit)
		read, err := io.Copy(io.Discard, reader)
		if size == limit && err != nil {
			t.Fatal(err)
		}
		if size > limit {
			if _, ok := err.(*http.MaxBytesError); !ok {
				t.Fatalf("expected MaxBytesError, got %v", err)
			}
		}
		t.Logf("wirebytes=%d consumed=%d err=%v", size, read, err)
	}
}

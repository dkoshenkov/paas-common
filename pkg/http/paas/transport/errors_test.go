package transport_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dkoshenkov/packages-go/middlewarex"

	"github.com/dkoshenkov/paas-common/pkg/http/paas/transport"
	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

func TestEncodeErrorHidesCause(t *testing.T) {
	secret := errors.New("private-token=secret-input")
	for _, tc := range []struct {
		name   string
		cause  error
		status int
		code   string
	}{
		{"timeout", middlewarex.TimeoutError(secret), 503, "DEPENDENCY_UNAVAILABLE"},
		{"internal", secret, 500, "INTERNAL_ERROR"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			transport.EncodeError(w, httptest.NewRequest(http.MethodPost, "/deployments/apply", nil), tc.cause)
			var body paastypes.TypesErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || body.Code != tc.code || body.Diagnostics == nil || body.ConflictingOperationId != nil {
				t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
			}
			if strings.Contains(w.Body.String(), "secret-input") {
				t.Fatal("internal cause exposed")
			}
		})
	}
}

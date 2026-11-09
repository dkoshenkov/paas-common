package validation_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/databaselifecycle"
	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

type lifecycleFixture struct{}

func (lifecycleFixture) Get(context.Context, t.DatabaseLifecycleDatabaseLifecycleGetQuery) (databaselifecycle.DatabaseLifecycleGetResponse, error) {
	return databaselifecycle.DatabaseLifecycleGetResponse200{}, nil
}

func (lifecycleFixture) Delete(context.Context, t.DatabaseLifecycleDeleteDatabaseRequest) (databaselifecycle.DatabaseLifecycleDeleteResponse, error) {
	return databaselifecycle.DatabaseLifecycleDeleteResponse202{}, nil
}

func (lifecycleFixture) RevokeAccess(context.Context, t.DatabaseLifecycleRevokeDatabaseAccessRequest) (databaselifecycle.DatabaseLifecycleRevokeAccessResponse, error) {
	return databaselifecycle.DatabaseLifecycleRevokeAccessResponse202{}, nil
}

func TestDatabaseLifecycleBodyLimits(test *testing.T) {
	mux := http.NewServeMux()
	databaselifecycle.RegisterDatabaseLifecycleHandlers(mux, lifecycleFixture{})
	for _, path := range []string{"/apps/database/revoke-access", "/apps/database/delete"} {
		for _, tc := range []struct {
			body   string
			status int
		}{{`{"unknown":true}`, 400}, {`{"x":"` + strings.Repeat("a", 1<<20) + `"}`, 413}} {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			mux.ServeHTTP(w, req)
			if w.Code != tc.status {
				test.Fatalf("%s: got %d want %d", path, w.Code, tc.status)
			}
		}
	}
}

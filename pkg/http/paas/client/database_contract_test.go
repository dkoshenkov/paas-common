package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dkoshenkov/packages-go/middlewarex/httpx"

	"github.com/dkoshenkov/paas-common/pkg/http/paas/client"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/database"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/transport"
	m "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	v "github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

type credentialFixture struct{ calls int }

func (f *credentialFixture) Credentials(_ context.Context, q m.DatabaseCredentialsRequest) (database.DatabaseCredentialsResponse, error) {
	f.calls++
	if v.CredentialsRequest(q) != nil {
		return database.DatabaseCredentialsResponse400{Body: apiError("INVALID_REQUEST")}, nil
	}
	switch q.AppId {
	case "missing":
		return database.DatabaseCredentialsResponse404{Body: apiError("NOT_FOUND")}, nil
	case "pending":
		return database.DatabaseCredentialsResponse409{Body: apiError("DATABASE_NOT_READY")}, nil
	case "unavailable":
		return database.DatabaseCredentialsResponse503{Body: apiError("DEPENDENCY_UNAVAILABLE")}, nil
	case "corrupt":
		return nil, errors.New("fixture-secret")
	}
	db, _ := v.ApplicationDatabaseName(q.AppId)
	username := db
	if q.Purpose == v.MigratorPurpose {
		username += "_migrator"
	}
	return database.DatabaseCredentialsResponse200{Body: m.DatabaseCredentialsResponse{AppId: q.AppId, Purpose: q.Purpose, Host: "postgres.paas-postgres.svc", Port: 5432, Database: db, Username: username, Password: "fixture-secret"}}, nil
}

func TestDatabaseCredentialsWireAndCache(t *testing.T) {
	fixture := new(credentialFixture)
	mux := http.NewServeMux()
	runtime := httpx.NewRuntime(httpx.WithErrorEncoder(httpx.ErrorEncoderFunc(transport.EncodeError)))
	transport.RegisterDatabaseHandlers(mux, fixture, runtime)
	svc, _ := client.NewDatabaseClient(transport.BaseURL, &http.Client{Transport: handlerTransport{handler: mux}})
	for _, purpose := range []string{v.ApplicationPurpose, v.MigratorPurpose} {
		q := m.DatabaseCredentialsRequest{AppId: "orders", Purpose: purpose}
		r, err := svc.Credentials(context.Background(), q)
		got, ok := r.(client.DatabaseCredentialsResponse200)
		if err != nil || !ok || got.Headers.Get("Cache-Control") != "no-store" || v.CredentialsResponse(q, got.Body) != nil {
			t.Fatal("invalid successful reveal")
		}
	}
	const path = "/apps/database/credentials"
	for _, tc := range []struct {
		status       int
		method, body string
		calls        int
	}{
		{400, "POST", `{"app_id":"x","purpose":"ADMIN"}`, 1},
		{400, "POST", `{"app_id":"x","purpose":null}`, 0},
		{400, "POST", `{"app_id":"x","purpose":"APPLICATION","other":"fixture-secret"}`, 0},
		{404, "POST", `{"app_id":"missing","purpose":"APPLICATION"}`, 1},
		{409, "POST", `{"app_id":"pending","purpose":"APPLICATION"}`, 1},
		{503, "POST", `{"app_id":"unavailable","purpose":"APPLICATION"}`, 1},
		{500, "POST", `{"app_id":"corrupt","purpose":"APPLICATION"}`, 1},
		{405, "GET", "", 0},
	} {
		prior := fixture.calls
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(tc.method, path, strings.NewReader(tc.body)))
		if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "fixture-secret") || fixture.calls != prior+tc.calls {
			t.Fatalf("missing redaction/cache/dispatch for %s %d", tc.method, tc.status)
		}
	}
	base := `{"app_id":"orders","purpose":"APPLICATION"}`
	for _, stream := range []bool{false, true} {
		for _, n := range []int{1 << 20, (1 << 20) + 1} {
			before := fixture.calls
			req := httptest.NewRequest("POST", path, strings.NewReader(base+strings.Repeat(" ", n-len(base))))
			if stream {
				req.ContentLength = -1
				req.TransferEncoding = []string{"chunked"}
			}
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing no-store at boundary")
			}
			if n == 1<<20 {
				if w.Code != 200 || fixture.calls != before+1 {
					t.Fatal("rejected inclusive limit")
				}
			} else {
				var e m.TypesErrorResponse
				if w.Code != 413 || json.Unmarshal(w.Body.Bytes(), &e) != nil || e.Code != "BODY_TOO_LARGE" || fixture.calls != before {
					t.Fatal("overflow reached service")
				}
			}
		}
	}
}

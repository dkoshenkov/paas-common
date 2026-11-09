package client_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/dkoshenkov/paas-common/pkg/http/paas/client"
	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

func TestDatabaseLifecycleClientAndHistoryOptIn(test *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /apps/database/lifecycle", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("app_id") != "app" {
			test.Error(r.URL)
		}
		_ = json.NewEncoder(w).Encode(t.DatabaseLifecycleDatabaseLifecycleSnapshot{AppId: "app", DatabaseName: "db", Revision: "r1", DatabaseState: "READY", ApplicationAccess: "ACTIVE", MigratorAccess: "ACTIVE"})
	})
	for _, path := range []string{"/apps/database/revoke-access", "/apps/database/delete"} {
		mux.HandleFunc("POST "+path, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"operation_id":"operation"}`))
		})
	}
	mux.HandleFunc("GET /operations/history", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("include_database_actions") != "true" {
			test.Error(r.URL)
		}
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	httpClient := &http.Client{Transport: handlerTransport{handler: mux}}
	c, err := client.NewDatabaseLifecycleClient("http://paas.test", httpClient)
	if err != nil {
		test.Fatal(err)
	}
	if _, err = c.Get(test.Context(), t.DatabaseLifecycleDatabaseLifecycleGetQuery{AppId: "app"}); err != nil {
		test.Fatal(err)
	}
	if _, err = c.RevokeAccess(test.Context(), t.DatabaseLifecycleRevokeDatabaseAccessRequest{AppId: "app", RequestId: "req", ExpectedRevision: "r1", Purpose: "APPLICATION"}); err != nil {
		test.Fatal(err)
	}
	if _, err = c.Delete(test.Context(), t.DatabaseLifecycleDeleteDatabaseRequest{AppId: "app", RequestId: "req", ExpectedRevision: "r1", ConfirmDatabaseName: "db"}); err != nil {
		test.Fatal(err)
	}
	history, err := client.NewMigrationsClient("http://paas.test", httpClient)
	if err != nil {
		test.Fatal(err)
	}
	include := true
	if _, err = history.History(test.Context(), t.MigrationsMigrationsHistoryQuery{IncludeDatabaseActions: &include}); err != nil {
		test.Fatal(err)
	}
}

package transport

import (
	"net/http"

	"github.com/dkoshenkov/packages-go/middlewarex/httpx"

	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/database"
)

// RegisterDatabaseHandlers sets the reveal endpoint's cache policy outside the
// generated decoder. Validation failures and oversized bodies must follow it too.
func RegisterDatabaseHandlers(mux *http.ServeMux, service database.DatabaseServer, runtime httpx.Runtime) {
	reveal := http.NewServeMux()
	database.RegisterDatabaseHandlersWithRuntime(reveal, service, runtime)
	mux.Handle("/apps/database/credentials", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		reveal.ServeHTTP(w, r)
	}))
}

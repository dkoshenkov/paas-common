package validation

import (
	"strings"
	"testing"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

func TestCredentialsIdentityAndSafeValues(test *testing.T) {
	id := strings.Repeat("a", 32)
	q := t.DatabaseCredentialsRequest{AppId: id, Purpose: "APPLICATION"}
	r := t.DatabaseCredentialsResponse{AppId: id, Purpose: q.Purpose, Host: "postgres.paas-postgres.svc", Port: 5432, Database: "app_" + id, Username: "app_" + id, Password: "redacted-fixture"}
	if err := CredentialsResponse(q, r); err != nil {
		test.Fatal(err)
	}
	for _, change := range []func(*t.DatabaseCredentialsResponse){func(v *t.DatabaseCredentialsResponse) { v.AppId = "other" }, func(v *t.DatabaseCredentialsResponse) { v.Purpose = "MIGRATOR" }, func(v *t.DatabaseCredentialsResponse) { v.Username += "_migrator" }, func(v *t.DatabaseCredentialsResponse) { v.Database = "paas_control" }, func(v *t.DatabaseCredentialsResponse) { v.Port = 22 }, func(v *t.DatabaseCredentialsResponse) { v.Host = "postgres://user@host" }, func(v *t.DatabaseCredentialsResponse) { v.Password = "\x1b[Hsecret" }} {
		bad := r
		change(&bad)
		if err := CredentialsResponse(q, bad); err == nil || strings.Contains(err.Error(), r.Password) {
			test.Fatal("unsafe or incorrectly accepted response")
		}
	}
	q.Purpose = "MIGRATOR"
	r.Purpose = q.Purpose
	r.Username += "_migrator"
	if err := CredentialsResponse(q, r); err != nil {
		test.Fatal(err)
	}
	if _, err := ApplicationDatabaseName("bad id"); err == nil {
		test.Fatal("accepted invalid ID")
	}
	if n, err := ApplicationDatabaseName("legacy"); err != nil || n != "app_f7ebaeec628f5aca7381d34c0303a77e" {
		test.Fatal("invalid legacy name")
	}
}

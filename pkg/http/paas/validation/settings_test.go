package validation_test

import (
	"errors"
	"strings"
	"testing"

	model "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	v "github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

func emptyPolicy() model.SettingsTypesComponentPolicy {
	return model.SettingsTypesComponentPolicy{
		DatabaseGrants: []model.SettingsTypesDatabaseGrant{}, External: model.SettingsTypesExternal{Mode: "none", Domains: []string{}}, ServiceGrants: []model.SettingsTypesServiceGrant{},
	}
}

func grant() model.SettingsTypesDatabaseGrant {
	return model.SettingsTypesDatabaseGrant{DeploymentName: "backend.v1", ContainerKind: "container", ContainerName: "worker", Keys: pointer([]string{"username", "password"})}
}

func TestSettingsPolicy(tt *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*model.SettingsTypesComponentPolicy)
		valid  bool
	}{
		{"default", func(*model.SettingsTypesComponentPolicy) {}, true},
		{"all", func(p *model.SettingsTypesComponentPolicy) { p.External.Mode = "all" }, true},
		{"nil grants", func(p *model.SettingsTypesComponentPolicy) { p.DatabaseGrants = nil }, false},
		{"nil services", func(p *model.SettingsTypesComponentPolicy) { p.ServiceGrants = nil }, false},
		{"nil domains", func(p *model.SettingsTypesComponentPolicy) { p.External.Domains = nil }, false},
		{"unexpected domains", func(p *model.SettingsTypesComponentPolicy) { p.External.Domains = []string{"example.com"} }, false},
		{"unknown mode", func(p *model.SettingsTypesComponentPolicy) { p.External.Mode = "unspecified" }, false},
		{"key list", func(p *model.SettingsTypesComponentPolicy) {
			p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{grant()}
		}, true},
		{"init whole secret", func(p *model.SettingsTypesComponentPolicy) {
			g := grant()
			g.ContainerKind = "init"
			g.Keys = nil
			g.WholeSecret = pointer(true)
			p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{g}
		}, true},
		{"both permissions", func(p *model.SettingsTypesComponentPolicy) {
			g := grant()
			g.WholeSecret = pointer(true)
			p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{g}
		}, false},
		{"empty whole permission", func(p *model.SettingsTypesComponentPolicy) {
			g := grant()
			g.Keys = nil
			g.WholeSecret = pointer(false)
			p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{g}
		}, false},
		{"missing permissions", func(p *model.SettingsTypesComponentPolicy) {
			g := grant()
			g.Keys = nil
			p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{g}
		}, false},
		{"empty keys", func(p *model.SettingsTypesComponentPolicy) {
			g := grant()
			g.Keys = pointer([]string{})
			p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{g}
		}, false},
		{"invalid secret key", func(p *model.SettingsTypesComponentPolicy) {
			g := grant()
			g.Keys = pointer([]string{"../secret"})
			p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{g}
		}, false},
		{"repeated key", func(p *model.SettingsTypesComponentPolicy) {
			g := grant()
			g.Keys = pointer([]string{"x", "x"})
			p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{g}
		}, false},
		{"container kind", func(p *model.SettingsTypesComponentPolicy) {
			g := grant()
			g.ContainerKind = "ephemeral"
			p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{g}
		}, false},
		{"invalid container name", func(p *model.SettingsTypesComponentPolicy) {
			g := grant()
			g.ContainerName = "db.app"
			p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{g}
		}, false},
		{"foreign deployment path", func(p *model.SettingsTypesComponentPolicy) {
			g := grant()
			g.DeploymentName = "ns/api"
			p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{g}
		}, false},
		{"duplicate target", func(p *model.SettingsTypesComponentPolicy) {
			p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{grant(), grant()}
		}, false},
		{"service reference", func(p *model.SettingsTypesComponentPolicy) {
			p.ServiceGrants = []model.SettingsTypesServiceGrant{{TargetAppId: "other", TargetComponentId: "api", ServiceName: "mail-svc"}}
		}, true},
		{"incomplete service", func(p *model.SettingsTypesComponentPolicy) {
			p.ServiceGrants = []model.SettingsTypesServiceGrant{{TargetAppId: "other", ServiceName: "mail-svc"}}
		}, false},
	} {
		tt.Run(tc.name, func(tt *testing.T) {
			p := emptyPolicy()
			tc.mutate(&p)
			q := model.SettingsTypesSetPolicyRequest{AppId: "a", ComponentId: "c", ExpectedRevision: "r1", Policy: p}
			if err := v.SetPolicyRequest(q); (err == nil) != tc.valid {
				tt.Fatal(err)
			}
		})
	}
}

func TestSettingsAllowlistReadAndWrite(tt *testing.T) {
	p := emptyPolicy()
	p.External.Mode = "allowlist"
	p.External.Domains = []string{"mail.example.com"}
	q := model.SettingsTypesSetPolicyRequest{AppId: "a", ComponentId: "c", ExpectedRevision: "r1", Policy: p}
	if err := v.SetPolicyRequest(q); !errors.Is(err, v.ErrEgressAllowlistUnavailable) {
		tt.Fatal(err)
	}
	if err := v.ComponentPolicySnapshot(model.SettingsTypesComponentPolicySnapshot{AppId: "a", ComponentId: "c", Revision: "r1", Policy: p}); err != nil {
		tt.Fatal(err)
	}
	for _, domain := range []string{"*", "*.example.com", "https://example.com", "mail.example:443", "127.0.0.1", "fe80::1", "K.example"} {
		q.Policy.External.Domains = []string{domain}
		if err := v.SetPolicyRequest(q); err == nil || errors.Is(err, v.ErrEgressAllowlistUnavailable) {
			tt.Fatalf("invalid domain %q got %v", domain, err)
		}
	}
}

func TestSettingsRegistrationAndRevisions(tt *testing.T) {
	for _, n := range []int{95, 96, 97} {
		id := strings.Repeat("x", n)
		if err := v.CreateAppRequest(model.SettingsTypesCreateAppRequest{RequestId: id, Name: "app"}); (err == nil) != (n <= 96) {
			tt.Fatal(n, err)
		}
		if err := v.RemoveMigratorRequest(model.SettingsTypesRemoveMigratorRequest{AppId: "a", ExpectedRevision: id}); (err == nil) != (n <= 96) {
			tt.Fatal(n, err)
		}
	}
	for _, id := range []string{"", "bad\n", "\xff"} {
		if v.CreateComponentRequest(model.SettingsTypesCreateComponentRequest{AppId: "a", Name: "n", RequestId: id}) == nil {
			tt.Fatal("invalid request_id")
		}
	}
	if v.CreateAppRequest(model.SettingsTypesCreateAppRequest{Name: strings.Repeat("я", 65), RequestId: "opaque"}) == nil {
		tt.Fatal("oversized display name")
	}
	if v.MigratorSnapshot(model.SettingsTypesMigratorSnapshot{AppId: "a", Revision: "initial"}) != nil {
		tt.Fatal("empty setting must retain revision")
	}
	if v.ComponentPolicyQuery(model.SettingsSettingsComponentPolicyQuery{AppId: "foreign app", ComponentId: "c"}) == nil {
		tt.Fatal("invalid app ID")
	}
	if v.AppMigratorQuery(model.SettingsSettingsAppMigratorQuery{}) == nil {
		tt.Fatal("missing app ID")
	}
}

func TestSettingsMigratorImage(tt *testing.T) {
	digest := "sha256:" + strings.Repeat("a", 64)
	for _, tc := range []struct {
		image string
		valid bool
	}{
		{"ghcr.io/org/schema@" + digest, true},
		{"registry.example:5000/repo/schema:v2@" + digest, true},
		{"[2001:db8::1]:5000/schema@" + digest, true},
		{"ghcr.io/org/schema:latest", false},
		{"@" + digest, false},
		{"registry/path/@" + digest, false},
		{"https://registry/schema@" + digest, false},
		{"ghcr.io/org/schema@sha256:abcd", false},
		{"ghcr.io/org/schema@sha512:" + strings.Repeat("a", 128), false},
	} {
		q := model.SettingsTypesSetMigratorRequest{AppId: "app", ExpectedRevision: "r1", Migrator: model.SettingsTypesMigrator{Image: tc.image}}
		if err := v.SetMigratorRequest(q); (err == nil) != tc.valid {
			tt.Fatal(tc.image, err)
		}
	}
}

func TestSettingsBoundaryNames(tt *testing.T) {
	app := model.SettingsTypesCreateComponentRequest{AppId: "a", Name: "valid", RequestId: "k"}
	if v.CreateComponentRequest(app) != nil {
		tt.Fatal("valid component rejected")
	}
	app.AppId = "/"
	if v.CreateComponentRequest(app) == nil {
		tt.Fatal("invalid app ID accepted")
	}
	if v.MigratorSnapshot(model.SettingsTypesMigratorSnapshot{AppId: "a", Revision: "", Migrator: nil}) == nil {
		tt.Fatal("empty revision accepted")
	}
	name63 := strings.Repeat("a", 63)
	domain253 := strings.Repeat(name63+".", 3) + strings.Repeat("b", 61)
	for _, tc := range []struct {
		domain string
		ok     bool
	}{{name63, true}, {name63 + "a", false}, {domain253, true}, {domain253 + "b", false}} {
		p := emptyPolicy()
		p.External = model.SettingsTypesExternal{Mode: "allowlist", Domains: []string{tc.domain}}
		err := v.ComponentPolicySnapshot(model.SettingsTypesComponentPolicySnapshot{AppId: "a", ComponentId: "c", Revision: "r", Policy: p})
		if (err == nil) != tc.ok {
			tt.Fatal(tc.domain, err)
		}
	}
	for _, tc := range []struct {
		key string
		ok  bool
	}{{strings.Repeat("a", 253), true}, {strings.Repeat("a", 254), false}, {"a/b", false}, {".", false}, {"..x", false}} {
		p := emptyPolicy()
		g := grant()
		g.Keys = pointer([]string{tc.key})
		p.DatabaseGrants = []model.SettingsTypesDatabaseGrant{g}
		if err := v.ComponentPolicySnapshot(model.SettingsTypesComponentPolicySnapshot{AppId: "a", ComponentId: "c", Revision: "r", Policy: p}); (err == nil) != tc.ok {
			tt.Fatal(tc.key, err)
		}
	}
}

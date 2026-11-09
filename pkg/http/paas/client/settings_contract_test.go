package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/dkoshenkov/packages-go/middlewarex/httpx"

	"github.com/dkoshenkov/paas-common/pkg/http/paas/client"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/settings"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/transport"
	m "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	v "github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

// Echo validated targets through the generated boundary. This fixture does
// not persist requests or implement writer-side concurrency.
type settingsFixture struct{ calls int }

func (s *settingsFixture) CreateApp(_ context.Context, q m.SettingsTypesCreateAppRequest) (settings.SettingsCreateAppResponse, error) {
	s.calls++
	summary := m.CatalogTypesAppSummary{AppId: "app", Name: q.Name}
	switch q.RequestId {
	case "replayed":
		return settings.SettingsCreateAppResponse200{Body: summary}, nil
	case "conflict":
		return settings.SettingsCreateAppResponse409{Body: apiError("REQUEST_ID_CONFLICT")}, nil
	default:
		return settings.SettingsCreateAppResponse201{Body: summary}, nil
	}
}

func (s *settingsFixture) CreateComponent(_ context.Context, q m.SettingsTypesCreateComponentRequest) (settings.SettingsCreateComponentResponse, error) {
	s.calls++
	summary := m.CatalogTypesComponentSummary{AppId: q.AppId, ComponentId: "api", Name: q.Name}
	if q.RequestId == "replayed" {
		return settings.SettingsCreateComponentResponse200{Body: summary}, nil
	}
	return settings.SettingsCreateComponentResponse201{Body: summary}, nil
}

func policyFixture() m.SettingsTypesComponentPolicy {
	return m.SettingsTypesComponentPolicy{DatabaseGrants: []m.SettingsTypesDatabaseGrant{}, External: m.SettingsTypesExternal{Mode: "none", Domains: []string{}}, ServiceGrants: []m.SettingsTypesServiceGrant{}}
}

func (s *settingsFixture) ComponentPolicy(_ context.Context, q m.SettingsSettingsComponentPolicyQuery) (settings.SettingsComponentPolicyResponse, error) {
	s.calls++
	if q.ComponentId == "missing" {
		return settings.SettingsComponentPolicyResponse404{Body: apiError("NOT_FOUND")}, nil
	}
	return settings.SettingsComponentPolicyResponse200{Body: m.SettingsTypesComponentPolicyObservedSnapshot{AppId: q.AppId, ComponentId: q.ComponentId, Revision: "r0", Policy: policyFixture()}}, nil
}

func (s *settingsFixture) SetComponentPolicy(_ context.Context, q m.SettingsTypesSetPolicyRequest) (settings.SettingsSetComponentPolicyResponse, error) {
	s.calls++
	if q.ExpectedRevision != "r0" {
		return settings.SettingsSetComponentPolicyResponse409{Body: apiError("POLICY_CHANGED")}, nil
	}
	if err := v.SetPolicyRequest(q); err == v.ErrEgressAllowlistUnavailable {
		return settings.SettingsSetComponentPolicyResponse409{Body: apiError(err.Error())}, nil
	} else if err != nil {
		return settings.SettingsSetComponentPolicyResponse400{Body: apiError("INVALID_REQUEST")}, nil
	}
	return settings.SettingsSetComponentPolicyResponse200{Body: m.SettingsTypesComponentPolicySnapshot{AppId: q.AppId, ComponentId: q.ComponentId, Revision: "r1", Policy: q.Policy}}, nil
}

func (s *settingsFixture) AppMigrator(_ context.Context, q m.SettingsSettingsAppMigratorQuery) (settings.SettingsAppMigratorResponse, error) {
	s.calls++
	if q.AppId == "unavailable" {
		return settings.SettingsAppMigratorResponse503{Body: apiError("DEPENDENCY_UNAVAILABLE")}, nil
	}
	return settings.SettingsAppMigratorResponse200{Body: m.SettingsTypesMigratorSnapshot{AppId: q.AppId, Revision: "initial"}}, nil
}

func (s *settingsFixture) SetAppMigrator(_ context.Context, q m.SettingsTypesSetMigratorRequest) (settings.SettingsSetAppMigratorResponse, error) {
	s.calls++
	return settings.SettingsSetAppMigratorResponse200{Body: m.SettingsTypesMigratorSnapshot{AppId: q.AppId, Revision: "next", Migrator: &q.Migrator}}, nil
}

func (s *settingsFixture) RemoveAppMigrator(_ context.Context, q m.SettingsTypesRemoveMigratorRequest) (settings.SettingsRemoveAppMigratorResponse, error) {
	s.calls++
	return settings.SettingsRemoveAppMigratorResponse200{Body: m.SettingsTypesMigratorSnapshot{AppId: q.AppId, Revision: "removed"}}, nil
}

func settingsHTTP(s *settingsFixture) *http.Client {
	mux := http.NewServeMux()
	runtime := httpx.NewRuntime(httpx.WithErrorEncoder(httpx.ErrorEncoderFunc(transport.EncodeError)))
	settings.RegisterSettingsHandlersWithRuntime(mux, s, runtime)
	return &http.Client{Transport: handlerTransport{handler: mux}}
}

func TestSettingsClientAndHandler(t *testing.T) {
	fixture := new(settingsFixture)
	c, err := client.NewSettingsClient("http://paas.test", settingsHTTP(fixture))
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	check := func(got any, err error, want any) {
		t.Helper()
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, %v; want %#v", got, err, want)
		}
	}

	app := m.CatalogTypesAppSummary{AppId: "app", Name: "Orders"}
	for _, tc := range []struct {
		id   string
		want any
	}{{"new", client.SettingsCreateAppResponse201{Body: app}}, {"replayed", client.SettingsCreateAppResponse200{Body: app}}, {"conflict", client.SettingsCreateAppResponse409{Body: apiError("REQUEST_ID_CONFLICT")}}} {
		got, err := c.CreateApp(ctx, m.SettingsTypesCreateAppRequest{RequestId: tc.id, Name: app.Name})
		check(got, err, tc.want)
	}
	component := m.CatalogTypesComponentSummary{AppId: "app", ComponentId: "api", Name: "API"}
	for _, tc := range []struct {
		id   string
		want any
	}{{"new", client.SettingsCreateComponentResponse201{Body: component}}, {"replayed", client.SettingsCreateComponentResponse200{Body: component}}} {
		got, err := c.CreateComponent(ctx, m.SettingsTypesCreateComponentRequest{RequestId: tc.id, AppId: "app", Name: component.Name})
		check(got, err, tc.want)
	}
	initial := m.SettingsTypesComponentPolicySnapshot{AppId: "app", ComponentId: "api", Revision: "r0", Policy: policyFixture()}
	gotPolicy, err := c.ComponentPolicy(ctx, m.SettingsSettingsComponentPolicyQuery{AppId: "app", ComponentId: "api"})
	check(gotPolicy, err, client.SettingsComponentPolicyResponse200{Body: m.SettingsTypesComponentPolicyObservedSnapshot{AppId: initial.AppId, ComponentId: initial.ComponentId, Revision: initial.Revision, Policy: initial.Policy}})
	gotPolicy, err = c.ComponentPolicy(ctx, m.SettingsSettingsComponentPolicyQuery{AppId: "other", ComponentId: "missing"})
	check(gotPolicy, err, client.SettingsComponentPolicyResponse404{Body: apiError("NOT_FOUND")})
	changed := initial
	changed.Revision = "r1"
	changed.Policy.DatabaseGrants = []m.SettingsTypesDatabaseGrant{{DeploymentName: "api", ContainerName: "api", ContainerKind: "container", Keys: pointer([]string{"password"})}, {DeploymentName: "api", ContainerName: "bootstrap", ContainerKind: "init", WholeSecret: pointer(true)}}
	q := m.SettingsTypesSetPolicyRequest{AppId: "app", ComponentId: "api", ExpectedRevision: "r0", Policy: changed.Policy}
	gotSet, err := c.SetComponentPolicy(ctx, q)
	check(gotSet, err, client.SettingsSetComponentPolicyResponse200{Body: changed})
	q.ExpectedRevision = "old"
	gotSet, err = c.SetComponentPolicy(ctx, q)
	check(gotSet, err, client.SettingsSetComponentPolicyResponse409{Body: apiError("POLICY_CHANGED")})
	q.ExpectedRevision = "r0"
	q.Policy.External = m.SettingsTypesExternal{Mode: "allowlist", Domains: []string{"mail.example.com"}}
	gotSet, err = c.SetComponentPolicy(ctx, q)
	check(gotSet, err, client.SettingsSetComponentPolicyResponse409{Body: apiError("EGRESS_ALLOWLIST_UNAVAILABLE")})

	m0 := m.SettingsTypesMigratorSnapshot{AppId: "app", Revision: "initial"}
	gotM, err := c.AppMigrator(ctx, m.SettingsSettingsAppMigratorQuery{AppId: "app"})
	check(gotM, err, client.SettingsAppMigratorResponse200{Body: m0})
	gotM, err = c.AppMigrator(ctx, m.SettingsSettingsAppMigratorQuery{AppId: "unavailable"})
	check(gotM, err, client.SettingsAppMigratorResponse503{Body: apiError("DEPENDENCY_UNAVAILABLE")})
	image := m.SettingsTypesMigrator{Image: "ghcr.io/org/schema@sha256:" + strings.Repeat("a", 64)}
	setM, err := c.SetAppMigrator(ctx, m.SettingsTypesSetMigratorRequest{AppId: "app", ExpectedRevision: "initial", Migrator: image})
	check(setM, err, client.SettingsSetAppMigratorResponse200{Body: m.SettingsTypesMigratorSnapshot{AppId: "app", Revision: "next", Migrator: &image}})
	removed, err := c.RemoveAppMigrator(ctx, m.SettingsTypesRemoveMigratorRequest{AppId: "app", ExpectedRevision: "next"})
	check(removed, err, client.SettingsRemoveAppMigratorResponse200{Body: m.SettingsTypesMigratorSnapshot{AppId: "app", Revision: "removed"}})
}

func TestSettingsMalformedRequestsNeverReachService(t *testing.T) {
	fixture := new(settingsFixture)
	h := settingsHTTP(fixture)
	for _, tc := range []struct{ method, path, body string }{
		{"POST", "/apps/create", `{"name":"x","request_id":"i","secret":"sensitive-canary"}`},
		{"POST", "/components/set-policy", `{"app_id":"a","component_id":"c","expected_revision":"r","policy":{"database_grants":[],"service_grants":[],"external":{"mode":"none","domains":[],"sensitive-canary":true}}}`},
		{"POST", "/apps/set-migrator", `{"app_id":"a","expected_revision":"r","migrator":null}`},
		{"POST", "/apps/remove-migrator", `{"app_id":"a"}`},
		{"GET", "/components/policy?app_id=a&app_id=b&component_id=c", ""},
		{"GET", "/apps/migrator", ""},
	} {
		req, err := http.NewRequestWithContext(t.Context(), tc.method, "http://paas.test"+tc.path, strings.NewReader(tc.body))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := h.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		var rejected m.TypesErrorResponse
		if resp.StatusCode != 400 || json.Unmarshal(body, &rejected) != nil || rejected.Code != "INVALID_REQUEST" || strings.Contains(string(body), "sensitive-canary") {
			t.Fatalf("route %s: %d %s", tc.path, resp.StatusCode, body)
		}
	}
	if fixture.calls != 0 {
		t.Fatal("invalid request reached service")
	}
}

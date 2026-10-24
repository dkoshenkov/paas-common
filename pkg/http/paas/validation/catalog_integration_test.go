package validation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/dkoshenkov/packages-go/middlewarex/httpx"

	"github.com/dkoshenkov/paas-common/pkg/http/paas/client"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/apps"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/components"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/deployments"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/transport"
	pt "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

// This fixture only demonstrates SDK boundaries, not database cursor/lookup behavior.
type catalogFixture struct {
	unavailable, badPage bool
	lookups              int
	listQuery            pt.CatalogAppsListQuery
	componentQuery       pt.CatalogComponentsListQuery
}
type (
	appsFixture       struct{ f *catalogFixture }
	componentsFixture struct{ f *catalogFixture }
)

func catalogError(code string) pt.TypesErrorResponse {
	return pt.TypesErrorResponse{Code: code, Message: "Safe explanation", Diagnostics: []pt.TypesDiagnostic{}}
}

func (s appsFixture) Get(_ context.Context, q pt.CatalogAppsGetQuery) (apps.AppsGetResponse, error) {
	if validation.AppGetQuery(q) != nil {
		return apps.AppsGetResponse400{Body: catalogError("INVALID_REQUEST")}, nil
	}
	s.f.lookups++
	if s.f.unavailable {
		return apps.AppsGetResponse503{Body: catalogError("DEPENDENCY_UNAVAILABLE")}, nil
	}
	if q.AppId != "a" && q.AppId != "b" {
		return apps.AppsGetResponse404{Body: catalogError("APP_NOT_FOUND")}, nil
	}
	return apps.AppsGetResponse200{Body: pt.CatalogTypesAppSummary{AppId: q.AppId, Name: "Same name"}}, nil
}

func (s appsFixture) List(_ context.Context, q pt.CatalogAppsListQuery) (apps.AppsListResponse, error) {
	s.f.listQuery = q
	if validation.AppListQuery(q) != nil {
		return apps.AppsListResponse400{Body: catalogError("INVALID_REQUEST")}, nil
	}
	s.f.lookups++
	if s.f.unavailable {
		return apps.AppsListResponse503{Body: catalogError("DEPENDENCY_UNAVAILABLE")}, nil
	}
	if q.Cursor != nil && *q.Cursor != "fixture-next" {
		return apps.AppsListResponse400{Body: catalogError("INVALID_CURSOR")}, nil
	}
	ids := []string{"a", "b"}
	if q.Cursor != nil {
		ids = []string{"b"}
	}
	limit := validation.DefaultCatalogLimit
	if q.Limit != nil {
		limit = *q.Limit
	}
	var next *string
	if len(ids) > limit {
		ids = ids[:limit]
		next = pointer("fixture-next")
	}
	if s.f.badPage {
		ids = []string{"b", "a"}
	}
	items := make([]pt.CatalogTypesAppSummary, 0, len(ids))
	for _, id := range ids {
		items = append(items, pt.CatalogTypesAppSummary{AppId: id, Name: "Same name"})
	}
	return apps.AppsListResponse200{Body: pt.CatalogTypesAppPage{Items: items, NextCursor: next}}, nil
}

func (s componentsFixture) Get(_ context.Context, q pt.CatalogComponentsGetQuery) (components.ComponentsGetResponse, error) {
	if validation.ComponentGetQuery(q) != nil {
		return components.ComponentsGetResponse400{Body: catalogError("INVALID_REQUEST")}, nil
	}
	s.f.lookups++
	if s.f.unavailable {
		return components.ComponentsGetResponse503{Body: catalogError("DEPENDENCY_UNAVAILABLE")}, nil
	}
	if q.AppId != "a" && q.AppId != "b" {
		return components.ComponentsGetResponse404{Body: catalogError("APP_NOT_FOUND")}, nil
	}
	if q.ComponentId != "c-"+q.AppId {
		return components.ComponentsGetResponse404{Body: catalogError("COMPONENT_NOT_FOUND")}, nil
	}
	return components.ComponentsGetResponse200{Body: pt.CatalogTypesComponentSummary{AppId: q.AppId, ComponentId: q.ComponentId, Name: "API"}}, nil
}

func (s componentsFixture) List(_ context.Context, q pt.CatalogComponentsListQuery) (components.ComponentsListResponse, error) {
	s.f.componentQuery = q
	if validation.ComponentListQuery(q) != nil {
		return components.ComponentsListResponse400{Body: catalogError("INVALID_REQUEST")}, nil
	}
	s.f.lookups++
	if s.f.unavailable {
		return components.ComponentsListResponse503{Body: catalogError("DEPENDENCY_UNAVAILABLE")}, nil
	}
	if q.AppId != "a" && q.AppId != "b" {
		return components.ComponentsListResponse404{Body: catalogError("APP_NOT_FOUND")}, nil
	}
	if q.Cursor != nil {
		return components.ComponentsListResponse400{Body: catalogError("INVALID_CURSOR")}, nil
	}
	items := []pt.CatalogTypesComponentSummary{}
	if q.AppId == "a" {
		items = append(items, pt.CatalogTypesComponentSummary{AppId: "a", ComponentId: "c-a", Name: "API"})
	}
	return components.ComponentsListResponse200{Body: pt.CatalogTypesComponentPage{AppId: q.AppId, Items: items}}, nil
}

func catalogClients(t *testing.T, f *catalogFixture) (*client.AppsClient, *client.ComponentsClient, *http.ServeMux) {
	t.Helper()
	mux := http.NewServeMux()
	runtime := httpx.NewRuntime(httpx.WithErrorEncoder(httpx.ErrorEncoderFunc(transport.EncodeError)))
	apps.RegisterAppsHandlersWithRuntime(mux, appsFixture{f}, runtime)
	components.RegisterComponentsHandlersWithRuntime(mux, componentsFixture{f}, runtime)
	hc := &http.Client{Transport: localTransport{handler: mux}}
	a, err := client.NewAppsClient("http://paas.test", hc)
	if err != nil {
		t.Fatal(err)
	}
	c, err := client.NewComponentsClient("http://paas.test", hc)
	if err != nil {
		t.Fatal(err)
	}
	return a, c, mux
}

func TestCatalogTypedStatuses(t *testing.T) {
	f := &catalogFixture{}
	a, c, _ := catalogClients(t, f)
	cases := []struct {
		name        string
		unavailable bool
		call        func() (any, error)
		want        any
	}{
		{"apps-list-200", false, func() (any, error) { return a.List(t.Context(), pt.CatalogAppsListQuery{}) }, client.AppsListResponse200{}},
		{"apps-list-400", false, func() (any, error) { return a.List(t.Context(), pt.CatalogAppsListQuery{Limit: pointer(0)}) }, client.AppsListResponse400{}},
		{"apps-list-503", true, func() (any, error) { return a.List(t.Context(), pt.CatalogAppsListQuery{}) }, client.AppsListResponse503{}},
		{"apps-get-200", false, func() (any, error) { return a.Get(t.Context(), pt.CatalogAppsGetQuery{AppId: "a"}) }, client.AppsGetResponse200{}},
		{"apps-get-400", false, func() (any, error) { return a.Get(t.Context(), pt.CatalogAppsGetQuery{}) }, client.AppsGetResponse400{}},
		{"apps-get-404", false, func() (any, error) { return a.Get(t.Context(), pt.CatalogAppsGetQuery{AppId: "unknown"}) }, client.AppsGetResponse404{}},
		{"apps-get-503", true, func() (any, error) { return a.Get(t.Context(), pt.CatalogAppsGetQuery{AppId: "a"}) }, client.AppsGetResponse503{}},
		{"components-list-200", false, func() (any, error) { return c.List(t.Context(), pt.CatalogComponentsListQuery{AppId: "a"}) }, client.ComponentsListResponse200{}},
		{"components-list-400", false, func() (any, error) { return c.List(t.Context(), pt.CatalogComponentsListQuery{}) }, client.ComponentsListResponse400{}},
		{"components-list-404", false, func() (any, error) { return c.List(t.Context(), pt.CatalogComponentsListQuery{AppId: "unknown"}) }, client.ComponentsListResponse404{}},
		{"components-list-503", true, func() (any, error) { return c.List(t.Context(), pt.CatalogComponentsListQuery{AppId: "a"}) }, client.ComponentsListResponse503{}},
		{"components-get-200", false, func() (any, error) {
			return c.Get(t.Context(), pt.CatalogComponentsGetQuery{AppId: "a", ComponentId: "c-a"})
		}, client.ComponentsGetResponse200{}},
		{"components-get-400", false, func() (any, error) { return c.Get(t.Context(), pt.CatalogComponentsGetQuery{AppId: "a"}) }, client.ComponentsGetResponse400{}},
		{"components-get-404", false, func() (any, error) {
			return c.Get(t.Context(), pt.CatalogComponentsGetQuery{AppId: "a", ComponentId: "c-b"})
		}, client.ComponentsGetResponse404{}},
		{"components-get-503", true, func() (any, error) {
			return c.Get(t.Context(), pt.CatalogComponentsGetQuery{AppId: "a", ComponentId: "c-a"})
		}, client.ComponentsGetResponse503{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f.unavailable = tc.unavailable
			got, err := tc.call()
			if err != nil {
				t.Fatal(err)
			}
			if reflect.TypeOf(got) != reflect.TypeOf(tc.want) {
				t.Fatalf("got %T, want %T", got, tc.want)
			}
			if !strings.HasSuffix(tc.name, "200") {
				data, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				var body pt.TypesErrorResponse
				if err := json.Unmarshal(data, &body); err != nil {
					t.Fatal(err)
				}
				if err := validation.ErrorResponse(body); err != nil {
					t.Fatal(err)
				}
				if tc.unavailable && body.Code != "DEPENDENCY_UNAVAILABLE" {
					t.Fatal("storage failure hidden")
				}
			}
		})
	}
}

func TestCatalogQueryBeforeLookup(t *testing.T) {
	f := &catalogFixture{}
	a, c, mux := catalogClients(t, f)
	for _, path := range []string{"/apps/get", "/apps/get?app_id=a&app_id=b", "/components/list", "/components/get?app_id=a", "/apps/list?limit=oops"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 400 || f.lookups != 0 {
			t.Fatalf("decoder boundary: %s %d %d", path, w.Code, f.lookups)
		}
	}
	if _, err := a.Get(t.Context(), pt.CatalogAppsGetQuery{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.List(t.Context(), pt.CatalogComponentsListQuery{AppId: "a", Limit: pointer(0)}); err != nil {
		t.Fatal(err)
	}
	if f.lookups != 0 {
		t.Fatal("empty/range query reached lookup")
	}
	got, err := c.Get(t.Context(), pt.CatalogComponentsGetQuery{AppId: "unknown", ComponentId: "c-a"})
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := got.(client.ComponentsGetResponse404); !ok || e.Body.Code != "APP_NOT_FOUND" {
		t.Fatalf("unknown app: %#v", got)
	}
	foreign, err := c.Get(t.Context(), pt.CatalogComponentsGetQuery{AppId: "a", ComponentId: "c-b"})
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := foreign.(client.ComponentsGetResponse404); !ok || e.Body.Code != "COMPONENT_NOT_FOUND" {
		t.Fatalf("foreign: %#v", foreign)
	}
}

func TestCatalogSelectionToPreview(t *testing.T) {
	f := &catalogFixture{}
	a, c, mux := catalogClients(t, f)
	first, err := a.List(t.Context(), pt.CatalogAppsListQuery{Limit: pointer(1)})
	if err != nil {
		t.Fatal(err)
	}
	page := first.(client.AppsListResponse200).Body
	if err := validation.AppPage(page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) > 1 || page.NextCursor == nil || f.listQuery.Limit == nil || *f.listQuery.Limit != 1 {
		t.Fatal("limit/next query lost")
	}
	second, err := a.List(t.Context(), pt.CatalogAppsListQuery{Cursor: page.NextCursor, Limit: pointer(100)})
	if err != nil {
		t.Fatal(err)
	}
	if p := second.(client.AppsListResponse200).Body; p.Items[0].AppId != "b" || p.NextCursor != nil || *f.listQuery.Cursor != *page.NextCursor {
		t.Fatal("cursor round-trip")
	}
	selected := page.Items[0]
	app, err := a.Get(t.Context(), pt.CatalogAppsGetQuery{AppId: selected.AppId})
	if err != nil {
		t.Fatal(err)
	}
	if app.(client.AppsGetResponse200).Body.AppId != selected.AppId {
		t.Fatal("wrong app")
	}
	result, err := c.List(t.Context(), pt.CatalogComponentsListQuery{AppId: selected.AppId, Limit: pointer(1)})
	if err != nil {
		t.Fatal(err)
	}
	cp := result.(client.ComponentsListResponse200).Body
	if err := validation.ComponentPage(cp); err != nil {
		t.Fatal(err)
	}
	if cp.AppId != selected.AppId || len(cp.Items) > 1 || f.componentQuery.AppId != selected.AppId {
		t.Fatal("component scope")
	}
	component := cp.Items[0]
	item, err := c.Get(t.Context(), pt.CatalogComponentsGetQuery{AppId: selected.AppId, ComponentId: component.ComponentId})
	if err != nil {
		t.Fatal(err)
	}
	detail := item.(client.ComponentsGetResponse200).Body
	if err := validation.ComponentSummary(detail); err != nil {
		t.Fatal(err)
	}
	if detail.AppId != selected.AppId || detail.ComponentId != component.ComponentId {
		t.Fatal("request binding")
	}
	empty, err := c.List(t.Context(), pt.CatalogComponentsListQuery{AppId: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if p := empty.(client.ComponentsListResponse200).Body; p.Items == nil || len(p.Items) != 0 || p.NextCursor != nil {
		t.Fatal("empty page shape")
	}
	bad, err := c.List(t.Context(), pt.CatalogComponentsListQuery{AppId: "a", Cursor: page.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := bad.(client.ComponentsListResponse400); !ok || e.Body.Code != "INVALID_CURSOR" {
		t.Fatal("fixture cursor scope")
	}
	previewService := &envelopeService{}
	deployments.RegisterDeploymentsHandlers(mux, previewService)
	dc, err := client.NewDeploymentsClient("http://paas.test", &http.Client{Transport: localTransport{handler: mux}})
	if err != nil {
		t.Fatal(err)
	}
	req := request()
	req.AppId = detail.AppId
	req.ComponentId = detail.ComponentId
	preview, err := dc.Preview(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := preview.(client.DeploymentsPreviewResponse200); !ok || previewService.prepared != 1 {
		t.Fatal("selection to preview")
	}
	f.badPage = true
	malformed, err := a.List(t.Context(), pt.CatalogAppsListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if err := validation.AppPage(malformed.(client.AppsListResponse200).Body); err == nil {
		t.Fatal("consumer trusted malformed decoded page")
	}
	// A valid DTO page can still exceed the consumer's requested limit.
	large := cp
	large.Items = append(append([]pt.CatalogTypesComponentSummary{}, cp.Items...), pt.CatalogTypesComponentSummary{AppId: "a", ComponentId: "c-z", Name: "worker"})
	if err := validation.ComponentPage(large); err != nil {
		t.Fatal(err)
	}
	if err := validateCatalogSelection("a", 1, large); err == nil {
		t.Fatal("consumer accepted oversized response")
	}
	// A self-consistent foreign page still requires consumer/request binding.
	if err := validateCatalogSelection("b", 1, cp); err == nil {
		t.Fatal("consumer accepted wrong requested app")
	}
}

func validateCatalogSelection(appID string, limit int, page pt.CatalogTypesComponentPage) error {
	if err := validation.ComponentPage(page); err != nil {
		return err
	}
	if len(page.Items) > limit {
		return fmt.Errorf("response exceeds requested limit")
	}
	if page.AppId != appID {
		return fmt.Errorf("response app_id differs from request")
	}
	return nil
}

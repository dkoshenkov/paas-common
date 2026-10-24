package validation_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

func TestCatalogBoundaries(t *testing.T) {
	for _, limit := range []int{-1, 0, 1, 50, 100, 101} {
		err := validation.AppListQuery(paastypes.CatalogAppsListQuery{Limit: pointer(limit)})
		if (err == nil) != (limit >= 1 && limit <= 100) {
			t.Fatalf("limit %d: %v", limit, err)
		}
	}
	if err := validation.AppListQuery(paastypes.CatalogAppsListQuery{}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"", false}, {strings.Repeat("я", 512), true}, {strings.Repeat("я", 512) + "a", false}, {"a\nb", false}, {string([]byte{255}), false}, {"position+%&?", true},
	} {
		if err := validation.AppListQuery(paastypes.CatalogAppsListQuery{Cursor: &tc.value}); (err == nil) != tc.valid {
			t.Fatalf("cursor boundary: %v", err)
		}
	}
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"", false}, {strings.Repeat("я", 64), true}, {strings.Repeat("я", 64) + "a", false}, {"a\x1bb", false}, {"a\u0085b", false}, {string([]byte{255}), false},
	} {
		err := validation.AppSummary(paastypes.CatalogTypesAppSummary{AppId: "app", Name: tc.value})
		if (err == nil) != tc.valid {
			t.Fatalf("name boundary: %v", err)
		}
	}
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"", false}, {"A-b_c.9", true}, {strings.Repeat("a", 63), true}, {strings.Repeat("a", 64), false}, {"-a", false}, {"a_", false}, {"а", false}, {"a/b", false},
	} {
		err := validation.ComponentSummary(paastypes.CatalogTypesComponentSummary{AppId: tc.value, ComponentId: tc.value, Name: "API"})
		if (err == nil) != tc.valid {
			t.Fatalf("ID boundary: %v", err)
		}
	}
	for _, check := range []func() error{
		func() error { return validation.AppGetQuery(paastypes.CatalogAppsGetQuery{}) },
		func() error { return validation.ComponentGetQuery(paastypes.CatalogComponentsGetQuery{AppId: "app"}) },
		func() error {
			return validation.ComponentGetQuery(paastypes.CatalogComponentsGetQuery{ComponentId: "c"})
		},
		func() error { return validation.ComponentListQuery(paastypes.CatalogComponentsListQuery{}) },
		func() error {
			return validation.ComponentListQuery(paastypes.CatalogComponentsListQuery{AppId: "app", Limit: pointer(0)})
		},
	} {
		if check() == nil {
			t.Fatal("invalid query accepted")
		}
	}
	// Opaque request IDs do not acquire the response label grammar.
	if err := validation.ComponentGetQuery(paastypes.CatalogComponentsGetQuery{AppId: "opaque/id", ComponentId: "opaque/id"}); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogPagesAndNonMutation(t *testing.T) {
	a := paastypes.CatalogTypesAppSummary{AppId: "a", Name: "duplicate name"}
	b := paastypes.CatalogTypesAppSummary{AppId: "b", Name: "duplicate name"}
	for _, tc := range []struct {
		name  string
		p     paastypes.CatalogTypesAppPage
		valid bool
	}{
		{"empty", paastypes.CatalogTypesAppPage{Items: []paastypes.CatalogTypesAppSummary{}}, true},
		{"nil", paastypes.CatalogTypesAppPage{}, false},
		{"sorted", paastypes.CatalogTypesAppPage{Items: []paastypes.CatalogTypesAppSummary{a, b}, NextCursor: pointer("next")}, true},
		{"unsorted", paastypes.CatalogTypesAppPage{Items: []paastypes.CatalogTypesAppSummary{b, a}}, false},
		{"duplicate", paastypes.CatalogTypesAppPage{Items: []paastypes.CatalogTypesAppSummary{a, a}}, false},
		{"empty-next", paastypes.CatalogTypesAppPage{Items: []paastypes.CatalogTypesAppSummary{}, NextCursor: pointer("next")}, false},
		{"bad-next", paastypes.CatalogTypesAppPage{Items: []paastypes.CatalogTypesAppSummary{a}, NextCursor: pointer("")}, false},
		{"too-many", paastypes.CatalogTypesAppPage{Items: make([]paastypes.CatalogTypesAppSummary, 101)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Copy slices directly: generated JSON intentionally rejects nil collections.
			before := append([]paastypes.CatalogTypesAppSummary(nil), tc.p.Items...)
			if err := validation.AppPage(tc.p); (err == nil) != tc.valid {
				t.Fatalf("page: %v", err)
			}
			for i := range before {
				if before[i] != tc.p.Items[i] {
					t.Fatal("input changed")
				}
			}
		})
	}
	component := paastypes.CatalogTypesComponentSummary{AppId: "app", ComponentId: "a", Name: "API"}
	for _, tc := range []struct {
		name  string
		edit  func(*paastypes.CatalogTypesComponentPage)
		valid bool
	}{
		{"valid", func(*paastypes.CatalogTypesComponentPage) {}, true},
		{"empty", func(p *paastypes.CatalogTypesComponentPage) { p.Items = []paastypes.CatalogTypesComponentSummary{} }, true},
		{"foreign", func(p *paastypes.CatalogTypesComponentPage) { p.Items[0].AppId = "other" }, false},
		{"bad-parent", func(p *paastypes.CatalogTypesComponentPage) { p.AppId = "" }, false},
		{"nil", func(p *paastypes.CatalogTypesComponentPage) { p.Items = nil }, false},
		{"duplicate", func(p *paastypes.CatalogTypesComponentPage) { p.Items = append(p.Items, p.Items[0]) }, false},
		{"unsorted", func(p *paastypes.CatalogTypesComponentPage) {
			p.Items[0].ComponentId = "z"
			p.Items = append(p.Items, component)
		}, false},
		{"bad-name", func(p *paastypes.CatalogTypesComponentPage) { p.Items[0].Name = "" }, false},
		{"empty-next", func(p *paastypes.CatalogTypesComponentPage) {
			p.Items = []paastypes.CatalogTypesComponentSummary{}
			p.NextCursor = pointer("next")
		}, false},
		{"bad-next", func(p *paastypes.CatalogTypesComponentPage) { p.NextCursor = pointer("\n") }, false},
		{"too-many", func(p *paastypes.CatalogTypesComponentPage) {
			p.Items = make([]paastypes.CatalogTypesComponentSummary, 101)
		}, false},
	} {
		t.Run("component-"+tc.name, func(t *testing.T) {
			p := paastypes.CatalogTypesComponentPage{AppId: "app", Items: []paastypes.CatalogTypesComponentSummary{component}}
			tc.edit(&p)
			before := append([]paastypes.CatalogTypesComponentSummary(nil), p.Items...)
			if err := validation.ComponentPage(p); (err == nil) != tc.valid {
				t.Fatalf("component page: %v", err)
			}
			for i := range before {
				if before[i] != p.Items[i] {
					t.Fatal("input changed")
				}
			}
		})
	}
}

func TestCatalogJSONShape(t *testing.T) {
	for _, raw := range []string{`{}`, `{"items":null}`, `{"items":[],"next_cursor":null}`} {
		var page paastypes.CatalogTypesAppPage
		if err := json.Unmarshal([]byte(raw), &page); err == nil {
			t.Fatal("missing/null accepted")
		}
	}
	data, err := json.Marshal(paastypes.CatalogTypesComponentPage{AppId: "app", Items: []paastypes.CatalogTypesComponentSummary{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "next_cursor") || !strings.Contains(string(data), `"items":[]`) {
		t.Fatalf("omission: %s", data)
	}
}

func TestCatalogMaximumPage(t *testing.T) {
	p := paastypes.CatalogTypesAppPage{Items: []paastypes.CatalogTypesAppSummary{}}
	for i := 0; i < 100; i++ {
		p.Items = append(p.Items, paastypes.CatalogTypesAppSummary{AppId: fmt.Sprintf("app-%03d", i), Name: "API"})
	}
	if err := validation.AppPage(p); err != nil {
		t.Fatal(err)
	}
}

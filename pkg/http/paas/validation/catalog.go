package validation

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

const (
	DefaultCatalogLimit   = 50
	MaxCatalogLimit       = 100
	MaxCatalogNameBytes   = 128
	MaxCatalogCursorBytes = 1024
)

// AppListQuery checks optional pagination values, without interpreting the cursor.
func AppListQuery(q paastypes.CatalogAppsListQuery) error { return catalogQuery(q.Limit, q.Cursor) }

// AppGetQuery checks presence of an opaque ID, not lookup or its server format.
func AppGetQuery(q paastypes.CatalogAppsGetQuery) error {
	if q.AppId == "" {
		return fmt.Errorf("app_id: required nonempty ID")
	}
	return nil
}

// ComponentListQuery checks presence and pagination, not application existence.
func ComponentListQuery(q paastypes.CatalogComponentsListQuery) error {
	if q.AppId == "" {
		return fmt.Errorf("app_id: required nonempty ID")
	}
	return catalogQuery(q.Limit, q.Cursor)
}

// ComponentGetQuery checks presence, not component ownership or existence.
func ComponentGetQuery(q paastypes.CatalogComponentsGetQuery) error {
	if q.AppId == "" || q.ComponentId == "" {
		return fmt.Errorf("app_id and component_id: required nonempty IDs")
	}
	return nil
}

func catalogQuery(limit *int, cursor *string) error {
	if limit != nil && (*limit < 1 || *limit > MaxCatalogLimit) {
		return fmt.Errorf("limit: require 1 through 100")
	}
	return catalogCursor(cursor)
}

func catalogCursor(cursor *string) error {
	if cursor != nil && !catalogText(*cursor, MaxCatalogCursorBytes) {
		return fmt.Errorf("cursor: require nonempty UTF-8 up to 1024 bytes without controls")
	}
	return nil
}

func catalogText(value string, maxBytes int) bool {
	return value != "" && len(value) <= maxBytes && utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func catalogID(value string) bool {
	if value == "" || len(value) > 63 {
		return false
	}
	alnum := func(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' }
	if !alnum(value[0]) || !alnum(value[len(value)-1]) {
		return false
	}
	for i := range len(value) {
		if c := value[i]; !alnum(c) && c != '-' && c != '_' && c != '.' {
			return false
		}
	}
	return true
}

// AppSummary checks a catalog snapshot, not readiness, uniqueness across pages or persistence.
func AppSummary(app paastypes.CatalogTypesAppSummary) error {
	if !catalogID(app.AppId) {
		return fmt.Errorf("app_id: require a label-compatible ID")
	}
	if !catalogText(app.Name, MaxCatalogNameBytes) {
		return fmt.Errorf("name: require nonempty UTF-8 up to 128 bytes without controls")
	}
	return nil
}

// ComponentSummary checks shape. The consumer must also compare IDs with its request.
func ComponentSummary(component paastypes.CatalogTypesComponentSummary) error {
	if !catalogID(component.ComponentId) {
		return fmt.Errorf("component_id: require a label-compatible ID")
	}
	return AppSummary(paastypes.CatalogTypesAppSummary{AppId: component.AppId, Name: component.Name})
}

func catalogPage(count int, next *string) error {
	if count > MaxCatalogLimit {
		return fmt.Errorf("items: maximum 100 entries")
	}
	if count == 0 && next != nil {
		return fmt.Errorf("next_cursor: forbidden for an empty page")
	}
	return catalogCursor(next)
}

// AppPage checks one page without mutation, lookup or cursor decoding.
// The consumer must additionally enforce its requested limit.
func AppPage(page paastypes.CatalogTypesAppPage) error {
	if page.Items == nil {
		return fmt.Errorf("items: require an array, not null")
	}
	if err := catalogPage(len(page.Items), page.NextCursor); err != nil {
		return err
	}
	for i, app := range page.Items {
		if err := AppSummary(app); err != nil {
			return fmt.Errorf("items[%d]: %w", i, err)
		}
		if i > 0 && page.Items[i-1].AppId >= app.AppId {
			return fmt.Errorf("items[%d]: IDs must be unique and sorted", i)
		}
	}
	return nil
}

// ComponentPage checks one page and its internal app binding. The consumer must
// also compare app_id and count with the request. It does not check cursor scope.
func ComponentPage(page paastypes.CatalogTypesComponentPage) error {
	if !catalogID(page.AppId) {
		return fmt.Errorf("app_id: require a label-compatible ID")
	}
	if page.Items == nil {
		return fmt.Errorf("items: require an array, not null")
	}
	if err := catalogPage(len(page.Items), page.NextCursor); err != nil {
		return err
	}
	for i, component := range page.Items {
		if err := ComponentSummary(component); err != nil {
			return fmt.Errorf("items[%d]: %w", i, err)
		}
		if component.AppId != page.AppId {
			return fmt.Errorf("items[%d].app_id: differs from page app_id", i)
		}
		if i > 0 && page.Items[i-1].ComponentId >= component.ComponentId {
			return fmt.Errorf("items[%d]: IDs must be unique and sorted", i)
		}
	}
	return nil
}

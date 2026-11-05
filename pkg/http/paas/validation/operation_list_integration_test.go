package validation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/dkoshenkov/packages-go/middlewarex/httpx"

	"github.com/dkoshenkov/paas-common/pkg/http/paas/client"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/operations"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/transport"
	pt "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

// Only an SDK fixture: tokens refer to fixed in-memory rows, not database cursors.
type operationListService struct {
	calls, reads, getCalls int
	unavailable, gone      bool
	seen                   pt.OperationsOperationsListQuery
	getID                  string
	override               *pt.TypesOperationPage
}

const operationFixtureCursor = "fixture-cursor+%&?/RUNNING/id-1"

func fixtureOperationCursor(state *string, id string) string {
	scope := "all"
	if state != nil {
		scope = *state
	}
	return "fixture-cursor+%&?/" + scope + "/" + id
}

func (s *operationListService) List(_ context.Context, q pt.OperationsOperationsListQuery) (operations.OperationsListResponse, error) {
	s.calls++
	s.seen = q
	if validation.OperationListQuery(q) != nil {
		return operations.OperationsListResponse400{Body: catalogError("INVALID_REQUEST")}, nil
	}
	s.reads++
	if s.unavailable {
		return operations.OperationsListResponse503{Body: catalogError("DEPENDENCY_UNAVAILABLE")}, nil
	}
	if s.override != nil {
		return operations.OperationsListResponse200{Body: *s.override}, nil
	}
	items := []pt.TypesOperation{}
	for i, state := range []string{"ACCEPTED", "RUNNING", "RUNNING", "SUCCEEDED", "FAILED", "RECOVERY_REQUIRED"} {
		if q.State == nil || *q.State == state {
			items = append(items, listedOperation(fmt.Sprintf("id-%d", i), state))
		}
	}
	if q.Cursor != nil {
		start := -1
		for i, op := range items {
			if *q.Cursor == fixtureOperationCursor(q.State, op.OperationId) {
				start = i + 1
				break
			}
		}
		if start < 0 {
			return operations.OperationsListResponse400{Body: catalogError("INVALID_CURSOR")}, nil
		}
		items = items[start:]
	}
	limit := validation.DefaultCatalogLimit
	if q.Limit != nil {
		limit = *q.Limit
	}
	var next *string
	if len(items) > limit {
		items = items[:limit]
		next = pointer(fixtureOperationCursor(q.State, items[len(items)-1].OperationId))
	}
	return operations.OperationsListResponse200{Body: pt.TypesOperationPage{Items: items, NextCursor: next}}, nil
}

func (s *operationListService) Get(_ context.Context, q pt.OperationsOperationsGetQuery) (operations.OperationsGetResponse, error) {
	s.getCalls++
	s.getID = q.OperationId
	if s.gone {
		return operations.OperationsGetResponse404{Body: catalogError("OPERATION_NOT_FOUND")}, nil
	}
	return operations.OperationsGetResponse200{Body: listedOperation(q.OperationId, "SUCCEEDED")}, nil
}

func operationListClient(t *testing.T, s *operationListService) (*client.OperationsClient, *http.ServeMux) {
	t.Helper()
	mux := http.NewServeMux()
	runtime := httpx.NewRuntime(httpx.WithErrorEncoder(httpx.ErrorEncoderFunc(transport.EncodeError)))
	operations.RegisterOperationsHandlersWithRuntime(mux, s, runtime)
	c, err := client.NewOperationsClient("http://paas.test", &http.Client{Transport: localTransport{handler: mux}})
	if err != nil {
		t.Fatal(err)
	}
	return c, mux
}

func checkedOperationList(q pt.OperationsOperationsListQuery, page pt.TypesOperationPage) error {
	if err := validation.OperationPage(page); err != nil {
		return err
	}
	limit := validation.DefaultCatalogLimit
	if q.Limit != nil {
		limit = *q.Limit
	}
	if len(page.Items) > limit {
		return fmt.Errorf("response exceeds requested limit")
	}
	for _, op := range page.Items {
		if q.State != nil && op.State != *q.State {
			return fmt.Errorf("response differs from requested state")
		}
	}
	return nil
}

func TestOperationListWireAndSelection(t *testing.T) {
	s := &operationListService{}
	c, _ := operationListClient(t, s)
	all, err := c.List(t.Context(), pt.OperationsOperationsListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	page := all.(client.OperationsListResponse200).Body
	if err := checkedOperationList(pt.OperationsOperationsListQuery{}, page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 6 || s.seen.State != nil || s.seen.Limit != nil || s.seen.Cursor != nil || page.NextCursor != nil {
		t.Fatal("absent query/default fixture")
	}
	for _, state := range []string{"ACCEPTED", "RUNNING", "SUCCEEDED", "FAILED", "RECOVERY_REQUIRED"} {
		q := pt.OperationsOperationsListQuery{State: &state, Limit: pointer(100)}
		r, err := c.List(t.Context(), q)
		if err != nil {
			t.Fatal(err)
		}
		p := r.(client.OperationsListResponse200).Body
		if err := checkedOperationList(q, p); err != nil {
			t.Fatal(err)
		}
		if len(p.Items) == 0 || *s.seen.State != state || *s.seen.Limit != 100 {
			t.Fatal("state/limit query lost")
		}
	}
	q := pt.OperationsOperationsListQuery{State: pointer("RUNNING"), Limit: pointer(1)}
	first, err := c.List(t.Context(), q)
	if err != nil {
		t.Fatal(err)
	}
	p := first.(client.OperationsListResponse200).Body
	if err := checkedOperationList(q, p); err != nil {
		t.Fatal(err)
	}
	if len(p.Items) != 1 || p.NextCursor == nil || *p.NextCursor != operationFixtureCursor {
		t.Fatal("first page")
	}
	selected := p.Items[0]
	q.Cursor = p.NextCursor
	q.Limit = pointer(100)
	second, err := c.List(t.Context(), q)
	if err != nil {
		t.Fatal(err)
	}
	p2 := second.(client.OperationsListResponse200).Body
	if err := checkedOperationList(q, p2); err != nil {
		t.Fatal(err)
	}
	if len(p2.Items) != 1 || p2.Items[0].OperationId <= selected.OperationId || p2.NextCursor != nil || *s.seen.Cursor != operationFixtureCursor {
		t.Fatal("cursor round-trip or changed limit")
	}
	detail, err := c.Get(t.Context(), pt.OperationsOperationsGetQuery{OperationId: selected.OperationId})
	if err != nil {
		t.Fatal(err)
	}
	op := detail.(client.OperationsGetResponse200).Body
	if err := validation.Operation(op); err != nil {
		t.Fatal(err)
	}
	if op.OperationId != selected.OperationId || s.getID != selected.OperationId || op.State != "SUCCEEDED" || s.getCalls != 1 {
		t.Fatal("list to get; changed state allowed")
	}
	s.gone = true
	gone, err := c.Get(t.Context(), pt.OperationsOperationsGetQuery{OperationId: selected.OperationId})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := gone.(client.OperationsGetResponse404); !ok {
		t.Fatal("get after fixture cleanup")
	}
}

func TestOperationListBoundariesAndErrors(t *testing.T) {
	s := &operationListService{}
	c, mux := operationListClient(t, s)
	for _, path := range []string{
		"/operations/list?state=RUNNING&state=FAILED", "/operations/list?limit=1&limit=2", "/operations/list?cursor=a&cursor=b", "/operations/list?limit=nope",
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != 400 || s.calls != 0 || s.reads != 0 {
			t.Fatalf("decoder before service: %s %d", path, w.Code)
		}
		var body pt.TypesErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if err := validation.ErrorResponse(body); err != nil || body.Code != "INVALID_REQUEST" {
			t.Fatal("transport body")
		}
	}
	for _, q := range []pt.OperationsOperationsListQuery{
		{State: pointer("")}, {State: pointer("running")}, {State: pointer("UNKNOWN")}, {Limit: pointer(0)}, {Limit: pointer(101)}, {Cursor: pointer("")},
	} {
		r, err := c.List(t.Context(), q)
		if err != nil {
			t.Fatal(err)
		}
		e, ok := r.(client.OperationsListResponse400)
		if !ok || e.Body.Code != "INVALID_REQUEST" || s.reads != 0 {
			t.Fatalf("service validation before reads: %#v %d", r, s.reads)
		}
	}
	// Fixed fixture token is scoped to RUNNING; other endpoints' cursors are invalid.
	for _, q := range []pt.OperationsOperationsListQuery{
		{State: pointer("FAILED"), Cursor: pointer(operationFixtureCursor)},
		{Cursor: pointer(operationFixtureCursor)},
		{State: pointer("RUNNING"), Cursor: pointer("catalog-cursor")},
	} {
		r, err := c.List(t.Context(), q)
		if err != nil {
			t.Fatal(err)
		}
		if e, ok := r.(client.OperationsListResponse400); !ok || e.Body.Code != "INVALID_CURSOR" {
			t.Fatalf("cursor scope: %#v", r)
		}
	}
	s.unavailable = true
	r, err := c.List(t.Context(), pt.OperationsOperationsListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	e, ok := r.(client.OperationsListResponse503)
	if !ok || e.Body.Code != "DEPENDENCY_UNAVAILABLE" {
		t.Fatal("storage failure hidden as empty page")
	}
	if err := validation.ErrorResponse(e.Body); err != nil {
		t.Fatal(err)
	}
	s.unavailable = false
	s.override = &pt.TypesOperationPage{Items: []pt.TypesOperation{}}
	empty, err := c.List(t.Context(), pt.OperationsOperationsListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	p := empty.(client.OperationsListResponse200).Body
	if p.Items == nil || len(p.Items) != 0 || p.NextCursor != nil {
		t.Fatal("empty list shape")
	}
}

func TestOperationListConsumerChecksDecodedPage(t *testing.T) {
	s := &operationListService{}
	c, _ := operationListClient(t, s)
	q := pt.OperationsOperationsListQuery{State: pointer("RUNNING"), Limit: pointer(1)}
	for _, tc := range []struct {
		name     string
		items    []pt.TypesOperation
		validDTO bool
	}{
		{"unsorted", []pt.TypesOperation{listedOperation("b", "RUNNING"), listedOperation("a", "RUNNING")}, false},
		{"invalid-operation", []pt.TypesOperation{listedOperation("a", "UNKNOWN")}, false},
		{"wrong-filter", []pt.TypesOperation{listedOperation("a", "SUCCEEDED")}, true},
		{"too-many-for-request", []pt.TypesOperation{listedOperation("a", "RUNNING"), listedOperation("b", "RUNNING")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s.override = &pt.TypesOperationPage{Items: tc.items}
			r, err := c.List(t.Context(), q)
			if err != nil {
				t.Fatal(err)
			}
			p := r.(client.OperationsListResponse200).Body
			if (validation.OperationPage(p) == nil) != tc.validDTO {
				t.Fatal("unexpected snapshot validation")
			}
			if err := checkedOperationList(q, p); err == nil {
				t.Fatal("consumer trusted decoded response")
			}
			if !reflect.DeepEqual(p.Items, tc.items) {
				t.Fatal("consumer changed response")
			}
		})
	}
}

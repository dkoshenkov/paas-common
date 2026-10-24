package validation_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	pt "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

func listedOperation(id, state string) pt.TypesOperation {
	op := operation()
	op.OperationId = id
	op.State = state
	if state == "SUCCEEDED" || state == "FAILED" {
		op.FinishedAt = pointer(op.UpdatedAt)
	}
	if state == "FAILED" || state == "RECOVERY_REQUIRED" {
		op.Diagnostics = []pt.TypesDiagnostic{{Severity: "ERROR", Code: "EXPLANATION", Message: "Safe explanation"}}
	}
	return op
}

func TestOperationListQueryBounds(t *testing.T) {
	if err := validation.OperationListQuery(pt.OperationsOperationsListQuery{}); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"ACCEPTED", "RUNNING", "SUCCEEDED", "FAILED", "RECOVERY_REQUIRED", "", "running", "UNKNOWN"} {
		err := validation.OperationListQuery(pt.OperationsOperationsListQuery{State: &state})
		valid := state != "" && state != "running" && state != "UNKNOWN"
		if (err == nil) != valid {
			t.Fatalf("state filter result: %v", err)
		}
	}
	for _, limit := range []int{-1, 0, 1, 50, 100, 101} {
		err := validation.OperationListQuery(pt.OperationsOperationsListQuery{Limit: &limit})
		if (err == nil) != (limit >= 1 && limit <= 100) {
			t.Fatalf("limit %d: %v", limit, err)
		}
	}
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"", false}, {"position+%&?", true}, {strings.Repeat("я", 512), true}, {strings.Repeat("я", 512) + "a", false}, {"x\ny", false}, {string([]byte{255}), false},
	} {
		err := validation.OperationListQuery(pt.OperationsOperationsListQuery{Cursor: &tc.value})
		if (err == nil) != tc.valid {
			t.Fatalf("cursor boundary: %v", err)
		}
	}
	q := pt.OperationsOperationsListQuery{State: pointer("RUNNING"), Limit: pointer(1), Cursor: pointer("cursor")}
	before, err := json.Marshal(q)
	if err != nil {
		t.Fatal(err)
	}
	if err := validation.OperationListQuery(q); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(q)
	if err != nil || string(before) != string(after) {
		t.Fatal("query changed")
	}
}

func TestOperationPageSnapshots(t *testing.T) {
	for _, state := range []string{"ACCEPTED", "RUNNING", "SUCCEEDED", "FAILED", "RECOVERY_REQUIRED"} {
		p := pt.TypesOperationPage{Items: []pt.TypesOperation{listedOperation("opaque/id", state)}}
		if err := validation.OperationPage(p); err != nil {
			t.Fatalf("%s: %v", state, err)
		}
	}
	cases := []struct {
		name  string
		edit  func(*pt.TypesOperationPage)
		valid bool
	}{
		{"valid", func(*pt.TypesOperationPage) {}, true},
		{"empty", func(p *pt.TypesOperationPage) { p.Items = []pt.TypesOperation{} }, true},
		{"nil", func(p *pt.TypesOperationPage) { p.Items = nil }, false},
		{"duplicate", func(p *pt.TypesOperationPage) { p.Items = append(p.Items, p.Items[0]) }, false},
		{"unsorted", func(p *pt.TypesOperationPage) { p.Items = append(p.Items, listedOperation("a", "RUNNING")) }, false},
		{"invalid-state", func(p *pt.TypesOperationPage) { p.Items[0].State = "UNKNOWN" }, false},
		{"invalid-finished", func(p *pt.TypesOperationPage) { p.Items[0].FinishedAt = pointer(p.Items[0].UpdatedAt) }, false},
		{"nil-diagnostics", func(p *pt.TypesOperationPage) { p.Items[0].Diagnostics = nil }, false},
		{"empty-next", func(p *pt.TypesOperationPage) { p.NextCursor = pointer("") }, false},
		{"long-next", func(p *pt.TypesOperationPage) { p.NextCursor = pointer(strings.Repeat("a", 1025)) }, false},
		{"empty-page-next", func(p *pt.TypesOperationPage) { p.Items = []pt.TypesOperation{}; p.NextCursor = pointer("cursor") }, false},
		{"too-many", func(p *pt.TypesOperationPage) { p.Items = make([]pt.TypesOperation, 101) }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := pt.TypesOperationPage{Items: []pt.TypesOperation{listedOperation("b", "RUNNING")}}
			tc.edit(&p)
			if err := validation.OperationPage(p); (err == nil) != tc.valid {
				t.Fatalf("page result: %v", err)
			}
		})
	}
	p := pt.TypesOperationPage{Items: []pt.TypesOperation{}}
	for i := 0; i < 100; i++ {
		p.Items = append(p.Items, listedOperation(fmt.Sprintf("id-%03d", i), "RUNNING"))
	}
	if err := validation.OperationPage(p); err != nil {
		t.Fatal(err)
	}
}

func TestOperationPageJSONAndNonMutation(t *testing.T) {
	for _, raw := range []string{`{}`, `{"items":null}`, `{"items":[],"next_cursor":null}`} {
		var p pt.TypesOperationPage
		if err := json.Unmarshal([]byte(raw), &p); err == nil {
			t.Fatal("missing/null page accepted")
		}
	}
	data, err := json.Marshal(pt.TypesOperationPage{Items: []pt.TypesOperation{}})
	if err != nil || string(data) != `{"items":[]}` {
		t.Fatalf("empty page omission: %s %v", data, err)
	}
	for _, ids := range [][]string{{"a", "b"}, {"b", "a"}, {"a", "a"}} {
		p := pt.TypesOperationPage{Items: []pt.TypesOperation{}, NextCursor: pointer("next")}
		for _, id := range ids {
			p.Items = append(p.Items, listedOperation(id, "RUNNING"))
		}
		before, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		err = validation.OperationPage(p)
		if (err == nil) != (ids[0] < ids[1]) {
			t.Fatalf("validation result: %v", err)
		}
		after, err := json.Marshal(p)
		if err != nil || string(after) != string(before) {
			t.Fatal("page changed")
		}
	}
	p := pt.TypesOperationPage{Items: []pt.TypesOperation{listedOperation("sensitive-value", "UNKNOWN")}}
	if err := validation.OperationPage(p); err == nil || strings.Contains(err.Error(), "sensitive-value") || strings.Contains(err.Error(), "UNKNOWN") {
		t.Fatal("unsafe or absent validation error")
	}
}

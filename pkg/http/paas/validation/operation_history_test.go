package validation

import (
	"encoding/json"
	"strings"
	"testing"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

func historyFixture() t.TypesOperation {
	return t.TypesOperation{OperationId: "op", AppId: "app", ComponentId: "component", State: "ACCEPTED", CreatedAt: "2026-11-05T00:00:00Z", UpdatedAt: "2026-11-05T00:00:00Z", Diagnostics: []t.TypesDiagnostic{}}
}

func TestOperationHistoryBounds(tst *testing.T) {
	for _, tc := range []struct {
		name  string
		max   int
		field func(*t.TypesOperation) *string
	}{
		{"id", 96, func(o *t.TypesOperation) *string { return &o.OperationId }},
		{"app", 256, func(o *t.TypesOperation) *string { return &o.AppId }},
		{"component", 256, func(o *t.TypesOperation) *string { return &o.ComponentId }},
		{"stage", 64, func(o *t.TypesOperation) *string { o.Stage = new(string); return o.Stage }},
		{"code", 64, func(o *t.TypesOperation) *string { return &o.Diagnostics[0].Code }},
		{"message", 1024, func(o *t.TypesOperation) *string { return &o.Diagnostics[0].Message }},
		{"file", 256, func(o *t.TypesOperation) *string { o.Diagnostics[0].File = new(string); return o.Diagnostics[0].File }},
		{"path", 512, func(o *t.TypesOperation) *string {
			o.Diagnostics[0].FieldPath = new(string)
			return o.Diagnostics[0].FieldPath
		}},
	} {
		tst.Run(tc.name, func(tst *testing.T) {
			for _, value := range []string{strings.Repeat("a", tc.max), strings.Repeat("é", tc.max/2), strings.Repeat("a", tc.max+1), "bad\x00", "bad\x7f", "bad\u0085", string([]byte{0xff})} {
				o := historyFixture()
				o.Diagnostics = []t.TypesDiagnostic{{Code: "NOTICE", Message: "Safe message", Severity: "WARN"}}
				if tc.name == "path" {
					if strings.HasPrefix(value, "é") {
						value = "/a" + value[2:]
					} else if strings.HasPrefix(value, "a") {
						value = "/" + value[1:]
					} else {
						value = "/" + value
					}
				}
				*tc.field(&o) = value
				want := len(value) <= tc.max && !strings.ContainsAny(value, "\x00\x7f\u0085") && !strings.Contains(value, string([]byte{0xff}))
				if (OperationHistory(o) == nil) != want {
					tst.Fatalf("length %d: valid=%v", len(value), want)
				}
			}
		})
	}
	for _, n := range []int{32, 33} {
		o := historyFixture()
		for range n {
			o.Diagnostics = append(o.Diagnostics, t.TypesDiagnostic{Code: "NOTICE", Message: "safe", Severity: "WARN"})
		}
		if (OperationHistory(o) == nil) != (n == 32) {
			tst.Fatal("diagnostic count", n)
		}
	}
	for _, n := range []int{40, 41} {
		o := historyFixture()
		o.CreatedAt = "2026-11-05T00:00:00." + strings.Repeat("1", n-21) + "Z"
		if (OperationHistory(o) == nil) != (n == 40) {
			tst.Fatal("timestamp", n)
		}
	}
}

func TestOperationHistoryWireAndLegacy(tst *testing.T) {
	o := historyFixture()
	for range 32 {
		o.Diagnostics = append(o.Diagnostics, t.TypesDiagnostic{Code: "NOTICE", Message: strings.Repeat("<", 1024), Severity: "WARN"})
	}
	if Operation(o) != nil || OperationHistory(o) == nil {
		tst.Fatal("escaping bound or legacy behavior")
	}
	o = historyFixture()
	o.OperationId = strings.Repeat("a", 97)
	if Operation(o) != nil || OperationHistory(o) == nil {
		tst.Fatal("legacy ID validator changed")
	}
	for _, state := range []string{"ACCEPTED", "RUNNING", "SUCCEEDED", "FAILED", "RECOVERY_REQUIRED"} {
		o = historyFixture()
		o.State = state
		if state == "FAILED" || state == "RECOVERY_REQUIRED" {
			o.Diagnostics = []t.TypesDiagnostic{{Code: "SAFE_ERROR", Message: "safe", Severity: "ERROR"}}
		}
		if state == "SUCCEEDED" || state == "FAILED" {
			o.FinishedAt = &o.UpdatedAt
		}
		before, _ := json.Marshal(o)
		if err := OperationHistory(o); err != nil {
			tst.Fatal(state, err)
		}
		after, _ := json.Marshal(o)
		if string(before) != string(after) {
			tst.Fatal("mutation")
		}
	}
	o = historyFixture()
	o.Diagnostics = nil
	if OperationHistory(o) == nil {
		tst.Fatal("null diagnostics")
	}
	o = historyFixture()
	o.State = "FAILED"
	if OperationHistory(o) == nil {
		tst.Fatal("invalid terminal shape")
	}
	if historyJSON(strings.Repeat("x", MaxOperationHistoryBytes-2), MaxOperationHistoryBytes) != nil || historyJSON(strings.Repeat("x", MaxOperationHistoryBytes-1), MaxOperationHistoryBytes) == nil {
		tst.Fatal("wire boundary")
	}
	if historyJSON(strings.Repeat("x", MaxOperationHistoryPageBytes-2), MaxOperationHistoryPageBytes) != nil || historyJSON(strings.Repeat("x", MaxOperationHistoryPageBytes-1), MaxOperationHistoryPageBytes) == nil {
		tst.Fatal("page wire boundary")
	}
}

func TestOperationHistoryPage(tst *testing.T) {
	if OperationHistoryPage(t.TypesOperationPage{Items: []t.TypesOperation{}}) != nil {
		tst.Fatal("empty page")
	}
	if OperationHistoryPage(t.TypesOperationPage{}) == nil {
		tst.Fatal("null page")
	}
	op := historyFixture()
	if OperationHistoryPage(t.TypesOperationPage{Items: []t.TypesOperation{op, op}}) == nil {
		tst.Fatal("duplicate page")
	}
	op.Diagnostics = []t.TypesDiagnostic{{Code: "NOTICE", Message: "safe", Severity: "INFO", FieldPath: new(string)}}
	if err := OperationHistory(op); err != nil {
		tst.Fatal("root JSON pointer", err)
	}
	op.OperationId = ""
	if OperationHistory(op) == nil {
		tst.Fatal("empty identifier")
	}
}

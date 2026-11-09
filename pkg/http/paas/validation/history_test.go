package validation_test

import (
	"encoding/json"
	"strings"
	"testing"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	v "github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

func TestHistoryOperationBounds(tt *testing.T) {
	for _, tc := range []struct {
		field string
		max   int
		set   func(*t.TypesOperation, string)
	}{
		{"operation_id", 96, func(o *t.TypesOperation, s string) { o.OperationId = s }},
		{"app_id", 256, func(o *t.TypesOperation, s string) { o.AppId = s }},
		{"component_id", 256, func(o *t.TypesOperation, s string) { o.ComponentId = s }},
		{"stage", 64, func(o *t.TypesOperation, s string) { o.Stage = &s }},
		{"code", 64, func(o *t.TypesOperation, s string) { o.Diagnostics[0].Code = s }},
		{"message", 1024, func(o *t.TypesOperation, s string) { o.Diagnostics[0].Message = s }},
		{"file", 512, func(o *t.TypesOperation, s string) { o.Diagnostics[0].File = &s }},
		{"field_path", 512, func(o *t.TypesOperation, s string) { o.Diagnostics[0].FieldPath = &s }},
	} {
		tt.Run(tc.field, func(tt *testing.T) {
			op := operation()
			op.Diagnostics = []t.TypesDiagnostic{diagnostic()}
			for _, n := range []int{tc.max - 1, tc.max, tc.max + 1} {
				tc.set(&op, "/"+strings.Repeat("x", n-1))
				if err := v.HistoryOperation(op); (err == nil) != (n <= tc.max) {
					tt.Fatalf("size %d: %v", n, err)
				}
			}
			for _, s := range []string{"/\xff", "/a\u0085"} {
				tc.set(&op, s)
				if v.HistoryOperation(op) == nil {
					tt.Error("invalid string accepted")
				}
			}
		})
	}
	for _, field := range []string{"created_at", "updated_at", "finished_at"} {
		for _, n := range []int{40, 41} {
			op := operation()
			op.State = "SUCCEEDED"
			op.FinishedAt = pointer(op.UpdatedAt)
			stamp := "2026-10-20T19:00:00." + strings.Repeat("1", n-21) + "Z"
			switch field {
			case "created_at":
				op.CreatedAt = stamp
			case "updated_at":
				op.UpdatedAt = stamp
			case "finished_at":
				op.FinishedAt = &stamp
			}
			if err := v.HistoryOperation(op); (err == nil) != (n == 40) {
				tt.Fatalf("%s %d-byte timestamp: %v", field, n, err)
			}
		}
	}
	op := operation()
	op.State = "FAILED"
	if v.HistoryOperation(op) == nil {
		tt.Fatal("invalid terminal state")
	}
	op = operation()
	op.Diagnostics = make([]t.TypesDiagnostic, 32)
	for i := range op.Diagnostics {
		op.Diagnostics[i] = diagnostic()
	}
	if err := v.HistoryOperation(op); err != nil {
		tt.Fatal(err)
	}
	op.Diagnostics = append(op.Diagnostics, diagnostic())
	if v.HistoryOperation(op) == nil || v.Operation(op) != nil {
		tt.Fatal("history count limit must only apply to history")
	}
}

func TestHistoryWireBoundary(tt *testing.T) {
	op := operation()
	// Fill the encoded DTO with HTML-escaped text, leaving room in one message
	// to reach the exact byte boundary without exceeding an individual limit.
	for i := 0; i < 16; i++ {
		d := diagnostic()
		d.Message = strings.Repeat("<", 650)
		op.Diagnostics = append(op.Diagnostics, d)
	}
	wire, err := json.Marshal(op)
	if err != nil {
		tt.Fatal(err)
	}
	remaining := (64 << 10) - len(wire)
	for i := range op.Diagnostics {
		n := min(1024-len(op.Diagnostics[i].Message), remaining)
		if n < 0 {
			tt.Fatal("invalid fixture size")
		}
		op.Diagnostics[i].Message += strings.Repeat("x", n)
		remaining -= n
	}
	if remaining != 0 {
		tt.Fatal("fixture did not reach the wire boundary")
	}
	if err := v.HistoryOperation(op); err != nil {
		tt.Fatal(err)
	}
	op.ComponentId += "x"
	if v.HistoryOperation(op) == nil {
		tt.Fatal("oversized wire representation accepted")
	}
}

func TestHistoryPageWireSize(tt *testing.T) {
	op := operation()
	page := t.TypesOperationPage{Items: []t.TypesOperation{op}}
	encoded, err := json.Marshal(page)
	if err != nil {
		tt.Fatal(err)
	}
	// Isolate the page envelope limit; individual row limits are checked on read.
	page.Items[0].AppId += strings.Repeat("x", (8<<20)-len(encoded))
	if err := v.HistoryPageSize(page); err != nil {
		tt.Fatal(err)
	}
	page.Items[0].AppId += "x"
	if v.HistoryPageSize(page) == nil {
		tt.Fatal("page above 8 MiB accepted")
	}
	if v.HistoryPageSize(t.TypesOperationPage{}) == nil {
		tt.Fatal("unencodable page accepted")
	}
}

func TestHistoryDiagnosticFileRollout(t *testing.T) {
	for _, n := range []int{256, 257, 512, 513} {
		op := operation()
		d := diagnostic()
		name := strings.Repeat("x", n)
		d.File = &name
		op.Diagnostics = append(op.Diagnostics, d)
		if err := v.HistoryOperation(op); (err == nil) != (n <= 512) {
			t.Fatalf("%d-byte file: %v", n, err)
		}
		if *op.Diagnostics[0].File != name {
			t.Fatal("filename changed during validation")
		}
	}
}

package validation_test

import (
	"testing"

	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

func preview() paastypes.TypesPreviewResponse {
	p := plan()
	return paastypes.TypesPreviewResponse{
		Diagnostics: []paastypes.TypesDiagnostic{}, Plan: &p,
		PreviewId: pointer("preview"), ExpiresAt: pointer("2026-10-20T19:30:00Z"),
	}
}

func operation() paastypes.TypesOperation {
	return paastypes.TypesOperation{
		OperationId: "op", AppId: "app", ComponentId: "component", State: "RUNNING",
		CreatedAt: "2026-10-20T19:00:00Z", UpdatedAt: "2026-10-20T19:00:01.000Z",
		Diagnostics: []paastypes.TypesDiagnostic{},
	}
}

func TestPreviewResponseShapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*paastypes.TypesPreviewResponse)
	}{
		{"nil-diagnostics", func(p *paastypes.TypesPreviewResponse) { p.Diagnostics = nil }},
		{"missing-plan", func(p *paastypes.TypesPreviewResponse) { p.Plan = nil }},
		{"missing-id", func(p *paastypes.TypesPreviewResponse) { p.PreviewId = nil }},
		{"empty-id", func(p *paastypes.TypesPreviewResponse) { p.PreviewId = pointer("") }},
		{"missing-expiry", func(p *paastypes.TypesPreviewResponse) { p.ExpiresAt = nil }},
		{"expiry-offset", func(p *paastypes.TypesPreviewResponse) { p.ExpiresAt = pointer("2026-10-20T19:30:00+00:00") }},
		{"expiry-bad-date", func(p *paastypes.TypesPreviewResponse) { p.ExpiresAt = pointer("2026-02-30T19:30:00Z") }},
		{"error-with-id", func(p *paastypes.TypesPreviewResponse) { p.Diagnostics = []paastypes.TypesDiagnostic{diagnostic()} }},
		{"invalid-plan", func(p *paastypes.TypesPreviewResponse) { p.Plan.Resources[0].Action = "DELETE" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := preview()
			tc.edit(&p)
			if err := validation.PreviewResponse(p); err == nil {
				t.Fatal("invalid preview accepted")
			}
		})
	}
	for _, p := range []paastypes.TypesPreviewResponse{
		preview(),
		{Diagnostics: []paastypes.TypesDiagnostic{diagnostic()}},
	} {
		if err := validation.PreviewResponse(p); err != nil {
			t.Fatal(err)
		}
	}
	warned := preview()
	d := diagnostic()
	d.Severity = "WARN"
	warned.Diagnostics = []paastypes.TypesDiagnostic{d}
	if err := validation.PreviewResponse(warned); err != nil {
		t.Fatal(err)
	}
}

func TestOperationSnapshotRules(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*paastypes.TypesOperation)
	}{
		{"missing-id", func(o *paastypes.TypesOperation) { o.OperationId = "" }},
		{"missing-app", func(o *paastypes.TypesOperation) { o.AppId = "" }},
		{"missing-component", func(o *paastypes.TypesOperation) { o.ComponentId = "" }},
		{"unknown-state", func(o *paastypes.TypesOperation) { o.State = "QUEUED" }},
		{"empty-stage", func(o *paastypes.TypesOperation) { o.Stage = pointer("") }},
		{"running-finished", func(o *paastypes.TypesOperation) { o.FinishedAt = pointer(o.UpdatedAt) }},
		{"terminal-unfinished", func(o *paastypes.TypesOperation) { o.State = "SUCCEEDED" }},
		{"null-diagnostics", func(o *paastypes.TypesOperation) { o.Diagnostics = nil }},
		{"failed-unexplained", func(o *paastypes.TypesOperation) { o.State, o.FinishedAt = "FAILED", pointer(o.UpdatedAt) }},
		{"recovery-unexplained", func(o *paastypes.TypesOperation) { o.State = "RECOVERY_REQUIRED" }},
		{"unknown-outcome-running", func(o *paastypes.TypesOperation) {
			d := diagnostic()
			d.Code = "OUTCOME_UNKNOWN"
			o.Diagnostics = []paastypes.TypesDiagnostic{d}
		}},
		{"unknown-outcome-terminal", func(o *paastypes.TypesOperation) {
			d := diagnostic()
			d.Code = "OUTCOME_UNKNOWN"
			o.Diagnostics, o.State, o.FinishedAt = []paastypes.TypesDiagnostic{d}, "FAILED", pointer(o.UpdatedAt)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := operation()
			tc.edit(&o)
			if err := validation.Operation(o); err == nil {
				t.Fatal("invalid operation accepted")
			}
		})
	}
	for _, state := range []string{"ACCEPTED", "RUNNING", "SUCCEEDED", "FAILED", "RECOVERY_REQUIRED"} {
		o := operation()
		o.State = state
		if state == "SUCCEEDED" || state == "FAILED" {
			o.FinishedAt = pointer(o.UpdatedAt)
		}
		if state == "FAILED" || state == "RECOVERY_REQUIRED" {
			d := diagnostic()
			if state == "FAILED" {
				d.Code = "OUTCOME_KNOWN_PARTIAL"
			} else {
				d.Code = "OUTCOME_UNKNOWN"
			}
			o.Diagnostics = []paastypes.TypesDiagnostic{d}
		}
		if err := validation.Operation(o); err != nil {
			t.Fatalf("%s: %v", state, err)
		}
	}
}

func TestTimestampWireFormat(t *testing.T) {
	for _, stamp := range []string{"", "2026-10-20", "2026-10-20T9:00:00Z", "2026-10-20T19:00:00,1Z", "2026-10-20T19:00:00.Z", "2026-10-20T19:00:00+00:00", "2026-10-20T19:00:00z"} {
		o := operation()
		o.UpdatedAt = stamp
		if err := validation.Operation(o); err == nil {
			t.Fatalf("invalid timestamp accepted: %q", stamp)
		}
	}
	for _, stamp := range []string{"2026-10-20T19:00:00Z", "2026-10-20T19:00:00.000Z", "2026-10-20T19:00:00.123456789Z"} {
		o := operation()
		o.UpdatedAt = stamp
		if err := validation.Operation(o); err != nil {
			t.Fatalf("valid timestamp rejected: %v", err)
		}
	}
}

func TestErrorResponseRelationships(t *testing.T) {
	base := paastypes.TypesErrorResponse{Code: "COMPONENT_BUSY", Message: "Component is busy", Diagnostics: []paastypes.TypesDiagnostic{}, ConflictingOperationId: pointer("blocking-op")}
	if err := validation.ErrorResponse(base); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*paastypes.TypesErrorResponse)
	}{
		{"missing-blocker", func(e *paastypes.TypesErrorResponse) { e.ConflictingOperationId = nil }},
		{"empty-blocker", func(e *paastypes.TypesErrorResponse) { e.ConflictingOperationId = pointer("") }},
		{"irrelevant-blocker", func(e *paastypes.TypesErrorResponse) { e.Code = "PLAN_CHANGED" }},
		{"empty-code", func(e *paastypes.TypesErrorResponse) { e.Code = "" }},
		{"empty-message", func(e *paastypes.TypesErrorResponse) { e.Message = "" }},
		{"nil-diagnostics", func(e *paastypes.TypesErrorResponse) { e.Diagnostics = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := base
			tc.edit(&e)
			if err := validation.ErrorResponse(e); err == nil {
				t.Fatal("invalid error response accepted")
			}
		})
	}
	for _, code := range []string{"PLAN_CHANGED", "RECHECK_FAILED", "ACCEPTANCE_UNKNOWN", "PREVIEW_CAPACITY_EXCEEDED"} {
		e := base
		e.Code, e.ConflictingOperationId = code, nil
		if err := validation.ErrorResponse(e); err != nil {
			t.Fatal(err)
		}
	}
	unknown := base
	unknown.Code = "UNKNOWN_CODE"
	if err := validation.ErrorResponse(unknown); err != nil {
		t.Fatalf("open code catalog was closed by shape validation: %v", err)
	}
	unknown.ConflictingOperationId = pointer("")
	if err := validation.ErrorResponse(unknown); err == nil {
		t.Fatal("empty blocker ID accepted for unknown code")
	}
}

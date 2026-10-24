package validation_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

func pointer[T any](value T) *T { return &value }

func request() paastypes.TypesPreviewRequest {
	return paastypes.TypesPreviewRequest{AppId: "app", ComponentId: "component", Files: []paastypes.TypesManifestFile{{Name: "api.yaml", Content: "kind: Deployment\n"}}}
}

func diagnostic() paastypes.TypesDiagnostic {
	return paastypes.TypesDiagnostic{Severity: "ERROR", Code: "INVALID_MANIFEST", Message: "Invalid manifest"}
}

func plan() paastypes.TypesPlan {
	return paastypes.TypesPlan{
		Resources:         []paastypes.TypesResourceChange{{ApiVersion: "apps/v1", Kind: "Deployment", Namespace: "app", Name: "api", Action: "CREATE", ChangedFields: []string{""}}},
		AdditionalActions: []paastypes.TypesAdditionalAction{},
	}
}

func TestEnvelopeBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*paastypes.TypesPreviewRequest)
	}{
		{"missing-app", func(r *paastypes.TypesPreviewRequest) { r.AppId = "" }},
		{"missing-component", func(r *paastypes.TypesPreviewRequest) { r.ComponentId = "" }},
		{"nil-files", func(r *paastypes.TypesPreviewRequest) { r.Files = nil }},
		{"empty-files", func(r *paastypes.TypesPreviewRequest) { r.Files = []paastypes.TypesManifestFile{} }},
		{"too-many-files", func(r *paastypes.TypesPreviewRequest) { r.Files = make([]paastypes.TypesManifestFile, 65) }},
		{"duplicate-name", func(r *paastypes.TypesPreviewRequest) { r.Files = append(r.Files, r.Files[0]) }},
		{"empty-name", func(r *paastypes.TypesPreviewRequest) { r.Files[0].Name = "" }},
		{"utf8-bytes", func(r *paastypes.TypesPreviewRequest) { r.Files[0].Name = strings.Repeat("я", 257) }},
		{"invalid-utf8", func(r *paastypes.TypesPreviewRequest) { r.Files[0].Name = "\xff" }},
		{"terminal-control", func(r *paastypes.TypesPreviewRequest) { r.Files[0].Name = "\x1b[31msecret" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := request()
			tc.edit(&r)
			err := validation.PreviewRequest(r)
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("expected a safe validation error, got %v", err)
			}
		})
	}
	r := request()
	r.Files = make([]paastypes.TypesManifestFile, 64)
	for i := range r.Files {
		r.Files[i].Name = fmt.Sprintf("%d.yaml", i)
	}
	r.Files[0].Name = strings.Repeat("я", 256)
	before := append([]paastypes.TypesManifestFile(nil), r.Files...)
	if err := validation.PreviewRequest(r); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, r.Files) {
		t.Fatal("validation mutated the snapshot")
	}
	if validation.ApplyRequest(paastypes.TypesApplyRequest{}) == nil || validation.OperationQuery(paastypes.OperationsOperationsGetQuery{}) == nil {
		t.Fatal("empty opaque IDs accepted")
	}
	if err := validation.ApplyRequest(paastypes.TypesApplyRequest{PreviewId: "opaque-id"}); err != nil {
		t.Fatal(err)
	}
	if err := validation.OperationQuery(paastypes.OperationsOperationsGetQuery{OperationId: "opaque-id"}); err != nil {
		t.Fatal(err)
	}
}

func TestDiagnosticCoordinates(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*paastypes.TypesDiagnostic)
	}{
		{"severity", func(d *paastypes.TypesDiagnostic) { d.Severity = "UNKNOWN" }},
		{"empty-code", func(d *paastypes.TypesDiagnostic) { d.Code = "" }},
		{"empty-message", func(d *paastypes.TypesDiagnostic) { d.Message = "" }},
		{"document-without-file", func(d *paastypes.TypesDiagnostic) { d.Document = pointer(1) }},
		{"zero-document", func(d *paastypes.TypesDiagnostic) { d.File, d.Document = pointer("api.yaml"), pointer(0) }},
		{"line-without-file", func(d *paastypes.TypesDiagnostic) { d.Line = pointer(1) }},
		{"column-without-line", func(d *paastypes.TypesDiagnostic) { d.Column = pointer(1) }},
		{"zero-column", func(d *paastypes.TypesDiagnostic) {
			d.File, d.Line, d.Column = pointer("api.yaml"), pointer(1), pointer(0)
		}},
		{"unescaped-tilde", func(d *paastypes.TypesDiagnostic) { d.FieldPath = pointer("/bad~2field") }},
		{"not-pointer", func(d *paastypes.TypesDiagnostic) { d.FieldPath = pointer("spec.field") }},
		{"field-without-document", func(d *paastypes.TypesDiagnostic) { d.File, d.FieldPath = pointer("api.yaml"), pointer("/kind") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := diagnostic()
			tc.edit(&d)
			if err := validation.Diagnostic(d); err == nil {
				t.Fatal("invalid diagnostic accepted")
			}
		})
	}
	for _, d := range []paastypes.TypesDiagnostic{
		diagnostic(),
		{Severity: "WARN", Code: "SOURCE", Message: "File-level problem", File: pointer("api.yaml"), Line: pointer(1)},
		{Severity: "INFO", Code: "FIELD", Message: "Envelope field", FieldPath: pointer("/files/0/name")},
		{Severity: "ERROR", Code: "FIELD", Message: "Root", File: pointer("api.yaml"), Document: pointer(2), FieldPath: pointer("")},
		{Severity: "ERROR", Code: "FIELD", Message: "Escaped key", File: pointer("api.yaml"), Document: pointer(1), FieldPath: pointer("/data/a~1b~0c")},
	} {
		if err := validation.Diagnostic(d); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlanActionsAndIdentity(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*paastypes.TypesPlan)
	}{
		{"null-resources", func(p *paastypes.TypesPlan) { p.Resources = nil }},
		{"null-actions", func(p *paastypes.TypesPlan) { p.AdditionalActions = nil }},
		{"version-not-identity", func(p *paastypes.TypesPlan) {
			r := p.Resources[0]
			r.ApiVersion = "apps/v2"
			p.Resources = append(p.Resources, r)
		}},
		{"unsorted-resources", func(p *paastypes.TypesPlan) {
			r := p.Resources[0]
			r.ApiVersion, r.Kind = "v1", "ConfigMap"
			p.Resources = append(p.Resources, r)
		}},
		{"empty-identity", func(p *paastypes.TypesPlan) { p.Resources[0].Name = "" }},
		{"bad-api-version", func(p *paastypes.TypesPlan) { p.Resources[0].ApiVersion = "apps/v1/extra" }},
		{"unknown-action", func(p *paastypes.TypesPlan) { p.Resources[0].Action = "DELETE" }},
		{"create-not-root", func(p *paastypes.TypesPlan) { p.Resources[0].ChangedFields = []string{"/spec"} }},
		{"null-fields", func(p *paastypes.TypesPlan) { p.Resources[0].ChangedFields = nil }},
		{"empty-update", func(p *paastypes.TypesPlan) {
			p.Resources[0].Action, p.Resources[0].ChangedFields = "UPDATE", []string{}
		}},
		{"no-change-fields", func(p *paastypes.TypesPlan) { p.Resources[0].Action = "NO_CHANGE" }},
		{"duplicate-fields", func(p *paastypes.TypesPlan) {
			p.Resources[0].Action, p.Resources[0].ChangedFields = "UPDATE", []string{"/spec", "/spec"}
		}},
		{"unsorted-fields", func(p *paastypes.TypesPlan) {
			p.Resources[0].Action, p.Resources[0].ChangedFields = "UPDATE", []string{"/z", "/a"}
		}},
		{"bad-field-pointer", func(p *paastypes.TypesPlan) {
			p.Resources[0].Action, p.Resources[0].ChangedFields = "UPDATE", []string{"/bad~"}
		}},
		{"unknown-additional-action", func(p *paastypes.TypesPlan) {
			p.AdditionalActions = []paastypes.TypesAdditionalAction{{Kind: "ROLLBACK", AppId: "app"}}
		}},
		{"duplicate-additional-action", func(p *paastypes.TypesPlan) {
			a := paastypes.TypesAdditionalAction{Kind: "INITIAL_MIGRATION", AppId: "app"}
			p.AdditionalActions = []paastypes.TypesAdditionalAction{a, a}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := plan()
			tc.edit(&p)
			if err := validation.Plan(p); err == nil {
				t.Fatal("invalid plan accepted")
			}
		})
	}
	for _, action := range []string{"CREATE", "UPDATE", "NO_CHANGE"} {
		p := plan()
		p.Resources[0].Action = action
		if action == "UPDATE" {
			p.Resources[0].ChangedFields = []string{"/metadata/labels/a~1b", "/spec/replicas"}
		}
		if action == "NO_CHANGE" {
			p.Resources[0].ChangedFields = []string{}
		}
		if err := validation.Plan(p); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlanValidationDoesNotRewriteInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*paastypes.TypesPlan)
	}{
		{"valid", func(*paastypes.TypesPlan) {}},
		{"unsorted-resources", func(p *paastypes.TypesPlan) {
			r := p.Resources[0]
			r.ApiVersion, r.Kind = "v1", "ConfigMap"
			p.Resources = append(p.Resources, r)
		}},
		{"unsorted-fields", func(p *paastypes.TypesPlan) {
			p.Resources[0].Action = "UPDATE"
			p.Resources[0].ChangedFields = []string{"/z", "/a"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := plan()
			tc.edit(&p)
			before, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			err = validation.Plan(p)
			if (err == nil) != (tc.name == "valid") {
				t.Fatalf("unexpected result: %v", err)
			}
			after, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("plan changed during validation")
			}
		})
	}
}

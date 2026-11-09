package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/dkoshenkov/packages-go/middlewarex/httpx"

	c "github.com/dkoshenkov/paas-common/pkg/http/paas/client"
	s "github.com/dkoshenkov/paas-common/pkg/http/paas/server/migrations"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/transport"
	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	v "github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

type migrationFixture struct {
	calls  int
	record t.MigrationTypesOperationRecord
	deploy t.MigrationTypesDeployPreview
}

func (f *migrationFixture) Preview(_ context.Context, q t.MigrationTypesPreviewRequest) (s.MigrationsPreviewResponse, error) {
	f.calls++
	return s.MigrationsPreviewResponse200{Body: t.MigrationTypesPreview{PreviewId: "preview", ExpiresAt: f.record.CreatedAt, Diagnostics: []t.TypesDiagnostic{}, Migration: t.MigrationTypesAppMigration{AppId: q.AppId, Revision: q.ExpectedMigratorRevision, Image: f.record.Migration.Image, Reason: "MANUAL"}}}, nil
}

func (f *migrationFixture) Apply(_ context.Context, q t.TypesApplyRequest) (s.MigrationsApplyResponse, error) {
	f.calls++
	switch q.PreviewId {
	case "expired":
		return s.MigrationsApplyResponse410{Body: apiError("PREVIEW_EXPIRED")}, nil
	case "replay":
		return s.MigrationsApplyResponse200{Body: f.record}, nil
	default:
		return s.MigrationsApplyResponse202{Body: f.record}, nil
	}
}

func (f *migrationFixture) PreviewDeployment(_ context.Context, _ t.MigrationTypesDeployRequest) (s.MigrationsPreviewDeploymentResponse, error) {
	f.calls++
	return s.MigrationsPreviewDeploymentResponse200{Body: f.deploy}, nil
}

func (f *migrationFixture) ApplyDeployment(_ context.Context, q t.TypesApplyRequest) (s.MigrationsApplyDeploymentResponse, error) {
	f.calls++
	if q.PreviewId == "wrong-kind" {
		return s.MigrationsApplyDeploymentResponse409{Body: apiError("PREVIEW_KIND_MISMATCH")}, nil
	}
	rec := f.record
	rec.TargetType = "COMPONENT"
	rec.ComponentId = pointer("c")
	info := *rec.Migration
	info.Reason = "REQUESTED"
	rec.Migration = &info
	return s.MigrationsApplyDeploymentResponse202{Body: rec}, nil
}

func (f *migrationFixture) Inspect(_ context.Context, q t.MigrationsMigrationsInspectQuery) (s.MigrationsInspectResponse, error) {
	f.calls++
	if q.OperationId != f.record.OperationId {
		return s.MigrationsInspectResponse404{Body: apiError("NOT_FOUND")}, nil
	}
	return s.MigrationsInspectResponse200{Body: f.record}, nil
}

func (f *migrationFixture) History(_ context.Context, q t.MigrationsMigrationsHistoryQuery) (s.MigrationsHistoryResponse, error) {
	f.calls++
	if q.Cursor != nil {
		return s.MigrationsHistoryResponse400{Body: apiError("INVALID_CURSOR")}, nil
	}
	return s.MigrationsHistoryResponse200{Body: t.MigrationTypesOperationPage{Items: []t.MigrationTypesOperationRecord{f.record}}}, nil
}

func (f *migrationFixture) MigrationLog(_ context.Context, q t.MigrationsMigrationsMigrationLogQuery) (s.MigrationsMigrationLogResponse, error) {
	f.calls++
	return s.MigrationsMigrationLogResponse200{Body: t.MigrationTypesLogPage{OperationId: q.OperationId, Items: []t.MigrationTypesLogEvent{{Seq: 2, At: f.record.CreatedAt, Code: "VERSION_APPLIED", Version: pointer(10)}}}}, nil
}

func TestMigrationSDK(tt *testing.T) {
	ctx := tt.Context()
	f := migrationFixture{record: t.MigrationTypesOperationRecord{OperationId: "o", TargetType: "APP", AppId: "a", State: "RUNNING", Diagnostics: []t.TypesDiagnostic{}, CreatedAt: "2026-11-08T00:00:00Z", UpdatedAt: "2026-11-08T00:00:00Z", Migration: &t.MigrationTypesMigration{Image: "ghcr.io/org/m@sha256:" + strings.Repeat("a", 64), Revision: "r", Reason: "MANUAL"}}}
	f.deploy = t.MigrationTypesDeployPreview{Diagnostics: []t.TypesDiagnostic{{Code: "INVALID", Severity: "ERROR", Message: "invalid manifest"}}}
	mux := http.NewServeMux()
	s.RegisterMigrationsHandlersWithRuntime(mux, &f, httpx.NewRuntime(httpx.WithErrorEncoder(httpx.ErrorEncoderFunc(transport.EncodeError))))
	httpClient := &http.Client{Transport: handlerTransport{handler: mux}}
	api, err := c.NewMigrationsClient("http://paas.test", httpClient)
	if err != nil {
		tt.Fatal(err)
	}
	preview, err := api.Preview(ctx, t.MigrationTypesPreviewRequest{AppId: "a", ExpectedMigratorRevision: "r"})
	p, ok := preview.(c.MigrationsPreviewResponse200)
	if err != nil || !ok || v.MigrationPreview(p.Body) != nil {
		tt.Fatal(preview, err)
	}
	apply, err := api.Apply(ctx, t.TypesApplyRequest{PreviewId: "preview"})
	accepted, ok := apply.(c.MigrationsApplyResponse202)
	if err != nil || !ok || v.OperationRecord(accepted.Body) != nil {
		tt.Fatal(apply, err)
	}
	apply, err = api.Apply(ctx, t.TypesApplyRequest{PreviewId: "replay"})
	if _, ok := apply.(c.MigrationsApplyResponse200); err != nil || !ok {
		tt.Fatal(apply, err)
	}
	apply, err = api.Apply(ctx, t.TypesApplyRequest{PreviewId: "expired"})
	if _, ok := apply.(c.MigrationsApplyResponse410); err != nil || !ok {
		tt.Fatal(apply, err)
	}
	combo, err := api.PreviewDeployment(ctx, t.MigrationTypesDeployRequest{AppId: "a", ComponentId: "c", ExpectedMigratorRevision: "r", Files: []t.TypesManifestFile{{Name: "m.yaml", Content: "x"}}})
	dep, ok := combo.(c.MigrationsPreviewDeploymentResponse200)
	if err != nil || !ok || v.MigratingDeployPreview(dep.Body) != nil {
		tt.Fatal(combo, err)
	}
	ca, err := api.ApplyDeployment(ctx, t.TypesApplyRequest{PreviewId: "combo"})
	component, ok := ca.(c.MigrationsApplyDeploymentResponse202)
	if err != nil || !ok || v.OperationRecord(component.Body) != nil {
		tt.Fatal(ca, err)
	}
	ca, err = api.ApplyDeployment(ctx, t.TypesApplyRequest{PreviewId: "wrong-kind"})
	if _, ok := ca.(c.MigrationsApplyDeploymentResponse409); err != nil || !ok {
		tt.Fatal(ca, err)
	}
	inspection, err := api.Inspect(ctx, t.MigrationsMigrationsInspectQuery{OperationId: "o"})
	if item, ok := inspection.(c.MigrationsInspectResponse200); err != nil || !ok || item.Body.ComponentId != nil {
		tt.Fatal(inspection, err)
	}
	history, err := api.History(ctx, t.MigrationsMigrationsHistoryQuery{State: pointer("RUNNING"), Limit: pointer(1)})
	h, ok := history.(c.MigrationsHistoryResponse200)
	if err != nil || !ok || v.MigrationHistoryPage(h.Body) != nil {
		tt.Fatal(history, err)
	}
	lq := t.MigrationsMigrationsMigrationLogQuery{OperationId: "o", After: pointer(1), Limit: pointer(1)}
	log, err := api.MigrationLog(ctx, lq)
	l, ok := log.(c.MigrationsMigrationLogResponse200)
	if err != nil || !ok || v.MigrationLogPage(l.Body, lq) != nil {
		tt.Fatal(log, err)
	}
	before := f.calls
	for _, tc := range []struct {
		path, body string
		status     int
	}{
		{"/migrations/preview", `{"app_id":"a","expected_migrator_revision":"r","image":"canary"}`, 400},
		{"/deployments/preview-with-migration", `{"app_id":"a","component_id":"c","expected_migrator_revision":"r","files":[{"name":"f","content":"x","image":"canary"}]}`, 400},
		{"/migrations/apply", strings.Repeat(" ", (1<<20)+1), 413},
		{"/deployments/apply-with-migration", strings.Repeat(" ", (1<<20)+1), 413},
		{"/deployments/preview-with-migration", strings.Repeat(" ", (8<<20)+1), 413},
	} {
		req, err := http.NewRequestWithContext(ctx, "POST", "http://paas.test"+tc.path, strings.NewReader(tc.body))
		if err != nil {
			tt.Fatal(err)
		}
		req.ContentLength = -1
		res, err := httpClient.Do(req)
		if err != nil {
			tt.Fatal(err)
		}
		var body t.TypesErrorResponse
		err = json.NewDecoder(res.Body).Decode(&body)
		_ = res.Body.Close()
		if err != nil || res.StatusCode != tc.status || v.ErrorResponse(body) != nil || strings.Contains(body.Message, "canary") {
			tt.Fatal(tc.path, res.StatusCode, body, err)
		}
	}
	if f.calls != before {
		tt.Fatal("invalid request reached migration service")
	}
}

package validation_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	v "github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

func TestMigrationTargetAndHistory(tt *testing.T) {
	now := "2026-11-07T12:00:00Z"
	image := "example.org/db@sha256:" + strings.Repeat("a", 64)
	op := t.MigrationTypesOperationRecord{OperationId: "o", TargetType: "APP", AppId: "orders", State: "ACCEPTED", Diagnostics: []t.TypesDiagnostic{}, CreatedAt: now, UpdatedAt: now, Migration: &t.MigrationTypesMigration{Image: image, Revision: "m", Reason: "MANUAL"}}
	if err := v.OperationRecord(op); err != nil {
		tt.Fatal(err)
	}
	op.ComponentId = pointer("c")
	if v.OperationRecord(op) == nil {
		tt.Fatal("APP exposed component_id")
	}
	op.TargetType = "COMPONENT"
	if v.OperationRecord(op) == nil {
		tt.Fatal("COMPONENT accepted manual app migration")
	}
	op.Migration.Reason = "REQUESTED"
	if err := v.OperationRecord(op); err != nil {
		tt.Fatal(err)
	}
	op.Migration = nil
	if err := v.OperationRecord(op); err != nil {
		tt.Fatal("legacy projection", err)
	}
}

func TestLegacyRecordAtFullWireSize(tt *testing.T) {
	now := "2026-11-08T00:00:00Z"
	old := t.TypesOperation{OperationId: "o", AppId: "a", ComponentId: "c", State: "ACCEPTED", CreatedAt: now, UpdatedAt: now, Diagnostics: []t.TypesDiagnostic{}}
	size := func(x any) int {
		b, e := json.Marshal(x)
		if e != nil {
			tt.Fatal(e)
		}
		return len(b)
	}
	// '<' encodes as six wire bytes, while diagnostics themselves remain
	// inside the 1024-byte message cap. Finish with unescaped ASCII.
	for i := 0; i < 10; i++ {
		old.Diagnostics = append(old.Diagnostics, t.TypesDiagnostic{Severity: "WARN", Code: "W", Message: strings.Repeat("<", 1024)})
	}
	for i := 0; i < 6; i++ {
		old.Diagnostics = append(old.Diagnostics, t.TypesDiagnostic{Severity: "WARN", Code: "W", Message: "x"})
	}
	remaining := v.MaxHistoryOperationBytes - size(old)
	for i := 10; i < len(old.Diagnostics) && remaining > 0; i++ {
		n := min(remaining, 1023)
		old.Diagnostics[i].Message += strings.Repeat("x", n)
		remaining -= n
	}
	if size(old) != 65536 {
		tt.Fatal(size(old))
	}
	converted, err := v.LegacyComponentRecord(old)
	if err != nil {
		tt.Fatal(err)
	}
	if size(converted) != 65562 || v.OperationRecord(converted) == nil || v.ReadableOperationRecord(converted) != nil {
		tt.Fatal("new/legacy limits", size(converted))
	}
	withMigration := converted
	withMigration.Migration = &t.MigrationTypesMigration{Image: "org/job@sha256:" + strings.Repeat("c", 64), Revision: "r", Reason: "INITIAL"}
	if v.ReadableOperationRecord(withMigration) == nil {
		tt.Fatal("new migration-bearing record incorrectly grandfathered")
	}
	legacy, ok, err := v.ComponentProjection(converted)
	if err != nil || !ok || !reflect.DeepEqual(old, legacy) {
		tt.Fatal("lossy legacy conversion", err)
	}
	if err := v.MigrationHistoryPage(t.MigrationTypesOperationPage{Items: []t.MigrationTypesOperationRecord{converted}}); err != nil {
		tt.Fatal(err)
	}
	old.Diagnostics[len(old.Diagnostics)-1].Message += "x"
	if _, err := v.LegacyComponentRecord(old); err == nil {
		tt.Fatal("too-large original accepted")
	}
	converted.Diagnostics = old.Diagnostics
	if v.ReadableOperationRecord(converted) == nil {
		tt.Fatal("too-large legacy view accepted")
	}
}

func TestMigrationPreviews(tt *testing.T) {
	now := "2026-11-08T00:00:00Z"
	img := "org/job@sha256:" + strings.Repeat("b", 64)
	req := t.MigrationTypesPreviewRequest{AppId: "app", ExpectedMigratorRevision: "r"}
	if v.MigrationPreviewRequest(req) != nil {
		tt.Fatal(req)
	}
	req.ExpectedMigratorRevision = ""
	if v.MigrationPreviewRequest(req) == nil {
		tt.Fatal("missing revision")
	}
	deploy := t.MigrationTypesDeployRequest{AppId: "app", ComponentId: "cmp", ExpectedMigratorRevision: "r", Files: []t.TypesManifestFile{{Name: "file", Content: "yaml"}}}
	if v.MigratingDeployRequest(deploy) != nil {
		tt.Fatal(deploy)
	}
	deploy.Files = nil
	if v.MigratingDeployRequest(deploy) == nil {
		tt.Fatal("empty package")
	}
	deploy.ExpectedMigratorRevision = ""
	if v.MigratingDeployRequest(deploy) == nil {
		tt.Fatal("missing revision")
	}
	manual := t.MigrationTypesPreview{PreviewId: "p", ExpiresAt: now, Diagnostics: []t.TypesDiagnostic{}, Migration: t.MigrationTypesAppMigration{AppId: "app", Image: img, Revision: "r", Reason: "MANUAL"}}
	for _, tc := range []struct {
		mut func(*t.MigrationTypesPreview)
		ok  bool
	}{
		{func(*t.MigrationTypesPreview) {}, true}, {func(p *t.MigrationTypesPreview) { p.PreviewId = "" }, false}, {func(p *t.MigrationTypesPreview) { p.Migration.Reason = "INITIAL" }, false}, {func(p *t.MigrationTypesPreview) { p.Migration.Image = "bad:tag" }, false}, {func(p *t.MigrationTypesPreview) { p.Migration.AppId = "/" }, false}, {func(p *t.MigrationTypesPreview) { p.Migration.Revision = "" }, false}, {func(p *t.MigrationTypesPreview) { p.Diagnostics = nil }, false}, {func(p *t.MigrationTypesPreview) {
			p.Diagnostics = []t.TypesDiagnostic{{Code: "E", Message: "error", Severity: "ERROR"}}
		}, false}, {func(p *t.MigrationTypesPreview) { p.ExpiresAt = "tomorrow" }, false},
	} {
		p := manual
		tc.mut(&p)
		if (v.MigrationPreview(p) == nil) != tc.ok {
			tt.Fatal(p)
		}
	}
	for _, reason := range []string{"INITIAL", "REQUESTED"} {
		m := manual.Migration
		m.Reason = reason
		action := t.MigrationTypesAdditionalAction{AppId: m.AppId, Kind: reason + "_MIGRATION"}
		plan := t.MigrationTypesPlan{Resources: []t.TypesResourceChange{}, AdditionalActions: []t.MigrationTypesAdditionalAction{action}}
		p := t.MigrationTypesDeployPreview{PreviewId: &manual.PreviewId, ExpiresAt: &now, Migration: &m, Plan: &plan, Diagnostics: []t.TypesDiagnostic{}}
		if err := v.MigratingDeployPreview(p); err != nil {
			tt.Fatal(err)
		}
		plan.AdditionalActions[0].AppId = "other"
		if v.MigratingDeployPreview(p) == nil {
			tt.Fatal("cross app action")
		}
		plan.AdditionalActions[0] = action
		plan.AdditionalActions[0].Kind = "MANUAL"
		if v.MigratingDeployPreview(p) == nil {
			tt.Fatal("wrong action")
		}
		plan.AdditionalActions[0] = action
		m.Reason = "MANUAL"
		plan.AdditionalActions[0].Kind = "MANUAL_MIGRATION"
		if v.MigratingDeployPreview(p) == nil {
			tt.Fatal("manual deployment")
		}
		m.Reason = reason
		plan.AdditionalActions = []t.MigrationTypesAdditionalAction{action, action}
		if v.MigratingDeployPreview(p) == nil {
			tt.Fatal("two jobs")
		}
		plan.AdditionalActions = []t.MigrationTypesAdditionalAction{action}
		p.Migration = nil
		if v.MigratingDeployPreview(p) == nil {
			tt.Fatal("missing migration")
		}
		p.Plan = nil
		p.PreviewId = nil
		p.ExpiresAt = nil
		p.Diagnostics = []t.TypesDiagnostic{{Code: "INVALID", Severity: "ERROR", Message: "invalid"}}
		if err := v.MigratingDeployPreview(p); err != nil {
			tt.Fatal(err)
		}
		p.Migration = &m
		if v.MigratingDeployPreview(p) == nil {
			tt.Fatal("invalid manifest with migration")
		}
	}
}

func TestMigrationLogBounds(tt *testing.T) {
	const at = "2026-11-08T00:00:00Z"
	q := t.MigrationsMigrationsMigrationLogQuery{OperationId: "o"}
	if v.MigrationLogQuery(q) != nil {
		tt.Fatal(q)
	}
	for _, n := range []int{-1, 0, 1, 50, 51} {
		q.Limit = &n
		if (v.MigrationLogQuery(q) == nil) != (n >= 1 && n <= 50) {
			tt.Fatal(n)
		}
	}
	q.Limit = nil
	q.After = pointer(-1)
	if v.MigrationLogQuery(q) == nil {
		tt.Fatal(q)
	}
	q.After = pointer(0)
	q.OperationId = ""
	if v.MigrationLogQuery(q) == nil {
		tt.Fatal(q)
	}
	q.OperationId = "o"
	p := t.MigrationTypesLogPage{OperationId: "o", Items: []t.MigrationTypesLogEvent{}}
	if err := v.MigrationLogPage(p, q); err != nil {
		tt.Fatal(err)
	}
	for i, code := range []string{"JOB_CREATED", "JOB_STARTED", "VERSION_APPLIED", "NO_PENDING_MIGRATIONS", "JOB_SUCCEEDED", "JOB_FAILED", "LOG_TRUNCATED"} {
		p.Items = append(p.Items, t.MigrationTypesLogEvent{Seq: i + 1, At: at, Code: code, Version: pointer(0)})
	}
	p.Truncated = true
	p.NextAfter = pointer(7)
	if err := v.MigrationLogPage(p, q); err != nil {
		tt.Fatal(err)
	}
	for _, mut := range []func(*t.MigrationTypesLogEvent){func(e *t.MigrationTypesLogEvent) { e.Seq = 0 }, func(e *t.MigrationTypesLogEvent) { e.Code = "SQL" }, func(e *t.MigrationTypesLogEvent) { e.At = "bad" }, func(e *t.MigrationTypesLogEvent) { e.Version = pointer(-1) }, func(e *t.MigrationTypesLogEvent) { e.At = "2026-11-08T00:00:00." + strings.Repeat("1", 23) + "Z" }} {
		e := p.Items[0]
		mut(&e)
		if v.MigrationLogEvent(e) == nil {
			tt.Fatal(e)
		}
	}
	q.After = pointer(1)
	if v.MigrationLogPage(p, q) == nil {
		tt.Fatal("old event emitted")
	}
	q.After = nil
	p.NextAfter = pointer(8)
	if v.MigrationLogPage(p, q) == nil {
		tt.Fatal("wrong next")
	}
	p.NextAfter = nil
	q.Limit = pointer(6)
	if v.MigrationLogPage(p, q) == nil {
		tt.Fatal("ignored limit")
	}
	q.Limit = nil
	p.OperationId = "x"
	if v.MigrationLogPage(p, q) == nil {
		tt.Fatal("foreign log")
	}
	p.OperationId = "o"
	events := make([]t.MigrationTypesLogEvent, v.MaxMigrationEvents)
	for i := range events {
		events[i] = t.MigrationTypesLogEvent{Seq: i + 1, At: at, Code: "JOB_STARTED"}
	}
	if err := v.MigrationLogSnapshot(events, false); err != nil {
		tt.Fatal(err)
	}
	if v.MigrationLogSnapshot(append(events, p.Items[0]), true) == nil {
		tt.Fatal("retained >128")
	}
	events[2].Seq = 4
	events = events[:3]
	if v.MigrationLogSnapshot(events, false) == nil || v.MigrationLogSnapshot(events, true) != nil {
		tt.Fatal("gap handling")
	}
	events[0].Seq = 2
	if v.MigrationLogSnapshot(events, true) == nil {
		tt.Fatal("missing initial seq")
	}
}

func TestMigrationHistoryFilters(tt *testing.T) {
	if v.MigrationInspectQuery(t.MigrationsMigrationsInspectQuery{OperationId: "o"}) != nil || v.MigrationInspectQuery(t.MigrationsMigrationsInspectQuery{}) == nil {
		tt.Fatal("inspect ID")
	}
	if v.MigrationHistoryQuery(t.MigrationsMigrationsHistoryQuery{State: pointer("RUNNING"), Limit: pointer(100)}) != nil || v.MigrationHistoryQuery(t.MigrationsMigrationsHistoryQuery{State: pointer("UNKNOWN")}) == nil {
		tt.Fatal("history query")
	}
	for _, code := range []string{"DATABASE_BUSY", "ROUTE_BUSY"} {
		e := t.TypesErrorResponse{Code: code, Diagnostics: []t.TypesDiagnostic{}, Message: "busy"}
		if v.ErrorResponse(e) == nil {
			tt.Fatal("missing blocker", code)
		}
		e.ConflictingOperationId = pointer("o")
		if v.ErrorResponse(e) != nil {
			tt.Fatal(e)
		}
	}
}

func TestMigrationRecordInvalidShapes(tt *testing.T) {
	now := "2026-11-08T00:00:00Z"
	seed := t.MigrationTypesOperationRecord{OperationId: "o", AppId: "app", TargetType: "APP", State: "ACCEPTED", CreatedAt: now, UpdatedAt: now, Diagnostics: []t.TypesDiagnostic{}}
	for _, edit := range []func(*t.MigrationTypesOperationRecord){
		func(r *t.MigrationTypesOperationRecord) { r.TargetType = "other" },
		func(r *t.MigrationTypesOperationRecord) { r.TargetType = "COMPONENT" },
		func(r *t.MigrationTypesOperationRecord) { r.TargetType = "COMPONENT"; r.ComponentId = pointer("") },
		func(r *t.MigrationTypesOperationRecord) { r.ComponentId = pointer("c") },
		func(r *t.MigrationTypesOperationRecord) { r.Migration = nil },
		func(r *t.MigrationTypesOperationRecord) { r.Migration.Reason = "INITIAL" },
		func(r *t.MigrationTypesOperationRecord) { r.Migration.Image = "tag" },
		func(r *t.MigrationTypesOperationRecord) { r.Migration.Revision = "" },
		func(r *t.MigrationTypesOperationRecord) { r.AppId = "" },
		func(r *t.MigrationTypesOperationRecord) { r.OperationId = "" },
		func(r *t.MigrationTypesOperationRecord) { r.State = "other" },
	} {
		r := seed
		r.Migration = &t.MigrationTypesMigration{Image: "org/db@sha256:" + strings.Repeat("a", 64), Revision: "r", Reason: "MANUAL"}
		edit(&r)
		if v.OperationRecord(r) == nil || v.ReadableOperationRecord(r) == nil {
			tt.Fatal("invalid record accepted", r)
		}
		if _, ok, err := v.ComponentProjection(r); ok || err == nil {
			tt.Fatal("invalid projection")
		}
	}
	seed.Migration = &t.MigrationTypesMigration{Image: "org/db@sha256:" + strings.Repeat("a", 64), Revision: "r", Reason: "MANUAL"}
	if _, ok, err := v.ComponentProjection(seed); ok || err != nil {
		tt.Fatal("app shown in legacy", err)
	}
	same := seed
	same.OperationId = "o2"
	for _, tc := range []struct {
		page  t.MigrationTypesOperationPage
		valid bool
	}{
		{t.MigrationTypesOperationPage{}, false},
		{t.MigrationTypesOperationPage{Items: []t.MigrationTypesOperationRecord{}, NextCursor: pointer("token")}, false},
		{t.MigrationTypesOperationPage{Items: []t.MigrationTypesOperationRecord{seed, same}}, true},
		{t.MigrationTypesOperationPage{Items: []t.MigrationTypesOperationRecord{seed, seed}}, false},
		{t.MigrationTypesOperationPage{Items: []t.MigrationTypesOperationRecord{same, seed}}, false},
		{t.MigrationTypesOperationPage{Items: []t.MigrationTypesOperationRecord{{}}}, false},
	} {
		if (v.MigrationHistoryPage(tc.page) == nil) != tc.valid {
			tt.Fatal(tc)
		}
	}
}

func TestMigrationJournalPages(tt *testing.T) {
	q := t.MigrationsMigrationsMigrationLogQuery{OperationId: "o"}
	p := t.MigrationTypesLogPage{OperationId: "o", Items: []t.MigrationTypesLogEvent{{Seq: 1, At: "2026-11-08T00:00:00." + strings.Repeat("1", 19) + "Z", Code: "JOB_CREATED"}}}
	if len(p.Items[0].At) != 40 || v.MigrationLogPage(p, q) != nil {
		tt.Fatal(p)
	}
	p.NextAfter = pointer(1)
	if v.MigrationLogPage(p, q) != nil {
		tt.Fatal("one-item page")
	}
	p.NextAfter = nil
	q.Limit = pointer(1)
	if v.MigrationLogPage(p, q) != nil {
		tt.Fatal("exact query limit")
	}
	q.Limit = nil
	ev := p.Items[0]
	ev.Code = "LOG_TRUNCATED"
	p.Items = []t.MigrationTypesLogEvent{ev}
	if v.MigrationLogSnapshot(p.Items, false) == nil || v.MigrationLogPage(p, q) == nil {
		tt.Fatal("missing truncation marker")
	}
	p.Truncated = true
	if v.MigrationLogSnapshot(p.Items, true) != nil || v.MigrationLogPage(p, q) != nil {
		tt.Fatal(p)
	}
	if v.MigrationLogSnapshot(nil, false) == nil {
		tt.Fatal("nil snapshot")
	}
	items := []t.MigrationTypesLogEvent{ev, ev}
	if v.MigrationLogSnapshot(items, true) == nil {
		tt.Fatal("repeated log seq")
	}
	items[1].Seq = 2
	items[0].Seq = 2
	items[1].Seq = 3
	if v.MigrationLogSnapshot(items, true) == nil {
		tt.Fatal("initial gap")
	}
	// With a preserved first event, a later gap is permitted only for an
	// explicitly truncated log whose next position is taken from actual seq.
	items[0].Seq = 1
	if v.MigrationLogSnapshot(items, true) != nil {
		tt.Fatal(items)
	}
	q.OperationId = ""
	if v.MigrationLogPage(p, q) == nil {
		tt.Fatal("page for invalid query")
	}
}

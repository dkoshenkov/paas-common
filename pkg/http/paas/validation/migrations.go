package validation

import (
	"errors"
	"slices"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

var errMigrationShape = errors.New("invalid migration target, revision or response shape")

func MigrationPreviewRequest(q t.MigrationTypesPreviewRequest) error {
	return appRevision(q.AppId, q.ExpectedMigratorRevision)
}

func MigratingDeployRequest(q t.MigrationTypesDeployRequest) error {
	if err := appRevision(q.AppId, q.ExpectedMigratorRevision); err != nil {
		return err
	}
	return PreviewRequest(t.TypesPreviewRequest{AppId: q.AppId, ComponentId: q.ComponentId, Files: q.Files})
}

func migrationIdentity(app, image, revision string) error {
	if err := appRevision(app, revision); err != nil {
		return err
	}
	return migrator(image)
}

func MigrationPreview(p t.MigrationTypesPreview) error {
	if p.PreviewId == "" || p.Migration.Reason != "MANUAL" {
		return errMigrationShape
	}
	if err := diagnostics(p.Diagnostics); err != nil {
		return err
	}
	if slices.ContainsFunc(p.Diagnostics, func(d t.TypesDiagnostic) bool { return d.Severity == "ERROR" }) {
		return errMigrationShape
	}
	if err := timestamp(p.ExpiresAt); err != nil {
		return err
	}
	return migrationIdentity(p.Migration.AppId, p.Migration.Image, p.Migration.Revision)
}

func MigratingDeployPreview(p t.MigrationTypesDeployPreview) error {
	// Share the established two response shapes, resource ordering and field
	// pointers. The migration action below has its own closed kind set.
	standard := t.TypesPreviewResponse{Diagnostics: p.Diagnostics, PreviewId: p.PreviewId, ExpiresAt: p.ExpiresAt}
	if p.Plan != nil {
		standard.Plan = &t.TypesPlan{Resources: p.Plan.Resources, AdditionalActions: []t.TypesAdditionalAction{}}
	}
	if err := PreviewResponse(standard); err != nil {
		return err
	}
	if p.Plan == nil {
		if p.Migration != nil {
			return errMigrationShape
		}
		return nil
	}
	if p.Migration == nil || len(p.Plan.AdditionalActions) != 1 {
		return errMigrationShape
	}
	migration := p.Migration
	action := p.Plan.AdditionalActions[0]
	if action.AppId != migration.AppId || (migration.Reason != "INITIAL" && migration.Reason != "REQUESTED") || action.Kind != migration.Reason+"_MIGRATION" {
		return errMigrationShape
	}
	return migrationIdentity(migration.AppId, migration.Image, migration.Revision)
}

func MigrationHistoryQuery(q t.MigrationsMigrationsHistoryQuery) error {
	return OperationListQuery(t.OperationsOperationsListQuery{State: q.State, Limit: q.Limit, Cursor: q.Cursor})
}

func MigrationInspectQuery(q t.MigrationsMigrationsInspectQuery) error {
	if !HistoryOperationID(q.OperationId) {
		return errHistoryBounds
	}
	return nil
}

func MigrationHistoryPage(p t.MigrationTypesOperationPage) error {
	if p.Items == nil {
		return errMigrationShape
	}
	if err := catalogPage(len(p.Items), p.NextCursor); err != nil {
		return err
	}
	for i, op := range p.Items {
		if err := ReadableOperationRecord(op); err != nil {
			return err
		}
		if i > 0 && p.Items[i-1].OperationId >= op.OperationId {
			return errMigrationShape
		}
	}
	return historyWireSize(p, MaxHistoryPageBytes)
}

const (
	MaxMigrationEvents       = 128
	MaxMigrationLogBytes     = 32 << 10
	DefaultMigrationLogLimit = 20
	MaxMigrationLogLimit     = 50
)

func MigrationLogQuery(q t.MigrationsMigrationsMigrationLogQuery) error {
	if !HistoryOperationID(q.OperationId) || q.After != nil && *q.After < 0 || q.Limit != nil && (*q.Limit < 1 || *q.Limit > MaxMigrationLogLimit) {
		return errMigrationShape
	}
	return nil
}

func MigrationLogEvent(e t.MigrationTypesLogEvent) error {
	if e.Seq < 1 || e.Version != nil && *e.Version < 0 {
		return errMigrationShape
	}
	switch e.Code {
	case "JOB_CREATED", "JOB_STARTED", "VERSION_APPLIED", "NO_PENDING_MIGRATIONS", "JOB_SUCCEEDED", "JOB_FAILED", "LOG_TRUNCATED":
	default:
		return errMigrationShape
	}
	if len(e.At) > 40 {
		return errHistoryBounds
	}
	return timestamp(e.At)
}

// MigrationLogPage checks that the response belongs to the query. seq can
// have gaps after truncation; the resume position must equal the last event.
func MigrationLogPage(p t.MigrationTypesLogPage, q t.MigrationsMigrationsMigrationLogQuery) error {
	if err := MigrationLogQuery(q); err != nil {
		return err
	}
	limit, after := DefaultMigrationLogLimit, 0
	if q.Limit != nil {
		limit = *q.Limit
	}
	if q.After != nil {
		after = *q.After
	}
	if p.OperationId != q.OperationId || p.Items == nil || len(p.Items) > limit {
		return errMigrationShape
	}
	for _, event := range p.Items {
		if err := MigrationLogEvent(event); err != nil {
			return err
		}
		if event.Seq <= after || event.Code == "LOG_TRUNCATED" && !p.Truncated {
			return errMigrationShape
		}
		after = event.Seq
	}
	if p.NextAfter != nil && (len(p.Items) == 0 || *p.NextAfter != after) {
		return errMigrationShape
	}
	// Page metadata is not charged to the persisted event-array budget.
	return historyWireSize(p.Items, MaxMigrationLogBytes)
}

// MigrationLogSnapshot validates the retained event array before a writer
// commits it or a reader paginates it. A truncated tail can leave a gap
// before the terminal event.
func MigrationLogSnapshot(items []t.MigrationTypesLogEvent, truncated bool) error {
	if items == nil || len(items) > MaxMigrationEvents {
		return errMigrationShape
	}
	previous := 0
	for i, e := range items {
		if err := MigrationLogEvent(e); err != nil {
			return err
		}
		if e.Code == "LOG_TRUNCATED" && !truncated {
			return errMigrationShape
		}
		if e.Seq <= previous || (i == 0 || !truncated) && e.Seq != previous+1 {
			return errMigrationShape
		}
		previous = e.Seq
	}
	return historyWireSize(items, MaxMigrationLogBytes)
}

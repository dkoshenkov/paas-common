package validation

import t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"

// LegacyComponentRecord upgrades a stored legacy Operation after validating
// the actual legacy payload, including its original wire-size boundary.
func LegacyComponentRecord(old t.TypesOperation) (t.MigrationTypesOperationRecord, error) {
	if err := HistoryOperation(old); err != nil {
		return t.MigrationTypesOperationRecord{}, err
	}
	out := t.MigrationTypesOperationRecord{OperationId: old.OperationId, AppId: old.AppId, ComponentId: &old.ComponentId, TargetType: "COMPONENT", State: old.State, Stage: old.Stage, Diagnostics: old.Diagnostics, CreatedAt: old.CreatedAt, UpdatedAt: old.UpdatedAt, FinishedAt: old.FinishedAt}
	return out, nil
}

func operationFields(r t.MigrationTypesOperationRecord) t.TypesOperation {
	op := t.TypesOperation{OperationId: r.OperationId, AppId: r.AppId, State: r.State, Stage: r.Stage, Diagnostics: r.Diagnostics, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, FinishedAt: r.FinishedAt}
	if r.ComponentId != nil {
		op.ComponentId = *r.ComponentId
	}
	return op
}

// OperationRecord validates both target shapes without inventing a component
// identity for app-wide migrations.
func OperationRecord(r t.MigrationTypesOperationRecord) error {
	if err := recordFields(r); err != nil {
		return err
	}
	return historyWireSize(r, MaxHistoryOperationBytes)
}

// ReadableOperationRecord accepts the unchanged legacy payload plus the
// 26 bytes of its COMPONENT discriminator. Only the store can establish
// legacy provenance; new persisted records must pass OperationRecord.
func ReadableOperationRecord(r t.MigrationTypesOperationRecord) error {
	if err := recordFields(r); err != nil {
		return err
	}
	if r.TargetType == "COMPONENT" && r.Migration == nil {
		return historyWireSize(operationFields(r), MaxHistoryOperationBytes)
	}
	return historyWireSize(r, MaxHistoryOperationBytes)
}

func recordTarget(r t.MigrationTypesOperationRecord) error {
	switch r.TargetType {
	case "APP":
		if r.ComponentId != nil || (r.Migration == nil) == (r.DatabaseAction == nil) || (r.Migration != nil && r.Migration.Reason != "MANUAL") {
			return errMigrationShape
		}
	case "COMPONENT":
		if r.ComponentId == nil || !catalogText(*r.ComponentId, 256) || r.DatabaseAction != nil {
			return errMigrationShape
		}
		if r.Migration != nil && r.Migration.Reason != "INITIAL" && r.Migration.Reason != "REQUESTED" {
			return errMigrationShape
		}
	default:
		return errMigrationShape
	}
	return nil
}

func recordFields(r t.MigrationTypesOperationRecord) error {
	if err := recordTarget(r); err != nil {
		return err
	}
	if r.Migration != nil {
		if !catalogText(r.Migration.Revision, settingsTokenBytes) {
			return errMigrationShape
		}
		if err := migrator(r.Migration.Image); err != nil {
			return err
		}
	}
	if a := r.DatabaseAction; a != nil {
		switch a.Action {
		case "REVOKE_ACCESS":
			if a.Purpose == nil || (*a.Purpose != ApplicationPurpose && *a.Purpose != MigratorPurpose) {
				return errMigrationShape
			}
		case "DELETE":
			if a.Purpose != nil {
				return errMigrationShape
			}
		default:
			return errMigrationShape
		}
	}
	op := operationFields(r)
	if !HistoryOperationID(op.OperationId) || !catalogText(op.AppId, 256) {
		return errHistoryBounds
	}
	return historyFields(op)
}

// ComponentProjection exposes the old wire shape only for component targets.
func ComponentProjection(r t.MigrationTypesOperationRecord) (t.TypesOperation, bool, error) {
	if err := ReadableOperationRecord(r); err != nil {
		return t.TypesOperation{}, false, err
	}
	if r.TargetType != "COMPONENT" {
		return t.TypesOperation{}, false, nil
	}
	return operationFields(r), true, nil
}

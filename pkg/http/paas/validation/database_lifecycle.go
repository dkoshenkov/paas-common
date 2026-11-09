package validation

import (
	"errors"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

var errDatabaseLifecycle = errors.New("invalid database lifecycle value")

func DatabaseLifecycleQuery(q t.DatabaseLifecycleDatabaseLifecycleGetQuery) error {
	if !catalogID(q.AppId) {
		return errDatabaseLifecycle
	}
	return nil
}

func RevokeDatabaseAccessRequest(q t.DatabaseLifecycleRevokeDatabaseAccessRequest) error {
	if appRevision(q.AppId, q.ExpectedRevision) != nil || !catalogText(q.RequestId, settingsTokenBytes) || q.Purpose != ApplicationPurpose && q.Purpose != MigratorPurpose {
		return errDatabaseLifecycle
	}
	return nil
}

func DeleteDatabaseRequest(q t.DatabaseLifecycleDeleteDatabaseRequest) error {
	expected, err := ApplicationDatabaseName(q.AppId)
	if err != nil || appRevision(q.AppId, q.ExpectedRevision) != nil || !catalogText(q.RequestId, settingsTokenBytes) || q.ConfirmDatabaseName != expected {
		return errDatabaseLifecycle
	}
	return nil
}

func DatabaseLifecycleSnapshot(s t.DatabaseLifecycleDatabaseLifecycleSnapshot) error {
	expected, err := ApplicationDatabaseName(s.AppId)
	if err != nil || expected != s.DatabaseName || appRevision(s.AppId, s.Revision) != nil || s.ActiveOperationId != nil && !HistoryOperationID(*s.ActiveOperationId) {
		return errDatabaseLifecycle
	}
	switch s.DatabaseState {
	case "PENDING", "READY", "DELETING", "DELETED", "RECOVERY_REQUIRED":
	default:
		return errDatabaseLifecycle
	}
	for _, access := range []string{s.ApplicationAccess, s.MigratorAccess} {
		switch access {
		case "ACTIVE", "REVOKING", "REVOKED", "REMOVED":
		default:
			return errDatabaseLifecycle
		}
	}
	return nil
}

func DatabaseLifecycleOperation(s t.DatabaseLifecycleDatabaseLifecycleOperation) error {
	if !HistoryOperationID(s.OperationId) {
		return errDatabaseLifecycle
	}
	return nil
}

package validation

import (
	"testing"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

func TestDatabaseLifecycleAndOperationShape(tst *testing.T) {
	db, _ := ApplicationDatabaseName("orders")
	snapshot := t.DatabaseLifecycleDatabaseLifecycleSnapshot{AppId: "orders", DatabaseName: db, Revision: "r1", DatabaseState: "READY", ApplicationAccess: "ACTIVE", MigratorAccess: "ACTIVE"}
	if err := DatabaseLifecycleSnapshot(snapshot); err != nil {
		tst.Fatal(err)
	}
	q := t.DatabaseLifecycleDeleteDatabaseRequest{AppId: "orders", ExpectedRevision: "r1", RequestId: "request", ConfirmDatabaseName: db}
	if err := DeleteDatabaseRequest(q); err != nil {
		tst.Fatal(err)
	}
	q.ConfirmDatabaseName = "other"
	if DeleteDatabaseRequest(q) == nil {
		tst.Fatal("accepted wrong database name")
	}
	now := "2026-11-08T00:00:00Z"
	purpose := "APPLICATION"
	op := t.MigrationTypesOperationRecord{OperationId: "op", AppId: "orders", TargetType: "APP", State: "ACCEPTED", CreatedAt: now, UpdatedAt: now, Diagnostics: []t.TypesDiagnostic{}, DatabaseAction: &t.MigrationTypesDatabaseAction{Action: "REVOKE_ACCESS", Purpose: &purpose}}
	if err := OperationRecord(op); err != nil {
		tst.Fatal(err)
	}
	if _, ok, err := ComponentProjection(op); err != nil || ok {
		tst.Fatalf("component projection: %v %v", ok, err)
	}
	op.DatabaseAction.Purpose = nil
	if OperationRecord(op) == nil {
		tst.Fatal("accepted revoke without purpose")
	}
	op.DatabaseAction.Action = "DELETE"
	if err := OperationRecord(op); err != nil {
		tst.Fatal(err)
	}
	op.DatabaseAction.Purpose = &purpose
	if OperationRecord(op) == nil {
		tst.Fatal("accepted delete with purpose")
	}
	op.DatabaseAction.Purpose = nil
	op.Migration = &t.MigrationTypesMigration{}
	if OperationRecord(op) == nil {
		tst.Fatal("accepted two actions")
	}
}

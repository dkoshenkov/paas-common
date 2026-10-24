package validation

import (
	"fmt"

	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

// OperationListQuery checks the state filter and pagination shape, not cursor scope.
// Callers must explicitly invoke it before reading the store.
func OperationListQuery(q paastypes.OperationsOperationsListQuery) error {
	if q.State != nil {
		switch *q.State {
		case "ACCEPTED", "RUNNING", "SUCCEEDED", "FAILED", "RECOVERY_REQUIRED":
		default:
			return fmt.Errorf("state: unknown filter")
		}
	}
	return catalogQuery(q.Limit, q.Cursor)
}

// OperationPage checks one page of snapshots without changing or sorting it.
// Consumers must additionally enforce their requested state and limit. This does
// not check cursor scope, completeness, transitions, persistence or retention.
func OperationPage(page paastypes.TypesOperationPage) error {
	if page.Items == nil {
		return fmt.Errorf("items: require an array, not null")
	}
	if err := catalogPage(len(page.Items), page.NextCursor); err != nil {
		return err
	}
	for i, op := range page.Items {
		if err := Operation(op); err != nil {
			return fmt.Errorf("items[%d]: %w", i, err)
		}
		if i > 0 && page.Items[i-1].OperationId >= op.OperationId {
			return fmt.Errorf("items[%d]: IDs must be unique and sorted", i)
		}
	}
	return nil
}

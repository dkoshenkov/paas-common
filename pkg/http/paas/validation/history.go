package validation

import (
	"encoding/json"
	"errors"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

const (
	MaxHistoryOperationBytes = 64 << 10
	MaxHistoryPageBytes      = 8 << 20
)

var errHistoryBounds = errors.New("operation history field or response exceeds runtime profile")

func historyText(s string, max int) bool {
	return s == "" || catalogText(s, max)
}

func historyOptional(s *string, max int) bool {
	return s == nil || historyText(*s, max)
}

// HistoryOperationID validates an opaque history lookup or cursor position.
func HistoryOperationID(id string) bool { return catalogText(id, 96) }

func historyIdentity(op t.TypesOperation) bool {
	return HistoryOperationID(op.OperationId) && catalogText(op.AppId, 256) && catalogText(op.ComponentId, 256)
}

func historyTimes(op t.TypesOperation) bool {
	return len(op.CreatedAt) <= 40 && len(op.UpdatedAt) <= 40 && (op.FinishedAt == nil || len(*op.FinishedAt) <= 40)
}

func historyDiagnostic(d t.TypesDiagnostic) bool {
	return catalogText(d.Code, 64) && catalogText(d.Message, 1024) && historyOptional(d.File, MaxFileNameBytes) && historyOptional(d.FieldPath, 512)
}

// HistoryOperation validates the stored DTO and its complete public JSON encoding.
func HistoryOperation(op t.TypesOperation) error {
	if !historyIdentity(op) {
		return errHistoryBounds
	}
	if err := historyFields(op); err != nil {
		return err
	}
	return historyWireSize(op, MaxHistoryOperationBytes)
}

func historyFields(op t.TypesOperation) error {
	if len(op.Diagnostics) > 32 || !historyTimes(op) || !historyOptional(op.Stage, 64) {
		return errHistoryBounds
	}
	for _, d := range op.Diagnostics {
		if !historyDiagnostic(d) {
			return errHistoryBounds
		}
	}
	return operationState(op)
}

// HistoryPageSize checks the complete encoded page before committing a response.
// The reader validates each item, including its lookahead, before assembly.
func HistoryPageSize(page t.TypesOperationPage) error {
	return historyWireSize(page, MaxHistoryPageBytes)
}

func historyWireSize(value any, limit int) error {
	data, err := json.Marshal(value)
	if err != nil || len(data) > limit {
		return errHistoryBounds
	}
	return nil
}

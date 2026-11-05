package validation

import (
	"encoding/json"
	"fmt"
	"unicode"
	"unicode/utf8"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

const (
	MaxOperationHistoryBytes     = 64 << 10
	MaxOperationHistoryPageBytes = 8 << 20
)

// OperationHistoryID validates the bounded opaque identifier used by history.
func OperationHistoryID(id string) error { return historyText(id, 96, false) }

func historyText(s string, max int, empty bool) error {
	if (!empty && s == "") || len(s) > max || !utf8.ValidString(s) {
		return fmt.Errorf("invalid history text")
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return fmt.Errorf("invalid history text")
		}
	}
	return nil
}

// OperationHistory explicitly adds the read-only history profile to Operation.
// It neither changes the older validator nor recognizes secrets in free text.
func OperationHistory(op t.TypesOperation) error {
	if err := Operation(op); err != nil {
		return err
	}
	if err := OperationHistoryID(op.OperationId); err != nil {
		return err
	}
	for _, f := range []struct {
		value string
		max   int
	}{
		{op.AppId, 256}, {op.ComponentId, 256}, {op.CreatedAt, 40}, {op.UpdatedAt, 40},
	} {
		if err := historyText(f.value, f.max, false); err != nil {
			return err
		}
	}
	for _, f := range []struct {
		value *string
		max   int
	}{{op.Stage, 64}, {op.FinishedAt, 40}} {
		if f.value != nil {
			if err := historyText(*f.value, f.max, false); err != nil {
				return err
			}
		}
	}
	if len(op.Diagnostics) > 32 {
		return fmt.Errorf("too many history diagnostics")
	}
	for _, d := range op.Diagnostics {
		if err := historyText(d.Code, 64, false); err != nil {
			return err
		}
		if err := historyText(d.Message, 1024, false); err != nil {
			return err
		}
		if d.File != nil {
			if err := historyText(*d.File, 256, false); err != nil {
				return err
			}
		}
		if d.FieldPath != nil {
			if err := historyText(*d.FieldPath, 512, true); err != nil {
				return err
			}
		}
	}
	return historyJSON(op, MaxOperationHistoryBytes)
}

// OperationHistoryPage validates the complete response before successful headers.
func OperationHistoryPage(page t.TypesOperationPage) error {
	if err := OperationPage(page); err != nil {
		return err
	}
	for _, op := range page.Items {
		if err := OperationHistory(op); err != nil {
			return err
		}
	}
	return historyJSON(page, MaxOperationHistoryPageBytes)
}

func historyJSON(value any, limit int) error {
	data, err := json.Marshal(value)
	if err != nil || len(data) > limit {
		return fmt.Errorf("invalid history JSON size")
	}
	return nil
}

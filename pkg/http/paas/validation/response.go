package validation

import (
	"fmt"
	"strings"
	"time"

	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

// PreviewResponse checks the two permitted response shapes and public plan.
// It does not determine expiry, compare plans or establish durable acceptance.
func PreviewResponse(response paastypes.TypesPreviewResponse) error {
	if err := diagnostics(response.Diagnostics); err != nil {
		return err
	}
	hasError := false
	for _, d := range response.Diagnostics {
		hasError = hasError || d.Severity == "ERROR"
	}
	if hasError {
		if response.Plan != nil || response.PreviewId != nil || response.ExpiresAt != nil {
			return fmt.Errorf("preview: ERROR diagnostics forbid plan, preview_id and expires_at")
		}
		return nil
	}
	if response.Plan == nil || response.PreviewId == nil || *response.PreviewId == "" || response.ExpiresAt == nil {
		return fmt.Errorf("preview: success requires plan, nonempty preview_id and expires_at")
	}
	if err := timestamp(*response.ExpiresAt); err != nil {
		return fmt.Errorf("expires_at: %w", err)
	}
	return Plan(*response.Plan)
}

// Operation checks a state snapshot, not transitions, readiness or safe lock release.
func Operation(op paastypes.TypesOperation) error {
	if op.OperationId == "" || op.AppId == "" || op.ComponentId == "" {
		return fmt.Errorf("operation identity: required nonempty IDs")
	}
	terminal := false
	switch op.State {
	case "ACCEPTED", "RUNNING", "RECOVERY_REQUIRED":
	case "SUCCEEDED", "FAILED":
		terminal = true
	default:
		return fmt.Errorf("state: unknown value")
	}
	if op.Stage != nil && *op.Stage == "" {
		return fmt.Errorf("stage: require a nonempty code when present")
	}
	if terminal != (op.FinishedAt != nil) {
		return fmt.Errorf("finished_at: required only and always in terminal states")
	}
	for _, field := range []struct{ name, value string }{{"created_at", op.CreatedAt}, {"updated_at", op.UpdatedAt}} {
		if err := timestamp(field.value); err != nil {
			return fmt.Errorf("%s: %w", field.name, err)
		}
	}
	if op.FinishedAt != nil {
		if err := timestamp(*op.FinishedAt); err != nil {
			return fmt.Errorf("finished_at: %w", err)
		}
	}
	if err := diagnostics(op.Diagnostics); err != nil {
		return err
	}
	for _, d := range op.Diagnostics {
		if d.Code == "OUTCOME_UNKNOWN" && op.State != "RECOVERY_REQUIRED" {
			return fmt.Errorf("state: OUTCOME_UNKNOWN requires RECOVERY_REQUIRED")
		}
	}
	if op.State == "FAILED" || op.State == "RECOVERY_REQUIRED" {
		if len(op.Diagnostics) == 0 {
			return fmt.Errorf("diagnostics: require an explanation for failure or recovery")
		}
	}
	return nil
}

// ErrorResponse checks the current error shape and COMPONENT_BUSY relationship.
// It does not classify arbitrary errors, validate HTTP status or redact messages.
func ErrorResponse(response paastypes.TypesErrorResponse) error {
	if response.Code == "" || response.Message == "" {
		return fmt.Errorf("code and message: required nonempty strings")
	}
	if err := diagnostics(response.Diagnostics); err != nil {
		return err
	}
	if response.ConflictingOperationId != nil && *response.ConflictingOperationId == "" {
		return fmt.Errorf("conflicting_operation_id: require a nonempty ID when present")
	}
	if response.Code == "COMPONENT_BUSY" {
		if response.ConflictingOperationId == nil {
			return fmt.Errorf("conflicting_operation_id: required for COMPONENT_BUSY")
		}
	} else if response.Code == "PLAN_CHANGED" && response.ConflictingOperationId != nil {
		return fmt.Errorf("conflicting_operation_id: PLAN_CHANGED does not describe a blocking operation")
	}
	// The code catalog is open. Unknown codes do not establish whether a blocker exists.
	return nil
}

func timestamp(value string) error {
	if len(value) < 20 || !strings.HasSuffix(value, "Z") || value[10] != 'T' || value[13] != ':' || value[16] != ':' {
		return fmt.Errorf("require RFC3339 UTC with Z")
	}
	if len(value) > 20 {
		if value[19] != '.' || len(value) == 21 {
			return fmt.Errorf("require RFC3339 UTC with Z")
		}
		for _, digit := range value[20 : len(value)-1] {
			if digit < '0' || digit > '9' {
				return fmt.Errorf("require RFC3339 UTC with Z")
			}
		}
	}
	if _, err := time.Parse(time.RFC3339Nano, value); err != nil {
		return fmt.Errorf("require RFC3339 UTC with Z")
	}
	return nil
}

package validation

import (
	"fmt"
	"slices"
	"strings"

	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

// Plan checks the public plan's identities, ordering, actions and field pointers.
// It does not compare internal plans, infer ownership or validate Kubernetes objects.
func Plan(plan paastypes.TypesPlan) error {
	if plan.Resources == nil || plan.AdditionalActions == nil {
		return fmt.Errorf("plan: resources and additional_actions must be arrays, not null")
	}
	var previous [4]string
	for i, resource := range plan.Resources {
		identity, err := resourceIdentity(resource)
		if err != nil {
			return fmt.Errorf("resources[%d]: %w", i, err)
		}
		if i > 0 && slices.Compare(previous[:], identity[:]) >= 0 {
			return fmt.Errorf("resources[%d]: identities must be unique and sorted by group/kind/namespace/name", i)
		}
		previous = identity
		if err := changedFields(resource); err != nil {
			return fmt.Errorf("resources[%d]: %w", i, err)
		}
	}
	targets := make(map[string]bool, len(plan.AdditionalActions))
	for i, action := range plan.AdditionalActions {
		if action.Kind != "INITIAL_MIGRATION" || action.AppId == "" {
			return fmt.Errorf("additional_actions[%d]: require INITIAL_MIGRATION and a nonempty app_id", i)
		}
		if targets[action.AppId] {
			return fmt.Errorf("additional_actions[%d]: duplicate action for the target", i)
		}
		targets[action.AppId] = true
	}
	return nil
}

func resourceIdentity(resource paastypes.TypesResourceChange) ([4]string, error) {
	if resource.ApiVersion == "" || resource.Kind == "" || resource.Namespace == "" || resource.Name == "" {
		return [4]string{}, fmt.Errorf("resource identity fields must be nonempty")
	}
	group, version, qualified := strings.Cut(resource.ApiVersion, "/")
	if qualified {
		if group == "" || version == "" || strings.Contains(version, "/") {
			return [4]string{}, fmt.Errorf("api_version: invalid group/version")
		}
	} else {
		group = ""
	}
	return [4]string{group, resource.Kind, resource.Namespace, resource.Name}, nil
}

func changedFields(resource paastypes.TypesResourceChange) error {
	fields := resource.ChangedFields
	if fields == nil {
		return fmt.Errorf("changed_fields: require an array, not null")
	}
	switch resource.Action {
	case "CREATE":
		if len(fields) != 1 || fields[0] != "" {
			return fmt.Errorf("CREATE: changed_fields must contain only the root pointer")
		}
	case "UPDATE":
		if len(fields) == 0 {
			return fmt.Errorf("UPDATE: changed_fields must be nonempty")
		}
	case "NO_CHANGE":
		if len(fields) != 0 {
			return fmt.Errorf("NO_CHANGE: changed_fields must be empty")
		}
	default:
		return fmt.Errorf("action: unknown value")
	}
	for i, field := range fields {
		if !jsonPointer(field) {
			return fmt.Errorf("changed_fields[%d]: invalid JSON Pointer", i)
		}
		if i > 0 && fields[i-1] >= field {
			return fmt.Errorf("changed_fields[%d]: pointers must be unique and sorted", i)
		}
	}
	return nil
}

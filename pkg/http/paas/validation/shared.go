package validation

import (
	"fmt"
	"strings"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

const MaxSharedServicesLimit = 100

func SharedListQuery(q t.SharedSharedListQuery) error {
	if q.ConsumerAppId != nil && !catalogID(*q.ConsumerAppId) {
		return fmt.Errorf("consumer_app_id: invalid ID")
	}
	if q.Limit != nil && (*q.Limit < 1 || *q.Limit > MaxSharedServicesLimit) {
		return fmt.Errorf("limit: require 1 through 100")
	}
	return catalogCursor(q.Cursor)
}

func RegisterSharedServiceRequest(q t.SharedTypesRegisterSharedServiceRequest) error {
	if !catalogText(q.RequestId, settingsTokenBytes) || !catalogID(q.OwnerAppId) || !catalogID(q.ComponentId) || !serviceName(q.ServiceName) {
		return fmt.Errorf("invalid shared service registration")
	}
	return nil
}

func SetSharedServiceAccessRequest(q t.SharedTypesSetSharedServiceAccessRequest) error {
	if !catalogID(q.SharedServiceId) || !catalogID(q.ConsumerAppId) || !catalogText(q.ExpectedRevision, settingsTokenBytes) {
		return fmt.Errorf("invalid shared service access target or revision")
	}
	return nil
}

func SharedService(s t.SharedTypesSharedService) error {
	if !catalogID(s.SharedServiceId) || !catalogID(s.OwnerAppId) || !catalogID(s.ComponentId) || !serviceName(s.ServiceName) {
		return fmt.Errorf("invalid shared service identity")
	}
	parts := strings.Split(s.Address, ".")
	if len(parts) != 3 || parts[0] != s.ServiceName || !serviceName(parts[1]) || parts[2] != "svc" || !catalogText(s.Revision, settingsTokenBytes) {
		return fmt.Errorf("invalid service address or revision")
	}
	return nil
}

func SharedServicePage(p t.SharedTypesSharedServicePage) error {
	if p.Items == nil {
		return fmt.Errorf("items: require an array, not null")
	}
	if len(p.Items) > MaxSharedServicesLimit {
		return fmt.Errorf("items: maximum 100 entries")
	}
	if len(p.Items) == 0 && p.NextCursor != nil {
		return fmt.Errorf("next_cursor: forbidden for an empty page")
	}
	if err := catalogCursor(p.NextCursor); err != nil {
		return err
	}
	for i, item := range p.Items {
		if err := SharedService(item); err != nil {
			return fmt.Errorf("items[%d]: %w", i, err)
		}
		if i > 0 && p.Items[i-1].SharedServiceId >= item.SharedServiceId {
			return fmt.Errorf("items[%d]: services must be unique and sorted by ID", i)
		}
	}
	return nil
}

func SharedServiceAccess(s t.SharedTypesSharedServiceAccess) error {
	if !catalogID(s.SharedServiceId) || !catalogID(s.ConsumerAppId) || !catalogText(s.Revision, settingsTokenBytes) {
		return fmt.Errorf("invalid shared service access result")
	}
	return nil
}

func serviceName(name string) bool {
	if len(name) == 0 || len(name) > 63 || strings.ToLower(name) != name || name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}
	for i := range len(name) {
		c := name[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

func SharedAccessQuery(q t.SharedSharedAccessQuery) error {
	if !catalogID(q.SharedServiceId) || !catalogID(q.ConsumerAppId) {
		return fmt.Errorf("invalid shared service access target")
	}
	return nil
}

func SharedServiceAccessStatus(s t.SharedTypesSharedServiceAccessStatus) error {
	if !catalogID(s.SharedServiceId) || !catalogID(s.ConsumerAppId) || !catalogText(s.DesiredRevision, settingsTokenBytes) || (s.AppliedAllow == nil) != (s.AppliedRevision == nil) {
		return fmt.Errorf("invalid shared access status")
	}
	if s.AppliedRevision != nil && !catalogText(*s.AppliedRevision, settingsTokenBytes) {
		return fmt.Errorf("invalid applied revision")
	}
	switch s.State {
	case "PENDING":
		if s.BlockedReason != nil {
			return fmt.Errorf("unexpected blocked reason")
		}
	case "APPLIED":
		if s.BlockedReason != nil || s.AppliedRevision == nil || *s.AppliedRevision != s.DesiredRevision || *s.AppliedAllow != s.DesiredAllow {
			return fmt.Errorf("invalid applied state")
		}
	case "BLOCKED":
		if s.BlockedReason == nil || len(*s.BlockedReason) == 0 || len(*s.BlockedReason) > 64 {
			return fmt.Errorf("invalid blocked reason")
		}
		for _, c := range *s.BlockedReason {
			if (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
				return fmt.Errorf("invalid blocked reason")
			}
		}
	default:
		return fmt.Errorf("invalid state")
	}
	return nil
}

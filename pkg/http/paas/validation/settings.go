package validation

import (
	"errors"

	"github.com/distribution/reference"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"

	_ "crypto/sha256" // Register the digest required by the OCI reference parser.
)

// ErrEgressAllowlistUnavailable identifies a well-formed request for a mode
// that the first runtime cannot enforce. The service returns HTTP 409 and
// keeps both the previous intent and its revision.
var ErrEgressAllowlistUnavailable = errors.New("EGRESS_ALLOWLIST_UNAVAILABLE")

const settingsTokenBytes = 96

// MaxSettingsBytes bounds each POST body and each persisted component policy.
const MaxSettingsBytes = 1 << 20

func CreateAppRequest(q t.SettingsTypesCreateAppRequest) error {
	if !catalogText(q.RequestId, settingsTokenBytes) || !catalogText(q.Name, MaxCatalogNameBytes) {
		return errors.New("invalid registration request_id or name")
	}
	return nil
}

func CreateComponentRequest(q t.SettingsTypesCreateComponentRequest) error {
	if !catalogID(q.AppId) {
		return errors.New("invalid app_id")
	}
	return CreateAppRequest(t.SettingsTypesCreateAppRequest{RequestId: q.RequestId, Name: q.Name})
}

func AppMigratorQuery(q t.SettingsSettingsAppMigratorQuery) error {
	if !catalogID(q.AppId) {
		return errors.New("invalid app_id")
	}
	return nil
}

func ComponentPolicyQuery(q t.SettingsSettingsComponentPolicyQuery) error {
	if !catalogID(q.AppId) || !catalogID(q.ComponentId) {
		return errors.New("invalid component target")
	}
	return nil
}

func appRevision(id, revision string) error {
	if !catalogText(revision, settingsTokenBytes) {
		return errors.New("invalid settings revision")
	}
	return AppMigratorQuery(t.SettingsSettingsAppMigratorQuery{AppId: id})
}

func migrator(image string) error {
	ref, err := reference.Parse(image)
	canonical, ok := ref.(reference.Canonical)
	if err != nil || !ok || canonical.Digest().Algorithm() != "sha256" {
		return errors.New("migrator image requires a named OCI reference with a SHA-256 digest")
	}
	return nil
}

func SetMigratorRequest(q t.SettingsTypesSetMigratorRequest) error {
	if err := appRevision(q.AppId, q.ExpectedRevision); err != nil {
		return err
	}
	return migrator(q.Migrator.Image)
}

func RemoveMigratorRequest(q t.SettingsTypesRemoveMigratorRequest) error {
	return appRevision(q.AppId, q.ExpectedRevision)
}

func MigratorSnapshot(s t.SettingsTypesMigratorSnapshot) error {
	if err := appRevision(s.AppId, s.Revision); err != nil {
		return err
	}
	if s.Migrator != nil {
		return migrator(s.Migrator.Image)
	}
	return nil
}

func ComponentPolicySnapshot(s t.SettingsTypesComponentPolicySnapshot) error {
	if err := ComponentPolicyQuery(t.SettingsSettingsComponentPolicyQuery{AppId: s.AppId, ComponentId: s.ComponentId}); err != nil {
		return err
	}
	if !catalogText(s.Revision, settingsTokenBytes) {
		return errors.New("invalid settings revision")
	}
	return componentPolicy(s.Policy)
}

func SetPolicyRequest(q t.SettingsTypesSetPolicyRequest) error {
	if err := ComponentPolicySnapshot(t.SettingsTypesComponentPolicySnapshot{
		AppId: q.AppId, ComponentId: q.ComponentId, Revision: q.ExpectedRevision, Policy: q.Policy,
	}); err != nil {
		return err
	}
	if q.Policy.External.Mode == "allowlist" {
		return ErrEgressAllowlistUnavailable
	}
	return nil
}

// ComponentPolicyObservedSnapshot checks the optional network observation independently of desired policy.
func ComponentPolicyObservedSnapshot(s t.SettingsTypesComponentPolicyObservedSnapshot) error {
	if err := ComponentPolicySnapshot(t.SettingsTypesComponentPolicySnapshot{AppId: s.AppId, ComponentId: s.ComponentId, Revision: s.Revision, Policy: s.Policy}); err != nil {
		return err
	}
	if s.NetworkStatus == nil {
		return nil
	}
	n := s.NetworkStatus
	if !catalogText(n.DesiredRevision, settingsTokenBytes) || n.AppliedRevision != nil && !catalogText(*n.AppliedRevision, settingsTokenBytes) || n.Reason != nil && !networkReason(*n.Reason) {
		return errors.New("invalid component network status")
	}
	switch n.State {
	case "PENDING", "BLOCKED":
	case "APPLIED":
		if n.AppliedRevision == nil || *n.AppliedRevision != n.DesiredRevision {
			return errors.New("invalid applied component network revision")
		}
	default:
		return errors.New("invalid component network state")
	}
	return nil
}

func networkReason(reason string) bool {
	if len(reason) == 0 || len(reason) > 64 {
		return false
	}
	for _, c := range reason {
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

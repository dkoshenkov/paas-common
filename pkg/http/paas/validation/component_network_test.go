package validation_test

import (
	"testing"

	t "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	v "github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

func TestObservedComponentNetworkRevision(tst *testing.T) {
	s := t.SettingsTypesComponentPolicyObservedSnapshot{AppId: "app", ComponentId: "component", Revision: "revision", Policy: t.SettingsTypesComponentPolicy{DatabaseGrants: []t.SettingsTypesDatabaseGrant{}, ServiceGrants: []t.SettingsTypesServiceGrant{}, External: t.SettingsTypesExternal{Mode: "none", Domains: []string{}}}}
	if err := v.ComponentPolicyObservedSnapshot(s); err != nil {
		tst.Fatal(err)
	}
	old := "old"
	s.NetworkStatus = &t.SettingsTypesComponentNetworkStatus{DesiredRevision: "new", AppliedRevision: &old, State: "PENDING"}
	if err := v.ComponentPolicyObservedSnapshot(s); err != nil {
		tst.Fatal(err)
	}
	s.NetworkStatus.State = "APPLIED"
	if err := v.ComponentPolicyObservedSnapshot(s); err == nil {
		tst.Fatal("accepted stale applied revision")
	}
	s.NetworkStatus.AppliedRevision = &s.NetworkStatus.DesiredRevision
	if err := v.ComponentPolicyObservedSnapshot(s); err != nil {
		tst.Fatal(err)
	}
	reason := "private token message"
	s.NetworkStatus.Reason = &reason
	if err := v.ComponentPolicyObservedSnapshot(s); err == nil {
		tst.Fatal("accepted unsafe reason")
	}
	s.NetworkStatus.Reason = nil
	s.NetworkStatus.State = "UNKNOWN"
	if err := v.ComponentPolicyObservedSnapshot(s); err == nil {
		tst.Fatal("accepted unknown state")
	}
}

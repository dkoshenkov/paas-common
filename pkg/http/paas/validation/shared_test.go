package validation

import (
	"testing"

	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

func TestSharedValidation(tst *testing.T) {
	validService := paastypes.SharedTypesSharedService{SharedServiceId: "shared-1", OwnerAppId: "owner", ComponentId: "api", ServiceName: "redis", Address: "redis.paas-app-owner.svc", Revision: "r0"}
	for _, tc := range []struct {
		name  string
		check func() error
		valid bool
	}{
		{"list defaults", func() error { return SharedListQuery(paastypes.SharedSharedListQuery{}) }, true},
		{"list limit", func() error { return SharedListQuery(paastypes.SharedSharedListQuery{Limit: ptr(100)}) }, true},
		{"list invalid limit", func() error { return SharedListQuery(paastypes.SharedSharedListQuery{Limit: ptr(101)}) }, false},
		{"register", func() error {
			return RegisterSharedServiceRequest(paastypes.SharedTypesRegisterSharedServiceRequest{RequestId: "request", OwnerAppId: "owner", ComponentId: "api", ServiceName: "redis"})
		}, true},
		{"registration invalid service", func() error {
			return RegisterSharedServiceRequest(paastypes.SharedTypesRegisterSharedServiceRequest{RequestId: "request", OwnerAppId: "owner", ComponentId: "api", ServiceName: "Redis"})
		}, false},
		{"set access", func() error {
			return SetSharedServiceAccessRequest(paastypes.SharedTypesSetSharedServiceAccessRequest{SharedServiceId: "shared-1", ConsumerAppId: "consumer", ExpectedRevision: "r0"})
		}, true},
		{"service address", func() error { return SharedService(validService) }, true},
		{"address credentials rejected", func() error {
			s := validService
			s.Address = "http://user:password@redis.owner.svc"
			return SharedService(s)
		}, false},
		{"page sorted", func() error {
			return SharedServicePage(paastypes.SharedTypesSharedServicePage{Items: []paastypes.SharedTypesSharedService{validService}})
		}, true},
	} {
		tst.Run(tc.name, func(tst *testing.T) {
			err := tc.check()
			if (err == nil) != tc.valid {
				tst.Fatalf("error = %v, valid = %v", err, tc.valid)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestSharedAccessStatusValidation(t *testing.T) {
	allow := true
	revision := "r1"
	s := paastypes.SharedTypesSharedServiceAccessStatus{SharedServiceId: "shared-1", ConsumerAppId: "consumer", DesiredAllow: true, DesiredRevision: revision, AppliedAllow: &allow, AppliedRevision: &revision, State: "APPLIED"}
	if err := SharedServiceAccessStatus(s); err != nil {
		t.Fatal(err)
	}
	s.AppliedAllow = nil
	if SharedServiceAccessStatus(s) == nil {
		t.Fatal("accepted incomplete applied state")
	}
	s.AppliedAllow = &allow
	reason := "unsafe reason\n"
	s.State, s.BlockedReason = "BLOCKED", &reason
	if SharedServiceAccessStatus(s) == nil {
		t.Fatal("accepted unsafe reason")
	}
}

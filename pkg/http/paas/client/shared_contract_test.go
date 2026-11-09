package client_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/dkoshenkov/paas-common/pkg/http/paas/client"
	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

func TestGeneratedSharedClientRoutesAndBodies(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /shared-services/list", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("consumer_app_id") != "consumer" || r.URL.Query().Get("limit") != "25" || r.URL.Query().Get("cursor") != "next" {
			t.Errorf("query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(paastypes.SharedTypesSharedServicePage{Items: []paastypes.SharedTypesSharedService{}})
	})
	mux.HandleFunc("POST /shared-services/register", func(w http.ResponseWriter, r *http.Request) {
		var got paastypes.SharedTypesRegisterSharedServiceRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		want := paastypes.SharedTypesRegisterSharedServiceRequest{RequestId: "request", OwnerAppId: "owner", ComponentId: "api", ServiceName: "redis"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("register body = %#v", got)
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(paastypes.SharedTypesSharedService{SharedServiceId: "shared-1", OwnerAppId: "owner", ComponentId: "api", ServiceName: "redis", Address: "redis.paas-app-owner.svc", Revision: "r0"})
	})
	mux.HandleFunc("POST /shared-services/set-access", func(w http.ResponseWriter, r *http.Request) {
		var got paastypes.SharedTypesSetSharedServiceAccessRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		want := paastypes.SharedTypesSetSharedServiceAccessRequest{SharedServiceId: "shared-1", ConsumerAppId: "consumer", Allow: true, ExpectedRevision: "r0"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("access body = %#v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(paastypes.SharedTypesSharedServiceAccess{SharedServiceId: "shared-1", ConsumerAppId: "consumer", Allow: true, Revision: "r1"})
	})
	c, err := client.NewSharedClient("http://paas.test", &http.Client{Transport: handlerTransport{handler: mux}})
	if err != nil {
		t.Fatal(err)
	}
	list, err := c.List(t.Context(), paastypes.SharedSharedListQuery{ConsumerAppId: pointer("consumer"), Cursor: pointer("next"), Limit: pointer(25)})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := list.(client.SharedListResponse200); !ok {
		t.Fatalf("list response: %#v", list)
	}
	reg, err := c.Register(t.Context(), paastypes.SharedTypesRegisterSharedServiceRequest{RequestId: "request", OwnerAppId: "owner", ComponentId: "api", ServiceName: "redis"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.(client.SharedRegisterResponse201); !ok {
		t.Fatalf("register response: %#v", reg)
	}
	access, err := c.SetAccess(t.Context(), paastypes.SharedTypesSetSharedServiceAccessRequest{SharedServiceId: "shared-1", ConsumerAppId: "consumer", Allow: true, ExpectedRevision: "r0"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := access.(client.SharedSetAccessResponse200); !ok {
		t.Fatalf("access response: %#v", access)
	}
}

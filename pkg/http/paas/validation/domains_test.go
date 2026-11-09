package validation_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/dkoshenkov/packages-go/middlewarex/httpx"

	"github.com/dkoshenkov/paas-common/pkg/http/paas/client"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/domains"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/transport"
	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

type domainFixture struct{ calls int }

func TestDomainsTLSProfile(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{{"staging", true}, {"production", true}, {"unknown", false}, {"", false}} {
		value := tc.value
		err := validation.DomainsSnapshot(paastypes.DomainsTypesDomainsSnapshot{AppId: "app", Domains: []string{}, TlsProfile: &value})
		if (err == nil) != tc.valid {
			t.Fatalf("%q: %v", value, err)
		}
	}
}

func domainAPIError(code string) paastypes.TypesErrorResponse {
	return paastypes.TypesErrorResponse{Code: code, Message: "invalid request", Diagnostics: []paastypes.TypesDiagnostic{}}
}

func (s *domainFixture) List(_ context.Context, q paastypes.DomainsDomainsListQuery) (domains.DomainsListResponse, error) {
	if err := validation.DomainsListQuery(q); err != nil {
		return domains.DomainsListResponse400{Body: domainAPIError("INVALID_REQUEST")}, nil
	}
	s.calls++
	return domains.DomainsListResponse200{Body: paastypes.DomainsTypesDomainsSnapshot{AppId: q.AppId, Domains: []string{"api.example.com", "www.example.com"}}}, nil
}

func (s *domainFixture) Register(_ context.Context, q paastypes.DomainsTypesDomainRequest) (domains.DomainsRegisterResponse, error) {
	if err := validation.DomainRegisterRequest(q); err != nil {
		return domains.DomainsRegisterResponse400{Body: domainAPIError("INVALID_REQUEST")}, nil
	}
	s.calls++
	canonical, _ := validation.CanonicalDomainName(q.Domain)
	return domains.DomainsRegisterResponse201{Body: paastypes.DomainsTypesDomainSnapshot{AppId: q.AppId, Domain: canonical}}, nil
}

func (s *domainFixture) Remove(_ context.Context, q paastypes.DomainsTypesDomainRequest) (domains.DomainsRemoveResponse, error) {
	if err := validation.DomainRemoveRequest(q); err != nil {
		return domains.DomainsRemoveResponse400{Body: domainAPIError("INVALID_REQUEST")}, nil
	}
	s.calls++
	canonical, _ := validation.CanonicalDomainName(q.Domain)
	return domains.DomainsRemoveResponse200{Body: paastypes.DomainsTypesDomainSnapshot{AppId: q.AppId, Domain: canonical}}, nil
}

func TestDomainsGeneratedClientServerRoundTrip(t *testing.T) {
	s := &domainFixture{}
	mux := http.NewServeMux()
	runtime := httpx.NewRuntime(httpx.WithErrorEncoder(httpx.ErrorEncoderFunc(transport.EncodeError)))
	domains.RegisterDomainsHandlersWithRuntime(mux, s, runtime)
	c, err := client.NewDomainsClient("http://paas.test", &http.Client{Transport: localTransport{handler: mux}})
	if err != nil {
		t.Fatal(err)
	}

	listed, err := c.List(t.Context(), paastypes.DomainsDomainsListQuery{AppId: "app-1"})
	if err != nil {
		t.Fatal(err)
	}
	list, ok := listed.(client.DomainsListResponse200)
	if !ok || validation.DomainsSnapshot(list.Body) != nil || len(list.Body.Domains) != 2 {
		t.Fatalf("list response: %#v", listed)
	}

	registered, err := c.Register(t.Context(), paastypes.DomainsTypesDomainRequest{AppId: "app-1", Domain: "API.Example.COM."})
	if err != nil {
		t.Fatal(err)
	}
	created, ok := registered.(client.DomainsRegisterResponse201)
	if !ok || created.Body.Domain != "api.example.com" {
		t.Fatalf("registration response: %#v", registered)
	}
	registeredIDNA, err := c.Register(t.Context(), paastypes.DomainsTypesDomainRequest{AppId: "app-1", Domain: "xn--bcher-kva.example"})
	if err != nil {
		t.Fatal(err)
	}
	idnaName, ok := registeredIDNA.(client.DomainsRegisterResponse201)
	if !ok || idnaName.Body.Domain != "xn--bcher-kva.example" {
		t.Fatalf("IDNA registration response: %#v", registeredIDNA)
	}

	removed, err := c.Remove(t.Context(), paastypes.DomainsTypesDomainRequest{AppId: "app-1", Domain: "api.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	deleted, ok := removed.(client.DomainsRemoveResponse200)
	if !ok || deleted.Body.Domain != "api.example.com" || s.calls != 4 {
		t.Fatalf("removal response/call count: %#v, %d", removed, s.calls)
	}
}

func TestDomainsGeneratedClientServerRejectsInvalidRequest(t *testing.T) {
	s := &domainFixture{}
	mux := http.NewServeMux()
	runtime := httpx.NewRuntime(httpx.WithErrorEncoder(httpx.ErrorEncoderFunc(transport.EncodeError)))
	domains.RegisterDomainsHandlersWithRuntime(mux, s, runtime)
	c, err := client.NewDomainsClient("http://paas.test", &http.Client{Transport: localTransport{handler: mux}})
	if err != nil {
		t.Fatal(err)
	}
	listed, err := c.List(t.Context(), paastypes.DomainsDomainsListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := listed.(client.DomainsListResponse400); !ok {
		t.Fatalf("expected typed list 400, got %#v", listed)
	}
	for _, input := range []string{"api..example.com", "*.example.com", "api.example.com..", "täst.example", "xn--.example"} {
		response, err := c.Register(t.Context(), paastypes.DomainsTypesDomainRequest{AppId: "app-1", Domain: input})
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := response.(client.DomainsRegisterResponse400); !ok {
			t.Fatalf("%q: expected typed 400, got %#v", input, response)
		}
	}
	tooLarge, err := c.Register(t.Context(), paastypes.DomainsTypesDomainRequest{AppId: "app-1", Domain: strings.Repeat("a", 1<<20)})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tooLarge.(client.DomainsRegisterResponse413); !ok {
		t.Fatalf("expected typed oversized-body 413, got %#v", tooLarge)
	}
	if s.calls != 0 {
		t.Fatalf("invalid requests reached service: %d", s.calls)
	}
}

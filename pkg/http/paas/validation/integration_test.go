package validation_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dkoshenkov/packages-go/middlewarex/httpx"

	"github.com/dkoshenkov/paas-common/pkg/http/paas/client"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/deployments"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/operations"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/transport"
	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/validation"
)

type localTransport struct{ handler http.Handler }

func (rt localTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Body != nil {
		defer func() { _ = r.Body.Close() }()
	}
	w := httptest.NewRecorder()
	rt.handler.ServeHTTP(w, r)
	return w.Result(), nil
}

type envelopeService struct{ prepared int }

func (s *envelopeService) Preview(_ context.Context, req paastypes.TypesPreviewRequest) (deployments.DeploymentsPreviewResponse, error) {
	if err := validation.PreviewRequest(req); err != nil {
		return deployments.DeploymentsPreviewResponse400{Body: paastypes.TypesErrorResponse{
			Code: "INVALID_REQUEST", Message: err.Error(), Diagnostics: []paastypes.TypesDiagnostic{},
		}}, nil
	}
	s.prepared++
	return deployments.DeploymentsPreviewResponse200{Body: paastypes.TypesPreviewResponse{
		Diagnostics: []paastypes.TypesDiagnostic{diagnostic()},
	}}, nil
}

func (*envelopeService) Apply(context.Context, paastypes.TypesApplyRequest) (deployments.DeploymentsApplyResponse, error) {
	return nil, errors.New("unused fixture endpoint")
}

func TestSemanticEnvelopeAtServiceBoundary(t *testing.T) {
	s := &envelopeService{}
	mux := http.NewServeMux()
	runtime := httpx.NewRuntime(httpx.WithErrorEncoder(httpx.ErrorEncoderFunc(transport.EncodeError)))
	deployments.RegisterDeploymentsHandlersWithRuntime(mux, s, runtime)
	c, err := client.NewDeploymentsClient("http://paas.test", &http.Client{Transport: localTransport{handler: mux}})
	if err != nil {
		t.Fatal(err)
	}
	invalid := request()
	invalid.Files = []paastypes.TypesManifestFile{}
	duplicate := request()
	duplicate.Files = append(duplicate.Files, duplicate.Files[0])
	for _, req := range []paastypes.TypesPreviewRequest{invalid, duplicate} {
		response, err := c.Preview(t.Context(), req)
		if err != nil {
			t.Fatal(err)
		}
		bad, ok := response.(client.DeploymentsPreviewResponse400)
		if !ok || bad.Body.Code != "INVALID_REQUEST" || s.prepared != 0 {
			t.Fatalf("invalid envelope reached preparation: %#v, %d", response, s.prepared)
		}
		if err := validation.ErrorResponse(bad.Body); err != nil {
			t.Fatal(err)
		}
	}
	response, err := c.Preview(t.Context(), request())
	if err != nil {
		t.Fatal(err)
	}
	checked, ok := response.(client.DeploymentsPreviewResponse200)
	if !ok || s.prepared != 1 {
		t.Fatalf("valid envelope: %#v, %d", response, s.prepared)
	}
	if err := validation.PreviewResponse(checked.Body); err != nil {
		t.Fatal(err)
	}
}

type uncheckedOperationService struct{}

func (uncheckedOperationService) Get(context.Context, paastypes.OperationsOperationsGetQuery) (operations.OperationsGetResponse, error) {
	op := operation()
	op.State = "UNRECOGNIZED_STATE"
	return operations.OperationsGetResponse200{Body: op}, nil
}

func (uncheckedOperationService) List(context.Context, paastypes.OperationsOperationsListQuery) (operations.OperationsListResponse, error) {
	return nil, errors.New("unused fixture endpoint")
}

func TestConsumerRejectsSemanticallyInvalidResponse(t *testing.T) {
	mux := http.NewServeMux()
	operations.RegisterOperationsHandlers(mux, uncheckedOperationService{})
	c, err := client.NewOperationsClient("http://paas.test", &http.Client{Transport: localTransport{handler: mux}})
	if err != nil {
		t.Fatal(err)
	}
	response, err := c.Get(t.Context(), paastypes.OperationsOperationsGetQuery{OperationId: "op"})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := response.(client.OperationsGetResponse200)
	if !ok {
		t.Fatalf("unexpected response: %#v", response)
	}
	if err := validation.Operation(got.Body); err == nil {
		t.Fatal("structurally valid but semantically invalid state was trusted")
	}
}

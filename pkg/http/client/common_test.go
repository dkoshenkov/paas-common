package client_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dkoshenkov/paas-common/pkg/http/client"
	"github.com/dkoshenkov/paas-common/pkg/http/server/health"
	"github.com/dkoshenkov/paas-common/pkg/http/server/ready"
	httptypes "github.com/dkoshenkov/paas-common/pkg/http/types"
)

type commonService struct{}

type handlerTransport struct {
	handler http.Handler
}

func (transport handlerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, request)
	return recorder.Result(), nil
}

func (commonService) Health(context.Context) (httptypes.HealthHealthResponse, error) {
	return httptypes.HealthHealthResponse{Status: "ok"}, nil
}

func (commonService) Ready(context.Context) (httptypes.ReadyReadyResponse, error) {
	return httptypes.ReadyReadyResponse{Status: "ready"}, nil
}

func TestCommonHTTPRoundTrip(t *testing.T) {
	mux := http.NewServeMux()
	health.RegisterHealthHandlers(mux, commonService{})
	ready.RegisterReadyHandlers(mux, commonService{})
	httpClient := &http.Client{Transport: handlerTransport{handler: mux}}

	healthClient, err := client.NewHealthClient("http://common.test", httpClient)
	if err != nil {
		t.Fatal(err)
	}
	healthResponse, err := healthClient.Health(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if healthResponse.Status != "ok" {
		t.Fatalf("health status = %q, want ok", healthResponse.Status)
	}

	readyClient, err := client.NewReadyClient("http://common.test", httpClient)
	if err != nil {
		t.Fatal(err)
	}
	readyResponse, err := readyClient.Ready(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if readyResponse.Status != "ready" {
		t.Fatalf("ready status = %q, want ready", readyResponse.Status)
	}
}

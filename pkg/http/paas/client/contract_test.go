package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/dkoshenkov/packages-go/middlewarex/httpx"

	"github.com/dkoshenkov/paas-common/pkg/http/paas/client"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/deployments"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/server/operations"
	"github.com/dkoshenkov/paas-common/pkg/http/paas/transport"
	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

type serviceStub struct {
	calls   int
	preview func(paastypes.TypesPreviewRequest) (deployments.DeploymentsPreviewResponse, error)
	apply   func(paastypes.TypesApplyRequest) (deployments.DeploymentsApplyResponse, error)
	get     func(paastypes.OperationsOperationsGetQuery) (operations.OperationsGetResponse, error)
}

func (s *serviceStub) Preview(_ context.Context, req paastypes.TypesPreviewRequest) (deployments.DeploymentsPreviewResponse, error) {
	s.calls++
	if s.preview == nil {
		return nil, errors.New("unexpected Preview call")
	}
	return s.preview(req)
}

func (s *serviceStub) Apply(_ context.Context, req paastypes.TypesApplyRequest) (deployments.DeploymentsApplyResponse, error) {
	s.calls++
	if s.apply == nil {
		return nil, errors.New("unexpected Apply call")
	}
	return s.apply(req)
}

func (s *serviceStub) Get(_ context.Context, req paastypes.OperationsOperationsGetQuery) (operations.OperationsGetResponse, error) {
	s.calls++
	if s.get == nil {
		return nil, errors.New("unexpected Get call")
	}
	return s.get(req)
}

func (*serviceStub) List(context.Context, paastypes.OperationsOperationsListQuery) (operations.OperationsListResponse, error) {
	return nil, errors.New("unexpected List call")
}

type handlerTransport struct{ handler http.Handler }

func (transport handlerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.Body != nil {
		defer func() { _ = request.Body.Close() }()
	}
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, request)
	return recorder.Result(), nil
}

func harness(s *serviceStub) *http.Client {
	mux := http.NewServeMux()
	runtime := httpx.NewRuntime(httpx.WithErrorEncoder(httpx.ErrorEncoderFunc(transport.EncodeError)))
	deployments.RegisterDeploymentsHandlersWithRuntime(mux, s, runtime)
	operations.RegisterOperationsHandlersWithRuntime(mux, s, runtime)
	return &http.Client{Transport: handlerTransport{handler: mux}}
}

func pointer[T any](value T) *T { return &value }

func operation() paastypes.TypesOperation {
	return paastypes.TypesOperation{
		OperationId: "op-1", AppId: "app-1", ComponentId: "component-1", State: "ACCEPTED",
		Diagnostics: []paastypes.TypesDiagnostic{}, CreatedAt: "2026-10-20T00:00:00Z", UpdatedAt: "2026-10-20T00:00:00Z",
	}
}

func apiError(code string) paastypes.TypesErrorResponse {
	return paastypes.TypesErrorResponse{Code: code, Message: "Safe explanation", Diagnostics: []paastypes.TypesDiagnostic{}}
}

func TestPreviewYAMLAndOptionalFields(t *testing.T) {
	input := paastypes.TypesPreviewRequest{
		AppId: "app-1", ComponentId: "component-1",
		Files: []paastypes.TypesManifestFile{{Name: "deploy.yaml", Content: "# Пример\napiVersion: apps/v1\nkind: Deployment\n---\n# second document\n"}},
	}
	plan := paastypes.TypesPlan{
		Resources: []paastypes.TypesResourceChange{{
			ApiVersion: "apps/v1", Kind: "Deployment", Namespace: "app-1", Name: "api",
			Action: "CREATE", ChangedFields: []string{""},
		}}, AdditionalActions: []paastypes.TypesAdditionalAction{},
	}
	success := paastypes.TypesPreviewResponse{
		Diagnostics: []paastypes.TypesDiagnostic{}, Plan: &plan,
		PreviewId: pointer("preview-1"), ExpiresAt: pointer("2026-10-20T00:30:00Z"),
	}
	invalid := paastypes.TypesPreviewResponse{Diagnostics: []paastypes.TypesDiagnostic{{
		Severity: "ERROR", Code: "INVALID_MANIFEST", Message: "Unsupported resource",
		File: pointer("deploy.yaml"), Document: pointer(2), FieldPath: pointer("/kind"), Line: pointer(5), Column: pointer(1),
	}}}
	for name, want := range map[string]paastypes.TypesPreviewResponse{"success": success, "invalid": invalid} {
		t.Run(name, func(t *testing.T) {
			s := &serviceStub{preview: func(got paastypes.TypesPreviewRequest) (deployments.DeploymentsPreviewResponse, error) {
				if !reflect.DeepEqual(got, input) {
					t.Fatalf("request changed: %#v", got)
				}
				return deployments.DeploymentsPreviewResponse200{Body: want}, nil
			}}
			c, err := client.NewDeploymentsClient("http://paas.test", harness(s))
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.Preview(t.Context(), input)
			if err != nil || !reflect.DeepEqual(got, client.DeploymentsPreviewResponse200{Body: want}) {
				t.Fatalf("preview = %#v, %v", got, err)
			}
		})
	}
	encoded, err := json.Marshal(invalid)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"plan", "preview_id", "expires_at"} {
		if _, exists := fields[name]; exists {
			t.Fatalf("invalid preview unexpectedly contains %s", name)
		}
	}
}

func TestApplyTypedStatuses(t *testing.T) {
	op := operation()
	busy := apiError("COMPONENT_BUSY")
	busy.ConflictingOperationId = pointer("blocking-op")
	cases := []struct {
		name string
		give deployments.DeploymentsApplyResponse
		want client.DeploymentsApplyResponse
	}{
		{"repeat", deployments.DeploymentsApplyResponse200{Body: op}, client.DeploymentsApplyResponse200{Body: op}},
		{"accepted", deployments.DeploymentsApplyResponse202{Body: op}, client.DeploymentsApplyResponse202{Body: op}},
		{"invalid", deployments.DeploymentsApplyResponse400{Body: apiError("INVALID_REQUEST")}, client.DeploymentsApplyResponse400{Body: apiError("INVALID_REQUEST")}},
		{"missing", deployments.DeploymentsApplyResponse404{Body: apiError("NOT_FOUND")}, client.DeploymentsApplyResponse404{Body: apiError("NOT_FOUND")}},
		{"busy", deployments.DeploymentsApplyResponse409{Body: busy}, client.DeploymentsApplyResponse409{Body: busy}},
		{"expired", deployments.DeploymentsApplyResponse410{Body: apiError("PREVIEW_EXPIRED")}, client.DeploymentsApplyResponse410{Body: apiError("PREVIEW_EXPIRED")}},
		{"large", deployments.DeploymentsApplyResponse413{Body: apiError("BODY_TOO_LARGE")}, client.DeploymentsApplyResponse413{Body: apiError("BODY_TOO_LARGE")}},
		{"unknown", deployments.DeploymentsApplyResponse503{Body: apiError("ACCEPTANCE_UNKNOWN")}, client.DeploymentsApplyResponse503{Body: apiError("ACCEPTANCE_UNKNOWN")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &serviceStub{apply: func(req paastypes.TypesApplyRequest) (deployments.DeploymentsApplyResponse, error) {
				if req.PreviewId != "preview-1" {
					t.Fatalf("preview_id = %q", req.PreviewId)
				}
				return tc.give, nil
			}}
			c, err := client.NewDeploymentsClient("http://paas.test", harness(s))
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.Apply(t.Context(), paastypes.TypesApplyRequest{PreviewId: "preview-1"})
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("apply = %#v, %v; want %#v", got, err, tc.want)
			}
		})
	}
}

func TestOperationQueryAndStates(t *testing.T) {
	id := "op /?&+%#Пример"
	for _, state := range []string{"ACCEPTED", "RUNNING", "SUCCEEDED", "FAILED", "RECOVERY_REQUIRED"} {
		t.Run(state, func(t *testing.T) {
			op := operation()
			op.OperationId, op.State = id, state
			if state == "SUCCEEDED" || state == "FAILED" {
				op.FinishedAt = pointer("2026-10-20T00:01:00Z")
			}
			if state != "ACCEPTED" {
				op.Stage = pointer("fixture-stage")
			}
			s := &serviceStub{get: func(req paastypes.OperationsOperationsGetQuery) (operations.OperationsGetResponse, error) {
				if req.OperationId != id {
					t.Fatalf("query changed: %q", req.OperationId)
				}
				return operations.OperationsGetResponse200{Body: op}, nil
			}}
			c, err := client.NewOperationsClient("http://paas.test", harness(s))
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.Get(t.Context(), paastypes.OperationsOperationsGetQuery{OperationId: id})
			if err != nil || !reflect.DeepEqual(got, client.OperationsGetResponse200{Body: op}) {
				t.Fatalf("get = %#v, %v", got, err)
			}
		})
	}
}

func TestTransportErrorsBeforeService(t *testing.T) {
	valid := `{"app_id":"app-1","component_id":"component-1","files":[]}`
	cases := []struct {
		name, method, path, body string
		status                   int
		code                     string
	}{
		{"malformed", "POST", "/deployments/preview", `{secret-input`, 400, "INVALID_REQUEST"},
		{"missing", "POST", "/deployments/apply", `{}`, 400, "INVALID_REQUEST"},
		{"null", "POST", "/deployments/apply", `{"preview_id":null}`, 400, "INVALID_REQUEST"},
		{"unknown", "POST", "/deployments/preview", valid[:len(valid)-1] + `,"secret-input":true}`, 400, "INVALID_REQUEST"},
		{"nested-unknown", "POST", "/deployments/preview", `{"app_id":"a","component_id":"c","files":[{"name":"f","content":"x","secret-input":true}]}`, 400, "INVALID_REQUEST"},
		{"missing-query", "GET", "/operations/get", "", 400, "INVALID_REQUEST"},
		{"repeat-query", "GET", "/operations/get?operation_id=a&operation_id=b", "", 400, "INVALID_REQUEST"},
		{"large", "POST", "/deployments/preview", `{"app_id":"a","component_id":"c","files":[{"name":"f","content":"` + strings.Repeat("x", 8388608) + `"}]}`, 413, "BODY_TOO_LARGE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &serviceStub{}
			req, err := http.NewRequestWithContext(t.Context(), tc.method, "http://paas.test"+tc.path, strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			response, err := harness(s).Do(req)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := response.Body.Close(); err != nil {
					t.Error(err)
				}
			})
			data, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			var body paastypes.TypesErrorResponse
			if err := json.Unmarshal(data, &body); err != nil {
				t.Fatalf("not ErrorResponse: %s: %v", data, err)
			}
			if response.StatusCode != tc.status || body.Code != tc.code || body.Diagnostics == nil || s.calls != 0 {
				t.Fatalf("status=%d body=%s calls=%d", response.StatusCode, data, s.calls)
			}
			if strings.Contains(string(data), "secret-input") {
				t.Fatalf("unsafe error: %s", data)
			}
		})
	}
}

func TestNonNullCollectionsAndOptionalZero(t *testing.T) {
	if _, err := json.Marshal(paastypes.TypesPreviewRequest{AppId: "a", ComponentId: "c"}); err == nil {
		t.Fatal("nil required files must not serialize as null")
	}
	for _, data := range []string{`{"app_id":"a","component_id":"c","files":null}`, `{"component_id":"c","files":[]}`} {
		var request paastypes.TypesPreviewRequest
		if err := json.Unmarshal([]byte(data), &request); err == nil {
			t.Fatalf("invalid required fields accepted: %s", data)
		}
	}
	diagnostic := paastypes.TypesDiagnostic{Severity: "INFO", Code: "fixture", Message: "fixture", FieldPath: pointer("")}
	data, err := json.Marshal(diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	var decoded paastypes.TypesDiagnostic
	if err := json.Unmarshal(data, &decoded); err != nil || decoded.FieldPath == nil || *decoded.FieldPath != "" || decoded.Document != nil {
		t.Fatalf("optional empty path changed: %s, %v", data, err)
	}
}

func TestGeneratedClientDecodesTransportErrors(t *testing.T) {
	for _, status := range []int{400, 413} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			s := &serviceStub{}
			httpClient := harness(s)
			base := httpClient.Transport.(handlerTransport)
			httpClient.Transport = handlerTransport{handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body := `{"preview_id":null}`
				if status == 413 {
					body = `{"preview_id":"` + strings.Repeat("x", 8388608) + `"}`
				}
				r.Body = io.NopCloser(strings.NewReader(body))
				base.handler.ServeHTTP(w, r)
			})}
			c, err := client.NewDeploymentsClient("http://paas.test", httpClient)
			if err != nil {
				t.Fatal(err)
			}
			response, err := c.Apply(t.Context(), paastypes.TypesApplyRequest{PreviewId: "preview-1"})
			if err != nil {
				t.Fatal(err)
			}
			switch body := response.(type) {
			case client.DeploymentsApplyResponse400:
				if status != 400 || body.Body.Code != "INVALID_REQUEST" {
					t.Fatalf("unexpected 400: %#v", body)
				}
			case client.DeploymentsApplyResponse413:
				if status != 413 || body.Body.Code != "BODY_TOO_LARGE" {
					t.Fatalf("unexpected 413: %#v", body)
				}
			default:
				t.Fatalf("unexpected response: %#v", response)
			}
			if s.calls != 0 {
				t.Fatal("transport error reached service")
			}
		})
	}
}

func TestOperationTypedErrors(t *testing.T) {
	cases := []struct {
		name string
		give operations.OperationsGetResponse
		want client.OperationsGetResponse
	}{
		{"missing", operations.OperationsGetResponse404{Body: apiError("NOT_FOUND")}, client.OperationsGetResponse404{Body: apiError("NOT_FOUND")}},
		{"unavailable", operations.OperationsGetResponse503{Body: apiError("DEPENDENCY_UNAVAILABLE")}, client.OperationsGetResponse503{Body: apiError("DEPENDENCY_UNAVAILABLE")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &serviceStub{get: func(paastypes.OperationsOperationsGetQuery) (operations.OperationsGetResponse, error) {
				return tc.give, nil
			}}
			c, err := client.NewOperationsClient("http://paas.test", harness(s))
			if err != nil {
				t.Fatal(err)
			}
			got, err := c.Get(t.Context(), paastypes.OperationsOperationsGetQuery{OperationId: "op-1"})
			if err != nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("get = %#v, %v", got, err)
			}
		})
	}
}

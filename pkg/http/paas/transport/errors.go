// Package transport provides runtime adapters for the generated PaaS HTTP API.
package transport

import (
	"encoding/json"
	"net/http"

	"github.com/dkoshenkov/packages-go/middlewarex/httpx"

	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

// EncodeError adapts runtime errors without exposing decoder input or internal errors.
// Install it with httpx.WithErrorEncoder(httpx.ErrorEncoderFunc(EncodeError)).
// Services return declared response variants for domain errors.
func EncodeError(w http.ResponseWriter, _ *http.Request, err error) {
	status := httpx.DefaultStatusMapper(err)
	code, message := "INTERNAL_ERROR", "Internal server error"
	switch status {
	case http.StatusBadRequest:
		code, message = "INVALID_REQUEST", "Invalid request"
	case http.StatusRequestEntityTooLarge:
		code, message = "BODY_TOO_LARGE", "Request body exceeds the limit"
	case http.StatusServiceUnavailable:
		code, message = "DEPENDENCY_UNAVAILABLE", "Request could not be confirmed; retry with the same preview_id when applying"
	default:
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(paastypes.TypesErrorResponse{
		Code: code, Message: message, Diagnostics: []paastypes.TypesDiagnostic{},
	})
}

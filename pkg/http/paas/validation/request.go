// Package validation checks the context-free semantic rules of PaaS HTTP DTOs.
// It does not parse manifests or check ownership, database state or Kubernetes.
// Callers must explicitly invoke these checks; generated handlers do not install them.
package validation

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	paastypes "github.com/dkoshenkov/paas-common/pkg/http/paas/types"
)

const (
	MaxFiles         = 64
	MaxFileNameBytes = 512
)

// PreviewRequest checks the envelope before YAML parsing, without changing its input.
func PreviewRequest(req paastypes.TypesPreviewRequest) error {
	if req.AppId == "" || req.ComponentId == "" {
		return fmt.Errorf("app_id and component_id: required nonempty IDs")
	}
	if len(req.Files) == 0 || len(req.Files) > MaxFiles {
		return fmt.Errorf("files: require 1 through %d entries", MaxFiles)
	}
	names := make(map[string]bool, len(req.Files))
	for i, file := range req.Files {
		if err := fileName(file.Name); err != nil {
			return fmt.Errorf("files[%d].name: %w", i, err)
		}
		if names[file.Name] {
			return fmt.Errorf("files[%d].name: duplicate diagnostic name", i)
		}
		names[file.Name] = true
	}
	return nil
}

// ApplyRequest checks presence of the opaque preview ID, not its existence or TTL.
func ApplyRequest(req paastypes.TypesApplyRequest) error {
	if req.PreviewId == "" {
		return fmt.Errorf("preview_id: required nonempty ID")
	}
	return nil
}

// OperationQuery checks presence of the opaque operation ID, not its existence.
func OperationQuery(req paastypes.OperationsOperationsGetQuery) error {
	if req.OperationId == "" {
		return fmt.Errorf("operation_id: required nonempty ID")
	}
	return nil
}

func fileName(name string) error {
	if name == "" || len(name) > MaxFileNameBytes || !utf8.ValidString(name) {
		return fmt.Errorf("require a nonempty UTF-8 name up to %d bytes", MaxFileNameBytes)
	}
	if strings.ContainsFunc(name, unicode.IsControl) {
		return fmt.Errorf("control characters are forbidden")
	}
	return nil
}

// Diagnostic checks severity and source coordinates. It cannot detect credentials
// in text or replace the caller's redaction and safe terminal rendering.
func Diagnostic(d paastypes.TypesDiagnostic) error {
	switch d.Severity {
	case "ERROR", "WARN", "INFO":
	default:
		return fmt.Errorf("severity: unknown value")
	}
	if d.Code == "" || d.Message == "" {
		return fmt.Errorf("code and message: required nonempty strings")
	}
	if d.File != nil {
		if err := fileName(*d.File); err != nil {
			return fmt.Errorf("file: %w", err)
		}
	}
	if d.Document != nil && (d.File == nil || *d.Document < 1) {
		return fmt.Errorf("document: require a file and a positive document number")
	}
	if d.Line != nil && (d.File == nil || *d.Line < 1) {
		return fmt.Errorf("line: require a file and a positive line number")
	}
	if d.Column != nil && (d.Line == nil || *d.Column < 1) {
		return fmt.Errorf("column: require a line and a positive column number")
	}
	if d.FieldPath != nil {
		if !jsonPointer(*d.FieldPath) {
			return fmt.Errorf("field_path: invalid JSON Pointer")
		}
		if d.File != nil && d.Document == nil {
			return fmt.Errorf("field_path: a YAML field requires its document number")
		}
	}
	return nil
}

func diagnostics(items []paastypes.TypesDiagnostic) error {
	if items == nil {
		return fmt.Errorf("diagnostics: require an array, not null")
	}
	for i, d := range items {
		if err := Diagnostic(d); err != nil {
			return fmt.Errorf("diagnostics[%d]: %w", i, err)
		}
	}
	return nil
}

func jsonPointer(value string) bool {
	if value == "" {
		return true
	}
	if value[0] != '/' || !utf8.ValidString(value) {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] == '~' {
			i++
			if i == len(value) || (value[i] != '0' && value[i] != '1') {
				return false
			}
		}
	}
	return true
}

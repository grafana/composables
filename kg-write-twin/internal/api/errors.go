package api

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/grafana/kg-write-twin/internal/model"
)

// apiError is an internal representation mapped to an ApiError response body.
type apiError struct {
	httpCode  int
	status    string
	message   string
	subErrors []model.FieldError
}

// newRequestID mimics the real API's String.format("%16x", rnd): 16-wide,
// space-padded hex of a random 64-bit value.
func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%16x", binary.BigEndian.Uint64(b[:]))
}

// body renders the ApiError JSON with field order status, requestId, timestamp,
// message, subErrors (subErrors omitted when empty).
func (e apiError) body(now func() int64) []byte {
	type errorBody struct {
		Status    string             `json:"status"`
		RequestID string             `json:"requestId"`
		Timestamp int64              `json:"timestamp"`
		Message   string             `json:"message"`
		SubErrors []model.FieldError `json:"subErrors,omitempty"`
	}
	eb := errorBody{
		Status: e.status, RequestID: newRequestID(), Timestamp: now(),
		Message: e.message, SubErrors: e.subErrors,
	}
	out, _ := json.Marshal(eb)
	return out
}

// Constructors for the fixed error shapes captured from the real API.
func errTenantMissing() apiError {
	return apiError{424, "FAILED_DEPENDENCY", "No tenant selected for request", nil}
}
func errBadNamespace() apiError {
	return apiError{400, "BAD_REQUEST", "namespace must be of the form 'stacks-<stackId>'", nil}
}
func errNamespaceMismatch() apiError {
	return apiError{403, "FORBIDDEN", "namespace does not match the request tenant", nil}
}
func errTenantInit(v string) apiError {
	return apiError{500, "INTERNAL_SERVER_ERROR", fmt.Sprintf("Failed initializing tenantId=%s, cannot continue", v), nil}
}
func errUnsupportedMedia(ct string) apiError {
	return apiError{415, "UNSUPPORTED_MEDIA_TYPE", fmt.Sprintf("Content-Type '%s' is not supported", ct), nil}
}
func errParse(detail string) apiError {
	return apiError{400, "BAD_REQUEST", "JSON parse error: " + detail, nil}
}
func errValidation(message string, subErrors []model.FieldError) apiError {
	return apiError{422, "UNPROCESSABLE_ENTITY", message, subErrors}
}
func errNotFound(message string) apiError {
	return apiError{404, "NOT_FOUND", message, nil}
}
func errConflict(message string) apiError {
	return apiError{409, "CONFLICT", message, nil}
}
func errForbidden(message string) apiError {
	return apiError{403, "FORBIDDEN", message, nil}
}
func errMethodNotAllowed(method string) apiError {
	return apiError{405, "METHOD_NOT_ALLOWED", fmt.Sprintf("Request method '%s' is not supported", method), nil}
}

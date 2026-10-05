package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id"`
	Retryable  bool   `json:"retryable"`
	RetryAfter int    `json:"retry_after,omitempty"`
}

// These errors keep adapter failures within the public error contract without
// exposing parser input, private metadata or internal implementation details.
var (
	ErrBudgetExceeded   = appError("budget_exceeded", "Response exceeds the requested byte budget", http.StatusRequestEntityTooLarge, false)
	ErrValidationFailed = appError("validation_failed", "Invalid request", http.StatusUnprocessableEntity, false)
)

func (e Error) Error() string { return e.Code + ": " + e.Message }

func appError(code, message string, status int, retryable bool) error {
	return appErrorWithRetry(code, message, status, retryable, 0)
}
func appErrorWithRetry(code, message string, status int, retryable bool, retryAfter int) error {
	return codedError{body: Error{Code: code, Message: message, Retryable: retryable, RetryAfter: retryAfter}, status: status}
}

type codedError struct {
	body   Error
	status int
}

func (e codedError) Error() string   { return e.body.Error() }
func (e codedError) HTTPStatus() int { return e.status }

func statusFor(err error) int {
	var ce interface{ HTTPStatus() int }
	if errors.As(err, &ce) {
		return ce.HTTPStatus()
	}
	return http.StatusServiceUnavailable
}
func errorBody(err error, requestID string) Error {
	var ce codedError
	if errors.As(err, &ce) {
		ce.body.RequestID = requestID
		return ce.body
	}
	return Error{Code: "service_unavailable", Message: "Service unavailable", RequestID: requestID, Retryable: true}
}
func writeError(w http.ResponseWriter, err error, requestID string) {
	e := errorBody(err, requestID)
	w.Header().Set("Content-Type", "application/json")
	if e.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(e.RetryAfter))
	}
	w.WriteHeader(statusFor(err))
	_ = json.NewEncoder(w).Encode(e)
}

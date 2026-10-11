package genapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrorClass is the failure taxonomy every dialect maps into (GEN-API-
// ADAPTER-1 acceptance: error classification). Classes drive user-facing
// guidance and retry decisions; they never guess a success.
type ErrorClass string

const (
	ClassConfigError       ErrorClass = "config_error"         // user config missing/invalid; no HTTP spent
	ClassAuthError         ErrorClass = "auth_error"           // 401/403 — credential rejected
	ClassQuotaError        ErrorClass = "quota_error"          // 402 / billing-exhausted reports
	ClassRateLimitError    ErrorClass = "rate_limit"           // 429 without quota semantics
	ClassNotFoundError     ErrorClass = "not_found"            // 404 — endpoint or model
	ClassProviderError     ErrorClass = "provider_error"       // 5xx or provider-reported task failure
	ClassModerationError   ErrorClass = "moderation_error"     // provider content-policy rejection
	ClassTimeoutError      ErrorClass = "timeout"              // client/poll deadline exceeded
	ClassNetworkError      ErrorClass = "network_error"        // transport failures
	ClassParseError        ErrorClass = "response_parse_error" // 2xx but unexpected body shape
	ClassUnsupportedFormat ErrorClass = "unsupported_format"   // artifact sniff failed / size cap
)

// Error is the classified adapter failure.
type Error struct {
	Class           ErrorClass
	HTTPStatus      int
	ProviderMessage string
	Retryable       bool
	Cause           error
}

func (e *Error) Error() string {
	parts := []string{fmt.Sprintf("genapi %s", e.Class)}
	if e.HTTPStatus > 0 {
		parts = append(parts, fmt.Sprintf("http=%d", e.HTTPStatus))
	}
	if e.ProviderMessage != "" {
		parts = append(parts, e.ProviderMessage)
	}
	if e.Cause != nil {
		parts = append(parts, "cause: "+e.Cause.Error())
	}
	return strings.Join(parts, ": ")
}

func (e *Error) Unwrap() error { return e.Cause }

// AsError normalizes any error into *Error (unknown failures become
// provider_error, retryable unknown).
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var ge *Error
	if errors.As(err, &ge) {
		return ge
	}
	return &Error{Class: ClassProviderError, ProviderMessage: err.Error(), Retryable: true}
}

// classifyHTTP maps an HTTP failure status plus body onto the taxonomy.
func classifyHTTP(status int, body []byte) *Error {
	msg := extractProviderMessage(body)
	klass := ClassProviderError
	retryable := false
	switch {
	case status == 401 || status == 403:
		klass = ClassAuthError
	case status == 402:
		klass = ClassQuotaError
	case status == 404:
		klass = ClassNotFoundError
	case status == 408:
		klass, retryable = ClassTimeoutError, true
	case status == 429:
		klass, retryable = ClassRateLimitError, true
		if quotaHint(msg) {
			klass = ClassQuotaError
		}
	case status >= 500:
		retryable = true
	}
	if moderationHint(msg) {
		klass, retryable = ClassModerationError, false
	}
	return &Error{Class: klass, HTTPStatus: status, ProviderMessage: msg, Retryable: retryable}
}

// extractProviderMessage pulls the human-readable failure text out of a
// provider JSON body (common field names, bounded length, secrets never
// included because responses are not expected to contain credentials).
func extractProviderMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		return printable(body)
	}
	for _, path := range []string{"message", "error.message", "error", "detail", "fail_reason", "msg"} {
		if v, ok := lookupPath(decoded, path); ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return truncate(s, 400)
			}
		}
	}
	return truncate(printable(body), 200)
}

func quotaHint(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "quota") || strings.Contains(m, "billing") || strings.Contains(m, "insufficient") || strings.Contains(m, "credit")
}

func moderationHint(msg string) bool {
	m := strings.ToLower(msg)
	return (strings.Contains(m, "content policy") || strings.Contains(m, "content_policy") || strings.Contains(m, "flagged") || strings.Contains(m, "moderation") || strings.Contains(m, "prohibited content")) && strings.Contains(m, "content")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ResultPayload renders a classified error for tool results: the class,
// retry advice and provider message, with no credential material.
func (e *Error) ResultPayload() map[string]any {
	return map[string]any{
		"error_class":      string(e.Class),
		"retryable":        e.Retryable,
		"http_status":      e.HTTPStatus,
		"provider_message": e.ProviderMessage,
	}
}

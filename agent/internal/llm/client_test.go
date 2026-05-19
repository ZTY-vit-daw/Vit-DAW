package llm

import (
	"encoding/json"
	"testing"
)

func TestErrorMessageAcceptsStringAndObject(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "string", raw: `"system disk overloaded"`, want: "system disk overloaded"},
		{name: "object", raw: `{"message":"invalid model"}`, want: "invalid model"},
		{name: "nested", raw: `{"error":{"message":"bad key"}}`, want: "bad key"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := errorMessage(json.RawMessage(tc.raw))
			if got != tc.want {
				t.Fatalf("errorMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEndpointFor(t *testing.T) {
	tests := []struct {
		name string
		base string
		url  string
		kind string
	}{
		{name: "base v1", base: "https://api.example.com/v1", url: "https://api.example.com/v1/chat/completions", kind: "chat"},
		{name: "chat endpoint", base: "https://api.example.com/v1/chat/completions", url: "https://api.example.com/v1/chat/completions", kind: "chat"},
		{name: "responses endpoint", base: "https://api.example.com/v1/responses", url: "https://api.example.com/v1/responses", kind: "responses"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := endpointFor(tc.base)
			if got.URL != tc.url || got.Kind != tc.kind {
				t.Fatalf("endpointFor() = %#v, want url=%q kind=%q", got, tc.url, tc.kind)
			}
		})
	}
}

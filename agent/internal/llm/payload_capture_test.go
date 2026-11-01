package llm

// payload_capture_test.go — M3-NEUTRAL-ACTION-PATH-1（235）measurement face:
// env-gated full payload capture. Proves the card's gating contract:
//   1. default OFF with NO fallback path (unset env never writes anywhere —
//      no workspace default, no cwd default; §10 source-tree protection);
//   2. explicit "off"/"disabled" values stay off (telemetry vocabulary);
//   3. capture ON dumps the complete request messages (system + user +
//      history, full bytes) plus the response content (reply text + raw
//      HTTP body) into <dir>/agent_llm_payload_capture.jsonl;
//   4. failed calls are still recorded (every call, not only successes) —
//      request surface + error, so environment interrupts stay auditable.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/config"
)

func TestPayloadCaptureDefaultOffNoFallbackPath(t *testing.T) {
	t.Setenv("VIT_AGENT_LLM_PAYLOAD_CAPTURE", "")
	if got := payloadCapturePath(); got != "" {
		t.Fatalf("payloadCapturePath() = %q, want empty (default off, no fallback path)", got)
	}

	// Full-call negative: with the gate unset a real CompleteRequest must not
	// create any file — not in a supplied-looking cwd, not in a workspace-like
	// default. Run the call from a scratch cwd and sweep it afterwards.
	scratch := t.TempDir()
	prior, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(scratch); err != nil {
		t.Fatalf("chdir scratch: %v", err)
	}
	defer func() {
		_ = os.Chdir(prior)
	}()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	_, _ = client.CompleteRequest(context.Background(), config.EngineConfig{
		BaseURL: server.URL + "/v1", APIKey: "test", DefaultModel: "test-model",
	}, Request{Messages: []Message{{Role: "user", Content: "hello"}}})

	var wrote []string
	err = filepath.Walk(scratch, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !info.IsDir() {
			wrote = append(wrote, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk scratch: %v", err)
	}
	if len(wrote) != 0 {
		t.Fatalf("unset gate must write nothing anywhere; found %v", wrote)
	}
}

func TestPayloadCaptureExplicitOffValues(t *testing.T) {
	for _, value := range []string{"off", "disabled", "OFF", " Disabled "} {
		t.Setenv("VIT_AGENT_LLM_PAYLOAD_CAPTURE", value)
		if got := payloadCapturePath(); got != "" {
			t.Fatalf("payloadCapturePath() = %q for %q, want empty (explicit off)", got, value)
		}
	}
}

func TestPayloadCaptureWritesFullRequestAndResponse(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VIT_AGENT_LLM_PAYLOAD_CAPTURE", dir)

	rawBody := `{"choices":[{"message":{"content":"tool plan"}}],"usage":{"prompt_tokens":40,"completion_tokens":6}}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(rawBody))
	}))
	defer server.Close()

	messages := []Message{
		{Role: "system", Content: "system skeleton bytes\nline2"},
		{Role: "user", Content: "user turn with directives\n\nPer-turn directives"},
		{Role: "assistant", Content: "prior assistant turn"},
		{Role: "user", Content: "history follow-up"},
	}
	client := &Client{HTTPClient: server.Client()}
	_, err := client.CompleteRequest(context.Background(), config.EngineConfig{
		BaseURL: server.URL + "/v1", APIKey: "test", DefaultModel: "test-model",
	}, Request{
		Messages: messages,
		Metadata: RequestMetadata{Source: "smoke", ConversationID: "conv-1", GoalID: "goal-1"},
		Tools:    []map[string]any{{"name": "automation.set_points"}},
	})
	if err != nil {
		t.Fatalf("CompleteRequest error: %v", err)
	}

	path := filepath.Join(dir, payloadCaptureFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("capture file missing: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly one capture row, got %d", len(lines))
	}
	var record PayloadCaptureRecord
	if err := json.Unmarshal([]byte(lines[0]), &record); err != nil {
		t.Fatalf("capture row not valid JSON: %v", err)
	}
	if record.SchemaVersion != PayloadCaptureSchemaVersion {
		t.Fatalf("schema_version = %q, want %q", record.SchemaVersion, PayloadCaptureSchemaVersion)
	}
	if record.Source != "smoke" || record.ConversationID != "conv-1" || record.GoalID != "goal-1" {
		t.Fatalf("metadata not carried: %+v", record)
	}
	if record.Model != "test-model" || record.EndpointKind != "chat" {
		t.Fatalf("model/endpoint not carried: %q / %q", record.Model, record.EndpointKind)
	}
	// Full request messages, byte-identical and in order.
	if len(record.Request.Messages) != len(messages) {
		t.Fatalf("captured %d messages, want %d", len(record.Request.Messages), len(messages))
	}
	for index, want := range messages {
		got := record.Request.Messages[index]
		if got.Role != want.Role || got.Content != want.Content {
			t.Fatalf("message %d mismatch: got (%q, %q), want (%q, %q)", index, got.Role, got.Content, want.Role, want.Content)
		}
	}
	if len(record.Request.Tools) != 1 {
		t.Fatalf("tools surface not captured: %+v", record.Request.Tools)
	}
	// Response content: reply text + raw body bytes.
	if record.Response.Text != "tool plan" {
		t.Fatalf("response text = %q, want %q", record.Response.Text, "tool plan")
	}
	if record.Response.Raw != rawBody {
		t.Fatalf("raw response body not captured verbatim")
	}
	if record.Response.Error != "" {
		t.Fatalf("successful call captured an error: %q", record.Response.Error)
	}
}

func TestPayloadCaptureRecordsFailedCall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VIT_AGENT_LLM_PAYLOAD_CAPTURE", dir)

	// Dead endpoint: connection refused, no HTTP body — the call must still
	// be captured (request surface + error) per the "every call" contract.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	client := &Client{}
	_, err := client.CompleteRequest(context.Background(), config.EngineConfig{
		BaseURL: deadURL + "/v1", APIKey: "test", DefaultModel: "test-model",
	}, Request{Messages: []Message{{Role: "user", Content: "probe"}}})
	if err == nil {
		t.Fatalf("expected transport error from dead endpoint")
	}

	path := filepath.Join(dir, payloadCaptureFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed call must still be captured: %v", err)
	}
	var record PayloadCaptureRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("capture row not valid JSON: %v", err)
	}
	if len(record.Request.Messages) != 1 || record.Request.Messages[0].Content != "probe" {
		t.Fatalf("request surface not captured on failed call: %+v", record.Request.Messages)
	}
	if record.Response.Error == "" {
		t.Fatalf("call error not recorded: %+v", record.Response)
	}
	if record.Response.Text != "" || record.Response.Raw != "" {
		t.Fatalf("failed call must not carry fabricated response content: %+v", record.Response)
	}
}

package llm

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

func TestCompleteRequestParsesChatUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"ok"}}],
			"usage":{"prompt_tokens":12,"completion_tokens":5,"prompt_tokens_details":{"cached_tokens":7}}
		}`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	resp, err := client.CompleteRequest(context.Background(), config.EngineConfig{
		BaseURL:      server.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "test-model",
	}, Request{Messages: []Message{{Role: "user", Content: "hello"}}})
	if err != nil {
		t.Fatalf("CompleteRequest error: %v", err)
	}
	if resp.Text != "ok" || resp.RawKind != "chat" {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Usage.InputTokens != 12 || resp.Usage.OutputTokens != 5 || resp.Usage.CacheReadTokens != 7 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestCompleteRequestParsesResponsesTextAndMissingUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"hello from responses"}]}]}`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	resp, err := client.CompleteRequest(context.Background(), config.EngineConfig{
		BaseURL:      server.URL + "/v1/responses",
		APIKey:       "test",
		DefaultModel: "test-model",
	}, Request{Messages: []Message{{Role: "user", Content: "hello"}}})
	if err != nil {
		t.Fatalf("CompleteRequest error: %v", err)
	}
	if resp.Text != "hello from responses" || resp.RawKind != "responses" {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Usage != (Usage{}) {
		t.Fatalf("missing usage should stay zero: %+v", resp.Usage)
	}
}

func TestCompleteRequestAddsResponsesTools(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"ok"}`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	resp, err := client.CompleteRequest(context.Background(), config.EngineConfig{
		BaseURL:      server.URL + "/v1/responses",
		APIKey:       "test",
		DefaultModel: "gpt-test",
	}, Request{
		Messages:   []Message{{Role: "user", Content: "find plugin docs"}},
		Tools:      []map[string]any{{"type": "web_search"}},
		ToolChoice: "auto",
	})
	if err != nil {
		t.Fatalf("CompleteRequest error: %v", err)
	}
	if resp.Text != "ok" {
		t.Fatalf("response = %+v", resp)
	}
	tools, ok := body["tools"].([]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools not forwarded: %+v", body)
	}
	first, _ := tools[0].(map[string]any)
	if first["type"] != "web_search" || body["tool_choice"] != "auto" {
		t.Fatalf("unexpected tool body: %+v", body)
	}
}

func TestCompleteRequestPreferJSONDisablesResponsesStream(t *testing.T) {
	var body map[string]any
	var accept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept = r.Header.Get("Accept")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output_text":"ok"}`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	resp, err := client.CompleteRequest(context.Background(), config.EngineConfig{
		BaseURL:      server.URL + "/v1/responses",
		APIKey:       "test",
		DefaultModel: "gpt-test",
	}, Request{
		Messages:   []Message{{Role: "user", Content: "find plugin docs"}},
		Tools:      []map[string]any{{"type": "web_search"}},
		ToolChoice: "auto",
		PreferJSON: true,
	})
	if err != nil {
		t.Fatalf("CompleteRequest error: %v", err)
	}
	if resp.Text != "ok" || accept != "application/json" || body["stream"] != false {
		t.Fatalf("resp=%+v accept=%q body=%+v", resp, accept, body)
	}
}

func TestCompleteImageUnderstandingDefaultsToResponsesEndpoint(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"{\"schema\":\"plugin_ui_reference_digest.v1\"}"}]}]}`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	resp, err := client.CompleteImageUnderstanding(context.Background(), config.EngineConfig{
		BaseURL:      server.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "text-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: true, Provider: "default", Model: "vision-model"},
		},
	}, VisionRequest{
		Prompt: "inspect",
		Images: []ImageInput{{MIME: "image/png", DataBase64: "YWJj", Detail: "high"}},
	})
	if err != nil {
		t.Fatalf("CompleteImageUnderstanding error: %v", err)
	}
	if resp.Text == "" || resp.RawKind != "responses" {
		t.Fatalf("response = %+v", resp)
	}
	if body["model"] != "vision-model" {
		t.Fatalf("model = %v", body["model"])
	}
	input := body["input"].([]any)
	content := input[0].(map[string]any)["content"].([]any)
	if content[0].(map[string]any)["type"] != "input_text" {
		t.Fatalf("text content = %+v", content[0])
	}
	image := content[1].(map[string]any)
	if image["type"] != "input_image" {
		t.Fatalf("image content = %+v", image)
	}
	url := image["image_url"].(string)
	if !strings.HasPrefix(url, "data:image/png;base64,YWJj") {
		t.Fatalf("image url = %q", url)
	}
}

func TestCompleteImageUnderstandingBuildsExplicitChatImageInput(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"schema\":\"plugin_ui_reference_digest.v1\"}"}}]}`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	resp, err := client.CompleteImageUnderstanding(context.Background(), config.EngineConfig{
		BaseURL:      server.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "text-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: true, Provider: "default", BaseURL: server.URL + "/v1/chat/completions", Model: "vision-model"},
		},
	}, VisionRequest{
		Prompt: "inspect",
		Images: []ImageInput{{MIME: "image/png", DataBase64: "YWJj", Detail: "high"}},
	})
	if err != nil {
		t.Fatalf("CompleteImageUnderstanding error: %v", err)
	}
	if resp.Text == "" || resp.RawKind != "chat" {
		t.Fatalf("response = %+v", resp)
	}
	if body["model"] != "vision-model" {
		t.Fatalf("model = %v", body["model"])
	}
	messages := body["messages"].([]any)
	content := messages[0].(map[string]any)["content"].([]any)
	image := content[1].(map[string]any)
	if image["type"] != "image_url" {
		t.Fatalf("image content = %+v", image)
	}
	url := image["image_url"].(map[string]any)["url"].(string)
	if !strings.HasPrefix(url, "data:image/png;base64,YWJj") {
		t.Fatalf("image url = %q", url)
	}
}

func TestCompleteImageUnderstandingBuildsResponsesImageInput(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"ok"}]}]}`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	resp, err := client.CompleteImageUnderstanding(context.Background(), config.EngineConfig{
		BaseURL:      server.URL + "/v1/responses",
		APIKey:       "test",
		DefaultModel: "vision-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: true, Provider: "default"},
		},
	}, VisionRequest{
		Prompt: "inspect",
		Images: []ImageInput{{URL: "https://example.test/ui.png"}},
	})
	if err != nil {
		t.Fatalf("CompleteImageUnderstanding error: %v", err)
	}
	if resp.Text != "ok" || resp.RawKind != "responses" {
		t.Fatalf("response = %+v", resp)
	}
	input := body["input"].([]any)
	content := input[0].(map[string]any)["content"].([]any)
	if content[0].(map[string]any)["type"] != "input_text" {
		t.Fatalf("text content = %+v", content[0])
	}
	image := content[1].(map[string]any)
	if image["type"] != "input_image" || image["image_url"] != "https://example.test/ui.png" {
		t.Fatalf("image content = %+v", image)
	}
}

func TestCompleteImageUnderstandingParsesResponsesSSE(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.output_text.delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"{\\\"schema\\\":\"}\n\n"))
		_, _ = w.Write([]byte("event: response.output_text.delta\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"\\\"plugin_ui_reference_digest.v1\\\"}\"}\n\n"))
		_, _ = w.Write([]byte("event: response.completed\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":11,\"output_tokens\":3}}}\n\n"))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	resp, err := client.CompleteImageUnderstanding(context.Background(), config.EngineConfig{
		BaseURL:      server.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "vision-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: true, Provider: "default"},
		},
	}, VisionRequest{Prompt: "inspect", Images: []ImageInput{{URL: "https://example.test/ui.png"}}})
	if err != nil {
		t.Fatalf("CompleteImageUnderstanding error: %v", err)
	}
	if resp.Text != `{"schema":"plugin_ui_reference_digest.v1"}` {
		t.Fatalf("text = %q", resp.Text)
	}
	if resp.Usage.InputTokens != 11 || resp.Usage.OutputTokens != 3 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestCompleteImageUnderstandingReportsResponsesSSEFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: response.failed\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.failed\",\"response\":{\"status\":\"failed\",\"error\":{\"code\":\"upstream_error\",\"message\":\"Upstream request failed\"}}}\n\n"))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	_, err := client.CompleteImageUnderstanding(context.Background(), config.EngineConfig{
		BaseURL:      server.URL + "/v1",
		APIKey:       "test",
		DefaultModel: "vision-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: true, Provider: "default"},
		},
	}, VisionRequest{Prompt: "inspect", Images: []ImageInput{{URL: "https://example.test/ui.png"}}})
	if err == nil || !strings.Contains(err.Error(), "Upstream request failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestCompleteImageUnderstandingRouteUnavailable(t *testing.T) {
	client := &Client{}
	_, err := client.CompleteImageUnderstanding(context.Background(), config.EngineConfig{
		BaseURL:      "https://example.test/v1",
		APIKey:       "test",
		DefaultModel: "text-model",
		MultimodalRoutes: map[string]config.RouteConfig{
			"image_understanding": {Enabled: false, Provider: "default"},
		},
	}, VisionRequest{Prompt: "inspect", Images: []ImageInput{{URL: "https://example.test/ui.png"}}})
	if err != ErrImageUnderstandingRouteUnavailable {
		t.Fatalf("err = %v, want %v", err, ErrImageUnderstandingRouteUnavailable)
	}
}

func TestCompleteKeepsOldAPIBehavior(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"legacy ok"}}]}`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	text, err := client.Complete(context.Background(), config.EngineConfig{
		BaseURL:      server.URL,
		APIKey:       "test",
		DefaultModel: "test-model",
	}, []Message{{Role: "user", Content: "hello"}})
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}
	if text != "legacy ok" {
		t.Fatalf("text = %q", text)
	}
}

func TestCompleteRequestWritesTelemetryWithoutPromptContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent_llm_telemetry.jsonl")
	t.Setenv("VIT_AGENT_LLM_TELEMETRY_PATH", path)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices":[{"message":{"content":"ok"}}],
			"usage":{"input_tokens":20,"output_tokens":4,"input_tokens_details":{"cached_tokens":9}}
		}`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	_, err := client.CompleteRequest(context.Background(), config.EngineConfig{
		BaseURL:      server.URL,
		APIKey:       "test",
		DefaultModel: "test-model",
	}, Request{
		Messages: []Message{{Role: "user", Content: "secret prompt text"}},
		Metadata: RequestMetadata{
			Source:            "chat",
			ConversationID:    "chat_1",
			GoalID:            "goal_1",
			PromptFingerprint: "abc123",
			PromptStats:       map[string]any{"section_count": 2},
		},
	})
	if err != nil {
		t.Fatalf("CompleteRequest error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read telemetry: %v", err)
	}
	if strings.Contains(string(data), "secret prompt text") {
		t.Fatalf("telemetry leaked prompt content: %s", string(data))
	}
	var record telemetryRecord
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &record); err != nil {
		t.Fatalf("decode telemetry: %v", err)
	}
	if record.SchemaVersion != TelemetrySchemaVersion || record.Source != "chat" || record.ConversationID != "chat_1" || record.GoalID != "goal_1" {
		t.Fatalf("record metadata = %+v", record)
	}
	if record.Model != "test-model" || record.PromptFingerprint != "abc123" || record.MessageCount != 1 {
		t.Fatalf("record fields = %+v", record)
	}
	if record.Usage.InputTokens != 20 || record.Usage.OutputTokens != 4 || record.Usage.CacheReadTokens != 9 {
		t.Fatalf("record usage = %+v", record.Usage)
	}
}

func TestTelemetryDisabledDoesNotBreakRequest(t *testing.T) {
	t.Setenv("VIT_AGENT_LLM_TELEMETRY_PATH", "disabled")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	resp, err := client.CompleteRequest(context.Background(), config.EngineConfig{
		BaseURL:      server.URL,
		APIKey:       "test",
		DefaultModel: "test-model",
	}, Request{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Metadata: RequestMetadata{Source: "planner"},
	})
	if err != nil {
		t.Fatalf("CompleteRequest error: %v", err)
	}
	if resp.Text != "ok" {
		t.Fatalf("text = %q", resp.Text)
	}
}

func TestTelemetryWriteFailureDoesNotBreakRequest(t *testing.T) {
	t.Setenv("VIT_AGENT_LLM_TELEMETRY_PATH", t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client()}
	resp, err := client.CompleteRequest(context.Background(), config.EngineConfig{
		BaseURL:      server.URL,
		APIKey:       "test",
		DefaultModel: "test-model",
	}, Request{
		Messages: []Message{{Role: "user", Content: "hello"}},
		Metadata: RequestMetadata{Source: "chat"},
	})
	if err != nil {
		t.Fatalf("CompleteRequest error: %v", err)
	}
	if resp.Text != "ok" {
		t.Fatalf("text = %q", resp.Text)
	}
}

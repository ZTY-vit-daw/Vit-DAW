package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/genapi"
)

func writeGenapiConfig(t *testing.T, providers []genapi.ProviderConfig) string {
	t.Helper()
	dir := t.TempDir()
	raw, err := json.Marshal(map[string]any{"providers": providers})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "genapi_providers.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(genapi.EnvConfigPath, path)
	return path
}

func TestGenGenerateLandsArtifactsThroughExistingIngestPath(t *testing.T) {
	wav := make([]byte, 2048)
	copy(wav[0:4], "RIFF")
	copy(wav[8:12], "WAVE")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(wav)
	}))
	defer server.Close()

	cache := t.TempDir()
	writeGenapiConfig(t, []genapi.ProviderConfig{{
		ID: "local-sim", Dialect: "sync_binary", BaseURL: server.URL,
		DownloadDir: filepath.Join(cache, "dl"),
	}})

	kernel := &fakeKernelClient{replies: []map[string]any{
		projectStateReply(), // GEN-CTX-CONSUME-1: get_project_state precedes the API call
		{"status": "ok"},
		{"status": "ok", "clip_id": "1051", "asset_ref": "asset:abc", "active_take_id": "take-1"},
	}}
	h := NewWithSender(kernel, nil, nil)

	result, err := h.genGenerate(context.Background(), map[string]any{
		"provider_id": "local-sim",
		"prompt":      "steady sine",
		"track_id":    "1009",
		"start_time":  1.5,
	})
	if err != nil {
		t.Fatalf("genGenerate failed: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("result status: %#v", result)
	}
	ingested, _ := result["ingest"].([]map[string]any)
	if len(ingested) != 1 || ingested[0]["clip_id"] != "1051" || ingested[0]["asset_ref"] != "asset:abc" {
		t.Fatalf("ingest summary mismatch: %#v", result["ingest"])
	}
	// GEN-CTX-CONSUME-1: get_project_state (read) precedes register+ingest.
	if len(kernel.commands) != 3 {
		t.Fatalf("want state + register_job + ingest, got %d commands", len(kernel.commands))
	}
	if kernel.commands[0]["cmd"] != "get_project_state" {
		t.Fatalf("first command must be the state read for prompt injection: %#v", kernel.commands[0])
	}
	register := kernel.commands[1]
	if register["cmd"] != "aigc_register_job" || !strings.HasPrefix(register["job_id"].(string), "genapi-local-sim") {
		t.Fatalf("register_job mismatch: %#v", register)
	}
	if register["source"] != "genapi:local-sim" {
		t.Fatalf("register source must name the provider: %#v", register)
	}
	ingest := kernel.commands[2]
	if ingest["cmd"] != "bridge_ingest_generated_asset" {
		t.Fatalf("ingest command mismatch: %#v", ingest)
	}
	if ingest["track_id"] != "1009" || ingest["bucket"] != "generated" {
		t.Fatalf("ingest args mismatch: %#v", ingest)
	}
	if !strings.HasSuffix(ingest["file_path"].(string), ".wav") {
		t.Fatalf("ingest must carry the sniffed wav path: %#v", ingest)
	}
	if ingest["take_stack_id"] == "" || ingest["job_id"] == "" {
		t.Fatalf("ingest must carry job and take stack: %#v", ingest)
	}
}

func TestGenGenerateWithoutTrackIdSkipsIngest(t *testing.T) {
	wav := make([]byte, 64)
	copy(wav[0:4], "RIFF")
	copy(wav[8:12], "WAVE")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(wav)
	}))
	defer server.Close()

	cache := t.TempDir()
	writeGenapiConfig(t, []genapi.ProviderConfig{{
		ID: "local-sim", Dialect: "sync_binary", BaseURL: server.URL,
		DownloadDir: filepath.Join(cache, "dl"),
	}})
	kernel := &fakeKernelClient{replies: []map[string]any{projectStateReply()}}
	h := NewWithSender(kernel, nil, nil)

	result, err := h.genGenerate(context.Background(), map[string]any{
		"provider_id": "local-sim", "prompt": "x",
	})
	if err != nil {
		t.Fatalf("genGenerate failed: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("result: %#v", result)
	}
	// GEN-CTX-CONSUME-1: the state READ for prompt injection is legitimate
	// without track_id; the invariant is zero project WRITES.
	for _, c := range kernel.commands {
		switch c["cmd"] {
		case "aigc_register_job", "bridge_ingest_generated_asset":
			t.Fatalf("no track_id must mean zero kernel writes, saw %v", c["cmd"])
		}
	}
}

func TestGenGenerateUnknownProviderFailsConfigSide(t *testing.T) {
	writeGenapiConfig(t, nil)
	kernel := &fakeKernelClient{}
	h := NewWithSender(kernel, nil, nil)

	result, err := h.genGenerate(context.Background(), map[string]any{
		"provider_id": "ghost", "prompt": "x", "track_id": "1",
	})
	if err == nil {
		t.Fatal("unknown provider must fail")
	}
	payload, _ := result["result"].(map[string]any)
	if payload["error_class"] != "config_error" {
		t.Fatalf("classified payload mismatch: %#v", result)
	}
	if len(kernel.commands) != 0 {
		t.Fatalf("config failure must not touch the kernel, got %d", len(kernel.commands))
	}
}

func TestGenGenerateSurfacesProviderAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer server.Close()

	cache := t.TempDir()
	writeGenapiConfig(t, []genapi.ProviderConfig{{
		ID: "needs-key", Dialect: "sync_json_url", BaseURL: server.URL, APIKey: "wrong",
		DownloadDir: filepath.Join(cache, "dl"),
	}})
	kernel := &fakeKernelClient{replies: []map[string]any{projectStateReply()}}
	h := NewWithSender(kernel, nil, nil)

	result, err := h.genGenerate(context.Background(), map[string]any{
		"provider_id": "needs-key", "prompt": "x", "track_id": "1",
	})
	if err == nil {
		t.Fatal("401 must fail the tool")
	}
	payload, _ := result["result"].(map[string]any)
	if payload["error_class"] != "auth_error" || payload["provider_id"] != "needs-key" {
		t.Fatalf("auth failure payload mismatch: %#v", result)
	}
}

func TestGenProvidersNeverEchoesSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	writeGenapiConfig(t, []genapi.ProviderConfig{{
		ID: "prov-a", Dialect: "suno_relay", BaseURL: server.URL, APIKey: "super-secret", Model: "chirp-v3-5",
	}})
	h := NewWithSender(nil, nil, nil)

	result, err := h.genProviders()
	if err != nil {
		t.Fatalf("genProviders failed: %v", err)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "super-secret") {
		t.Fatalf("providers listing leaked the api key: %s", encoded)
	}
	providers, _ := result["providers"].([]map[string]any)
	if len(providers) != 1 || providers[0]["has_api_key"] != true || providers[0]["dialect"] != "suno_relay" {
		t.Fatalf("provider summary mismatch: %s", encoded)
	}
}

// --- GEN-CTX-CONSUME-1: unified-context first consumer (gen_api profile) ---

func projectStateReply() map[string]any {
	return map[string]any{
		"status": "ok",
		"music_context": map[string]any{
			"tempo_bpm":                  96.0,
			"tempo_setting_count":        1,
			"time_signature":             "4/4",
			"time_signature_numerator":   4,
			"time_signature_denominator": 4,
			"time_sig_setting_count":     1,
			"key_signature": map[string]any{
				"root_pitch": 60, "root_name": "C", "scale": "major", "set_by": "user",
			},
			"style_note": "暖色钢琴主导",
		},
		"markers": []any{
			map[string]any{"name": "verse", "start_seconds": 0.0, "end_seconds": 30.0},
		},
		"graph_revision": "rev-consume-1",
	}
}

// in_project: the rendered prompt must carry the auto blocks into the
// provider request body verbatim (protocol conversion must not alter them —
// design 03 §2), the reply must expose the injection evidence, and the
// artifacts still land through the existing ingest path.
func TestGenGenerateInjectsMusicContextBlocksIntoProviderRequest(t *testing.T) {
	var receivedBody string
	wav := make([]byte, 2048)
	copy(wav[0:4], "RIFF")
	copy(wav[8:12], "WAVE")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<16)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(wav)
	}))
	defer server.Close()

	writeGenapiConfig(t, []genapi.ProviderConfig{{
		ID: "local-sim", Dialect: "sync_binary", BaseURL: server.URL,
		DownloadDir: filepath.Join(t.TempDir(), "dl"),
	}})
	kernel := &fakeKernelClient{replies: []map[string]any{
		projectStateReply(), // get_project_state
		{"status": "ok"},    // aigc_register_job
		{"status": "ok", "clip_id": "1061", "asset_ref": "asset:x", "active_take_id": "take-9"},
	}}
	h := NewWithSender(kernel, nil, nil)

	result, err := h.genGenerate(context.Background(), map[string]any{
		"provider_id": "local-sim",
		"prompt":      "write a warm intro",
		"track_id":    "1001",
		"scene":       "in_project",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"== MUSIC CONTEXT (project, music.context.v1, revision rev-consume-1)",
		"tempo: 96.0 BPM",
		"key: C major (set by user)",
		"== CONSTRAINTS (machine-checked before write)",
		"write a warm intro",
	} {
		if !strings.Contains(receivedBody, want) {
			t.Errorf("provider request body lost %q (protocol conversion must not alter the auto blocks):\n%s", want, receivedBody)
		}
	}
	if strings.Index(receivedBody, "write a warm intro") < strings.Index(receivedBody, "== CONSTRAINTS") {
		t.Errorf("user free text must stay after the auto blocks:\n%s", receivedBody)
	}
	pc, _ := result["prompt_context"].(map[string]any)
	if pc == nil || pc["music_context_block"] != true || pc["constraints_block"] != true {
		t.Errorf("prompt_context evidence wrong: %v", pc)
	}
	if result["scene"] != "in_project" {
		t.Errorf("scene not surfaced: %v", result["scene"])
	}
	snap := pc["snapshot"].(map[string]any)
	if snap["tempo_bpm"] != 96.0 || snap["key_set"] != true {
		t.Errorf("snapshot evidence wrong: %v", snap)
	}
}

// free_creation: zero auto blocks (the provider body is the bare user text),
// no project-state fetch at all, and the ingest leg is refused by the
// write-front sentinel when track_id is passed.
func TestGenGenerateFreeCreationZeroInjectionAndRefusesLanding(t *testing.T) {
	var receivedBody string
	wav := make([]byte, 2048)
	copy(wav[0:4], "RIFF")
	copy(wav[8:12], "WAVE")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1<<16)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(wav)
	}))
	defer server.Close()

	writeGenapiConfig(t, []genapi.ProviderConfig{{
		ID: "local-sim", Dialect: "sync_binary", BaseURL: server.URL,
		DownloadDir: filepath.Join(t.TempDir(), "dl"),
	}})
	kernel := &fakeKernelClient{} // no queued replies — nothing may hit the kernel
	h := NewWithSender(kernel, nil, nil)

	result, err := h.genGenerate(context.Background(), map[string]any{
		"provider_id": "local-sim",
		"prompt":      "pure idea text",
		"track_id":    "1001",
		"scene":       "free_creation",
	})
	if err == nil {
		t.Fatal("free_creation with track_id must be refused")
	}
	if !strings.Contains(fmt.Sprint(result["error"]), "forbids project-write") {
		t.Errorf("refusal must be the sentinel error, got: %v", result["error"])
	}
	if strings.Contains(receivedBody, "MUSIC CONTEXT") || strings.Contains(receivedBody, "CONSTRAINTS") {
		t.Errorf("exempt scene must send zero auto context:\n%s", receivedBody)
	}
	if !strings.Contains(receivedBody, "pure idea text") {
		t.Errorf("user text must still reach the provider:\n%s", receivedBody)
	}
	for _, c := range kernel.commands {
		if name := fmt.Sprint(c["cmd"]); name == "get_project_state" || name == "bridge_ingest_generated_asset" || name == "aigc_register_job" {
			t.Errorf("exempt scene must not touch %s, but it was sent", name)
		}
	}
}

func TestGenGenerateSceneFailClosed(t *testing.T) {
	writeGenapiConfig(t, []genapi.ProviderConfig{{
		ID: "local-sim", Dialect: "sync_binary", BaseURL: "http://127.0.0.1:1",
		DownloadDir: filepath.Join(t.TempDir(), "dl"),
	}})
	h := NewWithSender(&fakeKernelClient{}, nil, nil)
	_, err := h.genGenerate(context.Background(), map[string]any{
		"provider_id": "local-sim",
		"prompt":      "x",
		"scene":       "in_projekt",
	})
	if err == nil || !strings.Contains(err.Error(), "unknown scene") {
		t.Fatalf("unknown scene must fail closed, got %v", err)
	}
}

// in_project without usable project truth fails closed before contacting the
// provider (hard fields must be present — no silent context-free generation).
func TestGenGenerateInProjectRequiresProjectTruth(t *testing.T) {
	writeGenapiConfig(t, []genapi.ProviderConfig{{
		ID: "local-sim", Dialect: "sync_binary", BaseURL: "http://127.0.0.1:1",
		DownloadDir: filepath.Join(t.TempDir(), "dl"),
	}})
	kernel := &fakeKernelClient{replies: []map[string]any{{"status": "ok"}}} // no music_context
	h := NewWithSender(kernel, nil, nil)
	result, err := h.genGenerate(context.Background(), map[string]any{
		"provider_id": "local-sim",
		"prompt":      "x",
		"track_id":    "1001",
	})
	if err == nil {
		t.Fatal("in_project without project truth must fail closed")
	}
	if !strings.Contains(fmt.Sprint(result["error"]), "requires open project truth") {
		t.Errorf("expected the project-truth refusal, got: %v", result["error"])
	}
}

// The injected blocks make in_project prompts non-empty by construction, so
// the empty-request fail-closed probe relies on this explicit free-text guard.
func TestGenGenerateEmptyPromptFailsClosed(t *testing.T) {
	writeGenapiConfig(t, []genapi.ProviderConfig{{
		ID: "local-sim", Dialect: "sync_binary", BaseURL: "http://127.0.0.1:1",
		DownloadDir: filepath.Join(t.TempDir(), "dl"),
	}})
	h := NewWithSender(&fakeKernelClient{}, nil, nil)
	result, err := h.genGenerate(context.Background(), map[string]any{
		"provider_id": "local-sim", "track_id": "1001",
	})
	if err == nil || !strings.Contains(fmt.Sprint(result["error"]), "needs a non-empty prompt") {
		t.Fatalf("empty prompt must fail closed pre-HTTP, got err=%v result=%v", err, result["error"])
	}
}

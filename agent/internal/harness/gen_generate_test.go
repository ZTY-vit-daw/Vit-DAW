package harness

import (
	"context"
	"encoding/json"
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
	if len(kernel.commands) != 2 {
		t.Fatalf("want register_job + ingest, got %d commands", len(kernel.commands))
	}
	register := kernel.commands[0]
	if register["cmd"] != "aigc_register_job" || !strings.HasPrefix(register["job_id"].(string), "genapi-local-sim") {
		t.Fatalf("register_job mismatch: %#v", register)
	}
	if register["source"] != "genapi:local-sim" {
		t.Fatalf("register source must name the provider: %#v", register)
	}
	ingest := kernel.commands[1]
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
	kernel := &fakeKernelClient{}
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
	if len(kernel.commands) != 0 {
		t.Fatalf("no track_id must mean zero kernel writes, got %d", len(kernel.commands))
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
	kernel := &fakeKernelClient{}
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

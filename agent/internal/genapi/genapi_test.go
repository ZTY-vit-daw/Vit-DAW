package genapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wavFixture returns a minimal sniffable WAV payload.
func wavFixture(size int) []byte {
	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	copy(header[8:12], "WAVE")
	body := make([]byte, size)
	for i := range body {
		body[i] = byte(i % 251)
	}
	return append(header, body...)
}

// mp3Fixture returns a minimal sniffable MP3 payload (ID3 magic).
func mp3Fixture(size int) []byte {
	out := []byte("ID3\x04\x00\x00\x00\x00\x00\x00")
	body := make([]byte, size)
	return append(out, body...)
}

func writeConfig(t *testing.T, dir string, providers []ProviderConfig) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"providers": providers})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "genapi_providers.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func baseProvider(dialect, baseURL string) ProviderConfig {
	return ProviderConfig{
		ID:          "testprov",
		Dialect:     dialect,
		BaseURL:     baseURL,
		APIKey:      "secret-key",
		DownloadDir: filepath.Join(os.TempDir(), "vit-genapi-test"), // overridden per test via t.TempDir below
	}
}

func TestSyncBinaryDownloadsWavArtifact(t *testing.T) {
	dir := t.TempDir()
	payload := wavFixture(2048)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/generate" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-key" {
			t.Errorf("bearer auth missing, got %q", got)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["prompt"] != "warm pad" {
			t.Errorf("prompt not normalized into request: %#v", body)
		}
		if body["duration"] != 12.0 {
			t.Errorf("duration_seconds must map to duration: %#v", body)
		}
		if _, ok := body["tempo"]; ok {
			t.Errorf("unset tempo_bpm must be dropped: %#v", body)
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	cfg := baseProvider("sync_binary", server.URL)
	cfg.DownloadDir = filepath.Join(dir, "cache")
	cfg.Model = "facebook/musicgen-small"
	res, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "warm pad", DurationSeconds: 12})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if len(res.Artifacts) != 1 {
		t.Fatalf("want 1 artifact, got %d", len(res.Artifacts))
	}
	a := res.Artifacts[0]
	if a.Format != "wav" || a.Bytes != int64(len(payload)) {
		t.Fatalf("artifact format/bytes mismatch: %#v", a)
	}
	if !strings.HasPrefix(a.LocalPath, filepath.Join(dir, "cache")) {
		t.Fatalf("artifact must land in configured download dir: %s", a.LocalPath)
	}
	raw, err := os.ReadFile(a.LocalPath)
	if err != nil || len(raw) != len(payload) {
		t.Fatalf("artifact file mismatch: err=%v len=%d", err, len(raw))
	}
}

func TestSyncJSONURLDownloadsFromProvider(t *testing.T) {
	dir := t.TempDir()
	audio := mp3Fixture(1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/make":
			_ = json.NewEncoder(w).Encode(map[string]any{"audio_url": serverURL(r.Host) + "/files/clip.mp3"})
		case "/files/clip.mp3":
			if got := r.Header.Get("Authorization"); got != "Bearer secret-key" {
				t.Errorf("same-host download should keep provider auth, got %q", got)
			}
			_, _ = w.Write(audio)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := baseProvider("sync_json_url", server.URL)
	cfg.GeneratePath = "/v1/make"
	cfg.DownloadDir = filepath.Join(dir, "cache")
	res, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "lo-fi beat"})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if res.Artifacts[0].Format != "mp3" {
		t.Fatalf("want mp3, got %s", res.Artifacts[0].Format)
	}
}

func TestSyncJSONBase64DecodesInlineAudio(t *testing.T) {
	dir := t.TempDir()
	audio := wavFixture(512)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "audio_base64": base64.StdEncoding.EncodeToString(audio)})
	}))
	defer server.Close()

	cfg := baseProvider("sync_json_base64", server.URL)
	cfg.DownloadDir = filepath.Join(dir, "cache")
	res, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "chime"})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if res.Artifacts[0].Format != "wav" || res.Artifacts[0].SourceURL != "inline:base64" {
		t.Fatalf("artifact mismatch: %#v", res.Artifacts[0])
	}
}

func TestAsyncPollJSONStatusFlow(t *testing.T) {
	dir := t.TempDir()
	audio := wavFixture(256)
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/generate":
			_ = json.NewEncoder(w).Encode(map[string]any{"task_id": "job-42"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/generate/job-42":
			polls++
			if polls == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{"status": "processing"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "succeeded", "output_url": serverURL(r.Host) + "/dl/42.wav"})
		case r.URL.Path == "/dl/42.wav":
			_, _ = w.Write(audio)
		default:
			t.Errorf("unexpected call %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := baseProvider("async_poll", server.URL)
	cfg.GeneratePath = "/v1/generate"
	cfg.PollPath = "/v1/generate/{task_id}"
	cfg.PollIntervalSeconds = 0.01
	cfg.DownloadDir = filepath.Join(dir, "cache")
	res, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "drone"})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if res.TaskID != "job-42" || res.Artifacts[0].Format != "wav" {
		t.Fatalf("result mismatch: task=%s artifact=%#v", res.TaskID, res.Artifacts[0])
	}
	if len(res.StatusSeen) < 2 || res.StatusSeen[0] != "processing" {
		t.Fatalf("status trail missing: %#v", res.StatusSeen)
	}
}

// serverURL rebuilds the test server URL from the request host (the server
// variable is not in scope inside the handler at definition time).
func serverURL(host string) string { return "http://" + host }

func TestAsyncPollStabilityHTTP202ThenAudio(t *testing.T) {
	dir := t.TempDir()
	audio := wavFixture(128)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "gen-7"})
			return
		}
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"in-progress"}`))
			return
		}
		w.Header().Set("Content-Type", "audio/wav")
		_, _ = w.Write(audio)
	}))
	defer server.Close()

	cfg := baseProvider("async_poll", server.URL)
	cfg.GeneratePath = "/v2beta/audio/generate"
	cfg.PollPath = "/v2beta/audio/results/{task_id}"
	cfg.TaskIDFields = []string{"id"}
	cfg.PollIntervalSeconds = 0.01
	cfg.DownloadDir = filepath.Join(dir, "cache")
	res, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "stab"})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if res.Artifacts[0].Format != "wav" || res.Artifacts[0].SourceURL != "response:body" {
		t.Fatalf("artifact mismatch: %#v", res.Artifacts[0])
	}
}

func TestSunoRelayFullFlowAndRequestShapes(t *testing.T) {
	dir := t.TempDir()
	audioA := mp3Fixture(700)
	audioB := mp3Fixture(900)
	var submitBody map[string]any
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer relay-key" {
			t.Errorf("relay bearer auth missing: %q", auth)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/suno/submit/music":
			_ = json.NewDecoder(r.Body).Decode(&submitBody)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "success", "message": "", "data": "task-s1"})
		case r.Method == http.MethodGet && r.URL.Path == "/suno/fetch/task-s1":
			polls++
			if polls == 1 {
				_ = json.NewEncoder(w).Encode(map[string]any{"task_id": "task-s1", "status": "IN_PROGRESS"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"task_id": "task-s1", "status": "SUCCESS",
				"data": []any{
					map[string]any{"id": "c1", "title": "Song A", "audio_url": serverURL(r.Host) + "/a.mp3", "metadata": map[string]any{"tags": "ambient", "duration": 91.5}},
					map[string]any{"id": "c2", "title": "Song B", "audio_url": serverURL(r.Host) + "/b.mp3", "metadata": map[string]any{"tags": "ambient", "duration": 88.0}},
				},
			})
		case r.URL.Path == "/a.mp3":
			_, _ = w.Write(audioA)
		case r.URL.Path == "/b.mp3":
			_, _ = w.Write(audioB)
		default:
			t.Errorf("unexpected call %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := ProviderConfig{
		ID: "suno-relay", Dialect: "suno_relay", BaseURL: server.URL,
		APIKey: "relay-key", Model: "chirp-v4", DownloadDir: filepath.Join(dir, "cache"),
		PollIntervalSeconds: 0.01,
	}
	res, err := Generate(context.Background(), cfg, GenerationRequest{
		Prompt: "upbeat summer pop about the sea", StyleTags: "pop", Instrumental: false,
	})
	if err != nil {
		t.Fatalf("suno relay generate failed: %v", err)
	}
	if submitBody["gpt_description_prompt"] != "upbeat summer pop about the sea" {
		t.Errorf("inspiration mode must send gpt_description_prompt: %#v", submitBody)
	}
	if submitBody["mv"] != "chirp-v4" {
		t.Errorf("model must map to mv: %#v", submitBody)
	}
	if _, ok := submitBody["prompt"]; ok {
		t.Errorf("inspiration mode must not send custom prompt: %#v", submitBody)
	}
	if len(res.Artifacts) != 2 {
		t.Fatalf("suno returns both candidates, want 2 artifacts got %d", len(res.Artifacts))
	}
	if res.Artifacts[0].Format != "mp3" || res.Artifacts[0].DurationSeconds != 91.5 {
		t.Fatalf("artifact metadata mismatch: %#v", res.Artifacts[0])
	}

	// Custom mode: lyrics present rewrites the request shape.
	submitBody = nil
	res, err = Generate(context.Background(), cfg, GenerationRequest{
		Lyrics: "[verse]\nhello", Title: "Sea", StyleTags: "pop", Instrumental: true,
	})
	if err != nil {
		t.Fatalf("custom mode failed: %v", err)
	}
	if submitBody["prompt"] != "[verse]\nhello" || submitBody["title"] != "Sea" || submitBody["tags"] != "pop" {
		t.Errorf("custom mode request mismatch: %#v", submitBody)
	}
	if submitBody["make_instrumental"] != true {
		t.Errorf("instrumental flag missing: %#v", submitBody)
	}
	if res.Artifacts == nil {
		t.Fatal("no artifacts")
	}
}

func TestSunoRelayLimitationsDisclosed(t *testing.T) {
	cfg := ProviderConfig{ID: "s", Dialect: "suno_relay", BaseURL: "http://x.example"}
	lim := cfg.Limitations(GenerationRequest{Prompt: "p", DurationSeconds: 30, TempoBPM: 120})
	if len(lim) != 2 {
		t.Fatalf("suno must disclose duration+tempo limitations, got %#v", lim)
	}
	if got := cfg.Limitations(GenerationRequest{Prompt: "p"}); len(got) != 0 {
		t.Fatalf("no limitations expected, got %#v", got)
	}
}

func TestErrorClassificationMatrix(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   ErrorClass
	}{
		{401, `{"error":{"message":"bad key"}}`, ClassAuthError},
		{403, `{"message":"forbidden"}`, ClassAuthError},
		{402, `{"message":"payment required"}`, ClassQuotaError},
		{404, `{"message":"no model"}`, ClassNotFoundError},
		{429, `{"message":"too many requests"}`, ClassRateLimitError},
		{429, `{"message":"quota exceeded for plan"}`, ClassQuotaError},
		{500, `{"message":"boom"}`, ClassProviderError},
		{200, `{"message":"content policy violation"}`, ClassProviderError}, // classifyHTTP not used on 2xx; direct check below
	}
	for _, tc := range cases[:len(cases)-1] {
		got := classifyHTTP(tc.status, []byte(tc.body))
		if got.Class != tc.want {
			t.Errorf("status %d body %s: want %s got %s", tc.status, tc.body, tc.want, got.Class)
		}
	}
	if got := moderationHint("flagged content policy violation"); !got {
		t.Error("moderation hint must trigger")
	}
}

func TestGenerateSurfacingClassifiedHTTPErrors(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		status int
		body   string
		want   ErrorClass
	}{
		{401, `{"error":{"message":"invalid api key"}}`, ClassAuthError},
		{404, `{"message":"unknown endpoint"}`, ClassNotFoundError},
		{429, `{"message":"rate limited"}`, ClassRateLimitError},
		{500, `{"message":"upstream boom"}`, ClassProviderError},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		cfg := baseProvider("sync_json_url", server.URL)
		cfg.DownloadDir = filepath.Join(dir, "cache")
		_, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "x"})
		server.Close()
		var ge *Error
		if !errors.As(err, &ge) {
			t.Fatalf("status %d: want classified *Error, got %v", tc.status, err)
		}
		if ge.Class != tc.want {
			t.Fatalf("status %d: want %s got %s (%s)", tc.status, tc.want, ge.Class, ge.Error())
		}
		if ge.HTTPStatus != tc.status {
			t.Fatalf("status %d: http status not carried", tc.status)
		}
	}
}

func TestRelayEnvelopeRejectionIsProviderError(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "insufficient_quota", "message": "no credit left"})
	}))
	defer server.Close()
	cfg := baseProvider("suno_relay", server.URL)
	cfg.DownloadDir = filepath.Join(dir, "cache")
	_, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "x"})
	var ge *Error
	if !errors.As(err, &ge) || ge.Class != ClassProviderError || !strings.Contains(ge.ProviderMessage, "insufficient_quota") {
		t.Fatalf("relay envelope rejection misclassified: %v", err)
	}
}

func TestPollFailureStatusCarriesFailReason(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "success", "data": "t9"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "FAIL", "fail_reason": "generation rejected by content moderation policy"})
	}))
	defer server.Close()
	cfg := baseProvider("suno_relay", server.URL)
	cfg.DownloadDir = filepath.Join(dir, "cache")
	cfg.PollIntervalSeconds = 0.01
	_, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "x"})
	var ge *Error
	if !errors.As(err, &ge) || ge.Class != ClassModerationError {
		t.Fatalf("fail_reason moderation must classify as moderation_error: %v", err)
	}
	if !strings.Contains(ge.ProviderMessage, "content moderation") {
		t.Fatalf("fail_reason must surface: %v", ge)
	}
}

func TestNetworkAndTimeoutClassification(t *testing.T) {
	dir := t.TempDir()
	// Connection-refused: closed server port.
	closed := httptest.NewServer(http.NewServeMux())
	closedURL := closed.URL
	closed.Close()

	cfg := baseProvider("sync_json_url", closedURL)
	cfg.DownloadDir = filepath.Join(dir, "cache")
	_, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "x"})
	var ge *Error
	if !errors.As(err, &ge) || ge.Class != ClassNetworkError || !ge.Retryable {
		t.Fatalf("refused connection must classify network_error retryable: %v", err)
	}

	// Poll deadline: provider that never finishes with a tiny timeout.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(map[string]any{"task_id": "t-slow"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "RUNNING"})
	}))
	defer server.Close()
	slow := baseProvider("async_poll", server.URL)
	slow.DownloadDir = filepath.Join(dir, "cache")
	slow.TimeoutSeconds = 1
	slow.PollIntervalSeconds = 0.2
	_, err = Generate(context.Background(), slow, GenerationRequest{Prompt: "x"})
	if !errors.As(err, &ge) || ge.Class != ClassTimeoutError {
		t.Fatalf("never-finishing poll must classify timeout: %v", err)
	}
}

func TestConfigValidationAndEnvKeyOverride(t *testing.T) {
	dir := t.TempDir()
	if err := (&ProviderConfig{ID: "x", Dialect: "nope", BaseURL: "http://a"}).Validate(); err == nil {
		t.Fatal("unknown dialect must fail")
	}
	if err := (&ProviderConfig{ID: "x", Dialect: "sync_binary"}).Validate(); err == nil {
		t.Fatal("missing base_url must fail")
	}
	if err := (&ProviderConfig{ID: "bad id!", Dialect: "sync_binary", BaseURL: "http://a"}).Validate(); err == nil {
		t.Fatal("invalid id must fail")
	}
	if err := (&ProviderConfig{ID: "x", Dialect: "sync_binary", BaseURL: "ftp://a"}).Validate(); err == nil {
		t.Fatal("non-http base_url must fail")
	}
	authed := ProviderConfig{ID: "x", Dialect: "sync_binary", BaseURL: "http://a", Auth: AuthConfig{Style: "bearer"}}
	if err := authed.Validate(); err == nil {
		t.Fatal("bearer without key must fail")
	}

	path := writeConfig(t, dir, []ProviderConfig{
		{ID: "p1", Dialect: "sync_binary", BaseURL: "http://a"},
		{ID: "p1", Dialect: "sync_binary", BaseURL: "http://b"},
	})
	if _, err := LoadProvidersFile(path); err == nil {
		t.Fatal("duplicate ids must fail")
	}

	envPath := writeConfig(t, dir, []ProviderConfig{
		{ID: "env-prov", Dialect: "sync_binary", BaseURL: "http://a", APIKey: "file-key"},
	})
	t.Setenv("VIT_GENAPI_KEY_ENV_PROV", "env-key")
	providers, err := LoadProvidersFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	if providers[0].Auth.Key() != "env-key" || !providers[0].EnvKeyApplied {
		t.Fatalf("env override must win: %#v", providers[0].Describe())
	}
	desc := providers[0].Describe()
	if fmt.Sprint(desc["has_api_key"]) != "true" || strings.Contains(fmt.Sprint(desc), "env-key") {
		t.Fatalf("describe must show key presence without the secret: %#v", desc)
	}
}

func TestExampleConfigParsesAndValidates(t *testing.T) {
	var fc fileConfig
	if err := json.Unmarshal([]byte(ExampleConfig), &fc); err != nil {
		t.Fatalf("example config must parse: %v", err)
	}
	if len(fc.Providers) != 3 {
		t.Fatalf("example config must ship three dialect examples, got %d", len(fc.Providers))
	}
	for i := range fc.Providers {
		if err := fc.Providers[i].Validate(); err != nil {
			t.Fatalf("example provider %d invalid: %v", i, err)
		}
	}
}

func TestAuthStylesApplied(t *testing.T) {
	dir := t.TempDir()
	var seen http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		_, _ = w.Write(wavFixture(64))
	}))
	defer server.Close()

	// header style
	cfg := baseProvider("sync_binary", server.URL)
	cfg.DownloadDir = filepath.Join(dir, "cache")
	cfg.Auth = AuthConfig{Style: "header", Header: "x-api-key"}
	if _, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "x"}); err != nil {
		t.Fatal(err)
	}
	if seen.Get("x-api-key") != "secret-key" || seen.Get("Authorization") != "" {
		t.Fatalf("header auth style mismatch: %#v", seen)
	}

	// cookie style (direct web-session reverse proxy deployments)
	cfg.Auth = AuthConfig{Style: "cookie", CookieName: "sunocookie"}
	if _, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "x"}); err != nil {
		t.Fatal(err)
	}
	if seen.Get("Cookie") != "sunocookie=secret-key" {
		t.Fatalf("cookie auth style mismatch: %q", seen.Get("Cookie"))
	}
}

func TestQueryAuthStyleAndParamMappingOverride(t *testing.T) {
	dir := t.TempDir()
	var gotPath string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write(wavFixture(64))
	}))
	defer server.Close()

	cfg := baseProvider("sync_binary", server.URL)
	cfg.DownloadDir = filepath.Join(dir, "cache")
	cfg.Auth = AuthConfig{Style: "query", QueryParam: "token"}
	cfg.ParamMapping = map[string]string{"duration_seconds": "length_secs", "tempo_bpm": "-"}
	cfg.ExtraParams = map[string]any{"sample_rate": 44100}
	if _, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "x", DurationSeconds: 9, TempoBPM: 100}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotPath, "token=secret-key") {
		t.Fatalf("query auth missing: %s", gotPath)
	}
	if gotBody["length_secs"] != 9.0 {
		t.Fatalf("param_mapping override not applied: %#v", gotBody)
	}
	if _, ok := gotBody["tempo"]; ok {
		t.Fatalf("'-' mapping must drop the field: %#v", gotBody)
	}
	if gotBody["sample_rate"] != 44100.0 {
		t.Fatalf("extra_params merged: %#v", gotBody)
	}
}

func TestCrossHostDownloadStripsCredential(t *testing.T) {
	dir := t.TempDir()
	audio := wavFixture(96)
	var authHeader string
	dl := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		_, _ = w.Write(audio)
	}))
	defer dl.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"audio_url": dl.URL + "/x.wav"})
	}))
	defer server.Close()

	cfg := baseProvider("sync_json_url", server.URL)
	cfg.DownloadDir = filepath.Join(dir, "cache")
	if _, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "x"}); err != nil {
		t.Fatal(err)
	}
	if authHeader != "" {
		t.Fatalf("cross-host presigned download must not carry provider credential, got %q", authHeader)
	}
}

func TestUnsupportedFormatAndSizeCap(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("this is definitely not audio data at all"))
	}))
	defer server.Close()

	cfg := baseProvider("sync_binary", server.URL)
	cfg.DownloadDir = filepath.Join(dir, "cache")
	_, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "x"})
	var ge *Error
	if !errors.As(err, &ge) || ge.Class != ClassUnsupportedFormat {
		t.Fatalf("garbage artifact must fail closed as unsupported_format: %v", err)
	}

	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(wavFixture(4096))
	}))
	defer big.Close()
	capped := baseProvider("sync_binary", big.URL)
	capped.DownloadDir = filepath.Join(dir, "cache2")
	capped.MaxArtifactBytes = 128
	_, err = Generate(context.Background(), capped, GenerationRequest{Prompt: "x"})
	if !errors.As(err, &ge) || ge.Class != ClassUnsupportedFormat {
		t.Fatalf("oversize artifact must fail closed: %v", err)
	}
}

func TestParseErrorOnNonJSONReply(t *testing.T) {
	dir := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>gateway error page</html>"))
	}))
	defer server.Close()
	cfg := baseProvider("sync_json_url", server.URL)
	cfg.DownloadDir = filepath.Join(dir, "cache")
	_, err := Generate(context.Background(), cfg, GenerationRequest{Prompt: "x"})
	var ge *Error
	if !errors.As(err, &ge) || ge.Class != ClassParseError {
		t.Fatalf("non-JSON 2xx must classify response_parse_error: %v", err)
	}
}

func TestMissingPromptFailsBeforeHTTP(t *testing.T) {
	dir := t.TempDir()
	cfg := baseProvider("sync_binary", "http://127.0.0.1:1")
	cfg.DownloadDir = filepath.Join(dir, "cache")
	_, err := Generate(context.Background(), cfg, GenerationRequest{})
	var ge *Error
	if !errors.As(err, &ge) || ge.Class != ClassConfigError {
		t.Fatalf("empty request must fail config-side: %v", err)
	}
}

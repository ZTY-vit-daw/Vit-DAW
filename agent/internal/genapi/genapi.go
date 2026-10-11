// Package genapi implements the unified generation-API adapter template
// (GEN-API-ADAPTER-1): the user fills in an API configuration
// (provider dialect / base_url / api_key / model) and every music-generation
// backend becomes usable through one code path — request construction,
// parameter normalization, response parsing, task polling, artifact download
// and error classification.
//
// Dialects (adapter shapes):
//
//	sync_binary       POST -> raw audio bytes (self-hosted MusicGen-style)
//	sync_json_url     POST -> JSON with a download URL
//	sync_json_base64  POST -> JSON with base64-encoded audio
//	async_poll        POST -> task id, poll until done (JSON status or HTTP
//	                   202/200-with-audio; Stability Stable Audio shape)
//	suno_relay        de-facto relay-station Suno surface
//	                   (/suno/submit/music + /suno/fetch/{task_id}), the
//	                   unofficial reverse-proxied web platform adapter. The
//	                   relay runs the reverse proxy; we speak its API.
//
// Artifacts land as local files; landing them on the timeline goes through
// the existing GeneratedAssetService ingest path (assets.register_job +
// assets.ingest_generated_asset), never a new kernel surface.
//
// Secrets discipline: api keys are read from the config file or per-provider
// env override and are never echoed in results, errors, or String() forms.
package genapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// GenerationRequest is the normalized, provider-independent generation
// input. Dialects map only the fields they support; unsupported fields are
// reported as limitations, not silently dropped semantics.
type GenerationRequest struct {
	Prompt          string  // free-form description (inspiration mode)
	Lyrics          string  // custom-mode lyrics (suno custom mode)
	StyleTags       string  // style / genre tags
	Title           string  // work title
	Instrumental    bool    // instrumental-only generation
	DurationSeconds float64 // target duration
	TempoBPM        float64 // target tempo
	MusicalKey      string  // target key, e.g. "C minor"
	Count           int     // requested candidate count
}

func (r GenerationRequest) essentiallyEmpty() bool {
	return strings.TrimSpace(r.Prompt) == "" && strings.TrimSpace(r.Lyrics) == ""
}

// Artifact is one downloaded generation product.
type Artifact struct {
	Kind            string // "audio"
	Format          string // wav / mp3 / ogg / flac / mid
	LocalPath       string // absolute path of the downloaded file
	SourceURL       string // provider URL the file came from (when applicable)
	Bytes           int64
	Title           string
	DurationSeconds float64
}

// Result is the adapter outcome; IngestInfo is filled by the harness layer.
type Result struct {
	ProviderID string
	Dialect    string
	Model      string
	TaskID     string
	Artifacts  []Artifact
	StatusSeen []string // bounded poll status trail (diagnostics)
}

// Generate runs the full unified pipeline for one provider config:
// submit/parse/poll/download with classified errors.
func Generate(ctx context.Context, cfg ProviderConfig, req GenerationRequest) (*Result, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if req.essentiallyEmpty() {
		return nil, &Error{Class: ClassConfigError, ProviderMessage: "generation request needs a prompt or lyrics"}
	}
	resolved := cfg.resolved()
	deadline := time.Duration(resolved.TimeoutSeconds) * time.Second
	if deadline <= 0 {
		deadline = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	res := &Result{ProviderID: cfg.ID, Dialect: cfg.Dialect, Model: cfg.Model}

	taskID, direct, err := resolved.submit(ctx, req, res)
	if err != nil {
		return nil, err
	}
	res.TaskID = taskID

	var artifacts []Artifact
	if direct != nil {
		artifacts = direct
	} else {
		artifacts, err = resolved.poll(ctx, taskID, res)
		if err != nil {
			return nil, err
		}
	}
	if len(artifacts) == 0 {
		return nil, &Error{Class: ClassParseError, ProviderMessage: "provider finished without audio artifacts"}
	}
	for i := range artifacts {
		if artifacts[i].Title == "" {
			artifacts[i].Title = fmt.Sprintf("%s %s", cfg.ID, artifacts[i].Format)
		}
	}
	res.Artifacts = artifacts
	return res, nil
}

// client builds the HTTP client honoring the provider timeout budget.
func (c *clientConfig) httpClient() *http.Client {
	perCall := time.Duration(c.PerCallTimeoutSeconds) * time.Second
	if perCall <= 0 {
		perCall = 60 * time.Second
	}
	return &http.Client{Timeout: perCall, Transport: http.DefaultTransport}
}

// call performs one HTTP call with auth applied and classifies transport and
// HTTP-status failures. The body is returned raw.
func (c *clientConfig) call(ctx context.Context, method, rawURL string, body io.Reader, contentType string) (*http.Response, []byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("invalid url %q: %v", rawURL, err)}
	}
	if c.Auth.Style == authQuery && c.Auth.Key() != "" {
		q := parsed.Query()
		q.Set(c.Auth.QueryParam, c.Auth.Key())
		parsed.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, parsed.String(), body)
	if err != nil {
		return nil, nil, &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("build request: %v", err)}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if key := c.Auth.Key(); key != "" {
		switch c.Auth.Style {
		case authBearer:
			req.Header.Set("Authorization", "Bearer "+key)
		case authHeader:
			req.Header.Set(c.Auth.Header, key)
		case authCookie:
			req.Header.Set("Cookie", c.Auth.cookieName()+"="+key)
		}
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("User-Agent", "vit-daw-genapi/1")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, &Error{Class: ClassTimeoutError, Retryable: true, ProviderMessage: "generation deadline exceeded", Cause: ctx.Err()}
		}
		return nil, nil, &Error{Class: ClassNetworkError, Retryable: true, ProviderMessage: err.Error(), Cause: err}
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	_ = resp.Body.Close()
	if err != nil {
		return nil, nil, &Error{Class: ClassNetworkError, Retryable: true, ProviderMessage: fmt.Sprintf("read response body: %v", err)}
	}
	if resp.StatusCode >= 400 {
		return resp, payload, classifyHTTP(resp.StatusCode, payload)
	}
	return resp, payload, nil
}

// postJSON submits the generation request and decodes the JSON reply.
func (c *clientConfig) postJSON(ctx context.Context, pathTpl string, payload map[string]any) (map[string]any, []byte, error) {
	endpoint := c.joinURL(pathTpl, "")
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("encode request: %v", err)}
		}
		body = strings.NewReader(string(encoded))
	}
	_, raw, err := c.call(ctx, http.MethodPost, endpoint, body, "application/json")
	if err != nil {
		return nil, raw, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, raw, &Error{
			Class:           ClassParseError,
			ProviderMessage: fmt.Sprintf("non-JSON reply from %s: %.200s", endpoint, printable(raw)),
			Retryable:       false,
		}
	}
	// Relay-station convention: {"code":"success","message":...} envelope;
	// a non-success code with 2xx is a provider-reported failure.
	if code, ok := firstStringPath(decoded, "code"); ok && !successEnvelopeCode(code) {
		msg, _ := firstStringPath(decoded, "message")
		return decoded, raw, &Error{Class: ClassProviderError, ProviderMessage: fmt.Sprintf("relay rejected submit (code=%s): %s", code, msg)}
	}
	return decoded, raw, nil
}

func successEnvelopeCode(code string) bool {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "", "success", "ok", "0", "200":
		return true
	}
	return false
}

// joinURL resolves an endpoint path template (may contain {task_id}) against
// the provider base URL.
func (c *clientConfig) joinURL(pathTpl, taskID string) string {
	p := pathTpl
	if taskID != "" {
		p = strings.ReplaceAll(p, "{task_id}", url.PathEscape(taskID))
	}
	if p == "" {
		p = "/"
	}
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	return strings.TrimRight(c.BaseURL, "/") + ensureSlash(p)
}

func ensureSlash(p string) string {
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}

func printable(raw []byte) string {
	s := strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' {
			return ' '
		}
		return r
	}, string(raw))
	return strings.TrimSpace(s)
}

// downloadArtifact fetches one artifact URL to the provider download dir,
// sniffs the audio format and enforces the size cap. Auth headers are only
// sent when the artifact host matches the provider host (presigned
// third-party download hosts must not receive the provider credential).
func (c *clientConfig) downloadArtifact(ctx context.Context, sourceURL, nameStem string, hint Artifact) (Artifact, error) {
	if sourceURL != "" && !strings.HasPrefix(sourceURL, "http://") && !strings.HasPrefix(sourceURL, "https://") {
		return Artifact{}, &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("artifact url is not http(s): %q", sourceURL)}
	}
	noAuth := false
	if sourceURL != "" {
		if base, err := url.Parse(c.BaseURL); err == nil {
			if target, err := url.Parse(sourceURL); err == nil {
				noAuth = !strings.EqualFold(target.Host, base.Host)
			}
		}
	}
	savedAuth := c.Auth
	if noAuth {
		c.Auth = AuthConfig{Style: authNone}
	}
	_, raw, err := c.call(ctx, http.MethodGet, sourceURL, nil, "")
	c.Auth = savedAuth
	if err != nil {
		return Artifact{}, err
	}
	format, sniffErr := sniffAudioFormat(raw)
	if sniffErr != nil {
		return Artifact{}, &Error{Class: ClassUnsupportedFormat, ProviderMessage: sniffErr.Error()}
	}
	if int64(len(raw)) > c.MaxBytes() {
		return Artifact{}, &Error{Class: ClassUnsupportedFormat, ProviderMessage: fmt.Sprintf("artifact %d bytes exceeds cap %d", len(raw), c.MaxBytes())}
	}
	name := path.Join(c.downloadDir(), sanitizeFileStem(nameStem)+"."+format)
	if werr := writeExclusively(name, raw); werr != nil {
		return Artifact{}, &Error{Class: ClassConfigError, ProviderMessage: werr.Error()}
	}
	return Artifact{
		Kind:            "audio",
		Format:          format,
		LocalPath:       name,
		SourceURL:       sourceURL,
		Bytes:           int64(len(raw)),
		Title:           hint.Title,
		DurationSeconds: hint.DurationSeconds,
	}, nil
}

// decodeBase64Artifact decodes an inline base64 audio payload and stores it
// through the same sniff/cap/save path as downloaded artifacts.
func (c *clientConfig) decodeBase64Artifact(ctx context.Context, b64 string, nameStem string, hint Artifact) (Artifact, error) {
	_ = ctx
	raw, err := decodeBase64Flexible(b64)
	if err != nil {
		return Artifact{}, &Error{Class: ClassParseError, ProviderMessage: fmt.Sprintf("decode base64 audio: %v", err)}
	}
	format, sniffErr := sniffAudioFormat(raw)
	if sniffErr != nil {
		return Artifact{}, &Error{Class: ClassUnsupportedFormat, ProviderMessage: sniffErr.Error()}
	}
	if int64(len(raw)) > c.MaxBytes() {
		return Artifact{}, &Error{Class: ClassUnsupportedFormat, ProviderMessage: fmt.Sprintf("artifact %d bytes exceeds cap %d", len(raw), c.MaxBytes())}
	}
	name := path.Join(c.downloadDir(), sanitizeFileStem(nameStem)+"."+format)
	if werr := writeExclusively(name, raw); werr != nil {
		return Artifact{}, &Error{Class: ClassConfigError, ProviderMessage: werr.Error()}
	}
	return Artifact{
		Kind:            "audio",
		Format:          format,
		LocalPath:       name,
		SourceURL:       "inline:base64",
		Bytes:           int64(len(raw)),
		Title:           hint.Title,
		DurationSeconds: hint.DurationSeconds,
	}, nil
}

package genapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout          = 300 * time.Second
	defaultPollIntervalSecs = 3.0
	maxBodyBytes            = 128 << 20 // 128 MiB read cap for one HTTP body
	defaultMaxArtifactBytes = 200 << 20
)

// dialect captures one adapter shape's defaults; user config overlays these.
type dialect struct {
	generatePath string
	pollPath     string
	pollMethod   string
	taskIDFields []string
	statusField  string
	doneStatuses []string
	failStatuses []string
	urlFields    []string
	base64Fields []string
	pollInterval float64
	timeoutSecs  int
	binarySubmit bool // sync_binary: submit reply IS the artifact
	pollHTTPDone bool // async_poll http-202/200-audio semantics (Stability)
	buildRequest func(d dialect, req GenerationRequest, cfg ProviderConfig) map[string]any
}

var dialectRegistry = map[string]dialect{
	// Self-hosted MusicGen-style: POST prompt -> audio bytes.
	"sync_binary": {
		generatePath: "/generate",
		binarySubmit: true,
		buildRequest: genericRequestBuilder,
	},
	// POST -> JSON containing artifact URL(s).
	"sync_json_url": {
		generatePath: "/generate",
		urlFields:    []string{"audio_url", "url", "output_url", "result_url", "data.url", "data.audio_url", "data[].url", "data[].audio_url", "artifacts[].url", "output[].url"},
		buildRequest: genericRequestBuilder,
	},
	// POST -> JSON containing base64 audio (Stability sync, Gemini wrappers).
	"sync_json_base64": {
		generatePath: "/generate",
		base64Fields: []string{"audio_base64", "b64_json", "audio", "data.b64_json", "data.audio_base64", "data[].b64_json", "artifacts[].base64", "candidates[].content.parts[].inline_data.data"},
		buildRequest: genericRequestBuilder,
	},
	// POST -> task id, poll until done (Replicate/MusicGPT-ish JSON status;
	// also tolerates Stability's HTTP-202 in-progress / HTTP-200 audio).
	"async_poll": {
		generatePath: "/generate",
		pollPath:     "/generate/{task_id}",
		pollMethod:   http.MethodGet,
		taskIDFields: []string{"task_id", "id", "job_id", "prediction_id", "data"},
		statusField:  "status",
		doneStatuses: []string{"succeeded", "success", "complete", "completed", "done", "finished"},
		failStatuses: []string{"failed", "fail", "error", "errored", "canceled", "cancelled", "terminated"},
		urlFields:    []string{"output_url", "audio_url", "url", "result_url", "output", "data.audio_url", "data[].audio_url", "data[].url", "artifacts[].url", "output[].url", "output[].audio_url"},
		base64Fields: []string{"audio_base64", "b64_json", "data.b64_json"},
		pollHTTPDone: true,
		pollInterval: 3,
		timeoutSecs:  600,
		buildRequest: genericRequestBuilder,
	},
	// Suno through a relay station (reverse proxy of the web platform).
	// De-facto surface popularized by new-api / suno-api relays:
	//   POST /suno/submit/music            -> {"code":"success","data":"<task_id>"}
	//   GET  /suno/fetch/{task_id}         -> task{status IN_PROGRESS|SUCCESS|FAIL,
	//                                          fail_reason, data[]{audio_url,...}}
	"suno_relay": {
		generatePath: "/suno/submit/music",
		pollPath:     "/suno/fetch/{task_id}",
		pollMethod:   http.MethodGet,
		taskIDFields: []string{"data", "task_id", "id"},
		statusField:  "status",
		doneStatuses: []string{"success", "completed", "complete"},
		failStatuses: []string{"fail", "failure", "failed", "error"},
		urlFields:    []string{"data[].audio_url", "audio_url", "data[].source_audio_url"},
		pollInterval: 5,
		timeoutSecs:  600,
		buildRequest: sunoRequestBuilder,
	},
}

func knownDialects() string {
	names := make([]string, 0, len(dialectRegistry))
	for name := range dialectRegistry {
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}

// genericRequestBuilder performs parameter normalization for the unified
// template: normalized request fields mapped onto provider field names,
// empty fields dropped, config overrides applied last.
func genericRequestBuilder(d dialect, req GenerationRequest, cfg ProviderConfig) map[string]any {
	mapping := map[string]string{
		"duration_seconds": "duration",
		"tempo_bpm":        "tempo",
		"style_tags":       "style",
		"musical_key":      "key",
		"title":            "title",
		"instrumental":     "instrumental",
		"count":            "count",
		"lyrics":           "lyrics",
	}
	for k, v := range cfg.ParamMapping {
		mapping[k] = v
	}
	body := map[string]any{"prompt": strings.TrimSpace(req.Prompt)}
	setIfSet(body, mapping, "duration_seconds", req.DurationSeconds)
	setIfSet(body, mapping, "tempo_bpm", req.TempoBPM)
	setIfSet(body, mapping, "style_tags", req.StyleTags)
	setIfSet(body, mapping, "musical_key", req.MusicalKey)
	setIfSet(body, mapping, "title", req.Title)
	if req.Instrumental {
		setIfSet(body, mapping, "instrumental", true)
	}
	if req.Count > 0 {
		setIfSet(body, mapping, "count", req.Count)
	}
	setIfSet(body, mapping, "lyrics", req.Lyrics)
	if strings.TrimSpace(cfg.Model) != "" {
		body["model"] = strings.TrimSpace(cfg.Model)
	}
	for k, v := range cfg.ExtraParams {
		body[k] = v
	}
	return body
}

// sunoRequestBuilder maps the normalized request onto the relay surface:
// lyrics present = custom mode (prompt+tags+title), otherwise inspiration
// mode (gpt_description_prompt). duration/tempo are not relay-expressible
// and surface as a limitation, not a silently-dropped field.
func sunoRequestBuilder(d dialect, req GenerationRequest, cfg ProviderConfig) map[string]any {
	body := map[string]any{}
	mv := strings.TrimSpace(cfg.Model)
	if mv == "" {
		mv = "chirp-v3-5"
	}
	body["mv"] = mv
	if strings.TrimSpace(req.Lyrics) != "" {
		body["prompt"] = req.Lyrics
		if strings.TrimSpace(req.Title) != "" {
			body["title"] = strings.TrimSpace(req.Title)
		}
		if strings.TrimSpace(req.StyleTags) != "" {
			body["tags"] = strings.TrimSpace(req.StyleTags)
		}
	} else {
		body["gpt_description_prompt"] = strings.TrimSpace(req.Prompt)
	}
	if req.Instrumental {
		body["make_instrumental"] = true
	}
	for k, v := range cfg.ExtraParams {
		body[k] = v
	}
	return body
}

func setIfSet(body map[string]any, mapping map[string]string, normalized string, value any) {
	name, ok := mapping[normalized]
	if !ok || strings.TrimSpace(name) == "" || name == "-" {
		return
	}
	switch v := value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return
		}
	case float64:
		if v == 0 {
			return
		}
	case int:
		if v == 0 {
			return
		}
	case bool:
	default:
		return
	}
	body[name] = value
}

// clientConfig is the resolved (defaults + user overlay) adapter runtime.
type clientConfig struct {
	ProviderConfig
	dialect dialect
}

// resolved overlays dialect defaults with the user config.
func (p ProviderConfig) resolved() *clientConfig {
	d, ok := dialectRegistry[p.Dialect]
	if !ok {
		d = dialectRegistry["sync_binary"]
	}
	c := &clientConfig{ProviderConfig: p, dialect: d}
	if strings.TrimSpace(c.GeneratePath) == "" {
		c.GeneratePath = d.generatePath
	}
	if strings.TrimSpace(c.PollPath) == "" {
		c.PollPath = d.pollPath
	}
	if strings.TrimSpace(c.PollMethod) == "" {
		c.PollMethod = d.pollMethod
	}
	if len(c.TaskIDFields) == 0 {
		c.TaskIDFields = d.taskIDFields
	}
	if strings.TrimSpace(c.StatusField) == "" {
		c.StatusField = d.statusField
	}
	if len(c.DoneStatuses) == 0 {
		c.DoneStatuses = d.doneStatuses
	}
	if len(c.FailStatuses) == 0 {
		c.FailStatuses = d.failStatuses
	}
	if len(c.URLFields) == 0 {
		c.URLFields = d.urlFields
	}
	if len(c.Base64Fields) == 0 {
		c.Base64Fields = d.base64Fields
	}
	if c.PollIntervalSeconds <= 0 {
		if d.pollInterval > 0 {
			c.PollIntervalSeconds = d.pollInterval
		} else {
			c.PollIntervalSeconds = defaultPollIntervalSecs
		}
	}
	if c.TimeoutSeconds <= 0 {
		if d.timeoutSecs > 0 {
			c.TimeoutSeconds = d.timeoutSecs
		} else {
			c.TimeoutSeconds = int(defaultTimeout.Seconds())
		}
	}
	if c.Auth.Style == "" {
		if c.Auth.Key() == "" {
			c.Auth.Style = authNone
		} else {
			c.Auth.Style = authBearer
		}
	}
	if c.Auth.Style == authHeader && strings.TrimSpace(c.Auth.Header) == "" {
		c.Auth.Header = "x-api-key"
	}
	if c.Auth.Style == authQuery && strings.TrimSpace(c.Auth.QueryParam) == "" {
		c.Auth.QueryParam = "api_key"
	}
	return c
}

// Limitations reports normalized fields this dialect cannot express, so
// callers can disclose instead of silently dropping them.
func (p ProviderConfig) Limitations(req GenerationRequest) []string {
	if p.Dialect != "suno_relay" {
		return nil
	}
	var out []string
	if req.DurationSeconds > 0 {
		out = append(out, "duration_seconds: the suno relay surface has no duration parameter")
	}
	if req.TempoBPM > 0 {
		out = append(out, "tempo_bpm: the suno relay surface has no tempo parameter")
	}
	return out
}

// submit sends the generation request. For sync dialects it also harvests
// the artifacts directly; for async dialects it returns the task id.
func (c *clientConfig) submit(ctx context.Context, req GenerationRequest, res *Result) (taskID string, direct []Artifact, err error) {
	body := c.dialect.buildRequest(c.dialect, req, c.ProviderConfig)
	if c.dialect.binarySubmit {
		// The reply IS the artifact: send JSON, accept raw audio bytes.
		endpoint := c.joinURL(c.GeneratePath, "")
		encoded, merr := json.Marshal(body)
		if merr != nil {
			return "", nil, &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("encode request: %v", merr)}
		}
		_, raw, err := c.call(ctx, http.MethodPost, endpoint, strings.NewReader(string(encoded)), "application/json")
		if err != nil {
			return "", nil, err
		}
		artifact, aerr := c.artifactFromRaw(ctx, raw, c.stem("", 0), Artifact{})
		if aerr != nil {
			return "", nil, aerr
		}
		return "", []Artifact{artifact}, nil
	}
	resp, raw, err := c.postJSON(ctx, c.GeneratePath, body)
	if err != nil {
		return "", nil, err
	}
	// Sync URL / base64 dialects finish on the submit reply.
	if !c.isAsync() {
		artifacts, perr := c.harvest(ctx, resp, raw, res)
		if perr != nil {
			return "", nil, perr
		}
		return taskIDFrom(resp, c.TaskIDFields), artifacts, nil
	}
	id := taskIDFrom(resp, c.TaskIDFields)
	if strings.TrimSpace(id) == "" {
		return "", nil, &Error{Class: ClassParseError, ProviderMessage: fmt.Sprintf("async dialect %s: no task id in submit reply (fields tried: %s): %.200s", c.Dialect, strings.Join(c.TaskIDFields, ","), printable(raw))}
	}
	return id, nil, nil
}

func (c *clientConfig) isAsync() bool {
	return strings.TrimSpace(c.PollPath) != ""
}

// poll drives the task to completion and downloads the artifacts.
func (c *clientConfig) poll(ctx context.Context, taskID string, res *Result) ([]Artifact, error) {
	interval := time.Duration(c.PollIntervalSeconds * float64(time.Second))
	if interval < 500*time.Millisecond {
		interval = 500 * time.Millisecond
	}
	pollURL := c.joinURL(c.PollPath, taskID)
	method := c.PollMethod
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	contentType := ""
	if method == http.MethodPost {
		contentType = "application/json"
		if len(c.PollBody) > 0 {
			if encoded, merr := json.Marshal(c.PollBody); merr == nil {
				body = strings.NewReader(string(encoded))
			}
		}
	}
	for attempt := 0; ; attempt++ {
		resp, raw, err := c.call(ctx, method, pollURL, body, contentType)
		if err != nil {
			if attempt == 0 && res != nil {
				res.StatusSeen = append(res.StatusSeen, "poll_error:"+string(AsError(err).Class))
			}
			// Transient poll transport failures get bounded retries.
			if ge := AsError(err); ge.Retryable && attempt < 3 {
				if !sleepCtx(ctx, interval) {
					return nil, &Error{Class: ClassTimeoutError, Retryable: true, ProviderMessage: "generation deadline exceeded during poll retries"}
				}
				continue
			}
			return nil, err
		}
		// Stability-style: 202 = still running, 200 + audio body = done.
		if c.dialect.pollHTTPDone && resp.StatusCode == http.StatusAccepted {
			res.StatusSeen = append(res.StatusSeen, "http_202_in_progress")
			if !sleepCtx(ctx, interval) {
				return nil, &Error{Class: ClassTimeoutError, Retryable: true, ProviderMessage: "generation deadline exceeded while waiting for result"}
			}
			continue
		}
		if isAudioContentType(resp.Header.Get("Content-Type")) {
			artifact, aerr := c.artifactFromRaw(ctx, raw, c.stem(taskID, 0), Artifact{})
			if aerr != nil {
				return nil, aerr
			}
			if res != nil {
				res.StatusSeen = append(res.StatusSeen, "http_200_audio")
			}
			return []Artifact{artifact}, nil
		}
		var decoded map[string]any
		if jerr := json.Unmarshal(raw, &decoded); jerr != nil {
			return nil, &Error{Class: ClassParseError, ProviderMessage: fmt.Sprintf("poll reply is not JSON: %.200s", printable(raw))}
		}
		status := strings.ToLower(strings.TrimSpace(stringAt(decoded, c.StatusField)))
		if status != "" && res != nil {
			res.StatusSeen = append(res.StatusSeen, status)
		}
		if status != "" && containsFold(c.FailStatuses, status) {
			reason := ""
			if v, ok := lookupPath(decoded, "fail_reason"); ok {
				if s, ok := v.(string); ok {
					reason = s
				}
			}
			if reason == "" {
				reason = extractProviderMessage(raw)
			}
			klass := ClassProviderError
			if moderationHint(reason) {
				klass = ClassModerationError
			}
			return nil, &Error{Class: klass, ProviderMessage: fmt.Sprintf("provider task %s reported failure (%s): %s", taskID, status, reason)}
		}
		if done := status == "" || containsFold(c.DoneStatuses, status); done {
			artifacts, perr := c.harvest(ctx, decoded, raw, res)
			if perr != nil {
				// Done but no artifacts yet is a real terminal mismatch for
				// a done-marked status; for empty-status JSON it may just be
				// an intermediate shape — keep polling on empty harvest only
				// when a status field exists and is not done.
				if status == "" {
					if !sleepCtx(ctx, interval) {
						return nil, &Error{Class: ClassTimeoutError, Retryable: true, ProviderMessage: "generation deadline exceeded while polling"}
					}
					continue
				}
				return nil, perr
			}
			return artifacts, nil
		}
		if !sleepCtx(ctx, interval) {
			return nil, &Error{Class: ClassTimeoutError, Retryable: true, ProviderMessage: fmt.Sprintf("generation deadline exceeded while task %s was %s", taskID, status)}
		}
	}
}

// harvest extracts artifacts from a completed JSON document: URLs first,
// then inline base64 payloads.
func (c *clientConfig) harvest(ctx context.Context, decoded map[string]any, raw []byte, res *Result) ([]Artifact, error) {
	_ = raw
	var artifacts []Artifact
	for i, u := range collectStrings(decoded, c.URLFields) {
		if strings.TrimSpace(u) == "" || !strings.HasPrefix(u, "http") {
			continue
		}
		hint := Artifact{Title: titleNear(decoded, c.URLFields[i]), DurationSeconds: durationNear(decoded)}
		artifact, err := c.downloadArtifact(ctx, u, c.stem(taskIDFrom(decoded, c.TaskIDFields), len(artifacts)), hint)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	if len(artifacts) == 0 {
		for i, b64 := range collectStrings(decoded, c.Base64Fields) {
			if strings.TrimSpace(b64) == "" {
				continue
			}
			artifact, err := c.decodeBase64Artifact(ctx, b64, c.stem(taskIDFrom(decoded, c.TaskIDFields), i), Artifact{})
			if err != nil {
				return nil, err
			}
			artifacts = append(artifacts, artifact)
			break
		}
	}
	return artifacts, nil
}

// artifactFromRaw sniffs and stores a raw (binary) submit/poll body.
func (c *clientConfig) artifactFromRaw(ctx context.Context, raw []byte, stem string, hint Artifact) (Artifact, error) {
	format, err := sniffAudioFormat(raw)
	if err != nil {
		return Artifact{}, &Error{Class: ClassUnsupportedFormat, ProviderMessage: err.Error()}
	}
	if int64(len(raw)) > c.MaxBytes() {
		return Artifact{}, &Error{Class: ClassUnsupportedFormat, ProviderMessage: fmt.Sprintf("artifact %d bytes exceeds cap %d", len(raw), c.MaxBytes())}
	}
	name := joinDownload(c.downloadDir(), sanitizeFileStem(stem)+"."+format)
	if werr := writeExclusively(name, raw); werr != nil {
		return Artifact{}, &Error{Class: ClassConfigError, ProviderMessage: werr.Error()}
	}
	return Artifact{Kind: "audio", Format: format, LocalPath: name, SourceURL: "response:body", Bytes: int64(len(raw)), Title: hint.Title, DurationSeconds: hint.DurationSeconds}, nil
}

// stem builds a deterministic artifact filename stem.
func (c *clientConfig) stem(taskID string, index int) string {
	stem := c.ID
	if strings.TrimSpace(taskID) != "" {
		stem += "-" + strings.TrimSpace(taskID)
	}
	if index > 0 {
		stem += fmt.Sprintf("-%d", index)
	}
	return stem
}

func (c *clientConfig) MaxBytes() int64 {
	if c.MaxArtifactBytes > 0 {
		return c.MaxArtifactBytes
	}
	return defaultMaxArtifactBytes
}

func (c *clientConfig) downloadDir() string {
	return resolvedDownloadDir(c.ID, c.DownloadDir)
}

func containsFold(list []string, s string) bool {
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), s) {
			return true
		}
	}
	return false
}

func isAudioContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	return strings.HasPrefix(ct, "audio/") || strings.Contains(ct, "octet-stream")
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

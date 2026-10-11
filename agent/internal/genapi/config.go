package genapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// EnvConfigPath overrides the provider config file location (tests and
// run-scoped smoke workdirs use this so ~/.vit is never touched).
const EnvConfigPath = "VIT_GENAPI_CONFIG"

// DefaultConfigPath is the user-facing config surface: one JSON file the
// user fills with provider entries (provider/base_url/api_key/model) and
// everything else is normalized here.
func DefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("genapi: user home: %w", err)
	}
	return filepath.Join(home, ".vit", "genapi_providers.json"), nil
}

// ExampleConfig is a documented starter configuration shipped in source so
// the user can copy it into ~/.vit/genapi_providers.json and fill the blanks.
const ExampleConfig = `{
  "providers": [
    {
      "id": "my-suno-relay",
      "dialect": "suno_relay",
      "base_url": "https://your-relay.example.com",
      "api_key": "sk-your-relay-key",
      "model": "chirp-v3-5",
      "poll_interval_seconds": 5,
      "timeout_seconds": 600
    },
    {
      "id": "stable-audio",
      "dialect": "async_poll",
      "base_url": "https://api.stability.ai",
      "generate_path": "/v2beta/audio/generate",
      "poll_path": "/v2beta/audio/results/{task_id}",
      "api_key": "sk-your-stability-key",
      "model": "stable-audio-2.5",
      "param_mapping": { "duration_seconds": "duration", "style_tags": "prompt_suffix" }
    },
    {
      "id": "local-musicgen",
      "dialect": "sync_binary",
      "base_url": "http://127.0.0.1:8000",
      "generate_path": "/generate",
      "api_key": "",
      "model": "facebook/musicgen-small"
    }
  ]
}`

// AuthConfig selects how the credential is attached to requests.
type AuthConfig struct {
	// Style: "" (auto: bearer when a key is present, none otherwise),
	// "bearer", "header", "query", "cookie", "none".
	Style string `json:"style,omitempty"`
	// Header is the header name for style=header (default x-api-key).
	Header string `json:"header,omitempty"`
	// QueryParam is the query parameter for style=query (default api_key).
	QueryParam string `json:"query_param,omitempty"`
	// CookieName is the cookie name for style=cookie (default session) —
	// the direct web-session reverse-proxy deployments use this.
	CookieName string `json:"cookie_name,omitempty"`
	// value carries the secret; never serialized back out.
	value string `json:"-"`
}

// Key returns the resolved credential with a copy-safe accessor.
func (a AuthConfig) Key() string { return a.value }

func (a AuthConfig) cookieName() string {
	if strings.TrimSpace(a.CookieName) != "" {
		return strings.TrimSpace(a.CookieName)
	}
	return "session"
}

const (
	authBearer = "bearer"
	authHeader = "header"
	authQuery  = "query"
	authCookie = "cookie"
	authNone   = "none"
)

// ProviderConfig is one user-filled adapter entry.
type ProviderConfig struct {
	ID      string            `json:"id"`
	Dialect string            `json:"dialect"`
	BaseURL string            `json:"base_url"`
	APIKey  string            `json:"api_key,omitempty"`
	Model   string            `json:"model,omitempty"`
	Auth    AuthConfig        `json:"auth,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`

	GeneratePath string         `json:"generate_path,omitempty"`
	PollPath     string         `json:"poll_path,omitempty"`
	PollMethod   string         `json:"poll_method,omitempty"`
	PollBody     map[string]any `json:"poll_body,omitempty"`

	ParamMapping map[string]string `json:"param_mapping,omitempty"`
	ExtraParams  map[string]any    `json:"extra_params,omitempty"`

	TaskIDFields []string `json:"task_id_fields,omitempty"`
	StatusField  string   `json:"status_field,omitempty"`
	DoneStatuses []string `json:"done_statuses,omitempty"`
	FailStatuses []string `json:"fail_statuses,omitempty"`
	URLFields    []string `json:"url_fields,omitempty"`
	Base64Fields []string `json:"base64_fields,omitempty"`

	PollIntervalSeconds   float64 `json:"poll_interval_seconds,omitempty"`
	TimeoutSeconds        int     `json:"timeout_seconds,omitempty"`
	PerCallTimeoutSeconds int     `json:"per_call_timeout_seconds,omitempty"`
	MaxArtifactBytes      int64   `json:"max_artifact_bytes,omitempty"`
	DownloadDir           string  `json:"download_dir,omitempty"`

	// EnvKeyApplied marks that the credential came from the per-provider
	// env override (diagnostics only, no secret material).
	EnvKeyApplied bool `json:"-"`
}

// fileConfig mirrors the on-disk JSON shape.
type fileConfig struct {
	Providers []ProviderConfig `json:"providers"`
}

// LoadProvidersFile reads and validates a provider config file.
func LoadProvidersFile(path string) ([]ProviderConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("no genapi provider config at %s (copy the ExampleConfig from agent/internal/genapi/config.go and fill provider/base_url/api_key/model)", path)}
		}
		return nil, &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("read %s: %v", path, err)}
	}
	var fc fileConfig
	if err := json.Unmarshal(raw, &fc); err != nil {
		return nil, &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("parse %s: %v", path, err)}
	}
	seen := map[string]bool{}
	for i := range fc.Providers {
		p := &fc.Providers[i]
		applyEnvKey(p)
		if err := p.Validate(); err != nil {
			return nil, err
		}
		if seen[p.ID] {
			return nil, &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("duplicate provider id %q in %s", p.ID, path)}
		}
		seen[p.ID] = true
	}
	return fc.Providers, nil
}

// ConfigPath resolves the active config location: env override first, then
// the default user path.
func ConfigPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv(EnvConfigPath)); p != "" {
		return p, nil
	}
	return DefaultConfigPath()
}

// LoadProviders loads the active config (env-overridden path or user
// default). Missing default file yields an empty list, not an error — the
// providers tool should report the setup hint instead of failing.
func LoadProviders() ([]ProviderConfig, string, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, "", err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) && strings.TrimSpace(os.Getenv(EnvConfigPath)) == "" {
			return nil, path, nil
		}
	}
	providers, err := LoadProvidersFile(path)
	if err != nil {
		return nil, path, err
	}
	return providers, path, nil
}

// FindProvider loads the active config and returns one entry by id.
func FindProvider(id string) (ProviderConfig, string, error) {
	providers, path, err := LoadProviders()
	if err != nil {
		return ProviderConfig{}, path, err
	}
	for _, p := range providers {
		if strings.EqualFold(p.ID, id) {
			return p, path, nil
		}
	}
	return ProviderConfig{}, path, &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("provider %q is not configured in %s", id, path)}
}

var providerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// Validate normalizes the entry and enforces the unified-template minimums.
func (p *ProviderConfig) Validate() error {
	applyEnvKey(p)
	p.ID = strings.TrimSpace(p.ID)
	p.Dialect = strings.ToLower(strings.TrimSpace(p.Dialect))
	p.BaseURL = strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	p.APIKey = strings.TrimSpace(p.APIKey)
	p.Model = strings.TrimSpace(p.Model)
	p.Auth.Style = strings.ToLower(strings.TrimSpace(p.Auth.Style))
	if p.ID == "" {
		return &Error{Class: ClassConfigError, ProviderMessage: "provider entry needs an id"}
	}
	if !providerIDPattern.MatchString(p.ID) {
		return &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("provider id %q must match %s", p.ID, providerIDPattern.String())}
	}
	if p.Dialect == "" {
		return &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("provider %q needs a dialect (one of %s)", p.ID, knownDialects())}
	}
	if _, ok := dialectRegistry[p.Dialect]; !ok {
		return &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("provider %q: unknown dialect %q (known: %s)", p.ID, p.Dialect, knownDialects())}
	}
	if p.BaseURL == "" {
		return &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("provider %q needs base_url", p.ID)}
	}
	if !strings.HasPrefix(p.BaseURL, "http://") && !strings.HasPrefix(p.BaseURL, "https://") {
		return &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("provider %q: base_url must be http(s)", p.ID)}
	}
	switch p.Auth.Style {
	case "", authNone, authBearer, authHeader, authQuery, authCookie:
	default:
		return &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("provider %q: unknown auth style %q", p.ID, p.Auth.Style)}
	}
	if requiresKey(p.Auth.Style) && p.Auth.Key() == "" {
		return &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("provider %q: auth style %q requires api_key", p.ID, p.Auth.Style)}
	}
	if p.TimeoutSeconds < 0 || p.TimeoutSeconds > 3600 {
		return &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("provider %q: timeout_seconds must be 0..3600", p.ID)}
	}
	if p.MaxArtifactBytes < 0 {
		return &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("provider %q: max_artifact_bytes must be >= 0", p.ID)}
	}
	return nil
}

func requiresKey(style string) bool {
	switch style {
	case authBearer, authHeader, authQuery, authCookie:
		return true
	}
	return false
}

// applyEnvKey resolves VIT_GENAPI_KEY_<ID> per-provider credential override.
func applyEnvKey(p *ProviderConfig) {
	suffix := regexp.MustCompile(`[^A-Za-z0-9]`).ReplaceAllString(strings.ToUpper(p.ID), "_")
	if v := strings.TrimSpace(os.Getenv("VIT_GENAPI_KEY_" + suffix)); v != "" {
		p.Auth.value = v
		p.EnvKeyApplied = true
		return
	}
	p.Auth.value = p.APIKey
}

// Describe returns a secret-free summary for disclosure surfaces.
func (p ProviderConfig) Describe() map[string]any {
	style := p.Auth.Style
	if style == "" {
		if p.Auth.Key() == "" {
			style = authNone
		} else {
			style = authBearer
		}
	}
	return map[string]any{
		"id":                    p.ID,
		"dialect":               p.Dialect,
		"base_url":              p.BaseURL,
		"model":                 p.Model,
		"auth_style":            style,
		"has_api_key":           p.Auth.Key() != "",
		"api_key_from":          map[bool]string{true: "env", false: "config_file"}[p.EnvKeyApplied],
		"timeout_seconds":       p.TimeoutSeconds,
		"poll_interval_seconds": p.PollIntervalSeconds,
		"download_dir":          p.DownloadDir,
	}
}

// SecretFreeJSON renders the whole provider list without credentials.
func SecretFreeJSON(providers []ProviderConfig) []map[string]any {
	out := make([]map[string]any, 0, len(providers))
	for _, p := range providers {
		out = append(out, p.Describe())
	}
	return out
}

// Timeout returns the resolved overall generation budget (dialect default
// applied). The harness layer and Generate must agree on this value.
func (p ProviderConfig) Timeout() time.Duration {
	if p.TimeoutSeconds > 0 {
		return time.Duration(p.TimeoutSeconds) * time.Second
	}
	if d, ok := dialectRegistry[p.Dialect]; ok && d.timeoutSecs > 0 {
		return time.Duration(d.timeoutSecs) * time.Second
	}
	return defaultTimeout
}

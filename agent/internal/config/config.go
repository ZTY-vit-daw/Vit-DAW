package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type EngineConfig struct {
	BaseURL          string                 `json:"baseUrl"`
	APIKey           string                 `json:"apiKey"`
	DefaultModel     string                 `json:"defaultModel"`
	MultimodalRoutes map[string]RouteConfig `json:"multimodalRoutes,omitempty"`
	Browser          BrowserConfig          `json:"browser,omitempty"`
}

type RouteConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider,omitempty"`
	BaseURL  string `json:"baseUrl,omitempty"`
	APIKey   string `json:"apiKey,omitempty"`
	Model    string `json:"model,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Fallback string `json:"fallback,omitempty"`
}

type BrowserConfig struct {
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode,omitempty"`
	ProfilePath     string `json:"profilePath,omitempty"`
	RememberSession bool   `json:"rememberSession,omitempty"`
}

func (c EngineConfig) Complete() bool {
	return strings.TrimSpace(c.BaseURL) != "" &&
		strings.TrimSpace(c.APIKey) != "" &&
		strings.TrimSpace(c.DefaultModel) != ""
}

func (c *EngineConfig) Normalize() {
	c.BaseURL = strings.TrimSpace(c.BaseURL)
	c.APIKey = strings.TrimSpace(c.APIKey)
	c.DefaultModel = strings.TrimSpace(c.DefaultModel)
	c.Browser.Mode = strings.TrimSpace(c.Browser.Mode)
	c.Browser.ProfilePath = strings.TrimSpace(c.Browser.ProfilePath)
	if c.Browser.Mode == "" {
		c.Browser.Mode = "webview2_companion"
	}
	if c.MultimodalRoutes == nil {
		c.MultimodalRoutes = DefaultMultimodalRoutes()
	}
	defaults := DefaultMultimodalRoutes()
	for key, route := range defaults {
		current, ok := c.MultimodalRoutes[key]
		if !ok {
			c.MultimodalRoutes[key] = route
			continue
		}
		current.Provider = strings.TrimSpace(current.Provider)
		current.BaseURL = strings.TrimSpace(current.BaseURL)
		current.APIKey = strings.TrimSpace(current.APIKey)
		current.Model = strings.TrimSpace(current.Model)
		current.Fallback = strings.TrimSpace(current.Fallback)
		if current.Provider == "" {
			current.Provider = route.Provider
		}
		if current.Priority <= 0 {
			current.Priority = route.Priority
		}
		c.MultimodalRoutes[key] = current
	}
}

func DefaultMultimodalRoutes() map[string]RouteConfig {
	return map[string]RouteConfig{
		"text":                {Provider: "default", Priority: 10},
		"image_understanding": {Provider: "default", Priority: 20},
		"image_generation":    {Provider: "default", Priority: 30},
		"audio_understanding": {Provider: "default", Priority: 40},
		"transcription":       {Provider: "default", Priority: 50},
		"video_browser":       {Provider: "default", Priority: 60},
		"embeddings":          {Provider: "default", Priority: 70},
	}
}

func RedactSecrets(cfg EngineConfig) (EngineConfig, map[string]bool) {
	routeKeys := map[string]bool{}
	cfg.APIKey = ""
	for key, route := range cfg.MultimodalRoutes {
		routeKeys[key] = strings.TrimSpace(route.APIKey) != ""
		route.APIKey = ""
		cfg.MultimodalRoutes[key] = route
	}
	return cfg, routeKeys
}

func PreserveBlankSecrets(next, current EngineConfig) EngineConfig {
	if strings.TrimSpace(next.APIKey) == "" {
		next.APIKey = strings.TrimSpace(current.APIKey)
	}
	if next.MultimodalRoutes == nil {
		next.MultimodalRoutes = map[string]RouteConfig{}
	}
	for key, route := range next.MultimodalRoutes {
		if strings.TrimSpace(route.APIKey) == "" {
			if currentRoute, ok := current.MultimodalRoutes[key]; ok {
				route.APIKey = strings.TrimSpace(currentRoute.APIKey)
				next.MultimodalRoutes[key] = route
			}
		}
	}
	return next
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: user home: %w", err)
	}
	return filepath.Join(home, ".vit", "config.json"), nil
}

func Load() (EngineConfig, string, error) {
	path, err := Path()
	if err != nil {
		return EngineConfig{}, "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fromEnv(), path, nil
		}
		return EngineConfig{}, path, fmt.Errorf("config: read %q: %w", path, err)
	}
	var cfg EngineConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return EngineConfig{}, path, fmt.Errorf("config: parse %q: %w", path, err)
	}
	if !cfg.Complete() {
		env := fromEnv()
		if strings.TrimSpace(cfg.BaseURL) == "" {
			cfg.BaseURL = env.BaseURL
		}
		if strings.TrimSpace(cfg.APIKey) == "" {
			cfg.APIKey = env.APIKey
		}
		if strings.TrimSpace(cfg.DefaultModel) == "" {
			cfg.DefaultModel = env.DefaultModel
		}
	}
	cfg.Normalize()
	return cfg, path, nil
}

func Save(cfg EngineConfig) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	cfg.Normalize()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return path, fmt.Errorf("config: mkdir %q: %w", dir, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return path, fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return path, fmt.Errorf("config: write %q: %w", path, err)
	}
	return path, nil
}

func fromEnv() EngineConfig {
	cfg := EngineConfig{
		BaseURL:      firstNonEmpty(os.Getenv("VIT_AGENT_LLM_BASE_URL"), os.Getenv("OPENAI_BASE_URL"), "https://api.openai.com/v1"),
		APIKey:       firstNonEmpty(os.Getenv("VIT_AGENT_LLM_API_KEY"), os.Getenv("OPENAI_API_KEY")),
		DefaultModel: firstNonEmpty(os.Getenv("VIT_AGENT_LLM_MODEL"), os.Getenv("OPENAI_MODEL")),
	}
	cfg.Normalize()
	return cfg
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

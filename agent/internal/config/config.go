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
	BaseURL      string `json:"baseUrl"`
	APIKey       string `json:"apiKey"`
	DefaultModel string `json:"defaultModel"`
}

func (c EngineConfig) Complete() bool {
	return strings.TrimSpace(c.BaseURL) != "" &&
		strings.TrimSpace(c.APIKey) != "" &&
		strings.TrimSpace(c.DefaultModel) != ""
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
	return cfg, path, nil
}

func Save(cfg EngineConfig) (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
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
	return EngineConfig{
		BaseURL:      firstNonEmpty(os.Getenv("VIT_AGENT_LLM_BASE_URL"), os.Getenv("OPENAI_BASE_URL"), "https://api.openai.com/v1"),
		APIKey:       firstNonEmpty(os.Getenv("VIT_AGENT_LLM_API_KEY"), os.Getenv("OPENAI_API_KEY")),
		DefaultModel: firstNonEmpty(os.Getenv("VIT_AGENT_LLM_MODEL"), os.Getenv("OPENAI_MODEL")),
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

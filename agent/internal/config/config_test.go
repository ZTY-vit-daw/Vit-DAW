package config

import "testing"

func TestNormalizeBaseURLAddsRightCodesV1(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "right codes proxy root",
			in:   "https://www.right.codes/claude-aws",
			want: "https://www.right.codes/claude-aws/v1",
		},
		{
			name: "right codes proxy root trailing slash",
			in:   "https://www.right.codes/claude-aws/",
			want: "https://www.right.codes/claude-aws/v1",
		},
		{
			name: "right codes already v1",
			in:   "https://www.right.codes/claude-aws/v1",
			want: "https://www.right.codes/claude-aws/v1",
		},
		{
			name: "right codes explicit chat endpoint",
			in:   "https://www.right.codes/claude-aws/v1/chat/completions",
			want: "https://www.right.codes/claude-aws/v1/chat/completions",
		},
		{
			name: "other provider",
			in:   "https://api.example.com/v1",
			want: "https://api.example.com/v1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeBaseURL(tc.in); got != tc.want {
				t.Fatalf("NormalizeBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestEngineConfigNormalizeBaseURL(t *testing.T) {
	cfg := EngineConfig{
		BaseURL:      " https://www.right.codes/claude-aws/ ",
		APIKey:       " key ",
		DefaultModel: " model ",
		MultimodalRoutes: map[string]RouteConfig{
			"text": {
				Provider: "default",
				BaseURL:  "https://www.right.codes/claude-aws",
			},
		},
	}
	cfg.Normalize()
	if cfg.BaseURL != "https://www.right.codes/claude-aws/v1" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if got := cfg.MultimodalRoutes["text"].BaseURL; got != "https://www.right.codes/claude-aws/v1" {
		t.Fatalf("route BaseURL = %q", got)
	}
}

func TestExplicitEnvOverridesCompleteFileConfig(t *testing.T) {
	t.Setenv("VIT_AGENT_LLM_BASE_URL", "http://127.0.0.1:18987/v1")
	t.Setenv("VIT_AGENT_LLM_API_KEY", "env-key")
	t.Setenv("VIT_AGENT_LLM_MODEL", "env-model")

	cfg := EngineConfig{
		BaseURL:      "https://api.invalid/v1",
		APIKey:       "file-key",
		DefaultModel: "file-model",
	}
	applyExplicitEnvOverrides(&cfg)
	cfg.Normalize()

	if cfg.BaseURL != "http://127.0.0.1:18987/v1" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.APIKey != "env-key" {
		t.Fatalf("APIKey = %q", cfg.APIKey)
	}
	if cfg.DefaultModel != "env-model" {
		t.Fatalf("DefaultModel = %q", cfg.DefaultModel)
	}
}

package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/config"
)

// M3-LLM-TIMEOUT-ADAPT-1：默认 fallback 位的 env 适配纯函数测试。
// 零真栈、零 LLM 计费——全部走构造注入（httptest/捕获型 warn 注入），
// 不产生任何出网请求。

// 五件之③（细分）：非法/越界 env → 回落 60s + 非空 warn 理由；界内值原样生效；
// 空值（未设）保持历史 60s。warn 走注入捕获，不触真实 stderr。
func TestResolveDefaultAttemptTimeout(t *testing.T) {
	cases := []struct {
		name      string
		envValue  string
		want      time.Duration
		wantWarn  bool
	}{
		{name: "unset keeps historical default", envValue: "", want: 60 * time.Second},
		{name: "blank keeps historical default", envValue: "   ", want: 60 * time.Second},
		{name: "in-range override wins", envValue: "90", want: 90 * time.Second},
		{name: "min bound allowed", envValue: "1", want: 1 * time.Second},
		{name: "max bound allowed", envValue: "3600", want: 3600 * time.Second},
		{name: "padded number accepted", envValue: " 120 ", want: 120 * time.Second},
		{name: "zero falls back with warn", envValue: "0", want: 60 * time.Second, wantWarn: true},
		{name: "negative falls back with warn", envValue: "-5", want: 60 * time.Second, wantWarn: true},
		{name: "above max falls back with warn", envValue: "3601", want: 60 * time.Second, wantWarn: true},
		{name: "non-integer falls back with warn", envValue: "abc", want: 60 * time.Second, wantWarn: true},
		{name: "float falls back with warn", envValue: "1.5", want: 60 * time.Second, wantWarn: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, warn := resolveDefaultAttemptTimeout(tc.envValue)
			if got != tc.want {
				t.Fatalf("timeout = %s, want %s (env=%q)", got, tc.want, tc.envValue)
			}
			if tc.wantWarn && warn == "" {
				t.Fatalf("env=%q must return a non-empty warn reason (no silent fallback)", tc.envValue)
			}
			if !tc.wantWarn && warn != "" {
				t.Fatalf("env=%q must not warn, got %q", tc.envValue, warn)
			}
			if tc.wantWarn && !strings.Contains(warn, llmTimeoutEnvVar) {
				t.Fatalf("warn reason must name the env var, got %q", warn)
			}
		})
	}
}

// captureWarn 替换包级 warn 注入点并返回恢复函数——测试间零全局状态残留。
func captureWarn(t *testing.T) func() string {
	t.Helper()
	var captured []string
	original := warnLLMTimeoutFallback
	warnLLMTimeoutFallback = func(reason string) { captured = append(captured, reason) }
	t.Cleanup(func() { warnLLMTimeoutFallback = original })
	return func() string { return strings.Join(captured, " | ") }
}

// 五件之①②④⑤：attemptHTTPClient 红线矩阵——env 未设=60s；env 覆盖生效；
// 显式 req.Timeout 优先于 env；NoTimeout 不受 env 影响。非法值回落+记录。
func TestAttemptHTTPClientRedlines(t *testing.T) {
	t.Run("env unset keeps 60s default", func(t *testing.T) {
		warnings := captureWarn(t)
		client := attemptHTTPClient(Request{}, "")
		if client.Timeout != 60*time.Second {
			t.Fatalf("default timeout = %s, want 60s", client.Timeout)
		}
		if warnings() != "" {
			t.Fatalf("default path must not warn, got %q", warnings())
		}
	})

	t.Run("env override reaches http client timeout", func(t *testing.T) {
		warnings := captureWarn(t)
		client := attemptHTTPClient(Request{}, "300")
		if client.Timeout != 300*time.Second {
			t.Fatalf("env override timeout = %s, want 300s", client.Timeout)
		}
		if warnings() != "" {
			t.Fatalf("valid override must not warn, got %q", warnings())
		}
	})

	t.Run("explicit req.Timeout beats env", func(t *testing.T) {
		warnings := captureWarn(t)
		client := attemptHTTPClient(Request{Timeout: 5 * time.Second}, "300")
		if client.Timeout != 5*time.Second {
			t.Fatalf("explicit timeout = %s, want 5s (env must not override it)", client.Timeout)
		}
		if warnings() != "" {
			t.Fatalf("explicit timeout path must not warn, got %q", warnings())
		}
	})

	t.Run("NoTimeout unaffected by env", func(t *testing.T) {
		warnings := captureWarn(t)
		client := attemptHTTPClient(Request{NoTimeout: true}, "300")
		if client.Timeout != 0 {
			t.Fatalf("NoTimeout client timeout = %s, want 0 (uncapped)", client.Timeout)
		}
		if warnings() != "" {
			t.Fatalf("NoTimeout path must not warn, got %q", warnings())
		}
	})

	t.Run("invalid env falls back and records", func(t *testing.T) {
		warnings := captureWarn(t)
		client := attemptHTTPClient(Request{}, "not-a-number")
		if client.Timeout != 60*time.Second {
			t.Fatalf("invalid env fallback timeout = %s, want 60s", client.Timeout)
		}
		if warnings() == "" {
			t.Fatal("invalid env fallback must be recorded (non-silent)")
		}
	})
}

// 行为级传导证明：env=1 真实传导到 CompleteRequest 构造的 http.Client——
// 挂 2s 的 httptest 后端 + >48KB payload（强制 attempts=1）下，
// env=1s 的请求必须失败且远小于历史 60s 默认；env 未设的同一请求必须成功
// （2s < 60s 默认窗）。零真实网络、零 LLM 计费。
func TestCompleteRequestEnvTimeoutReachesHTTPClient(t *testing.T) {
	t.Setenv("VIT_AGENT_LLM_TELEMETRY_PATH", "disabled")
	bigPayload := Request{
		Messages: []Message{{Role: "user", Content: strings.Repeat("x", 64*1024)}},
	}
	cfg := config.EngineConfig{BaseURL: "http://llm.invalid/v1", APIKey: "test-key", DefaultModel: "test-model"}

	t.Run("env=1s cuts the slow request", func(t *testing.T) {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer backend.Close()
		cfg.BaseURL = backend.URL
		t.Setenv(llmTimeoutEnvVar, "1")
		client := &Client{}
		started := time.Now()
		_, err := client.CompleteRequest(context.Background(), cfg, bigPayload)
		elapsed := time.Since(started)
		if err == nil {
			t.Fatal("2s endpoint must fail under a 1s env timeout")
		}
		if elapsed >= defaultLLMAttemptTimeout {
			t.Fatalf("env override must reach the http client: elapsed %s >= default %s", elapsed, defaultLLMAttemptTimeout)
		}
	})

	t.Run("env unset keeps 60s default window", func(t *testing.T) {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
		}))
		defer backend.Close()
		cfg.BaseURL = backend.URL
		t.Setenv(llmTimeoutEnvVar, "")
		client := &Client{}
		_, err := client.CompleteRequest(context.Background(), cfg, bigPayload)
		if err != nil {
			t.Fatalf("2s endpoint must succeed inside the historical 60s default: %v", err)
		}
	})
}

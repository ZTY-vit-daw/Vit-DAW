package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"vit-daw-agent/internal/config"
)

// FIX-ASK-SILENT-GAP-1 ②：显式 TimeoutIsTotalBudget 时，重试环共享一个
// deadline——退化端点（挂死/慢 503）不能再把一个小请求拉长成
// Timeout × attempts（M2 现场 60s×2+退避 ≈ 121s 静默墙）。
func TestCompleteRequestTotalBudgetBoundsHangingEndpoint(t *testing.T) {
	t.Setenv("VIT_AGENT_LLM_TELEMETRY_PATH", "disabled")
	client := &Client{HTTPClient: &http.Client{Transport: contextBlockTransport{}}}
	request := Request{
		Messages:             []Message{{Role: "user", Content: "ping"}},
		Timeout:              300 * time.Millisecond,
		TimeoutIsTotalBudget: true,
	}
	started := time.Now()
	_, err := client.CompleteRequest(context.Background(), config.EngineConfig{
		BaseURL: "http://llm.invalid/v1", APIKey: "test-key", DefaultModel: "test-model",
	}, request)
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("hanging endpoint must fail the request")
	}
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("total budget must bound the whole retry loop: budget=300ms, took %s", elapsed)
	}
}

// 同一退化形态（每次尝试都慢失败且可重试）下，opt-in 标志决定重试是否继续：
// 无标志 → 保持历史 per-attempt 语义（3 次尝试全部执行）；有标志 → 预算耗尽
// 即收敛。这是「不弱化既有重试语义」的回归钉。
func TestCompleteRequestPerAttemptTimeoutKeepsFullRetries(t *testing.T) {
	t.Setenv("VIT_AGENT_LLM_TELEMETRY_PATH", "disabled")
	var hits atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(60 * time.Millisecond)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	backend := httptest.NewServer(handler)
	defer backend.Close()
	cfg := config.EngineConfig{BaseURL: backend.URL, APIKey: "test-key", DefaultModel: "test-model"}

	t.Run("flag off keeps three attempts", func(t *testing.T) {
		hits.Store(0)
		client := &Client{}
		started := time.Now()
		_, err := client.CompleteRequest(context.Background(), cfg, Request{
			Messages: []Message{{Role: "user", Content: "ping"}},
			Timeout:  200 * time.Millisecond,
		})
		elapsed := time.Since(started)
		if err == nil {
			t.Fatal("503 endpoint must fail the request")
		}
		if got := hits.Load(); got != 3 {
			t.Fatalf("per-attempt timeout must keep the bounded retry count: hits=%d", got)
		}
		if elapsed > 5*time.Second {
			t.Fatalf("legacy retry shape unexpectedly slow: %s", elapsed)
		}
	})

	t.Run("flag on stops within the shared budget", func(t *testing.T) {
		hits.Store(0)
		client := &Client{}
		started := time.Now()
		_, err := client.CompleteRequest(context.Background(), cfg, Request{
			Messages:             []Message{{Role: "user", Content: "ping"}},
			Timeout:              200 * time.Millisecond,
			TimeoutIsTotalBudget: true,
		})
		elapsed := time.Since(started)
		if err == nil {
			t.Fatal("503 endpoint must fail the request")
		}
		if got := hits.Load(); got >= 3 {
			t.Fatalf("shared budget must stop the retry loop early: hits=%d", got)
		}
		if elapsed > 1500*time.Millisecond {
			t.Fatalf("shared budget must converge the loop: budget=200ms, took %s", elapsed)
		}
	})
}

// contextBlockTransport simulates a degenerate endpoint: the request hangs
// until its own context is done, then fails with the context error.
type contextBlockTransport struct{}

func (contextBlockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, fmt.Errorf("llm transport stalled: %w", req.Context().Err())
}

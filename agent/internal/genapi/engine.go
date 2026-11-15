package genapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Minimal local engine supervision (GEN-YUE-WIRE-1, audit H6 item 6).
//
// Contract: on-demand start, readiness probe, classified failure disclosure,
// and self-owned process management only. An engine that is already
// reachable is reused and reported external — the supervisor never kills or
// otherwise manages a process it did not start. There is exactly one
// supervisor per provider id per process; concurrent EnsureYueEngine calls
// for the same provider serialize on a mutex.

// EngineHandle reports the outcome of one EnsureYueEngine call.
type EngineHandle struct {
	// External=true means the engine was already reachable; the supervisor
	// owns nothing and Stop() is a no-op.
	External bool `json:"external"`
	// BaseURL is the engine HTTP surface that answered (or became ready).
	BaseURL string `json:"base_url"`
	// PID is the process the supervisor started (0 when External).
	PID int `json:"pid,omitempty"`
	// Command is the started command line (diagnostics).
	Command string `json:"command,omitempty"`
	// ReadyAfter is how long the start-to-ready wait took.
	ReadyAfter time.Duration `json:"ready_after,omitempty"`
}

var (
	engineMu       sync.Mutex
	engineByProvider = map[string]*engineRun{}
)

// engineRun tracks one self-started engine process.
type engineRun struct {
	cmd      *exec.Cmd
	handle   EngineHandle
	waitDone chan struct{}
}

// ProbeYueEngine reports whether the engine HTTP surface at baseURL answers
// any request (any HTTP status — including 404 — counts as up; connection
// refused counts as down; other transport errors count as down for a local
// 127.0.0.1 engine).
func ProbeYueEngine(ctx context.Context, baseURL string, timeout time.Duration) bool {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(pctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

// EnsureYueEngine makes sure the engine behind cfg is reachable, starting it
// per cfg.Engine when configured and not yet up. It never manages processes
// it did not start.
func EnsureYueEngine(ctx context.Context, cfg ProviderConfig) (*EngineHandle, error) {
	if ProbeYueEngine(ctx, cfg.BaseURL, 0) {
		return &EngineHandle{External: true, BaseURL: cfg.BaseURL}, nil
	}
	if cfg.Engine == nil || cfg.Engine.Command == "" {
		return nil, &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("engine at %s is not reachable and provider %q has no engine.command — start the engine or fill the engine block in the genapi provider config", cfg.BaseURL, cfg.ID)}
	}
	engineMu.Lock()
	defer engineMu.Unlock()
	// Someone may have started it while we waited on the mutex.
	if ProbeYueEngine(ctx, cfg.BaseURL, 0) {
		return &EngineHandle{External: true, BaseURL: cfg.BaseURL}, nil
	}
	if run, ok := engineByProvider[cfg.ID]; ok {
		// We started one before and still own it: wait for readiness of that
		// run instead of stacking a second process.
		if ProbeYueEngine(ctx, cfg.BaseURL, 0) {
			h := run.handle
			return &h, nil
		}
	}
	ec := cfg.Engine
	cmd := exec.Command(ec.Command, ec.Args...)
	if ec.WorkingDir != "" {
		cmd.Dir = ec.WorkingDir
	}
	env := os.Environ()
	for k, v := range ec.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env
	// Discard engine output so a chatty engine cannot block on a full pipe;
	// the engine's own log surface is its own concern.
	cmd.Stdout = nil
	cmd.Stderr = nil
	applyEngineSysProcAttr(cmd)
	if serr := cmd.Start(); serr != nil {
		return nil, &Error{Class: ClassConfigError, ProviderMessage: fmt.Sprintf("engine start failed for provider %q (%s): %v", cfg.ID, ec.Command, serr)}
	}
	handle := EngineHandle{
		BaseURL:   cfg.BaseURL,
		PID:       cmd.Process.Pid,
		Command:   ec.Command,
	}
	// cmd.Wait in the background makes early exits observable via the
	// channel (ProcessState stays nil until a Wait completes).
	waitDone := make(chan struct{})
	var waitErr error
	go func() {
		waitErr = cmd.Wait()
		close(waitDone)
	}()
	started := time.Now()
	deadline := time.Duration(ec.readyTimeout()) * time.Second
	interval := time.Duration(ec.pollInterval() * float64(time.Second))
	if interval <= 0 {
		interval = 2 * time.Second
	}
	for {
		if ProbeYueEngine(ctx, cfg.BaseURL, 0) {
			handle.ReadyAfter = time.Since(started)
			engineByProvider[cfg.ID] = &engineRun{cmd: cmd, handle: handle, waitDone: waitDone}
			return &handle, nil
		}
		select {
		case <-waitDone:
			// The wrapper/engine died before becoming ready.
			code := -1
			if cmd.ProcessState != nil {
				code = cmd.ProcessState.ExitCode()
			}
			if waitErr != nil {
				return nil, &Error{Class: ClassProviderError, ProviderMessage: fmt.Sprintf("engine process for provider %q exited before becoming ready at %s (code %d, %v) — check the engine deployment/logs", cfg.ID, cfg.BaseURL, code, waitErr)}
			}
			return nil, &Error{Class: ClassProviderError, ProviderMessage: fmt.Sprintf("engine process for provider %q exited early (code %d) before becoming ready at %s — check the engine deployment/logs", cfg.ID, code, cfg.BaseURL)}
		default:
		}
		if time.Since(started) >= deadline {
			_ = stopEngineProcessTree(cmd)
			<-waitDone
			return nil, &Error{Class: ClassNetworkError, Retryable: true, ProviderMessage: fmt.Sprintf("engine at %s did not become ready within %s (started %s) — the process was stopped; check the engine deployment", cfg.BaseURL, deadline, ec.Command)}
		}
		if !sleepCtx(ctx, interval) {
			_ = stopEngineProcessTree(cmd)
			<-waitDone
			return nil, &Error{Class: ClassTimeoutError, Retryable: true, ProviderMessage: "engine readiness wait canceled"}
		}
	}
}

// EngineStatus reports the supervisor state for disclosure surfaces.
func EngineStatus(providerID, baseURL string) map[string]any {
	engineMu.Lock()
	defer engineMu.Unlock()
	out := map[string]any{
		"provider_id": providerID,
		"base_url":    baseURL,
		"reachable":   ProbeYueEngine(context.Background(), baseURL, 0),
	}
	if run, ok := engineByProvider[providerID]; ok {
		out["owned_pid"] = run.handle.PID
		out["owned_command"] = run.handle.Command
		out["ready_after_ms"] = run.handle.ReadyAfter.Milliseconds()
	} else {
		out["owned_pid"] = 0
	}
	return out
}

// EngineStop stops the engine the supervisor started for providerID (only
// its own process; external engines are never touched). Returns whether a
// self-started engine was stopped.
func EngineStop(providerID string) (bool, error) {
	engineMu.Lock()
	defer engineMu.Unlock()
	run, ok := engineByProvider[providerID]
	if !ok {
		return false, nil
	}
	delete(engineByProvider, providerID)
	if serr := stopEngineProcessTree(run.cmd); serr != nil {
		return true, &Error{Class: ClassProviderError, ProviderMessage: fmt.Sprintf("engine stop for %s failed: %v", providerID, serr)}
	}
	if run.waitDone != nil {
		<-run.waitDone
	}
	return true, nil
}

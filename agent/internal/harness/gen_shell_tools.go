package harness

import (
	"context"
	"errors"

	"vit-daw-agent/internal/genapi"
	"vit-daw-agent/internal/genshell"
)

// Generation-shell management plane (GENSHELL-AGENT-1): the runner itself is
// a background service; these local tools give the conversation stream the
// start/stop/status handles (RiskDirect) and the conversation-flow submit
// (gen_shell.submit — 2026-10-17 card revision).

// StartGenShellRunner is the product-host wiring point (chat server Start
// calls it once; nil kernel face = honest no-op, tests stay goroutine-free).
func (h *Harness) StartGenShellRunner() {
	if r := h.genShellRunner(); r != nil {
		r.Start()
	}
}

// StopGenShellRunner shuts the runner down (chat server Close).
func (h *Harness) StopGenShellRunner() {
	if r := h.genShellRunner(); r != nil {
		r.Stop()
	}
}

func (h *Harness) genShellRunner() *genshell.Runner {
	if h == nil {
		return nil
	}
	return h.genShell
}

func (h *Harness) genShellStatus() (map[string]any, error) {
	r := h.genShellRunner()
	if r == nil || !runnerStarted(r) {
		return nil, errors.New("generation shell runner is not wired in this session (no kernel face)")
	}
	out := map[string]any{"status": "ok", "runner": r.Status()}
	if providers, path, err := genapi.LoadProviders(); err == nil {
		out["providers"] = genapi.SecretFreeJSON(providers)
		out["provider_config"] = path
	} else {
		out["providers"] = []any{}
		out["provider_error"] = err.Error()
	}
	return out, nil
}

func runnerStarted(r *genshell.Runner) bool {
	return r != nil && r.Status().Started
}

func (h *Harness) genShellStart() (map[string]any, error) {
	r := h.genShellRunner()
	if r == nil || !runnerStarted(r) {
		return nil, errors.New("generation shell runner is not wired in this session (no kernel face)")
	}
	r.Enable(true)
	r.PollOnce(context.Background()) // immediate first tick after resume
	return map[string]any{"status": "ok", "runner": r.Status()}, nil
}

func (h *Harness) genShellStop() (map[string]any, error) {
	r := h.genShellRunner()
	if r == nil || !runnerStarted(r) {
		return nil, errors.New("generation shell runner is not wired in this session (no kernel face)")
	}
	r.Enable(false)
	return map[string]any{"status": "ok", "runner": r.Status()}, nil
}

func (h *Harness) genShellSubmit(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	if h == nil || h.kernel == nil {
		return nil, errors.New("kernel client is nil")
	}
	send := func(kctx context.Context, command map[string]any) (map[string]any, error) {
		reply, _, err := h.KernelSendCommand(kctx, command)
		return reply, err
	}
	args := genshell.SubmitArgs{
		FreeText:        firstString(cmd, "free_text", "prompt"),
		TrackID:         firstString(cmd, "track_id", "target_track_id"),
		ShellNodeID:     firstString(cmd, "shell_node_id", "plugin_id"),
		ProviderID:      firstString(cmd, "provider_id", "provider"),
		Model:           firstString(cmd, "model"),
		TemplateID:      firstString(cmd, "template_id"),
		Scene:           firstString(cmd, "scene"),
		StartTime:       floatValueFromMap(cmd, "start_time", "start_seconds"),
		LengthBeats:     floatValueFromMap(cmd, "length_beats"),
		RenderOnly:      boolValueDefault(cmd["render_only"], false),
		Count:           intValueFromMap(cmd, "count"),
		DurationSeconds: floatValueFromMap(cmd, "duration_seconds", "duration"),
		StyleTags:       firstString(cmd, "style_tags", "tags", "style"),
		Instrumental:    boolValueDefault(cmd["instrumental"], false),
		Title:           firstString(cmd, "title"),
	}
	if form, ok := cmd["form"].(map[string]any); ok {
		args.Form = form
	}
	result, err := genshell.SubmitConversation(ctx, send, args)
	if err != nil {
		return map[string]any{"status": "error", "error": err.Error()}, err
	}
	return result, nil
}

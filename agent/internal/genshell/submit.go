package genshell

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/genapi"
)

// SubmitArgs is the conversation-flow submit input (gen_shell.submit tool,
// 2026-10-17 card revision: the agent chat stream submits into the kernel
// HOST queue through the already-product submit face — kernel side adds
// nothing).
type SubmitArgs struct {
	FreeText        string
	TrackID         string
	ShellNodeID     string
	ProviderID      string
	Model           string
	TemplateID      string
	Form            map[string]any
	Scene           string
	StartTime       float64
	LengthBeats     float64
	RenderOnly      bool
	Count           int
	DurationSeconds float64
	StyleTags       string
	Instrumental    bool
	Title           string
}

// SubmitConversation resolves the missing pieces (shell instance, provider,
// job id) and pushes the request into the kernel HOST queue via the
// gen_shell.pending submit face. The reply proves queueing by the queued job
// appearing in the same pending answer.
func SubmitConversation(ctx context.Context, send KernelCommand, args SubmitArgs) (map[string]any, error) {
	if strings.TrimSpace(args.FreeText) == "" {
		return nil, fmt.Errorf("gen_shell.submit needs free_text (the user free-text block)")
	}
	if strings.TrimSpace(args.TrackID) == "" && !args.RenderOnly {
		return nil, fmt.Errorf("gen_shell.submit needs track_id for in_project landing (pass track_id, or scene=free_creation to keep products off the timeline)")
	}

	shells, err := LoadedShells(ctx, send)
	if err != nil {
		return nil, err
	}
	var shellNodeID string
	if id := strings.TrimSpace(args.ShellNodeID); id != "" {
		for _, s := range shells {
			if s == id {
				shellNodeID = id
				break
			}
		}
		if shellNodeID == "" {
			return nil, fmt.Errorf("shell_node_id %q is not a loaded GenShell instance (loaded: %v)", id, shells)
		}
	} else {
		if len(shells) == 0 {
			return nil, fmt.Errorf("no loaded GenShell instance: load GenShell on a track (rack_add_node) before submitting")
		}
		shellNodeID = shells[0]
	}

	providerID := strings.TrimSpace(args.ProviderID)
	if providerID == "" {
		providers, _, perr := genapi.LoadProviders()
		if perr != nil {
			return nil, fmt.Errorf("no provider given and provider config unreadable: %v", perr)
		}
		if len(providers) == 0 {
			return nil, fmt.Errorf("no provider given and no genapi provider configured (see gen.providers)")
		}
		providerID = providers[0].ID
	}

	scene := strings.TrimSpace(args.Scene)
	if scene == "" {
		scene = string(SceneInProjectDefault)
	}

	jobID := NewJobID()
	request := map[string]any{
		"schema_version": RequestSchemaVersion,
		"job_id":         jobID,
		"shell_node_id":  shellNodeID,
		"track_id":       strings.TrimSpace(args.TrackID),
		"start_time":     args.StartTime,
		"length_beats":   args.LengthBeats,
		"scene":          scene,
		"provider_id":    providerID,
		"free_text":      args.FreeText,
		"gen_params": map[string]any{
			"duration_seconds": args.DurationSeconds,
			"count":            args.Count,
			"style_tags":       args.StyleTags,
			"instrumental":     args.Instrumental,
			"title":            args.Title,
		},
	}
	if model := strings.TrimSpace(args.Model); model != "" {
		request["model"] = model
	}
	if id := strings.TrimSpace(args.TemplateID); id != "" {
		request["template_id"] = id
	}
	if len(args.Form) > 0 {
		request["form"] = args.Form
	}
	if args.RenderOnly {
		request["render_only"] = true
	}

	reply, err := send(ctx, map[string]any{
		"cmd":          "gen_shell.pending",
		"debug_submit": request,
	})
	if err != nil {
		return nil, fmt.Errorf("submit transport failed: %w", err)
	}
	if status := stringField(reply, "status"); status != "ok" {
		msg := stringField(reply, "message")
		return nil, fmt.Errorf("submit rejected (status=%s): %s", status, msg)
	}
	queued := false
	if jobs, ok := reply["jobs"].([]any); ok {
		for _, item := range jobs {
			job, ok := item.(map[string]any)
			if ok && stringField(job, "job_id") == jobID {
				queued = true
				break
			}
		}
	}
	if !queued {
		return nil, fmt.Errorf("submit acknowledged but job %s missing from pending answer", jobID)
	}
	return map[string]any{
		"status":        "ok",
		"job_id":        jobID,
		"queued":        true,
		"shell_node_id": shellNodeID,
		"provider_id":   providerID,
		"scene":         scene,
		"note":          "queued in the kernel HOST queue; the agent runner picks it up on its next poll (2s cadence)",
	}, nil
}

// SceneInProjectDefault mirrors genctx.SceneInProject without importing it
// here (submit stays a thin builder; the runner re-validates scenes through
// genctx.ParseScene fail-closed).
const SceneInProjectDefault = "in_project"

// LoadedShells lists loaded GenShell rack nodes from the host registry
// (gen_shell.pending debug_shell_state section; instance_ready only).
func LoadedShells(ctx context.Context, send KernelCommand) ([]string, error) {
	reply, err := send(ctx, map[string]any{"cmd": "gen_shell.pending", "debug_shell_state": true})
	if err != nil {
		return nil, fmt.Errorf("gen_shell.pending failed: %w", err)
	}
	if status := stringField(reply, "status"); status != "ok" {
		return nil, fmt.Errorf("gen_shell.pending status=%s", status)
	}
	out := []string{}
	if shells, ok := reply["shells"].([]any); ok {
		for _, item := range shells {
			shell, ok := item.(map[string]any)
			if !ok || !boolValue(shell["instance_ready"]) {
				continue
			}
			if id := stringField(shell, "plugin_id"); id != "" {
				out = append(out, id)
			}
		}
	}
	return out, nil
}

func boolValue(v any) bool {
	b, _ := v.(bool)
	return b
}

// NewJobID mints a shell-side job id (件二 §2: uuid generated by the
// submitter).
func NewJobID() string {
	var buf [12]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("genshell-%d", time.Now().UnixNano())
	}
	return "genshell-" + hex.EncodeToString(buf[:])
}

// Package genshell implements the agent-side execution leg of the generation
// shell (GENSHELL-AGENT-1, design docs coord/runs/GEN-PLUGIN-SHELL-DESIGN-1
// 件一/件二/件三): a background runner that polls the kernel-side
// GenerationShellService queue (gen_shell.pending), claims queued jobs,
// renders prompts through the single genctx engine (profile=gen_plugin),
// generates through the unified genapi adapter, lands WAV artifacts on the
// timeline through the existing bridge_ingest_generated_asset path, and
// writes every state transition back through the aigc_register_job upsert
// face (H4 precedent: whole-record replacement — node_id/ghost_state/
// request_hash/source must ride every upsert).
//
// Kernel-side ownership stays where HOST-1 put it: the submit queue and the
// T1 parameter mirror belong to GenerationShellService; this package only
// reads the queue and pushes job-record upserts. There is no second queue
// and no new kernel command.
//
// Secrets discipline (件二 §3 / genapi precedent): provider credentials are
// never echoed into job records, status payloads, or logs — provider
// disclosure goes through genapi.Describe (has_api_key presence only).
package genshell

import (
	"context"
	"fmt"

	"vit-daw-agent/internal/genapi"
)

// RequestSchemaVersion is the vit.genshell.request.v1 submit payload schema
// (件二 §2); the kernel submit face rejects anything else.
const RequestSchemaVersion = "vit.genshell.request.v1"

// Job lifecycle states (件二 §1). They ride the existing job-record
// job_state string field; terminal states match the kernel service's
// isTerminalJobState.
const (
	StateQueued          = "queued"
	StateRunning         = "running"
	StateLanding         = "landing"
	StateDone            = "done"
	StateFailed          = "failed"
	StateCancelRequested = "cancel_requested"
	StateCancelled       = "cancelled"
	StateInterrupted     = "interrupted"
)

// JobSourcePrefix is the job-record source prefix HOST-1 stamps on shell
// jobs; orphan sweeping and ownership checks key on it.
const JobSourcePrefix = "gen_shell:"

// SubmitRequest is the vit.genshell.request.v1 payload (件二 §2). prompt /
// lyrics / tempo / musical_key are deliberately absent: the prompt is
// rendered agent-side and project features enter through the genctx snapshot
// only (统一化纪律 — 工程特征不经壳旁路).
type SubmitRequest struct {
	SchemaVersion string         `json:"schema_version"`
	JobID         string         `json:"job_id"`
	ShellNodeID   string         `json:"shell_node_id"`
	TrackID       string         `json:"track_id"`
	StartTime     float64        `json:"start_time"`
	LengthBeats   float64        `json:"length_beats"`
	Scene         string         `json:"scene"`
	ProviderID    string         `json:"provider_id"`
	Model         string         `json:"model,omitempty"`
	TemplateID    string         `json:"template_id,omitempty"`
	Form          map[string]any `json:"form,omitempty"`
	FreeText      string         `json:"free_text"`
	RenderOnly    bool           `json:"render_only,omitempty"`
	GenParams     GenParams      `json:"gen_params"`
}

// GenParams maps directly onto genapi.GenerationRequest fields (件二 §2).
type GenParams struct {
	DurationSeconds float64 `json:"duration_seconds"`
	Count           int     `json:"count"`
	StyleTags       string  `json:"style_tags"`
	Instrumental    bool    `json:"instrumental"`
	Title           string  `json:"title"`
}

// PendingJob is one entry of the gen_shell.pending jobs array (H3 assertion
// shape: job_id / state / cancel_requested / request; request is only
// present while the kernel service still holds the payload in memory).
type PendingJob struct {
	JobID           string         `json:"job_id"`
	State           string         `json:"state"`
	CancelRequested bool           `json:"cancel_requested"`
	Request         *SubmitRequest `json:"request,omitempty"`
}

// PendingAnswer is the gen_shell.pending reply body.
type PendingAnswer struct {
	Status    string       `json:"status"`
	Jobs      []PendingJob `json:"jobs"`
	Providers []any        `json:"providers"`
}

// KernelCommand is the narrow kernel face the runner needs; the harness
// adapts its KernelSendCommand into this shape (no harness import — the
// dependency direction stays genshell ← harness).
type KernelCommand func(ctx context.Context, command map[string]any) (map[string]any, error)

// GenerateFunc is the generation seam over the real genapi faces
// (FindProvider already resolved by the caller; the default is
// genapi.Generate itself). Tests substitute a fake so unit coverage never
// touches the network.
type GenerateFunc func(ctx context.Context, provider genapi.ProviderConfig, req genapi.GenerationRequest) (*genapi.Result, error)

// Terminal upsert helper: status messages carry the structured failure class
// prefix (件二 §3 — "class=<x>" + human detail; never silent).
func classMessage(class, detail string) string {
	if class == "" {
		class = "none"
	}
	if detail == "" {
		return fmt.Sprintf("class=%s", class)
	}
	return fmt.Sprintf("class=%s %s", class, detail)
}

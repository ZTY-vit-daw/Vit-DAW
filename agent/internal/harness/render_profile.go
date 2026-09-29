package harness

// Render -> delivery profile binding wiring (card RLM-PROFILE-2, roadmap D9).
// The stable render-surface anchor is the kernel render job id: render.start
// telemetry lands in Harness.renderResults (terminal ready/failed per job),
// and bindings key on that job id. Binding state lives in the rlm
// RenderBindingIndex (one render one profile) and is persisted with the
// project through chat's projectAgentRuntimeState. No kernel command surface
// is extended and capabilitycontext is untouched.

import (
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/rlm"
)

// BindRenderProfile binds one completed render (kernel job id) to exactly one
// delivery loudness profile. The render must have terminal telemetry cached
// with status ready (a deliverable exists); unknown or failed renders fail
// closed. Rebinding the same render to the same profile is idempotent; a
// different profile is rejected (one render one profile — multiple
// deliverables are multiple renders).
func (h *Harness) BindRenderProfile(renderID, profileID string) (rlm.RenderProfileBinding, error) {
	renderID = strings.TrimSpace(renderID)
	if renderID == "" {
		return rlm.RenderProfileBinding{}, fmt.Errorf("render profile bind: render_id is empty")
	}
	h.renderMu.Lock()
	defer h.renderMu.Unlock()
	result, ok := h.renderResults[renderID]
	if !ok {
		return rlm.RenderProfileBinding{}, fmt.Errorf("render profile bind: unknown render %q (no render telemetry cached for this job id)", renderID)
	}
	if result.Status != "ready" {
		return rlm.RenderProfileBinding{}, fmt.Errorf("render profile bind: render %q is not ready (status %q; only completed renders with a deliverable can be bound)", renderID, result.Status)
	}
	binding, err := rlm.NewRenderProfileBinding(renderID, profileID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return rlm.RenderProfileBinding{}, err
	}
	if err := h.renderProfileBindingIndexLocked().Bind(binding); err != nil {
		return rlm.RenderProfileBinding{}, err
	}
	return binding, nil
}

// ListRenderProfileBindings discloses all current bindings (with resolved
// profile vectors and assertion semantics; unresolvable bindings stay visible
// as unresolvable rows) plus the available built-in delivery profiles.
func (h *Harness) ListRenderProfileBindings() (map[string]any, error) {
	bindings := h.RenderProfileBindingsSnapshot()
	return map[string]any{
		"schema_version":     rlm.RenderBindingSchemaVersion,
		"bindings":           rlm.BuildRenderBindingDisclosure(bindings),
		"available_profiles": rlm.BuiltinDeliveryProfiles(),
	}, nil
}

// RenderProfileBindingsSnapshot returns the binding set sorted by render id
// (persistence snapshot for chat's project runtime state).
func (h *Harness) RenderProfileBindingsSnapshot() []rlm.RenderProfileBinding {
	h.renderMu.Lock()
	defer h.renderMu.Unlock()
	return h.renderProfileBindingIndexLocked().Bindings()
}

// RestoreRenderProfileBindings rebuilds binding state from a persisted set.
// Each record re-validates through the fail-closed constructor, so entries
// with unknown profile ids (retired built-ins) or binding conflicts are
// dropped and counted rather than executing or failing the whole restore.
func (h *Harness) RestoreRenderProfileBindings(bindings []rlm.RenderProfileBinding) int {
	h.renderMu.Lock()
	defer h.renderMu.Unlock()
	idx := h.renderProfileBindingIndexLocked()
	dropped := 0
	for _, binding := range bindings {
		rebuilt, err := rlm.NewRenderProfileBinding(binding.RenderID, binding.ProfileID, binding.BoundAt)
		if err != nil {
			dropped++
			continue
		}
		if err := idx.Bind(rebuilt); err != nil {
			dropped++
		}
	}
	return dropped
}

func (h *Harness) renderProfileBindingIndexLocked() *rlm.RenderBindingIndex {
	if h.renderProfileBindings == nil {
		h.renderProfileBindings = rlm.NewRenderBindingIndex()
	}
	return h.renderProfileBindings
}

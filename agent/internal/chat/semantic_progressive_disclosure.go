package chat

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
	"vit-daw-agent/internal/semanticorchestrator"
)

// semanticProgressiveDisclosureState returns the persisted state without
// trusting its concrete in-memory type. Interaction recovery commonly turns
// it into map[string]any, while same-process calls may retain the struct.
func semanticProgressiveDisclosureState(value any) (semanticorchestrator.State, bool) {
	if typed, ok := value.(semanticorchestrator.State); ok {
		return typed, true
	}
	raw, err := json.Marshal(value)
	if err != nil || string(raw) == "null" || len(raw) == 0 {
		return semanticorchestrator.State{}, false
	}
	var state semanticorchestrator.State
	if err := json.Unmarshal(raw, &state); err != nil || state.SchemaVersion != semanticorchestrator.SchemaVersion {
		return semanticorchestrator.State{}, false
	}
	return state, true
}

func semanticProgressiveDisclosurePersist(requestContext map[string]any, orchestrator *semanticorchestrator.Orchestrator) {
	if requestContext == nil || orchestrator == nil {
		return
	}
	requestContext["semantic_progressive_disclosure"] = orchestrator.State()
}

func semanticProgressiveDisclosureOrchestrator(requestContext map[string]any) (*semanticorchestrator.Orchestrator, error) {
	registry, err := processorregistry.Default()
	if err != nil {
		return nil, err
	}
	if state, ok := semanticProgressiveDisclosureState(requestContext["semantic_progressive_disclosure"]); ok {
		// A free-state internal resume starts a new bounded action cycle after
		// the previous receipt. Do not try to mutate a completed state machine.
		if state.Stage == semanticorchestrator.StageReceipt &&
			(contextBool(requestContext, "free_state_internal_resume") || len(firstMapFromAny(requestContext["free_state_latest_action_evidence"])) > 0) {
			return semanticorchestrator.New(registry)
		}
		return semanticorchestrator.Resume(registry, state)
	}
	return semanticorchestrator.New(registry)
}

// semanticProgressiveDisclosureAccept binds the model-owned intent and, when
// evidence is already present, advances through the observation/control-brief
// boundary. It never creates a view id or a family from user text.
func semanticProgressiveDisclosureAccept(requestContext map[string]any, intent processorintent.Intent, result agentloop.Result, observation *agentloop.RecentObservation) error {
	orchestrator, err := semanticProgressiveDisclosureOrchestrator(requestContext)
	if err != nil {
		return err
	}
	if orchestrator.State().Stage == semanticorchestrator.StageIntent {
		if err := orchestrator.AcceptIntent(intent); err != nil {
			return err
		}
	}
	if err := semanticProgressiveDisclosureRecordObservation(orchestrator, requestContext, result, observation); err != nil {
		return err
	}
	state := orchestrator.State()
	if state.Stage == semanticorchestrator.StageControlBrief {
		if err := orchestrator.SetControlBrief(); err != nil {
			return err
		}
	}
	semanticProgressiveDisclosurePersist(requestContext, orchestrator)
	return nil
}

func semanticProgressiveDisclosureRecordObservation(orchestrator *semanticorchestrator.Orchestrator,
	requestContext map[string]any, result agentloop.Result, observation *agentloop.RecentObservation) error {
	if orchestrator == nil || orchestrator.State().Stage != semanticorchestrator.StageObservation {
		return nil
	}
	if observation == nil {
		return fmt.Errorf("semantic progressive disclosure requires a successful model-requested observation before action")
	}
	status := strings.ToLower(strings.TrimSpace(observation.Status))
	// "partial" is disclosable by design for DAD source-only domain views
	// (e.g. track.band_dynamics), and the runner-side formal gates for the
	// admitted families already accept ready|partial with a fresh audit
	// receipt, so the disclosure level aligns here; error/rejected/missing/
	// empty stay rejected (FAM6-S2 wall 20260902_111733).
	if status != "ok" && status != "ready" && status != "success" && status != "partial" {
		return fmt.Errorf("semantic progressive disclosure requires a successful observation, got %q", observation.Status)
	}
	// The CCB audit receipt is the authoritative proof of what the model
	// requested and what the server executed. A generic views map is not enough:
	// accepting it would allow a caller to manufacture an apparent observation.
	audit := firstMapFromAny(observation.Summary["audit_receipt"])
	if len(audit) == 0 {
		return fmt.Errorf("semantic progressive disclosure requires a CCB observation audit receipt")
	}
	auditStatus := strings.ToLower(strings.TrimSpace(firstStringFromMap(audit, "status")))
	if auditStatus != "" && auditStatus != "ok" && auditStatus != "ready" && auditStatus != "success" && auditStatus != "executed" {
		return fmt.Errorf("semantic progressive disclosure observation audit is not successful: %s", auditStatus)
	}
	if firstStringFromMap(audit, "schema_version") != "ccb_observation_receipt.v1" {
		return fmt.Errorf("semantic progressive disclosure requires a v1 CCB observation audit receipt")
	}
	if !strings.EqualFold(strings.TrimSpace(firstStringFromMap(audit, "requested_by")), "model") {
		return fmt.Errorf("semantic progressive disclosure observation must be requested by the model")
	}
	if !boolValue(audit["view_set_matches"]) {
		return fmt.Errorf("semantic progressive disclosure observation audit reports a view-set mismatch")
	}
	auditModelViews := stringListValue(audit["model_requested_view_ids"])
	auditExecutedViews := stringListValue(audit["actual_executed_view_ids"])
	if len(auditModelViews) == 0 || len(auditExecutedViews) == 0 {
		return fmt.Errorf("semantic progressive disclosure observation audit is missing requested or executed view ids")
	}
	if strings.TrimSpace(firstStringFromMap(audit, "scope")) == "" || len(firstMapFromAny(audit["freshness"])) == 0 {
		return fmt.Errorf("semantic progressive disclosure observation audit is missing scope or freshness")
	}
	// Expanding the disclosure level to partial does not expand freshness:
	// a stale audit freshness stays rejected, mirroring the runner-side
	// "ready or partial and fresh" formal-gate wording.
	if strings.EqualFold(strings.TrimSpace(firstStringFromMap(firstMapFromAny(audit["freshness"]), "status")), "stale") {
		return fmt.Errorf("semantic progressive disclosure observation audit freshness is stale")
	}
	modelViews, _ := semanticProgressiveDisclosureViewIDs(requestContext, result, observation)
	if len(modelViews) == 0 {
		modelViews = append([]string(nil), auditModelViews...)
	}
	if err := semanticProgressiveDisclosureSameViewSet(modelViews, auditModelViews); err != nil {
		return fmt.Errorf("semantic progressive disclosure model request does not match CCB audit: %w", err)
	}
	if err := semanticProgressiveDisclosureSameViewSet(auditModelViews, auditExecutedViews); err != nil {
		return fmt.Errorf("semantic progressive disclosure CCB audit reports a view-set mismatch: %w", err)
	}
	if len(modelViews) == 0 || len(auditExecutedViews) == 0 {
		return fmt.Errorf("semantic progressive disclosure requires a successful model-requested observation before action")
	}
	return orchestrator.RecordObservation(modelViews, auditExecutedViews)
}

func semanticProgressiveDisclosureAdvanceToConfirmation(requestContext map[string]any, refs []string) error {
	orchestrator, err := semanticProgressiveDisclosureOrchestrator(requestContext)
	if err != nil {
		return err
	}
	state := orchestrator.State()
	if state.Stage == semanticorchestrator.StageObservation {
		return fmt.Errorf("semantic progressive disclosure is missing its observation stage")
	}
	if state.Stage == semanticorchestrator.StageControlBrief {
		if err := orchestrator.SetControlBrief(); err != nil {
			return err
		}
		state = orchestrator.State()
	}
	if state.Stage == semanticorchestrator.StagePhysicalTarget {
		if err := orchestrator.SetPhysicalTarget(); err != nil {
			return err
		}
		state = orchestrator.State()
	}
	if state.Stage == semanticorchestrator.StageControlBinding {
		if err := orchestrator.BindControlRefs(refs); err != nil {
			return err
		}
	}
	semanticProgressiveDisclosurePersist(requestContext, orchestrator)
	return nil
}

func semanticProgressiveDisclosureConfirm(requestContext map[string]any) error {
	orchestrator, err := semanticProgressiveDisclosureOrchestrator(requestContext)
	if err != nil {
		return err
	}
	if orchestrator.State().Stage != semanticorchestrator.StageConfirmation {
		return fmt.Errorf("semantic progressive disclosure confirmation is not pending")
	}
	if err := orchestrator.Confirm(); err != nil {
		return err
	}
	semanticProgressiveDisclosurePersist(requestContext, orchestrator)
	return nil
}

func semanticProgressiveDisclosureReceipt(requestContext map[string]any, receipt map[string]any) error {
	orchestrator, err := semanticProgressiveDisclosureOrchestrator(requestContext)
	if err != nil {
		return err
	}
	if err := orchestrator.RecordReceipt(receipt); err != nil {
		return err
	}
	semanticProgressiveDisclosurePersist(requestContext, orchestrator)
	return nil
}

func semanticProgressiveDisclosureViewIDs(requestContext map[string]any, result agentloop.Result, observation *agentloop.RecentObservation) ([]string, []string) {
	modelViews := []string{}
	if result.FreeStateDecision != nil {
		modelViews = append(modelViews, result.FreeStateDecision.RequestedViewIDs...)
	}
	if len(modelViews) == 0 {
		loop := firstMapFromAny(requestContext["free_state_reasoning_loop"])
		latest := firstMapFromAny(loop["latest_decision"])
		modelViews = append(modelViews, stringListValue(latest["requested_view_ids"])...)
	}
	if len(modelViews) == 0 {
		latest := firstMapFromAny(requestContext["free_state_latest_decision"])
		modelViews = append(modelViews, stringListValue(latest["requested_view_ids"])...)
	}

	executedViews := []string{}
	if observation != nil {
		summary := observation.Summary
		audit := firstMapFromAny(summary["audit_receipt"])
		executedViews = append(executedViews, stringListValue(audit["actual_executed_view_ids"])...)
		if len(modelViews) == 0 {
			modelViews = append(modelViews, stringListValue(audit["model_requested_view_ids"])...)
		}
	}
	return semanticProgressiveDisclosureUnique(modelViews), semanticProgressiveDisclosureUnique(executedViews)
}

func semanticProgressiveDisclosureUnique(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func semanticProgressiveDisclosureSameViewSet(left, right []string) error {
	canonical := func(values []string) ([]string, error) {
		seen := map[string]bool{}
		out := make([]string, 0, len(values))
		for _, raw := range values {
			value := strings.TrimSpace(raw)
			if value == "" || seen[value] {
				return nil, fmt.Errorf("view ids must be non-empty and unique")
			}
			seen[value] = true
			out = append(out, value)
		}
		sort.Strings(out)
		return out, nil
	}
	left, err := canonical(left)
	if err != nil {
		return err
	}
	right, err = canonical(right)
	if err != nil {
		return err
	}
	if len(left) != len(right) {
		return fmt.Errorf("view sets differ")
	}
	for index := range left {
		if left[index] != right[index] {
			return fmt.Errorf("view sets differ")
		}
	}
	return nil
}

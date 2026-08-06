package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/frequencycleanup"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/semanticeffect"
)

const c1PlannerTimeout = 3 * time.Minute

type c1EQTargetContext struct {
	Treatment frequencycleanup.TreatmentItem `json:"treatment"`
	Instance  semanticTreatmentInstance      `json:"instance"`
}

func (s *Server) planC1Treatment(ctx context.Context, conversationID, userText string, model frequencycleanup.Model, cfg config.EngineConfig) (frequencycleanup.TreatmentPlan, error) {
	if s == nil || s.llm == nil {
		return frequencycleanup.TreatmentPlan{}, fmt.Errorf("C1 treatment LLM is unavailable")
	}
	input := map[string]any{"user_request": userText, "diagnosis": map[string]any{"model_id": model.ModelID, "observation_id": model.ObservationID, "scope": "full_project", "tracks": model.Tracks, "diagnosis_candidates": model.Candidates, "coverage": model.Coverage, "limitations": model.Limitations}}
	payload, _ := json.Marshal(input)
	system := `You are the professional project-level decision phase of C1 Frequency Cleanup.
Return ONLY frequency_cleanup.treatment_plan.v1 JSON:
{"schema_version":"frequency_cleanup.treatment_plan.v1","summary":"project decision","items":[{"order":1,"track_id":"exact supplied id","classification":"static_eq|defer_dynamic_processing|defer_space_processing|defer_automation|arrangement_or_source|no_change","diagnosis_refs":["exact supplied candidate id"],"listening_goal":"required only for static_eq","rationale":"why this classification","constraints":[],"evidence_refs":[]}],"global_constraints":[],"evidence_refs":[],"limitations":[]}.

Rules:
- Return exactly one item for every supplied project track and no other track. Explicit no_change is required; omission is invalid.
- A track with silence_confirmed=true has deterministic source-and-post-fader all-zero evidence and must be classified no_change.
- Energy overlap is a candidate, not proof of masking. Use role and relative tonal evidence as context, not instrument-name dogma.
- static_eq means a time-invariant spectral correction justified by current evidence.
- Time-varying level/tone belongs to defer_dynamic_processing (C2); depth/width/reverb/delay to defer_space_processing (C3); section or movement changes to defer_automation (C4); musical/source problems to arrangement_or_source.
- Do not name or select plug-ins. Do not output frequencies, gain, Q, slopes, EQ shapes, compressor settings, space settings, automation values, stored mappings, web research, or tool calls.
- Deterministic code owns exact instance qualification, static-EQ topology/materialization, confirmation, execution, all-or-rollback, fresh same-tap post-FX verification, and Mixboard propagation.`
	req := llm.Request{Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(payload)}}, Metadata: llm.RequestMetadata{Source: "c1_project_treatment_planner", ConversationID: conversationID}, PreferJSON: true, Timeout: c1PlannerTimeout}
	response, err := s.llm.CompleteRequest(ctx, cfg, req)
	if err != nil {
		return frequencycleanup.TreatmentPlan{}, err
	}
	plan, decodeErr := decodeC1TreatmentPlan(response.Text, model)
	if decodeErr == nil {
		return plan, nil
	}
	req.Messages = append(req.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: fmt.Sprintf("Invalid C1 plan: %s. Return only corrected frequency_cleanup.treatment_plan.v1 JSON with every exact track classified once.", decodeErr)})
	response, err = s.llm.CompleteRequest(ctx, cfg, req)
	if err != nil {
		return frequencycleanup.TreatmentPlan{}, err
	}
	plan, err = decodeC1TreatmentPlan(response.Text, model)
	if err != nil {
		return frequencycleanup.TreatmentPlan{}, fmt.Errorf("C1 treatment plan remained invalid after repair: %w", err)
	}
	return plan, nil
}

func decodeC1TreatmentPlan(text string, model frequencycleanup.Model) (frequencycleanup.TreatmentPlan, error) {
	var plan frequencycleanup.TreatmentPlan
	if err := decodePluginRecommendationJSON(text, &plan); err != nil {
		return plan, err
	}
	return frequencycleanup.FinalizeTreatmentPlan(plan, model)
}

func (s *Server) planC1EQBatch(ctx context.Context, conversationID string, treatment frequencycleanup.TreatmentPlan, baseline frequencycleanup.TargetPostFXBaseline, targets []c1EQTargetContext, cfg config.EngineConfig, rejection string) (semanticeffect.Batch, error) {
	if s == nil || s.llm == nil {
		return semanticeffect.Batch{}, fmt.Errorf("C1 EQ LLM is unavailable")
	}
	if baseline.SchemaVersion != frequencycleanup.TargetPostFXBaselineSchema || !baseline.Readiness.CanProceed {
		return semanticeffect.Batch{}, fmt.Errorf("ready target-scoped post-FX baseline is required before EQ planning")
	}
	rows := make([]map[string]any, 0, len(targets))
	topologies := []map[string]any{}
	seen := map[string]bool{}
	for _, target := range targets {
		structural := b4EQStructuralTopology(target.Instance.Topology)
		ref := "eq_topology_" + semanticEQHash(structural)[:16]
		if !seen[ref] {
			seen[ref] = true
			topologies = append(topologies, map[string]any{"topology_ref": ref, "generic_eq_topology": structural})
		}
		rows = append(rows, map[string]any{"treatment": target.Treatment, "exact_target": map[string]any{"track_id": target.Instance.TrackID, "track_name": target.Treatment.TrackName, "plugin_id": target.Instance.PluginID, "plugin_name": target.Instance.PluginName}, "generic_eq_topology_ref": ref})
	}
	input := map[string]any{"treatment_plan": treatment, "target_post_fx_baseline": baseline, "qualified_targets": rows, "generic_eq_topology_catalog": topologies}
	if strings.TrimSpace(rejection) != "" {
		input["deterministic_materialization_rejection"] = rejection
	}
	payload, _ := json.Marshal(input)
	system := `You are C1's static-EQ planning phase after its whole-project classification.
Return ONLY one semantic_effect_batch.v1 JSON object with exactly one ordinary semantic_effect_action.v1 leaf for every supplied qualified static_eq target, in treatment order:
{"schema_version":"semantic_effect_batch.v1","project_goal":"copy treatment summary","atomic":true,"actions":[` + semanticeffect.StaticEQActionPromptExample + `]}.
Resolve each generic_eq_topology_ref through the supplied catalog. ` + semanticeffect.StaticEQAtomPromptRules + `

Rules:
- Copy each exact track_id/plugin_id pair once; never invent, omit, duplicate, or reorder.
- Use only 1-3 static upsert atoms and only bell, low_shelf, high_shelf, low_cut, or high_cut.
- Derive conservative acoustic values from listening_goal and evidence. Every numeric field needs field_origins.
- Do not output dynamic EQ, compression, spatial processing, automation, plug-in loading, stored mappings, web research, or vendor-specific rules.
- Deterministic shared code owns materialization, preimage, confirmation, execution, readback, rollback, and verification.`
	if strings.TrimSpace(rejection) != "" {
		system += "\nA prior candidate failed deterministic materialization. Treat the rejection as hard and choose reachable alternatives without changing targets."
	}
	req := llm.Request{Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(payload)}}, Metadata: llm.RequestMetadata{Source: "c1_project_eq_planner", ConversationID: conversationID, PromptStats: map[string]any{"target_count": len(targets), "unique_topology_count": len(topologies), "input_bytes": len(payload)}}, PreferJSON: true, Timeout: c1PlannerTimeout}
	response, err := s.llm.CompleteRequest(ctx, cfg, req)
	if err != nil {
		return semanticeffect.Batch{}, err
	}
	batch, decodeErr := decodeC1EQBatch(response.Text, treatment, targets)
	if decodeErr == nil {
		return batch, nil
	}
	req.Messages = append(req.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: fmt.Sprintf("Invalid C1 EQ batch: %s. Return corrected semantic_effect_batch.v1 JSON for every exact target.", decodeErr)})
	response, err = s.llm.CompleteRequest(ctx, cfg, req)
	if err != nil {
		return semanticeffect.Batch{}, err
	}
	batch, err = decodeC1EQBatch(response.Text, treatment, targets)
	if err != nil {
		return semanticeffect.Batch{}, fmt.Errorf("C1 EQ batch remained invalid after repair: %w", err)
	}
	return batch, nil
}

func decodeC1EQBatch(text string, treatment frequencycleanup.TreatmentPlan, targets []c1EQTargetContext) (semanticeffect.Batch, error) {
	var batch semanticeffect.Batch
	if err := decodePluginRecommendationJSON(text, &batch); err != nil {
		return batch, err
	}
	if err := batch.Validate(); err != nil {
		return batch, err
	}
	if len(batch.Actions) != len(targets) {
		return batch, fmt.Errorf("batch must contain exactly %d actions", len(targets))
	}
	for index, action := range batch.Actions {
		want := targets[index]
		if action.Target.TrackID != want.Instance.TrackID || action.Target.PluginID != want.Instance.PluginID {
			return batch, fmt.Errorf("action %d changed or reordered exact target", index+1)
		}
		if strings.TrimSpace(action.UserGoal) == "" {
			return batch, fmt.Errorf("action %d omitted listening goal", index+1)
		}
		if (action.Evidence.Basis == "observation" || action.Evidence.Basis == "both") && action.Evidence.ObservationID != treatment.ObservationID {
			return batch, fmt.Errorf("action %d changed observation identity", index+1)
		}
	}
	return batch, nil
}

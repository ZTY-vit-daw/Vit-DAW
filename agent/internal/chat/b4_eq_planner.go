package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/config"
	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/lowendrelation"
	"vit-daw-agent/internal/semanticeffect"
)

const b4EQPlannerTimeout = 3 * time.Minute

type b4EQTargetContext struct {
	Treatment lowendrelation.TreatmentTarget `json:"treatment"`
	Instance  semanticTreatmentInstance      `json:"instance"`
}

type b4InstanceSelection struct {
	TrackID  string `json:"track_id"`
	PluginID string `json:"plugin_id"`
}

// planB4InstanceSelections makes one bounded project-level choice when one or
// more targets already contain several qualified EQs. It never runs an Agent
// loop per track and it cannot invent or load an instance.
func (s *Server) planB4InstanceSelections(ctx context.Context, conversationID string, treatment lowendrelation.TreatmentPlan, ambiguous map[string][]semanticTreatmentInstance, cfg config.EngineConfig) (map[string]string, error) {
	if s == nil || s.llm == nil {
		return nil, fmt.Errorf("B4 instance-selection LLM is unavailable")
	}
	input := map[string]any{"treatment_plan": treatment, "ambiguous_targets": ambiguous}
	payload, _ := json.Marshal(input)
	system := `You select exact already-loaded generic EQ instances for one full-project B4 treatment plan.
Return ONLY JSON: {"schema_version":"b4.eq_instance_selection.v1","selections":[{"track_id":"exact supplied track","plugin_id":"exact qualified plugin on that track"}]}.
Return exactly one selection for every ambiguous target. Use only supplied identities. Consider the target listening goal and plugin identity/topology, but do not output EQ parameters, load a plugin, invoke tools, use profiles/learning/SPAL/web/the retired focus-position capability, or add prose.`
	req := llm.Request{Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(payload)}}, Metadata: llm.RequestMetadata{Source: "b4_project_eq_instance_selection", ConversationID: conversationID}, PreferJSON: true}
	response, err := s.llm.CompleteRequest(ctx, cfg, req)
	if err != nil {
		return nil, err
	}
	selected, decodeErr := decodeB4InstanceSelections(response.Text, ambiguous)
	if decodeErr == nil {
		return selected, nil
	}
	req.Messages = append(req.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: fmt.Sprintf("Invalid exact instance selection: %s. Return only corrected b4.eq_instance_selection.v1 JSON.", decodeErr)})
	response, err = s.llm.CompleteRequest(ctx, cfg, req)
	if err != nil {
		return nil, err
	}
	selected, err = decodeB4InstanceSelections(response.Text, ambiguous)
	if err != nil {
		return nil, fmt.Errorf("B4 instance selection remained invalid after repair: %w", err)
	}
	return selected, nil
}

func decodeB4InstanceSelections(text string, ambiguous map[string][]semanticTreatmentInstance) (map[string]string, error) {
	var value struct {
		SchemaVersion string                `json:"schema_version"`
		Selections    []b4InstanceSelection `json:"selections"`
	}
	if err := decodePluginRecommendationJSON(text, &value); err != nil {
		return nil, err
	}
	if value.SchemaVersion != "b4.eq_instance_selection.v1" || len(value.Selections) != len(ambiguous) {
		return nil, fmt.Errorf("one selection per ambiguous target is required")
	}
	out := map[string]string{}
	for _, selection := range value.Selections {
		options, ok := ambiguous[selection.TrackID]
		if !ok || out[selection.TrackID] != "" {
			return nil, fmt.Errorf("unknown or duplicate track_id %q", selection.TrackID)
		}
		valid := false
		for _, option := range options {
			if option.PluginID == selection.PluginID {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("plugin_id %q is not qualified on track %q", selection.PluginID, selection.TrackID)
		}
		out[selection.TrackID] = selection.PluginID
	}
	return out, nil
}

// planB4Treatment asks once for the project-level specialist relationship
// decision. It deliberately excludes plug-in identities and EQ values.
func (s *Server) planB4Treatment(ctx context.Context, conversationID, userText string, model lowendrelation.Model, cfg config.EngineConfig) (lowendrelation.TreatmentPlan, error) {
	if s == nil || s.llm == nil {
		return lowendrelation.TreatmentPlan{}, fmt.Errorf("B4 treatment LLM is unavailable")
	}
	input := map[string]any{
		"user_request": userText,
		"diagnosis": map[string]any{
			"model_id": model.ModelID, "observation_id": model.ObservationID,
			"scope": "full_project", "summary": model.Summary,
			"tracks": model.Tracks, "conflicts": model.Conflicts,
			"observations": model.Observations, "limitations": model.Limitations,
		},
	}
	payload, _ := json.Marshal(input)
	system := `You are the acoustic relationship-planning phase of B4, one stage in a full-project A-F mixing workflow.
The deterministic B4 diagnosis is supplied as compact JSON. Decide every project track that needs low-end relationship treatment and their processing order. This is one project-level decision, not a per-track Agent loop.

Return ONLY low_end_relation.treatment_plan.v1 JSON:
{"schema_version":"low_end_relation.treatment_plan.v1","summary":"project decision","targets":[{"order":1,"track_id":"exact supplied id","relationship_refs":["exact supplied conflict_id or observation id"],"listening_goal":"acoustic goal for this track in the relationship","constraints":[],"evidence_refs":[]}],"global_constraints":[],"evidence_refs":[],"limitations":[]}

Rules:
- The observation scope is the full project. Include all and only tracks that need treatment; zero targets is valid when no change is justified.
- Copy only supplied track_id and relationship identifiers. Never invent an identity.
- Express specialist relationship judgement, ordered treatment goals, and constraints only.
- Do not name or select plug-ins. Do not output plugin_id, topology, EQ shapes, frequencies, gains, Q, slopes, profiles, learning, SPAL, the retired focus-position capability, web research, or tool calls.
- Deterministic code will resolve exact EQ instances, materialize topology, confirm, execute, read back, roll back the whole batch, and verify with a fresh full-project observation.`
	req := llm.Request{Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(payload)}}, Metadata: llm.RequestMetadata{Source: "b4_project_treatment_planner", ConversationID: conversationID}, PreferJSON: true}
	response, err := s.llm.CompleteRequest(ctx, cfg, req)
	if err != nil {
		return lowendrelation.TreatmentPlan{}, err
	}
	plan, decodeErr := decodeB4TreatmentPlan(response.Text, model)
	if decodeErr == nil {
		return plan, nil
	}
	req.Messages = append(req.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: fmt.Sprintf("The treatment plan was invalid: %s\nReturn only a corrected low_end_relation.treatment_plan.v1 object using exact supplied identities.", decodeErr)})
	response, err = s.llm.CompleteRequest(ctx, cfg, req)
	if err != nil {
		return lowendrelation.TreatmentPlan{}, err
	}
	plan, err = decodeB4TreatmentPlan(response.Text, model)
	if err != nil {
		return lowendrelation.TreatmentPlan{}, fmt.Errorf("B4 treatment plan remained invalid after repair: %w", err)
	}
	return plan, nil
}

func decodeB4TreatmentPlan(text string, model lowendrelation.Model) (lowendrelation.TreatmentPlan, error) {
	var plan lowendrelation.TreatmentPlan
	if err := decodePluginRecommendationJSON(text, &plan); err != nil {
		return plan, err
	}
	return lowendrelation.FinalizeTreatmentPlan(plan, model)
}

// planB4EQBatch is one compact project call after every exact instance and
// topology has been deterministically qualified.
func (s *Server) planB4EQBatch(ctx context.Context, conversationID string, treatment lowendrelation.TreatmentPlan, targets []b4EQTargetContext, cfg config.EngineConfig, rejection string) (semanticeffect.Batch, error) {
	if s == nil || s.llm == nil {
		return semanticeffect.Batch{}, fmt.Errorf("B4 EQ LLM is unavailable")
	}
	input, topologyCount := b4EQPlannerInput(treatment, targets, rejection)
	payload, _ := json.Marshal(input)
	system := `You are the acoustic EQ-planning phase used by B4 after its full-project relationship decision.
Return ONLY one semantic_effect_batch.v1 JSON object. This is one project-level call; include exactly one ordinary semantic_effect_action.v1 leaf for every supplied qualified target, in treatment order.
{"schema_version":"semantic_effect_batch.v1","project_goal":"copy treatment summary","atomic":true,"actions":[` + semanticeffect.StaticEQActionPromptExample + `]}

Every actions[] leaf uses the ordinary Agent's shared generic static-EQ contract. Replace the example placeholders with the exact supplied target and listening/evidence values. Resolve each qualified target's generic_eq_topology_ref through generic_eq_topology_catalog; identical structural topologies are intentionally listed once and shared by reference.
` + semanticeffect.StaticEQAtomPromptRules + `

Rules:
- Copy every exact track_id/plugin_id pair once; never invent, omit, duplicate, or reorder a target.
- Each leaf uses 1-3 static upsert atoms and the same generic EQ semantics as the ordinary Agent: bell, low_shelf, high_shelf, low_cut, high_cut.
- Choose acoustic values from the B4 listening goal and relationship evidence, constrained by that target's supplied topology. Every numeric field needs field_origins with user_fixed, llm_selected, or context_inherited.
- Preserve project and target constraints. Do not output plugin loading, profiles, learning, SPAL, dynamic EQ, the retired focus-position capability, web research, or vendor-specific parameter rules.
- The batch is atomic: deterministic code owns materialization, requested/exact/quantized/rejected reporting, confirmation, execution, actual readback, project-wide rollback, and verification.`
	if strings.TrimSpace(rejection) != "" {
		system += "\nA prior candidate was rejected by deterministic topology materialization. Treat every listed rejection as hard and choose reachable alternatives without changing exact targets."
	}
	req := llm.Request{
		Messages: []llm.Message{{Role: "system", Content: system}, {Role: "user", Content: string(payload)}},
		Metadata: llm.RequestMetadata{
			Source: "b4_project_eq_planner", ConversationID: conversationID,
			PromptStats: map[string]any{"target_count": len(targets), "unique_topology_count": topologyCount, "input_bytes": len(payload)},
		},
		PreferJSON: true,
		Timeout:    b4EQPlannerTimeout,
	}
	response, err := s.llm.CompleteRequest(ctx, cfg, req)
	if err != nil {
		return semanticeffect.Batch{}, err
	}
	batch, decodeErr := decodeB4EQBatch(response.Text, treatment, targets)
	if decodeErr == nil {
		return batch, nil
	}
	req.Messages = append(req.Messages, llm.Message{Role: "assistant", Content: response.Text}, llm.Message{Role: "user", Content: fmt.Sprintf("The batch was invalid: %s\nReturn only a corrected semantic_effect_batch.v1 object for every exact supplied target. Every corrected atom must obey: %s", decodeErr, semanticeffect.StaticEQAtomPromptRules)})
	response, err = s.llm.CompleteRequest(ctx, cfg, req)
	if err != nil {
		return semanticeffect.Batch{}, err
	}
	batch, err = decodeB4EQBatch(response.Text, treatment, targets)
	if err != nil {
		return semanticeffect.Batch{}, fmt.Errorf("B4 EQ batch remained invalid after repair: %w", err)
	}
	return batch, nil
}

func b4EQPlannerInput(treatment lowendrelation.TreatmentPlan, targets []b4EQTargetContext, rejection string) (map[string]any, int) {
	rows := make([]map[string]any, 0, len(targets))
	topologies := make([]map[string]any, 0, len(targets))
	seenTopology := map[string]bool{}
	for _, target := range targets {
		structural := b4EQStructuralTopology(target.Instance.Topology)
		ref := "eq_topology_" + semanticEQHash(structural)[:16]
		if !seenTopology[ref] {
			seenTopology[ref] = true
			topologies = append(topologies, map[string]any{
				"topology_ref": ref, "generic_eq_topology": structural,
			})
		}
		rows = append(rows, map[string]any{
			"treatment": target.Treatment,
			"exact_target": map[string]any{
				"track_id": target.Instance.TrackID, "track_name": target.Treatment.TrackName,
				"plugin_id": target.Instance.PluginID, "plugin_name": target.Instance.PluginName,
			},
			"generic_eq_topology_ref": ref,
		})
	}
	input := map[string]any{
		"treatment_plan":              treatment,
		"qualified_targets":           rows,
		"generic_eq_topology_catalog": topologies,
	}
	if strings.TrimSpace(rejection) != "" {
		input["deterministic_materialization_rejection"] = rejection
	}
	return input, len(topologies)
}

func b4EQStructuralTopology(topology map[string]any) map[string]any {
	out := cloneContext(topology)
	delete(out, "track_id")
	delete(out, "plugin_id")
	delete(out, "topology_generation")
	return out
}

func decodeB4EQBatch(text string, treatment lowendrelation.TreatmentPlan, targets []b4EQTargetContext) (semanticeffect.Batch, error) {
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

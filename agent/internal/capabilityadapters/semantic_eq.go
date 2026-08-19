package capabilityadapters

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/lowendrelation"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/semanticeffect"
)

const AgentSemanticEQCapabilityID = "agent.effect.eq_control.v0"

// SemanticEQPlan is the read-only materialization of one LLM-authored EQ
// action. PlannedEdits/Writes and ParameterPreimage are produced by the
// existing generic EQ planner; the adapter only freezes them.
type SemanticEQPlan struct {
	Action semanticeffect.Action `json:"semantic_action"`
	// PCAAdmissionReceipt is the server-issued proof for the exact installed
	// processor selected before loading.  It is carried into the frozen action
	// set as an opaque map so the final executor can revalidate the same
	// installed binary even when the live rack parameter digest omits optional
	// identity fields.
	PCAAdmissionReceipt  map[string]any   `json:"pca_admission_receipt,omitempty"`
	TopologyGeneration   string           `json:"topology_generation"`
	Edits                []map[string]any `json:"edits"`
	PlannedEdits         []map[string]any `json:"planned_edits"`
	PlannedWrites        []map[string]any `json:"planned_writes"`
	ParameterPreimage    []map[string]any `json:"parameter_preimage"`
	ParameterSnapshot    []map[string]any `json:"parameter_snapshot"`
	PreviewResults       []map[string]any `json:"preview_results"`
	EvidenceSummary      map[string]any   `json:"evidence_summary,omitempty"`
	EvidenceRefs         []string         `json:"evidence_refs,omitempty"`
	PreviousObservation  string           `json:"previous_observation_id,omitempty"`
	AcousticVerification string           `json:"acoustic_verification"`
}

// SemanticEQBatchPlan freezes B4's project-level relationship decision and
// all already-materialized ordinary generic-EQ leaves into one governed
// action. Keeping a single top-level action lets the B4 mutation port provide
// project-wide all-or-rollback semantics without changing the existing
// single-instance EQ executor.
type SemanticEQBatchPlan struct {
	Treatment           lowendrelation.TreatmentPlan `json:"treatment_plan"`
	Diagnosis           lowendrelation.Model         `json:"diagnosis_model"`
	VerificationContext any                          `json:"verification_context,omitempty"`
	Batch               semanticeffect.Batch         `json:"semantic_batch"`
	Leaves              []SemanticEQPlan             `json:"leaves"`
}

// ProjectSemanticEQBatchSpec parameterizes the shared project-level freezer.
// Specialist layers own diagnosis and treatment semantics; this adapter owns
// only the already-materialized ordinary generic-EQ leaves and control-plane
// invariants.
type ProjectSemanticEQBatchSpec struct {
	CapabilityID        string
	CapabilityVersion   string
	PlanID              string
	ActionID            string
	ActionSetPrefix     string
	Command             string
	TargetRef           string
	CandidatePrefix     string
	VerificationRef     string
	Summary             string
	IdempotencyClass    string
	Treatment           any
	Diagnosis           any
	VerificationContext any
	Batch               semanticeffect.Batch
	Leaves              []SemanticEQPlan
	ExpectedTargetCount int
}

func FreezeSemanticEQBatch(plan SemanticEQBatchPlan, cut orchestration.ProjectCut, revision int64) (orchestration.Proposal, orchestration.ActionSet, error) {
	if plan.Treatment.SchemaVersion != lowendrelation.TreatmentPlanSchema || plan.Treatment.PlanID == "" || len(plan.Leaves) != len(plan.Batch.Actions) || len(plan.Leaves) != len(plan.Treatment.Targets) {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("complete B4 treatment and one frozen leaf per target are required")
	}
	return FreezeProjectSemanticEQBatch(ProjectSemanticEQBatchSpec{
		CapabilityID: lowendrelation.CapabilityID, CapabilityVersion: "v0", PlanID: plan.Treatment.PlanID,
		ActionID: "b4_semantic_eq_batch", ActionSetPrefix: "actionset_b4_eq_", Command: "b4.semantic_eq_batch.governed",
		TargetRef: "project:low_end_relationships", CandidatePrefix: "b4_eq_", VerificationRef: "static_mix.low_end_relation.eq_batch.verification.v1",
		Summary:          fmt.Sprintf("Apply %d full-project B4 generic-EQ target(s) as one all-or-rollback batch", len(plan.Leaves)),
		IdempotencyClass: "b4_project_eq_all_or_rollback", Treatment: plan.Treatment, Diagnosis: plan.Diagnosis,
		VerificationContext: plan.VerificationContext, Batch: plan.Batch, Leaves: plan.Leaves, ExpectedTargetCount: len(plan.Treatment.Targets),
	}, cut, revision)
}

func FreezeProjectSemanticEQBatch(spec ProjectSemanticEQBatchSpec, cut orchestration.ProjectCut, revision int64) (orchestration.Proposal, orchestration.ActionSet, error) {
	if err := spec.Batch.Validate(); err != nil {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("semantic EQ batch: %w", err)
	}
	if strings.TrimSpace(spec.CapabilityID) == "" || strings.TrimSpace(spec.CapabilityVersion) == "" || strings.TrimSpace(spec.PlanID) == "" || strings.TrimSpace(spec.Command) == "" || strings.TrimSpace(spec.VerificationRef) == "" {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("complete project semantic-EQ freezer identity is required")
	}
	if spec.ExpectedTargetCount < 1 || len(spec.Leaves) != len(spec.Batch.Actions) || len(spec.Leaves) != spec.ExpectedTargetCount {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("one frozen semantic-EQ leaf per executable target is required")
	}
	if revision < 1 {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("positive proposal revision is required")
	}
	leafRows := make([]map[string]any, 0, len(spec.Leaves))
	targetScope := make([]string, 0, len(spec.Leaves))
	for index, leaf := range spec.Leaves {
		if err := leaf.Action.Validate(); err != nil {
			return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("leaf %d: %w", index+1, err)
		}
		if leaf.Action.Target != spec.Batch.Actions[index].Target || leaf.TopologyGeneration == "" || len(leaf.Edits) == 0 || len(leaf.PlannedEdits) != len(leaf.Action.EQPlan.Atoms) || len(leaf.PlannedWrites) == 0 || len(leaf.ParameterPreimage) == 0 {
			return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("leaf %d lacks exact identity, frozen topology, writes, or preimage", index+1)
		}
		leafRows = append(leafRows, map[string]any{
			"semantic_action": leaf.Action, "track_id": leaf.Action.Target.TrackID, "plugin_id": leaf.Action.Target.PluginID, "topology_generation": leaf.TopologyGeneration,
			"edits": cloneSemanticEQRows(leaf.Edits), "planned_edits": cloneSemanticEQRows(leaf.PlannedEdits),
			"planned_writes": cloneSemanticEQRows(leaf.PlannedWrites), "parameter_preimage": cloneSemanticEQRows(leaf.ParameterPreimage),
			"parameter_snapshot": cloneSemanticEQRows(leaf.ParameterSnapshot), "preview_results": cloneSemanticEQRows(leaf.PreviewResults),
			"evidence_summary": clonePluginMap(leaf.EvidenceSummary), "evidence_refs": append([]string(nil), leaf.EvidenceRefs...),
		})
		targetScope = append(targetScope, "plugin:"+leaf.Action.Target.TrackID+":"+leaf.Action.Target.PluginID)
	}
	if cut.Hash == "" {
		cut.Hash = cut.ComputeHash()
	}
	if cut.Hash == "" || !cut.IsExecutable() {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("strong executable project cut is required")
	}
	args := map[string]any{"treatment_plan": spec.Treatment, "diagnosis_model": spec.Diagnosis, "semantic_batch": spec.Batch, "leaves": leafRows}
	if spec.VerificationContext != nil {
		args["verification_context"] = spec.VerificationContext
	}
	action := orchestration.Action{
		ID: firstNonEmptySemanticEQ(spec.ActionID, "project_semantic_eq_batch"), Command: spec.Command, TargetRef: firstNonEmptySemanticEQ(spec.TargetRef, "project:semantic_eq"),
		BeforeFingerprint: "project-eq-preimages:" + semanticEQJSONHash(leafRows),
		Args:              args,
		Compensatable:     true, IdempotencyClass: firstNonEmptySemanticEQ(spec.IdempotencyClass, "project_semantic_eq_all_or_rollback"),
	}
	actionSet := orchestration.ActionSet{ID: firstNonEmptySemanticEQ(spec.ActionSetPrefix, "actionset_project_eq_") + sanitizeSemanticEQID(spec.PlanID), CapabilityID: spec.CapabilityID, ProjectCutHash: cut.Hash, Actions: []orchestration.Action{action}}
	actionSet.Hash = actionSet.ComputeHash()
	proposal := orchestration.Proposal{
		ID: "proposal_" + actionSet.Hash[:16], Revision: revision, CapabilityID: spec.CapabilityID, CapabilityVer: spec.CapabilityVersion,
		ProjectCutHash: cut.Hash, CandidateID: firstNonEmptySemanticEQ(spec.CandidatePrefix, "project_eq_") + actionSet.Hash[:16], ActionSetHash: actionSet.Hash,
		TargetScope: targetScope, Risk: "bounded_reversible", VerificationRef: spec.VerificationRef,
		Summary: firstNonEmptySemanticEQ(spec.Summary, fmt.Sprintf("Apply %d project generic-EQ target(s) as one all-or-rollback batch", len(spec.Leaves))), CreatedAt: time.Now().UTC(),
	}
	return proposal, actionSet, nil
}

func firstNonEmptySemanticEQ(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func FreezeSemanticEQ(plan SemanticEQPlan, cut orchestration.ProjectCut, revision int64) (orchestration.Proposal, orchestration.ActionSet, error) {
	if err := plan.Action.Validate(); err != nil {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("semantic EQ action: %w", err)
	}
	if revision < 1 {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("positive proposal revision is required")
	}
	if strings.TrimSpace(plan.TopologyGeneration) == "" || len(plan.Edits) == 0 || len(plan.PlannedEdits) != len(plan.Action.EQPlan.Atoms) || len(plan.PlannedWrites) == 0 || len(plan.ParameterPreimage) == 0 {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("frozen topology, planned edits/writes, and parameter preimage are required")
	}
	if cut.Hash == "" {
		cut.Hash = cut.ComputeHash()
	}
	if cut.Hash == "" || !cut.IsExecutable() {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("strong executable project cut is required")
	}
	args := map[string]any{
		"semantic_action":         plan.Action,
		"pca_admission_receipt":   clonePluginMap(plan.PCAAdmissionReceipt),
		"track_id":                strings.TrimSpace(plan.Action.Target.TrackID),
		"plugin_id":               strings.TrimSpace(plan.Action.Target.PluginID),
		"topology_generation":     strings.TrimSpace(plan.TopologyGeneration),
		"edits":                   cloneSemanticEQRows(plan.Edits),
		"planned_edits":           cloneSemanticEQRows(plan.PlannedEdits),
		"planned_writes":          cloneSemanticEQRows(plan.PlannedWrites),
		"parameter_preimage":      cloneSemanticEQRows(plan.ParameterPreimage),
		"parameter_snapshot":      cloneSemanticEQRows(plan.ParameterSnapshot),
		"preview_results":         cloneSemanticEQRows(plan.PreviewResults),
		"evidence_summary":        clonePluginMap(plan.EvidenceSummary),
		"evidence_refs":           append([]string(nil), plan.EvidenceRefs...),
		"previous_observation_id": strings.TrimSpace(plan.PreviousObservation),
		"acoustic_verification":   strings.TrimSpace(plan.AcousticVerification),
	}
	preimageHash := semanticEQJSONHash(plan.ParameterPreimage)
	action := orchestration.Action{
		ID:                "semantic_eq:" + sanitizeSemanticEQID(plan.Action.Target.PluginID),
		Command:           "plugin_grabber.apply_eq_edits.governed",
		TargetRef:         "plugin:" + strings.TrimSpace(plan.Action.Target.TrackID) + ":" + strings.TrimSpace(plan.Action.Target.PluginID),
		BeforeFingerprint: "eq-preimage:" + preimageHash,
		Args:              args,
		Compensatable:     true,
		IdempotencyClass:  "semantic_eq_with_frozen_topology_and_preimage",
	}
	actionSet := orchestration.ActionSet{
		ID:             "actionset_semantic_eq_" + sanitizeSemanticEQID(plan.Action.Target.PluginID),
		CapabilityID:   AgentSemanticEQCapabilityID,
		ProjectCutHash: cut.Hash,
		Actions:        []orchestration.Action{action},
	}
	actionSet.Hash = actionSet.ComputeHash()
	if actionSet.Hash == "" {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("could not hash semantic EQ action set")
	}
	proposal := orchestration.Proposal{
		ID:              "proposal_" + actionSet.Hash[:16],
		Revision:        revision,
		CapabilityID:    AgentSemanticEQCapabilityID,
		CapabilityVer:   "v0",
		ProjectCutHash:  cut.Hash,
		CandidateID:     "semantic_eq_" + actionSet.Hash[:16],
		ActionSetHash:   actionSet.Hash,
		TargetScope:     []string{action.TargetRef},
		Risk:            "bounded_reversible",
		VerificationRef: "agent.effect.eq_control.verification.v0",
		Summary:         fmt.Sprintf("Apply %d atomic generic-EQ edit(s) to plugin %s on track %s", len(plan.Action.EQPlan.Atoms), plan.Action.Target.PluginID, plan.Action.Target.TrackID),
		CreatedAt:       time.Now().UTC(),
	}
	return proposal, actionSet, nil
}

func semanticEQJSONHash(value any) string {
	b, _ := json.Marshal(value)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func sanitizeSemanticEQID(value string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(value) {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "target"
	}
	return b.String()
}

func cloneSemanticEQRows(in []map[string]any) []map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make([]map[string]any, len(in))
	for i, row := range in {
		out[i] = clonePluginMap(row)
	}
	return out
}

func clonePluginMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = clonePluginValue(value)
	}
	return out
}

func clonePluginValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return clonePluginMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = clonePluginValue(item)
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(typed))
		for i, item := range typed {
			out[i] = clonePluginMap(item)
		}
		return out
	default:
		return value
	}
}

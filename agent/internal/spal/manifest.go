package spal

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"vit-daw-agent/internal/orchestration"
)

const (
	ActionCommand = "spal.apply.v0"

	OperationApply    = "apply"
	OperationRollback = "rollback"
)

type ExecutionManifest struct {
	SchemaVersion string              `json:"schema_version"`
	ID            string              `json:"id"`
	Instruction   Instruction         `json:"instruction"`
	Binding       RuntimeBinding      `json:"binding"`
	Writes        []PhysicalParameter `json:"writes"`
	Preimage      []PhysicalParameter `json:"preimage"`
	EvidenceRefs  []string            `json:"evidence_refs,omitempty"`
	Operation     string              `json:"operation,omitempty"`
	RollbackOf    string              `json:"rollback_of,omitempty"`
}

func CompileManifest(resolution Resolution, instruction Instruction, preimage []PhysicalParameter, adapter Adapter) (ExecutionManifest, error) {
	if resolution.Status != ResolutionBound || resolution.Binding == nil {
		return ExecutionManifest{}, fmt.Errorf("a bound SPAL resolution is required")
	}
	if adapter == nil {
		return ExecutionManifest{}, fmt.Errorf("SPAL adapter is required")
	}
	if err := instruction.Validate(); err != nil {
		return ExecutionManifest{}, err
	}
	if err := resolution.Binding.Valid(); err != nil {
		return ExecutionManifest{}, err
	}
	if resolution.Binding.Provider.ID != adapter.Descriptor().ID {
		return ExecutionManifest{}, fmt.Errorf("resolution binding does not belong to the supplied adapter")
	}
	writes, err := adapter.Compile(instruction, *resolution.Binding)
	if err != nil {
		return ExecutionManifest{}, err
	}
	if err := validatePhysicalParameters(writes); err != nil {
		return ExecutionManifest{}, fmt.Errorf("compiled writes: %w", err)
	}
	if err := validatePreimage(writes, preimage); err != nil {
		return ExecutionManifest{}, err
	}
	manifest := ExecutionManifest{
		SchemaVersion: SchemaVersion,
		Instruction:   cloneInstruction(instruction),
		Binding:       cloneBinding(*resolution.Binding),
		Writes:        cloneParameters(writes),
		Preimage:      cloneParameters(preimage),
		EvidenceRefs:  uniqueSorted(instruction.EvidenceRefs),
		Operation:     OperationApply,
	}
	manifest.ID = "spal_" + fingerprint(struct {
		Instruction Instruction
		Binding     RuntimeBinding
		Writes      []PhysicalParameter
		Preimage    []PhysicalParameter
		Operation   string
	}{manifest.Instruction, manifest.Binding, manifest.Writes, manifest.Preimage, manifest.Operation})[:24]
	return manifest, nil
}

// NewRollbackManifest makes a separately confirmable rollback action. It is
// safe only when the current physical values still exactly match the original
// postimage; any manual or concurrent edit fails closed before a Proposal is
// produced.
func NewRollbackManifest(original ExecutionManifest, current []PhysicalParameter) (ExecutionManifest, error) {
	if err := original.Validate(); err != nil {
		return ExecutionManifest{}, fmt.Errorf("original SPAL manifest: %w", err)
	}
	if original.operation() != OperationApply {
		return ExecutionManifest{}, fmt.Errorf("only an applied SPAL manifest may be rolled back")
	}
	if err := samePhysicalParameters(original.Writes, current); err != nil {
		return ExecutionManifest{}, fmt.Errorf("rollback is unsafe because current parameters no longer equal the original postimage: %w", err)
	}
	if err := validatePreimage(original.Preimage, current); err != nil {
		return ExecutionManifest{}, err
	}
	// A rollback moves from the original postimage back to its preimage.  Keep
	// the same frozen measurement scope, but reverse the expected signal
	// direction so a same-tap verification never evaluates a valid restoration
	// against the direction of the original apply action.
	rollbackInstruction := cloneInstruction(original.Instruction)
	if rollbackInstruction.ExpectedSignalChange.IsRequested() {
		switch strings.ToLower(strings.TrimSpace(rollbackInstruction.ExpectedSignalChange.Direction)) {
		case "decrease":
			rollbackInstruction.ExpectedSignalChange.Direction = "increase"
		case "increase":
			rollbackInstruction.ExpectedSignalChange.Direction = "decrease"
		}
	}
	manifest := ExecutionManifest{
		SchemaVersion: SchemaVersion,
		Instruction:   rollbackInstruction,
		Binding:       cloneBinding(original.Binding),
		Writes:        cloneParameters(original.Preimage),
		Preimage:      cloneParameters(current),
		EvidenceRefs:  uniqueSorted(append(append([]string(nil), original.EvidenceRefs...), "spal.rollback_of:"+original.ID)),
		Operation:     OperationRollback,
		RollbackOf:    original.ID,
	}
	manifest.ID = "spal_rollback_" + fingerprint(struct {
		OriginalID string
		Binding    RuntimeBinding
		Writes     []PhysicalParameter
		Preimage   []PhysicalParameter
	}{original.ID, manifest.Binding, manifest.Writes, manifest.Preimage})[:24]
	return manifest, nil
}

func (m ExecutionManifest) Validate() error {
	if m.SchemaVersion != SchemaVersion || strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("SPAL manifest schema version and id are required")
	}
	if err := m.Instruction.Validate(); err != nil {
		return err
	}
	if err := m.Binding.Valid(); err != nil {
		return err
	}
	if m.Instruction.SchemaID != m.Binding.SchemaID || m.Instruction.TargetRef != m.Binding.Instance.TargetRef {
		return fmt.Errorf("manifest instruction does not match runtime binding")
	}
	if err := validatePhysicalParameters(m.Writes); err != nil {
		return err
	}
	if err := validatePreimage(m.Writes, m.Preimage); err != nil {
		return err
	}
	switch m.operation() {
	case OperationApply:
		return nil
	case OperationRollback:
		if strings.TrimSpace(m.RollbackOf) == "" {
			return fmt.Errorf("rollback SPAL manifest requires rollback_of")
		}
		return nil
	default:
		return fmt.Errorf("unknown SPAL manifest operation %q", m.Operation)
	}
}

func (m ExecutionManifest) PreimageFingerprint() string {
	return fingerprint(struct {
		BindingID string
		Preimage  []PhysicalParameter
	}{m.Binding.ID, sortedParameters(m.Preimage)})
}

func (m ExecutionManifest) ToActionSet(capabilityID string, cut orchestration.ProjectCut) (orchestration.ActionSet, error) {
	if err := m.Validate(); err != nil {
		return orchestration.ActionSet{}, err
	}
	if strings.TrimSpace(capabilityID) == "" {
		return orchestration.ActionSet{}, fmt.Errorf("capability id is required")
	}
	if strings.TrimSpace(cut.Hash) == "" {
		cut.Hash = cut.ComputeHash()
	}
	if !cut.IsExecutable() {
		return orchestration.ActionSet{}, fmt.Errorf("an executable ProjectCut is required")
	}
	encoded, err := manifestMap(m)
	if err != nil {
		return orchestration.ActionSet{}, err
	}
	idempotencyClass := "bound_semantic_parameter_patch_with_preimage"
	if m.operation() == OperationRollback {
		idempotencyClass = "bound_semantic_parameter_rollback_to_preimage"
	}
	set := orchestration.ActionSet{
		ID:             "actionset_" + m.ID,
		CapabilityID:   capabilityID,
		ProjectCutHash: cut.Hash,
		Actions: []orchestration.Action{{
			ID:                "action_" + m.ID,
			Command:           ActionCommand,
			TargetRef:         m.Instruction.TargetRef,
			BeforeFingerprint: "spal:" + m.Binding.ID + ":" + m.PreimageFingerprint(),
			Args:              map[string]any{"spal_manifest": encoded},
			Compensatable:     true,
			IdempotencyClass:  idempotencyClass,
		}},
	}
	set.Hash = set.ComputeHash()
	return set, nil
}

func (m ExecutionManifest) FreezeProposal(capabilityID, capabilityVersion string, cut orchestration.ProjectCut, revision int64) (orchestration.Proposal, orchestration.ActionSet, error) {
	if revision < 1 {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("proposal revision must be positive")
	}
	set, err := m.ToActionSet(capabilityID, cut)
	if err != nil {
		return orchestration.Proposal{}, orchestration.ActionSet{}, err
	}
	summary := "Apply " + m.Instruction.SchemaID + " through a verified SPAL Provider."
	if m.operation() == OperationRollback {
		summary = "Restore the frozen physical preimage for SPAL manifest " + m.RollbackOf + "."
	}
	proposal := orchestration.Proposal{
		ID:              "proposal_" + set.Hash[:16],
		Revision:        revision,
		CapabilityID:    capabilityID,
		CapabilityVer:   firstNonEmpty(capabilityVersion, "v0"),
		ProjectCutHash:  set.ProjectCutHash,
		CandidateID:     m.ID,
		ActionSetHash:   set.Hash,
		TargetScope:     []string{m.Instruction.TargetRef},
		Risk:            "bounded_reversible",
		VerificationRef: "spal.spectral.static_bell.verification.v0",
		Summary:         summary,
	}
	return proposal, set, nil
}

func ManifestFromAction(action orchestration.Action) (ExecutionManifest, error) {
	if action.Command != ActionCommand {
		return ExecutionManifest{}, fmt.Errorf("action %s is not a SPAL action", action.ID)
	}
	raw, ok := action.Args["spal_manifest"]
	if !ok {
		return ExecutionManifest{}, fmt.Errorf("SPAL action %s omits spal_manifest", action.ID)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return ExecutionManifest{}, fmt.Errorf("encode SPAL action manifest: %w", err)
	}
	var manifest ExecutionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ExecutionManifest{}, fmt.Errorf("decode SPAL action manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return ExecutionManifest{}, err
	}
	return manifest, nil
}

func manifestMap(manifest ExecutionManifest) (map[string]any, error) {
	data, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("encode SPAL manifest: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decode SPAL manifest map: %w", err)
	}
	return out, nil
}

func validatePhysicalParameters(values []PhysicalParameter) error {
	if len(values) == 0 {
		return fmt.Errorf("at least one physical parameter is required")
	}
	seen := map[string]bool{}
	for _, value := range values {
		if err := value.Valid(); err != nil {
			return err
		}
		if seen[value.ParameterID] {
			return fmt.Errorf("duplicate physical parameter %s", value.ParameterID)
		}
		seen[value.ParameterID] = true
	}
	return nil
}

func validatePreimage(writes, preimage []PhysicalParameter) error {
	if err := validatePhysicalParameters(preimage); err != nil {
		return fmt.Errorf("complete physical preimage is required: %w", err)
	}
	known := map[string]bool{}
	for _, value := range preimage {
		known[value.ParameterID] = true
	}
	for _, write := range writes {
		if !known[write.ParameterID] {
			return fmt.Errorf("preimage omits physical parameter %s", write.ParameterID)
		}
	}
	return nil
}

func samePhysicalParameters(expected, actual []PhysicalParameter) error {
	indexed := map[string]PhysicalParameter{}
	for _, value := range actual {
		indexed[value.ParameterID] = value
	}
	for _, want := range expected {
		got, ok := indexed[want.ParameterID]
		if !ok {
			return fmt.Errorf("parameter %s missing", want.ParameterID)
		}
		if math.Abs(got.Value-want.Value) > 0.0001 {
			return fmt.Errorf("parameter %s = %.6f, want %.6f", want.ParameterID, got.Value, want.Value)
		}
	}
	return nil
}

func (m ExecutionManifest) operation() string {
	if operation := strings.TrimSpace(m.Operation); operation != "" {
		return operation
	}
	// Manifests frozen before the rollback field existed remain ordinary apply
	// actions and must stay executable/reconcilable.
	return OperationApply
}

func cloneInstruction(in Instruction) Instruction {
	out := in
	out.Parameters = make(map[string]float64, len(in.Parameters))
	for key, value := range in.Parameters {
		out.Parameters[key] = value
	}
	out.StringParameters = make(map[string]string, len(in.StringParameters))
	for key, value := range in.StringParameters {
		out.StringParameters[key] = value
	}
	out.EvidenceRefs = append([]string(nil), in.EvidenceRefs...)
	out.SignalProbeScope = cloneSignalProbeScope(in.SignalProbeScope)
	return out
}

func cloneSignalProbeScope(in SignalProbeScope) SignalProbeScope {
	out := in
	if in.StartSeconds != nil {
		value := *in.StartSeconds
		out.StartSeconds = &value
	}
	if in.EndSeconds != nil {
		value := *in.EndSeconds
		out.EndSeconds = &value
	}
	if in.TailSeconds != nil {
		value := *in.TailSeconds
		out.TailSeconds = &value
	}
	return out
}

func cloneBinding(in RuntimeBinding) RuntimeBinding {
	out := in
	out.Provider.SupportedSchemas = append([]string(nil), in.Provider.SupportedSchemas...)
	out.Provider.ConformanceEvidence = append([]string(nil), in.Provider.ConformanceEvidence...)
	out.Instance.Metadata = cloneStringMap(in.Instance.Metadata)
	out.ParameterBindings = make(map[string]ParameterBinding, len(in.ParameterBindings))
	for key, value := range in.ParameterBindings {
		out.ParameterBindings[key] = value
	}
	out.Invariants = cloneStringMap(in.Invariants)
	return out
}

func cloneParameters(in []PhysicalParameter) []PhysicalParameter {
	return append([]PhysicalParameter(nil), in...)
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func sortedParameters(values []PhysicalParameter) []PhysicalParameter {
	out := cloneParameters(values)
	sort.Slice(out, func(i, j int) bool { return out[i].ParameterID < out[j].ParameterID })
	return out
}

func uniqueSorted(values []string) []string {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

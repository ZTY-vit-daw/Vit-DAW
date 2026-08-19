// Package semanticorchestrator defines the deterministic progressive-
// disclosure state machine shared by abstract processor workflows. It does
// not infer a family, choose a plugin, or map parameters.
package semanticorchestrator

import (
	"fmt"
	"strings"

	"vit-daw-agent/internal/processorattestation"
	"vit-daw-agent/internal/processorintent"
	"vit-daw-agent/internal/processorregistry"
)

const SchemaVersion = "semantic_progressive_disclosure.v1"

type Stage string

const (
	StageIntent         Stage = "intent"
	StageObservation    Stage = "observation"
	StageControlBrief   Stage = "control_brief"
	StagePhysicalTarget Stage = "physical_target"
	StageControlBinding Stage = "control_binding"
	StageConfirmation   Stage = "confirmation"
	StageApplied        Stage = "applied"
	StageReceipt        Stage = "receipt"
	StageRejected       Stage = "rejected"
)

type State struct {
	SchemaVersion    string                          `json:"schema_version"`
	Stage            Stage                           `json:"stage"`
	Intent           processorintent.Intent          `json:"intent"`
	AdapterFamily    string                          `json:"adapter_family,omitempty"`
	PCAFamily        string                          `json:"pca_family,omitempty"`
	RequiredCoverage []processorattestation.Coverage `json:"required_coverage,omitempty"`
	ModelViewIDs     []string                        `json:"model_view_ids,omitempty"`
	ExecutedViewIDs  []string                        `json:"executed_view_ids,omitempty"`
	Confirmation     bool                            `json:"confirmation_required"`
	BoundControlRefs []string                        `json:"bound_control_refs,omitempty"`
	Receipt          map[string]any                  `json:"receipt,omitempty"`
	RejectionCode    string                          `json:"rejection_code,omitempty"`
	RejectionReason  string                          `json:"rejection_reason,omitempty"`
}

type Orchestrator struct {
	registry *processorregistry.Registry
	state    State
}

func New(registry *processorregistry.Registry) (*Orchestrator, error) {
	if registry == nil {
		return nil, fmt.Errorf("semantic orchestrator requires a processor registry")
	}
	return &Orchestrator{registry: registry, state: State{SchemaVersion: SchemaVersion, Stage: StageIntent}}, nil
}

// Resume rebuilds an orchestrator from its persisted state. The persisted
// state is treated as an observation, not as authority: every deterministic
// stage is replayed so tampering with family, coverage, view ids, control
// refs, or receipt ordering fails closed.
func Resume(registry *processorregistry.Registry, persisted State) (*Orchestrator, error) {
	o, err := New(registry)
	if err != nil {
		return nil, err
	}
	if persisted.SchemaVersion != "" && persisted.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("semantic progressive disclosure schema mismatch")
	}
	if persisted.Stage == "" || persisted.Stage == StageIntent {
		return o, nil
	}
	if persisted.Stage == StageRejected {
		if persisted.RejectionCode == "" || persisted.RejectionReason == "" {
			return nil, fmt.Errorf("rejected progressive disclosure state is incomplete")
		}
		o.state = persisted
		return o, nil
	}
	if err := o.AcceptIntent(persisted.Intent); err != nil {
		return nil, err
	}
	if len(persisted.ModelViewIDs) == 0 || len(persisted.ExecutedViewIDs) == 0 {
		return nil, fmt.Errorf("persisted progressive disclosure state is missing observation views")
	}
	if err := o.RecordObservation(persisted.ModelViewIDs, persisted.ExecutedViewIDs); err != nil {
		return nil, err
	}
	if persisted.Stage == StagePhysicalTarget || persisted.Stage == StageControlBinding ||
		persisted.Stage == StageConfirmation || persisted.Stage == StageApplied || persisted.Stage == StageReceipt {
		if err := o.SetControlBrief(); err != nil {
			return nil, err
		}
	}
	if persisted.Stage == StageControlBinding || persisted.Stage == StageConfirmation ||
		persisted.Stage == StageApplied || persisted.Stage == StageReceipt {
		if err := o.SetPhysicalTarget(); err != nil {
			return nil, err
		}
	}
	if persisted.Stage == StageControlBinding || persisted.Stage == StageConfirmation ||
		persisted.Stage == StageApplied || persisted.Stage == StageReceipt {
		if err := o.BindControlRefs(persisted.BoundControlRefs); err != nil {
			return nil, err
		}
	}
	if persisted.Stage == StageApplied || persisted.Stage == StageReceipt {
		if err := o.Confirm(); err != nil {
			return nil, err
		}
	}
	if persisted.Stage == StageReceipt {
		if err := o.RecordReceipt(persisted.Receipt); err != nil {
			return nil, err
		}
	}
	if o.state.Stage != persisted.Stage {
		return nil, fmt.Errorf("persisted progressive disclosure stage cannot be replayed: %s", persisted.Stage)
	}
	return o, nil
}

func (o *Orchestrator) State() State {
	if o == nil {
		return State{SchemaVersion: SchemaVersion, Stage: StageRejected, RejectionCode: "nil_orchestrator"}
	}
	out := o.state
	out.Intent.EvidenceRefs = append([]string(nil), o.state.Intent.EvidenceRefs...)
	out.Intent.RequiredCoverage = append([]string(nil), o.state.Intent.RequiredCoverage...)
	out.RequiredCoverage = append([]processorattestation.Coverage(nil), o.state.RequiredCoverage...)
	out.ModelViewIDs = append([]string(nil), o.state.ModelViewIDs...)
	out.ExecutedViewIDs = append([]string(nil), o.state.ExecutedViewIDs...)
	out.BoundControlRefs = append([]string(nil), o.state.BoundControlRefs...)
	if o.state.Receipt != nil {
		out.Receipt = cloneMap(o.state.Receipt)
	}
	return out
}

// AcceptIntent is the only entry that binds a family. The caller must provide
// the model-validated intent; no text or keyword is accepted here.
func (o *Orchestrator) AcceptIntent(intent processorintent.Intent) error {
	if o == nil {
		return fmt.Errorf("nil semantic orchestrator")
	}
	if err := intent.Validate(); err != nil {
		return o.reject("invalid_intent", err.Error())
	}
	if intent.Status != processorintent.StatusResolved {
		return o.reject("unresolved_intent", "resolved intent is required for an executable workflow")
	}
	definition, ok := o.registry.Resolve(intent.Family)
	if !ok {
		return o.reject("family_not_registered", intent.Family)
	}
	if definition.InspectOnly || intent.ControlMode == processorintent.ControlModeInspectOnly {
		return o.reject("inspect_only_family", intent.Family)
	}
	coverage, err := o.registry.PCARequiredCoverage(intent.Family, intent.RequiredCoverage)
	if err != nil {
		return o.reject("coverage_not_proven", err.Error())
	}
	o.state.Intent = intent
	o.state.AdapterFamily = definition.Family
	o.state.PCAFamily = definition.PCAFamily
	o.state.RequiredCoverage = coverage
	o.state.Stage = StageObservation
	return nil
}

// SelectAdapter resolves only the already-selected family. Registry lookup
// never receives user text and therefore cannot become a family router.
func (o *Orchestrator) SelectAdapter() (processorregistry.Definition, error) {
	if o == nil {
		return processorregistry.Definition{}, fmt.Errorf("nil semantic orchestrator")
	}
	if o.state.Stage == StageRejected || o.state.AdapterFamily == "" {
		return processorregistry.Definition{}, fmt.Errorf("family has not been selected by the model")
	}
	definition, ok := o.registry.Resolve(o.state.AdapterFamily)
	if !ok {
		return processorregistry.Definition{}, o.reject("family_not_registered", o.state.AdapterFamily)
	}
	return definition, nil
}

func (o *Orchestrator) RecordObservation(modelViewIDs, executedViewIDs []string) error {
	if o == nil {
		return fmt.Errorf("nil semantic orchestrator")
	}
	if err := exactViewSet(modelViewIDs, executedViewIDs); err != nil {
		return o.reject("observation_view_mismatch", err.Error())
	}
	if o.state.Stage != StageObservation {
		return fmt.Errorf("observation is not the current progressive-disclosure stage")
	}
	o.state.ModelViewIDs = append([]string(nil), modelViewIDs...)
	o.state.ExecutedViewIDs = append([]string(nil), executedViewIDs...)
	o.state.Stage = StageControlBrief
	return nil
}

func (o *Orchestrator) SetControlBrief() error {
	if o == nil || o.state.Stage != StageControlBrief {
		return fmt.Errorf("control brief is not the current progressive-disclosure stage")
	}
	o.state.Stage = StagePhysicalTarget
	return nil
}

func (o *Orchestrator) SetPhysicalTarget() error {
	if o == nil || o.state.Stage != StagePhysicalTarget {
		return fmt.Errorf("physical target is not the current progressive-disclosure stage")
	}
	o.state.Stage = StageControlBinding
	return nil
}

func (o *Orchestrator) BindControlRefs(refs []string) error {
	if o == nil || o.state.Stage != StageControlBinding {
		return fmt.Errorf("control binding is not the current progressive-disclosure stage")
	}
	if len(refs) == 0 {
		return o.reject("control_refs_missing", "at least one deterministic control_ref is required")
	}
	seen := map[string]bool{}
	for _, raw := range refs {
		ref := strings.TrimSpace(raw)
		if ref == "" || seen[ref] {
			return o.reject("control_refs_invalid", "control_ref values must be non-empty and unique")
		}
		seen[ref] = true
	}
	o.state.BoundControlRefs = append([]string(nil), refs...)
	o.state.Confirmation = true
	o.state.Stage = StageConfirmation
	return nil
}

func (o *Orchestrator) Confirm() error {
	if o == nil || o.state.Stage != StageConfirmation || !o.state.Confirmation {
		return fmt.Errorf("confirmation is not pending")
	}
	o.state.Stage = StageApplied
	return nil
}

func (o *Orchestrator) RecordReceipt(receipt map[string]any) error {
	if o == nil || o.state.Stage != StageApplied {
		return fmt.Errorf("receipt cannot be recorded before typed apply")
	}
	if len(receipt) == 0 {
		return o.reject("receipt_missing", "typed apply receipt is required")
	}
	o.state.Receipt = cloneMap(receipt)
	o.state.Stage = StageReceipt
	return nil
}

func (o *Orchestrator) Reject(code, reason string) error {
	if o == nil {
		return fmt.Errorf("nil semantic orchestrator")
	}
	return o.reject(code, reason)
}

func (o *Orchestrator) reject(code, reason string) error {
	o.state.Stage = StageRejected
	o.state.RejectionCode = strings.TrimSpace(code)
	o.state.RejectionReason = strings.TrimSpace(reason)
	return fmt.Errorf("%s: %s", o.state.RejectionCode, o.state.RejectionReason)
}

func exactViewSet(model, executed []string) error {
	if len(model) == 0 || len(executed) == 0 {
		return fmt.Errorf("observation view set must not be empty")
	}
	left, right := map[string]bool{}, map[string]bool{}
	for _, raw := range model {
		value := strings.TrimSpace(raw)
		if value == "" || left[value] {
			return fmt.Errorf("model observation views must be non-empty and unique")
		}
		left[value] = true
	}
	for _, raw := range executed {
		value := strings.TrimSpace(raw)
		if value == "" || right[value] {
			return fmt.Errorf("executed observation views must be non-empty and unique")
		}
		right[value] = true
	}
	if len(left) != len(right) {
		return fmt.Errorf("model and executed observation view sets differ")
	}
	for value := range left {
		if !right[value] {
			return fmt.Errorf("model and executed observation view sets differ")
		}
	}
	return nil
}

func cloneMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	out := make(map[string]any, len(value))
	for key, child := range value {
		out[key] = child
	}
	return out
}

package spallab

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"vit-daw-agent/internal/orchestration"
)

const (
	ReferenceEQProviderRegistrationCapabilityID = "spal.reference_eq_provider_registration.v0"
	ReferenceEQProviderRegistrationCommand      = "spal.reference_eq_provider.register"
)

// ProductReadyFor verifies the extra scope requirement for a record that is
// about to leave the lab fixture and be used by the Reference EQ product path.
// Legacy unscoped records deliberately remain readable by developer tooling,
// but cannot be selected by Ask Vit.
func (r ProviderRecord) ProductReadyFor(projectUUID string) error {
	if err := r.Valid(); err != nil {
		return err
	}
	projectUUID = strings.TrimSpace(projectUUID)
	if projectUUID == "" {
		return fmt.Errorf("current project uuid is required for a product Provider registration")
	}
	if strings.TrimSpace(r.ProjectUUID) == "" {
		return fmt.Errorf("Provider record is not scoped to a project")
	}
	if !strings.EqualFold(strings.TrimSpace(r.ProjectUUID), projectUUID) {
		return fmt.Errorf("Provider record belongs to a different project")
	}
	if !strings.EqualFold(strings.TrimSpace(r.Instance.Metadata["project_uuid"]), projectUUID) {
		return fmt.Errorf("Provider instance project scope does not match the current project")
	}
	return nil
}

// ProviderRegistrationActionSet freezes the exact conformance record that the
// user will approve.  Execution is only allowed to persist this record after a
// fresh VSP parameter readback revalidates the same instance and invariants.
func ProviderRegistrationActionSet(record ProviderRecord, cut orchestration.ProjectCut) (orchestration.ActionSet, error) {
	if err := record.ProductReadyFor(cut.ProjectUUID); err != nil {
		return orchestration.ActionSet{}, err
	}
	if cut.Hash == "" {
		cut.Hash = cut.ComputeHash()
	}
	if !cut.IsExecutable() {
		return orchestration.ActionSet{}, fmt.Errorf("Provider registration requires an executable Project Cut")
	}
	action := orchestration.Action{
		ID:                "spal_provider_registration:" + record.ID,
		Command:           ReferenceEQProviderRegistrationCommand,
		TargetRef:         record.Instance.TargetRef,
		BeforeFingerprint: providerRegistrationFingerprint(record),
		Args: map[string]any{
			"provider_record":    record,
			"provider_record_id": record.ID,
			"project_uuid":       record.ProjectUUID,
		},
		Compensatable:    false,
		IdempotencyClass: "conformance_record_idempotent_upsert",
	}
	set := orchestration.ActionSet{
		ID:             "actionset_spal_provider_registration_" + record.ID,
		CapabilityID:   ReferenceEQProviderRegistrationCapabilityID,
		ProjectCutHash: cut.Hash,
		Actions:        []orchestration.Action{action},
	}
	set.Hash = set.ComputeHash()
	return set, nil
}

func FreezeProviderRegistrationProposal(record ProviderRecord, cut orchestration.ProjectCut, revision int64) (orchestration.Proposal, orchestration.ActionSet, error) {
	if revision < 1 {
		return orchestration.Proposal{}, orchestration.ActionSet{}, fmt.Errorf("Provider registration proposal revision must be positive")
	}
	set, err := ProviderRegistrationActionSet(record, cut)
	if err != nil {
		return orchestration.Proposal{}, orchestration.ActionSet{}, err
	}
	proposal := orchestration.Proposal{
		ID:              "proposal_" + set.Hash[:16],
		Revision:        revision,
		CapabilityID:    ReferenceEQProviderRegistrationCapabilityID,
		CapabilityVer:   "v0",
		ProjectCutHash:  cut.Hash,
		CandidateID:     record.ID,
		ActionSetHash:   set.Hash,
		TargetScope:     []string{record.Instance.TargetRef},
		Risk:            "bounded_registry_write",
		VerificationRef: "spal.reference_eq_provider_registration.verification.v0",
		Summary:         "Register one conformed Reference EQ Provider instance for the current project.",
		CreatedAt:       time.Now().UTC(),
	}
	return proposal, set, nil
}

func ProviderRecordFromRegistrationAction(action orchestration.Action) (ProviderRecord, error) {
	if action.Command != ReferenceEQProviderRegistrationCommand {
		return ProviderRecord{}, fmt.Errorf("action %s is not a Reference EQ Provider registration", action.ID)
	}
	raw, ok := action.Args["provider_record"]
	if !ok || raw == nil {
		return ProviderRecord{}, fmt.Errorf("Provider registration action omits the frozen Provider record")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return ProviderRecord{}, fmt.Errorf("encode frozen Provider record: %w", err)
	}
	var record ProviderRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return ProviderRecord{}, fmt.Errorf("decode frozen Provider record: %w", err)
	}
	if id := strings.TrimSpace(fmt.Sprint(action.Args["provider_record_id"])); id != "" && id != record.ID {
		return ProviderRecord{}, fmt.Errorf("Provider registration record id does not match frozen record")
	}
	return record, nil
}

func providerRegistrationFingerprint(record ProviderRecord) string {
	return shortFingerprint(
		record.ProjectUUID,
		record.ID,
		record.Instance.TargetRef,
		record.Instance.TrackID,
		record.Instance.PluginID,
		record.CurrentParameterSignature,
		record.PluginSkillSignature,
		record.StaticBellInvariant.ParameterID,
		fmt.Sprintf("%.6f", record.StaticBellInvariant.Value),
		fmt.Sprintf("%.6f", record.StaticBellInvariant.NormalizedValue),
	)
}

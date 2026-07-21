package spallab

import (
	"context"
	"fmt"
	"strings"

	"vit-daw-agent/internal/orchestration"
)

// ProviderRegistrationPort persists only the conformance record frozen in an
// approved Proposal. It never discovers, loads, or substitutes a plug-in:
// discovery and live VSP readback have already happened in the registration
// planner, and Preflight repeats the readback validator immediately before the
// local registry write.
type ProviderRegistrationPort struct {
	Store          *ProviderStore
	ValidateRecord func(context.Context, ProviderRecord) error
}

func (p ProviderRegistrationPort) Preflight(ctx context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
	record, err := providerRegistrationRecordFromSet(actionSet, cut)
	if err != nil {
		return err
	}
	if p.Store == nil {
		return fmt.Errorf("SPAL Provider registration store is unavailable")
	}
	if p.ValidateRecord == nil {
		return fmt.Errorf("SPAL Provider registration requires a current conformance validator")
	}
	if err := p.ValidateRecord(ctx, record); err != nil {
		return fmt.Errorf("validate current Provider registration record: %w", err)
	}
	return nil
}

func (p ProviderRegistrationPort) Apply(ctx context.Context, action orchestration.Action, _ string) (orchestration.ActionReceipt, error) {
	record, err := ProviderRecordFromRegistrationAction(action)
	if err != nil {
		return registrationFailureReceipt(action, err), err
	}
	if p.Store == nil {
		err := fmt.Errorf("SPAL Provider registration store is unavailable")
		return registrationFailureReceipt(action, err), err
	}
	if p.ValidateRecord == nil {
		err := fmt.Errorf("SPAL Provider registration requires a current conformance validator")
		return registrationFailureReceipt(action, err), err
	}
	if err := p.ValidateRecord(ctx, record); err != nil {
		err = fmt.Errorf("validate current Provider registration record: %w", err)
		return registrationFailureReceipt(action, err), err
	}
	persisted, err := p.Store.Upsert(record)
	if err != nil {
		err = fmt.Errorf("persist verified SPAL Provider record: %w", err)
		return registrationFailureReceipt(action, err), err
	}
	return registrationAppliedReceipt(action, persisted, false), nil
}

// Reconcile never repeats an uncertain registry write. If the exact frozen
// record is present, the durable registration is treated as applied; if it is
// absent, recovery reports an explicit missing receipt rather than silently
// upserting it again.
func (p ProviderRegistrationPort) Reconcile(_ context.Context, action orchestration.Action, _ string, cut orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
	record, err := ProviderRecordFromRegistrationAction(action)
	if err != nil {
		return registrationFailureReceipt(action, err), err
	}
	if err := record.ProductReadyFor(cut.ProjectUUID); err != nil {
		return registrationFailureReceipt(action, err), err
	}
	if p.Store == nil {
		err := fmt.Errorf("SPAL Provider registration store is unavailable")
		return registrationFailureReceipt(action, err), err
	}
	records, err := p.Store.List()
	if err != nil {
		err = fmt.Errorf("read SPAL Provider registration store: %w", err)
		return registrationFailureReceipt(action, err), err
	}
	for _, existing := range records {
		if existing.ID != record.ID {
			continue
		}
		if err := existing.ProductReadyFor(cut.ProjectUUID); err != nil {
			return registrationFailureReceipt(action, err), err
		}
		if existing.CurrentParameterSignature != record.CurrentParameterSignature || existing.PluginSkillSignature != record.PluginSkillSignature || existing.Instance.PluginID != record.Instance.PluginID || existing.Instance.TrackID != record.Instance.TrackID {
			err := fmt.Errorf("persisted Provider record does not match the frozen registration")
			return registrationFailureReceipt(action, err), err
		}
		return registrationAppliedReceipt(action, existing, true), nil
	}
	err = fmt.Errorf("frozen Provider registration record is not present in the store")
	return registrationFailureReceipt(action, err), err
}

func providerRegistrationRecordFromSet(actionSet orchestration.ActionSet, cut orchestration.ProjectCut) (ProviderRecord, error) {
	if actionSet.CapabilityID != ReferenceEQProviderRegistrationCapabilityID {
		return ProviderRecord{}, fmt.Errorf("unexpected Provider registration capability %q", actionSet.CapabilityID)
	}
	if len(actionSet.Actions) != 1 {
		return ProviderRecord{}, fmt.Errorf("SPAL Provider registration requires exactly one action")
	}
	record, err := ProviderRecordFromRegistrationAction(actionSet.Actions[0])
	if err != nil {
		return ProviderRecord{}, err
	}
	if err := record.ProductReadyFor(cut.ProjectUUID); err != nil {
		return ProviderRecord{}, err
	}
	return record, nil
}

func registrationAppliedReceipt(action orchestration.Action, record ProviderRecord, reconciled bool) orchestration.ActionReceipt {
	evidence := append([]string{"spal.provider_record:" + record.ID}, record.ConformanceEvidence...)
	return orchestration.ActionReceipt{
		ActionID:        action.ID,
		Status:          "applied",
		EffectivelyOnce: true,
		EvidenceRefs:    appendUniqueRegistrationRefs(nil, evidence...),
		Details: map[string]any{
			"structural_readback":             "pass",
			"provider_record_id":              record.ID,
			"provider_project_uuid":           record.ProjectUUID,
			"provider_instance_id":            record.Instance.ID,
			"provider_track_id":               record.Instance.TrackID,
			"provider_plugin_id":              record.Instance.PluginID,
			"current_parameter_signature":     record.CurrentParameterSignature,
			"plugin_skill_signature":          record.PluginSkillSignature,
			"static_bell_invariant_parameter": record.StaticBellInvariant.ParameterID,
			"reconciled_from_store":           reconciled,
		},
	}
}

func registrationFailureReceipt(action orchestration.Action, err error) orchestration.ActionReceipt {
	message := "Provider registration failed"
	if err != nil {
		message = err.Error()
	}
	return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed", Error: message, EffectivelyOnce: false}
}

func appendUniqueRegistrationRefs(base []string, refs ...string) []string {
	seen := make(map[string]bool, len(base)+len(refs))
	for _, ref := range base {
		if ref = strings.TrimSpace(ref); ref != "" {
			seen[ref] = true
		}
	}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref != "" && !seen[ref] {
			seen[ref] = true
			base = append(base, ref)
		}
	}
	return base
}

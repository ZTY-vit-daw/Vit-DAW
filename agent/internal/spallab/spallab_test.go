package spallab

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"vit-daw-agent/internal/executionruntime"
	"vit-daw-agent/internal/executionverifiers"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/spal"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

func TestConformTDRNovaRequiresEvidenceAndCurrentMappings(t *testing.T) {
	request := labConformanceRequest()
	record, err := ConformTDRNova(request)
	if err != nil {
		t.Fatal(err)
	}
	if !record.LabOnly || record.Instance.Status != spal.InstanceVerified || record.Instance.Metadata["band_slot"] != "band1" {
		t.Fatalf("unexpected lab record: %#v", record)
	}

	request.StaticBellEvidence = nil
	if _, err := ConformTDRNova(request); err == nil {
		t.Fatal("missing static Bell evidence was accepted")
	}
	request = labConformanceRequest()
	request.ObservedParamIDs = []string{"1", "6", "4", "2"}
	if _, err := ConformTDRNova(request); err == nil {
		t.Fatal("stale/missing current parameter mapping was accepted")
	}
}

func TestProviderStorePersistsOnlyConformedLabInstances(t *testing.T) {
	store, err := NewProviderStore(filepath.Join(t.TempDir(), "providers.json"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := ConformTDRNova(labConformanceRequest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Upsert(record); err != nil {
		t.Fatal(err)
	}
	instances, err := store.Instances()
	if err != nil || len(instances) != 1 || instances[0].TargetRef != "track:bass" {
		t.Fatalf("instances=%#v err=%v", instances, err)
	}
	registry, err := store.Registry()
	if err != nil || len(registry.Providers()) != 1 {
		t.Fatalf("registry=%#v err=%v", registry, err)
	}
}

func TestLiveProviderValidationRejectsChangedStaticBellState(t *testing.T) {
	record, err := ConformTDRNova(labConformanceRequest())
	if err != nil {
		t.Fatal(err)
	}
	reply := labPluginReply(labTDRNovaSkill())
	if err := ValidateProviderRecordCurrent(record, reply); err != nil {
		t.Fatal(err)
	}
	for _, row := range reply["parameters"].([]map[string]any) {
		if row["id"] == "5" {
			row["value"] = 3.0
		}
	}
	if err := ValidateProviderRecordCurrent(record, reply); err == nil {
		t.Fatal("changed filter type was accepted as a static Bell")
	}
}

func TestConformFromCurrentPluginReplyCapturesBellInvariant(t *testing.T) {
	record, err := ConformTDRNovaFromPluginReply(TDRNovaConformanceRequest{
		TargetRef: "track:bass", TrackID: "bass", PluginID: "nova-1", BandSlot: "band1",
		StaticBellConfirmed: true, StaticBellEvidence: []string{"lab:operator-confirmed-static-bell"},
	}, labPluginReply(labTDRNovaSkill()))
	if err != nil {
		t.Fatal(err)
	}
	if record.StaticBellInvariant.ParameterID != "5" || record.StaticBellInvariant.ValueText != "Bell" {
		t.Fatalf("static Bell invariant was not captured: %#v", record.StaticBellInvariant)
	}
}

func TestConformFromCurrentPluginReplyAcceptsPluginLearningShortBandID(t *testing.T) {
	skill := labTDRNovaSkill()
	skill.Components[0].ID = "b1"
	record, err := ConformTDRNovaFromPluginReply(TDRNovaConformanceRequest{
		TargetRef: "track:bass", TrackID: "bass", PluginID: "nova-1", BandSlot: "band1",
		StaticBellConfirmed: true, StaticBellEvidence: []string{"lab:operator-confirmed-static-bell"},
	}, labPluginReply(skill))
	if err != nil {
		t.Fatal(err)
	}
	if record.Instance.Metadata["band_slot"] != "band1" {
		t.Fatalf("record = %+v", record)
	}
}

func TestLabSeparatesPlanApprovalAndExecution(t *testing.T) {
	providerStore, err := NewProviderStore(filepath.Join(t.TempDir(), "providers.json"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := ConformTDRNova(labConformanceRequest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := providerStore.Upsert(record); err != nil {
		t.Fatal(err)
	}
	sessions, err := orchestration.NewFileStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	lab, err := New(providerStore, sessions)
	if err != nil {
		t.Fatal(err)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "project-1", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	maxGain := 3.0
	planned, err := lab.Plan(context.Background(), PlanRequest{
		SessionID: "lab-session", Goal: "lab check static Bell", ProjectCut: cut,
		Instruction: spal.Instruction{
			SchemaID: spal.StaticBellControlID, TargetRef: "track:bass",
			Parameters:           map[string]float64{"center_frequency_hz": 92, "gain_db": -2.5, "q": 1.2},
			SafetyBounds:         spal.SafetyBounds{MaxAbsoluteGainDB: &maxGain},
			EvidenceRefs:         []string{"lab:manual-static-bell"},
			ExpectedSignalChange: spal.SignalExpectation{BandLowHz: 80, BandHighHz: 110, Direction: "decrease"},
		},
		PreimageReader: labPreimageReader{}, ValidateRecord: allowLabRecord,
	})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Proposal == nil || planned.ActionSet == nil || planned.Session == nil || planned.Session.Status != orchestration.StatusWaiting {
		t.Fatalf("planning did not produce a frozen waiting proposal: %#v", planned)
	}
	if _, err := lab.Execute(context.Background(), "lab-session", &labMutationPort{}, executionverifiers.SPAL{}, nil); err == nil {
		t.Fatal("execution before exact approval was accepted")
	}
	authorized, err := lab.Approve("lab-session", planned.Proposal.ID, "manual-lab-confirmation")
	if err != nil || authorized.Status != orchestration.StatusAuthorized {
		t.Fatalf("approval=%#v err=%v", authorized, err)
	}
	journal, err := NewJournal(filepath.Join(t.TempDir(), "journal.json"))
	if err != nil {
		t.Fatal(err)
	}
	port := &labMutationPort{}
	completed, err := lab.Execute(context.Background(), "lab-session", port, executionverifiers.SPAL{}, journal)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != orchestration.StatusNeedsReview || completed.Execution == nil || len(completed.Execution.Receipts) != 1 || !port.applied {
		t.Fatalf("unexpected governed lab execution: %#v", completed)
	}
	if entries, err := journal.Entries(); err != nil || len(entries) != 1 || entries[0].ActionSetHash != planned.ActionSet.Hash {
		t.Fatalf("journal entries=%#v err=%v", entries, err)
	}
}

func TestLabBlocksWithoutVerifiedProvider(t *testing.T) {
	providerStore, err := NewProviderStore(filepath.Join(t.TempDir(), "providers.json"))
	if err != nil {
		t.Fatal(err)
	}
	sessions, err := orchestration.NewFileStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	lab, err := New(providerStore, sessions)
	if err != nil {
		t.Fatal(err)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "project-1", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	planned, err := lab.Plan(context.Background(), PlanRequest{
		SessionID: "blocked", Goal: "lab check", ProjectCut: cut,
		Instruction:    spal.Instruction{SchemaID: spal.StaticBellControlID, TargetRef: "track:bass", Parameters: map[string]float64{"center_frequency_hz": 92, "gain_db": -2, "q": 1}},
		PreimageReader: labPreimageReader{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Outcome.Kind != orchestration.OutcomeBlocked || planned.Proposal != nil || planned.Session != nil {
		t.Fatalf("missing Provider did not remain blocked: %#v", planned)
	}
}

func TestLabRequiresLiveRecordValidationBeforePlanning(t *testing.T) {
	providerStore, err := NewProviderStore(filepath.Join(t.TempDir(), "providers.json"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := ConformTDRNova(labConformanceRequest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := providerStore.Upsert(record); err != nil {
		t.Fatal(err)
	}
	sessions, err := orchestration.NewFileStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	lab, err := New(providerStore, sessions)
	if err != nil {
		t.Fatal(err)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "project-1", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	_, err = lab.Plan(context.Background(), PlanRequest{
		SessionID: "no-validator", Goal: "lab check", ProjectCut: cut,
		Instruction:    spal.Instruction{SchemaID: spal.StaticBellControlID, TargetRef: "track:bass", Parameters: map[string]float64{"center_frequency_hz": 92, "gain_db": -2, "q": 1}},
		PreimageReader: labPreimageReader{},
	})
	if err == nil {
		t.Fatal("a persisted Provider was allowed without a current live validator")
	}
}

func TestLabRollbackIsASeparateProposalAndFailsClosedOnManualChange(t *testing.T) {
	providerStore, err := NewProviderStore(filepath.Join(t.TempDir(), "providers.json"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := ConformTDRNova(labConformanceRequest())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := providerStore.Upsert(record); err != nil {
		t.Fatal(err)
	}
	sessions, err := orchestration.NewFileStore(filepath.Join(t.TempDir(), "sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	lab, err := New(providerStore, sessions)
	if err != nil {
		t.Fatal(err)
	}
	cut := orchestration.ProjectCut{ProjectUUID: "project-1", ProjectEpoch: "epoch-1", BaseProjectRevision: "7", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	planned, err := lab.Plan(context.Background(), PlanRequest{
		SessionID: "original", Goal: "apply", ProjectCut: cut,
		Instruction:    spal.Instruction{SchemaID: spal.StaticBellControlID, TargetRef: "track:bass", Parameters: map[string]float64{"center_frequency_hz": 92, "gain_db": -2.5, "q": 1.2}},
		PreimageReader: labPreimageReader{}, ValidateRecord: allowLabRecord,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lab.Approve("original", planned.Proposal.ID, "manual-original"); err != nil {
		t.Fatal(err)
	}
	journal, err := NewJournal(filepath.Join(t.TempDir(), "journal.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lab.Execute(context.Background(), "original", &labMutationPort{}, executionverifiers.SPAL{}, journal); err != nil {
		t.Fatal(err)
	}
	original, ok := lab.Session("original")
	if !ok {
		t.Fatal("original session missing")
	}
	originalManifest, err := spal.ManifestFromAction(original.FrozenPlan.ActionSet.Actions[0])
	if err != nil {
		t.Fatal(err)
	}
	rollbackCut := orchestration.ProjectCut{ProjectUUID: "project-1", ProjectEpoch: "epoch-1", BaseProjectRevision: "8", Consistency: "strong"}
	rollbackCut.Hash = rollbackCut.ComputeHash()
	rollback, err := lab.PlanRollback(context.Background(), RollbackPlanRequest{
		SessionID: "rollback", OriginalSessionID: "original", ProjectCut: rollbackCut,
		PreimageReader: fixedPreimageReader{values: originalManifest.Writes}, ValidateRecord: allowLabRecord,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rollback.Proposal == nil || rollback.ActionSet == nil || rollback.Proposal.Summary == "" {
		t.Fatalf("rollback did not become a proposal: %#v", rollback)
	}
	rollbackManifest, err := spal.ManifestFromAction(rollback.ActionSet.Actions[0])
	if err != nil || rollbackManifest.Operation != spal.OperationRollback || rollbackManifest.RollbackOf != originalManifest.ID {
		t.Fatalf("rollback manifest=%#v err=%v", rollbackManifest, err)
	}
	if _, err := lab.Execute(context.Background(), "rollback", &labMutationPort{}, executionverifiers.SPAL{}, journal); err == nil {
		t.Fatal("rollback executed before its own approval")
	}
	if _, err := lab.Approve("rollback", rollback.Proposal.ID, "manual-rollback"); err != nil {
		t.Fatal(err)
	}
	completed, err := lab.Execute(context.Background(), "rollback", &labMutationPort{}, executionverifiers.SPAL{}, journal)
	// labMutationPort deliberately has no VSP-specific detail; the port unit
	// tests cover receipt detail persistence. This assertion verifies the
	// governed rollback lifecycle completed without a direct-write shortcut.
	if err != nil || completed.Execution == nil || completed.Status != orchestration.StatusNeedsReview {
		t.Fatalf("rollback execution=%#v err=%v", completed, err)
	}

	changed := append([]spal.PhysicalParameter(nil), originalManifest.Writes...)
	changed[0].Value = 0.37
	if _, err := lab.PlanRollback(context.Background(), RollbackPlanRequest{
		SessionID: "unsafe", OriginalSessionID: "original", ProjectCut: rollbackCut,
		PreimageReader: fixedPreimageReader{values: changed}, ValidateRecord: allowLabRecord,
	}); err == nil {
		t.Fatal("rollback overwriting a manual/concurrent value was accepted")
	}
}

type labPreimageReader struct{}

func allowLabRecord(context.Context, ProviderRecord) error { return nil }

func (labPreimageReader) CaptureSPALPreimage(_ context.Context, _ spal.RuntimeBinding, writes []spal.PhysicalParameter) ([]spal.PhysicalParameter, error) {
	preimage := make([]spal.PhysicalParameter, 0, len(writes))
	for _, write := range writes {
		value := 0.0
		switch write.ParameterID {
		case "1":
			value = 0
		case "6":
			value = 1
		case "4":
			value = 100
		case "2":
			value = 0
		case "3":
			value = 1
		default:
			return nil, fmt.Errorf("unexpected TDR Nova lab write %s", write.ParameterID)
		}
		preimage = append(preimage, spal.PhysicalParameter{ParameterID: write.ParameterID, Value: value, Unit: write.Unit})
	}
	return preimage, nil
}

type fixedPreimageReader struct{ values []spal.PhysicalParameter }

func (r fixedPreimageReader) CaptureSPALPreimage(_ context.Context, _ spal.RuntimeBinding, _ []spal.PhysicalParameter) ([]spal.PhysicalParameter, error) {
	return append([]spal.PhysicalParameter(nil), r.values...), nil
}

type labMutationPort struct{ applied bool }

func (p *labMutationPort) Preflight(_ context.Context, _ orchestration.ActionSet, _ orchestration.ProjectCut) error {
	return nil
}

func (p *labMutationPort) Apply(_ context.Context, action orchestration.Action, _ string) (orchestration.ActionReceipt, error) {
	manifest, err := spal.ManifestFromAction(action)
	if err != nil {
		return orchestration.ActionReceipt{}, err
	}
	p.applied = true
	return orchestration.ActionReceipt{
		ActionID: action.ID, Status: "applied", EvidenceRefs: []string{"lab:parameter-readback"},
		Details: map[string]any{"structural_readback": "pass", "spal_manifest_id": manifest.ID, "binding_id": manifest.Binding.ID},
	}, nil
}

var _ executionruntime.MutationPort = (*labMutationPort)(nil)

func labConformanceRequest() TDRNovaConformanceRequest {
	return TDRNovaConformanceRequest{
		TargetRef: "track:bass", TrackID: "bass", PluginID: "nova-1", BandSlot: "band1",
		PluginSkill: labTDRNovaSkill(), ObservedParamIDs: []string{"1", "6", "4", "2", "3", "5"},
		StaticBellInvariant: StaticBellInvariant{ParameterID: "5", Value: 2, NormalizedValue: 0.4, ValueText: "Bell"},
		StaticBellConfirmed: true, StaticBellEvidence: []string{"lab:operator-confirmed-static-bell"},
	}
}

func labTDRNovaSkill() plugingrabber.PluginSkillDocument {
	domain := func(unit string, min, max float64) *plugingrabber.PluginDisplayDomain {
		return &plugingrabber.PluginDisplayDomain{Unit: unit, Min: &min, Max: &max, Status: "confirmed", Confidence: 1}
	}
	mapping := func(id, unit string, min, max float64) plugingrabber.PluginSkillParamMap {
		return plugingrabber.PluginSkillParamMap{ParamID: id, Source: "teach_mode_user_demonstrated", Confidence: 1, Confirmed: true, Locked: true, Status: plugingrabber.PluginSkillStatusActive, DisplayDomain: domain(unit, min, max)}
	}
	return plugingrabber.PluginSkillDocument{
		SchemaVersion: plugingrabber.PluginSkillSchemaVersion,
		Identity:      plugingrabber.PluginSkillIdentity{Name: "TDR Nova", Format: "VST3", Version: "2.2.2", ProfileKey: "plugin_f9788adda4203df8", ParamSignatureHash: "p_lab_tdr_nova"},
		Components: []plugingrabber.PluginSkillComponent{{
			ID: "band1", Role: "eq_band", Params: map[string]plugingrabber.PluginSkillParamMap{
				"enable":     mapping("1", "toggle", 0, 1),
				"dyn_enable": mapping("6", "toggle", 0, 1),
				"frequency":  mapping("4", "Hz", 10, 40000),
				"gain":       mapping("2", "dB", -18, 18),
				"q":          mapping("3", "Q", 0.1, 10),
				"type":       mapping("5", "enum", 0, 5),
			},
		}},
	}
}

func labPluginReply(skill plugingrabber.PluginSkillDocument) map[string]any {
	rows := []map[string]any{
		{"id": "1", "value": 0.0, "normalized_value": 0.0, "value_text": "Off"},
		{"id": "6", "value": 0.0, "normalized_value": 0.0, "value_text": "Off"},
		{"id": "4", "value": 100.0, "normalized_value": 0.5, "value_text": "100 Hz"},
		{"id": "2", "value": 0.0, "normalized_value": 0.5, "value_text": "0 dB"},
		{"id": "3", "value": 1.0, "normalized_value": 0.2, "value_text": "1.00"},
		{"id": "5", "value": 2.0, "normalized_value": 0.4, "value_text": "Bell"},
	}
	return map[string]any{
		"status": "ok", "track_id": "bass", "plugin_id": "nova-1",
		"current_param_signature_hash": skill.Identity.ParamSignatureHash,
		"profile_param_signature_hash": skill.Identity.ParamSignatureHash,
		"plugin_skill":                 skill, "parameters": rows,
	}
}

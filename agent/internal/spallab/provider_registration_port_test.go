package spallab

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/spal"
)

func TestProviderRegistrationPortPersistsOnlyTheFrozenCurrentRecord(t *testing.T) {
	record := testProductProviderRecord()
	cut := orchestration.ProjectCut{ProjectUUID: record.ProjectUUID, ProjectEpoch: "epoch-1", BaseProjectRevision: "9", Consistency: "strong"}
	cut.Hash = cut.ComputeHash()
	actionSet, err := ProviderRegistrationActionSet(record, cut)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewProviderStore(filepath.Join(t.TempDir(), "providers.json"))
	if err != nil {
		t.Fatal(err)
	}
	validations := 0
	port := ProviderRegistrationPort{
		Store: store,
		ValidateRecord: func(_ context.Context, got ProviderRecord) error {
			validations++
			if got.ID != record.ID {
				t.Fatalf("validator received %q, want %q", got.ID, record.ID)
			}
			return nil
		},
	}
	if err := port.Preflight(context.Background(), actionSet, cut); err != nil {
		t.Fatal(err)
	}
	receipt, err := port.Apply(context.Background(), actionSet.Actions[0], "idempotency")
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Status != "applied" || receipt.Details["structural_readback"] != "pass" || receipt.Details["provider_record_id"] != record.ID {
		t.Fatalf("unexpected receipt: %#v", receipt)
	}
	if validations != 2 {
		t.Fatalf("validations=%d, want preflight plus apply", validations)
	}
	rows, err := store.List()
	if err != nil || len(rows) != 1 || rows[0].ID != record.ID {
		t.Fatalf("persisted rows=%#v err=%v", rows, err)
	}
	reconciled, err := port.Reconcile(context.Background(), actionSet.Actions[0], "idempotency", cut)
	if err != nil || reconciled.Status != "applied" || reconciled.Details["reconciled_from_store"] != true {
		t.Fatalf("reconciled=%#v err=%v", reconciled, err)
	}
}

func testProductProviderRecord() ProviderRecord {
	now := time.Now().UTC()
	return ProviderRecord{
		SchemaVersion: "vit.spal_lab.v0", ID: "provider-record", ProjectUUID: "project-1", LabOnly: true,
		ProviderID: spal.ExperimentalTDRNovaProviderID,
		Instance: spal.ProviderInstance{
			ID: "instance-1", ProviderID: spal.ExperimentalTDRNovaProviderID, TargetRef: "track:bass", TrackID: "bass", PluginID: "nova-1",
			PluginSignature: spal.ExperimentalTDRNovaSignature, Status: spal.InstanceVerified,
			Metadata: map[string]string{
				"band_slot": "band1", "static_bell_ready": "true", "spal_lab_only": "true", "plugin_skill_signature": "skill-sig", "project_uuid": "project-1",
			},
		},
		PluginSkillSignature: "skill-sig", CurrentParameterSignature: "param-sig", PluginProfileKey: "plugin_f9788adda4203df8", PluginVersion: "2.2.2",
		StaticBellInvariant: StaticBellInvariant{ParameterID: "5", Value: 1, NormalizedValue: .25, ValueText: "Bell"},
		StaticBellEvidence:  []string{"test:static-bell"}, ConformanceEvidence: []string{"test:conformance"}, ObservedParameterIDs: []string{"1", "2", "3", "4", "5", "6"},
		CreatedAt: now, UpdatedAt: now,
	}
}

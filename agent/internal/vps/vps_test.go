package vps

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/spallab"
	plugingrabber "vit-daw-agent/internal/workflows/plugingrabber"
)

var foundationTime = time.Date(2026, time.July, 15, 8, 30, 0, 0, time.UTC)

func TestLibraryPersistsVPSAndDerivesOnlyVerifiedCredentials(t *testing.T) {
	libraryPath := filepath.Join(t.TempDir(), "state", "vps_library_v3.json")
	library, err := NewLibrary(libraryPath)
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}
	library.now = func() time.Time { return foundationTime }
	inventory, err := library.UpsertInventory(NewInventoryEntry(PluginIdentity{Name: "Unlearned EQ", Format: "VST3", ProfileKey: "unlearned_eq"}, "plugin_scan", foundationTime))
	if err != nil {
		t.Fatalf("UpsertInventory: %v", err)
	}
	if inventory.Status != InventoryStatusDiscovered {
		t.Fatalf("inventory status = %s, want discovered", inventory.Status)
	}

	verified := testVerifiedVPS()
	stored, err := library.Upsert(verified)
	if err != nil {
		t.Fatalf("Upsert verified VPS: %v", err)
	}
	if stored.Revision != 1 {
		t.Fatalf("first persisted revision = %d, want 1", stored.Revision)
	}
	pending := testPendingVPS()
	if _, imported, err := library.ImportDraft(pending); err != nil || !imported {
		t.Fatalf("ImportDraft pending VPS = imported:%v err:%v, want true nil", imported, err)
	}

	reopened, err := NewLibrary(libraryPath)
	if err != nil {
		t.Fatalf("reopen library: %v", err)
	}
	reopened.now = func() time.Time { return foundationTime }
	documents, err := reopened.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(documents) != 2 {
		t.Fatalf("persisted documents = %d, want 2", len(documents))
	}
	inventoryEntries, err := reopened.ListInventory()
	if err != nil || len(inventoryEntries) != 1 || inventoryEntries[0].ID != inventory.ID {
		t.Fatalf("persisted inventory = %#v err=%v", inventoryEntries, err)
	}
	catalog, err := reopened.Catalog()
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(catalog.Entries) != 1 {
		t.Fatalf("catalog entries = %#v, want only verified credential", catalog.Entries)
	}
	entry := catalog.Entries[0]
	if entry.CredentialID != "credential-verified" || entry.VPSID != verified.ID || entry.CapabilityID != StaticEQCapabilityID {
		t.Fatalf("unexpected catalog entry: %#v", entry)
	}
	if entry.PluginIdentity.Name != "Verified EQ" || entry.PluginIdentity.Fingerprint.LegacyParameterSignature != "" {
		t.Fatalf("catalog should contain canonical identity, got %#v", entry.PluginIdentity)
	}
}

func TestDefaultLibraryPathHonorsExplicitEnvironment(t *testing.T) {
	want := filepath.Join(t.TempDir(), "user-level-vps.json")
	t.Setenv(libraryPathEnv, want)
	if got := DefaultLibraryPath(); got != want {
		t.Fatalf("DefaultLibraryPath = %q, want %q", got, want)
	}
	library, err := OpenDefaultLibrary()
	if err != nil || library.Path() != want {
		t.Fatalf("OpenDefaultLibrary = %#v err=%v, want path %q", library, err, want)
	}
}

func TestLibraryUpsertAdvancesRevisionWithoutAffectingUnrelatedImport(t *testing.T) {
	library, err := NewLibrary(filepath.Join(t.TempDir(), "vps.json"))
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}
	clock := foundationTime
	library.now = func() time.Time { return clock }

	first, err := library.Upsert(testVerifiedVPS())
	if err != nil {
		t.Fatalf("initial Upsert: %v", err)
	}
	clock = clock.Add(time.Minute)
	first.SafetyAndRollback.RollbackMode = "restore_previous_state"
	updated, err := library.Upsert(first)
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if updated.Revision != 2 || updated.CreatedAt != first.CreatedAt || !updated.UpdatedAt.Equal(clock) {
		t.Fatalf("unexpected revision/timestamps after update: %#v", updated)
	}

	pending := testPendingVPS()
	if _, imported, err := library.ImportDraft(pending); err != nil || !imported {
		t.Fatalf("first ImportDraft = imported:%v err:%v", imported, err)
	}
	pending.SafetyAndRollback.RollbackMode = "unsafe_overwrite_attempt"
	existing, imported, err := library.ImportDraft(pending)
	if err != nil || imported {
		t.Fatalf("duplicate ImportDraft = imported:%v err:%v, want false nil", imported, err)
	}
	if existing.SafetyAndRollback.RollbackMode == "unsafe_overwrite_attempt" {
		t.Fatalf("ImportDraft overwrote an existing migration document")
	}
}

func TestFingerprintChangeMarksCredentialStaleAndRemovesCatalogEligibility(t *testing.T) {
	base := testVerifiedVPS()
	cases := []struct {
		name   string
		mutate func(*PluginFingerprint)
		want   string
	}{
		{"installation", func(f *PluginFingerprint) { f.Installation = "binary:new" }, "installation"},
		{"parameter surface", func(f *PluginFingerprint) { f.ParameterSurface = "params:new" }, "parameter-surface"},
		{"display surface", func(f *PluginFingerprint) { f.DisplaySurface = "display:new" }, "display-surface"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := cloneVPS(base)
			current := document.PluginIdentity.Fingerprint
			tc.mutate(&current)
			changes, err := document.ReconcileCredentialFingerprints(current, foundationTime)
			if err != nil {
				t.Fatalf("ReconcileCredentialFingerprints: %v", err)
			}
			if len(changes) != 1 || changes[0].To != CredentialStale || !strings.Contains(changes[0].Reason, tc.want) {
				t.Fatalf("unexpected credential state changes: %#v", changes)
			}
			if document.Status != VPSStatusStale || document.ProviderCredentials[0].Dispatchable() {
				t.Fatalf("credential should be stale/non-dispatchable: %#v", document)
			}
			if err := document.Validate(); err != nil {
				t.Fatalf("stale document should stay valid: %v", err)
			}
			catalog, err := DeriveProviderCatalog([]VPSDocument{document}, foundationTime)
			if err != nil {
				t.Fatalf("DeriveProviderCatalog: %v", err)
			}
			if len(catalog.Entries) != 0 {
				t.Fatalf("stale credential entered catalog: %#v", catalog.Entries)
			}
		})
	}
}

func TestVerifiedStaticEQCredentialRequiresFullConformance(t *testing.T) {
	document := testVerifiedVPS()
	document.ProviderCredentials[0].Conformance.Operations = []string{OperationBellCut}
	if err := document.Validate(); err == nil || !strings.Contains(err.Error(), "required operations") {
		t.Fatalf("incomplete conformance Validate error = %v, want required operations", err)
	}
	document = testVerifiedVPS()
	document.ProviderCredentials[0].PluginFingerprint.DisplaySurface = ""
	if err := document.Validate(); err == nil || !strings.Contains(err.Error(), "complete VPS plugin fingerprint") {
		t.Fatalf("weak fingerprint Validate error = %v, want complete fingerprint", err)
	}
	document = testVerifiedVPS()
	document.CapabilityProfiles = nil
	if err := document.Validate(); err == nil || !strings.Contains(err.Error(), "missing its persisted capability profile") {
		t.Fatalf("missing persisted profile Validate error = %v, want profile rejection", err)
	}
	document = testVerifiedVPS()
	document.SemanticCapabilities = nil
	if err := document.Validate(); err == nil || !strings.Contains(err.Error(), "missing its semantic capability declaration") {
		t.Fatalf("missing semantic capability Validate error = %v, want declaration rejection", err)
	}
}

func TestCredentialRevocationAndExplicitRequalificationStateTransitions(t *testing.T) {
	document := testVerifiedVPS()
	if err := document.RevokeCredential("credential-verified", "mapping disproven", foundationTime); err != nil {
		t.Fatalf("RevokeCredential: %v", err)
	}
	if document.Status != VPSStatusRevoked || document.ProviderCredentials[0].Status != CredentialRevoked || document.ProviderCredentials[0].InvalidationReason != "mapping disproven" {
		t.Fatalf("revocation state = %#v", document)
	}
	catalog, err := DeriveProviderCatalog([]VPSDocument{document}, foundationTime)
	if err != nil || len(catalog.Entries) != 0 {
		t.Fatalf("revoked credential entered catalog: %#v err=%v", catalog.Entries, err)
	}

	migrated := testPendingVPS()
	verified := testVerifiedVPS()
	if err := migrated.Requalify(verified.PluginIdentity, verified.ProviderCredentials, foundationTime); err != nil {
		t.Fatalf("Requalify: %v", err)
	}
	if migrated.Status != VPSStatusVerified || migrated.Migration.RequiresRequalification || !migrated.ProviderCredentials[0].Dispatchable() {
		t.Fatalf("explicit requalification did not establish verified state: %#v", migrated)
	}
	catalog, err = DeriveProviderCatalog([]VPSDocument{migrated}, foundationTime)
	if err != nil || len(catalog.Entries) != 1 {
		t.Fatalf("requalified credential missing from catalog: %#v err=%v", catalog.Entries, err)
	}
}

func TestLibraryRejectsUnsafeCredentialLifecycleEdits(t *testing.T) {
	library, err := NewLibrary(filepath.Join(t.TempDir(), "vps_library.json"))
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}
	stored, err := library.Upsert(testVerifiedVPS())
	if err != nil {
		t.Fatalf("seed verified VPS: %v", err)
	}

	unsafeDowngrade := cloneVPS(stored)
	unsafeDowngrade.Status = VPSStatusMapped
	unsafeDowngrade.ProviderCredentials[0].Status = CredentialCandidate
	unsafeDowngrade.ProviderCredentials[0].Revision = 2
	if _, err := library.Upsert(unsafeDowngrade); err == nil || !strings.Contains(err.Error(), "cannot transition") {
		t.Fatalf("verified->candidate Upsert error = %v, want lifecycle rejection", err)
	}

	unsafeRemoval := cloneVPS(stored)
	unsafeRemoval.Status = VPSStatusMapped
	unsafeRemoval.ProviderCredentials = nil
	if _, err := library.Upsert(unsafeRemoval); err == nil || !strings.Contains(err.Error(), "cannot remove dispatchable") {
		t.Fatalf("verified credential removal error = %v, want rejection", err)
	}

	if CredentialPendingRequalification.CanTransitionTo(CredentialVerified) || CredentialRevoked.CanTransitionTo(CredentialCandidate) {
		t.Fatalf("credential lifecycle permits an unsafe automatic transition")
	}
}

func TestMigratePluginSkillV2CreatesRequalificationDraft(t *testing.T) {
	skill := testPluginSkillV2()
	skill.Safety = map[string]any{"max_gain_db": 6}
	document, err := MigratePluginSkillDocumentV2(skill, foundationTime)
	if err != nil {
		t.Fatalf("MigratePluginSkillDocumentV2: %v", err)
	}
	if document.Status != VPSStatusDraft || !document.Migration.RequiresRequalification || len(document.ProviderCredentials) != 0 {
		t.Fatalf("Plugin Skill migration granted unexpected authority: %#v", document)
	}
	if document.PluginIdentity.Fingerprint.LegacyParameterSignature != skill.Identity.ParamSignatureHash || document.PluginIdentity.Fingerprint.Complete() {
		t.Fatalf("Plugin Skill migration fingerprint = %#v, want legacy-only", document.PluginIdentity.Fingerprint)
	}
	if len(document.ControlSurface.Mappings) < 3 || len(document.ControlSurface.MacroRelations) != 1 || len(document.Topology.Resources) != 1 || len(document.CapabilityProfiles) != 1 || document.CapabilityProfiles[0].ID != StaticEQCapabilityID {
		t.Fatalf("Plugin Skill migration lost control/topology evidence: %#v", document)
	}
	if !strings.Contains(string(document.SafetyAndRollback.LegacyPolicy), "max_gain_db") {
		t.Fatalf("Plugin Skill migration lost legacy safety policy: %s", document.SafetyAndRollback.LegacyPolicy)
	}
	if err := document.Validate(); err != nil {
		t.Fatalf("migrated Plugin Skill draft should validate: %v", err)
	}
}

func TestMigrateLegacyVPSProfilePatchCreatesDraft(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"schema": "vit.plugin_skill.v1",
		"target": map[string]any{
			"plugin_id":   "plugin-legacy",
			"plugin_name": "Legacy EQ",
		},
		"profile_patch": plugingrabber.ProfilePatch{
			Class:           "eq",
			QuickControlIDs: []string{"gain", "frequency"},
			Aliases:         map[string]string{"gain": "Gain"},
			NormalizedRoles: map[string]string{"frequency": "frequency"},
			Safety:          map[string]any{"max_gain_db": 6},
			VirtualControls: []map[string]any{{"name": "eq.cut_region", "inputs": []string{"freq_hz", "gain_db"}, "resolver": "choose_free_band"}},
		},
	})
	if err != nil {
		t.Fatalf("marshal legacy payload: %v", err)
	}
	document, err := MigrateLegacyVPS(payload, foundationTime)
	if err != nil {
		t.Fatalf("MigrateLegacyVPS: %v", err)
	}
	if document.Status != VPSStatusDraft || !document.Migration.RequiresRequalification || len(document.ProviderCredentials) != 0 {
		t.Fatalf("legacy .vps import should be a draft: %#v", document)
	}
	if !containsAll(document.Migration.Sources, []string{migrationSourceProfilePatch, migrationSourceLegacyVPS}) || len(document.ControlSurface.Mappings) != 2 || len(document.ControlSurface.MacroRelations) != 1 {
		t.Fatalf("legacy .vps import lost migration evidence: %#v", document)
	}
	if !strings.Contains(string(document.SafetyAndRollback.LegacyPolicy), "max_gain_db") {
		t.Fatalf("legacy .vps import lost safety policy: %s", document.SafetyAndRollback.LegacyPolicy)
	}
}

func TestMigrateSPALV0ProviderRecordStaysPendingAndOutOfCatalog(t *testing.T) {
	record := testSPALV0ProviderRecord()
	document, err := MigrateSPALV0ProviderRecord(record, foundationTime)
	if err != nil {
		t.Fatalf("MigrateSPALV0ProviderRecord: %v", err)
	}
	if len(document.ProviderCredentials) != 1 || document.ProviderCredentials[0].Status != CredentialPendingRequalification || len(document.CapabilityProfiles) != 1 || document.CapabilityProfiles[0].ID != StaticEQCapabilityID {
		t.Fatalf("v0 record migration credential = %#v, want pending requalification", document.ProviderCredentials)
	}
	if document.ProviderCredentials[0].Dispatchable() || !document.Migration.RequiresRequalification {
		t.Fatalf("v0 record must not be dispatchable after migration")
	}
	credentialJSON, err := json.Marshal(document.ProviderCredentials[0])
	if err != nil {
		t.Fatalf("marshal migrated credential: %v", err)
	}
	for _, forbidden := range []string{"track_id", "plugin_id", "target_ref", "band_slot"} {
		if strings.Contains(string(credentialJSON), forbidden) {
			t.Fatalf("project-instance field %q leaked into reusable credential: %s", forbidden, credentialJSON)
		}
	}
	catalog, err := DeriveProviderCatalog([]VPSDocument{document}, foundationTime)
	if err != nil {
		t.Fatalf("DeriveProviderCatalog: %v", err)
	}
	if len(catalog.Entries) != 0 {
		t.Fatalf("migrated v0 record entered v3 Catalog: %#v", catalog.Entries)
	}
}

func TestImportSPALV0ProviderStoreIsReadOnlyAndNeverPromotes(t *testing.T) {
	providerStore, err := spallab.NewProviderStore(filepath.Join(t.TempDir(), "spal_v0.json"))
	if err != nil {
		t.Fatalf("NewProviderStore: %v", err)
	}
	if _, err := providerStore.Upsert(testSPALV0ProviderRecord()); err != nil {
		t.Fatalf("seed SPAL v0 provider store: %v", err)
	}
	library, err := NewLibrary(filepath.Join(t.TempDir(), "vps_library.json"))
	if err != nil {
		t.Fatalf("NewLibrary: %v", err)
	}
	results, err := library.ImportSPALV0ProviderStore(providerStore, foundationTime)
	if err != nil {
		t.Fatalf("ImportSPALV0ProviderStore: %v", err)
	}
	if len(results) != 1 || !results[0].Imported || results[0].VPS.ProviderCredentials[0].Status != CredentialPendingRequalification {
		t.Fatalf("unexpected import results: %#v", results)
	}
	// A second import is idempotent and must not mutate the legacy source.
	again, err := library.ImportSPALV0ProviderStore(providerStore, foundationTime.Add(time.Hour))
	if err != nil || len(again) != 1 || again[0].Imported {
		t.Fatalf("second SPAL v0 import = %#v err=%v, want existing record", again, err)
	}
	records, err := providerStore.List()
	if err != nil || len(records) != 1 || records[0].ID != "spal-v0-record" {
		t.Fatalf("v3 migration changed v0 store: records=%#v err=%v", records, err)
	}
	catalog, err := library.Catalog()
	if err != nil || len(catalog.Entries) != 0 {
		t.Fatalf("v0 import must not enter catalog: %#v err=%v", catalog.Entries, err)
	}
}

func testVerifiedVPS() VPSDocument {
	identity := PluginIdentity{
		Manufacturer: "Example Audio",
		Name:         "Verified EQ",
		Format:       "VST3",
		Version:      "1.0.0",
		ProfileKey:   "verified_eq",
		Fingerprint: PluginFingerprint{
			Installation:     "binary:verified-eq-1.0.0",
			ParameterSurface: "params:verified-eq-v1",
			DisplaySurface:   "display:verified-eq-v1",
		},
	}
	profile := StaticEQConformanceProfileV0()
	document := NewDraft(identity, foundationTime)
	document.ID = "vps-verified"
	document.Status = VPSStatusVerified
	document.SemanticCapabilities = []SemanticCapability{{
		ID:          StaticEQCapabilityID,
		Operations:  append([]string(nil), profile.RequiredOperations...),
		Parameters:  append([]string(nil), profile.RequiredParameters...),
		FilterTypes: append([]string(nil), profile.RequiredFilterTypes...),
		Status:      string(CredentialVerified),
	}}
	document.CapabilityProfiles = []CapabilityConformanceProfile{profile}
	document.ProviderCredentials = []ProviderCredential{{
		ID:                "credential-verified",
		Revision:          1,
		CapabilityID:      StaticEQCapabilityID,
		Status:            CredentialVerified,
		PluginFingerprint: identity.Fingerprint,
		Conformance: CredentialConformance{
			ProfileID:           profile.ID,
			ProfileVersion:      profile.Version,
			Operations:          append([]string(nil), profile.RequiredOperations...),
			Parameters:          append([]string(nil), profile.RequiredParameters...),
			FilterTypes:         append([]string(nil), profile.RequiredFilterTypes...),
			StaticEQBinding:     &StaticEQBinding{ComponentID: "band_1", FilterTypeParameterID: "type", FrequencyParameterID: "frequency", GainParameterID: "gain", QParameterID: "q", EnabledParameterID: "enabled"},
			WriteReadbackPassed: true,
			BoundaryTestsPassed: true,
			RollbackTestPassed:  true,
			EvidenceRefs:        []string{"test:write-readback", "test:boundary", "test:rollback"},
			CompletedAt:         foundationTime,
		},
		EvidenceRefs: []string{"test:credential"},
		IssuedAt:     foundationTime,
		UpdatedAt:    foundationTime,
	}}
	return document
}

func testPendingVPS() VPSDocument {
	identity := PluginIdentity{Name: "Legacy EQ", Format: "VST3", ProfileKey: "legacy_eq"}
	document := NewDraft(identity, foundationTime)
	document.ID = "vps-pending"
	document.Migration = MigrationMetadata{Sources: []string{"test"}, RequiresRequalification: true, ImportedAt: foundationTime}
	document.ProviderCredentials = []ProviderCredential{{
		ID:                 "credential-pending",
		Revision:           1,
		CapabilityID:       StaticEQCapabilityID,
		Status:             CredentialPendingRequalification,
		MigrationSource:    "test",
		InvalidationReason: "requires full v3 conformance",
	}}
	return document
}

func testPluginSkillV2() plugingrabber.PluginSkillDocument {
	minimum, maximum := 10.0, 40000.0
	return plugingrabber.PluginSkillDocument{
		SchemaVersion: plugingrabber.PluginSkillSchemaVersion,
		Identity: plugingrabber.PluginSkillIdentity{
			Manufacturer:       "Example Audio",
			Name:               "Learned EQ",
			Format:             "VST3",
			Version:            "2.0.0",
			ProfileKey:         "learned_eq",
			ParamSignatureHash: "p_legacy_ids_only",
		},
		Capabilities: plugingrabber.PluginSkillCapabilities{Types: []string{"eq"}},
		Components: []plugingrabber.PluginSkillComponent{{
			ID:   "band1",
			Role: "eq_band",
			Params: map[string]plugingrabber.PluginSkillParamMap{
				"frequency": {ParamID: "freq", Label: "Frequency", Confirmed: true, DisplayDomain: &plugingrabber.PluginDisplayDomain{Unit: "Hz", Min: &minimum, Max: &maximum}},
				"gain":      {ParamID: "gain", Label: "Gain", Confirmed: true},
				"q":         {ParamID: "q", Label: "Q", Confirmed: true},
			},
		}},
		Operations: []plugingrabber.PluginSkillOperation{{Name: "eq.cut_region", Inputs: []string{"freq_hz", "gain_db"}, Resolver: "choose_free_band", ComponentID: "band1"}},
	}
}

func testSPALV0ProviderRecord() spallab.ProviderRecord {
	return spallab.ProviderRecord{
		SchemaVersion: spallab.SchemaVersion,
		ID:            "spal-v0-record",
		LabOnly:       true,
		ProviderID:    spal.ExperimentalTDRNovaProviderID,
		Instance: spal.ProviderInstance{
			ID:              "spal-v0-instance",
			ProviderID:      spal.ExperimentalTDRNovaProviderID,
			TargetRef:       "track:legacy",
			TrackID:         "legacy",
			PluginID:        "nova-legacy",
			PluginSignature: spal.ExperimentalTDRNovaSignature,
			Status:          spal.InstanceVerified,
			Metadata: map[string]string{
				"band_slot":         "band1",
				"static_bell_ready": "true",
				"spal_lab_only":     "true",
			},
		},
		PluginSkillSignature:      "p_legacy",
		CurrentParameterSignature: "p_legacy",
		PluginProfileKey:          "plugin_legacy",
		PluginVersion:             "2.2.2",
		StaticBellInvariant:       spallab.StaticBellInvariant{ParameterID: "5", Value: 0.5, NormalizedValue: 0.5, ValueText: "Bell"},
		StaticBellEvidence:        []string{"test:bell"},
		ConformanceEvidence:       []string{"test:conformance"},
		ObservedParameterIDs:      []string{"1", "2", "3", "4", "5", "6"},
		CreatedAt:                 foundationTime,
		UpdatedAt:                 foundationTime,
	}
}

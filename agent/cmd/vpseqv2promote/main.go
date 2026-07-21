package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/vps"
)

const confirmation = "I_CONFIRM_EQ_V2_CREDENTIAL_PROMOTION"

type rawLibrary struct {
	SchemaVersion   string            `json:"schema_version"`
	PluginInventory json.RawMessage   `json:"plugin_inventory,omitempty"`
	Documents       []json.RawMessage `json:"documents"`
}

type promotionRecord struct {
	SchemaVersion string                `json:"schema_version"`
	PromotedAt    time.Time             `json:"promoted_at"`
	LibraryPath   string                `json:"library_path"`
	BackupPath    string                `json:"backup_path,omitempty"`
	VPSID         string                `json:"vps_id"`
	VPSRevision   int                   `json:"vps_revision"`
	CredentialID  string                `json:"credential_id"`
	Fingerprint   vps.PluginFingerprint `json:"plugin_fingerprint"`
	Schemas       []string              `json:"conformed_schemas"`
	EvidenceRefs  []string              `json:"evidence_refs"`
	LibrarySHA256 string                `json:"library_sha256,omitempty"`
	Applied       bool                  `json:"applied"`
}

func main() {
	libraryPath := flag.String("library", filepath.Join(os.Getenv("APPDATA"), "Vit", "Agent", "vps_library_v3.json"), "VPS v3 Library path")
	targetID := flag.String("vps-id", "vps_tdr_nova_conversational_candidate_20260717_semantic_v6", "target VPS ID")
	referenceID := flag.String("fingerprint-source-vps-id", "vps_8c9699daa17a3603278d", "verified same-installation VPS used only for the already observed fingerprint")
	recordPath := flag.String("record", "", "promotion record output path")
	apply := flag.Bool("apply", false, "write the targeted promotion")
	confirm := flag.String("confirm", "", "required explicit confirmation phrase")
	flag.Parse()
	if *apply && *confirm != confirmation {
		fail("-apply requires -confirm %s", confirmation)
	}

	data, err := os.ReadFile(*libraryPath)
	if err != nil {
		fail("read library: %v", err)
	}
	var library rawLibrary
	if err := json.Unmarshal(data, &library); err != nil {
		fail("decode library: %v", err)
	}
	if library.SchemaVersion != vps.LibrarySchemaVersion {
		fail("unexpected library schema %q", library.SchemaVersion)
	}

	targetIndex, target := findDocument(library.Documents, *targetID)
	if targetIndex < 0 {
		fail("target VPS %s not found", *targetID)
	}
	_, reference := findDocument(library.Documents, *referenceID)
	if reference == nil {
		fail("fingerprint source VPS %s not found", *referenceID)
	}
	fingerprint, err := verifiedReferenceFingerprint(reference)
	if err != nil {
		fail("fingerprint source: %v", err)
	}

	now := time.Now().UTC()
	evidence := promotionEvidenceRefs()
	binding := tdrNovaEQV2Binding()
	schemas := []string{spal.EQBandPatchControlID, spal.EQPassFilterPatchControlID, spal.EQOutputPatchControlID}
	credentialID := deterministicID("credential_eq_v2", *targetID, fingerprint.Installation, fingerprint.ParameterSurface, fingerprint.DisplaySurface)
	credential := vps.ProviderCredential{
		ID: credentialID, Revision: 1, CapabilityID: vps.EqualizerCapabilityID, Status: vps.CredentialVerified, PluginFingerprint: fingerprint,
		Conformance: vps.CredentialConformance{
			ProfileID: vps.EqualizerCapabilityID, ProfileVersion: vps.EqualizerProfileVersion,
			Operations:  []string{vps.OperationEQBandPatch, vps.OperationEQPassFilterPatch, vps.OperationEQOutputPatch},
			Parameters:  []string{"band_ref", "enabled", "response_shape", "frequency_hz", "gain_db", "q", "filter_kind", "cutoff_frequency_hz", "slope_db_per_octave", "bypass", "dry_mix_percent", "output_gain_db"},
			FilterTypes: []string{"bell", "low_shelf", "high_shelf", "highpass", "lowpass"}, ConformedSchemas: schemas, EQV2Binding: &binding,
			WriteReadbackPassed: true, BoundaryTestsPassed: true, RollbackTestPassed: true, BehaviorTestsPassed: true,
			BehaviorScope:        "user-confirmed GUI response geometry and control lifecycle; no transfer-function or musical-success claim",
			StateRetentionPassed: true, ResourceIsolationPassed: true, EvidenceRefs: evidence, CompletedAt: now,
		},
		Safety:       vps.SafetyAndRollback{Preconditions: []string{"Fresh runtime fingerprint must equal this Credential before every dispatch.", "Only conformed_schemas and persisted EQ v2 bindings may be dispatched."}, Bounds: []string{"Band gain -18..18 dB", "Output gain -20..20 dB", "Frequency 10..40000 Hz", "Q 0.1..6"}, RollbackMode: "restore_previous_state", Invalidators: []string{"plugin installation fingerprint changes", "parameter-surface fingerprint changes", "display-surface fingerprint changes", "accepted mapping changes"}},
		EvidenceRefs: evidence, IssuedAt: now, UpdatedAt: now,
	}

	setMap(target, "status", string(vps.VPSStatusVerified))
	setTargetFingerprint(target, fingerprint)
	upsertProfile(target, vps.EqualizerConformanceProfileV2())
	upsertSemanticCapability(target, map[string]any{"id": vps.EqualizerCapabilityID, "operations": credential.Conformance.Operations, "parameters": credential.Conformance.Parameters, "filter_types": credential.Conformance.FilterTypes, "status": "conformed", "evidence_refs": evidence})
	setConformedCapabilityState(target, schemas)
	confirmControlMappings(target)
	copySpecialCapabilityDigest(target)
	appendUniqueStrings(target, "conformance_evidence", evidence)
	setProviderCredential(target, credential)

	finalRaw, err := json.Marshal(target)
	if err != nil {
		fail("encode target: %v", err)
	}
	var typed vps.VPSDocument
	if err := json.Unmarshal(finalRaw, &typed); err != nil {
		fail("decode promoted VPS: %v", err)
	}
	if err := typed.Validate(); err != nil {
		fail("promoted VPS validation: %v", err)
	}
	catalog, err := vps.DeriveProviderCatalog([]vps.VPSDocument{typed}, now)
	if err != nil || len(catalog.Entries) != 1 {
		fail("promoted Catalog validation: entries=%d err=%v", len(catalog.Entries), err)
	}
	library.Documents[targetIndex] = finalRaw
	out, err := json.MarshalIndent(library, "", "  ")
	if err != nil {
		fail("encode library: %v", err)
	}
	out = append(out, '\n')

	record := promotionRecord{SchemaVersion: "vit.vps.eq_v2_promotion_record.v1", PromotedAt: now, LibraryPath: *libraryPath, VPSID: *targetID, VPSRevision: typed.Revision, CredentialID: credentialID, Fingerprint: fingerprint, Schemas: schemas, EvidenceRefs: evidence, Applied: *apply}
	if *apply {
		backup := *libraryPath + ".pre-eq-v2-" + now.Format("20060102T150405Z") + ".bak"
		if err := os.WriteFile(backup, data, 0o600); err != nil {
			fail("write backup: %v", err)
		}
		record.BackupPath = backup
		tmp := *libraryPath + ".eq-v2.tmp"
		if err := os.WriteFile(tmp, out, 0o600); err != nil {
			fail("write temporary library: %v", err)
		}
		if err := os.Rename(tmp, *libraryPath); err != nil {
			fail("replace library: %v", err)
		}
		sum := sha256.Sum256(out)
		record.LibrarySHA256 = "sha256:" + hex.EncodeToString(sum[:])
	}
	if strings.TrimSpace(*recordPath) != "" {
		recordBytes, _ := json.MarshalIndent(record, "", "  ")
		recordBytes = append(recordBytes, '\n')
		if err := os.WriteFile(*recordPath, recordBytes, 0o600); err != nil {
			fail("write promotion record: %v", err)
		}
	}
	encoded, _ := json.MarshalIndent(record, "", "  ")
	fmt.Println(string(encoded))
}

func findDocument(documents []json.RawMessage, id string) (int, map[string]any) {
	for index, raw := range documents {
		var document map[string]any
		if json.Unmarshal(raw, &document) == nil && text(document["id"]) == id {
			return index, document
		}
	}
	return -1, nil
}

func verifiedReferenceFingerprint(document map[string]any) (vps.PluginFingerprint, error) {
	identity := object(document["plugin_identity"])
	fingerprintMap := object(identity["fingerprint"])
	fingerprint := vps.PluginFingerprint{Installation: text(fingerprintMap["installation"]), ParameterSurface: text(fingerprintMap["parameter_surface"]), DisplaySurface: text(fingerprintMap["display_surface"]), LegacyParameterSignature: text(fingerprintMap["legacy_parameter_signature"])}
	if !fingerprint.Complete() {
		return fingerprint, fmt.Errorf("reference fingerprint is incomplete")
	}
	for _, item := range array(document["provider_credentials"]) {
		credential := object(item)
		if text(credential["status"]) != string(vps.CredentialVerified) {
			continue
		}
		var candidate vps.PluginFingerprint
		bytes, _ := json.Marshal(credential["plugin_fingerprint"])
		if json.Unmarshal(bytes, &candidate) == nil && candidate.Equal(fingerprint) {
			return fingerprint, nil
		}
	}
	return fingerprint, fmt.Errorf("reference has no verified Credential for its current fingerprint")
}

func tdrNovaEQV2Binding() spal.EQV2Binding {
	linear := func(id, unit string, min, max float64) spal.ParameterBinding {
		return spal.ParameterBinding{ParameterID: id, Unit: unit, Min: min, Max: max, Scale: "linear"}
	}
	log := func(id, unit string, min, max float64) spal.ParameterBinding {
		return spal.ParameterBinding{ParameterID: id, Unit: unit, Min: min, Max: max, Scale: "log"}
	}
	shape := func(id string) spal.EnumParameterBinding {
		return spal.EnumParameterBinding{ParameterID: id, Values: map[string]float64{"low_shelf": 0, "bell": .5, "high_shelf": .75}}
	}
	band := func(component string, base int) spal.EQV2BandBinding {
		return spal.EQV2BandBinding{ComponentID: component, Enabled: linear(fmt.Sprint(base+1), "toggle", 0, 1), GainDB: linear(fmt.Sprint(base+2), "dB", -18, 18), Q: log(fmt.Sprint(base+3), "Q", .1, 6), FrequencyHz: log(fmt.Sprint(base+4), "Hz", 10, 40000), ResponseShape: shape(fmt.Sprint(base + 5))}
	}
	slope := func(id string) spal.EnumParameterBinding {
		return spal.EnumParameterBinding{ParameterID: id, Values: map[string]float64{"6": 0, "12": 1.0 / 3.0, "24": 2.0 / 3.0, "72": 1}}
	}
	hp := &spal.EQV2PassFilterBinding{ComponentID: "hp", Enabled: linear("49", "toggle", 0, 1), CutoffFrequencyHz: log("50", "Hz", 10, 40000), SlopeDBPerOctave: slope("51")}
	lp := &spal.EQV2PassFilterBinding{ComponentID: "lp", Enabled: linear("53", "toggle", 0, 1), CutoffFrequencyHz: log("54", "Hz", 10, 40000), SlopeDBPerOctave: slope("55")}
	bypass, dry, output := linear("62", "toggle", 0, 1), linear("64", "%", 0, 100), linear("65", "dB", -20, 20)
	return spal.EQV2Binding{ConformedSchemas: []string{spal.EQBandPatchControlID, spal.EQPassFilterPatchControlID, spal.EQOutputPatchControlID}, Bands: map[string]spal.EQV2BandBinding{"b1": band("b1", 0), "b2": band("b2", 12), "b3": band("b3", 24), "b4": band("b4", 36)}, HighPass: hp, LowPass: lp, Output: &spal.EQV2OutputBinding{ComponentID: "io", Bypass: &bypass, DryMix: &dry, OutputGainDB: &output}}
}

func promotionEvidenceRefs() []string {
	values := []string{
		"EQ_V2_CONFORMANCE_ASSESSMENT_20260717.json",
		"vps_v3_conformance:fd6f5ba049bf332f",
		"runtime_draft_test_evidence/vps_draft_run_1ad4c404380f8d1d.json", "runtime_draft_test_evidence/vps_draft_run_1ea4fc0e4c00ffcd.json", "runtime_draft_test_evidence/vps_draft_run_692a44aa98047ad6.json", "runtime_draft_test_evidence/vps_draft_run_92257d2f731df57c.json", "runtime_draft_test_evidence/vps_draft_run_eb6f1ff7cb8b4422.json",
		"runtime_draft_test_evidence/vps_draft_run_24f83f7e13a732b8.json", "runtime_draft_test_evidence/vps_draft_run_f2a48661e793a584.json", "runtime_draft_test_evidence/vps_draft_run_fc49df21fe518702.json", "runtime_draft_test_evidence/vps_draft_run_163a29f4b2d1c7b9.json",
		"v6_semantic_smoke_evidence/v6_semantic_smoke_evidence_20260717.json",
	}
	sort.Strings(values)
	return values
}

func setTargetFingerprint(target map[string]any, fingerprint vps.PluginFingerprint) {
	identity := object(target["plugin_identity"])
	bytes, _ := json.Marshal(fingerprint)
	var value map[string]any
	_ = json.Unmarshal(bytes, &value)
	identity["fingerprint"] = value
	target["plugin_identity"] = identity
}
func upsertProfile(target map[string]any, profile vps.CapabilityConformanceProfile) {
	target["capability_conformance_profiles"] = upsertTypedByID(array(target["capability_conformance_profiles"]), profile.ID, profile)
}
func upsertSemanticCapability(target map[string]any, capability map[string]any) {
	target["semantic_capabilities"] = upsertMapByID(array(target["semantic_capabilities"]), text(capability["id"]), capability)
}
func setProviderCredential(target map[string]any, credential vps.ProviderCredential) {
	target["provider_credentials"] = upsertTypedByID(array(target["provider_credentials"]), credential.ID, credential)
}

func setConformedCapabilityState(target map[string]any, schemas []string) {
	allowed := map[string]bool{"eq.static_parametric.v1": true, "eq.pass_filter.highpass.v1": true, "eq.pass_filter.lowpass.v1": true, "eq.output_control.v1": true}
	items := array(target["semantic_capabilities"])
	for _, item := range items {
		capability := object(item)
		if allowed[text(capability["id"])] {
			capability["status"] = "conformed"
			capability["dispatchable"] = true
			capability["conformed_schemas"] = schemas
		}
	}
	target["semantic_capabilities"] = items
}

func confirmControlMappings(target map[string]any) {
	control := object(target["control_surface"])
	mappings := array(control["mappings"])
	for _, item := range mappings {
		mapping := object(item)
		component, slot := text(mapping["component_id"]), text(mapping["semantic_slot"])
		if !dispatchMapping(component, slot) {
			continue
		}
		mapping["confirmed"] = true
		mapping["binding_status"] = "conformed"
		mapping["execution_scope"] = "vps_eq_v2_credential"
		domain := object(mapping["display_domain"])
		configureDomain(domain, component, slot)
		mapping["display_domain"] = domain
		if slot == "response_shape" {
			mapping["enum_values"] = []string{"low_shelf", "bell", "high_shelf"}
		}
		if slot == "slope" {
			mapping["enum_values"] = []string{"6", "12", "24", "72"}
		}
	}
	control["mappings"] = mappings
	control["status"] = "conformed"
	control["draft_state"] = "credential_bound"
	target["control_surface"] = control
}

func dispatchMapping(component, slot string) bool {
	if strings.HasPrefix(component, "b") && len(component) == 2 {
		return slot == "enable" || slot == "gain" || slot == "q" || slot == "frequency" || slot == "response_shape"
	}
	if component == "hp" || component == "lp" {
		return slot == "enable" || slot == "frequency" || slot == "slope"
	}
	if component == "io" {
		return slot == "bypass" || slot == "dry_mix" || slot == "output_gain"
	}
	return false
}

func configureDomain(domain map[string]any, component, slot string) {
	domain["status"] = "conformed"
	domain["source"] = "user_gui_witness_plus_host_display_probe"
	switch slot {
	case "enable", "bypass":
		domain["unit"], domain["min"], domain["max"], domain["scale"] = "toggle", 0, 1, "linear"
	case "gain":
		domain["unit"], domain["min"], domain["max"], domain["scale"] = "dB", -18, 18, "linear"
	case "q":
		domain["unit"], domain["min"], domain["max"], domain["scale"] = "Q", .1, 6, "log"
	case "frequency":
		domain["unit"], domain["min"], domain["max"], domain["scale"] = "Hz", 10, 40000, "log"
	case "response_shape":
		domain["unit"], domain["min"], domain["max"], domain["scale"] = "enum", 0, 1, "enum"
	case "slope":
		domain["unit"], domain["min"], domain["max"], domain["scale"] = "dB/oct", 6, 72, "enum"
	case "dry_mix":
		domain["unit"], domain["min"], domain["max"], domain["scale"] = "%", 0, 100, "linear"
	case "output_gain":
		domain["unit"], domain["min"], domain["max"], domain["scale"] = "dB", -20, 20, "linear"
	}
	_ = component
}

func copySpecialCapabilityDigest(target map[string]any) {
	extensions := object(target["draft_extensions"])
	specials := array(extensions["special_capabilities"])
	for _, item := range specials {
		value := object(item)
		value["dispatchable"] = false
		value["invocation_policy"] = "user_initiated_only"
	}
	target["special_capabilities"] = specials
}
func appendUniqueStrings(target map[string]any, key string, values []string) {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range array(target[key]) {
		value := text(item)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	target[key] = out
}
func upsertTypedByID(items []any, id string, value any) []any {
	bytes, _ := json.Marshal(value)
	var mapped map[string]any
	_ = json.Unmarshal(bytes, &mapped)
	return upsertMapByID(items, id, mapped)
}
func upsertMapByID(items []any, id string, value map[string]any) []any {
	for index, item := range items {
		if text(object(item)["id"]) == id {
			items[index] = value
			return items
		}
	}
	return append(items, value)
}
func deterministicID(prefix string, values ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(values, "\x00")))
	return prefix + "_" + hex.EncodeToString(sum[:])[:16]
}
func object(value any) map[string]any {
	if mapped, ok := value.(map[string]any); ok {
		return mapped
	}
	return map[string]any{}
}
func array(value any) []any {
	if values, ok := value.([]any); ok {
		return values
	}
	return []any{}
}
func text(value any) string                               { return strings.TrimSpace(fmt.Sprint(value)) }
func setMap(target map[string]any, key string, value any) { target[key] = value }
func fail(format string, args ...any)                     { fmt.Fprintf(os.Stderr, format+"\n", args...); os.Exit(1) }

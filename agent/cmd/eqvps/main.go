package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/eqcontrolgraph"
)

type pathsFlag []string

func (values *pathsFlag) String() string { return strings.Join(*values, ",") }
func (values *pathsFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("path is empty")
	}
	*values = append(*values, value)
	return nil
}

type vitPage struct {
	Page struct {
		Index    int `json:"index"`
		Total    int `json:"total"`
		RowCount int `json:"row_count"`
	} `json:"page"`
	Parameters []vitParameter `json:"parameters"`
}

type vitParameter struct {
	ID              string      `json:"param_id"`
	Name            string      `json:"name"`
	NormalizedValue interface{} `json:"normalized_value"`
	ValueText       string      `json:"value_text"`
	DisplayProbe    struct {
		Samples []struct {
			NormalizedValue float64 `json:"normalized_value"`
			Text            string  `json:"text"`
		} `json:"samples"`
		DiscreteLabels []struct {
			Value interface{} `json:"value"`
			Label string      `json:"label"`
		} `json:"discrete_labels"`
	} `json:"display_probe"`
}

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: eqvps <validate|candidate|promote> [flags]")
	}
	switch os.Args[1] {
	case "validate":
		validateCommand(os.Args[2:])
	case "candidate":
		candidateCommand(os.Args[2:])
	case "promote":
		promoteCommand(os.Args[2:])
	default:
		fatalf("unknown command %q", os.Args[1])
	}
}

func promoteCommand(args []string) {
	flags := flag.NewFlagSet("promote", flag.ExitOnError)
	vps := flags.String("vps", "", "control-graph VPS document")
	verification := flags.String("verification", "", "candidate verification sidecar; defaults beside the VPS")
	smoke := flags.String("smoke", "", "passing generic A/B/C smoke summary")
	_ = flags.Parse(args)
	if strings.TrimSpace(*vps) == "" || strings.TrimSpace(*smoke) == "" {
		fatalf("-vps and -smoke are required")
	}
	document, err := eqcontrolgraph.LoadDocument(*vps)
	if err != nil {
		fatalf("%v", err)
	}
	path := strings.TrimSpace(*verification)
	if path == "" {
		path = eqcontrolgraph.AttestationPath(*vps)
	}
	attestation, err := eqcontrolgraph.LoadAttestation(path)
	if err != nil {
		fatalf("%v", err)
	}
	if err = eqcontrolgraph.ValidateAttestation(document, attestation, true); err != nil {
		fatalf("candidate attestation: %v", err)
	}
	if attestation.Status != "candidate" {
		fatalf("verification status must be candidate before promotion")
	}
	evidence, err := validateSmokeEvidence(*smoke, document)
	if err != nil {
		fatalf("smoke evidence: %v", err)
	}
	attestation.Status = "verified"
	attestation.VerifiedAt = time.Now().UTC()
	attestation.LiveChecks = eqcontrolgraph.LiveCheckEvidence{
		ApplyReadback: true, FormalUndoZeroDrift: true, UnloadRestoresBaseline: true,
	}
	attestation.Evidence = append(attestation.Evidence, evidence...)
	if err = eqcontrolgraph.SaveAttestation(path, attestation); err != nil {
		fatalf("save verification: %v", err)
	}
	writeResult(map[string]interface{}{"status": "verified", "verification_path": path,
		"verified_at": attestation.VerifiedAt, "evidence": evidence})
}

func validateSmokeEvidence(path string, document eqcontrolgraph.Document) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var summary map[string]interface{}
	if err = json.Unmarshal(raw, &summary); err != nil {
		return nil, err
	}
	if textValue(summary["schema_version"]) != "vit.eq_vps.control_graph.smoke_result.v1" ||
		textValue(summary["status"]) != "passed" {
		return nil, fmt.Errorf("summary is not a passing Control Graph smoke result")
	}
	if !strings.EqualFold(textValue(summary["plugin"]), document.Plugin.Name) {
		return nil, fmt.Errorf("summary plugin %q does not match %q", textValue(summary["plugin"]), document.Plugin.Name)
	}
	if final, _ := summary["final_zero_drift"].(bool); !final {
		return nil, fmt.Errorf("final zero drift is not proven")
	}
	a := objectValue(summary["A_without_vps"])
	b := objectValue(summary["B_with_vps"])
	c := objectValue(summary["C_after_unload"])
	if !jsonEquivalent(a["topology"], c["topology"]) {
		return nil, fmt.Errorf("unload did not restore the baseline topology")
	}
	bTopology := objectValue(b["topology"])
	if textValue(bTopology["mapping_source"]) != "vps_control_graph" {
		return nil, fmt.Errorf("B did not use vps_control_graph")
	}
	successCount := 0
	for _, item := range arrayValue(b["tests"]) {
		row := objectValue(item)
		if status := textValue(row["status"]); status != "exact" && status != "quantized" {
			continue
		}
		successCount++
		if zero, _ := row["zero_drift_after_undo"].(bool); !zero {
			return nil, fmt.Errorf("successful B test lacks zero-drift undo")
		}
		undo := objectValue(row["formal_undo"])
		rollback := objectValue(undo["rollback"])
		if textValue(undo["status"]) != "exact" || rollback["verified"] != true {
			return nil, fmt.Errorf("successful B test lacks verified formal undo")
		}
		if len(arrayValue(row["actual_readback"])) == 0 {
			return nil, fmt.Errorf("successful B test lacks actual readback")
		}
	}
	if successCount == 0 {
		return nil, fmt.Errorf("no successful B apply/readback case")
	}
	for _, key := range []string{"audio_probe_count", "learning_call_count", "profile_call_count", "spal_call_count", "b4_call_count"} {
		if value, ok := finiteNumber(objectValue(summary["audit"])[key]); !ok || value != 0 {
			return nil, fmt.Errorf("prohibited audit count %s is not zero", key)
		}
	}
	return []string{path, fmt.Sprintf("successful_apply_readback_cases=%d", successCount)}, nil
}

func objectValue(value interface{}) map[string]interface{} {
	object, _ := value.(map[string]interface{})
	return object
}

func arrayValue(value interface{}) []interface{} {
	array, _ := value.([]interface{})
	return array
}

func textValue(value interface{}) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func jsonEquivalent(left, right interface{}) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func validateCommand(args []string) {
	flags := flag.NewFlagSet("validate", flag.ExitOnError)
	vps := flags.String("vps", "", "control-graph VPS document")
	_ = flags.Parse(args)
	if strings.TrimSpace(*vps) == "" {
		fatalf("-vps is required")
	}
	document, err := eqcontrolgraph.LoadDocument(*vps)
	if err != nil {
		fatalf("%v", err)
	}
	hash, err := eqcontrolgraph.HashDocument(document)
	if err != nil {
		fatalf("%v", err)
	}
	writeResult(map[string]interface{}{"status": "valid", "schema_version": document.SchemaVersion,
		"vps_hash": hash, "binding_count": len(document.Bindings), "section_count": len(document.Sections)})
}

func candidateCommand(args []string) {
	flags := flag.NewFlagSet("candidate", flag.ExitOnError)
	vps := flags.String("vps", "", "control-graph VPS document")
	output := flags.String("output", "", "verification sidecar path; defaults beside the VPS")
	var surfaces pathsFlag
	flags.Var(&surfaces, "surface", "fresh Vit parameter page; repeat for pagination")
	_ = flags.Parse(args)
	if strings.TrimSpace(*vps) == "" || len(surfaces) == 0 {
		fatalf("-vps and at least one -surface are required")
	}
	document, err := eqcontrolgraph.LoadDocument(*vps)
	if err != nil {
		fatalf("%v", err)
	}
	surface, observedCount, err := loadVitSurface(document, surfaces)
	if err != nil {
		fatalf("%v", err)
	}
	checks, err := eqcontrolgraph.ValidateLive(document, surface)
	if err != nil {
		fatalf("live surface validation: %v", err)
	}
	hash, err := eqcontrolgraph.HashDocument(document)
	if err != nil {
		fatalf("%v", err)
	}
	attestation := eqcontrolgraph.Attestation{SchemaVersion: eqcontrolgraph.AttestationVersion,
		VPSHash: hash, SurfaceSignature: document.SurfaceSignature, Status: "candidate", StaticChecks: checks}
	path := strings.TrimSpace(*output)
	if path == "" {
		path = eqcontrolgraph.AttestationPath(*vps)
	}
	if err = eqcontrolgraph.SaveAttestation(path, attestation); err != nil {
		fatalf("save attestation: %v", err)
	}
	writeResult(map[string]interface{}{"status": "candidate", "verification_path": path, "vps_hash": hash,
		"static_checks": checks, "observed_parameter_count": observedCount,
		"limitations": []string{"identity and signature are document-bound inputs at candidate stage", "live apply/readback/undo/unload checks are not attested"}})
}

func loadVitSurface(document eqcontrolgraph.Document, paths []string) (eqcontrolgraph.LiveSurface, int, error) {
	parameters := map[string]vitParameter{}
	total := -1
	pageIndexes := map[int]bool{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return eqcontrolgraph.LiveSurface{}, 0, err
		}
		var page vitPage
		if err = json.Unmarshal(raw, &page); err != nil {
			return eqcontrolgraph.LiveSurface{}, 0, fmt.Errorf("decode %s: %w", path, err)
		}
		if page.Page.RowCount != len(page.Parameters) || page.Page.Total <= 0 {
			return eqcontrolgraph.LiveSurface{}, 0, fmt.Errorf("surface page %s has inconsistent pagination", path)
		}
		if total >= 0 && total != page.Page.Total {
			return eqcontrolgraph.LiveSurface{}, 0, fmt.Errorf("surface pages disagree on total parameter count")
		}
		total = page.Page.Total
		if pageIndexes[page.Page.Index] {
			return eqcontrolgraph.LiveSurface{}, 0, fmt.Errorf("surface repeats page index %d", page.Page.Index)
		}
		pageIndexes[page.Page.Index] = true
		for _, parameter := range page.Parameters {
			if strings.TrimSpace(parameter.ID) == "" {
				return eqcontrolgraph.LiveSurface{}, 0, fmt.Errorf("surface page %s contains an empty parameter ID", path)
			}
			if _, exists := parameters[parameter.ID]; exists {
				return eqcontrolgraph.LiveSurface{}, 0, fmt.Errorf("surface repeats parameter %s", parameter.ID)
			}
			parameters[parameter.ID] = parameter
		}
	}
	if len(parameters) != total {
		return eqcontrolgraph.LiveSurface{}, 0, fmt.Errorf("surface pagination is incomplete: observed %d of %d parameters", len(parameters), total)
	}
	surface := eqcontrolgraph.LiveSurface{Plugin: document.Plugin, Signature: document.SurfaceSignature}
	keys := make([]string, 0, len(document.Bindings))
	for key := range document.Bindings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		binding := document.Bindings[key]
		observed, ok := parameters[binding.ParameterID]
		if !ok {
			return eqcontrolgraph.LiveSurface{}, 0, fmt.Errorf("binding %s parameter %s is absent", key, binding.ParameterID)
		}
		live := eqcontrolgraph.LiveParameter{ID: observed.ID, Name: observed.Name, CurrentText: observed.ValueText}
		if value, ok := finiteNumber(observed.NormalizedValue); ok {
			live.CurrentNormalized = value
		}
		if binding.Kind == "number" {
			for _, sample := range observed.DisplayProbe.Samples {
				physical, ok := parseDisplayNumber(sample.Text, binding.Domain.Unit)
				if ok {
					live.Samples = append(live.Samples, eqcontrolgraph.LiveNumericSample{Normalized: sample.NormalizedValue, Physical: physical})
				}
			}
		} else {
			for _, item := range observed.DisplayProbe.DiscreteLabels {
				normalized, ok := finiteNumber(item.Value)
				if ok {
					live.EnumValues = append(live.EnumValues, eqcontrolgraph.LiveEnumValue{Normalized: normalized, Label: item.Label})
				}
			}
		}
		surface.Parameters = append(surface.Parameters, live)
	}
	return surface, len(parameters), nil
}

func parseDisplayNumber(text, unit string) (float64, bool) {
	lower := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(text), " ", ""))
	matchEnd := 0
	for matchEnd < len(lower) && (lower[matchEnd] == '+' || lower[matchEnd] == '-' || lower[matchEnd] == '.' ||
		lower[matchEnd] == ',' || lower[matchEnd] >= '0' && lower[matchEnd] <= '9') {
		matchEnd++
	}
	if matchEnd == 0 {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.ReplaceAll(lower[:matchEnd], ",", "."), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	if strings.EqualFold(strings.TrimSpace(unit), "Hz") && strings.HasPrefix(lower[matchEnd:], "k") {
		value *= 1000
	}
	return value, true
}

func finiteNumber(value interface{}) (float64, bool) {
	var number float64
	switch typed := value.(type) {
	case float64:
		number = typed
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		number = parsed
	default:
		return 0, false
	}
	return number, !math.IsNaN(number) && !math.IsInf(number, 0)
}

func writeResult(value interface{}) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fatalf("%v", err)
	}
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "eqvps: "+format+"\n", args...)
	os.Exit(1)
}

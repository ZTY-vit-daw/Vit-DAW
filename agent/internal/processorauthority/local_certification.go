package processorauthority

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/pluginsemantics"
)

const SchemaLocalCompressorCertification = "processor_authority.local_compressor_certification.v1"

// LocalCertificationOptions defines one deterministic, zero-LLM compressor
// certification against the live Agent HTTP tool surface.
type LocalCertificationOptions struct {
	AgentHTTP         string
	Entry             pluginsemantics.Entry
	OutputDir         string
	Timeout           time.Duration
	SnapshotTolerance float64
	HTTPClient        *http.Client
}

type LocalCertificationReport struct {
	SchemaVersion string           `json:"schema_version"`
	Status        string           `json:"status"`
	Verdict       string           `json:"verdict"`
	CompletedAt   string           `json:"completed_at"`
	Audit         map[string]any   `json:"audit"`
	Results       []map[string]any `json:"results"`
	SummaryPath   string           `json:"summary_path"`
	EvidencePath  string           `json:"evidence_path"`
}

type invokeResponse struct {
	Status string         `json:"status"`
	Result map[string]any `json:"result,omitempty"`
	Error  string         `json:"error,omitempty"`
}

func CertifyCompressor(opts LocalCertificationOptions) (LocalCertificationReport, error) {
	if strings.TrimSpace(opts.AgentHTTP) == "" {
		opts.AgentHTTP = "http://127.0.0.1:7878"
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 180 * time.Second
	}
	if opts.SnapshotTolerance <= 0 {
		opts.SnapshotTolerance = 0.0001
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: opts.Timeout}
	}
	if strings.TrimSpace(opts.Entry.Identifier) == "" || strings.TrimSpace(opts.Entry.PluginPath) == "" {
		return LocalCertificationReport{}, fmt.Errorf("local compressor certification requires exact identifier and plugin path")
	}
	if _, err := os.Stat(opts.Entry.PluginPath); err != nil {
		return LocalCertificationReport{}, fmt.Errorf("local compressor certification plugin path: %w", err)
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return LocalCertificationReport{}, fmt.Errorf("local compressor certification output directory is required")
	}
	if err := os.MkdirAll(opts.OutputDir, 0o700); err != nil {
		return LocalCertificationReport{}, fmt.Errorf("create certification output: %w", err)
	}
	if entries, err := os.ReadDir(opts.OutputDir); err != nil {
		return LocalCertificationReport{}, fmt.Errorf("inspect certification output: %w", err)
	} else if len(entries) != 0 {
		return LocalCertificationReport{}, fmt.Errorf("certification output directory must be empty: %s", opts.OutputDir)
	}

	base := strings.TrimRight(opts.AgentHTTP, "/")
	if _, err := health(opts.HTTPClient, base, opts.Timeout); err != nil {
		return LocalCertificationReport{}, err
	}
	caseID := slug(opts.Entry.Identifier)
	caseDir := filepath.Join(opts.OutputDir, "cases", "01_"+caseID)
	if err := os.MkdirAll(caseDir, 0o700); err != nil {
		return LocalCertificationReport{}, fmt.Errorf("create certification case: %w", err)
	}
	started := time.Now().UTC()
	evidence := map[string]any{
		"schema_version":    "processor_control_case_evidence.v1",
		"case":              map[string]any{"id": caseID, "plugin_name": opts.Entry.Name, "plugin_path": opts.Entry.PluginPath, "plugin_identifier": opts.Entry.Identifier},
		"frozen_resolution": map[string]any{"name": opts.Entry.Name, "manufacturer": opts.Entry.Manufacturer, "format": opts.Entry.Format, "identifier": opts.Entry.Identifier, "plugin_path": opts.Entry.PluginPath},
		"started_at":        started.Format(time.RFC3339Nano),
	}
	result := map[string]any{
		"id": caseID, "plugin_name": opts.Entry.Name, "identifier": opts.Entry.Identifier,
		"expectation": "compressor", "status": "failed", "temporary_track_deleted": false,
	}
	trackID := ""
	certErr := func() error {
		add, err := invoke(opts.HTTPClient, base, "track.add_audio", map[string]any{"name": "PCA v1 compressor certification " + caseID}, true, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["track_add_response"] = add
		trackID = firstTextMap(add.Result, "track_id", "id")
		if trackID == "" {
			return fmt.Errorf("track.add_audio omitted track_id")
		}
		loaded, err := invoke(opts.HTTPClient, base, "plugin.load_to_rack", map[string]any{"track_id": trackID, "plugin_path": opts.Entry.PluginPath, "plugin_name": opts.Entry.Name, "plugin_identifier": opts.Entry.Identifier}, true, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["plugin_load_response"] = loaded
		pluginID := firstTextMap(loaded.Result, "plugin_id", "node_id", "id")
		if pluginID == "" {
			return fmt.Errorf("plugin.load_to_rack omitted plugin_id")
		}
		result["plugin_id"] = pluginID
		before, err := invoke(opts.HTTPClient, base, "plugin.get_parameters", map[string]any{"track_id": trackID, "plugin_id": pluginID, "plugin_identifier": opts.Entry.Identifier, "include_parameters": true}, false, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["before_parameters_response"] = before
		beforeSnapshot := parameterSnapshot(before.Result)
		result["parameter_count"] = len(beforeSnapshot)
		inspect1, err := invoke(opts.HTTPClient, base, "plugin_grabber.inspect_compressor", map[string]any{"track_id": trackID, "plugin_id": pluginID}, false, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["inspect_response_1"] = inspect1
		inspect2, err := invoke(opts.HTTPClient, base, "plugin_grabber.inspect_compressor", map[string]any{"track_id": trackID, "plugin_id": pluginID}, false, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["inspect_response_2"] = inspect2
		gen1 := generation(inspect1.Result)
		gen2 := generation(inspect2.Result)
		if gen1 == "" || gen1 != gen2 {
			return fmt.Errorf("topology generation drifted: %q -> %q", gen1, gen2)
		}
		forward, restore, roles, err := selectReversibleControls(inspect2.Result)
		if err != nil {
			return err
		}
		evidence["selected_forward_controls"] = forward
		evidence["selected_restore_controls"] = restore
		applied, err := invoke(opts.HTTPClient, base, "plugin_grabber.apply_compressor_controls", map[string]any{"track_id": trackID, "plugin_id": pluginID, "atomic": true, "controls": forward}, true, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["apply_response"] = applied
		if len(asRows(applied.Result["controls"])) != len(forward) || !allReadback(asRows(applied.Result["controls"])) {
			return fmt.Errorf("typed apply omitted complete actual readback")
		}
		restored, err := invoke(opts.HTTPClient, base, "plugin_grabber.apply_compressor_controls", restoreArgs(trackID, pluginID, applied.Result, restore), true, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["restore_response"] = restored
		after, err := invoke(opts.HTTPClient, base, "plugin.get_parameters", map[string]any{"track_id": trackID, "plugin_id": pluginID, "plugin_identifier": opts.Entry.Identifier, "include_parameters": true}, false, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["after_parameters_response"] = after
		mismatches := snapshotMismatches(beforeSnapshot, parameterSnapshot(after.Result), opts.SnapshotTolerance)
		evidence["snapshot_mismatches"] = mismatches
		if len(mismatches) > 0 {
			return fmt.Errorf("complete parameter snapshot restore failed for %d parameters", len(mismatches))
		}
		result["status"] = "passed"
		result["topology_generation_stable"] = true
		result["selected_roles"] = roles
		result["apply_status"] = firstTextMap(applied.Result, "status")
		result["restore_status"] = firstTextMap(restored.Result, "status")
		result["write_count"] = len(asRows(applied.Result["writes"]))
		result["full_snapshot_restored"] = true
		return nil
	}()
	if certErr != nil {
		result["error"] = certErr.Error()
		evidence["exception"] = map[string]any{"message": certErr.Error()}
	}
	if trackID != "" {
		deleted, err := invoke(opts.HTTPClient, base, "track.delete", map[string]any{"track_id": trackID}, true, opts.Timeout)
		evidence["track_delete_response"] = deleted
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "last audio track") {
			guard, guardErr := invoke(opts.HTTPClient, base, "track.add_audio", map[string]any{"name": "PCA v1 headless cleanup guard"}, true, opts.Timeout)
			evidence["cleanup_guard_response"] = guard
			if guardErr == nil {
				deleted, err = invoke(opts.HTTPClient, base, "track.delete", map[string]any{"track_id": trackID}, true, opts.Timeout)
				evidence["track_delete_retry_response"] = deleted
			}
		}
		if err == nil && strings.EqualFold(deleted.Status, "ok") {
			result["temporary_track_deleted"] = true
		} else if certErr == nil && err != nil {
			certErr = err
		}
	}
	evidence["completed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	evidence["result"] = result
	evidencePath := filepath.Join(caseDir, "evidence.json")
	if err := writeJSONFile(evidencePath, evidence); err != nil {
		return LocalCertificationReport{}, err
	}
	result["temporary_track_deleted"] = result["temporary_track_deleted"] == true
	report := LocalCertificationReport{SchemaVersion: SchemaLocalCompressorCertification, Status: "passed", Verdict: "passed", CompletedAt: time.Now().UTC().Format(time.RFC3339Nano), Audit: map[string]any{"natural_language_chat_count": 0, "llm_call_count": 0}, Results: []map[string]any{result}, EvidencePath: evidencePath}
	if result["status"] != "passed" || result["temporary_track_deleted"] != true {
		report.Status, report.Verdict = "failed", "failed"
	}
	report.SummaryPath = filepath.Join(opts.OutputDir, "summary.json")
	if err := writeJSONFile(report.SummaryPath, report); err != nil {
		return LocalCertificationReport{}, err
	}
	if certErr != nil {
		return report, certErr
	}
	return report, nil
}

func health(client *http.Client, base string, timeout time.Duration) (map[string]any, error) {
	request, err := http.NewRequest(http.MethodGet, base+"/health", nil)
	if err != nil {
		return nil, err
	}
	ctx, cancel := timeAfter(timeout)
	defer cancel()
	request = request.WithContext(ctx)
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("agent health check: %w", err)
	}
	defer response.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return nil, err
	}
	if response.StatusCode >= 300 || !strings.EqualFold(firstTextMap(body, "status"), "ok") && !strings.EqualFold(firstTextMap(body, "status"), "ready") {
		return body, fmt.Errorf("agent health check failed: %v", body)
	}
	return body, nil
}

func invoke(client *http.Client, base, tool string, args map[string]any, confirmed bool, timeout time.Duration) (invokeResponse, error) {
	payload, _ := json.Marshal(map[string]any{"tool": tool, "args": args, "confirmed": confirmed, "source": "pcactl.certify_compressor"})
	request, err := http.NewRequest(http.MethodPost, base+"/agent/invoke", bytes.NewReader(payload))
	if err != nil {
		return invokeResponse{}, err
	}
	ctx, cancel := timeAfter(timeout)
	defer cancel()
	request = request.WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return invokeResponse{}, fmt.Errorf("invoke %s: %w", tool, err)
	}
	defer response.Body.Close()
	var body invokeResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return body, err
	}
	if response.StatusCode >= 300 || !strings.EqualFold(body.Status, "ok") {
		return body, fmt.Errorf("invoke %s failed: %s", tool, firstText(body.Error, body.Status))
	}
	return body, nil
}

func timeAfter(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func firstTextMap(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := row[key]; ok && value != nil && strings.TrimSpace(fmt.Sprint(value)) != "" {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}
func asRows(value any) []map[string]any {
	rows, _ := value.([]any)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if typed, ok := row.(map[string]any); ok {
			out = append(out, typed)
		}
	}
	return out
}
func allReadback(rows []map[string]any) bool {
	for _, row := range rows {
		value, ok := row["actual_readback"]
		if !ok || value == nil {
			return false
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			return false
		}
	}
	return len(rows) > 0
}
func generation(result map[string]any) string {
	if topology, ok := result["control_topology"].(map[string]any); ok {
		if value := firstTextMap(topology, "generation"); value != "" {
			return value
		}
	}
	if stage, ok := result["compressor_stage"].(map[string]any); ok {
		return firstTextMap(stage, "generation", "topology_generation")
	}
	return ""
}
func parameterSnapshot(result map[string]any) map[string]float64 {
	out := map[string]float64{}
	for _, row := range asRows(result["parameters"]) {
		id := firstTextMap(row, "param_id", "id")
		value, ok := row["normalized_value"].(float64)
		if id != "" && ok && math.IsNaN(value) == false && math.IsInf(value, 0) == false {
			out[id] = value
		}
	}
	return out
}
func snapshotMismatches(before, after map[string]float64, tolerance float64) []map[string]any {
	keys := map[string]bool{}
	for key := range before {
		keys[key] = true
	}
	for key := range after {
		keys[key] = true
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	out := []map[string]any{}
	for _, key := range ordered {
		left, lok := before[key]
		right, rok := after[key]
		if !lok || !rok || math.Abs(left-right) > tolerance {
			out = append(out, map[string]any{"param_id": key, "before": left, "after": right})
		}
	}
	return out
}
func slug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if b.Len() > 0 {
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func selectReversibleControls(result map[string]any) ([]map[string]any, []map[string]any, []string, error) {
	priorities := []string{"threshold", "input_drive", "reduction_amount", "low_level_amount", "high_level_amount", "ratio", "direction_curve", "attack", "release", "makeup_gain", "mix"}
	bindings := []map[string]any{}
	stage, _ := result["compressor_stage"].(map[string]any)
	for _, path := range asRows(stage["control_paths"]) {
		for _, section := range []string{"detector", "operating_point", "transfer", "timing", "gain_action"} {
			bindings = append(bindings, asRows(path[section])...)
		}
	}
	bindings = append(bindings, asRows(stage["output"])...)
	forward, restore, roles := []map[string]any{}, []map[string]any{}, []string{}
	used := map[string]bool{}
	for _, role := range priorities {
		for _, binding := range bindings {
			if firstTextMap(binding, "role") != role {
				continue
			}
			paramID := firstTextMap(binding, "param_id")
			if paramID == "" || used[paramID] {
				continue
			}
			controlRef := firstTextMap(binding, "control_ref")
			if controlRef == "" {
				continue
			}
			domain, _ := binding["domain"].(map[string]any)
			unit := strings.ToLower(firstTextMap(domain, "unit"))
			if unit == "enum" || unit == "toggle" {
				current := firstTextMap(binding, "current_text")
				labels := []string{}
				for _, row := range asRows(binding["reachable_values"]) {
					if label := firstTextMap(row, "label"); label != "" {
						labels = append(labels, label)
					}
				}
				alternate := ""
				for _, label := range labels {
					if !strings.EqualFold(label, current) {
						alternate = label
						break
					}
				}
				if current == "" || alternate == "" {
					continue
				}
				forward = append(forward, map[string]any{"control_ref": controlRef, "enum_label": alternate})
				restore = append(restore, map[string]any{"control_ref": controlRef, "enum_label": current})
			} else {
				current, ok := binding["current_physical"].(float64)
				if !ok || math.IsNaN(current) || math.IsInf(current, 0) {
					continue
				}
				alternate, ok := alternatePhysical(binding, current)
				if !ok {
					continue
				}
				forward = append(forward, map[string]any{"control_ref": controlRef, "display_value": alternate})
				restore = append(restore, map[string]any{"control_ref": controlRef, "display_value": current})
			}
			used[paramID] = true
			roles = append(roles, role)
			break
		}
		if len(forward) >= 2 {
			break
		}
	}
	if len(forward) == 0 {
		return nil, nil, nil, fmt.Errorf("no preferred compressor binding had a reversible measured target")
	}
	return forward, restore, roles, nil
}
func alternatePhysical(binding map[string]any, current float64) (float64, bool) {
	candidates := []float64{}
	for _, point := range asRows(binding["reachable_values"]) {
		if value, ok := point["physical"].(float64); ok && math.Abs(value-current) > 1e-4 {
			candidates = append(candidates, value)
		}
	}
	if domain, ok := binding["domain"].(map[string]any); ok {
		confidence, _ := domain["confidence"].(float64)
		if confidence >= 0.8 {
			for _, key := range []string{"min", "max"} {
				if value, ok := domain[key].(float64); ok && math.Abs(value-current) > 1e-4 {
					candidates = append(candidates, value)
				}
			}
		}
	}
	if len(candidates) == 0 {
		return 0, false
	}
	best := candidates[0]
	for _, value := range candidates[1:] {
		if math.Abs(value-current) > math.Abs(best-current) {
			best = value
		}
	}
	return best, true
}
func restoreArgs(trackID, pluginID string, result map[string]any, restore []map[string]any) map[string]any {
	args := map[string]any{"track_id": trackID, "plugin_id": pluginID, "atomic": true}
	if ref := firstTextMap(result, "restore_ref"); ref != "" {
		args["restore_ref"] = ref
	} else {
		args["controls"] = restore
	}
	return args
}

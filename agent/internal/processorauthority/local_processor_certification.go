package processorauthority

import (
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/pluginsemantics"
	"vit-daw-agent/internal/processorattestation"
)

const SchemaLocalProcessorCertificationV2 = "processor_authority.local_processor_certification.v2"

type LocalProcessorCertificationOptions struct {
	AgentHTTP              string
	Entry                  pluginsemantics.Entry
	OutputDir              string
	ProcessorFamily        string
	InspectTool            string
	ApplyTool              string
	Timeout                time.Duration
	SnapshotTolerance      float64
	HTTPClient             *http.Client
	Progress               func(stage string)
	LoadAuthorizationToken string
}

// CertifyProcessor runs the same disposable-track, two-inspection,
// typed-apply/readback/restore and full-snapshot cleanup gates as compressor
// certification, but selects controls from any v2 typed topology.
func CertifyProcessor(opts LocalProcessorCertificationOptions) (LocalCertificationReport, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 180 * time.Second
	}
	if opts.SnapshotTolerance <= 0 {
		opts.SnapshotTolerance = 0.0001
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = &http.Client{Timeout: opts.Timeout}
	}
	if strings.TrimSpace(opts.AgentHTTP) == "" {
		opts.AgentHTTP = "http://127.0.0.1:7878"
	}
	opts.ProcessorFamily = strings.ToLower(strings.TrimSpace(opts.ProcessorFamily))
	if !processorattestation.IsV2Family(opts.ProcessorFamily) {
		return LocalCertificationReport{}, fmt.Errorf("local processor certification requires a PCA v2 family")
	}
	if err := validateV2CertificationTools(opts.ProcessorFamily, opts.InspectTool, opts.ApplyTool); err != nil {
		return LocalCertificationReport{}, err
	}
	if strings.TrimSpace(opts.Entry.Identifier) == "" || strings.TrimSpace(opts.Entry.PluginPath) == "" {
		return LocalCertificationReport{}, fmt.Errorf("local processor certification requires exact identifier and plugin path")
	}
	if _, err := os.Stat(opts.Entry.PluginPath); err != nil {
		return LocalCertificationReport{}, fmt.Errorf("local processor certification plugin path: %w", err)
	}
	if strings.TrimSpace(opts.OutputDir) == "" {
		return LocalCertificationReport{}, fmt.Errorf("local processor certification output directory is required")
	}
	if err := os.MkdirAll(opts.OutputDir, 0o700); err != nil {
		return LocalCertificationReport{}, fmt.Errorf("create certification output: %w", err)
	}
	if entries, err := os.ReadDir(opts.OutputDir); err != nil {
		return LocalCertificationReport{}, fmt.Errorf("inspect certification output: %w", err)
	} else if len(entries) != 0 {
		return LocalCertificationReport{}, fmt.Errorf("certification output directory must be empty: %s", opts.OutputDir)
	}
	if strings.TrimSpace(opts.InspectTool) == "" || strings.TrimSpace(opts.ApplyTool) == "" {
		return LocalCertificationReport{}, fmt.Errorf("local processor certification requires inspect and apply tools")
	}
	base := strings.TrimRight(opts.AgentHTTP, "/")
	reportProcessorProgress(opts.Progress, "health_check")
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
		"schema_version":    SchemaLocalProcessorCertificationV2,
		"case":              map[string]any{"id": caseID, "plugin_name": opts.Entry.Name, "plugin_path": opts.Entry.PluginPath, "plugin_identifier": opts.Entry.Identifier},
		"frozen_resolution": map[string]any{"name": opts.Entry.Name, "manufacturer": opts.Entry.Manufacturer, "format": opts.Entry.Format, "identifier": opts.Entry.Identifier, "plugin_path": opts.Entry.PluginPath},
		"processor_family":  opts.ProcessorFamily, "inspect_tool": opts.InspectTool, "apply_tool": opts.ApplyTool, "started_at": started.Format(time.RFC3339Nano),
	}
	result := map[string]any{"id": caseID, "plugin_name": opts.Entry.Name, "identifier": opts.Entry.Identifier, "expectation": opts.ProcessorFamily, "processor_family": opts.ProcessorFamily, "status": "failed", "temporary_track_deleted": false}
	trackID := ""
	certErr := func() error {
		reportProcessorProgress(opts.Progress, "create_temporary_track")
		add, err := invokeProcessor(opts.HTTPClient, base, "track.add_audio", map[string]any{"name": "PCA v2 " + opts.ProcessorFamily + " certification " + caseID}, true, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["track_add_response"] = add
		trackID = firstTextMap(add.Result, "track_id", "id")
		if trackID == "" {
			return fmt.Errorf("track.add_audio omitted track_id")
		}
		reportProcessorProgress(opts.Progress, "load_plugin")
		loaded, err := invokeAuthorized(opts.HTTPClient, base, "plugin.load_to_rack", map[string]any{"track_id": trackID, "plugin_path": opts.Entry.PluginPath, "plugin_name": opts.Entry.Name, "plugin_identifier": opts.Entry.Identifier}, true, opts.Timeout, "pcactl.certify_processor", opts.LoadAuthorizationToken)
		if err != nil {
			return err
		}
		evidence["plugin_load_response"] = loaded
		pluginID := firstTextMap(loaded.Result, "plugin_id", "node_id", "id")
		if pluginID == "" {
			return fmt.Errorf("plugin.load_to_rack omitted plugin_id")
		}
		result["plugin_id"] = pluginID
		reportProcessorProgress(opts.Progress, "snapshot_parameters")
		before, err := invokeProcessor(opts.HTTPClient, base, "plugin.get_parameters", map[string]any{"track_id": trackID, "plugin_id": pluginID, "plugin_identifier": opts.Entry.Identifier, "include_parameters": true}, false, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["before_parameters_response"] = before
		beforeSnapshot := parameterSnapshot(before.Result)
		result["parameter_count"] = len(beforeSnapshot)
		reportProcessorProgress(opts.Progress, "inspect_topology")
		inspect1, err := invokeProcessor(opts.HTTPClient, base, opts.InspectTool, map[string]any{"track_id": trackID, "plugin_id": pluginID}, false, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["inspect_response_1"] = inspect1
		inspect2, err := invokeProcessor(opts.HTTPClient, base, opts.InspectTool, map[string]any{"track_id": trackID, "plugin_id": pluginID}, false, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["inspect_response_2"] = inspect2
		gen1, gen2 := genericGeneration(inspect1.Result), genericGeneration(inspect2.Result)
		if gen1 == "" || gen1 != gen2 {
			return fmt.Errorf("topology generation drifted: %q -> %q", gen1, gen2)
		}
		forward, restore, roles, err := selectV2ReversibleControls(inspect2.Result, opts.ProcessorFamily)
		if err != nil {
			return err
		}
		evidence["selected_forward_controls"] = forward
		evidence["selected_restore_controls"] = restore
		reportProcessorProgress(opts.Progress, "typed_apply")
		applied, err := invokeProcessor(opts.HTTPClient, base, opts.ApplyTool, map[string]any{"track_id": trackID, "plugin_id": pluginID, "atomic": true, "controls": forward}, true, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["apply_response"] = applied
		applyStatus := firstTextMap(applied.Result, "status")
		if !successful(applyStatus) {
			return fmt.Errorf("typed apply returned unsuccessful status %q", applyStatus)
		}
		if len(asRows(applied.Result["controls"])) != len(forward) || !allReadback(asRows(applied.Result["controls"])) {
			return fmt.Errorf("typed apply omitted complete actual readback")
		}
		reportProcessorProgress(opts.Progress, "typed_readback")
		reportProcessorProgress(opts.Progress, "typed_restore")
		restored, err := invokeProcessor(opts.HTTPClient, base, opts.ApplyTool, restoreArgs(trackID, pluginID, applied.Result, restore), true, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["restore_response"] = restored
		restoreStatus := firstTextMap(restored.Result, "status")
		if !successful(restoreStatus) {
			return fmt.Errorf("typed restore returned unsuccessful status %q", restoreStatus)
		}
		reportProcessorProgress(opts.Progress, "verify_full_snapshot")
		after, err := invokeProcessor(opts.HTTPClient, base, "plugin.get_parameters", map[string]any{"track_id": trackID, "plugin_id": pluginID, "plugin_identifier": opts.Entry.Identifier, "include_parameters": true}, false, opts.Timeout)
		if err != nil {
			return err
		}
		evidence["after_parameters_response"] = after
		mismatches := snapshotMismatches(beforeSnapshot, parameterSnapshot(after.Result), opts.SnapshotTolerance)
		evidence["snapshot_mismatches"] = mismatches
		if len(mismatches) > 0 {
			return fmt.Errorf("complete parameter snapshot restore failed for %d parameters", len(mismatches))
		}
		result["status"], result["topology_generation_stable"], result["selected_roles"] = "passed", true, roles
		result["apply_status"], result["restore_status"] = firstTextMap(applied.Result, "status"), firstTextMap(restored.Result, "status")
		result["write_count"], result["full_snapshot_restored"] = len(asRows(applied.Result["writes"])), true
		return nil
	}()
	if certErr != nil {
		result["error"] = certErr.Error()
		evidence["exception"] = map[string]any{"message": certErr.Error()}
	}
	if trackID != "" {
		reportProcessorProgress(opts.Progress, "cleanup_temporary_track")
		deleted, err := invokeProcessor(opts.HTTPClient, base, "track.delete", map[string]any{"track_id": trackID}, true, opts.Timeout)
		evidence["track_delete_response"] = deleted
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "last audio track") {
			guard, guardErr := invokeProcessor(opts.HTTPClient, base, "track.add_audio", map[string]any{"name": "PCA v2 headless cleanup guard"}, true, opts.Timeout)
			evidence["cleanup_guard_response"] = guard
			if guardErr == nil {
				deleted, err = invokeProcessor(opts.HTTPClient, base, "track.delete", map[string]any{"track_id": trackID}, true, opts.Timeout)
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
	report := LocalCertificationReport{SchemaVersion: SchemaLocalProcessorCertificationV2, Status: "passed", Verdict: "passed", CompletedAt: time.Now().UTC().Format(time.RFC3339Nano), Audit: map[string]any{"natural_language_chat_count": 0, "llm_call_count": 0}, Results: []map[string]any{result}, EvidencePath: evidencePath}
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

func reportProcessorProgress(progress func(string), stage string) {
	if progress != nil {
		progress(stage)
	}
}

func validateV2CertificationTools(family, inspectTool, applyTool string) error {
	pair, ok := map[string][2]string{
		processorattestation.FamilyLimiter:      {"plugin_grabber.inspect_limiter", "plugin_grabber.apply_limiter_controls"},
		processorattestation.FamilyGateExpander: {"plugin_grabber.inspect_gate_expander", "plugin_grabber.apply_gate_expander_controls"},
		processorattestation.FamilyDeEsser:      {"plugin_grabber.inspect_de_esser", "plugin_grabber.apply_de_esser_controls"},
		processorattestation.FamilyTransient:    {"plugin_grabber.inspect_transient_shaper", "plugin_grabber.apply_transient_shaper_controls"},
		processorattestation.FamilyMultiband:    {"plugin_grabber.inspect_multiband", "plugin_grabber.apply_multiband_controls"},
	}[family]
	if !ok {
		return fmt.Errorf("PCA v2 certification tools do not match family %s", family)
	}
	wantInspect, wantApply := pair[0], pair[1]
	if strings.TrimSpace(inspectTool) != wantInspect || strings.TrimSpace(applyTool) != wantApply {
		return fmt.Errorf("PCA v2 certification tools do not match family %s", family)
	}
	return nil
}

func genericGeneration(result map[string]any) string {
	if topology, ok := result["control_topology"].(map[string]any); ok {
		if value := firstTextMap(topology, "generation"); value != "" {
			return value
		}
	}
	if value := firstTextMap(result, "topology_generation", "generation"); value != "" {
		return value
	}
	var found string
	var visit func(any)
	visit = func(value any) {
		if found != "" {
			return
		}
		switch typed := value.(type) {
		case map[string]any:
			for _, key := range []string{"control_topology", "topology"} {
				if row, ok := typed[key].(map[string]any); ok {
					if gen := firstTextMap(row, "generation", "topology_generation"); gen != "" {
						found = gen
						return
					}
				}
			}
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				visit(typed[key])
			}
		case []any:
			for _, item := range typed {
				visit(item)
			}
		}
	}
	visit(result)
	return found
}

func selectV2ReversibleControls(result map[string]any, family string) ([]map[string]any, []map[string]any, []string, error) {
	type binding struct {
		row       map[string]any
		role, ref string
	}
	bindings := []binding{}
	var visit func(any)
	visit = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			ref := firstTextMap(typed, "control_ref")
			role := firstTextMap(typed, "role")
			if ref != "" && role != "" {
				bindings = append(bindings, binding{typed, role, ref})
			}
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				visit(typed[key])
			}
		case []any:
			for _, item := range typed {
				visit(item)
			}
		}
	}
	visit(result)
	usedAxis := map[string]bool{}
	usedParam := map[string]bool{}
	forward, restore, roles := []map[string]any{}, []map[string]any{}, []string{}
	for _, item := range bindings {
		axisList := processorattestation.V2CoverageForRoles(family, []string{item.role})
		if len(axisList) == 0 || usedAxis[axisList[0].Axis] || usedParam[firstTextMap(item.row, "param_id")] {
			continue
		}
		domain, _ := item.row["domain"].(map[string]any)
		unit := strings.ToLower(firstTextMap(domain, "unit"))
		currentText := firstTextMap(item.row, "current_text", "value_text")
		labels := []string{}
		for _, row := range asRows(item.row["reachable_values"]) {
			if label := firstTextMap(row, "label"); label != "" {
				labels = append(labels, label)
			}
		}
		if unit == "enum" || unit == "toggle" || len(labels) > 0 && firstTextMap(item.row, "current_physical") == "" {
			alternate := ""
			for _, label := range labels {
				if currentText == "" || !strings.EqualFold(label, currentText) {
					alternate = label
					break
				}
			}
			if currentText == "" || alternate == "" {
				continue
			}
			forward = append(forward, map[string]any{"control_ref": item.ref, "enum_label": alternate})
			restore = append(restore, map[string]any{"control_ref": item.ref, "enum_label": currentText})
		} else {
			current, ok := item.row["current_physical"].(float64)
			if !ok || math.IsNaN(current) || math.IsInf(current, 0) {
				continue
			}
			alternate, ok := alternatePhysical(item.row, current)
			if !ok {
				continue
			}
			forward = append(forward, map[string]any{"control_ref": item.ref, "display_value": alternate})
			restore = append(restore, map[string]any{"control_ref": item.ref, "display_value": current})
		}
		usedAxis[axisList[0].Axis], usedParam[firstTextMap(item.row, "param_id")] = true, true
		roles = append(roles, item.role)
		if len(forward) >= 2 {
			break
		}
	}
	if len(forward) == 0 {
		return nil, nil, nil, fmt.Errorf("no preferred v2 binding had a reversible measured target")
	}
	return forward, restore, roles, nil
}

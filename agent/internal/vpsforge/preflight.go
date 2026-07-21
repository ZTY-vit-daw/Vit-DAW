package vpsforge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/probeaudio"
	"vit-daw-agent/internal/vps"
)

const preflightSchema = "vit.vpsforge.preflight.v1"

// PreflightRequest describes a staging-only evidence run. Candidate badges are
// supplied by an authoring plan rather than inferred from a plugin name; they
// are output only as agent-inferred candidates and can never issue a
// Credential.
type PreflightRequest struct {
	Root              string
	HostURL           string
	ProbeAudioRoot    string
	CandidateBadge    string
	Category          string
	Task              string
	RequiredFeatures  []string
	RunParameterProbe bool
	RunFXMBaseline    bool
	Client            *http.Client
	Now               time.Time
}

type PreflightResult struct {
	Status    Status            `json:"status"`
	Workspace string            `json:"workspace"`
	Artifacts map[string]string `json:"artifacts"`
	Warnings  []string          `json:"warnings,omitempty"`
}

type adapterClient struct {
	baseURL string
	client  *http.Client
}

func newAdapterClient(baseURL string, client *http.Client) (*adapterClient, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" {
		return nil, fmt.Errorf("host adapter URL must be an http URL")
	}
	host := parsed.Hostname()
	if !strings.EqualFold(host, "localhost") {
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return nil, fmt.Errorf("host adapter URL must target a loopback address")
		}
	}
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	return &adapterClient{baseURL: baseURL, client: client}, nil
}

func (c *adapterClient) snapshot(ctx context.Context) (HostSnapshot, error) {
	data, err := c.request(ctx, http.MethodGet, "/v1/plugin/snapshot", nil)
	if err != nil {
		return HostSnapshot{}, err
	}
	var snapshot HostSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return HostSnapshot{}, fmt.Errorf("decode /v1/plugin/snapshot: %w", err)
	}
	if snapshot.Surface.SchemaVersion == "" {
		snapshot.Surface.SchemaVersion = SurfaceSchema
	}
	if snapshot.Surface.PluginIdentity.Name == "" {
		snapshot.Surface.PluginIdentity = snapshot.Identity
	}
	return snapshot, nil
}

func (c *adapterClient) operation(ctx context.Context, path string, payload any) (json.RawMessage, []string, error) {
	data, err := c.request(ctx, http.MethodPost, path, payload)
	if err != nil {
		return nil, nil, err
	}
	var envelope struct {
		Status string          `json:"status"`
		Result json.RawMessage `json:"result"`
		Logs   []string        `json:"logs"`
		Error  string          `json:"error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, nil, fmt.Errorf("decode adapter operation response: %w", err)
	}
	if strings.TrimSpace(envelope.Error) != "" {
		return nil, nil, fmt.Errorf("adapter operation failed: %s", envelope.Error)
	}
	return envelope.Result, envelope.Logs, nil
}

func (c *adapterClient) request(ctx context.Context, method, path string, payload any) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var issue struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &issue)
		return nil, fmt.Errorf("host adapter %s %s returned %s: %s", method, path, response.Status, firstNonEmpty(issue.Error, strings.TrimSpace(string(data))))
	}
	return data, nil
}

type preflightParameterProbe struct {
	SchemaVersion            string          `json:"schema_version"`
	Trust                    string          `json:"trust"`
	Status                   string          `json:"status"`
	Parameter                map[string]any  `json:"parameter,omitempty"`
	WriteResult              json.RawMessage `json:"write_result,omitempty"`
	StateRoundtripAfterWrite json.RawMessage `json:"state_roundtrip_after_write,omitempty"`
	RollbackResult           json.RawMessage `json:"rollback_result,omitempty"`
	FreshSnapshot            json.RawMessage `json:"fresh_snapshot,omitempty"`
	Error                    string          `json:"error,omitempty"`
	CapturedAt               time.Time       `json:"captured_at"`
}

type stateRoundtripResults struct {
	SchemaVersion   string          `json:"schema_version"`
	Trust           string          `json:"trust"`
	Status          string          `json:"status"`
	SaveResult      json.RawMessage `json:"save_result,omitempty"`
	RoundtripResult json.RawMessage `json:"roundtrip_result,omitempty"`
	Error           string          `json:"error,omitempty"`
	CapturedAt      time.Time       `json:"captured_at"`
}

// Preflight creates an isolated workspace and completes only the evidence that
// can be acquired by the native host. It never promotes observed data to
// conformed, never writes a canonical library, and never asks the user for
// semantic testimony.
func Preflight(ctx context.Context, request PreflightRequest) (PreflightResult, error) {
	root := filepath.Clean(strings.TrimSpace(request.Root))
	if root == "" || root == "." {
		return PreflightResult{}, fmt.Errorf("preflight workspace root is required")
	}
	if _, err := os.Stat(filepath.Join(root, manifestFile)); err == nil {
		return PreflightResult{}, fmt.Errorf("preflight workspace already exists: %s", root)
	}
	adapter, err := newAdapterClient(request.HostURL, request.Client)
	if err != nil {
		return PreflightResult{}, err
	}
	now := nowOrCurrent(request.Now)
	snapshot, err := adapter.snapshot(ctx)
	if err != nil {
		return PreflightResult{}, fmt.Errorf("capture native host snapshot: %w", err)
	}
	if strings.TrimSpace(snapshot.Identity.Name) == "" {
		return PreflightResult{}, fmt.Errorf("native host snapshot has no plugin identity")
	}
	badge := strings.TrimSpace(request.CandidateBadge)
	if badge == "" {
		badge = "unknown"
	}
	status, err := Init(InitRequest{
		Root:         root,
		Identity:     snapshot.Identity,
		Capabilities: []string{badge},
		HostProtocol: VST3HostAdapterProtocol,
		HostEndpoint: adapter.baseURL,
		Now:          now,
	})
	if err != nil {
		return PreflightResult{}, err
	}
	status, err = IngestSurface(root, snapshot.Surface, adapter.baseURL, now)
	if err != nil {
		return PreflightResult{}, err
	}
	warnings := []string{}

	identityArtifact := map[string]any{
		"schema_version": preflightSchema,
		"trust":          "observed", "captured_at": now,
		"plugin_identity":  snapshot.Identity,
		"host_protocol":    VST3HostAdapterProtocol,
		"host_endpoint":    adapter.baseURL,
		"adapter_snapshot": json.RawMessage(snapshot.AdapterSnapshot),
	}
	if err := writeJSON(filepath.Join(root, pluginIdentityFile), identityArtifact); err != nil {
		return PreflightResult{}, err
	}
	identityData, _ := json.Marshal(identityArtifact)
	if err := appendEvidence(root, EvidenceEntry{Kind: "plugin_identity", Trust: "observed", Source: adapter.baseURL, Summary: "Captured VST3 identity, class IDs, file fingerprint and host surface.", Artifact: pluginIdentityFile, CapturedAt: now, Data: identityData}); err != nil {
		return PreflightResult{}, err
	}

	parameterProbe := executeParameterProbe(ctx, adapter, snapshot, now, request.RunParameterProbe)
	if parameterProbe.Error != "" {
		warnings = append(warnings, "parameter probe: "+parameterProbe.Error)
	}
	if err := writeJSON(filepath.Join(root, parameterProbeFile), parameterProbe); err != nil {
		return PreflightResult{}, err
	}
	probeData, _ := json.Marshal(parameterProbe)
	if err := appendEvidence(root, EvidenceEntry{Kind: "parameter_probe", Trust: "observed", Source: adapter.baseURL, Summary: "Recorded an isolated host parameter write/readback/rollback probe without semantic interpretation.", Artifact: parameterProbeFile, CapturedAt: now, Data: probeData}); err != nil {
		return PreflightResult{}, err
	}

	stateResult := executeStateRoundtrip(ctx, adapter, now)
	if stateResult.Error != "" {
		warnings = append(warnings, "state roundtrip: "+stateResult.Error)
	}
	if err := writeJSON(filepath.Join(root, stateRoundtripFile), stateResult); err != nil {
		return PreflightResult{}, err
	}
	stateData, _ := json.Marshal(stateResult)
	if err := appendEvidence(root, EvidenceEntry{Kind: "state_roundtrip", Trust: "observed", Source: adapter.baseURL, Summary: "Recorded VST3 state serialization, reload and fresh parameter readback result.", Artifact: stateRoundtripFile, CapturedAt: now, Data: stateData}); err != nil {
		return PreflightResult{}, err
	}

	fxmBaseline := executeDefaultFXMBaseline(ctx, adapter, root, request.ProbeAudioRoot, now, request.RunFXMBaseline)
	if message, ok := fxmBaseline["error"].(string); ok && message != "" {
		warnings = append(warnings, "FXM default baseline: "+message)
	}
	if err := writeJSON(filepath.Join(root, fxmBaselineFile), fxmBaseline); err != nil {
		return PreflightResult{}, err
	}
	fxmData, _ := json.Marshal(fxmBaseline)
	if err := appendEvidence(root, EvidenceEntry{Kind: "fxm_default_baseline", Trust: "observed", Source: adapter.baseURL, Summary: "Captured matched default-state bypass and processed probe renders; this is not a semantic or musical-quality conclusion.", Artifact: fxmBaselineFile, CapturedAt: now, Data: fxmData}); err != nil {
		return PreflightResult{}, err
	}

	groups := buildInferredParameterGroups(snapshot, now)
	if err := writeJSON(filepath.Join(root, groupsFile), groups); err != nil {
		return PreflightResult{}, err
	}
	groupsData, _ := json.Marshal(groups)
	if err := appendEvidence(root, EvidenceEntry{Kind: "parameter_group_inference", Trust: "agent-inferred", Source: "vpsforge.preflight", Summary: "Grouped host labels structurally for review; groups are not executable semantic mappings.", Artifact: groupsFile, CapturedAt: now, Data: groupsData}); err != nil {
		return PreflightResult{}, err
	}

	candidates := buildCandidateBadges(request, snapshot, now)
	if err := writeJSON(filepath.Join(root, candidateBadgesFile), candidates); err != nil {
		return PreflightResult{}, err
	}
	badgeContract := buildBadgeActionContractArtifact(badge, now)
	if err := writeJSON(filepath.Join(root, badgeActionContractFile), badgeContract); err != nil {
		return PreflightResult{}, err
	}
	features := buildFeatureGaps(request, now)
	if err := writeJSON(filepath.Join(root, featureGapsFile), features); err != nil {
		return PreflightResult{}, err
	}
	witness := buildWitnessRounds(request, now)
	if err := writeJSON(filepath.Join(root, witnessRoundsFile), witness); err != nil {
		return PreflightResult{}, err
	}
	humanGaps := buildHumanEvidenceGaps(request, parameterProbe, stateResult, now)
	if err := writeJSON(filepath.Join(root, humanGapsFile), humanGaps); err != nil {
		return PreflightResult{}, err
	}
	skeleton := buildConformanceSkeleton(request, parameterProbe, stateResult, now)
	if err := writeJSON(filepath.Join(root, conformanceFile), skeleton); err != nil {
		return PreflightResult{}, err
	}
	badgeSkeleton := buildBadgeConformanceSkeleton(request, parameterProbe, stateResult, now)
	if err := writeJSON(filepath.Join(root, badgeSkeletonFile), badgeSkeleton); err != nil {
		return PreflightResult{}, err
	}
	for _, item := range []struct {
		kind, trust, summary, artifact string
		data                           any
	}{
		{"candidate_task_badges", "agent-inferred", "Prepared candidate task badges; none are granted or routeable.", candidateBadgesFile, candidates},
		{"badge_action_contract", "agent-inferred", "Prepared the fixed badge action grammar and audit table; it is not a conformance decision or Credential.", badgeActionContractFile, badgeContract},
		{"required_feature_gaps", "agent-inferred", "Prepared required-feature review gaps; no feature is conformed by preflight.", featureGapsFile, features},
		{"human_evidence_gaps", "unsupported/unknown", "Prepared the minimum planned human witness rounds without requesting testimony.", humanGapsFile, humanGaps},
		{"badge_conformance_skeleton", "unsupported/unknown", "Prepared a non-authoritative conformance skeleton; it cannot issue a Credential.", badgeSkeletonFile, badgeSkeleton},
		{"conformance_skeleton", "unsupported/unknown", "Prepared overall preflight conformance gaps and observed machine results.", conformanceFile, skeleton},
	} {
		data, _ := json.Marshal(item.data)
		if err := appendEvidence(root, EvidenceEntry{Kind: item.kind, Trust: item.trust, Source: "vpsforge.preflight", Summary: item.summary, Artifact: item.artifact, CapturedAt: now, Data: data}); err != nil {
			return PreflightResult{}, err
		}
	}
	if err := updateManifest(root, func(manifest *Manifest) {
		manifest.Status = "preflight_complete"
		manifest.HostAdapter.Protocol = VST3HostAdapterProtocol
		manifest.HostAdapter.Endpoint = adapter.baseURL
		manifest.HostAdapter.Status = "observed"
		manifest.UpdatedAt = now
	}); err != nil {
		return PreflightResult{}, err
	}
	status, err = Inspect(root)
	if err != nil {
		return PreflightResult{}, err
	}
	return PreflightResult{Status: status, Workspace: root, Artifacts: status.Manifest.Artifacts, Warnings: uniqueSorted(warnings)}, nil
}

func buildBadgeActionContractArtifact(badge string, now time.Time) map[string]any {
	artifact := map[string]any{
		"schema_version": "vit.vpsforge.badge_action_contract.v1",
		"trust":          "agent-inferred",
		"captured_at":    now,
		"badge_id":       badge,
		"status":         "unknown_badge_contract",
		"authority_boundary": []string{
			"This artifact supplies a fixed authoring grammar and audit sequence; it does not grant a Credential, Catalog entry, SPAL route or conformance.",
			"A VPS must separately provide action implementations and a feature matrix backed by the required audit evidence.",
		},
	}
	if contract, found := vps.BadgeContractFor(badge); found {
		artifact["status"] = "authoring_contract_available_not_conformed"
		artifact["contract"] = contract
	}
	return artifact
}

func executeParameterProbe(ctx context.Context, adapter *adapterClient, snapshot HostSnapshot, now time.Time, enabled bool) preflightParameterProbe {
	result := preflightParameterProbe{SchemaVersion: preflightSchema, Trust: "observed", Status: "unsupported/unknown", CapturedAt: now}
	if !enabled {
		result.Error = "parameter probe disabled"
		return result
	}
	parameter, target, ok := selectNeutralParameterProbe(snapshot.Surface.Parameters)
	if !ok {
		result.Error = "no host-controllable finite normalized parameter was available for a neutral probe"
		return result
	}
	result.Parameter = map[string]any{"id": parameter.ID, "host_label": parameter.Name, "preimage_normalized": *parameter.NormalizedValue, "requested_normalized": target, "selection_basis": "stable parameter ID order and host-controllable flag only; not parameter semantics"}
	write, _, err := adapter.operation(ctx, "/v1/plugin/parameters/write", map[string]any{"changes": []map[string]any{{"id": parameter.ID, "normalized": target}}})
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.WriteResult = write
	transactionID := stringJSONField(write, "transaction_id")
	if transactionID == "" {
		result.Error = "host write result omitted transaction_id; refusing to claim rollback"
		return result
	}
	// The write response is a fresh raw worker snapshot. Confirm that the
	// selected observed control actually reached the requested normalized value
	// before asking whether the resulting state can survive a reload.
	probeError := ""
	if !freshReadbackMatchesJSON(write, parameter.ID, target) {
		probeError = "fresh readback after parameter write does not match the requested normalized value"
	}

	// State persistence is tested while the controlled write is still active.
	// This catches controller-to-processor synchronization gaps that a
	// default-only state roundtrip cannot observe. Rollback below still runs
	// even if this evidence is incomplete or fails.
	roundtrip, _, roundtripErr := adapter.operation(ctx, "/v1/plugin/state/roundtrip", map[string]any{})
	result.StateRoundtripAfterWrite = roundtrip
	if roundtripErr != nil {
		probeError = firstNonEmpty(probeError, "state roundtrip after parameter write: "+roundtripErr.Error())
	} else if !boolJSONField(roundtrip, "parameter_readback_matches_preimage") {
		probeError = firstNonEmpty(probeError, "state reload after parameter write completed but fresh parameter readback did not match its preimage")
	}

	rollback, _, rollbackErr := adapter.operation(ctx, "/v1/plugin/rollback", map[string]any{"transaction_id": transactionID})
	result.RollbackResult = rollback
	if rollbackErr != nil {
		result.Error = rollbackErr.Error()
		return result
	}
	fresh, freshErr := adapter.snapshot(ctx)
	if freshErr == nil {
		result.FreshSnapshot = append(json.RawMessage(nil), fresh.AdapterSnapshot...)
	}
	if freshErr != nil {
		result.Error = freshErr.Error()
		return result
	}
	if !boolJSONField(rollback, "rollback_verified") {
		result.Error = "host rollback returned rollback_verified=false"
		return result
	}
	if restored, ok := findSurfaceParameter(fresh.Surface.Parameters, parameter.ID); !ok || restored.NormalizedValue == nil || math.Abs(*restored.NormalizedValue-*parameter.NormalizedValue) > 0.00001 {
		result.Error = "fresh readback after rollback does not match the captured preimage"
		return result
	}
	if probeError != "" {
		result.Error = probeError
		return result
	}
	result.Status = "observed_passed"
	return result
}

func selectNeutralParameterProbe(parameters []SurfaceParameter) (SurfaceParameter, float64, bool) {
	candidates := append([]SurfaceParameter(nil), parameters...)
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	// Prefer a continuous observed parameter, but do not derive any semantic
	// meaning from its label. If none exists, a discrete host-controllable
	// parameter remains usable as a bounded fallback.
	for _, continuousOnly := range []bool{true, false} {
		for _, parameter := range candidates {
			if !parameter.HostControllable || parameter.NormalizedValue == nil || !finiteHostValue(*parameter.NormalizedValue) {
				continue
			}
			discrete, _ := parameter.Observed["is_discrete"].(bool)
			boolean, _ := parameter.Observed["is_boolean"].(bool)
			if continuousOnly && (discrete || boolean) {
				continue
			}
			current := *parameter.NormalizedValue
			target := current + 0.01
			if target > 0.99 {
				target = current - 0.01
			}
			if target >= 0 && target <= 1 && math.Abs(target-current) >= 0.001 {
				return parameter, target, true
			}
		}
	}
	return SurfaceParameter{}, 0, false
}

func executeStateRoundtrip(ctx context.Context, adapter *adapterClient, now time.Time) stateRoundtripResults {
	result := stateRoundtripResults{SchemaVersion: preflightSchema, Trust: "observed", Status: "unsupported/unknown", CapturedAt: now}
	saved, _, saveErr := adapter.operation(ctx, "/v1/plugin/state/save", map[string]any{})
	result.SaveResult = saved
	if saveErr != nil {
		result.Error = saveErr.Error()
		return result
	}
	roundtrip, _, roundtripErr := adapter.operation(ctx, "/v1/plugin/state/roundtrip", map[string]any{})
	result.RoundtripResult = roundtrip
	if roundtripErr != nil {
		result.Error = roundtripErr.Error()
		return result
	}
	if !boolJSONField(roundtrip, "parameter_readback_matches_preimage") {
		result.Error = "state reload completed but fresh parameter readback did not match its preimage"
		return result
	}
	result.Status = "observed_passed"
	return result
}

func executeDefaultFXMBaseline(ctx context.Context, adapter *adapterClient, root, probeRoot string, now time.Time, enabled bool) map[string]any {
	result := map[string]any{
		"schema_version": preflightSchema, "trust": "observed", "status": "unsupported/unknown", "captured_at": now,
		"baseline_policy": "same_source_bytes_worker_sample_rate_and_plugin_default_state_for_bypass_and_processed",
		"renders":         []any{},
	}
	if !enabled {
		result["error"] = "FXM baseline disabled"
		return result
	}
	probeRoot = filepath.Clean(strings.TrimSpace(probeRoot))
	if probeRoot == "" || probeRoot == "." {
		result["error"] = "probe audio root is required for FXM default baseline"
		return result
	}
	var manifest probeaudio.Manifest
	if err := readJSON(filepath.Join(probeRoot, "manifest.json"), &manifest); err != nil {
		result["error"] = err.Error()
		return result
	}
	if manifest.SuiteID != probeaudio.SuiteID {
		result["error"] = fmt.Sprintf("probe audio suite must be %s", probeaudio.SuiteID)
		return result
	}
	saved, _, saveErr := adapter.operation(ctx, "/v1/plugin/state/save", map[string]any{})
	if saveErr != nil {
		result["error"] = "save default state before FXM baseline: " + saveErr.Error()
		return result
	}
	stateID := stringJSONField(saved, "state_id")
	renders := make([]any, 0, len(manifest.Assets))
	allPassed := true
	for _, asset := range manifest.Assets {
		input := filepath.Join(probeRoot, asset.File)
		entry := map[string]any{"asset_id": asset.ID, "input_file": input, "expected_input_sha256": asset.SHA256, "status": "observed"}
		for _, stage := range []struct {
			name   string
			bypass bool
		}{{"bypass_chain", true}, {"processed_chain", false}} {
			output := filepath.Join(root, "renders", "fxm_default_baseline", asset.ID, stage.name+".wav")
			payload := map[string]any{"input_path": input, "output_path": output, "bypass": stage.bypass}
			render, _, err := adapter.operation(ctx, "/v1/plugin/render", payload)
			if err != nil {
				entry[stage.name] = map[string]any{"status": "error", "error": err.Error()}
				allPassed = false
				continue
			}
			entry[stage.name] = json.RawMessage(render)
		}
		renders = append(renders, entry)
	}
	result["renders"] = renders
	if stateID != "" {
		if _, _, err := adapter.operation(ctx, "/v1/plugin/state/restore", map[string]any{"state_id": stateID}); err != nil {
			allPassed = false
			result["restore_error"] = err.Error()
		}
	}
	if allPassed {
		result["status"] = "observed_completed"
	} else {
		result["error"] = "one or more deterministic baseline renders or state restoration operations failed"
	}
	return result
}

func buildInferredParameterGroups(snapshot HostSnapshot, now time.Time) map[string]any {
	groups := map[string][]SurfaceParameter{}
	for _, parameter := range snapshot.Surface.Parameters {
		key := hostLabelGroupKey(parameter.Name)
		groups[key] = append(groups[key], parameter)
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]any, 0, len(keys))
	for _, key := range keys {
		parameters := groups[key]
		ids := make([]string, 0, len(parameters))
		labels := make([]string, 0, len(parameters))
		for _, parameter := range parameters {
			ids = append(ids, parameter.ID)
			labels = append(labels, parameter.Name)
		}
		sort.Strings(ids)
		sort.Strings(labels)
		rows = append(rows, map[string]any{
			"id": stableID("inferred_group", snapshot.Identity.Name, key), "label_evidence_prefix": key,
			"parameter_ids": ids, "host_labels": labels, "trust": "agent-inferred", "status": "not_executable",
			"basis": "deterministic grouping from observed host-label text only; no semantic mapping or task authority is implied",
		})
	}
	return map[string]any{"schema_version": preflightSchema, "trust": "agent-inferred", "captured_at": now, "groups": rows}
}

func hostLabelGroupKey(label string) string {
	fields := strings.Fields(strings.TrimSpace(label))
	if len(fields) == 0 {
		return "unlabelled"
	}
	if len(fields) >= 2 {
		if _, err := fmt.Sscanf(fields[1], "%d", new(int)); err == nil {
			return strings.Join(fields[:2], " ")
		}
	}
	return fields[0]
}

func buildCandidateBadges(request PreflightRequest, snapshot HostSnapshot, now time.Time) map[string]any {
	badge := firstNonEmpty(strings.TrimSpace(request.CandidateBadge), "unknown")
	return map[string]any{
		"schema_version": preflightSchema, "trust": "agent-inferred", "captured_at": now,
		"candidates": []any{map[string]any{
			"badge_id": badge, "category_id": strings.TrimSpace(request.Category), "task": strings.TrimSpace(request.Task),
			"required_features": uniqueSorted(request.RequiredFeatures), "status": "candidate_not_granted", "routing_eligible": false,
			"basis":       []string{"authoring-plan input", "observed local plugin identity: " + snapshot.Identity.Name},
			"limitations": []string{"a candidate badge cannot be granted from plugin name, labels, enumeration, or automatic probe results", "preflight cannot issue a Credential or enter Catalog"},
		}},
	}
}

func buildFeatureGaps(request PreflightRequest, now time.Time) map[string]any {
	features := uniqueSorted(request.RequiredFeatures)
	gaps := make([]any, 0, len(features))
	for _, feature := range features {
		gaps = append(gaps, map[string]any{"feature_id": feature, "status": "unsupported/unknown", "trust": "agent-inferred", "reason": "no human semantic confirmation and capability-scoped conformance exists"})
	}
	return map[string]any{
		"schema_version": preflightSchema, "trust": "agent-inferred", "captured_at": now,
		"required_badge": firstNonEmpty(request.CandidateBadge, "unknown"), "feature_gaps": gaps,
		"policy": "feature conditions are evaluated only after exact task-badge selection; they are not standalone routeable badges",
	}
}

func buildWitnessRounds(request PreflightRequest, now time.Time) map[string]any {
	badge := firstNonEmpty(request.CandidateBadge, "unknown")
	return map[string]any{
		"schema_version": preflightSchema, "trust": "unsupported/unknown", "captured_at": now,
		"strategy": "minimum two-round witness plan; no user testimony has been requested or recorded",
		"rounds": []any{
			map[string]any{"id": "witness_round_1", "badge_id": badge, "purpose": "Confirm the independently requested task, standard actions, control ownership and visible result semantics.", "status": "planned"},
			map[string]any{"id": "witness_round_2", "badge_id": badge, "purpose": "Confirm modes, routing, stateful behavior, exceptions and vendor-specific controls relevant to the selected task.", "status": "planned"},
		},
	}
}

func buildHumanEvidenceGaps(request PreflightRequest, probe preflightParameterProbe, state stateRoundtripResults, now time.Time) map[string]any {
	gaps := []any{
		map[string]any{"id": "semantic_task_definition", "status": "unsupported/unknown", "round": "witness_round_1", "reason": "host labels and automatic behavior are not semantic authority"},
		map[string]any{"id": "standard_action_mapping", "status": "unsupported/unknown", "round": "witness_round_1", "reason": "parameter enumeration has not confirmed vendor-neutral action mapping"},
		map[string]any{"id": "modes_routing_and_exceptions", "status": "unsupported/unknown", "round": "witness_round_2", "reason": "state, mode and routing semantics require a human witness"},
		map[string]any{"id": "vendor_special_capabilities", "status": "unsupported/unknown", "round": "witness_round_2", "reason": "special capabilities require their own schema and conformance before dispatch"},
	}
	return map[string]any{
		"schema_version": preflightSchema, "trust": "unsupported/unknown", "captured_at": now,
		"required_badge": firstNonEmpty(request.CandidateBadge, "unknown"), "gaps": gaps,
		"observed_machine_results": map[string]any{"parameter_probe": probe.Status, "state_roundtrip": state.Status},
	}
}

func buildConformanceSkeleton(request PreflightRequest, probe preflightParameterProbe, state stateRoundtripResults, now time.Time) map[string]any {
	return map[string]any{
		"schema_version": preflightSchema, "trust": "unsupported/unknown", "captured_at": now,
		"required_badge": firstNonEmpty(request.CandidateBadge, "unknown"), "credential_issuable": false,
		"checks": []any{
			map[string]any{"id": "identity_and_fingerprint", "status": "observed"},
			map[string]any{"id": "parameter_write_fresh_readback_rollback", "status": probe.Status},
			map[string]any{"id": "state_serialization_reload_restore", "status": state.Status},
			map[string]any{"id": "default_fxm_baseline", "status": "observed_or_unknown"},
			map[string]any{"id": "human_semantic_mapping", "status": "unsupported/unknown"},
			map[string]any{"id": "behavior_and_boundary_conformance", "status": "unsupported/unknown"},
			map[string]any{"id": "resource_isolation", "status": "unsupported/unknown"},
		},
	}
}

func buildBadgeConformanceSkeleton(request PreflightRequest, probe preflightParameterProbe, state stateRoundtripResults, now time.Time) map[string]any {
	return map[string]any{
		"schema_version": preflightSchema, "trust": "unsupported/unknown", "captured_at": now,
		"badge_id": firstNonEmpty(request.CandidateBadge, "unknown"), "category_id": strings.TrimSpace(request.Category),
		"status": "planned_not_conformed", "credential_issuable": false,
		"machine_observations":                 map[string]any{"parameter_probe": probe.Status, "state_roundtrip": state.Status},
		"required_human_and_conformance_gates": []string{"human semantic confirmation", "capability-scoped write/readback and rollback", "state restoration", "behavior testing", "boundary and resource-isolation testing", "explicit separate installation authority"},
	}
}

func stringJSONField(raw json.RawMessage, key string) string {
	var object map[string]any
	if json.Unmarshal(raw, &object) != nil {
		return ""
	}
	value, _ := object[key].(string)
	return strings.TrimSpace(value)
}

func boolJSONField(raw json.RawMessage, key string) bool {
	var object map[string]any
	if json.Unmarshal(raw, &object) != nil {
		return false
	}
	value, _ := object[key].(bool)
	return value
}

func freshReadbackMatchesJSON(raw json.RawMessage, id string, target float64) bool {
	var payload struct {
		FreshReadback struct {
			Parameters []struct {
				ID              string   `json:"id"`
				NormalizedValue *float64 `json:"normalized_value"`
			} `json:"parameters"`
		} `json:"fresh_readback"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	for _, parameter := range payload.FreshReadback.Parameters {
		if parameter.ID == id && parameter.NormalizedValue != nil {
			return math.Abs(*parameter.NormalizedValue-target) <= 0.00001
		}
	}
	return false
}

func findSurfaceParameter(parameters []SurfaceParameter, id string) (SurfaceParameter, bool) {
	for _, parameter := range parameters {
		if parameter.ID == id {
			return parameter, true
		}
	}
	return SurfaceParameter{}, false
}

package vpsforge

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/vps"
)

const (
	proQ3StaticLifecycleProbeSchema  = "vit.vpsforge.pro_q_3_static_lifecycle_probe.v1"
	proQ3StaticLifecycleProbeProfile = "fabfilter.pro-q-3.band-1.static-lifecycle.v1"
	proQ3StaticLifecycleEvidenceDir  = "evidence/pro_q_3_static_lifecycle_probe"
)

// ProQ3StaticLifecycleProbeRequest names a direct-worker staging experiment
// over already observed Pro-Q 3 selectors. The IDs and display labels remain
// observations; the probe does not convert them into a reusable VPS mapping.
type ProQ3StaticLifecycleProbeRequest struct {
	Root           string
	WorkerPath     string
	ProbeAudioRoot string

	BandUsedParameterID      string
	BandEnabledParameterID   string
	BandFrequencyParameterID string
	BandGainParameterID      string
	BandQParameterID         string
	BandShapeParameterID     string
	Now                      time.Time
}

type ProQ3StaticLifecycleProbeResult struct {
	Workspace       string `json:"workspace"`
	Status          string `json:"status"`
	ResultsArtifact string `json:"results_artifact"`
	RunArtifact     string `json:"run_artifact"`
	RenderDirectory string `json:"render_directory"`
}

type proQ3StaticLifecycleProbeArtifact struct {
	SchemaVersion string    `json:"schema_version"`
	Trust         string    `json:"trust"`
	Status        string    `json:"status"`
	CapturedAt    time.Time `json:"captured_at"`
	WorkspaceID   string    `json:"workspace_id"`
	Profile       string    `json:"profile"`
	RunArtifact   string    `json:"run_artifact"`

	AuthorityBoundary []string                        `json:"authority_boundary"`
	ProbeInput        proQ3CoreEQProbeInput           `json:"probe_input,omitempty"`
	ObservedControls  []proQ3LifecycleObservedControl `json:"observed_controls,omitempty"`
	InitialState      proQ3IsolationStateReceipt      `json:"initial_state,omitempty"`
	BypassRender      *proQ3LifecycleRender           `json:"bypass_render,omitempty"`
	ActiveState       proQ3LifecycleActiveState       `json:"active_state,omitempty"`
	Candidates        []proQ3LifecycleCandidate       `json:"candidates,omitempty"`
	FinalRollback     proQ3LifecycleFinalRollback     `json:"final_rollback,omitempty"`
	Limitations       []string                        `json:"limitations,omitempty"`
	Errors            []string                        `json:"errors,omitempty"`
}

type proQ3LifecycleObservedControl struct {
	Role              string  `json:"role"`
	ID                string  `json:"id"`
	HostLabel         string  `json:"host_label"`
	InitialNormalized float64 `json:"initial_normalized"`
	InitialDisplay    string  `json:"initial_display"`
	Automation        string  `json:"automation"`
	SelectionBoundary string  `json:"selection_boundary"`
}

type proQ3LifecycleChange struct {
	Role                string  `json:"role"`
	ID                  string  `json:"id"`
	HostLabel           string  `json:"host_label"`
	RequestedNormalized float64 `json:"requested_normalized"`
	ObservedNormalized  float64 `json:"observed_normalized"`
	ObservedDisplay     string  `json:"observed_display"`
}

type proQ3LifecycleActiveState struct {
	Configuration                     []proQ3LifecycleChange   `json:"configuration,omitempty"`
	TransactionID                     string                   `json:"transaction_id,omitempty"`
	FreshReadbackMatches              bool                     `json:"fresh_readback_matches"`
	StateRoundtripMatches             bool                     `json:"state_roundtrip_matches"`
	FreshSnapshotMatchesPostRoundtrip proQ3IsolationComparison `json:"fresh_snapshot_matches_post_roundtrip"`
	Render                            *proQ3LifecycleRender    `json:"render,omitempty"`
	Rollback                          proQ3LifecycleRollback   `json:"rollback"`
}

type proQ3LifecycleCandidate struct {
	ID                                string                   `json:"id"`
	Purpose                           string                   `json:"purpose"`
	Change                            proQ3LifecycleChange     `json:"change"`
	TransactionID                     string                   `json:"transaction_id,omitempty"`
	FreshReadbackMatches              bool                     `json:"fresh_readback_matches"`
	StateRoundtripMatches             bool                     `json:"state_roundtrip_matches"`
	FreshSnapshotMatchesPostRoundtrip proQ3IsolationComparison `json:"fresh_snapshot_matches_post_roundtrip"`
	Render                            *proQ3LifecycleRender    `json:"render,omitempty"`
	RollbackToActive                  proQ3LifecycleRollback   `json:"rollback_to_active"`
}

type proQ3LifecycleRollback struct {
	ReportedVerified bool                     `json:"reported_verified"`
	FreshReadback    proQ3IsolationComparison `json:"fresh_readback"`
}

type proQ3LifecycleRender struct {
	OutputPath    string                         `json:"output_path"`
	OutputSHA256  string                         `json:"output_sha256"`
	Response      []proQ3CoreEQFrequencyResponse `json:"response_vs_bypass_db,omitempty"`
	Peak          proQ3CoreEQExtreme             `json:"peak,omitempty"`
	Trough        proQ3CoreEQExtreme             `json:"trough,omitempty"`
	MaxAbsoluteDB float64                        `json:"max_absolute_delta_db"`
}

type proQ3LifecycleFinalRollback struct {
	InitialStateRestoreVerified bool                     `json:"initial_state_restore_verified"`
	FreshReadbackMatchesInitial proQ3IsolationComparison `json:"fresh_readback_matches_initial"`
}

// RunProQ3StaticLifecycleProbe tests bounded active, enabled-off and used-off
// states with fresh readback, state reload, deterministic impulse render and
// rollback. The measured deltas only describe observed test states.
func RunProQ3StaticLifecycleProbe(ctx context.Context, request ProQ3StaticLifecycleProbeRequest) (result ProQ3StaticLifecycleProbeResult, err error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return result, err
	}
	status, err := Inspect(root)
	if err != nil {
		return result, fmt.Errorf("inspect Pro-Q 3 static lifecycle workspace: %w", err)
	}
	if !isProQ3Identity(status.Manifest.PluginIdentity) {
		return result, fmt.Errorf("static lifecycle probe only accepts the observed FabFilter Pro-Q 3 reference workspace")
	}
	var scope equalizerV2ScopeReview
	if err := readJSON(filepath.Join(root, equalizerV2ScopeReviewFile), &scope); err != nil {
		return result, fmt.Errorf("read required equalizer.v2 scope review: %w", err)
	}
	if !proQ3LifecycleScopeIncludes(scope, "static_band_lifecycle") {
		return result, fmt.Errorf("equalizer.v2 scope review does not include static band lifecycle")
	}
	if err := validateProQ3LifecycleRequest(request); err != nil {
		return result, err
	}
	input, err := loadProQ3CoreEQInput(request.ProbeAudioRoot)
	if err != nil {
		return result, err
	}
	workerPath := filepath.Clean(strings.TrimSpace(request.WorkerPath))
	if info, statErr := os.Stat(workerPath); statErr != nil || info.IsDir() {
		if statErr != nil {
			return result, fmt.Errorf("inspect native VST3 worker: %w", statErr)
		}
		return result, fmt.Errorf("native VST3 worker path is a directory: %s", workerPath)
	}
	now := nowOrCurrent(request.Now)
	runID := stableID("pro_q_3_static_lifecycle", status.Manifest.WorkspaceID, now.Format(time.RFC3339Nano))
	renderDirectory := filepath.Join(root, "renders", "pro_q_3_static_lifecycle_probe", runID)
	runArtifact := filepath.ToSlash(filepath.Join(proQ3StaticLifecycleEvidenceDir, runID+".json"))
	result = ProQ3StaticLifecycleProbeResult{
		Workspace: root, ResultsArtifact: proQ3StaticLifecycleResultsFile, RunArtifact: runArtifact,
		RenderDirectory: relativeStereoProbePath(root, renderDirectory),
	}
	artifact := proQ3StaticLifecycleProbeArtifact{
		SchemaVersion: proQ3StaticLifecycleProbeSchema,
		Trust:         "observed",
		Status:        "unsupported/unknown",
		CapturedAt:    now,
		WorkspaceID:   status.Manifest.WorkspaceID,
		Profile:       proQ3StaticLifecycleProbeProfile,
		RunArtifact:   runArtifact,
		ProbeInput:    input,
		AuthorityBoundary: []string{
			"staging-only direct-worker experiment; no VPS Library, Credential, Catalog or SPAL route is changed",
			"parameter IDs, host labels, display values and measured impulse deltas remain observed evidence, not lifecycle semantics or executable mappings",
			"serialized plugin bytes are held only in memory for final restoration and never written into this artifact or the evidence ledger",
			"the test evaluates only bounded static candidates; it does not cover deferred Dynamic EQ, analysis/matching, spectrum or phase modes",
		},
		Limitations: []string{
			"Observed candidates named active, enabled-off and used-off are test labels only; a human-reviewed semantic contract is still required.",
			"Static impulse renders do not establish musical-quality conclusions or generic equalizer.v2 behavior.",
		},
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
	}
	pluginPath := strings.TrimSpace(status.Manifest.PluginIdentity.InstallPath)
	bypassPath := filepath.Join(renderDirectory, "00_bypass_impulse.wav")
	bypass, bypassErr := renderProQ3LifecycleBypass(ctx, workerPath, status.Manifest.PluginIdentity, pluginPath, input.Path, bypassPath)
	if bypassErr != nil {
		return result, bypassErr
	}
	artifact.BypassRender = &bypass
	adapter := NewVST3HostAdapter(workerPath)
	loaded := false
	initialValues := map[string]float64{}
	initialStateBase64 := ""
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if loaded && initialStateBase64 != "" {
			if _, restoreErr := proQ3IsolationOperation(cleanupCtx, adapter, "restore_state", map[string]any{"state_base64": initialStateBase64}); restoreErr != nil {
				artifact.Errors = appendProQ3IsolationError(artifact.Errors, "final initial-state restore: "+restoreErr.Error())
				if err == nil {
					err = fmt.Errorf("final initial-state restore: %w", restoreErr)
				}
			} else if fresh, snapshotErr := proQ3IsolationSnapshot(cleanupCtx, adapter); snapshotErr != nil {
				artifact.Errors = appendProQ3IsolationError(artifact.Errors, "final fresh snapshot: "+snapshotErr.Error())
				if err == nil {
					err = fmt.Errorf("final fresh snapshot: %w", snapshotErr)
				}
			} else {
				artifact.FinalRollback.InitialStateRestoreVerified = true
				artifact.FinalRollback.FreshReadbackMatchesInitial = proQ3IsolationCompare(initialValues, proQ3IsolationParameterMap(fresh.Surface.Parameters))
				if !artifact.FinalRollback.FreshReadbackMatchesInitial.Matches && err == nil {
					err = fmt.Errorf("final lifecycle fresh readback does not match complete initial preimage")
				}
			}
		}
		_ = adapter.Close()
		if err != nil {
			artifact.Status = "observed_failed"
			artifact.Errors = appendProQ3IsolationError(artifact.Errors, err.Error())
		} else {
			artifact.Status = "observed_completed"
		}
		if persistErr := persistProQ3StaticLifecycleArtifacts(root, artifact); persistErr != nil && err == nil {
			err = persistErr
			result.Status = "observed_failed"
			return
		}
		result.Status = artifact.Status
	}()

	if _, err = adapter.load(ctx, map[string]any{"plugin_path": pluginPath, "sample_rate": 48000, "block_size": 512}); err != nil {
		return result, fmt.Errorf("load isolated Pro-Q 3 for lifecycle probe: %w", err)
	}
	loaded = true
	snapshot, snapshotErr := proQ3IsolationSnapshot(ctx, adapter)
	if snapshotErr != nil {
		return result, fmt.Errorf("capture initial lifecycle snapshot: %w", snapshotErr)
	}
	if identityErr := proQ3IsolationValidateSnapshot(status.Manifest.PluginIdentity, pluginPath, snapshot); identityErr != nil {
		return result, identityErr
	}
	initialValues = proQ3IsolationParameterMap(snapshot.Surface.Parameters)
	artifact.InitialState, initialStateBase64, err = proQ3IsolationSaveState(ctx, adapter)
	if err != nil {
		return result, fmt.Errorf("save complete initial lifecycle state: %w", err)
	}
	controls, controlErr := proQ3LifecycleControls(snapshot.Surface.Parameters, request)
	if controlErr != nil {
		return result, controlErr
	}
	artifact.ObservedControls = controls

	activeChanges, changeErr := proQ3LifecycleActiveChanges(snapshot.Surface.Parameters, request)
	if changeErr != nil {
		return result, changeErr
	}
	activeWrite, writeErr := proQ3LifecycleWrite(ctx, adapter, activeChanges)
	if writeErr != nil {
		return result, fmt.Errorf("write active lifecycle candidate: %w", writeErr)
	}
	artifact.ActiveState.Configuration = proQ3LifecycleReadbackChanges(activeWrite, activeChanges)
	artifact.ActiveState.TransactionID = stringJSONField(activeWrite, "transaction_id")
	artifact.ActiveState.FreshReadbackMatches = artifact.ActiveState.TransactionID != "" && proQ3LifecycleFreshReadbackMatches(activeWrite, activeChanges)
	if !artifact.ActiveState.FreshReadbackMatches {
		return result, fmt.Errorf("active lifecycle write omitted matching fresh readback or transaction ID")
	}
	activeRoundtrip, roundtripErr := proQ3IsolationOperation(ctx, adapter, "roundtrip_state", map[string]any{})
	if roundtripErr != nil {
		return result, fmt.Errorf("roundtrip active lifecycle state: %w", roundtripErr)
	}
	artifact.ActiveState.StateRoundtripMatches = boolJSONField(activeRoundtrip, "parameter_readback_matches_preimage")
	if !artifact.ActiveState.StateRoundtripMatches {
		return result, fmt.Errorf("active lifecycle state roundtrip reported a parameter mismatch")
	}
	activeSnapshot, activeSnapshotErr := proQ3IsolationSnapshot(ctx, adapter)
	if activeSnapshotErr != nil {
		return result, fmt.Errorf("capture active lifecycle snapshot: %w", activeSnapshotErr)
	}
	activeValues := proQ3IsolationParameterMap(activeSnapshot.Surface.Parameters)
	artifact.ActiveState.FreshSnapshotMatchesPostRoundtrip = proQ3IsolationCompare(proQ3LifecycleExpectedValues(initialValues, activeChanges), activeValues)
	if !artifact.ActiveState.FreshSnapshotMatchesPostRoundtrip.Matches {
		return result, fmt.Errorf("active lifecycle snapshot after roundtrip does not match complete expected post-write state")
	}
	activeRender, activeRenderErr := renderProQ3Lifecycle(ctx, adapter, input.Path, filepath.Join(renderDirectory, "01_active.wav"), false, bypassPath)
	if activeRenderErr != nil {
		return result, activeRenderErr
	}
	artifact.ActiveState.Render = &activeRender

	for _, candidate := range []struct {
		id      string
		purpose string
		role    string
		value   float64
		file    string
	}{
		{"enabled_zero_candidate", "Measure the observed Enabled selector at normalized zero from the bounded active state.", "band_enabled", 0, "02_enabled_zero_candidate.wav"},
		{"used_zero_candidate", "Measure the observed Used selector at normalized zero from the bounded active state.", "band_used", 0, "03_used_zero_candidate.wav"},
	} {
		change, found := proQ3LifecycleChangeByRole(activeChanges, candidate.role)
		if !found {
			return result, fmt.Errorf("active lifecycle configuration omitted %s", candidate.role)
		}
		change.RequestedNormalized = candidate.value
		scenario, scenarioErr := runProQ3LifecycleCandidate(ctx, adapter, activeValues, input.Path, bypassPath, filepath.Join(renderDirectory, candidate.file), candidate.id, candidate.purpose, change)
		if scenarioErr != nil {
			return result, scenarioErr
		}
		artifact.Candidates = append(artifact.Candidates, scenario)
	}

	activeRollback, rollbackErr := proQ3IsolationOperation(ctx, adapter, "rollback", map[string]any{"transaction_id": artifact.ActiveState.TransactionID})
	if rollbackErr != nil {
		return result, fmt.Errorf("rollback active lifecycle state: %w", rollbackErr)
	}
	artifact.ActiveState.Rollback.ReportedVerified = boolJSONField(activeRollback, "rollback_verified")
	rolledBack, rollbackSnapshotErr := proQ3IsolationSnapshot(ctx, adapter)
	if rollbackSnapshotErr != nil {
		return result, fmt.Errorf("capture fresh snapshot after active lifecycle rollback: %w", rollbackSnapshotErr)
	}
	artifact.ActiveState.Rollback.FreshReadback = proQ3IsolationCompare(initialValues, proQ3IsolationParameterMap(rolledBack.Surface.Parameters))
	if !artifact.ActiveState.Rollback.ReportedVerified || !artifact.ActiveState.Rollback.FreshReadback.Matches {
		return result, fmt.Errorf("active lifecycle rollback did not restore complete initial preimage")
	}
	return result, nil
}

func validateProQ3LifecycleRequest(request ProQ3StaticLifecycleProbeRequest) error {
	if strings.TrimSpace(request.WorkerPath) == "" {
		return fmt.Errorf("native VST3 worker path is required")
	}
	if strings.TrimSpace(request.ProbeAudioRoot) == "" {
		return fmt.Errorf("Probe Audio root is required")
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"Band Used parameter ID", request.BandUsedParameterID},
		{"Band Enabled parameter ID", request.BandEnabledParameterID},
		{"Band Frequency parameter ID", request.BandFrequencyParameterID},
		{"Band Gain parameter ID", request.BandGainParameterID},
		{"Band Q parameter ID", request.BandQParameterID},
		{"Band Shape parameter ID", request.BandShapeParameterID},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("static lifecycle probe %s is required", field.name)
		}
	}
	return nil
}

func proQ3LifecycleScopeIncludes(scope equalizerV2ScopeReview, id string) bool {
	for _, item := range scope.IncludedScope {
		if item.ID == id && item.Status == "in_scope_pending_conformance" {
			return true
		}
	}
	return false
}

func proQ3LifecycleControls(parameters []SurfaceParameter, request ProQ3StaticLifecycleProbeRequest) ([]proQ3LifecycleObservedControl, error) {
	requested := []struct {
		role string
		id   string
	}{
		{"band_used", request.BandUsedParameterID},
		{"band_enabled", request.BandEnabledParameterID},
		{"band_frequency", request.BandFrequencyParameterID},
		{"band_gain", request.BandGainParameterID},
		{"band_q", request.BandQParameterID},
		{"band_shape", request.BandShapeParameterID},
	}
	controls := make([]proQ3LifecycleObservedControl, 0, len(requested))
	for _, item := range requested {
		parameter, ok := findSurfaceParameter(parameters, item.id)
		if !ok || parameter.NormalizedValue == nil || !parameter.HostControllable {
			return nil, fmt.Errorf("observed lifecycle selector %s (%s) is not a host-controllable parameter", item.role, item.id)
		}
		controls = append(controls, proQ3LifecycleObservedControl{
			Role: item.role, ID: parameter.ID, HostLabel: parameter.Name, InitialNormalized: *parameter.NormalizedValue,
			InitialDisplay: parameter.DisplayText, Automation: parameter.Automation,
			SelectionBoundary: "selected from the current observed Pro-Q 3 VST3 surface for a bounded staging test; it is not an executable semantic binding",
		})
	}
	return controls, nil
}

func proQ3LifecycleActiveChanges(parameters []SurfaceParameter, request ProQ3StaticLifecycleProbeRequest) ([]proQ3LifecycleChange, error) {
	frequency := observedCoreValue(parameters, request.BandFrequencyParameterID, 0.575188457965851)
	changes := []struct {
		role  string
		id    string
		value float64
	}{
		{"band_used", request.BandUsedParameterID, 1},
		{"band_enabled", request.BandEnabledParameterID, 1},
		{"band_frequency", request.BandFrequencyParameterID, frequency},
		{"band_gain", request.BandGainParameterID, 0.75},
		{"band_q", request.BandQParameterID, 0.5},
		{"band_shape", request.BandShapeParameterID, 0},
	}
	result := make([]proQ3LifecycleChange, 0, len(changes))
	for _, change := range changes {
		parameter, ok := findSurfaceParameter(parameters, change.id)
		if !ok || parameter.NormalizedValue == nil || !parameter.HostControllable {
			return nil, fmt.Errorf("active lifecycle selector %s (%s) is not host-controllable", change.role, change.id)
		}
		result = append(result, proQ3LifecycleChange{
			Role: change.role, ID: parameter.ID, HostLabel: parameter.Name, RequestedNormalized: change.value,
		})
	}
	return result, nil
}

func proQ3LifecycleWrite(ctx context.Context, adapter *VST3HostAdapter, changes []proQ3LifecycleChange) (json.RawMessage, error) {
	payload := make([]map[string]any, 0, len(changes))
	for _, change := range changes {
		payload = append(payload, map[string]any{"id": change.ID, "normalized": change.RequestedNormalized})
	}
	return proQ3IsolationOperation(ctx, adapter, "write", map[string]any{"changes": payload})
}

func proQ3LifecycleFreshReadbackMatches(raw json.RawMessage, changes []proQ3LifecycleChange) bool {
	for _, change := range changes {
		if !freshReadbackMatchesJSON(raw, change.ID, change.RequestedNormalized) {
			return false
		}
	}
	return true
}

func proQ3LifecycleReadbackChanges(raw json.RawMessage, changes []proQ3LifecycleChange) []proQ3LifecycleChange {
	for index := range changes {
		if value, ok := stereoFreshReadbackParameter(raw, changes[index].ID); ok {
			changes[index].ObservedNormalized = value.NormalizedValue
			changes[index].ObservedDisplay = value.DisplayValue
		}
	}
	return changes
}

func proQ3LifecycleExpectedValues(initial map[string]float64, changes []proQ3LifecycleChange) map[string]float64 {
	values := make(map[string]float64, len(initial))
	for id, value := range initial {
		values[id] = value
	}
	for _, change := range changes {
		values[change.ID] = change.RequestedNormalized
	}
	return values
}

func proQ3LifecycleChangeByRole(changes []proQ3LifecycleChange, role string) (proQ3LifecycleChange, bool) {
	for _, change := range changes {
		if change.Role == role {
			return change, true
		}
	}
	return proQ3LifecycleChange{}, false
}

func runProQ3LifecycleCandidate(ctx context.Context, adapter *VST3HostAdapter, activeValues map[string]float64, inputPath, bypassPath, outputPath, id, purpose string, change proQ3LifecycleChange) (proQ3LifecycleCandidate, error) {
	scenario := proQ3LifecycleCandidate{ID: id, Purpose: purpose, Change: change}
	write, err := proQ3LifecycleWrite(ctx, adapter, []proQ3LifecycleChange{change})
	if err != nil {
		return scenario, fmt.Errorf("write %s: %w", id, err)
	}
	scenario.Change = proQ3LifecycleReadbackChanges(write, []proQ3LifecycleChange{change})[0]
	scenario.TransactionID = stringJSONField(write, "transaction_id")
	scenario.FreshReadbackMatches = scenario.TransactionID != "" && proQ3LifecycleFreshReadbackMatches(write, []proQ3LifecycleChange{change})
	if !scenario.FreshReadbackMatches {
		return scenario, fmt.Errorf("%s write omitted matching fresh readback or transaction ID", id)
	}
	roundtrip, err := proQ3IsolationOperation(ctx, adapter, "roundtrip_state", map[string]any{})
	if err != nil {
		return scenario, fmt.Errorf("roundtrip %s: %w", id, err)
	}
	scenario.StateRoundtripMatches = boolJSONField(roundtrip, "parameter_readback_matches_preimage")
	if !scenario.StateRoundtripMatches {
		return scenario, fmt.Errorf("%s state roundtrip reported a parameter mismatch", id)
	}
	fresh, err := proQ3IsolationSnapshot(ctx, adapter)
	if err != nil {
		return scenario, fmt.Errorf("fresh snapshot %s: %w", id, err)
	}
	expected := proQ3LifecycleExpectedValues(activeValues, []proQ3LifecycleChange{change})
	scenario.FreshSnapshotMatchesPostRoundtrip = proQ3IsolationCompare(expected, proQ3IsolationParameterMap(fresh.Surface.Parameters))
	if !scenario.FreshSnapshotMatchesPostRoundtrip.Matches {
		return scenario, fmt.Errorf("%s fresh snapshot after roundtrip does not match complete expected state", id)
	}
	render, err := renderProQ3Lifecycle(ctx, adapter, inputPath, outputPath, false, bypassPath)
	if err != nil {
		return scenario, fmt.Errorf("render %s: %w", id, err)
	}
	scenario.Render = &render
	rollback, err := proQ3IsolationOperation(ctx, adapter, "rollback", map[string]any{"transaction_id": scenario.TransactionID})
	if err != nil {
		return scenario, fmt.Errorf("rollback %s: %w", id, err)
	}
	scenario.RollbackToActive.ReportedVerified = boolJSONField(rollback, "rollback_verified")
	restored, err := proQ3IsolationSnapshot(ctx, adapter)
	if err != nil {
		return scenario, fmt.Errorf("fresh snapshot after rollback %s: %w", id, err)
	}
	scenario.RollbackToActive.FreshReadback = proQ3IsolationCompare(activeValues, proQ3IsolationParameterMap(restored.Surface.Parameters))
	if !scenario.RollbackToActive.ReportedVerified || !scenario.RollbackToActive.FreshReadback.Matches {
		return scenario, fmt.Errorf("rollback %s did not restore complete active preimage", id)
	}
	return scenario, nil
}

// renderProQ3LifecycleBypass deliberately uses a separate disposable worker.
// Some VST3s expose a host-bypass controller that can be asynchronously
// reflected after processBlockBypassed. Keeping this render away from the
// mutable lifecycle instance preserves that instance's complete preimage for
// its subsequent write/roundtrip/rollback assertions.
func renderProQ3LifecycleBypass(ctx context.Context, workerPath string, expected vps.PluginIdentity, pluginPath, inputPath, outputPath string) (proQ3LifecycleRender, error) {
	adapter := NewVST3HostAdapter(workerPath)
	defer adapter.Close()
	if _, err := adapter.load(ctx, map[string]any{"plugin_path": pluginPath, "sample_rate": 48000, "block_size": 512}); err != nil {
		return proQ3LifecycleRender{}, fmt.Errorf("load separate bypass baseline worker: %w", err)
	}
	snapshot, err := proQ3IsolationSnapshot(ctx, adapter)
	if err != nil {
		return proQ3LifecycleRender{}, fmt.Errorf("capture separate bypass baseline snapshot: %w", err)
	}
	if err := proQ3IsolationValidateSnapshot(expected, pluginPath, snapshot); err != nil {
		return proQ3LifecycleRender{}, fmt.Errorf("validate separate bypass baseline identity: %w", err)
	}
	render, err := renderProQ3Lifecycle(ctx, adapter, inputPath, outputPath, true, "")
	if err != nil {
		return proQ3LifecycleRender{}, fmt.Errorf("render separate bypass baseline: %w", err)
	}
	return render, nil
}

func renderProQ3Lifecycle(ctx context.Context, adapter *VST3HostAdapter, inputPath, outputPath string, bypass bool, bypassPath string) (proQ3LifecycleRender, error) {
	render := proQ3LifecycleRender{OutputPath: outputPath}
	if _, err := proQ3IsolationOperation(ctx, adapter, "render", map[string]any{"input_path": inputPath, "output_path": outputPath, "bypass": bypass}); err != nil {
		return render, err
	}
	digest, err := stereoSHA256File(outputPath)
	if err != nil {
		return render, err
	}
	render.OutputSHA256 = digest
	if strings.TrimSpace(bypassPath) == "" {
		return render, nil
	}
	response, err := proQ3CoreEQImpulseResponse(bypassPath, outputPath)
	if err != nil {
		return render, err
	}
	render.Response = response
	render.Peak = proQ3CoreEQExtremeFromResponse(response, true)
	render.Trough = proQ3CoreEQExtremeFromResponse(response, false)
	for _, point := range response {
		render.MaxAbsoluteDB = math.Max(render.MaxAbsoluteDB, math.Abs(point.DeltaDB))
	}
	return render, nil
}

func persistProQ3StaticLifecycleArtifacts(root string, artifact proQ3StaticLifecycleProbeArtifact) error {
	if err := writeJSON(filepath.Join(root, proQ3StaticLifecycleResultsFile), artifact); err != nil {
		return err
	}
	runArtifact := filepath.FromSlash(strings.TrimSpace(artifact.RunArtifact))
	if runArtifact == "" {
		return fmt.Errorf("static lifecycle run artifact path is required")
	}
	if err := writeJSON(filepath.Join(root, runArtifact), artifact); err != nil {
		return err
	}
	ledgerData, _ := json.Marshal(map[string]any{
		"schema_version": artifact.SchemaVersion, "status": artifact.Status,
		"active_fresh_readback_matches":        artifact.ActiveState.FreshReadbackMatches,
		"active_state_roundtrip_matches":       artifact.ActiveState.StateRoundtripMatches,
		"candidate_count":                      len(artifact.Candidates),
		"initial_state_restore_verified":       artifact.FinalRollback.InitialStateRestoreVerified,
		"final_fresh_readback_matches_initial": artifact.FinalRollback.FreshReadbackMatchesInitial.Matches,
		"serialized_state_raw_bytes_persisted": false,
	})
	if err := appendEvidence(root, EvidenceEntry{
		Kind:       "pro_q_3_static_lifecycle_probe",
		Trust:      "observed",
		Source:     "vpsforge.direct_vst3_worker",
		Summary:    "Captured bounded static lifecycle candidates with fresh readback, state roundtrip, deterministic impulse renders and complete rollback. Results remain observed and do not define equalizer.v2 lifecycle semantics or issue any Credential.",
		Artifact:   filepath.ToSlash(runArtifact),
		CapturedAt: artifact.CapturedAt,
		Data:       ledgerData,
	}); err != nil {
		return err
	}
	return updateManifest(root, func(manifest *Manifest) {
		if manifest.Artifacts == nil {
			manifest.Artifacts = map[string]string{}
		}
		manifest.Artifacts["pro_q_3_static_lifecycle_probe_results"] = proQ3StaticLifecycleResultsFile
		manifest.UpdatedAt = artifact.CapturedAt
	})
}

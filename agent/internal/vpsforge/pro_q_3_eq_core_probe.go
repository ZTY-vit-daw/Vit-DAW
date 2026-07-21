package vpsforge

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/probeaudio"
	"vit-daw-agent/internal/vps"
)

const (
	proQ3CoreEQProbeSchema  = "vit.vpsforge.pro_q_3_eq_core_probe.v2"
	proQ3CoreEQProbeProfile = "fabfilter.pro-q-3.band-1.core-eq.v2"
	proQ3CoreEQResultsFile  = "pro_q_3_eq_core_probe_results.json"
	proQ3CoreEQEvidenceDir  = "evidence/pro_q_3_eq_core_probe"
	proQ3ImpulseProbeAsset  = "impulse_stereo"

	proQ3CoreResponseFrames = 32768
)

// ProQ3CoreEQProbeRequest deliberately names a narrow, staging-only reference
// experiment. The IDs must have been observed on the current Pro-Q 3 surface;
// this request neither infers an executable semantic binding nor grants a
// task badge or Credential.
type ProQ3CoreEQProbeRequest struct {
	Root           string
	HostURL        string
	ProbeAudioRoot string

	BandUsedParameterID      string
	BandEnabledParameterID   string
	BandFrequencyParameterID string
	BandGainParameterID      string
	BandQParameterID         string
	BandShapeParameterID     string
	BandSlopeParameterID     string
	PlacementParameterID     string

	Client *http.Client
	Now    time.Time
}

type ProQ3CoreEQProbeResult struct {
	Workspace       string `json:"workspace"`
	Status          string `json:"status"`
	ResultsArtifact string `json:"results_artifact"`
	RunArtifact     string `json:"run_artifact"`
	RenderDirectory string `json:"render_directory"`
}

type proQ3CoreEQProbeArtifact struct {
	SchemaVersion string    `json:"schema_version"`
	Trust         string    `json:"trust"`
	Status        string    `json:"status"`
	CapturedAt    time.Time `json:"captured_at"`
	WorkspaceID   string    `json:"workspace_id"`
	Profile       string    `json:"profile"`
	RunArtifact   string    `json:"run_artifact"`

	AuthorityBoundary []string                     `json:"authority_boundary"`
	ProbeInput        proQ3CoreEQProbeInput        `json:"probe_input,omitempty"`
	ObservedControls  []proQ3CoreEQObservedControl `json:"observed_controls,omitempty"`
	ShapeDiscovery    []proQ3CoreEQChoice          `json:"shape_discovery,omitempty"`
	SlopeDiscovery    []proQ3CoreEQChoice          `json:"slope_discovery,omitempty"`
	BypassRender      *proQ3CoreEQRender           `json:"bypass_render,omitempty"`
	Scenarios         []proQ3CoreEQScenario        `json:"scenarios,omitempty"`
	ObservedRelations []proQ3CoreEQRelation        `json:"observed_relations,omitempty"`
	FinalRollback     proQ3CoreEQFinalRollback     `json:"final_rollback,omitempty"`
	Unknowns          []string                     `json:"unknowns,omitempty"`
	Errors            []string                     `json:"errors,omitempty"`
}

type proQ3CoreEQProbeInput struct {
	AssetID string `json:"asset_id"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Policy  string `json:"policy"`
}

type proQ3CoreEQObservedControl struct {
	Role                string  `json:"role"`
	ID                  string  `json:"id"`
	HostLabel           string  `json:"host_label"`
	InitialNormalized   float64 `json:"initial_normalized"`
	InitialDisplay      string  `json:"initial_display"`
	Discrete            bool    `json:"discrete"`
	ObservedNumSteps    int     `json:"observed_num_steps"`
	Automation          string  `json:"automation"`
	SelectionLimitation string  `json:"selection_limitation"`
}

// proQ3CoreEQChoice is only a discovered VST3 display state. The test uses a
// display match as a bounded candidate selection and records that selection as
// observed data; this does not turn the display text into task semantics.
type proQ3CoreEQChoice struct {
	RequestedNormalized     float64 `json:"requested_normalized"`
	ObservedDisplay         string  `json:"observed_display"`
	TransactionID           string  `json:"transaction_id"`
	FreshReadbackMatches    bool    `json:"fresh_readback_matches"`
	StateRoundtripMatches   bool    `json:"state_roundtrip_matches"`
	RollbackVerified        bool    `json:"rollback_verified"`
	RollbackReadbackMatches bool    `json:"rollback_readback_matches"`
	Error                   string  `json:"error,omitempty"`
}

type proQ3CoreEQScenario struct {
	ID             string                      `json:"id"`
	Purpose        string                      `json:"purpose"`
	SelectionBasis string                      `json:"selection_basis"`
	Configuration  []proQ3CoreEQConfigChange   `json:"configuration,omitempty"`
	TransactionID  string                      `json:"transaction_id,omitempty"`
	FreshReadback  bool                        `json:"fresh_readback_matches"`
	StateRoundtrip bool                        `json:"state_roundtrip_matches"`
	Render         *proQ3CoreEQRender          `json:"render,omitempty"`
	Rollback       proQ3CoreEQScenarioRollback `json:"rollback"`
	Error          string                      `json:"error,omitempty"`
}

type proQ3CoreEQConfigChange struct {
	Role                string  `json:"role"`
	ID                  string  `json:"id"`
	HostLabel           string  `json:"host_label"`
	RequestedNormalized float64 `json:"requested_normalized"`
	ObservedNormalized  float64 `json:"observed_normalized"`
	ObservedDisplay     string  `json:"observed_display"`
}

type proQ3CoreEQScenarioRollback struct {
	Verified            bool     `json:"verified"`
	FreshReadback       bool     `json:"fresh_readback_matches_initial"`
	ParameterMismatches []string `json:"parameter_mismatches,omitempty"`
	Error               string   `json:"error,omitempty"`
}

type proQ3CoreEQRender struct {
	OutputPath        string                         `json:"output_path"`
	OutputSHA256      string                         `json:"output_sha256"`
	Response          []proQ3CoreEQFrequencyResponse `json:"response_vs_bypass_db,omitempty"`
	BandwidthResponse []proQ3CoreEQFrequencyResponse `json:"bandwidth_response_vs_bypass_db,omitempty"`
	Peak              proQ3CoreEQExtreme             `json:"peak,omitempty"`
	Trough            proQ3CoreEQExtreme             `json:"trough,omitempty"`
	Bandwidth3DB      *proQ3CoreEQBandwidth          `json:"bandwidth_3db,omitempty"`
	MaxAbsoluteDB     float64                        `json:"max_absolute_delta_db"`
	AnalysisWarning   string                         `json:"analysis_warning,omitempty"`
}

type proQ3CoreEQFrequencyResponse struct {
	FrequencyHz float64 `json:"frequency_hz"`
	DeltaDB     float64 `json:"delta_db"`
}

type proQ3CoreEQExtreme struct {
	FrequencyHz float64 `json:"frequency_hz"`
	DeltaDB     float64 `json:"delta_db"`
}

type proQ3CoreEQBandwidth struct {
	LowerHz float64 `json:"lower_hz"`
	UpperHz float64 `json:"upper_hz"`
	Ratio   float64 `json:"ratio"`
}

type proQ3CoreEQRelation struct {
	ID          string         `json:"id"`
	Trust       string         `json:"trust"`
	Status      string         `json:"status"`
	Values      map[string]any `json:"values,omitempty"`
	Limitations []string       `json:"limitations,omitempty"`
}

type proQ3CoreEQFinalRollback struct {
	InitialStateRestoreVerified bool     `json:"initial_state_restore_verified"`
	FreshReadbackMatches        bool     `json:"fresh_readback_matches_initial"`
	ParameterMismatches         []string `json:"parameter_mismatches,omitempty"`
	Error                       string   `json:"error,omitempty"`
}

var proQ3CoreEQResponseFrequencies = []float64{
	20, 31.5, 50, 80, 125, 200, 315, 500, 630, 800, 1000, 1250,
	1600, 2000, 2500, 3150, 4000, 5000, 6300, 8000, 10000, 12500,
	16000, 20000,
}

var proQ3CoreEQBandwidthFrequencies = proQ3CoreEQLogFrequencyGrid(80, 8000, 121)

// RunProQ3CoreEQProbe measures a small set of static Pro-Q 3 states using a
// deterministic impulse. It validates controlled writes, fresh readback,
// state retention and rollback for every scenario. Numeric response deltas
// remain observed evidence; no relation emitted by this function is a
// reusable semantic mapping, Credential or SPAL route.
func RunProQ3CoreEQProbe(ctx context.Context, request ProQ3CoreEQProbeRequest) (result ProQ3CoreEQProbeResult, err error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return result, err
	}
	status, err := Inspect(root)
	if err != nil {
		return result, fmt.Errorf("inspect Pro-Q 3 core EQ workspace: %w", err)
	}
	if err := validateProQ3CoreEQProbeRequest(request); err != nil {
		return result, err
	}
	now := nowOrCurrent(request.Now)
	runID := stableID("pro_q_3_core_eq", status.Manifest.WorkspaceID, now.Format(time.RFC3339Nano))
	renderDirectory := filepath.Join(root, "renders", "pro_q_3_eq_core_probe", runID)
	runArtifact := filepath.ToSlash(filepath.Join(proQ3CoreEQEvidenceDir, runID+".json"))
	result = ProQ3CoreEQProbeResult{Workspace: root, ResultsArtifact: proQ3CoreEQResultsFile, RunArtifact: runArtifact, RenderDirectory: relativeStereoProbePath(root, renderDirectory)}
	artifact := proQ3CoreEQProbeArtifact{
		SchemaVersion: proQ3CoreEQProbeSchema,
		Trust:         "observed",
		Status:        "unsupported/unknown",
		CapturedAt:    now,
		WorkspaceID:   status.Manifest.WorkspaceID,
		Profile:       proQ3CoreEQProbeProfile,
		RunArtifact:   runArtifact,
		AuthorityBoundary: []string{
			"staging-only reference experiment; no VPS Library, Credential, Catalog or SPAL route is changed",
			"all display text, fresh readbacks and response measurements are observed evidence, not executable EQ semantics",
			"the experiment does not promote a task badge, conformance status or vendor-special capability",
			"human GUI testimony remains separately user-confirmed and cannot be promoted by this probe",
		},
	}
	adapter, err := newAdapterClient(request.HostURL, request.Client)
	if err != nil {
		return result, err
	}

	loaded := false
	initialValues := map[string]float64{}
	initialStateID := ""
	defer func() {
		if loaded && initialStateID != "" {
			_, _, restoreErr := adapter.operation(context.Background(), "/v1/plugin/state/restore", map[string]any{"state_id": initialStateID})
			artifact.FinalRollback.InitialStateRestoreVerified = restoreErr == nil
			if restoreErr != nil {
				artifact.FinalRollback.Error = restoreErr.Error()
				if err == nil {
					err = fmt.Errorf("restore complete initial Pro-Q 3 state in finalizer: %w", restoreErr)
				}
			}
		}
		if loaded && len(initialValues) > 0 {
			finalSnapshot, snapshotErr := adapter.snapshot(context.Background())
			if snapshotErr != nil {
				artifact.FinalRollback.Error = snapshotErr.Error()
				if err == nil {
					err = fmt.Errorf("fresh snapshot after Pro-Q 3 core EQ probe: %w", snapshotErr)
				}
			} else {
				artifact.FinalRollback.FreshReadbackMatches, artifact.FinalRollback.ParameterMismatches = stereoParameterMapsMatch(initialValues, finalSnapshot.Surface.Parameters)
				if !artifact.FinalRollback.FreshReadbackMatches && err == nil {
					err = fmt.Errorf("Pro-Q 3 core EQ probe final fresh readback does not match the complete initial preimage")
				}
			}
		}
		if err != nil {
			artifact.Status = "observed_failed"
			artifact.Errors = appendUniqueStereoString(artifact.Errors, err.Error())
		} else if len(artifact.Unknowns) > 0 {
			artifact.Status = "observed_incomplete"
		} else {
			artifact.Status = "observed_completed"
		}
		if persistErr := persistProQ3CoreEQArtifacts(root, request.HostURL, artifact); persistErr != nil && err == nil {
			err = persistErr
			artifact.Status = "observed_failed"
		}
		if loaded {
			_, _, _ = adapter.operation(context.Background(), "/v1/plugin/unload", map[string]any{})
		}
		result.Status = artifact.Status
	}()

	input, probeErr := loadProQ3CoreEQInput(request.ProbeAudioRoot)
	if probeErr != nil {
		err = probeErr
		return result, err
	}
	artifact.ProbeInput = input
	if _, err = adapter.request(ctx, http.MethodPost, "/v1/plugin/load", map[string]any{
		"plugin_path": status.Manifest.PluginIdentity.InstallPath,
		"sample_rate": 48000,
		"block_size":  512,
	}); err != nil {
		err = fmt.Errorf("load isolated VST3 for Pro-Q 3 core EQ probe: %w", err)
		return result, err
	}
	loaded = true

	snapshot, snapshotErr := adapter.snapshot(ctx)
	if snapshotErr != nil {
		err = fmt.Errorf("capture fresh Pro-Q 3 core EQ snapshot: %w", snapshotErr)
		return result, err
	}
	if identityErr := samePluginFamily(status.Manifest.PluginIdentity, snapshot.Identity); identityErr != nil {
		err = identityErr
		return result, err
	}
	if !sameInstallPath(status.Manifest.PluginIdentity.InstallPath, snapshot.Identity.InstallPath) {
		err = fmt.Errorf("loaded VST3 install path does not match the isolated workspace")
		return result, err
	}
	if !isProQ3Identity(snapshot.Identity) {
		err = fmt.Errorf("Pro-Q 3 core EQ probe only accepts the observed FabFilter Pro-Q 3 reference plugin")
		return result, err
	}
	initialValues = stereoParameterValueMap(snapshot.Surface.Parameters)

	controls, controlsErr := proQ3CoreEQObservedControls(snapshot.Surface.Parameters, request)
	if controlsErr != nil {
		err = controlsErr
		return result, err
	}
	artifact.ObservedControls = controls
	saved, _, saveErr := adapter.operation(ctx, "/v1/plugin/state/save", map[string]any{})
	if saveErr != nil {
		err = fmt.Errorf("save complete initial Pro-Q 3 state: %w", saveErr)
		return result, err
	}
	initialStateID = stringJSONField(saved, "state_id")
	if initialStateID == "" {
		err = fmt.Errorf("save complete initial Pro-Q 3 state omitted state_id")
		return result, err
	}

	shape, shapeErr := requireProQ3CoreEQParameter(snapshot.Surface.Parameters, request.BandShapeParameterID)
	if shapeErr != nil {
		err = shapeErr
		return result, err
	}
	artifact.ShapeDiscovery, err = discoverProQ3CoreEQChoices(ctx, adapter, initialStateID, shape, "Shape")
	if err != nil {
		return result, err
	}
	bell, bellFound := proQ3CoreEQChoiceByDisplay(artifact.ShapeDiscovery, "Bell")
	if !bellFound {
		err = fmt.Errorf("observed Band 1 Shape discovery did not include a Bell display state")
		return result, err
	}
	lowCut, lowCutFound := proQ3CoreEQChoiceByContains(artifact.ShapeDiscovery, "low", "cut")
	highCut, highCutFound := proQ3CoreEQChoiceByContains(artifact.ShapeDiscovery, "high", "cut")
	if !lowCutFound {
		artifact.Unknowns = append(artifact.Unknowns, "the observed Shape sweep did not expose a candidate display containing low and cut; low-cut response remains unsupported/unknown")
	}
	if !highCutFound {
		artifact.Unknowns = append(artifact.Unknowns, "the observed Shape sweep did not expose a candidate display containing high and cut; high-cut response remains unsupported/unknown")
	}
	slope, slopeErr := requireProQ3CoreEQParameter(snapshot.Surface.Parameters, request.BandSlopeParameterID)
	if slopeErr != nil {
		err = slopeErr
		return result, err
	}
	artifact.SlopeDiscovery, err = discoverProQ3CoreEQChoices(ctx, adapter, initialStateID, slope, "Slope")
	if err != nil {
		return result, err
	}
	slope24, slope24Found := proQ3CoreEQChoiceByDisplay(artifact.SlopeDiscovery, "24 dB/oct")
	if !slope24Found {
		artifact.Unknowns = append(artifact.Unknowns, "the observed Slope sweep did not expose the 24 dB/oct display candidate; the bounded pass-filter slope remains unsupported/unknown")
	}

	bypassPath := filepath.Join(renderDirectory, "00_bypass_impulse.wav")
	bypass, bypassErr := renderProQ3CoreEQ(ctx, adapter, input.Path, bypassPath, true, "")
	if bypassErr != nil {
		err = bypassErr
		return result, err
	}
	artifact.BypassRender = &bypass

	baseFrequency := observedCoreValue(snapshot.Surface.Parameters, request.BandFrequencyParameterID, 0.575188457965851)
	baseQ := observedCoreValue(snapshot.Surface.Parameters, request.BandQParameterID, 0.5)
	basePlacement := observedCoreValue(snapshot.Surface.Parameters, request.PlacementParameterID, 0.5)
	scenarios := []proQ3CoreEQScenario{
		proQ3CoreEQScenarioTemplate("bell_lower_normalized_boost", "Measure one bounded Bell boost at a lower normalized Frequency test value.", bell, 0.450, 0.700, baseQ, basePlacement),
		proQ3CoreEQScenarioTemplate("bell_higher_normalized_boost", "Measure the same bounded Bell boost at a higher normalized Frequency test value.", bell, 0.720, 0.700, baseQ, basePlacement),
		proQ3CoreEQScenarioTemplate("bell_center_normalized_cut", "Measure a bounded Bell cut at the initial observed Frequency test value.", bell, baseFrequency, 0.300, baseQ, basePlacement),
		proQ3CoreEQScenarioTemplate("bell_wider_q_candidate", "Measure a bounded Bell boost with a lower normalized Q test value.", bell, baseFrequency, 0.700, 0.300, basePlacement),
		proQ3CoreEQScenarioTemplate("bell_narrower_q_candidate", "Measure a bounded Bell boost with a higher normalized Q test value.", bell, baseFrequency, 0.700, 0.700, basePlacement),
		proQ3CoreEQScenarioTemplate("placement_neutral_left_candidate", "Measure a Bell state with neutral Gain and an observed Left placement candidate.", bell, baseFrequency, 0.500, baseQ, 0.0),
		proQ3CoreEQScenarioTemplate("placement_neutral_side_candidate", "Measure a Bell state with neutral Gain and an observed Side placement candidate.", bell, baseFrequency, 0.500, baseQ, 1.0),
	}
	if lowCutFound {
		scenarios = append(scenarios, proQ3CoreEQScenarioTemplate("low_cut_candidate", "Measure the observed Shape candidate whose display contains Low and Cut.", lowCut, baseFrequency, 0.500, baseQ, basePlacement))
	}
	if highCutFound {
		scenarios = append(scenarios, proQ3CoreEQScenarioTemplate("high_cut_candidate", "Measure the observed Shape candidate whose display contains High and Cut.", highCut, baseFrequency, 0.500, baseQ, basePlacement))
	}
	if lowCutFound && slope24Found {
		scenarios = append(scenarios, proQ3CoreEQPassFilterScenarioTemplate("low_cut_24_db_per_octave_candidate", "Measure the observed Low Cut and 24 dB/oct display candidates in one bounded pass-filter state.", lowCut, slope24, baseFrequency, baseQ, basePlacement))
	}
	for index := range scenarios {
		scenario := scenarios[index]
		completed, scenarioErr := runProQ3CoreEQScenario(ctx, adapter, initialStateID, initialValues, snapshot.Surface.Parameters, request, input.Path, renderDirectory, bypass.OutputPath, scenario)
		artifact.Scenarios = append(artifact.Scenarios, completed)
		if scenarioErr != nil {
			err = scenarioErr
			return result, err
		}
	}
	artifact.ObservedRelations = proQ3CoreEQRelations(artifact.Scenarios)
	return result, nil
}

func validateProQ3CoreEQProbeRequest(request ProQ3CoreEQProbeRequest) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"host URL", request.HostURL},
		{"probe audio root", request.ProbeAudioRoot},
		{"Band 1 Used parameter id", request.BandUsedParameterID},
		{"Band 1 Enabled parameter id", request.BandEnabledParameterID},
		{"Band 1 Frequency parameter id", request.BandFrequencyParameterID},
		{"Band 1 Gain parameter id", request.BandGainParameterID},
		{"Band 1 Q parameter id", request.BandQParameterID},
		{"Band 1 Shape parameter id", request.BandShapeParameterID},
		{"Band 1 Slope parameter id", request.BandSlopeParameterID},
		{"Band 1 Stereo Placement parameter id", request.PlacementParameterID},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("Pro-Q 3 core EQ probe %s is required", field.name)
		}
	}
	return nil
}

func isProQ3Identity(identity vps.PluginIdentity) bool {
	manufacturer := strings.ToLower(strings.TrimSpace(identity.Manufacturer))
	name := strings.ToLower(strings.TrimSpace(identity.Name))
	return strings.Contains(manufacturer, "fabfilter") && strings.Contains(name, "pro-q 3")
}

func loadProQ3CoreEQInput(probeRoot string) (proQ3CoreEQProbeInput, error) {
	probeRoot = filepath.Clean(strings.TrimSpace(probeRoot))
	var manifest probeaudio.Manifest
	if err := readJSON(filepath.Join(probeRoot, "manifest.json"), &manifest); err != nil {
		return proQ3CoreEQProbeInput{}, err
	}
	if manifest.SuiteID != probeaudio.SuiteID || manifest.SampleRate != 48000 || manifest.Channels != 2 {
		return proQ3CoreEQProbeInput{}, fmt.Errorf("Pro-Q 3 core EQ probe requires %s at 48 kHz stereo", probeaudio.SuiteID)
	}
	for _, asset := range manifest.Assets {
		if asset.ID != proQ3ImpulseProbeAsset {
			continue
		}
		path := filepath.Join(probeRoot, asset.File)
		digest, err := stereoSHA256File(path)
		if err != nil {
			return proQ3CoreEQProbeInput{}, err
		}
		if digest != asset.SHA256 {
			return proQ3CoreEQProbeInput{}, fmt.Errorf("Pro-Q 3 core EQ probe input fingerprint does not match the Probe Audio manifest")
		}
		return proQ3CoreEQProbeInput{
			AssetID: asset.ID,
			Path:    path,
			SHA256:  digest,
			Policy:  "same deterministic impulse bytes, worker rate and render configuration are used for bypass and every controlled state",
		}, nil
	}
	return proQ3CoreEQProbeInput{}, fmt.Errorf("Probe Audio suite lacks %s", proQ3ImpulseProbeAsset)
}

func proQ3CoreEQObservedControls(parameters []SurfaceParameter, request ProQ3CoreEQProbeRequest) ([]proQ3CoreEQObservedControl, error) {
	roles := []struct {
		role string
		id   string
	}{
		{"band_used", request.BandUsedParameterID},
		{"band_enabled", request.BandEnabledParameterID},
		{"band_frequency", request.BandFrequencyParameterID},
		{"band_gain", request.BandGainParameterID},
		{"band_q", request.BandQParameterID},
		{"band_shape", request.BandShapeParameterID},
		{"band_slope", request.BandSlopeParameterID},
		{"stereo_placement", request.PlacementParameterID},
	}
	controls := make([]proQ3CoreEQObservedControl, 0, len(roles))
	seen := map[string]bool{}
	for _, item := range roles {
		parameter, err := requireProQ3CoreEQParameter(parameters, item.id)
		if err != nil {
			return nil, err
		}
		if seen[parameter.ID] {
			return nil, fmt.Errorf("Pro-Q 3 core EQ probe observed control %q is duplicated", parameter.ID)
		}
		if parameter.NormalizedValue == nil {
			return nil, fmt.Errorf("Pro-Q 3 core EQ probe observed control %q has no normalized preimage", parameter.ID)
		}
		seen[parameter.ID] = true
		controls = append(controls, proQ3CoreEQObservedControl{
			Role: item.role, ID: parameter.ID, HostLabel: parameter.Name,
			InitialNormalized: *parameter.NormalizedValue, InitialDisplay: parameter.DisplayText,
			Discrete:            stereoObservedBool(parameter.Observed, "is_discrete"),
			ObservedNumSteps:    stereoObservedInt(parameter.Observed, "num_steps"),
			Automation:          parameter.Automation,
			SelectionLimitation: "selected from an already observed stable VST3 parameter surface; the host label and ID are not executable semantic authority",
		})
	}
	return controls, nil
}

func requireProQ3CoreEQParameter(parameters []SurfaceParameter, id string) (SurfaceParameter, error) {
	parameter, ok := findSurfaceParameter(parameters, strings.TrimSpace(id))
	if !ok {
		return SurfaceParameter{}, fmt.Errorf("observed Pro-Q 3 core EQ parameter id %q is not in the current host surface", id)
	}
	if !parameter.HostControllable || !strings.EqualFold(parameter.Automation, "automatable") {
		return SurfaceParameter{}, fmt.Errorf("observed Pro-Q 3 core EQ parameter %q is not host-automatable", id)
	}
	return parameter, nil
}

func observedCoreValue(parameters []SurfaceParameter, id string, fallback float64) float64 {
	parameter, found := findSurfaceParameter(parameters, id)
	if !found || parameter.NormalizedValue == nil || !finiteHostValue(*parameter.NormalizedValue) {
		return fallback
	}
	return *parameter.NormalizedValue
}

func discoverProQ3CoreEQChoices(ctx context.Context, adapter *adapterClient, initialStateID string, parameter SurfaceParameter, controlName string) ([]proQ3CoreEQChoice, error) {
	if parameter.NormalizedValue == nil {
		return nil, fmt.Errorf("%s parameter %q has no normalized preimage", controlName, parameter.ID)
	}
	numSteps := stereoObservedInt(parameter.Observed, "num_steps")
	if numSteps < 2 || numSteps > 16 || !stereoObservedBool(parameter.Observed, "is_discrete") {
		return nil, fmt.Errorf("%s parameter %q is not a bounded discrete surface", controlName, parameter.ID)
	}
	preimage := *parameter.NormalizedValue
	choices := make([]proQ3CoreEQChoice, 0, numSteps)
	for index := 0; index < numSteps; index++ {
		if _, _, err := adapter.operation(ctx, "/v1/plugin/state/restore", map[string]any{"state_id": initialStateID}); err != nil {
			return choices, fmt.Errorf("restore initial state before %s discovery %d: %w", controlName, index, err)
		}
		requested := float64(index) / float64(numSteps-1)
		choice := proQ3CoreEQChoice{RequestedNormalized: requested}
		write, _, writeErr := adapter.operation(ctx, "/v1/plugin/parameters/write", map[string]any{"changes": []map[string]any{{"id": parameter.ID, "normalized": requested}}})
		if writeErr != nil {
			choice.Error = writeErr.Error()
			choices = append(choices, choice)
			return choices, fmt.Errorf("write observed %s candidate %.6f: %w", controlName, requested, writeErr)
		}
		choice.TransactionID = stringJSONField(write, "transaction_id")
		choice.FreshReadbackMatches = choice.TransactionID != "" && freshReadbackMatchesJSON(write, parameter.ID, requested)
		if fresh, found := stereoFreshReadbackParameter(write, parameter.ID); found {
			choice.ObservedDisplay = fresh.DisplayValue
		}
		if !choice.FreshReadbackMatches || strings.TrimSpace(choice.ObservedDisplay) == "" {
			choice.Error = controlName + " discovery write omitted matching fresh readback or display text"
			choices = append(choices, choice)
			return choices, fmt.Errorf("%s discovery candidate %.6f lacks required fresh readback", controlName, requested)
		}
		roundtrip, _, roundtripErr := adapter.operation(ctx, "/v1/plugin/state/roundtrip", map[string]any{})
		choice.StateRoundtripMatches = roundtripErr == nil && boolJSONField(roundtrip, "parameter_readback_matches_preimage") && freshReadbackMatchesJSON(roundtrip, parameter.ID, requested)
		rollback, _, rollbackErr := adapter.operation(ctx, "/v1/plugin/rollback", map[string]any{"transaction_id": choice.TransactionID})
		choice.RollbackVerified = rollbackErr == nil && boolJSONField(rollback, "rollback_verified")
		choice.RollbackReadbackMatches = rollbackErr == nil && freshReadbackMatchesJSON(rollback, parameter.ID, preimage)
		if roundtripErr != nil || !choice.StateRoundtripMatches || rollbackErr != nil || !choice.RollbackVerified || !choice.RollbackReadbackMatches {
			choice.Error = controlName + " discovery did not satisfy state-roundtrip or rollback verification"
			choices = append(choices, choice)
			return choices, fmt.Errorf("%s discovery candidate %.6f did not satisfy rollback requirements", controlName, requested)
		}
		choices = append(choices, choice)
	}
	return choices, nil
}

func proQ3CoreEQChoiceByDisplay(choices []proQ3CoreEQChoice, display string) (proQ3CoreEQChoice, bool) {
	for _, choice := range choices {
		if strings.EqualFold(strings.TrimSpace(choice.ObservedDisplay), strings.TrimSpace(display)) {
			return choice, true
		}
	}
	return proQ3CoreEQChoice{}, false
}

func proQ3CoreEQChoiceByContains(choices []proQ3CoreEQChoice, terms ...string) (proQ3CoreEQChoice, bool) {
	for _, choice := range choices {
		text := strings.ToLower(strings.TrimSpace(choice.ObservedDisplay))
		matched := true
		for _, term := range terms {
			if !strings.Contains(text, strings.ToLower(strings.TrimSpace(term))) {
				matched = false
				break
			}
		}
		if matched {
			return choice, true
		}
	}
	return proQ3CoreEQChoice{}, false
}

func proQ3CoreEQScenarioTemplate(id, purpose string, shape proQ3CoreEQChoice, frequency, gain, q, placement float64) proQ3CoreEQScenario {
	return proQ3CoreEQScenario{
		ID:             id,
		Purpose:        purpose,
		SelectionBasis: "bounded normalized test values plus fresh observed display readback; a candidate Shape display selection is not a reusable semantic binding",
		Configuration: []proQ3CoreEQConfigChange{
			{Role: "band_used", RequestedNormalized: 1},
			{Role: "band_enabled", RequestedNormalized: 1},
			{Role: "band_frequency", RequestedNormalized: frequency},
			{Role: "band_gain", RequestedNormalized: gain},
			{Role: "band_q", RequestedNormalized: q},
			{Role: "band_shape", RequestedNormalized: shape.RequestedNormalized},
			{Role: "stereo_placement", RequestedNormalized: placement},
		},
	}
}

// proQ3CoreEQPassFilterScenarioTemplate is deliberately limited to observed
// display candidates.  It measures a bounded state only; it does not turn a
// parameter label or a rendered response into a generic pass-filter mapping.
func proQ3CoreEQPassFilterScenarioTemplate(id, purpose string, shape, slope proQ3CoreEQChoice, frequency, q, placement float64) proQ3CoreEQScenario {
	scenario := proQ3CoreEQScenarioTemplate(id, purpose, shape, frequency, 0.5, q, placement)
	scenario.Configuration = append(scenario.Configuration, proQ3CoreEQConfigChange{Role: "band_slope", RequestedNormalized: slope.RequestedNormalized})
	scenario.SelectionBasis = "bounded normalized test values plus fresh observed Shape and Slope display readback; the displayed Low Cut / 24 dB/oct candidates are not reusable semantic bindings"
	return scenario
}

func runProQ3CoreEQScenario(ctx context.Context, adapter *adapterClient, initialStateID string, initialValues map[string]float64, parameters []SurfaceParameter, request ProQ3CoreEQProbeRequest, inputPath, renderDirectory, bypassPath string, scenario proQ3CoreEQScenario) (result proQ3CoreEQScenario, err error) {
	rollbackRequired := false
	defer func() {
		if rollbackRequired && scenario.TransactionID != "" {
			rollback, _, rollbackErr := adapter.operation(context.Background(), "/v1/plugin/rollback", map[string]any{"transaction_id": scenario.TransactionID})
			scenario.Rollback.Verified = rollbackErr == nil && boolJSONField(rollback, "rollback_verified")
			if rollbackErr != nil {
				scenario.Rollback.Error = rollbackErr.Error()
				if err == nil {
					err = fmt.Errorf("emergency rollback %s: %w", scenario.ID, rollbackErr)
				}
			} else if fresh, snapshotErr := adapter.snapshot(context.Background()); snapshotErr != nil {
				scenario.Rollback.Error = snapshotErr.Error()
				if err == nil {
					err = fmt.Errorf("fresh snapshot after emergency rollback %s: %w", scenario.ID, snapshotErr)
				}
			} else {
				scenario.Rollback.FreshReadback, scenario.Rollback.ParameterMismatches = stereoParameterMapsMatch(initialValues, fresh.Surface.Parameters)
				if !scenario.Rollback.Verified || !scenario.Rollback.FreshReadback {
					scenario.Rollback.Error = "emergency rollback was not verified against the complete initial parameter preimage"
					if err == nil {
						err = fmt.Errorf("emergency rollback %s did not restore the complete initial preimage", scenario.ID)
					}
				}
			}
		}
		if err != nil && scenario.Error == "" {
			scenario.Error = err.Error()
		}
		result = scenario
	}()
	if _, _, err := adapter.operation(ctx, "/v1/plugin/state/restore", map[string]any{"state_id": initialStateID}); err != nil {
		scenario.Error = err.Error()
		return scenario, fmt.Errorf("restore initial state before %s: %w", scenario.ID, err)
	}
	changes, err := proQ3CoreEQBindChanges(parameters, request, scenario.Configuration)
	if err != nil {
		scenario.Error = err.Error()
		return scenario, err
	}
	scenario.Configuration = changes
	payload := make([]map[string]any, 0, len(changes))
	for _, change := range changes {
		payload = append(payload, map[string]any{"id": change.ID, "normalized": change.RequestedNormalized})
	}
	write, _, writeErr := adapter.operation(ctx, "/v1/plugin/parameters/write", map[string]any{"changes": payload})
	if writeErr != nil {
		scenario.Error = writeErr.Error()
		return scenario, fmt.Errorf("write %s: %w", scenario.ID, writeErr)
	}
	scenario.TransactionID = stringJSONField(write, "transaction_id")
	rollbackRequired = scenario.TransactionID != ""
	scenario.FreshReadback = scenario.TransactionID != ""
	for index := range scenario.Configuration {
		if !freshReadbackMatchesJSON(write, scenario.Configuration[index].ID, scenario.Configuration[index].RequestedNormalized) {
			scenario.FreshReadback = false
		}
		if value, found := stereoFreshReadbackParameter(write, scenario.Configuration[index].ID); found {
			scenario.Configuration[index].ObservedNormalized = value.NormalizedValue
			scenario.Configuration[index].ObservedDisplay = value.DisplayValue
		}
	}
	if !scenario.FreshReadback {
		scenario.Error = "controlled write omitted a transaction id or matching fresh readback"
		return scenario, fmt.Errorf("%s lacks required fresh readback", scenario.ID)
	}
	roundtrip, _, roundtripErr := adapter.operation(ctx, "/v1/plugin/state/roundtrip", map[string]any{})
	scenario.StateRoundtrip = roundtripErr == nil && boolJSONField(roundtrip, "parameter_readback_matches_preimage")
	if scenario.StateRoundtrip {
		for _, change := range scenario.Configuration {
			if !freshReadbackMatchesJSON(roundtrip, change.ID, change.RequestedNormalized) {
				scenario.StateRoundtrip = false
				break
			}
		}
	}
	if roundtripErr != nil || !scenario.StateRoundtrip {
		scenario.Error = "controlled state did not survive state serialization/reload with matching fresh readback"
		return scenario, fmt.Errorf("%s state roundtrip did not retain the controlled configuration", scenario.ID)
	}
	output := filepath.Join(renderDirectory, scenario.ID+".wav")
	render, renderErr := renderProQ3CoreEQ(ctx, adapter, inputPath, output, false, bypassPath)
	if renderErr != nil {
		scenario.Error = renderErr.Error()
		return scenario, renderErr
	}
	if strings.Contains(scenario.ID, "_q_candidate") {
		dense, denseErr := proQ3CoreEQImpulseResponseAtFrequencies(bypassPath, output, proQ3CoreEQBandwidthFrequencies)
		if denseErr != nil {
			scenario.Error = denseErr.Error()
			return scenario, denseErr
		}
		render.BandwidthResponse = dense
		render.Bandwidth3DB = proQ3CoreEQResponseBandwidth(dense)
	}
	scenario.Render = &render
	rollback, _, rollbackErr := adapter.operation(ctx, "/v1/plugin/rollback", map[string]any{"transaction_id": scenario.TransactionID})
	scenario.Rollback.Verified = rollbackErr == nil && boolJSONField(rollback, "rollback_verified")
	if rollbackErr != nil {
		scenario.Rollback.Error = rollbackErr.Error()
		scenario.Error = rollbackErr.Error()
		return scenario, fmt.Errorf("rollback %s: %w", scenario.ID, rollbackErr)
	}
	if scenario.Rollback.Verified {
		rollbackRequired = false
	}
	fresh, freshErr := adapter.snapshot(ctx)
	if freshErr != nil {
		scenario.Rollback.Error = freshErr.Error()
		scenario.Error = freshErr.Error()
		return scenario, fmt.Errorf("fresh snapshot after rollback %s: %w", scenario.ID, freshErr)
	}
	scenario.Rollback.FreshReadback, scenario.Rollback.ParameterMismatches = stereoParameterMapsMatch(initialValues, fresh.Surface.Parameters)
	if !scenario.Rollback.Verified || !scenario.Rollback.FreshReadback {
		scenario.Rollback.Error = "rollback was not verified against the complete initial parameter preimage"
		scenario.Error = scenario.Rollback.Error
		return scenario, fmt.Errorf("rollback %s did not restore the complete initial preimage", scenario.ID)
	}
	return scenario, nil
}

func proQ3CoreEQBindChanges(parameters []SurfaceParameter, request ProQ3CoreEQProbeRequest, changes []proQ3CoreEQConfigChange) ([]proQ3CoreEQConfigChange, error) {
	ids := map[string]string{
		"band_used":        request.BandUsedParameterID,
		"band_enabled":     request.BandEnabledParameterID,
		"band_frequency":   request.BandFrequencyParameterID,
		"band_gain":        request.BandGainParameterID,
		"band_q":           request.BandQParameterID,
		"band_shape":       request.BandShapeParameterID,
		"band_slope":       request.BandSlopeParameterID,
		"stereo_placement": request.PlacementParameterID,
	}
	seen := map[string]bool{}
	for index := range changes {
		id, ok := ids[changes[index].Role]
		if !ok {
			return nil, fmt.Errorf("unknown Pro-Q 3 core EQ role %q", changes[index].Role)
		}
		parameter, err := requireProQ3CoreEQParameter(parameters, id)
		if err != nil {
			return nil, err
		}
		if seen[parameter.ID] {
			return nil, fmt.Errorf("Pro-Q 3 core EQ configuration duplicates parameter id %q", parameter.ID)
		}
		if changes[index].RequestedNormalized < 0 || changes[index].RequestedNormalized > 1 || !finiteHostValue(changes[index].RequestedNormalized) {
			return nil, fmt.Errorf("Pro-Q 3 core EQ role %q has invalid normalized test value", changes[index].Role)
		}
		seen[parameter.ID] = true
		changes[index].ID = parameter.ID
		changes[index].HostLabel = parameter.Name
	}
	return changes, nil
}

func renderProQ3CoreEQ(ctx context.Context, adapter *adapterClient, inputPath, outputPath string, bypass bool, bypassPath string) (proQ3CoreEQRender, error) {
	render := proQ3CoreEQRender{OutputPath: outputPath}
	if _, _, err := adapter.operation(ctx, "/v1/plugin/render", map[string]any{"input_path": inputPath, "output_path": outputPath, "bypass": bypass}); err != nil {
		return render, fmt.Errorf("render %s: %w", filepath.Base(outputPath), err)
	}
	digest, err := stereoSHA256File(outputPath)
	if err != nil {
		return render, err
	}
	render.OutputSHA256 = digest
	if bypass || strings.TrimSpace(bypassPath) == "" {
		return render, nil
	}
	response, err := proQ3CoreEQImpulseResponse(bypassPath, outputPath)
	if err != nil {
		return render, err
	}
	render.Response = response
	render.Peak = proQ3CoreEQExtremeFromResponse(response, true)
	render.Trough = proQ3CoreEQExtremeFromResponse(response, false)
	render.Bandwidth3DB = proQ3CoreEQResponseBandwidth(response)
	for _, point := range response {
		render.MaxAbsoluteDB = math.Max(render.MaxAbsoluteDB, math.Abs(point.DeltaDB))
	}
	return render, nil
}

func proQ3CoreEQImpulseResponse(bypassPath, processedPath string) ([]proQ3CoreEQFrequencyResponse, error) {
	return proQ3CoreEQImpulseResponseAtFrequencies(bypassPath, processedPath, proQ3CoreEQResponseFrequencies)
}

func proQ3CoreEQImpulseResponseAtFrequencies(bypassPath, processedPath string, frequencies []float64) ([]proQ3CoreEQFrequencyResponse, error) {
	baseRate, baseLeft, baseRight, err := readStereoPCM(bypassPath)
	if err != nil {
		return nil, err
	}
	processedRate, processedLeft, processedRight, err := readStereoPCM(processedPath)
	if err != nil {
		return nil, err
	}
	if baseRate != processedRate || len(baseLeft) == 0 || len(processedLeft) == 0 {
		return nil, fmt.Errorf("impulse response renders do not share a usable sample rate")
	}
	frames := minCoreInt(proQ3CoreResponseFrames, minCoreInt(len(baseLeft), len(processedLeft)))
	if frames < 1024 {
		return nil, fmt.Errorf("impulse response render is too short for response analysis")
	}
	base := make([]float64, frames)
	processed := make([]float64, frames)
	for index := 0; index < frames; index++ {
		base[index] = (baseLeft[index] + baseRight[index]) * 0.5
		processed[index] = (processedLeft[index] + processedRight[index]) * 0.5
	}
	response := make([]proQ3CoreEQFrequencyResponse, 0, len(frequencies))
	for _, frequency := range frequencies {
		baseMagnitude := proQ3CoreEQDFTMagnitude(base, frequency, float64(baseRate))
		processedMagnitude := proQ3CoreEQDFTMagnitude(processed, frequency, float64(baseRate))
		if baseMagnitude <= 1e-14 {
			return nil, fmt.Errorf("bypass impulse DFT magnitude is too small at %.2f Hz", frequency)
		}
		response = append(response, proQ3CoreEQFrequencyResponse{FrequencyHz: frequency, DeltaDB: 20 * math.Log10(math.Max(processedMagnitude, 1e-14)/baseMagnitude)})
	}
	return response, nil
}

func proQ3CoreEQLogFrequencyGrid(lower, upper float64, count int) []float64 {
	if lower <= 0 || upper <= lower || count < 2 {
		return nil
	}
	values := make([]float64, 0, count)
	ratio := upper / lower
	for index := 0; index < count; index++ {
		values = append(values, lower*math.Pow(ratio, float64(index)/float64(count-1)))
	}
	return values
}

func proQ3CoreEQDFTMagnitude(samples []float64, frequency, sampleRate float64) float64 {
	if frequency <= 0 || sampleRate <= 0 || len(samples) == 0 {
		return 0
	}
	real, imaginary := 0.0, 0.0
	angular := 2 * math.Pi * frequency / sampleRate
	for index, sample := range samples {
		phase := angular * float64(index)
		real += sample * math.Cos(phase)
		imaginary -= sample * math.Sin(phase)
	}
	return math.Hypot(real, imaginary)
}

func proQ3CoreEQExtremeFromResponse(response []proQ3CoreEQFrequencyResponse, maximum bool) proQ3CoreEQExtreme {
	if len(response) == 0 {
		return proQ3CoreEQExtreme{}
	}
	candidates := append([]proQ3CoreEQFrequencyResponse(nil), response...)
	sort.Slice(candidates, func(i, j int) bool {
		if maximum {
			return candidates[i].DeltaDB > candidates[j].DeltaDB
		}
		return candidates[i].DeltaDB < candidates[j].DeltaDB
	})
	return proQ3CoreEQExtreme{FrequencyHz: candidates[0].FrequencyHz, DeltaDB: candidates[0].DeltaDB}
}

func proQ3CoreEQResponseBandwidth(response []proQ3CoreEQFrequencyResponse) *proQ3CoreEQBandwidth {
	if len(response) == 0 {
		return nil
	}
	peak := proQ3CoreEQExtremeFromResponse(response, true)
	if peak.DeltaDB < 1 {
		return nil
	}
	threshold := peak.DeltaDB - 3
	within := []proQ3CoreEQFrequencyResponse{}
	for _, point := range response {
		if point.DeltaDB >= threshold {
			within = append(within, point)
		}
	}
	if len(within) < 2 || within[0].FrequencyHz <= 0 {
		return nil
	}
	return &proQ3CoreEQBandwidth{LowerHz: within[0].FrequencyHz, UpperHz: within[len(within)-1].FrequencyHz, Ratio: within[len(within)-1].FrequencyHz / within[0].FrequencyHz}
}

func proQ3CoreEQRelations(scenarios []proQ3CoreEQScenario) []proQ3CoreEQRelation {
	byID := map[string]proQ3CoreEQScenario{}
	for _, scenario := range scenarios {
		byID[scenario.ID] = scenario
	}
	relations := []proQ3CoreEQRelation{}
	appendRelation := func(id string, values map[string]any, limitation string) {
		relations = append(relations, proQ3CoreEQRelation{ID: id, Trust: "observed", Status: "measured_not_semantic", Values: values, Limitations: []string{limitation, "numeric response data does not grant an executable semantic mapping, badge or Credential"}})
	}
	if lower, higher := byID["bell_lower_normalized_boost"], byID["bell_higher_normalized_boost"]; lower.Render != nil && higher.Render != nil {
		appendRelation("normalized_frequency_test_peak_positions", map[string]any{"lower_normalized_peak_hz": lower.Render.Peak.FrequencyHz, "higher_normalized_peak_hz": higher.Render.Peak.FrequencyHz, "lower_normalized_peak_db": lower.Render.Peak.DeltaDB, "higher_normalized_peak_db": higher.Render.Peak.DeltaDB}, "test values are normalized host values; any frequency interpretation remains bounded to this plug-in version and test state")
	}
	if cut, ok := byID["bell_center_normalized_cut"]; ok && cut.Render != nil {
		appendRelation("bell_cut_response_trough", map[string]any{"trough_hz": cut.Render.Trough.FrequencyHz, "trough_db": cut.Render.Trough.DeltaDB}, "a deterministic impulse response is a static measurement, not a musical-quality conclusion")
	}
	if wider, narrower := byID["bell_wider_q_candidate"], byID["bell_narrower_q_candidate"]; wider.Render != nil && narrower.Render != nil {
		values := map[string]any{"wider_candidate_peak_hz": wider.Render.Peak.FrequencyHz, "narrower_candidate_peak_hz": narrower.Render.Peak.FrequencyHz}
		if wider.Render.Bandwidth3DB != nil {
			values["wider_candidate_bandwidth_ratio"] = wider.Render.Bandwidth3DB.Ratio
		}
		if narrower.Render.Bandwidth3DB != nil {
			values["narrower_candidate_bandwidth_ratio"] = narrower.Render.Bandwidth3DB.Ratio
		}
		appendRelation("q_candidate_response_width", values, "candidate names only describe normalized test ordering; Q semantics require separate review and conformance")
	}
	for _, id := range []string{"placement_neutral_left_candidate", "placement_neutral_side_candidate", "low_cut_candidate", "high_cut_candidate", "low_cut_24_db_per_octave_candidate"} {
		if scenario, ok := byID[id]; ok && scenario.Render != nil {
			appendRelation(id+"_response_delta", map[string]any{"max_absolute_delta_db": scenario.Render.MaxAbsoluteDB, "peak_hz": scenario.Render.Peak.FrequencyHz, "peak_db": scenario.Render.Peak.DeltaDB, "trough_hz": scenario.Render.Trough.FrequencyHz, "trough_db": scenario.Render.Trough.DeltaDB}, "the observed display candidate was chosen only for this staging test and is not a dispatchable control meaning")
		}
	}
	return relations
}

func persistProQ3CoreEQArtifacts(root, hostURL string, artifact proQ3CoreEQProbeArtifact) error {
	if err := writeJSON(filepath.Join(root, proQ3CoreEQResultsFile), artifact); err != nil {
		return err
	}
	runArtifact := filepath.FromSlash(strings.TrimSpace(artifact.RunArtifact))
	if runArtifact == "" {
		return fmt.Errorf("Pro-Q 3 core EQ probe run artifact path is required")
	}
	if err := writeJSON(filepath.Join(root, runArtifact), artifact); err != nil {
		return err
	}
	data, _ := json.Marshal(artifact)
	if err := appendEvidence(root, EvidenceEntry{
		Kind:       "pro_q_3_core_eq_behavior_probe",
		Trust:      "observed",
		Source:     strings.TrimSpace(hostURL),
		Summary:    "Captured controlled static Pro-Q 3 renders with preimage-protected writes, fresh readback, state roundtrip and rollback. Numeric response data is not semantic conformance and does not issue a Credential.",
		Artifact:   filepath.ToSlash(runArtifact),
		CapturedAt: artifact.CapturedAt,
		Data:       data,
	}); err != nil {
		return err
	}
	return updateManifest(root, func(current *Manifest) {
		if current.Artifacts == nil {
			current.Artifacts = map[string]string{}
		}
		current.Artifacts["pro_q_3_eq_core_probe_results"] = proQ3CoreEQResultsFile
		current.UpdatedAt = artifact.CapturedAt
	})
}

func minCoreInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

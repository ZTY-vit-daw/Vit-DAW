package vpsforge

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vit-daw-agent/internal/probeaudio"
)

const (
	stereoPlacementProbeSchema  = "vit.vpsforge.stereo_placement_probe.v1"
	stereoPlacementProbeProfile = "fabfilter.pro-q-3.band-1.stereo-placement.v1"
	stereoPlacementProbeAssetID = "stereo_phase_probe"
	stereoDeltaSilenceFloorDBFS = -120.0
)

// StereoPlacementProbeRequest is intentionally a narrow, staging-only
// reference experiment. It uses exact observed parameter IDs from one Pro-Q 3
// workspace; it does not infer a reusable routing schema or grant a badge.
type StereoPlacementProbeRequest struct {
	Root           string
	HostURL        string
	ProbeAudioRoot string

	BandUsedParameterID      string
	BandEnabledParameterID   string
	BandFrequencyParameterID string
	BandGainParameterID      string
	BandQParameterID         string
	BandShapeParameterID     string
	PlacementParameterID     string

	Client *http.Client
	Now    time.Time
}

type StereoPlacementProbeResult struct {
	Workspace       string `json:"workspace"`
	Status          string `json:"status"`
	ResultsArtifact string `json:"results_artifact"`
	ListeningGuide  string `json:"listening_guide"`
	RenderDirectory string `json:"render_directory"`
}

type stereoPlacementProbeArtifact struct {
	SchemaVersion string    `json:"schema_version"`
	Trust         string    `json:"trust"`
	Status        string    `json:"status"`
	CapturedAt    time.Time `json:"captured_at"`
	WorkspaceID   string    `json:"workspace_id"`
	Profile       string    `json:"profile"`

	AuthorityBoundary []string                   `json:"authority_boundary"`
	ProbeInput        stereoProbeInput           `json:"probe_input,omitempty"`
	ObservedPlacement stereoPlacementParameter   `json:"observed_placement_parameter,omitempty"`
	Discovery         []stereoPlacementCandidate `json:"placement_discovery,omitempty"`
	Configuration     stereoProbeConfiguration   `json:"configuration,omitempty"`
	BypassRender      *stereoPlacementRender     `json:"bypass_render,omitempty"`
	PlacementRenders  []stereoPlacementRender    `json:"placement_renders,omitempty"`
	FinalRollback     stereoPlacementRollback    `json:"final_rollback,omitempty"`
	ListeningPack     stereoListeningPack        `json:"listening_pack"`
	MetricLimitations []string                   `json:"metric_limitations,omitempty"`
	Errors            []string                   `json:"errors,omitempty"`
}

type stereoProbeInput struct {
	AssetID string `json:"asset_id"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Policy  string `json:"policy"`
}

type stereoPlacementParameter struct {
	ID                string  `json:"id"`
	HostLabel         string  `json:"host_label"`
	InitialNormalized float64 `json:"initial_normalized"`
	InitialDisplay    string  `json:"initial_display"`
	NumSteps          int     `json:"num_steps"`
	Automation        string  `json:"automation"`
}

type stereoPlacementCandidate struct {
	RequestedNormalized     float64 `json:"requested_normalized"`
	ObservedDisplayValue    string  `json:"observed_display_value"`
	TransactionID           string  `json:"transaction_id"`
	FreshReadbackMatches    bool    `json:"fresh_readback_matches"`
	StateRoundtripMatches   bool    `json:"state_roundtrip_matches"`
	RollbackVerified        bool    `json:"rollback_verified"`
	RollbackReadbackMatches bool    `json:"rollback_readback_matches"`
	Error                   string  `json:"error,omitempty"`
}

type stereoProbeConfiguration struct {
	Basis         string                    `json:"basis"`
	Changes       []stereoProbeConfigChange `json:"changes"`
	StateID       string                    `json:"state_id,omitempty"`
	TransactionID string                    `json:"transaction_id,omitempty"`
}

type stereoProbeConfigChange struct {
	Role                string  `json:"role"`
	ID                  string  `json:"id"`
	HostLabel           string  `json:"host_label"`
	RequestedNormalized float64 `json:"requested_normalized"`
	ObservedNormalized  float64 `json:"observed_normalized"`
	ObservedDisplay     string  `json:"observed_display"`
}

type stereoPlacementRender struct {
	PlacementNormalized     float64                        `json:"placement_normalized"`
	ObservedDisplay         string                         `json:"observed_display"`
	TransactionID           string                         `json:"transaction_id,omitempty"`
	FreshReadbackMatches    bool                           `json:"fresh_readback_matches"`
	StateRoundtripMatches   bool                           `json:"state_roundtrip_matches"`
	OutputPath              string                         `json:"output_path"`
	OutputSHA256            string                         `json:"output_sha256"`
	Metrics                 []stereoPlacementSegmentMetric `json:"metrics"`
	DeltaFromBypass         []stereoPlacementSegmentDelta  `json:"delta_from_bypass,omitempty"`
	RollbackVerified        bool                           `json:"rollback_verified"`
	RollbackReadbackMatches bool                           `json:"rollback_readback_matches"`
	Error                   string                         `json:"error,omitempty"`
}

type stereoPlacementSegmentMetric struct {
	Segment      string  `json:"segment"`
	StartSeconds float64 `json:"start_seconds"`
	EndSeconds   float64 `json:"end_seconds"`
	Frames       int     `json:"frames"`
	LeftRMSDBFS  float64 `json:"left_rms_dbfs"`
	RightRMSDBFS float64 `json:"right_rms_dbfs"`
	MidRMSDBFS   float64 `json:"mid_rms_dbfs"`
	SideRMSDBFS  float64 `json:"side_rms_dbfs"`
}

type stereoPlacementSegmentDelta struct {
	Segment               string   `json:"segment"`
	LeftDeltaDB           *float64 `json:"left_delta_db"`
	RightDeltaDB          *float64 `json:"right_delta_db"`
	MidDeltaDB            *float64 `json:"mid_delta_db"`
	SideDeltaDB           *float64 `json:"side_delta_db"`
	SilentReferenceFields []string `json:"silent_reference_fields,omitempty"`
}

type stereoPlacementRollback struct {
	TransactionID               string   `json:"transaction_id,omitempty"`
	HostRollbackVerified        bool     `json:"host_rollback_verified"`
	FreshReadbackMatchesInitial bool     `json:"fresh_readback_matches_initial"`
	ParameterMismatches         []string `json:"parameter_mismatches,omitempty"`
	Error                       string   `json:"error,omitempty"`
}

type stereoListeningPack struct {
	GuidePath       string   `json:"guide_path"`
	RenderDirectory string   `json:"render_directory"`
	Files           []string `json:"files"`
	ManualStatus    string   `json:"manual_status"`
	Instructions    []string `json:"instructions"`
}

type stereoRawParameter struct {
	ID              string  `json:"id"`
	HostLabel       string  `json:"host_label"`
	NormalizedValue float64 `json:"normalized_value"`
	DisplayValue    string  `json:"display_value"`
}

// RunStereoPlacementProbe produces objective, repeatable audio measurements
// and an offline comparison pack. Automated results remain observed. The pack
// is an audit artifact, not a substitute for a user listening concurrently
// while operating the live witness GUI. Every write obtains a worker preimage
// and rollback; the final configuration write is rolled back before the worker
// is unloaded.
func RunStereoPlacementProbe(ctx context.Context, request StereoPlacementProbeRequest) (result StereoPlacementProbeResult, err error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return StereoPlacementProbeResult{}, err
	}
	status, err := Inspect(root)
	if err != nil {
		return StereoPlacementProbeResult{}, fmt.Errorf("inspect stereo placement workspace: %w", err)
	}
	if err := validateStereoPlacementRequest(request); err != nil {
		return StereoPlacementProbeResult{}, err
	}
	now := nowOrCurrent(request.Now)
	runID := stableID("stereo_placement", status.Manifest.WorkspaceID, now.Format(time.RFC3339Nano))
	renderDirectory := filepath.Join(root, "renders", "stereo_placement_probe", runID)
	artifact := stereoPlacementProbeArtifact{
		SchemaVersion: stereoPlacementProbeSchema,
		Trust:         "observed",
		Status:        "unsupported/unknown",
		CapturedAt:    now,
		WorkspaceID:   status.Manifest.WorkspaceID,
		Profile:       stereoPlacementProbeProfile,
		AuthorityBoundary: []string{
			"staging-only render experiment; no VPS Library, Credential, Catalog or SPAL routing mutation",
			"display labels and render metrics are observed evidence, not semantic conformance",
			"concurrent live human listening remains pending; offline renders cannot grant a routing or audible-quality claim automatically",
		},
		ListeningPack: stereoListeningPack{
			GuidePath:       stereoPlacementGuideFile,
			RenderDirectory: relativeStereoProbePath(root, renderDirectory),
			ManualStatus:    "unsupported/unknown",
		},
		MetricLimitations: []string{"a delta is null when the bypass window is at or below -120 dBFS; use the absolute observed metric instead of interpreting a silence-floor ratio"},
	}
	result = StereoPlacementProbeResult{
		Workspace:       root,
		ResultsArtifact: stereoPlacementResultsFile,
		ListeningGuide:  stereoPlacementGuideFile,
		RenderDirectory: relativeStereoProbePath(root, renderDirectory),
	}

	adapter, err := newAdapterClient(request.HostURL, request.Client)
	if err != nil {
		return result, err
	}
	loaded := false
	configTransactionID := ""
	initialValues := map[string]float64{}

	// The deferred finalizer makes the rollback/persist order explicit even
	// when an individual discovered placement or render fails.
	defer func() {
		if configTransactionID != "" {
			artifact.FinalRollback.TransactionID = configTransactionID
			raw, _, rollbackErr := adapter.operation(ctx, "/v1/plugin/rollback", map[string]any{"transaction_id": configTransactionID})
			if rollbackErr != nil {
				artifact.FinalRollback.Error = rollbackErr.Error()
				if err == nil {
					err = fmt.Errorf("final stereo placement rollback: %w", rollbackErr)
				}
			} else {
				artifact.FinalRollback.HostRollbackVerified = boolJSONField(raw, "rollback_verified")
				finalSnapshot, snapshotErr := adapter.snapshot(ctx)
				if snapshotErr != nil {
					artifact.FinalRollback.Error = snapshotErr.Error()
					if err == nil {
						err = fmt.Errorf("fresh readback after final rollback: %w", snapshotErr)
					}
				} else {
					artifact.FinalRollback.FreshReadbackMatchesInitial, artifact.FinalRollback.ParameterMismatches = stereoParameterMapsMatch(initialValues, finalSnapshot.Surface.Parameters)
					if !artifact.FinalRollback.FreshReadbackMatchesInitial && err == nil {
						err = fmt.Errorf("fresh readback after final rollback does not match initial preimage")
					}
				}
			}
		}
		if err != nil {
			artifact.Status = "observed_failed"
			artifact.Errors = appendUniqueStereoString(artifact.Errors, err.Error())
		} else if configTransactionID != "" && (!artifact.FinalRollback.HostRollbackVerified || !artifact.FinalRollback.FreshReadbackMatchesInitial) {
			artifact.Status = "observed_failed"
			err = fmt.Errorf("final stereo placement rollback was not verified")
			artifact.Errors = appendUniqueStereoString(artifact.Errors, err.Error())
		} else {
			artifact.Status = "observed_completed"
		}
		artifact.ListeningPack.Files = stereoListeningFiles(artifact)
		artifact.ListeningPack.Instructions = stereoListeningInstructions(artifact)
		if persistErr := persistStereoPlacementArtifacts(root, status.Manifest, request.HostURL, artifact); persistErr != nil && err == nil {
			err = persistErr
		}
		if loaded {
			_, _, _ = adapter.operation(context.Background(), "/v1/plugin/unload", map[string]any{})
		}
		result.Status = artifact.Status
	}()

	probeInput, probeErr := loadStereoProbeInput(request.ProbeAudioRoot)
	if probeErr != nil {
		err = probeErr
		return result, err
	}
	artifact.ProbeInput = probeInput
	if _, err = adapter.request(ctx, http.MethodPost, "/v1/plugin/load", map[string]any{
		"plugin_path": status.Manifest.PluginIdentity.InstallPath,
		"sample_rate": 48000,
		"block_size":  512,
	}); err != nil {
		err = fmt.Errorf("load isolated VST3 for stereo placement probe: %w", err)
		return result, err
	}
	loaded = true

	snapshot, snapshotErr := adapter.snapshot(ctx)
	if snapshotErr != nil {
		err = fmt.Errorf("capture fresh stereo placement host snapshot: %w", snapshotErr)
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
	initialValues = stereoParameterValueMap(snapshot.Surface.Parameters)

	placement, placementErr := requireStereoProbeParameter(snapshot.Surface.Parameters, request.PlacementParameterID)
	if placementErr != nil {
		err = placementErr
		return result, err
	}
	if placement.NormalizedValue == nil {
		err = fmt.Errorf("placement parameter %s has no normalized preimage", placement.ID)
		return result, err
	}
	numSteps := stereoObservedInt(placement.Observed, "num_steps")
	if numSteps < 2 || numSteps > 12 || !stereoObservedBool(placement.Observed, "is_discrete") {
		err = fmt.Errorf("placement parameter %s is not a bounded discrete surface", placement.ID)
		return result, err
	}
	artifact.ObservedPlacement = stereoPlacementParameter{
		ID: placement.ID, HostLabel: placement.Name, InitialNormalized: *placement.NormalizedValue,
		InitialDisplay: placement.DisplayText, NumSteps: numSteps, Automation: placement.Automation,
	}

	candidates, discoveryErr := discoverStereoPlacementCandidates(ctx, adapter, placement, numSteps)
	artifact.Discovery = candidates
	if discoveryErr != nil {
		err = discoveryErr
		return result, err
	}
	stereoCandidate, ok := stereoCandidateByDisplay(candidates, "Stereo")
	if !ok {
		err = fmt.Errorf("observed placement sweep did not return a Stereo display value")
		return result, err
	}

	changes, configurationErr := controlledStereoProbeChanges(request, snapshot.Surface.Parameters, stereoCandidate.RequestedNormalized)
	if configurationErr != nil {
		err = configurationErr
		return result, err
	}
	configurationRaw, _, configurationWriteErr := adapter.operation(ctx, "/v1/plugin/parameters/write", map[string]any{"changes": stereoWritePayload(changes)})
	if configurationWriteErr != nil {
		err = fmt.Errorf("write controlled stereo placement test configuration: %w", configurationWriteErr)
		return result, err
	}
	configTransactionID = stringJSONField(configurationRaw, "transaction_id")
	if configTransactionID == "" {
		err = fmt.Errorf("controlled stereo placement write omitted transaction id")
		return result, err
	}
	if !stereoFreshReadbackMatches(configurationRaw, changes) {
		err = fmt.Errorf("controlled stereo placement configuration lacks matching fresh readback")
		return result, err
	}
	artifact.Configuration = stereoProbeConfiguration{
		Basis:         "controlled values use user-confirmed Pro-Q 3 GUI actions together with observed stable host IDs; values are a bounded test state, not a reusable semantic mapping",
		TransactionID: configTransactionID,
		Changes:       stereoConfigurationReadback(configurationRaw, changes),
	}
	savedConfig, _, saveErr := adapter.operation(ctx, "/v1/plugin/state/save", map[string]any{})
	if saveErr != nil {
		err = fmt.Errorf("save controlled stereo placement state: %w", saveErr)
		return result, err
	}
	artifact.Configuration.StateID = stringJSONField(savedConfig, "state_id")
	if artifact.Configuration.StateID == "" {
		err = fmt.Errorf("controlled stereo placement state save omitted state id")
		return result, err
	}

	bypassPath := filepath.Join(renderDirectory, "00_bypass.wav")
	bypassRender, bypassErr := renderStereoPlacement(ctx, adapter, probeInput.Path, bypassPath, true, 0, "bypass")
	if bypassErr != nil {
		err = bypassErr
		return result, err
	}
	artifact.BypassRender = &bypassRender

	for index, candidate := range candidates {
		if _, _, restoreErr := adapter.operation(ctx, "/v1/plugin/state/restore", map[string]any{"state_id": artifact.Configuration.StateID}); restoreErr != nil {
			err = fmt.Errorf("restore controlled state before placement %q: %w", candidate.ObservedDisplayValue, restoreErr)
			return result, err
		}
		entry := stereoPlacementRender{PlacementNormalized: candidate.RequestedNormalized, ObservedDisplay: candidate.ObservedDisplayValue}
		writeRaw, _, writeErr := adapter.operation(ctx, "/v1/plugin/parameters/write", map[string]any{"changes": []map[string]any{{"id": placement.ID, "normalized": candidate.RequestedNormalized}}})
		if writeErr != nil {
			entry.Error = writeErr.Error()
			artifact.PlacementRenders = append(artifact.PlacementRenders, entry)
			err = fmt.Errorf("write placement %q: %w", candidate.ObservedDisplayValue, writeErr)
			return result, err
		}
		entry.TransactionID = stringJSONField(writeRaw, "transaction_id")
		entry.FreshReadbackMatches = freshReadbackMatchesJSON(writeRaw, placement.ID, candidate.RequestedNormalized)
		if entry.TransactionID == "" || !entry.FreshReadbackMatches {
			entry.Error = "placement write omitted a transaction id or matching fresh readback"
			artifact.PlacementRenders = append(artifact.PlacementRenders, entry)
			err = fmt.Errorf("placement write %q did not satisfy preimage/fresh-readback requirements", candidate.ObservedDisplayValue)
			return result, err
		}
		roundtripRaw, _, roundtripErr := adapter.operation(ctx, "/v1/plugin/state/roundtrip", map[string]any{})
		entry.StateRoundtripMatches = roundtripErr == nil && boolJSONField(roundtripRaw, "parameter_readback_matches_preimage") && freshReadbackMatchesJSON(roundtripRaw, placement.ID, candidate.RequestedNormalized)
		if roundtripErr != nil || !entry.StateRoundtripMatches {
			entry.Error = firstNonEmpty(entry.Error, "placement state roundtrip did not retain the fresh write")
			artifact.PlacementRenders = append(artifact.PlacementRenders, entry)
			err = fmt.Errorf("state roundtrip after placement %q did not retain the test state", candidate.ObservedDisplayValue)
			return result, err
		}
		output := filepath.Join(renderDirectory, fmt.Sprintf("%02d_%s.wav", index+1, stereoSafeFileLabel(candidate.ObservedDisplayValue)))
		rendered, renderErr := renderStereoPlacement(ctx, adapter, probeInput.Path, output, false, candidate.RequestedNormalized, candidate.ObservedDisplayValue)
		if renderErr != nil {
			entry.Error = renderErr.Error()
			artifact.PlacementRenders = append(artifact.PlacementRenders, entry)
			err = renderErr
			return result, err
		}
		entry.OutputPath = rendered.OutputPath
		entry.OutputSHA256 = rendered.OutputSHA256
		entry.Metrics = rendered.Metrics
		entry.DeltaFromBypass = stereoMetricDeltas(bypassRender.Metrics, rendered.Metrics)

		rollbackRaw, _, rollbackErr := adapter.operation(ctx, "/v1/plugin/rollback", map[string]any{"transaction_id": entry.TransactionID})
		entry.RollbackVerified = rollbackErr == nil && boolJSONField(rollbackRaw, "rollback_verified")
		entry.RollbackReadbackMatches = rollbackErr == nil && freshReadbackMatchesJSON(rollbackRaw, placement.ID, stereoCandidate.RequestedNormalized)
		if rollbackErr != nil || !entry.RollbackVerified || !entry.RollbackReadbackMatches {
			entry.Error = firstNonEmpty(entry.Error, "placement rollback failed or did not return to the controlled Stereo state")
			artifact.PlacementRenders = append(artifact.PlacementRenders, entry)
			err = fmt.Errorf("rollback after placement %q was not verified", candidate.ObservedDisplayValue)
			return result, err
		}
		artifact.PlacementRenders = append(artifact.PlacementRenders, entry)
	}
	return result, nil
}

func validateStereoPlacementRequest(request StereoPlacementProbeRequest) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"host URL", request.HostURL}, {"probe audio root", request.ProbeAudioRoot},
		{"Band 1 Used parameter id", request.BandUsedParameterID}, {"Band 1 Enabled parameter id", request.BandEnabledParameterID},
		{"Band 1 Frequency parameter id", request.BandFrequencyParameterID}, {"Band 1 Gain parameter id", request.BandGainParameterID},
		{"Band 1 Q parameter id", request.BandQParameterID}, {"Band 1 Shape parameter id", request.BandShapeParameterID},
		{"Band 1 Stereo Placement parameter id", request.PlacementParameterID},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("stereo placement probe %s is required", field.name)
		}
	}
	return nil
}

func loadStereoProbeInput(probeRoot string) (stereoProbeInput, error) {
	probeRoot = filepath.Clean(strings.TrimSpace(probeRoot))
	var manifest probeaudio.Manifest
	if err := readJSON(filepath.Join(probeRoot, "manifest.json"), &manifest); err != nil {
		return stereoProbeInput{}, err
	}
	if manifest.SuiteID != probeaudio.SuiteID || manifest.SampleRate != 48000 || manifest.Channels != 2 {
		return stereoProbeInput{}, fmt.Errorf("stereo placement probe requires %s at 48 kHz stereo", probeaudio.SuiteID)
	}
	for _, asset := range manifest.Assets {
		if asset.ID != stereoPlacementProbeAssetID {
			continue
		}
		path := filepath.Join(probeRoot, asset.File)
		digest, err := stereoSHA256File(path)
		if err != nil {
			return stereoProbeInput{}, err
		}
		if digest != asset.SHA256 {
			return stereoProbeInput{}, fmt.Errorf("stereo probe input fingerprint does not match manifest")
		}
		return stereoProbeInput{AssetID: asset.ID, Path: path, SHA256: digest, Policy: "same source bytes and render settings are used for bypass and every observed placement state"}, nil
	}
	return stereoProbeInput{}, fmt.Errorf("probe audio suite lacks %s", stereoPlacementProbeAssetID)
}

func requireStereoProbeParameter(parameters []SurfaceParameter, id string) (SurfaceParameter, error) {
	parameter, ok := findSurfaceParameter(parameters, strings.TrimSpace(id))
	if !ok {
		return SurfaceParameter{}, fmt.Errorf("observed parameter id %q is not in the current host surface", id)
	}
	if !parameter.HostControllable || !strings.EqualFold(parameter.Automation, "automatable") {
		return SurfaceParameter{}, fmt.Errorf("observed parameter %q is not host-automatable", id)
	}
	return parameter, nil
}

func discoverStereoPlacementCandidates(ctx context.Context, adapter *adapterClient, parameter SurfaceParameter, numSteps int) ([]stereoPlacementCandidate, error) {
	candidates := make([]stereoPlacementCandidate, 0, numSteps)
	if parameter.NormalizedValue == nil {
		return candidates, fmt.Errorf("placement parameter has no normalized preimage")
	}
	preimage := *parameter.NormalizedValue
	for index := 0; index < numSteps; index++ {
		value := float64(index) / float64(numSteps-1)
		candidate := stereoPlacementCandidate{RequestedNormalized: value}
		writeRaw, _, writeErr := adapter.operation(ctx, "/v1/plugin/parameters/write", map[string]any{"changes": []map[string]any{{"id": parameter.ID, "normalized": value}}})
		if writeErr != nil {
			candidate.Error = writeErr.Error()
			candidates = append(candidates, candidate)
			return candidates, fmt.Errorf("discover placement value %.6f: %w", value, writeErr)
		}
		candidate.TransactionID = stringJSONField(writeRaw, "transaction_id")
		candidate.FreshReadbackMatches = freshReadbackMatchesJSON(writeRaw, parameter.ID, value)
		readback, found := stereoFreshReadbackParameter(writeRaw, parameter.ID)
		candidate.ObservedDisplayValue = readback.DisplayValue
		if candidate.TransactionID == "" || !candidate.FreshReadbackMatches || !found {
			candidate.Error = "placement discovery write omitted a transaction id or fresh parameter readback"
			candidates = append(candidates, candidate)
			return candidates, fmt.Errorf("placement discovery value %.6f lacks required fresh readback", value)
		}
		roundtripRaw, _, roundtripErr := adapter.operation(ctx, "/v1/plugin/state/roundtrip", map[string]any{})
		candidate.StateRoundtripMatches = roundtripErr == nil && boolJSONField(roundtripRaw, "parameter_readback_matches_preimage") && freshReadbackMatchesJSON(roundtripRaw, parameter.ID, value)
		rollbackRaw, _, rollbackErr := adapter.operation(ctx, "/v1/plugin/rollback", map[string]any{"transaction_id": candidate.TransactionID})
		candidate.RollbackVerified = rollbackErr == nil && boolJSONField(rollbackRaw, "rollback_verified")
		candidate.RollbackReadbackMatches = rollbackErr == nil && freshReadbackMatchesJSON(rollbackRaw, parameter.ID, preimage)
		if roundtripErr != nil || !candidate.StateRoundtripMatches || rollbackErr != nil || !candidate.RollbackVerified || !candidate.RollbackReadbackMatches {
			candidate.Error = "placement discovery did not satisfy state-roundtrip or rollback verification"
			candidates = append(candidates, candidate)
			return candidates, fmt.Errorf("placement discovery value %.6f did not satisfy rollback requirements", value)
		}
		candidates = append(candidates, candidate)
	}
	return candidates, nil
}

func controlledStereoProbeChanges(request StereoPlacementProbeRequest, parameters []SurfaceParameter, stereoNormalized float64) ([]stereoProbeConfigChange, error) {
	requested := []struct {
		role  string
		id    string
		value float64
	}{
		{"band_used", request.BandUsedParameterID, 1},
		{"band_enabled", request.BandEnabledParameterID, 1},
		// The normalized test state is deliberately checked through fresh host
		// display readback. It is an experiment setting, not a normative EQ map.
		{"band_frequency", request.BandFrequencyParameterID, 0.472},
		{"band_gain", request.BandGainParameterID, 0.700},
		{"band_q", request.BandQParameterID, 0.500},
		{"band_shape", request.BandShapeParameterID, 0},
		{"stereo_placement", request.PlacementParameterID, stereoNormalized},
	}
	seen := map[string]bool{}
	changes := make([]stereoProbeConfigChange, 0, len(requested))
	for _, item := range requested {
		parameter, err := requireStereoProbeParameter(parameters, item.id)
		if err != nil {
			return nil, err
		}
		if seen[parameter.ID] {
			return nil, fmt.Errorf("stereo probe configuration contains duplicate parameter id %q", parameter.ID)
		}
		seen[parameter.ID] = true
		changes = append(changes, stereoProbeConfigChange{Role: item.role, ID: parameter.ID, HostLabel: parameter.Name, RequestedNormalized: item.value})
	}
	return changes, nil
}

func stereoWritePayload(changes []stereoProbeConfigChange) []map[string]any {
	payload := make([]map[string]any, 0, len(changes))
	for _, change := range changes {
		payload = append(payload, map[string]any{"id": change.ID, "normalized": change.RequestedNormalized})
	}
	return payload
}

func stereoFreshReadbackMatches(raw json.RawMessage, changes []stereoProbeConfigChange) bool {
	for _, change := range changes {
		if !freshReadbackMatchesJSON(raw, change.ID, change.RequestedNormalized) {
			return false
		}
	}
	return true
}

func stereoConfigurationReadback(raw json.RawMessage, changes []stereoProbeConfigChange) []stereoProbeConfigChange {
	for index := range changes {
		if value, ok := stereoFreshReadbackParameter(raw, changes[index].ID); ok {
			changes[index].ObservedNormalized = value.NormalizedValue
			changes[index].ObservedDisplay = value.DisplayValue
		}
	}
	return changes
}

func stereoCandidateByDisplay(candidates []stereoPlacementCandidate, display string) (stereoPlacementCandidate, bool) {
	for _, candidate := range candidates {
		if strings.EqualFold(strings.TrimSpace(candidate.ObservedDisplayValue), strings.TrimSpace(display)) {
			return candidate, true
		}
	}
	return stereoPlacementCandidate{}, false
}

func stereoFreshReadbackParameter(raw json.RawMessage, id string) (stereoRawParameter, bool) {
	var payload struct {
		FreshReadback struct {
			Parameters []stereoRawParameter `json:"parameters"`
		} `json:"fresh_readback"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return stereoRawParameter{}, false
	}
	for _, parameter := range payload.FreshReadback.Parameters {
		if parameter.ID == id {
			return parameter, true
		}
	}
	return stereoRawParameter{}, false
}

func renderStereoPlacement(ctx context.Context, adapter *adapterClient, inputPath, outputPath string, bypass bool, normalized float64, display string) (stereoPlacementRender, error) {
	entry := stereoPlacementRender{PlacementNormalized: normalized, ObservedDisplay: display, OutputPath: outputPath}
	_, _, err := adapter.operation(ctx, "/v1/plugin/render", map[string]any{"input_path": inputPath, "output_path": outputPath, "bypass": bypass})
	if err != nil {
		return entry, fmt.Errorf("render %s placement: %w", display, err)
	}
	digest, err := stereoSHA256File(outputPath)
	if err != nil {
		return entry, err
	}
	entry.OutputSHA256 = digest
	metrics, err := stereoPlacementMetrics(outputPath)
	if err != nil {
		return entry, err
	}
	entry.Metrics = metrics
	return entry, nil
}

func stereoPlacementMetrics(path string) ([]stereoPlacementSegmentMetric, error) {
	sampleRate, left, right, err := readStereoPCM(path)
	if err != nil {
		return nil, err
	}
	segments := []struct {
		name       string
		start, end float64
	}{
		{"in_phase", 0, 2},
		{"opposite_phase", 2, 4},
		{"left_only", 4, 6},
		{"right_only", 6, 8},
		{"different_content", 8, 10},
	}
	metrics := make([]stereoPlacementSegmentMetric, 0, len(segments))
	for _, segment := range segments {
		start := maxStereoInt(0, int(math.Round(segment.start*float64(sampleRate))))
		end := minStereoInt(len(left), int(math.Round(segment.end*float64(sampleRate))))
		if end <= start {
			return nil, fmt.Errorf("render %s is shorter than required %s window", filepath.Base(path), segment.name)
		}
		leftPower, rightPower, midPower, sidePower := 0.0, 0.0, 0.0, 0.0
		for index := start; index < end; index++ {
			l, r := left[index], right[index]
			m, s := (l+r)/2, (l-r)/2
			leftPower += l * l
			rightPower += r * r
			midPower += m * m
			sidePower += s * s
		}
		frames := end - start
		metrics = append(metrics, stereoPlacementSegmentMetric{
			Segment: segment.name, StartSeconds: segment.start, EndSeconds: segment.end, Frames: frames,
			LeftRMSDBFS: stereoPowerDBFS(leftPower / float64(frames)), RightRMSDBFS: stereoPowerDBFS(rightPower / float64(frames)),
			MidRMSDBFS: stereoPowerDBFS(midPower / float64(frames)), SideRMSDBFS: stereoPowerDBFS(sidePower / float64(frames)),
		})
	}
	return metrics, nil
}

func stereoMetricDeltas(bypass, processed []stereoPlacementSegmentMetric) []stereoPlacementSegmentDelta {
	byName := map[string]stereoPlacementSegmentMetric{}
	for _, metric := range bypass {
		byName[metric.Segment] = metric
	}
	deltas := make([]stereoPlacementSegmentDelta, 0, len(processed))
	for _, metric := range processed {
		base, ok := byName[metric.Segment]
		if !ok {
			continue
		}
		left, leftSilent := stereoMetricDelta(metric.LeftRMSDBFS, base.LeftRMSDBFS)
		right, rightSilent := stereoMetricDelta(metric.RightRMSDBFS, base.RightRMSDBFS)
		mid, midSilent := stereoMetricDelta(metric.MidRMSDBFS, base.MidRMSDBFS)
		side, sideSilent := stereoMetricDelta(metric.SideRMSDBFS, base.SideRMSDBFS)
		silent := []string{}
		if leftSilent {
			silent = append(silent, "left")
		}
		if rightSilent {
			silent = append(silent, "right")
		}
		if midSilent {
			silent = append(silent, "mid")
		}
		if sideSilent {
			silent = append(silent, "side")
		}
		deltas = append(deltas, stereoPlacementSegmentDelta{Segment: metric.Segment, LeftDeltaDB: left, RightDeltaDB: right, MidDeltaDB: mid, SideDeltaDB: side, SilentReferenceFields: silent})
	}
	return deltas
}

func stereoMetricDelta(processed, bypass float64) (*float64, bool) {
	if bypass <= stereoDeltaSilenceFloorDBFS {
		return nil, true
	}
	value := processed - bypass
	return &value, false
}

func readStereoPCM(path string) (int, []float64, []float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, nil, nil, err
	}
	if len(data) < 44 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return 0, nil, nil, fmt.Errorf("%s is not a RIFF/WAVE file", filepath.Base(path))
	}
	var channels, sampleRate, bits int
	var samples []byte
	for offset := 12; offset+8 <= len(data); {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		chunkStart := offset + 8
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > len(data) {
			return 0, nil, nil, fmt.Errorf("%s contains a truncated %s chunk", filepath.Base(path), chunkID)
		}
		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return 0, nil, nil, fmt.Errorf("%s has an invalid fmt chunk", filepath.Base(path))
			}
			format := binary.LittleEndian.Uint16(data[chunkStart : chunkStart+2])
			if format != 1 {
				return 0, nil, nil, fmt.Errorf("%s is not PCM audio", filepath.Base(path))
			}
			channels = int(binary.LittleEndian.Uint16(data[chunkStart+2 : chunkStart+4]))
			sampleRate = int(binary.LittleEndian.Uint32(data[chunkStart+4 : chunkStart+8]))
			bits = int(binary.LittleEndian.Uint16(data[chunkStart+14 : chunkStart+16]))
		case "data":
			samples = data[chunkStart:chunkEnd]
		}
		offset = chunkEnd
		if offset%2 != 0 {
			offset++
		}
	}
	if channels != 2 || sampleRate <= 0 || len(samples) == 0 || (bits != 16 && bits != 24 && bits != 32) {
		return 0, nil, nil, fmt.Errorf("%s must be a non-empty 16/24/32-bit stereo PCM WAVE", filepath.Base(path))
	}
	bytesPerSample := bits / 8
	frameBytes := channels * bytesPerSample
	if len(samples)%frameBytes != 0 {
		return 0, nil, nil, fmt.Errorf("%s has a partial PCM frame", filepath.Base(path))
	}
	frames := len(samples) / frameBytes
	left, right := make([]float64, frames), make([]float64, frames)
	for frame := 0; frame < frames; frame++ {
		base := frame * frameBytes
		left[frame] = stereoDecodePCM(samples[base:base+bytesPerSample], bits)
		right[frame] = stereoDecodePCM(samples[base+bytesPerSample:base+2*bytesPerSample], bits)
	}
	return sampleRate, left, right, nil
}

func stereoDecodePCM(value []byte, bits int) float64 {
	switch bits {
	case 16:
		return float64(int16(binary.LittleEndian.Uint16(value))) / 32768.0
	case 24:
		integer := int32(value[0]) | int32(value[1])<<8 | int32(value[2])<<16
		if integer&0x800000 != 0 {
			integer |= ^int32(0xffffff)
		}
		return float64(integer) / 8388608.0
	case 32:
		return float64(int32(binary.LittleEndian.Uint32(value))) / 2147483648.0
	default:
		return 0
	}
}

func stereoPowerDBFS(power float64) float64 {
	return 10 * math.Log10(math.Max(power, 1e-18))
}

func stereoParameterValueMap(parameters []SurfaceParameter) map[string]float64 {
	values := make(map[string]float64, len(parameters))
	for _, parameter := range parameters {
		if parameter.NormalizedValue != nil {
			values[parameter.ID] = *parameter.NormalizedValue
		}
	}
	return values
}

func stereoParameterMapsMatch(expected map[string]float64, actual []SurfaceParameter) (bool, []string) {
	actualValues := stereoParameterValueMap(actual)
	mismatches := []string{}
	for id, expectedValue := range expected {
		actualValue, ok := actualValues[id]
		if !ok || math.Abs(actualValue-expectedValue) > 0.00001 {
			mismatches = append(mismatches, fmt.Sprintf("%s expected %.8f got %.8f", id, expectedValue, actualValue))
			if len(mismatches) >= 32 {
				break
			}
		}
	}
	return len(mismatches) == 0 && len(expected) == len(actualValues), mismatches
}

func persistStereoPlacementArtifacts(root string, manifest Manifest, hostURL string, artifact stereoPlacementProbeArtifact) error {
	if err := writeJSON(filepath.Join(root, stereoPlacementResultsFile), artifact); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, stereoPlacementGuideFile), []byte(stereoListeningGuideMarkdown(artifact)), 0o600); err != nil {
		return err
	}
	data, _ := json.Marshal(artifact)
	if err := appendEvidence(root, EvidenceEntry{
		Kind: "stereo_placement_audio_probe", Trust: "observed", Source: strings.TrimSpace(hostURL),
		Summary:  "Captured isolated stereo-placement renders, fresh readback, state roundtrip and rollback evidence; metrics do not grant routing semantics or a Credential.",
		Artifact: stereoPlacementResultsFile, CapturedAt: artifact.CapturedAt, Data: data,
	}); err != nil {
		return err
	}
	if err := appendEvidence(root, EvidenceEntry{
		Kind: "stereo_placement_listening_pack", Trust: "observed", Source: "vpsforge.stereo_placement_probe",
		Summary:  "Prepared deterministic offline WAV comparisons; concurrent live human auditory confirmation remains pending.",
		Artifact: stereoPlacementGuideFile, CapturedAt: artifact.CapturedAt,
	}); err != nil {
		return err
	}
	return updateManifest(root, func(current *Manifest) {
		if current.Artifacts == nil {
			current.Artifacts = map[string]string{}
		}
		current.Artifacts["stereo_placement_probe_results"] = stereoPlacementResultsFile
		current.Artifacts["stereo_placement_listening_guide"] = stereoPlacementGuideFile
		current.UpdatedAt = artifact.CapturedAt
	})
}

func stereoListeningGuideMarkdown(artifact stereoPlacementProbeArtifact) string {
	var builder strings.Builder
	builder.WriteString("# Pro-Q 3 Stereo Placement Offline Comparison Pack\n\n")
	builder.WriteString("This is a staging-only deterministic render artifact. Its labels and automated metrics are observed evidence; it does not issue a Credential or establish a routing claim by itself. It is not a concurrent GUI human witness and must not substitute for listening while operating the live witness instance.\n\n")
	builder.WriteString("## Files\n\n")
	for _, path := range artifact.ListeningPack.Files {
		builder.WriteString("- `" + path + "`\n")
	}
	builder.WriteString("\n## Optional audit review protocol\n\n")
	for _, instruction := range stereoListeningInstructions(artifact) {
		builder.WriteString("- " + instruction + "\n")
	}
	builder.WriteString("\n## Authority boundary\n\n")
	builder.WriteString("Do not use post-render playback to report a simultaneous GUI-operation witness. If you review these files, describe them only as an optional audit of the named observed render state. Record live subjective listening separately, and do not infer a Credential, task badge, or executable route from either source.\n")
	return builder.String()
}

func stereoListeningFiles(artifact stereoPlacementProbeArtifact) []string {
	files := []string{}
	if artifact.BypassRender != nil && artifact.BypassRender.OutputPath != "" {
		files = append(files, artifact.BypassRender.OutputPath)
	}
	for _, render := range artifact.PlacementRenders {
		if render.OutputPath != "" {
			files = append(files, render.OutputPath)
		}
	}
	return files
}

func stereoListeningInstructions(artifact stereoPlacementProbeArtifact) []string {
	return []string{
		"These files are offline audit artifacts, not a replacement for concurrent listening while operating the live VST3 witness GUI.",
		"Use the same normal playback level for 00_bypass.wav and each observed placement render. Do not change pan, balance, mono, spatial processing or any player DSP.",
		"The 0–2 s segment is L/R in phase; the 2–4 s segment is L/R opposite phase; 4–6 s is left-only; 6–8 s is right-only; 8–10 s contains different left/right content.",
		"For a display state named Left or Right, compare the 4–6 s and 6–8 s windows against bypass and describe which window audibly contains the controlled 440 Hz-band change.",
		"For a display state named Mid or Side, compare the in-phase 0–2 s and opposite-phase 2–4 s windows against bypass and describe which one audibly contains the controlled band change.",
		"Report uncertainty as unknown. The visual curve remaining unchanged is not an acoustic routing result.",
	}
}

func relativeStereoProbePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(relative)
}

func stereoSHA256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func stereoObservedInt(values map[string]any, key string) int {
	value, ok := values[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		return int(math.Round(typed))
	case int:
		return typed
	case int64:
		return int(typed)
	default:
		return 0
	}
}

func stereoObservedBool(values map[string]any, key string) bool {
	value, _ := values[key].(bool)
	return value
}

func sameInstallPath(expected, observed string) bool {
	return strings.EqualFold(filepath.Clean(strings.TrimSpace(expected)), filepath.Clean(strings.TrimSpace(observed)))
}

func stereoSafeFileLabel(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
			builder.WriteRune(character)
		} else {
			builder.WriteByte('_')
		}
	}
	return strings.Trim(builder.String(), "_")
}

func appendUniqueStereoString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func minStereoInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxStereoInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

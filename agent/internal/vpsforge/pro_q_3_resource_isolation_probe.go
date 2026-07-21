package vpsforge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/vps"
)

const (
	proQ3ResourceIsolationProbeSchema  = "vit.vpsforge.pro_q_3_resource_isolation_probe.v1"
	proQ3ResourceIsolationProbeProfile = "fabfilter.pro-q-3.resource-isolation-reload.v1"
	proQ3ResourceIsolationEvidenceDir  = "evidence/pro_q_3_resource_isolation_probe"
)

// ProQ3ResourceIsolationProbeRequest names a non-destructive staging-only
// experiment. It starts two direct native workers rather than reusing a
// workbench host process, so a failure in either VST3 instance is contained to
// that child worker.
type ProQ3ResourceIsolationProbeRequest struct {
	Root       string
	WorkerPath string
	SampleRate float64
	BlockSize  int
	Now        time.Time
}

type ProQ3ResourceIsolationProbeResult struct {
	Workspace       string `json:"workspace"`
	Status          string `json:"status"`
	ResultsArtifact string `json:"results_artifact"`
	RunArtifact     string `json:"run_artifact"`
}

type proQ3ResourceIsolationProbeArtifact struct {
	SchemaVersion string    `json:"schema_version"`
	Trust         string    `json:"trust"`
	Status        string    `json:"status"`
	CapturedAt    time.Time `json:"captured_at"`
	WorkspaceID   string    `json:"workspace_id"`
	Profile       string    `json:"profile"`
	RunArtifact   string    `json:"run_artifact"`

	AuthorityBoundary []string                      `json:"authority_boundary"`
	WorkerConfig      proQ3IsolationWorkerConfig    `json:"worker_configuration"`
	InstanceA         proQ3IsolationInstance        `json:"instance_a,omitempty"`
	InstanceB         proQ3IsolationInstance        `json:"instance_b,omitempty"`
	ControlledWrite   proQ3IsolationControlledWrite `json:"controlled_write,omitempty"`
	ResourceIsolation proQ3IsolationResourceCheck   `json:"resource_isolation,omitempty"`
	StateRoundtrip    proQ3IsolationStateRoundtrip  `json:"state_roundtrip,omitempty"`
	ReloadRestore     proQ3IsolationReloadRestore   `json:"reload_restore,omitempty"`
	FinalRollback     proQ3IsolationFinalRollback   `json:"final_rollback,omitempty"`
	Limitations       []string                      `json:"limitations,omitempty"`
	Errors            []string                      `json:"errors,omitempty"`
}

type proQ3IsolationWorkerConfig struct {
	WorkerPath string  `json:"worker_path"`
	SampleRate float64 `json:"sample_rate"`
	BlockSize  int     `json:"block_size"`
	Mode       string  `json:"mode"`
}

type proQ3IsolationInstance struct {
	Identity            vps.PluginIdentity         `json:"identity,omitempty"`
	InitialWorkerPID    int                        `json:"initial_worker_pid,omitempty"`
	ReloadedWorkerPID   int                        `json:"reloaded_worker_pid,omitempty"`
	InitialSurface      proQ3IsolationSurface      `json:"initial_surface,omitempty"`
	InitialState        proQ3IsolationStateReceipt `json:"initial_state,omitempty"`
	PostWriteState      proQ3IsolationStateReceipt `json:"post_write_state,omitempty"`
	InitialCrossCompare *proQ3IsolationComparison  `json:"initial_cross_instance_compare,omitempty"`
}

type proQ3IsolationSurface struct {
	ParameterCount     int       `json:"parameter_count"`
	StableParameterIDs int       `json:"stable_parameter_ids"`
	ParameterValueHash string    `json:"parameter_value_sha256"`
	CapturedAt         time.Time `json:"captured_at"`
}

// proQ3IsolationStateReceipt intentionally excludes state_base64. The
// serialized state is retained only in memory long enough to exercise a
// cross-worker restore, while this evidence artifact holds its bounded hash
// and byte count only.
type proQ3IsolationStateReceipt struct {
	StateSHA256     string `json:"state_sha256,omitempty"`
	StateBytes      int64  `json:"state_bytes,omitempty"`
	Serialized      bool   `json:"serialized"`
	RawStateOmitted bool   `json:"raw_state_omitted"`
}

type proQ3IsolationControlledWrite struct {
	ParameterID                       string  `json:"parameter_id,omitempty"`
	ObservedHostLabel                 string  `json:"observed_host_label,omitempty"`
	SelectionBasis                    string  `json:"selection_basis,omitempty"`
	PreimageNormalized                float64 `json:"preimage_normalized,omitempty"`
	RequestedNormalized               float64 `json:"requested_normalized,omitempty"`
	TransactionID                     string  `json:"transaction_id,omitempty"`
	WorkerFreshReadbackMatchesRequest bool    `json:"worker_fresh_readback_matches_request"`
	FreshSnapshotMatchesRequest       bool    `json:"fresh_snapshot_matches_request"`
}

type proQ3IsolationResourceCheck struct {
	SeparateWorkerProcessesObserved bool                     `json:"separate_worker_processes_observed"`
	AInitialWorkerPID               int                      `json:"a_initial_worker_pid,omitempty"`
	BInitialWorkerPID               int                      `json:"b_initial_worker_pid,omitempty"`
	BPreimageAfterAWrite            proQ3IsolationComparison `json:"b_preimage_after_a_write"`
	BPreimageAtFinalReadback        proQ3IsolationComparison `json:"b_preimage_at_final_readback"`
}

type proQ3IsolationStateRoundtrip struct {
	Completed                              bool                     `json:"completed"`
	ParameterReadbackMatchesWorkerPreimage bool                     `json:"parameter_readback_matches_worker_preimage"`
	FreshSnapshotMatchesPostWrite          proQ3IsolationComparison `json:"fresh_snapshot_matches_post_write"`
}

type proQ3IsolationReloadRestore struct {
	UnloadReported                bool                     `json:"unload_reported"`
	Reloaded                      bool                     `json:"reloaded"`
	ReloadedWorkerPID             int                      `json:"reloaded_worker_pid,omitempty"`
	ReloadedPIDDiffersFromInitial bool                     `json:"reloaded_pid_differs_from_initial"`
	PostWriteStateRestored        bool                     `json:"post_write_state_restored"`
	FreshSnapshotMatchesPostWrite proQ3IsolationComparison `json:"fresh_snapshot_matches_post_write"`
}

type proQ3IsolationFinalRollback struct {
	TransactionRollbackReportedVerified bool                     `json:"transaction_rollback_reported_verified"`
	TransactionRollbackFreshReadback    proQ3IsolationComparison `json:"transaction_rollback_fresh_readback"`
	ReloadedInitialStateRestored        bool                     `json:"reloaded_initial_state_restored"`
	FinalFreshReadbackMatchesInitial    proQ3IsolationComparison `json:"final_fresh_readback_matches_initial"`
}

// proQ3IsolationComparison proves a complete parameter-surface comparison
// without copying a potentially very large raw parameter surface into the
// evidence ledger. Individual mismatch descriptions are bounded.
type proQ3IsolationComparison struct {
	Matches                bool     `json:"matches"`
	ExpectedParameterCount int      `json:"expected_parameter_count"`
	ActualParameterCount   int      `json:"actual_parameter_count"`
	ExpectedValueSHA256    string   `json:"expected_value_sha256,omitempty"`
	ActualValueSHA256      string   `json:"actual_value_sha256,omitempty"`
	Mismatches             []string `json:"mismatches,omitempty"`
}

// RunProQ3ResourceIsolationProbe verifies the adapter's non-destructive
// resource boundary using two concurrently live Pro-Q 3 workers. It does not
// intentionally crash a commercial plug-in, change a canonical VPS artifact,
// grant a task badge, issue a Credential, or update Catalog/SPAL.
func RunProQ3ResourceIsolationProbe(ctx context.Context, request ProQ3ResourceIsolationProbeRequest) (result ProQ3ResourceIsolationProbeResult, err error) {
	root, err := preflightStagingWorkspaceRoot(request.Root)
	if err != nil {
		return result, err
	}
	status, err := Inspect(root)
	if err != nil {
		return result, fmt.Errorf("inspect Pro-Q 3 resource-isolation workspace: %w", err)
	}
	if !isProQ3Identity(status.Manifest.PluginIdentity) {
		return result, fmt.Errorf("resource-isolation probe only accepts the observed FabFilter Pro-Q 3 reference workspace")
	}
	pluginPath := strings.TrimSpace(status.Manifest.PluginIdentity.InstallPath)
	if pluginPath == "" {
		return result, fmt.Errorf("Pro-Q 3 workspace has no observed VST3 install path")
	}
	if !strings.EqualFold(filepath.Ext(pluginPath), ".vst3") {
		return result, fmt.Errorf("Pro-Q 3 workspace install path is not a VST3: %s", pluginPath)
	}
	if _, statErr := os.Stat(pluginPath); statErr != nil {
		return result, fmt.Errorf("inspect observed Pro-Q 3 VST3 path: %w", statErr)
	}

	workerPath := filepath.Clean(strings.TrimSpace(request.WorkerPath))
	if workerPath == "" || workerPath == "." {
		return result, fmt.Errorf("native VST3 worker path is required")
	}
	if info, statErr := os.Stat(workerPath); statErr != nil || info.IsDir() {
		if statErr != nil {
			return result, fmt.Errorf("inspect native VST3 worker: %w", statErr)
		}
		return result, fmt.Errorf("native VST3 worker path is a directory: %s", workerPath)
	}
	sampleRate := request.SampleRate
	if sampleRate == 0 {
		sampleRate = 48000
	}
	if !finiteHostValue(sampleRate) || sampleRate < 8000 || sampleRate > 384000 {
		return result, fmt.Errorf("resource-isolation sample rate must be within 8000..384000")
	}
	blockSize := request.BlockSize
	if blockSize == 0 {
		blockSize = 512
	}
	if blockSize < 16 || blockSize > 8192 {
		return result, fmt.Errorf("resource-isolation block size must be within 16..8192")
	}

	now := nowOrCurrent(request.Now)
	runID := stableID("pro_q_3_resource_isolation", status.Manifest.WorkspaceID, now.Format(time.RFC3339Nano))
	runArtifact := filepath.ToSlash(filepath.Join(proQ3ResourceIsolationEvidenceDir, runID+".json"))
	result = ProQ3ResourceIsolationProbeResult{
		Workspace: root, ResultsArtifact: proQ3ResourceIsolationResultsFile, RunArtifact: runArtifact,
	}
	artifact := proQ3ResourceIsolationProbeArtifact{
		SchemaVersion: proQ3ResourceIsolationProbeSchema,
		Trust:         "observed",
		Status:        "unsupported/unknown",
		CapturedAt:    now,
		WorkspaceID:   status.Manifest.WorkspaceID,
		Profile:       proQ3ResourceIsolationProbeProfile,
		RunArtifact:   runArtifact,
		AuthorityBoundary: []string{
			"staging-only direct-worker experiment; no VPS Library, Credential, Catalog or SPAL route is changed",
			"all snapshots, parameter values, state hashes and process IDs are observed host evidence, not executable EQ semantics",
			"serialized plugin bytes are held only in process memory for restore and are never written to the evidence artifact or ledger",
			"this non-destructive run does not deliberately crash a commercial plug-in and cannot by itself certify every crash-containment boundary",
		},
		WorkerConfig: proQ3IsolationWorkerConfig{
			WorkerPath: workerPath, SampleRate: sampleRate, BlockSize: blockSize, Mode: "two direct crash-isolated native workers",
		},
		Limitations: []string{
			"PID separation demonstrates separate child worker processes during this run; it does not prove an operating-system security sandbox.",
			"No intentional plug-in fault was triggered. Crash handling remains bounded by the worker supervisor design and future controlled regression coverage.",
			"Observed parameter IDs and host labels are not promoted to a reusable semantic mapping, task badge or Credential.",
		},
	}

	// Native calls are given a finite shared deadline even when the CLI caller
	// used context.Background. A timed-out worker is terminated by its
	// supervisor and cannot be reused with uncertain plug-in state.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
	}

	adapterA := NewVST3HostAdapter(workerPath)
	adapterB := NewVST3HostAdapter(workerPath)
	loadedA, loadedB := false, false
	initialAValues, initialBValues := map[string]float64{}, map[string]float64{}
	postWriteAValues := map[string]float64{}
	initialAStateBase64, postWriteAStateBase64 := "", ""
	finalInitialStateRestored := false

	defer func() {
		cleanupCtx, cancelCleanup := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancelCleanup()
		// If any step after the complete initial state save fails, make a
		// best-effort final restoration before closing the isolated child. The
		// plug-in cannot survive the worker close, but this verifies the same
		// state discipline used by a live host.
		if loadedA && initialAStateBase64 != "" && !finalInitialStateRestored {
			if _, restoreErr := proQ3IsolationOperation(cleanupCtx, adapterA, "restore_state", map[string]any{"state_base64": initialAStateBase64}); restoreErr != nil {
				artifact.Errors = appendProQ3IsolationError(artifact.Errors, "final initial-state restore: "+restoreErr.Error())
				if err == nil {
					err = fmt.Errorf("final initial-state restore: %w", restoreErr)
				}
			} else if fresh, snapshotErr := proQ3IsolationSnapshot(cleanupCtx, adapterA); snapshotErr != nil {
				artifact.Errors = appendProQ3IsolationError(artifact.Errors, "final fresh snapshot: "+snapshotErr.Error())
				if err == nil {
					err = fmt.Errorf("final fresh snapshot: %w", snapshotErr)
				}
			} else {
				artifact.FinalRollback.ReloadedInitialStateRestored = true
				artifact.FinalRollback.FinalFreshReadbackMatchesInitial = proQ3IsolationCompare(initialAValues, proQ3IsolationParameterMap(fresh.Surface.Parameters))
				if !artifact.FinalRollback.FinalFreshReadbackMatchesInitial.Matches && err == nil {
					err = fmt.Errorf("final fresh readback after initial-state restore does not match the complete A preimage")
				}
			}
		}
		if loadedB && len(initialBValues) > 0 {
			if fresh, snapshotErr := proQ3IsolationSnapshot(cleanupCtx, adapterB); snapshotErr != nil {
				artifact.Errors = appendProQ3IsolationError(artifact.Errors, "B final fresh snapshot: "+snapshotErr.Error())
				if err == nil {
					err = fmt.Errorf("B final fresh snapshot: %w", snapshotErr)
				}
			} else {
				artifact.ResourceIsolation.BPreimageAtFinalReadback = proQ3IsolationCompare(initialBValues, proQ3IsolationParameterMap(fresh.Surface.Parameters))
				if !artifact.ResourceIsolation.BPreimageAtFinalReadback.Matches && err == nil {
					err = fmt.Errorf("B final fresh readback does not match its complete preimage")
				}
			}
		}
		_ = adapterA.Close()
		_ = adapterB.Close()
		if err != nil {
			artifact.Status = "observed_failed"
			artifact.Errors = appendProQ3IsolationError(artifact.Errors, err.Error())
		} else {
			artifact.Status = "observed_completed"
		}
		if persistErr := persistProQ3ResourceIsolationArtifacts(root, artifact); persistErr != nil && err == nil {
			err = persistErr
			result.Status = "observed_failed"
			return
		}
		result.Status = artifact.Status
	}()

	loadRequest := map[string]any{"plugin_path": pluginPath, "sample_rate": sampleRate, "block_size": blockSize}
	if _, loadErr := adapterA.load(ctx, loadRequest); loadErr != nil {
		err = fmt.Errorf("load isolated Pro-Q 3 instance A: %w", loadErr)
		return result, err
	}
	loadedA = true
	artifact.InstanceA.InitialWorkerPID = adapterA.workerPID()
	if artifact.InstanceA.InitialWorkerPID == 0 {
		err = fmt.Errorf("instance A did not expose a live isolated worker PID")
		return result, err
	}
	if _, loadErr := adapterB.load(ctx, loadRequest); loadErr != nil {
		err = fmt.Errorf("load isolated Pro-Q 3 instance B: %w", loadErr)
		return result, err
	}
	loadedB = true
	artifact.InstanceB.InitialWorkerPID = adapterB.workerPID()
	if artifact.InstanceB.InitialWorkerPID == 0 {
		err = fmt.Errorf("instance B did not expose a live isolated worker PID")
		return result, err
	}
	artifact.ResourceIsolation.AInitialWorkerPID = artifact.InstanceA.InitialWorkerPID
	artifact.ResourceIsolation.BInitialWorkerPID = artifact.InstanceB.InitialWorkerPID
	artifact.ResourceIsolation.SeparateWorkerProcessesObserved = artifact.InstanceA.InitialWorkerPID != artifact.InstanceB.InitialWorkerPID
	if !artifact.ResourceIsolation.SeparateWorkerProcessesObserved {
		err = fmt.Errorf("instance A and B unexpectedly report the same native worker PID")
		return result, err
	}

	snapshotA, snapshotErr := proQ3IsolationSnapshot(ctx, adapterA)
	if snapshotErr != nil {
		err = fmt.Errorf("capture instance A initial snapshot: %w", snapshotErr)
		return result, err
	}
	snapshotB, snapshotErr := proQ3IsolationSnapshot(ctx, adapterB)
	if snapshotErr != nil {
		err = fmt.Errorf("capture instance B initial snapshot: %w", snapshotErr)
		return result, err
	}
	if identityErr := proQ3IsolationValidateSnapshot(status.Manifest.PluginIdentity, pluginPath, snapshotA); identityErr != nil {
		err = fmt.Errorf("validate instance A identity: %w", identityErr)
		return result, err
	}
	if identityErr := proQ3IsolationValidateSnapshot(status.Manifest.PluginIdentity, pluginPath, snapshotB); identityErr != nil {
		err = fmt.Errorf("validate instance B identity: %w", identityErr)
		return result, err
	}
	artifact.InstanceA.Identity = snapshotA.Identity
	artifact.InstanceB.Identity = snapshotB.Identity
	artifact.InstanceA.InitialSurface = proQ3IsolationSurfaceSummary(snapshotA)
	artifact.InstanceB.InitialSurface = proQ3IsolationSurfaceSummary(snapshotB)
	initialAValues = proQ3IsolationParameterMap(snapshotA.Surface.Parameters)
	initialBValues = proQ3IsolationParameterMap(snapshotB.Surface.Parameters)
	initialCrossCompare := proQ3IsolationCompare(initialAValues, initialBValues)
	artifact.InstanceA.InitialCrossCompare = &initialCrossCompare

	var saveErr error
	artifact.InstanceA.InitialState, initialAStateBase64, saveErr = proQ3IsolationSaveState(ctx, adapterA)
	if saveErr != nil {
		err = fmt.Errorf("save complete A preimage: %w", saveErr)
		return result, err
	}
	artifact.InstanceB.InitialState, _, saveErr = proQ3IsolationSaveState(ctx, adapterB)
	if saveErr != nil {
		err = fmt.Errorf("save independent B state: %w", saveErr)
		return result, err
	}

	parameter, target, found := selectNeutralParameterProbe(snapshotA.Surface.Parameters)
	if !found || !parameter.StableID || parameter.NormalizedValue == nil {
		err = fmt.Errorf("no stable, host-controllable Pro-Q 3 parameter was available for the controlled isolation write")
		return result, err
	}
	artifact.ControlledWrite = proQ3IsolationControlledWrite{
		ParameterID: parameter.ID, ObservedHostLabel: parameter.Name,
		SelectionBasis:     "stable parameter ID ordering and host-controllable flag only; the observed label and ID are not semantic authority",
		PreimageNormalized: *parameter.NormalizedValue, RequestedNormalized: target,
	}
	write, writeErr := proQ3IsolationOperation(ctx, adapterA, "write", map[string]any{
		"changes": []map[string]any{{"id": parameter.ID, "normalized": target}},
	})
	if writeErr != nil {
		err = fmt.Errorf("controlled A write: %w", writeErr)
		return result, err
	}
	artifact.ControlledWrite.TransactionID = stringJSONField(write, "transaction_id")
	artifact.ControlledWrite.WorkerFreshReadbackMatchesRequest = artifact.ControlledWrite.TransactionID != "" && freshReadbackMatchesJSON(write, parameter.ID, target)
	if !artifact.ControlledWrite.WorkerFreshReadbackMatchesRequest {
		err = fmt.Errorf("controlled A write omitted a transaction ID or matching worker fresh readback")
		return result, err
	}
	postWriteA, postWriteErr := proQ3IsolationSnapshot(ctx, adapterA)
	if postWriteErr != nil {
		err = fmt.Errorf("capture fresh A snapshot after controlled write: %w", postWriteErr)
		return result, err
	}
	if value, ok := proQ3IsolationParameterMap(postWriteA.Surface.Parameters)[parameter.ID]; ok && math.Abs(value-target) <= 0.00001 {
		artifact.ControlledWrite.FreshSnapshotMatchesRequest = true
	}
	if !artifact.ControlledWrite.FreshSnapshotMatchesRequest {
		err = fmt.Errorf("fresh A snapshot after controlled write does not match the requested normalized value")
		return result, err
	}
	postWriteAValues = proQ3IsolationParameterMap(postWriteA.Surface.Parameters)

	freshB, freshBErr := proQ3IsolationSnapshot(ctx, adapterB)
	if freshBErr != nil {
		err = fmt.Errorf("capture fresh B snapshot after A write: %w", freshBErr)
		return result, err
	}
	artifact.ResourceIsolation.BPreimageAfterAWrite = proQ3IsolationCompare(initialBValues, proQ3IsolationParameterMap(freshB.Surface.Parameters))
	if !artifact.ResourceIsolation.BPreimageAfterAWrite.Matches {
		err = fmt.Errorf("B fresh snapshot changed after an A-only controlled write")
		return result, err
	}

	artifact.InstanceA.PostWriteState, postWriteAStateBase64, saveErr = proQ3IsolationSaveState(ctx, adapterA)
	if saveErr != nil {
		err = fmt.Errorf("save controlled A state for cross-worker reload: %w", saveErr)
		return result, err
	}
	roundtrip, roundtripErr := proQ3IsolationOperation(ctx, adapterA, "roundtrip_state", map[string]any{})
	if roundtripErr != nil {
		err = fmt.Errorf("roundtrip A state: %w", roundtripErr)
		return result, err
	}
	artifact.StateRoundtrip.Completed = true
	artifact.StateRoundtrip.ParameterReadbackMatchesWorkerPreimage = boolJSONField(roundtrip, "parameter_readback_matches_preimage")
	if !artifact.StateRoundtrip.ParameterReadbackMatchesWorkerPreimage {
		err = fmt.Errorf("A state roundtrip reported a parameter preimage mismatch")
		return result, err
	}
	roundtrippedA, roundtrippedErr := proQ3IsolationSnapshot(ctx, adapterA)
	if roundtrippedErr != nil {
		err = fmt.Errorf("capture fresh A snapshot after state roundtrip: %w", roundtrippedErr)
		return result, err
	}
	artifact.StateRoundtrip.FreshSnapshotMatchesPostWrite = proQ3IsolationCompare(postWriteAValues, proQ3IsolationParameterMap(roundtrippedA.Surface.Parameters))
	if !artifact.StateRoundtrip.FreshSnapshotMatchesPostWrite.Matches {
		err = fmt.Errorf("fresh A snapshot after state roundtrip does not match the complete post-write preimage")
		return result, err
	}

	rollback, rollbackErr := proQ3IsolationOperation(ctx, adapterA, "rollback", map[string]any{"transaction_id": artifact.ControlledWrite.TransactionID})
	if rollbackErr != nil {
		err = fmt.Errorf("rollback controlled A write: %w", rollbackErr)
		return result, err
	}
	artifact.FinalRollback.TransactionRollbackReportedVerified = boolJSONField(rollback, "rollback_verified")
	if !artifact.FinalRollback.TransactionRollbackReportedVerified {
		err = fmt.Errorf("A rollback reported rollback_verified=false")
		return result, err
	}
	rolledBackA, rolledBackErr := proQ3IsolationSnapshot(ctx, adapterA)
	if rolledBackErr != nil {
		err = fmt.Errorf("capture fresh A snapshot after rollback: %w", rolledBackErr)
		return result, err
	}
	artifact.FinalRollback.TransactionRollbackFreshReadback = proQ3IsolationCompare(initialAValues, proQ3IsolationParameterMap(rolledBackA.Surface.Parameters))
	if !artifact.FinalRollback.TransactionRollbackFreshReadback.Matches {
		err = fmt.Errorf("A rollback fresh readback does not match the complete initial preimage")
		return result, err
	}

	unload, unloadErr := proQ3IsolationOperation(ctx, adapterA, "unload", map[string]any{})
	if unloadErr != nil {
		err = fmt.Errorf("unload A before independent worker reload: %w", unloadErr)
		return result, err
	}
	artifact.ReloadRestore.UnloadReported = boolJSONField(unload, "unloaded")
	if !artifact.ReloadRestore.UnloadReported {
		err = fmt.Errorf("A unload did not report unloaded=true")
		return result, err
	}
	loadedA = false

	if _, loadErr := adapterA.load(ctx, loadRequest); loadErr != nil {
		err = fmt.Errorf("reload A into a new native worker: %w", loadErr)
		return result, err
	}
	loadedA = true
	artifact.ReloadRestore.Reloaded = true
	artifact.ReloadRestore.ReloadedWorkerPID = adapterA.workerPID()
	artifact.InstanceA.ReloadedWorkerPID = artifact.ReloadRestore.ReloadedWorkerPID
	artifact.ReloadRestore.ReloadedPIDDiffersFromInitial = artifact.ReloadRestore.ReloadedWorkerPID != 0 && artifact.ReloadRestore.ReloadedWorkerPID != artifact.InstanceA.InitialWorkerPID
	if artifact.ReloadRestore.ReloadedWorkerPID == 0 {
		err = fmt.Errorf("reloaded A did not expose a live isolated worker PID")
		return result, err
	}
	reloadedA, reloadedErr := proQ3IsolationSnapshot(ctx, adapterA)
	if reloadedErr != nil {
		err = fmt.Errorf("capture reloaded A snapshot: %w", reloadedErr)
		return result, err
	}
	if identityErr := proQ3IsolationValidateSnapshot(status.Manifest.PluginIdentity, pluginPath, reloadedA); identityErr != nil {
		err = fmt.Errorf("validate reloaded A identity: %w", identityErr)
		return result, err
	}
	if _, restoreErr := proQ3IsolationOperation(ctx, adapterA, "restore_state", map[string]any{"state_base64": postWriteAStateBase64}); restoreErr != nil {
		err = fmt.Errorf("restore post-write A state after independent reload: %w", restoreErr)
		return result, err
	}
	artifact.ReloadRestore.PostWriteStateRestored = true
	restoredPostWriteA, restoredPostWriteErr := proQ3IsolationSnapshot(ctx, adapterA)
	if restoredPostWriteErr != nil {
		err = fmt.Errorf("capture fresh A snapshot after post-write restore: %w", restoredPostWriteErr)
		return result, err
	}
	artifact.ReloadRestore.FreshSnapshotMatchesPostWrite = proQ3IsolationCompare(postWriteAValues, proQ3IsolationParameterMap(restoredPostWriteA.Surface.Parameters))
	if !artifact.ReloadRestore.FreshSnapshotMatchesPostWrite.Matches {
		err = fmt.Errorf("fresh A snapshot after cross-worker restore does not match the complete saved post-write state")
		return result, err
	}

	if _, restoreErr := proQ3IsolationOperation(ctx, adapterA, "restore_state", map[string]any{"state_base64": initialAStateBase64}); restoreErr != nil {
		err = fmt.Errorf("restore initial A preimage after cross-worker check: %w", restoreErr)
		return result, err
	}
	finalInitialStateRestored = true
	artifact.FinalRollback.ReloadedInitialStateRestored = true
	finalA, finalAErr := proQ3IsolationSnapshot(ctx, adapterA)
	if finalAErr != nil {
		err = fmt.Errorf("capture final A snapshot after initial-state restore: %w", finalAErr)
		return result, err
	}
	artifact.FinalRollback.FinalFreshReadbackMatchesInitial = proQ3IsolationCompare(initialAValues, proQ3IsolationParameterMap(finalA.Surface.Parameters))
	if !artifact.FinalRollback.FinalFreshReadbackMatchesInitial.Matches {
		err = fmt.Errorf("final A fresh readback does not match the complete initial preimage")
		return result, err
	}
	return result, nil
}

func proQ3IsolationSnapshot(ctx context.Context, adapter *VST3HostAdapter) (HostSnapshot, error) {
	response, err := adapter.call(ctx, "snapshot", nil)
	if err != nil {
		return HostSnapshot{}, err
	}
	return NormalizeVST3WorkerSnapshot(response.Result, response.Logs)
}

func proQ3IsolationOperation(ctx context.Context, adapter *VST3HostAdapter, operation string, payload map[string]any) (json.RawMessage, error) {
	response, err := adapter.call(ctx, operation, payload)
	if err != nil {
		return nil, err
	}
	return response.Result, nil
}

func proQ3IsolationValidateSnapshot(expected vps.PluginIdentity, pluginPath string, snapshot HostSnapshot) error {
	if err := samePluginFamily(expected, snapshot.Identity); err != nil {
		return err
	}
	if !sameInstallPath(pluginPath, snapshot.Identity.InstallPath) {
		return fmt.Errorf("loaded VST3 install path does not match the isolated workspace")
	}
	if !isProQ3Identity(snapshot.Identity) {
		return fmt.Errorf("loaded VST3 is not the observed FabFilter Pro-Q 3 reference plugin")
	}
	return nil
}

func proQ3IsolationSaveState(ctx context.Context, adapter *VST3HostAdapter) (proQ3IsolationStateReceipt, string, error) {
	raw, err := proQ3IsolationOperation(ctx, adapter, "save_state", map[string]any{})
	if err != nil {
		return proQ3IsolationStateReceipt{}, "", err
	}
	return proQ3IsolationStateReceiptFromJSON(raw)
}

func proQ3IsolationStateReceiptFromJSON(raw json.RawMessage) (proQ3IsolationStateReceipt, string, error) {
	var payload struct {
		StateSHA256 string `json:"state_sha256"`
		StateBytes  int64  `json:"state_bytes"`
		StateBase64 string `json:"state_base64"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return proQ3IsolationStateReceipt{}, "", fmt.Errorf("decode save-state receipt: %w", err)
	}
	if strings.TrimSpace(payload.StateSHA256) == "" || payload.StateBytes <= 0 || strings.TrimSpace(payload.StateBase64) == "" {
		return proQ3IsolationStateReceipt{}, "", fmt.Errorf("save-state receipt omitted a hash, byte count, or in-memory restore payload")
	}
	return proQ3IsolationStateReceipt{
		StateSHA256: strings.TrimSpace(payload.StateSHA256), StateBytes: payload.StateBytes, Serialized: true, RawStateOmitted: true,
	}, payload.StateBase64, nil
}

func proQ3IsolationSurfaceSummary(snapshot HostSnapshot) proQ3IsolationSurface {
	values := proQ3IsolationParameterMap(snapshot.Surface.Parameters)
	stableIDs := 0
	for _, parameter := range snapshot.Surface.Parameters {
		if parameter.StableID {
			stableIDs++
		}
	}
	return proQ3IsolationSurface{
		ParameterCount: len(snapshot.Surface.Parameters), StableParameterIDs: stableIDs,
		ParameterValueHash: proQ3IsolationParameterDigest(values), CapturedAt: snapshot.Surface.CapturedAt,
	}
}

func proQ3IsolationParameterMap(parameters []SurfaceParameter) map[string]float64 {
	values := make(map[string]float64, len(parameters))
	for _, parameter := range parameters {
		if parameter.ID != "" && parameter.NormalizedValue != nil && finiteHostValue(*parameter.NormalizedValue) {
			values[parameter.ID] = *parameter.NormalizedValue
		}
	}
	return values
}

func proQ3IsolationCompare(expected, actual map[string]float64) proQ3IsolationComparison {
	comparison := proQ3IsolationComparison{
		ExpectedParameterCount: len(expected), ActualParameterCount: len(actual),
		ExpectedValueSHA256: proQ3IsolationParameterDigest(expected), ActualValueSHA256: proQ3IsolationParameterDigest(actual),
	}
	ids := make([]string, 0, len(expected))
	for id := range expected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		actualValue, found := actual[id]
		if found && math.Abs(expected[id]-actualValue) <= 0.00001 {
			continue
		}
		if found {
			comparison.Mismatches = append(comparison.Mismatches, fmt.Sprintf("%s expected %.8f got %.8f", id, expected[id], actualValue))
		} else {
			comparison.Mismatches = append(comparison.Mismatches, fmt.Sprintf("%s missing from fresh surface", id))
		}
		if len(comparison.Mismatches) >= 32 {
			return comparison
		}
	}
	extra := make([]string, 0)
	for id := range actual {
		if _, found := expected[id]; !found {
			extra = append(extra, id)
		}
	}
	sort.Strings(extra)
	for _, id := range extra {
		comparison.Mismatches = append(comparison.Mismatches, fmt.Sprintf("%s unexpectedly present in fresh surface", id))
		if len(comparison.Mismatches) >= 32 {
			return comparison
		}
	}
	comparison.Matches = len(comparison.Mismatches) == 0 && len(expected) == len(actual)
	return comparison
}

func proQ3IsolationParameterDigest(values map[string]float64) string {
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	hash := sha256.New()
	for _, id := range ids {
		_, _ = fmt.Fprintf(hash, "%s=%016x;", id, math.Float64bits(values[id]))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func appendProQ3IsolationError(values []string, message string) []string {
	message = strings.TrimSpace(message)
	if message == "" {
		return values
	}
	if len(message) > 4096 {
		message = message[:4096] + "...[truncated]"
	}
	for _, existing := range values {
		if existing == message {
			return values
		}
	}
	return append(values, message)
}

func persistProQ3ResourceIsolationArtifacts(root string, artifact proQ3ResourceIsolationProbeArtifact) error {
	if err := writeJSON(filepath.Join(root, proQ3ResourceIsolationResultsFile), artifact); err != nil {
		return err
	}
	runArtifact := filepath.FromSlash(strings.TrimSpace(artifact.RunArtifact))
	if runArtifact == "" {
		return fmt.Errorf("Pro-Q 3 resource-isolation run artifact path is required")
	}
	if err := writeJSON(filepath.Join(root, runArtifact), artifact); err != nil {
		return err
	}
	// Keep the evidence ledger bounded and deliberately omit the raw plugin
	// state as well as the full parameter surface (which already belongs in the
	// dedicated observed snapshot artifact).
	ledgerData, _ := json.Marshal(map[string]any{
		"schema_version": artifact.SchemaVersion, "status": artifact.Status,
		"a_worker_pid": artifact.ResourceIsolation.AInitialWorkerPID, "b_worker_pid": artifact.ResourceIsolation.BInitialWorkerPID,
		"separate_worker_processes_observed":        artifact.ResourceIsolation.SeparateWorkerProcessesObserved,
		"b_preimage_after_a_write_matches":          artifact.ResourceIsolation.BPreimageAfterAWrite.Matches,
		"a_transaction_rollback_verified":           artifact.FinalRollback.TransactionRollbackReportedVerified,
		"a_roundtrip_matches_preimage":              artifact.StateRoundtrip.ParameterReadbackMatchesWorkerPreimage,
		"a_cross_worker_restore_matches_post_write": artifact.ReloadRestore.FreshSnapshotMatchesPostWrite.Matches,
		"a_final_fresh_readback_matches_initial":    artifact.FinalRollback.FinalFreshReadbackMatchesInitial.Matches,
		"serialized_state_raw_bytes_persisted":      false,
	})
	if err := appendEvidence(root, EvidenceEntry{
		Kind:       "pro_q_3_resource_isolation_probe",
		Trust:      "observed",
		Source:     "vpsforge.direct_vst3_worker",
		Summary:    "Observed two simultaneous isolated Pro-Q 3 workers, an A-only controlled write, B fresh-readback isolation, A state roundtrip, rollback, independent-worker reload and state restoration. No task badge, Credential, Catalog entry or SPAL route was changed.",
		Artifact:   filepath.ToSlash(runArtifact),
		CapturedAt: artifact.CapturedAt,
		Data:       ledgerData,
	}); err != nil {
		return err
	}
	return updateManifest(root, func(current *Manifest) {
		if current.Artifacts == nil {
			current.Artifacts = map[string]string{}
		}
		current.Artifacts["pro_q_3_resource_isolation_probe_results"] = proQ3ResourceIsolationResultsFile
		current.UpdatedAt = artifact.CapturedAt
	})
}

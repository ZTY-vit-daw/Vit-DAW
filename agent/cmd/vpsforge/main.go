package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/fxm"
	"vit-daw-agent/internal/probeaudio"
	"vit-daw-agent/internal/vps"
	"vit-daw-agent/internal/vpsforge"
)

func main() {
	if len(os.Args) < 2 {
		if err := runDesktop(nil); err != nil {
			fmt.Fprintln(os.Stderr, "vpsforge:", err)
			os.Exit(1)
		}
		return
	}
	var value any
	var err error
	switch os.Args[1] {
	case "init":
		value, err = runInit(os.Args[2:])
	case "status":
		value, err = runStatus(os.Args[2:])
	case "validate":
		value, err = runValidate(os.Args[2:])
	case "ingest-surface":
		value, err = runIngestSurface(os.Args[2:])
	case "measure":
		value, err = runMeasure(os.Args[2:])
	case "probe":
		value, err = runProbe(os.Args[2:])
	case "preflight":
		value, err = runPreflight(os.Args[2:])
	case "stereo-placement-probe":
		value, err = runStereoPlacementProbe(os.Args[2:])
	case "pro-q-3-eq-core-probe":
		value, err = runProQ3CoreEQProbe(os.Args[2:])
	case "pro-q-3-resource-isolation-probe":
		value, err = runProQ3ResourceIsolationProbe(os.Args[2:])
	case "pro-q-3-review-summary":
		value, err = runProQ3ReviewSummary(os.Args[2:])
	case "equalizer-v2-proposal-draft":
		value, err = runEqualizerV2ProposalDraft(os.Args[2:])
	case "equalizer-v2-scope-record":
		value, err = runEqualizerV2ScopeRecord(os.Args[2:])
	case "equalizer-v2-static-eq-matrix":
		value, err = runEqualizerV2StaticEQMatrix(os.Args[2:])
	case "pro-q-3-static-lifecycle-probe":
		value, err = runProQ3StaticLifecycleProbe(os.Args[2:])
	case "equalizer-v2-conformance-decision-draft":
		value, err = runEqualizerV2ConformanceDecisionDraft(os.Args[2:])
	case "pro-q-3-static-eq-draft-test-profile":
		value, err = runProQ3StaticEQDraftTestProfile(os.Args[2:])
	case "pro-q-3-static-eq-stage-vps":
		value, err = runProQ3StaticEQStagingVPS(os.Args[2:])
	case "pro-q-3-equalizer-v2-promotion":
		value, err = runProQ3EqualizerV2Promotion(os.Args[2:])
	case "vit-host-binding":
		value, err = runVitHostBinding(os.Args[2:])
	case "serve":
		err = runServe(os.Args[2:])
		if err == nil {
			return
		}
	case "host":
		err = runHost(os.Args[2:])
		if err == nil {
			return
		}
	case "witness":
		err = runWitness(os.Args[2:])
		if err == nil {
			return
		}
	case "witness-record":
		value, err = runWitnessRecord(os.Args[2:])
	case "desktop":
		err = runDesktop(os.Args[2:])
		if err == nil {
			return
		}
	case "test-audio":
		value, err = runTestAudio(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "vpsforge:", err)
		os.Exit(1)
	}
	encoded, _ := json.MarshalIndent(value, "", "  ")
	fmt.Println(string(encoded))
}

func runInit(args []string) (any, error) {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated authoring workspace directory")
	manufacturer := fs.String("manufacturer", "", "plugin manufacturer")
	name := fs.String("name", "", "plugin name")
	format := fs.String("format", "VST3", "VST3, CLAP, AAX or AU")
	version := fs.String("version", "", "plugin version")
	path := fs.String("install-path", "", "observed plugin installation path")
	capabilities := fs.String("capabilities", "unknown", "comma-separated candidate capability ids")
	host := fs.String("host-endpoint", "", "optional local host-adapter endpoint")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.Init(vpsforge.InitRequest{Root: *workspace, Identity: vps.PluginIdentity{Manufacturer: *manufacturer, Name: *name, Format: *format, Version: *version, InstallPath: *path}, Capabilities: split(*capabilities), HostEndpoint: *host})
}

func runStatus(args []string) (any, error) {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	root := fs.String("workspace", "", "workspace")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.Inspect(*root)
}
func runValidate(args []string) (any, error) {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	root := fs.String("workspace", "", "workspace")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.Validate(*root), nil
}

func runIngestSurface(args []string) (any, error) {
	fs := flag.NewFlagSet("ingest-surface", flag.ContinueOnError)
	root := fs.String("workspace", "", "workspace")
	input := fs.String("input", "", "surface snapshot JSON")
	source := fs.String("source", "manual-host-export", "evidence source")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	var snapshot vpsforge.SurfaceSnapshot
	if err := readJSON(*input, &snapshot); err != nil {
		return nil, err
	}
	return vpsforge.IngestSurface(*root, snapshot, *source, snapshot.CapturedAt)
}

func runMeasure(args []string) (any, error) {
	fs := flag.NewFlagSet("measure", flag.ContinueOnError)
	root := fs.String("workspace", "", "workspace")
	input := fs.String("input", "", "FXM input JSON containing baseline and processed measurements")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	var measurement fxm.Input
	if err := readJSON(*input, &measurement); err != nil {
		return nil, err
	}
	return vpsforge.RecordFXM(*root, measurement, timeFromString(measurement.CreatedAt))
}

func runProbe(args []string) (any, error) {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	root := fs.String("workspace", "", "workspace")
	host := fs.String("host", "", "local plugin-host adapter base URL")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.ProbeHost(context.Background(), *root, *host, nil)
}

func runPreflight(args []string) (any, error) {
	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "new isolated staging workspace directory")
	host := fs.String("host", "", "loopback VST3 host adapter base URL")
	probeAudio := fs.String("probe-audio", defaultProbeAudioPath(), "VPS Forge Probe Audio suite directory")
	badge := fs.String("candidate-badge", "unknown", "authoring-plan candidate task badge; never granted by preflight")
	category := fs.String("category", "", "browse-only classification directory")
	task := fs.String("task", "", "candidate user task identifier")
	features := fs.String("required-features", "", "comma-separated candidate feature conditions")
	runParameterProbe := fs.Bool("run-parameter-probe", true, "run one isolated write/readback/rollback probe")
	runFXM := fs.Bool("run-fxm-default-baseline", true, "render default-state bypass and processed Probe Audio evidence")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.Preflight(context.Background(), vpsforge.PreflightRequest{
		Root: *workspace, HostURL: *host, ProbeAudioRoot: *probeAudio,
		CandidateBadge: *badge, Category: *category, Task: *task, RequiredFeatures: split(*features),
		RunParameterProbe: *runParameterProbe, RunFXMBaseline: *runFXM,
	})
}

// runStereoPlacementProbe is a Pro-Q 3 reference experiment. It uses a
// separate host-worker process, deterministic stereo Probe Audio, state
// roundtrips and verified rollback; all outputs remain under the supplied
// staging workspace and never issue a Credential.
func runStereoPlacementProbe(args []string) (any, error) {
	fs := flag.NewFlagSet("stereo-placement-probe", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight Pro-Q 3 workspace")
	host := fs.String("host", "", "loopback VST3 host adapter base URL")
	probeAudio := fs.String("probe-audio", defaultProbeAudioPath(), "VPS Forge Probe Audio suite directory")
	bandUsed := fs.String("band-used-id", "0", "observed Pro-Q 3 Band 1 Used parameter ID")
	bandEnabled := fs.String("band-enabled-id", "1", "observed Pro-Q 3 Band 1 Enabled parameter ID")
	bandFrequency := fs.String("band-frequency-id", "2", "observed Pro-Q 3 Band 1 Frequency parameter ID")
	bandGain := fs.String("band-gain-id", "3", "observed Pro-Q 3 Band 1 Gain parameter ID")
	bandQ := fs.String("band-q-id", "7", "observed Pro-Q 3 Band 1 Q parameter ID")
	bandShape := fs.String("band-shape-id", "8", "observed Pro-Q 3 Band 1 Shape parameter ID")
	placement := fs.String("placement-id", "10", "observed Pro-Q 3 Band 1 Stereo Placement parameter ID")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.RunStereoPlacementProbe(context.Background(), vpsforge.StereoPlacementProbeRequest{
		Root: *workspace, HostURL: *host, ProbeAudioRoot: *probeAudio,
		BandUsedParameterID: *bandUsed, BandEnabledParameterID: *bandEnabled,
		BandFrequencyParameterID: *bandFrequency, BandGainParameterID: *bandGain,
		BandQParameterID: *bandQ, BandShapeParameterID: *bandShape, PlacementParameterID: *placement,
		Now: time.Now().UTC(),
	})
}

// runProQ3CoreEQProbe is a staging-only, observed behavior experiment for
// the first Pro-Q 3 reference workspace. It does not create a VPS mapping,
// issue a Credential, update a Catalog or enable a SPAL route.
func runProQ3CoreEQProbe(args []string) (any, error) {
	fs := flag.NewFlagSet("pro-q-3-eq-core-probe", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight Pro-Q 3 workspace")
	host := fs.String("host", "", "loopback VST3 host adapter base URL")
	probeAudio := fs.String("probe-audio", defaultProbeAudioPath(), "VPS Forge Probe Audio suite directory")
	bandUsed := fs.String("band-used-id", "0", "observed Pro-Q 3 Band 1 Used parameter ID")
	bandEnabled := fs.String("band-enabled-id", "1", "observed Pro-Q 3 Band 1 Enabled parameter ID")
	bandFrequency := fs.String("band-frequency-id", "2", "observed Pro-Q 3 Band 1 Frequency parameter ID")
	bandGain := fs.String("band-gain-id", "3", "observed Pro-Q 3 Band 1 Gain parameter ID")
	bandQ := fs.String("band-q-id", "7", "observed Pro-Q 3 Band 1 Q parameter ID")
	bandShape := fs.String("band-shape-id", "8", "observed Pro-Q 3 Band 1 Shape parameter ID")
	bandSlope := fs.String("band-slope-id", "9", "observed Pro-Q 3 Band 1 Slope parameter ID")
	placement := fs.String("placement-id", "10", "observed Pro-Q 3 Band 1 Stereo Placement parameter ID")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.RunProQ3CoreEQProbe(context.Background(), vpsforge.ProQ3CoreEQProbeRequest{
		Root: *workspace, HostURL: *host, ProbeAudioRoot: *probeAudio,
		BandUsedParameterID: *bandUsed, BandEnabledParameterID: *bandEnabled,
		BandFrequencyParameterID: *bandFrequency, BandGainParameterID: *bandGain,
		BandQParameterID: *bandQ, BandShapeParameterID: *bandShape, BandSlopeParameterID: *bandSlope, PlacementParameterID: *placement,
		Now: time.Now().UTC(),
	})
}

// runProQ3ResourceIsolationProbe uses two direct crash-isolated workers. It
// does not contact the long-lived HTTP adapter or alter any VPS authority
// state; its output is staging-only observed evidence.
func runProQ3ResourceIsolationProbe(args []string) (any, error) {
	fs := flag.NewFlagSet("pro-q-3-resource-isolation-probe", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight Pro-Q 3 workspace")
	worker := fs.String("worker", defaultVST3WorkerPath(), "native vpsforge_vst3_worker.exe path")
	sampleRate := fs.Float64("sample-rate", 48000, "native worker sample rate")
	blockSize := fs.Int("block-size", 512, "native worker block size")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.RunProQ3ResourceIsolationProbe(context.Background(), vpsforge.ProQ3ResourceIsolationProbeRequest{
		Root: *workspace, WorkerPath: *worker, SampleRate: *sampleRate, BlockSize: *blockSize, Now: time.Now().UTC(),
	})
}

// runProQ3ReviewSummary creates a read-only, agent-inferred audit handoff
// from already staged evidence. It never alters the VPS draft's mappings or
// grants a task badge, Credential, Catalog entry or SPAL route.
func runProQ3ReviewSummary(args []string) (any, error) {
	fs := flag.NewFlagSet("pro-q-3-review-summary", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight Pro-Q 3 workspace")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.WriteProQ3EqualizerReviewSummary(vpsforge.ProQ3EqualizerReviewSummaryRequest{
		Root: *workspace, Now: time.Now().UTC(),
	})
}

// runEqualizerV2ProposalDraft creates a local review-only draft from existing
// staged evidence. It has no submission route and cannot freeze a badge or
// change any Credential, Catalog, SPAL route, or canonical VPS artifact.
func runEqualizerV2ProposalDraft(args []string) (any, error) {
	fs := flag.NewFlagSet("equalizer-v2-proposal-draft", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight Pro-Q 3 workspace")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.WriteEqualizerV2ConformanceProposalDraft(vpsforge.EqualizerV2ConformanceProposalDraftRequest{
		Root: *workspace, Now: time.Now().UTC(),
	})
}

// runEqualizerV2ScopeRecord archives an explicit user-confirmed first-version
// boundary without defining executable mappings or changing a badge.
func runEqualizerV2ScopeRecord(args []string) (any, error) {
	fs := flag.NewFlagSet("equalizer-v2-scope-record", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight Pro-Q 3 workspace")
	statement := fs.String("statement", "", "verbatim user confirmation of the proposed first-version scope")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.RecordEqualizerV2ScopeReview(vpsforge.EqualizerV2ScopeReviewRequest{
		Root: *workspace, UserStatement: *statement, Now: time.Now().UTC(),
	})
}

// runEqualizerV2StaticEQMatrix derives an agent-inferred coverage matrix from
// the explicit scope, observed machine evidence and user-confirmed records.
// It cannot define executable action mappings or change a badge.
func runEqualizerV2StaticEQMatrix(args []string) (any, error) {
	fs := flag.NewFlagSet("equalizer-v2-static-eq-matrix", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight Pro-Q 3 workspace")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.WriteEqualizerV2StaticEQReviewMatrix(vpsforge.EqualizerV2StaticEQReviewMatrixRequest{
		Root: *workspace, Now: time.Now().UTC(),
	})
}

// runProQ3StaticLifecycleProbe is a direct-worker, staging-only check of
// bounded Used/Enabled lifecycle candidates. It does not create a badge
// mapping or alter any authorization-bearing artifact.
func runProQ3StaticLifecycleProbe(args []string) (any, error) {
	fs := flag.NewFlagSet("pro-q-3-static-lifecycle-probe", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight Pro-Q 3 workspace")
	worker := fs.String("worker", defaultVST3WorkerPath(), "native vpsforge_vst3_worker.exe path")
	probeAudio := fs.String("probe-audio", defaultProbeAudioPath(), "VPS Forge Probe Audio suite directory")
	bandUsed := fs.String("band-used-id", "0", "observed Pro-Q 3 Band 1 Used parameter ID")
	bandEnabled := fs.String("band-enabled-id", "1", "observed Pro-Q 3 Band 1 Enabled parameter ID")
	bandFrequency := fs.String("band-frequency-id", "2", "observed Pro-Q 3 Band 1 Frequency parameter ID")
	bandGain := fs.String("band-gain-id", "3", "observed Pro-Q 3 Band 1 Gain parameter ID")
	bandQ := fs.String("band-q-id", "7", "observed Pro-Q 3 Band 1 Q parameter ID")
	bandShape := fs.String("band-shape-id", "8", "observed Pro-Q 3 Band 1 Shape parameter ID")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.RunProQ3StaticLifecycleProbe(context.Background(), vpsforge.ProQ3StaticLifecycleProbeRequest{
		Root: *workspace, WorkerPath: *worker, ProbeAudioRoot: *probeAudio,
		BandUsedParameterID: *bandUsed, BandEnabledParameterID: *bandEnabled,
		BandFrequencyParameterID: *bandFrequency, BandGainParameterID: *bandGain,
		BandQParameterID: *bandQ, BandShapeParameterID: *bandShape, Now: time.Now().UTC(),
	})
}

// runEqualizerV2ConformanceDecisionDraft synthesizes a non-authoritative
// decision handoff from the review matrix and observed staging results.
func runEqualizerV2ConformanceDecisionDraft(args []string) (any, error) {
	fs := flag.NewFlagSet("equalizer-v2-conformance-decision-draft", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight Pro-Q 3 workspace")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.WriteEqualizerV2ConformanceDecisionDraft(vpsforge.EqualizerV2ConformanceDecisionDraftRequest{
		Root: *workspace, Now: time.Now().UTC(),
	})
}

// runProQ3StaticEQDraftTestProfile prepares an isolated candidate library for
// real local Vit Draft-test transport. It contains no Credential and cannot
// become visible to the Provider Catalog or SPAL.
func runProQ3StaticEQDraftTestProfile(args []string) (any, error) {
	fs := flag.NewFlagSet("pro-q-3-static-eq-draft-test-profile", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight Pro-Q 3 workspace")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.WriteProQ3StaticEQDraftTestProfile(vpsforge.ProQ3StaticEQDraftTestProfileRequest{
		Root: *workspace, Now: time.Now().UTC(),
	})
}

func runProQ3StaticEQStagingVPS(args []string) (any, error) {
	fs := flag.NewFlagSet("pro-q-3-static-eq-stage-vps", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight Pro-Q 3 workspace")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.WriteProQ3StaticEQStagingVPS(vpsforge.ProQ3StaticEQStagingVPSRequest{Root: *workspace, Now: time.Now().UTC()})
}

// runProQ3EqualizerV2Promotion assembles the current independent-Adapter,
// Vit-host and archived user-witness evidence into a formal candidate.  It is
// dry-run by default.  A canonical Library/Catalog admission needs the exact
// explicit authorization phrase documented by the command output.
func runProQ3EqualizerV2Promotion(args []string) (any, error) {
	fs := flag.NewFlagSet("pro-q-3-equalizer-v2-promotion", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "current isolated r4-style Pro-Q 3 staging workspace")
	witnessWorkspace := fs.String("witness-workspace", "", "archived Pro-Q 3 workspace containing user GUI and direct-worker evidence")
	canonicalLibrary := fs.String("canonical-library", vps.DefaultLibraryPath(), "canonical local VPS Library; dry-run does not write it")
	authorization := fs.String("authorization", "", "exact phrase required to atomically write the canonical VPS Library")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.PromoteProQ3EqualizerV2(vpsforge.ProQ3EqualizerV2PromotionRequest{
		Root: *workspace, WitnessWorkspace: *witnessWorkspace, CanonicalLibraryPath: *canonicalLibrary,
		Authorization: *authorization, Now: time.Now().UTC(),
	})
}

// runVitHostBinding records the explicit, observed mapping boundary between
// the independent Adapter surface and one real Vit host instance.  It is
// staging-only and cannot create a Credential, Catalog entry or SPAL route.
func runVitHostBinding(args []string) (any, error) {
	fs := flag.NewFlagSet("vit-host-binding", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight workspace")
	agentURL := fs.String("agent", "http://127.0.0.1:7878", "loopback Vit Agent base URL")
	trackID := fs.String("track-id", "", "selected Vit track id")
	pluginID := fs.String("plugin-id", "", "selected Vit plugin instance id")
	required := fs.String("required-params", "", "comma-separated explicit physical parameter IDs required by the reviewed action")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.RecordVitHostBinding(context.Background(), vpsforge.VitHostBindingRequest{
		Root: *workspace, AgentURL: *agentURL, TrackID: *trackID, PluginID: *pluginID, RequiredParameterIDs: split(*required), Now: time.Now().UTC(),
	})
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	root := fs.String("workspace", "", "workspace")
	listen := fs.String("listen", "127.0.0.1:8899", "loopback listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("vpsforge serve must bind to a loopback address")
	}
	if _, err := vpsforge.Inspect(*root); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "vpsforge serving %s on http://%s\n", *root, *listen)
	return http.ListenAndServe(*listen, vpsforge.Handler(*root))
}

// runHost starts the independent VST3 adapter. It is intentionally separate
// from `serve`: this process can host a plugin but does not own a Forge
// workspace, VPS Library, Credential, Catalog, or SPAL route.
func runHost(args []string) error {
	fs := flag.NewFlagSet("host", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:8900", "loopback listen address")
	worker := fs.String("worker", defaultVST3WorkerPath(), "native vpsforge_vst3_worker.exe path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("vpsforge host must bind to a loopback address")
	}
	adapter := vpsforge.NewVST3HostAdapter(*worker)
	defer adapter.Close()
	fmt.Fprintf(os.Stderr, "vpsforge VST3 host adapter listening on http://%s\n", *listen)
	return http.ListenAndServe(*listen, adapter.Handler())
}

// runWitness starts a one-plugin, user-operated GUI process for manual
// evidence. It deliberately does not start Vit and never writes a VPS Library,
// Credential, Catalog, or SPAL route. The GUI process is separate from the
// Forge workbench so a plug-in crash is contained to that process.
func runWitness(args []string) error {
	fs := flag.NewFlagSet("witness", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated preflight workspace; uses its observed VST3 install path")
	pluginPath := fs.String("plugin-path", "", "VST3 path when no workspace is supplied")
	witness := fs.String("witness", defaultVST3WitnessPath(), "native vpsforge_vst3_witness.exe path")
	audioFile := fs.String("audio-file", "", "optional local WAV/AIFF source for concurrent live witnessing")
	noLoop := fs.Bool("no-loop", false, "do not loop the selected live witness source")
	inputGainDB := fs.Float64("input-gain-db", -18, "live witness source input gain in dB (-60..0)")
	sampleRate := fs.Float64("sample-rate", 48000, "GUI host sample rate")
	blockSize := fs.Int("block-size", 512, "GUI host block size")
	if err := fs.Parse(args); err != nil {
		return err
	}

	resolvedPlugin := strings.TrimSpace(*pluginPath)
	if strings.TrimSpace(*workspace) != "" {
		status, err := vpsforge.Inspect(*workspace)
		if err != nil {
			return fmt.Errorf("inspect witness workspace: %w", err)
		}
		expected := strings.TrimSpace(status.Manifest.PluginIdentity.InstallPath)
		if expected == "" {
			return fmt.Errorf("witness workspace has no observed VST3 install path")
		}
		if resolvedPlugin == "" {
			resolvedPlugin = expected
		} else if !strings.EqualFold(filepath.Clean(resolvedPlugin), filepath.Clean(expected)) {
			return fmt.Errorf("-plugin-path must match the isolated workspace's observed install path")
		}
	}
	if resolvedPlugin == "" {
		return fmt.Errorf("witness requires -workspace or -plugin-path")
	}
	if !strings.EqualFold(filepath.Ext(resolvedPlugin), ".vst3") {
		return fmt.Errorf("witness supports only a .vst3 plugin path")
	}
	if _, err := os.Stat(resolvedPlugin); err != nil {
		return fmt.Errorf("witness VST3 path is not accessible: %w", err)
	}
	if _, err := os.Stat(*witness); err != nil {
		return fmt.Errorf("witness executable is not available: %w", err)
	}
	if *sampleRate < 8000 || *sampleRate > 384000 {
		return fmt.Errorf("witness -sample-rate must be within 8000..384000")
	}
	if *blockSize < 16 || *blockSize > 8192 {
		return fmt.Errorf("witness -block-size must be within 16..8192")
	}
	if *inputGainDB < -60 || *inputGainDB > 0 {
		return fmt.Errorf("witness -input-gain-db must be within -60..0")
	}
	resolvedAudioFile := strings.TrimSpace(*audioFile)
	if resolvedAudioFile != "" {
		info, err := os.Stat(resolvedAudioFile)
		if err != nil {
			return fmt.Errorf("witness audio source is not accessible: %w", err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("witness audio source must be a regular file")
		}
	}

	commandArgs := []string{
		"--plugin-path", resolvedPlugin,
		"--sample-rate", strconv.FormatFloat(*sampleRate, 'f', -1, 64),
		"--block-size", strconv.Itoa(*blockSize),
		"--input-gain-db", strconv.FormatFloat(*inputGainDB, 'f', -1, 64),
	}
	if resolvedAudioFile != "" {
		commandArgs = append(commandArgs, "--audio-file", resolvedAudioFile)
	}
	if *noLoop {
		commandArgs = append(commandArgs, "--no-loop")
	}
	command := exec.Command(*witness, commandArgs...)
	command.Dir = filepath.Dir(resolvedPlugin)
	return command.Start()
}

// runWitnessRecord archives raw user GUI testimony in an existing isolated
// preflight workspace. It neither interprets the testimony as a semantic
// mapping nor writes a VPS Library, Credential, Catalog, or SPAL route.
func runWitnessRecord(args []string) (any, error) {
	fs := flag.NewFlagSet("witness-record", flag.ContinueOnError)
	workspace := fs.String("workspace", "", "isolated staging/preflight workspace")
	roundID := fs.String("round", "", "existing witness round id")
	statement := fs.String("statement", "", "raw user GUI witness statement; retained verbatim")
	promptContext := fs.String("context", "", "optional prompt or claim the raw user statement is confirming")
	var screenshots repeatedStringFlag
	fs.Var(&screenshots, "screenshot", "path to a PNG, JPG, JPEG or WEBP screenshot; repeat for multiple attachments")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return vpsforge.RecordHumanWitness(vpsforge.HumanWitnessRecordRequest{
		Root:            *workspace,
		RoundID:         *roundID,
		UserStatement:   *statement,
		PromptContext:   *promptContext,
		ScreenshotPaths: screenshots,
		Now:             time.Now().UTC(),
	})
}

func defaultVST3WorkerPath() string {
	executable, err := os.Executable()
	if err != nil {
		return "vpsforge_vst3_worker.exe"
	}
	root := filepath.Dir(executable)
	// This first candidate is the release packaging location. The second makes
	// local source-tree development reproducible without touching VitApp.
	for _, candidate := range []string{
		filepath.Join(root, "vpsforge_vst3_worker.exe"),
		filepath.Join(root, "native-host", "build", "vpsforge_vst3_worker_artefacts", "Release", "vpsforge_vst3_worker.exe"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join(root, "vpsforge_vst3_worker.exe")
}

func defaultVST3WitnessPath() string {
	executable, err := os.Executable()
	if err != nil {
		return "vpsforge_vst3_witness.exe"
	}
	root := filepath.Dir(executable)
	// Keep the user-facing launcher next to vpsforge.exe. The build-tree
	// fallback is intentionally only for local Forge development.
	for _, candidate := range []string{
		filepath.Join(root, "vpsforge_vst3_witness.exe"),
		filepath.Join(root, "native-host", "build", "vpsforge_vst3_witness_artefacts", "Release", "vpsforge_vst3_witness.exe"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return filepath.Join(root, "vpsforge_vst3_witness.exe")
}

func defaultProbeAudioPath() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	root := filepath.Dir(executable)
	for _, candidate := range []string{
		filepath.Join(root, "assets", "vps_probe_audio_v1"),
		filepath.Join(root, "..", "assets", "vps_probe_audio_v1"),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func runDesktop(args []string) error {
	fs := flag.NewFlagSet("desktop", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:0", "loopback listen address; port 0 selects a free port")
	open := fs.Bool("open-browser", true, "open the workbench in the default browser")
	if err := fs.Parse(args); err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("vpsforge desktop must bind to a loopback address")
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	url := "http://" + listener.Addr().String() + "/"
	fmt.Fprintln(os.Stderr, "Vit VPS Forge:", url)
	if *open {
		_ = openBrowser(url)
	}
	manager := vpsforge.NewDesktopManager(vpsforge.DesktopOptions{
		StagingRoot:       defaultPreflightStagingPath(),
		WitnessExecutable: defaultVST3WitnessPath(),
	})
	return http.Serve(listener, manager.Handler())
}

func defaultPreflightStagingPath() string {
	executable, err := os.Executable()
	if err != nil {
		return "staging"
	}
	return filepath.Join(filepath.Dir(executable), "staging", "preflight")
}

func runTestAudio(args []string) (any, error) {
	fs := flag.NewFlagSet("test-audio", flag.ContinueOnError)
	output := fs.String("output", "", "probe-audio output directory")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return probeaudio.Generate(*output, time.Now().UTC())
}

func openBrowser(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url)
	case "darwin":
		command = exec.Command("open", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	return command.Start()
}

func readJSON(path string, output any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, output)
}
func split(value string) []string {
	out := []string{}
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
func timeFromString(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	return parsed
}

type repeatedStringFlag []string

func (values *repeatedStringFlag) String() string {
	return strings.Join(*values, ",")
}

func (values *repeatedStringFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: vpsforge <desktop|init|status|validate|ingest-surface|measure|probe|preflight|stereo-placement-probe|pro-q-3-eq-core-probe|pro-q-3-static-lifecycle-probe|pro-q-3-resource-isolation-probe|pro-q-3-review-summary|equalizer-v2-proposal-draft|equalizer-v2-scope-record|equalizer-v2-static-eq-matrix|equalizer-v2-conformance-decision-draft|pro-q-3-static-eq-draft-test-profile|pro-q-3-static-eq-stage-vps|pro-q-3-equalizer-v2-promotion|serve|host|witness|witness-record|test-audio> [flags]")
}

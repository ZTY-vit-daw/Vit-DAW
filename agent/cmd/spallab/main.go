// spallab is a deliberately explicit developer CLI for the SPAL v0 reference
// vertical slice. It is not a user-facing mixing command and never loads a
// plug-in automatically.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"vit-daw-agent/internal/executionports"
	"vit-daw-agent/internal/executionverifiers"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/projectcut"
	"vit-daw-agent/internal/spal"
	"vit-daw-agent/internal/spallab"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "SPAL Lab:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return errors.New("a SPAL Lab command is required")
	}
	switch args[0] {
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	case "conform":
		return runConform(args[1:])
	case "plan":
		return runPlan(args[1:])
	case "rollback-plan":
		return runRollbackPlan(args[1:])
	case "approve":
		return runApprove(args[1:])
	case "execute":
		return runExecute(args[1:])
	case "status":
		return runStatus(args[1:])
	case "signal-check":
		return runSignalCheck(args[1:])
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown SPAL Lab command %q", args[0])
	}
}

type commonFlags struct {
	kernelEndpoint string
	timeout        time.Duration
	providerStore  string
	sessionStore   string
	journal        string
}

func addCommonFlags(fs *flag.FlagSet, includeKernel bool) *commonFlags {
	flags := &commonFlags{}
	if includeKernel {
		fs.StringVar(&flags.kernelEndpoint, "kernel", envOr("VIT_AGENT_ZMQ_REQ_URL", "tcp://127.0.0.1:5555"), "VSP kernel ZMQ endpoint")
		fs.DurationVar(&flags.timeout, "timeout", 120*time.Second, "kernel request timeout")
	}
	fs.StringVar(&flags.providerStore, "provider-store", spallab.DefaultProviderStorePath(), "SPAL Lab provider store")
	fs.StringVar(&flags.sessionStore, "session-store", spallab.DefaultSessionStorePath(), "SPAL Lab orchestration store")
	fs.StringVar(&flags.journal, "journal", spallab.DefaultJournalPath(), "SPAL Lab execution journal")
	return flags
}

func runConform(args []string) error {
	fs := flag.NewFlagSet("conform", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	common := addCommonFlags(fs, true)
	trackID := fs.String("track-id", "", "existing target track id")
	pluginID := fs.String("plugin-id", "", "existing TDR Nova plug-in id")
	targetRef := fs.String("target-ref", "", "semantic target reference, e.g. track:bass")
	bandSlot := fs.String("band-slot", "band1", "explicit TDR Nova band slot")
	confirmed := fs.Bool("static-bell-confirmed", false, "operator explicitly confirmed the selected band is a static Bell")
	evidence := fs.String("evidence", "", "comma-separated conformance evidence refs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := require(*trackID, "track-id"); err != nil {
		return err
	}
	if err := require(*pluginID, "plugin-id"); err != nil {
		return err
	}
	if err := require(*targetRef, "target-ref"); err != nil {
		return err
	}
	client := kernel.New(common.kernelEndpoint, common.timeout)
	ctx, cancel := context.WithTimeout(context.Background(), common.timeout)
	defer cancel()
	result, err := client.SendVSPLegacyCommand(ctx, map[string]any{"cmd": "get_plugin_parameters", "track_id": *trackID, "plugin_id": *pluginID})
	if err != nil {
		return fmt.Errorf("read current plug-in parameters: %w", err)
	}
	reply := result.LegacyLikeReply()
	if err := requireOK(reply); err != nil {
		return fmt.Errorf("read current plug-in parameters: %w", err)
	}
	record, err := spallab.ConformTDRNovaFromPluginReply(spallab.TDRNovaConformanceRequest{
		TargetRef: *targetRef, TrackID: *trackID, PluginID: *pluginID, BandSlot: *bandSlot,
		StaticBellConfirmed: *confirmed, StaticBellEvidence: splitRefs(*evidence),
	}, reply)
	if err != nil {
		return err
	}
	store, err := spallab.NewProviderStore(common.providerStore)
	if err != nil {
		return err
	}
	record, err = store.Upsert(record)
	if err != nil {
		return err
	}
	return printJSON(record)
}

func runPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	common := addCommonFlags(fs, true)
	sessionID := fs.String("session-id", "", "new laboratory session id")
	goal := fs.String("goal", "SPAL Lab static Bell check", "human-readable lab goal")
	targetRef := fs.String("target-ref", "", "semantic target reference")
	frequency := fs.Float64("frequency-hz", 0, "semantic center frequency in Hz")
	gain := fs.Float64("gain-db", 0, "semantic gain in dB")
	q := fs.Float64("q", 0, "semantic Q")
	maxGain := fs.Float64("max-abs-gain-db", 0, "optional semantic gain safety bound")
	evidence := fs.String("evidence", "", "comma-separated diagnosis/lab evidence refs")
	bandLow := fs.Float64("expected-band-low-hz", 0, "optional expected-signal low frequency")
	bandHigh := fs.Float64("expected-band-high-hz", 0, "optional expected-signal high frequency")
	direction := fs.String("expected-direction", "", "optional expected signal direction: increase|decrease")
	probeClipID := fs.String("signal-probe-clip-id", "", "optional frozen clip id for automatic same-tap L2 verification")
	probeStart := fs.String("signal-probe-start-seconds", "", "optional frozen verification range start (requires end)")
	probeEnd := fs.String("signal-probe-end-seconds", "", "optional frozen verification range end (requires start)")
	probeTail := fs.String("signal-probe-tail-seconds", "", "optional frozen L2 render tail in seconds (0..10)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *sessionID == "" {
		*sessionID = fmt.Sprintf("spal_lab_%d", time.Now().UnixNano())
	}
	for _, item := range []struct {
		name  string
		value string
	}{{"target-ref", *targetRef}} {
		if err := require(item.value, item.name); err != nil {
			return err
		}
	}
	if *frequency <= 0 || *q <= 0 {
		return errors.New("frequency-hz and q must be positive")
	}
	if *maxGain < 0 {
		return errors.New("max-abs-gain-db must be positive when supplied")
	}
	if err := validateSignalFlags(*bandLow, *bandHigh, *direction); err != nil {
		return err
	}
	probeScope, err := signalProbeScope(*probeClipID, *probeStart, *probeEnd, *probeTail)
	if err != nil {
		return err
	}
	client := kernel.New(common.kernelEndpoint, common.timeout)
	ctx, cancel := context.WithTimeout(context.Background(), common.timeout)
	defer cancel()
	available, err := client.VSPFeature(ctx, "plugin.set_params_batch")
	if err != nil {
		return fmt.Errorf("negotiate VSP plugin batch capability: %w", err)
	}
	if !available {
		return errors.New("current VSP kernel does not advertise plugin.set_params_batch")
	}
	state, err := client.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil {
		return fmt.Errorf("read current VSP Project Cut: %w", err)
	}
	cut, err := projectcut.Build(projectcut.BuildRequest{
		State: state, Guarantee: projectcut.GuaranteeKernelBarrier,
		DependencyFingerprints: []string{"spal_lab.provider_store:" + common.providerStore},
		TargetFingerprints:     []string{"spal_lab.target:" + *targetRef},
		ContractVersions:       []string{spallab.SchemaVersion, spal.SchemaVersion},
	})
	if err != nil {
		return fmt.Errorf("build SPAL Lab Project Cut: %w", err)
	}
	providers, err := spallab.NewProviderStore(common.providerStore)
	if err != nil {
		return err
	}
	sessions, err := orchestration.NewFileStore(common.sessionStore)
	if err != nil {
		return err
	}
	lab, err := spallab.New(providers, sessions)
	if err != nil {
		return err
	}
	instruction := spal.Instruction{
		SchemaID: spal.StaticBellControlID, TargetRef: *targetRef,
		Parameters:   map[string]float64{"center_frequency_hz": *frequency, "gain_db": *gain, "q": *q},
		EvidenceRefs: splitRefs(*evidence),
	}
	if *maxGain > 0 {
		instruction.SafetyBounds.MaxAbsoluteGainDB = maxGain
	}
	if *direction != "" {
		instruction.ExpectedSignalChange = spal.SignalExpectation{BandLowHz: *bandLow, BandHighHz: *bandHigh, Direction: *direction}
	}
	instruction.SignalProbeScope = probeScope
	planned, err := lab.Plan(ctx, spallab.PlanRequest{
		SessionID: *sessionID, Goal: *goal, ProjectCut: cut, Instruction: instruction,
		PreimageReader: executionports.SPALVSPInspector{Client: client}, ValidateRecord: liveRecordValidator(client),
	})
	if err != nil {
		return err
	}
	return printJSON(planned)
}

func runApprove(args []string) error {
	fs := flag.NewFlagSet("approve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	common := addCommonFlags(fs, false)
	sessionID := fs.String("session-id", "", "planned SPAL Lab session id")
	proposalID := fs.String("proposal-id", "", "exact frozen proposal id displayed by plan")
	source := fs.String("source", "manual-spal-lab-confirmation", "human approval source reference")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := require(*sessionID, "session-id"); err != nil {
		return err
	}
	if err := require(*proposalID, "proposal-id"); err != nil {
		return err
	}
	providers, err := spallab.NewProviderStore(common.providerStore)
	if err != nil {
		return err
	}
	sessions, err := orchestration.NewFileStore(common.sessionStore)
	if err != nil {
		return err
	}
	lab, err := spallab.New(providers, sessions)
	if err != nil {
		return err
	}
	approved, err := lab.Approve(*sessionID, *proposalID, *source)
	if err != nil {
		return err
	}
	return printJSON(approved)
}

func runRollbackPlan(args []string) error {
	fs := flag.NewFlagSet("rollback-plan", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	common := addCommonFlags(fs, true)
	originalSessionID := fs.String("session-id", "", "original completed SPAL Lab session id")
	rollbackSessionID := fs.String("rollback-session-id", "", "new rollback SPAL Lab session id")
	goal := fs.String("goal", "", "optional rollback goal")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := require(*originalSessionID, "session-id"); err != nil {
		return err
	}
	if *rollbackSessionID == "" {
		*rollbackSessionID = fmt.Sprintf("spal_lab_rollback_%d", time.Now().UnixNano())
	}
	client := kernel.New(common.kernelEndpoint, common.timeout)
	ctx, cancel := context.WithTimeout(context.Background(), common.timeout)
	defer cancel()
	available, err := client.VSPFeature(ctx, "plugin.set_params_batch")
	if err != nil {
		return fmt.Errorf("negotiate VSP plugin batch capability: %w", err)
	}
	if !available {
		return errors.New("current VSP kernel does not advertise plugin.set_params_batch")
	}
	state, err := client.VSPStateSnapshot(ctx, "project.timeline")
	if err != nil {
		return fmt.Errorf("read current VSP Project Cut: %w", err)
	}
	cut, err := projectcut.Build(projectcut.BuildRequest{
		State: state, Guarantee: projectcut.GuaranteeKernelBarrier,
		DependencyFingerprints: []string{"spal_lab.rollback_of:" + *originalSessionID},
		ContractVersions:       []string{spallab.SchemaVersion, spal.SchemaVersion},
	})
	if err != nil {
		return fmt.Errorf("build SPAL Lab rollback Project Cut: %w", err)
	}
	providers, err := spallab.NewProviderStore(common.providerStore)
	if err != nil {
		return err
	}
	sessions, err := orchestration.NewFileStore(common.sessionStore)
	if err != nil {
		return err
	}
	lab, err := spallab.New(providers, sessions)
	if err != nil {
		return err
	}
	planned, err := lab.PlanRollback(ctx, spallab.RollbackPlanRequest{
		SessionID: *rollbackSessionID, OriginalSessionID: *originalSessionID, Goal: *goal,
		ProjectCut: cut, PreimageReader: executionports.SPALVSPInspector{Client: client}, ValidateRecord: liveRecordValidator(client),
	})
	if err != nil {
		return err
	}
	return printJSON(planned)
}

func runExecute(args []string) error {
	fs := flag.NewFlagSet("execute", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	common := addCommonFlags(fs, true)
	sessionID := fs.String("session-id", "", "approved SPAL Lab session id")
	kernelSubURL := fs.String("kernel-sub", envOr("VIT_AGENT_ZMQ_SUB_URL", "tcp://127.0.0.1:5556"), "kernel ZMQ PUB endpoint for same-tap L2 signal evidence")
	signalProbeTimeout := fs.Duration("signal-probe-timeout", 30*time.Second, "maximum wait for each L2 signal render probe")
	autoSignalProbe := fs.Bool("auto-l2-signal-probe", true, "capture frozen same-tap L2 signal evidence when the kernel supports it")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := require(*sessionID, "session-id"); err != nil {
		return err
	}
	providers, err := spallab.NewProviderStore(common.providerStore)
	if err != nil {
		return err
	}
	sessions, err := orchestration.NewFileStore(common.sessionStore)
	if err != nil {
		return err
	}
	lab, err := spallab.New(providers, sessions)
	if err != nil {
		return err
	}
	journal, err := spallab.NewJournal(common.journal)
	if err != nil {
		return err
	}
	client := kernel.New(common.kernelEndpoint, common.timeout)
	ctx, cancel := context.WithTimeout(context.Background(), common.timeout)
	defer cancel()
	var signal executionports.SPALSignalCapturer
	if *autoSignalProbe {
		available, err := client.VSPFeature(ctx, "audio.l2_render_probe")
		if err != nil {
			return fmt.Errorf("negotiate VSP L2 render probe capability: %w", err)
		}
		if available {
			signal = executionports.SPALL2RenderProbe{Client: client, SubscriptionURL: *kernelSubURL, Timeout: *signalProbeTimeout}
		}
	}
	port := &executionports.SPALSignalCapturePort{
		Mutation: &executionports.SPALVSPPort{Client: client},
		Signal:   signal,
	}
	completed, err := lab.Execute(ctx, *sessionID, port, executionverifiers.SPAL{Signal: executionverifiers.ReceiptSPALSignalProbe{}}, journal)
	if err != nil {
		return err
	}
	return printJSON(completed)
}

func runStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	common := addCommonFlags(fs, false)
	sessionID := fs.String("session-id", "", "SPAL Lab session id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := require(*sessionID, "session-id"); err != nil {
		return err
	}
	sessions, err := orchestration.NewFileStore(common.sessionStore)
	if err != nil {
		return err
	}
	session, ok := sessions.Load(*sessionID)
	if !ok {
		return fmt.Errorf("SPAL Lab session %s not found", *sessionID)
	}
	return printJSON(session)
}

func runSignalCheck(args []string) error {
	fs := flag.NewFlagSet("signal-check", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	common := addCommonFlags(fs, false)
	sessionID := fs.String("session-id", "", "completed SPAL Lab session id")
	beforePath := fs.String("before", "", "before RenderProbe JSON path")
	afterPath := fs.String("after", "", "after RenderProbe JSON path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	for _, item := range []struct{ name, value string }{{"session-id", *sessionID}, {"before", *beforePath}, {"after", *afterPath}} {
		if err := require(item.value, item.name); err != nil {
			return err
		}
	}
	sessions, err := orchestration.NewFileStore(common.sessionStore)
	if err != nil {
		return err
	}
	session, ok := sessions.Load(*sessionID)
	if !ok || session.FrozenPlan == nil || len(session.FrozenPlan.ActionSet.Actions) != 1 {
		return errors.New("SPAL Lab session does not contain exactly one frozen action")
	}
	manifest, err := spal.ManifestFromAction(session.FrozenPlan.ActionSet.Actions[0])
	if err != nil {
		return err
	}
	before, err := readRenderProbe(*beforePath)
	if err != nil {
		return err
	}
	after, err := readRenderProbe(*afterPath)
	if err != nil {
		return err
	}
	return printJSON(spal.VerifySignalDirection(manifest.Instruction.ExpectedSignalChange, before, after))
}

func readRenderProbe(path string) (spal.RenderProbe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return spal.RenderProbe{}, fmt.Errorf("read render probe %s: %w", path, err)
	}
	var probe spal.RenderProbe
	if err := json.Unmarshal(data, &probe); err != nil {
		return spal.RenderProbe{}, fmt.Errorf("decode render probe %s: %w", path, err)
	}
	return probe, nil
}

func validateSignalFlags(low, high float64, direction string) error {
	direction = strings.TrimSpace(direction)
	provided := low != 0 || high != 0 || direction != ""
	if !provided {
		return nil
	}
	if low <= 0 || high <= low || direction == "" {
		return errors.New("expected-band-low-hz, expected-band-high-hz and expected-direction must be supplied together")
	}
	if direction != "increase" && direction != "decrease" {
		return errors.New("expected-direction must be increase or decrease")
	}
	return nil
}

func signalProbeScope(clipID, startText, endText, tailText string) (spal.SignalProbeScope, error) {
	start, err := optionalSignalFloat(startText, "signal-probe-start-seconds")
	if err != nil {
		return spal.SignalProbeScope{}, err
	}
	end, err := optionalSignalFloat(endText, "signal-probe-end-seconds")
	if err != nil {
		return spal.SignalProbeScope{}, err
	}
	tail, err := optionalSignalFloat(tailText, "signal-probe-tail-seconds")
	if err != nil {
		return spal.SignalProbeScope{}, err
	}
	clipID = strings.TrimSpace(clipID)
	if clipID == "" && start == nil && end == nil && tail == nil {
		return spal.SignalProbeScope{}, nil
	}
	scope := spal.SignalProbeScope{
		TapPoint: "track_post_fader", RenderMode: "offline_probe", ClipID: clipID,
		StartSeconds: start, EndSeconds: end, TailSeconds: tail,
	}
	if err := scope.Validate(); err != nil {
		return spal.SignalProbeScope{}, err
	}
	return scope, nil
}

func optionalSignalFloat(value, name string) (*float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("-%s must be a finite number", name)
	}
	if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return nil, fmt.Errorf("-%s must be a finite number", name)
	}
	return &parsed, nil
}

func require(value, name string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("-%s is required", name)
	}
	return nil
}

func requireOK(reply map[string]any) error {
	if reply == nil {
		return errors.New("empty kernel reply")
	}
	if strings.EqualFold(strings.TrimSpace(fmt.Sprint(reply["status"])), "error") {
		return errors.New(strings.TrimSpace(fmt.Sprint(reply["message"])))
	}
	return nil
}

func liveRecordValidator(client *kernel.Client) func(context.Context, spallab.ProviderRecord) error {
	return func(ctx context.Context, record spallab.ProviderRecord) error {
		if client == nil {
			return errors.New("kernel client is nil")
		}
		result, err := client.SendVSPLegacyCommand(ctx, map[string]any{
			"cmd": "get_plugin_parameters", "track_id": record.Instance.TrackID, "plugin_id": record.Instance.PluginID,
		})
		if err != nil {
			return err
		}
		reply := result.LegacyLikeReply()
		if err := requireOK(reply); err != nil {
			return err
		}
		return spallab.ValidateProviderRecordCurrent(record, reply)
	}
}

func splitRefs(value string) []string {
	parts := strings.Split(value, ",")
	refs := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			refs = append(refs, part)
		}
	}
	return refs
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func printJSON(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(data))
	return err
}

func printUsage(out *os.File) {
	fmt.Fprint(out, `SPAL Lab v0 (developer-only, no automatic plug-in loading)

Usage:
  spallab conform -track-id <id> -plugin-id <id> -target-ref track:<name> \
    -band-slot band1 -static-bell-confirmed -evidence <ref>
  spallab plan -session-id <id> -target-ref track:<name> -frequency-hz 92 \
    -gain-db -2.5 -q 1.2 [-expected-band-low-hz 80 -expected-band-high-hz 110 -expected-direction decrease \
    -signal-probe-clip-id <clip-id> | -signal-probe-start-seconds 0 -signal-probe-end-seconds 12]
  spallab approve -session-id <id> -proposal-id <exact proposal id>
  spallab execute -session-id <id> [-kernel-sub tcp://127.0.0.1:5556]
  spallab rollback-plan -session-id <completed id> -rollback-session-id <new id>
  spallab approve -session-id <new rollback id> -proposal-id <exact proposal id>
  spallab execute -session-id <new rollback id>
  spallab status -session-id <id>
  spallab signal-check -session-id <id> -before before.json -after after.json

`)
}

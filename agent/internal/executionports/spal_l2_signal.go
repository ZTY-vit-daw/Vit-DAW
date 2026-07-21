package executionports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/go-zeromq/zmq4"

	"vit-daw-agent/internal/executionruntime"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/orchestration"
	"vit-daw-agent/internal/spal"
)

const (
	defaultSPALL2SubscriptionURL = "tcp://127.0.0.1:5556"
	defaultSPALL2ProbeTimeout    = 30 * time.Second
	defaultSPALL2SubscriberWarm  = 200 * time.Millisecond
)

// SPALSignalCapturer obtains one same-tap render observation. Its boundary is
// deliberately smaller than a mutation port: an unavailable observation must
// produce durable inconclusive evidence, never prevent or retry a frozen
// parameter action that remains structurally executable.
type SPALSignalCapturer interface {
	CaptureSPALSignal(context.Context, spal.ExecutionManifest, string, string) (spal.RenderProbe, error)
}

// SPALSignalCapturePort decorates the real mutation port. It captures a
// frozen verification scope immediately before and after an applied action,
// then embeds both probes in the Action Receipt. The Coordinator persists that
// Receipt before the verifier reads it, so a later recovery cannot mistake an
// in-memory result for durable evidence.
type SPALSignalCapturePort struct {
	Mutation executionruntime.MutationPort
	Signal   SPALSignalCapturer
}

func (p *SPALSignalCapturePort) Preflight(ctx context.Context, actionSet orchestration.ActionSet, cut orchestration.ProjectCut) error {
	if p == nil || p.Mutation == nil {
		return fmt.Errorf("SPAL signal capture mutation port is required")
	}
	return p.Mutation.Preflight(ctx, actionSet, cut)
}

func (p *SPALSignalCapturePort) Apply(ctx context.Context, action orchestration.Action, idempotencyKey string) (orchestration.ActionReceipt, error) {
	if p == nil || p.Mutation == nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "failed"}, fmt.Errorf("SPAL signal capture mutation port is required")
	}
	manifest, manifestErr := spal.ManifestFromAction(action)
	if manifestErr != nil || !manifest.Instruction.ExpectedSignalChange.IsRequested() || !manifest.Instruction.SignalProbeScope.IsConfigured() {
		return p.Mutation.Apply(ctx, action, idempotencyKey)
	}

	evidence := spal.NewSignalProbeEvidence(manifest.Instruction.SignalProbeScope)
	if p.Signal == nil {
		evidence.AddCaptureError(errors.New("no L2 same-tap signal probe is attached"))
	} else {
		before, err := p.Signal.CaptureSPALSignal(ctx, manifest, "before", idempotencyKey+":signal:before")
		if err != nil {
			evidence.AddCaptureError(fmt.Errorf("before: %w", err))
		} else {
			evidence.Before = &before
		}
	}
	if err := ctx.Err(); err != nil {
		return receiptWithSignalEvidence(orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown_after_cancel"}, evidence), err
	}

	receipt, applyErr := p.Mutation.Apply(ctx, action, idempotencyKey)
	if applyErr == nil && strings.EqualFold(strings.TrimSpace(receipt.Status), "applied") && p.Signal != nil {
		after, err := p.Signal.CaptureSPALSignal(ctx, manifest, "after", idempotencyKey+":signal:after")
		if err != nil {
			evidence.AddCaptureError(fmt.Errorf("after: %w", err))
		} else {
			evidence.After = &after
		}
	}
	return receiptWithSignalEvidence(receipt, evidence), applyErr
}

func (p *SPALSignalCapturePort) Reconcile(ctx context.Context, action orchestration.Action, idempotencyKey string, cut orchestration.ProjectCut) (orchestration.ActionReceipt, error) {
	if p == nil || p.Mutation == nil {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("SPAL signal capture mutation port is required")
	}
	port, ok := p.Mutation.(executionruntime.ReconcilePort)
	if !ok {
		return orchestration.ActionReceipt{ActionID: action.ID, Status: "unknown"}, fmt.Errorf("wrapped SPAL mutation port does not support reconciliation")
	}
	// Reconciliation must not silently create a new before/after pair after a
	// process crash. It restores only structural state; absent durable signal
	// evidence remains explicitly inconclusive.
	return port.Reconcile(ctx, action, idempotencyKey, cut)
}

func receiptWithSignalEvidence(receipt orchestration.ActionReceipt, evidence spal.SignalProbeEvidence) orchestration.ActionReceipt {
	evidence.RefreshEvidenceRefs()
	if receipt.Details == nil {
		receipt.Details = map[string]any{}
	}
	receipt.Details["spal_signal_evidence"] = evidence
	receipt.EvidenceRefs = mergeSPALSignalRefs(receipt.EvidenceRefs, evidence.EvidenceRefs...)
	return receipt
}

func mergeSPALSignalRefs(base []string, refs ...string) []string {
	seen := make(map[string]bool, len(base)+len(refs))
	out := make([]string, 0, len(base)+len(refs))
	for _, value := range append(append([]string(nil), base...), refs...) {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

// SPALL2ProbeClient is the narrow VSP read-only command surface needed to
// request a kernel-side L2 render probe. It intentionally has no plug-in
// parameter write method.
type SPALL2ProbeClient interface {
	SendVSPLegacyCommandWithIDs(context.Context, map[string]any, string, string) (*kernel.VSPCommandResult, error)
}

// SPALL2RenderProbe captures L2 render events directly from the kernel PUB
// stream. It is a transport implementation of the semantic verification
// boundary, not an adapter and not a capability-layer tool call.
type SPALL2RenderProbe struct {
	Client          SPALL2ProbeClient
	SubscriptionURL string
	Timeout         time.Duration
	SubscriberWarm  time.Duration
}

func (p SPALL2RenderProbe) CaptureSPALSignal(ctx context.Context, manifest spal.ExecutionManifest, phase, requestID string) (spal.RenderProbe, error) {
	if p.Client == nil {
		return spal.RenderProbe{}, fmt.Errorf("L2 render probe client is required")
	}
	if err := manifest.Validate(); err != nil {
		return spal.RenderProbe{}, err
	}
	if !manifest.Instruction.ExpectedSignalChange.IsRequested() {
		return spal.RenderProbe{}, fmt.Errorf("SPAL action does not request signal-direction verification")
	}
	scope := manifest.Instruction.SignalProbeScope
	if err := scope.Validate(); err != nil {
		return spal.RenderProbe{}, err
	}
	if !scope.IsConfigured() {
		return spal.RenderProbe{}, fmt.Errorf("no frozen signal probe scope is available")
	}
	phase = strings.ToLower(strings.TrimSpace(phase))
	if phase != "before" && phase != "after" {
		return spal.RenderProbe{}, fmt.Errorf("signal probe phase must be before or after")
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return spal.RenderProbe{}, fmt.Errorf("stable signal probe request id is required")
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = defaultSPALL2ProbeTimeout
	}
	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	subscriptionURL := strings.TrimSpace(p.SubscriptionURL)
	if subscriptionURL == "" {
		subscriptionURL = defaultSPALL2SubscriptionURL
	}
	sub := zmq4.NewSub(opCtx, zmq4.WithTimeout(120*time.Millisecond), zmq4.WithAutomaticReconnect(true))
	defer sub.Close()
	if err := sub.SetOption(zmq4.OptionSubscribe, ""); err != nil {
		return spal.RenderProbe{}, fmt.Errorf("subscribe L2 render probe stream: %w", err)
	}
	if err := sub.Dial(subscriptionURL); err != nil {
		return spal.RenderProbe{}, fmt.Errorf("dial L2 render probe stream %s: %w", subscriptionURL, err)
	}
	if warm := p.SubscriberWarm; warm <= 0 {
		warmSPALL2Subscriber(opCtx, defaultSPALL2SubscriberWarm)
	} else {
		warmSPALL2Subscriber(opCtx, warm)
	}

	result, err := p.Client.SendVSPLegacyCommandWithIDs(opCtx, l2ProbeCommand(manifest, requestID), requestID, "")
	if err != nil {
		return spal.RenderProbe{}, fmt.Errorf("start %s L2 render probe: %w", phase, err)
	}
	if err := l2ProbeReplyOK(result); err != nil {
		return spal.RenderProbe{}, fmt.Errorf("start %s L2 render probe: %w", phase, err)
	}
	return collectSPALL2ProbeEvent(opCtx, sub, manifest, requestID)
}

func warmSPALL2Subscriber(ctx context.Context, duration time.Duration) {
	if duration <= 0 {
		return
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func l2ProbeCommand(manifest spal.ExecutionManifest, requestID string) map[string]any {
	scope := manifest.Instruction.SignalProbeScope
	command := map[string]any{
		"cmd":                   "l2_render_probe",
		"request_id":            requestID,
		"track_id":              manifest.Binding.Instance.TrackID,
		"tap_point":             strings.ToLower(strings.TrimSpace(scope.TapPoint)),
		"render_mode":           strings.ToLower(strings.TrimSpace(scope.RenderMode)),
		"analysis_band_low_hz":  manifest.Instruction.ExpectedSignalChange.BandLowHz,
		"analysis_band_high_hz": manifest.Instruction.ExpectedSignalChange.BandHighHz,
		"analysis_band_id":      "spal_target",
	}
	if clipID := strings.TrimSpace(scope.ClipID); clipID != "" {
		command["clip_id"] = clipID
	}
	if scope.StartSeconds != nil {
		command["start_seconds"] = *scope.StartSeconds
	}
	if scope.EndSeconds != nil {
		command["end_seconds"] = *scope.EndSeconds
	}
	if scope.TailSeconds != nil {
		command["tail_seconds"] = *scope.TailSeconds
	}
	return command
}

func l2ProbeReplyOK(result *kernel.VSPCommandResult) error {
	if result == nil {
		return errors.New("empty kernel reply")
	}
	reply := result.LegacyLikeReply()
	if strings.EqualFold(l2String(reply["status"]), "error") {
		return errors.New(firstL2String(reply, "message", "error", "status"))
	}
	return nil
}

func collectSPALL2ProbeEvent(ctx context.Context, sub zmq4.Socket, manifest spal.ExecutionManifest, requestID string) (spal.RenderProbe, error) {
	for {
		select {
		case <-ctx.Done():
			return spal.RenderProbe{}, fmt.Errorf("wait for L2 render probe event: %w", ctx.Err())
		default:
		}
		message, err := sub.Recv()
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "timeout") {
				continue
			}
			if errors.Is(err, context.Canceled) {
				return spal.RenderProbe{}, fmt.Errorf("wait for L2 render probe event: %w", err)
			}
			return spal.RenderProbe{}, fmt.Errorf("receive L2 render probe event: %w", err)
		}
		event := map[string]any{}
		if err := json.Unmarshal([]byte(spalZMQPayload(message)), &event); err != nil {
			continue
		}
		if !strings.EqualFold(l2String(event["command"]), "l2_render_probe_ready") || !strings.EqualFold(l2String(event["feature_type"]), "l2_render_probe") {
			continue
		}
		if l2String(event["request_id"]) != requestID || l2String(event["track_id"]) != manifest.Binding.Instance.TrackID {
			continue
		}
		return l2EventToRenderProbe(event, manifest)
	}
}

func spalZMQPayload(message zmq4.Msg) string {
	if len(message.Frames) == 0 {
		return ""
	}
	return string(message.Frames[len(message.Frames)-1])
}

func l2EventToRenderProbe(event map[string]any, manifest spal.ExecutionManifest) (spal.RenderProbe, error) {
	if !strings.EqualFold(l2String(event["status"]), "ready") {
		return spal.RenderProbe{}, fmt.Errorf("L2 render probe status %q: %s", l2String(event["status"]), firstL2String(event, "reason", "message", "error"))
	}
	scope := manifest.Instruction.SignalProbeScope
	tapPoint := strings.ToLower(strings.TrimSpace(l2String(event["tap_point"])))
	renderMode := strings.ToLower(strings.TrimSpace(l2String(event["render_mode"])))
	if tapPoint != strings.ToLower(strings.TrimSpace(scope.TapPoint)) || renderMode != strings.ToLower(strings.TrimSpace(scope.RenderMode)) {
		return spal.RenderProbe{}, fmt.Errorf("L2 render probe scope does not match frozen tap/render mode")
	}
	if l2String(event["track_id"]) != manifest.Binding.Instance.TrackID {
		return spal.RenderProbe{}, fmt.Errorf("L2 render probe track does not match frozen SPAL binding")
	}
	if clipID := strings.TrimSpace(scope.ClipID); clipID != "" && l2String(event["clip_id"]) != clipID {
		return spal.RenderProbe{}, fmt.Errorf("L2 render probe clip does not match frozen signal scope")
	}
	if err := validateL2QualityEvidence(l2Map(event["quality_evidence"])); err != nil {
		return spal.RenderProbe{}, err
	}
	probe := spal.RenderProbe{
		TapPoint:       tapPoint,
		RenderMode:     renderMode,
		RenderRevision: l2String(event["render_revision"]),
		TrackID:        l2String(event["track_id"]),
		ClipID:         l2String(event["clip_id"]),
		SourceRevision: l2String(event["source_revision"]),
		ClipRevision:   l2String(event["clip_revision"]),
		EvidenceRef:    l2String(event["evidence_ref"]),
	}
	if probe.RenderRevision == "" || probe.EvidenceRef == "" {
		return spal.RenderProbe{}, fmt.Errorf("L2 render probe omitted render revision or evidence reference")
	}
	bands, err := l2Bands(event["bands"])
	if err != nil {
		return spal.RenderProbe{}, err
	}
	probe.Bands = bands
	return probe, nil
}

func validateL2QualityEvidence(evidence map[string]any) error {
	if len(evidence) == 0 {
		return fmt.Errorf("L2 render probe omitted quality evidence")
	}
	nonzero, nonzeroOK := l2Bool(evidence["nonzero"])
	sumAbs, sumOK := spalNumber(evidence["sum_abs"])
	maxAbs, maxOK := spalNumber(evidence["max_abs"])
	nanInf, nanOK := spalNumber(evidence["nan_inf_count"])
	if !nonzeroOK || !nonzero || !sumOK || !maxOK || !nanOK || sumAbs <= 0 || maxAbs <= 0 || nanInf != 0 {
		return fmt.Errorf("L2 render probe quality evidence is not usable")
	}
	return nil
}

func l2Bands(value any) ([]spal.BandEnergy, error) {
	rows := l2Map(value)
	if len(rows) == 0 {
		return nil, fmt.Errorf("L2 render probe omitted band energy rows")
	}
	bands := make([]spal.BandEnergy, 0, len(rows))
	for _, row := range rows {
		fields := l2Map(row)
		low, lowOK := spalNumber(fields["min_hz"])
		high, highOK := spalNumber(fields["max_hz"])
		energy, energyOK := spalNumber(fields["energy_db"])
		if !lowOK || !highOK || !energyOK || !l2Finite(low) || !l2Finite(high) || !l2Finite(energy) || low < 0 || high <= low {
			continue
		}
		bands = append(bands, spal.BandEnergy{MinHz: low, MaxHz: high, EnergyDB: energy})
	}
	if len(bands) == 0 {
		return nil, fmt.Errorf("L2 render probe contains no usable band energy rows")
	}
	sort.Slice(bands, func(i, j int) bool {
		if bands[i].MinHz == bands[j].MinHz {
			return bands[i].MaxHz < bands[j].MaxHz
		}
		return bands[i].MinHz < bands[j].MinHz
	})
	return bands, nil
}

func l2Map(value any) map[string]any {
	if row, ok := value.(map[string]any); ok {
		return row
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var row map[string]any
	if json.Unmarshal(data, &row) != nil {
		return nil
	}
	return row
}

func l2String(value any) string {
	if value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func firstL2String(row map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := l2String(row[key]); value != "" {
			return value
		}
	}
	return ""
}

func l2Bool(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		text := strings.ToLower(strings.TrimSpace(typed))
		switch text {
		case "true", "1", "yes":
			return true, true
		case "false", "0", "no":
			return false, true
		}
	}
	return false, false
}

func l2Finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

var _ executionruntime.MutationPort = (*SPALSignalCapturePort)(nil)
var _ executionruntime.ReconcilePort = (*SPALSignalCapturePort)(nil)
var _ SPALSignalCapturer = SPALL2RenderProbe{}

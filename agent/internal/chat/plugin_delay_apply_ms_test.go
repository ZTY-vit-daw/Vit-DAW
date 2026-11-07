package chat

import (
	"context"
	"fmt"
	"math"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/shadow"
)

// DELAY-APPLY-MS-FIX-1 paired tests. The negative anchor is the observed
// defect (300 S0 run4/run5 T4 write_faces): apply_delay_controls with
// value_ms=250 inverted through the global power fit onto normalized
// 0.35065, the kernel displayed "352 ms", and the apply still reported
// apply_status=exact.

// hDelayDelaySecSamples replays the measured H-Delay "Delay Sec" display
// probe samples (normalized -> ms) the S0 forensics captured.
var hDelayDelaySecSamples = [][2]float64{{0, 1}, {0.25, 251}, {0.5, 501}, {0.75, 1540}, {1, 3500}}

// fakeDelayApplyKernel models a delay-provable surface whose "Delay Sec"
// display follows one of two truth models over the same measured samples:
//   - polyline: linear between the probe samples (the samples are exact truth;
//     the measured polyline inversion is then exactly right);
//   - geometric: exponential within each sample segment — a vendor curve that
//     deviates from any polyline between the probe samples, so a landing that
//     is off target must be reported honestly instead of "exact".
type fakeDelayApplyKernel struct {
	params    map[string]float64
	geometric bool
	writes    [][]map[string]any
}

func newFakeDelayApplyKernel(geometric bool) *fakeDelayApplyKernel {
	return &fakeDelayApplyKernel{params: map[string]float64{"delay": 0.374, "feedback": 0.3}, geometric: geometric}
}

// displayMS evaluates the fake vendor truth curve at a normalized position.
func (fake *fakeDelayApplyKernel) displayMS(normalized float64) float64 {
	samples := hDelayDelaySecSamples
	for i := 1; i < len(samples); i++ {
		prev, next := samples[i-1], samples[i]
		if normalized <= next[0] || i == len(samples)-1 {
			fraction := (normalized - prev[0]) / (next[0] - prev[0])
			fraction = math.Max(0, math.Min(1, fraction))
			if fake.geometric && prev[1] > 0 {
				return prev[1] * math.Pow(next[1]/prev[1], fraction)
			}
			return prev[1] + fraction*(next[1]-prev[1])
		}
	}
	return samples[len(samples)-1][1]
}

func (fake *fakeDelayApplyKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	if firstNonEmptyText(command, "cmd") != "get_plugin_parameters" {
		return nil, "", fmt.Errorf("unexpected command %v", command)
	}
	samples := make([]map[string]any, 0, len(hDelayDelaySecSamples))
	for _, point := range hDelayDelaySecSamples {
		samples = append(samples, map[string]any{"normalized_value": point[0],
			"text": fmt.Sprintf("%.0f ms", point[1])})
	}
	rows := []map[string]any{
		{"id": "delay", "name": "Delay Sec", "host_controllable": true,
			"normalized_value": fake.params["delay"],
			"value_text":       fmt.Sprintf("%.0f ms", fake.displayMS(fake.params["delay"])),
			"display_probe":    map[string]any{"mode": "read_only_value_to_string", "samples": samples}},
		{"id": "feedback", "name": "Feedback", "host_controllable": true,
			"normalized_value": fake.params["feedback"],
			"value_text":       fmt.Sprintf("%.0f ", math.Round(fake.params["feedback"] * 100)),
			"display_probe": map[string]any{"mode": "read_only_value_to_string", "samples": []map[string]any{
				{"normalized_value": 0.0, "text": "0 "}, {"normalized_value": 0.25, "text": "25 "},
				{"normalized_value": 0.5, "text": "50 "}, {"normalized_value": 0.75, "text": "75 "},
				{"normalized_value": 1.0, "text": "99 "}}}},
	}
	reply := map[string]any{"status": "ok", "track_id": "track-1", "plugin_id": "delay-1", "parameters": rows}
	return reply, "", nil
}

func (fake *fakeDelayApplyKernel) SendVSPCommand(_ context.Context, command string, args map[string]any) (*kernel.VSPCommandResult, error) {
	if command != "plugin.set_params_batch" {
		return nil, fmt.Errorf("unexpected VSP command %s", command)
	}
	rows := mapRowsValue(args["parameters"])
	for _, row := range rows {
		id := firstNonEmptyText(row, "parameter_id")
		value, ok := firstNumericAny(row, "normalized_value")
		if id == "" || !ok {
			return nil, fmt.Errorf("invalid fake delay write %v", row)
		}
		fake.params[id] = value
	}
	fake.writes = append(fake.writes, rows)
	return &kernel.VSPCommandResult{Payload: map[string]any{"status": "ok"}}, nil
}

func (fake *fakeDelayApplyKernel) delayTestControlRef(t *testing.T, server *Server) string {
	t.Helper()
	_, summary, err := server.readLiveDelayControlSurface(context.Background(), "track-1", "delay-1")
	if err != nil {
		t.Fatalf("readLiveDelayControlSurface: %v", err)
	}
	for _, row := range mapRowsValue(mapValue(summary["delay_stage"])["timing"]) {
		if firstNonEmptyText(row, "role") == "time" {
			return firstNonEmptyText(row, "control_ref")
		}
	}
	t.Fatal("no delay timing control_ref in summary")
	return ""
}

// Negative anchor: the global power fit behind the parametric inversion is
// the pre-fix defect shape — inverting 250 ms through it yields normalized
// ~0.35065, which the kernel displays as 352 ms. The fix must detect the
// untrustworthy fit and invert on the measured polyline instead.
func TestCompressorPhysicalTargetHDelayCurveInversion(t *testing.T) {
	curve := hDelayDelaySecSamples
	if tracks := compressorCurveFitTracksSamples(curve); tracks {
		t.Fatal("H-Delay curve fit was trusted despite ~11% span residual")
	}
	// Pre-fix parametric shape (the 352 ms landing).
	parametric, ok := eqNormalizedFromCurve(250, curve)
	if !ok || parametric < 0.3500 || parametric > 0.3515 {
		t.Fatalf("parametric inversion=%v ok=%v, want ~0.35065 (the observed 352 ms defect shape)", parametric, ok)
	}
	// Fixed path: binding-level inversion lands on the measured polyline.
	binding := map[string]any{"role": "time", "curve": curve}
	normalized, _, _, _, err := compressorPhysicalTarget(binding, 250, "ms")
	if err != nil {
		t.Fatalf("compressorPhysicalTarget: %v", err)
	}
	if normalized < 0.248 || normalized > 0.250 {
		t.Fatalf("normalized=%v, want the polyline inversion ~0.249 (249-251 ms)", normalized)
	}
	segment, ok := compressorCurveSegmentInverse(curve, 250)
	if !ok || segment < 0.248 || segment > 0.250 {
		t.Fatalf("segment inverse=%v ok=%v", segment, ok)
	}
}

// 250 ms through apply_delay_controls lands at 249-251 ms and reports
// status=exact only because the display readback proves it.
func TestApplyDelayControlsMsTargetLandsWithinTolerance(t *testing.T) {
	fake := newFakeDelayApplyKernel(false /* polyline truth */)
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	ref := fake.delayTestControlRef(t, server)
	result, err := server.applyPluginGrabberDelayControls(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "delay-1", "atomic": true,
		"controls": []map[string]any{{"control_ref": ref, "value_ms": 250.0}}}, nil)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := firstNonEmptyText(result, "status"); got != "exact" {
		t.Fatalf("overall status=%q controls=%v", got, result["controls"])
	}
	controls := mapRowsValue(result["controls"])
	if len(controls) != 1 || firstNonEmptyText(controls[0], "status") != "exact" {
		t.Fatalf("controls=%v", result["controls"])
	}
	normalized := fake.params["delay"]
	if normalized < 0.248 || normalized > 0.250 {
		t.Fatalf("landed normalized=%v, want ~0.249", normalized)
	}
	displayed := fake.displayMS(normalized)
	if displayed < 248.5 || displayed > 251.5 {
		t.Fatalf("displayed %v ms, want 249-251", displayed)
	}
}

// Honesty: when the vendor curve deviates from the measured polyline between
// samples, a landing that misses the requested physical target must downgrade
// to status=approximate instead of claiming exact.
func TestApplyDelayControlsHonestStatusOnPhysicalMiss(t *testing.T) {
	fake := newFakeDelayApplyKernel(true /* geometric truth */)
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	ref := fake.delayTestControlRef(t, server)
	result, err := server.applyPluginGrabberDelayControls(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "delay-1", "atomic": true,
		"controls": []map[string]any{{"control_ref": ref, "value_ms": 250.0}}}, nil)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	controls := mapRowsValue(result["controls"])
	if len(controls) != 1 {
		t.Fatalf("controls=%v", result["controls"])
	}
	if got := firstNonEmptyText(controls[0], "status"); got != "approximate" {
		t.Fatalf("per-control status=%q, want approximate for a physical miss", got)
	}
	if got := firstNonEmptyText(result, "status"); got != "approximate" {
		t.Fatalf("overall status=%q, want approximate", got)
	}
	// The transaction still holds: the normalized landing is exactly the
	// planned write and the miss is only the between-samples curve deviation.
	normalized := fake.params["delay"]
	if normalized < 0.248 || normalized > 0.250 {
		t.Fatalf("landed normalized=%v, want the same polyline plan ~0.249", normalized)
	}
}

// Status precedence: quantized discrete targets keep their existing
// semantics; writes with no physical claim stay exact.
func TestDelayWriteLandingStatusPrecedence(t *testing.T) {
	if got := delayWriteLandingStatus(eqWriteStep{Quantized: true, RequestedPhysical: func() *float64 { v := 250.0; return &v }()},
		[]map[string]any{{"param_id": "delay", "value_text": "352 ms"}}); got != "quantized" {
		t.Fatalf("quantized precedence broken: %q", got)
	}
	if got := delayWriteLandingStatus(eqWriteStep{}, nil); got != "exact" {
		t.Fatalf("no-physical-claim write must stay exact: %q", got)
	}
	if got := delayWriteLandingStatus(eqWriteStep{ParamID: "delay", Role: "time", RequestedPhysical: func() *float64 { v := 250.0; return &v }()},
		[]map[string]any{{"param_id": "delay", "value_text": "not parsable"}}); got != "approximate" {
		t.Fatalf("unparsable readback must not claim exact: %q", got)
	}
	if got := delayWriteLandingStatus(eqWriteStep{ParamID: "delay", Role: "time", RequestedPhysical: func() *float64 { v := 250.0; return &v }()},
		[]map[string]any{{"param_id": "delay", "value_text": "352 ms"}}); got != "approximate" {
		t.Fatalf("352 ms for a 250 ms request must not claim exact: %q", got)
	}
	if got := delayWriteLandingStatus(eqWriteStep{ParamID: "delay", Role: "time", RequestedPhysical: func() *float64 { v := 250.0; return &v }()},
		[]map[string]any{{"param_id": "delay", "value_text": "251 ms"}}); got != "exact" {
		t.Fatalf("251 ms for a 250 ms request is within tolerance: %q", got)
	}
}

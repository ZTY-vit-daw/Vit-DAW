package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/shadow"
)

type fakeTransientShaperKernel struct {
	params      map[string]float64
	mode        float64
	batchCalls  [][]map[string]any
	mutateOther bool
	failBatch   bool
}

func newFakeTransientShaperKernel() *fakeTransientShaperKernel {
	return &fakeTransientShaperKernel{params: map[string]float64{"attack": .5, "sustain": .5, "duration": .5, "release": .5, "guard": 0, "mode": 0}}
}

func (fake *fakeTransientShaperKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	if firstNonEmptyText(command, "cmd") != "get_plugin_parameters" {
		return nil, "", fmt.Errorf("unexpected command %v", command)
	}
	rows := []map[string]any{
		fake.transientParameter("attack", "Attack", []string{"-100", "-50", "0", "50", "100"}),
		fake.transientParameter("sustain", "Sustain", []string{"-100", "-50", "0", "50", "100"}),
		fake.transientParameter("duration", "Duration", []string{"1 ms", "10 ms", "50 ms", "100 ms", "500 ms"}),
		fake.transientParameter("release", "Release", []string{"1 ms", "10 ms", "50 ms", "100 ms", "500 ms"}),
		fake.transientParameter("guard", "Guard", []string{"Off", "Clip", "Limit"}),
		fake.transientModeParameter(),
	}
	reply := map[string]any{"status": "ok", "track_id": "track-1", "plugin_id": "transient-1", "parameters": rows}
	encoded, _ := json.Marshal(reply)
	return reply, string(encoded), nil
}

func (fake *fakeTransientShaperKernel) SendVSPCommand(_ context.Context, command string, args map[string]any) (*kernel.VSPCommandResult, error) {
	if command != "plugin.set_params_batch" {
		return nil, fmt.Errorf("unexpected VSP command %s", command)
	}
	rows := mapRowsValue(args["parameters"])
	copied := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		id := firstNonEmptyText(row, "parameter_id")
		value, ok := firstNumericAny(row, "normalized_value")
		if id == "" || !ok {
			return nil, errors.New("invalid fake transient write")
		}
		if id == "mode" {
			fake.mode = value
		} else {
			fake.params[id] = value
		}
		copied = append(copied, map[string]any{"parameter_id": id, "normalized_value": value})
	}
	fake.batchCalls = append(fake.batchCalls, copied)
	if fake.mutateOther && len(fake.batchCalls) == 1 {
		fake.params["guard"] = .8
	}
	if fake.failBatch && len(fake.batchCalls) == 1 {
		return &kernel.VSPCommandResult{Payload: map[string]any{"status": "error", "message": "injected transient failure"}}, nil
	}
	return &kernel.VSPCommandResult{Payload: map[string]any{"status": "ok"}}, nil
}

func (fake *fakeTransientShaperKernel) transientParameter(id, name string, labels []string) map[string]any {
	normalized := fake.params[id]
	index := int(math.Round(normalized * float64(len(labels)-1)))
	if index < 0 {
		index = 0
	}
	if index >= len(labels) {
		index = len(labels) - 1
	}
	samples := make([]map[string]any, 0, len(labels))
	for i, label := range labels {
		samples = append(samples, map[string]any{"normalized_value": float64(i) / float64(len(labels)-1), "text": label})
	}
	return map[string]any{"id": id, "name": name, "host_controllable": true, "normalized_value": normalized, "value_text": labels[index], "display_probe": map[string]any{"mode": "samples", "label": name, "samples": samples}}
}

func (fake *fakeTransientShaperKernel) transientModeParameter() map[string]any {
	labels := []string{"Full Range", "Dual Band", "Shelf EQ"}
	normalized := fake.mode
	index := int(math.Round(normalized * 2))
	if index > 2 {
		index = 2
	}
	discrete := make([]map[string]any, 0, len(labels))
	samples := make([]map[string]any, 0, len(labels))
	for i, label := range labels {
		value := float64(i) / 2
		discrete = append(discrete, map[string]any{"index": i, "value": value, "label": label})
		samples = append(samples, map[string]any{"normalized_value": value, "text": label})
	}
	return map[string]any{"id": "mode", "name": "Mode", "host_controllable": true, "normalized_value": normalized, "value_text": labels[index], "display_probe": map[string]any{"mode": "samples", "label": "Mode", "samples": samples, "discrete_labels": discrete}}
}

func transientTestControlRef(summary map[string]any, role string) string {
	stage := mapValue(summary["transient_shaper_stage"])
	for _, section := range []string{"envelope_action", "detector", "timing", "shape", "mode", "output"} {
		for _, row := range mapRowsValue(stage[section]) {
			if firstNonEmptyText(row, "role") == role {
				return firstNonEmptyText(row, "control_ref")
			}
		}
	}
	return ""
}

func TestApplyTransientShaperControlsRestoresFullPreimageOnFailure(t *testing.T) {
	for _, tc := range []struct {
		name             string
		sideEffect, fail bool
	}{{"side effect", true, false}, {"batch failure", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeTransientShaperKernel()
			fake.mutateOther, fake.failBatch = tc.sideEffect, tc.fail
			server := New(nil, shadow.New(nil), nil)
			server.eqKernelOverride = fake
			_, summary, err := server.readLiveTransientShaperControlSurface(context.Background(), "track-1", "transient-1")
			if err != nil {
				t.Fatal(err)
			}
			ref := transientTestControlRef(summary, "attack_amount")
			if ref == "" {
				t.Fatalf("missing attack ref: %+v", summary)
			}
			before := map[string]float64{}
			for key, value := range fake.params {
				before[key] = value
			}
			result, err := server.applyPluginGrabberTransientShaperControls(context.Background(), map[string]any{"track_id": "track-1", "plugin_id": "transient-1", "atomic": true, "controls": []map[string]any{{"control_ref": ref, "percent": 50.0}}}, nil)
			if err == nil {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if tc.sideEffect && !strings.Contains(err.Error(), "unplanned_parameter_change") {
				t.Fatalf("err=%v", err)
			}
			for key, value := range before {
				if fake.params[key] != value {
					t.Fatalf("preimage drift %s: before=%v after=%v", key, value, fake.params)
				}
			}
			if len(fake.batchCalls) != 2 {
				t.Fatalf("batch calls=%d", len(fake.batchCalls))
			}
		})
	}
}

func TestTransientShaperControlRefsExcludeAuxiliaryLimiter(t *testing.T) {
	fake := newFakeTransientShaperKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	_, summary, err := server.readLiveTransientShaperControlSurface(context.Background(), "track-1", "transient-1")
	if err != nil {
		t.Fatal(err)
	}
	if transientTestControlRef(summary, "limiter") != "" {
		t.Fatal("auxiliary limiter received transient control ref")
	}
	if transientTestControlRef(summary, "attack_amount") == "" {
		t.Fatal("attack control ref missing")
	}
}

func TestTransientShaperModeChangeRejected(t *testing.T) {
	fake := newFakeTransientShaperKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	_, summary, err := server.readLiveTransientShaperControlSurface(context.Background(), "track-1", "transient-1")
	if err != nil {
		t.Fatal(err)
	}
	ref := transientTestControlRef(summary, "processing_mode")
	if ref == "" {
		t.Fatal("mode control ref missing")
	}
	_, err = server.applyPluginGrabberTransientShaperControls(context.Background(), map[string]any{"track_id": "track-1", "plugin_id": "transient-1", "atomic": true, "controls": []map[string]any{{"control_ref": ref, "enum_label": "Dual Band"}}}, nil)
	if err == nil || transientShaperControlFailureCode(err) != "mode_change_requires_reinspect" {
		t.Fatalf("err=%v", err)
	}
}

func TestTransientShaperUnitBoundaryDistinguishesAmountsFromTiming(t *testing.T) {
	percentBinding := map[string]any{"physical_unit": "%"}
	if err := validateTransientShaperRequestUnit("attack_amount", "%", percentBinding); err != nil {
		t.Fatal(err)
	}
	if err := validateTransientShaperRequestUnit("attack_amount", "ms", percentBinding); transientShaperControlFailureCode(err) != "unit_role_mismatch" {
		t.Fatalf("signed attack amount accepted as timing: %v", err)
	}
	dbBinding := map[string]any{"physical_unit": "dB"}
	if err := validateTransientShaperRequestUnit("detector_sensitivity", "dB", dbBinding); err != nil {
		t.Fatal(err)
	}
}

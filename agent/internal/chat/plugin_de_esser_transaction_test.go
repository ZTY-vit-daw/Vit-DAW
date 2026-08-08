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

type fakeDeEsserKernel struct {
	params      map[string]float64
	batchCalls  [][]map[string]any
	mutateOther bool
	failBatch   bool
}

func newFakeDeEsserKernel() *fakeDeEsserKernel {
	return &fakeDeEsserKernel{params: map[string]float64{"threshold": .5, "range": .5, "frequency": .5, "monitor": 0, "unplanned": .1}}
}

func (fake *fakeDeEsserKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	if firstNonEmptyText(command, "cmd") != "get_plugin_parameters" {
		return nil, "", fmt.Errorf("unexpected command %v", command)
	}
	rows := []map[string]any{
		fake.parameter("threshold", "De-Esser Threshold", []string{"-60 dB", "-45 dB", "-30 dB", "-15 dB", "0 dB"}),
		fake.parameter("range", "Sibilance Range", []string{"0 dB", "6 dB", "12 dB", "18 dB", "24 dB"}),
		fake.parameter("frequency", "Focus Frequency", []string{"2000 Hz", "4500 Hz", "7000 Hz", "9500 Hz", "12000 Hz"}),
		fake.parameter("monitor", "Sibilance Monitor", []string{"Off", "On"}),
		fake.parameter("unplanned", "Meter State", []string{"0", ".25", ".5", ".75", "1"}),
	}
	reply := map[string]any{"status": "ok", "track_id": "track-1", "plugin_id": "deesser-1", "parameters": rows}
	encoded, _ := json.Marshal(reply)
	return reply, string(encoded), nil
}

func (fake *fakeDeEsserKernel) SendVSPCommand(_ context.Context, command string, args map[string]any) (*kernel.VSPCommandResult, error) {
	if command != "plugin.set_params_batch" {
		return nil, fmt.Errorf("unexpected VSP command %s", command)
	}
	rows := mapRowsValue(args["parameters"])
	copied := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		id := firstNonEmptyText(row, "parameter_id")
		value, ok := firstNumericAny(row, "normalized_value")
		if id == "" || !ok {
			return nil, errors.New("invalid fake De-esser write")
		}
		fake.params[id] = value
		copied = append(copied, map[string]any{"parameter_id": id, "normalized_value": value})
	}
	fake.batchCalls = append(fake.batchCalls, copied)
	if fake.mutateOther && len(fake.batchCalls) == 1 {
		fake.params["unplanned"] = .9
	}
	if fake.failBatch && len(fake.batchCalls) == 1 {
		return &kernel.VSPCommandResult{Payload: map[string]any{"status": "error", "message": "injected De-esser failure"}}, nil
	}
	return &kernel.VSPCommandResult{Payload: map[string]any{"status": "ok"}}, nil
}

func (fake *fakeDeEsserKernel) parameter(id, name string, labels []string) map[string]any {
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

func (fake *fakeDeEsserKernel) snapshot() map[string]float64 {
	out := map[string]float64{}
	for key, value := range fake.params {
		out[key] = value
	}
	return out
}

func deEsserTestControlRef(summary map[string]any, role string) string {
	for _, stage := range mapRowsValue(summary["de_esser_stages"]) {
		for _, section := range []string{"operating_point", "frequency_selectivity", "timing", "mode", "output"} {
			for _, binding := range mapRowsValue(stage[section]) {
				if firstNonEmptyText(binding, "role") == role {
					return firstNonEmptyText(binding, "control_ref")
				}
			}
		}
	}
	return ""
}

func TestApplyDeEsserControlsPublishesAndConsumesRestoreRef(t *testing.T) {
	fake := newFakeDeEsserKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	_, summary, err := server.readLiveDeEsserControlSurface(context.Background(), "track-1", "deesser-1")
	if err != nil {
		t.Fatal(err)
	}
	ref := deEsserTestControlRef(summary, "threshold")
	before := fake.snapshot()
	applied, err := server.applyPluginGrabberDeEsserControls(context.Background(), map[string]any{"track_id": "track-1", "plugin_id": "deesser-1", "atomic": true, "controls": []map[string]any{{"control_ref": ref, "value_db": -15.0}}}, nil)
	if err != nil {
		t.Fatalf("apply=%+v err=%v", applied, err)
	}
	restoreRef := firstNonEmptyText(applied, "restore_ref")
	if restoreRef == "" || fake.params["threshold"] == before["threshold"] {
		t.Fatalf("apply=%+v params=%v", applied, fake.params)
	}
	restored, err := server.applyPluginGrabberDeEsserControls(context.Background(), map[string]any{"track_id": "track-1", "plugin_id": "deesser-1", "atomic": true, "restore_ref": restoreRef}, nil)
	if err != nil || !fakeSnapshotsEqual(before, fake.snapshot()) {
		t.Fatalf("restore=%+v err=%v before=%v after=%v", restored, err, before, fake.snapshot())
	}
}

func TestApplyDeEsserControlsRestoresFullPreimageOnFailure(t *testing.T) {
	for _, tc := range []struct {
		name             string
		sideEffect, fail bool
	}{{"side effect", true, false}, {"batch failure", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeDeEsserKernel()
			fake.mutateOther, fake.failBatch = tc.sideEffect, tc.fail
			server := New(nil, shadow.New(nil), nil)
			server.eqKernelOverride = fake
			_, summary, err := server.readLiveDeEsserControlSurface(context.Background(), "track-1", "deesser-1")
			if err != nil {
				t.Fatal(err)
			}
			ref := deEsserTestControlRef(summary, "threshold")
			before := fake.snapshot()
			result, err := server.applyPluginGrabberDeEsserControls(context.Background(), map[string]any{"track_id": "track-1", "plugin_id": "deesser-1", "atomic": true, "controls": []map[string]any{{"control_ref": ref, "value_db": -15.0}}}, nil)
			if err == nil {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if tc.sideEffect && !strings.Contains(err.Error(), "unplanned_parameter_change") {
				t.Fatalf("err=%v", err)
			}
			if !fakeSnapshotsEqual(before, fake.snapshot()) {
				t.Fatalf("full preimage not restored before=%v after=%v", before, fake.snapshot())
			}
			if len(fake.batchCalls) != 2 {
				t.Fatalf("batch calls=%d", len(fake.batchCalls))
			}
		})
	}
}

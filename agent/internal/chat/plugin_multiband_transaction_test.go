package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"

	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/harness"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/shadow"
)

type fakeMultibandKernel struct {
	params       map[string]float64
	batches      [][]map[string]any
	rowsOverride []map[string]any
}

func newFakeMultibandKernel() *fakeMultibandKernel {
	return &fakeMultibandKernel{params: map[string]float64{"x1": .25, "x2": .70, "knee": .50, "t1": .50, "g1": .50, "a1": .50, "r1": .50, "t2": .50, "g2": .50, "a2": .50, "r2": .50}}
}

func (f *fakeMultibandKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	if firstNonEmptyText(command, "cmd") != "get_plugin_parameters" {
		return nil, "", fmt.Errorf("unexpected command %v", command)
	}
	rows := f.rowsOverride
	if rows == nil {
		rows = []map[string]any{
			f.mbParameter("x1", "Crossover 1", "400 Hz", "Hz", 400), f.mbParameter("x2", "Crossover 2", "4000 Hz", "Hz", 4000),
			f.mbParameter("knee", "Knee", "Soft", "enum", 0),
			f.mbParameter("t1", "Band 1 Threshold", "-12 dB", "dB", -12), f.mbParameter("g1", "Band 1 Gain", "0 dB", "dB", 0),
			f.mbParameter("a1", "Band 1 Attack", "10 ms", "ms", 10), f.mbParameter("r1", "Band 1 Release", "100 ms", "ms", 100),
			f.mbParameter("t2", "Band 2 Threshold", "-12 dB", "dB", -12), f.mbParameter("g2", "Band 2 Gain", "0 dB", "dB", 0),
			f.mbParameter("a2", "Band 2 Attack", "10 ms", "ms", 10), f.mbParameter("r2", "Band 2 Release", "100 ms", "ms", 100),
		}
	}
	reply := map[string]any{"status": "ok", "track_id": "track-mb", "plugin_id": "mb-1", "parameters": rows}
	b, _ := json.Marshal(reply)
	return reply, string(b), nil
}

func multibandNegativeRows(f *fakeMultibandKernel, kind string) []map[string]any {
	rows := []map[string]any{
		f.mbParameter("x1", "Crossover 1", "400 Hz", "Hz", 400),
		f.mbParameter("x2", "Crossover 2", "4000 Hz", "Hz", 4000),
		f.mbParameter("shared", "Knee", "Soft", "enum", 0),
	}
	if kind == "sc_eq" {
		return append(rows,
			f.mbParameter("sc1", "SC EQ Band 1 Frequency", "100 Hz", "Hz", 100),
			f.mbParameter("sc2", "SC EQ Band 2 Frequency", "2 kHz", "Hz", 2000),
		)
	}
	rows = append(rows,
		f.mbParameter("t1", "Band 1 Threshold", "-12 dB", "dB", -12),
		f.mbParameter("g1", "Band 1 Gain", "0 dB", "dB", 0),
		f.mbParameter("a1", "Band 1 Attack", "10 ms", "ms", 10),
		f.mbParameter("t2", "Band 2 Threshold", "-12 dB", "dB", -12),
		f.mbParameter("g2", "Band 2 Gain", "0 dB", "dB", 0),
		f.mbParameter("a2", "Band 2 Attack", "10 ms", "ms", 10),
	)
	switch kind {
	case "dynamic_eq":
		rows = append(rows, f.mbParameter("deq", "Band 1 Dynamic EQ Threshold", "-12 dB", "dB", -12))
	case "de_esser":
		rows = append(rows, f.mbParameter("ds", "De-esser Mode", "Wide", "enum", 0))
	case "maximizer":
		rows = append(rows, f.mbParameter("ceil", "Band 1 Ceiling", "-1 dB", "dB", -1))
	}
	return rows
}

func (f *fakeMultibandKernel) SendVSPCommand(_ context.Context, command string, args map[string]any) (*kernel.VSPCommandResult, error) {
	if command != "plugin.set_params_batch" {
		return nil, fmt.Errorf("unexpected VSP command %s", command)
	}
	rows := mapRowsValue(args["parameters"])
	batch := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		id := firstNonEmptyText(row, "parameter_id")
		value, ok := firstNumericAny(row, "normalized_value")
		if id == "" || !ok {
			return nil, fmt.Errorf("invalid write")
		}
		f.params[id] = value
		batch = append(batch, row)
	}
	f.batches = append(f.batches, batch)
	return &kernel.VSPCommandResult{Payload: map[string]any{"status": "ok"}}, nil
}

func (f *fakeMultibandKernel) mbParameter(id, name, valueText, unit string, physical float64) map[string]any {
	normalized := f.params[id]
	samples := []map[string]any{}
	var values []string
	switch unit {
	case "Hz":
		values = []string{"100 Hz", "400 Hz", "4000 Hz", "10000 Hz", "20000 Hz"}
	case "dB":
		values = []string{"-30 dB", "-20 dB", "-12 dB", "-6 dB", "0 dB"}
	case "ms":
		values = []string{"1 ms", "10 ms", "100 ms", "500 ms", "1000 ms"}
	}
	if len(values) > 0 {
		index := int(math.Round(normalized * float64(len(values)-1)))
		if index < 0 {
			index = 0
		}
		if index >= len(values) {
			index = len(values) - 1
		}
		valueText = values[index]
	}
	for i, text := range values {
		samples = append(samples, map[string]any{"normalized_value": float64(i) / 4, "text": text})
	}
	return map[string]any{"id": id, "name": name, "host_controllable": true, "normalized_value": normalized, "value_text": valueText, "unit": unit, "display_probe": map[string]any{"mode": "samples", "label": valueText, "samples": samples}}
}

func multibandTestRef(summary map[string]any, section, band, role string) string {
	if section == "filterbank" {
		for _, row := range mapRowsValue(mapValue(summary["filterbank"])["crossovers"]) {
			if firstNonEmptyText(row, "role") == role {
				return firstNonEmptyText(row, "control_ref")
			}
		}
	}
	for _, cell := range mapRowsValue(summary["band_cells"]) {
		if band != "" && firstNonEmptyText(cell, "band_key") != band {
			continue
		}
		for _, row := range mapRowsValue(cell[section]) {
			if firstNonEmptyText(row, "role") == role {
				return firstNonEmptyText(row, "control_ref")
			}
		}
	}
	return ""
}

func TestApplyMultibandRejectsInvalidCrossoverOrderBeforeWrite(t *testing.T) {
	fake := newFakeMultibandKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	_, summary, err := server.readLiveMultibandControlSurface(context.Background(), "track-mb", "mb-1")
	if err != nil {
		t.Fatal(err)
	}
	x1, x2 := multibandTestRef(summary, "filterbank", "", "crossover"), ""
	for _, row := range mapRowsValue(mapValue(summary["filterbank"])["crossovers"]) {
		if firstNonEmptyText(row, "param_id") == "x2" {
			x2 = firstNonEmptyText(row, "control_ref")
		}
	}
	if x1 == "" || x2 == "" {
		t.Fatalf("missing crossover refs: %q %q", x1, x2)
	}
	result, err := server.applyPluginGrabberMultibandControls(context.Background(), map[string]any{"track_id": "track-mb", "plugin_id": "mb-1", "atomic": true, "controls": []map[string]any{{"control_ref": x1, "value_hz": 5000.0}, {"control_ref": x2, "value_hz": 1000.0}}}, nil)
	if err == nil || multibandControlFailureCode(err) != "invalid_crossover_order" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(fake.batches) != 0 {
		t.Fatalf("invalid crossover order wrote %d batches", len(fake.batches))
	}
}

func TestPluginGrabberWorkflowExecutorBridgesMultibandInspectAndApply(t *testing.T) {
	fake := newFakeMultibandKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	executor := pluginGrabberWorkflowExecutor{server: server, base: &recordingToolExecutor{}}
	inspect, err := executor.RunToolCall(context.Background(), executorpkg.Input{ToolCall: planner.ToolCall{ID: "inspect", Tool: pluginGrabberInspectMultibandTool, Args: map[string]any{"track_id": "track-mb", "plugin_id": "mb-1"}}})
	if err != nil || inspect.Status != "ok" || len(mapRowsValue(inspect.Result["band_cells"])) != 2 {
		t.Fatalf("inspect=%#v err=%v", inspect, err)
	}
	ref := multibandTestRef(inspect.Result, "gain_action", "band_1", "gain")
	if ref == "" {
		t.Fatal("missing band gain control ref")
	}
	apply, err := executor.RunToolCall(context.Background(), executorpkg.Input{ToolCall: planner.ToolCall{ID: "apply", Tool: pluginGrabberApplyMultibandTool, Args: map[string]any{"track_id": "track-mb", "plugin_id": "mb-1", "atomic": true, "controls": []map[string]any{{"control_ref": ref, "value_db": -6.0}}}}})
	if err != nil || apply.Status != "ok" || firstNonEmptyText(apply.Result, "status") == "rejected" {
		t.Fatalf("apply=%#v err=%v", apply, err)
	}
	if len(fake.batches) != 1 {
		t.Fatalf("batches=%d", len(fake.batches))
	}
}

func TestMultibandGenericWriteGuardOwnsCrossover(t *testing.T) {
	fake := newFakeMultibandKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	resp, blocked := server.guardMultibandOwnedGenericParameterWrite(context.Background(), harness.InvokeRequest{Tool: "plugin.set_parameter", Args: map[string]any{"track_id": "track-mb", "plugin_id": "mb-1", "param_id": "x1", "value": .2}})
	if !blocked || firstNonEmptyText(resp.Result, "rejection_code") != "typed_multiband_control_required" {
		t.Fatalf("resp=%+v blocked=%v", resp, blocked)
	}
}

func TestMultibandNegativeTypedApplyDoesNotWrite(t *testing.T) {
	tests := []struct {
		name string
		kind string
		code string
	}{
		{name: "SC EQ", kind: "sc_eq", code: "not_multiband_dynamics"},
		{name: "dynamic EQ", kind: "dynamic_eq", code: "unsupported_dynamic_eq"},
		{name: "de-esser", kind: "de_esser", code: "unsupported_de_esser"},
		{name: "multiband maximizer", kind: "maximizer", code: "unsupported_multiband_maximizer"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeMultibandKernel()
			fake.rowsOverride = multibandNegativeRows(fake, test.kind)
			server := New(nil, shadow.New(nil), nil)
			server.eqKernelOverride = fake
			result, err := server.applyPluginGrabberMultibandControls(context.Background(), map[string]any{
				"track_id": "track-mb", "plugin_id": "mb-1", "atomic": true,
				"controls": []map[string]any{{"control_ref": "not-used", "value_db": -6.0}},
			}, nil)
			if err == nil || multibandControlFailureCode(err) != test.code {
				t.Fatalf("result=%+v err=%v want=%s", result, err, test.code)
			}
			if len(fake.batches) != 0 {
				t.Fatalf("negative %s wrote %d batches", test.name, len(fake.batches))
			}
		})
	}
}

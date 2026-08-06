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

type fakeCompressorKernel struct {
	params      map[string]float64
	batchCalls  [][]map[string]any
	readCalls   int
	failBatchAt map[int]bool
	mutateOther bool
	brokenProbe bool
}

func newFakeCompressorKernel() *fakeCompressorKernel {
	return &fakeCompressorKernel{params: map[string]float64{"threshold": 0.5, "ratio": 0.25, "attack": 0.5, "unplanned": 0.1}, failBatchAt: map[int]bool{}}
}

func (fake *fakeCompressorKernel) SendCommand(_ context.Context, command map[string]any) (map[string]any, string, error) {
	if firstNonEmptyText(command, "cmd") != "get_plugin_parameters" {
		return nil, "", fmt.Errorf("unexpected command %v", command)
	}
	fake.readCalls++
	rows := []map[string]any{
		fake.compParameter("threshold", "Threshold", "threshold", []string{"-60 dB", "-45 dB", "-30 dB", "-15 dB", "0 dB"}),
		fake.compParameter("ratio", "Ratio", "ratio", []string{"1:1", "2:1", "4:1", "8:1", "20:1"}),
		fake.compParameter("attack", "Attack", "attack", []string{"0.1 ms", "1 ms", "10 ms", "30 ms", "100 ms"}),
		fake.compParameter("unplanned", "Detector State", "detector", []string{"0", "0.25", "0.5", "0.75", "1"}),
	}
	reply := map[string]any{"status": "ok", "track_id": "track-1", "plugin_id": "comp-1", "parameters": rows}
	encoded, _ := json.Marshal(reply)
	return reply, string(encoded), nil
}

func (fake *fakeCompressorKernel) SendVSPCommand(_ context.Context, command string, args map[string]any) (*kernel.VSPCommandResult, error) {
	if command != "plugin.set_params_batch" {
		return nil, fmt.Errorf("unexpected VSP command %s", command)
	}
	rows := mapRowsValue(args["parameters"])
	copyRows := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		id := firstNonEmptyText(row, "parameter_id")
		value, ok := firstNumericAny(row, "normalized_value")
		if id == "" || !ok {
			return nil, errors.New("invalid fake compressor write")
		}
		fake.params[id] = value
		copyRows = append(copyRows, map[string]any{"parameter_id": id, "normalized_value": value})
	}
	fake.batchCalls = append(fake.batchCalls, copyRows)
	if fake.mutateOther && len(fake.batchCalls) == 1 {
		fake.params["unplanned"] = 0.9
	}
	if fake.failBatchAt[len(fake.batchCalls)] {
		return &kernel.VSPCommandResult{Payload: map[string]any{"status": "error", "message": "injected compressor failure"}}, nil
	}
	return &kernel.VSPCommandResult{Payload: map[string]any{"status": "ok"}}, nil
}

func (fake *fakeCompressorKernel) compParameter(id, name, role string, labels []string) map[string]any {
	normalized := fake.params[id]
	currentLabel := labels[int(math.Round(normalized*float64(len(labels)-1)))]
	samples := make([]map[string]any, 0, len(labels))
	for index, label := range labels {
		if fake.brokenProbe {
			label = currentLabel
		}
		samples = append(samples, map[string]any{"normalized_value": float64(index) / float64(len(labels)-1), "text": label})
	}
	return map[string]any{"id": id, "name": name, "host_controllable": true, "normalized_value": normalized,
		"value_text":    currentLabel,
		"display_probe": map[string]any{"mode": "samples", "label": role, "samples": samples}}
}

func (fake *fakeCompressorKernel) snapshot() map[string]float64 {
	result := map[string]float64{}
	for key, value := range fake.params {
		result[key] = value
	}
	return result
}

func TestApplyCompressorControlsUsesAtomicBatchAndRestoresSideEffects(t *testing.T) {
	for _, test := range []struct {
		name        string
		mutateOther bool
		failBatchAt map[int]bool
	}{
		{name: "side effect", mutateOther: true},
		{name: "batch failure", failBatchAt: map[int]bool{1: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeCompressorKernel()
			fake.mutateOther, fake.failBatchAt = test.mutateOther, test.failBatchAt
			server := New(nil, shadow.New(nil), nil)
			server.eqKernelOverride = fake
			digest, summary, err := server.readLiveCompressorControlSurface(context.Background(), "track-1", "comp-1")
			if err != nil {
				t.Fatal(err)
			}
			_ = digest
			if err := attachCompressorControlRefs(summary, "track-1", "comp-1"); err != nil {
				t.Fatal(err)
			}
			path := mapRowsValue(mapValue(summary["compressor_stage"])["control_paths"])[0]
			ref := firstNonEmptyText(mapRowsValue(path["operating_point"])[0], "control_ref")
			before := fake.snapshot()
			result, err := server.applyPluginGrabberCompressorControls(context.Background(), map[string]any{
				"track_id": "track-1", "plugin_id": "comp-1", "atomic": true,
				"controls": []map[string]any{{"control_ref": ref, "value_db": -18.0}},
			}, nil)
			if err == nil || (result != nil && firstNonEmptyText(result, "status") != "rejected") {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if test.mutateOther && !strings.Contains(err.Error(), "unplanned_parameter_change") {
				t.Fatalf("side effect error=%v", err)
			}
			if !fakeSnapshotsEqual(before, fake.snapshot()) {
				t.Fatalf("full compressor preimage not restored before=%v after=%v", before, fake.snapshot())
			}
			if len(fake.batchCalls) != 2 {
				t.Fatalf("batch calls=%d, want mutation plus rollback", len(fake.batchCalls))
			}
		})
	}
}

func TestApplyCompressorControlsPublishesExactNormalizedRestoreRef(t *testing.T) {
	fake := newFakeCompressorKernel()
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	_, summary, err := server.readLiveCompressorControlSurface(context.Background(), "track-1", "comp-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := attachCompressorControlRefs(summary, "track-1", "comp-1"); err != nil {
		t.Fatal(err)
	}
	path := mapRowsValue(mapValue(summary["compressor_stage"])["control_paths"])[0]
	ref := firstNonEmptyText(mapRowsValue(path["operating_point"])[0], "control_ref")
	before := fake.snapshot()
	applied, err := server.applyPluginGrabberCompressorControls(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "comp-1", "atomic": true,
		"controls": []map[string]any{{"control_ref": ref, "value_db": -15.0}},
	}, nil)
	if err != nil || firstNonEmptyText(applied, "status") != "exact" {
		t.Fatalf("apply=%+v err=%v", applied, err)
	}
	restoreRef := firstNonEmptyText(applied, "restore_ref")
	if restoreRef == "" || fakeSnapshotsEqual(before, fake.snapshot()) {
		t.Fatalf("missing restore ref or apply did not mutate: result=%+v", applied)
	}
	restored, err := server.applyPluginGrabberCompressorControls(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "comp-1", "atomic": true, "restore_ref": restoreRef,
	}, nil)
	if err != nil || firstNonEmptyText(restored, "status") != "exact" || restored["restored"] != true {
		t.Fatalf("restore=%+v err=%v", restored, err)
	}
	if !fakeSnapshotsEqual(before, fake.snapshot()) {
		t.Fatalf("normalized preimage drifted: before=%v after=%v", before, fake.snapshot())
	}
}

func TestApplyCompressorControlsUsesTransactionalProbeForBrokenValueToString(t *testing.T) {
	fake := newFakeCompressorKernel()
	fake.brokenProbe = true
	server := New(nil, shadow.New(nil), nil)
	server.eqKernelOverride = fake
	_, summary, err := server.readLiveCompressorControlSurface(context.Background(), "track-1", "comp-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := attachCompressorControlRefs(summary, "track-1", "comp-1"); err != nil {
		t.Fatal(err)
	}
	path := mapRowsValue(mapValue(summary["compressor_stage"])["control_paths"])[0]
	binding := mapRowsValue(path["operating_point"])[0]
	if required, _ := binding["transactional_probe_required"].(bool); !required {
		t.Fatalf("broken value_to_string binding omitted fallback marker: %+v", binding)
	}
	before := fake.snapshot()
	applied, err := server.applyPluginGrabberCompressorControls(context.Background(), map[string]any{
		"track_id": "track-1", "plugin_id": "comp-1", "atomic": true,
		"controls": []map[string]any{{"control_ref": firstNonEmptyText(binding, "control_ref"), "value_db": -15.0}},
	}, nil)
	if err != nil || firstNonEmptyText(applied, "status") != "exact" {
		t.Fatalf("transactional fallback apply=%+v err=%v", applied, err)
	}
	if fake.params["threshold"] == before["threshold"] {
		t.Fatalf("transactional fallback did not reach a new threshold: before=%v after=%v", before, fake.snapshot())
	}
}

func fakeSnapshotsEqual(before, after map[string]float64) bool {
	if len(before) != len(after) {
		return false
	}
	for key, value := range before {
		if math.Abs(after[key]-value) > 1e-9 {
			return false
		}
	}
	return true
}

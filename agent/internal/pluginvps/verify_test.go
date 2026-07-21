package pluginvps

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"vit-daw-agent/internal/vst3host"
)

type fakeVerifierWorker struct {
	doc             Document
	rollbackOK      bool
	writes          int
	rollbacks       int
	closed          bool
	lastTransaction string
}

func (w *fakeVerifierWorker) Close() error {
	w.closed = true
	return nil
}

func (w *fakeVerifierWorker) Call(_ context.Context, operation string, request map[string]any) (vst3host.Response, error) {
	switch operation {
	case "load":
		return workerJSONResponse(map[string]any{
			"identity": map[string]any{
				"name": w.doc.Plugin.Name, "format": w.doc.Plugin.Format, "version": w.doc.Plugin.Version,
				"install_path": w.doc.Plugin.InstallPath, "file_fingerprint": w.doc.Plugin.InstallationHash,
			},
			"parameters": []map[string]any{
				{"id": "enabled", "normalized_value": 0, "display_value": "Off"},
				{"id": "shape", "normalized_value": 0, "display_value": "Bell"},
				{"id": "frequency", "normalized_value": 0.5, "display_value": "632.455 Hz"},
				{"id": "gain", "normalized_value": 0.5, "display_value": "0 dB"},
				{"id": "q", "normalized_value": 0.5, "display_value": "1.095 Q"},
				{"id": "unbound", "normalized_value": 0, "display_value": "0"},
			},
		})
	case "write":
		changes, _ := request["changes"].([]map[string]any)
		if len(changes) != 1 {
			return vst3host.Response{}, fmt.Errorf("unexpected changes: %#v", request["changes"])
		}
		id, _ := changes[0]["id"].(string)
		normalized, _ := changes[0]["normalized"].(float64)
		w.writes++
		w.lastTransaction = fmt.Sprintf("tx-%d", w.writes)
		return workerJSONResponse(map[string]any{
			"transaction_id": w.lastTransaction,
			"fresh_readback": map[string]any{"parameters": []map[string]any{{
				"id": id, "normalized_value": normalized, "display_value": fakeDisplay(id, normalized),
			}}},
		})
	case "rollback":
		if request["transaction_id"] != w.lastTransaction {
			return vst3host.Response{}, fmt.Errorf("rollback transaction = %#v, want %q", request["transaction_id"], w.lastTransaction)
		}
		w.rollbacks++
		return workerJSONResponse(map[string]any{"rollback_verified": w.rollbackOK, "fresh_readback": map[string]any{}})
	default:
		return vst3host.Response{}, fmt.Errorf("unexpected operation %q", operation)
	}
}

func workerJSONResponse(value any) (vst3host.Response, error) {
	raw, err := json.Marshal(value)
	return vst3host.Response{Result: raw}, err
}

func fakeDisplay(id string, normalized float64) string {
	switch id {
	case "enabled":
		if normalized == 0 {
			return "Off"
		}
		return "On"
	case "shape":
		if normalized == 0 {
			return "Bell"
		}
		return "Notch"
	case "frequency":
		return fmt.Sprintf("%.6f Hz", 20*math.Pow(1000, normalized))
	case "gain":
		return fmt.Sprintf("%.6f dB", -18+36*normalized)
	case "q":
		return fmt.Sprintf("%.6f Q", 0.1*math.Pow(120, normalized))
	default:
		return fmt.Sprintf("%.6f", normalized)
	}
}

func TestVerifyUsesFreshReadbackAndRollbackBeforeStamping(t *testing.T) {
	doc := testDocument()
	worker := &fakeVerifierWorker{doc: doc, rollbackOK: true}
	now := time.Date(2026, time.July, 21, 3, 4, 5, 0, time.UTC)
	result, err := Verify(context.Background(), doc, VerifyOptions{
		WorkerPath: "fake-worker",
		Now:        func() time.Time { return now },
		StartWorker: func(string) (VerifierWorker, error) {
			return worker, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !worker.closed {
		t.Fatal("worker was not closed")
	}
	if result.Checks != 13 || worker.writes != result.Checks || worker.rollbacks != result.Checks {
		t.Fatalf("checks=%d writes=%d rollbacks=%d", result.Checks, worker.writes, worker.rollbacks)
	}
	if !result.Document.Verified || result.Document.Verification == nil {
		t.Fatalf("document was not stamped: %#v", result.Document)
	}
	stamp := result.Document.Verification
	if !stamp.VerifiedAt.Equal(now) || stamp.InstallationHash != testInstallationHash || stamp.ParameterSurfaceHash != HashSurface([]string{"enabled", "shape", "frequency", "gain", "q", "unbound"}) {
		t.Fatalf("verification stamp = %#v", stamp)
	}
}

func TestVerifyFailsClosedWhenRollbackCannotBeVerified(t *testing.T) {
	doc := testDocument()
	worker := &fakeVerifierWorker{doc: doc, rollbackOK: false}
	result, err := Verify(context.Background(), doc, VerifyOptions{
		WorkerPath: "fake-worker",
		StartWorker: func(string) (VerifierWorker, error) {
			return worker, nil
		},
	})
	if err == nil {
		t.Fatalf("verify unexpectedly passed: %#v", result)
	}
	if result.Document.Verified {
		t.Fatalf("failed verification stamped document: %#v", result.Document)
	}
	if worker.writes != 1 || worker.rollbacks != 1 || !worker.closed {
		t.Fatalf("worker discipline writes=%d rollbacks=%d closed=%v", worker.writes, worker.rollbacks, worker.closed)
	}
}

func TestCompareDisplayValueTreatsToggleAsLabels(t *testing.T) {
	for _, tc := range []struct {
		display    string
		normalized float64
	}{
		{"Off", 0}, {"Disabled", 0}, {"Unused", 0}, {"On", 1}, {"Enabled", 1}, {"Used", 1},
	} {
		if err := compareDisplayValue(tc.display, tc.normalized, tc.normalized, "toggle"); err != nil {
			t.Fatalf("%q at %.0f: %v", tc.display, tc.normalized, err)
		}
	}
}

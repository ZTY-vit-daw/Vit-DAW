package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/com"
	"vit-daw-agent/internal/mixboard"
)

// MAT-1 root cause pinned at unit level: the kernel-side dual-tap probe
// completes and writes its evidence artifact, but with no COM evidence
// workspace-root environment variable supplied to the agent process the
// paired observation fails closed with the R2-observed named error instead of
// silently degrading.
func TestPrepareMixObservationCOMEvidenceNamesMissingWorkspaceRootSupply(t *testing.T) {
	for _, key := range []string{"VIT_DAW_DEV_ROOT", "VIT_DEV_ROOT", "VIT_ROOT", "VIT_MIXBOARD_ROOT"} {
		t.Setenv(key, "")
	}
	pairID := "com2_unsupplied_root"
	kernelRoot := t.TempDir()
	artifactPath := filepath.Join(kernelRoot, "VitApp", "Workspace", "Artifacts", "com_evidence", pairID, pairID+".json")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte(`{"schema_version":"dad.compressor_dual_tap_evidence.v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	receipt := validCOMReceipt(pairID)
	receiptMap := map[string]any{}
	data, _ := json.Marshal(receipt)
	_ = json.Unmarshal(data, &receiptMap)
	h := &Harness{comProbeCollect: func(_ context.Context, _ map[string]any, _, _, _ string) (map[string]any, map[string]any, error) {
		return receiptMap, map[string]any{"status": "ok", "pair_id": pairID}, nil
	}}
	_, _, err := h.prepareMixObservationCOMEvidence(context.Background(), map[string]any{
		"com_mode": com.ModePairedIO, "track_id": "1007", "plugin_id": "1021", "clip_id": "2001",
		"topology_class": "amount_driven", "topology_generation": "topology-1", "start_sample": 0, "end_sample": 480000,
	}, mixboard.TargetRef{Kind: "track", ID: "1007"}, map[string]any{"track_id": "1007", "clip_id": "2001"})
	if err == nil || !strings.Contains(err.Error(), "COM evidence workspace root is unavailable") {
		t.Fatalf("missing workspace-root supply was not named fail-closed: %v", err)
	}
	if _, statErr := os.Stat(artifactPath); statErr != nil {
		t.Fatalf("kernel-side evidence artifact vanished during the failed supply: %v", statErr)
	}
}

// The same probe with the supply rooted at the kernel's real workspace parent
// completes: this is the state the launch-env fix hands the live branch.
func TestPrepareMixObservationCOMEvidenceSucceedsWithSuppliedWorkspaceRoot(t *testing.T) {
	pairID := "com2_supplied_root"
	root := t.TempDir()
	t.Setenv("VIT_DAW_DEV_ROOT", root)
	t.Setenv("VIT_DEV_ROOT", "")
	t.Setenv("VIT_ROOT", "")
	artifactPath := filepath.Join(root, "VitApp", "Workspace", "Artifacts", "com_evidence", pairID, pairID+".json")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte(`{"schema_version":"dad.compressor_dual_tap_evidence.v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	receipt := validCOMReceipt(pairID)
	receiptMap := map[string]any{}
	data, _ := json.Marshal(receipt)
	_ = json.Unmarshal(data, &receiptMap)
	h := &Harness{comProbeCollect: func(_ context.Context, _ map[string]any, _, _, _ string) (map[string]any, map[string]any, error) {
		return receiptMap, map[string]any{"status": "ok", "pair_id": pairID}, nil
	}}
	out, capture, err := h.prepareMixObservationCOMEvidence(context.Background(), map[string]any{
		"com_mode": com.ModePairedIO, "track_id": "1007", "plugin_id": "1021", "clip_id": "2001",
		"topology_class": "amount_driven", "topology_generation": "topology-1", "start_sample": 0, "end_sample": 480000,
	}, mixboard.TargetRef{Kind: "track", ID: "1007"}, map[string]any{"track_id": "1007", "clip_id": "2001"})
	if err != nil {
		t.Fatalf("supplied workspace root still failed: %v", err)
	}
	if firstString(out, "com_artifact_path") != artifactPath || firstString(capture, "status") != com.StatusReady {
		t.Fatalf("supplied COM capture out=%+v capture=%+v", out, capture)
	}
}

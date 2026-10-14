package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/logx"
	"vit-daw-agent/internal/shadow"
)

// vspIdentitySnapshot builds a VSP state snapshot the way the kernel reply
// parser would: project UUID always present, the identity triple present only
// when the typed reply carried it (revision>0 / non-empty epoch and hash).
func vspIdentitySnapshot(uuid string, revision int64, epoch, hash string) *kernel.VSPStateResult {
	return &kernel.VSPStateResult{
		LegacyState: map[string]any{
			"status":       "ok",
			"project_uuid": uuid,
			"tracks":       []any{},
		},
		Revision:     revision,
		ProjectEpoch: epoch,
		SnapshotHash: hash,
	}
}

func identityFromShadowSummary(t *testing.T, h *Harness) (revision, epoch, hash, uuid string) {
	t.Helper()
	summary := h.shadow.Summary()
	return firstString(summary, "project_revision"),
		firstString(summary, "project_epoch"),
		firstString(summary, "snapshot_hash"),
		firstString(summary, "project_uuid")
}

// Form ①: a snapshot that carries a new identity triple updates the shadow
// (existing semantics untouched by preservation).
func TestVSPShadowIdentityUpdatesOnCompleteSnapshot(t *testing.T) {
	h := New(nil, shadow.New(nil), nil)
	state, _ := h.vspShadowState(vspIdentitySnapshot("proj-a", 11, "epoch_aaa", "hash_aaa"))
	h.shadow.Initialize(state)
	revision, epoch, hash, uuid := identityFromShadowSummary(t, h)
	if revision != "11" || epoch != "epoch_aaa" || hash != "hash_aaa" || uuid != "proj-a" {
		t.Fatalf("initial bind missing: revision=%q epoch=%q hash=%q uuid=%q", revision, epoch, hash, uuid)
	}

	state, restored := h.vspShadowState(vspIdentitySnapshot("proj-a", 12, "epoch_bbb", "hash_bbb"))
	if len(restored) != 0 {
		t.Fatalf("complete snapshot must not be touched by preservation, restored=%v", restored)
	}
	h.shadow.Initialize(state)
	revision, epoch, hash, _ = identityFromShadowSummary(t, h)
	if revision != "12" || epoch != "epoch_bbb" || hash != "hash_bbb" {
		t.Fatalf("new identity not adopted: revision=%q epoch=%q hash=%q", revision, epoch, hash)
	}
}

// Form ②: a snapshot that drops the identity triple while the shadow still
// holds the last known identity for the same project keeps that identity and
// records the identity_preserved_on_incomplete_snapshot telemetry marker.
func TestVSPShadowIdentityPreservedOnIncompleteSnapshot(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "agent.log")
	h := New(nil, shadow.New(nil), logx.New(false, logPath, 100))
	state, _ := h.vspShadowState(vspIdentitySnapshot("proj-a", 11, "epoch_aaa", "hash_aaa"))
	h.shadow.Initialize(state)

	incomplete, restored := h.vspShadowState(vspIdentitySnapshot("proj-a", 0, "", ""))
	want := []string{"project_epoch", "project_revision", "snapshot_hash"}
	if strings.Join(restored, ",") != strings.Join(want, ",") {
		t.Fatalf("preserved fields = %v, want %v", restored, want)
	}
	h.shadow.Initialize(incomplete)
	revision, epoch, hash, _ := identityFromShadowSummary(t, h)
	if revision != "11" || epoch != "epoch_aaa" || hash != "hash_aaa" {
		t.Fatalf("last known identity not preserved: revision=%q epoch=%q hash=%q", revision, epoch, hash)
	}

	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read agent log: %v", err)
	}
	if !strings.Contains(string(logged), "identity_preserved_on_incomplete_snapshot") {
		t.Fatalf("telemetry marker missing from log: %s", logged)
	}

	// Partial form: only the missing field is restored, present fields win.
	state, _ = h.vspShadowState(vspIdentitySnapshot("proj-a", 12, "epoch_bbb", "hash_bbb"))
	h.shadow.Initialize(state)
	partial, restored := h.vspShadowState(vspIdentitySnapshot("proj-a", 12, "epoch_bbb", ""))
	if strings.Join(restored, ",") != "snapshot_hash" {
		t.Fatalf("partial preservation fields = %v, want [snapshot_hash]", restored)
	}
	h.shadow.Initialize(partial)
	revision, epoch, hash, _ = identityFromShadowSummary(t, h)
	if revision != "12" || epoch != "epoch_bbb" || hash != "hash_bbb" {
		t.Fatalf("partial preservation wrong: revision=%q epoch=%q hash=%q", revision, epoch, hash)
	}
}

// Form ③: a snapshot that drops the identity triple while the shadow was
// never bound stays unbound (fail closed unchanged).
func TestVSPShadowIdentityStaysUnboundWithoutLastKnown(t *testing.T) {
	h := New(nil, shadow.New(nil), nil)
	state, restored := h.vspShadowState(vspIdentitySnapshot("proj-a", 0, "", ""))
	if len(restored) != 0 {
		t.Fatalf("unbound shadow must not gain identity, restored=%v", restored)
	}
	h.shadow.Initialize(state)
	revision, epoch, hash, uuid := identityFromShadowSummary(t, h)
	if revision != "" || epoch != "" || hash != "" {
		t.Fatalf("unbound shadow must stay unbound: revision=%q epoch=%q hash=%q uuid=%q", revision, epoch, hash, uuid)
	}
}

// Cross-project guard: a snapshot from another project never inherits the
// previous project's identity (real project switches keep the baseline reset
// path of the shadow change comparator).
func TestVSPShadowIdentityNotPreservedAcrossProjects(t *testing.T) {
	h := New(nil, shadow.New(nil), nil)
	state, _ := h.vspShadowState(vspIdentitySnapshot("proj-a", 11, "epoch_aaa", "hash_aaa"))
	h.shadow.Initialize(state)

	incomplete, restored := h.vspShadowState(vspIdentitySnapshot("proj-b", 0, "", ""))
	if len(restored) != 0 {
		t.Fatalf("cross-project snapshot must not inherit identity, restored=%v", restored)
	}
	h.shadow.Initialize(incomplete)
	revision, epoch, hash, uuid := identityFromShadowSummary(t, h)
	if uuid != "proj-b" {
		t.Fatalf("project switch not adopted: uuid=%q", uuid)
	}
	if revision != "" || epoch != "" || hash != "" {
		t.Fatalf("cross-project snapshot must stay unbound: revision=%q epoch=%q hash=%q", revision, epoch, hash)
	}
}

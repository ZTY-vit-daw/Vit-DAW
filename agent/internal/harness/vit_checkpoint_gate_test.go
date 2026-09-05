package harness

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"vit-daw-agent/internal/history"
	"vit-daw-agent/internal/shadow"
)

// countingKernelClient wraps fakeKernelClient with a mutex-guarded kernel
// snapshot-export counter: the async checkpoint worker runs on its own
// goroutine, so the counter must be race-safe even without -race.
type countingKernelClient struct {
	fakeKernelClient
	mu            sync.Mutex
	snapshotCount int
}

func (c *countingKernelClient) SendCommand(ctx context.Context, cmd map[string]any) (map[string]any, string, error) {
	reply, raw, err := c.fakeKernelClient.SendCommand(ctx, cmd)
	name := firstString(cmd, "cmd")
	if name == "project.snapshot_export" || name == "project_snapshot_export" {
		c.mu.Lock()
		c.snapshotCount++
		c.mu.Unlock()
	}
	return reply, raw, err
}

func (c *countingKernelClient) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshotCount
}

func newVitGateHarness(t *testing.T, kernel KernelSender, revision int64) (*Harness, string) {
	t.Helper()
	root := t.TempDir()
	projectPath := filepath.Join(root, "gate.vit")
	const projectUUID = "vitproj_gate_test"
	history.BindProjectIdentity(projectPath, projectUUID)
	if _, err := history.EnsureWorkingSession(projectPath, projectUUID); err != nil {
		t.Fatal(err)
	}
	shadowProject := shadow.New(nil)
	shadowProject.Initialize(map[string]any{
		"project_path":     projectPath,
		"project_uuid":     projectUUID,
		"project_revision": revision,
		"tracks":           []any{},
	})
	h := New(nil, shadowProject, nil)
	h.kernel = kernel
	return h, projectPath
}

func seedHeadCheckpoint(t *testing.T, projectPath, revision string) string {
	t.Helper()
	checkpoint, err := history.Checkpoint(map[string]any{
		"project_path":         projectPath,
		"message":              "seed",
		"source":               "test",
		"project_snapshot_xml": "<project/>",
		"project_revision":     revision,
	})
	if err != nil {
		t.Fatal(err)
	}
	return firstString(checkpoint, "commit_id")
}

func vitNodeFromResult(t *testing.T, out map[string]any) history.ConversationNode {
	t.Helper()
	typed, ok := out["node"].(history.ConversationNode)
	if !ok {
		t.Fatalf("vit node missing from append result: %+v", out)
	}
	return typed
}

func waitForVitCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", timeout)
}

// Zero-change turns (revision unchanged since the last covered checkpoint)
// must not export a kernel snapshot nor write a checkpoint: the node binds to
// the current HEAD. This is the path that took 85-114s per reply pre-gate.
func TestVitConversationCheckpointSkipsUnchangedRevision(t *testing.T) {
	kernel := &countingKernelClient{}
	h, projectPath := newVitGateHarness(t, kernel, 5)
	seedHead := seedHeadCheckpoint(t, projectPath, "5")

	out := h.RecordConversationNodeForProjectWithData(context.Background(), projectPath, "vit", "hello again", "g1", "r1", nil)
	if !boolValueDefault(out["available"], false) {
		t.Fatalf("node append failed: %+v", out)
	}
	if got := kernel.count(); got != 0 {
		t.Fatalf("zero-change turn must not export kernel snapshot, got %d exports", got)
	}
	if node := vitNodeFromResult(t, out); node.CommitID != seedHead {
		t.Fatalf("skipped turn node should bind HEAD %s, got %s", seedHead, node.CommitID)
	}
}

// A revision-advancing turn returns before the kernel export runs; the
// checkpoint lands asynchronously, and the appended node is rebound to the new
// commit so node→commit restore semantics survive the async move.
func TestVitConversationCheckpointAsyncAdvancesAndRebinds(t *testing.T) {
	kernel := &countingKernelClient{fakeKernelClient: fakeKernelClient{replies: []map[string]any{
		{"status": "ok", "snapshot_xml": "<project revision='6'/>", "project_path": "kernel-side.vit"},
	}}}
	h, projectPath := newVitGateHarness(t, kernel, 6)
	oldHead := seedHeadCheckpoint(t, projectPath, "5")

	out := h.RecordConversationNodeForProjectWithData(context.Background(), projectPath, "vit", "changed the mix", "g1", "r1", nil)
	if !boolValueDefault(out["available"], false) {
		t.Fatalf("node append failed: %+v", out)
	}
	node := vitNodeFromResult(t, out)
	if node.CommitID != oldHead {
		t.Fatalf("async turn node should append against old HEAD %s before rebind, got %s", oldHead, node.CommitID)
	}

	// Wait for the worker to finish ENTIRELY (checkpoint + rebind), not just
	// for HEAD to advance: the rebind runs after the commit inside the
	// worker's lock section, and reading the graph in between sees the old
	// node binding.
	waitForVitCondition(t, 5*time.Second, func() bool {
		h.vitGateMu.Lock()
		live := h.vitCheckpointLive[projectPath]
		h.vitGateMu.Unlock()
		if live {
			return false
		}
		revision, _, err := history.HeadCheckpointRevision(map[string]any{"project_path": projectPath})
		return err == nil && revision == "6"
	})
	_, newHead, err := history.HeadCheckpointRevision(map[string]any{"project_path": projectPath})
	if err != nil || newHead == "" || newHead == oldHead {
		t.Fatalf("async checkpoint missing: head=%s err=%v", newHead, err)
	}

	messages, err := history.ConversationMessages(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatal(err)
	}
	graph, ok := messages["conversation_graph"].(history.ConversationGraph)
	if !ok {
		t.Fatalf("conversation graph missing: %+v", messages)
	}
	rebound := false
	for _, n := range graph.Nodes {
		if n.ID == node.ID {
			rebound = true
			if n.CommitID != newHead {
				t.Fatalf("node %s not rebound to async commit %s, got %s", n.ID, newHead, n.CommitID)
			}
		}
	}
	if !rebound {
		t.Fatalf("node %s missing from graph after rebind", node.ID)
	}

	// A later turn at the same revision is gated: exactly one export total.
	outTwo := h.RecordConversationNodeForProjectWithData(context.Background(), projectPath, "vit", "hello again", "g2", "r2", nil)
	if !boolValueDefault(outTwo["available"], false) {
		t.Fatalf("second node append failed: %+v", outTwo)
	}
	if got := kernel.count(); got != 1 {
		t.Fatalf("post-checkpoint turn must be gated, want 1 export total, got %d", got)
	}
	if nodeTwo := vitNodeFromResult(t, outTwo); nodeTwo.CommitID != newHead {
		t.Fatalf("gated turn node should bind new HEAD %s, got %s", newHead, nodeTwo.CommitID)
	}
}

// When the async checkpoint fails, the optimistic revision cover is rolled
// back so the next turn retries instead of skipping forever.
func TestVitConversationCheckpointAsyncFailureRollsBackGate(t *testing.T) {
	// No queued reply: SendCommand returns a bare ok map without snapshot_xml,
	// so enrichment produces no snapshot payload and Checkpoint falls back to
	// reading the (nonexistent) project file and fails.
	failing := &countingKernelClient{}
	h, projectPath := newVitGateHarness(t, failing, 6)
	seedHeadCheckpoint(t, projectPath, "5")

	out := h.RecordConversationNodeForProjectWithData(context.Background(), projectPath, "vit", "turn that fails to checkpoint", "g1", "r1", nil)
	if !boolValueDefault(out["available"], false) {
		t.Fatalf("node append failed: %+v", out)
	}
	node := vitNodeFromResult(t, out)

	waitForVitCondition(t, 5*time.Second, func() bool {
		h.vitGateMu.Lock()
		defer h.vitGateMu.Unlock()
		return !h.vitCheckpointLive[projectPath]
	})
	h.vitGateMu.Lock()
	covered, stillCovered := h.vitCoveredRevision[projectPath]
	h.vitGateMu.Unlock()
	if stillCovered && covered == "6" {
		t.Fatal("failed async checkpoint must roll the optimistic revision cover back")
	}

	// The worker must have rolled the optimistic cover back: assert the retry
	// path by swapping in a healthy kernel and re-recording at the same
	// revision — if the gate had stayed optimistically covered, this turn
	// would skip and HEAD would never advance to "6".
	healthy := &countingKernelClient{fakeKernelClient: fakeKernelClient{replies: []map[string]any{
		{"status": "ok", "snapshot_xml": "<project revision='6'/>", "project_path": "kernel-side.vit"},
	}}}
	h.kernel = healthy
	outTwo := h.RecordConversationNodeForProjectWithData(context.Background(), projectPath, "vit", "retry turn", "g2", "r2", nil)
	if !boolValueDefault(outTwo["available"], false) {
		t.Fatalf("retry node append failed: %+v", outTwo)
	}
	nodeTwo := vitNodeFromResult(t, outTwo)
	// Wait for the worker to finish ENTIRELY (checkpoint + rebind), not just
	// for HEAD to advance: the rebind runs after the commit inside the
	// worker's lock section, and reading the graph in between sees the old
	// node binding.
	waitForVitCondition(t, 5*time.Second, func() bool {
		h.vitGateMu.Lock()
		live := h.vitCheckpointLive[projectPath]
		h.vitGateMu.Unlock()
		if live {
			return false
		}
		revision, _, err := history.HeadCheckpointRevision(map[string]any{"project_path": projectPath})
		return err == nil && revision == "6"
	})
	_, newHead, err := history.HeadCheckpointRevision(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatal(err)
	}
	messages, err := history.ConversationMessages(map[string]any{"project_path": projectPath})
	if err != nil {
		t.Fatal(err)
	}
	graph := messages["conversation_graph"].(history.ConversationGraph)
	for _, n := range graph.Nodes {
		if (n.ID == node.ID || n.ID == nodeTwo.ID) && n.CommitID != newHead {
			t.Fatalf("node %s not rebound to retry commit %s, got %s", n.ID, newHead, n.CommitID)
		}
	}
}

// First node ever (no HEAD checkpoint): the legacy synchronous checkpoint
// still runs on the response path — the node append needs a commit to exist.
func TestVitConversationCheckpointSyncsWhenNoHeadExists(t *testing.T) {
	kernel := &countingKernelClient{fakeKernelClient: fakeKernelClient{replies: []map[string]any{
		{"status": "ok", "snapshot_xml": "<project/>", "project_path": "kernel-side.vit"},
	}}}
	h, projectPath := newVitGateHarness(t, kernel, 3)

	out := h.RecordConversationNodeForProjectWithData(context.Background(), projectPath, "vit", "first turn", "g1", "r1", nil)
	if !boolValueDefault(out["available"], false) {
		t.Fatalf("node append failed: %+v", out)
	}
	if got := kernel.count(); got != 1 {
		t.Fatalf("first node must checkpoint synchronously, got %d exports", got)
	}
	node := vitNodeFromResult(t, out)
	if node.CommitID == "" {
		t.Fatal("first node must bind the sync checkpoint commit")
	}
	revision, head, err := history.HeadCheckpointRevision(map[string]any{"project_path": projectPath})
	if err != nil || revision != "3" || head != node.CommitID {
		t.Fatalf("sync checkpoint missing revision binding: rev=%q head=%s node=%s err=%v", revision, head, node.CommitID, err)
	}
}

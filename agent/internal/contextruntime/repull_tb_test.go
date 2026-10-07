package contextruntime

// repull_tb_test.go — T-B1/T-B2/T-B3（§6.2 退场一致性组，蓝图"句柄重拉
// 一致性"）：既有读侧组合（ref.query 等值谓词 + Expand，L1-3 接口冻结——
// 零工具动词/schema/谓词改动）之上的三态包装。
//
//	T-B1 退场后重拉 resolved：turn k 观察票 → 退场（executor ref 态）→
//	    turn k+n 重拉 → state=resolved；内容与原票 hash 一致；freshness 透传。
//	T-B2 退场后重拉 stale：物化层标脏（v0 bootstrap 的标脏面=产物消失
//	    marked_stale 事件，行经事件代保留）→ 重拉 → state=stale 且
//	    freshness=stale 原样，无任何升级路径。
//	    （诚实边界：v0 的 stale 行内容工件已不在盘——handle 空缺如实；
//	    L1-2 物化层 v1 内容变更标脏后，handle/summary 自然补全。）
//	T-B3 退场后重拉 missing：隔离工作区（t.TempDir 副本语义）移除票工件
//	    → 重建 → 重拉 → state=missing 且 pointer 指向正确账本条目
//	    （ledger_entry_id + turn_id），非静默（响应含指引）。

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vit-daw-agent/internal/llm"
	"vit-daw-agent/internal/queryengine"
)

const tbSessionUUID = "vitproj_tbrepull"

type tbTree struct {
	agentRoot string
	ticket    string
}

func tbNewTree(t *testing.T) tbTree {
	t.Helper()
	base := t.TempDir()
	tree := tbTree{agentRoot: filepath.Join(base, "agent", ".vit_agent")}
	if err := os.MkdirAll(filepath.Join(tree.agentRoot, tbSessionUUID, "observations"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return tree
}

func (tr *tbTree) writeTicket(t *testing.T, obsID string) []byte {
	t.Helper()
	packet := map[string]any{
		"schema_version": "mixboard_observation.v1",
		"observation_id": obsID,
		"mix_session_id": "mix_tb",
		"status":         "ready",
		"target_ref":     map[string]any{"kind": "track", "id": "T1", "label": "tb-lead"},
		"created_at":     "2026-10-07T00:00:00Z",
		"dom_projection": map[string]any{
			"schema_version": "dom_projection.v1", "status": "ready", "mode": "source_only",
		},
	}
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatalf("marshal ticket: %v", err)
	}
	tr.ticket = filepath.Join(tr.agentRoot, tbSessionUUID, "observations", obsID+".json")
	if err := os.WriteFile(tr.ticket, data, 0o644); err != nil {
		t.Fatalf("write ticket: %v", err)
	}
	return data
}

func tbEngine(t *testing.T, tree tbTree) *queryengine.Engine {
	t.Helper()
	store := queryengine.NewBootstrapStore(queryengine.BootstrapConfig{
		AgentRoot:    tree.agentRoot,
		PollInterval: 10 * time.Millisecond,
	})
	engine := queryengine.NewEngine(store, nil)
	if err := engine.Sync(context.Background()); err != nil {
		t.Fatalf("engine sync: %v", err)
	}
	return engine
}

// tbFirstDomRef 取引擎当前 dom 行的 canonical ref（测试不臆造坐标——读侧
// 给什么重拉什么）。
func tbFirstDomRef(t *testing.T, engine *queryengine.Engine) string {
	t.Helper()
	result, err := engine.Query(context.Background(), queryengine.RefQuery{
		Kinds: queryengine.KindPredicate{Kinds: []string{"dom"}},
	})
	if err != nil {
		t.Fatalf("query dom rows: %v", err)
	}
	if len(result.Rows) == 0 {
		t.Fatalf("no dom rows materialized from synthetic ticket")
	}
	return result.Rows[0].Ref
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestTB1RepullResolvedAfterExit：退场（ref 态）后重拉 resolved，内容身份
// 与原票一致，freshness 物化层透传。
func TestTB1RepullResolvedAfterExit(t *testing.T) {
	tree := tbNewTree(t)
	original := tree.writeTicket(t, "obs_tb1")
	engine := tbEngine(t, tree)
	ref := tbFirstDomRef(t, engine)

	// turn k 边界：观察票退场（executor ref 态——票=天然可重拉句柄）。
	executor := NewExitExecutor(ExitExecutorConfig{Now: fixedClock(t)})
	exit := executor.OnTurnBoundary(context.Background(), TurnBoundaryEvent{
		TurnID: "turn-k",
		Window: WindowState{Units: []WindowUnit{{
			Unit: ExitUnit{Kind: ExitUnitObservationBundle, ID: "obs_tb1"}, TurnID: "turn-k",
			HandleRef: ref,
		}}},
	})
	if len(exit.Decisions) != 1 || exit.Decisions[0].Action != ExitRef || exit.Decisions[0].HandleRef != ref {
		t.Fatalf("T-B1 red: ticketed observation must exit as ref: %+v", exit.Decisions)
	}

	// turn k+n：票已出窗，重拉（等值谓词 + expand summary）。
	service := &RepullService{Engine: engine}
	response := service.Repull(context.Background(), ref)
	if response.State != RepullResolved {
		t.Fatalf("T-B1 red: state=%s, want resolved (%s)", response.State, response.Note)
	}
	if response.Freshness != queryengine.FreshnessMaterialReuse {
		t.Fatalf("T-B1 red: freshness must pass through materialize layer verbatim, got %q", response.Freshness)
	}
	if response.Handle == "" || response.Bytes == 0 {
		t.Fatalf("resolved response must carry handle+bytes: %+v", response)
	}
	reread, err := os.ReadFile(response.Handle)
	if err != nil {
		t.Fatalf("read resolved handle: %v", err)
	}
	if sha256Hex(reread) != sha256Hex(original) {
		t.Fatalf("T-B1 red: re-pulled content hash diverges from the original ticket")
	}
	if response.Summary == nil {
		t.Fatalf("resolved response must carry expand-budget summary")
	}
}

// TestTB2RepullStaleNoUpgrade：物化层标脏后重拉 → stale 原样，无升级路径。
func TestTB2RepullStaleNoUpgrade(t *testing.T) {
	ctx := context.Background()
	tree := tbNewTree(t)
	tree.writeTicket(t, "obs_tb2")
	engine := tbEngine(t, tree)
	ref := tbFirstDomRef(t, engine)

	// 订阅事件流（poll 10ms），确保 marked_stale 可见性进 latest 代。
	if err := engine.Start(ctx); err != nil {
		t.Fatalf("engine start: %v", err)
	}
	// 构造变更→物化层标脏：v0 bootstrap 的标脏面=产物消失（marked_stale
	// 只改状态不删行，行经事件代保留——§5.1.1）。
	if err := os.Remove(tree.ticket); err != nil {
		t.Fatalf("remove ticket: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	staleSeen := false
	for time.Now().Before(deadline) {
		result, err := engine.Query(ctx, queryengine.RefQuery{Kinds: queryengine.KindPredicate{Kinds: []string{"dom"}}})
		if err == nil && len(result.Rows) > 0 && result.Rows[0].Freshness == queryengine.FreshnessStale {
			staleSeen = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !staleSeen {
		t.Fatalf("T-B2 setup red: materialize layer never marked the row stale (poll/event path)")
	}

	service := &RepullService{Engine: engine}
	response := service.Repull(ctx, ref)
	if response.State != RepullStale {
		t.Fatalf("T-B2 red: state=%s, want stale (%s)", response.State, response.Note)
	}
	if response.Freshness != queryengine.FreshnessStale {
		t.Fatalf("T-B2 red: freshness must stay stale verbatim (no upgrade), got %q", response.Freshness)
	}
	if !strings.Contains(response.Note, "stale") {
		t.Fatalf("stale response note must state the no-upgrade semantics: %q", response.Note)
	}
	if response.State == RepullResolved {
		t.Fatalf("T-B2 red: stale upgraded to resolved — forbidden")
	}
}

// TestTB3RepullMissingWithLedgerPointer：票工件消失（隔离工作区）且事件代
// 不含行 → missing + pointer{ledger_entry_id, turn_id} 指引，非静默。
func TestTB3RepullMissingWithLedgerPointer(t *testing.T) {
	ctx := context.Background()
	tree := tbNewTree(t)
	tree.writeTicket(t, "obs_tb3")
	engine := tbEngine(t, tree)
	ref := tbFirstDomRef(t, engine)

	// 结论先入账（retain 接线）+ 伴随索引注记 LastSeenTurn=turn-3。
	projectDir := t.TempDir()
	executor := NewExitExecutor(ExitExecutorConfig{Now: fixedClock(t)})
	exit := executor.OnTurnBoundary(ctx, TurnBoundaryEvent{
		TurnID: "turn-3",
		Window: WindowState{Units: []WindowUnit{{
			Unit: ExitUnit{Kind: ExitUnitObservationBundle, ID: "obs_tb3"}, TurnID: "turn-3",
			RetainedStatement: "T1 峰值因子 3.2dB（结论级，票工件退场后兜底）",
			EvidenceRefs:      []string{ref},
		}}},
	})
	if len(exit.Decisions) != 1 || exit.Decisions[0].Action != ExitRetain {
		t.Fatalf("T-B3 setup: retain decision expected: %+v", exit.Decisions)
	}
	written, err := WriteRetains(projectDir, exit, fixedClock(t)())
	if err != nil || len(written) != 1 {
		t.Fatalf("T-B3 setup: WriteRetains: %v / %d", err, len(written))
	}
	history := []llm.Message{{Role: "assistant", Content: "观察结论见 " + ref}}
	companion := (&HistoryRefIndexer{}).Build(ctx, history, "turn-3")

	// 隔离副本上移除票工件（§10 防污染：t.TempDir 隔离工作区）→ 重建。
	if err := os.Remove(tree.ticket); err != nil {
		t.Fatalf("remove ticket: %v", err)
	}
	if err := engine.Sync(ctx); err != nil {
		t.Fatalf("engine resync: %v", err)
	}

	service := &RepullService{Engine: engine, ProjectDir: projectDir, History: companion}
	response := service.Repull(ctx, ref)
	if response.State != RepullMissing {
		t.Fatalf("T-B3 red: state=%s, want missing (%s)", response.State, response.Note)
	}
	if response.Pointer == nil {
		t.Fatalf("T-B3 red: missing response must carry pointer (不静默)")
	}
	if response.Pointer.LedgerEntryID != written[0].EntryID {
		t.Fatalf("T-B3 red: pointer ledger_entry_id=%d, want %d", response.Pointer.LedgerEntryID, written[0].EntryID)
	}
	if response.Pointer.TurnID != "turn-3" {
		t.Fatalf("T-B3 red: pointer turn_id=%q, want turn-3 (LastSeenTurn 直通)", response.Pointer.TurnID)
	}
	if !strings.Contains(response.Note, fmt.Sprintf("#%d", written[0].EntryID)) || !strings.Contains(response.Note, "turn-3") {
		t.Fatalf("T-B3 red: note must guide to the ledger row non-silently: %q", response.Note)
	}
}

// TestTB3RepullMissingHonestWithoutLedger：无账本兜底时 pointer 字段诚实
// 归零 + Note 明示「结论未留」——不臆造指引。
func TestTB3RepullMissingHonestWithoutLedger(t *testing.T) {
	ctx := context.Background()
	tree := tbNewTree(t)
	tree.writeTicket(t, "obs_tb3b")
	engine := tbEngine(t, tree)
	ref := tbFirstDomRef(t, engine)
	if err := os.Remove(tree.ticket); err != nil {
		t.Fatalf("remove ticket: %v", err)
	}
	if err := engine.Sync(ctx); err != nil {
		t.Fatalf("engine resync: %v", err)
	}
	service := &RepullService{Engine: engine}
	response := service.Repull(ctx, ref)
	if response.State != RepullMissing || response.Pointer == nil {
		t.Fatalf("missing response must still carry pointer struct: %+v", response)
	}
	if response.Pointer.LedgerEntryID != 0 || !strings.Contains(response.Note, "结论未留") {
		t.Fatalf("no-ledger case must be honest (entry 0 + explicit note): %+v", response)
	}
}

// TestRepullOpaqueAndLegacyNotSilent：opaque/legacy/malformed 输入的重拉
// 响应不静默不猜（missing + 说明注记）。
func TestRepullOpaqueAndLegacyNotSilent(t *testing.T) {
	ctx := context.Background()
	tree := tbNewTree(t)
	tree.writeTicket(t, "obs_tb4")
	engine := tbEngine(t, tree)
	service := &RepullService{Engine: engine}

	for _, raw := range []string{
		"weird://not-registered",
		"dom_0123456789abcdef",
		"vit://mom/broken",
	} {
		response := service.Repull(ctx, raw)
		if response.State != RepullMissing {
			t.Fatalf("%s: state=%s, want missing (不在结构化重拉面)", raw, response.State)
		}
		if response.Note == "" {
			t.Fatalf("%s: missing response must not be silent", raw)
		}
		if strings.Contains(response.Note, "entry #") {
			t.Fatalf("%s: no ledger wired — note must not fabricate guidance", raw)
		}
	}
}

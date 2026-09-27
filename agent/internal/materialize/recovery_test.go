package materialize

// recovery_test.go — MAT-D 红先行（docs/MATERIALIZATION_V1_DESIGN.md §5.5：
// 崩溃恢复+乱序收敛；卡 coord/cards/todo/2026-09-28-MAT-D.md）。
//
// 崩溃恢复三测（红：Metrics 恢复记账字段未定义=编译红，语义见各测注释）：
//
//	TestManifestHalfWriteSalvagesCompleteRows   manifest 半写（截断 JSON）→ 流式
//	                                           抢救完整行+截断记账，不砖死启动
//	TestManifestCorruptRowsSkippedWithAccounting 损坏行（非法 ref/坐标重复）→
//	                                           跳过+显式记账（不静默丢行也不整库拒载）
//	TestOpenStoreEmptyDirIsExplicitError        空目录 → 显式 ErrNoManifestDir
//	                                           （不得静默落到 CWD 相对路径读清单）
//
// 乱序收敛两测（锁位：MAT-C TestDuplicateAndReorderedEventsConverge 已覆盖
// "revision 门吞掉旧 delta 后的两轨收敛"，本组补事件级乱序窗口——store 不持
// revision 门，晚到旧通知直接到达 Handle* 入口时的安全性与收敛性）：
//
//	TestLateOldEventDoesNotRollBackGeneration   晚到旧事件不回退代际（宁多标重
//	                                           标脏可见，重算后收敛回一致内容）
//	TestEventLevelReorderingWindowConverges     事件级乱序窗口：invalidate 收据与
//	                                           arrive 遥测四条投递序（正/反×
//	                                           批量/夹重算）收敛态全等
//
// 语义裁定申报（相对 MAT-A 的 fail-loud 细化，MAT-D 卡面采信）：manifest 是
// 可重建的缓存索引（§2.1：内存索引可从 manifest+CAS 全量重建；行丢失=lazy
// 重算回补，无正确性损失），因此损坏走"抢救+跳过+记账"而非整库拒绝——
// 记账（Metrics.RecoverySkippedRows / RecoverySalvaged）使损失可见，非静默。

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/shadow"
)

// ---------------------------------------------------------------------------
// 崩溃恢复（§5.5）
// ---------------------------------------------------------------------------

// TestManifestHalfWriteSalvagesCompleteRows：SaveManifest 的 tmp+rename 使最终
// 文件要么旧要么新，但磁盘损坏/外部截断仍可产生半写文件。期望：OpenStore 流式
// 抢救截断点之前已完整落盘的行（全部降级 material_reuse），截断经
// Metrics.RecoverySalvaged 记账可见，抢救后的库可继续提交与再落盘。
// 顺带锁定：遗留 .manifest-*.tmp 碎片（保存中断残骸）不影响装载。
func TestManifestHalfWriteSalvagesCompleteRows(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	mustUpsert(t, s,
		Row{Ref: testRef("dom", "T3", "current", hashN(1)), Payload: map[string]any{"peak": -3.0}},
		Row{Ref: testRef("dom", "T7", "current", hashN(1)), Payload: map[string]any{"peak": -4.5}},
		Row{Ref: testRef("tom", "proj", "current", hashN(3)), Payload: map[string]any{"tracks": 2}},
	)
	if _, err := s.MarkStale(RowMatch{Kinds: []string{"dom"}, ScopeValues: []string{"T7"}}, "chg-halfwrite"); err != nil {
		t.Fatalf("MarkStale: %v", err)
	}
	gen := s.Generation()
	if err := s.SaveManifest(); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}

	manifestPath := filepath.Join(dir, manifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	// 截断点=最后一个 "ref" 键起点：行序 rowKey 字典序（dom/T3、dom/T7、tom/proj），
	// 截断后 dom/T3 与 dom/T7 完整、tom/proj 半写丢失（>0 行抢救且<3 行，证明
	// 截断真的发生了）。
	cut := bytes.LastIndex(data, []byte(`"ref"`))
	if cut <= 0 {
		t.Fatalf("manifest 缺少 ref 键，无法构造半写")
	}
	if err := os.WriteFile(manifestPath, data[:cut], 0o644); err != nil {
		t.Fatalf("truncate manifest: %v", err)
	}
	// 保存中断的 tmp 残骸：装载必须无视。
	if err := os.WriteFile(filepath.Join(dir, ".manifest-crash.tmp"), []byte(`{"partial`), 0o644); err != nil {
		t.Fatal(err)
	}

	s2, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("半写 manifest 应抢救恢复而非砖死启动: %v", err)
	}
	m := s2.Metrics()
	if !m.RecoverySalvaged {
		t.Fatalf("半写恢复应置 RecoverySalvaged 记账（可见性），got %+v", m)
	}
	view := snapshotRows(t, s2)
	if len(view) < 1 || len(view) >= 3 {
		t.Fatalf("半写抢救应保留截断点前的完整行（1..2 行），got %d 行: %s", len(view), matCFormatRows(view))
	}
	for _, row := range view {
		if row.Freshness != agentprotocol.FreshnessMaterialReuse {
			t.Fatalf("抢救行应降级 material_reuse（§2.1），%s/%s got %q", row.Ref.Kind, row.Ref.ScopeValue, row.Freshness)
		}
	}
	if s2.Generation() != gen {
		t.Fatalf("半写截断在 rows 段，header 应完整抢救 generation：%d（want %d）", s2.Generation(), gen)
	}

	// 抢救后的库可继续提交并再次落盘（rename 覆盖写修复半写文件）。
	mustUpsert(t, s2, Row{Ref: testRef("tim", "T1", "obs-2", hashN(9))})
	if s2.Generation() < gen {
		t.Fatalf("抢救后提交不得回退代际：%d < %d", s2.Generation(), gen)
	}
	if err := s2.SaveManifest(); err != nil {
		t.Fatalf("抢救后 SaveManifest: %v", err)
	}
	s3, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("修复后再装载: %v", err)
	}
	if got := len(snapshotRows(t, s3)); got != len(view)+1 {
		t.Fatalf("修复后再装载应含抢救行+新行：%d（want %d）", got, len(view)+1)
	}
	if s3.Metrics().RecoverySalvaged {
		t.Fatalf("修复后的完整 manifest 不应再记截断")
	}
}

// TestManifestCorruptRowsSkippedWithAccounting：整文件可解析但个别行损坏
// （非法 ref 字符串/坐标重复）——期望：好行正常装载，损坏行跳过且经
// Metrics.RecoverySkippedRows 显式记账（跳过≠静默：计数可见、可审计）。
func TestManifestCorruptRowsSkippedWithAccounting(t *testing.T) {
	dir := t.TempDir()
	goodRef, err := agentprotocol.FormatRef(testRef("dom", "T5", "current", hashN(2)))
	if err != nil {
		t.Fatalf("FormatRef: %v", err)
	}
	manifest := fmt.Sprintf(`{
  "schema": 1,
  "generation": 7,
  "saved_at": "2026-09-28T00:00:00Z",
  "rows": [
    {"ref": %q, "payload": {"peak": -1.5}},
    {"ref": "definitely-not-a-ref"},
    {"ref": %q}
  ]
}`, goodRef, goodRef)
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := OpenStore(dir)
	if err != nil {
		t.Fatalf("损坏行应跳过+记账而非整库拒载: %v", err)
	}
	view := snapshotRows(t, s)
	if len(view) != 1 {
		t.Fatalf("应只装载 1 条完好行，got %d: %s", len(view), matCFormatRows(view))
	}
	if view[0].Ref.ScopeValue != "T5" || view[0].Freshness != agentprotocol.FreshnessMaterialReuse {
		t.Fatalf("完好行坐标/降级不符：%+v", view[0])
	}
	if got := s.Metrics().RecoverySkippedRows; got != 2 {
		t.Fatalf("两条损坏行（非法 ref+坐标重复）应显式记账 RecoverySkippedRows=2，got %d", got)
	}
	if s.Metrics().RecoverySalvaged {
		t.Fatalf("整文件可解析，不应记半写截断")
	}
	if s.Generation() != 7 {
		t.Fatalf("generation 应从 manifest 还原：got %d want 7", s.Generation())
	}
}

// TestOpenStoreEmptyDirIsExplicitError：OpenStore("") 不得退化为 CWD 相对路径
// 读 manifest.json（静默挂载进程工作目录是越权读），必须显式报
// ErrNoManifestDir（与 NewStore 内存库/SavedManifest 的既有显式边界对齐）。
func TestOpenStoreEmptyDirIsExplicitError(t *testing.T) {
	if _, err := OpenStore(""); !errors.Is(err, ErrNoManifestDir) {
		t.Fatalf("OpenStore 空目录应显式报 ErrNoManifestDir，got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 乱序收敛（§5.5 事件级乱序窗口；MAT-C revision 门之外的补充）
// ---------------------------------------------------------------------------

// TestLateOldEventDoesNotRollBackGeneration：晚到的旧事件（基于旧 revision 的
// track.level 收据，在较新事件已重算收敛之后才到达）——期望：
//  1. 代际单调不回退（每笔有状态变化的提交恰 +1，晚到事件至多再推进）；
//  2. 宁多标可见：受影响行被重标 stale（over-mark 是安全方向，§4.2）；
//  3. 重算后收敛回晚到事件前的内容（DepInputs 反映工程终态，旧通知不产
//     旧内容——收敛性由"重算读当前输入"保证）。
func TestLateOldEventDoesNotRollBackGeneration(t *testing.T) {
	p := newMatCSynthProject()
	s := NewStore()
	if err := RegisterDefaultAdapters(s, nil); err != nil {
		t.Fatalf("RegisterDefaultAdapters: %v", err)
	}
	runIncrementalStep(t, s, p.DepInputs())
	assertNonTrivialView(t, s)

	// 较新事件：加轨（project.structure 全曲域）→ 重算收敛。
	s.HandleReceipt(p.applyAddTrack("T9", "bass"))
	runIncrementalStep(t, s, p.DepInputs())
	genHigh := s.Generation()
	converged := snapshotRows(t, s)
	if len(converged) == 0 {
		t.Fatal("收敛基线不应为空")
	}

	// 晚到的旧事件：store 无 revision 门（门在 shadow 侧，MAT-C 已测），旧收据
	// 直接到达 HandleReceipt。
	late := shadow.ChangeReceipt{
		ChangeID:        "late-old-delta-001",
		AffectedScopes:  []string{"track.level"},
		ChangedEntities: []shadow.ChangeEntity{{Kind: "track", ID: "T3"}},
	}
	s.HandleReceipt(late)
	if s.Generation() < genHigh {
		t.Fatalf("晚到旧事件回退了代际：%d < %d", s.Generation(), genHigh)
	}
	staleView := snapshotRows(t, s)
	staleHit := false
	for _, row := range staleView {
		if row.Ref.Kind == "dom" && row.Ref.ScopeValue == "T3" {
			if row.Freshness == agentprotocol.FreshnessStale {
				staleHit = true
			} else {
				t.Fatalf("晚到旧事件后 dom/T3 行应为 stale（宁多标可见），got %q", row.Freshness)
			}
		}
	}
	if !staleHit {
		t.Fatalf("晚到旧事件未产生任何 dom/T3 stale 行（脏传播未发生）: %s", matCFormatRows(staleView))
	}

	runIncrementalStep(t, s, p.DepInputs())
	if s.Generation() < genHigh {
		t.Fatalf("晚到旧事件重算后回退了代际：%d < %d", s.Generation(), genHigh)
	}
	after := snapshotRows(t, s)
	if len(after) != len(converged) {
		t.Fatalf("晚到旧事件重算后行数漂移：%d（基线 %d）\nafter=%s\nbase=%s", len(after), len(converged), matCFormatRows(after), matCFormatRows(converged))
	}
	for i := range after {
		if !reflect.DeepEqual(after[i], converged[i]) {
			t.Fatalf("晚到旧事件重算后未收敛回一致内容（第 %d 行）\nbase=%+v\nafter=%+v", i, converged[i], after[i])
		}
	}
}

// TestEventLevelReorderingWindowConverges：事件级乱序窗口——同一组事件
// （invalidate 收据：T3 gain 变更；arrive 遥测：T7 波形到达）在四条投递序下
// （正序/乱序 × 批量投递/事件间夹一次 lazy 重算）终态视图必须全等，且各腿
// 内代际全程单调。MAT-C 的乱序测有 revision 门在 shadow 侧吞旧 delta；本测
// 把两种通知不经任何门直接按不同顺序喂给 store 入口（到达序不可假设）。
func TestEventLevelReorderingWindowConverges(t *testing.T) {
	type leg struct {
		name       string
		reversed   bool // false=T3 收据先投，true=T7 遥测先投
		interleave bool // true=两事件之间插入一次 lazy 重算
	}
	legs := []leg{
		{"forward_batched", false, false},
		{"forward_interleaved", false, true},
		{"reordered_batched", true, false},
		{"reordered_interleaved", true, true},
	}
	views := map[string][]agentprotocol.MaterializedRow{}
	for _, lg := range legs {
		p := newMatCSynthProject()
		s := NewStore()
		if err := RegisterDefaultAdapters(s, nil); err != nil {
			t.Fatalf("RegisterDefaultAdapters: %v", err)
		}
		runIncrementalStep(t, s, p.DepInputs()) // 基线物化（各腿同种子）

		gens := []int64{s.Generation()}
		gainStep := func() {
			s.HandleReceipt(p.applyGainChange("T3", -9.0))
			gens = append(gens, s.Generation())
		}
		waveStep := func() {
			s.HandleFeatureArrival(p.applyWaveformArrival("T7", "wf-1"))
			gens = append(gens, s.Generation())
		}
		first, second := gainStep, waveStep
		if lg.reversed {
			first, second = waveStep, gainStep
		}
		first()
		if lg.interleave {
			runIncrementalStep(t, s, p.DepInputs())
			gens = append(gens, s.Generation())
		}
		second()
		runIncrementalStep(t, s, p.DepInputs())
		gens = append(gens, s.Generation())

		for i := 1; i < len(gens); i++ {
			if gens[i] < gens[i-1] {
				t.Fatalf("%s：第 %d 步代际回退 %d < %d", lg.name, i, gens[i], gens[i-1])
			}
		}
		views[lg.name] = snapshotRows(t, s)
	}

	base := legs[0].name
	for _, kind := range []string{"tom", "acp", "dom"} {
		hit := false
		for _, row := range views[base] {
			if row.Ref.Kind == kind {
				hit = true
				break
			}
		}
		if !hit {
			t.Fatalf("基线腿缺 %s 行（收敛比较空洞）: %s", kind, matCFormatRows(views[base]))
		}
	}
	for _, lg := range legs[1:] {
		if len(views[lg.name]) != len(views[base]) {
			t.Fatalf("%s 行数分歧：%d（基线 %d）\n%s\nbase=%s", lg.name, len(views[lg.name]), len(views[base]), matCFormatRows(views[lg.name]), matCFormatRows(views[base]))
		}
		for i := range views[lg.name] {
			if !reflect.DeepEqual(views[lg.name][i], views[base][i]) {
				t.Fatalf("%s 第 %d 行与基线分歧（乱序窗口未收敛）\nbase=%+v\nleg =%+v", lg.name, i, views[base][i], views[lg.name][i])
			}
		}
	}
}

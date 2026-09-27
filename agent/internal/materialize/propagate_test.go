package materialize

// propagate_test.go — MAT-B 红先行（G2-A 四测试+表 B 锁定，先于实现落库）。
//
// 规格权威：docs/MATERIALIZATION_V1_DESIGN.md §3.1（三挂点+Notifier）/§4.2（表 B
// 域→kind 初值+宁多勿漏）/§4.3（传播算法：收据→表 A scopes→表 B kinds→行坐标
// 匹配→MarkStale）/§5.2（G2-A 四测试，签名级草稿）。
//
// 锁定的契约（卡面验收线）：
//
//	G2-A① 级联          变更只指向 A 输入域，B 行（声明依赖 A 输出域）也收
//	                     marked_stale——级联经表 B 传播（§5.2 合成链）。
//	G2-A② scope 精确    track T3 的 acoustic.l3 事件只标脏 scope(track,T3) 行
//	                     不碰 T7；mix.* 全曲域行全命中。
//	G2-A③ 宁多勿漏      未知域/空 scopes → 全部行标脏（含登记型 kind）。
//	G2-A④ stale 不升级  读路径（SnapshotView/Resolve）永不把 stale 行升回
//	                     current；回到 current 的唯一路径=写侧 Upsert 提交。
//	表 B 锁定            生产表 B 逐条目锁定设计 §4.2 初值；kind ⊆ refschema
//	                     注册表；表 A 词表（shadow addChangeScopes 输出集）全覆盖。
//	分类                 #9 收编（l2_render_probe_ready→feature.l2_render_probe）/
//	                     L3 三族→acoustic.l3.<family>/render 事件不经 arrive 路径
//	                     （hook 3 专职）/未知事件不动作/HandleRenderJob 全 kind 兜底。

import (
	"context"
	"sort"
	"testing"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/shadow"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func projectRef(kind, scopeValue, hash string) agentprotocol.Ref {
	ref := testRef(kind, scopeValue, "obs-1", hash)
	ref.ScopeKind = "project"
	return ref
}

func currentRows(t *testing.T, s *Store, refs ...agentprotocol.Ref) {
	t.Helper()
	rows := make([]Row, 0, len(refs))
	for _, ref := range refs {
		rows = append(rows, Row{Ref: ref, Payload: map[string]any{"seed": true}, Handle: ""})
	}
	mustUpsert(t, s, rows...)
}

func freshnessByCoordinate(t *testing.T, s *Store) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, row := range snapshotRows(t, s) {
		out[row.Ref.Kind+"/"+row.Ref.ScopeKind+"/"+row.Ref.ScopeValue] = row.Freshness
	}
	return out
}

func assertFreshness(t *testing.T, got map[string]string, wantKindScope, wantFreshness string) {
	t.Helper()
	gotFreshness, ok := got[wantKindScope]
	if !ok {
		t.Fatalf("行缺失：%s（freshness 表=%v）", wantKindScope, got)
	}
	if gotFreshness != wantFreshness {
		t.Fatalf("%s freshness=%s want=%s", wantKindScope, gotFreshness, wantFreshness)
	}
}

func trackEntity(id string) shadow.ChangeEntity {
	return shadow.ChangeEntity{Kind: "track", ID: id}
}

// ---------------------------------------------------------------------------
// G2-A① 多级链级联失效（设计 §5.2）
// ---------------------------------------------------------------------------

// TestInvalidationCascadesThroughMultiLevelChain 锁定：根输入域 invalidate 经表 B
// 多级传播——A 的输入域变更不仅标脏 A，还级联标脏声明依赖 A 输出域的 B（§4.3
// 传播算法对 kind 输出域的传递闭包）。合成链用注册 kind（dom/tim，Upsert 注册表
// 门所限）+测试表条目表达（生产表 B 无 kind 输出域条目——§4.1 E1/E2 v1 不进图，
// 机制由本测锁定，边由 MAT-C+ 按需登记）。
func TestInvalidationCascadesThroughMultiLevelChain(t *testing.T) {
	s := newStoreWithTableB([]TableBEntry{
		{Domain: "test.root", Kinds: []string{"dom"}},  // 根输入域 → A（dom 当 TestKindA 位）
		{Domain: "dom.output", Kinds: []string{"tim"}}, // A 输出域 → B（tim 声明依赖）
	})
	currentRows(t, s,
		testRef("dom", "T3", "obs-1", hashN(1)),
		testRef("tim", "T3", "obs-1", hashN(2)),
	)
	ch, cancel, err := s.Subscribe(context.Background())
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer cancel()

	// 变更只指向 A（dom）的输入域；B（tim）行也必须收到 marked_stale——级联经表 B 传播。
	s.HandleReceipt(shadow.ChangeReceipt{
		ChangeID:        "shadow_change_000001",
		AffectedScopes:  []string{"test.root"},
		ChangedEntities: []shadow.ChangeEntity{trackEntity("T3")},
	})

	events := drainEvents(t, ch, 2)
	gotOps := map[string]agentprotocol.MaterializedOp{}
	for _, ev := range events {
		if ev.Op != agentprotocol.MaterializedOpMarkedStale {
			t.Fatalf("期望 marked_stale，得到 op=%s kind=%s", ev.Op, ev.Row.Ref.Kind)
		}
		gotOps[ev.Row.Ref.Kind+"/"+ev.Row.Ref.ScopeValue] = ev.Op
	}
	for _, want := range []string{"dom/T3", "tim/T3"} {
		if _, ok := gotOps[want]; !ok {
			t.Fatalf("级联未命中 %s（收到=%v）", want, gotOps)
		}
	}
	stale := freshnessByCoordinate(t, s)
	assertFreshness(t, stale, "dom/track/T3", agentprotocol.FreshnessStale)
	assertFreshness(t, stale, "tim/track/T3", agentprotocol.FreshnessStale)

	// ② 触发 B 重算（写侧提交点=Upsert，重算适配归 MAT-C）→ Metrics.Recomputes 前进。
	before := s.Metrics().PerKind["tim"].Recomputes
	if err := s.Upsert([]Row{{Ref: testRef("tim", "T3", "obs-1", hashN(3)), Payload: map[string]any{"seed": true}}}); err != nil {
		t.Fatalf("重算提交 Upsert: %v", err)
	}
	after := s.Metrics().PerKind["tim"].Recomputes
	if after != before+1 {
		t.Fatalf("Recomputes[tim] %d→%d，期望 +1（重算确已发生）", before, after)
	}

	// ③ 重算后 B 行 Hash 变化、回到 current（stale→current 唯一路径=写侧提交）。
	view := snapshotRows(t, s)
	row, ok := rowByScope(view, "tim", "T3")
	if !ok {
		t.Fatalf("tim/T3 行缺失")
	}
	if row.Ref.Hash != hashN(3) {
		t.Fatalf("重算后 Hash=%s want=%s（内容身份未更新）", row.Ref.Hash, hashN(3))
	}
	if row.Freshness != agentprotocol.FreshnessCurrent {
		t.Fatalf("重算提交后 freshness=%s want=current", row.Freshness)
	}
}

// ---------------------------------------------------------------------------
// G2-A② scope 精确（设计 §5.2）
// ---------------------------------------------------------------------------

// TestInvalidationScopePrecision 锁定：track 级事件只标脏该 track 坐标行（#6 只标
// scope(track,T3) 不碰 T7）；mix.* 全曲域行全命中；域未命中的 kind 不动。
func TestInvalidationScopePrecision(t *testing.T) {
	seedRows := []agentprotocol.Ref{
		testRef("dom", "T3", "obs-1", hashN(1)),
		testRef("dom", "T7", "obs-1", hashN(2)),
		testRef("tim", "T3", "obs-1", hashN(3)),
		testRef("tim", "T7", "obs-1", hashN(4)),
		testRef("mom", "T3", "obs-1", hashN(5)),
		testRef("mom", "T7", "obs-1", hashN(6)),
		testRef("acp", "T3", "rev-abcd0001", hashN(7)),
		testRef("rlm", "T3", "obs-1", hashN(8)),
		testRef("fxm", "T3", "obs-1", hashN(9)),
	}

	t.Run("acoustic_l3_arrival_track_scoped", func(t *testing.T) {
		s := NewStore()
		currentRows(t, s, seedRows...)
		s.HandleFeatureArrival(map[string]any{
			"command":      "audio_feature_data_ready",
			"feature_type": "band_energy_summary",
			"track_id":     "T3",
			"clip_id":      "C1",
			"status":       "ready",
		})
		got := freshnessByCoordinate(t, s)
		// 表 B acoustic.l3.band_energy → mom/acp/dom/tim，仅 scope(track,T3)。
		for _, hit := range []string{"dom/track/T3", "tim/track/T3", "mom/track/T3", "acp/track/T3"} {
			assertFreshness(t, got, hit, agentprotocol.FreshnessStale)
		}
		// T7 不碰；域未命中的 rlm/fxm 不动（§4.2 acoustic.l3 行不含 rlm/fxm）。
		for _, miss := range []string{"dom/track/T7", "tim/track/T7", "mom/track/T7", "rlm/track/T3", "fxm/track/T3"} {
			assertFreshness(t, got, miss, agentprotocol.FreshnessCurrent)
		}
		// arrive 计数（§5.1 Arrivals：事件级到达计数，按受影响 kind）。
		metrics := s.Metrics().PerKind
		for _, kind := range []string{"dom", "tim", "mom", "acp"} {
			if metrics[kind].Arrivals != 1 {
				t.Fatalf("Arrivals[%s]=%d want=1", kind, metrics[kind].Arrivals)
			}
		}
		if metrics["rlm"].Arrivals != 0 || metrics["fxm"].Arrivals != 0 {
			t.Fatalf("域未命中 kind 不应计 Arrivals（rlm=%d fxm=%d）", metrics["rlm"].Arrivals, metrics["fxm"].Arrivals)
		}
	})

	t.Run("receipt_mix_domain_marks_all_rows", func(t *testing.T) {
		s := NewStore()
		currentRows(t, s, seedRows...)
		s.HandleReceipt(shadow.ChangeReceipt{
			ChangeID:        "shadow_change_000002",
			AffectedScopes:  []string{"track.level", "mix.multitrack_relationship"},
			ChangedEntities: []shadow.ChangeEntity{trackEntity("T3")},
		})
		got := freshnessByCoordinate(t, s)
		// track.level → dom/mom/tim/rlm 仅 T3；mix.multitrack_relationship → mom 全曲域全命中。
		for _, hit := range []string{"dom/track/T3", "tim/track/T3", "mom/track/T3", "rlm/track/T3", "mom/track/T7"} {
			assertFreshness(t, got, hit, agentprotocol.FreshnessStale)
		}
		for _, miss := range []string{"dom/track/T7", "tim/track/T7", "acp/track/T3", "fxm/track/T3"} {
			assertFreshness(t, got, miss, agentprotocol.FreshnessCurrent)
		}
	})
}

// ---------------------------------------------------------------------------
// G2-A③ 宁多勿漏（设计 §4.2/§5.2）
// ---------------------------------------------------------------------------

// TestUnknownScopeFailsLoudAllDirty 锁定：未知域/空 scopes → 全部行标脏（含登记型
// kind——project.state 兜底行两列：全部 precomputable+全部登记行）。失效正确性的
// 第一敌人是漏标，过标只是浪费一次重算。
func TestUnknownScopeFailsLoudAllDirty(t *testing.T) {
	seedRows := []agentprotocol.Ref{
		testRef("dom", "T3", "obs-1", hashN(1)),
		testRef("mom", "T3", "obs-1", hashN(2)),
		testRef("tim", "T3", "obs-1", hashN(3)),
		testRef("rlm", "T3", "obs-1", hashN(4)), // 登记型（半物化）也标脏
		testRef("fxm", "T3", "obs-1", hashN(5)), // 登记型
		projectRef("com", "current", hashN(6)),  // 登记型
	}

	t.Run("empty_scopes", func(t *testing.T) {
		s := NewStore()
		currentRows(t, s, seedRows...)
		s.HandleReceipt(shadow.ChangeReceipt{ChangeID: "shadow_change_000003"})
		got := freshnessByCoordinate(t, s)
		for _, coord := range []string{"dom/track/T3", "mom/track/T3", "tim/track/T3", "rlm/track/T3", "fxm/track/T3", "com/project/current"} {
			assertFreshness(t, got, coord, agentprotocol.FreshnessStale)
		}
	})

	t.Run("unknown_scope", func(t *testing.T) {
		s := NewStore()
		currentRows(t, s, seedRows...)
		s.HandleReceipt(shadow.ChangeReceipt{
			ChangeID:       "shadow_change_000004",
			AffectedScopes: []string{"totally.unknown.domain"},
		})
		got := freshnessByCoordinate(t, s)
		for _, coord := range []string{"dom/track/T3", "mom/track/T3", "tim/track/T3", "rlm/track/T3", "fxm/track/T3", "com/project/current"} {
			assertFreshness(t, got, coord, agentprotocol.FreshnessStale)
		}
	})

	t.Run("track_scoped_domain_without_entities_overmarks", func(t *testing.T) {
		// track.* 域但收据不带 track 实体 → 无法收窄 → 该域 kind 全行标脏（宁多勿漏）。
		s := NewStore()
		currentRows(t, s, seedRows...)
		s.HandleReceipt(shadow.ChangeReceipt{
			ChangeID:       "shadow_change_000005",
			AffectedScopes: []string{"track.level"},
		})
		got := freshnessByCoordinate(t, s)
		for _, coord := range []string{"dom/track/T3", "mom/track/T3", "tim/track/T3", "rlm/track/T3"} {
			assertFreshness(t, got, coord, agentprotocol.FreshnessStale)
		}
		// 域未命中 kind 不动（track.level 不含 fxm/com/acp）。
		assertFreshness(t, got, "fxm/track/T3", agentprotocol.FreshnessCurrent)
		assertFreshness(t, got, "com/project/current", agentprotocol.FreshnessCurrent)
	})
}

// ---------------------------------------------------------------------------
// G2-A④ stale 不删行不升级（设计 §5.2）
// ---------------------------------------------------------------------------

// TestStaleNotDeletedNotUpgradedViaReceiptPropagation 锁定 G2-A④（设计 §5.2 签名
// 名 TestStaleNotDeletedNotUpgraded 已被 MAT-A T1 用于 MarkStale 原语级锁定，本测
// 是传播入口级同语义）：HandleReceipt 标脏后行仍在、Freshness=stale；读路径
// （SnapshotView/Resolve）永不把 stale 行升回 current；回到 current 的唯一路径=
// 写侧 Upsert 提交（对齐 QUERY_ENGINE T6 与 M2 旧行保留审计）。
func TestStaleNotDeletedNotUpgradedViaReceiptPropagation(t *testing.T) {
	s := NewStore()
	currentRows(t, s,
		testRef("dom", "T3", "obs-1", hashN(1)),
		testRef("dom", "T7", "obs-1", hashN(2)),
	)
	s.HandleReceipt(shadow.ChangeReceipt{
		ChangeID:        "shadow_change_000006",
		AffectedScopes:  []string{"track.level"},
		ChangedEntities: []shadow.ChangeEntity{trackEntity("T3")},
	})

	// 不删行：T3 行仍在且 stale；未命中 T7 保持 current。
	got := freshnessByCoordinate(t, s)
	assertFreshness(t, got, "dom/track/T3", agentprotocol.FreshnessStale)
	assertFreshness(t, got, "dom/track/T7", agentprotocol.FreshnessCurrent)

	// 读路径反复读不升级：SnapshotView×3 + Resolve。
	for i := 0; i < 3; i++ {
		got = freshnessByCoordinate(t, s)
		assertFreshness(t, got, "dom/track/T3", agentprotocol.FreshnessStale)
	}
	resolved, err := s.Resolve(context.Background(), testRef("dom", "T3", "obs-1", hashN(1)))
	if err != nil {
		t.Fatalf("Resolve stale 行应命中（不删行）: %v", err)
	}
	if resolved.Freshness != agentprotocol.FreshnessStale {
		t.Fatalf("Resolve freshness=%s want=stale", resolved.Freshness)
	}
	got = freshnessByCoordinate(t, s)
	assertFreshness(t, got, "dom/track/T3", agentprotocol.FreshnessStale)

	// 唯一回 current 路径=写侧 Upsert（重算提交）。
	if err := s.Upsert([]Row{{Ref: testRef("dom", "T3", "obs-1", hashN(9)), Payload: map[string]any{"seed": true}}}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got = freshnessByCoordinate(t, s)
	assertFreshness(t, got, "dom/track/T3", agentprotocol.FreshnessCurrent)
}

// ---------------------------------------------------------------------------
// 表 B 锁定（设计 §4.2，数据表+表驱动测试）
// ---------------------------------------------------------------------------

// TestTableBInitialValuesLocked 逐条目锁定生产表 B 初值（设计 §4.2 表+arrive 域）。
// 修改任一条目（漏域/漏 kind/错 kind）必须改本测——表驱动锁定的意义就在于此。
func TestTableBInitialValuesLocked(t *testing.T) {
	want := map[string][]string{
		// invalidate 域（表 A 词表，§4.2 主表）
		"track.level":                     {"dom", "mom", "tim", "rlm"}, // rlm=登记（半物化，只登记不重算 §4.5）
		"track.stereo_space":              {"dom", "mom"},
		"track.basic_energy":              {"dom", "tim"},
		"track.time_dynamics":             {"dom", "tim"},
		"track.timbre_frequency":          {"dom", "tim"},
		"mix.multitrack_relationship":     {"mom"},
		"mix.frequency_relationship":      {"mom"},
		"mix.masking_relationship":        {"mom"},
		"project.headroom":                {"dom", "rlm"},
		"project.structure":               {"tom", "tim", "epm"},
		"processor.identity_and_controls": {"com"},
		"processor.behavior":              {"com"},
		"processor.change_delta":          {"com"},
		"comparison.before_after":         {"fxm"},
		// arrive 域（§4.2 末段：#4/#5→feature.*；#6-#8→acoustic.l3.<family>）
		"feature.waveform":            {"dom", "tim"},
		"feature.spectral":            {"dom", "tim"},
		"acoustic.l3.band_energy":     {"mom", "acp", "dom", "tim"},
		"acoustic.l3.stereo_relation": {"mom", "acp", "dom", "tim"},
		"acoustic.l3.loudness":        {"mom", "acp", "dom", "tim"},
		// #9 收编（§2.2 批 4）：probe 证据到达（masking 帧→mom；轨道级测量→dom/tim）
		"feature.l2_render_probe": {"mom", "dom", "tim"},
	}

	entries := TableBEntries()
	if len(entries) != len(want) {
		t.Fatalf("表 B 条目数=%d want=%d（entries=%v）", len(entries), len(want), entries)
	}
	got := map[string][]string{}
	for _, entry := range entries {
		kinds := append([]string(nil), entry.Kinds...)
		sort.Strings(kinds)
		got[entry.Domain] = kinds
	}
	for domain, wantKinds := range want {
		gotKinds, ok := got[domain]
		if !ok {
			t.Fatalf("表 B 缺域 %q（got=%v）", domain, got)
		}
		wantSorted := append([]string(nil), wantKinds...)
		sort.Strings(wantSorted)
		if len(gotKinds) != len(wantSorted) {
			t.Fatalf("域 %q kinds=%v want=%v", domain, gotKinds, wantSorted)
		}
		for i := range wantSorted {
			if gotKinds[i] != wantSorted[i] {
				t.Fatalf("域 %q kinds=%v want=%v", domain, gotKinds, wantSorted)
			}
		}
	}

	// 全部 kind ∈ refschema 注册表（§4.1 启动期校验的测试面）。
	registered := map[string]bool{}
	for _, kind := range agentprotocol.RegisteredRefKinds() {
		registered[kind] = true
	}
	for domain, kinds := range got {
		for _, kind := range kinds {
			if !registered[kind] {
				t.Fatalf("域 %q 引用未注册 kind %q", domain, kind)
			}
		}
	}

	// 表 A 词表覆盖（shadow addChangeScopes 输出集，shadow/change.go:200-222 锚）：
	// 每个 invalidate 域要么在表 B，要么是 project.state（兜底路径，算法实现，
	// 锁定于 TestUnknownScopeFailsLoudAllDirty）。
	tableAVocabulary := []string{
		"track.level", "track.stereo_space", "track.basic_energy", "track.time_dynamics",
		"track.timbre_frequency", "mix.multitrack_relationship", "mix.frequency_relationship",
		"mix.masking_relationship", "project.headroom", "project.structure",
		"processor.identity_and_controls", "processor.behavior", "processor.change_delta",
		"comparison.before_after", "project.state",
	}
	for _, domain := range tableAVocabulary {
		if domain == "project.state" {
			continue // 兜底（§4.2 主表 project.state 行=全部 kind 保守标脏，由算法兜底实现）
		}
		if _, ok := got[domain]; !ok {
			t.Fatalf("表 A 域 %q 未进表 B（漏标=静默错误数据）", domain)
		}
	}
	if _, ok := got["project.state"]; ok {
		t.Fatalf("project.state 应由算法兜底实现，不应是表条目（条目化会冻结全 kind 列表）")
	}
}

// ---------------------------------------------------------------------------
// 事件分类（#9 收编/L3 三族/render 专职/未知不动作）
// ---------------------------------------------------------------------------

// TestFeatureArrivalClassification 锁定 arrive 事件分类：#9 l2_render_probe_ready →
// feature.l2_render_probe（mom/dom/tim）；L3 三族 → acoustic.l3.<family>；render 遥测
// 不经 arrive 路径（hook 3 NotifyRenderJob 专职，§3.1）；未知事件不动作（#15-#19
// 排除）；无 track_id 的 arrive 无法收窄 → 宁多勿漏全行。
func TestFeatureArrivalClassification(t *testing.T) {
	t.Run("l2_render_probe_ready_collapse", func(t *testing.T) {
		s := NewStore()
		currentRows(t, s,
			testRef("mom", "T3", "obs-1", hashN(1)),
			testRef("dom", "T3", "obs-1", hashN(2)),
			testRef("tim", "T3", "obs-1", hashN(3)),
			testRef("fxm", "T3", "obs-1", hashN(4)),
		)
		s.HandleFeatureArrival(map[string]any{
			"command":         "l2_render_probe_ready",
			"feature_type":    "l2_render_probe",
			"request_id":      "req-l2-1",
			"track_id":        "T3",
			"tap_point":       "track_post_fader",
			"render_revision": "rr-1",
		})
		got := freshnessByCoordinate(t, s)
		for _, hit := range []string{"mom/track/T3", "dom/track/T3", "tim/track/T3"} {
			assertFreshness(t, got, hit, agentprotocol.FreshnessStale)
		}
		assertFreshness(t, got, "fxm/track/T3", agentprotocol.FreshnessCurrent)
	})

	t.Run("l3_families_map_to_own_domains", func(t *testing.T) {
		for family, featureType := range map[string]string{
			"band_energy":     "band_energy_summary",
			"stereo_relation": "stereo_relation_summary",
			"loudness":        "loudness_summary",
		} {
			s := NewStore()
			currentRows(t, s,
				testRef("mom", "T9", "obs-1", hashN(1)),
				testRef("acp", "T9", "rev-abcd0001", hashN(2)),
			)
			s.HandleFeatureArrival(map[string]any{
				"command":      "audio_feature_data_ready",
				"feature_type": featureType,
				"track_id":     "T9",
			})
			got := freshnessByCoordinate(t, s)
			for _, hit := range []string{"mom/track/T9", "acp/track/T9"} {
				assertFreshness(t, got, hit, agentprotocol.FreshnessStale)
			}
			_ = family // 域名锁定在 TestTableBInitialValuesLocked（三族各自条目）
		}
	})

	t.Run("render_telemetry_not_via_arrival", func(t *testing.T) {
		s := NewStore()
		currentRows(t, s, testRef("dom", "T3", "obs-1", hashN(1)))
		s.HandleFeatureArrival(map[string]any{
			"topic": "render", "subtopic": "render_done", "job_id": "job-1", "status": "ok",
		})
		got := freshnessByCoordinate(t, s)
		assertFreshness(t, got, "dom/track/T3", agentprotocol.FreshnessCurrent)
	})

	t.Run("unknown_event_no_op", func(t *testing.T) {
		s := NewStore()
		currentRows(t, s, testRef("dom", "T3", "obs-1", hashN(1)))
		s.HandleFeatureArrival(map[string]any{"topic": "levels", "meters": []any{1.0, 2.0}})
		s.HandleFeatureArrival(map[string]any{"command": "something_new", "feature_type": "x"})
		got := freshnessByCoordinate(t, s)
		assertFreshness(t, got, "dom/track/T3", agentprotocol.FreshnessCurrent)
	})

	t.Run("arrival_without_track_overmarks", func(t *testing.T) {
		s := NewStore()
		currentRows(t, s,
			testRef("dom", "T3", "obs-1", hashN(1)),
			testRef("dom", "T7", "obs-1", hashN(2)),
		)
		s.HandleFeatureArrival(map[string]any{
			"command":      "audio_feature_data_ready",
			"feature_type": "band_energy_summary",
		})
		got := freshnessByCoordinate(t, s)
		for _, hit := range []string{"dom/track/T3", "dom/track/T7"} {
			assertFreshness(t, got, hit, agentprotocol.FreshnessStale)
		}
	})
}

// TestHandleRenderJobMarksAllKinds 锁定 hook 3 语义：render_done/render_failed =
// render 换代 → 全 kind 保守标脏（批 3 粗粒度：旧测量整体失真，宁可全标）。
func TestHandleRenderJobMarksAllKinds(t *testing.T) {
	s := NewStore()
	currentRows(t, s,
		testRef("dom", "T3", "obs-1", hashN(1)),
		testRef("mom", "T3", "obs-1", hashN(2)),
		testRef("fxm", "T3", "obs-1", hashN(3)),
		projectRef("com", "current", hashN(4)),
	)
	s.HandleRenderJob("job-render-1", "ready", `D:\tmp\out.wav`)
	got := freshnessByCoordinate(t, s)
	for _, coord := range []string{"dom/track/T3", "mom/track/T3", "fxm/track/T3", "com/project/current"} {
		assertFreshness(t, got, coord, agentprotocol.FreshnessStale)
	}
}

// TestNotifierAdapterWiring 锁定：Store 的 Notifier() 适配器三方法直通三个入口
// （挂点侧持接口，物化层实现，§3.1）。
func TestNotifierAdapterWiring(t *testing.T) {
	s := NewStore()
	currentRows(t, s, testRef("dom", "T3", "obs-1", hashN(1)))
	var n Notifier = s.Notifier()
	n.NotifyShadowChange(shadow.ChangeReceipt{
		ChangeID:        "shadow_change_000007",
		AffectedScopes:  []string{"track.level"},
		ChangedEntities: []shadow.ChangeEntity{trackEntity("T3")},
	})
	got := freshnessByCoordinate(t, s)
	assertFreshness(t, got, "dom/track/T3", agentprotocol.FreshnessStale)

	n.NotifyTelemetry(map[string]any{
		"command": "l2_render_probe_ready", "feature_type": "l2_render_probe", "track_id": "T3",
	})
	n.NotifyRenderJob("job-x", "failed", "")
	got = freshnessByCoordinate(t, s)
	assertFreshness(t, got, "dom/track/T3", agentprotocol.FreshnessStale)
}

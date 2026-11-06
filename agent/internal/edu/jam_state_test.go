package edu

// 乐局状态机单测（乐局 3a）：全旅程推进（以 279 定稿卡为夹具）、attempt 计数、
// suspended fail-closed、restore 旧状态缺省语义/fail-closed、JSON 往返（§11）。

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// fixtureObservation 组装一步推进的观察（真实运行时由读取命令组装；测试
// 直接以卡内 notes 表为轨槽快照，保持纯数据契约）。
func fixtureObservation(card *LevelCard, slot string, notes []Note, events []ObservedEvent, tempoSet bool) Observation {
	return Observation{
		At:       "2026-11-07T12:00:00Z",
		Slots:    map[string][]Note{slot: notes},
		Events:   events,
		TempoSet: tempoSet,
	}
}

func themeNotes(t *testing.T, card *LevelCard) []Note {
	t.Helper()
	notes, ok := card.AssetNotes("theme")
	if !ok {
		t.Fatal("theme notes unresolvable")
	}
	return notes
}

func corruptedNotes(t *testing.T, card *LevelCard) []Note {
	t.Helper()
	notes, ok := card.AssetNotes("theme_corrupted")
	if !ok {
		t.Fatal("corrupted notes unresolvable")
	}
	return notes
}

func checkState(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func newState(t *testing.T, levelID, conv, proj string, nodeIDs []string, at string) JamState {
	t.Helper()
	s, err := NewJamState(levelID, conv, proj, nodeIDs, at)
	checkState(t, err)
	return s
}

func advance(t *testing.T, s JamState, card *LevelCard, obs Observation) JamState {
	t.Helper()
	next, err := AdvanceJam(s, card, obs)
	checkState(t, err)
	return next
}

func nodeState(s *JamState, nodeID string) JamNodeState {
	return s.nodeStateOf(nodeID)
}

func TestNewJamStateValidation(t *testing.T) {
	if _, err := NewJamState("lvl", "", "proj", []string{"n0"}, "t"); err == nil {
		t.Fatal("empty conversation_id accepted, want reject（双绑定必填）")
	}
	if _, err := NewJamState("lvl", "conv", "", []string{"n0"}, "t"); err == nil {
		t.Fatal("empty project_uuid accepted, want reject")
	}
	if _, err := NewJamState("lvl", "conv", "proj", []string{"n0", "n0"}, "t"); err == nil {
		t.Fatal("duplicate node ids accepted, want reject")
	}
	s := newState(t, "lvl", "conv", "proj", []string{"n0", "n1"}, "t0")
	if s.SchemaVersion != JamStateSchemaVersion {
		t.Fatalf("schema = %q", s.SchemaVersion)
	}
	if nodeState(&s, "n0").State != JamNodeActive || nodeState(&s, "n1").State != JamNodePending {
		t.Fatalf("initial states = %+v", s.Nodes)
	}
	if len(s.Events) != 1 || s.Events[0].Kind != JamEventNodeEntered || s.Events[0].Seq != 1 {
		t.Fatalf("initial events = %+v", s.Events)
	}
}

// 全旅程：n0→n6 逐节点推进到完成，含 n1/n3 的失败尝试计数。
func TestAdvanceJamFullJourney(t *testing.T) {
	card := loadFixtureLevel(t)
	nodeIDs := make([]string, len(card.Nodes))
	for i, n := range card.Nodes {
		nodeIDs[i] = n.ID
	}
	s := newState(t, card.ID, "conv-1", "proj-1", nodeIDs, "2026-11-07T10:00:00Z")
	theme := themeNotes(t, card)
	corrupted := corruptedNotes(t, card)

	// n0：clip_exists(melody_imported) + human_replied。
	s = advance(t, s, card, fixtureObservation(card, "melody_imported", theme,
		[]ObservedEvent{{Name: "human_replied"}}, false))
	if nodeState(&s, "n0_photo_score_open").State != JamNodeDone {
		t.Fatal("n0 not done")
	}
	if nodeState(&s, "n1_read_and_fix").State != JamNodeActive {
		t.Fatal("n1 not entered after n0 done")
	}

	// n1：学生导入的是改错旋律 → 判定 fail，attempt 计数。
	s = advance(t, s, card, fixtureObservation(card, "melody_imported", corrupted, nil, false))
	n1 := nodeState(&s, "n1_read_and_fix")
	if n1.State != JamNodeActive || n1.Attempt != 1 {
		t.Fatalf("n1 after corrupted attempt = %+v, want active/attempt=1", n1)
	}
	lastEvent := s.Events[len(s.Events)-1]
	if lastEvent.Kind != JamEventAttemptFailed || lastEvent.NodeID != "n1_read_and_fix" {
		t.Fatalf("last event = %+v, want attempt_failed@n1", lastEvent)
	}
	// 修正后过（theme vs theme）。
	s = advance(t, s, card, fixtureObservation(card, "melody_imported", theme, nil, false))
	if nodeState(&s, "n1_read_and_fix").State != JamNodeDone {
		t.Fatal("n1 not done after fix")
	}

	// n2：user_confirmed(tempo) + tempo_set（无 judge 叶 → 不计 attempt）。
	s = advance(t, s, card, Observation{
		At:       "2026-11-07T10:05:00Z",
		Events:   []ObservedEvent{{Name: "user_confirmed", Key: "tempo"}},
		TempoSet: true,
	})
	if nodeState(&s, "n2_set_tempo").State != JamNodeDone {
		t.Fatal("n2 not done")
	}
	if nodeState(&s, "n2_set_tempo").Attempt != 0 {
		t.Fatal("event-only node counted an attempt")
	}

	// n3：目标轨 melody_corrupted。先喂完好旋律（target=corrupted 轨被修正前
	// 用 theme 数据冒充观察不算教学语义——这里直接用 corrupted→fail→修正→pass
	// 的双步验证 attempt 与判定参照）。
	s = advance(t, s, card, fixtureObservation(card, "melody_corrupted", corrupted, nil, false))
	if n3 := nodeState(&s, "n3_find_wrong_note"); n3.State != JamNodeActive || n3.Attempt != 1 {
		t.Fatalf("n3 = %+v, want active/attempt=1", n3)
	}
	s = advance(t, s, card, fixtureObservation(card, "melody_corrupted", theme, nil, false))
	if nodeState(&s, "n3_find_wrong_note").State != JamNodeDone {
		t.Fatal("n3 not done after fix")
	}

	// n4：user_chose(variation_direction)。
	s = advance(t, s, card, Observation{
		At:     "2026-11-07T10:20:00Z",
		Events: []ObservedEvent{{Name: "user_chose", Key: "variation_direction"}},
	})
	if nodeState(&s, "n4_variation_direction").State != JamNodeDone {
		t.Fatal("n4 not done")
	}

	// n5：all[user_feedback_given min 1, user_confirmed(final)]——缺一件不过。
	s = advance(t, s, card, Observation{
		At:     "2026-11-07T10:30:00Z",
		Events: []ObservedEvent{{Name: "user_feedback_given"}},
	})
	if nodeState(&s, "n5_make_it_ours").State == JamNodeDone {
		t.Fatal("n5 done with only half of all[] conditions")
	}
	s = advance(t, s, card, Observation{
		At:     "2026-11-07T10:35:00Z",
		Events: []ObservedEvent{{Name: "user_feedback_given"}, {Name: "user_confirmed", Key: "final"}},
	})
	if nodeState(&s, "n5_make_it_ours").State != JamNodeDone {
		t.Fatal("n5 not done with both conditions")
	}

	// n6：export_completed → 全部完成。
	s = advance(t, s, card, Observation{
		At:     "2026-11-07T10:40:00Z",
		Events: []ObservedEvent{{Name: "export_completed"}},
	})
	if !s.AllNodesDone(card) {
		t.Fatal("jam not completed after n6")
	}
	// 再推进 → ErrJamCompleted（幂等哨兵）。
	if _, err := AdvanceJam(s, card, Observation{At: "2026-11-07T10:41:00Z"}); !errors.Is(err, ErrJamCompleted) {
		t.Fatalf("advance after completion = %v, want ErrJamCompleted", err)
	}

	// 事件表审计：seq 严格递增、node_entered×7、node_done×7。
	if len(s.Events) < 14 {
		t.Fatalf("events = %d, want ≥14", len(s.Events))
	}
	entered, done := 0, 0
	for i, e := range s.Events {
		if i > 0 && e.Seq <= s.Events[i-1].Seq {
			t.Fatalf("event seq not monotonic at %d: %+v", i, e)
		}
		switch e.Kind {
		case JamEventNodeEntered:
			entered++
		case JamEventNodeDone:
			done++
		}
	}
	if entered != 7 || done != 7 {
		t.Fatalf("entered=%d done=%d, want 7/7", entered, done)
	}
}

func TestAdvanceJamBindingAndSuspended(t *testing.T) {
	card := loadFixtureLevel(t)
	s := newState(t, "other-level", "c", "p", []string{"n0_photo_score_open"}, "t")
	if _, err := AdvanceJam(s, card, Observation{}); err == nil || !strings.Contains(err.Error(), "binding mismatch") {
		t.Fatalf("want binding mismatch, got %v", err)
	}
	// suspended 当前节点：restore 含 suspended 值（枚举占位保留）→ fail-closed。
	s2 := newState(t, card.ID, "c", "p", []string{"n0_photo_score_open"}, "t")
	s2.Nodes[0].State = JamNodeSuspended
	if _, err := AdvanceJam(s2, card, Observation{}); !errors.Is(err, ErrSuspendedNode) {
		t.Fatalf("advance on suspended = %v, want ErrSuspendedNode（挂起接口零实现）", err)
	}
}

func TestAdvanceJamUnobservedTargetIsNotAttempt(t *testing.T) {
	card := loadFixtureLevel(t)
	s := newState(t, card.ID, "c", "p",
		[]string{"n1_read_and_fix"}, "t") // 直接从 n1 起（单节点卡序）
	// 目标轨槽未观察：数据缺口=叶不满足，但不计 attempt。
	s2 := advance(t, s, card, Observation{At: "t", Events: []ObservedEvent{}})
	if n := nodeState(&s2, "n1_read_and_fix"); n.Attempt != 0 {
		t.Fatalf("unobserved target counted attempt: %+v", n)
	}
}

// ---- restore：旧状态缺省语义 + fail-closed + 往返（§11）----

func TestRestoreJamStateRoundTrip(t *testing.T) {
	card := loadFixtureLevel(t)
	s := newState(t, card.ID, "conv-9", "proj-9",
		[]string{"n0_photo_score_open", "n1_read_and_fix"}, "2026-11-07T09:00:00Z")
	s = advance(t, s, card, fixtureObservation(card, "melody_imported", themeNotes(t, card),
		[]ObservedEvent{{Name: "human_replied"}}, false))
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, report, err := RestoreJamState(data, card)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if report != (RestoreReport{}) {
		t.Fatalf("report = %+v, want zero（无丢弃）", report)
	}
	remarshal, _ := json.Marshal(got)
	if !bytes.Equal(data, remarshal) {
		t.Fatalf("roundtrip mismatch:\n%s\n%s", data, remarshal)
	}
}

func TestRestoreJamStateFailClosed(t *testing.T) {
	card := loadFixtureLevel(t)
	// 未知 schema_version → 拒绝（v0 是首版，无旧版本可兼容）。
	bad := `{"schema_version":"vit.jam_state.v9","level_id":"first-run-twinkle","nodes":[]}`
	if _, _, err := RestoreJamState([]byte(bad), card); err == nil || !strings.Contains(err.Error(), "fail-closed") {
		t.Fatalf("unknown schema accepted: %v", err)
	}
	// 空 schema_version → 拒绝。
	empty := `{"level_id":"first-run-twinkle","nodes":[]}`
	if _, _, err := RestoreJamState([]byte(empty), card); err == nil {
		t.Fatal("missing schema accepted")
	}
	// level 绑定不符 → 拒绝。
	mis := `{"schema_version":"vit.jam_state.v0","level_id":"another","nodes":[]}`
	if _, _, err := RestoreJamState([]byte(mis), card); err == nil || !strings.Contains(err.Error(), "level binding") {
		t.Fatalf("level mismatch accepted: %v", err)
	}
}

func TestRestoreJamStateOldDefaultsAndDrops(t *testing.T) {
	card := loadFixtureLevel(t)
	// 旧记录缺省语义：nodes 项缺 attempt/entered_at → 0/""；suspended 原值保留。
	old := `{
	  "schema_version": "vit.jam_state.v0",
	  "level_id": "first-run-twinkle",
	  "conversation_id": "c1", "project_uuid": "p1",
	  "nodes": [
	    {"node_id": "n0_photo_score_open", "state": "done"},
	    {"node_id": "n1_read_and_fix", "state": "done"},
	    {"node_id": "n2_set_tempo", "state": "suspended"},
	    {"node_id": "ghost_node", "state": "active"},
	    {"node_id": "n3_find_wrong_note", "state": "paused"}
	  ],
	  "events": [
	    {"seq": 1, "kind": "node_entered", "node_id": "n0_photo_score_open"},
	    {"seq": 2, "kind": "legacy_event_kind", "node_id": "n0_photo_score_open"}
	  ]
	}`
	s, report, err := RestoreJamState([]byte(old), card)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if report.DroppedUnknownNodes != 1 || report.DroppedUnknownStates != 1 || report.KeptUnknownEvents != 1 {
		t.Fatalf("report = %+v, want 1/1/1（drop 计数，never execute）", report)
	}
	if len(s.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3（ghost 与 paused 丢弃）", len(s.Nodes))
	}
	n1 := nodeState(&s, "n1_read_and_fix")
	if n1.Attempt != 0 || n1.EnteredAt != "" {
		t.Fatalf("old defaults not applied: %+v", n1)
	}
	if nodeState(&s, "n2_set_tempo").State != JamNodeSuspended {
		t.Fatal("suspended 枚举占位值未保留")
	}
	if len(s.Events) != 2 {
		t.Fatalf("audit events = %d, want 2（未知种类保留原文）", len(s.Events))
	}
	// 推进入口：当前节点=suspended（卡序首个非 done）→ fail-closed。
	if _, err := AdvanceJam(s, card, Observation{At: "t"}); !errors.Is(err, ErrSuspendedNode) {
		t.Fatalf("advance into suspended = %v, want ErrSuspendedNode", err)
	}
}

func TestLegalJamTransitionsTable(t *testing.T) {
	// v0 有入口的迁移= pending→active 与 active→done；suspended 双向仅声明
	//（枚举占位，12 月不用，无入口 API——本测试即迁移表文档）。
	want := map[string][]string{
		JamNodePending:   {JamNodeActive},
		JamNodeActive:    {JamNodeDone, JamNodeSuspended},
		JamNodeSuspended: {JamNodeActive},
	}
	for from, tos := range want {
		got := legalJamTransitions[from]
		if len(got) != len(tos) {
			t.Fatalf("transitions[%s] = %v, want %v", from, got, tos)
		}
		for i := range tos {
			if got[i] != tos[i] {
				t.Fatalf("transitions[%s] = %v, want %v", from, got, tos)
			}
		}
	}
	if _, ok := legalJamTransitions[JamNodeDone]; ok {
		t.Fatal("done must be terminal")
	}
}

func TestEvaluateNodeConditionsEventKeyFiltering(t *testing.T) {
	card := loadFixtureLevel(t)
	// n2 的 user_confirmed 须带 key=tempo：同事件错 key 不满足。
	ev, err := EvaluateNodeConditions(card, "n2_set_tempo", Observation{
		At:       "t",
		TempoSet: true,
		Events:   []ObservedEvent{{Name: "user_confirmed", Key: "variation_direction"}},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if ev.Satisfied {
		t.Fatal("user_confirmed with wrong key satisfied n2, want fail")
	}
	// min_count：n5 user_feedback_given min_count=1，两件套缺 user_confirmed(final)。
	ev, err = EvaluateNodeConditions(card, "n5_make_it_ours", Observation{
		At:     "t",
		Events: []ObservedEvent{{Name: "user_feedback_given"}, {Name: "user_feedback_given"}},
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if ev.Satisfied {
		t.Fatal("n5 satisfied without user_confirmed(final)")
	}
}

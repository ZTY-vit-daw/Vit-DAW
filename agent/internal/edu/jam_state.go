package edu

// 乐局状态机纯数据层（乐局 3a，EDU-LEVEL-RUNTIME-A 目标 1）。
//
// 迁移图（v0 有入口的迁移加粗语义见函数注释）：
//
//	pending ──enter──▶ active ──done_when ✓──▶ done
//	                      │
//	                      └─judge attempt ✗──▶ active（attempt+1，事件 attempt_failed）
//
//	枚举占位（12 月不用，§五-16 挂起接口=零实现零字段预留）：
//	  active → suspended / suspended → active 合法迁移已声明但 v0 无入口 API；
//	  旧状态含 suspended 值时 restore 原值保留（§11 旧值缺省语义），Advance
//	  遇 suspended 当前节点 fail-closed 返回 ErrSuspendedNode。
//
// 接口不变式（§五-16）：本文件纯数据零 controller 引用——不进
// orchestrationcontroller 枚举、不进路由白名单；推进=纯函数
// (state, observation) → state'，不读时钟（时间戳由调用方经 Observation.At 注入）。
//
// 事件表为追加型审计表：只增不改；restore 对未知事件种类保留原文（审计
// 完整性优先），但未知种类不驱动任何迁移。

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

// rangeOrFull 将卡内 range_beats 对（缺省=全片段开区间 [0, +Inf)）。
func rangeOrFull(rb []float64) (float64, float64) {
	if len(rb) == 2 {
		return rb[0], rb[1]
	}
	return 0, math.Inf(1)
}

// JamStateSchemaVersion 是乐局状态的持久化版本号（restore fail-closed 依据）。
const JamStateSchemaVersion = "vit.jam_state.v0"

// 节点状态枚举（闭集）。
const (
	JamNodePending   = "pending"   // 尚未进入
	JamNodeActive    = "active"    // 当前推进中节点
	JamNodeDone      = "done"      // done_when 已满足
	JamNodeSuspended = "suspended" // 枚举占位：12 月不用（挂起接口零实现 §五-16）
)

// jamNodeStates 节点状态合法值闭集（restore 校验用）。
var jamNodeStates = map[string]bool{
	JamNodePending: true, JamNodeActive: true, JamNodeDone: true, JamNodeSuspended: true,
}

// legalJamTransitions 声明合法迁移表。v0 提供入口的迁移= pending→active
// （NewJamState/Advance 进入下一节点）与 active→done（Advance 判定通过）；
// suspended 双向迁移仅枚举声明（合法但无入口 API），供测试与后续卡扩展。
var legalJamTransitions = map[string][]string{
	JamNodePending:   {JamNodeActive},
	JamNodeActive:    {JamNodeDone, JamNodeSuspended},
	JamNodeSuspended: {JamNodeActive},
}

// 事件种类枚举（闭集；restore 对未知种类保留原文但不驱动迁移）。
const (
	JamEventNodeEntered   = "node_entered"   // pending → active
	JamEventNodeDone      = "node_done"      // active → done（done_when 满足）
	JamEventAttemptFailed = "attempt_failed" // judge 输入已观察但判定未过（attempt+1）
)

// jamEventKinds 事件种类合法值闭集。
var jamEventKinds = map[string]bool{
	JamEventNodeEntered: true, JamEventNodeDone: true, JamEventAttemptFailed: true,
}

// 推进哨兵错误。
var (
	// ErrJamCompleted 全部节点 done：乐局已完成，不再推进。
	ErrJamCompleted = errors.New("edu/jam: all nodes done; jam completed")
	// ErrSuspendedNode 当前节点处于 suspended（枚举占位）：v0 无恢复入口，fail-closed。
	ErrSuspendedNode = errors.New("edu/jam: current node suspended; v0 declares no resume entry point (挂起接口零实现)")
)

// JamNodeState 是一个乐局节点的运行状态（节点数组元素）。
type JamNodeState struct {
	NodeID    string `json:"node_id"`
	State     string `json:"state"`
	Attempt   int    `json:"attempt"`              // judge 尝试次数（事件/结构观察不计入）
	EnteredAt string `json:"entered_at,omitempty"` // RFC3339；进入 active 的时刻（调用方时钟）
}

// JamEvent 是事件追加表的元素（追加型审计：Seq 递增、只增不改）。
type JamEvent struct {
	Seq    int            `json:"seq"`
	Kind   string         `json:"kind"`
	NodeID string         `json:"node_id,omitempty"`
	At     string         `json:"at,omitempty"` // RFC3339（Observation.At 注入）
	Detail map[string]any `json:"detail,omitempty"`
}

// JamState 是乐局运行状态快照：schema_version + 会话/工程双绑定 + 节点数组
// + 事件追加表。纯数据：不含任何 controller/路由引用，JSON 往返即持久化。
type JamState struct {
	SchemaVersion  string         `json:"schema_version"`
	LevelID        string         `json:"level_id"`
	ConversationID string         `json:"conversation_id"` // 会话绑定
	ProjectUUID    string         `json:"project_uuid"`    // 工程绑定
	Nodes          []JamNodeState `json:"nodes"`
	Events         []JamEvent     `json:"events"`
}

// ObservedEvent 是一次推进窗口内观察到的交互事件（运行时从事件流投影，
// Name/Key 与乐局卡事件枚举对齐）。
type ObservedEvent struct {
	Name string `json:"name"`
	Key  string `json:"key,omitempty"`
}

// Observation 是一次推进的纯数据观察输入（零 IO，全部由调用方从读取命令
// 返回值组装；判定器输入=notes 数组的契约在此落型）。
type Observation struct {
	At    string            `json:"at"`              // RFC3339；事件时间戳来源（本层不读时钟）
	Slots map[string][]Note `json:"slots,omitempty"` // 运行时轨槽名 → clip notes 快照；键存在即 clip_exists=true
	// TrackNonEmpty 是 track_non_empty 谓词的观察值（轨槽名 → 非空）。
	TrackNonEmpty map[string]bool `json:"track_non_empty,omitempty"`
	// TempoSet 是 tempo_set 谓词的观察值。
	TempoSet bool `json:"tempo_set,omitempty"`
	// Events 是窗口内交互事件观察。
	Events []ObservedEvent `json:"events,omitempty"`
}

// NewJamState 创建初始状态：全部节点 pending，首节点进入 active
// （事件 node_entered seq=1）。conversation_id 与 project_uuid 双绑定必填。
func NewJamState(levelID, conversationID, projectUUID string, nodeIDs []string, at string) (JamState, error) {
	if strings.TrimSpace(levelID) == "" {
		return JamState{}, fmt.Errorf("edu/jam: level id required")
	}
	if strings.TrimSpace(conversationID) == "" || strings.TrimSpace(projectUUID) == "" {
		return JamState{}, fmt.Errorf("edu/jam: conversation_id and project_uuid are both required (双绑定)")
	}
	if len(nodeIDs) == 0 {
		return JamState{}, fmt.Errorf("edu/jam: at least one node id required")
	}
	seen := map[string]bool{}
	for _, id := range nodeIDs {
		if strings.TrimSpace(id) == "" || seen[id] {
			return JamState{}, fmt.Errorf("edu/jam: node ids must be non-empty and unique (got %q)", id)
		}
		seen[id] = true
	}
	s := JamState{
		SchemaVersion:  JamStateSchemaVersion,
		LevelID:        levelID,
		ConversationID: conversationID,
		ProjectUUID:    projectUUID,
		Nodes:          make([]JamNodeState, len(nodeIDs)),
		Events:         []JamEvent{},
	}
	for i, id := range nodeIDs {
		s.Nodes[i] = JamNodeState{NodeID: id, State: JamNodePending}
	}
	enterNode(&s, 0, at)
	return s, nil
}

// enterNode 将节点数组第 idx 项 pending → active 并追加 node_entered 事件。
func enterNode(s *JamState, idx int, at string) {
	s.Nodes[idx].State = JamNodeActive
	s.Nodes[idx].EnteredAt = at
	s.appendEvent(JamEventNodeEntered, s.Nodes[idx].NodeID, at, nil)
}

// appendEvent 追加一条事件（Seq = 既有最大 Seq + 1；restore 后仍保持递增）。
func (s *JamState) appendEvent(kind, nodeID, at string, detail map[string]any) {
	maxSeq := 0
	for _, e := range s.Events {
		if e.Seq > maxSeq {
			maxSeq = e.Seq
		}
	}
	s.Events = append(s.Events, JamEvent{
		Seq: maxSeq + 1, Kind: kind, NodeID: nodeID, At: at, Detail: detail,
	})
}

// nodeStateOf 返回节点当前状态（状态数组缺项=suspended restore 丢弃或
// 新卡节点，按 pending 缺省语义处理）。
func (s *JamState) nodeStateOf(nodeID string) JamNodeState {
	for _, n := range s.Nodes {
		if n.NodeID == nodeID {
			return n
		}
	}
	return JamNodeState{NodeID: nodeID, State: JamNodePending}
}

// CurrentJamNode 返回当前节点（卡序首个 active 或 suspended 的节点）与
// 其在卡节点表中的索引；无当前节点（全部 done）时 ok=false。
func (s *JamState) CurrentJamNode(card *LevelCard) (idx int, node JamNodeState, ok bool) {
	for i, cn := range card.Nodes {
		st := s.nodeStateOf(cn.ID)
		if st.State == JamNodeActive || st.State == JamNodeSuspended {
			return i, st, true
		}
	}
	return 0, JamNodeState{}, false
}

// AllNodesDone 报告卡内全部节点均已 done（finale closure 前提之一）。
func (s *JamState) AllNodesDone(card *LevelCard) bool {
	for _, cn := range card.Nodes {
		if s.nodeStateOf(cn.ID).State != JamNodeDone {
			return false
		}
	}
	return true
}

// LeafEvaluation 是 done_when 一个叶子的一次评估结果。
type LeafEvaluation struct {
	Kind      string        `json:"kind"` // judge | event | structure
	Name      string        `json:"name"` // 判定器/事件/结构谓词名
	Satisfied bool          `json:"satisfied"`
	Reason    string        `json:"reason,omitempty"`  // 未满足时的缺口人读说明
	Verdict   *JudgeVerdict `json:"verdict,omitempty"` // judge 叶携带判定结果
}

// NodeEvaluation 是一个节点 done_when 的整体评估结果。
type NodeEvaluation struct {
	NodeID    string           `json:"node_id"`
	Satisfied bool             `json:"satisfied"`
	Leaves    []LeafEvaluation `json:"leaves"`
	// JudgedAttempts 报告本次评估中实际执行了判定的 judge 叶数（attempt 计数依据）。
	JudgedAttempts int `json:"judged_attempts"`
}

// EvaluateNodeConditions 以纯数据观察评估节点 done_when（all/any 组合、
// depth ≤1 由校验门保证；本函数对未验证卡返回 error 不 panic）。
// 判定器叶：目标轨槽未观察 → 叶不满足（数据缺口非错误）；参照表缺 → error
// （卡引用断裂属校验门漏网，fail-closed 上交）。
func EvaluateNodeConditions(card *LevelCard, nodeID string, obs Observation) (NodeEvaluation, error) {
	nodeIdx := -1
	for i, n := range card.Nodes {
		if n.ID == nodeID {
			nodeIdx = i
			break
		}
	}
	if nodeIdx < 0 {
		return NodeEvaluation{}, fmt.Errorf("edu/jam: unknown node %q", nodeID)
	}
	ev, err := evaluateDoneWhen(card, &card.Nodes[nodeIdx].DoneWhen, obs)
	if err != nil {
		return NodeEvaluation{}, fmt.Errorf("edu/jam: node %s: %w", nodeID, err)
	}
	return NodeEvaluation{NodeID: nodeID, Satisfied: ev.Satisfied, Leaves: ev.Leaves, JudgedAttempts: ev.JudgedAttempts}, nil
}

func evaluateDoneWhen(card *LevelCard, dw *DoneWhen, obs Observation) (NodeEvaluation, error) {
	if dw.Leaf != nil {
		return evalLeaf(card, dw.Leaf, obs)
	}
	comb := "all"
	children := dw.All
	if len(children) == 0 {
		comb = "any"
		children = dw.Any
	}
	if len(children) == 0 {
		return NodeEvaluation{}, fmt.Errorf("done_when combinator %q has no children", comb)
	}
	result := NodeEvaluation{Satisfied: comb == "all"} // all 初值 true；any 初值 false
	for i := range children {
		sub, err := evaluateDoneWhen(card, &children[i], obs)
		if err != nil {
			return NodeEvaluation{}, err
		}
		result.Leaves = append(result.Leaves, sub.Leaves...)
		result.JudgedAttempts += sub.JudgedAttempts
		if comb == "all" {
			result.Satisfied = result.Satisfied && sub.Satisfied
		} else {
			result.Satisfied = result.Satisfied || sub.Satisfied
		}
	}
	return result, nil
}

func evalLeaf(card *LevelCard, leaf *ConditionLeaf, obs Observation) (NodeEvaluation, error) {
	var leafRes LeafEvaluation
	switch {
	case leaf.Judge != "":
		leafRes.Kind = "judge"
		leafRes.Name = leaf.Judge
		judge, ok := LookupJudge(leaf.Judge)
		if !ok {
			return NodeEvaluation{}, fmt.Errorf("unknown judge %q (registry closed set)", leaf.Judge)
		}
		target, ok := obs.Slots[leaf.Target.Clip]
		if !ok {
			leafRes.Satisfied = false
			leafRes.Reason = fmt.Sprintf("target slot %q not observed", leaf.Target.Clip)
			return NodeEvaluation{Satisfied: false, Leaves: []LeafEvaluation{leafRes}}, nil
		}
		// 参照源二选一：asset=卡内嵌 notes 表（零栈可判）；clip=运行时轨槽观察。
		var ref []Note
		if leaf.Reference.Asset != "" {
			table, has := card.AssetNotes(leaf.Reference.Asset)
			if !has {
				return NodeEvaluation{}, fmt.Errorf("judge leaf references unknown notes table %q", leaf.Reference.Asset)
			}
			ref = table
		} else if refClip, has := obs.Slots[leaf.Reference.Clip]; has {
			ref = refClip
		} else {
			leafRes.Satisfied = false
			leafRes.Reason = fmt.Sprintf("reference %q not resolvable (asset table or slot observation)", leaf.Reference.Clip)
			return NodeEvaluation{Satisfied: false, Leaves: []LeafEvaluation{leafRes}}, nil
		}
		tStart, tEnd := rangeOrFull(leaf.Target.RangeBeats)
		rStart, rEnd := rangeOrFull(leaf.Reference.RangeBeats)
		in := JudgeInput{
			JudgeName:        leaf.Judge,
			TargetNotes:      windowNotes(target, tStart, tEnd),
			ReferenceNotes:   windowNotes(ref, rStart, rEnd),
			AcceptTransposed: leaf.AcceptTransposed != nil && *leaf.AcceptTransposed,
			SetMode:          leaf.SetMode,
		}
		verdict := judge(in)
		leafRes.Satisfied = verdict.Passed
		leafRes.Verdict = &verdict
		if !verdict.Passed {
			leafRes.Reason = verdict.Summary
		}
		return NodeEvaluation{Satisfied: verdict.Passed, Leaves: []LeafEvaluation{leafRes}, JudgedAttempts: 1}, nil

	case leaf.Event != "":
		leafRes.Kind = "event"
		leafRes.Name = leaf.Event
		need := leaf.MinCount
		if need < 1 {
			need = 1
		}
		count := 0
		for _, e := range obs.Events {
			if e.Name != leaf.Event {
				continue
			}
			if leaf.Key != "" && e.Key != leaf.Key {
				continue
			}
			count++
		}
		leafRes.Satisfied = count >= need
		leafRes.Reason = fmt.Sprintf("observed %d/%d of event %q (key=%q)", count, need, leaf.Event, leaf.Key)
		return NodeEvaluation{Satisfied: leafRes.Satisfied, Leaves: []LeafEvaluation{leafRes}}, nil

	case leaf.Structure != "":
		leafRes.Kind = "structure"
		leafRes.Name = leaf.Structure
		switch leaf.Structure {
		case "clip_exists":
			_, exists := obs.Slots[leaf.Clip]
			leafRes.Satisfied = exists
			leafRes.Reason = fmt.Sprintf("clip %q exists=%v", leaf.Clip, exists)
		case "track_non_empty":
			v, ok := obs.TrackNonEmpty[leaf.Track]
			leafRes.Satisfied = ok && v
			leafRes.Reason = fmt.Sprintf("track %q non_empty=%v observed=%v", leaf.Track, v, ok)
		case "tempo_set":
			leafRes.Satisfied = obs.TempoSet
			leafRes.Reason = fmt.Sprintf("tempo_set=%v", obs.TempoSet)
		default:
			return NodeEvaluation{}, fmt.Errorf("unknown structure predicate %q", leaf.Structure)
		}
		return NodeEvaluation{Satisfied: leafRes.Satisfied, Leaves: []LeafEvaluation{leafRes}}, nil

	default:
		return NodeEvaluation{}, fmt.Errorf("done_when leaf has no judge/event/structure discriminator")
	}
}

// AdvanceJam 推进状态机：评估当前节点 done_when——满足则 done 并进入下一
// pending 节点（卡序）；不满足但有 judge 输入被评估则 attempt+1 并记录
// attempt_failed 事件；观察无 judge 输入（事件/结构观察）则状态不变。
// 纯函数：不修改入参（值语义复制），时间戳取 obs.At。
func AdvanceJam(state JamState, card *LevelCard, obs Observation) (JamState, error) {
	if card == nil {
		return JamState{}, fmt.Errorf("edu/jam: level card required")
	}
	if state.LevelID != card.ID {
		return JamState{}, fmt.Errorf("edu/jam: state/card level binding mismatch (%q vs %q)", state.LevelID, card.ID)
	}
	_, cur, ok := state.CurrentJamNode(card) // 节点按 node_id 寻址，索引仅诊断用
	if !ok {
		if state.AllNodesDone(card) {
			return state, ErrJamCompleted
		}
		return JamState{}, fmt.Errorf("edu/jam: no active node but jam not completed (inconsistent state)")
	}
	if cur.State == JamNodeSuspended {
		return JamState{}, fmt.Errorf("%w (node %s)", ErrSuspendedNode, cur.NodeID)
	}
	ev, err := EvaluateNodeConditions(card, cur.NodeID, obs)
	if err != nil {
		return JamState{}, err
	}

	// 同步节点数组中的当前节点项（nodeStateOf 缺项时补登记）。
	nodeArrIdx := -1
	for i := range state.Nodes {
		if state.Nodes[i].NodeID == cur.NodeID {
			nodeArrIdx = i
			break
		}
	}
	if nodeArrIdx < 0 {
		state.Nodes = append(state.Nodes, cur)
		nodeArrIdx = len(state.Nodes) - 1
	}

	if ev.Satisfied {
		// active → done（legalJamTransitions 声明内迁移）。
		state.Nodes[nodeArrIdx].State = JamNodeDone
		state.appendEvent(JamEventNodeDone, cur.NodeID, obs.At, map[string]any{
			"leaves": leafSummaries(ev.Leaves),
		})
		// 进入下一 pending 节点（卡序首个未 done 且未登记 active 的节点）。
		for _, cn := range card.Nodes {
			st := state.nodeStateOf(cn.ID)
			if st.State == JamNodePending {
				enterPendingNode(&state, cn.ID, obs.At)
				break
			}
		}
		return state, nil
	}

	// 未满足：有 judge 输入被评估 → attempt+1（每次观察即一次判定尝试）。
	if ev.JudgedAttempts > 0 {
		state.Nodes[nodeArrIdx].Attempt++
		state.appendEvent(JamEventAttemptFailed, cur.NodeID, obs.At, map[string]any{
			"attempt": state.Nodes[nodeArrIdx].Attempt,
			"leaves":  leafSummaries(ev.Leaves),
		})
	}
	return state, nil
}

// enterPendingNode 将卡序中仍为 pending 的节点置 active（restore 丢弃后
// 节点数组可能缺项：先补登记再进入）。
func enterPendingNode(s *JamState, nodeID, at string) {
	for i := range s.Nodes {
		if s.Nodes[i].NodeID == nodeID {
			enterNode(s, i, at)
			return
		}
	}
	s.Nodes = append(s.Nodes, JamNodeState{NodeID: nodeID, State: JamNodePending})
	enterNode(s, len(s.Nodes)-1, at)
}

func leafSummaries(leaves []LeafEvaluation) []string {
	out := make([]string, 0, len(leaves))
	for _, l := range leaves {
		mark := "✗"
		if l.Satisfied {
			mark = "✓"
		}
		out = append(out, fmt.Sprintf("%s %s:%s", mark, l.Kind, l.Name))
	}
	return out
}

// RestoreReport 记录 restore 的丢弃/保留计数（mix_workflow_queue_view.go:670
// 模式：未知节点 id 或状态值 drop fail-closed 并计数，never execute）。
type RestoreReport struct {
	DroppedUnknownNodes  int `json:"dropped_unknown_nodes"`  // node_id 不在卡内
	DroppedUnknownStates int `json:"dropped_unknown_states"` // 状态值不在枚举内（连同节点丢弃）
	KeptUnknownEvents    int `json:"kept_unknown_events"`    // 未知事件种类：审计保留原文，不驱动迁移
}

// RestoreJamState 从持久化 JSON 恢复状态（§11 旧状态缺省语义 + restore
// fail-closed）：
//   - schema_version 缺失或非 vit.jam_state.v0 → 整体拒绝（fail-closed；
//     v0 是首个版本，不存在更旧记录，无版本号即外来数据）；
//   - level_id 与卡不符 → 拒绝（绑定防串局）；
//   - 节点：不在卡内的 node_id、状态值不在枚举内 → 丢弃并计数（不执行）；
//     缺 attempt/entered_at 字段 → 缺省 0/""；
//   - suspended 值原样保留（枚举占位语义；Advance 遇之 fail-closed）；
//   - 事件表：未知种类保留原文并计数（审计完整性优先，不驱动迁移）。
func RestoreJamState(data []byte, card *LevelCard) (JamState, RestoreReport, error) {
	var report RestoreReport
	if card == nil {
		return JamState{}, report, fmt.Errorf("edu/jam: level card required")
	}
	var s JamState
	if err := json.Unmarshal(data, &s); err != nil {
		return JamState{}, report, fmt.Errorf("edu/jam: restore unmarshal: %w", err)
	}
	if s.SchemaVersion != JamStateSchemaVersion {
		return JamState{}, report, fmt.Errorf(
			"edu/jam: restore fail-closed: schema_version %q != %q (拒绝执行未知版本状态)",
			s.SchemaVersion, JamStateSchemaVersion)
	}
	if s.LevelID != card.ID {
		return JamState{}, report, fmt.Errorf(
			"edu/jam: restore fail-closed: level binding %q != card %q", s.LevelID, card.ID)
	}
	known := map[string]bool{}
	for _, n := range card.Nodes {
		known[n.ID] = true
	}
	cleanNodes := make([]JamNodeState, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		if !known[n.NodeID] {
			report.DroppedUnknownNodes++
			continue
		}
		if !jamNodeStates[n.State] {
			report.DroppedUnknownStates++
			continue
		}
		if n.Attempt < 0 {
			n.Attempt = 0
		}
		cleanNodes = append(cleanNodes, n)
	}
	s.Nodes = cleanNodes
	if s.Events == nil {
		s.Events = []JamEvent{}
	}
	for _, e := range s.Events {
		if !jamEventKinds[e.Kind] {
			report.KeptUnknownEvents++
		}
	}
	return s, report, nil
}

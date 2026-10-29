package chat

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"vit-daw-agent/internal/agentloop"
	executorpkg "vit-daw-agent/internal/executor"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/planner"
	"vit-daw-agent/internal/projectstore"
)

// MIXLAYER-M1-2 mix_workflow_queue.v0 — the professional mix workflow queue
// rendered as a projection view ON TOP of the M1-1 blackboard persistence
// layer. The queue owns no storage of its own: node states are re-derived
// every build by folding the two durable blackboard record streams (the
// mixboard decision ledger and the M1-1 free-state conclusion store) plus the
// project-runtime node annotations the agent writes back explicitly.
//
// Red line (card MIXLAYER-M1-2): the queue is NOT an orchestration entry.
// Nothing here dispatches capabilities or mutates the kernel project; the
// queue only reflects work that already happened (ledger records, settle
// conclusions) and agent/user annotations (skip/defer/hold). Execution stays
// inside the free-state loop.
//
// Authoritative node set and state list: docs/agent_action_workflow_v1_master_plan.md
// §6.1 (A1-A5 / B1-B4 / C1-C5 / D1-D4 / E1-E4 / F1) and §6.2 (9 states).
const (
	MixWorkflowQueueSchemaVersion = "mix_workflow_queue.v0"

	MixWorkflowNodeNotStarted        = "not_started"
	MixWorkflowNodeInProgress        = "in_progress"
	MixWorkflowNodeNeedsConfirmation = "needs_confirmation"
	MixWorkflowNodeCompleted         = "completed"
	MixWorkflowNodeBlocked           = "blocked"
	MixWorkflowNodeSkipped           = "skipped"
	MixWorkflowNodeDeferred          = "deferred"
	MixWorkflowNodeNeedsRecheck      = "needs_recheck"
	MixWorkflowNodeStale             = "stale"

	// Tools the agent loop uses to read the queue and write node annotations.
	// They are intercepted in chat before the base executor; the free-state
	// loop's deterministic preflights emit them (they are not LLM catalog
	// tools), so no catalog or capability-routing registration is involved.
	MixWorkflowQueueViewTool      = "mix_queue_view"
	MixWorkflowQueueNodeStateTool = "mix_queue_node_state"
)

var mixWorkflowNodeStateLabels = map[string]string{
	MixWorkflowNodeNotStarted:        "未开始",
	MixWorkflowNodeInProgress:        "进行中",
	MixWorkflowNodeNeedsConfirmation: "等待确认",
	MixWorkflowNodeCompleted:         "完成",
	MixWorkflowNodeBlocked:           "阻塞",
	MixWorkflowNodeSkipped:           "跳过",
	MixWorkflowNodeDeferred:          "延期",
	MixWorkflowNodeNeedsRecheck:      "需要复查",
	MixWorkflowNodeStale:             "已过期",
}

type mixWorkflowNodeDef struct {
	ID            string
	Family        string
	FamilyLabel   string
	Title         string
	CapabilityIDs []string
}

// mixWorkflowQueueNodes is the fixed v0 node catalog. Capability IDs are the
// capability families each node owns; ledger records and routed free-state
// conclusions map onto nodes through them. C4 (段落自动化) is owned by the
// M3-C4-AUTO-INTENT-1 section-automation choreography (decision ledger
// records under fine_mix.section_automation.v1 light it up); nodes without
// capability IDs (C5/D/E/F1) have no runtime yet — they stay not_started
// until real work lands (the view renders real state only).
var mixWorkflowQueueNodes = []mixWorkflowNodeDef{
	{"A1", "project_prep", "工程准备", "工程接收与整理", []string{"project_prep.import_intake.v0"}},
	{"A2", "project_prep", "工程准备", "技术完整性检查", []string{"project_prep.technical_integrity.v0"}},
	{"A3", "project_prep", "工程准备", "智能轨道整理", []string{"project_prep.track_organization.v0"}},
	{"A4", "project_prep", "工程准备", "片段编辑与清理", []string{"project_prep.clip_edit_cleanup.v0"}},
	{"A5", "project_prep", "工程准备", "段落地图", []string{"project_prep.section_marker_map.v0"}},
	{"B1", "static_mix", "粗混（静态）", "增益分级", []string{"static_mix.gain_staging.v0"}},
	{"B2", "static_mix", "粗混（静态）", "静态主次与音量平衡", []string{"static_mix.static_balance.v0"}},
	{"B3", "static_mix", "粗混（静态）", "声像布局", []string{"static_mix.pan_layout.v0"}},
	{"B4", "static_mix", "粗混（静态）", "低频关系", []string{"static_mix.low_end_relation.v0"}},
	{"C1", "fine_mix", "细混处理", "频段清理", []string{"fine_mix.frequency_cleanup.v0", "fine_mix.frequency_cleanup.v1"}},
	{"C2", "fine_mix", "细混处理", "动态控制", []string{"fine_mix.dynamic_control.v0", "fine_mix.dynamic_control.v1"}},
	{"C3", "fine_mix", "细混处理", "空间与深度", []string{"fine_mix.space_depth.v0"}},
	{"C4", "fine_mix", "细混处理", "段落自动化", []string{automationChoreoCapabilityID}},
	{"C5", "fine_mix", "细混处理", "总线处理", nil},
	{"D1", "pre_master", "母带前检查", "全局响度与峰值", nil},
	{"D2", "pre_master", "母带前检查", "翻译检查", nil},
	{"D3", "pre_master", "母带前检查", "参考曲对比", nil},
	{"D4", "pre_master", "母带前检查", "版本输出", nil},
	{"E1", "mastering", "母带", "母带 EQ", nil},
	{"E2", "mastering", "母带", "母带动态与响度", nil},
	{"E3", "mastering", "母带", "母带空间与立体声安全", nil},
	{"E4", "mastering", "母带", "最终导出与质检", nil},
	{"F1", "review", "混音审查", "混音审查", nil},
}

// mixWorkflowNodeByCapability resolves capability IDs (and the routing
// fallback "mixing_capability_stage.<node>" ids) to a queue node.
var mixWorkflowNodeByCapability = func() map[string]string {
	byID := map[string]string{}
	for _, node := range mixWorkflowQueueNodes {
		for _, capability := range node.CapabilityIDs {
			byID[capability] = node.ID
		}
	}
	return byID
}()

func mixWorkflowNodeForCapability(capabilityID string) string {
	capabilityID = strings.TrimSpace(capabilityID)
	if capabilityID == "" {
		return ""
	}
	if nodeID, ok := mixWorkflowNodeByCapability[capabilityID]; ok {
		return nodeID
	}
	// capabilityIDForStage fallback family: "mixing_capability_stage.b2".
	if stage, ok := strings.CutPrefix(capabilityID, "mixing_capability_stage."); ok {
		nodeID := strings.ToUpper(strings.TrimSpace(stage))
		if mixWorkflowNodeDefByID(nodeID) != nil {
			return nodeID
		}
	}
	return ""
}

func mixWorkflowNodeDefByID(nodeID string) *mixWorkflowNodeDef {
	nodeID = strings.ToUpper(strings.TrimSpace(nodeID))
	for index := range mixWorkflowQueueNodes {
		if mixWorkflowQueueNodes[index].ID == nodeID {
			return &mixWorkflowQueueNodes[index]
		}
	}
	return nil
}

func mixWorkflowValidNodeState(state string) bool {
	_, ok := mixWorkflowNodeStateLabels[state]
	return ok
}

// mixWorkflowDerivedSignal is one blackboard-derived state signal. Derived
// signals are evidence-only: completed/recheck/stale can never be written
// through the annotation face.
type mixWorkflowDerivedSignal struct {
	Kind   string // activity | completed | recheck | stale | blocked
	NodeID string
	At     time.Time
	Basis  string
}

const (
	mixWorkflowSignalActivity  = "activity"
	mixWorkflowSignalCompleted = "completed"
	mixWorkflowSignalRecheck   = "recheck"
	mixWorkflowSignalStale     = "stale"
	mixWorkflowSignalBlocked   = "blocked"
)

// mixWorkflowDerivedTransition applies one derived signal under the 9-state
// machine. Illegal transitions return ok=false and the caller drops (and
// counts) the signal — the fold never fabricates a state jump evidence does
// not license (e.g. a second activity burst cannot reopen a completed node;
// only recheck/stale evidence reopens it).
func mixWorkflowDerivedTransition(current, kind string) (string, bool) {
	switch kind {
	case mixWorkflowSignalActivity:
		switch current {
		case MixWorkflowNodeCompleted:
			return current, false
		default:
			return MixWorkflowNodeInProgress, true
		}
	case mixWorkflowSignalCompleted:
		return MixWorkflowNodeCompleted, true
	case mixWorkflowSignalRecheck:
		switch current {
		case MixWorkflowNodeNotStarted, MixWorkflowNodeSkipped:
			return current, false
		default:
			return MixWorkflowNodeNeedsRecheck, true
		}
	case mixWorkflowSignalStale:
		switch current {
		case MixWorkflowNodeNotStarted, MixWorkflowNodeSkipped:
			return current, false
		default:
			return MixWorkflowNodeStale, true
		}
	case mixWorkflowSignalBlocked:
		switch current {
		case MixWorkflowNodeCompleted:
			return current, false
		default:
			return MixWorkflowNodeBlocked, true
		}
	}
	return current, false
}

// mixWorkflowAnnotationTargetOK reports whether the annotation (write-back)
// face may set the target state at all: terminal evidence states are
// derived-only, the agent cannot command "completed".
func mixWorkflowAnnotationTargetOK(target string) bool {
	switch target {
	case MixWorkflowNodeInProgress, MixWorkflowNodeNeedsConfirmation, MixWorkflowNodeBlocked, MixWorkflowNodeSkipped, MixWorkflowNodeDeferred:
		return true
	default:
		return false
	}
}

// mixWorkflowExplicitTransition validates one annotation write-back against
// the current folded state. A completed node cannot be annotated over — the
// ledger's recheck/stale evidence must reopen it first; everything else the
// annotation face owns (user/agent choices: hold, skip, defer, resume) is
// legal.
func mixWorkflowExplicitTransition(current, target string) error {
	if !mixWorkflowValidNodeState(target) {
		return fmt.Errorf("unknown mix workflow node state %q", target)
	}
	if !mixWorkflowAnnotationTargetOK(target) {
		return fmt.Errorf("state %q is evidence-derived only (node %s annotations cannot command it)", target, "queue")
	}
	if current == target {
		return nil
	}
	if current == MixWorkflowNodeCompleted {
		return fmt.Errorf("node is completed; only ledger recheck/stale evidence may reopen it (current=%s target=%s)", current, target)
	}
	return nil
}

// MixWorkflowQueueNodeAnnotation is one explicit agent/user write-back onto a
// node. Annotations ride the project agent runtime state (same durable
// mechanism as capability routes): project-keyed, restart-surviving, and
// re-validated against newer blackboard evidence on every fold.
type MixWorkflowQueueNodeAnnotation struct {
	NodeID         string    `json:"node_id"`
	State          string    `json:"state"`
	Reason         string    `json:"reason,omitempty"`
	EvidenceRefs   []string  `json:"evidence_refs,omitempty"`
	ConversationID string    `json:"conversation_id,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type MixWorkflowQueueNodeView struct {
	NodeID      string    `json:"node_id"`
	Family      string    `json:"family"`
	FamilyLabel string    `json:"family_label"`
	Title       string    `json:"title"`
	State       string    `json:"state"`
	StateLabel  string    `json:"state_label"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
	Basis       []string  `json:"basis,omitempty"`
}

type MixWorkflowQueueSources struct {
	DecisionRecords             int      `json:"decision_records"`
	MappedDecisionSignals       int      `json:"mapped_decision_signals"`
	UnmappedDecisionRecords     int      `json:"unmapped_decision_records"`
	FreeStateConclusions        int      `json:"free_state_conclusions"`
	RoutedConclusionSignals     int      `json:"routed_conclusion_signals"`
	UnroutedConclusions         int      `json:"unrouted_conclusions"`
	AmbiguousRouteConversations int      `json:"ambiguous_route_conversations"`
	NodeAnnotations             int      `json:"node_annotations"`
	DroppedIllegalTransitions   int      `json:"dropped_illegal_transitions"`
	ReadErrors                  []string `json:"read_errors,omitempty"`
}

type MixWorkflowQueueView struct {
	SchemaVersion string                     `json:"schema_version"`
	ProjectUUID   string                     `json:"project_uuid"`
	GeneratedAt   time.Time                  `json:"generated_at"`
	Nodes         []MixWorkflowQueueNodeView `json:"nodes"`
	StateSummary  map[string]int             `json:"state_summary"`
	Sources       MixWorkflowQueueSources    `json:"sources"`
}

// mixWorkflowQueueTimelineEntry folds signals and annotations in one time
// order: newest evidence or annotation wins per node, under the machine.
type mixWorkflowQueueTimelineEntry struct {
	at      time.Time
	nodeID  string
	apply   func(current string) (string, bool)
	basis   string
	isWrite bool
}

// BuildMixWorkflowQueueView folds the durable blackboard streams plus the
// annotation overlay into the queue view. Every stage is fail-open for
// display: a board read failure is recorded in Sources.ReadErrors and the
// rest of the view still renders (missing files on old projects are the
// documented empty-board default, not errors).
func (s *Server) BuildMixWorkflowQueueView(ctx context.Context) MixWorkflowQueueView {
	view := MixWorkflowQueueView{
		SchemaVersion: MixWorkflowQueueSchemaVersion,
		GeneratedAt:   time.Now().UTC(),
		StateSummary:  map[string]int{},
	}
	projectUUID := s.mixWorkflowQueueProjectUUID(ctx)
	view.ProjectUUID = projectUUID

	decisions, conclusions := mixboard.ProjectDecisionBoard{}, mixboard.FreeStateConclusionBoard{}
	if projectUUID != "" {
		store := mixboard.NewStore("")
		board, err := store.ReadProjectDecisionBoard(projectUUID)
		if err != nil {
			view.Sources.ReadErrors = append(view.Sources.ReadErrors, "decision_board: "+err.Error())
		} else {
			decisions = board
		}
		fscBoard, err := store.ReadFreeStateConclusionBoard(projectUUID)
		if err != nil {
			view.Sources.ReadErrors = append(view.Sources.ReadErrors, "free_state_conclusions: "+err.Error())
		} else {
			conclusions = fscBoard
		}
	} else {
		view.Sources.ReadErrors = append(view.Sources.ReadErrors, "no project uuid bound: queue rendered from annotations only")
	}

	timeline := s.mixWorkflowQueueTimeline(decisions, conclusions, &view.Sources)
	states, basis := foldMixWorkflowQueueTimeline(timeline, &view.Sources)

	annotations := s.currentMixWorkflowQueueAnnotations()
	view.Nodes = make([]MixWorkflowQueueNodeView, 0, len(mixWorkflowQueueNodes))
	for _, def := range mixWorkflowQueueNodes {
		state := states[def.ID]
		if state == "" {
			// No ledger evidence for this node: the annotation (write-back)
			// overlay is the honest display fallback — an in_progress/hold/
			// skip/defer annotation shows as-is, never inventing completion
			// evidence (completed stays ledger-derived only).
			if annotation, ok := annotations[def.ID]; ok && strings.TrimSpace(annotation.State) != "" {
				state = annotation.State
			}
		}
		if state == "" {
			state = MixWorkflowNodeNotStarted
		}
		view.StateSummary[state]++
		node := MixWorkflowQueueNodeView{
			NodeID: def.ID, Family: def.Family, FamilyLabel: def.FamilyLabel, Title: def.Title,
			State: state, StateLabel: mixWorkflowNodeStateLabels[state],
			Basis: basis[def.ID],
		}
		if entries := basis[def.ID]; len(entries) > 0 {
			node.UpdatedAt = foldMixWorkflowLatestAt(timeline, def.ID)
		}
		view.Nodes = append(view.Nodes, node)
	}
	return view
}

// currentMixWorkflowQueueAnnotations snapshots the in-memory annotation
// overlay (M1-2 write-back face) for read-only view folding.
func (s *Server) currentMixWorkflowQueueAnnotations() map[string]MixWorkflowQueueNodeAnnotation {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.mixWorkflowQueueNodes) == 0 {
		return nil
	}
	out := make(map[string]MixWorkflowQueueNodeAnnotation, len(s.mixWorkflowQueueNodes))
	for id, annotation := range s.mixWorkflowQueueNodes {
		out[id] = annotation
	}
	return out
}

// WriteMixWorkflowQueueNodeState is the annotation (write-back) face: one
// explicit node-state annotation, validated against the current folded state
// and persisted on the project agent runtime state. It mutates no project
// audio state — this is bookkeeping the queue re-derives from.
func (s *Server) WriteMixWorkflowQueueNodeState(ctx context.Context, nodeID, targetState, reason string, evidenceRefs []string, conversationID string) (MixWorkflowQueueNodeAnnotation, error) {
	def := mixWorkflowNodeDefByID(nodeID)
	if def == nil {
		return MixWorkflowQueueNodeAnnotation{}, fmt.Errorf("unknown mix workflow queue node %q", nodeID)
	}
	targetState = strings.TrimSpace(targetState)
	if !mixWorkflowValidNodeState(targetState) {
		return MixWorkflowQueueNodeAnnotation{}, fmt.Errorf("unknown mix workflow node state %q", targetState)
	}
	if !mixWorkflowAnnotationTargetOK(targetState) {
		return MixWorkflowQueueNodeAnnotation{}, fmt.Errorf("node %s state %q is evidence-derived only; annotations may hold/skip/defer/resume but not command completion evidence", def.ID, targetState)
	}
	current := s.currentMixWorkflowNodeState(ctx, def.ID)
	if err := mixWorkflowExplicitTransition(current, targetState); err != nil {
		return MixWorkflowQueueNodeAnnotation{}, err
	}
	annotation := MixWorkflowQueueNodeAnnotation{
		NodeID:         def.ID,
		State:          targetState,
		Reason:         strings.TrimSpace(reason),
		ConversationID: strings.TrimSpace(conversationID),
		UpdatedAt:      time.Now().UTC(),
	}
	for _, ref := range evidenceRefs {
		if ref = strings.TrimSpace(ref); ref != "" {
			annotation.EvidenceRefs = append(annotation.EvidenceRefs, ref)
		}
	}
	if s != nil {
		s.mu.Lock()
		if s.mixWorkflowQueueNodes == nil {
			s.mixWorkflowQueueNodes = map[string]MixWorkflowQueueNodeAnnotation{}
		}
		s.mixWorkflowQueueNodes[def.ID] = annotation
		s.mu.Unlock()
		if err := s.persistCurrentProjectWorkspaceChecked(); err != nil && s.logger != nil {
			s.logger.Warn("[mix-workflow-queue] node annotation persisted in memory only node=%s error=%v", def.ID, err)
		}
	}
	return annotation, nil
}

// currentMixWorkflowNodeState returns one node's folded state (empty when the
// node sits at the not_started default).
func (s *Server) currentMixWorkflowNodeState(ctx context.Context, nodeID string) string {
	view := s.BuildMixWorkflowQueueView(ctx)
	for _, node := range view.Nodes {
		if node.NodeID == strings.ToUpper(strings.TrimSpace(nodeID)) {
			return node.State
		}
	}
	return ""
}

func (s *Server) mixWorkflowQueueProjectUUID(ctx context.Context) string {
	if s == nil {
		return ""
	}
	if uuid := strings.TrimSpace(s.activeWorkspaceUUID); uuid != "" {
		return uuid
	}
	if s.harness != nil && ctx != nil {
		if _, projectUUID := s.harness.CurrentProjectIdentity(ctx); strings.TrimSpace(projectUUID) != "" {
			return strings.TrimSpace(projectUUID)
		}
	}
	if roots, ok := projectstore.Current(); ok && strings.TrimSpace(roots.ProjectUUID) != "" {
		return strings.TrimSpace(roots.ProjectUUID)
	}
	return ""
}

// mixWorkflowQueueTimeline builds the merged signal+annotation timeline from
// the two blackboard boards and the in-memory annotation overlay.
func (s *Server) mixWorkflowQueueTimeline(decisions mixboard.ProjectDecisionBoard, conclusions mixboard.FreeStateConclusionBoard, sources *MixWorkflowQueueSources) []mixWorkflowQueueTimelineEntry {
	timeline := make([]mixWorkflowQueueTimelineEntry, 0, len(decisions.Decisions)+len(conclusions.Conclusions))

	sources.DecisionRecords = len(decisions.Decisions)
	decisionRows := append([]mixboard.DecisionView(nil), decisions.Decisions...)
	sort.SliceStable(decisionRows, func(i, j int) bool {
		if decisionRows[i].CompletedAt.Equal(decisionRows[j].CompletedAt) {
			return decisionRows[i].RecordID < decisionRows[j].RecordID
		}
		return decisionRows[i].CompletedAt.Before(decisionRows[j].CompletedAt)
	})
	for _, decision := range decisionRows {
		nodeID := mixWorkflowNodeForCapability(decision.CapabilityID)
		if nodeID == "" {
			sources.UnmappedDecisionRecords++
			continue
		}
		kind, ok := mixWorkflowDecisionSignal(decision.CurrentStatus)
		if !ok {
			continue
		}
		at := decision.CompletedAt
		basis := fmt.Sprintf("mixboard-decision:%s (%s)", decision.RecordID, decision.CapabilityID)
		timeline = append(timeline, mixWorkflowQueueTimelineEntry{
			at: at, nodeID: nodeID,
			apply: func(current string) (string, bool) { return mixWorkflowDerivedTransition(current, kind) },
			basis: basis,
		})
		sources.MappedDecisionSignals++
	}

	sources.FreeStateConclusions = len(conclusions.Conclusions)
	routeIndex := s.mixWorkflowRouteCapabilityIndex(&sources.AmbiguousRouteConversations)
	for _, conclusion := range conclusions.Conclusions {
		capability := routeIndex[conclusion.ConversationID]
		nodeID := mixWorkflowNodeForCapability(capability)
		if nodeID == "" {
			sources.UnroutedConclusions++
			continue
		}
		kind := mixWorkflowSignalActivity
		if strings.Contains(strings.ToLower(conclusion.Outcome+";"+conclusion.DecisionStatus), "capability_blocked") {
			kind = mixWorkflowSignalBlocked
		}
		basis := fmt.Sprintf("free-state-conclusion:%s (%s→%s)", conclusion.ConclusionID, capability, conclusion.Outcome)
		timeline = append(timeline, mixWorkflowQueueTimelineEntry{
			at: conclusion.SettledAt, nodeID: nodeID,
			apply: func(current string) (string, bool) { return mixWorkflowDerivedTransition(current, kind) },
			basis: basis,
		})
		sources.RoutedConclusionSignals++
	}

	if s != nil {
		s.mu.Lock()
		annotations := make([]MixWorkflowQueueNodeAnnotation, 0, len(s.mixWorkflowQueueNodes))
		for _, annotation := range s.mixWorkflowQueueNodes {
			annotations = append(annotations, annotation)
		}
		s.mu.Unlock()
		sources.NodeAnnotations = len(annotations)
		for _, annotation := range annotations {
			target, nodeID, at := annotation.State, strings.ToUpper(strings.TrimSpace(annotation.NodeID)), annotation.UpdatedAt
			basis := fmt.Sprintf("queue-annotation:%s→%s", annotation.NodeID, annotation.State)
			if annotation.Reason != "" {
				basis += " (" + annotation.Reason + ")"
			}
			timeline = append(timeline, mixWorkflowQueueTimelineEntry{
				at: at, nodeID: nodeID, isWrite: true,
				apply: func(current string) (string, bool) {
					if err := mixWorkflowExplicitTransition(current, target); err != nil {
						return current, false
					}
					return target, true
				},
				basis: basis,
			})
		}
	}

	sort.SliceStable(timeline, func(i, j int) bool { return timeline[i].at.Before(timeline[j].at) })
	return timeline
}

// mixWorkflowDecisionSignal maps a decision ledger CurrentStatus onto a
// queue signal. cancelled/superseded records are not node progress.
func mixWorkflowDecisionSignal(currentStatus string) (string, bool) {
	switch strings.TrimSpace(currentStatus) {
	case mixboard.DecisionVerified:
		return mixWorkflowSignalCompleted, true
	case mixboard.DecisionNeedsReview:
		return mixWorkflowSignalRecheck, true
	case mixboard.DecisionStale:
		return mixWorkflowSignalStale, true
	case mixboard.DecisionFailed, mixboard.DecisionInconclusive:
		return mixWorkflowSignalActivity, true
	default: // cancelled / superseded / future values
		return "", false
	}
}

// mixWorkflowRouteCapabilityIndex joins conclusion conversation ids onto the
// routed capability. Only unambiguous conversations map: a conversation that
// routed to more than one distinct capability cannot honestly attribute its
// conclusions to one node, so it is counted ambiguous and left unmapped.
func (s *Server) mixWorkflowRouteCapabilityIndex(ambiguous *int) map[string]string {
	index := map[string]string{}
	if s == nil {
		return index
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	byConversation := map[string]map[string]bool{}
	for _, record := range s.capabilityRoutes {
		conversationID := strings.TrimSpace(record.ConversationID)
		capability := ""
		if record.Assessment != nil {
			capability = strings.TrimSpace(record.Assessment.SelectedCapability)
		}
		if conversationID == "" || capability == "" {
			continue
		}
		if byConversation[conversationID] == nil {
			byConversation[conversationID] = map[string]bool{}
		}
		byConversation[conversationID][capability] = true
	}
	for conversationID, capabilities := range byConversation {
		if len(capabilities) != 1 {
			if ambiguous != nil {
				*ambiguous++
			}
			continue
		}
		for capability := range capabilities {
			index[conversationID] = capability
		}
	}
	return index
}

func foldMixWorkflowQueueTimeline(timeline []mixWorkflowQueueTimelineEntry, sources *MixWorkflowQueueSources) (map[string]string, map[string][]string) {
	states := map[string]string{}
	basis := map[string][]string{}
	for _, entry := range timeline {
		if mixWorkflowNodeDefByID(entry.nodeID) == nil {
			sources.DroppedIllegalTransitions++
			continue
		}
		current := states[entry.nodeID]
		if current == "" {
			current = MixWorkflowNodeNotStarted
		}
		next, ok := entry.apply(current)
		if !ok {
			sources.DroppedIllegalTransitions++
			continue
		}
		states[entry.nodeID] = next
		basis[entry.nodeID] = append(basis[entry.nodeID], entry.basis)
	}
	return states, basis
}

func foldMixWorkflowLatestAt(timeline []mixWorkflowQueueTimelineEntry, nodeID string) time.Time {
	var latest time.Time
	for _, entry := range timeline {
		if entry.nodeID == nodeID && entry.at.After(latest) {
			latest = entry.at
		}
	}
	return latest
}

// RenderMixWorkflowQueueView renders the zh chat-facing view. The data source
// line keeps the projection honest: every state names where it came from.
func RenderMixWorkflowQueueView(view MixWorkflowQueueView) string {
	var builder strings.Builder
	builder.WriteString("混音工作队列 ")
	builder.WriteString(MixWorkflowQueueSchemaVersion)
	builder.WriteString(fmt.Sprintf("（%d 节点 / 9 态）\n", len(view.Nodes)))
	currentFamily := ""
	for _, node := range view.Nodes {
		if node.FamilyLabel != currentFamily {
			currentFamily = node.FamilyLabel
			builder.WriteString("\n")
			builder.WriteString(currentFamily)
			builder.WriteString("：")
		} else {
			builder.WriteString(" · ")
		}
		builder.WriteString(fmt.Sprintf("%s %s[%s]", node.NodeID, node.Title, node.StateLabel))
	}
	builder.WriteString(fmt.Sprintf("\n\n自由态结论（跨会话）：%d 条", view.Sources.FreeStateConclusions))
	s := view.Sources
	builder.WriteString(fmt.Sprintf(
		"\n数据来源：决策账本 %d 条（映射节点信号 %d / 未映射 %d）· 自由态结论 %d 条（映射节点 %d / 未映射 %d / 会话歧义 %d）· 节点标注 %d 条 · 丢弃非法迁移 %d 条",
		s.DecisionRecords, s.MappedDecisionSignals, s.UnmappedDecisionRecords,
		s.FreeStateConclusions, s.RoutedConclusionSignals, s.UnroutedConclusions, s.AmbiguousRouteConversations,
		s.NodeAnnotations, s.DroppedIllegalTransitions))
	if len(s.ReadErrors) > 0 {
		builder.WriteString(fmt.Sprintf("\n读取受限：%s", strings.Join(s.ReadErrors, "；")))
	}
	return builder.String()
}

// restoreMixWorkflowQueueAnnotations sanitizes a restored runtime state
// overlay (AGENTS §11): nil maps from pre-M1-2 records stay empty; unknown
// node ids or states are dropped fail-closed and counted, never executed.
func restoreMixWorkflowQueueAnnotations(restored map[string]MixWorkflowQueueNodeAnnotation) map[string]MixWorkflowQueueNodeAnnotation {
	clean := map[string]MixWorkflowQueueNodeAnnotation{}
	for nodeID, annotation := range restored {
		def := mixWorkflowNodeDefByID(nodeID)
		if def == nil || !mixWorkflowValidNodeState(annotation.State) || !mixWorkflowAnnotationTargetOK(annotation.State) {
			continue
		}
		annotation.NodeID = def.ID
		clean[def.ID] = annotation
	}
	return clean
}

// handleMixWorkflowQueueChat is the deterministic chat-side queue face. It
// runs BEFORE semantic-entry classification: queue reads and annotation
// write-backs are agent faces, not NL mix requests, and must not depend on
// the classifier's mood (run 194717: the annotation command was classified
// semantic_entry_unresolved and never reached the loop preflights). The
// message-loop preflights stay as the in-loop carrier for conversations that
// do reach the loop.
func (s *Server) handleMixWorkflowQueueChat(ctx context.Context, conversationID, mode, text string) (ChatResponse, bool) {
	text = strings.TrimSpace(text)
	base := ChatResponse{
		ConversationID: conversationID,
		AgentMode:      firstNonEmpty(strings.TrimSpace(mode), agentModeDefault),
		Workflow:       "mix_workflow_queue",
		GoalStatus:     "completed",
		StopReason:     "done",
	}
	if nodeID, state, reason, ok := agentloop.ParseMixWorkflowQueueNodeStateCommand(text); ok {
		annotation, err := s.WriteMixWorkflowQueueNodeState(ctx, nodeID, state, reason, nil, conversationID)
		if err != nil {
			base.Reply = "节点标注没有回写：" + err.Error()
			base.GoalStatus = "failed"
			base.StopReason = "mix_workflow_queue_annotation_rejected"
			base.Error = err.Error()
			base.WorkflowData = map[string]any{"status": "rejected", "node_id": nodeID, "target_state": state, "error": err.Error()}
			return base, true
		}
		def := mixWorkflowNodeDefByID(annotation.NodeID)
		line := fmt.Sprintf("已回写节点标注：%s %s[%s]", annotation.NodeID, def.Title, mixWorkflowNodeStateLabels[annotation.State])
		if annotation.Reason != "" {
			line += "（" + annotation.Reason + "）"
		}
		base.Reply = line
		base.WorkflowData = map[string]any{"status": "recorded", "node_id": annotation.NodeID, "node_state": annotation.State}
		return base, true
	}
	if agentloop.MixWorkflowQueueStatusRequest(text) {
		view := s.BuildMixWorkflowQueueView(ctx)
		base.Reply = RenderMixWorkflowQueueView(view)
		base.WorkflowData = map[string]any{"schema_version": view.SchemaVersion, "view": mixWorkflowQueueViewMap(view)}
		return base, true
	}
	return ChatResponse{}, false
}

// mixWorkflowQueueToolRequest matches the two queue tools the deterministic
// preflights emit. They never reach the base executor: the queue view is a
// chat-side projection, not a harness command.
func mixWorkflowQueueToolRequest(call planner.ToolCall) bool {
	switch strings.TrimSpace(call.Tool) {
	case MixWorkflowQueueViewTool, MixWorkflowQueueNodeStateTool:
		return true
	default:
		return false
	}
}

// invokeMixWorkflowQueueTool serves both agent-facing queue tools:
// mix_queue_view (read face) and mix_queue_node_state (annotation write-back
// face). Both are bookkeeping only — no kernel mutation, no capability
// dispatch (red line: the queue is not an execution entry).
func (e pluginGrabberWorkflowExecutor) invokeMixWorkflowQueueTool(ctx context.Context, in executorpkg.Input, toolCallID string) (executorpkg.Result, error) {
	server := e.server
	tool := strings.TrimSpace(in.ToolCall.Tool)
	base := executorpkg.Result{ToolCallID: toolCallID, Tool: tool, Status: "ok"}
	switch tool {
	case MixWorkflowQueueViewTool:
		view := server.BuildMixWorkflowQueueView(ctx)
		render := RenderMixWorkflowQueueView(view)
		base.CommandName = "mix_workflow_queue_view"
		base.Result = map[string]any{
			"schema_version": view.SchemaVersion,
			"message":        render,
			"view":           mixWorkflowQueueViewMap(view),
		}
		base.Preview = firstLine(render)
		return base, nil
	case MixWorkflowQueueNodeStateTool:
		nodeID := firstStringFromMap(in.ToolCall.Args, "node_id", "node")
		target := firstStringFromMap(in.ToolCall.Args, "state", "target_state")
		reason := firstStringFromMap(in.ToolCall.Args, "reason")
		conversationID := firstStringFromMap(in.Context, "conversation_id", "conversationId")
		var evidenceRefs []string
		if raw, ok := in.ToolCall.Args["evidence_refs"].([]any); ok {
			for _, item := range raw {
				if ref := strings.TrimSpace(fmt.Sprintf("%v", item)); ref != "" && ref != "map[]" {
					evidenceRefs = append(evidenceRefs, ref)
				}
			}
		}
		annotation, err := server.WriteMixWorkflowQueueNodeState(ctx, nodeID, target, reason, evidenceRefs, conversationID)
		if err != nil {
			base.Status = "error"
			base.Error = err.Error()
			base.Result = map[string]any{"status": "rejected", "error": err.Error()}
			return base, nil
		}
		def := mixWorkflowNodeDefByID(annotation.NodeID)
		line := fmt.Sprintf("已回写节点标注：%s %s[%s]", annotation.NodeID, def.Title, mixWorkflowNodeStateLabels[annotation.State])
		if annotation.Reason != "" {
			line += "（" + annotation.Reason + "）"
		}
		base.CommandName = "mix_workflow_queue_node_state"
		base.Result = map[string]any{
			"status":     "recorded",
			"message":    line,
			"annotation": annotation,
			"node_id":    annotation.NodeID,
			"node_state": annotation.State,
		}
		base.Preview = line
		return base, nil
	default:
		base.Status = "error"
		base.Error = "unknown mix workflow queue tool: " + tool
		return base, nil
	}
}

func mixWorkflowQueueViewMap(view MixWorkflowQueueView) map[string]any {
	nodes := make([]map[string]any, 0, len(view.Nodes))
	for _, node := range view.Nodes {
		nodes = append(nodes, map[string]any{
			"node_id": node.NodeID, "family": node.Family, "title": node.Title,
			"state": node.State, "state_label": node.StateLabel,
		})
	}
	return map[string]any{
		"schema_version": view.SchemaVersion,
		"project_uuid":   view.ProjectUUID,
		"generated_at":   view.GeneratedAt,
		"nodes":          nodes,
		"state_summary":  view.StateSummary,
		"sources":        view.Sources,
	}
}

func firstLine(text string) string {
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		return strings.TrimSpace(text[:index])
	}
	return strings.TrimSpace(text)
}

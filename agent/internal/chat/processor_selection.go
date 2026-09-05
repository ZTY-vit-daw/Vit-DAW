package chat

import (
	"context"
	"strings"
	"time"

	"vit-daw-agent/internal/agentprotocol"
	"vit-daw-agent/internal/processorregistry"
)

// AGENT-1 A0（三层面审计计划 §2.3 A0，2026-09-05）：processor_selection 持久化
// 契约。当 plugin-bound 自由态域（broadband_compression 先行，static_eq 占位）
// 经 VIT_AGENT_DOMAIN_ROUTES 走 processor_selection 路由（domain_routing.go）时，
// 处理器选择结果在此落成一条服务端持有的持久化记录：
//
//   - 记录含 target_ref / semantic_processor_intent / selected_family /
//     target_mode(existing_plugin|load_required|native) / candidate_key /
//     evidence_refs / project_revision / pca_admission_receipt /
//     authorization_state；
//   - 路径、指纹、PCA 内部字段（pca_admission_receipt 的内容）只存在服务端
//     工作区状态里，任何模型可见面只能使用 processorSelectionModelView 的
//     白名单投影（候选 key 与能力摘要）；
//   - 旧路径（legacy_native 落 pending mix tick）不携带路由 marker，因此
//     永远不产生 selection 记录（RED④）；
//   - 记录跟随 conversation 的最新一条，authorization_state 沿
//     selected → load_pending → qualified（或任一环节 cancelled）演进；
//     参数执行/回读的收口状态属于后续里程碑（AGENT-2 evaluator 与 M15 手测
//     链），不在本卡伪造。
const (
	processorSelectionSchema = "processor_selection.v1"

	// processorSelectionRouteContextKey 是 native-domain 路由器在按域开关
	// 取向 processor_selection 并放行到语义策略层时写入 requestContext 的
	// marker。它只在开关显式启用的新路径上出现，是所有记录写入的唯一闸门。
	processorSelectionRouteContextKey = "processor_selection_route"

	processorSelectionAuthSelected    = "selected"
	processorSelectionAuthLoadPending = "load_pending"
	processorSelectionAuthQualified   = "qualified"
	processorSelectionAuthCancelled   = "cancelled"
)

// ProcessorSelectionRecord is the durable server-owned audit of one governed
// processor selection. PCAAdmissionReceipt keeps the full server receipt after
// post-load qualification; it is deliberately absent from every model-facing
// projection (processorSelectionModelView is the only sanctioned disclosure).
type ProcessorSelectionRecord struct {
	SchemaVersion           string         `json:"schema_version"`
	SelectionID             string         `json:"selection_id"`
	ConversationID          string         `json:"conversation_id"`
	GoalID                  string         `json:"goal_id,omitempty"`
	RunID                   string         `json:"run_id,omitempty"`
	ActionDomain            string         `json:"action_domain"`
	TargetRef               map[string]any `json:"target_ref,omitempty"`
	SemanticProcessorIntent map[string]any `json:"semantic_processor_intent,omitempty"`
	SelectedFamily          string         `json:"selected_family"`
	TargetMode              string         `json:"target_mode"`
	CandidateKey            string         `json:"candidate_key,omitempty"`
	EvidenceRefs            []string       `json:"evidence_refs,omitempty"`
	ProjectRevision         string         `json:"project_revision,omitempty"`
	PCAAdmissionReceipt     map[string]any `json:"pca_admission_receipt,omitempty"`
	AuthorizationState      string         `json:"authorization_state"`
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
}

// processorSelectionRouteMarker is set by routeAcceptedImprovementProposalNativeDomain
// when the per-domain switch routes a plugin-bound domain away from the legacy
// mix-tick path. Everything the record needs from the originating proposal is
// frozen here once, so the later write points (strategy choice, user selection,
// candidate chosen, post-load qualification) stay one-line hooks.
type processorSelectionRouteMarker struct {
	ActionDomain   string
	ConversationID string
	GoalID         string
	RunID          string
	TargetRef      map[string]any
	EvidenceRefs   []string
	Intent         map[string]any
}

func processorSelectionRouteFromContext(requestContext map[string]any) (processorSelectionRouteMarker, bool) {
	row := firstMapFromAny(requestContext[processorSelectionRouteContextKey])
	if len(row) == 0 {
		return processorSelectionRouteMarker{}, false
	}
	domain := strings.ToLower(strings.TrimSpace(firstStringFromMap(row, "action_domain")))
	if domain == "" {
		return processorSelectionRouteMarker{}, false
	}
	return processorSelectionRouteMarker{
		ActionDomain:   domain,
		ConversationID: firstStringFromMap(row, "conversation_id"),
		GoalID:         firstStringFromMap(row, "goal_id"),
		RunID:          firstStringFromMap(row, "run_id"),
		TargetRef:      cloneContext(firstMapFromAny(row["target_ref"])),
		EvidenceRefs:   stringListValue(row["evidence_refs"]),
		Intent:         cloneContext(firstMapFromAny(row["semantic_processor_intent"])),
	}, true
}

// markProcessorSelectionRoute freezes the routing marker into the live request
// context. The marker carries only proposal-level facts (target, evidence
// refs, intent wording) — no paths, fingerprints, or PCA internals — so its
// propagation through interaction payloads stays inside the model-safe
// boundary.
func markProcessorSelectionRoute(requestContext map[string]any, interaction PendingInteraction, proposal agentprotocol.ImprovementProposal, domain string) {
	if requestContext == nil {
		return
	}
	intent := cloneContext(firstMapFromAny(requestContext["free_state_semantic_processor_intent"]))
	if len(intent) == 0 {
		// No admitted domain-row anchor rode this continuation (the
		// broadband_compression row deliberately has none yet); record the
		// server-derived family axis from the admitted action domain instead.
		// Descriptive only — this never re-enters the free-state intent channel.
		intent = map[string]any{
			"family": processorFamilyForTreatmentProcessorType(improvementProposalProcessorType(proposal)),
			"intent": strings.TrimSpace(proposal.ImprovementIntent),
		}
	}
	requestContext[processorSelectionRouteContextKey] = map[string]any{
		"schema_version":            processorSelectionSchema,
		"action_domain":             domain,
		"route":                     DomainRouteProcessorSelection,
		"target_ref":                cloneContext(proposal.Target),
		"evidence_refs":             append([]string(nil), proposal.EvidenceRefs...),
		"semantic_processor_intent": intent,
		"conversation_id":           interaction.ConversationID,
		"goal_id":                   firstNonEmpty(interaction.GoalID, firstStringFromMap(requestContext, "goal_id")),
		"run_id":                    firstNonEmpty(interaction.RunID, firstStringFromMap(requestContext, "run_id")),
	}
}

// processorSelectionModelView is the ONLY model-facing projection of a
// selection record: candidate key plus a capability summary. Paths,
// fingerprints, attestation references, and the PCA receipt stay server-side
// by construction (allow-list, not redaction).
func processorSelectionModelView(record ProcessorSelectionRecord) map[string]any {
	view := map[string]any{
		"schema_version":      processorSelectionSchema,
		"selection_id":        record.SelectionID,
		"selected_family":     record.SelectedFamily,
		"target_mode":         record.TargetMode,
		"candidate_key":       record.CandidateKey,
		"authorization_state": record.AuthorizationState,
	}
	if registry, registryErr := processorregistry.Default(); registryErr == nil && registry != nil {
		if definition, ok := registry.Resolve(record.SelectedFamily); ok {
			view["capability_summary"] = map[string]any{
				"coverage_vocabulary": append([]string(nil), definition.CoverageVocabulary...),
				"inspect_only":        definition.InspectOnly,
			}
		}
	}
	return view
}

func (s *Server) storeProcessorSelection(record ProcessorSelectionRecord) {
	if s == nil || strings.TrimSpace(record.ConversationID) == "" || strings.TrimSpace(record.SelectionID) == "" {
		return
	}
	now := time.Now().UTC()
	record.UpdatedAt = now
	s.mu.Lock()
	if s.processorSelections == nil {
		s.processorSelections = map[string]ProcessorSelectionRecord{}
	}
	if previous, ok := s.processorSelections[record.SelectionID]; ok && !previous.CreatedAt.IsZero() {
		record.CreatedAt = previous.CreatedAt
	} else if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	s.processorSelections[record.SelectionID] = record
	s.mu.Unlock()
	if s.logger != nil {
		s.logger.Info("[processor.selection] stored id=%s conversation=%s domain=%s family=%s mode=%s candidate=%s auth=%s revision=%s",
			record.SelectionID, record.ConversationID, record.ActionDomain, record.SelectedFamily,
			record.TargetMode, record.CandidateKey, record.AuthorizationState, record.ProjectRevision)
	}
	// Same durability contract as capability routes: in-memory stays
	// authoritative on a contended save, and a persistent failure must stay
	// visible instead of silently degrading to a memory-only record.
	if err := s.persistCurrentProjectWorkspaceChecked(); err != nil && s.logger != nil {
		s.logger.Warn("[processor.selection] runtime state save deferred id=%s error=%v", record.SelectionID, err)
	}
}

// latestProcessorSelection returns the newest record of a conversation. The
// chain (strategy → candidate → qualification) is sequential per proposal, so
// recency by UpdatedAt is the correct identity for the one-line update hooks.
func (s *Server) latestProcessorSelection(conversationID string) (string, ProcessorSelectionRecord, bool) {
	if s == nil || strings.TrimSpace(conversationID) == "" {
		return "", ProcessorSelectionRecord{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	latestID := ""
	var latest ProcessorSelectionRecord
	for id, record := range s.processorSelections {
		if record.ConversationID != conversationID {
			continue
		}
		if latestID == "" || record.UpdatedAt.After(latest.UpdatedAt) {
			latestID, latest = id, record
		}
	}
	return latestID, latest, latestID != ""
}

func (s *Server) updateLatestProcessorSelection(conversationID string, mutate func(*ProcessorSelectionRecord)) {
	if s == nil {
		return
	}
	id, record, ok := s.latestProcessorSelection(conversationID)
	if !ok {
		return
	}
	mutate(&record)
	s.storeProcessorSelectionRecordPreservingID(id, record)
}

// storeProcessorSelectionRecordPreservingID is the in-place update path: it
// rewrites the same map slot instead of appending a new SelectionID.
func (s *Server) storeProcessorSelectionRecordPreservingID(id string, record ProcessorSelectionRecord) {
	now := time.Now().UTC()
	record.UpdatedAt = now
	s.mu.Lock()
	if s.processorSelections == nil {
		s.processorSelections = map[string]ProcessorSelectionRecord{}
	}
	if previous, ok := s.processorSelections[id]; ok && !previous.CreatedAt.IsZero() {
		record.CreatedAt = previous.CreatedAt
	}
	s.processorSelections[id] = record
	s.mu.Unlock()
	if s.logger != nil {
		s.logger.Info("[processor.selection] updated id=%s conversation=%s family=%s mode=%s candidate=%s auth=%s",
			id, record.ConversationID, record.SelectedFamily, record.TargetMode, record.CandidateKey, record.AuthorizationState)
	}
	if err := s.persistCurrentProjectWorkspaceChecked(); err != nil && s.logger != nil {
		s.logger.Warn("[processor.selection] runtime state save deferred id=%s error=%v", id, err)
	}
}

// processorSelectionProjectRevision reads the current project revision the
// same way the treatment-strategy instance reader does (live kernel state
// first, harness summary as fallback); empty stays honest when neither is
// reachable.
func (s *Server) processorSelectionProjectRevision(ctx context.Context) string {
	if s == nil {
		return ""
	}
	var state map[string]any
	if s.harness != nil {
		state = s.harness.UserStateSummary(ctx)
	}
	if client := s.eqKernelClient(); client != nil {
		if liveState, _, err := client.SendCommand(ctx, map[string]any{"cmd": "get_project_state"}); err == nil && kernelReplyOK(liveState) {
			state = liveState
		}
	}
	if len(state) == 0 {
		return ""
	}
	return firstStringFromMap(state, "project_revision", "revision")
}

// recordProcessorSelectionFromStrategy materializes the record the moment the
// governed strategy layer has produced a validated materialization choice
// (direct mode). Without the routing marker this is a no-op, which is exactly
// RED④: the legacy path and the pre-existing ordinary semantic flows never
// write a selection record.
func (s *Server) recordProcessorSelectionFromStrategy(ctx context.Context, requestContext map[string]any, choice semanticTreatmentChoice) {
	marker, ok := processorSelectionRouteFromContext(requestContext)
	if !ok {
		return
	}
	// One live record per conversation chain: a still-selected (or further
	// advanced) record owns this conversation; a cancelled chain may spawn a
	// fresh record on a later proposal.
	if _, existing, exists := s.latestProcessorSelection(marker.ConversationID); exists &&
		existing.AuthorizationState != processorSelectionAuthCancelled {
		return
	}
	s.storeProcessorSelection(ProcessorSelectionRecord{
		SchemaVersion:           processorSelectionSchema,
		SelectionID:             "psel_" + randomID(),
		ConversationID:          marker.ConversationID,
		GoalID:                  marker.GoalID,
		RunID:                   marker.RunID,
		ActionDomain:            marker.ActionDomain,
		TargetRef:               marker.TargetRef,
		SemanticProcessorIntent: marker.Intent,
		SelectedFamily:          processorFamilyForTreatmentProcessorType(choice.ProcessorType),
		TargetMode:              choice.TargetMode,
		CandidateKey:            choice.InstanceKey,
		EvidenceRefs:            marker.EvidenceRefs,
		ProjectRevision:         s.processorSelectionProjectRevision(ctx),
		AuthorizationState:      processorSelectionAuthSelected,
	})
}

// recordProcessorSelectionFromSelectedChoice is the choice_required twin: the
// record lands when the user has actually selected one offered strategy row.
func (s *Server) recordProcessorSelectionFromSelectedChoice(ctx context.Context, interaction PendingInteraction, selected map[string]any) {
	marker, ok := processorSelectionRouteFromContext(interaction.RequestContext)
	if !ok {
		return
	}
	choice := semanticTreatmentChoice{
		ProcessorType: firstStringFromMap(selected, "processor_type"),
		TargetMode:    firstStringFromMap(selected, "target_mode"),
		InstanceKey:   firstStringFromMap(selected, "instance_key"),
	}
	if strings.TrimSpace(choice.TargetMode) == "" {
		return
	}
	// Patch only the interaction-owned IDs into the stored marker; the
	// proposal-frozen target/evidence/intent must survive verbatim.
	requestContext := cloneContext(interaction.RequestContext)
	if requestContext == nil {
		requestContext = map[string]any{}
	}
	markerRow := cloneContext(firstMapFromAny(requestContext[processorSelectionRouteContextKey]))
	if len(markerRow) == 0 {
		return
	}
	markerRow["conversation_id"] = firstNonEmpty(marker.ConversationID, interaction.ConversationID)
	markerRow["goal_id"] = firstNonEmpty(marker.GoalID, interaction.GoalID)
	markerRow["run_id"] = firstNonEmpty(marker.RunID, interaction.RunID)
	requestContext[processorSelectionRouteContextKey] = markerRow
	s.recordProcessorSelectionFromStrategy(ctx, requestContext, choice)
}

// markProcessorSelectionCandidateChosen advances the record when the user has
// picked one PCA-admitted recommendation candidate and the load plan (a
// separate authorization boundary) is about to be created.
func (s *Server) markProcessorSelectionCandidateChosen(requestContext map[string]any, candidateKey string) {
	marker, ok := processorSelectionRouteFromContext(requestContext)
	if !ok {
		return
	}
	s.updateLatestProcessorSelection(marker.ConversationID, func(record *ProcessorSelectionRecord) {
		record.CandidateKey = firstNonEmpty(candidateKey, record.CandidateKey)
		record.AuthorizationState = processorSelectionAuthLoadPending
	})
}

// markProcessorSelectionQualified attaches the server-held PCA receipt after
// post-load qualification succeeded. The receipt content stays in this record
// only; the model-facing views keep using processorSelectionModelView.
func (s *Server) markProcessorSelectionQualified(requestContext map[string]any, receipt map[string]any) {
	marker, ok := processorSelectionRouteFromContext(requestContext)
	if !ok {
		return
	}
	s.updateLatestProcessorSelection(marker.ConversationID, func(record *ProcessorSelectionRecord) {
		if len(receipt) > 0 {
			record.PCAAdmissionReceipt = cloneContext(receipt)
		}
		record.AuthorizationState = processorSelectionAuthQualified
	})
}

// markProcessorSelectionCancelled closes the record when the user abandons
// the chain at the selection or recommendation boundary.
func (s *Server) markProcessorSelectionCancelled(requestContext map[string]any) {
	marker, ok := processorSelectionRouteFromContext(requestContext)
	if !ok {
		return
	}
	s.updateLatestProcessorSelection(marker.ConversationID, func(record *ProcessorSelectionRecord) {
		record.AuthorizationState = processorSelectionAuthCancelled
	})
}

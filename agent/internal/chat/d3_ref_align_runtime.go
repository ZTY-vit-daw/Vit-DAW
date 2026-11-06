package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/agentloop"
	"vit-daw-agent/internal/experiment"
	"vit-daw-agent/internal/kernel"
	"vit-daw-agent/internal/loudnesswatch"
	"vit-daw-agent/internal/mixboard"
	"vit-daw-agent/internal/rlm"
	agentruntime "vit-daw-agent/internal/runtime"
)

// D3-REF-ALIGN-THIN-1：D3 参考曲对齐·薄 runtime，第一档=内置参考标准
// （2026-11-07 范围修订：对齐目标用 RLM 内置交付预设 builtin:spotify 等
// ——254 的 loudnesswatch builtin 同源；自定义参考音频上传=第二档另卡，
// audition.* 试听命令族缺口维持登记与本卡无关）。观察载体复用
// pre_master.observation.v0 pack 已建的 D3 面（MIXLAYER-M1-4：RLM
// reference_level_model.projection.v0 的 RenderBindings 披露）；本文件按
// D1 settle / D2 stereo check 同款薄层模式只补四件事：
//
//  1. 显式检查分支（d3RefAlignIntent 触发）：在观察报告之上跑判据纯函数，
//     verdict 三态（aligned/gap_found/unmeasurable，LW verdict 模式——
//     未绑定参考标准/投影缺数据=unmeasurable 一等公民）；
//  2. verdict 落 store：ref_align_verdicts.v0（LW verdict store 同款纪律：
//     外来 schema fail-closed、dedupe、上限裁剪），与 loudness_watch_
//     verdicts.json / stereo_check_verdicts.json 同目录存放；
//  3. 动作面（与 D2 的纯披露不同，本卡面明确「纯电平类建议可走既有 gain
//     动作则挂卡」）：对齐差距的第一档全部是电平类（integrated band 越界
//     /sample peak 超 TP 上限），loudnesswatch.SuggestGainAction 的有界
//     track_gain_adjust 建议可诚实改变它们——走 D1 settle 同款既有
//     mix.propose_tick → 用户确认 → mix.apply_tick 确认链（零裸写）；无可
//     定位载体（建议 nil）时如实披露不发明动作（loudnesswatch/action.go
//     同款 unaddressable 纪律）；
//  4. D3 点亮：queue view 的 D3 节点 not_started→in_progress /
//     needs_confirmation（annotation 面；completed 留给账本证据推导，
//     M1-2 红线不变）。
//
// 判断权冻结令：本文件无 LLM 参与；入口是显式检查意图词（词法命中在
// semantic_entry 之前，判据腿零 LLM）。参考标准解析次序固定：render→
// delivery 绑定（投影 D3 面，消费不改写）优先，其次意图点名的内置预设
// （一次性判断不写绑定面），两者皆缺=unmeasurable（no_reference_standard）。
// 差距计算委托 loudnesswatch.Compare（approximate 口径限定词、peak 必要
// 条件语义、band 两侧判定全部继承，不另立标准）；测量行=feature snapshot
// 的 loudness_summaries（D1 settle 同源）。
const d3RefAlignCapabilityID = "pre_master.ref_align.v0"

// d3RefAlignTriggerKind marks explicit D3 checks on verdict records.
const d3RefAlignTriggerKind = "d3_ref_align"

// Verdict states (three-state, LW verdict pattern; card-pinned vocabulary).
const (
	d3RefAlignVerdictAligned      = "aligned"
	d3RefAlignVerdictGapFound     = "gap_found"
	d3RefAlignVerdictUnmeasurable = "unmeasurable"
)

// Action-face values: gap+addressable parks the existing governed gain card
// (gain_card_v1); every other outcome discloses honestly.
const (
	d3RefAlignActionNoneNeeded       = "none_needed"
	d3RefAlignActionGainCard         = "gain_card_v1"
	d3RefAlignActionDisclosureOnly   = "disclosure_only_v1"
	d3RefAlignNoAddressableReason    = "差距已量化但无可定位载体：loudness_summaries 行未能解析出最越界 clip 的所属轨道——按「不发明动作」红线仅披露，不虚构写目标"
	d3RefAlignUnmeasurableActionNote = "无法测量即无可对齐面：仅披露参考标准解析结果，不产生动作"
)

// d3RefAlignVerdictStoreSchemaVersion is the only accepted ref-align verdict
// store schema.
const d3RefAlignVerdictStoreSchemaVersion = "ref_align_verdicts.v0"

// d3RefAlignMaxKeptVerdicts bounds the verdict history (LW store 同款).
const d3RefAlignMaxKeptVerdicts = 50

// d3RefAlignIntent reports whether the message asks for the explicit D3
// reference-alignment face. Explicit D markers already route into the pack
// through the D1-D3 observation entry; these check-request phrases (same
// family shape as D1's 响度检查/峰值检查族 and D2's 翻译检查族) select the
// deep face inside the handler. 裸话题词（「参考曲」「spotify」「响度」单独
// 出现）不是检查请求，不在此列——裸话术入口按卡面走 semantic_entry 实测
// （263 教训），能过则不建词表。
func d3RefAlignIntent(message string) bool {
	text := strings.ToLower(strings.TrimSpace(message))
	if text == "" {
		return false
	}
	return textHasAny(text,
		"参考对齐", "参考曲对比", "参考曲对齐", "参考标准检查", "参考对比", "对齐检查",
		"交付标准检查", "交付对齐", "ref align", "reference align", "reference check",
		"delivery standard check", "align to the reference",
	)
}

// d3RefStandard is the resolved first-tier reference standard (a builtin
// delivery profile) plus where it came from. Basis: render_binding (projection
// D3 face) or intent_named (one-shot, no binding written).
type d3RefStandard struct {
	Profile  rlm.DeliveryProfile
	RenderID string
	Basis    string // "render_binding" | "intent_named"
}

// d3ResolveRefStandard resolves the alignment target, pure function:
//  1. first bound render→delivery binding on the RLM projection (render_
//     profile_bind 写端不在本 pack，这里只读消费投影已披露的绑定，不改写)；
//  2. otherwise a builtin profile named in the message (tier-1 lexical tier:
//     spotify/ebu/apple/aes/gy|国标；一次性判断，不写绑定面)；
//  3. otherwise no standard (unmeasurable face).
//
// Unresolvable bindings (projection discloses them fail-closed) never select a
// standard; they are surfaced in the unmeasurable reason instead.
func d3ResolveRefStandard(proj rlm.Projection, message string) (d3RefStandard, bool) {
	unresolvable := ""
	for _, binding := range proj.RenderBindings {
		if binding.Status != rlm.RenderBindingStatusBound {
			unresolvable = binding.ProfileID
			continue
		}
		profile := rlm.DeliveryProfile{}
		if binding.Profile != nil {
			profile = *binding.Profile
		} else if resolved, ok := rlm.LookupDeliveryProfile(binding.ProfileID); ok {
			profile = resolved
		} else {
			unresolvable = binding.ProfileID
			continue
		}
		return d3RefStandard{Profile: profile, RenderID: binding.RenderID, Basis: "render_binding"}, true
	}
	if profileID, ok := d3IntentNamedProfile(message); ok {
		if profile, resolved := rlm.LookupDeliveryProfile(profileID); resolved {
			return d3RefStandard{Profile: profile, Basis: "intent_named"}, true
		}
	}
	_ = unresolvable // 由调用面经投影 limitations 披露（rlm_render_profile_binding_unresolvable）
	return d3RefStandard{}, false
}

// d3IntentNamedProfile resolves a builtin delivery profile named in the check
// message (tier-1 reference-standard selection from user intent). Chinese
// markers match as substrings; Latin markers match with ASCII-letter
// boundaries (an "aesthetics" must not select aes, while 按apple/spotify的
// with CJK neighbors still match).
func d3IntentNamedProfile(message string) (string, bool) {
	text := strings.ToLower(message)
	if strings.Contains(text, "国标") || strings.Contains(text, "gy/t") {
		return "builtin:gy_282_2014", true
	}
	for _, marker := range []struct {
		token     string
		profileID string
	}{
		{"spotify", "builtin:spotify"},
		{"ebu", "builtin:ebu_r128"},
		{"apple", "builtin:apple_music"},
		{"aes", "builtin:aes_td1004"},
	} {
		if d3ContainsLatinToken(text, marker.token) {
			return marker.profileID, true
		}
	}
	return "", false
}

// d3ContainsLatinToken reports whether the lowercased text contains the ASCII
// token with non-ASCII-letter neighbors on both sides (CJK neighbors are
// allowed: the lexical tier mixes scripts).
func d3ContainsLatinToken(text, token string) bool {
	for start := 0; ; {
		index := strings.Index(text[start:], token)
		if index < 0 {
			return false
		}
		index += start
		end := index + len(token)
		if (index == 0 || !isASCIILetter(text[index-1])) && (end == len(text) || !isASCIILetter(text[end])) {
			return true
		}
		start = index + 1
	}
}

func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// d3RefGapRow is one quantified alignment gap (metric/measured/threshold plus
// the signed delta; positive = above the bound).
type d3RefGapRow struct {
	ClipID    string  `json:"clip_id,omitempty"`
	Metric    string  `json:"metric"`
	Measured  float64 `json:"measured"`
	Threshold float64 `json:"threshold"`
	DeltaDB   float64 `json:"delta_db"`
}

// d3RefAlignVerdict is the ref_align.v0 record. Pure data: the assessor
// leaves identity/trigger/时间戳 empty; the runtime fills them.
type d3RefAlignVerdict struct {
	SchemaVersion    string        `json:"schema_version"`
	Verdict          string        `json:"verdict"`
	ProfileID        string        `json:"profile_id,omitempty"`
	ProfileBasis     string        `json:"profile_basis,omitempty"`
	RenderID         string        `json:"render_id,omitempty"`
	MeasurementGrade string        `json:"measurement_grade"`
	BandMinLUFS      *float64      `json:"band_min_lufs,omitempty"`
	BandMaxLUFS      *float64      `json:"band_max_lufs"`
	TruePeakMaxDBTP  float64       `json:"true_peak_max_dbtp"`
	GapRows          []d3RefGapRow `json:"gap_rows,omitempty"`
	MeasuredClips    int           `json:"measured_clips"`
	PeakCheck        string        `json:"peak_check,omitempty"`
	// LWVerdict carries the delegated loudnesswatch verdict kinds for audit
	// (pass/breach/unmeasurable); the D3 vocabulary stays aligned/gap_found/
	// unmeasurable on the payload face.
	LWVerdict          string `json:"lw_verdict,omitempty"`
	UnmeasurableReason string `json:"unmeasurable_reason,omitempty"`
	Summary            string `json:"summary"`
}

// d3AssessRefAlignment is the pure criterion (zero I/O, no clock): the RLM
// projection (render bindings = D3 reference face) plus the loudness summary
// rows plus the user intent in, one three-state verdict out. 差距计算委托
// loudnesswatch.Compare（approximate 口径与 peak 必要条件语义继承，不另立
// 标准）；无参考标准（未绑定且意图未点名）/无测量行/行缺 integrated 值
// = unmeasurable 一等公民。
func d3AssessRefAlignment(proj rlm.Projection, rows []map[string]any, message string) (d3RefAlignVerdict, *loudnesswatch.Verdict) {
	verdict := d3RefAlignVerdict{SchemaVersion: d3RefAlignVerdictStoreSchemaVersion}
	standard, ok := d3ResolveRefStandard(proj, message)
	if !ok {
		verdict.Verdict = d3RefAlignVerdictUnmeasurable
		verdict.UnmeasurableReason = "no_reference_standard: 当前无 render→delivery 绑定，且检查意图未点名内置预设（第一档=内置参考标准；自定义参考音频上传属第二档另卡；绑定写端 render_profile_bind 不在本只读面）"
		verdict.Summary = "无法测量：" + verdict.UnmeasurableReason + "。"
		return verdict, nil
	}
	verdict.ProfileID = standard.Profile.ProfileID
	verdict.ProfileBasis = standard.Basis
	verdict.RenderID = standard.RenderID
	verdict.MeasurementGrade = loudnesswatch.MeasurementGradeApproximate
	verdict.BandMaxLUFS = &standard.Profile.Integrated.Max
	if standard.Profile.Integrated.Min != nil {
		min := *standard.Profile.Integrated.Min
		verdict.BandMinLUFS = &min
	}
	verdict.TruePeakMaxDBTP = standard.Profile.TruePeakMaxDBTP

	lwVerdict := loudnesswatch.Compare(rows, loudnesswatch.Config{
		Enabled:       true,
		ProfileSource: standard.Profile.ProfileID,
	})
	verdict.LWVerdict = lwVerdict.Verdict
	verdict.MeasuredClips = len(lwVerdict.Measured)
	verdict.PeakCheck = lwVerdict.PeakCheck
	for _, detail := range lwVerdict.BreachDetails {
		verdict.GapRows = append(verdict.GapRows, d3RefGapRow{
			ClipID: detail.ClipID, Metric: detail.Metric,
			Measured: detail.Measured, Threshold: detail.Threshold, DeltaDB: detail.DeltaDB,
		})
	}
	switch lwVerdict.Verdict {
	case loudnesswatch.VerdictBreach:
		verdict.Verdict = d3RefAlignVerdictGapFound
	case loudnesswatch.VerdictPass:
		verdict.Verdict = d3RefAlignVerdictAligned
	default:
		verdict.Verdict = d3RefAlignVerdictUnmeasurable
		verdict.UnmeasurableReason = lwVerdict.UnmeasurableReason
	}
	verdict.Summary = d3RefAlignSummary(verdict, standard)
	return verdict, &lwVerdict
}

// d3RefAlignSummary renders the user-visible verdict line: standard context
// first (which profile, where it came from), then the delegated approximate
// conclusion. The approximate qualifier is inherited verbatim from the LW
// summary discipline (never claims BS.1770 grade).
func d3RefAlignSummary(verdict d3RefAlignVerdict, standard d3RefStandard) string {
	basis := "意图点名（一次性判断，未写绑定）"
	if standard.Basis == "render_binding" {
		basis = "render 绑定（render " + standard.RenderID + "）"
	}
	head := fmt.Sprintf("参考标准 %s（%s；integrated band %s，true peak ≤ %+.1f dBTP）。近似口径结论：",
		standard.Profile.ProfileID, basis, d3BandText(verdict), standard.Profile.TruePeakMaxDBTP)
	switch verdict.Verdict {
	case d3RefAlignVerdictGapFound:
		return head + fmt.Sprintf("检出 %d 处对齐差距（已测 %d 载体）。", len(verdict.GapRows), verdict.MeasuredClips)
	case d3RefAlignVerdictAligned:
		return head + fmt.Sprintf("已测 %d 载体均在目标带内（未见越界）。", verdict.MeasuredClips)
	default:
		return head + "无法测量：" + verdict.UnmeasurableReason + "。"
	}
}

func d3BandText(verdict d3RefAlignVerdict) string {
	if verdict.BandMinLUFS != nil {
		return fmt.Sprintf("[%+.1f, %+.1f] LUFS", *verdict.BandMinLUFS, *verdict.BandMaxLUFS)
	}
	return fmt.Sprintf("≤ %+.1f LUFS", *verdict.BandMaxLUFS)
}

// d3RefAlignVerdictRecord is one stored verdict plus trigger telemetry (LW
// VerdictRecord 同款形态).
type d3RefAlignVerdictRecord struct {
	Key             string            `json:"key"`
	ProjectUUID     string            `json:"project_uuid,omitempty"`
	ProjectRevision string            `json:"project_revision,omitempty"`
	VerdictKind     string            `json:"verdict_kind"`
	Verdict         d3RefAlignVerdict `json:"verdict"`
	TriggerKind     string            `json:"trigger_kind"`
	TriggerRef      string            `json:"trigger_ref,omitempty"`
	RecordedAt      string            `json:"recorded_at"`
	LastRecheckedAt string            `json:"last_rechecked_at,omitempty"`
	RecheckCount    int               `json:"recheck_count,omitempty"`
	GapCount        int               `json:"gap_count"`
	MeasuredClips   int               `json:"measured_clips"`
	ProfileID       string            `json:"profile_id,omitempty"`
	ProfileBasis    string            `json:"profile_basis,omitempty"`
}

type d3RefAlignVerdictStoreFile struct {
	SchemaVersion string                    `json:"schema_version"`
	UpdatedAt     string                    `json:"updated_at"`
	ChecksTotal   int64                     `json:"checks_total"`
	Verdicts      []d3RefAlignVerdictRecord `json:"verdicts"`
}

// d3RefAlignVerdictStoreMu serializes store file access in-process (LW/D2 同款).
var d3RefAlignVerdictStoreMu sync.Mutex

// d3RefAlignDedupeKey mirrors the LW pinned dedupe key: project uuid + project
// revision + verdict kind.
func d3RefAlignDedupeKey(projectUUID, projectRevision, verdictKind string) string {
	return projectUUID + "|" + projectRevision + "|" + verdictKind
}

// d3RefAlignVerdictStorePath colocates the ref-align verdict history with the
// mixboard feature snapshot of the current project binding (loudness_watch_
// verdicts.json / stereo_check_verdicts.json 同目录).
func d3RefAlignVerdictStorePath() string {
	return filepath.Join(filepath.Dir(mixboard.FeatureSnapshotPath(map[string]any{})), "ref_align_verdicts.json")
}

// recordD3RefAlignVerdict persists one verdict with the LW store discipline:
// load-once-per-call, foreign schema fail-closed to an empty history (AGENTS
// §11), dedupe by pinned key (rechecks bump the audit counters, never
// duplicate), max-kept trim, write-through. Failures WARN through the logger
// and never fail the check response (observation parasite, LW 同款); the
// dedupe key is returned regardless so the queue-node evidence stays stable.
func recordD3RefAlignVerdict(path string, record d3RefAlignVerdictRecord, now time.Time, warn func(format string, args ...any)) string {
	d3RefAlignVerdictStoreMu.Lock()
	defer d3RefAlignVerdictStoreMu.Unlock()
	store := d3RefAlignVerdictStoreFile{SchemaVersion: d3RefAlignVerdictStoreSchemaVersion}
	if data, err := os.ReadFile(path); err == nil {
		loaded := d3RefAlignVerdictStoreFile{}
		if err := json.Unmarshal(data, &loaded); err != nil {
			warn("ref align verdict store unmarshal failed path=%s err=%v (starting an empty history)", path, err)
		} else if loaded.SchemaVersion != "" && loaded.SchemaVersion != d3RefAlignVerdictStoreSchemaVersion {
			warn("ref align verdict store: schema version %q is not %q (fail closed to empty history)", loaded.SchemaVersion, d3RefAlignVerdictStoreSchemaVersion)
		} else {
			if loaded.SchemaVersion == "" {
				loaded.SchemaVersion = d3RefAlignVerdictStoreSchemaVersion
			}
			store = loaded
		}
	}
	key := d3RefAlignDedupeKey(record.ProjectUUID, record.ProjectRevision, record.VerdictKind)
	record.Key = key
	stamp := now.UTC().Format(time.RFC3339Nano)
	store.SchemaVersion = d3RefAlignVerdictStoreSchemaVersion
	store.UpdatedAt = stamp
	store.ChecksTotal++
	deduped := false
	for index := range store.Verdicts {
		if store.Verdicts[index].Key == key {
			store.Verdicts[index].LastRecheckedAt = stamp
			store.Verdicts[index].RecheckCount++
			store.Verdicts[index].TriggerKind = record.TriggerKind
			store.Verdicts[index].TriggerRef = record.TriggerRef
			store.Verdicts[index].GapCount = record.GapCount
			store.Verdicts[index].MeasuredClips = record.MeasuredClips
			store.Verdicts[index].ProfileID = record.ProfileID
			store.Verdicts[index].ProfileBasis = record.ProfileBasis
			store.Verdicts[index].Verdict = record.Verdict
			deduped = true
			break
		}
	}
	if !deduped {
		record.RecordedAt = stamp
		store.Verdicts = append(store.Verdicts, record)
	}
	if len(store.Verdicts) > d3RefAlignMaxKeptVerdicts {
		store.Verdicts = store.Verdicts[len(store.Verdicts)-d3RefAlignMaxKeptVerdicts:]
	}
	if path == "" {
		return key
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		warn("ref align verdict store mkdir failed path=%s err=%v", path, err)
		return key
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		warn("ref align verdict store marshal failed err=%v", err)
		return key
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		warn("ref align verdict store write failed path=%s err=%v", path, err)
	}
	return key
}

// handleD3RefAlignCheck runs the D3 reference-alignment face on top of the
// already-built D1-D3 observation report and RLM projection. read_only for the
// project until a gap parks the governed gain card: the only state this face
// writes directly is the ref-align verdict store (observation bookkeeping) and
// the D3 queue-node annotation; any project mutation goes exclusively through
// the pending-confirmation mix-tick chain (D1 settle 同款，零裸写).
func (s *Server) handleD3RefAlignCheck(ctx context.Context, conversationID string, req ChatRequest, goal agentruntime.Goal, sessionID string, state *kernel.VSPStateResult, report d1d3PreMasterReport, rlmProjection rlm.Projection) ChatResponse {
	rows := readLoudnessSummariesFromFeatureSnapshot()
	verdict, lwVerdict := d3AssessRefAlignment(rlmProjection, rows, req.Message)

	projectRevision := ""
	if state != nil && state.Revision > 0 {
		projectRevision = fmt.Sprintf("%d", state.Revision)
	}
	record := d3RefAlignVerdictRecord{
		ProjectUUID:     s.d1SettleProjectIdentity(ctx),
		ProjectRevision: projectRevision,
		VerdictKind:     verdict.Verdict,
		Verdict:         verdict,
		TriggerKind:     d3RefAlignTriggerKind,
		TriggerRef:      sessionID,
		GapCount:        len(verdict.GapRows),
		MeasuredClips:   verdict.MeasuredClips,
		ProfileID:       verdict.ProfileID,
		ProfileBasis:    verdict.ProfileBasis,
	}
	storeKey := recordD3RefAlignVerdict(d3RefAlignVerdictStorePath(), record, time.Now(), s.warnD3RefAlign)

	// 动作面（卡面口径：纯电平类建议可走既有 gain 动作则挂卡）：gap_found
	// 且 SuggestGainAction 能定位最越界载体 → 一条有界推子建议挂既有
	// mix_tick 确认链（D1 settle 同款；确认前零 mutation）；建议 nil（无
	// 可定位载体/剂量钳到零）→ 纯披露。aligned/unmeasurable 不产动作。
	var suggestion *loudnesswatch.ActionSuggestion
	if verdict.Verdict == d3RefAlignVerdictGapFound && lwVerdict != nil {
		suggestion = loudnesswatch.SuggestGainAction(*lwVerdict, rows, experiment.D1S1MaxAbsDeltaDB)
	}
	actionFace, actionReason := d3RefAlignActionFace(verdict, suggestion)

	workflowData := map[string]any{
		"session_id":          sessionID,
		"capability_id":       d3RefAlignCapabilityID,
		"canary_stage":        "d3_ref_align",
		"read_only":           true,
		"mutation_performed":  false,
		"schema":              d1d3ObservationSchema,
		"observation":         report,
		"d3_ref_align":        d3RefAlignPayload(verdict, storeKey, actionFace, actionReason),
		"observation_carrier": premasterObservationCapabilityID,
	}
	response := ChatResponse{
		ConversationID: conversationID,
		GoalID:         goal.GoalID,
		RunID:          goal.RunID,
		Reply:          d3RefAlignReply(verdict, report, actionFace, actionReason, suggestion),
		Workflow:       "capability_runtime_v1",
		WorkflowData:   workflowData,
		GoalStatus:     string(agentruntime.StatusCompleted),
		StopReason:     "d3_ref_align_checked",
	}
	if suggestion != nil {
		candidate := d3RefAlignPendingCandidate(verdict, suggestion, report, conversationID)
		autoApply := authorityModeFromContext(req.Context) == experiment.AuthorityFull
		s.storePendingMixTickCandidateForMode(conversationID, goal.GoalID, goal.RunID, candidate, autoApply)
		if autoApply {
			executed := s.executePendingMixTickCandidate(ctx, conversationID, req, agentModeFromContext(req.Context), candidate)
			executed.WorkflowData = mergeContext(executed.WorkflowData, map[string]any{
				"capability_id": d3RefAlignCapabilityID, "full_access_auto_authorized": true,
			})
			s.writeD3RefAlignQueueNode(conversationID, verdict, storeKey, "in_progress")
			return executed
		}
		request := mixTickInteractionRequest(conversationID, goal.GoalID, goal.RunID, candidate)
		s.storePendingInteraction(request, request.Payload)
		response.InteractionRequests = append(response.InteractionRequests, request)
		response.NeedsConfirmation = true
		workflowData["pending_action"] = suggestion
		workflowData["mutation_performed"] = false
	}
	// 点亮：检查已跑 = in_progress；挂了卡 = needs_confirmation（上方 full
	// access 分支已写 in_progress）。
	nodeState := "in_progress"
	if suggestion != nil && authorityModeFromContext(req.Context) != experiment.AuthorityFull {
		nodeState = "needs_confirmation"
	}
	s.writeD3RefAlignQueueNode(conversationID, verdict, storeKey, nodeState)
	return response
}

// d3RefAlignActionFace pins the action-face value + honest reason per outcome.
func d3RefAlignActionFace(verdict d3RefAlignVerdict, suggestion *loudnesswatch.ActionSuggestion) (string, string) {
	switch {
	case suggestion != nil:
		return d3RefAlignActionGainCard, ""
	case verdict.Verdict == d3RefAlignVerdictGapFound:
		return d3RefAlignActionDisclosureOnly, d3RefAlignNoAddressableReason
	case verdict.Verdict == d3RefAlignVerdictAligned:
		return d3RefAlignActionNoneNeeded, "对齐目标带内，无需动作"
	default:
		return d3RefAlignActionDisclosureOnly, d3RefAlignUnmeasurableActionNote
	}
}

// d3RefAlignPendingCandidate builds the governed pending-confirmation
// candidate from the gap suggestion (v1 domain: bounded track_gain_adjust,
// D1 settle 同款操作族).
func d3RefAlignPendingCandidate(verdict d3RefAlignVerdict, suggestion *loudnesswatch.ActionSuggestion, report d1d3PreMasterReport, conversationID string) agentloop.PendingMixTickCandidate {
	evidence := map[string]any{
		"source":          "d3_ref_align",
		"capability_id":   d3RefAlignCapabilityID,
		"verdict_kind":    verdict.Verdict,
		"metric":          suggestion.Metric,
		"measured":        suggestion.Measured,
		"threshold":       suggestion.Threshold,
		"breach_delta_db": suggestion.Measured - suggestion.Threshold,
		"config_basis":    "参考标准 " + verdict.ProfileID + "（" + verdict.ProfileBasis + "）",
	}
	evidence["verdict_summary"] = verdict.Summary
	evidence["clip_id"] = suggestion.ClipID
	fingerprint := map[string]any{
		"conversation_id": conversationID,
		"target_scope":    "selected_track",
		"mix_session_id":  "d3_ref_align_" + report.ObservationID,
		"config_basis":    evidence["config_basis"],
	}
	return agentloop.PendingMixTickCandidate{
		Operation:                 loudnesswatch.ActionOperationGainAdjust,
		TrackID:                   suggestion.TrackID,
		DeltaDB:                   suggestion.DeltaDB,
		ObservationID:             report.ObservationID,
		Evidence:                  evidence,
		CreatedFromReply:          "D3 参考曲对齐差距建议",
		ExpiresAfterContextChange: true,
		Status:                    "pending_confirmation",
		Fingerprint:               fingerprint,
	}
}

func (s *Server) warnD3RefAlign(format string, args ...any) {
	if s != nil && s.logger != nil {
		s.logger.Warn("[d3.ref_align] "+format, args...)
	}
}

// d3RefAlignPayload is the structured disclosure for WorkflowData.
func d3RefAlignPayload(verdict d3RefAlignVerdict, storeKey, actionFace, actionReason string) map[string]any {
	payload := map[string]any{
		"capability_id":     d3RefAlignCapabilityID,
		"checked":           true,
		"verdict":           verdict.Verdict,
		"verdict_summary":   verdict.Summary,
		"gap_count":         len(verdict.GapRows),
		"measured_clips":    verdict.MeasuredClips,
		"measurement_grade": verdict.MeasurementGrade,
		"action_face":       actionFace,
		"verdict_store_key": storeKey,
	}
	if actionReason != "" {
		payload["action_face_reason"] = actionReason
	}
	if verdict.ProfileID != "" {
		payload["profile_id"] = verdict.ProfileID
		payload["profile_basis"] = verdict.ProfileBasis
	}
	if verdict.RenderID != "" {
		payload["render_id"] = verdict.RenderID
	}
	if verdict.BandMaxLUFS != nil {
		payload["band_max_lufs"] = *verdict.BandMaxLUFS
	}
	if verdict.BandMinLUFS != nil {
		payload["band_min_lufs"] = *verdict.BandMinLUFS
	}
	if verdict.PeakCheck != "" {
		payload["peak_check"] = verdict.PeakCheck
	}
	if verdict.UnmeasurableReason != "" {
		payload["unmeasurable_reason"] = verdict.UnmeasurableReason
	}
	return payload
}

// d3RefAlignReply renders the user-visible alignment report: standard context,
// the delegated verdict summary, and the honest action-face boundary (card
// pending-confirmation or disclosure).
func d3RefAlignReply(verdict d3RefAlignVerdict, report d1d3PreMasterReport, actionFace, actionReason string, suggestion *loudnesswatch.ActionSuggestion) string {
	lines := []string{"D3 参考曲对齐检查（第一档=内置参考标准，schema " + d1d3ObservationSchema + " 载体，只读判断）。"}
	if verdict.ProfileID == "" {
		lines = append(lines, "参考标准解析："+verdict.UnmeasurableReason+"。")
	} else {
		basis := "意图点名（一次性判断，未写绑定）"
		if verdict.ProfileBasis == "render_binding" {
			basis = "render 绑定（render " + verdict.RenderID + "）"
		}
		lines = append(lines, fmt.Sprintf("参考标准：%s（%s）。", verdict.ProfileID, basis))
	}
	lines = append(lines, "判断结果："+verdict.Summary)
	if verdict.Verdict == d3RefAlignVerdictGapFound {
		for _, gap := range verdict.GapRows {
			lines = append(lines, fmt.Sprintf("差距 %s：%s %.1f 对界限 %.1f（%+.1f dB）。", gap.ClipID, gap.Metric, gap.Measured, gap.Threshold, gap.DeltaDB))
		}
	}
	switch {
	case suggestion != nil:
		lines = append(lines, "对齐建议（待确认，未改工程）："+suggestion.Basis+"。")
		lines = append(lines, "确认后我会通过 mix.propose_tick → mix.apply_tick 执行这一步；确认前不会修改工程。")
	case actionFace == d3RefAlignActionDisclosureOnly:
		lines = append(lines, "本次不生成写操作确认卡："+actionReason+"。未修改工程。")
	default:
		lines = append(lines, "无需动作，未修改工程。")
	}
	lines = append(lines, "检查结果已记入 ref align verdict 记录；D3 节点状态可在队列视图查看。")
	return strings.Join(lines, "\n")
}

// writeD3RefAlignQueueNode advances the workflow queue's D3 node out of
// not_started (annotation-face discipline: in_progress / needs_confirmation
// only — completion stays ledger-derived). A write failure WARNs and never
// blocks the check response.
func (s *Server) writeD3RefAlignQueueNode(conversationID string, verdict d3RefAlignVerdict, storeKey, nodeState string) {
	reason := "D3 参考曲对齐检查已执行（" + verdict.Verdict + "）"
	evidence := []string{"d3_ref_align:" + d3RefAlignCapabilityID}
	if storeKey != "" {
		evidence = append(evidence, "ref_align_verdict:"+storeKey)
	}
	nodeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := s.WriteMixWorkflowQueueNodeState(nodeCtx, "D3", nodeState, reason, evidence, conversationID); err != nil {
		if s.logger != nil {
			s.logger.Warn("[d3.ref_align] queue node annotation failed conversation=%s err=%v", conversationID, err)
		}
	} else if s.logger != nil {
		s.logger.Info("[d3.ref_align] D3 node advanced conversation=%s state=%s verdict=%s", conversationID, nodeState, verdict.Verdict)
	}
}

// 命令面：analyze_midi_transformations（工具名 midi.analyze_transformations，
// RiskDirect 只读）。数据全部来自既有内核命令（get_project_state 的
// music_context、get_midi_clip_data、get_midi_clip_notes），零内核改动；
// 计算复用本包检测核（DetectTransformations），本文件不含检测逻辑。
//
// 返回结构按 256 报告点①草案（EDU-CAP-RECON-1）+ 段 1 追认（sequence_quality
// 双语义），llm_context 字段族借 EPM 惯例（epm/types.go LLMContext：
// summary_md / compact_facts / do_not_include_raw_package / evidence_refs /
// limitation_notes / suggested_next_step；Confidence 双字段惯例 =
// confidence 字符串 + confidence_score 数值 + basis）。
//
// bar 守门（草案点②）：music_context.tempo_setting_count==1 且 tempo/拍号
// 可用且 clip 全局起点可得时，findings[].segments[].bar_start/bar_end 才有
// 值；任一条件不满足时 bar 字段为 null 并在 limitations 披露原因。
// 多段拍号（time_sig_setting_count>1）同样使 bar 换算不可靠，一并守门。
//
// 输入解析 fail-closed：未知 transformations 值、非法 key、min_match_notes<2、
// 逆序/零宽 range_beats 均返回错误，不静默纠正。
package edu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// SchemaVersion 是本命令返回结构的版本号（消费方=乐局卡/编曲 SYMLINK）。
const AnalyzeTransformationsSchemaVersion = "midi.analyze_transformations.v0"

// KernelSendFunc 发送一条内核命令并返回回复 map。由调用方（harness 挂线）
// 提供；本包通过它取 get_project_state / get_midi_clip_data /
// get_midi_clip_notes 三个既有只读命令的真值。
type KernelSendFunc func(ctx context.Context, cmd map[string]any) (map[string]any, error)

// analyzeRequest 是解析后的命令输入（零值语义已在解析阶段补全）。
type analyzeRequest struct {
	ClipID          string
	TrackID         string // 可选；空 = 不传
	RangeStart      float64
	RangeEnd        float64
	HasRange        bool
	Transformations []Kind // nil = 全族
	MinMatchNotes   int    // 已补默认 3
	Key             *Key   // nil = 不启用 diatonic
}

// parseAnalyzeArgs 解析并校验命令参数（fail-closed）。
func parseAnalyzeArgs(args map[string]any) (analyzeRequest, error) {
	var req analyzeRequest
	if args == nil {
		return req, fmt.Errorf("analyze_midi_transformations: args required")
	}
	req.ClipID = strings.TrimSpace(stringOfAny(args["clip_id"]))
	if req.ClipID == "" {
		return req, fmt.Errorf("analyze_midi_transformations: clip_id is required")
	}
	req.TrackID = strings.TrimSpace(stringOfAny(args["track_id"]))

	if raw := args["range_beats"]; raw != nil {
		pair, ok := raw.([]any)
		if !ok || len(pair) != 2 {
			return req, fmt.Errorf("analyze_midi_transformations: range_beats must be [start_beats, end_beats]")
		}
		start, okS := numberOfAny(pair[0])
		end, okE := numberOfAny(pair[1])
		if !okS || !okE {
			return req, fmt.Errorf("analyze_midi_transformations: range_beats entries must be numbers")
		}
		if start < 0 || end <= start {
			return req, fmt.Errorf("analyze_midi_transformations: range_beats requires 0 <= start < end (got [%v, %v])", start, end)
		}
		req.RangeStart, req.RangeEnd, req.HasRange = start, end, true
	}

	if raw := args["transformations"]; raw != nil {
		list, ok := raw.([]any)
		if !ok {
			return req, fmt.Errorf("analyze_midi_transformations: transformations must be an array of repeat|sequence|inversion|retrograde")
		}
		seen := map[Kind]bool{}
		for _, item := range list {
			name := strings.TrimSpace(stringOfAny(item))
			k := Kind(name)
			switch k {
			case KindRepeat, KindSequence, KindInversion, KindRetrograde:
			default:
				return req, fmt.Errorf("analyze_midi_transformations: unknown transformation %q (valid: repeat, sequence, inversion, retrograde)", name)
			}
			seen[k] = true
		}
		// 空数组视为省略（全族），与省略语义一致并在返回中披露。
		for _, k := range []Kind{KindRepeat, KindSequence, KindInversion, KindRetrograde} {
			if seen[k] {
				req.Transformations = append(req.Transformations, k)
			}
		}
	}

	if raw := args["min_match_notes"]; raw != nil {
		v, ok := numberOfAny(raw)
		if !ok || v != math.Trunc(v) {
			return req, fmt.Errorf("analyze_midi_transformations: min_match_notes must be an integer")
		}
		if v != 0 { // 0 与省略同义（默认 3）
			n := int(v)
			if n < 2 {
				return req, fmt.Errorf("analyze_midi_transformations: min_match_notes must be >= 2 (single notes carry no interval; omit or 0 for default 3)")
			}
			req.MinMatchNotes = n
		}
	}
	if req.MinMatchNotes == 0 {
		req.MinMatchNotes = 3
	}

	if raw := args["key"]; raw != nil {
		obj, ok := raw.(map[string]any)
		if !ok {
			return req, fmt.Errorf("analyze_midi_transformations: key must be an object {tonic: 0-11 pitch class, mode: \"major\"|omitted}")
		}
		tonic, okT := numberOfAny(obj["tonic"])
		if !okT || tonic != math.Trunc(tonic) || tonic < 0 || tonic > 11 {
			return req, fmt.Errorf("analyze_midi_transformations: key.tonic must be an integer pitch class 0-11")
		}
		mode := strings.TrimSpace(stringOfAny(obj["mode"]))
		if mode != "" && mode != "major" {
			return req, fmt.Errorf("analyze_midi_transformations: key.mode only supports \"major\" (leave empty or set \"major\")")
		}
		req.Key = &Key{Tonic: int(tonic), Mode: mode}
	}
	return req, nil
}

// musicContextSnapshot 是从 get_project_state music_context 披露块提取的
// 换算依据（字段名与内核 appendMusicContextDisclosure 一致）。
type musicContextSnapshot struct {
	TempoBPM               float64
	TempoSettingCount      int
	TimeSignature          string
	Numerator              int
	Denominator            int
	TimeSigSettingCount    int // 0 = 内核未披露
	HasMusicContext        bool
	HasTempo               bool
	HasTimeSignatureString bool
}

func parseMusicContext(state map[string]any) musicContextSnapshot {
	var snap musicContextSnapshot
	mc, ok := state["music_context"].(map[string]any)
	if !ok {
		return snap
	}
	snap.HasMusicContext = true
	if v, ok := numberOfAny(mc["tempo_bpm"]); ok {
		snap.TempoBPM = v
		snap.HasTempo = true
	}
	if v, ok := intOfAny(mc["tempo_setting_count"]); ok {
		snap.TempoSettingCount = v
	}
	if s := stringOfAny(mc["time_signature"]); s != "" {
		snap.TimeSignature = s
		snap.HasTimeSignatureString = true
	}
	snap.Numerator, _ = intOfAny(mc["time_signature_numerator"])
	snap.Denominator, _ = intOfAny(mc["time_signature_denominator"])
	snap.TimeSigSettingCount, _ = intOfAny(mc["time_sig_setting_count"])
	return snap
}

// AnalyzeTransformationsCommand 执行 analyze_midi_transformations：
// 取内核真值 → 按 range_beats 分窗 → 调检测核 → bar 守门 → 组装返回。
// 确定性：同输入（含同一内核数据）同输出，analysis_id 为输入内容的稳定哈希。
func AnalyzeTransformationsCommand(ctx context.Context, send KernelSendFunc, args map[string]any) (map[string]any, error) {
	if send == nil {
		return nil, fmt.Errorf("analyze_midi_transformations: requires a kernel command sender")
	}
	req, err := parseAnalyzeArgs(args)
	if err != nil {
		return nil, err
	}

	// 1) music_context（失败不致命：bar 守门降级 + limitation 披露）。
	stateReply, stateErr := send(ctx, map[string]any{"cmd": "get_project_state"})
	mc := musicContextSnapshot{}
	if stateErr == nil && kernelReplyOK(stateReply) {
		mc = parseMusicContext(stateReply)
	}

	// 2) clip 全局起点（失败不致命：同上降级）。
	clipDataCmd := map[string]any{"cmd": "get_midi_clip_data", "clip_id": req.ClipID}
	if req.TrackID != "" {
		clipDataCmd["track_id"] = req.TrackID
	}
	clipOriginKnown := false
	var clipOriginBeats float64
	if reply, err := send(ctx, clipDataCmd); err == nil && kernelReplyOK(reply) {
		if startSec, ok := numberOfAny(reply["start_seconds"]); ok && startSec >= 0 && mc.HasTempo && mc.TempoSettingCount == 1 && mc.TempoBPM > 0 {
			clipOriginBeats = startSec * mc.TempoBPM / 60.0
			clipOriginKnown = true
		}
	}

	// 3) notes（失败致命：无数据即无分析）。
	notesCmd := map[string]any{"cmd": "get_midi_clip_notes", "clip_id": req.ClipID}
	if req.TrackID != "" {
		notesCmd["track_id"] = req.TrackID
	}
	notesReply, err := send(ctx, notesCmd)
	if err != nil {
		return nil, fmt.Errorf("analyze_midi_transformations: get_midi_clip_notes failed: %w", err)
	}
	if !kernelReplyOK(notesReply) {
		return nil, fmt.Errorf("analyze_midi_transformations: get_midi_clip_notes failed: %s", kernelReplyError(notesReply))
	}
	notes, err := parseClipNotes(notesReply)
	if err != nil {
		return nil, err
	}
	echoTrackID := req.TrackID
	if echoTrackID == "" {
		echoTrackID = strings.TrimSpace(stringOfAny(notesReply["track_id"]))
	}

	// 4) 分窗 + 检测核。
	windowed := notes
	if req.HasRange {
		windowed = windowNotes(notes, req.RangeStart, req.RangeEnd)
	}
	findings, err := DetectTransformations(windowed, DetectOptions{
		Transformations: req.Transformations,
		MinMatchNotes:   req.MinMatchNotes,
		Key:             req.Key,
	})
	if err != nil {
		return nil, fmt.Errorf("analyze_midi_transformations: %w", err)
	}

	// 5) bar 守门判定（守门条件全部来自换算依据本身，fail-honest：
	// 任一依据缺失/多段时 bar 字段为 null，原因写入 limitations）。
	barReason := ""
	switch {
	case !mc.HasMusicContext:
		barReason = "get_project_state 未返回 music_context"
	case !mc.HasTempo || mc.TempoBPM <= 0:
		barReason = "music_context 缺少可用 tempo_bpm"
	case mc.TempoSettingCount <= 0:
		barReason = "music_context 未披露 tempo_setting_count（无法确认恒定 tempo）"
	case mc.TempoSettingCount > 1:
		barReason = fmt.Sprintf("工程 tempo_setting_count=%d（多段 tempo 下 beat 换算非线性，首版仅支持恒定 tempo）", mc.TempoSettingCount)
	case mc.Numerator <= 0 || mc.Denominator <= 0:
		barReason = "music_context 拍号（time_signature_numerator/denominator）不可用"
	case mc.TimeSigSettingCount > 1:
		barReason = fmt.Sprintf("工程 time_sig_setting_count=%d（多段拍号下 bar 换算不可靠）", mc.TimeSigSettingCount)
	case !clipOriginKnown:
		barReason = "clip 全局起点不可得（get_midi_clip_data 失败或缺 start_seconds/tempo）"
	}
	barsUsable := barReason == ""

	// 6) 组装返回（typed struct → map，字段名以 json tag 为契约）。
	resp := buildAnalyzeResponse(req, mc, clipOriginKnown, clipOriginBeats, barsUsable, barReason, windowed, findings, echoTrackID)
	return responseToMap(resp)
}

// windowNotes 返回 start 落在 [start,end)（半开窗口，端点带 1e-6 容差：
// 恰好落在 end 上的音符属于下一窗口，不纳入）内的音符，保持原相对顺序
// （检测核内部会再排序）。
func windowNotes(notes []Note, start, end float64) []Note {
	out := make([]Note, 0, len(notes))
	for _, n := range notes {
		if n.Start >= start-beatEpsilon && n.Start < end-beatEpsilon {
			out = append(out, n)
		}
	}
	return out
}

// parseClipNotes 解析 get_midi_clip_notes 回复的 notes[]（id/start/length/
// pitch）。行内缺字段或越界 fail-closed 报错，不静默跳过。
func parseClipNotes(reply map[string]any) ([]Note, error) {
	rows, ok := reply["notes"].([]any)
	if !ok {
		return nil, fmt.Errorf("analyze_midi_transformations: get_midi_clip_notes reply has no notes[] array")
	}
	notes := make([]Note, 0, len(rows))
	for i, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("analyze_midi_transformations: notes[%d] is not an object", i)
		}
		id := strings.TrimSpace(stringOfAny(row["id"]))
		if id == "" {
			return nil, fmt.Errorf("analyze_midi_transformations: notes[%d] has empty id", i)
		}
		pitchF, okP := numberOfAny(row["pitch"])
		startF, okS := numberOfAny(row["start"])
		lengthF, okL := numberOfAny(row["length"])
		if !okP || !okS || !okL {
			return nil, fmt.Errorf("analyze_midi_transformations: notes[%s] missing pitch/start/length", id)
		}
		if pitchF != math.Trunc(pitchF) || pitchF < 0 || pitchF > 127 {
			return nil, fmt.Errorf("analyze_midi_transformations: notes[%s] pitch %v outside 0-127", id, pitchF)
		}
		if startF < 0 || lengthF < 0 {
			return nil, fmt.Errorf("analyze_midi_transformations: notes[%s] negative start/length", id)
		}
		notes = append(notes, Note{ID: id, Pitch: int(pitchF), Start: startF, Length: lengthF})
	}
	return notes, nil
}

// ---- 返回结构（字段名 = 256 草案 + 段 1 追认，EPM 惯例字段族） ----

// AnalyzeResponse 是命令返回的顶层结构。
type AnalyzeResponse struct {
	Status            string            `json:"status"`
	SchemaVersion     string            `json:"schema_version"`
	AnalysisID        string            `json:"analysis_id"`
	ClipID            string            `json:"clip_id"`
	TrackID           string            `json:"track_id"`
	TimeUnit          string            `json:"time_unit"`
	AnalyzedNoteCount int               `json:"analyzed_note_count"`
	MusicContextEcho  MusicContextEcho  `json:"music_context_echo"`
	ClipOrigin        AnalyzeClipOrigin `json:"clip_origin"`
	Findings          []AnalyzeFinding  `json:"findings"`
	LLMContext        AnalyzeLLMContext `json:"llm_context"`
	Limitations       []string          `json:"limitations"`
}

// MusicContextEcho 回显换算依据（来源=get_project_state music_context）。
// 指针字段：null = 内核未披露或不可用（bar 守门原因见 limitations）。
type MusicContextEcho struct {
	TempoBPM                 *float64 `json:"tempo_bpm"`
	TimeSignature            *string  `json:"time_signature"`
	TimeSignatureNumerator   *int     `json:"time_signature_numerator"`
	TimeSignatureDenominator *int     `json:"time_signature_denominator"`
	TempoSettingCount        *int     `json:"tempo_setting_count"`
	TimeSigSettingCount      *int     `json:"time_sig_setting_count,omitempty"`
}

// AnalyzeClipOrigin 是 clip 全局 beat 起点（秒×bpm/60 换算，恒定 tempo 下）。
type AnalyzeClipOrigin struct {
	StartBeatsGlobal *float64 `json:"start_beats_global"`
}

// AnalyzeFinding 是 bar 锚定的一条结构判定（段 1 Finding 的命令面投影）。
type AnalyzeFinding struct {
	Kind                   Kind             `json:"kind"`
	SequenceQuality        SequenceQuality  `json:"sequence_quality,omitempty"` // 仅 sequence：chromatic|diatonic
	Label                  string           `json:"label"`
	Segments               []AnalyzeSegment `json:"segments"`
	IntervalPattern        []int            `json:"interval_pattern"`
	ShiftSemitones         int              `json:"shift_semitones"` // sequence=首音半音差；repeat=0
	DiatonicSteps          int              `json:"diatonic_steps,omitempty"`
	InversionAxisSemitones int              `json:"inversion_axis_semitones,omitempty"`
	Confidence             string           `json:"confidence"` // high|medium|low（本检测核只产精确匹配，恒 high）
	ConfidenceScore        float64          `json:"confidence_score"`
	Basis                  []string         `json:"basis"`
	RequiresConfirmation   bool             `json:"requires_confirmation"` // 纯只读分析，恒 false（对齐 EPM 惯例）
}

// AnalyzeSegment 是 finding 中参与对比的一个段（原型或匹配）。
type AnalyzeSegment struct {
	Role        string   `json:"role"` // prototype | match
	StartBeats  float64  `json:"start_beats"`
	LengthBeats float64  `json:"length_beats"`
	BarStart    *int     `json:"bar_start"` // 1-based；拍号/tempo 不可靠时为 null
	BarEnd      *int     `json:"bar_end"`
	NoteIDs     []string `json:"note_ids"`
}

// AnalyzeLLMContext 借 EPM LLMContext 字段族（epm/types.go）。
type AnalyzeLLMContext struct {
	SummaryMD              string           `json:"summary_md"`
	CompactFacts           []map[string]any `json:"compact_facts"`
	DoNotIncludeRawPackage bool             `json:"do_not_include_raw_package"`
	EvidenceRefs           []string         `json:"evidence_refs"`
	LimitationNotes        []string         `json:"limitation_notes"`
	SuggestedNextStep      string           `json:"suggested_next_step"`
}

func buildAnalyzeResponse(req analyzeRequest, mc musicContextSnapshot, clipOriginKnown bool, clipOriginBeats float64, barsUsable bool, barReason string, windowed []Note, findings []Finding, echoTrackID string) AnalyzeResponse {
	limitations := []string{
		"单声部假设：多轨合并导入的 clip 未分离声部，声部交织可能产生伪匹配或漏配",
	}
	if barReason != "" {
		limitations = append(limitations, "bar 坐标未换算："+barReason)
	}
	if req.Key == nil {
		limitations = append(limitations, "调内模进（sequence/diatonic）未启用：未提供 key 参数")
	}
	if len(windowed) == 0 {
		limitations = append(limitations, "分析窗口内无音符（findings 为空）")
	}

	outFindings := make([]AnalyzeFinding, 0, len(findings))
	for _, f := range findings {
		outFindings = append(outFindings, buildAnalyzeFinding(f, clipOriginKnown, clipOriginBeats, barsUsable, mc))
	}

	resp := AnalyzeResponse{
		Status:            "ok",
		SchemaVersion:     AnalyzeTransformationsSchemaVersion,
		AnalysisID:        analyzeID(req, windowed, findings),
		ClipID:            req.ClipID,
		TrackID:           echoTrackID,
		TimeUnit:          "beats",
		AnalyzedNoteCount: len(windowed),
		Findings:          outFindings,
		Limitations:       limitations,
	}
	resp.MusicContextEcho = buildMusicContextEcho(mc)
	if clipOriginKnown {
		v := clipOriginBeats
		resp.ClipOrigin.StartBeatsGlobal = &v
	}
	resp.LLMContext = buildAnalyzeLLMContext(req.ClipID, req, outFindings, len(windowed), barsUsable, limitations)
	return resp
}

func buildMusicContextEcho(mc musicContextSnapshot) MusicContextEcho {
	echo := MusicContextEcho{}
	if mc.HasTempo {
		v := mc.TempoBPM
		echo.TempoBPM = &v
	}
	if mc.HasTimeSignatureString {
		v := mc.TimeSignature
		echo.TimeSignature = &v
	}
	if mc.Numerator > 0 {
		v := mc.Numerator
		echo.TimeSignatureNumerator = &v
	}
	if mc.Denominator > 0 {
		v := mc.Denominator
		echo.TimeSignatureDenominator = &v
	}
	if mc.TempoSettingCount > 0 {
		v := mc.TempoSettingCount
		echo.TempoSettingCount = &v
	}
	if mc.TimeSigSettingCount > 0 {
		v := mc.TimeSigSettingCount
		echo.TimeSigSettingCount = &v
	}
	return echo
}

func buildAnalyzeFinding(f Finding, clipOriginKnown bool, clipOriginBeats float64, barsUsable bool, mc musicContextSnapshot) AnalyzeFinding {
	segments := []AnalyzeSegment{
		buildAnalyzeSegment("prototype", f.Prototype, clipOriginKnown, clipOriginBeats, barsUsable, mc),
		buildAnalyzeSegment("match", f.Match, clipOriginKnown, clipOriginBeats, barsUsable, mc),
	}
	basis := []string{"interval_pattern_exact_match", "rhythm_exact_match"}
	if f.Kind == KindSequence && f.SequenceQuality == SeqDiatonic {
		basis = append(basis, "diatonic_scale_degree_match")
	}
	af := AnalyzeFinding{
		Kind:                   f.Kind,
		SequenceQuality:        f.SequenceQuality,
		Label:                  findingLabel(f),
		Segments:               segments,
		IntervalPattern:        f.IntervalPattern,
		ShiftSemitones:         f.ShiftSemitones,
		DiatonicSteps:          f.DiatonicSteps,
		InversionAxisSemitones: f.InversionAxisSemitones,
		Confidence:             "high",
		ConfidenceScore:        0.99,
		Basis:                  basis,
		RequiresConfirmation:   false,
	}
	if af.IntervalPattern == nil {
		af.IntervalPattern = []int{}
	}
	return af
}

func buildAnalyzeSegment(role string, seg Segment, clipOriginKnown bool, clipOriginBeats float64, barsUsable bool, mc musicContextSnapshot) AnalyzeSegment {
	as := AnalyzeSegment{
		Role:        role,
		StartBeats:  seg.StartBeats,
		LengthBeats: seg.EndBeats - seg.StartBeats,
		NoteIDs:     seg.NoteIDs,
	}
	if as.NoteIDs == nil {
		as.NoteIDs = []string{}
	}
	if !barsUsable || !clipOriginKnown {
		return as
	}
	globalStart := clipOriginBeats + seg.StartBeats
	globalEnd := clipOriginBeats + seg.EndBeats
	if seg.EndBeats > seg.StartBeats {
		// 末音恰落在小节线上时归入前一小节（含边界语义）。
		globalEnd -= 1e-9
	}
	barStart := barOf(globalStart, mc.Numerator)
	barEnd := barOf(globalEnd, mc.Numerator)
	if barEnd < barStart {
		barEnd = barStart
	}
	as.BarStart = &barStart
	as.BarEnd = &barEnd
	return as
}

// barOf 把全局 beat 换算为 1-based 小节号。
func barOf(globalBeat float64, numerator int) int {
	return int(math.Floor(globalBeat/float64(numerator))) + 1
}

// findingLabel 生成教学面中文标签（256 草案示例形态）。
func findingLabel(f Finding) string {
	switch f.Kind {
	case KindRepeat:
		return "原样重复"
	case KindSequence:
		dir := "上移"
		if f.ShiftSemitones < 0 {
			dir = "下移"
		}
		abs := f.ShiftSemitones
		if abs < 0 {
			abs = -abs
		}
		if f.SequenceQuality == SeqDiatonic {
			return fmt.Sprintf("模进（调内，%s %d 半音）", dir, abs)
		}
		return fmt.Sprintf("模进（移调，%s %d 半音）", dir, abs)
	case KindInversion:
		return "倒影"
	case KindRetrograde:
		return "逆行"
	default:
		return string(f.Kind)
	}
}

func buildAnalyzeLLMContext(clipID string, req analyzeRequest, findings []AnalyzeFinding, noteCount int, barsUsable bool, limitations []string) AnalyzeLLMContext {
	var sb strings.Builder
	if len(findings) == 0 {
		sb.WriteString(fmt.Sprintf("未检出变换族结构关系（repeat/sequence/inversion/retrograde，分析 %d 音）。", noteCount))
	} else {
		sb.WriteString(fmt.Sprintf("检出 %d 条变换族结构关系（分析 %d 音）。", len(findings), noteCount))
		for _, f := range findings {
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf("%s：原型 %s → 匹配 %s。", f.Label, describeSegmentPos(f.Segments[0]), describeSegmentPos(f.Segments[1])))
		}
	}
	kinds := make([]string, 0, 4)
	for _, k := range req.Transformations {
		kinds = append(kinds, string(k))
	}
	if len(kinds) == 0 {
		kinds = []string{"repeat", "sequence", "inversion", "retrograde"}
	}
	sort.Strings(kinds)
	facts := []map[string]any{
		{"finding_count": len(findings)},
		{"analyzed_note_count": noteCount},
		{"transformations": strings.Join(kinds, ",")},
		{"diatonic_enabled": req.Key != nil},
		{"bar_coordinates": barsUsable},
	}
	evidence := make([]string, 0, len(findings))
	for _, f := range findings {
		proto, match := f.Segments[0], f.Segments[1]
		evidence = append(evidence, fmt.Sprintf("clip:%s/notes:%s-%s+%s-%s",
			clipID,
			proto.NoteIDs[0], proto.NoteIDs[len(proto.NoteIDs)-1],
			match.NoteIDs[0], match.NoteIDs[len(match.NoteIDs)-1]))
	}
	next := "如需聚焦，可用 range_beats 缩小窗口或调整 min_match_notes 后复跑。"
	if len(findings) > 0 {
		next = "可用 marker 标注原型/匹配段并循环播放对比；如需逐音判定学生片段，按 note_ids 与原型段做音程序列比对。"
	}
	return AnalyzeLLMContext{
		SummaryMD:              sb.String(),
		CompactFacts:           facts,
		DoNotIncludeRawPackage: true,
		EvidenceRefs:           evidence,
		LimitationNotes:        append([]string(nil), limitations...),
		SuggestedNextStep:      next,
	}
}

func describeSegmentPos(seg AnalyzeSegment) string {
	if seg.BarStart != nil && seg.BarEnd != nil {
		if *seg.BarStart == *seg.BarEnd {
			return fmt.Sprintf("第 %d 小节", *seg.BarStart)
		}
		return fmt.Sprintf("第 %d-%d 小节", *seg.BarStart, *seg.BarEnd)
	}
	return fmt.Sprintf("beats %.2f-%.2f", seg.StartBeats, seg.StartBeats+seg.LengthBeats)
}

// analyzeID 由规范化输入（参数 + 窗口内音符 + findings 摘要）生成稳定 id，
// 同输入同输出（供 trajectory/课时卡回指）。
func analyzeID(req analyzeRequest, windowed []Note, findings []Finding) string {
	h := sha256.New()
	kinds := make([]string, 0, len(req.Transformations))
	for _, k := range req.Transformations {
		kinds = append(kinds, string(k))
	}
	sort.Strings(kinds)
	fmt.Fprintf(h, "v0|clip=%s|track=%s|range=%t:%v:%v|kinds=%s|min=%d|key=%v",
		req.ClipID, req.TrackID, req.HasRange, req.RangeStart, req.RangeEnd,
		strings.Join(kinds, ","), req.MinMatchNotes, req.Key)
	ns := sortedNotes(windowed)
	for _, n := range ns {
		fmt.Fprintf(h, ";n=%s:%d:%.9f:%.9f", n.ID, n.Pitch, n.Start, n.Length)
	}
	for _, f := range findings {
		fmt.Fprintf(h, ";f=%s/%s/%d/%d/%d/%d", f.Kind, f.SequenceQuality, f.ShiftSemitones, f.DiatonicSteps, f.Prototype.StartIndex, f.Match.StartIndex)
	}
	return "analyze_midi_transformations_" + hex.EncodeToString(h.Sum(nil))[:16]
}

// responseToMap 经 JSON 往返把 typed 返回转成 map（字段名以 json tag 为准）。
func responseToMap(resp AnalyzeResponse) (map[string]any, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("analyze_midi_transformations: encode response: %w", err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("analyze_midi_transformations: decode response: %w", err)
	}
	return out, nil
}

// ---- 小工具（容差数值提取，输入来自 encoding/json 的 map[string]any） ----

func kernelReplyOK(reply map[string]any) bool {
	if reply == nil {
		return false
	}
	status := strings.ToLower(strings.TrimSpace(stringOfAny(reply["status"])))
	return status == "ok" || status == "success"
}

func kernelReplyError(reply map[string]any) string {
	if msg := strings.TrimSpace(stringOfAny(reply["message"])); msg != "" {
		return msg
	}
	if msg := strings.TrimSpace(stringOfAny(reply["error"])); msg != "" {
		return msg
	}
	return "status=" + stringOfAny(reply["status"])
}

func stringOfAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func numberOfAny(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func intOfAny(v any) (int, bool) {
	f, ok := numberOfAny(v)
	if !ok || f != math.Trunc(f) {
		return 0, false
	}
	return int(f), true
}

package legamedesign

// 映射表 T-1..T-21 落点（契约 §6.2 v0-frozen）：设计源结构 → 卡输出结构。
// 输出结构是本包的"卡 YAML 序列化面"：字段名与现行 edu.LevelCard 逐键一致，
// 产物在 Compile() 末尾强制过 edu.ParseLevelCard 终检（§6.1），字段漂移即 E。

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// ---- 卡输出结构（yaml 标签与 agent/internal/edu/lesson_card.go 对齐）----

type cardOut struct {
	SchemaVersion string                     `yaml:"schema_version"`
	ID            string                     `yaml:"id"`
	Title         string                     `yaml:"title"`
	Story         string                     `yaml:"story,omitempty"`
	LeadDefault   string                     `yaml:"lead_default"`
	Players       []string                   `yaml:"players"`
	Timing        cardTiming                 `yaml:"timing"`
	Assets        []cardAsset                `yaml:"assets,omitempty"`
	RuntimeSlots  map[string]cardRuntimeSlot `yaml:"runtime_slots"`
	Notes         map[string][]cardNote      `yaml:"notes,omitempty"`
	Objectives    []cardObjective            `yaml:"objectives"`
	Nodes         []cardNode                 `yaml:"nodes"`
	Guidance      cardGuidance               `yaml:"guidance"`
	WriteBoundary cardWriteBoundary          `yaml:"write_boundary"`
	Finale        cardFinale                 `yaml:"finale"`
	Budget        cardBudget                 `yaml:"budget"`
}

type cardTiming struct {
	BPMDefault    float64                   `yaml:"bpm_default"`
	TimeSignature string                    `yaml:"time_signature"`
	BeatsPerBar   int                       `yaml:"beats_per_bar"`
	PickupBeats   float64                   `yaml:"pickup_beats"`
	ClipOrigins   map[string]cardClipOrigin `yaml:"clip_origins,omitempty"`
	BarSpan       cardBarSpan               `yaml:"bar_span"`
}

type cardClipOrigin struct {
	StartBeatsGlobal float64 `yaml:"start_beats_global"`
	FirstBar         int     `yaml:"first_bar"`
}

type cardBarSpan struct {
	FirstBar int `yaml:"first_bar"`
	LastBar  int `yaml:"last_bar"`
}

type cardAsset struct {
	ID       string   `yaml:"id"`
	Kind     string   `yaml:"kind"`
	File     string   `yaml:"file"`
	SHA256   string   `yaml:"sha256"`
	Key      *cardKey `yaml:"key,omitempty"`
	NotesRef string   `yaml:"notes_ref"`
}

type cardKey struct {
	Tonic int    `yaml:"tonic"`
	Mode  string `yaml:"mode,omitempty"`
}

type cardRuntimeSlot struct {
	TrackIndex   int     `yaml:"track_index"`
	Kind         string  `yaml:"kind"`
	Preload      *string `yaml:"preload"`
	InitialState string  `yaml:"initial_state"`
}

type cardNote struct {
	Pitch  int     `yaml:"pitch"`
	Start  float64 `yaml:"start"`
	Length float64 `yaml:"length"`
}

type cardObjective struct {
	ID    string    `yaml:"id"`
	Text  string    `yaml:"text"`
	Check cardCheck `yaml:"check"`
}

type cardCheck struct {
	Judge  string   `yaml:"judge,omitempty"`
	Event  string   `yaml:"event,omitempty"`
	Key    string   `yaml:"key,omitempty"`
	Node   string   `yaml:"node,omitempty"`
	Kind   string   `yaml:"kind,omitempty"`
	Rubric []string `yaml:"rubric,omitempty"`
}

type cardNode struct {
	ID         string        `yaml:"id"`
	Title      string        `yaml:"title"`
	Lead       string        `yaml:"lead"`
	Goal       string        `yaml:"goal"`
	ModelDoes  []string      `yaml:"model_does,omitempty"`
	HintLadder []string      `yaml:"hint_ladder,omitempty"`
	DoneWhen   cardDoneWhen  `yaml:"done_when"`
	Teaching   *cardTeaching `yaml:"teaching,omitempty"`
}

type cardTeaching struct {
	Note       string   `yaml:"note,omitempty"`
	Objectives []string `yaml:"objectives,omitempty"`
}

// cardDoneWhen 是 done_when 输出形态：叶子（judge/event/structure）或单层
// 组合子 all/any（T-16 只产生 all；卡校验门组合子深度=1）。
type cardDoneWhen struct {
	All       []cardDoneWhen `yaml:"all,omitempty"`
	Any       []cardDoneWhen `yaml:"any,omitempty"`
	Judge     string         `yaml:"judge,omitempty"`
	Target    *cardRangeRef  `yaml:"target,omitempty"`
	Reference *cardRangeRef  `yaml:"reference,omitempty"`
	Event     string         `yaml:"event,omitempty"`
	Key       string         `yaml:"key,omitempty"`
	Structure string         `yaml:"structure,omitempty"`
	Clip      string         `yaml:"clip,omitempty"`
	Track     string         `yaml:"track,omitempty"`
}

type cardRangeRef struct {
	Clip       string    `yaml:"clip,omitempty"`
	Asset      string    `yaml:"asset,omitempty"`
	RangeBeats []float64 `yaml:"range_beats,omitempty"`
	RangeBars  []int     `yaml:"range_bars,omitempty"`
}

type cardGuidance struct {
	PullbackAfterTurns int      `yaml:"pullback_after_turns"`
	Soft               string   `yaml:"soft,omitempty"`
	Avoid              []string `yaml:"avoid"`
}

type cardWriteBoundary struct {
	Allow            []string `yaml:"allow"`
	ConfirmDowngrade string   `yaml:"confirm_downgrade"`
}

type cardFinale struct {
	CompletionBeforeExport bool             `yaml:"completion_before_export"`
	ClosureCheck           cardClosureCheck `yaml:"closure_check"`
	Ritual                 []string         `yaml:"ritual,omitempty"`
	Settle                 cardSettle       `yaml:"settle"`
}

type cardClosureCheck struct {
	StructureComplete cardStructureComplete `yaml:"structure_complete"`
	SoftSignals       cardSoftSignals       `yaml:"soft_signals"`
}

type cardStructureComplete struct {
	AllNodesDone        bool     `yaml:"all_nodes_done"`
	NoPlaceholderBlanks []string `yaml:"no_placeholder_blanks,omitempty"`
}

type cardSoftSignals struct {
	UserClosingLanguage cardUserClosing `yaml:"user_closing_language"`
}

type cardUserClosing struct {
	Role     string `yaml:"role"`
	Required bool   `yaml:"required"`
}

type cardSettle struct {
	MemoryWrites      []string `yaml:"memory_writes,omitempty"`
	ObjectivesCheck   string   `yaml:"objectives_check,omitempty"`
	TrajectoryArchive bool     `yaml:"trajectory_archive"`
}

type cardBudget struct {
	MaxTurns    int    `yaml:"max_turns"`
	MaxLLMCalls int    `yaml:"max_llm_calls"`
	OnStopLoss  string `yaml:"on_stop_loss"`
}

// trackMappingRecord 是 R3 的"设计轨名→声明序轨道坐标"映射记录（进 manifest）。
type trackMappingRecord struct {
	TrackID    string `json:"track_id"`
	TrackIndex int    `json:"track_index"`
	Kind       string `json:"kind"`
}

// 卡 v0 runtime_slots kind 闭集映射（T-12；assist_generated→blank_to_fill 为
// 契约 §7 解释备案②提案值，W01-C 落实语义对账）。
var runtimeSlotKindByRole = map[string]string{
	"reference":        "reference",
	"user_work":        "workspace",
	"assist_generated": "blank_to_fill",
	"generated_result": "workspace",
}

// 30 调 token → 卡资产 key（T-13 的 tonic 音级表；R6a=C）。
// edu 卡 v0 判定 key 仅支持 major（prepareKey 拒绝 minor），minor 见 deriveAssetKey。
type keySignatureInfo struct {
	Tonic int
	Mode  string
}

var keySignatureTable = map[string]keySignatureInfo{
	"C": {0, "major"}, "G": {7, "major"}, "D": {2, "major"}, "A": {9, "major"},
	"E": {4, "major"}, "B": {11, "major"}, "F#": {6, "major"}, "C#": {1, "major"},
	"F": {5, "major"}, "Bb": {10, "major"}, "Eb": {3, "major"}, "Ab": {8, "major"},
	"Db": {1, "major"}, "Gb": {6, "major"}, "Cb": {11, "major"},
	"Am": {9, "minor"}, "Em": {4, "minor"}, "Bm": {11, "minor"}, "F#m": {6, "minor"},
	"C#m": {1, "minor"}, "G#m": {8, "minor"}, "D#m": {3, "minor"}, "A#m": {10, "minor"},
	"Dm": {2, "minor"}, "Gm": {7, "minor"}, "Cm": {0, "minor"}, "Fm": {5, "minor"},
	"Bbm": {10, "minor"}, "Ebm": {3, "minor"}, "Abm": {8, "minor"},
}

// buildCompiledCard 执行 T-1..T-21 映射（诊断聚合进 r；返回 nil error 表示结构可序列化）。
func buildCompiledCard(d *Design, input CompileInput, admitted []admittedMaterial, r *Report) (*cardOut, []trackMappingRecord, error) {
	card := &cardOut{SchemaVersion: CardSchemaVersion} // T-1

	// T-2：id/title 直拷；卡拒绝空值。
	if strings.TrimSpace(d.ID) == "" {
		r.errorf(RuleRequired, "id", "T-2: id 为空（卡加载口要求非空 id）")
	}
	if strings.TrimSpace(d.Title) == "" {
		r.errorf(RuleRequired, "title", "T-2: title 为空（卡拒绝空 title，T-2 E）")
	}
	card.ID = d.ID
	card.Title = d.Title

	// T-3：brief → story 模板句（缺→W + 兜底句）。
	card.Story = buildStory(d.Brief, r)

	// T-4：节点 lead 全一致→lead_default（混写→E-FLOW）。
	leadDefault, leadErr := deriveLeadDefault(d.Flow, r)
	if leadErr != nil {
		return nil, nil, leadErr
	}
	card.LeadDefault = leadDefault

	// T-5：固定双人。
	card.Players = []string{"human", "model"}

	// T-6/T-7/T-8/T-9/T-11：timing。
	_, timingErr := buildTiming(d, input, card, r)
	if timingErr != nil {
		return nil, nil, timingErr
	}

	// T-12：tracks → runtime_slots（R3 声明序坐标；映射记录进 manifest）。
	runtimeSlots, trackMapping, blankSlots := buildRuntimeSlots(d, admitted, r)
	card.RuntimeSlots = runtimeSlots

	// T-13：准入材料 → assets + notes 表（sha256 已由 Compile 物化）。
	assets, notes, assetErr := buildAssets(d, admitted, r)
	if assetErr != nil {
		return nil, nil, assetErr
	}
	card.Assets = assets
	card.Notes = notes

	// 表接口自身合法性（T-14/T-17 登记表；未接线时编译会在使用点/E-CAP 处失败）。
	input.Judgements.validate(d, r)

	// T-14/T-15：checks→卡判定面、objectives→卡 objectives（R4 拆分）。
	objectives, objectiveIDsByCheck := buildObjectives(d, input.Judgements, r)
	card.Objectives = objectives

	// T-16：nodes 映射（done_when 由 complete_when 的可编译项组成；全不可编译→E-FLOW）。
	card.Nodes = buildNodes(d, input.Judgements, objectiveIDsByCheck, runtimeSlots, r)

	// T-18：guidance（四字段 join→soft；avoid=[]；pullback 缺省注入 6 + W）。
	card.Guidance = buildGuidance(d, r)

	// T-17：write_boundary 仅由 CAP-BRIDGE 闭包表产生；confirm_downgrade 固定 none。
	card.WriteBoundary = cardWriteBoundary{
		Allow:            input.Capabilities.writeBoundary(d, r),
		ConfirmDowngrade: "none",
	}

	// T-19：submission/completion/evidence_recording → finale + settle 模板。
	card.Finale = buildFinale(d, blankSlots)

	// T-20：budget（R5 已保证值可解析；覆盖参数优先并出 I）。
	maxTurns, maxCalls := resolveBudget(d, input.Overrides, r)
	onLimit := "preserve_and_pause"
	if d.Budget != nil && d.Budget.OnLimit != nil {
		onLimit = *d.Budget.OnLimit
	}
	card.Budget = cardBudget{MaxTurns: maxTurns, MaxLLMCalls: maxCalls, OnStopLoss: onLimit}

	return card, trackMapping, nil
}

// ---- T-3 ----

func buildStory(b *BriefBlock, r *Report) string {
	const fallback = "（设计源未提供剧情句）"
	if b == nil {
		r.warnf(RuleDefault, "brief", "T-3: brief 缺失 → story 用编译模板兜底句（W）")
		return fallback
	}
	mt := strings.TrimSpace(b.MusicTask)
	fd := strings.TrimSpace(b.FinalDeliverable)
	switch {
	case mt != "" && fd != "":
		return fmt.Sprintf("本局任务：%s。最终交付：%s。", mt, fd)
	case mt != "":
		r.warnf(RuleDefault, "brief.final_deliverable", "T-3: brief.final_deliverable 缺失 → story 仅含任务句（W）")
		return fmt.Sprintf("本局任务：%s。", mt)
	case fd != "":
		r.warnf(RuleDefault, "brief.music_task", "T-3: brief.music_task 缺失 → story 仅含交付句（W）")
		return fmt.Sprintf("最终交付：%s。", fd)
	default:
		r.warnf(RuleDefault, "brief", "T-3: brief.music_task/final_deliverable 均空 → story 兜底句（W）")
		return fallback
	}
}

// ---- T-4 ----

func deriveLeadDefault(f *Flow, r *Report) (string, error) {
	if f == nil || len(f.Nodes) == 0 {
		return "", fmt.Errorf("flow.nodes 为空，无法推导 lead_default（T-4）")
	}
	leads := map[string]bool{}
	for _, n := range f.Nodes {
		leads[n.Lead] = true
	}
	if len(leads) != 1 {
		sorted := make([]string, 0, len(leads))
		for lead := range leads {
			sorted = append(sorted, lead)
		}
		sort.Strings(sorted)
		r.errorf(RuleFlow, "flow.nodes", "T-4: 节点 lead 混写 %v → 卡 lead_default 需设计侧统一或显式覆盖参数（E-FLOW）", sorted)
		return "", fmt.Errorf("lead 混写: %v", sorted)
	}
	for lead := range leads {
		return lead, nil
	}
	return "", fmt.Errorf("lead 缺失")
}

// ---- T-6/T-7/T-8/T-9/T-11 ----

func buildTiming(d *Design, input CompileInput, card *cardOut, r *Report) (int, error) {
	if d.Workspace == nil || d.Workspace.Timing == nil || d.Workspace.Timing.TimeSignature == nil {
		r.errorf(RuleRequired, "workspace.timing.time_signature", "T-6: time_signature 缺失（乐局无计量不可编译）")
		return 0, fmt.Errorf("time_signature 缺失")
	}
	ts := *d.Workspace.Timing.TimeSignature
	beatsPerBar := timeSignatureNumerator(ts) // T-7：分子
	if beatsPerBar < 1 {
		r.errorf(RuleRequired, "workspace.timing.time_signature", "T-7: time_signature %q 无法换算 beats_per_bar", ts)
		return 0, fmt.Errorf("time_signature 非法")
	}
	card.Timing.TimeSignature = ts
	card.Timing.BeatsPerBar = beatsPerBar

	// T-8：tempo（覆盖参数优先；R5 已保证源或覆盖至少一侧有值且 >0）。
	card.Timing.BPMDefault = resolveTempo(d, input.Overrides, r)

	// T-9：pickup_beats（缺省 0 直拷）。
	if d.Workspace.Timing.PickupBeats != nil {
		card.Timing.PickupBeats = *d.Workspace.Timing.PickupBeats
	}

	// T-11：clip 型 region → clip_origins（轨内声明序推导）+ bar_span（最外层小节窗）。
	origins := map[string]cardClipOrigin{}
	perTrackNextBar := map[string]int{}
	perTrackNextBeat := map[string]float64{}
	lastBar := 0
	for i := range d.Workspace.Regions {
		rg := &d.Workspace.Regions[i]
		if rg.Locator == nil || rg.Locator.ClipAlias == nil {
			continue // scope 型 region 不产生 origin（T-11）
		}
		lb := fmt.Sprintf("workspace.regions[%d].locator", i)
		if rg.Locator.Bars == nil {
			r.errorf(RuleMapping, lb+".bars", "T-11: clip 型 region 未声明 bars，无法推导 clip_origins/bar_span（推导歧义→E）")
			continue
		}
		if *rg.Locator.Bars < 1 {
			r.errorf(RuleMapping, lb+".bars", "T-11: clip 型 region bars=%d 非法（须 ≥1）", *rg.Locator.Bars)
			continue
		}
		alias := *rg.Locator.ClipAlias
		if alias == "" {
			continue // 源校验已 E-REQ
		}
		if _, dup := origins[alias]; dup {
			r.errorf(RuleMapping, lb+".clip_alias", "T-11: clip alias %q 重复，clip_origins 推导歧义→E", alias)
			continue
		}
		track := rg.Track
		if perTrackNextBar[track] == 0 {
			perTrackNextBar[track] = 1
		}
		firstBar := perTrackNextBar[track]
		origins[alias] = cardClipOrigin{StartBeatsGlobal: perTrackNextBeat[track], FirstBar: firstBar}
		perTrackNextBar[track] = firstBar + *rg.Locator.Bars
		perTrackNextBeat[track] += float64(*rg.Locator.Bars) * float64(beatsPerBar)
		if end := firstBar + *rg.Locator.Bars - 1; end > lastBar {
			lastBar = end
		}
	}
	if len(origins) == 0 {
		r.errorf(RuleMapping, "workspace.regions", "T-11: 无 clip 型 region（clip_alias），无法推导 bar_span/clip_origins（推导歧义→E）")
		return beatsPerBar, fmt.Errorf("bar_span 推导歧义")
	}
	card.Timing.ClipOrigins = origins
	if lastBar < 1 {
		lastBar = 1
	}
	card.Timing.BarSpan = cardBarSpan{FirstBar: 1, LastBar: lastBar}
	return beatsPerBar, nil
}

// timeSignatureNumerator 返回 N/D 的 N（源校验已保证格式）。
func timeSignatureNumerator(s string) int {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 0
	}
	n := 0
	for _, ch := range parts[0] {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

func resolveTempo(d *Design, ov CompileOverrides, r *Report) float64 {
	var src *float64
	if d.Workspace != nil && d.Workspace.Timing != nil {
		src = d.Workspace.Timing.TempoBPM
	}
	if ov.TempoBPM != nil {
		if src != nil && math.Abs(*src-*ov.TempoBPM) > 1e-9 {
			r.infof(RuleNote, "workspace.timing.tempo_bpm", "R5: 显式覆盖参数 %v 优先于设计源 %v（用户问清后补齐）", *ov.TempoBPM, *src)
		}
		return *ov.TempoBPM
	}
	if src == nil {
		return 0 // R5 已在 ValidateCompileReadiness 出 E，不会到此
	}
	return *src
}

// ---- T-12 ----

func buildRuntimeSlots(d *Design, admitted []admittedMaterial, r *Report) (map[string]cardRuntimeSlot, []trackMappingRecord, []string) {
	slots := map[string]cardRuntimeSlot{}
	records := make([]trackMappingRecord, 0)
	blankSlots := make([]string, 0)
	if d.Workspace == nil {
		return slots, records, blankSlots
	}
	admittedByMaterial := map[string]admittedMaterial{}
	for _, a := range admitted {
		admittedByMaterial[a.material.ID] = a
	}
	for i := range d.Workspace.Tracks {
		t := &d.Workspace.Tracks[i]
		kind, ok := runtimeSlotKindByRole[t.Role]
		if !ok {
			r.errorf(RuleMapping, fmt.Sprintf("workspace.tracks[%d].role", i), "T-12: track.role %q 无 kind 映射", t.Role)
			continue
		}
		var preload *string
		initialState := "empty"
		if t.InitialMaterial != nil {
			if _, admittedOK := admittedByMaterial[*t.InitialMaterial]; !admittedOK {
				r.errorf(RuleRef, fmt.Sprintf("workspace.tracks[%d].initial_material", i), "T-12: initial_material %q 未准入资产（材料须已准入，否则 E-REF）", *t.InitialMaterial)
			} else {
				id := *t.InitialMaterial
				preload = &id
				initialState = "loaded"
			}
		}
		slots[t.ID] = cardRuntimeSlot{TrackIndex: i + 1, Kind: kind, Preload: preload, InitialState: initialState}
		records = append(records, trackMappingRecord{TrackID: t.ID, TrackIndex: i + 1, Kind: kind})
		if kind == "blank_to_fill" {
			blankSlots = append(blankSlots, t.ID)
		}
	}
	return slots, records, blankSlots
}

// ---- T-13 ----

func buildAssets(d *Design, admitted []admittedMaterial, r *Report) ([]cardAsset, map[string][]cardNote, error) {
	assets := make([]cardAsset, 0, len(admitted))
	notes := map[string][]cardNote{}
	key := deriveAssetKey(d, r)
	for _, a := range admitted {
		assets = append(assets, cardAsset{
			ID:       a.material.ID,
			Kind:     "midi_reference", // T-13：midi/midi_or_audio → midi_reference
			File:     a.fileRel,
			SHA256:   a.sha256,
			Key:      key,
			NotesRef: "notes." + a.material.ID,
		})
		notes[a.material.ID] = a.notes
	}
	return assets, notes, nil
}

// deriveAssetKey 按 R6a/T-13 从 key_signature 推导资产判定 key（tonic 音级 + mode）。
// 卡 v0 判定 key 仅大调（edu.prepareKey 限 major）：minor 不写 key 并出 W-MAP，
// key_signature 的消费方=记谱拼写/YuE2 ABC K:（T-10 已声明不进卡）。
func deriveAssetKey(d *Design, r *Report) *cardKey {
	token := "C"
	if d.Workspace != nil && d.Workspace.Timing != nil && d.Workspace.Timing.KeySignature != nil {
		token = *d.Workspace.Timing.KeySignature
	} else {
		r.infof(RuleNote, "workspace.timing.key_signature", "§6.4: key_signature 缺 → 编译缺省 C（I）")
	}
	info, ok := keySignatureTable[token]
	if !ok {
		return nil // 源校验 E-SCHEMA 已拦（R6a 闭集），此处防御
	}
	if info.Mode != "major" {
		r.warnf(RuleDefault, "workspace.timing.key_signature", "R6a/T-13: key_signature %q 为小调；卡 v0 资产判定 key 仅 support major（edu.prepareKey），不写 key（消费方=记谱/ABC K:，T-10）", token)
		return nil
	}
	return &cardKey{Tonic: info.Tonic, Mode: "major"}
}

// ---- T-14 / T-15 ----

// checkByIDIndex 建 check id → *Check 索引。
func checkByIDIndex(d *Design) map[string]*Check {
	idx := make(map[string]*Check, len(d.Checks))
	for i := range d.Checks {
		idx[d.Checks[i].ID] = &d.Checks[i]
	}
	return idx
}

// verbalRubricCheck 把 check 的 criterion+evidence 确定性拆句为 rubric 行
// （T-14 rubric 分支；空句丢弃，至少 1 行——源校验已保证 criterion/evidence 非空）。
func verbalRubricCheck(c *Check) cardCheck {
	lines := splitRubricLines(c.Criterion)
	lines = append(lines, splitRubricLines(c.Evidence)...)
	if len(lines) == 0 {
		lines = []string{c.Criterion}
	}
	return cardCheck{Kind: "verbal_rubric", Rubric: lines}
}

// splitRubricLines 按中文/英文句末标点确定性拆句（不做"理解"，仅分句）。
func splitRubricLines(text string) []string {
	var lines []string
	var b strings.Builder
	flush := func() {
		if s := strings.TrimSpace(b.String()); s != "" {
			lines = append(lines, s)
		}
		b.Reset()
	}
	for _, ch := range text {
		switch ch {
		case '。', '；', ';', '\n':
			flush()
		default:
			b.WriteRune(ch)
		}
	}
	flush()
	return lines
}

// objectiveCheckFace 决定 check 在卡 objective.check 上的形态：
//   - rubric → verbal_rubric（T-14 原生分支）
//   - user_judgment 已登记事件 → event 叶
//   - user_judgment 未登记 → verbal_rubric + W-CHK
//   - deterministic → verbal_rubric + W-CHK（白名单结构叶只用于 done_when）
func objectiveCheckFace(c *Check, reg JudgementRegistry, r *Report) cardCheck {
	switch c.Method {
	case "rubric":
		return verbalRubricCheck(c)
	case "user_judgment":
		if event, ok := reg.UserJudgment[c.ID]; ok && containsString(JudgementEvents, event) {
			return cardCheck{Event: event}
		}
		r.warnf(RuleCheckWarn, "checks", "T-14: user_judgment check %q 无事件映射登记 → 卡 objective 降 verbal_rubric（advisory，W-CHK）", c.ID)
		return verbalRubricCheck(c)
	case "deterministic":
		r.warnf(RuleCheckWarn, "checks", "T-14: deterministic check %q 在卡 objective 判定面无结构叶（白名单只用于 done_when）→ 证据降 verbal_rubric（advisory，W-CHK）", c.ID)
		return verbalRubricCheck(c)
	default:
		return cardCheck{}
	}
}

func buildObjectives(d *Design, reg JudgementRegistry, r *Report) ([]cardObjective, map[string][]string) {
	checks := checkByIDIndex(d)
	objectives := make([]cardObjective, 0, len(d.Objectives))
	objectiveIDsByCheck := map[string][]string{}
	for oi := range d.Objectives {
		o := &d.Objectives[oi]
		if len(o.EvidenceChecks) == 0 {
			r.warnf(RuleDefault, fmt.Sprintf("objectives[%d].evidence_checks", oi), "T-15: 目标 %q 无 evidence_checks → 不进卡（W）", o.ID)
			continue
		}
		if len(o.EvidenceChecks) == 1 {
			cid := o.EvidenceChecks[0]
			c, ok := checks[cid]
			if !ok {
				continue // 源校验 E-REF 已拦
			}
			objectives = append(objectives, cardObjective{ID: o.ID, Text: o.Description, Check: objectiveCheckFace(c, reg, r)})
			objectiveIDsByCheck[cid] = append(objectiveIDsByCheck[cid], o.ID)
			continue
		}
		// T-15/R4：多证据拆分为 id#k（k 从 1 起），W 诊断列明拆分。
		splitIDs := make([]string, 0, len(o.EvidenceChecks))
		for k, cid := range o.EvidenceChecks {
			c, ok := checks[cid]
			if !ok {
				continue
			}
			cardID := fmt.Sprintf("%s#%d", o.ID, k+1)
			splitIDs = append(splitIDs, cardID)
			objectives = append(objectives, cardObjective{ID: cardID, Text: o.Description, Check: objectiveCheckFace(c, reg, r)})
			objectiveIDsByCheck[cid] = append(objectiveIDsByCheck[cid], cardID)
		}
		r.warnf(RuleDefault, fmt.Sprintf("objectives[%d].evidence_checks", oi), "T-15/R4: 目标 %q 多证据（%d 条）拆分进卡：%v（W）", o.ID, len(o.EvidenceChecks), splitIDs)
	}
	return objectives, objectiveIDsByCheck
}

// ---- T-16 ----

// judgementLeaf 把 complete_when 的 check 映射为 done_when 叶（T-14 三分支）：
// deterministic→白名单结构叶；user_judgment→登记事件叶；rubric/未登记→无叶+W-CHK。
func judgementLeaf(checkID string, checks map[string]*Check, reg JudgementRegistry, trackIDs map[string]bool, r *Report) (cardDoneWhen, bool) {
	c, ok := checks[checkID]
	if !ok {
		return cardDoneWhen{}, false // 源校验 E-REF 已拦
	}
	switch c.Method {
	case "deterministic":
		sp, registered := reg.Deterministic[checkID]
		if !registered {
			r.warnf(RuleCheckWarn, "checks", "T-14: deterministic check %q 无结构叶登记 → 降 advisory_checks（W-CHK）", checkID)
			return cardDoneWhen{}, false
		}
		switch sp.Structure {
		case StructureClipExists:
			if !trackIDs[sp.Clip] {
				r.errorf(RuleMapping, "checks", "T-14: 登记表 %q 结构叶 clip=%q 不是已知轨槽", checkID, sp.Clip)
				return cardDoneWhen{}, false
			}
			return cardDoneWhen{Structure: sp.Structure, Clip: sp.Clip}, true
		case StructureTrackNonEmpty:
			if !trackIDs[sp.Track] {
				r.errorf(RuleMapping, "checks", "T-14: 登记表 %q 结构叶 track=%q 不是已知轨槽", checkID, sp.Track)
				return cardDoneWhen{}, false
			}
			return cardDoneWhen{Structure: sp.Structure, Track: sp.Track}, true
		case StructureTempoSet:
			return cardDoneWhen{Structure: sp.Structure}, true
		default:
			r.errorf(RuleMapping, "checks", "T-14: 登记表 %q 结构谓词 %q 不在白名单 %v", checkID, sp.Structure, StructurePredicates)
			return cardDoneWhen{}, false
		}
	case "user_judgment":
		event, registered := reg.UserJudgment[checkID]
		if !registered || !containsString(JudgementEvents, event) {
			r.warnf(RuleCheckWarn, "checks", "T-14: user_judgment check %q 无事件映射登记 → 降 advisory_checks（W-CHK）", checkID)
			return cardDoneWhen{}, false
		}
		return cardDoneWhen{Event: event}, true
	default: // rubric：教学侧非判定器（契约 §2 纪律 2）
		r.warnf(RuleCheckWarn, "checks", "T-14: rubric check %q 非判定器（无 done_when 叶）→ 降 advisory（W-CHK）", checkID)
		return cardDoneWhen{}, false
	}
}

func buildNodes(d *Design, reg JudgementRegistry, objectiveIDsByCheck map[string][]string, slots map[string]cardRuntimeSlot, r *Report) []cardNode {
	trackIDs := make(map[string]bool, len(slots))
	for id := range slots {
		trackIDs[id] = true
	}
	checks := checkByIDIndex(d)
	nodes := make([]cardNode, 0)
	if d.Flow == nil {
		return nodes
	}
	for ni := range d.Flow.Nodes {
		n := &d.Flow.Nodes[ni]
		leaves := make([]cardDoneWhen, 0, len(n.CompleteWhen))
		for _, cid := range n.CompleteWhen {
			if leaf, ok := judgementLeaf(cid, checks, reg, trackIDs, r); ok {
				leaves = append(leaves, leaf)
			}
		}
		var doneWhen cardDoneWhen
		switch len(leaves) {
		case 0:
			r.errorf(RuleFlow, fmt.Sprintf("flow.nodes[%d].complete_when", ni), "T-16: 节点 %q 无可编译完成条件（全不可编译→E-FLOW，节点不可运行）", n.ID)
		case 1:
			doneWhen = leaves[0]
		default:
			doneWhen = cardDoneWhen{All: leaves}
		}
		var guidance *NodeGuidance = n.Guidance
		var teaching *cardTeaching
		note := ""
		hintLadder := []string(nil)
		if guidance != nil {
			note = strings.Join(trimNonEmpty(guidance.TeachingPoints), "；")
			hintLadder = guidance.HintLadder
		}
		objectiveRefs := make([]string, 0)
		seenObj := map[string]bool{}
		for _, cid := range n.CollectEvidence {
			for _, oid := range objectiveIDsByCheck[cid] {
				if !seenObj[oid] {
					seenObj[oid] = true
					objectiveRefs = append(objectiveRefs, oid)
				}
			}
		}
		if note != "" || len(objectiveRefs) > 0 {
			teaching = &cardTeaching{Note: note, Objectives: objectiveRefs}
		}
		nodes = append(nodes, cardNode{
			ID:         n.ID,
			Title:      n.Title,
			Lead:       n.Lead,
			Goal:       n.Task,
			ModelDoes:  n.ModelMayDo,
			HintLadder: hintLadder,
			DoneWhen:   doneWhen,
			Teaching:   teaching,
		})
	}
	return nodes
}

func trimNonEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s := strings.TrimSpace(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ---- T-18 ----

func buildGuidance(d *Design, r *Report) cardGuidance {
	softParts := []string{}
	if d.Guidance != nil {
		for _, field := range []string{d.Guidance.Approach, d.Guidance.HelpPolicy, d.Guidance.ExplorationPolicy, d.Guidance.LearnerDecisionPolicy} {
			if s := strings.TrimSpace(field); s != "" {
				softParts = append(softParts, s)
			}
		}
	}
	// T-18/§6.4：设计源 schema 无 pullback_after_turns 字段 → 恒定注入保守值 6 并 W。
	r.warnf(RuleDefault, "guidance.pullback_after_turns", "T-18/§6.4: 设计源未声明 pullback_after_turns → 编译注入保守值 6（W，模板值须评审可见）")
	return cardGuidance{PullbackAfterTurns: 6, Soft: strings.Join(softParts, "；"), Avoid: []string{}}
}

// ---- T-19 ----

func buildFinale(d *Design, blankSlots []string) cardFinale {
	// 缺省语义（§4.12/§4.13）：submission/completion 段缺 → 保守缺省（true / 设计源实例值）。
	userConfirmClosure := true
	requiredChecks := []string{}
	if d.Completion != nil {
		if d.Completion.UserConfirmClosure != nil {
			userConfirmClosure = *d.Completion.UserConfirmClosure
		}
		requiredChecks = d.Completion.RequiredChecks
	}
	freezeSnapshot := true
	reports := []string{"assignment_report", "key_context_summary"}
	if d.Submission != nil {
		if d.Submission.FreezeProjectAndEvidenceSnapshot != nil {
			freezeSnapshot = *d.Submission.FreezeProjectAndEvidenceSnapshot
		}
		if d.Submission.Reports != nil {
			reports = d.Submission.Reports
		}
	}
	memoryWrites := make([]string, 0, len(reports))
	for _, rep := range reports {
		memoryWrites = append(memoryWrites, "report."+rep)
	}
	return cardFinale{
		CompletionBeforeExport: true,
		ClosureCheck: cardClosureCheck{
			StructureComplete: cardStructureComplete{
				AllNodesDone:        true,
				NoPlaceholderBlanks: blankSlots,
			},
			SoftSignals: cardSoftSignals{
				UserClosingLanguage: cardUserClosing{Role: "weighted", Required: userConfirmClosure},
			},
		},
		Ritual: []string{"name_together", "export", "settle"},
		Settle: cardSettle{
			MemoryWrites:      memoryWrites,
			ObjectivesCheck:   strings.Join(requiredChecks, ","),
			TrajectoryArchive: freezeSnapshot,
		},
	}
}

// ---- T-20 ----

func resolveBudget(d *Design, ov CompileOverrides, r *Report) (int, int) {
	var srcTurns, srcCalls *int
	if d.Budget != nil {
		srcTurns = d.Budget.MaxTurns
		srcCalls = d.Budget.MaxLLMCalls
	}
	return resolveBudgetValue("budget.max_turns", "轮次预算", srcTurns, ov.MaxTurns, r),
		resolveBudgetValue("budget.max_llm_calls", "LLM 调用预算", srcCalls, ov.MaxLLMCalls, r)
}

func resolveBudgetValue(path, label string, src, ov *int, r *Report) int {
	if ov != nil {
		if src != nil && *src != *ov {
			r.infof(RuleNote, path, "R5: 显式覆盖参数 %d 优先于设计源 %d（%s）", *ov, *src, label)
		}
		return *ov
	}
	if src != nil {
		return *src
	}
	return 0 // R5 已在 ValidateCompileReadiness 出 E，不会到此
}

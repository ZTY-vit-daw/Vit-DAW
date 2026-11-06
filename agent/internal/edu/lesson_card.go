package edu

// 乐局卡（level card）加载与校验门（乐局 3a，EDU-LEVEL-RUNTIME-A 目标 3）。
//
// 校验门覆盖设计稿 §1.9 的 12 条拒绝路径 + §5 的 6 条新增对齐项（279 定稿
// 轮卡面「280 校验门对齐面终态」逐项落位）：
//
//	拒绝路径：
//	 1  未知 judge 名（done_when 与 objectives.check）——专家写不出不存在的检查方式
//	 2  未知 event 名
//	 3  未知 structure 谓词
//	 4  bar 引用越界（超出 bar_span 或不被资产 notes 表覆盖）
//	 5  range_beats 逆序/零宽；range_bars 与 range_beats 同给但不一致
//	 6  min_match_notes < 2（evidence_hooks 参数，复用 parseAnalyzeArgs）
//	 7  key.tonic 越界 0-11 / mode 非 major（复用 parseAnalyzeArgs）
//	 8  transformations 白名单外（复用 parseAnalyzeArgs）
//	 9  node.id 重复
//	10  assets 引用不存在的 id / 缺 sha256（含 derived_from/notes_ref/preload）
//	11  schema_version 未知（restore fail-closed 同型）
//	12  objectives[].check 引用未知 judge/event（verbal_rubric 显式豁免）
//	新增对齐项：
//	①事件/结构谓词枚举进拒绝路径（=路径 2/3）
//	②pitch_set_match 语义定案（set_mode 二值闭集，仅 pitch_set_match 可带）
//	③accept_transposed 卡面裁决字段（仅 note_sequence_match 可带）
//	④组合子（all/any）嵌套深度限一层
//	⑤内嵌 notes 表=判定输入源（AssetNotes 合成 id asset:<id>#<k>，零栈可判）
//	⑥runtime_slots 升格为必检卡字段（280 建议裁定，见卡面回执：块必在+
//	  内部一致，不硬编码轨数——轨数=3 是 first-run-twinkle 实例约束）
//
// 解析 fail-closed：顶层未知字段拒绝（yaml KnownFields）；叶子未知参数字段
// 拒绝（Keys 捕获后校验）；与 276 parseAnalyzeArgs 同口径不静默纠正。

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LevelCardSchemaVersion 是本校验门认知的乐局卡 schema 版本（拒绝路径 11）。
const LevelCardSchemaVersion = "vit.level_card.v0"

// 交互事件枚举（闭集；运行时提供）。
var LevelEventNames = []string{
	"human_replied", "user_confirmed", "user_chose",
	"user_feedback_given", "user_identified", "export_completed",
}

// 结构谓词枚举（闭集）。
var LevelStructureNames = []string{"clip_exists", "track_non_empty", "tempo_set"}

// evidence_hooks 可挂的分析面（闭集；276 段 2 命令）。
const levelHookAnalyzeName = "midi.analyze_transformations"

var levelEventSet = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range LevelEventNames {
		m[n] = true
	}
	return m
}()

var levelStructureSet = func() map[string]bool {
	m := map[string]bool{}
	for _, n := range LevelStructureNames {
		m[n] = true
	}
	return m
}()

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ---- 卡结构（yaml 标签与 coord/levels/first-run-twinkle/level.yaml 逐键对齐）----

// LevelCard 是 vit.level_card.v0 乐局卡的结构化形态（校验后即运行时输入）。
type LevelCard struct {
	SchemaVersion string                     `yaml:"schema_version" json:"schema_version"`
	ID            string                     `yaml:"id" json:"id"`
	Title         string                     `yaml:"title" json:"title"`
	Story         string                     `yaml:"story" json:"story"`
	LeadDefault   string                     `yaml:"lead_default" json:"lead_default"`
	Players       []string                   `yaml:"players" json:"players"`
	Timing        TimingBlock                `yaml:"timing" json:"timing"`
	PersonaSlot   PersonaSlotBlock           `yaml:"persona_slot" json:"persona_slot"`
	Assets        []AssetSpec                `yaml:"assets" json:"assets"`
	RuntimeSlots  map[string]RuntimeSlotSpec `yaml:"runtime_slots" json:"runtime_slots"`
	Notes         map[string][]CardNote      `yaml:"notes" json:"notes"`
	Objectives    []ObjectiveSpec            `yaml:"objectives" json:"objectives"`
	Nodes         []LevelNode                `yaml:"nodes" json:"nodes"`
	Guidance      GuidanceBlock              `yaml:"guidance" json:"guidance"`
	WriteBoundary WriteBoundaryBlock         `yaml:"write_boundary" json:"write_boundary"`
	Finale        FinaleBlock                `yaml:"finale" json:"finale"`
	Budget        BudgetBlock                `yaml:"budget" json:"budget"`
}

// TimingBlock 携带小节换算显式依据（257 修订②：判定器不做小节推断）。
type TimingBlock struct {
	BPMDefault    float64                   `yaml:"bpm_default" json:"bpm_default"`
	TimeSignature string                    `yaml:"time_signature" json:"time_signature"`
	BeatsPerBar   int                       `yaml:"beats_per_bar" json:"beats_per_bar"`
	PickupBeats   float64                   `yaml:"pickup_beats" json:"pickup_beats"`
	ClipOrigins   map[string]ClipOriginSpec `yaml:"clip_origins" json:"clip_origins"`
	BarSpan       BarSpanSpec               `yaml:"bar_span" json:"bar_span"`
}

// ClipOriginSpec 是一个 clip 资产的全局起点（bar↔beat 换算依据）。
type ClipOriginSpec struct {
	StartBeatsGlobal float64 `yaml:"start_beats_global" json:"start_beats_global"`
	FirstBar         int     `yaml:"first_bar" json:"first_bar"`
}

// BarSpanSpec 是卡内 bar 引用域（1-based 闭区间）。
type BarSpanSpec struct {
	FirstBar int `yaml:"first_bar" json:"first_bar"`
	LastBar  int `yaml:"last_bar" json:"last_bar"`
}

// BarSpanBeats 返回 bar_span 的 beats 覆盖长度（(last-first+1)*bpb）。
func (t TimingBlock) BarSpanBeats() float64 {
	return float64((t.BarSpan.LastBar - t.BarSpan.FirstBar + 1) * t.BeatsPerBar)
}

type PersonaSlotBlock struct {
	ID     string `yaml:"id" json:"id"`
	Form   string `yaml:"form" json:"form"`
	Source string `yaml:"source" json:"source"`
}

// AssetSpec 是一个卡内资产（判定参照/预置素材）。
type AssetSpec struct {
	ID          string          `yaml:"id" json:"id"`
	Kind        string          `yaml:"kind" json:"kind"`
	File        string          `yaml:"file,omitempty" json:"file,omitempty"`
	SHA256      string          `yaml:"sha256,omitempty" json:"sha256,omitempty"`
	Key         *Key            `yaml:"key,omitempty" json:"key,omitempty"`
	NotesRef    string          `yaml:"notes_ref,omitempty" json:"notes_ref,omitempty"`
	DerivedFrom string          `yaml:"derived_from,omitempty" json:"derived_from,omitempty"`
	Deviations  []DeviationSpec `yaml:"deviations,omitempty" json:"deviations,omitempty"`
}

// DeviationSpec 描述派生资产相对基础表的一个损坏点。
type DeviationSpec struct {
	AtBeat   float64 `yaml:"at_beat" json:"at_beat"`
	Pitch    int     `yaml:"pitch" json:"pitch"`
	Replaces int     `yaml:"replaces" json:"replaces"`
}

// RuntimeSlotSpec 是一个运行时轨槽（开放问题⑤-4/⑥：升格必检卡字段）。
type RuntimeSlotSpec struct {
	TrackIndex   int     `yaml:"track_index" json:"track_index"`
	Kind         string  `yaml:"kind" json:"kind"`
	Preload      *string `yaml:"preload" json:"preload"` // null 合法（无预置）
	InitialState string  `yaml:"initial_state" json:"initial_state"`
}

// CardNote 是内嵌 notes 表元素（beats 域，判定参照权威）。
type CardNote struct {
	Pitch  int     `yaml:"pitch" json:"pitch"`
	Start  float64 `yaml:"start" json:"start"`
	Length float64 `yaml:"length" json:"length"`
}

type ObjectiveSpec struct {
	ID    string    `yaml:"id" json:"id"`
	Text  string    `yaml:"text" json:"text"`
	Check CheckSpec `yaml:"check" json:"check"`
}

// CheckSpec 是 objectives[].check 的三形态联合（judge/event 叶 + verbal_rubric
// 显式豁免标记——教学侧非判定器，路径 12 的豁免落点）。
type CheckSpec struct {
	Judge  string   `json:"judge,omitempty"`
	Event  string   `json:"event,omitempty"`
	Key    string   `json:"key,omitempty"`
	Node   string   `json:"node,omitempty"`
	Kind   string   `json:"kind,omitempty"` // verbal_rubric
	Rubric []string `json:"rubric,omitempty"`
	Keys   []string `json:"-"` // 原始键（fail-closed 超集检查）
}

// UnmarshalYAML 捕获原始键后按已知字段解码（未知键留给校验门拒绝）。
func (c *CheckSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("check must be a mapping")
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		k, v := value.Content[i], value.Content[i+1]
		c.Keys = append(c.Keys, k.Value)
		var err error
		switch k.Value {
		case "judge":
			err = v.Decode(&c.Judge)
		case "event":
			err = v.Decode(&c.Event)
		case "key":
			err = v.Decode(&c.Key)
		case "node":
			err = v.Decode(&c.Node)
		case "kind":
			err = v.Decode(&c.Kind)
		case "rubric":
			err = v.Decode(&c.Rubric)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// LevelNode 是一个关卡节点。
type LevelNode struct {
	ID             string        `yaml:"id" json:"id"`
	Title          string        `yaml:"title" json:"title"`
	Lead           string        `yaml:"lead" json:"lead"`
	Goal           string        `yaml:"goal" json:"goal"`
	ModelDoes      []string      `yaml:"model_does,omitempty" json:"model_does,omitempty"`
	NeedsHuman     bool          `yaml:"needs_human,omitempty" json:"needs_human,omitempty"`
	Fallback       string        `yaml:"fallback,omitempty" json:"fallback,omitempty"`
	DoneWhen       DoneWhen      `yaml:"done_when" json:"done_when"`
	Then           []string      `yaml:"then,omitempty" json:"then,omitempty"`
	HintLadder     []string      `yaml:"hint_ladder,omitempty" json:"hint_ladder,omitempty"`
	Teaching       TeachingBlock `yaml:"teaching,omitempty" json:"teaching,omitempty"`
	AdvisoryChecks []string      `yaml:"advisory_checks,omitempty" json:"advisory_checks,omitempty"`
	Note           string        `yaml:"note,omitempty" json:"note,omitempty"`
	FinaleRef      string        `yaml:"finale_ref,omitempty" json:"finale_ref,omitempty"`
}

type TeachingBlock struct {
	Note          string         `yaml:"note,omitempty" json:"note,omitempty"`
	Objectives    []string       `yaml:"objectives,omitempty" json:"objectives,omitempty"`
	EvidenceHooks []EvidenceHook `yaml:"evidence_hooks,omitempty" json:"evidence_hooks,omitempty"`
}

// EvidenceHook 是教学锚点（analyze 参数族=276 parseAnalyzeArgs 字段族）。
type EvidenceHook struct {
	ID               string          `yaml:"id" json:"id"`
	Analyze          string          `yaml:"analyze" json:"analyze"`
	Params           map[string]any  `yaml:"params" json:"params"`
	ExpectedFindings []FindingAssert `yaml:"expected_findings,omitempty" json:"expected_findings,omitempty"`
	ExpectAbsent     []FindingAssert `yaml:"expect_absent,omitempty" json:"expect_absent,omitempty"`
	ExpectPresent    []FindingAssert `yaml:"expect_present,omitempty" json:"expect_present,omitempty"`
}

// FindingAssert 是 expected_findings/expect_absent/expect_present 断言元素
// （字段族=设计稿 §1.3；断言语义校验属教学面，不进结构校验门——仅闭集+域检查）。
type FindingAssert struct {
	Kind            string `yaml:"kind" json:"kind"`
	SequenceQuality string `yaml:"sequence_quality,omitempty" json:"sequence_quality,omitempty"`
	ProtoBars       []int  `yaml:"proto_bars,omitempty" json:"proto_bars,omitempty"`
	MatchBars       []int  `yaml:"match_bars,omitempty" json:"match_bars,omitempty"`
	ShiftSemitones  int    `yaml:"shift_semitones,omitempty" json:"shift_semitones,omitempty"`
	DiatonicSteps   int    `yaml:"diatonic_steps,omitempty" json:"diatonic_steps,omitempty"`
	NoteCount       int    `yaml:"note_count,omitempty" json:"note_count,omitempty"`
}

type GuidanceBlock struct {
	PullbackAfterTurns int      `yaml:"pullback_after_turns" json:"pullback_after_turns"`
	Soft               string   `yaml:"soft,omitempty" json:"soft,omitempty"`
	Avoid              []string `yaml:"avoid,omitempty" json:"avoid,omitempty"`
}

type WriteBoundaryBlock struct {
	Allow            []string `yaml:"allow" json:"allow"`
	ConfirmDowngrade string   `yaml:"confirm_downgrade" json:"confirm_downgrade"`
}

type FinaleBlock struct {
	CompletionBeforeExport bool `yaml:"completion_before_export" json:"completion_before_export"`
	ClosureCheck           struct {
		StructureComplete struct {
			AllNodesDone        bool               `yaml:"all_nodes_done" json:"all_nodes_done"`
			NoPlaceholderBlanks []string           `yaml:"no_placeholder_blanks" json:"no_placeholder_blanks"`
			WholeSongPlayable   map[string]float64 `yaml:"whole_song_playable" json:"whole_song_playable"`
		} `yaml:"structure_complete" json:"structure_complete"`
		WorkSurfaceQuiet struct {
			NoPendingProposals  bool `yaml:"no_pending_proposals" json:"no_pending_proposals"`
			NoUnansweredQueries bool `yaml:"no_unanswered_queries" json:"no_unanswered_queries"`
			QuietTurnsMin       int  `yaml:"quiet_turns_min" json:"quiet_turns_min"`
		} `yaml:"work_surface_quiet" json:"work_surface_quiet"`
		SoftSignals struct {
			UserClosingLanguage struct {
				Role     string `yaml:"role" json:"role"`
				Required bool   `yaml:"required" json:"required"`
			} `yaml:"user_closing_language" json:"user_closing_language"`
		} `yaml:"soft_signals" json:"soft_signals"`
	} `yaml:"closure_check" json:"closure_check"`
	Ask struct {
		SemanticTemplate string `yaml:"semantic_template" json:"semantic_template"`
	} `yaml:"ask" json:"ask"`
	UserDirectExport string   `yaml:"user_direct_export" json:"user_direct_export"`
	Ritual           []string `yaml:"ritual" json:"ritual"`
	Settle           struct {
		MemoryWrites      []string `yaml:"memory_writes" json:"memory_writes"`
		ObjectivesCheck   string   `yaml:"objectives_check" json:"objectives_check"`
		TrajectoryArchive bool     `yaml:"trajectory_archive" json:"trajectory_archive"`
	} `yaml:"settle" json:"settle"`
	Edges struct {
		CompletionNeqStop   bool `yaml:"completion_neq_stop" json:"completion_neq_stop"`
		CompletionNeqFreeze bool `yaml:"completion_neq_freeze" json:"completion_neq_freeze"`
	} `yaml:"edges" json:"edges"`
}

type BudgetBlock struct {
	MaxTurns    int    `yaml:"max_turns" json:"max_turns"`
	MaxLLMCalls int    `yaml:"max_llm_calls" json:"max_llm_calls"`
	OnStopLoss  string `yaml:"on_stop_loss" json:"on_stop_loss"`
}

// ---- done_when 条件文法（组合子 depth ≤1 + 三族叶子）----

// DoneWhen 是 done_when 条件树：叶子（Leaf）或单层组合子（All/Any）。
type DoneWhen struct {
	All  []DoneWhen     `yaml:"all,omitempty" json:"all,omitempty"`
	Any  []DoneWhen     `yaml:"any,omitempty" json:"any,omitempty"`
	Leaf *ConditionLeaf `yaml:"-" json:"leaf,omitempty"`
}

// UnmarshalYAML 区分组合子与叶子；组合子递归解码（深度检查在校验门）。
func (d *DoneWhen) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("done_when must be a mapping (leaf or {all|any: [...]})")
	}
	hasAll, hasAny := false, false
	for i := 0; i+1 < len(value.Content); i += 2 {
		switch value.Content[i].Value {
		case "all":
			hasAll = true
		case "any":
			hasAny = true
		}
	}
	if hasAll && hasAny {
		return fmt.Errorf("done_when cannot mix all and any (combinator depth discipline)")
	}
	if hasAll || hasAny {
		key := "all"
		if hasAny {
			key = "any"
		}
		for i := 0; i+1 < len(value.Content); i += 2 {
			if value.Content[i].Value != key {
				continue
			}
			var children []DoneWhen
			if err := value.Content[i+1].Decode(&children); err != nil {
				return fmt.Errorf("done_when.%s: %w", key, err)
			}
			if key == "all" {
				d.All = children
			} else {
				d.Any = children
			}
		}
		return nil
	}
	leaf := &ConditionLeaf{}
	if err := leaf.unmarshalFrom(value); err != nil {
		return err
	}
	d.Leaf = leaf
	return nil
}

// RangeRef 是 judge 叶的区间寻址（clip=运行时轨槽名 / asset=卡内资产）。
type RangeRef struct {
	Clip       string    `yaml:"clip,omitempty" json:"clip,omitempty"`
	Asset      string    `yaml:"asset,omitempty" json:"asset,omitempty"`
	RangeBeats []float64 `yaml:"range_beats,omitempty" json:"range_beats,omitempty"`
	RangeBars  []int     `yaml:"range_bars,omitempty" json:"range_bars,omitempty"`
}

// ConditionLeaf 是 done_when 三族叶子的联合；Keys 捕获原始键供 fail-closed
// 超集检查（未知参数字段=专家笔误，校验门拒绝不静默忽略）。
type ConditionLeaf struct {
	Judge            string   `json:"judge,omitempty"`
	Target           RangeRef `yaml:"target,omitempty" json:"target,omitempty"`
	Reference        RangeRef `yaml:"reference,omitempty" json:"reference,omitempty"`
	AcceptTransposed *bool    `yaml:"accept_transposed,omitempty" json:"accept_transposed,omitempty"`
	SetMode          string   `yaml:"set_mode,omitempty" json:"set_mode,omitempty"`

	Event    string `json:"event,omitempty"`
	Key      string `json:"key,omitempty"` // event 叶（key 限定）/structure 叶保留位
	MinCount int    `yaml:"min_count,omitempty" json:"min_count,omitempty"`

	Structure string `json:"structure,omitempty"`
	Clip      string `json:"clip,omitempty"`  // structure: clip_exists 参数
	Track     string `json:"track,omitempty"` // structure: track_non_empty 参数

	Keys []string `json:"-"`
}

func (l *ConditionLeaf) unmarshalFrom(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("done_when leaf must be a mapping")
	}
	for i := 0; i+1 < len(value.Content); i += 2 {
		k, v := value.Content[i], value.Content[i+1]
		l.Keys = append(l.Keys, k.Value)
		var err error
		switch k.Value {
		case "judge":
			err = v.Decode(&l.Judge)
		case "target":
			err = v.Decode(&l.Target)
		case "reference":
			err = v.Decode(&l.Reference)
		case "accept_transposed":
			var b bool
			if err = v.Decode(&b); err == nil {
				l.AcceptTransposed = &b
			}
		case "set_mode":
			err = v.Decode(&l.SetMode)
		case "event":
			err = v.Decode(&l.Event)
		case "key":
			err = v.Decode(&l.Key)
		case "min_count":
			err = v.Decode(&l.MinCount)
		case "structure":
			err = v.Decode(&l.Structure)
		case "clip":
			err = v.Decode(&l.Clip)
		case "track":
			err = v.Decode(&l.Track)
		}
		if err != nil {
			return fmt.Errorf("done_when leaf field %q: %w", k.Value, err)
		}
	}
	return nil
}

// AssetNotes 返回资产的可判定音符投影（新增对齐项⑤）：解析 notes_ref 链
// （derived_from 沿链到基础表），合成 note id asset:<id>#<k>（k=表序），并
// 逐层应用 deviations（派生资产的损坏点）——与 279 MIDI 确定性投影对账基线一致。
func (c *LevelCard) AssetNotes(assetID string) ([]Note, bool) {
	// 收集 derived 链（self → base），环与深度护栏。
	chain := []string{}
	cur := assetID
	for i := 0; i < 8; i++ {
		spec := c.assetByID(cur)
		if spec == nil {
			return nil, false
		}
		if spec.NotesRef != "" {
			chain = append(chain, cur)
			break
		}
		if spec.DerivedFrom == "" {
			return nil, false
		}
		chain = append(chain, cur)
		cur = spec.DerivedFrom
	}
	base := chain[len(chain)-1]
	baseSpec := c.assetByID(base)
	if baseSpec == nil {
		return nil, false
	}
	tableName := strings.TrimPrefix(baseSpec.NotesRef, "notes.")
	table, ok := c.Notes[tableName]
	if !ok || len(table) == 0 {
		return nil, false
	}
	notes := make([]Note, len(table))
	for k, cn := range table {
		notes[k] = Note{
			ID:     fmt.Sprintf("asset:%s#%d", assetID, k),
			Pitch:  cn.Pitch,
			Start:  cn.Start,
			Length: cn.Length,
		}
	}
	// 沿链自 base 向派生方向应用 deviations（链序 base→…→assetID）。
	for i := len(chain) - 1; i >= 0; i-- {
		spec := c.assetByID(chain[i])
		for _, dev := range spec.Deviations {
			for k := range notes {
				if floatEq(notes[k].Start, dev.AtBeat) {
					notes[k].Pitch = dev.Pitch
				}
			}
		}
	}
	return notes, true
}

func (c *LevelCard) assetByID(id string) *AssetSpec {
	for i := range c.Assets {
		if c.Assets[i].ID == id {
			return &c.Assets[i]
		}
	}
	return nil
}

// ---- 解析入口 ----

// ParseLevelCard 解析并校验乐局卡字节流（零 IO）。解析失败或校验拒绝均返回
// error（拒绝原因聚合列出）。
func ParseLevelCard(data []byte) (*LevelCard, error) {
	var card LevelCard
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // 顶层/嵌套未知字段拒绝（专家笔误 fail-closed）
	if err := dec.Decode(&card); err != nil {
		return nil, fmt.Errorf("level card parse: %w", err)
	}
	if err := ValidateLevelCard(&card); err != nil {
		return nil, err
	}
	return &card, nil
}

// LoadLevelCard 从文件加载乐局卡并做文件级核对（资产文件存在+sha256 对账、
// 人格块源文件存在、卡 id 与目录名一致——内容侧惯例在加载口收口）。
func LoadLevelCard(path string) (*LevelCard, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("level card read: %w", err)
	}
	card, err := ParseLevelCard(data)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	if base := filepath.Base(dir); base != card.ID {
		return nil, fmt.Errorf("level card id %q != directory name %q", card.ID, base)
	}
	for i := range card.Assets {
		a := &card.Assets[i]
		if a.File == "" {
			continue
		}
		buf, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(a.File)))
		if err != nil {
			return nil, fmt.Errorf("asset %q file %s: %w", a.ID, a.File, err)
		}
		sum := sha256.Sum256(buf)
		if got := hex.EncodeToString(sum[:]); got != a.SHA256 {
			return nil, fmt.Errorf("asset %q sha256 mismatch: card=%s actual=%s", a.ID, a.SHA256, got)
		}
	}
	if card.PersonaSlot.Source != "" {
		if _, err := os.Stat(filepath.Join(dir, card.PersonaSlot.Source)); err != nil {
			return nil, fmt.Errorf("persona_slot.source %s: %w", card.PersonaSlot.Source, err)
		}
	}
	return card, nil
}

// ---- 校验门 ----

type levelCardValidator struct {
	card         *LevelCard
	errs         []string
	assetIDs     map[string]bool
	slotNames    map[string]bool
	nodeIDs      map[string]bool
	objectiveIDs map[string]bool
}

// ValidateLevelCard 执行 12+6 校验项；拒绝原因聚合返回（nil=通过）。
func ValidateLevelCard(card *LevelCard) error {
	v := &levelCardValidator{
		card:         card,
		assetIDs:     map[string]bool{},
		slotNames:    map[string]bool{},
		nodeIDs:      map[string]bool{},
		objectiveIDs: map[string]bool{},
	}
	v.validateTopLevel()
	v.validateTiming()
	v.validateAssets()
	v.validateNotesTables()
	v.validateRuntimeSlots()
	v.validateObjectives()
	v.validateNodes()
	v.validateTailBlocks()
	if len(v.errs) > 0 {
		sort.Strings(v.errs)
		return fmt.Errorf("level card rejected (%d 项):\n  - %s", len(v.errs), strings.Join(v.errs, "\n  - "))
	}
	return nil
}

func (v *levelCardValidator) failf(format string, args ...any) {
	v.errs = append(v.errs, fmt.Sprintf(format, args...))
}

func (v *levelCardValidator) validateTopLevel() {
	c := v.card
	if c.SchemaVersion != LevelCardSchemaVersion { // 拒绝路径 11
		v.failf("[11] schema_version %q != %q（restore fail-closed 同型：拒绝未知版本）", c.SchemaVersion, LevelCardSchemaVersion)
	}
	if strings.TrimSpace(c.ID) == "" {
		v.failf("id 不能为空")
	}
	if strings.TrimSpace(c.Title) == "" {
		v.failf("title 不能为空")
	}
	if c.LeadDefault != "human" && c.LeadDefault != "model" {
		v.failf("lead_default 须为 human|model（got %q）", c.LeadDefault)
	}
	hasHuman, hasModel := false, false
	for _, p := range c.Players {
		if p == "human" {
			hasHuman = true
		}
		if p == "model" {
			hasModel = true
		}
	}
	if !hasHuman || !hasModel {
		v.failf("players 须含 human 与 model（got %v）", c.Players)
	}
	for i := range c.Assets {
		a := &c.Assets[i]
		if strings.TrimSpace(a.ID) == "" {
			v.failf("assets[%d].id 为空", i)
			continue
		}
		if v.assetIDs[a.ID] {
			v.failf("[10] assets id 重复: %s", a.ID)
		}
		v.assetIDs[a.ID] = true
	}
	for _, n := range c.Nodes {
		if v.nodeIDs[n.ID] {
			v.failf("[9] node.id 重复: %s", n.ID) // 拒绝路径 9
		}
		v.nodeIDs[n.ID] = true
	}
	for _, o := range c.Objectives {
		if v.objectiveIDs[o.ID] {
			v.failf("objectives id 重复: %s", o.ID)
		}
		v.objectiveIDs[o.ID] = true
	}
	for name := range c.RuntimeSlots {
		v.slotNames[name] = true
	}
}

func (v *levelCardValidator) validateTiming() {
	t := v.card.Timing
	if t.BeatsPerBar < 1 {
		v.failf("timing.beats_per_bar 须为正整数（got %d）", t.BeatsPerBar)
	}
	if t.BarSpan.FirstBar < 1 || t.BarSpan.LastBar < t.BarSpan.FirstBar {
		v.failf("timing.bar_span 非法（first=%d last=%d，须 1≤first≤last）", t.BarSpan.FirstBar, t.BarSpan.LastBar)
	}
	if t.BPMDefault <= 0 {
		v.failf("timing.bpm_default 须为正数（got %v）", t.BPMDefault)
	}
	for name, o := range t.ClipOrigins {
		if o.StartBeatsGlobal < 0 {
			v.failf("timing.clip_origins[%s].start_beats_global 为负", name)
		}
		if o.FirstBar < 1 {
			v.failf("timing.clip_origins[%s].first_bar 须 ≥1", name)
		}
	}
}

func (v *levelCardValidator) validateAssets() {
	for i := range v.card.Assets {
		a := &v.card.Assets[i]
		where := fmt.Sprintf("assets[%s]", a.ID)
		if a.File != "" { // 拒绝路径 10：带文件的资产必带 sha256
			if !sha256Pattern.MatchString(a.SHA256) {
				v.failf("[10] %s 缺 sha256 或非 64 位小写 hex（got %q）", where, a.SHA256)
			}
		}
		if a.DerivedFrom != "" && !v.assetIDs[a.DerivedFrom] {
			v.failf("[10] %s.derived_from 引用未知资产 %q", where, a.DerivedFrom)
		}
		if a.NotesRef != "" {
			if err := v.resolveNotesRef(a.NotesRef); err != nil {
				v.failf("[10] %s.notes_ref %s（got %q）", where, err, a.NotesRef)
			}
		}
		if a.Kind == "midi_reference" && a.File != "" && a.NotesRef == "" && a.DerivedFrom == "" {
			v.failf("[10] %s midi_reference 带文件但缺 notes_ref（或 derived_from 派生标记）", where)
		}
		if a.Key != nil { // 拒绝路径 7 同型（首版仅大调）
			if _, err := prepareKey(*a.Key); err != nil {
				v.failf("[7] %s.key 非法: %v", where, err)
			}
		}
		// deviations：replaces 必须命中所指基础表音高（含 derived 链解析）。
		if len(a.Deviations) > 0 {
			if _, ok := v.card.AssetNotes(a.ID); !ok {
				v.failf("[10] %s deviations 无法解析基础 notes 表", where)
			} else {
				for _, d := range a.Deviations {
					if !deviationHitsBase(v.card, a, d) {
						v.failf("%s.deviations %+v 未命中原表音（replaces 不符）", where, d)
					}
				}
			}
		}
	}
}

// deviationHitsBase 校验 deviation 的 replaces 命中「基础表（未应用任何
// deviation）」中 at_beat 处的唯一音高。
func deviationHitsBase(card *LevelCard, a *AssetSpec, d DeviationSpec) bool {
	// 求基础表（沿 derived_from 链到 notes_ref 持有者，不应用 deviations）。
	cur := a
	for i := 0; i < 8 && cur != nil; i++ {
		if cur.NotesRef != "" {
			tableName := strings.TrimPrefix(cur.NotesRef, "notes.")
			table, ok := card.Notes[tableName]
			if !ok {
				return false
			}
			hits := 0
			matched := false
			for _, n := range table {
				if floatEq(n.Start, d.AtBeat) {
					hits++
					matched = n.Pitch == d.Replaces
				}
			}
			return hits == 1 && matched
		}
		cur = card.assetByID(cur.DerivedFrom)
	}
	return false
}

func (v *levelCardValidator) resolveNotesRef(ref string) error {
	parts := strings.Split(ref, ".")
	if len(parts) != 2 || parts[0] != "notes" {
		return fmt.Errorf("须为 notes.<表名> 形态")
	}
	table, ok := v.card.Notes[parts[1]]
	if !ok || len(table) == 0 {
		return fmt.Errorf("引用未知/空 notes 表")
	}
	return nil
}

func (v *levelCardValidator) validateNotesTables() {
	for name, table := range v.card.Notes {
		for k, n := range table {
			if n.Pitch < 0 || n.Pitch > 127 {
				v.failf("notes.%s[%d].pitch 越界（%d）", name, k, n.Pitch)
			}
			if n.Start < 0 {
				v.failf("notes.%s[%d].start 为负", name, k)
			}
			if n.Length <= 0 {
				v.failf("notes.%s[%d].length 须为正", name, k)
			}
		}
	}
}

// validateRuntimeSlots 落实新增对齐项⑥（280 建议裁定）：runtime_slots 升格
// 必检卡字段——块必在、track_index 唯一、preload 引用可解；不硬编码轨数
// （三轨=first-run-twinkle 实例约束，留在内容侧 lint）。
func (v *levelCardValidator) validateRuntimeSlots() {
	slots := v.card.RuntimeSlots
	if len(slots) == 0 {
		v.failf("[⑥] runtime_slots 必填（轨槽是 target.clip/hook clip_id/结构谓词的寻址坐标系；升格建议见卡面回执）")
		return
	}
	seen := map[int]string{}
	names := make([]string, 0, len(slots))
	for name := range slots {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := slots[name]
		holder, dup := seen[s.TrackIndex]
		if dup {
			v.failf("[⑥] runtime_slots track_index %d 重复（%s 与 %s）", s.TrackIndex, holder, name)
		}
		seen[s.TrackIndex] = name
		if s.Preload != nil && !v.assetIDs[*s.Preload] {
			v.failf("[10] runtime_slots[%s].preload 引用未知资产 %q", name, *s.Preload)
		}
	}
}

func (v *levelCardValidator) validateObjectives() {
	for _, o := range v.card.Objectives {
		where := fmt.Sprintf("objectives[%s]", o.ID)
		if strings.TrimSpace(o.Text) == "" {
			v.failf("%s.text 为空", where)
		}
		c := o.Check
		allowed := map[string]bool{"judge": true, "event": true, "key": true, "node": true, "kind": true, "rubric": true}
		for _, k := range c.Keys {
			if !allowed[k] {
				v.failf("%s.check 未知字段 %q", where, k)
			}
		}
		switch {
		case c.Judge != "":
			if _, ok := LookupJudge(c.Judge); !ok {
				v.failf("[12] %s.check 引用未知 judge %q", where, c.Judge)
			}
			if c.Node != "" && !v.nodeIDs[c.Node] {
				v.failf("[12] %s.check.node 引用未知节点 %q", where, c.Node)
			}
		case c.Event != "":
			if !levelEventSet[c.Event] {
				v.failf("[12] %s.check 引用未知 event %q", where, c.Event)
			}
		case c.Kind == "verbal_rubric":
			if len(c.Rubric) == 0 { // 显式豁免标记：教学侧非判定器，但 rubric 必填
				v.failf("[12] %s.check verbal_rubric 缺 rubric", where)
			}
		default:
			v.failf("[12] %s.check 无可识别形态（judge/event/kind=verbal_rubric）", where)
		}
	}
}

func (v *levelCardValidator) validateNodes() {
	if len(v.card.Nodes) == 0 {
		v.failf("nodes 不能为空")
		return
	}
	for i := range v.card.Nodes {
		n := &v.card.Nodes[i]
		where := fmt.Sprintf("nodes[%s]", n.ID)
		if strings.TrimSpace(n.ID) == "" {
			v.failf("nodes[%d].id 为空", i)
			continue
		}
		if n.Lead != "human" && n.Lead != "model" {
			v.failf("%s.lead 须为 human|model（got %q）", where, n.Lead)
		}
		if n.DoneWhen.Leaf == nil && len(n.DoneWhen.All) == 0 && len(n.DoneWhen.Any) == 0 {
			v.failf("%s 缺 done_when", where)
			continue
		}
		v.validateDoneWhen(where+".done_when", &n.DoneWhen, true)
		for _, ref := range n.Teaching.Objectives {
			if !v.objectiveIDs[ref] {
				v.failf("%s.teaching.objectives 引用未知目标 %q", where, ref)
			}
		}
		for h := range n.Teaching.EvidenceHooks {
			v.validateHook(fmt.Sprintf("%s.hooks[%s]", where, n.Teaching.EvidenceHooks[h].ID), &n.Teaching.EvidenceHooks[h])
		}
	}
}

// validateDoneWhen 递归校验条件树（组合子深度 ≤1=新增对齐项④；枚举=路径 1/2/3；
// 区间/引用=路径 4/5/10）。
func (v *levelCardValidator) validateDoneWhen(where string, dw *DoneWhen, isRoot bool) {
	if dw.Leaf != nil {
		if len(dw.All) > 0 || len(dw.Any) > 0 {
			v.failf("%s 叶子与组合子混写", where)
			return
		}
		v.validateLeaf(where, dw.Leaf)
		return
	}
	children := dw.All
	comb := "all"
	if len(children) == 0 {
		children = dw.Any
		comb = "any"
	}
	if len(children) == 0 {
		v.failf("%s 组合子 %s 无子条件", where, comb)
		return
	}
	if !isRoot {
		v.failf("[④] %s 组合子嵌套超过一层（all/any 内不得再含组合子）", where)
		return
	}
	for i := range children {
		v.validateDoneWhen(fmt.Sprintf("%s.%s[%d]", where, comb, i), &children[i], false)
	}
}

var leafJudgeKeys = map[string]bool{
	"judge": true, "target": true, "reference": true, "accept_transposed": true, "set_mode": true,
}

func (v *levelCardValidator) validateLeaf(where string, l *ConditionLeaf) {
	disc := 0
	if l.Judge != "" {
		disc++
	}
	if l.Event != "" {
		disc++
	}
	if l.Structure != "" {
		disc++
	}
	if disc != 1 {
		v.failf("%s 叶子须恰含一个判别键 judge|event|structure（got keys=%v）", where, l.Keys)
		return
	}
	switch {
	case l.Judge != "": // 拒绝路径 1 + 新增②③
		if _, ok := LookupJudge(l.Judge); !ok {
			v.failf("[1] %s 未知 judge %q（注册表闭集: %v）", where, l.Judge, RegisteredJudges())
			return
		}
		for _, k := range l.Keys {
			if !leafJudgeKeys[k] {
				v.failf("%s judge 叶未知参数字段 %q", where, k)
			}
		}
		if l.AcceptTransposed != nil && l.Judge != JudgeNoteSequenceMatch {
			v.failf("[③] %s accept_transposed 仅 note_sequence_match 可带（got judge=%s）", where, l.Judge)
		}
		if l.SetMode != "" {
			if l.Judge != JudgePitchSetMatch {
				v.failf("[②] %s set_mode 仅 pitch_set_match 可带（got judge=%s）", where, l.Judge)
			} else if l.SetMode != PitchSetModePitchClass && l.SetMode != PitchSetModeAbsolute {
				v.failf("[②] %s set_mode 未知口径 %q（闭集: %v）", where, l.SetMode, PitchSetModes)
			}
		}
		if !v.slotNames[l.Target.Clip] {
			v.failf("[10] %s.target.clip 引用未知轨槽 %q", where, l.Target.Clip)
		}
		v.checkRangeRef(where+".target", l.Target, "")
		switch {
		case l.Reference.Asset != "":
			if !v.assetIDs[l.Reference.Asset] {
				v.failf("[10] %s.reference.asset 引用未知资产 %q", where, l.Reference.Asset)
			} else {
				v.checkRangeRef(where+".reference", l.Reference, l.Reference.Asset)
			}
			if l.Reference.Clip != "" {
				v.failf("%s.reference asset 与 clip 互斥", where)
			}
		case l.Reference.Clip != "":
			if !v.slotNames[l.Reference.Clip] {
				v.failf("[10] %s.reference.clip 引用未知轨槽 %q", where, l.Reference.Clip)
			}
			v.checkRangeRef(where+".reference", l.Reference, "")
		default:
			v.failf("%s.reference 缺 asset 或 clip", where)
		}

	case l.Event != "": // 拒绝路径 2
		if !levelEventSet[l.Event] {
			v.failf("[2] %s 未知 event %q（闭集: %v）", where, l.Event, LevelEventNames)
			return
		}
		for _, k := range l.Keys {
			if k != "event" && k != "key" && k != "min_count" {
				v.failf("%s event 叶未知参数字段 %q", where, k)
			}
		}
		if l.MinCount < 0 {
			v.failf("%s.min_count 须 ≥0", where)
		}

	case l.Structure != "": // 拒绝路径 3
		if !levelStructureSet[l.Structure] {
			v.failf("[3] %s 未知 structure 谓词 %q（闭集: %v）", where, l.Structure, LevelStructureNames)
			return
		}
		for _, k := range l.Keys {
			if k != "structure" && k != "clip" && k != "track" && k != "key" {
				v.failf("%s structure 叶未知参数字段 %q", where, k)
			}
		}
		switch l.Structure {
		case "clip_exists":
			if !v.slotNames[l.Clip] {
				v.failf("[10] %s.structure clip_exists 引用未知轨槽 %q", where, l.Clip)
			}
		case "track_non_empty":
			if strings.TrimSpace(l.Track) == "" {
				v.failf("%s.structure track_non_empty 缺 track", where)
			}
		}
	}
}

// checkRangeRef 校验区间寻址（路径 4/5）：range_beats 形态与域、range_bars
// 域与换算、两者一致性、资产表覆盖。originKey 非空时按 clip_origins 换算
// bar 窗口（缺 origin 条目且带 range_bars → fail-closed）。
func (v *levelCardValidator) checkRangeRef(where string, ref RangeRef, assetID string) {
	t := v.card.Timing
	spanBeats := t.BarSpanBeats()
	tableEnd := math.Inf(1)
	if assetID != "" {
		if notes, ok := v.card.AssetNotes(assetID); ok {
			end := 0.0
			for _, n := range notes {
				if e := n.Start + n.Length; e > end {
					end = e
				}
			}
			tableEnd = end
		}
	}
	hasBeats := len(ref.RangeBeats) > 0
	hasBars := len(ref.RangeBars) > 0
	if hasBeats && len(ref.RangeBeats) != 2 {
		v.failf("[5] %s.range_beats 须为 [start, end) 二元组", where)
		return
	}
	if hasBars && len(ref.RangeBars) != 2 {
		v.failf("[4] %s.range_bars 须为 [first, last] 二元组", where)
		return
	}
	if !hasBeats && !hasBars {
		return // 全片段（缺省 [0,+Inf)）：域检查交由表覆盖（无表则免检）
	}
	var beatStart, beatEnd float64
	if hasBeats {
		s, e := ref.RangeBeats[0], ref.RangeBeats[1]
		if s < 0 || e <= s { // 拒绝路径 5：逆序/零宽/负
			v.failf("[5] %s.range_beats 逆序/零宽/负（[%.6g, %.6g)）", where, s, e)
			return
		}
		if e > spanBeats+beatEpsilon { // 拒绝路径 4：超 bar_span
			v.failf("[4] %s.range_beats 超出 bar_span 覆盖（end=%.6g > span=%.6g）", where, e, spanBeats)
			return
		}
		if e > tableEnd+beatEpsilon { // 拒绝路径 4：不被资产表覆盖
			v.failf("[4] %s.range_beats 不被 notes 表覆盖（end=%.6g > table_end=%.6g）", where, e, tableEnd)
			return
		}
		beatStart, beatEnd = s, e
	}
	if hasBars {
		b0, b1 := ref.RangeBars[0], ref.RangeBars[1]
		if b0 < t.BarSpan.FirstBar || b1 > t.BarSpan.LastBar || b0 > b1 {
			v.failf("[4] %s.range_bars 越界（[%d,%d] ∉ bar_span[%d,%d]）", where, b0, b1, t.BarSpan.FirstBar, t.BarSpan.LastBar)
			return
		}
		originKey := ref.Clip
		if originKey == "" {
			originKey = ref.Asset
		}
		origin, ok := t.ClipOrigins[originKey]
		if !ok {
			v.failf("[4] %s.range_bars 需 timing.clip_origins[%s] 换算依据（缺条目，fail-closed）", where, originKey)
			return
		}
		barBeats := float64(t.BeatsPerBar)
		w0 := origin.StartBeatsGlobal + float64(b0-origin.FirstBar)*barBeats
		w1 := origin.StartBeatsGlobal + float64(b1-origin.FirstBar+1)*barBeats
		if w1 > tableEnd+beatEpsilon {
			v.failf("[4] %s.range_bars 窗口 [%.6g, %.6g) 不被 notes 表覆盖", where, w0, w1)
			return
		}
		if hasBeats && !floatEq(w0, beatStart) { // 拒绝路径 5：两形态不一致
			v.failf("[5] %s range_bars 与 range_beats 不一致（bars→[%.6g, %.6g) vs beats→[%.6g, %.6g)）",
				where, w0, w1, beatStart, beatEnd)
		}
	}
}

func (v *levelCardValidator) validateHook(where string, h *EvidenceHook) {
	if strings.TrimSpace(h.ID) == "" {
		v.failf("%s.id 为空", where)
	}
	if h.Analyze != levelHookAnalyzeName {
		v.failf("%s.analyze 未知分析面 %q（闭集: %s）", where, h.Analyze, levelHookAnalyzeName)
		return
	}
	// 参数族：先 fail-closed 键检查，再整包复用运行时 parseAnalyzeArgs
	//（路径 5/6/7/8 与命令面零语义漂移）。
	allowedKeys := map[string]bool{
		"clip_id": true, "track_id": true, "range_beats": true,
		"transformations": true, "min_match_notes": true, "key": true,
	}
	for k := range h.Params {
		if !allowedKeys[k] {
			v.failf("%s.params 未知参数字段 %q（超出 parseAnalyzeArgs 字段族）", where, k)
		}
	}
	if req, err := parseAnalyzeArgs(h.Params); err != nil {
		v.failf("%s.params %v", where, err)
	} else if !v.slotNames[req.ClipID] {
		v.failf("[10] %s.params.clip_id 引用未知轨槽 %q（运行时轨槽名绑定）", where, req.ClipID)
	}
	if len(h.ExpectedFindings) == 0 && len(h.ExpectAbsent) == 0 && len(h.ExpectPresent) == 0 {
		v.failf("%s 缺 expected_findings/expect_absent/expect_present 之一", where)
	}
	for _, section := range []struct {
		name  string
		items []FindingAssert
	}{
		{"expected_findings", h.ExpectedFindings},
		{"expect_absent", h.ExpectAbsent},
		{"expect_present", h.ExpectPresent},
	} {
		for _, f := range section.items {
			w := fmt.Sprintf("%s.%s[%s]", where, section.name, f.Kind)
			if f.Kind != string(KindRepeat) && f.Kind != string(KindSequence) &&
				f.Kind != string(KindInversion) && f.Kind != string(KindRetrograde) {
				v.failf("[8] %s kind 白名单外 %q", w, f.Kind)
			}
			if f.SequenceQuality != "" && f.SequenceQuality != string(SeqChromatic) && f.SequenceQuality != string(SeqDiatonic) {
				v.failf("%s sequence_quality 非法 %q", w, f.SequenceQuality)
			}
			for _, bk := range []struct {
				name string
				bars []int
			}{{"proto_bars", f.ProtoBars}, {"match_bars", f.MatchBars}} {
				if len(bk.bars) == 0 {
					continue
				}
				if len(bk.bars) != 2 {
					v.failf("[4] %s.%s 须为二元组", w, bk.name)
					continue
				}
				if bk.bars[0] < v.card.Timing.BarSpan.FirstBar || bk.bars[1] > v.card.Timing.BarSpan.LastBar || bk.bars[0] > bk.bars[1] {
					v.failf("[4] %s.%s 越界（%v ∉ bar_span）", w, bk.name, bk.bars)
				}
			}
		}
	}
}

func (v *levelCardValidator) validateTailBlocks() {
	g := v.card.Guidance
	if g.PullbackAfterTurns < 1 {
		v.failf("guidance.pullback_after_turns 须为正整数（got %d）", g.PullbackAfterTurns)
	}
	wb := v.card.WriteBoundary
	if len(wb.Allow) == 0 {
		v.failf("write_boundary.allow 不能为空")
	}
	seenCmd := map[string]bool{}
	for _, cmd := range wb.Allow {
		if strings.TrimSpace(cmd) == "" {
			v.failf("write_boundary.allow 含空命令名")
		}
		if seenCmd[cmd] {
			v.failf("write_boundary.allow 重复项 %q", cmd)
		}
		seenCmd[cmd] = true
	}
	// v0 fail-closed：confirm_downgrade 只认 none（用户裁定 2026-11-07 乐局内
	// 不降级；新增值须先扩本门再写卡）。
	if wb.ConfirmDowngrade != "none" {
		v.failf("write_boundary.confirm_downgrade v0 仅认 %q（got %q）", "none", wb.ConfirmDowngrade)
	}
	for _, slot := range v.card.Finale.ClosureCheck.StructureComplete.NoPlaceholderBlanks {
		if !v.slotNames[slot] {
			v.failf("[10] finale no_placeholder_blanks 引用未知轨槽 %q", slot)
		}
	}
	b := v.card.Budget
	if b.MaxTurns < 1 {
		v.failf("budget.max_turns 须为正整数")
	}
	if b.MaxLLMCalls < 1 {
		v.failf("budget.max_llm_calls 须为正整数")
	}
	if strings.TrimSpace(b.OnStopLoss) == "" {
		v.failf("budget.on_stop_loss 缺失（§8 止损口径）")
	}
}

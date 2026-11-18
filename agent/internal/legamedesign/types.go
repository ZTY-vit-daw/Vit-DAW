// Package legamedesign 实现乐局设计源 vit.legame_design.v0 的解析与语义校验门。
//
// 卡：G0-LEGAME-PARSE-1（W01-A）；权威契约 docs/LEGAME_DESIGN_CONTRACT_V0.md
// （v0-frozen，2026-11-18 用户裁定 R1–R6 回填；§3/§4/§5/§7）。
//
// 边界（越界不属于本包）：
//
//   - 只做 Parse+Validate：不实现 design→level_card 编译（W01-B），不做工程包装配、
//     也不做"是否启用乐局"的判定（W01-C/425）。启用由项目包元数据
//     legame.{enabled,design} 决定（R2）；YAML 的 schema_version 仅承担版本键职责。
//   - 预留版本 vit.project_intent.v0 仅登记（R1）：识别为预留值、不报错、不做 L1
//     校验；非乐局解析路径（M2+）不在本包实现。
//   - 零 IO：Parse 只接受字节流。materials[].file 的存在性/哈希对账（MAT-FILE）、
//     catalog 核验状态（W-MAT 第三触发）、§4.11 显式关闭证据记录的 W、§4.3 空
//     evidence_checks 的 W、T-15/T-16 映射降级 W 等编译期规则归 W01-B（契约 §5.2
//     未给这些项规则号，本包不抢实现）。
//   - 禁止静默纠正：未知字段（yaml KnownFields(true)）、未知枚举、类型错、引用悬空、
//     L1 必填段缺失一律聚合为 E 诊断返回；唯一允许的归一是 R6b 明示的
//     locator.music_region 单值/列表→[]string（契约 §4.4）。
//   - R5（tempo/budget 缺失→E，不注入缺省）是编译期 fail-closed 规则：源校验入口
//     （Parse/Validate）允许设计期显式 null（契约 §4.6/§4.14 的"编译 E+覆盖参数"），
//     R5 判定由 ValidateCompileReadiness 在"未提供的值也无显式覆盖参数"时执行，
//     W01-B 编译前必须调用（卡面 R5 与本卡"正例 0 E"并存的落点，见回执说明）。
package legamedesign

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ---- 版本键（契约 §3.3 / §4.1）----

const (
	// SchemaVersionLegameDesign 是乐局设计源唯一认知的版本键。
	SchemaVersionLegameDesign = "vit.legame_design.v0"
	// SchemaVersionProjectIntent 是预留的非乐局意图文档版本键（R1=A）：
	// 仅登记；非乐局解析/注入路径（M2+）未实现，本包识别后不按乐局校验、不报错。
	SchemaVersionProjectIntent = "vit.project_intent.v0"
)

// ---- 闭集枚举（契约 §4；"枚举"为 v0 闭集，新增值须先扩契约再扩实现）----

var (
	// ObjectiveKinds 是 objectives[].kind 闭集。
	ObjectiveKinds = []string{"learning", "music_task"}
	// MaterialKinds 是 materials[].kind 闭集。
	MaterialKinds = []string{"text", "image", "symbolic_reference", "midi_or_audio", "midi", "material_group"}
	// TrackRoles 是 workspace.tracks[].role 闭集。
	TrackRoles = []string{"reference", "user_work", "assist_generated", "generated_result"}
	// CheckPurposes 是 checks[].purpose 闭集。
	CheckPurposes = []string{"task_progress", "learning_evidence"}
	// CheckMethods 是 checks[].method 闭集。
	CheckMethods = []string{"user_judgment", "rubric", "deterministic"}
	// CheckOnUnavailable 是 checks[].on_unavailable 闭集。
	CheckOnUnavailable = []string{"request_review", "pause_and_report"}
	// NodeLeads 是 flow.nodes[].lead 闭集。
	NodeLeads = []string{"human", "model"}
	// FlowProgressions 是 flow.progression 闭集（v0 仅 ordered）。
	FlowProgressions = []string{"ordered"}
	// LocatorScopes 是 workspace.regions[].locator.scope 闭集。
	LocatorScopes = []string{"selected_reference_excerpt", "four_clips", "prototype_and_variant", "generated_audio_versions"}
	// EvidenceGranularities 是 evidence_recording.requested_granularity 闭集。
	EvidenceGranularities = []string{"key_action", "region", "saved_revision"}
	// EvidenceCheckpoints 是 evidence_recording.checkpoints 闭集。
	EvidenceCheckpoints = []string{"node_entry", "meaningful_revision", "submission"}
	// EvidenceUnknownSources 是 evidence_recording.unknown_source 闭集（v0 仅此值）。
	EvidenceUnknownSources = []string{"mark_unverified"}
	// SubmissionOnReturns 是 submission.on_return 闭集（v0 仅此值）。
	SubmissionOnReturns = []string{"preserve_history_and_continue"}
	// SubmissionReports 是 submission.reports 闭集。
	SubmissionReports = []string{"assignment_report", "key_context_summary"}
	// CompletionExports 是 completion.export 闭集（v0 仅此值）。
	CompletionExports = []string{"optional"}
	// BudgetOnLimits 是 budget.on_limit 闭集（v0 仅此值）。
	BudgetOnLimits = []string{"preserve_and_pause"}
)

// ---- 调号 30 调封闭枚举（R6a=C）----

// MajorKeySignatures 是 15 个大调（R6a=C；扩值须先扩契约）。
var MajorKeySignatures = []string{"C", "G", "D", "A", "E", "B", "F#", "C#", "F", "Bb", "Eb", "Ab", "Db", "Gb", "Cb"}

// MinorKeySignatures 是 15 个关系小调（R6a=C）。
var MinorKeySignatures = []string{"Am", "Em", "Bm", "F#m", "C#m", "G#m", "D#m", "A#m", "Dm", "Gm", "Cm", "Fm", "Bbm", "Ebm", "Abm"}

var keySignatureSet = func() map[string]bool {
	m := make(map[string]bool, len(MajorKeySignatures)+len(MinorKeySignatures))
	for _, k := range MajorKeySignatures {
		m[k] = true
	}
	for _, k := range MinorKeySignatures {
		m[k] = true
	}
	return m
}()

// IsValidKeySignature 报告 token 是否属于 R6a=C 的 30 调封闭枚举
// （token 写法=大写根音+可选 #/b，小调加后缀 m；集合外一律 E-SCHEMA）。
func IsValidKeySignature(token string) bool { return keySignatureSet[token] }

// ---- 结构（yaml 标签与 coord/lesson-designs/music-creation-unit2/design.v1.yaml 逐键对齐）----

// Design 是 vit.legame_design.v0 设计源的结构化形态（契约 §4.1 顶层 17 段）。
type Design struct {
	SchemaVersion      string              `yaml:"schema_version"`
	ID                 string              `yaml:"id"`
	Revision           *int                `yaml:"revision"`
	Title              string              `yaml:"title"`
	Brief              *BriefBlock         `yaml:"brief"`
	Objectives         []Objective         `yaml:"objectives"`
	Materials          []Material          `yaml:"materials"`
	Workspace          *Workspace          `yaml:"workspace"`
	CapabilityRequests []CapabilityRequest `yaml:"capability_requests"`
	Checks             []Check             `yaml:"checks"`
	Flow               *Flow               `yaml:"flow"`
	Guidance           *GuidanceBlock      `yaml:"guidance"`
	EvidenceRecording  *EvidenceRecording  `yaml:"evidence_recording"`
	Submission         *Submission         `yaml:"submission"`
	Completion         *Completion         `yaml:"completion"`
	Budget             *Budget             `yaml:"budget"`
	PendingQuestions   []string            `yaml:"pending_questions"`
}

// BriefBlock 是 brief（L0 可选，全子字段可选；契约 §4.2）。
type BriefBlock struct {
	Audience         string   `yaml:"audience"`
	MusicTask        string   `yaml:"music_task"`
	FinalDeliverable string   `yaml:"final_deliverable"`
	Prerequisites    []string `yaml:"prerequisites"`
	EstimatedMinutes *int     `yaml:"estimated_minutes"` // null=不以课时限制（仅 I-NOTE）
}

// Objective 是 objectives[]（L1 必填段；契约 §4.3）。
type Objective struct {
	ID             string   `yaml:"id"`
	Kind           string   `yaml:"kind"`
	Description    string   `yaml:"description"`
	EvidenceChecks []string `yaml:"evidence_checks"`
}

// Material 是 materials[]（L1 必填段；契约 §4.4）。
type Material struct {
	ID      string           `yaml:"id"`
	Kind    string           `yaml:"kind"`
	File    *string          `yaml:"file"` // null=登记未备料（合法；W-MAT）
	Purpose string           `yaml:"purpose"`
	Source  string           `yaml:"source"`
	Locator *MaterialLocator `yaml:"locator"`
}

// MaterialLocator 是材料 locator（§4.4；pdf/printed pages 可空可缺，music_region 三态归一）。
type MaterialLocator struct {
	PDFPages     []int       `yaml:"pdf_pages"`
	PrintedPages []int       `yaml:"printed_pages"`
	MusicRegion  MusicRegion `yaml:"music_region"`
}

// MusicRegion 是 locator.music_region 的归一形态（R6b=A）：设计源允许单字符串、
// 字符串列表或 null，解析后统一为 []string（null/缺省=nil）。
// 该归一是契约明示的归一，不属于"静默纠正"。
type MusicRegion []string

// UnmarshalYAML 接受标量字符串/字符串列表/null 三态并归一为 []string；
// 其他 YAML 形态/类型一律返回错误（经 Parse 聚合为 E-SCHEMA）。
func (m *MusicRegion) UnmarshalYAML(value *yaml.Node) error {
	for value.Kind == yaml.AliasNode {
		value = value.Alias
	}
	switch value.Kind {
	case yaml.ScalarNode:
		if value.Tag == "!!null" {
			*m = nil
			return nil
		}
		if value.Tag != "!!str" {
			return fmt.Errorf("music_region 须为字符串或字符串列表（got %s 标量）", value.Tag)
		}
		var s string
		if err := value.Decode(&s); err != nil {
			return fmt.Errorf("music_region 须为字符串或字符串列表: %w", err)
		}
		*m = MusicRegion{s}
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			return fmt.Errorf("music_region 列表项须为字符串: %w", err)
		}
		*m = MusicRegion(list)
		return nil
	default:
		return fmt.Errorf("music_region 须为字符串/字符串列表/null（got YAML kind %v）", value.Kind)
	}
}

// Workspace 是 workspace（L0/L1 混合；契约 §4.6）。
type Workspace struct {
	Timing  *Timing  `yaml:"timing"`
	Tracks  []Track  `yaml:"tracks"`
	Regions []Region `yaml:"regions"` // L1 必填（非空）
}

// Timing 是 workspace.timing（L0 可选；§4.6 两层计量落点）。
type Timing struct {
	TimeSignature *string  `yaml:"time_signature"` // N/D（N,D 正整数）
	KeySignature  *string  `yaml:"key_signature"`  // R6a=C 30 调封闭枚举；设计层字段不进卡（T-10）
	TempoBPM      *float64 `yaml:"tempo_bpm"`      // >0；null=未定（R5 编译 E+覆盖参数）
	PickupBeats   *float64 `yaml:"pickup_beats"`   // >=0（缺省 0）
}

// Track 是 workspace.tracks[]（L1 可选；§4.6）。
type Track struct {
	ID              string  `yaml:"id"`
	Name            string  `yaml:"name"`
	Role            string  `yaml:"role"`
	InitialMaterial *string `yaml:"initial_material"` // null 合法；非 null ⊆ materials[].id（REF-MAT）
	EditableByUser  *bool   `yaml:"editable_by_user"` // 设计层字段不进卡（I-NOTE）
}

// Region 是 workspace.regions[]（L1 必填段；§4.6）。
type Region struct {
	ID      string         `yaml:"id"`
	Track   string         `yaml:"track"`   // ⊆ tracks[].id（REF-REGION）
	Locator *RegionLocator `yaml:"locator"` // 联合形态闭集，恰一种寻址语义（REG-LOC）
	Purpose string         `yaml:"purpose"`
}

// RegionLocator 是 region locator 联合形态闭集（§4.6）：
// scope 型 / clip 型（clip_alias）/ 多 clip 型（scope=four_clips + clip_aliases + bars_per_clip），
// 外加 region 级 time_signature 计量覆盖声明（须=工程计量，REG-METER）。
type RegionLocator struct {
	Scope         *string  `yaml:"scope"`
	ClipAlias     *string  `yaml:"clip_alias"`
	ClipAliases   []string `yaml:"clip_aliases"`
	Bars          *int     `yaml:"bars"`
	BarsPerClip   *int     `yaml:"bars_per_clip"`
	TimeSignature *string  `yaml:"time_signature"`
}

// CapabilityRequest 是 capability_requests[]（L1 可选；§4.7）。
type CapabilityRequest struct {
	ID       string  `yaml:"id"`
	Request  string  `yaml:"request"`
	Required *bool   `yaml:"required"` // required=true 且 CAP-BRIDGE 闭包表无条目→E-CAP
	Fallback *string `yaml:"fallback"` // null 合法=无 fallback
}

// Check 是 checks[]（L1 必填段；§4.8）。
type Check struct {
	ID            string `yaml:"id"`
	Purpose       string `yaml:"purpose"`
	Method        string `yaml:"method"`
	Target        string `yaml:"target"` // ⊆ regions[].id ∪ {submission_snapshot}（REF-CHK）
	Criterion     string `yaml:"criterion"`
	Evidence      string `yaml:"evidence"`
	OnUnavailable string `yaml:"on_unavailable"`
}

// Flow 是 flow（L1 必填段；§4.9）。
type Flow struct {
	Progression string `yaml:"progression"`
	Nodes       []Node `yaml:"nodes"`
}

// Node 是 flow.nodes[]（§4.9）。
type Node struct {
	ID               string        `yaml:"id"`
	Title            string        `yaml:"title"`
	ObjectiveRefs    []string      `yaml:"objective_refs"` // 空表合法；每项 ⊆ objectives[].id
	Task             string        `yaml:"task"`
	Lead             string        `yaml:"lead"`
	UsesMaterials    []string      `yaml:"uses_materials"`    // 不进卡（I-NOTE；注入语境 W02）
	UsesRegions      []string      `yaml:"uses_regions"`      // 不进卡（I-NOTE）
	UsesCapabilities []string      `yaml:"uses_capabilities"` // 不进卡（I-NOTE）
	HumanDecides     []string      `yaml:"human_decides"`     // 不进卡（I-NOTE）
	ModelMayDo       []string      `yaml:"model_may_do"`      // 编译→卡 model_does（T-16）
	Freedom          *NodeFreedom  `yaml:"freedom"`           // 不进卡（I-NOTE）
	Guidance         *NodeGuidance `yaml:"guidance"`
	CompleteWhen     []string      `yaml:"complete_when"` // 非空（FLOW-CW）；每项 ⊆ checks[].id
	CollectEvidence  []string      `yaml:"collect_evidence"`
	LeavesBehind     string        `yaml:"leaves_behind"` // 不进卡（I-NOTE）
	Transition       string        `yaml:"transition"`    // 不进卡（I-NOTE）
}

// NodeFreedom 是节点 freedom（自由文本两栏；不进卡）。
type NodeFreedom struct {
	Fixed []string `yaml:"fixed"`
	Open  []string `yaml:"open"`
}

// NodeGuidance 是节点级教学（introduction/teaching_points/hint_ladder/on_stuck）。
type NodeGuidance struct {
	Introduction   string   `yaml:"introduction"`
	TeachingPoints []string `yaml:"teaching_points"`
	HintLadder     []string `yaml:"hint_ladder"`
	OnStuck        string   `yaml:"on_stuck"`
}

// GuidanceBlock 是顶层 guidance（L0 可选，全自由文本；§4.10）。
type GuidanceBlock struct {
	Approach              string `yaml:"approach"`
	HelpPolicy            string `yaml:"help_policy"`
	ExplorationPolicy     string `yaml:"exploration_policy"`
	LearnerDecisionPolicy string `yaml:"learner_decision_policy"`
}

// EvidenceRecording 是 evidence_recording（L1 可选；§4.11）。
// 段缺省=编译期保守缺省注入（I-NOTE）；显式 false→W 归 W01-B（§5.2 未给规则号）。
type EvidenceRecording struct {
	RequestedGranularity       []string `yaml:"requested_granularity"`
	RecordInitiatorAndExecutor *bool    `yaml:"record_initiator_and_executor"`
	RecordHelpDisclosed        *bool    `yaml:"record_help_disclosed"`
	RecordVerifiedProgression  *bool    `yaml:"record_verified_progression"`
	PinSubmissionLogRange      *bool    `yaml:"pin_submission_log_range"`
	UnknownSource              *string  `yaml:"unknown_source"`
	Checkpoints                []string `yaml:"checkpoints"`
}

// Submission 是 submission（L1 可选；§4.12）。
type Submission struct {
	FreezeProjectAndEvidenceSnapshot *bool    `yaml:"freeze_project_and_evidence_snapshot"`
	TeacherReviewRequired            *bool    `yaml:"teacher_review_required"`
	ReviewByObjective                *bool    `yaml:"review_by_objective"`
	RetainPreviousSubmissions        *bool    `yaml:"retain_previous_submissions"`
	OnReturn                         *string  `yaml:"on_return"`
	Reports                          []string `yaml:"reports"`
	IncludeRawEvidenceIndex          *bool    `yaml:"include_raw_evidence_index"`
	SummaryClaimsRequireEvidenceRefs *bool    `yaml:"summary_claims_require_evidence_refs"`
}

// Completion 是 completion（L1 可选；§4.13）。
type Completion struct {
	RequiredDeliverables     []string `yaml:"required_deliverables"`
	RequiredChecks           []string `yaml:"required_checks"`             // ⊆ checks[].id（REF-COMP）
	LearningEvidenceToRecord []string `yaml:"learning_evidence_to_record"` // ⊆ checks[].id（REF-COMP）
	UserConfirmClosure       *bool    `yaml:"user_confirm_closure"`
	Export                   *string  `yaml:"export"`
}

// Budget 是 budget（L0 可选；§4.14）。两数值 null=运行时决定；编译目标要求 ≥1（R5）。
type Budget struct {
	MaxTurns    *int    `yaml:"max_turns"`
	MaxLLMCalls *int    `yaml:"max_llm_calls"`
	OnLimit     *string `yaml:"on_limit"`
}

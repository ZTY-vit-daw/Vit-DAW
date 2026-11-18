package legamedesign

// 编译期表接口与装配输入（契约 §6.2 T-14/T-17、§6.3 装配绑定、§4.4/T-13 材料准入）。
//
// 两张登记表是显式输入而非文本推断：判定面（T-14）与写边界（T-17）只能来自
// 人工登记表；设计源 request/criterion 文本不参与命令/判定推导（契约 §2 纪律 2、
// §6.3 第 1 条）。本卡给出表结构 + first-course（music-creation-unit2）登记条目；
// 运行期命令面接线归 W01-C（425）。

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"vit-daw-agent/internal/edu"
)

// ---- T-14 判定面登记表 ----

// 卡 v0 done_when 白名单结构谓词（与 edu.LevelStructureNames 同口径；本表是
// T-14 "白名单"的编译侧常量，扩值须先扩契约/校验门）。
const (
	StructureClipExists    = "clip_exists"
	StructureTrackNonEmpty = "track_non_empty"
	StructureTempoSet      = "tempo_set"
)

// StructurePredicates 是 T-14 deterministic 白名单结构谓词闭集。
var StructurePredicates = []string{StructureClipExists, StructureTrackNonEmpty, StructureTempoSet}

// JudgementEvents 是卡 event 叶合法事件名闭集（取自现行卡校验门，零漂移）。
var JudgementEvents = edu.LevelEventNames

// StructurePredicate 是一个已登记的 deterministic 判定面（T-14）：
// clip_exists 用 Clip（运行时轨槽名=track.id）、track_non_empty 用 Track。
type StructurePredicate struct {
	Structure string
	Clip      string
	Track     string
}

// JudgementRegistry 是 checks→卡判定面的显式登记表（T-14 白名单/事件映射表）。
// 未登记的 check 一律降级（W-CHK），不得从 criterion 文本生成判定。
type JudgementRegistry struct {
	// Deterministic：method=deterministic 的 check.id → 白名单结构叶。
	Deterministic map[string]StructurePredicate
	// UserJudgment：method=user_judgment 的 check.id → 卡 event 叶事件名。
	UserJudgment map[string]string
}

// FirstCourseJudgementRegistry 是 first-course（music-creation-unit2）登记条目。
// 依据=设计源 checks[].criterion 的语义人工登记（非文本解析）：
//   - deterministic 判定面：结构叶只能表达"槽有 clip/槽非空/工程 tempo 已设"；
//     c-reports（target=submission_snapshot）卡 v0 无快照判定面，登记为最弱可运行
//     结构叶（result-track 非空）并在映射表注释记录语义张力；
//   - user_judgment 判定面：按各 check 语境登记 user_confirmed / user_feedback_given /
//     user_chose（契约 T-14 允用事件闭集）。
func FirstCourseJudgementRegistry() JudgementRegistry {
	return JudgementRegistry{
		Deterministic: map[string]StructurePredicate{
			"c-rhythm-preview":    {Structure: StructureClipExists, Clip: "melody-track"},
			"c-q1-structure":      {Structure: StructureClipExists, Clip: "melody-track"},
			"c-q2-structure":      {Structure: StructureClipExists, Clip: "melody-track"},
			"c-q3-structure":      {Structure: StructureClipExists, Clip: "theme-track"},
			"c-generation-record": {Structure: StructureTrackNonEmpty, Track: "result-track"},
			"c-generated-pair":    {Structure: StructureTrackNonEmpty, Track: "result-track"},
			// c-reports：target=submission_snapshot，卡 v0 判定面无法表达快照；
			// 登记为 result-track 非空（成曲轨有内容=报表素材已产生的最弱结构证据）。
			"c-reports": {Structure: StructureTrackNonEmpty, Track: "result-track"},
		},
		UserJudgment: map[string]string{
			"c-intent":                  "user_confirmed",
			"c-listening-participation": "user_feedback_given",
			"c-humming":                 "user_feedback_given",
			"c-version-choice":          "user_chose",
		},
	}
}

// validate 校验登记表自身合法性并给出可审计诊断：
//   - 未登记到本设计源的 check id → W-MAP（登记表漂移，不静默忽略）
//   - 结构谓词闭集外 / 事件名闭集外 / clip/track 槽名未知 → E-MAP
//   - 登记了 method 不匹配的 check（如 deterministic 表里登记 rubric check）→ W-MAP
func (reg JudgementRegistry) validate(d *Design, r *Report) {
	methods := map[string]string{}
	for _, c := range d.Checks {
		methods[c.ID] = c.Method
	}
	trackIDs := map[string]bool{}
	if d.Workspace != nil {
		for _, t := range d.Workspace.Tracks {
			trackIDs[t.ID] = true
		}
	}
	for id, sp := range reg.Deterministic {
		if method, ok := methods[id]; !ok {
			r.warnf(RuleDefault, "checks", "T-14 判定面登记表含设计源不存在的 check id %q（登记表漂移）", id)
			continue
		} else if method != "deterministic" {
			r.warnf(RuleDefault, "checks", "T-14 登记表把 %q（method=%s）登记为 deterministic 结构叶，method 不匹配（登记表需修正）", id, method)
		}
		switch sp.Structure {
		case StructureClipExists:
			if strings.TrimSpace(sp.Clip) == "" || !trackIDs[sp.Clip] {
				r.errorf(RuleMapping, "checks", "T-14 登记表 %q: clip_exists.clip=%q 不是已知运行时轨槽（track.id）", id, sp.Clip)
			}
		case StructureTrackNonEmpty:
			if strings.TrimSpace(sp.Track) == "" || !trackIDs[sp.Track] {
				r.errorf(RuleMapping, "checks", "T-14 登记表 %q: track_non_empty.track=%q 不是已知运行时轨槽（track.id）", id, sp.Track)
			}
		case StructureTempoSet:
			// 无参数
		default:
			r.errorf(RuleMapping, "checks", "T-14 登记表 %q: 结构谓词 %q 不在白名单 %v", id, sp.Structure, StructurePredicates)
		}
	}
	for id, event := range reg.UserJudgment {
		if method, ok := methods[id]; !ok {
			r.warnf(RuleDefault, "checks", "T-14 判定面登记表含设计源不存在的 check id %q（登记表漂移）", id)
			continue
		} else if method != "user_judgment" {
			r.warnf(RuleDefault, "checks", "T-14 登记表把 %q（method=%s）登记为 user_judgment 事件叶，method 不匹配（登记表需修正）", id, method)
		}
		if !containsString(JudgementEvents, event) {
			r.errorf(RuleMapping, "checks", "T-14 登记表 %q: 事件 %q 不在卡事件闭集 %v", id, event, JudgementEvents)
		}
	}
	// 未登记的两类 check 会在使用点（T-14/T-15/T-16）出 W-CHK，这里不重复。
}

// ---- T-17 CAP-BRIDGE 闭包表 ----

// CapabilityBridge 是 CAP-BRIDGE 能力→引擎命令名闭包表（写边界 allow 的唯一来源）。
// required=true 的能力无登记条目 → E-CAP（编译拒绝，§6.3 第 1 条安全边界）。
type CapabilityBridge struct {
	Closures map[string][]string
}

// FirstCourseCapabilityBridge 是 first-course 登记条目：命令名取自仓库现行命令面
// （与 coord/levels/first-run-twinkle/level.yaml write_boundary 已核对集合一致）；
// 只读能力登记空闭包（不产生写命令）；生成/报表类命令面接线归 W01-C（425）。
func FirstCourseCapabilityBridge() CapabilityBridge {
	return CapabilityBridge{Closures: map[string][]string{
		"compose-ui": {
			"add_midi_notes", "add_midi_notes_bulk", "apply_midi_note_patch",
			"create_midi_clip", "delete_midi_notes", "import_midi_to_track",
			"mutate_midi_notes", "project.markers.upsert",
		},
		"play-reference":    {}, // 只读：加载/播放/预览，不写工程
		"edit-and-audition": {"add_midi_notes", "add_midi_notes_bulk", "apply_midi_note_patch", "delete_midi_notes", "mutate_midi_notes"},
		"tempo-and-meter":   {"set_tempo"},
		"compare-versions":  {}, // 只读：A/B 比较
		"record-check":      {}, // 只读：工程检查读取
		"yue2-finale":       {}, // 生成命令面接线归 425
		"build-reports":     {}, // 报表读取/落盘面归 425
	}}
}

// writeBoundary 由闭包表产生 allow 命令集（排序去重）+ 固定 confirm_downgrade=none。
func (b CapabilityBridge) writeBoundary(d *Design, r *Report) []string {
	seen := map[string]bool{}
	for i := range d.CapabilityRequests {
		c := &d.CapabilityRequests[i]
		closure, registered := b.Closures[c.ID]
		required := c.Required != nil && *c.Required
		if !registered {
			if required {
				r.errorf(RuleCap, fmt.Sprintf("capability_requests[%d]", i), "CAP-BRIDGE：required=true 但闭包表无登记条目（capability_request %q，T-17）→ 编译拒绝", c.ID)
			}
			continue
		}
		for _, cmd := range closure {
			if strings.TrimSpace(cmd) == "" {
				r.errorf(RuleCap, fmt.Sprintf("capability_requests[%d]", i), "CAP-BRIDGE 闭包表登记空命令名（capability_request %q）", c.ID)
				continue
			}
			seen[cmd] = true
		}
	}
	allow := make([]string, 0, len(seen))
	for cmd := range seen {
		allow = append(allow, cmd)
	}
	sort.Strings(allow)
	if len(allow) == 0 {
		r.errorf(RuleMapping, "write_boundary.allow", "闭包表未产生任何写命令，卡 v0 write_boundary.allow 不能为空（T-17）")
	}
	return allow
}

// ---- T-13 材料准入（catalog 核验状态 + 包内相对路径） ----

const catalogSchemaVersion = "vit.lesson_material_catalog.v0"

// AdmissibleCatalogStatuses 是 v0 材料准入口径：catalog 核验状态达标的闭集。
// 首包唯一已生成符号数据的终态=generated_symbolic_checked（judgement 输入=符号面，
// 不冒充音频听核）；transcribed_unverified 等状态一律排除并出 W-MAT（待备料清单）。
// 新增状态须先扩本表（装配表变更，随契约/装配卡评审）。
var AdmissibleCatalogStatuses = []string{"generated_symbolic_checked"}

// materialCatalog 是 assets/catalog.yaml 的最小读取面（材料库索引，TEXTBOOK-ALIGNMENT §3）。
// 只读取准入所需字段；其余字段（excerpt/concepts/...）不进编译域。
type materialCatalog struct {
	SchemaVersion string        `yaml:"schema_version"`
	Items         []catalogItem `yaml:"items"`
}

type catalogItem struct {
	ID         string `yaml:"id"`
	File       string `yaml:"file"`
	Status     string `yaml:"status"`
	SoundCheck string `yaml:"sound_check"`
}

// loadMaterialCatalog 读取包根下 catalog；文件缺失返回 (nil,nil)（无 catalog=全部
// 可播放材料无法确认核验状态→排除出卡并出 W，编译不因此失败）。
func loadMaterialCatalog(pkgRoot, rel string) (*materialCatalog, error) {
	abs := filepath.Join(pkgRoot, filepath.FromSlash(rel))
	raw, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cat materialCatalog
	if err := yaml.Unmarshal(raw, &cat); err != nil {
		return nil, fmt.Errorf("catalog yaml 解码: %w", err)
	}
	if cat.SchemaVersion != catalogSchemaVersion {
		return nil, fmt.Errorf("catalog schema_version %q != %q", cat.SchemaVersion, catalogSchemaVersion)
	}
	seen := map[string]bool{}
	for i := range cat.Items {
		norm := normalizeMaterialPath(cat.Items[i].File)
		if norm == "" {
			continue
		}
		if seen[norm] {
			return nil, fmt.Errorf("catalog 文件路径重复登记: %s", norm)
		}
		seen[norm] = true
		cat.Items[i].File = norm
	}
	return &cat, nil
}

func (c *materialCatalog) byFile(norm string) (catalogItem, bool) {
	if c == nil {
		return catalogItem{}, false
	}
	for _, item := range c.Items {
		if item.File == norm {
			return item, true
		}
	}
	return catalogItem{}, false
}

// normalizeMaterialPath 归一材料/卡资产相对路径（POSIX 斜杠 + Clean；不含绝对路径判定，
// 绝对路径/越界已由源校验 MAT-PATH 拒绝）。
func normalizeMaterialPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "./")
	return p
}

var playableMaterialKinds = []string{"midi", "midi_or_audio"}

type admittedPlan struct {
	index    int
	material *Material
	catalog  catalogItem
	fileRel  string
}

type excludedPlan struct {
	index    int
	material *Material
	reason   string
}

type admissionPlan struct {
	admitted []admittedPlan
	excluded []excludedPlan
}

// admittedMaterial 是物化后的准入材料（文件字节+sha256+notes 表）。
type admittedMaterial struct {
	index    int
	material *Material
	catalog  catalogItem
	fileRel  string
	sha256   string
	data     []byte
	notes    []cardNote
}

// planMaterialAdmission 是 T-13 准入判定（纯函数，零 IO）：
// file 非空 + kind ∈ {midi, midi_or_audio} + catalog 核验状态达标 → 准入；
// 其余→排除（原因进 W-MAT 待备料清单 / manifest 回执）。
func planMaterialAdmission(d *Design, catalog *materialCatalog) admissionPlan {
	plan := admissionPlan{}
	for i := range d.Materials {
		m := &d.Materials[i]
		norm := ""
		if m.File != nil {
			norm = normalizeMaterialPath(*m.File)
		}
		reason := ""
		switch {
		case m.File == nil:
			reason = "file:null 登记未备料（不阻塞编译，但不得编译为可播放资产，§4.5）"
		case !containsString(playableMaterialKinds, m.Kind):
			reason = fmt.Sprintf("kind=%s 非可播放材料（文本/图像/符号/材料组只进材料库加载面，不进卡 assets，§4.5）", m.Kind)
		case catalog == nil:
			reason = "catalog 缺失，无法确认核验状态（装配输入缺 catalog.yaml）"
		default:
			item, ok := catalog.byFile(norm)
			if !ok {
				reason = "catalog 未登记该文件（file 无核验状态）"
				break
			}
			if !containsString(AdmissibleCatalogStatuses, item.Status) {
				reason = fmt.Sprintf("catalog 核验状态 %q 未达标（达标=%v）", item.Status, AdmissibleCatalogStatuses)
				break
			}
			plan.admitted = append(plan.admitted, admittedPlan{index: i, material: m, catalog: item, fileRel: norm})
			continue
		}
		plan.excluded = append(plan.excluded, excludedPlan{index: i, material: m, reason: reason})
	}
	return plan
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

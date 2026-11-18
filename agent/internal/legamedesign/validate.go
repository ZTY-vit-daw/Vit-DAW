package legamedesign

import (
	"fmt"
	"math"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// windowsDrivePathPattern 识别 Windows 盘符绝对路径（MAT-PATH：禁止绝对路径）。
var windowsDrivePathPattern = regexp.MustCompile(`^[A-Za-z]:`)

// validator 持有一次校验的上下文（id 索引 + REG-METER 基准）。
type validator struct {
	d    *Design
	opts Options
	r    *Report

	objectiveIDs  map[string]bool
	materialIDs   map[string]bool
	checkIDs      map[string]bool
	trackIDs      map[string]bool
	regionIDs     map[string]bool
	capabilityIDs map[string]bool
	nodeIDs       map[string]bool

	// workspaceTimeSignature 是已通过 N/D 格式校验的工程拍号（REG-METER 比对基准）。
	workspaceTimeSignature *string
}

// validateDesign 是 Parse/Validate 共用的校验入口（契约 §5.2 规则逐条落地）。
func validateDesign(d *Design, opts Options) *Report {
	r := newReport()
	if d == nil {
		r.errorf(RuleSchema, "", "设计源为空（nil）")
		return r
	}
	switch d.SchemaVersion {
	case SchemaVersionLegameDesign:
	case SchemaVersionProjectIntent:
		r.infof(RuleNote, "schema_version", "预留值 %q 仅登记（R1）：非乐局解析路径（M2+）未实现，本包不做 L1 校验、不按乐局报错（R2：启用判定在项目包元数据，归 W01-C/425）", d.SchemaVersion)
		return r
	default:
		r.errorf(RuleSchema, "schema_version", "schema_version 缺失或未知：%q（唯一认知值 %q；预留 %q 仅登记；未知版本 fail-closed 拒绝，R1/R2）", d.SchemaVersion, SchemaVersionLegameDesign, SchemaVersionProjectIntent)
		return r
	}

	v := &validator{
		d:             d,
		opts:          opts,
		r:             r,
		objectiveIDs:  map[string]bool{},
		materialIDs:   map[string]bool{},
		checkIDs:      map[string]bool{},
		trackIDs:      map[string]bool{},
		regionIDs:     map[string]bool{},
		capabilityIDs: map[string]bool{},
		nodeIDs:       map[string]bool{},
	}
	v.validateL0()
	v.collectIDs()
	v.requireL1Sections()
	v.validateObjectives()
	v.validateMaterials()
	v.validateWorkspace()
	v.validateCapabilities()
	v.validateChecks()
	v.validateFlow()
	v.validateEvidenceRecording()
	v.validateSubmission()
	v.validateCompletion()
	v.validateBudget()
	return r
}

// ---- 小工具 ----

func enumContains(allowed []string, value string) bool { return slices.Contains(allowed, value) }

func enumText(allowed []string) string { return strings.Join(allowed, "|") }

// validateRequiredEnum 校验必填枚举：缺失/空白→E-REQ；非空但闭集外→E-SCHEMA。
func validateRequiredEnum(r *Report, rulePath, field, value string, allowed []string) {
	if strings.TrimSpace(value) == "" {
		r.errorf(RuleRequired, rulePath, "%s 必填（v0 闭集: %s）", field, enumText(allowed))
		return
	}
	if !enumContains(allowed, value) {
		r.errorf(RuleSchema, rulePath, "%s 枚举外取值 %q（v0 闭集: %s；扩值须先扩契约）", field, value, enumText(allowed))
	}
}

// validateOptionalEnum 校验可选枚举：nil=缺省（不提示）；非 nil 但闭集外→E-SCHEMA。
func validateOptionalEnum(r *Report, rulePath, field string, value *string, allowed []string) {
	if value == nil {
		return
	}
	if !enumContains(allowed, *value) {
		r.errorf(RuleSchema, rulePath, "%s 枚举外取值 %q（v0 闭集: %s）", field, *value, enumText(allowed))
	}
}

// validateListEnum 校验列表项枚举（枚举外→E-SCHEMA）。
func validateListEnum(r *Report, rulePath, field string, items []string, allowed []string) {
	for k, item := range items {
		if !enumContains(allowed, item) {
			r.errorf(RuleSchema, fmt.Sprintf("%s[%d]", rulePath, k), "%s 枚举外取值 %q（v0 闭集: %s）", field, item, enumText(allowed))
		}
	}
}

// requireText 校验必填非空字符串（E-REQ）。
func requireText(r *Report, rulePath, field, value string) {
	if strings.TrimSpace(value) == "" {
		r.errorf(RuleRequired, rulePath, "%s 必填且非空", field)
	}
}

// validateTimeSignature 校验 "N/D" 形态（N,D 正整数；契约 §4.6）；返回空串=合法。
func validateTimeSignature(sig string) string {
	parts := strings.Split(sig, "/")
	if len(parts) != 2 {
		return fmt.Sprintf("time_signature %q 须为 N/D 形态（N,D 正整数）", sig)
	}
	n, errN := strconv.Atoi(parts[0])
	d, errD := strconv.Atoi(parts[1])
	if errN != nil || errD != nil || n < 1 || d < 1 {
		return fmt.Sprintf("time_signature %q 非法规格：N,D 须为正整数", sig)
	}
	return ""
}

func isValidTempo(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

// collectIDs 收集各段 id 并就地校验空 id（E-REQ）与段内重复（E-REF）。
func (v *validator) collectIDs() {
	d := v.d
	v.objectiveIDs = collectSectionIDs(v.r, "objectives", len(d.Objectives), func(i int) string { return d.Objectives[i].ID })
	v.materialIDs = collectSectionIDs(v.r, "materials", len(d.Materials), func(i int) string { return d.Materials[i].ID })
	v.checkIDs = collectSectionIDs(v.r, "checks", len(d.Checks), func(i int) string { return d.Checks[i].ID })
	v.capabilityIDs = collectSectionIDs(v.r, "capability_requests", len(d.CapabilityRequests), func(i int) string { return d.CapabilityRequests[i].ID })
	if d.Workspace != nil {
		v.trackIDs = collectSectionIDs(v.r, "workspace.tracks", len(d.Workspace.Tracks), func(i int) string { return d.Workspace.Tracks[i].ID })
		v.regionIDs = collectSectionIDs(v.r, "workspace.regions", len(d.Workspace.Regions), func(i int) string { return d.Workspace.Regions[i].ID })
	} else {
		v.trackIDs = map[string]bool{}
		v.regionIDs = map[string]bool{}
	}
	if d.Flow != nil {
		v.nodeIDs = collectSectionIDs(v.r, "flow.nodes", len(d.Flow.Nodes), func(i int) string { return d.Flow.Nodes[i].ID })
	} else {
		v.nodeIDs = map[string]bool{}
	}
}

// collectSectionIDs 收集一段的 id：空 id→E-REQ；段内重复→E-REF（§5.2 各列表 id 唯一）。
func collectSectionIDs(r *Report, section string, n int, idAt func(int) string) map[string]bool {
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		id := idAt(i)
		rulePath := fmt.Sprintf("%s[%d].id", section, i)
		if strings.TrimSpace(id) == "" {
			r.errorf(RuleRequired, rulePath, "必填 id 为空")
			continue
		}
		if seen[id] {
			r.errorf(RuleRef, rulePath, "id 重复: %q（各列表 id 须唯一，§5.2 E-REF）", id)
			continue
		}
		seen[id] = true
	}
	return seen
}

// requireL1Sections 校验 L1 五个硬必填段（契约 §3.1；E-REQ）。
func (v *validator) requireL1Sections() {
	d := v.d
	if len(d.Objectives) == 0 {
		v.r.errorf(RuleRequired, "objectives", "L1 必填段缺失或为空列表（§3.1 五个硬必填段之一）")
	}
	if len(d.Materials) == 0 {
		v.r.errorf(RuleRequired, "materials", "L1 必填段缺失或为空列表（§3.1 五个硬必填段之一）")
	}
	if len(d.Checks) == 0 {
		v.r.errorf(RuleRequired, "checks", "L1 必填段缺失或为空列表（§3.1 五个硬必填段之一）")
	}
	if d.Flow == nil {
		v.r.errorf(RuleRequired, "flow", "L1 必填段缺失（§3.1 五个硬必填段之一）")
	} else if len(d.Flow.Nodes) == 0 {
		v.r.errorf(RuleRequired, "flow.nodes", "flow.nodes 不能为空（L1 必填段 flow 非空，§4.9）")
	}
	if d.Workspace == nil {
		v.r.errorf(RuleRequired, "workspace.regions", "L1 必填段缺失或为空列表（workspace.regions，§3.1）")
	} else if len(d.Workspace.Regions) == 0 {
		v.r.errorf(RuleRequired, "workspace.regions", "L1 必填段缺失或为空列表（workspace.regions，§3.1）")
	}
}

// validateL0 校验 L0 意图层（除 schema_version 外全可选；§4.1/§4.2）。
func (v *validator) validateL0() {
	d := v.d
	if d.Revision != nil && *d.Revision < 1 {
		v.r.errorf(RuleSchema, "revision", "revision 须 ≥1（got %d；缺省 0=未修订跟踪，§4.1）", *d.Revision)
	}
	if d.Brief != nil && d.Brief.EstimatedMinutes == nil {
		v.r.infof(RuleNote, "brief.estimated_minutes", "estimated_minutes=null：不以课时限制（仅提示；不阻塞编译，§4.2）")
	}
	if len(d.PendingQuestions) > 0 {
		v.r.infof(RuleNote, "pending_questions", "pending_questions 共 %d 条：编译回执原样罗列（I；T-21）", len(d.PendingQuestions))
	}
}

// checkRefs 校验引用列表（每项须在 known 集合内；空项/悬空→E-REF）。
func (v *validator) checkRefs(rulePath, ruleName string, refs []string, known map[string]bool) {
	for k, ref := range refs {
		if strings.TrimSpace(ref) == "" {
			v.r.errorf(RuleRef, fmt.Sprintf("%s[%d]", rulePath, k), "%s 引用为空 id", ruleName)
			continue
		}
		if !known[ref] {
			v.r.errorf(RuleRef, fmt.Sprintf("%s[%d]", rulePath, k), "%s 引用不存在的 id %q", ruleName, ref)
		}
	}
}

// ---- objectives / materials ----

func (v *validator) validateObjectives() {
	for i := range v.d.Objectives {
		o := &v.d.Objectives[i]
		base := fmt.Sprintf("objectives[%d]", i)
		validateRequiredEnum(v.r, base+".kind", "kind", o.Kind, ObjectiveKinds)
		requireText(v.r, base+".description", "description", o.Description)
		v.checkRefs(base+".evidence_checks", "REF-OBJ", o.EvidenceChecks, v.checkIDs)
	}
}

func (v *validator) validateMaterials() {
	for i := range v.d.Materials {
		m := &v.d.Materials[i]
		base := fmt.Sprintf("materials[%d]", i)
		validateRequiredEnum(v.r, base+".kind", "kind", m.Kind, MaterialKinds)
		requireText(v.r, base+".purpose", "purpose", m.Purpose)
		requireText(v.r, base+".source", "source", m.Source)
		if m.Source != "" && (strings.Contains(m.Source, "未听核") || strings.Contains(m.Source, "待转写")) {
			v.r.warnf(RuleMatWarn, base+".source", "source 声明含 未听核/待转写 字样：内容面待核（catalog 核验状态决定资产准入，§4.4/§6.3）")
		}
		switch {
		case m.File == nil:
			v.r.warnf(RuleMatWarn, base+".file", "file:null=登记未备料（合法；不阻塞编译，但不得编译为可播放资产，§4.5）")
		default:
			if msg := validateMaterialFilePath(*m.File); msg != "" {
				v.r.errorf(RuleMat, base+".file", "%s", msg)
			}
		}
	}
}

// validateMaterialFilePath 校验材料相对路径（MAT-PATH：绝对路径/含 ".." 段/反斜杠→E）。
// 存在性与哈希对账（MAT-FILE）归装配/编译期（§4.4/§6.3），本包零 IO 不查。
func validateMaterialFilePath(file string) string {
	if file == "" {
		return "MAT-PATH：file 为空字符串（未备料须写 null；空串不是合法相对路径）"
	}
	if strings.Contains(file, "\\") {
		return "MAT-PATH：须用 POSIX 斜杠 '/',不得含反斜杠（包内相对路径，§4.4）"
	}
	if windowsDrivePathPattern.MatchString(file) || path.IsAbs(file) || strings.HasPrefix(file, "//") {
		return fmt.Sprintf("MAT-PATH：禁止绝对路径 %q（须相对设计源 YAML 所在目录）", file)
	}
	for _, seg := range strings.Split(file, "/") {
		if seg == ".." {
			return fmt.Sprintf("MAT-PATH：路径 %q 含 '..' 段（禁止越出包根，fail-closed）", file)
		}
	}
	return ""
}

// ---- workspace（timing / tracks / regions）----

func (v *validator) validateWorkspace() {
	ws := v.d.Workspace
	if ws == nil {
		return
	}
	v.validateTiming(ws)
	v.validateTracks(ws)
	v.validateRegions(ws)
}

func (v *validator) validateTiming(ws *Workspace) {
	t := ws.Timing
	if t == nil {
		return
	}
	if t.TimeSignature != nil {
		if msg := validateTimeSignature(*t.TimeSignature); msg != "" {
			v.r.errorf(RuleSchema, "workspace.timing.time_signature", "%s", msg)
		} else {
			v.workspaceTimeSignature = t.TimeSignature
		}
	}
	if t.KeySignature != nil {
		if !IsValidKeySignature(*t.KeySignature) {
			v.r.errorf(RuleSchema, "workspace.timing.key_signature", "key_signature %q 不在 R6a=C 的 30 调封闭枚举（15 大调+15 关系小调；token=大写根音+可选 #/b+小调后缀 m）", *t.KeySignature)
		} else {
			v.r.infof(RuleNote, "workspace.timing.key_signature", "设计层字段 %q 不迁移进卡（T-10）；消费方=记谱拼写+YuE2 ABC K:", *t.KeySignature)
		}
	}
	if t.TempoBPM != nil {
		if !isValidTempo(*t.TempoBPM) {
			v.r.errorf(RuleSchema, "workspace.timing.tempo_bpm", "tempo_bpm 须为 >0 的有限数（got %v）；null=未定（R5：编译 E+显式覆盖参数）", *t.TempoBPM)
		}
	}
	if t.PickupBeats != nil {
		if math.IsNaN(*t.PickupBeats) || math.IsInf(*t.PickupBeats, 0) || *t.PickupBeats < 0 {
			v.r.errorf(RuleSchema, "workspace.timing.pickup_beats", "pickup_beats 须 ≥0 的有限数（got %v）", *t.PickupBeats)
		}
	}
}

func (v *validator) validateTracks(ws *Workspace) {
	editableDeclared := 0
	for i := range ws.Tracks {
		t := &ws.Tracks[i]
		base := fmt.Sprintf("workspace.tracks[%d]", i)
		requireText(v.r, base+".name", "name", t.Name)
		validateRequiredEnum(v.r, base+".role", "role", t.Role, TrackRoles)
		if t.InitialMaterial != nil {
			switch {
			case *t.InitialMaterial == "":
				v.r.errorf(RuleRequired, base+".initial_material", "initial_material 为空字符串（无预置须写 null）")
			case !v.materialIDs[*t.InitialMaterial]:
				v.r.errorf(RuleRef, base+".initial_material", "REF-MAT 引用不存在的 material id %q", *t.InitialMaterial)
			}
		}
		if t.EditableByUser != nil {
			editableDeclared++
		}
	}
	if editableDeclared > 0 {
		v.r.infof(RuleNote, "workspace.tracks[*].editable_by_user", "设计层字段（共 %d 处声明）不迁移进卡：前端/交互层消费（I-NOTE）", editableDeclared)
	}
}

func (v *validator) validateRegions(ws *Workspace) {
	aliases := map[string]string{} // clip alias → 首次声明路径（REG-ALIAS 全局唯一）
	for i := range ws.Regions {
		rg := &ws.Regions[i]
		base := fmt.Sprintf("workspace.regions[%d]", i)
		if strings.TrimSpace(rg.Track) == "" {
			v.r.errorf(RuleRequired, base+".track", "track 必填（⊆ tracks[].id）")
		} else if !v.trackIDs[rg.Track] {
			v.r.errorf(RuleRef, base+".track", "REF-REGION 引用不存在的 track id %q", rg.Track)
		}
		requireText(v.r, base+".purpose", "purpose", rg.Purpose)
		if rg.Locator == nil {
			v.r.errorf(RuleRequired, base+".locator", "locator 必填（联合形态闭集：scope 型或 clip_alias 型，§4.6）")
			continue
		}
		loc := rg.Locator
		lb := base + ".locator"
		hasScope := loc.Scope != nil
		hasClipAlias := loc.ClipAlias != nil
		switch {
		case hasScope && hasClipAlias:
			v.r.errorf(RuleRequired, lb, "REG-LOC：scope 与 clip_alias 混写（须恰一种寻址语义，§4.6）")
		case !hasScope && !hasClipAlias:
			v.r.errorf(RuleRequired, lb, "REG-LOC：缺寻址形态（须 scope 型或 clip_alias 型；clip_aliases 不能单独作寻址）")
		}
		if hasScope {
			if !enumContains(LocatorScopes, *loc.Scope) {
				v.r.errorf(RuleSchema, lb+".scope", "scope 枚举外取值 %q（v0 闭集: %s）", *loc.Scope, enumText(LocatorScopes))
			}
		}
		switch {
		case hasScope && !hasClipAlias:
			// scope 型（可选 clip_aliases；bars 不适用；bars_per_clip 仅 four_clips）
			if loc.Bars != nil {
				v.r.errorf(RuleRequired, lb+".bars", "REG-LOC：bars 仅 clip 型 locator 可用（scope 型不得声明）")
			}
			if loc.BarsPerClip != nil {
				if loc.Scope == nil || *loc.Scope != "four_clips" {
					v.r.errorf(RuleRequired, lb+".bars_per_clip", "REG-LOC：bars_per_clip 仅 scope=four_clips 可用")
				} else if *loc.BarsPerClip < 1 {
					v.r.errorf(RuleRequired, lb+".bars_per_clip", "REG-LOC：bars_per_clip 须 ≥1（got %d）", *loc.BarsPerClip)
				}
			}
		case hasClipAlias && !hasScope:
			// clip 型（可选 time_signature/bars；不接受 clip_aliases/bars_per_clip）
			if len(loc.ClipAliases) > 0 {
				v.r.errorf(RuleRequired, lb, "REG-LOC：clip_alias 与 clip_aliases 混写（多 clip 型须 scope=four_clips）")
			}
			if loc.BarsPerClip != nil {
				v.r.errorf(RuleRequired, lb+".bars_per_clip", "REG-LOC：bars_per_clip 仅 scope=four_clips 可用")
			}
			if *loc.ClipAlias == "" {
				v.r.errorf(RuleRequired, lb+".clip_alias", "REG-ALIAS：clip_alias 为空字符串")
			}
			if loc.Bars != nil && *loc.Bars < 1 {
				v.r.errorf(RuleRequired, lb+".bars", "REG-LOC：bars 须 ≥1（got %d）", *loc.Bars)
			}
		}
		// clip alias 全局唯一（REG-ALIAS；H3 作业身份，跨区域重复→E）
		if hasClipAlias && *loc.ClipAlias != "" {
			v.recordClipAlias(aliases, *loc.ClipAlias, lb+".clip_alias")
		}
		for k, alias := range loc.ClipAliases {
			itemPath := fmt.Sprintf("%s.clip_aliases[%d]", lb, k)
			if alias == "" {
				v.r.errorf(RuleRequired, itemPath, "REG-ALIAS：clip alias 为空字符串")
				continue
			}
			v.recordClipAlias(aliases, alias, itemPath)
		}
		// REG-METER：region 级 time_signature 须等于 workspace.timing.time_signature
		if loc.TimeSignature != nil {
			if msg := validateTimeSignature(*loc.TimeSignature); msg != "" {
				v.r.errorf(RuleSchema, lb+".time_signature", "%s", msg)
			} else if v.workspaceTimeSignature == nil {
				v.r.errorf(RuleRequired, lb+".time_signature", "REG-METER：region 声明 time_signature 但 workspace.timing.time_signature 缺失/非法（region 不得覆盖工程计量，§4.6）")
			} else if *loc.TimeSignature != *v.workspaceTimeSignature {
				v.r.errorf(RuleRequired, lb+".time_signature", "REG-METER：%q ≠ workspace.timing.time_signature %q（region 不得覆盖工程拍号）", *loc.TimeSignature, *v.workspaceTimeSignature)
			}
		}
	}
}

func (v *validator) recordClipAlias(seen map[string]string, alias, rulePath string) {
	if first, ok := seen[alias]; ok {
		v.r.errorf(RuleRequired, rulePath, "REG-ALIAS：clip alias %q 跨区域重复（首次声明于 %s；clip_alias 是作业身份，须全局唯一，§6.3）", alias, first)
		return
	}
	seen[alias] = rulePath
}

// ---- capability_requests / checks ----

func (v *validator) validateCapabilities() {
	for i := range v.d.CapabilityRequests {
		c := &v.d.CapabilityRequests[i]
		base := fmt.Sprintf("capability_requests[%d]", i)
		requireText(v.r, base+".request", "request", c.Request)
		if c.Required == nil {
			v.r.errorf(RuleRequired, base+".required", "required 必填（bool；缺省不得视为 false）")
			continue
		}
		if v.opts.CapabilityClosure == nil {
			continue // stub 未接线：CAP-BRIDGE 闭包表由 W01-C 登记（本卡 E-CAP 表接口）
		}
		if *c.Required && strings.TrimSpace(c.ID) != "" && !v.opts.CapabilityClosure.HasCommandClosure(c.ID) {
			v.r.errorf(RuleCap, base, "CAP-BRIDGE：required=true 但闭包表无登记条目（capability_request %q；写边界来源=登记闭包表，禁止 request 文本猜测，§6.3）→ 编译拒绝", c.ID)
		}
	}
}

func (v *validator) validateChecks() {
	for i := range v.d.Checks {
		c := &v.d.Checks[i]
		base := fmt.Sprintf("checks[%d]", i)
		validateRequiredEnum(v.r, base+".purpose", "purpose", c.Purpose, CheckPurposes)
		validateRequiredEnum(v.r, base+".method", "method", c.Method, CheckMethods)
		validateRequiredEnum(v.r, base+".on_unavailable", "on_unavailable", c.OnUnavailable, CheckOnUnavailable)
		if strings.TrimSpace(c.Target) == "" {
			v.r.errorf(RuleRequired, base+".target", "target 必填（region id 或唯一非 region 目标 submission_snapshot）")
		} else if c.Target != "submission_snapshot" && !v.regionIDs[c.Target] {
			v.r.errorf(RuleRef, base+".target", "REF-CHK 引用未知 region %q（合法目标=regions[].id ∪ {submission_snapshot}）", c.Target)
		}
		requireText(v.r, base+".criterion", "criterion", c.Criterion)
		requireText(v.r, base+".evidence", "evidence", c.Evidence)
		if v.opts.CheckMapping == nil {
			continue // stub 未接线：判定面映射表由 W01-B 登记（本卡 W-CHK 表接口）
		}
		if strings.TrimSpace(c.ID) != "" && !v.opts.CheckMapping.MapsToCardJudgement(c.ID) {
			v.r.warnf(RuleCheckWarn, base, "check %q 无法映射到卡判定面（T-14 白名单/事件映射表未登记）→ 编译期降 advisory_checks；禁止从 criterion 文本生成判定（§2 纪律 2）", c.ID)
		}
	}
}

// ---- flow ----

func (v *validator) validateFlow() {
	f := v.d.Flow
	if f == nil {
		return
	}
	if strings.TrimSpace(f.Progression) == "" {
		v.r.errorf(RuleRequired, "flow.progression", "progression 必填（v0 闭集: %s）", enumText(FlowProgressions))
	} else if !enumContains(FlowProgressions, f.Progression) {
		v.r.errorf(RuleSchema, "flow.progression", "progression 枚举外取值 %q（v0 仅 ordered；扩值须扩门）", f.Progression)
	}
	usesMaterials, usesRegions, usesCapabilities := 0, 0, 0
	humanDecides, freedom, leavesBehind, transition := 0, 0, 0, 0
	for j := range f.Nodes {
		n := &f.Nodes[j]
		base := fmt.Sprintf("flow.nodes[%d]", j)
		requireText(v.r, base+".title", "title", n.Title)
		v.checkRefs(base+".objective_refs", "REF-NODE-OBJ", n.ObjectiveRefs, v.objectiveIDs)
		requireText(v.r, base+".task", "task", n.Task)
		validateRequiredEnum(v.r, base+".lead", "lead", n.Lead, NodeLeads)
		v.checkRefs(base+".uses_materials", "REF-NODE-MAT", n.UsesMaterials, v.materialIDs)
		v.checkRefs(base+".uses_regions", "REF-NODE-REG", n.UsesRegions, v.regionIDs)
		v.checkRefs(base+".uses_capabilities", "REF-NODE-CAP", n.UsesCapabilities, v.capabilityIDs)
		if len(n.CompleteWhen) == 0 {
			v.r.errorf(RuleRequired, base+".complete_when", "FLOW-CW：complete_when 必填且非空（节点无完成条件不可运行，§4.9）")
		} else {
			v.checkRefs(base+".complete_when", "REF-NODE-CHK", n.CompleteWhen, v.checkIDs)
			if v.opts.CheckMapping != nil && !anyCheckMapped(v.opts.CheckMapping, n.CompleteWhen) {
				v.r.warnf(RuleCheckWarn, base+".complete_when", "节点完成条件全部无法映射到卡判定面（编译期无 done_when 叶；W01-B 按 E-FLOW 处理）")
			}
		}
		v.checkRefs(base+".collect_evidence", "REF-NODE-CHK", n.CollectEvidence, v.checkIDs)

		usesMaterials += len(n.UsesMaterials)
		usesRegions += len(n.UsesRegions)
		usesCapabilities += len(n.UsesCapabilities)
		if len(n.HumanDecides) > 0 {
			humanDecides++
		}
		if n.Freedom != nil {
			freedom++
		}
		if strings.TrimSpace(n.LeavesBehind) != "" {
			leavesBehind++
		}
		if strings.TrimSpace(n.Transition) != "" {
			transition++
		}
	}
	if usesMaterials > 0 {
		v.r.infof(RuleNote, "flow.nodes[*].uses_materials", "设计层引用（共 %d 项）不进卡：编译进每轮注入语境（W02/H2 域；T-16 I）", usesMaterials)
	}
	if usesRegions > 0 {
		v.r.infof(RuleNote, "flow.nodes[*].uses_regions", "设计层引用（共 %d 项）不进卡：编译进每轮注入语境（W02/H2 域；T-16 I）", usesRegions)
	}
	if usesCapabilities > 0 {
		v.r.infof(RuleNote, "flow.nodes[*].uses_capabilities", "设计层引用（共 %d 项）不进卡：编译进每轮注入语境（W02/H2 域；T-16 I）", usesCapabilities)
	}
	if humanDecides > 0 {
		v.r.infof(RuleNote, "flow.nodes[*].human_decides", "设计层字段（共 %d 个节点声明）不进卡：每轮注入语境（W02/H2 域；T-16 I）", humanDecides)
	}
	if freedom > 0 {
		v.r.infof(RuleNote, "flow.nodes[*].freedom", "设计层字段（共 %d 个节点声明）不进卡：注入语境（W02；T-16 I）", freedom)
	}
	if leavesBehind > 0 {
		v.r.infof(RuleNote, "flow.nodes[*].leaves_behind", "设计层字段（共 %d 个节点声明）不进卡：回执摘要（T-16 I）", leavesBehind)
	}
	if transition > 0 {
		v.r.infof(RuleNote, "flow.nodes[*].transition", "设计层字段（共 %d 个节点声明）不迁移为卡 then（语义不同；T-16 I）", transition)
	}
}

func anyCheckMapped(m CheckJudgementMapping, checkIDs []string) bool {
	for _, id := range checkIDs {
		if m.MapsToCardJudgement(id) {
			return true
		}
	}
	return false
}

// ---- L1 可选段 ----

func (v *validator) validateEvidenceRecording() {
	er := v.d.EvidenceRecording
	if er == nil {
		v.r.infof(RuleNote, "evidence_recording", "段未声明：编译期按 §4.11 保守缺省表注入并出 I（§3.1/§6.4 缺省注入逐项出 I）")
		return
	}
	validateListEnum(v.r, "evidence_recording.requested_granularity", "requested_granularity", er.RequestedGranularity, EvidenceGranularities)
	validateOptionalEnum(v.r, "evidence_recording.unknown_source", "unknown_source", er.UnknownSource, EvidenceUnknownSources)
	validateListEnum(v.r, "evidence_recording.checkpoints", "checkpoints", er.Checkpoints, EvidenceCheckpoints)
}

func (v *validator) validateSubmission() {
	s := v.d.Submission
	if s == nil {
		v.r.infof(RuleNote, "submission", "段未声明：编译期按 §4.12 缺省表注入并出 I（§3.1/§6.4 缺省注入逐项出 I）")
		return
	}
	validateOptionalEnum(v.r, "submission.on_return", "on_return", s.OnReturn, SubmissionOnReturns)
	validateListEnum(v.r, "submission.reports", "reports", s.Reports, SubmissionReports)
}

func (v *validator) validateCompletion() {
	c := v.d.Completion
	if c == nil {
		v.r.infof(RuleNote, "completion", "段未声明：编译期按 §4.13 缺省表注入并出 I（§3.1/§6.4 缺省注入逐项出 I）")
		return
	}
	v.checkRefs("completion.required_checks", "REF-COMP", c.RequiredChecks, v.checkIDs)
	v.checkRefs("completion.learning_evidence_to_record", "REF-COMP", c.LearningEvidenceToRecord, v.checkIDs)
	validateOptionalEnum(v.r, "completion.export", "export", c.Export, CompletionExports)
}

func (v *validator) validateBudget() {
	b := v.d.Budget
	if b == nil {
		return
	}
	if b.MaxTurns != nil && *b.MaxTurns < 1 {
		v.r.errorf(RuleSchema, "budget.max_turns", "max_turns 须 ≥1（got %d；null=运行时决定，R5 编译 E+显式覆盖参数）", *b.MaxTurns)
	}
	if b.MaxLLMCalls != nil && *b.MaxLLMCalls < 1 {
		v.r.errorf(RuleSchema, "budget.max_llm_calls", "max_llm_calls 须 ≥1（got %d；null=运行时决定，R5 编译 E+显式覆盖参数）", *b.MaxLLMCalls)
	}
	validateOptionalEnum(v.r, "budget.on_limit", "on_limit", b.OnLimit, BudgetOnLimits)
}

// ==== R5：tempo/budget 编译就绪判定（fail-closed，不注入缺省）====

// CompileOverrides 是编译期显式覆盖参数（R5=A 的"或显式覆盖参数"落点）。
// nil 表示未提供该覆盖；编译入口须把命令行/用户问清后的值传进来。
type CompileOverrides struct {
	TempoBPM    *float64
	MaxTurns    *int
	MaxLLMCalls *int
}

// ValidateCompileReadiness 执行 R5 编译就绪判定（契约 §5.2 W-BUDGET / §6.4 缺项表）：
//   - tempo_bpm 为 null/未声明且无显式覆盖参数 → E（不注入缺省，诊断可行动）；
//   - budget.max_turns / max_llm_calls 为 null/未声明且无显式覆盖参数 → E（同上）；
//   - 提供了覆盖参数时补写该值并出 I（"使用显式覆盖参数"），不产生缺省注入 W。
//
// 该入口与源校验（Validate）分离：设计期允许显式 null（§4.6/§4.14），
// 编译前必须调用本函数（W01-B 编译入口），未通过即拒绝编译。
func ValidateCompileReadiness(d *Design, ov CompileOverrides) *Report {
	r := newReport()
	if d == nil {
		r.errorf(RuleSchema, "", "设计源为空（nil）")
		return r
	}
	if d.SchemaVersion != SchemaVersionLegameDesign {
		r.errorf(RuleSchema, "schema_version", "schema_version %q 不是乐局设计源（%q），不参与乐局编译（R2：启用判定在项目包元数据）", d.SchemaVersion, SchemaVersionLegameDesign)
		return r
	}

	var srcTempo *float64
	if d.Workspace != nil && d.Workspace.Timing != nil {
		srcTempo = d.Workspace.Timing.TempoBPM
	}
	switch {
	case ov.TempoBPM != nil:
		if !isValidTempo(*ov.TempoBPM) {
			r.errorf(RuleSchema, "workspace.timing.tempo_bpm", "覆盖参数 tempo 须为 >0 的有限数（got %v）", *ov.TempoBPM)
		} else if srcTempo == nil {
			r.infof(RuleNote, "workspace.timing.tempo_bpm", "R5：设计源未声明 tempo_bpm，使用显式覆盖参数 %v（不注入缺省）", *ov.TempoBPM)
		}
	case srcTempo == nil:
		r.errorf(RuleBudget, "workspace.timing.tempo_bpm", "R5=A fail-closed：tempo_bpm 缺失（null/未声明）且无显式覆盖参数；不注入缺省——请补写设计源或显式传入 tempo 覆盖参数（用户/LLM 问清后填）")
	}

	var srcTurns, srcCalls *int
	if d.Budget != nil {
		srcTurns = d.Budget.MaxTurns
		srcCalls = d.Budget.MaxLLMCalls
	}
	checkR5Int(r, "budget.max_turns", srcTurns, ov.MaxTurns, "轮次预算")
	checkR5Int(r, "budget.max_llm_calls", srcCalls, ov.MaxLLMCalls, "LLM 调用预算")
	return r
}

func checkR5Int(r *Report, rulePath string, src, ov *int, label string) {
	switch {
	case ov != nil:
		if *ov < 1 {
			r.errorf(RuleSchema, rulePath, "覆盖参数 %s 须 ≥1（got %d）", label, *ov)
		} else if src == nil {
			r.infof(RuleNote, rulePath, "R5：设计源未声明 %s，使用显式覆盖参数 %d（不注入缺省）", label, *ov)
		}
	case src == nil:
		r.errorf(RuleBudget, rulePath, "R5=A fail-closed：%s 缺失（null/未声明）且无显式覆盖参数；不注入缺省——请补写设计源或显式传入覆盖参数（用户/LLM 问清后填）", label)
	}
}

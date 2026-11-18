package legamedesign

import (
	"fmt"
	"sort"
	"strings"
)

// Severity 是诊断分级（契约 §5.1）：E 阻断 / W 降级待核 / I 提示。
type Severity int

const (
	// SeverityError 是阻断级（校验/编译失败，退出码非 0）。
	SeverityError Severity = iota
	// SeverityWarning 是可编译但发生声明性降级（进入回执与编译 manifest）。
	SeverityWarning
	// SeverityInfo 是信息性提示（回执附录）。
	SeverityInfo
)

// String 返回单字母分级标记（E/W/I，与契约 §5.1 口径一致）。
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "E"
	case SeverityWarning:
		return "W"
	default:
		return "I"
	}
}

// 规则号（契约 §5.2；前缀可机读，消息为人读）。
const (
	// RuleSchema（E-SCHEMA）：未知 schema_version、未知字段、枚举外取值、类型/格式错。
	RuleSchema = "E-SCHEMA"
	// RuleRequired（E-REQ）：必填段/必填字段缺失或为空（含 FLOW-CW）；REG-METER/REG-LOC/REG-ALIAS 违例。
	RuleRequired = "E-REQ"
	// RuleRef（E-REF）：REF-* 引用不存在的 id；各列表 id 重复。
	RuleRef = "E-REF"
	// RuleMat（E-MAT）：MAT-PATH（绝对路径/越出包根）；MAT-FILE 归装配/编译期（§4.4/§6.3）。
	RuleMat = "E-MAT"
	// RuleCap（E-CAP）：CAP-BRIDGE（capability_request 无闭包表条目且 required=true）。
	// 本卡为表接口 stub：Options.CapabilityClosure 未接线（nil）时不产生。
	RuleCap = "E-CAP"
	// RuleCheckWarn（W-CHK）：check/complete_when 项无法映射到卡判定面 → 编译期降 advisory。
	// 本卡为映射表接口 stub：Options.CheckMapping 未接线（nil）时不产生。
	RuleCheckWarn = "W-CHK"
	// RuleMatWarn（W-MAT）：file:null 材料；catalog 核验状态不足（归 W01-B/C）；source 含"未听核/待转写"。
	RuleMatWarn = "W-MAT"
	// RuleBudget 是契约 §5.2 保留的规则号 W-BUDGET；R5=A 后其出口等级为 E（fail-closed，
	// 不注入缺省）。诊断前缀保留 W-BUDGET 以便按契约 §5.2 对账，等级见 Severity 字段。
	RuleBudget = "W-BUDGET"
	// RuleNote（I-NOTE）：设计层字段不进卡、estimated_minutes=null、pending_questions 计数、
	// 可选段缺省注入提示等。
	RuleNote = "I-NOTE"
)

// AllRules 返回契约 §5.2 规则号全集（对账用）。
func AllRules() []string {
	return []string{RuleSchema, RuleRequired, RuleRef, RuleMat, RuleCap, RuleCheckWarn, RuleMatWarn, RuleBudget, RuleNote}
}

// Diagnostic 是单条聚合诊断：规则号前缀（机读）+ 字段路径 + 人读提示（契约 §5.2 输出形态）。
type Diagnostic struct {
	Severity Severity
	Rule     string
	Path     string
	Message  string
}

// String 返回契约 §5.2 规定的诊断文本形态："[规则号] 字段路径 人读提示"。
func (d Diagnostic) String() string {
	if d.Path == "" {
		return fmt.Sprintf("[%s] %s", d.Rule, d.Message)
	}
	return fmt.Sprintf("[%s] %s %s", d.Rule, d.Path, d.Message)
}

// Report 是 E/W/I 聚合诊断集合（同 ValidateLevelCard 的聚合风格）。
type Report struct {
	diags []Diagnostic
}

func newReport() *Report { return &Report{} }

func (r *Report) addf(sev Severity, rule, path, format string, args ...any) {
	r.diags = append(r.diags, Diagnostic{Severity: sev, Rule: rule, Path: path, Message: fmt.Sprintf(format, args...)})
}

func (r *Report) errorf(rule, path, format string, args ...any) {
	r.addf(SeverityError, rule, path, format, args...)
}

func (r *Report) warnf(rule, path, format string, args ...any) {
	r.addf(SeverityWarning, rule, path, format, args...)
}

func (r *Report) infof(rule, path, format string, args ...any) {
	r.addf(SeverityInfo, rule, path, format, args...)
}

// Diagnostics 返回按插入序排列的诊断副本（确定性：校验遍历顺序固定，无 map 迭代）。
func (r *Report) Diagnostics() []Diagnostic {
	out := make([]Diagnostic, len(r.diags))
	copy(out, r.diags)
	return out
}

// Errors 返回全部 E 级诊断。
func (r *Report) Errors() []Diagnostic { return r.filter(SeverityError) }

// Warnings 返回全部 W 级诊断。
func (r *Report) Warnings() []Diagnostic { return r.filter(SeverityWarning) }

// Infos 返回全部 I 级诊断。
func (r *Report) Infos() []Diagnostic { return r.filter(SeverityInfo) }

func (r *Report) filter(sev Severity) []Diagnostic {
	var out []Diagnostic
	for _, d := range r.diags {
		if d.Severity == sev {
			out = append(out, d)
		}
	}
	return out
}

// Counts 返回 E/W/I 三档计数。
func (r *Report) Counts() (errors, warnings, infos int) {
	for _, d := range r.diags {
		switch d.Severity {
		case SeverityError:
			errors++
		case SeverityWarning:
			warnings++
		default:
			infos++
		}
	}
	return errors, warnings, infos
}

// HasErrors 报告是否存在 E 级诊断（=校验门拒绝）。
func (r *Report) HasErrors() bool {
	for _, d := range r.diags {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// RuleCount 返回指定规则号的诊断条数（对账/测试用）。
func (r *Report) RuleCount(rule string) int {
	n := 0
	for _, d := range r.diags {
		if d.Rule == rule {
			n++
		}
	}
	return n
}

// Err 在存在 E 级诊断时返回聚合错误（nil=校验通过）；W/I 不构成错误。
func (r *Report) Err() error {
	errors, warnings, infos := r.Counts()
	if errors == 0 {
		return nil
	}
	lines := make([]string, 0, errors)
	for _, d := range r.sorted() {
		if d.Severity == SeverityError {
			lines = append(lines, "  - "+d.String())
		}
	}
	return fmt.Errorf("legame design rejected (%d E / %d W / %d I):\n%s", errors, warnings, infos, strings.Join(lines, "\n"))
}

// String 返回聚合诊断的稳定文本（E→W→I，同级按规则号/路径排序）。
func (r *Report) String() string {
	errors, warnings, infos := r.Counts()
	var b strings.Builder
	fmt.Fprintf(&b, "legame design diagnostics: %d E / %d W / %d I", errors, warnings, infos)
	for _, d := range r.sorted() {
		b.WriteString("\n  ")
		b.WriteString(d.Severity.String())
		b.WriteString(" ")
		b.WriteString(d.String())
	}
	return b.String()
}

// sorted 返回稳定排序后的诊断副本（E→W→I，再看规则号、路径、消息）。
func (r *Report) sorted() []Diagnostic {
	out := r.Diagnostics()
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity < out[j].Severity
		}
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Message < out[j].Message
	})
	return out
}

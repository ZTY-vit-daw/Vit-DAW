package legamedesign

import (
	"bytes"
	"errors"
	"io"

	"gopkg.in/yaml.v3"
)

// Options 携带校验门的表接口（E-CAP / W-CHK）。零值=表未接线：
//   - CapabilityClosure  nil → 不产生 E-CAP（闭包表由 W01-C 登记；W01-A 为 stub）；
//   - CheckMapping       nil → 不产生 W-CHK（判定面映射表由 W01-B 登记；本包不从
//     criterion 文本推断判定面，契约 §2 纪律 2）。
//
// 非 nil 空集合=已接线但表内无条目，会按规则产生 E-CAP/W-CHK（fail-closed）。
type Options struct {
	CapabilityClosure CapabilityClosure
	CheckMapping      CheckJudgementMapping
}

// CapabilityClosure 是 CAP-BRIDGE 能力命令闭包表的只读视图（契约 §6.3 第 1 条）：
// capability_request.id → 是否存在登记的命令名闭包。required=true 且无条目→E-CAP。
type CapabilityClosure interface {
	HasCommandClosure(capabilityID string) bool
}

// CheckJudgementMapping 是 checks→卡判定面映射表的只读视图（契约 §6.2 T-14：
// deterministic 白名单 / rubric→verbal_rubric / user_judgment→event 映射的登记结果）。
// 表内无条目→W-CHK（编译期降 advisory_checks）。
type CheckJudgementMapping interface {
	MapsToCardJudgement(checkID string) bool
}

// CapabilityClosureSet 是 map 形态的 CAP-BRIDGE 闭包表（装配侧便捷实现/测试用）。
type CapabilityClosureSet map[string]bool

// HasCommandClosure 报告 capability_request.id 是否已登记命令闭包。
func (s CapabilityClosureSet) HasCommandClosure(capabilityID string) bool { return s[capabilityID] }

// CheckJudgementSet 是 map 形态的判定面映射表（check.id → 是否可映射）。
type CheckJudgementSet map[string]bool

// MapsToCardJudgement 报告 check.id 是否已登记可映射到卡判定面。
func (s CheckJudgementSet) MapsToCardJudgement(checkID string) bool { return s[checkID] }

// Parse 解析并校验设计源字节流（零 IO；yaml KnownFields(true) fail-closed）。
// E 级诊断存在时返回 (nil, report)，调用方可以 report.Err() 取得聚合错误；
// W/I 诊断不影响返回（调用方应读取 report.Warnings()/Infos() 进入回执）。
func Parse(data []byte) (*Design, *Report) { return ParseWithOptions(data, Options{}) }

// ParseWithOptions 同 Parse，额外接入 E-CAP/W-CHK 表接口（契约 §5.2 stub 面）。
func ParseWithOptions(data []byte, opts Options) (*Design, *Report) {
	var d Design
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true) // 未知字段 fail-closed（与 edu.ParseLevelCard 同口径）
	if err := dec.Decode(&d); err != nil {
		r := newReport()
		r.errorf(RuleSchema, "", "YAML 解析失败（KnownFields(true) fail-closed，未知字段/类型错见下）: %v", err)
		return nil, r
	}
	// 设计源须恰好一个 YAML 文档：多文档静默忽略=静默纠正，fail-closed 拒绝。
	var extra any
	if err := dec.Decode(&extra); err == nil {
		r := newReport()
		r.errorf(RuleSchema, "", "设计源含多个 YAML 文档（须恰好一个；多文档 fail-closed 拒绝）")
		return nil, r
	} else if !errors.Is(err, io.EOF) {
		r := newReport()
		r.errorf(RuleSchema, "", "YAML 解析失败（第二个文档解码错误）: %v", err)
		return nil, r
	}
	r := ValidateWithOptions(&d, opts)
	if r.HasErrors() {
		return nil, r
	}
	return &d, r
}

// Validate 对已解析的设计源执行语义校验（零 IO；E/W/I 聚合返回）。
func Validate(d *Design) *Report { return ValidateWithOptions(d, Options{}) }

// ValidateWithOptions 同 Validate，额外接入 E-CAP/W-CHK 表接口。
func ValidateWithOptions(d *Design, opts Options) *Report {
	return validateDesign(d, opts)
}

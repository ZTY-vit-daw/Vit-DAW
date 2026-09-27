package materialize

// mode.go — 三态运行 flag（MAT-C，设计 §7.1 裁决 R6）。
//
//	VIT_DAW_MATERIALIZATION=off|shadow|on     # 默认 off（harness 既有 VIT_DAW_* 惯例）
//	VIT_DAW_MATERIALIZED_KINDS=tom,acp,dom    # 白名单：flag≠off 时参与物化的 kind 子集
//
// 三态语义（§7.1 表）：
//
//	off（现状）  Notifier=nil、零装配，与现状逐字节一致（off 态锁定测试见证）；
//	shadow（影子）全速物化（事件接线+脏传播+lazy 重算+登记），读端不消费；每轮
//	              observe 后对账：现算产物 vs 物化行逐行 hash 比，分歧入
//	              Metrics.ShadowDivergences；
//	on（切换）    物化层行为同 shadow；读端命中装配/miss 回填归 MAT-E（本卡 on
//	              态不消费读端——装配边界随回执申报）。
//
// 非法 mode 值 fail-loud 报错并回退 off（旁路层宁可不开不可开错）。

import (
	"fmt"
	"strings"
)

// 环境变量名（§7.1 原文）。
const (
	EnvMode  = "VIT_DAW_MATERIALIZATION"
	EnvKinds = "VIT_DAW_MATERIALIZED_KINDS"
)

// Mode 是三态 flag 的取值。
type Mode string

const (
	ModeOff    Mode = "off"
	ModeShadow Mode = "shadow"
	ModeOn     Mode = "on"
)

// defaultWhitelist 是 flag≠off 且未显式给白名单时的默认集合（§7.2 三个先切
// kind 及依据；修改须同步 mode 测试锁定）。
var defaultWhitelist = []string{"tom", "acp", "dom"}

// Config 是解析后的运行配置。off 态 Kinds 恒空（不参与物化）；shadow/on 态
// Kinds 非空（显式白名单或默认三 kind）。白名单保留调用方原样条目——未知
// kind 名不在此过滤（无适配器即不参与重算，宁多勿漏由传播兜底），typo 的
// 暴露归影子对账期。
type Config struct {
	Mode  Mode
	Kinds []string
}

// ConfigFromEnv 从 getenv（通常 os.Getenv；测试注入）解析三态配置。
// 返回错误=非法 mode（零值 Config 已回退 off）；getenv nil=off。
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	if getenv == nil {
		return Config{Mode: ModeOff}, nil
	}
	raw := strings.ToLower(strings.TrimSpace(getenv(EnvMode)))
	switch raw {
	case "", string(ModeOff):
		return Config{Mode: ModeOff}, nil
	case string(ModeShadow), string(ModeOn):
		kinds := parseKindList(getenv(EnvKinds))
		if len(kinds) == 0 {
			kinds = append([]string(nil), defaultWhitelist...)
		}
		return Config{Mode: Mode(raw), Kinds: kinds}, nil
	default:
		return Config{Mode: ModeOff}, fmt.Errorf("materialize: 非法 %s=%q（合法 off|shadow|on；已回退 off）", EnvMode, raw)
	}
}

// parseKindList 拆逗号白名单：小写归一、去空白、剔空项。
func parseKindList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if kind := strings.ToLower(strings.TrimSpace(part)); kind != "" {
			out = append(out, kind)
		}
	}
	return out
}

// package pullharness — L1-5 pull 模式新 harness 骨架（HARNESS_V1_DESIGN §3）。
//
// 本包是纯新增面（A/B 并存前提）：旧 harness（agentloop/chat 双入口）零改动，
// 生产缺省仍走 push（§11 兼容义务）。IMPL-A 落地三件：
//
//	mode.go   flag 面：VIT_DAW_HARNESS（pull|push，env wins）+
//	          agent_runtime_config.json 键 harness_mode（会话启动读取），
//	          缺省 push；UTF-8 BOM 容错沿 CONFIG-BOM-1 单点教训。
//	budget.go ObservationBudget（MaxCycles/ProbeCost/MaxProbeCost，§3.4）：
//	          超限=止损终态 budget_exhausted 独立失败分类（AGENTS §8 分记）。
//	loop.go   PullLoop 六注入+七步循环（§3.1）：T1=每工具批后 OnTurnBoundary
//	          （TurnID=<RunID>:cycle:<n>），T2=run 终态（TurnID=RunID，沿
//	          exit_retain.go 先例）；溢出 fail-closed 复用
//	          contextruntime.ModelContextOverflow（不新造）。
//
// 接线归后续卡：冷启动底座装配=B（§4.1 空槽），快路径 Router=C/D（§5），
// 真实工具面与协议解析=D（§3.3），G3 A/B 执行=D（§6）。
package pullharness

// mode.go — harness 模式选择 flag 面（HARNESS_V1_DESIGN §6）。
//
// 优先级（与 blind 配置面同型先例，chat/audition_events.go）：
// env VIT_DAW_HARNESS > VitApp/Workspace/agent_runtime_config.json 键
// "harness_mode" > 缺省 push。会话启动读取一次，运行中不改。
// CONFIG-BOM-1 教训：Windows 写入方（PowerShell 5.1 Set-Content/旧版记事本）
// 默认产 UTF-8 BOM，encoding/json 直接拒绝——解析前剥 BOM 前缀，仅此一处
// 容错；其余解析失败 fail-visible（config_invalid 显式出现在 Source），
// 模式回落 push（缺省=旧路径零变化）。

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// harness 模式取值。pull=新 harness（本包）；push=旧 harness（缺省）。
const (
	ModePull = "pull"
	ModePush = "push"
)

const (
	// HarnessModeEnvVar 环境变量开关（pull|push，设了就赢——与 blind 面
	// "the environment variable wins when it is set" 同型）。
	HarnessModeEnvVar = "VIT_DAW_HARNESS"

	// 配置文件面：$VIT_DAW_AGENT_ROOT（缺省 D:/Vit_DAW）+
	// VitApp/Workspace/agent_runtime_config.json，键 "harness_mode"。
	harnessModeConfigEnvRoot = "VIT_DAW_AGENT_ROOT"
	harnessModeConfigRoot    = "D:/Vit_DAW"
	harnessModeConfigRelPath = "VitApp/Workspace/agent_runtime_config.json"
	HarnessModeConfigKey     = "harness_mode"
)

// HarnessMode 是一次模式解析的结果：Mode 恒为 pull|push（无效输入一律
// 回落 push），Source 记录裁定来源（fail-visible：config_invalid 等
// 显式可见，不静默吞掉——blind 配置面先例）。
type HarnessMode struct {
	Mode   string
	Source string
}

// ResolveHarnessMode 在会话启动时解析 harness 模式（本包无运行中重读面）。
func ResolveHarnessMode() HarnessMode {
	if raw, ok := os.LookupEnv(HarnessModeEnvVar); ok && strings.TrimSpace(raw) != "" {
		if mode, valid := normalizeHarnessMode(raw); valid {
			return HarnessMode{Mode: mode, Source: "environment:" + HarnessModeEnvVar}
		}
		// env 设了但值非法：env wins 语义下不回落配置面，push 兜底 + 显式留痕。
		return HarnessMode{Mode: ModePush, Source: "environment_invalid:" + HarnessModeEnvVar + "=" + strings.TrimSpace(raw)}
	}

	path := harnessModeConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return HarnessMode{Mode: ModePush, Source: "default:absent"}
		}
		return HarnessMode{Mode: ModePush, Source: "config_unreadable:" + path}
	}
	// CONFIG-BOM-1：仅剥 UTF-8 BOM 前缀，其余解析失败保持 fail-visible。
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return HarnessMode{Mode: ModePush, Source: "config_invalid:" + path}
	}
	value, present := config[HarnessModeConfigKey]
	if !present {
		return HarnessMode{Mode: ModePush, Source: "config:" + path}
	}
	text, ok := value.(string)
	if !ok {
		return HarnessMode{Mode: ModePush, Source: "config_invalid:" + path}
	}
	if mode, valid := normalizeHarnessMode(text); valid {
		return HarnessMode{Mode: mode, Source: "config:" + path}
	}
	return HarnessMode{Mode: ModePush, Source: "config_invalid:" + path}
}

func harnessModeConfigPath() string {
	root := harnessModeConfigRoot
	if envRoot := strings.TrimSpace(os.Getenv(harnessModeConfigEnvRoot)); envRoot != "" {
		root = envRoot
	}
	root = filepath.FromSlash(strings.ReplaceAll(root, "\\", "/"))
	return filepath.Join(root, filepath.FromSlash(harnessModeConfigRelPath))
}

// normalizeHarnessMode 只认 pull|push（大小写/空白宽容）；第二个返回值
// false=值非法（调用方按各自面 fail-visible）。
func normalizeHarnessMode(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case ModePull:
		return ModePull, true
	case ModePush:
		return ModePush, true
	}
	return ModePush, false
}

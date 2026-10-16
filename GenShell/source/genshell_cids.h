//------------------------------------------------------------------------
// GenShell — 生成插件壳体（GENSHELL-CPP-1 骨架）
//
// 身份与常量：类 CID 固定（GEN-PLUGIN-SHELL-DESIGN-1 件一 §7——CID 钉死入契约头）。
// T1 参数面 / T2 信封 / T3 chunk 的值域权威 = coord/runs/GEN-PLUGIN-SHELL-DESIGN-1。
//------------------------------------------------------------------------
#pragma once

#include "pluginterfaces/base/funknown.h"
#include "pluginterfaces/vst/vsttypes.h"

namespace Steinberg {
namespace Vst {

//------------------------------------------------------------------------
// 类标识（固定 GUID，勿改——工程/宿主侧按此识别 GenShell 实体）
//------------------------------------------------------------------------
// {7A5C1B90-3E44-4D6A-9B2E-4C1F0A6B8D01} processor
const FUID GenShellProcessorCID (0x7A5C1B90, 0x3E444D6A, 0x9B2E4C1F, 0x0A6B8D01);
// {7A5C1B90-3E44-4D6A-9B2E-4C1F0A6B8D02} controller
const FUID GenShellControllerCID (0x7A5C1B90, 0x3E444D6A, 0x9B2E4C1F, 0x0A6B8D02);

//------------------------------------------------------------------------
// 插件身份
//------------------------------------------------------------------------
constexpr const char* kGenShellName = "GenShell";
constexpr const char* kGenShellVendor = "Vit Audio";
constexpr const char* kGenShellVersion = "1.0.0.0";
// 子类别用 PlugType::kInstrumentSynth（ivstaudioprocessor.h，"Instrument|Synth"）

//------------------------------------------------------------------------
// T1 参数面（件一 §3 定稿枚举）——五个状态参数 + 两个 debug 计数参数
//------------------------------------------------------------------------
enum GenShellParamId : ParamID
{
	kParamJobState = 0,        // 离散 8：0=idle..7=cancelled（件二 §1 生命周期）
	kParamJobProgress = 1,     // 归一化 0..1
	kParamJobErrorClass = 2,   // 离散 15：0=none..14=interrupted（件二 §3）
	kParamSceneMode = 3,       // 离散 3：0=in_project,1=free_creation,2=experiment_control
	kParamBypass = 4,          // VST3 标准绕过（kIsBypass）
	// debug 参数（T2 回执计数暴露，件五卡1 验收 3）——不进 chunk
	kParamMsgEchoCount = 200,     // 收到的 vit.genshell.msg.v1 信封条数（已回执）
	kParamMsgUnknownCount = 201,  // 未知 schema_version 被丢弃条数（fail-closed 计数披露）
};

// 计数参数的量程上限（离散步数；normalized = count / kMsgCountStepMax）
constexpr int32 kMsgCountStepMax = 100000;

// SceneMode 值名（离散参数字符串表 + T3 chunk scene_mode 字符串的单一映射源）
constexpr int32 kSceneModeCount = 3;
extern const char16_t* kSceneModeNames[kSceneModeCount]; // "in_project","free_creation","experiment_control"

int32 sceneModeIndexFromName (const char* nameUtf8);              // 未知 → -1
const char* sceneModeNameFromIndex (int32 index);                 // 越界 → "in_project"（fail-closed 默认）

// JobState / JobErrorClass 值名表（T1 显示面）
extern const char16_t* const kJobStateNames[8];
extern const char16_t* const kJobErrorClassNames[15];

//------------------------------------------------------------------------
// T2 消息信封（vit.genshell.msg.v1，件一 §4）——JSON 置于 IMessage 属性 "data"
//------------------------------------------------------------------------
constexpr const char* kGenShellMsgSchema = "vit.genshell.msg.v1";
constexpr const char* kGenShellMsgAttr = "data";

//------------------------------------------------------------------------
// T3 chunk schema（vit.genshell.state.v1，件一 §5）——见 genshell_state.h
//------------------------------------------------------------------------
constexpr const char* kGenShellStateSchema = "vit.genshell.state.v1";

} // namespace Vst
} // namespace Steinberg

//------------------------------------------------------------------------
// GenShell — T1 参数面注册实现
//------------------------------------------------------------------------
#include "genshell_params.h"
#include "genshell_cids.h"

#include "public.sdk/source/vst/vstparameters.h"

namespace Steinberg {
namespace Vst {
class ParameterContainer;
} // namespace Vst
} // namespace Steinberg

namespace GenShell {

using namespace Steinberg::Vst;

namespace {
// char16_t 字面量 → String128 缓冲（appendString 收数组参数）
void appendStringFromLiteral (StringListParameter* p, const char16_t* name)
{
	String128 buf;
	int32_t i = 0;
	for (; name[i] != 0 && i < 127; ++i)
		buf[i] = name[i];
	buf[i] = 0;
	p->appendString (buf);
}
} // namespace

void registerControllerParameters (ParameterContainer& parameters)
{
	{ // JobState：离散 8 值（件二 §1）
		auto* p = new StringListParameter (u"JobState", kParamJobState);
		for (const auto name : kJobStateNames)
			appendStringFromLiteral (p, name);
		parameters.addParameter (p);
	}
	{ // JobProgress：归一化连续 0..1
		parameters.addParameter (new Parameter (u"JobProgress", kParamJobProgress, u"", 0., 0,
		                                        ParameterInfo::kCanAutomate));
	}
	{ // JobErrorClass：离散 15 值（件二 §3）
		auto* p = new StringListParameter (u"JobErrorClass", kParamJobErrorClass);
		for (const auto name : kJobErrorClassNames)
			appendStringFromLiteral (p, name);
		parameters.addParameter (p);
	}
	{ // SceneMode：离散 3 值（件一 §3）
		auto* p = new StringListParameter (u"SceneMode", kParamSceneMode);
		for (Steinberg::int32 i = 0; i < kSceneModeCount; ++i)
			appendStringFromLiteral (p, kSceneModeNames[i]);
		parameters.addParameter (p);
	}
	{ // Bypass：VST3 标准绕过（图内播放静音）
		parameters.addParameter (new Parameter (u"Bypass", kParamBypass, u"", 0., 1,
		                                        ParameterInfo::kCanAutomate | ParameterInfo::kIsBypass));
	}
	{ // debug：T2 回执计数 / 未知 schema 丢弃计数（只读，不进 chunk）
		// RangeParameter 签名：(title, tag, units, minPlain, maxPlain, defaultValuePlain, stepCount, flags)
		parameters.addParameter (new RangeParameter (u"MsgEchoCount", kParamMsgEchoCount, u"msg", 0.,
		                                             static_cast<ParamValue> (kMsgCountStepMax), 0.,
		                                             kMsgCountStepMax, ParameterInfo::kIsReadOnly));
		parameters.addParameter (new RangeParameter (u"MsgUnknownSchemaCount", kParamMsgUnknownCount, u"msg", 0.,
		                                             static_cast<ParamValue> (kMsgCountStepMax), 0.,
		                                             kMsgCountStepMax, ParameterInfo::kIsReadOnly));
	}
}

} // namespace GenShell

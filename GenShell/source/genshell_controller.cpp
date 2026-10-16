//------------------------------------------------------------------------
// GenShell controller 实现
//------------------------------------------------------------------------
#include "genshell_controller.h"
#include "genshell_convert.h"
#include "genshell_editor.h"
#include "genshell_params.h"

#include "pluginterfaces/base/ibstream.h"

#include "../../thirdparty/json/nlohmann_json.hpp"

#include <cstdio>

namespace Steinberg {
namespace Vst {

//------------------------------------------------------------------------
GenShellController::GenShellController ()
{
	// T1 参数面 + debug 计数参数（EditController 基类 parameters 容器）
	GenShell::registerControllerParameters (parameters);
}

tresult PLUGIN_API GenShellController::initialize (FUnknown* context)
{
	return EditController::initialize (context);
}

tresult PLUGIN_API GenShellController::terminate () { return EditController::terminate (); }

//------------------------------------------------------------------------
// T3：宿主把 processor chunk 推给 controller（参数与展示态同步）
//------------------------------------------------------------------------
tresult PLUGIN_API GenShellController::setComponentState (IBStream* stream)
{
	if (stream == nullptr)
		return kInvalidArgument;
	std::string text;
	uint8 byte = 0;
	while (stream->read (&byte, 1, nullptr) == kResultOk)
	{
		if (byte == 0)
			break;
		text.push_back (static_cast<char> (byte));
	}
	bool rejected = false;
	const auto state = GenShell::ShellState::fromJson (text, rejected);
	{
		std::lock_guard<std::mutex> lock (mStateMutex);
		mState = state;
		mChunkSchemaRejected = rejected;
	}
	if (rejected)
		fprintf (stderr, "[GenShell] setComponentState: unknown schema_version rejected; form reset to defaults\n");

	// scene_mode → SceneMode 参数
	const auto idx = sceneModeIndexFromName (state.scene_mode.c_str ());
	const auto normalized = (idx < 0 ? 0 : idx) / static_cast<ParamValue> (kSceneModeCount - 1);
	setParamNormalized (kParamSceneMode, normalized);
	return kResultOk;
}

// controller 自身 state（宿主通常存 processor chunk；getState/setState 保对称）
tresult PLUGIN_API GenShellController::getState (IBStream* stream)
{
	if (stream == nullptr)
		return kInvalidArgument;
	std::string text;
	{
		std::lock_guard<std::mutex> lock (mStateMutex);
		text = mState.toJson ();
	}
	int32 written = 0;
	if (stream->write (const_cast<char*> (text.data ()), static_cast<uint32> (text.size ()), &written) != kResultOk)
		return kResultFalse;
	char zero = 0;
	stream->write (&zero, 1, &written);
	return kResultOk;
}

tresult PLUGIN_API GenShellController::setState (IBStream* stream)
{
	if (stream == nullptr)
		return kInvalidArgument;
	std::string text;
	uint8 byte = 0;
	while (stream->read (&byte, 1, nullptr) == kResultOk)
	{
		if (byte == 0)
			break;
		text.push_back (static_cast<char> (byte));
	}
	bool rejected = false;
	const auto state = GenShell::ShellState::fromJson (text, rejected);
	{
		std::lock_guard<std::mutex> lock (mStateMutex);
		mState = state;
		mChunkSchemaRejected = rejected;
	}
	// 恢复的 chunk 同步回参数面（JUCE 宿主的恢复顺序=component setState →
	// controller setComponentState → controller setState，本方法是最后写者）
	syncSceneModeParamFromState ();
	return kResultOk;
}

//------------------------------------------------------------------------
// SceneMode 参数写入 → 同步 controller 侧展示态（宿主写=壳 GUI scene 选择的便捷镜像）
// 并向 processor 端点发内部 state 同步（JUCE 直连 processor↔controller 连接点；
// processor 侧参数值只有 process() 送达路径，静止工程下保存 component chunk 前
// 需主动同步，否则只存 IComponent state 的宿主会丢 scene_mode）
//------------------------------------------------------------------------
tresult PLUGIN_API GenShellController::setParamNormalized (ParamID tag, ParamValue value)
{
	if (tag == kParamSceneMode)
	{
		const auto idx = static_cast<int32> (value * (kSceneModeCount - 1) + 0.5);
		{
			std::lock_guard<std::mutex> lock (mStateMutex);
			mState.scene_mode = sceneModeNameFromIndex (idx);
		}
		sendStateSyncToProcessor ();
	}
	return EditController::setParamNormalized (tag, value);
}

void GenShellController::syncSceneModeParamFromState ()
{
	std::string scene;
	{
		std::lock_guard<std::mutex> lock (mStateMutex);
		scene = mState.scene_mode;
	}
	const auto idx = sceneModeIndexFromName (scene.c_str ());
	const auto normalized = (idx < 0 ? 0 : idx) / static_cast<ParamValue> (kSceneModeCount - 1);
	EditController::setParamNormalized (kParamSceneMode, normalized);
}

void GenShellController::sendStateSyncToProcessor ()
{
	if (getPeer () == nullptr)
		return;
	IPtr<IMessage> msg (allocateMessage (), false);
	if (msg == nullptr)
		return;
	// 内部同步消息（非 T2 契约信封）：messageID=genshell_state_sync，data=state JSON。
	// 卡2 宿主服务接入 T2 时须按未知 messageID 丢弃（fail-closed 同纪律）。
	msg->setMessageID ("genshell_state_sync");
	std::string jsonText;
	{
		std::lock_guard<std::mutex> lock (mStateMutex);
		jsonText = mState.toJson ();
	}
	std::u16string wide;
	GenShell::utf8ToUtf16 (jsonText, wide);
	msg->getAttributes ()->setString (kGenShellMsgAttr, reinterpret_cast<const TChar*> (wide.c_str ()));
	sendMessage (msg);
}

//------------------------------------------------------------------------
// 编辑器（VSTGUI 静态六区骨架）
//------------------------------------------------------------------------
IPlugView* PLUGIN_API GenShellController::createView (FIDString name)
{
	if (name != nullptr && strcmp (name, ViewType::kEditor) == 0)
	{
		auto* view = new GenShellEditorView (this);
		return view;
	}
	return nullptr;
}

//------------------------------------------------------------------------
// T2 壳侧端点：收信封 → 合法 schema 计数+回 echo；未知 schema 计数+丢弃
//------------------------------------------------------------------------
void GenShellController::bumpCountParam (ParamID id)
{
	const int64 count = (id == kParamMsgEchoCount ? ++mEchoCount : ++mUnknownSchemaCount);
	if (auto* param = parameters.getParameter (id))
	{
		const auto normalized =
			static_cast<ParamValue> (count) / static_cast<ParamValue> (kMsgCountStepMax);
		param->setNormalized (normalized);
		// 宿主通知（无 handler 时静默——单测进程场景）
		FUnknownPtr<IComponentHandler> handler (getComponentHandler ());
		if (handler)
		{
			handler->beginEdit (id);
			handler->performEdit (id, normalized);
			handler->endEdit (id);
		}
	}
}

void GenShellController::sendEchoReply (const std::string& echoedType)
{
	if (getPeer () == nullptr)
		return; // 未连接（如单测外场景）——计数仍保留
	IPtr<IMessage> msg (allocateMessage (), false);
	if (msg == nullptr)
		return;
	msg->setMessageID ("echo");
	nlohmann::json payload;
	payload["schema_version"] = kGenShellMsgSchema;
	payload["type"] = "echo";
	payload["payload"] = {{"echoed_type", echoedType}, {"count", mEchoCount.load ()}};
	const auto text = payload.dump ();
	std::u16string wide;
	GenShell::utf8ToUtf16 (text, wide);
	msg->getAttributes ()->setString (kGenShellMsgAttr, reinterpret_cast<const TChar*> (wide.c_str ()));
	sendMessage (msg);
}

void GenShellController::handleEnvelopeJson (const std::string& jsonText)
{
	try
	{
		const auto parsed = nlohmann::json::parse (jsonText);
		if (!parsed.is_object () ||
			parsed.value ("schema_version", std::string ()) != kGenShellMsgSchema)
		{
			bumpCountParam (kParamMsgUnknownCount); // 丢弃+计数披露（件一 §4 fail-closed）
			return;
		}
		const auto type = parsed.value ("type", std::string ());
		bumpCountParam (kParamMsgEchoCount);
		sendEchoReply (type);
	}
	catch (const nlohmann::json::exception&)
	{
		bumpCountParam (kParamMsgUnknownCount); // 坏 JSON 同按未知处理
	}
}

tresult PLUGIN_API GenShellController::notify (IMessage* message)
{
	if (message == nullptr)
		return kInvalidArgument;
	if (auto* attrs = message->getAttributes ())
	{
		std::vector<TChar> buf (65536);
		const auto size = static_cast<uint32> (buf.size () * sizeof (TChar));
		if (attrs->getString (kGenShellMsgAttr, buf.data (), size) == kResultTrue)
		{
			std::u16string_view wide (buf.data ());
			const auto nul = wide.find (u'\0');
			if (nul != std::u16string_view::npos)
				wide = wide.substr (0, nul);
			std::string text;
			GenShell::utf16ToUtf8 (wide, text);
			handleEnvelopeJson (text);
		}
	}
	return kResultOk;
}

} // namespace Vst
} // namespace Steinberg

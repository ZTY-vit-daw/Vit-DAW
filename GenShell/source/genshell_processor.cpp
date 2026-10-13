//------------------------------------------------------------------------
// GenShell processor 实现
//------------------------------------------------------------------------
#include "genshell_processor.h"
#include "genshell_convert.h"

#include "pluginterfaces/vst/ivstparameterchanges.h"
#include "pluginterfaces/base/ibstream.h"

#include <cstring>
#include <string_view>
#include <vector>

namespace Steinberg {
namespace Vst {

//------------------------------------------------------------------------
GenShellProcessor::GenShellProcessor ()
{
	// instrument 形态：1 事件输入总线 + 1 立体声输出总线
	eventInputs.push_back (
		IPtr<EventBus> (new EventBus (u"Events In", kMain, BusInfo::kDefaultActive, 16), false));
	audioOutputs.push_back (IPtr<AudioBus> (
		new AudioBus (u"Stereo Out", kMain, BusInfo::kDefaultActive, SpeakerArr::kStereo), false));

	setControllerClass (GenShellControllerCID);
}

//------------------------------------------------------------------------
tresult PLUGIN_API GenShellProcessor::initialize (FUnknown* context)
{
	const auto result = Component::initialize (context);
	if (result != kResultOk)
		return result;
	return kResultOk;
}

tresult PLUGIN_API GenShellProcessor::terminate () { return Component::terminate (); }

//------------------------------------------------------------------------
// IAudioProcessor
//------------------------------------------------------------------------
tresult PLUGIN_API GenShellProcessor::setBusArrangements (SpeakerArrangement* /*inputs*/, int32 numIns,
                                                          SpeakerArrangement* outputs, int32 numOuts)
{
	if (numIns == 0 && numOuts == 1 && outputs != nullptr && outputs[0] == SpeakerArr::kStereo)
		return kResultOk;
	return kResultFalse;
}

tresult PLUGIN_API GenShellProcessor::getBusArrangement (BusDirection dir, int32 index,
                                                         SpeakerArrangement& arr)
{
	if (dir == kOutput && index == 0)
	{
		arr = SpeakerArr::kStereo;
		return kResultOk;
	}
	return kResultFalse;
}

tresult PLUGIN_API GenShellProcessor::canProcessSampleSize (int32 symbolicSampleSize)
{
	return symbolicSampleSize == kSample32 ? kResultOk : kResultFalse;
}

tresult PLUGIN_API GenShellProcessor::setupProcessing (ProcessSetup& newSetup)
{
	if (newSetup.symbolicSampleSize != kSample32)
		return kResultFalse;
	mSetup = newSetup;
	return kResultOk;
}

tresult PLUGIN_API GenShellProcessor::process (ProcessData& data)
{
	// 1) 参数变化（宿主推送；含 SceneMode/Bypass）
	if (data.inputParameterChanges != nullptr)
	{
		const auto queueCount = data.inputParameterChanges->getParameterCount ();
		for (int32 q = 0; q < queueCount; ++q)
		{
			auto* queue = data.inputParameterChanges->getParameterData (q);
			if (queue == nullptr)
				continue;
			const auto id = queue->getParameterId ();
			int32 points = queue->getPointCount ();
			if (points <= 0)
				continue;
			ParamValue value = 0.;
			int32 sampleOffset = 0;
			if (queue->getPoint (points - 1, sampleOffset, value) == kResultOk)
			{
				std::lock_guard<std::mutex> lock (mParamMutex);
				switch (id)
				{
					case kParamJobState: mJobStateNorm = value; break;
					case kParamJobProgress: mJobProgressNorm = value; break;
					case kParamJobErrorClass: mJobErrorClassNorm = value; break;
					case kParamSceneMode: mSceneModeNorm = value; break;
					case kParamBypass: mBypassNorm = value; break;
					default: break; // debug 计数参数只存在于 controller（运行时态，不进音频线程）
				}
			}
		}
	}

	// 2) T0：IProcessContext 消费（仅缓存——展示/区域换算用，永不进提示词，件一 §6）
	if (data.processContext != nullptr)
	{
		const auto& ctx = *data.processContext;
		if ((ctx.state & ProcessContext::kTempoValid) != 0)
			mTempoBpm.store (ctx.tempo, std::memory_order_relaxed);
		if ((ctx.state & ProcessContext::kTimeSigValid) != 0)
		{
			mTimeSigNumerator.store (ctx.timeSigNumerator, std::memory_order_relaxed);
			mTimeSigDenominator.store (ctx.timeSigDenominator, std::memory_order_relaxed);
		}
	}

	// 3) instrument 静音输出（骨架：零填充 + 静音标志）
	if (data.numOutputs > 0 && data.numSamples > 0 && data.outputs != nullptr)
	{
		for (int32 bus = 0; bus < data.numOutputs; ++bus)
		{
			auto& out = data.outputs[bus];
			for (int32 ch = 0; ch < out.numChannels; ++ch)
			{
				if (out.channelBuffers32[ch] != nullptr)
					std::memset (out.channelBuffers32[ch], 0,
					             sizeof (float) * static_cast<size_t> (data.numSamples));
			}
			out.silenceFlags = 0xFFFFFFFF; // 全通道静音声明
		}
	}
	return kResultOk;
}

//------------------------------------------------------------------------
// T3 chunk
//------------------------------------------------------------------------
void GenShellProcessor::collectParamsIntoState ()
{
	// scene_mode 从参数值同步进 state（其余 chunk 字段骨架无宿主写入口）
	mState.scene_mode = sceneModeNameFromIndex (static_cast<int32> (mSceneModeNorm * (kSceneModeCount - 1) + 0.5));
}

void GenShellProcessor::applyStateToParams ()
{
	const auto idx = sceneModeIndexFromName (mState.scene_mode.c_str ());
	mSceneModeNorm = (idx < 0 ? 0 : idx) / static_cast<double> (kSceneModeCount - 1);
}

tresult PLUGIN_API GenShellProcessor::setState (IBStream* stream)
{
	if (stream == nullptr)
		return kInvalidArgument;
	// 读全字节（NUL 终止符兼容）
	std::string text;
	uint8 byte = 0;
	while (stream->read (&byte, 1, nullptr) == kResultOk)
	{
		if (byte == 0)
			break;
		text.push_back (static_cast<char> (byte));
	}
	bool rejected = false;
	{
		std::lock_guard<std::mutex> lock (mParamMutex);
		mState = GenShell::ShellState::fromJson (text, rejected);
		applyStateToParams ();
	}
	if (rejected)
		fprintf (stderr, "[GenShell] chunk rejected (unknown schema or bad JSON); defaults kept, len=%zu\n",
		         text.size ());
	return kResultOk; // 未知 schema=丢弃+默认，fail-closed 不崩（件一 §5）
}

tresult PLUGIN_API GenShellProcessor::getState (IBStream* stream)
{
	if (stream == nullptr)
		return kInvalidArgument;
	{
		std::lock_guard<std::mutex> lock (mParamMutex);
		collectParamsIntoState ();
	}
	const auto text = mState.toJson ();
	int32 written = 0;
	if (stream->write (const_cast<char*> (text.data ()), static_cast<uint32> (text.size ()), &written) != kResultOk)
		return kResultFalse;
	char zero = 0;
	stream->write (&zero, 1, &written);
	return kResultOk;
}

tresult PLUGIN_API GenShellProcessor::setActive (TBool state)
{
	return Component::setActive (state);
}

//------------------------------------------------------------------------
tresult PLUGIN_API GenShellProcessor::notify (IMessage* message)
{
	// T2 富消息语义在 controller（件一 §4）。processor 端点只处理壳体内部同步：
	// messageID=genshell_state_sync（controller 在 scene_mode 变更时推送 state JSON，
	// 使静止工程下保存 component chunk 不丢值——process() 未跑时参数值无送达路径）。
	if (message == nullptr)
		return kInvalidArgument;
	const char* mid = message->getMessageID ();
	if (mid == nullptr || strcmp (mid, "genshell_state_sync") != 0)
		return kResultOk; // 未知内部消息：丢弃（不计数——非 T2 契约信封）
	if (auto* attrs = message->getAttributes ())
	{
		std::vector<TChar> buf (65536);
		if (attrs->getString (kGenShellMsgAttr, buf.data (),
		                      static_cast<uint32> (buf.size () * sizeof (TChar))) == kResultTrue)
		{
			std::u16string_view wide (buf.data ());
			const auto nul = wide.find (u'\0');
			if (nul != std::u16string_view::npos)
				wide = wide.substr (0, nul);
			std::string text;
			GenShell::utf16ToUtf8 (wide, text);
			bool rejected = false;
			auto state = GenShell::ShellState::fromJson (text, rejected);
			{
				std::lock_guard<std::mutex> lock (mParamMutex);
				mState = std::move (state);
				applyStateToParams ();
			}
		}
	}
	return kResultOk;
}

double GenShellProcessor::paramNormalized (ParamID id) const
{
	std::lock_guard<std::mutex> lock (mParamMutex);
	switch (id)
	{
		case kParamJobState: return mJobStateNorm;
		case kParamJobProgress: return mJobProgressNorm;
		case kParamJobErrorClass: return mJobErrorClassNorm;
		case kParamSceneMode: return mSceneModeNorm;
		case kParamBypass: return mBypassNorm;
		default: return 0.;
	}
}

} // namespace Vst
} // namespace Steinberg

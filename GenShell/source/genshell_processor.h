//------------------------------------------------------------------------
// GenShell processor — IAudioProcessor instrument 静音骨架 + T0 消费 + T3 chunk
//------------------------------------------------------------------------
#pragma once

#include "genshell_cids.h"
#include "genshell_state.h"

#include "base/source/fobject.h"
#include "pluginterfaces/vst/ivstaudioprocessor.h"
#include "pluginterfaces/vst/ivstprocesscontext.h"
#include "public.sdk/source/vst/vstcomponent.h"

#include <atomic>
#include <mutex>

namespace Steinberg {
namespace Vst {

//------------------------------------------------------------------------
class GenShellProcessor : public Component, public IAudioProcessor, public IProcessContextRequirements
{
public:
	GenShellProcessor ();

	//--- IPluginBase（Component 已实现，转发）---
	tresult PLUGIN_API initialize (FUnknown* context) SMTG_OVERRIDE;
	tresult PLUGIN_API terminate () SMTG_OVERRIDE;

	//--- IAudioProcessor---
	tresult PLUGIN_API setBusArrangements (SpeakerArrangement* inputs, int32 numIns,
	                                        SpeakerArrangement* outputs, int32 numOuts) SMTG_OVERRIDE;
	tresult PLUGIN_API getBusArrangement (BusDirection dir, int32 index,
	                                      SpeakerArrangement& arr) SMTG_OVERRIDE;
	tresult PLUGIN_API canProcessSampleSize (int32 symbolicSampleSize) SMTG_OVERRIDE;
	uint32 PLUGIN_API getLatencySamples () SMTG_OVERRIDE { return 0; }
	tresult PLUGIN_API setupProcessing (ProcessSetup& newSetup) SMTG_OVERRIDE;
	tresult PLUGIN_API setProcessing (TBool state) SMTG_OVERRIDE { return kResultOk; }
	tresult PLUGIN_API process (ProcessData& data) SMTG_OVERRIDE;
	uint32 PLUGIN_API getTailSamples () SMTG_OVERRIDE { return 0; }

	//--- IProcessContextRequirements（T0：tempo/拍号）---
	uint32 PLUGIN_API getProcessContextRequirements () SMTG_OVERRIDE
	{
		return IProcessContextRequirements::kNeedTempo | IProcessContextRequirements::kNeedTimeSignature |
		       IProcessContextRequirements::kNeedProjectTimeMusic | IProcessContextRequirements::kNeedBarPositionMusic |
		       IProcessContextRequirements::kNeedTransportState;
	}

	//--- IComponent chunk（T3）---
	tresult PLUGIN_API setActive (TBool state) SMTG_OVERRIDE;
	tresult PLUGIN_API setState (IBStream* state) SMTG_OVERRIDE;
	tresult PLUGIN_API getState (IBStream* state) SMTG_OVERRIDE;

	//--- IConnectionPoint（ComponentBase 已实现 connect/disconnect；消息由宿主↔controller 链路承担，
	//    processor 侧 notify 只记计数不处理）---
	tresult PLUGIN_API notify (IMessage* message) SMTG_OVERRIDE;

	OBJ_METHODS (GenShellProcessor, Component)
	DEFINE_INTERFACES
		DEF_INTERFACE (IAudioProcessor)
		DEF_INTERFACE (IProcessContextRequirements)
	END_DEFINE_INTERFACES (Component)
	REFCOUNT_METHODS (Component)

	// T0 缓存读取（仅展示/换算用；骨架不外发，测试与 GUI 面板用）
	double cachedTempoBpm () const { return mTempoBpm.load (std::memory_order_relaxed); }
	int32 cachedTimeSigNumerator () const { return mTimeSigNumerator.load (std::memory_order_relaxed); }
	int32 cachedTimeSigDenominator () const { return mTimeSigDenominator.load (std::memory_order_relaxed); }

	// processor 侧参数值缓存（宿主经 inputParameterChanges 推送）
	double paramNormalized (ParamID id) const;

protected:
	GenShell::ShellState mState; // T3 权威（scene_mode 与参数缓存同步）

private:
	void applyStateToParams ();
	void collectParamsIntoState ();

	std::atomic<double> mTempoBpm { 120. };
	std::atomic<int32> mTimeSigNumerator { 4 };
	std::atomic<int32> mTimeSigDenominator { 4 };

	// 音频线程侧参数值（宿主推送；mutex 保护与 getState 并发——save 时机在非音频线程）
	mutable std::mutex mParamMutex;
	double mSceneModeNorm { 0. }; // SceneMode 归一化（stepCount=2）
	double mBypassNorm { 0. };
	double mJobStateNorm { 0. };
	double mJobProgressNorm { 0. };
	double mJobErrorClassNorm { 0. };
	ProcessSetup mSetup {};
};

} // namespace Vst
} // namespace Steinberg

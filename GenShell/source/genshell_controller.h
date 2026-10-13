//------------------------------------------------------------------------
// GenShell controller — IEditController + T2 IConnectionPoint 壳侧端点
//------------------------------------------------------------------------
#pragma once

#include "genshell_cids.h"
#include "genshell_state.h"

#include "pluginterfaces/vst/ivsteditcontroller.h"
#include "pluginterfaces/vst/ivstmessage.h"
#include "public.sdk/source/vst/vsteditcontroller.h"

#include <atomic>
#include <mutex>

namespace Steinberg {
namespace Vst {

//------------------------------------------------------------------------
class GenShellController : public EditController
{
public:
	GenShellController ();

	tresult PLUGIN_API initialize (FUnknown* context) SMTG_OVERRIDE;
	tresult PLUGIN_API terminate () SMTG_OVERRIDE;

	//--- IEditController---
	tresult PLUGIN_API setComponentState (IBStream* state) SMTG_OVERRIDE;
	IPlugView* PLUGIN_API createView (FIDString name) SMTG_OVERRIDE;
	tresult PLUGIN_API setParamNormalized (ParamID tag, ParamValue value) SMTG_OVERRIDE;
	tresult PLUGIN_API setState (IBStream* state) SMTG_OVERRIDE;
	tresult PLUGIN_API getState (IBStream* state) SMTG_OVERRIDE;

	//--- IConnectionPoint（T2 壳侧端点；ComponentBase 提供基础实现，此处覆写 notify）---
	tresult PLUGIN_API notify (IMessage* message) SMTG_OVERRIDE;

	OBJ_METHODS (GenShellController, EditController)
	REFCOUNT_METHODS (EditController)

private:
	// T2：处理一条信封 JSON。合法 schema → 计数+回执 echo；未知 → 计数+丢弃。
	void handleEnvelopeJson (const std::string& jsonText);
	void sendEchoReply (const std::string& echoedType);
	void bumpCountParam (ParamID id); // 计数参数 +1（经 componentHandler performEdit 通知宿主）
	void syncSceneModeParamFromState ();  // chunk 恢复 → SceneMode 参数
	void sendStateSyncToProcessor ();     // scene_mode 变更 → processor 内部同步消息

	GenShell::ShellState mState; // controller 侧展示副本（setComponentState 同步）
	bool mChunkSchemaRejected = false;
	std::atomic<int64_t> mEchoCount { 0 };
	std::atomic<int64_t> mUnknownSchemaCount { 0 };
	mutable std::mutex mStateMutex;
};

} // namespace Vst
} // namespace Steinberg

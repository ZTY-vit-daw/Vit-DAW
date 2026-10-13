//------------------------------------------------------------------------
// GenShell editor — VSTGUI 静态六区骨架（件三 §4 六区占位；无业务逻辑）
// 视觉面零断言（卡1端测边界）：结构=六区容器+标题标签，验收只看进程存活。
//------------------------------------------------------------------------
#pragma once

#include "pluginterfaces/vst/ivstplugview.h"
#include "public.sdk/source/common/pluginview.h"

namespace VSTGUI {
class CFrame;
}

namespace Steinberg {
namespace Vst {

class GenShellController;

//------------------------------------------------------------------------
class GenShellEditorView : public CPluginView
{
public:
	explicit GenShellEditorView (GenShellController* controller);

	tresult PLUGIN_API isPlatformTypeSupported (FIDString type) SMTG_OVERRIDE;
	tresult PLUGIN_API attached (void* parent, FIDString platformType) SMTG_OVERRIDE;
	tresult PLUGIN_API removed () SMTG_OVERRIDE;
	tresult PLUGIN_API onSize (ViewRect* newSize) SMTG_OVERRIDE;
	tresult PLUGIN_API getSize (ViewRect* size) SMTG_OVERRIDE;
	tresult PLUGIN_API canResize () SMTG_OVERRIDE { return kResultFalse; }

	OBJ_METHODS (GenShellEditorView, CPluginView)

private:
	void buildContent (); // 六区构建（frame 存在时）

	GenShellController* mController { nullptr }; // 弱引用（宿主持有两者生命周期）
	VSTGUI::CFrame* mFrame { nullptr };
};

} // namespace Vst
} // namespace Steinberg

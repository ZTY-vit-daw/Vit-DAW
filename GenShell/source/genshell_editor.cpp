//------------------------------------------------------------------------
// GenShell editor 实现 — VSTGUI 六区静态骨架
//------------------------------------------------------------------------
#include "genshell_editor.h"
#include "genshell_controller.h"

#include "pluginterfaces/gui/iplugview.h"
#include "vstgui/vstgui.h"

#include <cstring>

using namespace VSTGUI;

namespace Steinberg {
namespace Vst {

//------------------------------------------------------------------------
namespace {

// 六区布局（固定 760x520；件三 §4）
constexpr CCoord kEditorWidth = 760;
constexpr CCoord kEditorHeight = 520;

constexpr auto kColorBg = CColor (0x1E, 0x1E, 0x24);      // 全局底
constexpr auto kColorTitle = CColor (0x10, 0x10, 0x14);   // 标题条
constexpr auto kColorZone1 = CColor (0x2B, 0x4A, 0x6F);   // 模板选择
constexpr auto kColorZone2 = CColor (0x2F, 0x5D, 0x50);   // 结构化字段表单
constexpr auto kColorZone3 = CColor (0x57, 0x4A, 0x2E);   // 自由文本区
constexpr auto kColorZone4 = CColor (0x4A, 0x2E, 0x57);   // 工程上下文面板
constexpr auto kColorZone5 = CColor (0x5D, 0x2F, 0x3A);   // 生成参数区
constexpr auto kColorZone6 = CColor (0x33, 0x44, 0x55);   // 作业区
constexpr auto kColorText = kWhiteCColor;
constexpr auto kColorTextDim = CColor (0xC8, 0xC8, 0xD0);

CTextLabel* makeLabel (CViewContainer* parent, const CRect& r, const char* text, bool dim = false)
{
	auto* label = new CTextLabel (r, text);
	label->setFontColor (dim ? kColorTextDim : kColorText);
	label->setBackColor (CColor (0, 0, 0, 0)); // 透明（区底色透出）
	label->setMouseEnabled (false);
	parent->addView (label);
	return label;
}

CViewContainer* makeZone (CViewContainer* parent, const CRect& r, const CColor& bg)
{
	auto* zone = new CViewContainer (r);
	zone->setBackgroundColor (bg);
	zone->setMouseEnabled (false);
	parent->addView (zone);
	return zone;
}

} // namespace

//------------------------------------------------------------------------
GenShellEditorView::GenShellEditorView (GenShellController* controller)
	: CPluginView ()
	, mController (controller)
{
	setRect (ViewRect (0, 0, static_cast<int32> (kEditorWidth), static_cast<int32> (kEditorHeight)));
}

tresult PLUGIN_API GenShellEditorView::isPlatformTypeSupported (FIDString type)
{
	if (type == nullptr)
		return kInvalidArgument;
#if SMTG_OS_MACOS
	return (strcmp (type, Steinberg::kPlatformTypeNSView) == 0) ? kResultTrue : kResultFalse;
#elif SMTG_OS_WINDOWS
	return (strcmp (type, Steinberg::kPlatformTypeHWND) == 0) ? kResultTrue : kResultFalse;
#else
	return kResultFalse;
#endif
}

tresult PLUGIN_API GenShellEditorView::attached (void* parent, FIDString /*platformType*/)
{
	if (parent == nullptr)
		return kInvalidArgument;
	if (mFrame == nullptr)
	{
		const CRect frameRect (0, 0, kEditorWidth, kEditorHeight);
		mFrame = new CFrame (frameRect, nullptr);
		mFrame->setBackgroundColor (kColorBg);
		buildContent ();
	}
	mFrame->open (parent);
	return kResultOk;
}

tresult PLUGIN_API GenShellEditorView::removed ()
{
	if (mFrame != nullptr)
	{
		mFrame->close ();
		mFrame->forget ();
		mFrame = nullptr;
	}
	return kResultOk;
}

tresult PLUGIN_API GenShellEditorView::onSize (ViewRect* newSize)
{
	if (newSize == nullptr)
		return kInvalidArgument;
	// 固定尺寸骨架：不响应宿主改尺寸
	return kResultFalse;
}

tresult PLUGIN_API GenShellEditorView::getSize (ViewRect* size)
{
	if (size == nullptr)
		return kInvalidArgument;
	*size = ViewRect (0, 0, static_cast<int32> (kEditorWidth), static_cast<int32> (kEditorHeight));
	return kResultOk;
}

void GenShellEditorView::buildContent ()
{
	if (mFrame == nullptr)
		return;
	auto* root = static_cast<CViewContainer*> (mFrame); // CFrame 即 CViewContainer
	root->setBackgroundColor (kColorBg);

	// 标题条
	{
		const CRect r (0, 0, kEditorWidth, 26);
		auto* zone = makeZone (root, r, kColorTitle);
		makeLabel (zone, CRect (8, 5, 400, 22), "GenShell — 生成插件壳体（骨架 v1）");
	}

	// ① 模板选择（下拉；静态占位）
	{
		const CRect r (0, 26, kEditorWidth, 88);
		auto* zone = makeZone (root, r, kColorZone1);
		makeLabel (zone, CRect (10, 6, 380, 24), "① 模板选择");
		makeLabel (zone, CRect (10, 30, 380, 46), "内置+用户模板下拉（卡4接线）", true);
	}

	// ② 结构化字段表单 / ③ 自由文本区
	{
		{
			const CRect r (0, 88, 380, 288);
			auto* zone = makeZone (root, r, kColorZone2);
			makeLabel (zone, CRect (10, 6, 360, 24), "② 结构化字段表单");
			makeLabel (zone, CRect (10, 30, 360, 46), "按模板 fields 动态渲染（卡4接线）", true);
		}
		{
			const CRect r (380, 88, kEditorWidth, 288);
			auto* zone = makeZone (root, r, kColorZone3);
			makeLabel (zone, CRect (10, 6, 360, 24), "③ 自由文本区");
			makeLabel (zone, CRect (10, 30, 360, 46), "用户原文=USER 块尾段（卡4接线）", true);
		}
	}

	// ④ 工程上下文面板 / ⑤ 生成参数区
	{
		{
			const CRect r (0, 288, 380, 428);
			auto* zone = makeZone (root, r, kColorZone4);
			makeLabel (zone, CRect (10, 6, 360, 24), "④ 工程上下文面板");
			makeLabel (zone, CRect (10, 30, 360, 46), "context_snapshot 只读展示（卡2/4接线）", true);
		}
		{
			const CRect r (380, 288, kEditorWidth, 428);
			auto* zone = makeZone (root, r, kColorZone5);
			makeLabel (zone, CRect (10, 6, 360, 24), "⑤ 生成参数区");
			makeLabel (zone, CRect (10, 30, 360, 46), "provider/scene/区域+[生成][取消]（卡4接线）", true);
		}
	}

	// ⑥ 作业区
	{
		const CRect r (0, 428, kEditorWidth, kEditorHeight);
		auto* zone = makeZone (root, r, kColorZone6);
		makeLabel (zone, CRect (10, 6, 380, 24), "⑥ 作业区");
		makeLabel (zone, CRect (10, 30, 480, 46),
		           "状态/进度/失败类别+重试/候选 take 列表（卡2/4接线）", true);
	}
}

} // namespace Vst
} // namespace Steinberg

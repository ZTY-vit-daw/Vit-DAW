//------------------------------------------------------------------------
// GenShell 模块工厂（VST3 SDK 直实现——JUCE 裁剪版 SDK 无 public.sdk/source/main，
// 按 IPluginFactory3 契约手写；GetPluginFactory 为 bundle 唯一导出入口）
//------------------------------------------------------------------------
#include "genshell_cids.h"
#include "genshell_controller.h"
#include "genshell_processor.h"

#include "pluginterfaces/base/ipluginbase.h"
#include "pluginterfaces/base/funknown.h"
#include "pluginterfaces/vst/ivstaudioprocessor.h"
#include "pluginterfaces/vst/ivsteditcontroller.h"

#include <cstring>

using namespace Steinberg;
using namespace Steinberg::Vst;

#if defined(__APPLE__)
#define GENSHELL_EXPORT __attribute__ ((visibility ("default")))
#elif defined(_WIN32) || defined(_WIN64)
#define GENSHELL_EXPORT __declspec (dllexport)
#else
#define GENSHELL_EXPORT __attribute__ ((visibility ("default")))
#endif

//------------------------------------------------------------------------
namespace {

class GenShellFactory final : public FObject, public IPluginFactory3
{
public:
	GenShellFactory () = default;

	//--- IPluginFactory ---
	tresult PLUGIN_API getFactoryInfo (PFactoryInfo* info) SMTG_OVERRIDE
	{
		if (info == nullptr)
			return kInvalidArgument;
		std::strncpy (info->vendor, kGenShellVendor, sizeof (info->vendor) - 1);
		std::strncpy (info->url, "https://github.com/ZTY-vit-daw", sizeof (info->url) - 1);
		std::strncpy (info->email, "vit@example.invalid", sizeof (info->email) - 1);
		info->flags = PFactoryInfo::kNoFlags;
		return kResultOk;
	}
	int32 PLUGIN_API countClasses () SMTG_OVERRIDE { return 2; }
	tresult PLUGIN_API getClassInfo (int32 index, PClassInfo* info) SMTG_OVERRIDE
	{
		PClassInfo2 info2 {};
		if (getClassInfo2 (index, &info2) != kResultOk)
			return kInvalidArgument;
		if (info == nullptr)
			return kInvalidArgument;
		std::memcpy (info->cid, info2.cid, sizeof (TUID));
		info->cardinality = info2.cardinality;
		std::memcpy (info->category, info2.category, sizeof (info->category));
		std::memcpy (info->name, info2.name, sizeof (info->name));
		return kResultOk;
	}
	tresult PLUGIN_API createInstance (FIDString cid, FIDString iid, void** obj) SMTG_OVERRIDE
	{
		if (cid == nullptr || iid == nullptr || obj == nullptr)
			return kInvalidArgument;
		const FUID classId = FUID::fromTUID (cid);
		if (classId == GenShellProcessorCID)
		{
			auto* p = new GenShellProcessor;
			const auto result = p->queryInterface (iid, obj);
			p->release (); // queryInterface 成功时已 retain 给调用方
			return result;
		}
		if (classId == GenShellControllerCID)
		{
			auto* c = new GenShellController;
			const auto result = c->queryInterface (iid, obj);
			c->release ();
			return result;
		}
		*obj = nullptr;
		return kNoInterface;
	}
	//--- IPluginFactory2 ---
	tresult PLUGIN_API getClassInfo2 (int32 index, PClassInfo2* info) SMTG_OVERRIDE
	{
		if (info == nullptr || index < 0 || index >= countClasses ())
			return kInvalidArgument;
		TUID processorCid, controllerCid;
		GenShellProcessorCID.toTUID (processorCid);
		GenShellControllerCID.toTUID (controllerCid);
		if (index == 0)
		{
			*info = PClassInfo2 (processorCid, PClassInfo::kManyInstances, kVstAudioEffectClass,
			                     kGenShellName, 0, PlugType::kInstrumentSynth, kGenShellVendor,
			                     kGenShellVersion, kVstVersionString);
		}
		else
		{
			*info = PClassInfo2 (controllerCid, PClassInfo::kManyInstances, kVstComponentControllerClass,
			                     kGenShellName, 0, "", kGenShellVendor, kGenShellVersion, kVstVersionString);
		}
		return kResultOk;
	}

	//--- IPluginFactory3 ---
	tresult PLUGIN_API getClassInfoUnicode (int32 index, PClassInfoW* info) SMTG_OVERRIDE
	{
		if (info == nullptr)
			return kInvalidArgument;
		PClassInfo2 info2 {};
		if (getClassInfo2 (index, &info2) != kResultOk)
			return kInvalidArgument;
		std::memcpy (info->cid, info2.cid, sizeof (TUID));
		info->cardinality = info2.cardinality;
		info->classFlags = info2.classFlags;
		// PClassInfoW：category/subCategories 为 char8，name/vendor/version/sdkVersion 为 char16
		std::memcpy (info->category, info2.category, sizeof (info->category));
		std::memcpy (info->subCategories, info2.subCategories, sizeof (info->subCategories));
		auto copyAscii = [] (char16* dst, const char8* src, size_t cap) {
			size_t i = 0;
			for (; src[i] != 0 && i < cap - 1; ++i)
				dst[i] = static_cast<char16> (src[i]);
			dst[i] = 0;
		};
		copyAscii (info->name, info2.name, sizeof (info->name));
		copyAscii (info->vendor, info2.vendor, sizeof (info->vendor));
		copyAscii (info->version, info2.version, sizeof (info->version));
		copyAscii (info->sdkVersion, info2.sdkVersion, sizeof (info->sdkVersion));
		return kResultOk;
	}
	tresult PLUGIN_API setHostContext (FUnknown* /*context*/) SMTG_OVERRIDE { return kNotImplemented; }

	OBJ_METHODS (GenShellFactory, FObject)
	DEFINE_INTERFACES
		DEF_INTERFACE (IPluginFactory)
		DEF_INTERFACE (IPluginFactory2)
		DEF_INTERFACE (IPluginFactory3)
	END_DEFINE_INTERFACES (FObject)
	REFCOUNT_METHODS (FObject)
};

static IPtr<GenShellFactory> gFactory;

} // namespace

//------------------------------------------------------------------------
// bundle 入口（宿主经 CFBundle/GetProcAddress 取符号 GetPluginFactory）
//------------------------------------------------------------------------
extern "C" GENSHELL_EXPORT IPluginFactory* PLUGIN_API GetPluginFactory ()
{
	if (gFactory == nullptr)
		gFactory = owned (new GenShellFactory ());
	else
		gFactory->addRef ();
	return gFactory;
}

#if defined(__APPLE__)
// macOS VST3 装载约定（module_mac.mm）：bundleEntry/bundleExit 必须导出。
// VSTGUI 4.10+ 要求使用前 VSTGUI::init（字体等全局初始化）——装载点即此处。
#include <CoreFoundation/CoreFoundation.h>
#include "vstgui/lib/vstguiinit.h"
extern "C" GENSHELL_EXPORT bool bundleEntry (CFBundleRef bundle)
{
	VSTGUI::init (bundle);
	return true;
}
extern "C" GENSHELL_EXPORT bool bundleExit ()
{
	VSTGUI::exit ();
	return true;
}
#elif defined(_WIN32) || defined(_WIN64)
extern "C" GENSHELL_EXPORT bool InitDll () { return true; }
extern "C" GENSHELL_EXPORT bool ExitDll () { return true; }
#endif

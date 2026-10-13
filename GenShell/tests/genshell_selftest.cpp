//------------------------------------------------------------------------
// GenShell 壳单测进程（模拟宿主端——件五卡1 验收 3：卡2 宿主服务前的临时形态）
//
// 用 VST3 SDK hosting 模块加载构建产物 GenShell.vst3，做两组断言：
//   A. T2 自环：宿主端 IConnectionPoint 发 context_snapshot 信封 → 壳回执
//      (a) controller debug 参数 MsgEchoCount == 1
//      (b) 宿主端收到 echo 回执消息（payload.count == 1）
//      (c) 负探针：未知 schema_version → MsgUnknownSchemaCount == 1 且无 echo
//   B. T3 chunk 全保真往返（表单草稿等无 GUI 入口字段在此证明）：
//      构造含 form_draft/pending_requests/job_history/audition_buffer_ref 的 chunk
//      → processor setState → getState → 逐字段一致；
//      → controller setComponentState → SceneMode 参数镜像（free_creation=0.5）
//   C. 参数面在场：controller 7 参数（T1 五 + debug 二）名称齐全
//   D. 编辑器对象创建不崩（不 attach——进程存活即过）
//
// 退出码 0 = 全部断言通过。
//------------------------------------------------------------------------
#include "genshell_cids.h"
#include "genshell_convert.h"

#include "pluginterfaces/base/funknown.h"
#include "pluginterfaces/base/ipluginbase.h"
#include "pluginterfaces/base/ibstream.h"
#include "pluginterfaces/vst/ivstaudioprocessor.h"
#include "pluginterfaces/vst/ivstcomponent.h"
#include "pluginterfaces/vst/ivsteditcontroller.h"
#include "pluginterfaces/vst/ivstmessage.h"
#include "pluginterfaces/gui/iplugview.h"
#include "pluginterfaces/vst/ivstprocesscontext.h"
#include "public.sdk/source/vst/hosting/hostclasses.h"
#include "public.sdk/source/vst/hosting/module.h"

#include <cstdio>
#include <cstring>
#include <string>
#include <vector>

using namespace Steinberg;
using namespace Steinberg::Vst;

//------------------------------------------------------------------------
namespace {

int gFailures = 0;

void check (bool ok, const std::string& label)
{
	printf ("[%s] %s\n", ok ? "PASS" : "FAIL", label.c_str ());
	if (!ok)
		++gFailures;
}

//------------------------------------------------------------------------
// 内存 IBStream
class MemStream final : public IBStream
{
public:
	DECLARE_FUNKNOWN_METHODS
	MemStream () { FUNKNOWN_CTOR }
	explicit MemStream (const std::string& data) : data_ (data.begin (), data.end ()) { FUNKNOWN_CTOR }
	~MemStream () { FUNKNOWN_DTOR }

	tresult PLUGIN_API read (void* buffer, int32 numBytes, int32* bytesRead) override
	{
		if (buffer == nullptr || numBytes < 0)
			return kInvalidArgument;
		const auto n = static_cast<size_t> (numBytes);
		const auto avail = data_.size () - pos_;
		const auto take = n < avail ? n : avail;
		std::memcpy (buffer, data_.data () + pos_, take);
		pos_ += take;
		if (bytesRead)
			*bytesRead = static_cast<int32> (take);
		return take > 0 || avail > 0 || numBytes == 0 ? kResultOk : kResultFalse;
	}
	tresult PLUGIN_API write (void* buffer, int32 numBytes, int32* bytesWritten) override
	{
		if (buffer == nullptr || numBytes < 0)
			return kInvalidArgument;
		const auto* p = static_cast<const uint8*> (buffer);
		data_.insert (data_.end (), p, p + numBytes);
		pos_ = data_.size ();
		if (bytesWritten)
			*bytesWritten = numBytes;
		return kResultOk;
	}
	tresult PLUGIN_API seek (int64 pos, int32 mode, int64* result) override
	{
		int64 target = pos;
		if (mode == IBStream::kIBSeekSet)
			target = pos;
		else if (mode == IBStream::kIBSeekCur)
			target = static_cast<int64> (pos_) + pos;
		else if (mode == IBStream::kIBSeekEnd)
			target = static_cast<int64> (data_.size ()) + pos;
		else
			return kInvalidArgument;
		if (target < 0)
			return kInvalidArgument;
		pos_ = static_cast<size_t> (target);
		if (result)
			*result = target;
		return kResultOk;
	}
	tresult PLUGIN_API tell (int64* pos) override
	{
		if (pos)
			*pos = static_cast<int64> (pos_);
		return kResultOk;
	}
	std::string text () const { return std::string (data_.begin (), data_.end ()); }

private:
	std::vector<uint8> data_;
	size_t pos_ { 0 };
};

IMPLEMENT_REFCOUNT (MemStream)
tresult PLUGIN_API MemStream::queryInterface (const TUID iid, void** obj)
{
	QUERY_INTERFACE (iid, obj, FUnknown::iid, FUnknown)
	QUERY_INTERFACE (iid, obj, IBStream::iid, IBStream)
	*obj = nullptr;
	return kNoInterface;
}

//------------------------------------------------------------------------
// 宿主端 IConnectionPoint：记录收到的消息
class HostConnectionPoint final : public IConnectionPoint
{
public:
	DECLARE_FUNKNOWN_METHODS
	HostConnectionPoint () { FUNKNOWN_CTOR }
	~HostConnectionPoint () { FUNKNOWN_DTOR }

	tresult PLUGIN_API connect (IConnectionPoint* other) override
	{
		peer = other;
		return kResultOk;
	}
	tresult PLUGIN_API disconnect (IConnectionPoint* /*other*/) override
	{
		peer = nullptr;
		return kResultOk;
	}
	tresult PLUGIN_API notify (IMessage* message) override
	{
		if (message == nullptr)
			return kInvalidArgument;
		lastMessageId = message->getMessageID () ? message->getMessageID () : "";
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
				lastMessageJson = GenShell::utf16ToUtf8 (wide);
			}
		}
		++received;
		return kResultOk;
	}
	// 宿主→壳投递
	tresult deliver (IMessage* m)
	{
		if (peer == nullptr)
			return kResultFalse;
		return peer->notify (m);
	}

	IPtr<IConnectionPoint> peer;
	int received { 0 };
	std::string lastMessageId;
	std::string lastMessageJson;
};

IMPLEMENT_REFCOUNT (HostConnectionPoint)
tresult PLUGIN_API HostConnectionPoint::queryInterface (const TUID iid, void** obj)
{
	QUERY_INTERFACE (iid, obj, FUnknown::iid, FUnknown)
	QUERY_INTERFACE (iid, obj, IConnectionPoint::iid, IConnectionPoint)
	*obj = nullptr;
	return kNoInterface;
}

//------------------------------------------------------------------------
// 宿主侧信封构造（HostMessage + "data" 属性 = 信封 JSON；手工拼串——结构固定）
IPtr<IMessage> makeEnvelope (const char* type, const std::string& payloadJson)
{
	auto* msg = new HostMessage ();
	msg->setMessageID (type);
	const std::string jsonText =
		std::string ("{\"schema_version\":\"") + kGenShellMsgSchema + "\",\"type\":\"" + type +
		"\",\"payload\":" + payloadJson + "}";
	std::u16string wide;
	GenShell::utf8ToUtf16 (jsonText, wide);
	msg->getAttributes ()->setString (kGenShellMsgAttr, reinterpret_cast<const TChar*> (wide.c_str ()));
	return msg;
}

} // namespace

//------------------------------------------------------------------------
int main (int argc, char** argv)
{
	if (argc < 2)
	{
		fprintf (stderr, "usage: genshell_selftest <path-to-GenShell.vst3>\n");
		return 2;
	}
	const std::string bundlePath = argv[1];

	std::string moduleError;
	auto module = VST3::Hosting::Module::create (bundlePath, moduleError);
	if (!module)
	{
		fprintf (stderr, "module load failed: %s\n", moduleError.c_str ());
		return 2;
	}
	check (true, "bundle loaded: " + bundlePath);

	// 工厂（hosting 包装 → 原生 IPluginFactory）
	IPluginFactory* factory = module->getFactory ().get ();
	if (factory == nullptr)
	{
		fprintf (stderr, "GetPluginFactory returned null\n");
		return 2;
	}
	TUID procCid, ctrlCid;
	GenShellProcessorCID.toTUID (procCid);
	GenShellControllerCID.toTUID (ctrlCid);

	// processor
	Vst::IComponent* component = nullptr;
	if (factory->createInstance (reinterpret_cast<FIDString> (procCid), Vst::IComponent::iid,
	                             (void**) &component) != kResultOk ||
	    component == nullptr)
	{
		fprintf (stderr, "processor createInstance failed\n");
		return 2;
	}
	HostApplication hostApp;
	if (component->initialize (&hostApp) != kResultOk)
	{
		fprintf (stderr, "processor initialize failed\n");
		return 2;
	}
	check (true, "processor initialize ok");

	// controller（经 getControllerClassId 路径核对）
	TUID controllerClassId {};
	check (component->getControllerClassId (controllerClassId) == kResultOk &&
	           FUID (controllerClassId) == GenShellControllerCID,
	       "getControllerClassId == GenShellControllerCID");
	Vst::IEditController* controller = nullptr;
	if (factory->createInstance (reinterpret_cast<FIDString> (ctrlCid), Vst::IEditController::iid,
	                             (void**) &controller) != kResultOk ||
	    controller == nullptr)
	{
		fprintf (stderr, "controller createInstance failed\n");
		return 2;
	}
	if (controller->initialize (&hostApp) != kResultOk)
	{
		fprintf (stderr, "controller initialize failed\n");
		return 2;
	}
	check (true, "controller initialize ok");

	//-------------------------------------------------------------
	// C. 参数面在场（T1 五 + debug 二）
	//-------------------------------------------------------------
	{
		const auto count = controller->getParameterCount ();
		std::vector<std::string> names;
		for (int32 i = 0; i < count; ++i)
		{
			ParameterInfo info {};
			if (controller->getParameterInfo (i, info) == kResultOk)
			{
				std::u16string_view wide (info.title);
				names.push_back (GenShell::utf16ToUtf8 (wide));
			}
		}
		check (count == 7, "controller parameter count == 7 (got " + std::to_string (count) + ")");
		for (const char* want : {"JobState", "JobProgress", "JobErrorClass", "SceneMode", "Bypass",
		                         "MsgEchoCount", "MsgUnknownSchemaCount"})
		{
			bool found = false;
			for (const auto& n : names)
				if (n == want)
					found = true;
			check (found, std::string ("parameter present: ") + want);
		}
	}

	//-------------------------------------------------------------
	// A. T2 自环
	//-------------------------------------------------------------
	HostConnectionPoint hostPoint;
	{
		FUnknownPtr<IConnectionPoint> pluginConn (controller);
		check (pluginConn != nullptr, "controller exposes IConnectionPoint");
		// VST3 连接语义：两端各自 connect（ComponentBase 不回调）
		check (hostPoint.connect (pluginConn) == kResultOk, "host side connected");
		check (pluginConn->connect (&hostPoint) == kResultOk, "plugin side connected");

		// 1) context_snapshot 信封
		auto msg = makeEnvelope ("context_snapshot",
		                         "{\"music_context\":{\"tempo_bpm\":120.0},\"injection_ready\":false}");
		check (hostPoint.deliver (msg) == kResultOk, "context_snapshot delivered");
		check (hostPoint.received == 1, "echo reply received (1 message back)");
		check (hostPoint.lastMessageId == "echo", "reply message id == echo");
		check (hostPoint.lastMessageJson.find ("\"count\":1") != std::string::npos,
		       "echo payload count==1");
		const auto echoNorm = controller->getParamNormalized (kParamMsgEchoCount);
		check (echoNorm > 0.0 && echoNorm < 1.0,
		       "MsgEchoCount param > 0 (normalized=" + std::to_string (echoNorm) + ")");
		check (controller->getParamNormalized (kParamMsgUnknownCount) == 0.0,
		       "MsgUnknownSchemaCount still 0");

		// 2) 未知 schema 负探针（丢弃+计数，无 echo）
		IPtr<IMessage> bad (new HostMessage (), false);
		bad->setMessageID ("context_snapshot");
		{
			const std::string badJson = "{\"schema_version\":\"vit.genshell.msg.vX\",\"type\":\"x\"}";
			std::u16string wide;
			GenShell::utf8ToUtf16 (badJson, wide);
			bad->getAttributes ()->setString (kGenShellMsgAttr,
			                                  reinterpret_cast<const TChar*> (wide.c_str ()));
		}
		check (hostPoint.deliver (bad) == kResultOk, "unknown-schema envelope delivered");
		check (hostPoint.received == 1, "no echo for unknown schema");
		check (controller->getParamNormalized (kParamMsgUnknownCount) > 0.0,
		       "MsgUnknownSchemaCount param > 0 after unknown schema");

		// 3) SceneMode 参数写读回（T1 宿主写面）
		check (controller->setParamNormalized (kParamSceneMode, 0.5) == kResultOk,
		       "set SceneMode normalized 0.5");
		check (controller->getParamNormalized (kParamSceneMode) == 0.5, "SceneMode readback 0.5");

		pluginConn->disconnect (&hostPoint);
		hostPoint.disconnect (pluginConn);
	}

	//-------------------------------------------------------------
	// B. T3 chunk 全保真往返（processor setState→getState + controller setComponentState）
	//-------------------------------------------------------------
	{
		const std::string draftChunk =
			"{\"schema_version\":\"vit.genshell.state.v1\","
			"\"selected_template\":\"pad_bed_v1\","
			"\"form_draft\":{\"sections\":\"verse 8 bars + chorus 8 bars\",\"mood\":\"dark\"},"
			"\"scene_mode\":\"free_creation\","
			"\"pending_requests\":[{\"schema_version\":\"vit.genshell.request.v1\",\"job_id\":\"j-1\"}],"
			"\"job_history\":[{\"job_id\":\"j-0\",\"terminal_state\":\"done\",\"clip_id\":\"c-0\","
			"\"take_stack_id\":\"ts-0\",\"artifact_paths\":[\"/tmp/a.wav\",\"/tmp/b.wav\"]}],"
			"\"audition_buffer_ref\":{\"job_id\":\"j-0\",\"take_id\":\"t-0\"}}";

		MemStream inStream (draftChunk);
		check (component->setState (&inStream) == kResultOk, "processor setState (draft chunk)");

		MemStream outStream;
		check (component->getState (&outStream) == kResultOk, "processor getState");
		const auto round = outStream.text ();

		check (round.find ("\"selected_template\":\"pad_bed_v1\"") != std::string::npos,
		       "roundtrip selected_template");
		check (round.find ("\"sections\":\"verse 8 bars + chorus 8 bars\"") != std::string::npos,
		       "roundtrip form_draft.sections 保真");
		check (round.find ("\"mood\":\"dark\"") != std::string::npos, "roundtrip form_draft.mood 保真");
		check (round.find ("\"scene_mode\":\"free_creation\"") != std::string::npos,
		       "roundtrip scene_mode");
		check (round.find ("j-1") != std::string::npos, "roundtrip pending_requests 保真");
		check (round.find ("\"job_id\":\"j-0\"") != std::string::npos &&
		           round.find ("\"artifact_paths\":[\"/tmp/a.wav\",\"/tmp/b.wav\"]") != std::string::npos,
		       "roundtrip job_history 保真");
		check (round.find ("\"take_id\":\"t-0\"") != std::string::npos,
		       "roundtrip audition_buffer_ref 保真");

		// controller setComponentState 镜像
		MemStream ctrlStream (draftChunk);
		check (controller->setComponentState (&ctrlStream) == kResultOk, "controller setComponentState");
		check (controller->getParamNormalized (kParamSceneMode) == 0.5,
		       "controller SceneMode mirror = 0.5 (free_creation)");

		// 未知 schema 负探针：fail-closed 取默认
		MemStream badStream ("{\"schema_version\":\"vit.genshell.state.vX\",\"scene_mode\":\"whatever\"}");
		check (component->setState (&badStream) == kResultOk, "setState unknown schema no-crash");
		MemStream afterBad;
		component->getState (&afterBad);
		const auto afterBadText = afterBad.text ();
		check (afterBadText.find ("\"selected_template\":\"song_segment_v1\"") != std::string::npos,
		       "unknown schema → defaults kept (fail-closed)");
	}

	//-------------------------------------------------------------
	// D. 编辑器对象创建不崩（不 attach）
	//-------------------------------------------------------------
	{
		IPlugView* view = controller->createView (ViewType::kEditor);
		check (view != nullptr, "createView(editor) returns view");
		if (view != nullptr)
		{
			ViewRect r {};
			check (view->getSize (&r) == kResultOk && r.getWidth () == 760 && r.getHeight () == 520,
			       "view size 760x520");
			view->release ();
		}
	}

	//-------------------------------------------------------------
	// 音频处理路径冒烟（静音输出 + T0 上下文缓存不崩）
	//-------------------------------------------------------------
	{
		FUnknownPtr<IAudioProcessor> audio (component);
		check (audio != nullptr, "processor exposes IAudioProcessor");
		if (audio)
		{
			ProcessSetup setup {kRealtime, kSample32, 48000, 512};
			check (audio->setupProcessing (setup) == kResultOk, "setupProcessing 48k/512");
			check (component->setActive (true) == kResultOk, "setActive(true)");
			float left[512] = {}, right[512] = {};
			Sample32* chans[2] = {left, right};
			AudioBusBuffers outBuf {};
			outBuf.channelBuffers32 = chans;
			outBuf.numChannels = 2;
			ProcessContext ctx {};
			ctx.tempo = 96.;
			ctx.state = ProcessContext::kTempoValid | ProcessContext::kPlaying | ProcessContext::kTimeSigValid;
			ctx.timeSigNumerator = 3;
			ctx.timeSigDenominator = 4;
			ProcessData data {};
			data.numSamples = 512;
			data.numOutputs = 1;
			data.outputs = &outBuf;
			data.processContext = &ctx;
			check (audio->process (data) == kResultOk, "process() 512 samples ok");
			bool silent = true;
			for (int i = 0; i < 512; ++i)
				if (left[i] != 0.f || right[i] != 0.f)
					silent = false;
			check (silent, "instrument skeleton silent output");
			component->setActive (false);
		}
	}

	controller->terminate ();
	controller->release ();
	component->terminate ();
	component->release ();

	printf ("%s: %d failures\n", gFailures == 0 ? "SELFTEST GREEN" : "SELFTEST RED", gFailures);
	return gFailures == 0 ? 0 : 1;
}

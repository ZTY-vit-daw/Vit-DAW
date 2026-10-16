//------------------------------------------------------------------------
// GenerationShellService — GENSHELL-HOST-1（件五卡2 / 件一 §2 降级 A / 件二 §4）
//
// 宿主侧生成壳服务：
//   · 壳实例注册表（edit plugin cache 扫描，identifier 识别 GenShell 实体）
//   · 提交队列（submit → job upsert queued + ghost 置位；cancel → cancel_requested）
//   · gen_shell.pending 读命令（jobs + providers；providers v1 恒空——内核无
//     genapi provider 知识，agent runner 天然自带目录）
//   · 下行：job_progress / job_terminal 走 T1 参数镜像（JobState/JobProgress/
//     JobErrorClass 经 te::AutomatableParameter 写入，与 set_plugin_param 同径）；
//     context_snapshot / provider_catalog / audition_ready 的壳侧消费者在卡 4
//     （GUI），v1 由服务状态缓存承载（GUI 500ms 进程内轮询，件一 §4）。
//
// T2 传输结论（主选核实，件一 §2）：JUCE 宿主在装载期把壳的 processor↔controller
// 两个 IConnectionPoint 互联（juce_VST3PluginFormatImpl.h
// interconnectComponentAndController），SDK ComponentBase::connect 单 peer 且
// fail-closed（已有 peer 再 connect 返回 kResultFalse，vstcomponentbase.cpp）；
// 活体 IEditController* 是 JUCE 包装层私有成员、公开面无访问器，独立类壳的
// IComponent::queryInterface(IEditController) 不成立（JUCE 自身 fetchController
// 的工厂回退路径佐证）。→ 主选不通，启用降级 A（参数面+chunk 组合编码）。
// 队列语义不降：submit/cancel/pending/JobState 镜像/ghost 断言与主选同口径。
// 每实例注册时执行只读探针并把结果记入注册表（烟测 debug 面回执核实证据）。
//------------------------------------------------------------------------
#include "GenerationShellService.h"

#include "../Core/VitAIGCJobRuntime.h"
#include "../Core/VitAsyncGhostPolicy.h"
#include "../Core/VitPaths.h"

// VST3 接口头（header-only 使用；iid 经 INLINE_UID_OF 取 constexpr TUID，
// 不引入 coreiids.cpp 链接依赖——JUCE 模块内已编译副本不跨模块导出复用）
#include "pluginterfaces/base/funknown.h"
#include "pluginterfaces/vst/ivstcomponent.h"
#include "pluginterfaces/vst/ivsteditcontroller.h"
#include "pluginterfaces/vst/ivstmessage.h"

#include <map>
#include <vector>

namespace vit
{

namespace
{

// —— 契约常量（coord/runs/GEN-PLUGIN-SHELL-DESIGN-1 件一 §4/§5、件二 §2；与
// GenShell/source/genshell_cids.h 同源，内核侧不引壳体头）——
constexpr const char* kMsgSchema = "vit.genshell.msg.v1";
constexpr const char* kRequestSchema = "vit.genshell.request.v1";
constexpr const char* kShellIdentifierToken = "GenShell";
constexpr const char* kMsgAttr = "data";
constexpr const char* kJobSourcePrefix = "gen_shell:";
constexpr const char* kStateSyncMessageId = "genshell_state_sync";

// JobState 8 值枚举（件一 §3；壳侧 kJobStateNames 同序）
int jobStateIndex (const juce::String& state)
{
    static const juce::String names[] = { "idle", "queued", "running", "downloading",
                                          "landing", "done", "failed", "cancelled" };
    for (int i = 0; i < 8; ++i)
        if (state == names[i])
            return i;
    return -1; // cancel_requested / interrupted 等记录态无参数枚举值——不镜像
}

bool isTerminalJobState (const juce::String& state)
{
    return state == "done" || state == "failed" || state == "cancelled" || state == "interrupted";
}

juce::StringArray jobErrorClassNames()
{
    return { "none", "config_error", "auth_error", "quota_error", "rate_limit", "not_found",
             "provider_error", "moderation_error", "timeout", "network_error",
             "response_parse_error", "unsupported_format", "landing_error", "context_error",
             "interrupted" };
}

juce::String resultToString (Steinberg::tresult r)
{
    switch (r)
    {
        case Steinberg::kResultOk: return "kResultOk";
        case Steinberg::kResultFalse: return "kResultFalse";
        case Steinberg::kNoInterface: return "kNoInterface";
        case Steinberg::kInvalidArgument: return "kInvalidArgument";
        default: break;
    }
    return "tresult_" + juce::String (static_cast<int> (r));
}

juce::String makeStatusReply (const juce::String& status, const juce::String& message)
{
    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", status);
    response->setProperty ("message", message);
    return juce::JSON::toString (juce::var (response.release()));
}

juce::String makeErrorReply (const juce::String& message)
{
    return makeStatusReply ("error", message);
}

// 极简 SHA-256（FIPS 180-4；VitApp 的 JuceHeader 不含 juce_cryptography——
// 件二 §1 requestHash=sha256 契约自含实现，零新模块依赖）
juce::String sha256Hex (const juce::String& text)
{
    struct Sha256
    {
        uint32_t h[8] = { 0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
                          0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19 };
        static uint32_t rotr (uint32_t x, int n) { return (x >> n) | (x << (32 - n)); }
        void process (const uint8_t* p)
        {
            static const uint32_t k[64] = {
                0x428a2f98,0x71374491,0xb5c0fbcf,0xe9b5dba5,0x3956c25b,0x59f111f1,0x923f82a4,0xab1c5ed5,
                0xd807aa98,0x12835b01,0x243185be,0x550c7dc3,0x72be5d74,0x80deb1fe,0x9bdc06a7,0xc19bf174,
                0xe49b69c1,0xefbe4786,0x0fc19dc6,0x240ca1cc,0x2de92c6f,0x4a7484aa,0x5cb0a9dc,0x76f988da,
                0x983e5152,0xa831c66d,0xb00327c8,0xbf597fc7,0xc6e00bf3,0xd5a79147,0x06ca6351,0x14292967,
                0x27b70a85,0x2e1b2138,0x4d2c6dfc,0x53380d13,0x650a7354,0x766a0abb,0x81c2c92e,0x92722c85,
                0xa2bfe8a1,0xa81a664b,0xc24b8b70,0xc76c51a3,0xd192e819,0xd6990624,0xf40e3585,0x106aa070,
                0x19a4c116,0x1e376c08,0x2748774c,0x34b0bcb5,0x391c0cb3,0x4ed8aa4a,0x5b9cca4f,0x682e6ff3,
                0x748f82ee,0x78a5636f,0x84c87814,0x8cc70208,0x90befffa,0xa4506ceb,0xbef9a3f7,0xc67178f2 };
            uint32_t w[64];
            for (int i = 0; i < 16; ++i)
                w[i] = (uint32_t (p[i * 4]) << 24) | (uint32_t (p[i * 4 + 1]) << 16)
                     | (uint32_t (p[i * 4 + 2]) << 8) | uint32_t (p[i * 4 + 3]);
            for (int i = 16; i < 64; ++i)
            {
                const auto s0 = rotr (w[i - 15], 7) ^ rotr (w[i - 15], 18) ^ (w[i - 15] >> 3);
                const auto s1 = rotr (w[i - 2], 17) ^ rotr (w[i - 2], 19) ^ (w[i - 2] >> 10);
                w[i] = w[i - 16] + s0 + w[i - 7] + s1;
            }
            uint32_t a = h[0], b = h[1], c = h[2], d = h[3];
            uint32_t e = h[4], f = h[5], g = h[6], hh = h[7];
            for (int i = 0; i < 64; ++i)
            {
                const auto S1 = rotr (e, 6) ^ rotr (e, 11) ^ rotr (e, 25);
                const auto ch = (e & f) ^ (~e & g);
                const auto t1 = hh + S1 + ch + k[i] + w[i];
                const auto S0 = rotr (a, 2) ^ rotr (a, 13) ^ rotr (a, 22);
                const auto maj = (a & b) ^ (a & c) ^ (b & c);
                const auto t2 = S0 + maj;
                hh = g; g = f; f = e; e = d + t1; d = c; c = b; b = a; a = t1 + t2;
            }
            h[0] += a; h[1] += b; h[2] += c; h[3] += d;
            h[4] += e; h[5] += f; h[6] += g; h[7] += hh;
        }
    };
    Sha256 sha;
    const auto utf8 = text.toUTF8();
    const auto total = utf8.sizeInBytes();
    const uint64_t bitLen = uint64_t (total) * 8;
    size_t offset = 0;
    while (total - offset >= 64)
    {
        sha.process (reinterpret_cast<const uint8_t*> (utf8.getAddress() + offset));
        offset += 64;
    }
    uint8_t block[64] = {};
    const auto rem = total - offset;
    std::memcpy (block, utf8.getAddress() + offset, rem);
    block[rem] = 0x80;
    if (rem >= 56)
    {
        sha.process (block);
        std::memset (block, 0, 56);
    }
    for (int i = 0; i < 8; ++i)
        block[56 + i] = uint8_t (bitLen >> (56 - i * 8));
    sha.process (block);
    juce::String hex;
    for (const auto word : sha.h)
        hex += juce::String::toHexString (int (word)).paddedLeft ('0', 8);
    return hex.toLowerCase();
}

// 记录字段的完整搬运（upsert 是整记录替换——与 aigc_register_job 同语义，
// 推进/取消时不可丢字段）
VitAIGCJobRecord recordFromJobVar (const juce::DynamicObject& job, const juce::String& stateOverride)
{
    VitAIGCJobRecord record;
    record.jobId = job.getProperty ("job_id").toString();
    record.nodeId = job.getProperty ("node_id").toString();
    record.jobState = stateOverride.isNotEmpty() ? stateOverride : job.getProperty ("job_state").toString();
    record.ghostState = job.getProperty ("ghost_state").toString();
    record.statusMessage = job.getProperty ("status_message").toString();
    record.requestHash = job.getProperty ("request_hash").toString();
    record.source = job.getProperty ("source").toString();
    record.assetRef = job.getProperty ("asset_ref").toString();
    record.trackId = job.getProperty ("track_id").toString();
    record.clipId = job.getProperty ("clip_id").toString();
    return record;
}

} // namespace

//------------------------------------------------------------------------
// T2 host 端点。降级 A 下仅作探针 connect 目标与未来厂商壳上行入口；当前
// GenShell 的两个端点被 JUCE 互联占用，本端点不会收到活连接。notify 解析
// vit.genshell.msg.v1 信封（submit_request/cancel_request 入队）；messageID=
// genshell_state_sync（壳内部 processor←controller 同步消息，卡1 回执事实④）
// 与未知 schema/messageID 一律丢弃+计数披露，不静默不崩。
//------------------------------------------------------------------------
struct GenerationShellService::Impl
{
    class HostEndpoint final : public Steinberg::Vst::IConnectionPoint
    {
    public:
        DECLARE_FUNKNOWN_METHODS
        HostEndpoint() { FUNKNOWN_CTOR }
        ~HostEndpoint() { FUNKNOWN_DTOR }

        Steinberg::tresult PLUGIN_API connect (Steinberg::Vst::IConnectionPoint* other) override
        {
            peer = other;
            return Steinberg::kResultOk;
        }

        Steinberg::tresult PLUGIN_API disconnect (Steinberg::Vst::IConnectionPoint* other) override
        {
            if (other != nullptr && peer == other)
                peer = nullptr;
            return Steinberg::kResultOk;
        }

        Steinberg::tresult PLUGIN_API notify (Steinberg::Vst::IMessage* message) override;

        // 引用计数成员（addRef/release 出线定义——DECLARE 宏已声明虚函数）。
        // 端点为 Impl 成员对象、connect 不 addRef（SDK 行为），v1 无外部引用方。
        std::atomic<Steinberg::uint32> refCount { 1 };

        Impl* owner = nullptr; // 端点是 Impl 成员——生命周期嵌套，回指针安全
        Steinberg::Vst::IConnectionPoint* peer = nullptr;
    };

    struct ShellEntry
    {
        juce::String pluginItemId; // rack 节点 id（EditItemID 串；件二 §2 shell_node_id）
        juce::String identifier;   // e.g. VST3-GenShell-…
        juce::String pluginName;
        te::ExternalPlugin* ext = nullptr;             // 借用（plugin cache 持有）
        juce::AudioPluginInstance* instance = nullptr; // 借用（同上）
        bool instanceReady = false;
        // 主选核实探针（每实例一次，只读；connect 尝试失败即证占用，无状态残留）
        juce::String probeQiEditController = "not_probed";
        juce::String probeQiConnectionPoint = "not_probed";
        juce::String probeConnectAttempt = "not_probed";
        // T1 镜像最后写入（debug 证据）
        juce::String lastJobStateMirror = "none";
        float lastJobProgressMirror = 0.0f;
        juce::String lastJobErrorClassMirror = "none";
    };

    struct DownlinkCounters
    {
        int jobProgress = 0;
        int jobTerminal = 0;
        int contextSnapshot = 0;
        int providerCatalog = 0;
        int auditionReady = 0;
    };

    EditGetter getEdit;
    CurrentProjectPathGetter getCurrentProjectPath;
    HostEndpoint hostEndpoint;

    juce::CriticalSection lock;
    std::vector<ShellEntry> shells;
    std::map<juce::String, juce::String> requestPayloads; // jobId → request JSON
    std::map<juce::String, juce::String> lastSeenStates;  // jobId → job_state
    DownlinkCounters downlink;
    int unknownMessageDrops = 0;

    Impl()
    {
        hostEndpoint.owner = this;
    }

    juce::File effectiveProjectFile() const
    {
        if (getCurrentProjectPath != nullptr)
        {
            const auto currentPath = getCurrentProjectPath();
            if (currentPath.isNotEmpty())
                return juce::File (currentPath);
        }
        return paths::ensureDefaultProjectXmlFileExists();
    }

    const ShellEntry* findShellByNode (const juce::String& nodeId) const
    {
        for (const auto& shell : shells)
            if (shell.pluginItemId == nodeId)
                return &shell;
        return nullptr;
    }

    // T1 镜像写入（降级 A 的 job_progress/job_terminal 下行载体）。
    // 与 set_plugin_param 同写径：te::AutomatableParameter::setNormalisedParameter。
    void mirrorParam (ShellEntry& shell, const juce::String& paramName, float normalized)
    {
        if (shell.ext == nullptr)
            return;
        for (auto* param : shell.ext->getAutomatableParameters())
        {
            if (param == nullptr || param->getParameterName() != paramName)
                continue;
            param->setNormalisedParameter (juce::jlimit (0.0f, 1.0f, normalized), juce::sendNotification);
            break;
        }
        shell.ext->flushPluginStateToValueTree();
    }

    void mirrorJobProgress (ShellEntry& shell, const juce::String& state,
                            const juce::String& statusMessage, double progress)
    {
        const auto stateIdx = jobStateIndex (state);
        if (stateIdx >= 0)
        {
            mirrorParam (shell, "JobState", static_cast<float> (stateIdx) / 7.0f);
            shell.lastJobStateMirror = state;
        }
        mirrorParam (shell, "JobProgress", static_cast<float> (juce::jlimit (0.0, 1.0, progress)));
        shell.lastJobProgressMirror = static_cast<float> (juce::jlimit (0.0, 1.0, progress));
        // JobErrorClass：终态 failed 从 status_message 的 class= 前缀解析（件二 §3）
        if (state == "failed")
        {
            const auto classes = jobErrorClassNames();
            const auto token = statusMessage.fromFirstOccurrenceOf ("class=", false, false)
                                         .upToFirstOccurrenceOf (" ", false, false)
                                         .trim();
            const auto idx = classes.indexOf (token);
            shell.lastJobErrorClassMirror = idx >= 0 ? classes[idx] : juce::String ("none");
            mirrorParam (shell, "JobErrorClass", static_cast<float> (idx >= 0 ? idx : 0) / 14.0f);
        }
        ++downlink.jobProgress;
    }

    // 主选核实探针：component 直取 IEditController（主选第一步）+ processor 端点
    // 占用核实（单 peer fail-closed——被 JUCE 互联占用时期望 kResultFalse）
    void probeShell (ShellEntry& shell)
    {
        if (shell.instance == nullptr)
            return;
        JUCE_BEGIN_IGNORE_WARNINGS_GCC_LIKE ("-Wdeprecated-declarations")
        auto* component = static_cast<Steinberg::Vst::IComponent*> (shell.instance->getPlatformSpecificData());
        JUCE_END_IGNORE_WARNINGS_GCC_LIKE
        if (component == nullptr)
        {
            shell.probeQiEditController = "no_platform_data";
            return;
        }
        {
            void* ptr = nullptr;
            const auto r = component->queryInterface (INLINE_UID_OF (Steinberg::Vst::IEditController), &ptr);
            shell.probeQiEditController = resultToString (r);
            if (r == Steinberg::kResultOk && ptr != nullptr)
                static_cast<Steinberg::FUnknown*> (ptr)->release();
        }
        {
            void* ptr = nullptr;
            const auto r = component->queryInterface (INLINE_UID_OF (Steinberg::Vst::IConnectionPoint), &ptr);
            shell.probeQiConnectionPoint = resultToString (r);
            if (r == Steinberg::kResultOk && ptr != nullptr)
            {
                auto* cp = static_cast<Steinberg::Vst::IConnectionPoint*> (ptr);
                shell.probeConnectAttempt = resultToString (cp->connect (&hostEndpoint));
                cp->disconnect (&hostEndpoint);
                cp->release();
            }
        }
    }

    void refreshRegistry()
    {
        auto* edit = getEdit != nullptr ? getEdit() : nullptr;
        if (edit == nullptr)
            return;

        std::vector<ShellEntry> next;
        for (auto* plugin : edit->getPluginCache().getPlugins())
        {
            if (plugin == nullptr)
                continue;
            auto* ext = dynamic_cast<te::ExternalPlugin*> (plugin);
            if (ext == nullptr)
                continue;
            if (! ext->getIdentifierString().contains (kShellIdentifierToken))
                continue;

            ShellEntry entry;
            entry.pluginItemId = plugin->itemID.toString();
            entry.identifier = ext->getIdentifierString();
            entry.pluginName = plugin->getName();
            entry.ext = ext;
            entry.instance = ext->getAudioPluginInstance();
            entry.instanceReady = entry.instance != nullptr;
            if (const auto* prev = findShellByNode (entry.pluginItemId))
            {
                entry.probeQiEditController = prev->probeQiEditController;
                entry.probeQiConnectionPoint = prev->probeQiConnectionPoint;
                entry.probeConnectAttempt = prev->probeConnectAttempt;
                entry.lastJobStateMirror = prev->lastJobStateMirror;
                entry.lastJobProgressMirror = prev->lastJobProgressMirror;
                entry.lastJobErrorClassMirror = prev->lastJobErrorClassMirror;
            }
            else if (entry.instanceReady)
            {
                probeShell (entry);
            }
            next.push_back (std::move (entry));
        }
        shells = std::move (next);
    }

    // 降级 A 上行 scaffold：壳 chunk 的 pending_requests 轮询（件一 §5——壳侧
    // 写入自有 state，宿主轮询拉取）。骨架壳无写入口（卡 4 GUI 接线），本路径
    // 当前恒空，先于消费者就位。
    void pollShellPendingRequests()
    {
        auto* edit = getEdit != nullptr ? getEdit() : nullptr;
        if (edit == nullptr)
            return;
        for (auto& shell : shells)
        {
            if (shell.instance == nullptr)
                continue;
            juce::MemoryBlock chunk;
            shell.instance->getStateInformation (chunk);
            const auto parsed = juce::JSON::parse (chunk.toString());
            const auto* pending = parsed.getProperty ("pending_requests", juce::var()).getArray();
            if (pending == nullptr)
                continue;
            for (const auto& item : *pending)
            {
                const auto jobId = item.getDynamicObject() != nullptr
                                       ? item.getDynamicObject()->getProperty ("job_id").toString().trim()
                                       : juce::String();
                if (jobId.isEmpty() || requestPayloads.count (jobId) != 0
                    || lastSeenStates.count (jobId) != 0)
                    continue;
                juce::String error;
                submitRequest (juce::JSON::toString (item, true), shell.pluginItemId, error);
            }
        }
    }

    // job 记录差异 → 下行镜像 + 终态 ghost 清除（件二 §1：failed/cancelled 清除）
    void refreshDownlinks()
    {
        const auto projectFile = effectiveProjectFile();
        for (const auto& jobVar : VitAIGCJobRuntime::snapshotProjectJobs (projectFile))
        {
            auto* job = jobVar.getDynamicObject();
            if (job == nullptr)
                continue;
            const auto source = job->getProperty ("source").toString();
            if (! source.startsWith (kJobSourcePrefix))
                continue;
            const auto jobId = job->getProperty ("job_id").toString();
            const auto state = job->getProperty ("job_state").toString();
            const auto statusMessage = job->getProperty ("status_message").toString();
            const auto it = lastSeenStates.find (jobId);
            if (it != lastSeenStates.end() && it->second == state)
                continue;
            lastSeenStates[jobId] = state;

            if (auto* shell = const_cast<ShellEntry*> (findShellByNode (job->getProperty ("node_id").toString())))
                mirrorJobProgress (*shell, state, statusMessage, isTerminalJobState (state) ? 1.0 : 0.5);

            if (! isTerminalJobState (state))
                continue;
            ++downlink.jobTerminal;
            if ((state == "failed" || state == "cancelled")
                && job->getProperty ("ghost_state").toString() == "mute")
            {
                auto record = recordFromJobVar (*job, {});
                record.ghostState = VitAsyncGhostPolicy::normalise ("bypass"); // ghost 清除
                VitAIGCJobRuntime::upsertJob (projectFile, record);
            }
        }
    }

    void refresh()
    {
        const juce::ScopedLock sl (lock);
        refreshRegistry();
        pollShellPendingRequests();
        refreshDownlinks();
    }

    // 提交入队（生产=T2/降级 A 上行；烟测=debug_submit 测试钩子走同一路径）
    juce::String submitRequest (const juce::String& requestJson,
                                const juce::String& hostBoundNodeId,
                                juce::String& error)
    {
        const auto parsed = juce::JSON::parse (requestJson);
        auto* request = parsed.getDynamicObject();
        if (request == nullptr)
        {
            error = "submit_request payload is not a JSON object";
            return {};
        }
        if (request->getProperty ("schema_version").toString() != kRequestSchema)
        {
            error = "submit_request schema_version must be " + juce::String (kRequestSchema);
            return {};
        }
        auto jobId = request->getProperty ("job_id").toString().trim();
        if (jobId.isEmpty())
            jobId = juce::Uuid().toString(); // 契约=壳侧生成；测试钩子缺省时宿主代生成
        request->setProperty ("job_id", jobId);

        auto nodeId = request->getProperty ("shell_node_id").toString().trim();
        if (nodeId.isEmpty())
            nodeId = hostBoundNodeId; // 宿主回填（件二 §2）
        request->setProperty ("shell_node_id", nodeId);

        if (findShellByNode (nodeId) == nullptr)
        {
            error = "shell_node_id does not match a loaded GenShell instance: " + nodeId;
            return {};
        }
        if (requestPayloads.count (jobId) != 0 || lastSeenStates.count (jobId) != 0)
        {
            error = "job_id already submitted: " + jobId;
            return {};
        }

        const auto canonical = juce::JSON::toString (parsed, true);
        VitAIGCJobRecord record;
        record.jobId = jobId;
        record.nodeId = nodeId;
        record.jobState = "queued";
        record.ghostState = VitAsyncGhostPolicy::normalise ("mute"); // ghost 置位（件二 §1）
        record.statusMessage = "class=none queued via generation shell service (transport=downgrade_a)";
        record.requestHash = sha256Hex (canonical);
        record.source = juce::String (kJobSourcePrefix) + nodeId;
        record.trackId = request->getProperty ("track_id").toString().trim();
        VitAIGCJobRuntime::upsertJob (effectiveProjectFile(), record);

        requestPayloads[jobId] = canonical;
        lastSeenStates[jobId] = "queued";
        if (auto* shell = const_cast<ShellEntry*> (findShellByNode (nodeId)))
            mirrorJobProgress (*shell, "queued", record.statusMessage, 0.0);
        return jobId;
    }

    juce::String cancelRequest (const juce::String& jobId, juce::String& error)
    {
        if (jobId.isEmpty())
        {
            error = "cancel_request requires a non-empty job_id";
            return {};
        }
        const auto projectFile = effectiveProjectFile();
        for (const auto& jobVar : VitAIGCJobRuntime::snapshotProjectJobs (projectFile))
        {
            auto* job = jobVar.getDynamicObject();
            if (job == nullptr || job->getProperty ("job_id").toString() != jobId)
                continue;
            if (! job->getProperty ("source").toString().startsWith (kJobSourcePrefix))
            {
                error = "job is not a generation shell job: " + jobId;
                return {};
            }
            if (isTerminalJobState (job->getProperty ("job_state").toString()))
            {
                error = "job already terminal, cancel ignored: " + jobId;
                return {};
            }
            auto record = recordFromJobVar (*job, "cancel_requested");
            VitAIGCJobRuntime::upsertJob (projectFile, record);
            lastSeenStates[jobId] = "cancel_requested";
            return jobId;
        }
        error = "job not found: " + jobId;
        return {};
    }

    // gen_shell.pending 应答（件二 §4 结构：jobs+providers）
    void appendPendingAnswer (juce::DynamicObject& response)
    {
        const auto projectFile = effectiveProjectFile();
        // runner 应当作用的作业=非终态；按 job_id 排序保证确定性。
        // map 存 var（持有 DynamicObject 引用）——不能存裸指针：range-for 的
        // 临时返回数组在循环末析构，裸指针即悬垂（首跑 state="" 的根因）。
        std::map<juce::String, juce::var> pending;
        for (const auto& jobVar : VitAIGCJobRuntime::snapshotProjectJobs (projectFile))
        {
            auto* job = jobVar.getDynamicObject();
            if (job == nullptr)
                continue;
            const auto state = job->getProperty ("job_state").toString();
            if (! job->getProperty ("source").toString().startsWith (kJobSourcePrefix)
                || isTerminalJobState (state))
                continue;
            pending.emplace (job->getProperty ("job_id").toString(), jobVar);
        }
        juce::Array<juce::var> jobsArray;
        for (const auto& [jobId, jobVar] : pending)
        {
            auto* job = jobVar.getDynamicObject();
            auto entry = std::make_unique<juce::DynamicObject>();
            entry->setProperty ("job_id", jobId);
            const auto state = job->getProperty ("job_state").toString();
            entry->setProperty ("state", state);
            entry->setProperty ("cancel_requested", state == "cancel_requested");
            const auto it = requestPayloads.find (jobId);
            if (it != requestPayloads.end())
                entry->setProperty ("request", juce::JSON::parse (it->second));
            jobsArray.add (juce::var (entry.release()));
        }
        response.setProperty ("jobs", juce::var (jobsArray));
        response.setProperty ("providers", juce::var (juce::Array<juce::var>()));
    }

    // 烟测 debug 面（生产载荷不带 debug_shell_state 即不出现）
    void appendShellDebugSection (juce::DynamicObject& response)
    {
        juce::Array<juce::var> shellsArray;
        for (const auto& shell : shells)
        {
            auto entry = std::make_unique<juce::DynamicObject>();
            entry->setProperty ("plugin_id", shell.pluginItemId);
            entry->setProperty ("identifier", shell.identifier);
            entry->setProperty ("plugin_name", shell.pluginName);
            entry->setProperty ("instance_ready", shell.instanceReady);
            entry->setProperty ("t2_transport", "downgrade_a_param_chunk");
            entry->setProperty ("probe_qi_edit_controller", shell.probeQiEditController);
            entry->setProperty ("probe_qi_connection_point", shell.probeQiConnectionPoint);
            entry->setProperty ("probe_connect_attempt", shell.probeConnectAttempt);
            entry->setProperty ("last_job_state_mirror", shell.lastJobStateMirror);
            entry->setProperty ("last_job_progress_mirror", shell.lastJobProgressMirror);
            entry->setProperty ("last_job_error_class_mirror", shell.lastJobErrorClassMirror);
            shellsArray.add (juce::var (entry.release()));
        }
        response.setProperty ("shells", juce::var (shellsArray));

        auto counters = std::make_unique<juce::DynamicObject>();
        counters->setProperty ("job_progress", downlink.jobProgress);
        counters->setProperty ("job_terminal", downlink.jobTerminal);
        counters->setProperty ("context_snapshot", downlink.contextSnapshot);
        counters->setProperty ("provider_catalog", downlink.providerCatalog);
        counters->setProperty ("audition_ready", downlink.auditionReady);
        response.setProperty ("downlink_counters", juce::var (counters.release()));
        response.setProperty ("unknown_message_drops", unknownMessageDrops);
    }

    // notify 上行入口（T2 信封）——解析在锁外，落地走与测试钩子同一条内部路径
    void handleUplinkEnvelopeJson (const juce::String& jsonText)
    {
        const auto parsed = juce::JSON::parse (jsonText);
        auto* envelope = parsed.getDynamicObject();
        if (envelope == nullptr || envelope->getProperty ("schema_version").toString() != kMsgSchema)
        {
            dropUnknownMessage(); // 丢弃+计数披露（件一 §4 fail-closed）
            return;
        }
        const auto type = envelope->getProperty ("type").toString();
        const auto payloadVar = envelope->getProperty ("payload");
        if (type == "submit_request" && payloadVar.isObject())
        {
            const juce::ScopedLock sl (lock);
            juce::String error;
            submitRequest (juce::JSON::toString (payloadVar, true), {}, error);
        }
        else if (type == "cancel_request" && payloadVar.isObject())
        {
            const juce::ScopedLock sl (lock);
            juce::String error;
            cancelRequest (payloadVar.getDynamicObject()->getProperty ("job_id").toString().trim(), error);
        }
        else
        {
            dropUnknownMessage();
        }
    }

    void dropUnknownMessage()
    {
        const juce::ScopedLock sl (lock);
        ++unknownMessageDrops;
    }
};

Steinberg::uint32 PLUGIN_API GenerationShellService::Impl::HostEndpoint::addRef() { return ++refCount; }
Steinberg::uint32 PLUGIN_API GenerationShellService::Impl::HostEndpoint::release() { return --refCount; }

Steinberg::tresult PLUGIN_API GenerationShellService::Impl::HostEndpoint::queryInterface (const Steinberg::TUID iid,
                                                                                         void** obj)
{
    QUERY_INTERFACE (iid, obj, Steinberg::FUnknown::iid, Steinberg::FUnknown)
    QUERY_INTERFACE (iid, obj, Steinberg::Vst::IConnectionPoint::iid, Steinberg::Vst::IConnectionPoint)
    *obj = nullptr;
    return Steinberg::kNoInterface;
}

//------------------------------------------------------------------------
// HostEndpoint::notify —— T2 上行信封（降级 A 下当前无活连接；厂商壳/未来接入时生效）
//------------------------------------------------------------------------
Steinberg::tresult PLUGIN_API GenerationShellService::Impl::HostEndpoint::notify (Steinberg::Vst::IMessage* message)
{
    if (message == nullptr)
        return Steinberg::kInvalidArgument;
    if (owner == nullptr)
        return Steinberg::kResultOk;
    const char* messageId = message->getMessageID();
    // 壳内部 processor←controller 同步消息：按未知 messageID 丢弃（卡1 回执事实④）
    if (messageId == nullptr || juce::String (messageId) == juce::String (kStateSyncMessageId))
    {
        owner->dropUnknownMessage();
        return Steinberg::kResultOk;
    }
    if (auto* attrs = message->getAttributes ())
    {
        Steinberg::Vst::TChar buf[65536];
        if (attrs->getString (kMsgAttr, buf, sizeof (buf)) == Steinberg::kResultTrue)
        {
            const auto text = juce::String (juce::CharPointer_UTF16 (reinterpret_cast<const juce::CharPointer_UTF16::CharType*> (buf)));
            owner->handleUplinkEnvelopeJson (text);
        }
    }
    return Steinberg::kResultOk;
}

//------------------------------------------------------------------------
GenerationShellService::GenerationShellService (EditGetter editGetter,
                                                CurrentProjectPathGetter currentProjectPathGetter)
    : impl (std::make_unique<Impl>())
{
    impl->getEdit = std::move (editGetter);
    impl->getCurrentProjectPath = std::move (currentProjectPathGetter);
}

GenerationShellService::~GenerationShellService() = default;

juce::String GenerationShellService::handleGenShellPending (const juce::DynamicObject& object,
                                                            const juce::String&)
{
    const juce::ScopedLock sl (impl->lock);

    // 注册表先行：debug_submit 需按 shell_node_id 绑定已装载壳实例（首调用即提交
    // 也要成立——钩子不假设此前已有过一次 gen_shell.pending 探测调用）
    impl->refreshRegistry();

    // 烟测测试钩子（卡面许可「经服务直调测试钩子伪提交/伪取消」；生产载荷不含）
    const auto debugSubmit = object.getProperty ("debug_submit");
    if (debugSubmit.isObject())
    {
        juce::String error;
        const auto jobId = impl->submitRequest (juce::JSON::toString (debugSubmit, true), {}, error);
        if (jobId.isEmpty())
            return makeErrorReply ("gen_shell.pending debug_submit rejected: " + error);
    }
    const auto debugCancel = object.getProperty ("debug_cancel");
    if (debugCancel.isObject())
    {
        juce::String error;
        const auto jobId = impl->cancelRequest (
            debugCancel.getDynamicObject()->getProperty ("job_id").toString().trim(), error);
        if (jobId.isEmpty())
            return makeErrorReply ("gen_shell.pending debug_cancel rejected: " + error);
    }

    impl->pollShellPendingRequests();
    impl->refreshDownlinks();

    auto response = std::make_unique<juce::DynamicObject>();
    response->setProperty ("status", "ok");
    impl->appendPendingAnswer (*response);
    if (static_cast<bool> (object.getProperty ("debug_shell_state")))
        impl->appendShellDebugSection (*response);
    return juce::JSON::toString (juce::var (response.release()));
}

} // namespace vit

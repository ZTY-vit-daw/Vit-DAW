# evidence.md — KERNEL-OFFRENDER-REPRO-1 (301) 离线渲染静音守卫复现

写于 2026-11-07 23:10（四腿跑完收口取证；判读对账 RUNBOOK §8 跑前口径）。

## 0. Run 身份与栈协议

| 腿 | run 目录 | 命令形态 | 退出码 | 栈 |
|---|---|---|---|---|
| M0 attempt-1 | `m0_20261107_2256/` | 301 ps1 + main agent 现场 | py exit 3（配置面中止） | 登记 6baa4646/释放 0fe253dd |
| M0 attempt-2 | `m0b_20261107_2301/` | 277 ps1/驱动器 verbatim + 277 agent 62F444B7 逐位复用 + **插桩内核** | **py exit 4 @render（seg3a_r2 复现）** | 登记 0fe253dd/释放 152a3368 |
| M2 | `m2_direct_grand_rhapsody_20261107_2305/` | 301 ps1 `-KernelOffrenderReproSmoke grand_rhapsody` | py exit 0=chain_complete（观测=静音） | 登记 152a3368/释放 ad9cb7fd |
| M3 | `m3_direct_surge_xt_20261107_2308/` | 同 M2 `surge_xt` | py exit 0=chain_complete（观测=**非静音**） | 登记 ad9cb7fd/释放随本卡收口笔 |

每腿：净空亲核→登记推 main→≤60s 复核→启动→PS1 throw/完成后 owner 亲拆→TEARDOWN_ALL_ZERO 亲核（22:58:35 / 23:03:44 / 23:06:43 / 23:09:47，六口 TCP 5555/5556/5557/7878/8787/18787+UDP 4444/4445+VitApp/VitAgent/vsphub/Godot/python 全零）。

## 1. 被测面（§9 记录）

- **kernel=插桩构建** `D:/Vit_DAW_wt_301repro/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe` SHA256 `CC9FDACC20A544E9E0F884409B9FD330F4B51BC4D71FD20B352E845ECFA6D886`（22:51:57，31,798,784 B）。构建：`cmake -S …/VitApp -B …/build_release -G "Visual Studio 18 2026" -DVIT_TRACKTION_ENGINE_DIR=D:/Vit_DAW/tracktion_engine` + `cmake --build --config Release --parallel`，exit 0；插桩串 `VIT301_GUARD !hasLoadedInstance early-return` 二进制在场亲证（wt_228 基线 0）。
- **源面唯一差异**=引擎树 25 行一次性留痕（§5）；VitApp 源 e10d5a6d（wt_228 基线）→origin/main 零 diff 亲证（`git diff --stat e10d5a6d origin/main -- VitApp/` 空）。
- wt_228 既有内核 `F9E3DA0A…ED0D67A`（mtime 2026-10-30 22:23）全程未动。
- M0a2 agent=wt_277impl `agent/bin/VitAgent.exe` SHA256 `62F444B7F1F817492DF96FE765CA12FF30CBF1450F116750F6928EFEAC2D28B3`＝seg3a_r2 被测 agent 逐位同款（§9 -SkipBuild 前提满足）；M2/M3 agent=wt_301repro 现场构建（main Go 零改动，纯内核 ZMQ 腿无需 277 能力面）。
- M0a1（配置面中止，如实记录）：误用 main 现场构建 agent——277 段 1 arrangement pack 装载能力披露仅存于 port/arr-srcload-1 未合入，模型诚实回「工具集不含插件装载入口」→ exit 3；attempt-2 换 277 面修正（298「三次调用如实记录」先例，非原样重跑）。

## 2. 守卫留痕插桩（域内最小修，落地=§5.1 留痕口径补齐）

- 位置：`D:/Vit_DAW/tracktion_engine/modules/tracktion_engine/plugins/external/tracktion_ExternalPlugin.cpp` `applyToBuffer`（297 E4 锚 1336-1342）两守卫，+25 行，**只加日志不改行为**。
- 形态：每守卫×（live/render 相位=fc.isRendering）各一个 `static std::atomic<bool>` 一次性 `exchange`——直播装载窗命中不烧掉离线渲染窗证据位；日志行含 `VIT301_GUARD`、守卫名、`getDebugName()`、`rendering=0/1`、ISO8601 时戳；出口 `TRACKTION_LOG`（=juce::Logger→ZmqLogger→`VitApp/Workspace/Logs/VitHeadlessServer*.log`，VitApp Main.cpp:91-97 亲读）。
- 管线有效性三重亲证：①内核日志含 `Rebuilding Wave Device List...`（=tracktion_DeviceManager.cpp:848 同 TRACKTION_LOG 宏）每 run 在场；②插桩串在新二进制、不在 wt_228；③M3 同管线出声腿全链健康。
- 引擎树插桩 diff 保留在位（engine repo b4397498 之上未提交，+25 行），处置（landing/回退）交主管；patch 全文见 §6。

## 3. 结果矩阵与判定

| 腿 | 挂载 | 插件 | 渲染产物 | VIT301_GUARD |
|---|---|---|---|---|
| M0a2（=seg3a_r2 逐字复刻） | rack+chat | Grand Rhapsody（WaveShell1-VST3 17.1） | 8.0s **数字静音**（1,411,200 字节 nonzero=0；**data sha256 `7a9c38df5318a026…`=seg3a_r2 逐位同款**） | **0 行** |
| M2 | 直挂 track.plugins | Grand Rhapsody | 8.0s **数字静音**（peak 0/rms 0，render_done 非 fallback） | **0 行** |
| M3 | 直挂 track.plugins | **Surge XT** | **非静音**：peak_abs=6823（-13.63 dBFS）/rms 1854（-24.95 dBFS），render_done 非 fallback | 0 行 |

链面全绿（三腿）：预检 17 instruments/fixture 单轨/instantiate 或 rack.add_node status=ok+可见性 matched_by=id/16 音 clip 回读一致/start_render→render_done。

### 判定（交主管裁定）

1. **297 候选②（守卫 fail-silent 早退）被实证排除**：两条静音腿（rack+直挂）中两守卫零命中——静音渲染发生时 `hasLoadedInstance`/`isInstancePrepared` 均为真，`applyToBuffer` 未在守卫处早退。留痕插桩已随本卡落地（§5.1 口径补齐），守卫面清白入卷。
2. **缺陷定性=插件特异性（WaveShell 类 VST3 离线渲染静音）**：同一插桩内核+同一驱动链+同一 fixture 形态，唯一变量插件（Grand Rhapsody/WaveShell1-VST3 vs Surge XT）→ 静音 vs 出声。297 报告候选②条目下的「次要保留」（WaveShell 类 VST3 在 setNonRealtime(true)+独立 prepareToPlay 下的插件特异行为）升格为**主定性**：兼容性缺陷方向（插件黑名单/预热策略），非物化缺陷方向。
3. **rack 结构与合路面出罪**：M2 直挂仍静音（非 rack 特有）；M3 同链出声（MIDI 馈入九环节+合路链运行时贯通，297 候选①/③运行时复活面亦被 M3 压制——同一图同一链 Surge XT 全通）。
4. M1 实时对照腿未跑（卡面可选；四腿已定论）：遗留问题=GR **实时播放**是否出声——出声=WaveShell 离线特异；不出声=GR 在本环境任何路径都哑（装载面另有隐情）。建议归修复/处置卡（transport play+电平 tap 探针），本卡不扩。

### 次级观察

- M3 summary 的 observation 模板串写死「specific to the rack/chat path」（该串按 M2 非静音假设预写）——实际判定以数值面为准（M2/M3 对照→插件特异性），驱动器模板串缺陷如实注记，不影响证据。
- M0a2 chat 回合 5 行 flash（含 semantic_entry 2/工具轮 3）+strip-silence 1 行；M2/M3 telemetry 文件未创建=零调用亲证。

## 4. 计费（telemetry 亲读）

- M0a1：3 行 deepseek-flash（strip-silence 1+chat 2）≪¥0.1。
- M0a2：6 行 deepseek-flash（strip-silence 1+chat 5；in 46,732/out 1,992/cache 25,472）≈¥0.05-0.1。
- M2/M3：**零 LLM**（零调用，文件未创建）。
- 合计 ≈¥0.1-0.2，预算 ≈¥1-3 内（RUNBOOK §8 申报兑现；「单 run 无重跑」按 M0a1→a2 配置修正如实分账，两轮均非原样重跑）。

## 5. 工件清单（随 port/kernel-offrender-repro-1 入库；wav 留盘 gitignore）

- `RUNBOOK.md`（§8 跑前口径）+本 `evidence.md`。
- `m0_20261107_2256/`：summary+chat_response（attempt-1 配置面中止件）。
- `m0b_20261107_2301/`：ps1/arr console+summary+chat_response+guard_trace_count.txt（=0）+kernel_log 4.5MB 全量+logs/（telemetry 6 行+prompt_render）+wav/.vit 留盘。
- `m2_direct_grand_rhapsody_20261107_2305/`、`m3_direct_surge_xt_20261107_2308/`：console+summary（数值面+guard_trace）+guard_trace 工件+kernel_log 1.5MB+wav/.vit 留盘。
- 内核日志原始位：`D:/Vit_DAW_wt_301repro/VitApp/Workspace/Logs/VitHeadlessServer*.log`（四腿四文件，拷贝入卷）。

## 6. 引擎树插桩 patch 全文（tracktion_engine repo，未提交，处置交主管）

```diff
--- a/modules/tracktion_engine/plugins/external/tracktion_ExternalPlugin.cpp
+++ b/modules/tracktion_engine/plugins/external/tracktion_ExternalPlugin.cpp
@@ -1335,11 +1335,36 @@ void ExternalPlugin::prepareIncomingMidiMessages (MidiMessageArray& incoming, in

 void ExternalPlugin::applyToBuffer (const PluginRenderContext& fc)
 {
+    // VIT-301 (KERNEL-OFFRENDER-REPRO-1): one-shot guard traces, log-only, no behavior change.
+    // One shot per (guard x rendering phase): a live-load-window hit must not consume the
+    // offline-render-phase evidence slot.
     if (! hasLoadedInstance.load (std::memory_order_acquire))
+    {
+        static std::atomic<bool> vit301TraceLive { false }, vit301TraceRender { false };
+        auto& vit301Traced = fc.isRendering ? vit301TraceRender : vit301TraceLive;
+
+        if (! vit301Traced.exchange (true, std::memory_order_relaxed))
+            TRACKTION_LOG ("VIT301_GUARD !hasLoadedInstance early-return plugin=\""
+                             + juce::String (getDebugName())
+                             + "\" rendering=" + juce::String (fc.isRendering ? 1 : 0)
+                             + " t=" + juce::Time::getCurrentTime().toISO8601 (true));
+
         return;
+    }

     if (! isInstancePrepared.load (std::memory_order_acquire))
+    {
+        static std::atomic<bool> vit301TraceLive { false }, vit301TraceRender { false };
+        auto& vit301Traced = fc.isRendering ? vit301TraceRender : vit301TraceLive;
+
+        if (! vit301Traced.exchange (true, std::memory_order_relaxed))
+            TRACKTION_LOG ("VIT301_GUARD !isInstancePrepared early-return plugin=\""
+                             + juce::String (getDebugName())
+                             + "\" rendering=" + juce::String (fc.isRendering ? 1 : 0)
+                             + " t=" + juce::Time::getCurrentTime().toISO8601 (true));
+
         return;
+    }
```

## 7. 端测边界声明

- 本卡端测=agent HTTP/ZMQ 面真栈烟测（四腿，脚本退出码+wav 物理计量+内核日志留痕）；**不覆盖 webui 渲染面与 Godot 用户旅程**（本卡改动域=tracktion_engine 两守卫+scripts+工件，无 webui/Godot 改动，无新增用户手测义务）。
- 渲染面断言建议（卡面遗留裁量项）：渲染路径实例状态断言是否加——本卡证据=守卫面清白（两静音腿零命中），断言的价值已转移为「离线渲染前 WaveShell 实例 non-realtime 自检/黑名单」面，交主管随修复卡裁量。

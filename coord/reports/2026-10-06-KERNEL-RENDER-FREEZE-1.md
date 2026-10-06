# KERNEL-RENDER-FREEZE-1 取证报告：start_render 冻死 JUCE 消息线程——根因定位（ABBA 死锁）+修复方案上交

- 卡片：`coord/cards/doing/2026-10-06-KERNEL-RENDER-FREEZE-1.md`（池序 8，MIDI-RECON-1 K1 腿，取证先行卡）
- 执行：PC 执行侧值班会话，2026-10-06 晚；领取时 origin/main `c8d27400`，分支 `port/kernel-render-freeze-1`（**零代码改动**——现有证据足以闭合，未动用 ≤10 行诊断日志额度）
- 被测内核：`Export/staging/runtime/VitApp.exe`，sha256 `959c06cfce58…2bd7c9`，mtime 2026-10-06 20:04（=MIDI-CMD-REGISTER-1 裁定部署的构建，内核源码与 HEAD c8d27400 一致——其间仅 coord 变更）
- 概率面声明：本卡全部断言为确定性命令面 + 静态代码分析 + 线程栈取证，**零 LLM 参与**；冻结在当前构建 1 次复现成功（另有首试 1 次行为级复现），叠加 MIDI-RECON-1 两轮 + 2026-09-06 战役多轮，同型 n≥5。

---

## 结论（TL;DR）

**不是 headless 设备问题（H1 否），不是回调未调度（H2 前半否），而是 tracktion EditRenderer 生命周期的一个确定性 ABBA 死锁：渲染完成回调（在消息线程上）析构 Handle 时 `renderThread.join()` 等待渲染线程退出；而渲染线程在完成回调返回后的收尾析构（`~NodeRenderContext`）里 `callBlocking` 等待消息线程服务——互等到永远。看门狗（H3）是 juce::Timer 驱动、跑在消息线程上，线程一死它同死，122 秒自愈永不触发。**

修复面清晰且在我方代码内（不碰 third_party）：**消息线程上任何路径都不得析构 EditRenderer::Handle**。本仓已有两处违规点（完成回调 `renderHandle.reset()` 与迟到回调 `releaseWedgedRenderHandle`），方案见 §6。

**重要发现：本缺陷 2026-09-06 已被一次完整的符号化取证战役锁定过**（`artifacts/render_wedge_evidence/`，含 minidump、StackWalk64 栈、WCT 等待链、/Zi 重构建符号化），当时结论与本次完全一致，但未沉淀为修复卡。本次复用其工具与符号化结论，在当前 HEAD 构建上增量复现并抓齐三件套（栈/WCT/minidump）。

---

## 一、渲染链调用栈逐帧（目标①）

### 线程模型（谁在哪个线程上）

- **ZMQ 网络线程**（`ZmqGateway::run`，`ZmqGateway.cpp:161-264`）：收命令 → `juce::MessageManager::callAsync`（`:202`）把 handler 投递到**消息线程** → 等待 future（默认 5000ms，`:237-244`）。消息线程冻结时它仍活着，回 `"Timed out waiting for JUCE command handling"`（本次复现实测回包原文）。
- **JUCE 消息线程**：一切命令 handler、`VitHeadlessService::timerCallback`（50ms，`VitHeadlessService.h:45`）→ `productionCoordinator->tick()`（`VitHeadlessService.cpp:787`）→ `checkRenderWatchdog()`（`VitProductionCoordinator.cpp:1044`）都在此线程。**看门狗与命令面同线程，是 H3 的结构性根源。**

### startOfflineRender 链（`VitProductionCoordinator.cpp:630-768`）

**同步段（消息线程上，render.start handler 内）：**

| 帧 | 锚点 | 说明 |
|---|---|---|
| `handleStartRender` | `TransportAudioService.cpp:1195-1223` | 参数解析；无阻塞点 |
| `startOfflineRender` 前置 | `VitProductionCoordinator.cpp:639-655` | rendering 标志/看门狗 arm（`:972-977` 仅写原子时间戳，**不起任何定时器**） |
| 设备查询 | `:668-671` | `getCurrentAudioDevice()` 为 null 时直接回落 48000Hz——**廉价 getter，无初始化/阻塞，H1 在此被否定** |
| `EditRenderer::render` | `tracktion_Renderer.cpp:679-717` | ①`ScopedRenderStatus` 建立（`:687`）；②`createRenderTask`→`RenderTask` 构造（`:147`）内 `callBlocking(createNodeForEdit)`——**callBlocking 在消息线程上走同线程快速路径**（`tracktion_AsyncFunctionUtils.h:173-177`），即渲染图创建同步发生在消息线程（本次冻例中它正常完成）；③起**裸 `std::thread`** 渲染线程（`:694`）；④返回 Handle |
| 回执 | `:743-768` | "Render started" ——日志证实回执发出（两轮 MIDI-RECON+本次皆然） |

**异步段（渲染线程上）：**

| 帧 | 锚点 | 说明 |
|---|---|---|
| 线程循环 | `tracktion_Renderer.cpp:700-713` | `for(;;){ if(cancelled)→finishedCallback(Cancelled); if(runJob()==again) continue; finishedCallback(结果) }` |
| `RenderTask::runJob` | `:173-184` | `renderAudio` |
| `renderAudio` 初始化 | `:273-296` | `callBlocking` 上创建 `NodeRenderContext`（`tracktion_NodeRenderContext.cpp:39-151`，断言消息线程执行）——**渲染线程 post 回消息线程并阻塞等待**；若 status 失败（如 "Didn't find any audio to render"，`tracktion_Renderer.h:82` 默认 `checkNodesForAudio=true`）**提前 return true 且不 reset nodeRenderContext**（`:291-295`） |
| `renderAudio` 成功路径 | `:298-304` | 渲染完在渲染线程上 `nodeRenderContext.reset()`（`:301`）——析构里的 callBlocking（见下）此时能被活着的消息线程服务 |
| 完成回调 | `VitProductionCoordinator.cpp:675-741` | `finishedCallback`（渲染线程上执行）→ `MessageManager::callAsync` 把完成 lambda 投回消息线程（`:683`） |
| **线程收尾（捕获析构）** | `tracktion_Renderer.cpp:694-714` lambda 捕获 | `task`（RenderTask unique_ptr）析构 → `~NodeRenderContext`（`tracktion_NodeRenderContext.cpp:153-177`）→ **`:168 callBlocking([this]{ nodePlayer.reset(); })`**——渲染线程再次等消息线程 |

**完成 lambda（回到消息线程上，冻点）：**

| 帧 | 锚点 | 说明 |
|---|---|---|
| stale 判定 | `VitProductionCoordinator.cpp:686-693` | jid 不符 → `releaseWedgedRenderHandle`（**违规点②**：`:979-985` erase 会析构 parked Handle → join，同样在消息线程） |
| `renderHandle.reset()` | `:696` | **违规点①**：`~Handle`（`tracktion_Renderer.cpp:663-667`）= `cancel(); renderThread.join();`——**消息线程 join 渲染线程** |
| 遥测发布 | `:722-739` | render_done/render_failed 在 join 之后才轮到——**永远到不了**，解释 MIDI-RECON"零遥测" |

## 二、死锁机制与双侧栈证据（目标③）

**死锁环（失败路径下为确定性，非竞态——消息队列 FIFO 保证）：**

```
消息线程: 处理完成消息 M1 → renderHandle.reset() → ~Handle → renderThread.join() ─┐
   ▲                                                                              │ 等
   │ 服务                                                                          │ 渲
   │                                                                              │ 渲
渲染线程: finishedCallback 投递 M1 → 返回 → 捕获析构 → ~NodeRenderContext          │ 线
          → callBlocking 投递 M2 → WaitableEvent::wait ────────────────────────────┘ 程
```

M1（完成消息）先于 M2（析构的 callBlocking）入队，消息线程先处理 M1 → join → 永远到不了 M2 → 渲染线程永远等 M2 → **互等**。成功路径不冻的原因：`renderAudio:301` 在 finishedCallback **之前**就 reset 了 nodeRenderContext（其 callBlocking 先入队、先被服务），线程随后干净退出，join 秒回——这解释了 2026-09-06 战役"p01 必冻 3/3、p03 健康"的工程相关性：**失败路径（MIDI-only 无 instrument → "Didn't find any audio to render"）必冻；成功路径不冻。**

### 当前 HEAD 构建复现（run `coord/runs/KERNEL-RENDER-FREEZE-1/20261006_210431`，内核 pid 33280）

- 命令序列全绿：add_track(1016)→insert_midi_clip(1019)→add_midi_notes(3音)→readback 一致→start_render 回 "Render started"（job b640f8c1…，RTT 0s）。
- 冻结确认：+8s 后 get_project_state → **5.00s 整回 error `"Timed out waiting for JUCE command handling"`**（网络线程活着、消息线程死了的标准指纹）；内核日志末 3 条= received ×3 无 dispatching（与 MIDI-RECON run2 逐字同型，`kernel_frozen.log:908` 后）。
- 产物文件：不存在（status fail 在 writer 创建之前返回，`tracktion_NodeRenderContext.cpp:94` 早退先于 `:104`）。
- **栈（`stacks_frozen.json`，46 线程）**：
  - 消息线程 tid 7188：`NtWaitForSingleObject ← WaitForSingleObjectEx ← Thrd_join ← VitApp+0x6efb43 ← … ← VitApp+0x9ae457 ← BaseThreadInitThunk`（main 线程入口）
  - 渲染线程 tid 34304：`NtWaitForAlertByThreadId ← RtlSleepConditionVariableSRW ← SleepConditionVariableSRW ← Cnd_wait ← VitApp+0xca62ff ← +0x69a0b2 ← +0xe7817e ← +0x6f8d82 ← +0xda0629 ← +0x7028e3 ← thread_start<>`（std::thread 蹦床，上下文切换仅 5 次=起跑即阻塞）
- **WCT（`wct_frozen.json`）**：tid 7188 Blocked → `ThreadWait(Owned)` → **tid 34304 Blocked**——等待链直连两条冻结栈。
- minidump：`minidump_frozen.dmp`（232,391 B）备查。
- 拆栈：graceful shutdown 无回包（冻结面预期）→ `taskkill /F /PID 33280` 成功 → 复查无 VitApp 进程、5555/5556/7878 无 LISTENING（仅客户端 TIME_WAIT 残影）。

### 2026-09-06 符号化证据（`artifacts/render_wedge_evidence/`，同代码同机制，行级定位）

`symbol1/frames_resolved.json`（/Zi 重构建 + dbghelp 符号化，与抓栈偏移经 `layout_mappability.json` 核对可映射）：

- 消息线程：`main(Main.cpp:108) → dispatchNextMessageOnSystemQueue → InternalMessageQueue::dispatchMessages → MessageManager::callAsync<VitProductionCoordinator::startOfflineRender::lambda_1>::AsyncCallInvoker::messageCallback → _Destroy_range<pair<String,shared_ptr<EditRenderer::Handle>>> → EditRenderer::Handle::~Handle → std::thread::join(thread:133)`
- 渲染线程：`std::thread::_Invoke<EditRenderer::render::lambda_1> → lambda_1::~lambda_1(捕获析构) → RenderTask::~RenderTask → NodeRenderContext::~NodeRenderContext(tracktion_NodeRenderContext.cpp:170) → tracktion::engine::callBlocking(AsyncFunctionUtils.h:277) → juce::WaitableEvent::wait`

与本次当前构建的模块偏移栈逐帧同构（当前构建无 PDB，以结构同构+WCT+代码路径唯一性闭合）。当轮还测得：watchdog 在消息线程存活时**确实会**在 140s 触发并发布 render_failed（L2 probe 路径，`wedge2b1/sweep_matrix_consolidated.md`），随后消息线程死于另一变体——迟到完成回调走 `releaseWedgedRenderHandle` erase parked Handle → join（违规点②）。

## 三、假设裁定（目标②）

| 假设 | 裁定 | 依据 |
|---|---|---|
| **H1** headless 无设备下设备查询/初始化在消息线程同步阻塞或死锁 | **否** | `VitProductionCoordinator.cpp:668-671` 是廉价 getter+48k 回落；本卡最小复现在同一 headless 环境跑通全部命令直至 render.start 回执；冻结点在完成回调 join（栈实证），与设备无关。Sep-06 战役 p03 同环境渲染健康亦为反证 |
| **H2** 回调从未被调度 vs 被调度但内部死等 | **后半成立（变体）**：完成回调**被调度且开始执行**（消息线程进入 lambda），随后卡在 `renderHandle.reset()`→join——"内部死等"成立；渲染线程侧则是"被调度但等不到服务"（callBlocking M2 永不入队处理）。修复面随之明确：两侧都不能等对方 |
| **H3** 看门狗失效机理 | **坐实：与冻点同线程共死** | `armRenderWatchdog`（`:972-977`）只写原子时间戳；`checkRenderWatchdog` 仅由 `tick()`←`timerCallback`（50ms juce::Timer，消息线程）驱动。消息线程 join 死 ⇒ timer 死 ⇒ 看门狗死。Sep-06 L2 路径（消息线程当时未死）140s 触发成功，证明看门狗本身逻辑有效 |

## 四、为什么"零 render_failed 遥测、零产物"

完成 lambda 的执行顺序是 `renderHandle.reset()`（join，冻死）→ 才轮到 render_done/render_failed 发布（`:722-739`）——遥测被 join 挡在身后，永远发不出。产物缺失是失败路径在 writer 创建前早退（`tracktion_NodeRenderContext.cpp:94→:104` 顺序）。两者都不是独立缺陷，均随死锁解。

## 五、修复方案建议（目标④，上交决策侧定夺；本卡不实施）

### 方案 A（推荐，根因修复）：Handle 析构搬离消息线程

完成回调与迟到回调中**永不在线程上析构 Handle**——`renderHandle.reset()` 改为把 Handle move 给一个 detached 清理线程（或统一并入 wedgedRenderHandles 由后台清扫），join 发生在清理线程上。渲染线程的收尾 callBlocking 由继续泵送的消息线程服务后正常退出，清理线程 join 收敛；真 wedged 插件场景下仅泄漏一个清理线程，不再冻消息线程。

- **改动面**：仅 `VitProductionCoordinator.cpp` 渲染完成两分支（`:696` 与 `:691→:979-985`），估 ≤15 行；`wedgedRenderHandles` parking 机制已是现成先例（`:1006-1010`）。
- **风险**：清理线程与下一 job 的 jobId 竞态需沿用现有 stale-jid 判定；L2 probe 分支（`:699-719`）同样受益（其完成也走同一 lambda）。
- **连带修复**：render_failed 遥测恢复、看门狗恢复可用（timer 活着）、`cancel_render`（已注册，`CommandDispatcher.cpp:3155`）恢复可用。

### 方案 B（纵深防御，建议随 A 同卡或紧随）：注定失败渲染的前置短路 + 遥测语义

渲染前同步预检"工程是否有可渲染音频内容"（audio clip 或 instrument），无则同步回明确 error（人话："工程里没有任何可发声内容（MIDI 轨无乐器），离线渲染必失败"），不起 EditRenderer。把本卡确认的必现触发面（MIDI-only 工程）挡在线程创建之前，同时给 `render.start` 补可测试的失败语义。

- **改动面**：`startOfflineRender` 同步段或 `handleStartRender`，一处预检；**不修死锁本身**——A 落地前，任何"启动后失败/取消"的渲染仍会冻（如插件装载失败、写盘失败、cancel 竞态），故 B 不可单独作为修复验收。

### 方案 C（结构性，长期方向，不建议本轮）：渲染全生命周期搬离消息线程

tracktion 的 `RenderTask` 构造（createNodeForEdit）与 `NodeRenderContext` 创建/销毁**按断言要求在消息线程执行**（`TRACKTION_ASSERT_MESSAGE_THREAD`），上游模型假定"消息线程活着且泵送"；我方若把渲染整体搬到专用线程，等于重写与上游的协作契约，波及全部渲染族调用面（L2 probe、A/B render probe、freeze 渲染），需全量重测。A 已在不碰 third_party 的前提下消除死锁，C 收益/成本比低，记录为技术债方向。

### 补充（H3 加固，可选）：看门狗搬离消息线程

`armRenderWatchdog` 时起 detached 定时线程，到期只做原子标志翻转+发布 render_failed（不碰 Handle 析构）。作为 A 之后的第二道防线（防"渲染线程真 wedged in 插件"时清理线程永久 join 的资源泄漏告警）。与 A 可同卡实施，估 ≤20 行。

## 六、工件清单与申报

- run `coord/runs/KERNEL-RENDER-FREEZE-1/20261006_210431/`（主复现）：probe_events.jsonl（命令序列+回执+冻结判定）、stacks_frozen.json（46 线程栈）、wct_frozen.json（等待链）、minidump_frozen.dmp、kernel_frozen.log（内核日志，start_render 后 received-only）、head.txt、kernel_exe_sha256.txt、git_status.txt
- run `coord/runs/KERNEL-RENDER-FREEZE-1/20261006_203747/`（首试）：行为级复现成功（ping 冻结+kernel_frozen.log 同型），栈采集因符号下载工具停滞（ole32.pdb 0 字节挂起）+ 内核随驱动进程树被杀而未取得——已如实记录；教训（采集进程与内核解耦、符号仅用缓存）已用于第二轮
- 驱动脚本：`coord/runs/KERNEL-RENDER-FREEZE-1/driver_krf1.py`（首试，废弃形态）、`probe_krf1.py`（定版）、`capture_krf1.py`（定版，ensure_pdb 仅缓存补丁）——复用 `scripts/render_wedge_probe.py`（Sep-06 战役工具）既有抓栈/WCT/dump 能力，未另起炉灶
- **诊断日志申报：0 行**（未动用额度）；**生产代码改动：0**；分支 `port/kernel-render-freeze-1` 无实现提交（空分支，可由决策侧清理）
- 强杀申报：graceful shutdown 无回包（预期）→ `taskkill /F /PID 33280` → 端口复查 5555/5556/7878 无 LISTENING、无 VitApp/VitAgent 残留
- 既有证据引用：`coord/runs/MIDI-RECON-1/20261006_190707`（行 39424-39439+尾部 received-only）、`20261006_191932`（行 10789-10804）；`artifacts/render_wedge_evidence/`（Sep-06 符号化战役——**建议决策侧验收后考虑将其结论沉淀入现行文档**，该目录含 253MB PDB，归档策略由决策侧定）
- 环境边界：headless 无音频设备、无 GUI；Windows 10 26200；栈抓取=ctypes dbghelp StackWalk64（系统 PDB 走 msdl 缓存），WCT=advapi32，minidump=MiniDumpWriteDump（comsvcs 等价路径）

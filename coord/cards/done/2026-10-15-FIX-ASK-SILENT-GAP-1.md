# FIX-ASK-SILENT-GAP-1：ask→slice 启动 122 秒静默空窗（LLM 前卡点定位+反馈面）

- 发卡：GLM 主管决策侧 / 2026-10-15 早窗（依据=M2-EXPERIENCE-1 首跑取证，[README](../../runs/M2-EXPERIENCE-1/README.md)）
- 派发确认：**已确认（用户 2026-10-15 晚窗「进入晚窗，可以发卡」=取证腿；「可以发卡」=修复腿，同日晚窗二度确认）**；验收负责人：GLM 主管决策流
- 池序 117；目标仓库=D:\Vit_DAW；P1 / ≤1 天（勘察先行）
- 模型分级：L2 / GLM 执行会话
- **两腿结构（2026-10-15 晚窗派发裁定）**：本轮派发=**取证腿**——零生产代码合入（临时打点仅存取证 worktree，不入 main）；取证件+修复面清单交付后停手，修复腿由决策侧核对与 116 文件域后再派。真栈取证 run 与 116 烟测腿按栈占用登记串行。

## 缺陷实锤

- 话术 09:53:11 落账 → slice/turn 09:55:13.794 才启动（**122s**），期间零轨迹事件、零 UI 反馈（用户 60s 判死、随后按停止）；
- journal_records=0 / observations_count=0：**LLM 一次未被调用**，卡点在 ask 前置上下文组装段（mixboard 快照在 +63s 才落盘，其后还有 ~60s 无工件空窗——疑有等待/超时环）；
- 对照：smoke 同链路（6 轨 fixture，run12/13）无此空窗；本场次为**空工程（0 轨）**——疑放大因素，非充分排除项。

## 修复目标（取证先行）

1. 定位 122s 的构成：ask 处理到 slice 启动之间的阶段打点（mixboard 快照触发点、DAD/CCB 预热、空工程下的等待/超时环），先出取证件再改；
2. 反馈面：ask 到 slice 启动之间必须有用户可见的忙碌状态/轨迹事件（「正在观察工程…」级），不留静默空窗。

## 文件域

agent/internal/chat ask 处理链 + goalrunner slice 启动前置段（领取时以打点/trace 实际锚点为准）+ webui 反馈位；与 116 卡（turn_control.go 停止路径）域不相交可并行。

## 验收标准

- 取证件：122s 空窗的阶段分解（含空工程 vs 有轨工程对照各一轮）；
- 修复后真栈：ask 后 ≤3s 内出现首个可见反馈/轨迹事件；空工程不再出现分钟级静默；
- `go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

打点发现卡点在内核侧（C++）→ 锚点上交，不扩域。

## 领取

- 时间：2026-10-15 18:06 +0800
- origin/main：e115fd8a（领取前 fetch 核对；本卡与 116/118 三张 10-15 卡此前仅存主管本地未跟踪态，随本领取提交首次入库）
- owner：GLM-5.3 L2 PC 执行会话（ZCode，D:\Vit_DAW）
- 分支：port/fix-ask-silent-gap-1-recon（取证 worktree 内含临时打点，不合入 main）+ 协调分支 coord/asg1-claim（仅卡面状态）
- worktree：D:/Vit_DAW_wt_asg1
- 本轮范围：取证腿（阶段打点定位 122s 构成；空工程 vs 有轨工程真栈对照；取证件+修复面清单）；修复腿不在本轮

## 回执

### 取证腿（2026-10-15 晚窗交付，修复腿待派）

- **取证件**：[coord/runs/FIX-ASK-SILENT-GAP-1/FORENSICS.md](../../runs/FIX-ASK-SILENT-GAP-1/FORENSICS.md)（随 port/fix-ask-silent-gap-1-recon 入库，commit 见下）
- **结论**：122s 空窗 = 语义入口分类器一次 LLM 调用（ordinary_agent_semantic_entry，deepseek-flash）在服务退化下的重试环独占——60s 超时×2 + 退避 750ms + 104ms "Service is too busy" 终拒 = **120.855s，与 M2 现场遥测 total_ms=120855 逐毫秒吻合**；其后 1.4s 失败收尾+prepare，09:55:13.794 slice 开启。journal=0 只覆盖 message loop，语义入口调用不在 journal（卡面前提修正）。mixboard 快照 +63s 落盘=内核波形遥测后台摄取（harness.go:4180 ingestKernelWaveformTelemetry），与卡点无关。
- **空工程真相**：非放大因素（语义入口载荷与轨数无关）；实为叠加缺陷——语义入口失败 fail-open 后**跳过空工程容量短路**（goalrunner_chat.go:247 只在 err==nil 进 planObservationFirstCapabilityRoute），0 轨工程本应 ~2s 短路作答（A 轮真栈实证：4.0s 全轮、零 slice），却被放进 slice 管线。
- **真栈对照**：A 轮（空工程+健康 LLM）4.0s 短路作答；B 轮（6 轨 fixture+健康 LLM）ask→slice=11.6s（前导 4.4s+语义入口 6.2s+prepare <1s）。烟测无空窗=当晚服务健康（10-14 遥测 6990/8745ms 无错），非工程差异。
- **修复面清单**（5 项，锚点=main e115fd8a 行号，详见取证件 §4）：① semantic_entry.go:132-140 入口无 deadline；② llm/client.go:157-174 默认 60s×3 尝试最坏 ~121s；③ goalrunner_chat.go:233-262 fail-open 跳过容量短路；④ server.go:2509-2523+goalrunner_chat.go:234 分类期零用户反馈（反馈面）；⑤（次级新发现）B 轮前导段 4.4s 使首个可见事件推迟至 t+4.8s，已超 ≤3s 预算。与 116 文件域：未触 turn_control.go/harness 停止路径；goalrunner_chat.go 为两卡共享文件（分类段 vs 停止段不同函数），交决策侧核对后派修复腿。
- **§8 记账**：计划 2 轮实跑 2 轮全达成；LLM 实耗 5 次全成功 0 超时（预探测 0.83s/A 轮入口 2.09s/B 轮入口 6.16s+B 轮 message loop 3.78s）；退化态 live 复现未执行（需服务退化或改全局 config，历史遥测两独立样本+常量吻合已充分，如实登记边界）。
- **临时打点**：port/fix-ask-silent-gap-1-recon 提交 0eb31135（[asg1] 阶段日志：cmd/vitagent/main.go+internal/chat/server.go+goalrunner_chat.go+internal/llm/client.go 四文件；探针 scripts/asg1_ask_gap_forensics.ps1+asg1_prompt_codes.txt）——**不入 main**，修复腿可按需 cherry-pick。
- **栈占用/释放**：占用登记 9b817024；18:21–18:26 两轮取证后手动 teardown（VitAgent 15200/VitApp 11620/Godot 12276+22764），复核四端口 5555/5556/7878/8787 监听=0、四类进程=0；释放登记随本提交。
- 取证腿到此停手；修复腿待决策侧核对派发。

## 取证腿验收（决策侧）

- **采信（2026-10-15 晚窗）**：根因链逐毫秒实锚（122s=语义入口 LLM 重试环 120.855s，与 M2 遥测 total_ms=120855 吻合）+叠加缺陷发现（fail-open 跳过空工程容量短路）+A/B 真栈对照+修复面 5 项带行号锚点；零生产代码入 main 亲核（打点全留 recon 分支 0eb31135）；§8 记账干净（2/2 达成、退化态 live 复现边界如实）。journal=0 卡面前提修正采信（语义入口调用不在 journal——决策侧早窗「LLM 零调用」表述同步勘误：LLM 被调用且在重试环中烧了 120s）。
- **修复腿解锁条件**：基于 116 合入后 main（19e5592e）重领取；文件域核对=修复面①③④触 semantic_entry.go/llm/client.go/goalrunner_chat.go/server.go:2509-2523，与 116 已合面（server.go 字段区/turn_control.go/runtime.go）不同函数区，重领取时以合入后 main 锚点为准；修复面⑤（前导 4.4s>3s 预算）为次级项，含入修复腿范围与否由新领取提交声明。recon 分支打点可 cherry-pick 复用（0eb31135）。

## 修复腿领取

- 时间：2026-10-15 19:28 +0800
- origin/main：01934926（领取前 fetch 核对）
- owner：GLM-5.3 L2 PC 执行会话（ZCode，修复腿，D:\Vit_DAW）
- 分支：port/fix-ask-silent-gap-1（实现）+ coord/asg1-fix-claim（本领取提交）
- worktree：D:/Vit_DAW_wt_asg1（取证腿后复用，干净态重新 checkout）
- 范围：修复面 ①-④ 全做；**⑤ 做**——实现方式=ask 受理即发首个忙碌事件（server.go handleChat note 分叉后、前导段之前），不依赖前导段提速，B 轮形态下首事件 ~t+0.05s，同时满足 ≤3s 预算；前导段本身的 4.4s 构成不再细分（如后续仍需优化另开卡）
- ②实现口径声明：`llm.Request` 显式 Timeout 的重试语义保持 per-attempt 不动（b4/c1/c2 planner 与 mix_session 均在用，改语义越域）；总预算经**新增 opt-in 字段**（TimeoutIsTotalBudget）只在语义入口路径启用——message loop（无显式 Timeout，默认 60s×3）零变化

## 修复腿回执（2026-10-15 晚窗交付，待决策验收）

- **范围**：修复面 ①-⑤ 全做（⑤=ask 受理即发首事件；前导段本身提速不在范围，遗留观察见下）。
- **实现**（port/fix-ask-silent-gap-1 四提交：dc534e83 修复主体 / 66f58835 烟测场景 / a811555a ⑤发射点加固 / 2d09b52b 工件；被测 agent=a811555a）：
  - **①** semantic_entry.go：`planSemanticEntry` 的 llm.Request 设 `Timeout=12s`（`var semanticEntryRequestBudget`，单测可覆写；初判与 decode 修复两次调用各携带预算）——健康观测最慢 6.2s 的 ~2× 余量，退化服务 12s 限时失败快速返回走 fail-open。
  - **②** llm/client.go：Request 新增 **opt-in** 字段 `TimeoutIsTotalBudget`——启用时 `ctx.WithTimeout` 包住整个有界重试环（挂死端点不再 60s×2+退避≈121s）。**默认路径（message loop 无显式 Timeout=60s/次×3）与既有显式 Timeout 用户（b4/c1/c2 planner=3min、mix_session=30s 的 per-attempt 语义）零变化**；`TestCompleteRequestPerAttemptTimeoutKeepsFullRetries` 钉死无标志时 3 次尝试完整执行（慢 503 计数亲核）。
  - **③** capability_routing.go `semanticEntryUnavailableCapacityAnswer`：入口分类失败 fail-open 时跑**零 LLM** 容量事实评估（与 planObservationFirstCapabilityRoute 同源：observeCapacityProjectFacts→assessFreeStateCapacity→capacityFactShortCircuit）——命中即短路直答（Reply 追加「意图分类服务本次不可用，以上直接来自工程结构事实」+WorkflowData.semantic_entry_unavailable=true+goal 诚实 completed）；不命中返回 false 走原管线；不持久化任何合成分类决策；revision 未绑定不断言空（诚实边界保持，负例单测在卷）。
  - **④⑤** server.go handleChat：ask 受理即发 `item.started/semantic_entry` 忙碌事件（`req.Note==nil` 守卫；位置=**全部前导段之前**——run1 实测工作区前导段吃 2.2s 后 a811555a 前移）；goalrunner_chat 分类结局发 `item.completed`（失败态对用户可见）；writeChat 兜底幂等收口（continue/续跑等不经分类的快路径不残留）。**webui 零改动**（activity lane 既有消费面：pending spinner→item.completed 撤除；事件 id/生命周期语义经 webui messageLifecycle 源码核对接通）。AGENT-F4 时序契约测试按新契约修订（首事件=忙碌指示器新钉；过滤指示器族后 turn.started 先于分类+companion 紧随的原断言不动）。
- **单测**（95 包 0 FAIL；gofmt 净=暂存 LF 内容逐一核验，工作树 CRLF 为 autocrlf 假象）：internal/llm/client_total_budget_test.go（挂死端点 300ms 精确收敛+per-attempt 保持）+ internal/chat/semantic_entry_ask_gap_test.go（**退化路径收敛上限**：300ms 假客户端注入不真烧长时限；空工程短路端到端经 runAgentLoopChat 真分类块驱动；失败态事件生命周期；有轨诊断/未观察负例）。
- **真栈烟测**（dev_agent_smoke.ps1 新场景 `ask_feedback`，journey 式 berth=kernel+agent 自启自拆；**未启 Godot**——断言面全部在 agent HTTP 事件/状态面，端测边界如实声明，渲染面由 webui 零改动+事件消费面源码核对背书）：两 run 均 **PASS exit 0**。run2 20261015_200257（干净）：A 轮空工程（project.new，track_count=0 亲核）**首反馈 118.2ms**≤3s、短路作答 6.5s 收口（stop=capacity_fact_short_circuit）；B 轮 6 轨（spv1_p01 隔离副本 project.open，track_count=6 亲核）**首反馈 1.6ms**（修复前 turn.started=t+4.8s）、slice 3477ms、stop 后 turn.stopped 收口。run1 20261015_195610：B 轮 slice 探测踩 PowerShell `@('')` 陷阱致 stop 过早发射，但四门禁仍过+顺带实证**分类期 stop 不楔死**（turn 仍 limit_reached 干净收口，与 116 修复相容）；脚本已修（slice_id 非空门），run2 为修正后干净复跑（代码态已变，非同断点重试）。
- **§8 记账**：计划=A×1+B×1（A 允许 1 次 LLM 形态 miss 重跑，未动用）；实跑 2 次场景调用均 exit 0 首过、零确定性断点失败、止损线未触；LLM 实耗≈6 次全成功 0 超时（语义入口×4：A 轮两 run 各 ~6s；B 轮两 run 各 ~1-2.4s + message loop×2 单 turn limit_reached）。失败分类未启用（无失败轮）。
- **栈纪律**：占用登记 28c396e9（19:51 四端口/四类进程零核对；占用期间顺带勘误归档 116 腿 PC 面跨端误挂条目——其占用/释放在 MAC-RUNTIME-STACK.md c98fb86a，PC 物理面+Mac 登记面双印证）；两 run 均场景自 teardown，20:10 复核四端口监听=0、VitApp/VitAgent/vsphub/Godot 进程=0；释放登记随本提交。被测=worktree 构建 agent（a811555a）+主检出 VitApp.exe B6565DCF85D1DA86（C++ 面零改动）。
- **工件**：coord/runs/FIX-ASK-SILENT-GAP-1/20261015_{195610,200257}/（timeline/events_final/state_final/summary/head 轻件随 2d09b52b 入库；kernel_workspace/project_B/fixture_stems/agent_drafts 重件留盘 worktree 不入库）。
- **打点声明**：**未 cherry-pick** recon 分支 [asg1] 打点（0eb31135）——事件/状态面自带可测性无需源内打点；探针方法（raw TCP POST+事件面 100ms 轮询+slice 后 stop 控预算）承 asg1_ask_gap_forensics.ps1 改造为 PASS/FAIL 门禁场景。
- **遗留观察（不属本卡，供决策侧备查）**：(a) b4/c1/c2 planner 显式 Timeout=3min 仍为 per-attempt 语义（最坏 3×3min+退避）——如需收紧可按同 opt-in 机制另行启用；(b) B 轮前导段（工作区激活/uiContextSnapshot ~2.2s）本身未优化，仅用户反馈不再等待它；(c) A 轮今晚入口分类 ~6s（服务偏慢但健康）——12s 预算内，若未来常见超限可评估下调或分级。

## 验收（决策侧）

- **修复腿 pass（合入 merge commit 见上）** / 2026-10-15 晚窗（GLM 主管决策流）：dc534e83+a811555a 五面 diff 亲读（①12s 预算/②opt-in TimeoutIsTotalBudget 默认路径零变化/③零 LLM 短路+诚实披露文案/④⑤受理即发+幂等收口，webui 零改动）；**决策侧独立亲跑 95 包 0 FAIL**；run2 工件亲核（A 轮 first_event_ms=118.2+capacity_fact_short_circuit 收口 6.5s；B 轮 first_event_ms=1.6+slice 3477ms+turn.stopped）与回执逐项吻合；边界采信=烟测断言面 agent HTTP（Godot 未启）、渲染面由 webui 零改动+事件生命周期源码核对背书；AGENT-F4 契约修订=④⑤ 目标语义本身，过滤后原断言保持。遗留观察三项（planner per-attempt/前导 2.2s/入口 ~6s）登记备查不阻塞。

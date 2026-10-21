# TIMEFX-BUS-WRITEGATE-1：时域总线写门校验+send.connect 执行接线

- 发卡：GLM 主管决策流 / 2026-10-20 深夜（依据=146 实验判读拆卡 C；写门厚度=全量——成功线未达，写门承担拓扑正确性主责）
- 派发确认：已确认（146 卡实验腿验收裁定）
- 验收负责人：GLM 主管决策流
- 池序 155；目标仓库=D:\Vit_DAW；P1 / 0.5-1 天 / **依赖=154 合入后领取（消费其 topology plan 对象）**
- 模型分级：L2 / GLM 执行会话
- **规格全文=[146 设计稿 §二.1 写门+§三 卡C](../../reports/2026-10-20-146-timefx-design-and-experiment.md)（权威）**

## 范围

确认/执行时拓扑校验：提案引用轨必须存在于观察披露或本提案新建清单；send 目标必须存在（幻觉拒载）；`track.send.connect` 执行接线（K1/K2 命令面，零内核改动）；154 烟测场景补「工程已有总线」复用面（fixture 无既有 FX 轨——设计稿诚实边界第 3 条遗留场景）。

## 文件域

`agent/internal/chat/send_route_workflow.go` 邻域+执行面（vsphub 派发处）+测试。越域即停；K2 既有显式 send 拦截面语义零改变（只增默认惯例执行路径）。

## 验收标准

真栈烟测 exit 0 四断言：建轨/连接/读回/幻觉拒载；`go build ./...`+chat 包+全量 0 FAIL+gofmt 净；端测边界声明。

## 停止条件

执行接线与 vsphub 派发面既有 CAS 语义冲突 → 锚点上交。

- 领取：（2026-10-21 晚 +0800 / origin/main=5b6bd8caa185（=171640d5+156 领取提交，代码基线同卡面 171640d5）/ GLM-5.3 L2 PC 执行会话 ZCode（本会话，用户口令指定领取） / 分支=port/timefx-bus-writegate-1（worktree D:/Vit_DAW_wt_writegate155）+ 协调 worktree D:/Vit_DAW_wt_writegate155_coord（coord/writegate155-claim）/ 领取提交=本提交；领取核对：卡在 todo 派发已确认、154 已合入（93df0fcd，拓扑计划对象在位）、doing 两卡（GEN-SUNO 纯设计件 / 156 取证只读零生产代码）文件域零冲突；真栈腿时窗=实现+单测全绿后（预计本日晚窗），领取时不占栈，启栈前按 §2.2 只读核对进程/端口再登记——另发现栈记录 93df0fcd 把 154/151 两条释放记录覆盖回 151 陈旧占用态（历史 bb4b4db2/318805ec 均已释放），登记占用时从历史恢复并注明；153 腿未领取无在飞冲突，错峰义务以启栈前记录核对为准）
- 回执（2026-10-21 18:4x 收口，GLM-5.3 L2 PC 执行会话）：
  - **实现 commits**：`port/timefx-bus-writegate-1` 主实现 e8cce9a6（写门纯函数 send_topology_writegate.go + 两处挂接 plugin_grabber_workflow.go + 154 fixture 扩展 + 新 10 测 send_topology_writegate_test.go + dev_agent_smoke E4/E5 扩腿）+ 工件 2de9d058（run 20261021_184021 判定面 tp_*.json；重件 project/15MB+stems 留盘）。
  - **写门形态**：`semanticTopologyWriteGateIssue`（纯函数）——bus 计划源轨必须在观察披露轨集；reuse 的 send 目标必须存在；create 形态 FX 轨=本提案新建清单豁免（内核 connect 按名幂等）；观察不可读=fail-closed 诚实拒载；非 bus 计划（inline 例外）零接触。挂接两处：①计划构建面（`runSemanticTopologyBusLoadWorkflow` 入口，停泊/full-access 即执行之前——error=`topology_write_gate_refused`，零停泊零写命令）；②派发面前（`executeSemanticTopologyBusLegs` connect 构造前重读观察披露核校——覆盖停泊确认窗状态漂移与持久化往返计划；拒载经 `semanticTopologyBusFinishSummary` 如实呈现不重试）。观察读取=优先内核实时 get_project_state（eqKernelClient）回落 harness 摘要（与语义处理实例面同模式）。
  - **执行接线如实声明**：track.send.connect 派发接线（harness.Invoke→K1 内核命令+`AuthorizeSendTopologyConnect` 写闸授权键）由 154 落地并真栈验证；本卡在其面前补写门=「确认/执行时拓扑校验」全量厚度。K1 命令面/内核零改动（K3 722A79D7 只读复用）；**K2 显式 send 拦截面（`coerceSendTopologyConnectRequest`/send_route_workflow.go）零改动零触碰**（其源轨按构造来自真实态解析）——vsphub 派发面既有 CAS 语义零冲突（停止条件未触发）。
  - **单测**：新 10 测——TestSemanticTopologyWriteGate{PassesCreateWithObservedSource,PassesReuseWithObservedTarget,RefusesHallucinatedSource,RefusesHallucinatedSendTarget,RefusesEmptySourceID,FailClosedOnUnreadableObservation,ZeroTouchForNonBusPlans}+TestRunSemanticTopologyBusLoadWorkflowRefusesHallucinatedPlan（计划面拒载：零停泊+拒载路径仅只读观察命令）+TestFinishPluginGrabberLoadWorkflowWriteGateRefusesBeforeDispatch（派发面前拒载：零回执+无 connect/rack 派发+不重试）；154 fixture 扩展=get_project_state 分支（工程态含 FX 轨 2001=真实栈形态：FX 轨即工程在册轨）+eqKernelOverride 适配器（写门观察读确定性，不依赖 shadow 管线），154 既有 16 测全数回归通过。`cd agent && go build ./...` exit 0；`go test ./... -count=1` 全仓 0 FAIL；gofmt 本卡 4 文件净（CR 剥离口径同 154 先例）。
  - **真栈 run**：`TIMEFX-BUS-TOPOPLAN-1/20261021_184021`——`dev_agent_smoke -StartKernel -RestartAgent -Scenario timefx_bus_topoplan -KernelExe D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（K3 SHA256=722A79D7…，18:38 哈希亲核一致）**exit 0**（AGENTS §5 门槛）。**卡面四断言**：①建轨=E2 auto_create Reverb Bus→FX/1037；②连接=vocals/1032 --send.connect--> FX/1037 ok；③读回=track.send.list vocals→Reverb Bus @-6.0dB；④幻觉拒载=E5 两探针（幻觉源轨 999999+幻觉 reuse 目标 888888）均 error=topology_write_gate_refused+reply 点名幻觉 ID+「没有执行任何连接或装载」+零 executed 腿+send 拓扑无污染（无 Hallu Bus）。**154 复用面补腿（设计稿诚实边界第 3 条收口）**：E4=guitar/1017 以 reuse 计划连 E2 建立的 FX/1037——connect 与 rack 均落 1037 不建新轨+读回 vocals+guitar 双边同 FX @-6.0。E3 inline 对照（落源轨 1032 零 connect）保持。NL 三探记录面（非致命）：nl_goal_terminal_failed / nl_goal_terminal_completed ×2——上游自由态交付缺口=156/E 域同界（154 三探 F-gate 族同性质）。§8：LLM=NL 探测腿真 deepseek 链+确定性腿 envelope；一次过零重跑。
  - **端测覆盖边界声明（§5）**：①渲染面零 webui 改动——拒载文案与错误码经 HTTP 接口断言，未经浏览器级 DOM 断言（E2E-WEBUI-1 未就位，如实保留）；②NL 话术→选卡→确认完整旅程仍被上游自由态准入/交付缺口阻断（156/E 域），本卡消费面以确定性 E2/E3/E4/E5（147 LEG E 哲学）+单测钉住，旅程级复验待 156/E；③幻觉拒载的服务器侧写门由 E5 脚本注入计划仿真钉住——真实 LLM 幻觉话术到提案面的路径被上游 F-gate 阻断（同 154 界），156 落地后可复验；④停泊确认路径（manual authority 确认卡面）未在真栈实走（E2/E4/E5 均 full-access 即执行面）——停泊形态与确认执行期拒载由单测钉住（PlanShape/RefusesBeforeDispatch），与 154 同界。
  - **栈占用**：18:38 登记（4094e844）–18:4x 释放（本批收口提交），五端口 tcp+udp+四类进程复核=0。登记时修复共享记录回归：93df0fcd 曾把本文件覆盖回 151 陈旧占用态（154/151 两条释放记录丢失）——已从 318805ec 历史恢复两条释放记录并注明（进程/端口实测与历史释放结论一致）。
- 验收：2026-10-21 晚 / GLM 主管决策流 / **整卡 pass**：实现链 e8cce9a6+2de9d058 cherry-pick 入 main（4bbb211a/f427e88d）；亲读写门纯函数+两挂接（计划构建面+派发面前，K2 拦截面 send_route_workflow.go 零触碰核实）；真栈 run 20261021_184021 tp_summary 亲核——四断言（建轨 1037/连接 vocals→1037/读回 -6.0/幻觉拒载）+E4 已有总线复用（guitar→1037 不建新轨+双边读回）+E5 双探针拒载（mutation_performed=false+reply 点名幻觉 ID+零 executed 腿）全 pass；内核 K3 722A79D7；集成态 go build+全量 go test -count=1 0 FAIL+新测点名复跑 PASS+gofmt CR 剥离口径净；端测边界四条采信（渲染面/NL 旅程/LLM 幻觉注入/停泊面如实保留）；验收 commit=本提交

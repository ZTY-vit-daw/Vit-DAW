# M3-LLM-NEUTRAL-EMBED-1 真栈工件与结果（RESULTS）

- 卡：coord/cards/doing/2026-10-30-M3-LLM-NEUTRAL-EMBED-1.md（池序 222）
- 执行：ZCode GLM-5.3 PC 执行会话（222 卡会话）；worktree D:/Vit_DAW_wt/m3-neutral-embed @ port/m3-llm-neutral-embed-1
- 领取基线：本地 main `80d4a22c`（领取提交 41101e68）；占用登记 c80e33e7（2026-10-30 21:43）
- 一句话结论：**修复面兑现+部署亲证通过——neutral 装配 7/7 render 指引行在场（system 14,772→15,468B）；重验组 1 三连同型 no_route 止损在指引在场形态下复现，卡面停止条件 2 精确命中：变体话术通道在结构上无法到达 automation 工具链（三道闸锚点上交，非 prompt 迭代问题），组 2/3 未消耗。**

## 一、修复实现（面 1）

- 插入点勘察定案：**agentloop 装配组装层**（卡面授权候选位），非 ruleset embed 新段（`agent/rules/ruleset/` 不在文件域，越域即停纪律）也非 carriers（`contextruntime/carriers/` 同越域）。
- 改动四文件（全在 `agent/internal/agentloop/` 域内）：
  1. `neutral_automation_route.go`（新）：指引常量+`message_loop_neutral_family_automation_route` Section ID+自治 CacheKey `neutral-family-automation-route:v1`；
  2. `message_loop.go` `neutralFamilySystemSections`：bundle 主路径在 L1 域（`ctx.layer.rules*`）段后插入指引段（位形对齐 carriers `semantic_topology_convention` 先例）；回落分支=骨架段+指引段两 Section；
  3. `neutral_automation_route_test.go`（新）三面：**同源同文锁定**（chat 族 `chat.discipline.plugins_mixing` 段内该行逐字节比对——chat 段变更时红，强制同步=「不新写文案」的执法面）+ 生产装配在场断言 + 普通族（message_loop_system）零回退断言；
  4. `carrier_neutral_test.go`：透传/回落形态锁定更新（透传+指引段插入位）。
- **同源同文**：指引文本=chat 段第 10 行逐字复制（694B 无改写），判断权冻结令合规（让既有指引到达，不新增判断话术）。
- chat/server.go **零改动**（确认话术分支零回退的自然形态；217 指引行原位未动）。
- 回归：agentloop 全包 + chat + promptruntime + carriers + harness 全绿；T-A3 字节稳定测试（system 跨轮恒等）绿。

## 二、重验执行（面 2，220 断言体 v2 原口径）

- 命令（worktree 根执行，220 同形）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt/m3-neutral-embed/scripts/dev_agent_smoke.ps1 `
  -StartKernel -RestartAgent -AutomationLLMRouteSmoke -NoChatSmoke -NoStripSilenceSmoke `
  -KernelExe D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe `
  -RunArtifactsDir <run 目录>
```

- run：llmroute2_20261030_1（21:43）；ps1 exit 0（COMPLETED——**220 的 GUIDANCE_MISSING exit 2 消失**，脚本指引检查通过）。
- **部署面亲证**：`logs/agent_prompt_render.jsonl` 7/7 行全部 `message_loop_neutral_family_selection` tag，system 均含指引行原文（"For parameter automation envelope requests…"）与 `automation.set_points`/`automation.get_curve` 工具名，长度 14,772B（220 缺陷形态）→15,468B（+696B=指引段+连接）。
- kernel=主检出 VitApp.exe 只读复用（sha256 `c1e32dfe` 运行前亲核=217-224 腿同源零漂移；本卡 C++ 零改动）；agent=worktree 自建（含本卡修复）；无 UI；vsphub 不需要；VIT_PROJECT_XML 隔离（git 干净 run 副本）。
- **组 1 三连同型 no_route 止损**（脚本自动停止，组 2/3 轮次未消耗——220 同口径）：

| 轮 | verdict | 形态 |
|---|---|---|
| r1 | no_route | 话术→free-state needs_observation→turn 预算耗尽（limit_reached/waiting_continue）→「继续」→确定性 settle 回复「现在没有可执行的待确认混音动作」（stop_reason=no_pending_mix_tick_candidate，**无对应 LLM 调用**） |
| r2 | no_route | 同型；「继续」被续跑队列吞（automatic_continuation_already_scheduled，确定性回执） |
| r3 | no_route | 与 r2 完全同型 |

- 工具调用面（三轮合计）：仅 `ccb.observation_request`/`ccb.observation_catalog`，零 automation.*。
- **LLM 轮次真实计费**（deepseek-flash）：10 调用 / 106,073 in / 6,619 out（+23,936 cache read）；轮面分布=每轮 `ordinary_agent_semantic_entry`（696in 语义入口小轮）+`message_loop`（neutral 决策轮 13.7-15.9K in；r2/r3 含自动续跑后台轮）。逐调用在 `logs/agent_llm_telemetry.jsonl`。

## 三、停止条件 2 定案：三道闸（锚点上交，LLM 路由设计讨论）

指引已到达模型（render 亲证），但变体话术通道在结构上无法把诉求送达 automation 工具链：

1. **free-state 决策轮工具面封闭**：neutral 装配的 allowed tools=ccb.* only，骨架规则「Do not call any mutation…tool in this phase」——模型合法动作集不含 automation；指引行（"use automation.set_points…"）与骨架规则**同场冲突**，模型在轮内即使识别诉求也无合法输出形态（free_state 形状枚举里没有 automation 通道）。
2. **turn 预算**：观察探索耗尽 MaxTurns（8/slice）→ waiting_continue 截停（220 RESULTS §四.3 预警面的实测复现）。
3. **settle 面无 LLM 轮**：goal 终态回复=no_pending_mix_tick_candidate 确定性文本；含指引+automation catalog 条目的 `message_loop_system` 完整装配轮**从未被变体话术触达**（220 run1 唯一例外是被动「确认写入」语境）。

设计讨论候选案（决策侧裁，225 澄清上移同域）：
- 案 A：218 词表扩面——「越来越 X」类时变分界词进武装词表，确定性路由前置（不依赖 LLM 轮行为）；
- 案 B：`ordinary_agent_semantic_entry` 语义入口轮（在场，696in 小轮）增设 automation 诉求分类→路由编舞链而非 free-state goal；
- 案 C：free-state 终态增设 automation 边界形态（capability_blocked 带 automation 边界→relay 编舞确认链）；
- 案 D：settle 面补一轮 LLM（message_loop_system 装配，指引+catalog 已在场）。

## 四、次生评估面定案（越域上交另拆卡）

- 「模型视角轨无插件」断点=**内核 VSP 紧凑面**：`VitApp/Source/Service/VspKernelReference.cpp:603` `normaliseTrackArray` 白名单不含 `plugins`/`plugin_count`（轨级 Volume&Pan 内置插件非 rack 节点，紧凑态只有 volume_db/pan 数值+`plugin_param_digest` 哈希，plugin_id 不可见）；内核完整序列化面 `TrackService.cpp:120` `createTrackState` 有 plugins 数据——剥除发生在紧凑归一化。
- agent 侧链路已备好：mixboard `project_package.go:163` `buildTrackSummaries` 已消费 `plugins/rack_nodes/plugin_chain`；未来放行点=capabilitycontext `free_state_observation.go:997` `compactProjectTracksSummaryForStructure` 6 字段白名单。
- 按卡面「需动内核 VSP 紧凑面→锚点上交另拆卡，不阻塞面 1/2」执行。

## 五、迭代留痕

1. 本 run 一次到位（无迭代轮）：组 1 止损后脚本按设计停止（exit 0 COMPLETED，组级失败留 summary 不 throw）。
2. 时戳：占用登记 21:43（推送后远端亲读置顶+进程端口即时复核 21:43:03）；拆栈 21:46（VitAgent 10732+子 24860、VitApp 23656+子 34328 手拆，路径亲核=本腿所属）→复核全零。
3. 主检出 `VitApp/Workspace/default_project.xml` mtime 19:58:35（224 记录在案）=登记前遗留，本腿零触碰（VIT_PROJECT_XML 隔离副本承接内核写入；运行时工程文件剔除按 218 先例）。

## 六、成本与端测边界声明

- **覆盖**=agent /agent/chat HTTP 面真实 LLM 轮次（10 调用真实计费）×prompt render 逐 turn 装配取证（7/7 指引在场亲证）×agent log 工具调用流×内核 ZMQ setup/曲线基线；与 220 同栈同断言体（差异仅=本卡 agent 修复）。
- **不覆盖**=指引在场且通道可达前提下的模型路由行为（三道闸前置未达）；set_points 确认链（未触发）；choreo pending 互不干扰（组 1 第 3 轮止损，腿未嵌入）；组 2/3 话术（止损后未消耗）；webui 渲染面与用户旅程（E2E-WEBUI-1/JOURNEY-1 既有边界）。
- 生产改动=本卡修复四文件（agentloop 域内）；scripts/ 零改动（220 断言体原样复用，口径未改）。

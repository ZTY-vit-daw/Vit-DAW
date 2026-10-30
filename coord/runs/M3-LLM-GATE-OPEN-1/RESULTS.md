# M3-LLM-GATE-OPEN-1 真栈工件与结果（RESULTS）

- 卡：coord/cards/doing/2026-10-31-M3-LLM-GATE-OPEN-1.md（池序 227）
- 执行：ZCode GLM-5.3 PC 执行会话（227 卡会话）；worktree D:/Vit_DAW_wt_227 @ port/m3-llm-gate-open-1
- 领取基线：origin/main b42c93c7（228 领取提交）+merge 主管发卡分叉 d753565c（ac602287）——**共享 main 追平**：origin/main 此前不含 222 实现合入（在主管本地未推分叉），本卡渲染面以 222 交付物为基座，处置=merge 入工作分支后领取提交推共享 main（97ce48e5）；主管下次推送=fast-forward。冲突仅 222 done 卡（取主管含验收版）。
- 一句话结论：**闸 1 开放兑现+部署亲证通过（8/8 neutral render 工具面+治理门语义在场，system 15,468→17,045B）；零裸写治理链单测端到端钉死；重验腿组 1 三连 no_route 止损在全新形态下复现——三前提齐备（render 亲证）但 closure 相位轮节奏（audioClosureMessageLoopBudget MaxTurns=1-2+异步续跑泄流+观察边界确定性 settle）使模型在烟测窗口内从未获得执行写链的轮结构：闸 2 深层真身锚点上交（chat 域越界），非模型能力面、非闸 1 布线缺陷。**

## 一、闸 1 开放实现（五点，全 agentloop+骨架 twin）

1. **准入**：`free_state_automation_surface.go`（新）——`admitFreeStateDecisionRoundToolSurface` 把四工具并入 free-state 决策轮有效 AllowedTools（`plugin.get_parameters`/`automation.get_curve` 读直放；`automation.set_points`/`automation.clear_region` 写经 harness 治理门 RequiresConfirmation——manual=确认卡/显式完全权限=直执，223 stamp 形态，触发权=模型工具调用）。四入口接线（Start/Continue/ResumeAfterConfirmation+pull `resumeContinue`）；诊断专用轮与非 free-state 轮零变化；chat 侧 ccb-only 授予不动（决策轮工具面=决策循环自身相位策略）。
2. **显示**：`messageLoopNeutralFamilyAllowedTools` 过滤器放行+别名归一去重；`messageLoopNeutralFamilyCatalog` 四行目录（含参数要点+治理门语义）。
3. **校验**：`messageLoopFreeStateOutputIssue` needs_observation 分支放行 automation 调用（ccb 专属 view-set/target/指纹检查只对 ccb 执行；同轮至多一个 mutation；诊断专用轮拒绝）。
4. **执行守卫**：`messageLoopToolGuardIssue` free-state mutation 一刀切弹回改**贯穿式**——面内 mutation 落通用检查链（read-only barrier/toolpolicy/mix-intent 既有语义全保持：明说只读的轮写仍被拦）后至治理门；面外 mutation 与诊断专用轮仍就地弹回。
5. **骨架 twin**：mutation 禁令改「automation 面经治理门+每轮至多一写；面外仍禁」（`agent/rules/ruleset/resources/sections/neutral_family.discipline.rules.txt` 第 6/7 行+`ccb_model_prompt.go` legacy 常量同文；字节级 parity 测试 `TestNeutralFamilySkeletonMatchesRulesetEmbedByteForByte` 在场锁定）。

闸 2 配合=`noteFreeStateAutomationEngagement`：free-state 轮首次执行 automation 面调用一次性 +4 turn/+120s headroom（对齐 settle+4 先例；观察 slice 语义不动）——**本次重验证实触发器过晚**（见 §三）。
闸 3 勘察定案=决策轮内全链可达：调工具→needs_confirmation 暂停=治理门（`runner.go:600-625`）→`ResumeAfterConfirmation` 确认恢复→写入→读回→终态汇报——**settle 面无需补轮**（chat 族既有通用机制，223 已验形态）。

## 二、测试面（全绿）

- 本卡十面（`free_state_automation_surface_test.go`）：名字归一/准入幂等+诊断排除/校验器正负例（面外 mutation 弹回+同轮双写弹回+诊断专用弹回）/执行守卫贯穿式/显示面/骨架 twin 语义+旧一刀切句消失/headroom 一次为限/**零裸写端到端主负例**（free-state 轮模型 set_points→executor 首见 Confirmed=false needs_confirmation 暂停 WaitingConfirmation→确认恢复才 Confirmed=true——任何路径无未经确认写）。
- 回归：agent 全量 96 包 ok 零失败；ruleset/promptruntime/carriers/chat/harness/tools 全绿；222 同源锁定测试（指引段）未触仍绿。

## 三、重验腿执行（228 前置已并入）

- 228 合入形态：origin/main 只有 228 的登记/回执，实现合入滞后（与 222 同型的主管本地分叉形态）——merge `origin/port/m3-track-plugin-disclose-1`（0369fe75，冲突仅登记面），内核=228 worktree 自建 `VitApp.exe` sha256 **f9e3da0a** 运行前亲核（=228 回执同值）只读复用；本卡 C++ 零改动，内核源同树。
- 命令（worktree 根执行，220/222 同形原口径**未改**）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_227/scripts/dev_agent_smoke.ps1 `
  -StartKernel -RestartAgent -AutomationLLMRouteSmoke -NoChatSmoke -NoStripSilenceSmoke `
  -KernelExe D:/Vit_DAW_wt_228/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe `
  -RunArtifactsDir <run 目录>
```

- run：llmroute3_20261031_1（登记 8c06c815 于 22:43 先于启动）；**ps1 exit 0 COMPLETED**；prompt_guidance_rendered=true。
- **部署面亲证（三前提之二，render 8/8）**：每轮 system 17,045B（222 形态 15,468B→+1,577B=准入面+目录行+骨架改写），逐字证据：
  - `Allowed tools:
ccb.observation_catalog, ccb.observation_request, plugin.get_parameters, automation.get_curve, automation.set_points, automation.clear_region`
  - `The automation surface is admitted in this phase: plugin.get_parameters and automation.get_curve are read-only; automation.set_points and automation.clear_region are mutations that always pass the run…governance gate…`
- **组 1 三连 no_route 止损（脚本自动停止，组 2/3 未消耗——220 同口径）——形态与 222 不同**：

| 轮 | verdict | 形态 |
|---|---|---|
| r1 | no_route | 话术→1 个模型轮（needs_observation：catalog discovery）→**limit_reached**→「继续」→确定性 settle `no_pending_mix_tick_candidate`「现在没有可执行的待确认混音动作…」（goal completed，观察边界 settle） |
| r2 | no_route | 同型；4 个模型轮（1 前台+3 自动续跑后台）后「继续」被续跑队列吞（automatic_continuation_already_scheduled） |
| r3 | no_route | 同型 r2（3 个模型轮） |

- **工具调用面（三轮合计）**：仅 ccb.observation_catalog/ccb.observation_request——零 automation.*（模型从未到达执行相，非拒绝调用）。
- **根因锚点（闸 2 深层真身）**：变体话术开 minimal audio closure（fs4_diagnostic_round）→ `audioClosureMessageLoopBudget`（`agent/internal/chat/audio_closure_controller.go:438-447`）把每 slice **MaxTurns 压到 1-2**（候选 frontier 未选=1）+ 自动续跑异步泄流（每请求只推进 1-2 轮）+ 观察边界确定性 settle 终结 goal——模型的合法动作集已含 automation（render 亲证）但轮结构使其在烟测窗口（话术+≤3 followup）内无法走完「观察→解析 plugin→读量程→写」链；本卡 headroom 触发器（automation 面首次执行）因此从未点火。**222 卡面对闸 2 的「MaxTurns 8/slice」描述在此修正：真实挤压点是 closure 预算 1-2 轮**（222 的 10 调用/3 轮分布与该形态一致）。
- **LLM 轮次真实计费**（deepseek-flash）：11 调用 / 124,621 in / 11,639 out（+29,696 cache read）；轮面=每轮 ordinary_agent_semantic_entry（696in）+message_loop 决策轮（14.0-16.4K in）。逐调用在 `logs/agent_llm_telemetry.jsonl`。

## 四、定性：越域锚点上交（非模型能力面、非闸 1 缺陷）

- **不是停止条件 3**（三前提齐备仍 no_route=模型能力面）：三前提中之二 render 亲证在场（指引+工具面同场一致，222 的同场冲突已消除）；第三前提（plugin_id 可见）未被执行相触及；模型行为是合规的 catalog-first 探索（改进合同指令面要求），从未获得「证据在手+写机会」的轮。归因=通道轮结构，非判断力。
- **是停止条件 2 形态**（接线需动 chat 核心编排→锚点上交扩域裁定）：修复面在 `chat/audio_closure_controller.go`（closure 轮预算与自动化诉求的配合——候选方向：automation 面在场时 closure 轮预算最小 headroom/续跑调度对写链临界段的原子保留/观察边界 settle 前的 automation 相检查）——全部越出本卡文件域（agentloop），§11 越域即停。
- 本 run 一次到位（无迭代轮）：组 1 止损后脚本按设计停止；§8 纪律下不原样重跑（同一版本同断点已三连同型）。

## 五、次生记录

- 228 的 todo/ 陈旧卡面副本清除（20489009）与本卡领取合并相关——228 会话已处置（防双领取），无遗留。
- 拆栈：owner 手拆 VitAgent 34848/VitApp 18784（pid 路径亲核=D:/Vit_DAW_wt_227/agent/bin/VitAgent.exe+D:/Vit_DAW_wt_228/…/VitApp.exe=本腿所属）→复核 VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787/5178 listen=0、UDP 4444/4445=0。主检出零触碰（VIT_PROJECT_XML 隔离副本承接内核写入）。
- kernel f9e3da0a 运行前后零漂移（只读复用，本腿未重建）。

## 六、成本与端测边界声明

- **覆盖**=闸 1 开放的 agent /agent/chat HTTP 面真实 LLM 轮次（11 调用真实计费）×prompt render 逐 turn 装配取证（8/8 工具面+治理门语义在场亲证）×agent 遥测工具流×内核 ZMQ setup/曲线基线（exit 0）×零裸写治理链单测端到端（确认卡→确认→写入→读回的 agentloop 级全链）。
- **不覆盖**=通道轮结构修复后的组 1 真实通过形态（本腿锚点上交）；组 2/3 话术（止损后未消耗）；choreo pending 腿（未嵌入）；set_points 确认链的**真栈**首次端到端（单测级已验，真栈级待轮结构修复后随重验腿）；webui 渲染面与用户旅程（E2E-WEBUI-1/JOURNEY-1 既有边界）。
- 生产改动=agentloop 五文件+骨架 twin 两处+测试（卡面域内）；scripts/ 零改动（220 断言体原样复用，口径未改）。

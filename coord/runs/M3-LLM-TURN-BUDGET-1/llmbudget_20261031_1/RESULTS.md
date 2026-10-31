# M3-LLM-TURN-BUDGET-1 真栈工件与结果（RESULTS）

- 卡：coord/cards/doing/2026-10-31-M3-LLM-TURN-BUDGET-1.md（池序 229）
- 执行：ZCode GLM-5.3 PC 执行会话（229 卡会话）；worktree D:/Vit_DAW-229 @ port/m3-llm-turn-budget-1
- 领取基线：origin/main=33568012（227 合入后主干），领取提交 9a37df80；实现 commit 见卡面回执
- 命令（220/222/227 同形，断言体 v2 原口径未改，scripts/ 零改动）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW-229/scripts/dev_agent_smoke.ps1 `
  -StartKernel -RestartAgent -AutomationLLMRouteSmoke -NoChatSmoke -NoStripSilenceSmoke `
  -KernelExe D:/Vit_DAW_wt_228/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe `
  -RunArtifactsDir D:/Vit_DAW-229/coord/runs/M3-LLM-TURN-BUDGET-1/llmbudget_20261031_1
```

- run：llmbudget_20261031_1（占用登记 0008b5f1 于 10:59 先于启动；10:49:44 起持续净空亲核+226 释放 2a773e11 在案后接管）；**ps1 exit 0 COMPLETED**；kernel=228 worktree VitApp.exe sha256 F9E3DA0A 运行前亲核（只读复用，本卡 C++ 零改动，227 同形）。
- 一句话结论：**预算面（本卡标的）修复兑现并经 LLM 真轮验证——limit_reached 泄流形态消失（227 每轮 1-4 调用被截停 → 本轮 34 调用/轮内完整走 catalog→视图请求→轨道选择→轨道级观察）；但组 1 三连 no_route 止损在全新更深形态复现：阻塞点从预算层推进到观察证据层——裸轨工程（无音频 clip）的 acoustic package 状态=missing（l1_static 全 missing）→ 派生观察（频段动态/电平 levels/多轨关系）源证据缺失 → FS 准入门（G3/G5）无法满足 → 模型 capability_blocked 诚实停边。观察证据面越出本卡文件域（agentloop G 门/harness 观察层/scripts 工程形态），锚点+证据上交主管裁定。**

## 一、链路进度（对照 227 形态）

| 维度 | 227（llmroute3） | 本轮（llmbudget） | 结论 |
|---|---|---|---|
| LLM 调用/轮 | r1=1、r2=4、r3=3（每片 1-2 轮即 limit_reached） | **r1=8、r2=12、r3=14**（34 总调用） | 预算扩面生效（mode 基础 10 turns） |
| 截停形态 | limit_reached → waiting_continue → 异步续跑泄流（每片 1-2 轮） | **零 limit_reached 终局**；r2/r3 链满预算后异步续跑接管（设计行为），脚本「继续」期间见 automatic_continuation_already_scheduled | 泄流形态消失 |
| 模型行为深度 | 零 automation 调用（catalog discovery 即被截停） | catalog→observation_request→**轨道选择（Track 2/1010）→轨道级观察请求**→证据评估→capability_blocked 诚实申报（不瞎写不绕道） | 工具面+预算面四前提中三就位且被真实使用 |
| 工具调用面 | 仅 ccb.catalog/request（截停前） | 同 ccb 两工具（观察层）；automation 四工具已在 Allowed tools（render 17,045B 不变）但观察证据不足使模型合规地停在准入门——**未到执行相，非模型不会用** | — |
| verdict | no_route×3 | no_route×3（**新形态**：capability_blocked 纯文本申报未被脚本 boundary 词表匹配） | 阻塞点下移一层 |

## 二、最终验收（验收门判定）

- **组 1 no_route 消失：未达**（三连 no_route 止损，脚本按设计停止，组 2/3 未消耗）。
- **set_points 确认链端到端首绿：未达**（模型未到执行相）。
- ps1 exit 0=验证卡完成语义（执行到止损=完成），非验收门通过。

## 三、新根因锚点（观察证据层，越域上交）

1. **acoustic package missing**：`.vit_derived/.../acoustic_package_status.json`（已摘录 evidence/）`status="missing"`、l1_static 全部 missing、`source=mix.observe_read_first`——两根裸轨（无音频 clip）→ 无 clip 语义内容 → 声学分析无源 → 派生观察不可用。
2. **观察返回不可用**（r3 模型终局申报原文，evidence/free_state_loop_decisions.json）：「针对当前选中的 Track 2（编号 1010）……频段动态观察返回不可用（源证据缺失），电平观察的 levels 读取失败，多轨关系只返回部分结果（响度顺序、峰值顺序缺失）」。
3. **FS 准入门 G3/G5**：qualified scan/frontier 观察证据要求在无音频工程上结构性不可满足 → capability_blocked。r1 的「工程标识不一致」表述为模型对观察拒绝的归纳（工程级观察同样 missing），与 r3 同根因。
4. **结构性矛盾**：写 volume 渐变（set_points）在工程结构面（轨在场+插件在场）即安全可执行，不需要频段/电平音频证据；但 FS 准入门要求的观察证据在裸轨工程上必然缺失——**测试工程形态（220 断言体 setup=两根裸轨）与 FS 准入语义的交互问题**，非预算面、非闸 1、非模型能力面（模型行为=终态架构期望的诚实停边）。

## 四、上交选项（主管裁定，非本卡处置）

- A. **测试工程补音频 clip**（scripts setup 域——220 断言体判定逻辑不动、setup 工程形态变更需口径裁定）；
- B. **FS 准入门对无音频工程的适配**（agentloop G 门语义：结构面证据（轨+插件在场）足以准入最小可回退写试验——越域）；
- C. **判定面口径**：模型 capability_blocked 诚实申报在 220 词表下= no_route（boundary 词表未匹配申报文案）——判定面窄化问题，属断言体口径域（本卡禁改，交主管）。

## 五、实现面与测试（全绿，已在 port 分支）

- `audioClosureMessageLoopBudget` 意图感知分配：识别面 `audioClosureAutomationRoundContext`（与 227 `admitFreeStateDecisionRoundToolSurface` 同结构判定）命中即保留 mode 基础预算（goal 10 turns/16 calls），预算三面成文（来源=工具面同源/上限=mode 基础不叠加/衰减=识别面窄+治理门与每轮一 mutation 限速不放松）；诊断轮与非 FS closure 轮观察 slice 语义零回退；续跑片同函数同判定继承同预算语义（面 2）；settle 补轮不开（面 3，227 定案维持）。
- 新增 `audio_closure_turn_budget_test.go` 六面；chat+agentloop 全绿；96 包全量回归零失败。

## 六、§8 计费与失败分类

- LLM 轮真实计费：34 调用（r1=8/r2=12/r3=14，deepseek-flash；telemetry 逐调用在 logs/agent_llm_telemetry.jsonl；本遥测面 usage 字段空，token 计量以 provider 账单为准，回执如实披露此限制）。
- 失败分类：no_route×3（同型新形态：capability_blocked 申报）；零 env_interrupt、零 confirm_broken、零 write_fail、零 lexicon_hit。
- 一次到位无迭代轮：组 1 止损后脚本按设计停止；§8 纪律下不原样重跑（同版本同断点三连同型，且根因已定位为结构性观察证据缺失，非偶发）。

## 七、覆盖边界声明

- **覆盖**=预算面修复的真实 LLM 验证（34 计费调用×轮内多步链×工具面 render 17,045B 恒定）+ 组 1 三轮完整工件取证 + acoustic/loop 证据摘录。
- **不覆盖**=组 2/3 话术（止损后未消耗）；观察证据面修复后的组 1 真实通过形态（越域上交后由后续卡承接）；webui 渲染面与用户旅程（E2E-WEBUI-1/JOURNEY-1 既有边界）。
- 拆栈：owner 手拆 VitAgent 34740/VitApp 9236（pid 路径亲核=本腿所属：D:\Vit_DAW-229\agent\bin\VitAgent.exe+wt_228 内核）→复核 VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787/5178 listen=0、UDP 4444/4445=0。kernel F9E3DA0A 运行前后零漂移（只读复用）。派生态剔除按 218/222 先例（关键证据摘录 evidence/ 后剔除 .vit_agent/.vit_history/.vit_derived/工程文件）。

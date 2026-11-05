# M3-TIMEWINDOW-INTENT-ARM-1：时间窗话术确定性武装（组 3 修复，词表扩展）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=251 验收裁定 §二：组 3 断点层=语义入口仲裁器、措辞链不可达+主管诊断：缺的武装门在确定性词表；用户 2026-11-05 裁定「继续往后测试和修复，最终一并手测」）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 253；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=251 已在 main（0e58f3a3）✓；L1 / PC 执行会话；验证腿需真栈（一次）

## 背景与诊断（发卡侧已勘，锚点可直接用）

251 定案：「30 秒之后动起来」全落到 LLM 仲裁器被判 unresolved（12/12，模板回复）。主管亲勘 `agent/internal/chat/automation_intent.go`：

- 武装门=`TimeVarying qualifier + (语义结果词|处理动词)`（automationIntentArmed:119）；
- 「秒之后/秒后」**不在** `automationTimeVaryingQualifiers`（:20，全是渐变词）；「动起来」**不在** `automationTreatmentVerbs`（:60）——永不武装；
- 而**区域层时间窗机制已存在**：BareSpanSeconds（「30 秒」裸跨度）→ clarification 通道给可确认候选窗（:142 注释，M3-C4-AUTO-CLARIFY-UX-1 建）；澄清卡 StopReason=`automation_choreo_clarify_*`+workflow=`section_automation_choreo`（automation_choreo.go:35/512）——**均在 smoke lexicon_hit 判定面内**（脚本 looks_lexicon_hit 两条件之一）。

即：只差词表两处补词，武装后走既有澄清卡链路即达组 3 成功形态 lexicon_hit（该形态零 LLM 调用）。

## 目标

1. **词表扩展**（最小面，词表=路由权威）：
   - `automationTimeVaryingQualifiers` 增补时间窗标记：`秒之后`、`秒后`（评估是否含 `秒之内`/`到 N 秒` 窗式；禁止裸 `秒` 单字入表——会把「等 30 秒再说」类误武装）；
   - `automationTreatmentVerbs` 增补 `动起来`（窄词，禁止裸 `动`——「别动/动不了」反例）；
2. **单测**（正例+负例成对）：
   - 正：`30 秒之后动起来` 武装（category 空→澄清卡问参数）；`30 秒之后调整一下` 武装；
   - 负：`把这条轨存在感调强一点`（静态路径不变）、`30 秒`（裸跨度无动词不武装）、`别动`、`帮我把空间铺开`（无时间词不武装——**组 2 判别基线不动**）、`等 30 秒再说`；
3. **真栈验证**：`-LlmRouteGroups "1,3"` 单 run（deepseek-flash 注入+载荷全开，-KernelExe 显式）——**组 1 必须保持绿**（回归门槛，tool_chain_ok 组判据过）+组 3 预期 `group_pass=true`（lexicon_hit 确定性达成，5/5 或 ≥4/5）；组 3 轮 lexicon_hit=零 LLM 损耗，预算≈组 1 五轮+setup/choreo 腿 ≈**¥2-3**。

## 约束

- 文件域=`agent/internal/chat/automation_intent.go`（+如武装需接线：automation_choreo.go 最小面）+chat 包测试；**禁触 semantic_entry.go**（仲裁器不在本卡域——251 判别结论维持：措辞链之外的杠杆本卡只补确定性面）；
- pre-model takeover 哲学不变：路由靠词表不靠 LLM；category 未知→澄清卡问，不猜（既有设计）；
- 组 2 行为不得变化（其判别基线=251/249 结论）；组 1 needle 面不得回退（载荷 needle 亲证）；
- §5 门槛：真栈 run exit 0=交付门槛；§8 单 run 无重跑。

## 停止条件

- 组 1 回归红→立即停（本卡首要红线=不破坏稳定绿），取证上交；
- 词表扩展引发负例误武装→收窄词表；仍不止→取证上交；
- 澄清卡链路意外形态（新断点）→只取证不扩域。

## 领取

- 2026-11-05 18:37 +0800；领取时 origin/main=c7c22e432abf428b1ae3f1862feec9eeb620fb82（=本地 main=HEAD；依赖 251 验收合入 0e58f3a3 已在 main ✓）
- owner=ZCode GLM-5.3-Flash L1 PC 执行会话（253 卡会话）；机器=PC；分支=port/m3-timewindow-intent-arm-1；worktree=D:/Vit_DAW_wt_253impl（自 c7c22e43 新建）
- 领取时主检出状态（非本卡改动，本卡不触，留档备查）：8 个 tracked 脏文件=M VitApp/Workspace/Settings/Settings.xml、M VitApp/Workspace/default_project.xml、D coord/cards/blocked/2026-10-30-M3-CHAT-AUTH-STAMP-1.md、M coord/cards/done/2026-10-30-M3-CHAT-AUTH-STAMP-1.md、M×3 coord/runs/UX-AB-AUDITION-IMPL-1/aba174_chat_20261022_233614/{driver_console.txt,logs/agent_llm_telemetry.jsonl,logs/agent_prompt_render.jsonl}、M coord/runs/UX-AB-AUDITION-IMPL-1/assert_aba174.py；另有 untracked 若干（design png/html、reports 等）。diff --stat 汇总=8 files changed, 4469 insertions(+), 4429 deletions(-)
- 栈登记：真栈腿执行前按 §2.2 另行登记占用（登记时最近释放=2026-11-05 10:50:22，251 收口 TEARDOWN_ALL_ZERO）

## 回执

- 实现 commit=`0593e05a`（port/m3-timewindow-intent-arm-1；唯一 diff=automation_intent.go +8/-1+automation_intent_test.go +35，2 files changed）；工件 commit=`1a5270bc`（同分支）
- 本地验证：`go build ./...` OK；`go test ./internal/chat -count=1` 全绿（127.152s 存量零回退）；`TestAutomationIntentTimeWindowArming`（正 3/负 5 按卡面清单）+`TestAutomationIntentArmingBoundary` verbose PASS
- 真栈单 run（§8 无重跑）：run ID=`automation_llm_route_20261105_184935`，18:49:35 启动（占用登记 f2719977 推送+owner 重读+≤60s 即时复核 PROCS/端口=0 后）；命令=dev_agent_smoke.ps1 -RepoRoot D:\Vit_DAW_wt_253impl -StartKernel -RestartAgent -AutomationLLMRouteSmoke **-LlmRouteGroups "1,3"** -LlmModelOverride deepseek-flash -KernelExe（wt_228 VitApp.exe SHA256 F9E3DA0A66263F0B…D67A 18:45 亲核）；python 腿 [DONE] status=COMPLETED exit=0（19:03:35）；**ps1 PS1_EXIT=0（§5 交付门槛过）**；Strip Silence 前置腿全绿零 warn
- 组 1 回归证据（首要红线）：llm_samples=5，verdicts=[no_route, tool_chain_ok×4]，successes=4，**group_pass=true**，choreo_pending_leg=pending_intact——红线绿；r1 no_route=模型行为样本如实记录（组判据内，取证件 rounds/g1_r1.json 留 worktree，不扩域）
- 组 3 逐轮 verdict：r1-r5 **全 lexicon_hit**（counts_as_llm_sample=false 零 LLM 判定轮），stop_reason=`automation_choreo_clarify_category`+workflow=`section_automation_choreo`，successes=5/5，**group_pass=true**，pending_intact——词表补词后「30 秒之后动起来」首轮确定性武装走既有澄清卡，与卡面预期形态逐字一致；251 全仲裁 unresolved 形态消失
- payload needle 面：payload_guidance/payload_automation_surfaces/payload_param_surface 全 available=true+prompt_guidance_rendered=true（未回退）
- 计费：90 调用全 deepseek-flash in 1,376,092/out 121,208（telemetry 覆盖 ps1 全程）≈¥1.7-2.3（基准=251 口径），预算 ≈¥2-3 内
- 拆栈：owner 手拆 VitAgent(21444)/VitApp(31936)，**TEARDOWN_ALL_ZERO 亲核 19:09:05**（PROCS=0+TCP 7878/5555/5556/8787/18787=0+UDP 4444/4445=0）
- 工件：coord/runs/M3-TIMEWINDOW-INTENT-ARM-1/armrun1/（RECEIPT.md+summary slim 7.6KB+telemetry 154KB+双 console）随 **port/m3-timewindow-intent-arm-1 @ 1a5270bc** 入库；大件（payload 6.5MB/prompt_render 3.6MB/summary 全量 3.2MB/rounds 2.9MB）留 worktree run 目录；clip.wav 已删
- 端测覆盖边界声明：真栈腿覆盖 Groups 1,3 全判定面；**组 2 话术未在本 run 重跑**（判别基线维持 251/249 结论，单测负例「帮我把空间铺开」守其入口不变）；webui 渲染面/用户旅程不在本卡域
- 词表评估记录：「秒之内」「到 N 秒」窗式**未入表**（卡面授权评估项）——本卡正例矩阵无覆盖+窗式需 regex 而非 substring，扩面不扩验证面违反最小面；如后续话术家族需要另立卡
- 终验=GLM 主管决策流

## 验收

- **2026-11-05 pass 合入**（GLM 主管决策流）——[2026-11-05-M3-TIMEWINDOW-INTENT-ARM-1-pass.md](../../rulings/2026-11-05-M3-TIMEWINDOW-INTENT-ARM-1-pass.md)：实现最小面亲核（词表两处+测试，零接线未触仲裁器）+单测 worktree 亲复跑双 PASS（负例全绿）+真栈判定面亲读（组 1 回归绿 4/5+组 3 lexicon_hit 5/5+needle 82/90 旧句 0）+96 行全 flash ¥2.3 卡面内+栈两笔在册。**组 3 修复定案，M3 链 229→253 全线收口**。

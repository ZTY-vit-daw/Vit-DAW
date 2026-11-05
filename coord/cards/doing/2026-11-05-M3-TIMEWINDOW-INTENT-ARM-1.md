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

- （执行侧回填：完整命令/退出码/run ID/HEAD/计费/判定/组 1 回归证据/组 3 逐轮 verdict）

## 验收

- （主管回填）

# M3-TERMINAL-ROUTE-WORDING-1：终局轮直调形态补全（面二措辞四项，含设计裁决落地）

- 发卡：GLM 主管决策流 / 2026-11-04（依据=244 勘察报告 §2.2/§2.3/§2.4+裁定 [2026-11-04-M3-BREAKPOINT-FORENSICS-1-pass.md](../rulings/2026-11-04-M3-BREAKPOINT-FORENSICS-1-pass.md) 设计裁决）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 246；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=245 已合入（终局轮披露在场是本卡判据的前置）✓ 待 245 验收后领取；L1 / PC 执行会话
- **并行注记（§2.2 ③）**：与 245（M3-CHAT-HOST-CARRY-FIX-1）文件域不相交；真栈腿**错峰**——245 先跑，见其释放登记后再登记启动本卡栈腿。

## 设计前提（主管已裁，用户可推翻）

时变目标**不入 admitted 提案词表**（2026-10-31 vocabulary-full-retirement+automation 工具链设立初衷）；本卡全部为措辞/示例/规则句修改，不新增提案域、不动准入/治理/执行代码。

## 目标（四项措辞，全部句级 add/replace）

1. **终局指令补第三形态**：`freeStateTerminalTurnSentence`（`agent/internal/agentloop/free_state_reasoning.go:792`）+渲染（`ccb_model_prompt.go:420-444`）——终局二选「a bounded improvement proposal, or an explicit terminal statement of where you are blocked」补第三形态：**全写要素在手时直接发起 mutation 调用（automation.set_points），运行时治理门自动拦成确认卡**；与 v2 触发权句同语义同源。
2. **时变形态示例**：`ccb_model_prompt.go:130` v2 Action Path 段邻接示例块——补一个 `final:true`+`automation.set_points` tool_call（plugin_id/param_id/points 合法形态）的时变示例，消除「唯一 final 示例=静态提案」的邻接缺陷。
3. **确认语义桥接句**：v2 句邻接补一句——时变目标走直调后由运行时门拦卡，确认卡内自然可核对时间/数值，**无需文本预问**（桥接提案世界确认门与直调世界不预问的双语义）。
4. **目标级规则句时变豁免**：载荷节区「Once the target-level evidence … return final=true needs_experiment …」（报告 §2.2 #3，落点由执行侧按载荷锚点定位：ccb 组装或 ruleset section）加时变豁免子句（time-varying goals 走 automation 直调，不走该句）。

## 同源锁链纪律（243 先例）

涉 ruleset sections（neutral_family.discipline/chat.discipline.plugins_mixing）与 `agent/internal/chat/server.go` fallback 的同源行按 parity 测试强制同步；`automation_llm_route_smoke.py` needle 同步（新句在场+被顶掉的旧二选句计数）；版本号纪律（涉及段落 CacheKey/version bump）。

## 判据与验收标准

- **单测**：新句锚+被替换旧句负断言（四面同步面）+needle 同步；缺省组行为不变。
- **真栈腿**（-Groups 1 单组一次，deepseek-flash）：判据同链口径——**组 1 绿（no_route 消失+tool_chain_ok set_points 包络形态）**或新断点取证包；载荷亲证（新句/新示例在场+被读：response 引用或行为转变）；栈登记/释放全流程。
- §8 预算 ≈¥1.5-3；计费披露；端测边界声明（无 webui 渲染面改动则声明不适用）。

## 停止条件

- 措辞破坏既有测试（零回退红）→回退两案上交；
- 组 1 仍不绿→按链口径出新断点取证包上交，不原样重跑不自修；
- 目标级规则句落点涉及非预期文件域→停上交。

## 文件域

`agent/internal/agentloop/ccb_model_prompt.go`+`agent/internal/agentloop/free_state_reasoning.go`+`agent/internal/agentloop/neutral_automation_route.go`（如涉）+ruleset sections 同源文件+`agent/internal/chat/server.go` fallback+`scripts/automation_llm_route_smoke.py`（needle）+测试。越域即停上交。

## 领取

- （执行侧回填：会话/worktree/分支/领取基线 HEAD+status+diff --stat/栈登记）

## 回执

- （执行侧回填）

## 验收

- （主管回填）

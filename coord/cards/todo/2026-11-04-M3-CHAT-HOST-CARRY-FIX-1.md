# M3-CHAT-HOST-CARRY-FIX-1：终局轮披露跨 run 边界存活（面一 T1）+前台吞回执升级与终局补投递（面三 T1）

- 发卡：GLM 主管决策流 / 2026-11-04（依据=244 勘察报告 [2026-11-04-m3-breakpoint-forensics-1.md](../reports/2026-11-04-m3-breakpoint-forensics-1.md) §1.2/§1.4/§3.1/§3.3+裁定 [2026-11-04-M3-BREAKPOINT-FORENSICS-1-pass.md](../rulings/2026-11-04-M3-BREAKPOINT-FORENSICS-1-pass.md)）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 245；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=244 报告已在 main（319e3444）✓；L1 / PC 执行会话
- **并行注记（§2.2 ③）**：本卡与 246（M3-TERMINAL-ROUTE-WORDING-1）同时派发，文件域不相交；两卡均含真栈腿——**错峰执行，本卡先跑**，246 见本卡释放登记后再登记启动。

## 目标（三面，全部确定性判据）

1. **面一 T1 披露跨边界存活**：`agent/internal/chat/free_state_reasoning_loop.go`——`freeStateReasoningLoop` struct 增 `AutomationSurfaces []map[string]any` 字段（json tag `automation_surfaces,omitempty`）+`freeStateLoopFromAny`/`freeStateLoopMap`/`cloneFreeStateLoop`/`mergeFreeStateLoops` 对应携带与合并分支（沿用行合并+`project_revision` 严格前进过期语义与上限 `freeStateAutomationSurfaceLedgerLimit=8`，锚点见报告 §1.2 步骤 1/3）。
2. **面三 T1a 吞回执升级**：`agent/internal/chat/goalrunner_chat.go:194-204` 邻接——`automatic_continuation_already_scheduled` 回执的 `WorkflowData` 携带 pending continuation 当前状态（attempt/status 之外补最近一次终局/观察回执摘要，pendingmanager 已有面可复用则复用，不新建协议）。
3. **面三 T1b 终局补投递**：goal 终局落状态处（`goalrunner_chat.go:3359-3381` 记账邻接）对原 conversation 补投递一条终态摘要回执（capability_blocked/needs_experiment+提案概要，一段文本，无新 UX 面）。

## 判据与验收标准

- **单测**：①round-trip 保键（map 含 automation_surfaces 行→`freeStateLoopFromAny`→字段在→`freeStateLoopMap` 回写键在）②merge 语义（revision 前进过期+上限 8）③吞回执携带状态断言 ④终局补投递断言；既有测试同步（回执文本/WF 数据变化影响的断言面）。
- **真栈腿**（-Groups 1 单组一次，deepseek-flash，载荷捕获全开）：**确定性 needle=终局轮请求载荷含 1011/0.740818**（模型行为面只记录不设门）+「继续」回执携带状态+终局摘要投递可见。脚本退出码 0；栈登记/释放全流程+TEARDOWN_ALL_ZERO。
- §8 预算 ≈¥1.5-2（单组单 run，止损三连同型自动中止沿用）；计费披露。

## 停止条件

- needle 仍缺披露（终局轮载荷无 1011/0.740818）→不扩大修复面，取证包上交（含边界导入链路逐点 dump）；
- 修复破坏既有测试（零回退红）→回退两案上交；
- merge/投递触点超出文件域→停上交。

## 文件域

`agent/internal/chat/free_state_reasoning_loop.go`+`agent/internal/chat/goalrunner_chat.go`+chat 包测试。越域即停上交。

## 领取

- （执行侧回填：会话/worktree/分支/领取基线 HEAD+status+diff --stat/栈登记）

## 回执

- （执行侧回填）

## 验收

- （主管回填）

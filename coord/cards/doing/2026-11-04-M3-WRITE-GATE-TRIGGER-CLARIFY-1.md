# M3-WRITE-GATE-TRIGGER-CLARIFY-1：写入确认流触发权澄清（骨架措辞因果修正）+组 1 定向重跑

- 发卡：GLM 主管决策流 / 2026-11-04（依据=242 验收新真断点定性（754db257 合入）+写入链路畅通 choreo 腿亲证+g2 同类确认面聚合）
- 派发确认：待确认（链内续卡，用户在场时口头确认即可开工）
- 验收负责人：GLM 主管决策流
- 池序 243；目标仓库=**主仓 agent**；P1 / 0.5 天 / 依赖=242 已在 main（754db257）✓；L1 / PC 执行会话

## 标的（242 验收+主管亲核，不重考）

- **断点形态**：模型持有全部写前要素（范围+当前值 0.74+时间窗 0-6s+完整写入计划，载荷逐字亲证）但停驻**纯文本征询确认**——五样本统一形态（「提交后会在确认卡里等你确认」/「写入前会弹出确认卡」/「你确认后即可试听」），agent 面 `needs_confirmation=False`（无 pending candidate 形成）。
- **设计事实**：free_state_automation_surface.go 头注「触发权=模型工具调用，与 Claude Code permission gate 同构」——模型**调用** set_points 时治理门自动拦截成确认卡；纯文本征询不形成任何卡。写入链路畅通由 242 choreo settle 腿亲证（真实写入+两点读回 1e-06）。
- **根因假说（主判）**：骨架措辞因果倒置可误读——ccb_model_prompt.go:123「call automation.set_points … it passes the runtime governance gate (user confirmation card) **before any write**」+ :714「mutations that always pass the runtime governance gate (a user confirmation card, or direct execution only under an explicit full-access grant) **before any write**」——模型可解读为「先取得用户确认、再调用」，实际语义是「直接调用；运行时自动拦卡/全权限下直执」。
- **同类聚合**：组 2 confirm_broken×3（plugin_selection_required，外部插件候选确认面停驻）=同类「不发起触发动作、停在文本征询」形态，本卡措辞修正为共享修复候选（观察面非门）。

## 目标（四面）

1. **措辞因果修正**：ccb_model_prompt.go 两处+neutral_automation_route.go 指引——明确「**make the mutation call directly when you hold all write elements; the runtime gate pauses it into a confirmation card automatically (or executes directly under an explicit full-access grant); a plain-text request never forms a confirmation card — do not pre-ask in text**」。红线：既有 needle token 不破坏（涉及 smoke needle 同步则按文件域条款微扩）；措辞为 add/replace 句级修改，不动准入/治理/执行代码。
2. **组过滤重跑**：smoke 脚本加组过滤参数（如 `-Groups 1`，仅跑指定组；缺省行为不变）——242 超预算根因（组间自动推进）的流程修复；重跑**仅组 1**，同 fixture 同引擎（**deepseek-flash**），载荷捕获+needle+param_surface 全开。判据同链口径：组 1 no_route 消失+`tool_chain_ok`（set_points 包络形态）。
3. **g2 观察腿（非门）**：门过且预算余量时以 `-Groups 1,2` 形态跑组 2 观察 plugin_selection_required 是否同解（如实记录，非验收门）。
4. **单测**：措辞 needle 断言同步（新句在场+旧误读句已替换）+组过滤参数单测/手测（缺省行为不变）。

## 预期与诚实边界

本卡=指引层修正（模型行为面，概率性），§8 声明：预算 1-2 轮仅组 1（≈¥1.5-3 含 g2 观察腿）；成功条件=组 1 绿（no_route 消失+tool_chain_ok）或新真断点取证（若仍停确认面且载荷亲证新指引在场被读→升级为接线问题勘察 pending candidate 形成路径，上交不自修）。**组 1 绿→链收口**（组 2/3+统一手测转正）。

## 停止条件

- 措辞/needle 改动破坏既有测试（零回退红）→回退两案上交；
- 组 1 绿→链收口，不开新卡；
- 仍停确认面+新指引载荷亲证在场被读→接线问题取证包上交（勘察 pending candidate 路径），不原样重跑；
- LLM 面栈不可用→环境中断如实记录。

## 文件域

`agent/internal/agentloop/ccb_model_prompt.go`+`agent/internal/agentloop/neutral_automation_route.go`+`scripts/automation_llm_route_smoke.py`（组过滤+needle 同步）+测试。越域即停上交。

## 验收标准

- 单测+相关包全绿（缺省组行为不变断言）；
- 真栈重跑腿：场景腿 COMPLETED+载荷亲证（新指引句在场+被读[response 引用或行为转变]）+组 1 判据或新断点取证包；栈登记/释放全流程；§8 计费披露（仅组 1 定向）。

## 领取

- 2026-11-04；origin/main=d24878af（含 754db257 ✓）；owner=GLM-5.3 PC 执行会话（ZCode，用户当面派卡确认）
- 分支 port/m3-write-gate-trigger-clarify-1；worktree D:\Vit_DAW_wt_243impl；领取 worktree D:\Vit_DAW_wt_243claim（coord/243-claim）
- 领取基线：HEAD=d24878af；status=clean（新检出 worktree）；diff --stat=空
- 真栈占用：实现完成后按 §2.2 登记（见回执）

## 回执

- （执行侧回填）

## 验收

- （主管回填）

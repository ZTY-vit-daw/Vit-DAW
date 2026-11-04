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

- **实现 commit b6cbc007 + 工件 commit f139165e（port/m3-write-gate-trigger-clarify-1）**；HEAD=b6cbc007 时 status 仅本卡 diff（9 文件 +135/-28）。
- **目标 1（措辞因果修正）**：实际修正面=卡面两处之外同文件第三处同模式误读源一并修正——ccb_model_prompt.go 三处（L123 action path v1→v2 直接调用+运行时自动拦卡+纯文本不成卡不预问；catalog 目录行；legacy 模板 L714 位）+同源锁链强制三跳：agent/rules/ruleset/resources/sections/neutral_family.discipline.rules.txt（parity 字节锁）+chat.discipline.plugins_mixing.txt 同源行（TestNeutralFamilyAutomationRouteMatchesChatRulesetLine 锁）+agent/internal/chat/server.go chat fallback 模板（chat 包 ruleset_parity_test 锁）+neutral_automation_route.go 指引追加触发权句+CacheKey v1→v2。文件域微扩披露：ruleset 两个 txt+server.go 三文件为同源/parity 锁实现卡面目标的必然伴随面（与 needle 同步条款同精神）；卡面「两处」为发卡时勘察计数，实际同文件三处同模式。
- **目标 2（组过滤重跑）**：automation_llm_route_smoke.py 加 -Groups/--groups（parse_groups 纯函数校验：未知/空/非数值拒绝，缺省 "1,2,3" 全组行为字节不变——手测断言全 PASS+CLI 形态验证）+trigger needle 判读扩展（rows_with_trigger_clarification+rows_with_old_clause 计数）+dev_agent_smoke.ps1 -LlmRouteGroups 透传（文件域微扩：真栈驱动通道）。真栈 writegate1：-LlmRouteGroups 1 单 run 亲证（groups_requested=[1]+组 1 外零执行）。
- **目标 3（g2 观察腿）**：未跑——门未过+预算超线，卡面「门过且预算余量」双条件不满足。
- **目标 4（单测）**：action_path 测试锚集更新（新句锚×4+before any write 负断言）+automation_surface 测试（skeleton/catalog/legacy 三面新锚+负断言）+全量 go test ./... 零 FAIL（agentloop/ruleset/chat 重点三包复验绿）。
- **真栈判定（组 1 门未过→新断点取证包上呈，按停止条件 3 精神收口）**：verdicts=[no_route,no_route,no_route] 三连同型止损自动中止（非跑满）；**242 断点形态消失**（三轮零文本征询话术+零 set_points 尝试）+**新指引句在场被读亲证**（157/162 计费行 v2 句+0/162 旧句残留+r1 行为转变「好把渐强包络的点落在有效区间内」）；**新真断点双形态**：①r1 观察回执不满足循环（40/42 轮 needs_observation；get_parameters 带调用仍申报「范围未读取」；终局轮 capability_blocked 申报缺标识+时间跨度与中途持有 0-6s 区间矛盾）②durable continuation 轮间污染（r1 goal 挂起→r2/r3 前台 already_scheduled 吞+后台 84/34 调用空转）。归因候选（接线问题勘察面）：pending candidate 形成路径+观察回执跨轮投影可见性+continuation goal 存活窗口与 smoke 逐轮新对话的交互。不自修、不原样重跑。
- **§8 计费**：162 调用 in 2,555,882/out 252,212 ≈¥3.1 上界（242 同口径 ¥1.103/百万 token）——超卡面 ¥3 上界如实披露：r1 单轮 43 调用长链+continuation 后台空转 118 调用为主因；三连止损自动封住后续花费；单 run 无重跑。
- **栈**：占用登记 0930b61e（登记前亲核全零 18:24+push 后 18:28:01 ≤60s 即时复核全零）；拆栈 TEARDOWN_ALL_ZERO 亲核 19:04:13（PROCS=0+TCP 5555/5556/7878/8787/18787/5178=0+UDP 4444/4445=0）；释放登记随本批推 main。
- **工件**：coord/runs/M3-WRITE-GATE-TRIGGER-CLARIFY-1/writegate1/（RECEIPT.md 判读+summary/console/rounds g1×3+telemetry 162 行+payload_capture 168 行 12M+driver_console 含 PS1_EXIT=0+head=b6cbc007）；prompt_render 6.2M 留 worktree 不入库（242 先例）；clip.wav+.vit_* 已删（240 教训）。端测边界声明：本卡无 webui 渲染面改动（AGENTS §5 渲染面条款不适用）。

## 验收

- （主管回填）

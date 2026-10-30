# M3-LLM-NEUTRAL-EMBED-1：neutral 装配分支补 automation 路由指引（220 停止条件 2 部署缺陷修复）+重验

- 发卡：GLM 主管决策流 / 2026-10-30 晚（依据=220 验收定案：render 逐行 10/10 变体 turn 亲证 `message_loop_neutral_family_selection` 装配分支 system prompt 不含 217 automation 指引，embed 双插入只覆盖完整装配路径；RESULTS §四折卡建议①）
- 派发确认：待确认（220 用户质询「连入 LLM 后生效吗」的未竟面=修复后重验，属用户既有诉求延续）
- 验收负责人：GLM 主管决策流
- 池序 222；目标仓库=**主仓 agent**；P1 / 0.5 天 / 依赖=220 工件已合入（732b5c35）✓；L2 / PC 执行会话（装配链判断+LLM 重验判读）

## 标的（220 定案，不重考）

词表未命中的变体话术进 message loop 走 neutral family selection 装配路径，该路径 system prompt **不含** 217 的 automation 路由指引行（server.go:6309 指引+embed 双插入只覆盖 message_loop_system 完整装配）→ 模型从未收到指引 → 观察探索后纯文本（no_route）。render 证据：`coord/runs/M3-C4-AUTO-LLM-ROUTE-VERIFY-1/llmroute_20261030_{1,2}/logs/agent_prompt_render.jsonl` 逐行可回指。

## 目标（两面）

1. **修复**：neutral family selection 装配分支补 automation 指引插入——与 217 既有 embed 同源同文（不新写文案），插入点勘察定案（agentloop 装配路径 vs server.go 双插入点扩展，选最小侵入面）；确认话术分支（message_loop_system）指引在场形态零回退。
2. **重验**：复用 220 断言体 v2（scripts/automation_llm_route_smoke.py）跑三组×5 轮（口径承 220 §8 成文：组级 ≥4/5、失败五分类、同型三连止损、环境中断须原始日志）；**组 1 先行**（同话术「这段让它越来越突出一点」），指引在场形态下 no_route 应消失。

## 次生评估面（勘察后定进退）

观察面剥轨级插件（模型视角「Track 无插件」vs 内核真值轨级 Volume&Pan 在场，207 紧凑面同族）——若修复面在 agent 观察投影域内（同卡可做）则一并披露轨级插件；若需动内核 VSP 紧凑面→锚点上交另拆卡，不阻塞面 1/2。

## 文件域

`agent/internal/agentloop/`（装配路径）+`agent/internal/chat/server.go`（指引插入点）+`scripts/automation_llm_route_smoke.py`（重验断言复用，如需扩）+测试。越域即停。

## 停止条件

- 勘察发现 neutral 分支指引插入会造成消息类型路由回归（确认话术分支行为变化）→锚点+两案权衡上交；
- 重验组 1 指引在场仍三连 no_route→属模型能力面，锚点上交转 LLM 路由设计讨论，不盲目迭代 prompt。

## 领取

- 时间：2026-10-30 21:24
- 领取基线：本地 main HEAD `80d4a22c`（origin/main 当时=`4a3a4746`，落后 4 个主管 223/224 收口提交，随领取提交 fast-forward 同步；依赖 732b5c35 亲核为 origin/main 祖先 ✓）
- 领取时 HEAD/status/diff --stat：HEAD=80d4a22c；worktree 干净（主检出已有改动=VitApp/Workspace 运行时文件+UX-AB-AUDITION run 工件，与本卡文件域不相交）
- owner：ZCode GLM-5.3 PC 执行会话（222 卡执行流）
- 分支：`port/m3-llm-neutral-embed-1`；worktree：`D:\Vit_DAW_wt\m3-neutral-embed`
- 真栈腿：错峰承袭，领取后核 PC-RUNTIME-STACK.md（224 已拆栈空闲）；占用前按 §2.2 登记
- 领取提交：见 main 提交历史（仅含 coord/ 卡状态变更）

## 定位升级（2026-10-30 晚，判断权归属裁定后）

本卡升级为**前置卡**：neutral 指引=LLM 判断层地基，225（澄清上移 LLM）硬依赖本卡完成。优先级维持 P1，领取排序前移（223 后即可）。

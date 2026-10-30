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

## 回执（执行侧自验完成，待主管验收）

- 实现 commit：**port/m3-llm-neutral-embed-1 @ de476086**（修复四文件）+ dd9b2d11/f4c8f82f（run 工件+运行时派生态剔除）；工件=coord/runs/M3-LLM-NEUTRAL-EMBED-1/{RESULTS.md,llmroute2_20261030_1}。
- **面 1（修复）兑现**：`neutralFamilySystemSections` 两形态（bundle 主路径 L1 域后插入+回落骨架段后追加）挂 `message_loop_neutral_family_automation_route` Section；文本=chat 族 `chat.discipline.plugins_mixing` 段内指引行逐字同源同文 694B 零改写（同源由锁定测试执法：chat 段变更即红）；CacheKey 自治 `neutral-family-automation-route:v1`；插入点勘察定案=agentloop 装配组装层（ruleset embed/carriers 均越卡面文件域，agentloop 为卡面授权候选位）；**chat/server.go 零改动**=确认话术分支（message_loop_system）零回退自然形态+零回退断言测试在包。
- **面 2（重验）执行+停止条件 2 定案**：220 断言体 v2 原口径（scripts/ 零改动）；ps1 exit 0 COMPLETED（**220 的 GUIDANCE_MISSING exit 2 消失**）；render 7/7 亲证指引行在场（system 14,772→15,468B）；组 1 三连同型 no_route 止损合规（组 2/3 未消耗）；计费 10 调用/106,073in/6,619out（+23,936 cache read）。
- **停止条件 2 上交（卡面原文命中）**：指引在场仍三连 no_route——非 prompt 问题，是**通道结构面**（三道闸）：①free-state 决策轮 allowed tools=ccb.* only+骨架规则禁 mutation，指引与骨架同场冲突，模型合法动作集无 automation；②turn 预算（观察探索耗尽 MaxTurns 8/slice→waiting_continue，220 §四.3 预警实测复现）；③settle 面无 LLM 轮（goal 终态=no_pending_mix_tick_candidate 确定性文本，含指引+catalog 的 message_loop_system 轮从未被变体话术触达）。设计讨论四案（A 词表扩面/B semantic_entry 分类轮增 automation 分流/C free-state 终态 automation 边界形态/D settle 面补 LLM 轮）见 RESULTS §三——**225（澄清上移 LLM）的依赖面=本锚点族**。
- **次生评估面定案（越域另拆卡）**：「模型视角轨无插件」断点=内核 VSP 紧凑面 `VitApp/Source/Service/VspKernelReference.cpp:603` `normaliseTrackArray` 白名单不含 `plugins`/`plugin_count`（轨级 Volume&Pan 非 rack 节点，紧凑态仅 volume_db/pan+plugin_param_digest 哈希；内核完整面 TrackService.cpp:120 createTrackState 有数据）；agent 侧消费链已备（mixboard project_package.go:163 已读 plugins）+放行点锚点（free_state_observation.go:997 六字段白名单）。按卡面「需动内核→另拆卡，不阻塞」执行。
- 端测边界声明：覆盖=真实栈 LLM 轮次（10 调用真实计费）+render 逐 turn 取证+agent log 工具流+内核 ZMQ 面基线；**不覆盖**=通道可达前提下的模型路由行为（三道闸前置未达）/set_points 确认链/choreo pending 腿/组 2-3/webui 渲染面。
- 纪律核对：真栈占用登记 c80e33e7/释放同批入库（21:43 登记→21:46 拆栈全零复核）；主检出工程文件零触碰（mtime 亲证）；运行时工程+派生态剔除（218 先例）；§12 领取基线在卡面；无 stop 未披露项。

## 验收（主管裁定 2026-10-30 深夜，GLM 主管决策流）

- **整卡 pass（修复面+重验执行形态）**：面 1 兑现亲核（同源锁定测试三面=变更即红强制同步机制佳；server.go 零改动=确认分支零回退自然成立）；面 2 执行合规（指引在场亲证后组 1 仍三连 no_route——卡面停止条件 2 精确命中，执行侧止损上交而非盲目迭代 prompt=纪律正确）。
- **三道闸定案接受为架构事实**：闸 1（free-state 决策轮 allowed tools=ccb.* only+骨架禁 mutation）为**变体话术到不了 automation 的结构根因**——指引与骨架同场冲突亲证。此为判断权归属裁定（2026-10-30）的**最深一层固化**：LLM 轮工具面被封死。四案裁定转卡 227（用户参与：涉及 free-state 安全面形态）。
- **225 依赖修正**：原依赖「222 完成=LLM 地基」——现定案地基未通（三道闸），225（澄清上移）暂挂至 227 裁定落地。
- 次生面（轨级插件披露）拆卡 228（内核紧凑面小卡，锚点已定案）。
- 合入：main=本次 merge；计费 10 调用/106,073in/6,619out 披露接受。

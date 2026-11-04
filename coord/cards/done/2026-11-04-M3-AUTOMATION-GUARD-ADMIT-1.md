# M3-AUTOMATION-GUARD-ADMIT-1：automation surface 已准入 mutation 工具放行至治理门（guard 准入修复）

- 发卡：GLM 主管决策流 / 2026-11-04（依据=246 验收裁定 [2026-11-04-M3-TERMINAL-ROUTE-WORDING-1-pass.md](../../rulings/2026-11-04-M3-TERMINAL-ROUTE-WORDING-1-pass.md) 新断点定性+三轮 guard 拒绝工件）
- 派发确认：已确认（链内续卡，主管发卡；用户口头开工即有效）
- 验收负责人：GLM 主管决策流
- 池序 247；目标仓库=主仓 agent；P1 / 0.5 天 / 依赖=246 已在 main（139fa254）✓；L1 / PC 执行会话；栈空闲（246 已释放）

## 标的（246 验收亲证，不重考）

模型 v3 措辞后已直调 set_points（7 次 agent log 亲证），但 chat executor 包装层 `freeStateInvokeMutationReason`（goalrunner_chat.go:855-863 每工具先过）+`free_state_invoke_guard.go:26-29`（active free-state 全拒 MutatesProject）先于治理门截杀（r1 10 次「mutation tool automation.set_points is forbidden」）→run failed→no_route×3。guard 是「free-state 只读观察」旧契约执法者，与 M3 automation surface 准入（决策轮直调→治理门拦卡，`TestFreeStateSetPointsZeroBareWriteGovernanceChain` 先例）正面冲突。

## 目标

1. **guard 准入**：`freeStateInvokeMutationReason` 对 automation surface 已准入的两个 mutation 工具（`automation.set_points`、`automation.clear_region`）放行——放行≠执行，它们继续走下游既有治理面（`typedPluginApplyConfirmationResponse`/agentloop 治理链）形成确认卡。**daw.invoke 与其余一切 MutatesProject/RefreshAfter 工具维持全拒不动。**
2. **治理红线**：确认卡仍是唯一授权点——不得出现 bare write；不触准入判定/治理门/执行器逻辑（越域即停）。
3. **单测**：①active free-state 下 set_points/clear_region invoke 通过 guard 且形成 needs_confirmation（非 error 非 bare write）②其余 MutatesProject 仍拒（原语义回归）③daw.invoke 仍拒④放行后确认前 project 无 mutation（零裸写断言）。
4. **真栈重跑**：-Groups 1 单组一次（deepseek-flash，载荷+agent log 全开）——**链口径判据：组 1 绿（no_route 消失+tool_chain_ok set_points 包络形态）或新断点取证包**；终局三选句（246 项一）真栈检验搭车（TERMINAL TURN 轮到达时新句在场被读）。

## 预期与诚实边界

模型面已直调（246 亲证），本卡=确定性接线；首绿窗口真实打开但 §8 口径不变：预算 ≈¥1.5-3；成功=组 1 绿或新断点取证（上交不自修）；若放行后 set_points 未形成确认卡而直接执行→**立即停上交**（治理门覆盖面缺口=设计问题升级主管，不得自行补门）。

## 停止条件

- 放行导致既有 guard/治理测试红（零回退）→回退两案上交；
- 出现 bare write 任何形态→立即回退上交；
- 组 1 仍不绿→新断点取证包上交，不原样重跑；
- 越域（free_state_invoke_guard.go+goalrunner_chat.go+chat 测试之外）即停上交。

## 文件域

`agent/internal/chat/free_state_invoke_guard.go`+`agent/internal/chat/goalrunner_chat.go`（如需接线）+chat 包测试。

## 领取

- 2026-11-04 晚 / origin/main=064b7dd7（=247 发卡提交，246 验收合入后 HEAD）/ owner=ZCode GLM-5.3 PC 执行会话（247 卡会话，用户口头开工）/ 实现 worktree `D:\Vit_DAW_wt_247impl` @ 分支 `port/m3-automation-guard-admit-1`（基线 064b7dd7）；协调 worktree `D:\Vit_DAW_wt_247claim` @ `coord/247-claim`（仅本卡状态提交）。
- 领取基线：主检出 HEAD=064b7dd7 与 origin/main 一致；既有工作树改动均为运行时工程状态（VitApp/Workspace xml 两件）与历史 run 工件/旧卡状态（UX-AB-AUDITION 等），与本卡文件域（agent/internal/chat）不相交；`git diff --stat` 摘要 8 files +4469/-4429（全为领取前已有，非本卡 diff）。
- 文件域核对：doing 池仅 GEN-SUNO-WEB-DESIGN-1（设计类，域不相交）；本卡域=free_state_invoke_guard.go+goalrunner_chat.go（如需）+chat 包测试，与其零相交。
- 栈：空闲（246 已释放，2026-11-04 22:04:51 TEARDOWN_ALL_ZERO）；本卡真栈腿登记见回执。

## 回执

- **执行侧自验完成，转 done 待主管验收**。实现 commit=0089b558（分支 `port/m3-automation-guard-admit-1`，基线 064b7dd7；diff 恰好两文件=卡面文件域零越域：free_state_invoke_guard.go +23（两 lookup 分支各加 `freeStateAutomationAdmittedCommand(spec.CommandName)` 白名单放行，锚 spec.CommandName=别名经 catalog 解析自动覆盖；daw.invoke 检查位次不变仍最先全拒）+free_state_invoke_guard_automation_admit_test.go 新增 133 行）；工件 commit=366779d0（coord/runs/M3-AUTOMATION-GUARD-ADMIT-1/guardrun1/ + RECEIPT.md 判读；payload_capture 4.6M+prompt_render 2.6M 留 worktree，clip.wav 已删）。**goalrunner_chat.go 零改动**（guard 函数被 server HTTP 边界与 executor 包装层两入口共用，单函数改动双面生效）。
- **guard 准入落地 ✓**：放行≠执行——两工具续走下游既有治理面（`validateAutomationWrite` fail-closed 域验证→harness.go `spec.RequiresConfirmation` 确认卡），确认卡仍是唯一授权点；daw.invoke 与其余一切 MutatesProject/RefreshAfter/known-mutation 原语义不动；不触治理门/准入/执行器逻辑（代码亲证链：guard→typedPluginApplyConfirmationResponse 不认识 automation 不拦→base executor→harness.Invoke）。
- **单测四件 ✓**（chat 包）：①executor 包装层（246 断点位）active free-state 下 set_points/clear_region 通过 guard 且经真实 harness 路径形成 needs_confirmation（非 error、mutation_performed 非真、无 written/removed_points/point_count 执行回执字段）+非 free-state 路径 Confirmed=false 同停确认卡（准入未弱化确认门）②其余 MutatesProject 仍拒（plugin.set_parameter/rack.add_node/plugin_grabber 家族）+admitted 名伴生注入（automation.set_points+args.cmd=plugin.set_parameter）仍拒③daw.invoke 三形态（直调/snake/admitted 伴生）仍拒④零裸写并入①（mutation_performed 非真+无执行回执字段）。既有 guard 测试（HTTP 边界/executor 包装层/loop 结束不残留）全绿。
- **全仓回归 ✓**：`go build ./...`+`go test ./...` -count=1 exit 0 零红（chat 118s+agentloop 5.4s+harness 17s 在内全包）。
- **真栈腿（guardrun1，-Groups 1 单组单 run，deepseek-flash 70/70 注入亲证）：组 1 绿（卡面链口径判据达成）——链上首次 confirmed write reached kernel**：
  1. **r1/r2 两轮一致 tool_chain_ok**：r1「confirmed write reached kernel (points=4)」tools_called=ccb.observation_request→catalog→plugin.get_parameters→**automation.set_points**，pre_confirm_zero_write=True，kernel 0→4 点（0/8/16/24s 0.74→0.95）；r2「points=5」同链+**automation.get_curve 读回**（settle_mentions_get_curve=True）；
  2. **no_route 消失**：246 同栈同引擎三轮 no_route×3（guard 拒 10 次）→本轮三轮零 no_route；模型直调全部放行至治理门形成确认卡，场景腿确认后写达内核；
  3. **r3 环境中断（非模型/功能失败，§8 原始日志排除）**：r3 模型面直调亲证（telemetry 22:38 三调用+message_loop_debug 22:38:20/29 final_gate raw 含 set_points tool_call）；随后 agent 陷入 **continuation.park 空转风暴**（22:40:59 起 729 条/2 分钟，期间 VSP→8787 refused 累积 59）→python 轮询 10061 fail-fast→无 g1_r3.json/summary→ps1 exit 1（throw=无 summary 按设计，非功能断言失败）；agent 全程存活（22:29:47 启动至手拆无重启）。两轮一致已足，不重跑；**park 风暴+listener 拒绝=新断点观察（非本卡域，供主管裁决是否开卡）**；
  4. **终局三选句（246 项一搭车）真栈检验达成**：needle_scan——TERMINAL TURN 轮 4 行且三选句 4/4 在场被读；其余四 needle 65/70（缺席=setup 短请求 244/246 同型）；旧二选句 0；r1 终局轮 unparseable+fallback（模型照直调示例带 tool_call 被 final gate 拒，246 ruling 形态偏差同型观察）、r2 settle 停 needs_confirmation（治理门正常拦卡）；
- **红线核对 ✓**：零 bare write（r1/r2 pre_confirm_zero_write=True 确认前 0 点）；set_points 均成确认卡后经确认执行（「未成卡直接执行→停上交」未触发）；既有测试零红。
- **计费（§8）**：70 次计费调用（零错误）in 1,082,281/out 77,918，粗上界 ≈¥1.71；卡面 ¥1.5-3 预算内。
- **栈纪律（§2.2）**：占用 22:27（fd84e9e8；登记前 22:26:32 亲核全零+push 后远端 owner 重读；首启一次 shell 路径参数失败 exit 1 零进程启动，22:29:39 复核全零后重启）→释放 TEARDOWN_ALL_ZERO 22:47:17 亲核（PROCS=0+TCP 六口=0+UDP=0，owner 手拆 VitAgent 39360/VitApp 45300）。
- 端测覆盖边界声明：本卡改动面=guard+chat 测试；无 webui/渲染面改动（不适用）；真栈腿覆盖 agent HTTP/LLM 载荷/telemetry/agent log/message_loop debug/kernel 写达面（r3 因环境中断未达 summary 汇总面，rounds 逐轮 JSON 在）。

## 验收

- **pass（实现合入 7b06c019）+链首绿成立**；裁定=[2026-11-04-M3-AUTOMATION-GUARD-ADMIT-1-pass.md](../../rulings/2026-11-04-M3-AUTOMATION-GUARD-ADMIT-1-pass.md)。
- 亲核：guard diff 亲读/r1 治理链载荷亲证（确认卡→确认后 0→4 点写达内核+零裸写）/r2 读回/needle 三选句 4/4/r3 风暴证据（729 park+10061+直调亲证）/计费 ¥1.71。
- 口径裁定：有效轮 2/2 判据达成=链首绿；r3 主管再分类为 agent 子系统缺陷（非纯环境）开卡 248；ps1 exit 1≠全组 exit-0，链收口前补 3/3 稳定 run；r1 终局 unparseable=246 同型 T2 观察挂账。

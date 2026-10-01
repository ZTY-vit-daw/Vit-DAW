# M1 复验第三轮（2026-10-01 18:30–18:36）活栈取证笔记

- 取证人：决策会话（只读活栈 GET + 日志直读 + 会话图/持久层文件）；栈=Godot(18:30:32)→VitApp(23888)→VitAgent(9728, **bin/VitAgent.exe 18:14 重建=含 FS-PARK/WEBUI-MSG-ORDER 修复的 main d42629d8 态**——四个症状全部出自修复后代码）
- 会话：webui_mupe9yh1；goal_36b2f2e8（第一句「检查一下当前工程有什么问题吗」18:32:02）→ park → user_stop(18:34:07)；goal_6b07c186（第二句「检查一下当前选中的drums轨道的低频」18:35:32）→ 立即失败
- 证据：evidence/ 五件（事件流 36 条/agent-state/ui-state/会话图/LLM 遥测窗口）

## 症状与根因（详析见会话记录，锚点复核过）

1. **100s「首观察」**＝LLM 服务面：18:33:19 一次调用 http_ms=60000 整超时（rightapi.ai DeepSeek 端点）→重试 31.7s→解析修复 5.8s≈98s；观察 item 本身 3s。与 09-30「50s=重试+门循环」同族但更糟（整超时命中）。
2. **取消滞后+停止挂起期间干预仍被应用（真缺陷）**：用户在 CCB 观察运行中（18:32:18–18:33:58 空窗内）按取消；停止=异步标记+轮边界生效（turn_control.go:326-336）；在途轮继续走完「决策→18:34:04 EQ 应用→18:34:06 试听准备→park」后 18:34:07 停止才落地。
3. **第二句立即失败（真缺陷，会话楔死）**：`turn.failed, stop_reason=audio_closure_controller_failure, error="conversation is already owned by minimal_audio_closure controller audio_closure_997396f72e46902c"`。机制链：finalizeStoppedTurn（turn_control.go:246-271）把 Experiment 置 Stopped（→freeStateJudgmentBoundary false，free_state_reasoning_loop.go:1276-1278，FS-PARK Part A 采纳被排除——设计如此）**但不碰 audioclosure**；closure 停 fs7 非终态→settleAudioClosureOwner 只认 Terminal（audio_closure_controller.go:1390-1392）→所有权不释放→新 goal 新 closure 被 ensureAudioClosureOwner（:236-240）拒绝。turn_close_guard 的孤儿结算路径（turn_close_guard.go:42-68→206-248）被「任务 human_judgment_required+判定交互在飞」挡住不适用。**会话 webui_mupe9yh1 永久楔死**（新会话不受影响——所有权按会话隔离）。
4. **消息序倒挂（活态渲染残留）**：用户目视=第二句输入+报错都在第一轮输出**上面**；服务端会话图节点序正确（ask→回执→ask→错误，时间线性，evidence/conversation-graph.json）。第一轮链内容（轨迹块/A-B 判定卡/park 消息族）钉流底、新消息插其上——WEBUI-MSG-ORDER-1 修的是 isChainResultChatMessage 钉尾+turnGroups 回吸，**A/B 判定卡/park 交互消息族走另一条投递通道未被覆盖**（该卡回执已声明活态路径未真栈断言）。

## 顺带正面记录

- 18:34:22 audition 重落座 recovered_from=stopped——RESEAT 修复真机在场。
- mix_tick 完全访问模式 display 文案正确（「这一步将直接应用，不再等待逐条确认」）。
- 第一句 7s 出回执（「我还在继续处理这个任务」execution_receipt）——快速回执链正常。

## 取证盲区（记 known-issues 候选）

- handleTurnStop 无请求日志（取消按下时刻只能靠用户证词——本次已获：CCB 观察运行中）。
- agent_last.log 被 authority restore INFO 刷屏（每秒 3 行）——AUTH-RESTORE-LOGSPAM-1 已在池，取证成本实证+1。

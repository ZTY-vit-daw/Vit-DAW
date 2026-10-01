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

## 追记：第四轮复验（2026-10-01 20:33–20:36，会话 webui_mupimj6f，栈=新二进制 20:31+新 dist 20:24）

- **修复生效面**：消息序正确（用户确认）；park→新输入→默认采纳正确触发（20:35:33 round.decision+settled=adopted 链）；新 goal/run 正常开启（FS-PARK 主语义真机闭环）。
- **新缺陷（采纳路径姊妹案）**：新 goal goal_65e6beee（"可以听听看drums音轨的clip低频怎么样吗"）零执行即收尾，回复=「任务已经满足其持久化契约中的证据与结算条件。」——audioClosureSettlementReply 的 StopTaskSettled 映射句（audio_closure_controller.go:1663-1664）。机制：FS-PARK Part A 采纳结算 experiment+task（EventTaskSettled）但**不触碰会话 audio closure**（judgment_park_continuation.go 无 audioClosures 调用）；新 goal 经 prepareAudioClosureContext（audio_closure_controller.go:62-66）绑定会话既有活跃 closure→其 TaskState=Settled→audioClosureSettleFromResult（:1327-1333 StateSettled→StopTaskSettled）立即结算收尾。与 FS-STOP-APPLY-1 Part B 同类（会话级持久态未随收尾释放），路径不同（采纳 vs 停止）。
- **A/B 判定按钮无效再证**：park 期间（20:35:01–20:35:33）用户点击 A好/B好/补充——事件流零 judgment 类事件（点击未产生任何服务端痕迹）；证据=events-webui_mupimj6f.json。AB-JUDGMENT-CARD-1 假设 a/b 的现场再强化。
- **VITNOTE 面板不可交互（M8 新阻断）**：dock 面 N 键开窗成功但面板零交互（含收起按钮）；键盘链（N）通、鼠标链全灭——疑输入路由/叠层被 dock 上层消费或命中测试未达面板（vit_note_panel.gd 按钮接线在位：collapse:85/send:104/anchor:164；manager 层 MOUSE_FILTER_IGNORE:27 正确）。待取证卡钉死。

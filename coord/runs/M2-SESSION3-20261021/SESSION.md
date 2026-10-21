# M2 体验场次③实录（2026-10-21 深夜，用户手测，重启后新栈）

- 场次：用户重启链路后（Godot→前端→启动页→六轨工程 spv1_p06）同入口手测；栈 22:46:51 起（内核 21244+agent 26148+hub 7244）；conversation=webui_mvi87m8p / goal=goal_ebe0644d39aaf5bf；authority=full_project_access。
- 被测二进制：agent=主检出 083DE7BB（含 158 种子观察+160 文案，22:27 构建）+内核 722A79D7+前端 c5cac57（含 161 send 自动呈现）——**三层全部为新版本**。
- 本件=主管现场取证（栈保持用户在场未拆）。

## 用户话术与结果

「给人声加点空间感」（场次②轮 1 同话术）。用户观察：①send 槽在 FX 轨出现；②人声轨无 send；③webui 对话流显示「中断」而非完成；④担心「模型不够聪明」。

## 取证结论

1. **158 生效**：22:47:40 `[free-state.seed] booked observation=obs_…0f3047679c95 revision=2 scan_satisfied=true`——冷启动种子凭据入账，G3 首拍满足，**无边界停**（场次②同话术断点闭合）。
2. **链路首次完整走通（机械面）**：观察（catalog+request）→plugin.semantic_search（4s）→track.send.list→**track.send.connect（22:48:50）**→rack_add_node（Pro-R 1047→1037）→vit_checkpoint（revision=4）→22:48:56 `agent_loop_chat status=completed stop=semantic_dynamic_execution_completed executed=3`+`f6.gate chainEnded=true respGoal=completed runtimeGoal=completed`+journal 入账（source=free_state_d1_s1，d1_semantic_dyn 动作）。
3. **❌ 目标轨错（→卡 162）**：connect 结果 source=**piano/1027**→Reverb Bus/1037 @-6.0——用户指名「人声」(1032)被丢弃。机制：157 确定性解析只挂自由态循环面；模糊意图经语义入口路由到 147 全接入动态链后，计划目标轨=模型自选。机械面全对、意图保真断在交接面。
4. **✅ 161 生效（现场实证）**：用户看到 FX 轨 send 槽自动出现（旧版不会）；「人声轨没有 send」的正解=send 在**钢琴轨**（查错轨非缺陷）。
5. **❌ PlanBar 误标（→卡 163）**：webui `composer/PlanBar.tsx:40,68` 链活信号熄灭即灰态「已中断」——活跃度启发式不读 goal 终态，completed 与真中断同形。
6. **模型智力判读**：本场唯一模型决策失误=选轨（钢琴 vs 人声）；其余全部断点均为系统层（种子凭据/GUI 刷新/状态显示/意图接线），且逐项有确定性修法。架构哲学验证成立：承重墙=确定性规则，模型只承担提案/解释——选轨这个面正在被 157 同款确定性解析接管（卡 162）。

## 工件

- `agent_last.log`（全量，seed/observation/动态链/f6.gate/journal 链）
- `kernel_send_connect_excerpt.txt`（connect 结果铁证段 source=piano/1027）
- 原始日志：VitHeadlessServer2026-10-21_22-46-51.log（现场保留）

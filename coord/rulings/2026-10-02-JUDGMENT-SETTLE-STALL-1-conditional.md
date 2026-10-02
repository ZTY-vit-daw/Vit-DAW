# Ruling：JUDGMENT-SETTLE-STALL-1 — conditional pass（进程内全过；真栈烟测+用户手测复验待泊位释放后补）（2026-10-02 决策会话）

## 验收四层

1. **定性采信**（嫌疑③成立）：round 落了 user_judgment_pending 轮决定（边界驻留成立）但 **human_audition_ready 靶响应未落→判定通道三层全关**（arming 拒绝/experiment 层 arm 拒绝/record 拒绝）→webui 判定席 canJudge 永假**从未渲染**→用户点击是回放 select（"已完成/用户已选择"=select 事件既有渲染前缀，非判定落账）——**同时解释用户反馈 #3「无其他反馈」与 #4 新输入报错**；嫌疑①（409）从未发生、嫌疑②（mix-tick 分流）结构性不可达，排除论证在案。与 D1-SETTLE-TAIL-MAC-1 同族不同形（本侧轮决定已落/Mac 侧停 observing 调度切片）——两卡各自继续，不并轨。
2. **修复 diff 亲核**（1a83e462，6 文件 +804/-11）：experiment 层 `RequestUserJudgmentAtParkedBoundary` 守卫武装（armed 与 parked-boundary 两形态同则）+chat 层幂等重试武装（存量恢复：用户活栈 webui_muqwy5sv 楔死形态可被新会话自愈）+判定结算后闭包收口/owner 释放+**judgment.settled 可见新回复**（确定性闭环模板+审计身份=通用确认卡语义，用户裁定兑现）+所有权防楔（终态陈旧 owner 退役/两活闭包仍 fail-closed/Save 失败仍释放）。
3. **我方独立复跑**：定向新测试 PASS（0.04s）+chat 全包 82.2s PASS+experiment 包 PASS+全仓 `-count=1` 零 FAIL（worktree 亲跑）。
4. **RED 复现证据**：judgment_settle_stall_test.go（428 行）以 `jsssParkedBoundaryServer` 构造活栈定格形态，pre-fix 三失败=活栈三症状（armed=false/409 identity mismatch/"already owned by minimal_audio_closure"）逐条编码在案。

## 裁定

- **conditional pass**：进程内证据链完整；**真栈烟测（AGENTS §5 门槛）+用户手测复验（判定→可见新回复→新输入全新 goal）**为终验条件——泊位已释放（用户现场可关）后补做，补做通过即转 pass。
- cherry-pick 1a83e462 合 main；活栈遗留楔死态由幂等重试武装自愈（新会话验证）。
- **LLM 亲写结算文案**（判定入 LLM 回合 vs 现确定性模板）：登记为增强候选转用户裁定——决策侧意见：模板已满足「可见反馈+审计身份」，LLM 亲写建议挂 D1-SETTLE-TAIL 定性后另立小卡，不阻塞本链。
- AB-JUDGMENT/FS-ADOPT/本卡三者的「新输入全链」终验合并到同一场用户手测。

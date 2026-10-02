# D1-SETTLE-TAIL-MAC-1：d1 settle 尾段停滞——materiality/target_response 迟落不落致 round 卡死（Mac 首跑暴露，P1）

- 池序 29（P1——阻塞 mac d1 冒测 exit-0；D1-EQ-READBACK-550A-1 终验挂本卡）；来源=D1-EQ-READBACK-550A-1 真栈验收上交（[rulings/2026-10-02-D1-EQ-READBACK-550A-1-conditional.md](../../rulings/2026-10-02-D1-EQ-READBACK-550A-1-conditional.md)；证据=[runs/D1-EQ-READBACK-550A-1/d1_550a_20261002_182113/](../../runs/D1-EQ-READBACK-550A-1/d1_550a_20261002_182113/)）
- 优先级 / 预估 / 依赖：P1 / 取证 0.5 天+修复 0.5 天 / 建议 AUTH-RESTORE-LOGSPAM-1 先行或同会话附带（取证能见度前置：settle 切片证据被遥测刷屏淹没）
- 模型分级：L2 / GLM 首选（时序敏感缺陷取证+chat/settle 机械域）
- 已核实事实（上交证据在案，勿重跑已证部分）：
  1. 轮1（round_1_d1_smoke_report.json，已补正归档）：error="acoustic materiality record is missing"——settle 尾段调度切片 7 分钟未落 materiality/target_response，round 停 observing/post_action_evaluation，park 已到；LLM 干预已 applied（主面证据完整，非干预失败）。
  2. 轮2：error="D1 loop projection is missing"（早期变体，d1 loop projection 未持久化——与轮1 是否同根待取证）。
  3. PC 同 evaluator 全过；Mac 首跑暴露——mac 时序敏感嫌疑（未证，取证时勿预设立场）。
  4. 上交锚点（领取时实锚复核，漂移即修正）：agent 内 free_state_d1_runtime.go:703-710 尾段链/调度切片。
  5. 复跑工具在案：scripts/run_d1_eq_readback_550a_smoke_mac.sh（止损分类已修为按 error 字段，勿回退）。
- 目标：
  1. **取证**：settle 尾段为何 7 分钟不落 materiality/target_response——三假设分诊：调度切片饥饿（goroutine/队列）/等待条件永真（事件丢失或谓词永假）/超时与重试形态错误；轮2 "D1 loop projection is missing" 是否同根。
  2. **修复**：按根因修（域内）；不得以放宽等待/静默吞错收口。
  3. **终验联动**：修复后 d1 冒测（含 550A 锁定场景复跑，沿用 D1-EQ-READBACK-550A-1 脚本）exit-0——同时完成 D1-EQ-READBACK-550A-1 终验条件。
- 文件域：agent chat/settle 机械域（free_state_d1_runtime 尾段链一带，实锚后申报）；若根因在内核调度面→实证上交转内核卡。
- 验收标准：根因取证报告+修复 diff+相关包全绿+全量 0 FAIL+d1 冒测 exit-0（run 工件可回指）+泊位声明。
- 停止条件：①取证发现缺陷在内核调度面（agent 侧无法闭环）→ 实证上交；②时序缺陷无法在 mac 稳定复现（需构造确定性复现路径，无法构造则如实上交定方案）。
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 取证根因 / 冒测 run ID / 泊位声明）
- 验收：（裁定文件 / 验收 commit）

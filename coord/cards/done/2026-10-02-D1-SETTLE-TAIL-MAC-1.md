# D1-SETTLE-TAIL-MAC-1：d1 settle 尾段停滞——materiality/target_response 迟落不落致 round 卡死（Mac 首跑暴露，P1）

- 池序 32（P1——阻塞 mac d1 冒测 exit-0；D1-EQ-READBACK-550A-1 终验挂本卡）；来源=D1-EQ-READBACK-550A-1 真栈验收上交（[rulings/2026-10-02-D1-EQ-READBACK-550A-1-conditional.md](../../rulings/2026-10-02-D1-EQ-READBACK-550A-1-conditional.md)；证据=[runs/D1-EQ-READBACK-550A-1/d1_550a_20261002_182113/](../../runs/D1-EQ-READBACK-550A-1/d1_550a_20261002_182113/)）。池序归一（2026-10-02 决策侧）：原入池误标 29 与 VITNOTE-V2-IMPL-B 撞号（B/C/D=29/30/31 在案），顺延 32（PC 注记 4c1ecff）
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
- 领取：2026-10-02 / origin/main=8c3ff03 / port/d1-settle-tail-mac-1（Mac 执行侧，独立 worktree ~/Documents/vit-d1settle-mac1）
- 回执：
  - 实现 commit：port/d1-settle-tail-mac-1@1662943（agent/internal/chat/goalrunner_chat.go 两处：applied 边界武装扩到 parked-resumable goal；arm 核心槽位移——answerable park 存活、legacy shell 取消、runnable/裸占用仍拒）。+3 回归钉（traj_auto_settle_test.go）。
  - 取证根因：**假设②（等待条件永真形态）**——2026-10-02 d1_550a r1：generic mix-tick 可答 park 先落（10:26:09.7）钳住 goal（waiting_confirmation），干预 apply（10:26:12）落在其上后，settle 检查点的两条武装边界（applied 边界 goal==completed 门、recordGoalResult completed-over-owed 门）均不可达（parked goal 永不发 result），调度器按设计握着 answerable park 十分钟，materiality/target_response 全窗未落。假设①（调度饥饿）③（超时/重试形态）证据排除（调度全程在拍、地板已授、无重试在等）。**轮2 不同根**：G6_target_evidence+G8_target_consistency 准入门拒案（fail-closed 设计内，模型提案质量方差）。"Mac 时序敏感"定性为模型提案流方差（park 先于 apply 的窗口期交错），非机器时序。取证报告：coord/runs/D1-SETTLE-TAIL-MAC-1/FORENSICS.md。
  - 冒测：**run d1_550a_20261003_180509 exit-0**（round2 PASS：static_eq@1017 rev6→7，readback -2dB verified=True，settle 记录落地，judgment_park=true；round1 为无关模型方差断点）——同时完成 D1-EQ-READBACK-550A-1 终验条件。证据+md5 逐文件核对：coord/runs/D1-SETTLE-TAIL-MAC-1/d1_550a_20261003_180509/；环境类失败尝试留痕与 A/B 取证见同目录 SMOKE_RUN.md。
  - 泊位声明：修复后 settle 尾段自动跑完并泊在人工判定边界（judgment_park=true，closure fs7_improvement_proposal，无机器越权收口）；被位移的可答 park 存活可答。测试：chat 包全绿；全量 87 包 0 FAIL（go test ./... exit 0）。
  - 端测边界：本卡端测=真栈 550A 冒测（agent HTTP 面驱动全程，Godot 面外，同 journey1 mac 先例）；渲染面/用户旅程面未涉（改动为 agent 调度机械域，无 webui 变更）。
- 验收：**pass**（2026-10-03 决策会话）——[rulings/2026-10-03-D1-SETTLE-TAIL-MAC-1-pass.md](../../rulings/2026-10-03-D1-SETTLE-TAIL-MAC-1-pass.md)：取证三假设分诊+修复 diff（武装门扩展/槽位移三分支/闩守卫沿用）+与 PC 同域修复（334d8f4/d8b7e2d）零文件交集正交性核对+我方复跑 87 包 0 FAIL+终验 run 亲读（round2 独立复核 applied/readback/judgment_park）；实现 1662943 cherry-pick 合 main；**D1-EQ-READBACK-550A-1 终验同场闭合（conditional 转正 pass）**。

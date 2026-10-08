# Ruling：AUTH-RESTORE-LOGSPAM-1 —— pass（2026-10-08 晚窗，决策侧）

- 执行侧：flash（用户转交）；实现 `e494a54d` @ `port/auth-restore-logspam-1`（worktree D:/Vit_DAW_wt_authrestore，base 9e30ac5f）
- 落位：cherry-pick → main `2436fb99`；worktree/分支随裁定清理

## 验收亲核

1. **来源取证偏差申报采信**：卡面假设前端 GET 轮询触发——执行侧实锚推翻（GET 有同身份+同 session 早退护栏），主源=continuation scheduler 250ms tick 无条件 reload（500 行/120s≈4.17 行/s 节奏吻合）；修复点与卡面文件域一致（restore 日志点本身，对任何触发源生效）——按停止条件纪律如实申报，不推翻卡面目标。
2. **闩语义零变化实证**：diff 内 authorityModeExplicit 零触碰（我方 grep 实证）；s.authorityMode 赋值分支原样；日志级别与闩行为解耦。
3. **取舍采信**："跳过"而非"降 debug"——logx 全级别落盘，降级不满足"稳定态 0 行"验收口径；INFO 文本逐字不变保 grep 连续性。
4. **测试与门**：2 新测（adopted 重复静默+真变再打 / explicit 复刻证据形态收敛+分歧可见）我方复跑绿；全量 90 包 0 FAIL（含 28 处既有 restore 消费调用面）；gofmt blob 净。
5. **端测边界**：手验轮询窗口 0 行留真实栈活栈观察（日志卫生单点，按卡面 Go 全量门验收）——归 gate 或后续活栈观察，不阻塞本裁定。

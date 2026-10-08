# AUTH-RESTORE-LOGSPAM-1：authority restore 日志轮询刷屏（每秒 3 条 INFO）

- 池序 12（P3 日志卫生）；来源=M1 复验活栈取证（证据=evidence/authority-logspam-count.txt：agent_last.log 全 500 行均为 `[authority] restore kept explicit mode=full_project_access disk=full_project_access`，2 分钟窗口）
- 优先级 / 预估 / 依赖：P3 / 0.25 天 / 无；agent chat authority 域
- 模型分级：L0 写卡即走 / flash 可接
- 已核实事实：前端轮询（webui/Godot authority 面）每次 GET 都触发 restore 路径并打 INFO——restore 语义上是"盘上态恢复"，稳定态下无新信息却每秒 3 条。
- 目标：restore 日志降噪——仅模式实际变化或首个恢复时打 INFO，重复 restore（模式未变）降为 debug 或跳过；不改变 restore 行为本身（闩语义零变化）。
- 文件域：agent 侧 authority restore 日志点（chat/server.go authority 闩一带，实锚后申报，≤1 文件+测试如需）。
- 验收标准：go 相关包全绿+全量 0 FAIL+手验轮询窗口日志行数从每秒 3 条降为 0（稳定态）。
- 停止条件：取证发现刷屏另有来源（非轮询触发）→ 如实申报。
- 领取：2026-10-08 / origin/main `9e30ac5f3da8a90f51b420b66c7a66f0b8cf2a9c` / `port/auth-restore-logspam-1`（独立 worktree `D:/Vit_DAW_wt_authrestore`，领取时干净）
- 回执：`coord/runs/AUTH-RESTORE-LOGSPAM-1/receipt.md` 随分支提交（commit hash 见分支提交记录）；锚点=chat/server.go `restoreProjectAgentRuntimeStateLocked` authority 块（原 ：7297-7307）+ Server 结构体 dedupe 记账两字段；降噪对比=稳定态 500 行/2 分钟 → 0 行（重复 restore 整行跳过，仅首个/变化各 1 行 INFO；来源取证=scheduler 250ms tick 重载而非前端 GET，已申报）
- 验收：**pass（2026-10-08 晚窗决策侧，rulings/2026-10-08-AUTH-RESTORE-LOGSPAM-1-pass.md）**——来源取证偏差申报采信（主源=scheduler tick 非 GET，修复点同域）+闩零触碰我方 grep 实证+跳过/降级取舍采信+2 测试复跑绿+全量 90 包 0 FAIL；cherry-pick e494a54d→main 2436fb99；手验窗口 0 行留活栈观察归 gate

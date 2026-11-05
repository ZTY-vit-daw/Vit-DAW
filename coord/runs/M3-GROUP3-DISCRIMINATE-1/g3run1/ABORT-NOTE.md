# ABORT-NOTE — g3run1 第一次启动（环境中断，零 LLM 损耗）

- 时间：2026-11-05 10:32:50 启动 → 10:33:07 退出（17 秒）
- 原因：Git Bash 调 powershell 时 `-File scripts\dev_agent_smoke.ps1` 的反斜杠被 bash 转义吞掉（AGENTS §5 已知陷阱：Windows 路径在 bash 中的引号/反斜杠），PowerShell 报「-File 形式参数的实际参数 "scriptsdev_agent_smoke.ps1" 不存在」，退出码 127
- 损耗：**零 LLM 损耗、零进程启动**（ps1 未执行任何栈操作；占用登记后全零复核 10:32:43 亲核）
- 处置：按卡面停止条件第 2 条（环境中断→零 LLM 损耗的配置修正重启合规）——路径改正斜杠 `scripts/dev_agent_smoke.ps1`（249 回执同款写法）重启
- 取证：ps1_console_abort1_bashpath.log（本目录）

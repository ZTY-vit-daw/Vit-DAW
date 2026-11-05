# g23run1-aborted：启动配置遗漏 -KernelExe（环境中断，LLM 零损耗）

- 时间：2026-11-05 09:36:40 启动 → 09:39:xx owner 止损
- 归因：**执行侧启动配置错误**（非脚本缺陷、非功能断言失败）——启动命令遗漏卡面指明的 `-KernelExe` 参数（先例=238），脚本默认取 `$RepoRoot\Export\staging\runtime\VitApp.exe`；wt_249run 为新建 worktree，git 不含构建产物，该路径不存在 → `warn: kernel exe not found`（非 fail）→ kernel 未启动 → ZMQ 5555/5556 不通 → python 腿 setup 无法连 kernel。
- LLM 损耗：**零**——`automation_llm_route_console.txt` 空、`agent_llm_telemetry.jsonl` 不存在、无 rounds 目录（python 尚在 setup 阶段）。
- 处置：owner 手拆（taskkill python 49332 + VitAgent 39456），复核全零（PROCS=0、TCP 六口=0）；ps1 后台任务以 exit 1 退出（no summary artifact throw，止损路径符合预期）。
- 分类：§8 环境中断（执行侧配置错误类）；修正启动参数（补 `-KernelExe`）后重启 = 配置修正，不属"同一断点原样重跑"。
- 教训：新建 worktree 无 Export/staging 产物，凡 `-StartKernel` 必须显式 `-KernelExe` 或先跑 build_release 复制链。

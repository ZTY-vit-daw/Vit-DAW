# PROGRESS — FIX-RESTARTAGENT-ENV-1（池序 326，P2）

- 执行 owner：GLM-5.3-flash（freeflash）·PC·ZCode 免费车道量产执行子代理（主管会话 2026-11-14 晚窗派发）
- 分支：port/fix-restartagent-env-1（本文件所在分支）；worktree：D:/Vit_DAW_wt_env1；协调分支：coord/env1-claim（worktree D:/Vit_DAW_wt_env1coord）
- 基线：origin/main = 5bb10b0a04c63f9e804fc1a4e88fb8bbb53a02b9（fetch 亲证；派发基线 bfe0356e 的后代，bfe0356e..5bb10b0a scripts/ 域零变更）。期间并行流 353（ENV-GODOTCPP-RESTORE-1，godot-cpp 仓外域）与 UI-LOADING-1 done 落 main，均与本卡文件域不相交。

## 步骤日志（断点续跑从这里读）

- [x] S0 领取前核对（2026-11-14）：fetch exit 0；bfe0356e 是 origin/main 祖先；doing 卡域核对（353=godot-cpp 仓外、UI-LOADING=webui loading、WAVESHELL=取证已毕仅 chat 腿、ARRLOOP/CLOSURE=agent Go 域）与本卡 scripts 域不相交；wt_329impl 无 scripts 未提交改动；目标分支/路径空闲。证据：git rev-parse/merge-base 输出（会话日志）。
- [x] S1 勘察取证（只读，证据锚点见 forensics.md）：
  - 脚本侧重启链共 4 个 agent 启动点，全部裸 `Start-Process`（无 -UseNewEnvironment、无 env 白名单/清除）：
    - scripts/dev_agent_smoke.ps1:2012（主 -RestartAgent 链；stop 在 :1811-1817/-SkipBuild :1830-1837）
    - scripts/dev_agent_smoke.ps1:7604（harness_ab 逐模式重启；前置 :7596/:7598 additive env）
    - scripts/dev_agent_smoke.ps1:9629（chat auth stamp 重启）
    - scripts/restart_agent.ps1:25（独立重启脚本；:10-13 additive draft isolation env）
  - 脚本 additive 环境写入（设前 agent 启动）：VIT_HISTORY_DRAFT_ROOT(:1373-1385)、VIT_WRITE_LEASE_EVENTS_PATH(:1960-1964)、VIT_AGENT_LLM_TELEMETRY_PATH(:1969-1975)、VIT_AGENT_LLM_TIMEOUT_SEC(:1982-1985)；自恢复仅涉自身临时变量（:1862-1867、:1903-1915）。=既有约定=全量继承+additive。
  - 复用路径（真丢失面）：agent 端口已在听且未传 -RestartAgent 时复用现有 agent（:2024-2037），现役 agent env 停留在其启动时刻，新会话 env 不生效——:1976-1980 注释已声明"a reused agent will not pick it up"。
  - Go 侧：agent/internal/shelltools/shelltools.go:45 `exec.CommandContext` 不设 cmd.Env=全量继承；:12-19 allowlist 不含 powershell/脚本→agent 无法经此工具重启栈；agent 源码对 dev_agent_smoke/restart_agent 零引用（grep 全仓零命中）→重启链纯脚本侧。
  - 历史证据：321 卡 done/:34 域外记录（run1 起栈 agent 未带缝 env）+stack_receipt.json run1 INVALID；322 编排器 phase 2b psutil 验证同链透传成功（coord/runs/PLUGIN-CONTROL-EVAL-PROQ3-FREE-1/RUNBOOK.md:60）→321 观察与 322 实证矛盾，丢失边界待实证复现裁决（候选：复用路径/env 未在实际调用进程/registry 作用域陷阱）。
- [x] S2 领取提交：卡回填+todo→doing，coord/env1-claim 推 main（下一步执行）。

## 下一步

- S3 零栈 PS 实验（Start-Process 继承机制实证 E1/E2 + 复用路径丢失面模拟 E3），产出 forensics.md 取证清单。
- S4 修复 diff（按取证结论：契约注释一致化 ± 新兄弟验证脚本）。
- S5 验证腿（优先零栈+声明；若需真栈按 §2.2 全程登记）。
- S6 回执。

## 纪律备忘

- 本 PROGRESS 与全部工件随 port/fix-restartagent-env-1 分支入库推远端（§6.1），不落主检出未跟踪副本。
- 卡片状态变更（领取/完成）走 coord/env1-claim coord-only 推 main；实现经 port 分支待主管验收。

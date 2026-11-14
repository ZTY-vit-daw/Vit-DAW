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
- [x] S2 领取提交：卡回填+todo→doing，coord/env1-claim 推 main。执行记录：首次推被拒（347 终验+webui 合入落 main），按 §2.1.3 fetch 重读=卡仍在 todo 无他人领取、域零冲突→rebase 重放后推成功。origin/main=2318e22c，卡在 doing/。
- [x] S3 取证完成（commit d8acda9e）：forensics.md 结论=四个脚本启动点全量透传零丢失（E1/E2/E3/E3b 六断言 ALL_PASS exit 0，PS 5.1.26100.9711 Desktop）；真实丢失面=复用路径（:2024-2037，已声明语义）+调用会话 env 缺失（操作面）；321 观察精确归因、与 322 实证矛盾消解；停止条件（非脚本层丢失/有意安全设计）均未触发。
- [x] S4 修复完成（commit 见分支）：dev_agent_smoke.ps1 +27（主契约块 ~1990+P2 ~7625+P3 ~9650 指针注释）/restart_agent.ps1 +7/新兄弟 scripts/restart_agent_env_passthrough_check.ps1（断言工具：外栈守卫 exit 2/phase1 设缝型 probe 变量→dev_agent_smoke -SkipBuild -RestartAgent 真重启→psutil 活体读回断言/phase2 无变量回归断言零变化/finally 拆栈+§10 global_project_config.json 快照还原哈希亲证；退出码 0/1/2/3/4/5/6 分型）；既有脚本 diff 纯注释零行为变化；三文件 PSParser PARSE_ERRORS=0。

## 下一步：S5 真栈轻腿（§2.2 全程）

- 预构建命令（worktree 无 gitignore 二进制，先建后跑；agent 二进制不在被测面，仅脚本域变更）：
  `cd D:/Vit_DAW_wt_env1/agent && go build -o bin/VitAgent.exe ./cmd/vitagent`
- 验证命令：`powershell.exe -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_env1/scripts/restart_agent_env_passthrough_check.ps1 -RepoRoot D:/Vit_DAW_wt_env1 -ArtifactsDir <runs>/verify_<stamp>`
- §2.2 流程：读 resources 顶部→独立净空亲核（五端口+五进程名）→占用登记推 main→≤60s 复核→跑验证（agent-only 先试，state gate 若需内核则 -StartKernel 重跑）→finally 拆栈释放亲核→释放登记推 main。
- 若无内核 state gate 失败：记录 typed 失败→改用 -StartKernel 重跑（占用登记相应放宽内核端口）。

- [x] S5 真栈轻腿完成：占用登记 3ce55cac（main 亲证 22:15，净空亲核 22:14 五端口+五进程名=0）→ 轮 1 22:20 装置缺陷轮（StrictMode PropertyNotFoundStrict；phase1 数据面已阳性 pid=3812 两键=1；finally 拆栈+配置还原完成；根因修复 commit af645034）→ 轮 2 22:30 **有效轮 ALL_PASS exit 0**（phase1 透传：真栈 agent pid=7156 两键=1+烟测 exit 0；phase2 回归：pid=7116 两键缺省+烟测 exit 0 零变化；无内核 state gate 通过，-StartKernel 预案未启用）→ 拆栈五端口独立复核 BUSY_COUNT=0 + global_project_config.json 哈希前后一致 4B835024…C48AA（§10）。细节见 RUN-RECORD.md。

## 下一步

- S6 回执：卡面回填+doing→done+coord-only 推 main；实现留 port/fix-restartagent-env-1 待主管验收（合入 main 由决策侧 cherry-pick）。

## 纪律备忘

- 本 PROGRESS 与全部工件随 port/fix-restartagent-env-1 分支入库推远端（§6.1），不落主检出未跟踪副本。
- 卡片状态变更（领取/完成）走 coord/env1-claim coord-only 推 main；实现经 port 分支待主管验收。

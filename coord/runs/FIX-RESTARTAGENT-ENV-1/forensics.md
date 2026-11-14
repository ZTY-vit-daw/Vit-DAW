# 取证清单：-RestartAgent（及同族）重启链 env 透传行为 — FIX-RESTARTAGENT-ENV-1

- 取证人：GLM-5.3-flash（freeflash）·PC·ZCode 免费车道量产执行子代理 / 2026-11-14
- 方法：代码锚点穷举（脚本域全量 grep+精读）+ 零栈机制实证（PS 5.1 代理子进程实验，零内核/零 agent/零 UI）+ 历史工件对照（321 run1 / 322 编排器）
- 证据根：coord/runs/FIX-RESTARTAGENT-ENV-1/evidence/（随 port/fix-restartagent-env-1 分支入库）

## 1. 结论（先答卡面三问）

- **哪些透传**：重启链（Stop→Start-Process 重启）对父进程环境变量是**全量透传**。调用方 shell 会话 env → powershell.exe 脚本进程 → `Start-Process` 出的 VitAgent.exe，逐边界全量继承；脚本自有 additive 变量（draft root / write-lease / telemetry / llm-timeout）叠加其上。四个脚本启动点**无白名单、无清除、无 -UseNewEnvironment**。
- **哪些丢**：重启链本身**零丢失**。真实丢失面只有一个——**复用路径**：agent 端口已在听且未传 `-RestartAgent` 时脚本复用现役 agent（dev_agent_smoke.ps1:2024-2037），现役 agent 的 env 冻结在**它自己启动时刻**，之后任何会话新设的变量都不生效（实验 E3 实证）。脚本注释 :1976-1980 已明示此语义（"a reused agent will not pick it up"）——属**已声明的有意设计**（env 生效须重启），非缺陷。次候选（装置操作面，非脚本层）：变量设到 User/Machine registry 作用域或另一会话，实际调用进程 env 本就没有——.NET `SetEnvironmentVariable(...,"User")` 不改当前进程 env 块（文档化行为，本卡未做 registry 写实验）。
- **在哪个进程边界丢**：复用场景不在"进程边界"丢，而是**根本不发生新进程边界**——没有启动新 agent，env 自然不更新。重启链的每个真实进程边界均实证零丢失（E1/E2/E3b）。

## 2. 代码锚点（file:line，基线 5bb10b0a）

### P1 主重启链（-RestartAgent）
- 停止：`scripts/dev_agent_smoke.ps1:1811-1817`（带构建腿）/`:1830-1837`（-SkipBuild 腿）——`Stop-AgentForPaths`（定义 `:1180-1200`，纯进程停止，无 env 操作）
- 启动：`scripts/dev_agent_smoke.ps1:2012` `Start-Process -FilePath $AgentExe -ArgumentList $args -WorkingDirectory $AgentDir -WindowStyle Hidden`——裸 Start-Process，全量继承脚本进程 env
- 复用判定：`:1987-1989`（重采样 listener）→ `:2004` 端口净空才启动 → `:2024-2037` 端口已被占=复用现役 agent（env 不刷新）+ SMOKE-STACK-GUARD-1 外栈防御 `:2030-2032`
- 脚本自有 additive env（均在 agent 启动前 Set-Item，caller 传入值优先）：`VIT_HISTORY_DRAFT_ROOT` :1373-1385、`VIT_WRITE_LEASE_EVENTS_PATH` :1960-1964、`VIT_AGENT_LLM_TELEMETRY_PATH` :1969-1975、`VIT_AGENT_LLM_TIMEOUT_SEC` :1982-1985
- 脚本临时变量自恢复（仅涉自身加的变量，不清 caller env）：`VIT_PROJECT_XML`（journey 内核腿）:1856-1868、`VIT_SKIP_DEV_AUTOSTART`（Godot 子进程）:1903-1915

### P2 harness_ab 逐模式重启（同族腿）
- `scripts/dev_agent_smoke.ps1:7590-7608`——Stop-Process :7591 → additive `VIT_DAW_HARNESS` :7596 + `VIT_AGENT_LLM_TELEMETRY_PATH` :7598 → Start-Process :7604。同 P1 语义。

### P3 chat auth stamp 中途重启（同族腿）
- `scripts/dev_agent_smoke.ps1:9619-9629`——`Stop-AgentForPaths` :9619 → Start-Process :9629。同 P1 语义。

### P4 独立重启脚本
- `scripts/restart_agent.ps1:24-25`——裸 Start-Process，全量继承调用方 env；:10-13 additive draft isolation（`vit_stack_draft_isolation.ps1`）。

### Go 侧（只读取证，零改动）
- agent 源码对 `dev_agent_smoke`/`restart_agent` **零引用**（grep agent/ 全仓零命中）→ 重启链纯脚本侧，agent 不自行重启栈。
- `agent/internal/shelltools/shelltools.go:45` `exec.CommandContext(...)` 不设 `cmd.Env` = Go 子进程全量继承 agent env；`:12-19` allowlist 仅 go test/build/doc、git -C、gofmt、cmake --build——**powershell/脚本不在白名单**，agent 无法经 shell 工具调重启脚本（:39-41 fail-closed）。
- → 卡面停止条件"丢失点在非脚本层（Go agent env 处理）"**未触发**。

## 3. 实验证据（零栈；PS 5.1.26100.9711 Desktop）

实验脚本：evidence/env_inheritance_experiments.ps1；结果：evidence/env_inheritance_experiments_result.json；命令 `powershell.exe -NoProfile -ExecutionPolicy Bypass -File ...env_inheritance_experiments.ps1 -OutDir ...evidence` **exit 0**，六断言 ALL_PASS：

| 实验 | 模拟边界 | 断言 | 结果 |
|---|---|---|---|
| E1 | 脚本进程→Start-Process 子进程（:2012 同型） | PS 进程 `$env:X=1` 后 Start-Process 子进程 env 含 X | PASS |
| E2 | 调用方 shell→powershell.exe -File→Start-Process 孙进程（AGENTS §1 调用形态，A+B 两边界） | 外层设的 X 穿两层到达孙进程 | PASS |
| E3 | 复用路径（:2024-2037 同型）：进程启动后会话新设变量，进程不重启 | 运行中进程 env 含旧变量、**缺**新变量 | PASS |
| E3b | 重启路径（P1 同型）：同会话停止旧进程再 Start-Process | 新进程 env 同时含旧+新变量 | PASS |

证据文件：e1_child_env.txt / e2_grandchild_env.txt / e3_surrogate_env_snapshot1.txt / e3_surrogate_env_snapshot2.txt（各含完整 env 快照，probe 行在场）。

## 4. 历史观察对照（321 观察与 322 实证的矛盾消解）

- **321 run1 INVALID**（coord/runs/RACK-PARAM-PERSIST-FIX-1/stack_persist_probe/stack_receipt.json probe_runs.run1）：起栈 agent 未带 BAREFREE 缝 env，EQ 治理守卫拦裸写；处置=带 env 重启 agent（psutil 亲证）后 run2/3/4 全 PASS——**重启后 env 生效 = E3b 同型实证**，321 现场本身已证明重启链透传可行。321 域外记录（done/2026-11-08-RACK-PARAM-PERSIST-FIX-1.md:34"起栈不传递调用方 PS 会话 env"）表述过宽：receipt 未记录起栈会话的 env 态与是否复用，按本卡取证，精确归因=复用路径（E3）或调用会话 env 缺失（操作面），非重启链丢失。
- **322 编排器链透传成功**（coord/runs/PLUGIN-CONTROL-EVAL-PROQ3-FREE-1/RUNBOOK.md:60 + orchestrator phase 2b psutil 活体守卫）：编排会话设缝 env → dev_agent_smoke -StartKernel -RestartAgent → agent 进程两键非"1"即 exit 4——024318 首跑在 phase 2b **通过**（败于 phase 2c go test 面，f9ef49f2 已修），即编排→脚本→agent 全链透传实证。
- 合并结论：**重启链全量透传成立**；不一致的来源是复用路径与调用会话 env 态，两者都是操作契约问题而非机制缺陷。

## 5. 修复方向（卡面目标 2）

按脚本既有约定择**全量透传**（P1-P4 现状即此，additive-only）：
1. 契约注释一致化：P1/P2/P3/P4 启动点补齐同一份透传契约注释（全量继承+additive-only+复用不刷新+停止条件交叉引用），把隐式行为显式化为可审计契约；
2. 新兄弟验证脚本 `scripts/restart_agent_env_passthrough_check.ps1`：把契约变成可断言面——设缝型 probe 变量→经 dev_agent_smoke.ps1 -SkipBuild -RestartAgent 真重启→psutil 活体读 agent 进程 env 断言一致→清变量回归腿断言不设变量行为零变化；自带外栈守卫（发现非本腿栈即拒跑）与 §10 global_project_config.json 快照还原。
3. 行为零变化：注释与新增脚本不改任何既有执行路径（P1-P4 代码零 diff）。

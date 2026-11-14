# FIX-RESTARTAGENT-ENV-1：-RestartAgent 环境变量透传（PENDING 分流②）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=gate 2026-11-09 §三 + 322 首跑 024318 取证：编排进程 SEAM env 与重启 agent 的 env 面不一致，f9ef49f2 已修 go test 子进程面，脚本透传面挂账）
- 派发确认：已确认（gate 2026-11-09 §六 首排④ PENDING×4 分流）
- 验收负责人：GLM 主管决策流
- 池序 326；P2 / 0.5 天 / 依赖=零；L1 / PC；脚本域

## 目标

1. 复现取证：`dev_agent_smoke.ps1 -RestartAgent`（及同族脚本重启 agent 的路径）对父进程环境变量（如 `VIT_PLUGIN_CONTROL_EVAL_SEAM` 系）的透传行为——哪些透传/哪些丢/在哪个进程边界丢；
2. 修复：重启链路 env 透传一致化（明确白名单或全量透传，按脚本既有约定择一并写注释）；
3. 验证：设变量→起栈重启 agent→agent 进程内读回（health/debug 面或日志锚点）断言一致；不设变量时行为与现状零变化（回归）。

## 文件域

`scripts/dev_agent_smoke.ps1` ±同族脚本；零 agent 源码；零内核。

## 验收标准

一次真栈轻窗口（可与 282 轻腿同窗）：透传断言 exit 0 + 无变量回归 exit 0；栈协议两条登记。

## 停止条件

透传丢失点在非脚本层（Go agent 自身 env 处理）→ 取证上交接域（agent 源码域另卡）。

## 领取/回执/验收

- 领取：2026-11-14 晚窗 / origin/main=5bb10b0a04c63f9e804fc1a4e88fb8bbb53a02b9（fetch 亲证；=派发基线 bfe0356e 的后代，bfe0356e..5bb10b0a scripts/ 域零变更）/ owner=GLM-5.3-flash（freeflash）·PC·ZCode 免费车道量产执行子代理（主管会话 2026-11-14 晚窗派发，池序 326）/ 分支=port/fix-restartagent-env-1 / worktree=D:/Vit_DAW_wt_env1 / 领取提交=coord/env1-claim→main（2318e22c，被拒一次=347 终验落 main，§2.1.3 重放后推成）。在飞卡域核对：353 ENV-GODOTCPP-RESTORE-1（godot-cpp 仓外目录+工件域）、347 UI-LOADING-1（webui loading）、329 WAVESHELL（取证已毕仅 chat 实施腿）、273/271（agent Go 域）——与本卡（scripts/dev_agent_smoke.ps1+restart_agent.ps1±新兄弟验证脚本）均不相交。工件/断点=coord/runs/FIX-RESTARTAGENT-ENV-1/PROGRESS.md（随 port 分支推远端）。
- 回执（2026-11-14 22:4x 收口）：**执行侧自验完成（doing→done 待验收）**；实现分支=port/fix-restartagent-env-1（远端在库）：45261f9f 契约注释+断言工具 / af645034 装置轮1修复 / ff14ee68 run 工件；工件根=coord/runs/FIX-RESTARTAGENT-ENV-1/（forensics.md+RUN-RECORD.md+evidence/ 零栈实验+verify_ 两轮，随分支入库）
  - **取证结论（卡面目标 1，清单=forensics.md §2 全锚点）**：四个 agent 启动点（dev_agent_smoke.ps1 主链 :2012 / harness_ab :7604 / auth stamp :9629 / restart_agent.ps1:25）全部裸 `Start-Process`=**全量透传零丢失**（无白名单/无清除/无 -UseNewEnvironment）；脚本 additive env（draft root/com root/lease/telemetry/llm-timeout）叠加其上；Go 侧零引用重启脚本+shelltools.go:45 全量继承+:12-19 allowlist 不含脚本=**停止条件（非脚本层丢失）未触发**。真实丢失面=**复用路径**（dev_agent_smoke.ps1:2024-2037 未传 -RestartAgent 时现役 agent env 冻结在其启动时刻，:1976-1980 已声明语义=**有意设计**）+调用会话 env 缺失（操作面，含 registry 作用域陷阱）。321 域外记录（done/:34"起栈不传递调用方 PS 会话 env"）精确归因：run1 INVALID=复用路径或起栈会话无 env，非重启链丢失（321 现场带 env 重启后 run2/3/4 全 PASS=重启链透传自身实证；322 编排链 phase 2b psutil 亦实证）。零栈机制实验 E1/E2/E3/E3b 六断言 ALL_PASS exit 0（PS 5.1.26100.9711 Desktop）。
  - **修复（卡面目标 2）**：按既有约定择**全量透传**（additive-only）——dev_agent_smoke.ps1 主契约块（~:1990）+P2/P3 指针注释+restart_agent.ps1 契约注释（共 +34 行**纯注释零行为 diff**，PSParser PARSE_ERRORS=0×3 文件）+新兄弟断言工具 scripts/restart_agent_env_passthrough_check.ps1（外栈守卫 exit 2/phase1 透传断言/phase2 无变量回归/finally 拆栈+§10 配置快照还原哈希亲证；退出码 0/1/2/3/4/5/6 分型）。
  - **验证（卡面目标 3）**：真栈轻腿轮 2 **ALL_PASS exit 0**——phase1 会话设 VIT_PLUGIN_CONTROL_EVAL_SEAM=1+BARE_WRITE=1→dev_agent_smoke -SkipBuild -RestartAgent 真停真启 exit 0→psutil 活体读 agent pid=7156 两键均="1"；phase2 清变量重启→pid=7116 两键缺省+烟测 exit 0 零变化；无内核 state gate 通过（-StartKernel 预案未启用）。§2.2 栈协议全程：净空亲核 22:14（五端口+五进程名=0）→占用 3ce55cac→轮 1 装置缺陷轮（StrictMode 可选属性崩溃；数据面已阳性；根因修复）→轮 2 有效轮一次通过→拆栈独立复核 BUSY_COUNT=0→释放 54089017；§10 global_project_config.json 哈希前后一致 4B835024…C48AA。
  - **端测边界声明**：本卡验收面=脚本重启链 env 行为（进程 env 活体读回断言），无渲染面/用户旅程主张；零 agent 源码/零内核/零前端改动；agent 二进制仅为 restart 载体（-SkipBuild，sha256 52569D9B22A582DC6839D47E0D068470B037E6388C483CC877C1BB269FBC79C4，worktree 现场构建 exit 0，不在被测面）。失败分类如实记录：轮 1=装置缺陷轮（非环境中断/非功能缺陷），轮 2=有效轮。
  - 停止条件：未触发。遗留边界一条：restart_agent.ps1:5 硬编码绝对路径 `D:\Vit_DAW\agent\bin\VitAgent.exe`（与 dev_agent_smoke.ps1 的 Resolve-RepoRoot 不一致，非 env 主题，卡面域外未动，供主管裁量）。
- 验收：（主管回填）

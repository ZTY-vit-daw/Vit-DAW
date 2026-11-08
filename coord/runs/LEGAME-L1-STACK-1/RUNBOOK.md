# LEGAME-L1-STACK-1 RUNBOOK（§8 跑前口径，2026-11-08 深夜成文）

- 卡：314 LEGAME-DIR-1 + 315 LEGAME-AUTOINTRO-1 + 316 LEGAME-FRESH-1（同 owner 三卡真栈腿合一栈窗口）
- 分支 `port/legame-l1-1`（bdf2e0fc+cd5aa5b0+c34aed13+栈腿工具笔）；worktree `D:/Vit_DAW_wt_314l1`
- agent=wt_314l1 现场构建 VitAgent.exe（ps1 -SkipBuild 未用=每 run 现建，二进制即工作树）；kernel=wt_228 VitApp.exe `F9E3DA0A…0D67A` 只读复用（225-312 同一二进制）

## 跑前一次性布位（栈外，腿前执行）

1. `python scripts/legame_placement.py install-levels`（装入**真实** `~/.vit/levels/first-run-twinkle/`——dir314 腿的产品级证明面；幂等，安装后该机乐局卡解析默认走产品级，repo 开发态需 env 覆写[既有语义]）
2. RUNBOOK 本文件+栈腿工具笔 commit 推送（工件纪律 §6.1）

## Run 清单（单 run 无重跑；三 run 三目的，非互为重跑）

| Run | 卡/验收点 | 命令（PS，仓库根） | 前置 |
|---|---|---|---|
| A | 314：无 env 非 repo cwd 三件套→「开始乐局」enter 生效→exit | `.\scripts\dev_agent_smoke.ps1 -StartKernel -RestartAgent -LegameSmoke -LegameMode dir314 -NoChatSmoke -NoStripSilenceSmoke -RunArtifactsDir coord\runs\LEGAME-L1-STACK-1\runA_<stamp>` | install-levels 已跑；驱动断言 VIT_EDU_LEVELS_DIR 未设（ps1 env 块清除+亲检 throw） |
| B | 315：白话触发自动开场（无命令词）→binding 落盘→新会话恢复播报；316：目标工程哈希不动（预演副本制） | 同上 `-LegameMode intro` `-RunArtifactsDir ...\runB_<stamp>` | runA 栈拆净 |
| C | 315×316：relay（真命令，重铺 runB 污染过的副本工作区）→新 agent 进程白话→**再开场**（重铺=新工程语义）；重铺后三问（exit 0 在卷/哈希=模板/残留零）腿外亲核 | `python scripts\legame_placement.py relay --project <runB副本路径> --output ...\runC_relay_summary.json` → 同上 `-LegameMode intro -LegameProject <runB副本路径>` `-RunArtifactsDir ...\runC_<stamp>` | runB 栈拆净；relay exit 0 后再启栈 |

- 每 run 间 owner 亲拆栈并核净（dev_agent_smoke 完成态不自拆，设计如此）；三 run 同属一次栈占用登记窗口（§2.2 一次登记，run 间进程归零复核在卷）。
- 工件目录=coord/runs/LEGAME-L1-STACK-1/<runX_stamp>/（新目录不覆盖；随 port/legame-l1-1 推送）。

## §8 口径

- **运行次数**：每 run 各一次，禁止同 run 重跑；A/B/C 为不同验收面非重复轮。
- **预算**：乐局面确定性零 LLM（telemetry 断言 jam 会话 0 行）；诚实上界=若 intro/broadcast 未拦截，白话轮漏进语义入口计费 ≤2 调用/会话 deepseek-flash（三 run 合计上界 ≤6 调用，≪¥0.01，失败症状证据记录不计成功）。
- **成功条件**：run A=py exit 0（edu_jam_entered+first-run-twinkle 叙事+exit 终态）；run B=py exit 0（白话 edu_jam_entered+binding opened 落盘+新会话 edu_jam_recovery_broadcast+%APPDATA% 目标哈希=e0e95338…2441e3）；run C=relay exit 0+py exit 0（再开场）+重铺三问腿外亲核；PS1 退出码 0。
- **失败分类**：py exit 2=环境（栈/传输/fixture/激活窗口耗尽——门拒绝 by-design）；py exit 4=确定性面缺陷（自动开场未触发/播报错/卡非产品级装载/目标哈希漂移）取证上交不自修；PS1 throw=对应腿失败；wrapper 假启动（栈零启动）不计 run（266 先例）。
- **止损线**：同一确定性断点禁止原样重跑；先取证分列（环境/面缺陷），面缺陷上交主管。
- **预演副本制**：全程禁在 `%APPDATA%` 目标工程实跑乐局面——A/B/C 均用隔离副本（run 工件目录 place-copy/relay）；目标工程仅哈希只读断言。
- **栈纪律**：占用登记推 main 成功→≤60s 即时复核（六口 TCP 5555/5556/5557/7878/8787/18787 listen=0+UDP 4444/4445=0+VitApp/VitAgent/vsphub/Godot/python 进程全零）→启栈；317 在栈则等（不覆盖不抢登）；finally 亲拆+TEARDOWN_ALL_ZERO 亲核+释放登记。
- **边界声明**：agent HTTP/ZMQ 面；渲染面/用户旅程面不在覆盖（AGENTS §5；GUI 最后一米=用户手测本体）。

# LEGAME-L1-STACK-1 真栈腿证据索引（2026-11-09 00:0x 收口）

- 卡：314 LEGAME-DIR-1 / 315 LEGAME-AUTOINTRO-1 / 316 LEGAME-FRESH-1（同 owner 合一栈窗口）
- 栈占用登记：main `421f1665`（2026-11-08 23:54；净空两轮亲核 23:53:36/23:53:57 全零+推送后 ≤60s 复核 23:55:17 全零；320 23:41 释放亲证）
- 被测：agent=wt_314l1 现场构建（go build，非 -SkipBuild；源 HEAD=`9b87531e` @ port/legame-l1-1；runA/B/C 同源）；agent 二进制 SHA256 `65b9976bcd4b982a65f847bb7b005c960833384b9dae124e803abfbdf40bbaca`；kernel=`D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe` SHA256 `f9e3da0a66263f0b6e3fed5cda0fb214c460fce8053234240c470d515ed0d67a` 只读复用（登记同款）
- 前置：`python scripts/legame_placement.py install-levels` → 真实 `~/.vit/levels/first-run-twinkle/`（8 文件 sha256 核对；此后本机乐局卡默认解析走产品级——repo 开发态需 env 覆写，既有语义）

## Run 账

| run | 目的 | 完整命令（要点） | 退出码 | 结果 |
|---|---|---|---|---|
| runA_launch（不计 run） | — | 同 runA 但缺 `-KernelExe`（默认 Export\staging 不在 worktree） | py 2 / PS1 1 | 栈零启动类（266 先例）；零 LLM 零断言；残留 VitAgent 16380 亲拆 |
| runA_attempt2（环境类取证） | dir314 | 见 runA | py 2 / PS1 1 | 激活窗口耗尽（30s）——**根因=驱动用裸 ZMQ open_project，不建立 agent 工作区身份**（311/281 流程实为 save_as_project 建立+启动页 invoke 面传播；l4_genesis 场景同款 `version.project_opened` 面实证）；驱动改走产品启动页面（commit `9b87531e`）；零 LLM 零乐局断言；栈残留亲拆 |
| **runA**（有效） | 314：无 env 非 repo cwd → enter 生效 | `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -StartKernel -RestartAgent -KernelExe <wt_228 kernel> -LegameSmoke -LegameMode dir314 -NoChatSmoke -NoStripSilenceSmoke -RunArtifactsDir coord\runs\LEGAME-L1-STACK-1\runA_20261109_000437` | **PS1 0 / py 0** | 启动页面进入（project.open+version.project_opened+project.state 取 uuid）→ activated=true →「开始乐局」**enter_attempts=1** → edu_jam_entered/first-run-twinkle/开场叙事 →「结束乐局」edu_jam_exited；VIT_EDU_LEVELS_DIR 全程未设（ps1 env 块清除+驱动亲检+ps1 throw 三重守卫）——**卡自 ~/.vit/levels 装载实证** |
| **runB** | 315：白话自动开场+恢复播报；316：目标零接触 | 同上 `-LegameMode intro` `-RunArtifactsDir ...\runB_20261109_000508` | **PS1 0 / py 0** | conv-A 白话「在吗」（无命令词）→ **edu_jam_entered**（开场叙事）；binding 回写 opened_at=2026-11-08T16:05:15Z+opened_conversation_id=conv-A；conv-B「嗨」→ **edu_jam_recovery_broadcast**「接着上局：拍谱开局（1/7）——继续或问我都行。」；%APPDATA% 目标 SHA256==e0e95338…2441e3（预演副本制）；两会话零 LLM 行 |
| relay（runC 前置） | 316：真 relay 命令对 runB 会话污染态 | `python scripts/legame_placement.py relay --project <runB 副本> --output ...\relay_summary.json` | **0** | 重铺前 .vit_agent={binding+vitproj 工作区}+.vit_history 会话残留 → 重铺后工程 sha256==模板 / .vit_agent 仅 binding（opened_at=null，bound_at 溯源保留）/ .vit_history 消失——**重铺三问全过** |
| **runC** | 315×316：重铺后再开场（新工程语义） | 同 runB + `-LegameProject <runB 副本>` `-RunArtifactsDir ...\runC_20261109_000557` | **PS1 0 / py 0** | 新 agent 进程白话 → **自动开场再次触发**（edu_jam_entered，opened_at=16:06:03Z 重写为新会话）；新会话恢复播报照常；目标哈希不动；零 LLM |

- 工件：本目录 run{A,B,C}_*/（summary/console/chat_responses/place_copy/relay/gui 隔离件）；probe2.vit/probe_save_as.vit=attempt2 取证期的 ZMQ 探针落盘（保留作取证）
- 计费：三有效 run 乐局/开场/播报会话 **telemetry 零行**（driver 断言 conv 键 0/None）；attempt1/2 零 LLM；合计 ≈¥0
- 栈纪律：owner 亲拆 VitAgent/VitApp ×3（16868/7800、9560/16224、15932/21096），**TEARDOWN_ALL_ZERO 亲核 00:06:13**（六口 TCP listen=0+UDP 4444/4445=0+VitApp/VitAgent/vsphub/python 全零）；释放登记随本笔
- 边界（AGENTS §5）：agent HTTP/ZMQ 面（真实内核+真实 agent）；Godot 聊天面板视觉呈现不在断言面——GUI 最后一米=用户手测本体；工程进入腿复刻启动页 API 面（project.open/version.project_opened，l4_genesis 同款），Godot GUI 进程未在场

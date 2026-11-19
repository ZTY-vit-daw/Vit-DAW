# 裁定：TUTOR-SIM-2 UI 自动开场 PASS

- 日期：2026-11-19 22:48 +0800
- 卡：436 TUTOR-SIM-2
- 裁定：**ProjectSurface 工程打开自动开场 + 真实 VitShell 三步手测 PASS**。
- 代码：ca65166d → main 28cde623；UI 主管亲跑 npm run test 65 文件/771 tests exit 0、npm run build exit 0；dist index 419BFC44…。
- 手测窗口：coord/runs/TUTOR-SIM-2/berth_20261119_2240（recents/authority before；agent_last.log/tutor_lines.log；teardown）。

## 手测事实

- 用户打开 D:/Vit_DAW_handtest/TUTOR-SIM-1/live.vit（含 .vit_agent/tutor_sim.yaml），未输入任何启动命令。
- agent log 22:46:51 [tutor_sim] start conversation=tutor_ca3e0b07053c89a3 experiment=tutor-sim-3step project=... step=1/3，证明 ProjectSurface 自动调用 /agent/tutor/start。
- 22:47:02 / 22:47:07 两次 turn passed=false stop=tutor_sim_retry，未推进。
- 22:47:11 step2 passed=true；22:47:15 step3 passed=true；22:47:19 completed=true stop=tutor_sim_completed。
- 用户确认“没问题，可以通过”；无「开始乐局/写好了/乐局状态」命令词；真实 flash。
- 普通工程 start 探测返回 unavailable 时 UI 静默；composer 后续消息使用 tutor conversation_id。
- teardown：Vit 系进程/TCP 六口全零；recents/Settings/default_project 还原一致（tutor_lines=6）。

## 覆盖边界

- 本轮无状态/进度卡、无问题/输入控件、无谱面/音频；这些是下一段。
- 无浏览器级 DOM 自动化烟测；本次真实 VitShell 手测是唯一渲染/旅程证据，已由用户亲测。
- 导师会话仍为服务端内存态；agent 重启需重新 start（本轮接受）。
- 实验 YAML 是 vit.tutor_sim.v0，不是生产 design.v1；生产节点的 teaching/model_does/材料/检查尚未接入。

## 下一步

- TUTOR-SIM-3：状态/进度卡 + 问题/输入卡（对话流内网页控件），再接生产 YAML 节点 brief/材料/音频；音乐控件（谱面/Tap/播放器）随后。
- 433 A2a 作品反馈作为导师工具候选；434 产品 v0 superseded。

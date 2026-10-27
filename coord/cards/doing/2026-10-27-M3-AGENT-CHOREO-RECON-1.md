# M3-AGENT-CHOREO-RECON-1：M3 步骤④勘察——意图层包络指令+范围 goal 编舞单轮终态+披露面解锁现状

- 发卡：GLM 主管决策流 / 2026-10-27 早窗（依据=phase-ordering-v2 M3 第 4 条「agent 范围 goal 编舞（单轮终态，原 FREESTATE IMPL-1/2 并入）+意图层包络指令」+gate 2026-10-26 §5 首排③+决策档 2026-10-25-form-rulings-user-first：**形态类先过用户面**）
- 派发确认：已确认（gate 2026-10-26 首排）
- 验收负责人：GLM 主管决策流
- 池序 197；目标仓库=**主仓 D:\Vit_DAW**（agent/ 只读勘察；零生产改动）；P2 / 0.5-1 天 / 依赖=无（M3-KERNEL-1 catalog/harness 面已合入 ✓；186-195 前端包络链闭环 ✓）
- 模型分级：L1 / PC 执行会话
- 并行与资源：纯只读勘察——无真栈腿、无文件域冲突（工件落 run 目录）。

## 目标（勘察七问；产出=锚点+现状+形态选项包，给用户过面后再拆实施卡）

1. **意图层包络指令词表现状**：意图面当前动词/词表对 automation 的覆盖（M3-RECON-1 §4 问 4 锚点核对）；「这段处理到 X」→包络指令的映射缺口清单。
2. **goal 编舞单轮终态设计输入**：FREESTATE-REGION-IMPL-1-STOP 保全面复用评估（runs/FREESTATE-REGION-IMPL-1-STOP/——range_goal 字段/武装/测试骨架哪些可复用、哪些按单轮终态重写）；goalrunner 挂点锚点核对（原卡锚 goalrunner_chat.go:280-330 分支族+audio_closure_controller admit/record 是否漂移）；原 DESIGN.md 哪些条款被 186-195 落地后已过时。
3. **披露面解锁清单**：context_manifest 四处 Excluded 现状（M3-KERNEL-1 裁定 5——automation 摘除点的具体文件/行）；catalog 三工具 spec（automation.get_curve/set_points/clear_region）与 argspec 现状复核；解锁后 agent 侧最小演示路径（一条从用户话术到内核写曲线的链，纸面推演即可）。
4. **编舞呈现面**：goal 编舞执行过程/结果在 webui/Godot 的呈现现状（观察面/回执面锚点）；单轮终态下用户可感知面缺什么。
5. **DAD 曲线回流现状**：harness 摄入白名单 automation 曲线摘要型（M3-KERNEL-1 已落）现状+证据链形态；编舞链是否消费。
6. **形态选项包（交用户面，本卡核心交付）**：≥2 案含推荐——意图层指令词表形态/编舞触发形态/呈现形态；各案成本与边界各两行。
7. **拆卡建议**：单轮终态实施分段建议（卡序+文件域预划+验收形态）。

## 文件域

`coord/runs/M3-AGENT-CHOREO-RECON-1/`（勘察报告+形态选项包）——**零生产文件改动**（agent/ VitApp/ webui/ 前端仓全只读）。越域即停。

## 验收标准

勘察报告入 run 目录（七问逐问有锚点或「锚点漂移」如实标注；形态选项包含推荐案与成本行）+卡内回执链接；工作树零生产 diff（git status 亲核）。

## 停止条件

- goalrunner/intent/context_manifest 锚点与卡面引用族大面积不符（结构已重构）→锚点清单上交；
- FREESTATE-REGION-IMPL-1-STOP 工件缺失/不可读→如实记录，复用评估降级为「无法评估」，不虚构。

## 领取

- 时间：2026-10-27 12:12（PC 本地）
- origin/main hash：2fa99bd3（领取基线，fast-forward 无冲突）
- owner：GLM-5.3（ZCode PC 执行会话）；机器=PC（Windows）
- 分支：coord/M3-AGENT-CHOREO-RECON-1-claim（协调分支，仅卡状态提交）
- worktree：D:\Vit_DAW_coord_197（勘察只读在主仓 D:\Vit_DAW 进行，工件落 run 目录后经协调分支提交）
- 领取提交：（本提交，见卡内下方回执补填）

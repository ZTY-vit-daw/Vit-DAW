# FS-FRONTIER-OBSERVE-GAP-1：candidate frontier 不含用户指名轨+新循环观察断层（取证+设计）

- 发卡：GLM 主管决策流 / 2026-10-20 深夜（依据=146 取证 L1 三缺口+实验 F-gate 机制取证：frontier=首轮 project 级观察产物不含用户指名轨 vocals/1032，模型定向观察被 final_gate 连拒 4 次→自报边界；原 146 拆卡 E）
- 派发确认：已确认（146 卡实验腿验收裁定）
- 验收负责人：GLM 主管决策流
- 池序 156；目标仓库=D:\Vit_DAW；P2 / 取证 0.5 天（实现另拆卡）/ 依赖=无；与 154/155 文件域不相交可并行
- 模型分级：L2 / GLM 执行会话
- **证据基座=[146 取证 §① L1](../../reports/2026-10-20-146-timefx-bus-forensics.md)+[实验 RESULTS §3](../../runs/146-SOFT-GUIDE-EXP/20261020_b2arm/RESULTS.md)（frontier 弹回原文+prompt_render 披露在卷）**

## 待证与设计

1. frontier 的生成面：首轮 project 级观察如何裁 candidate track set（为何 vocals 落选）、G6 target evidence 与 frontier 的耦合语义；
2. 新循环 ledger 零结转（free_state_reasoning_loop.go:265-270）+无首轮强制观察——两个 L1 缺口的修法设计（强制观察步 vs 结转 vs frontier 并入用户指名轨三案对比，涉 FS 状态机语义须谨慎）；
3. 修复对 146 惯例线的意义：154/155 的软面解释只在「模型能到提案面」时起作用——本卡是模糊意图链的前置打通；
4. 实现卡拆分建议（含 §11 持久化兼容回归要求）。

## 文件域

取证只读；产出设计件入 `coord/runs/FS-FRONTIER-OBSERVE-GAP-1/`。本卡不改生产代码。

## 验收标准

两缺口各有归因结论（代码锚点+工件回指）+修法对比与建议，交主管定稿拆卡。

## 停止条件

牵出 FS 状态机统一收敛（T10）级结构性问题 → 上交边界收窄（同 149 先例）。

- 领取：（2026-10-21 18:14 +0800 / origin/main=171640d5d91b4ede74b0ac92932a32cb3e7be7db / GLM-5.3 L2 PC 执行会话 ZCode（本会话，用户口令指定领取） / 分支=port/fs-frontier-observe-gap-1（worktree D:/Vit_DAW_wt_fsog1）+ 协调 worktree D:/Vit_DAW_wt_fsog1_coord（coord/fsog1-claim）/ 领取提交=本提交；领取核对：卡在 todo 派发已确认、依赖=无；doing 仅 GEN-SUNO-WEB-DESIGN-1（纯设计件）文件域零冲突；纯只读取证不占真栈（无需登记 PC-RUNTIME-STACK），156 与 154/155 合入后的 main 不相交可并行）
- 回执：（结论文件 / 锚点清单 / 设计稿路径）
- 验收：（裁定文件 / 验收 commit）

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
- 回执（2026-10-21 晚窗收口，GLM-5.3 L2 PC 执行会话，零生产代码改动、未占真栈）：
  - **结论文件**：`coord/runs/FS-FRONTIER-OBSERVE-GAP-1/RESULTS.md`——缺口①归因=candidate 只从三个 mix 扫描视图（`freeStateCandidateScanViewIDs`，audio_closure_controller.go:1270）的 top-k band 重叠行派生、用户意图盲；守卫（free_state_reasoning.go:1444-1461）把缺席转成不可观察+G6（free_state_gate.go:312-365）+G8（:444-484）双重锁死提案面，blocked 族唯一合法出口（模型自报=诚实行为）。缺口②归因=结转断点 free_state_reasoning_loop.go:265-270（merge 仅 active 生效，终态词表 :191-199）+blocked 族空 ledger 畅通（free_state_reasoning.go:212 段）。
  - **新 finding**：观察供应非平稳——A1/A2 同请求消费正午相差 73 秒的两份持久观察（obs_T123522 vs obs_T123635/123642），band 行内容不同；frontier 内容是 ledger 供应的非平稳函数（工件回指：logs_sge1/agent_prompt_render.jsonl+arm/{A1,A2,A3}/events.json，弹回 4+4+1 条 stage=free_state_final_gate 在卷）。
  - **设计稿**：`coord/runs/FS-FRONTIER-OBSERVE-GAP-1/DESIGN.md`——三案对比（A1 种子观察/A2 结算门槛否决/B ledger 结转/C 指名轨并入）+组合建议（B+A1 同卡无争议先行；C 优先但挂 C4 红线扩展解释+C5 G5 措辞两个裁定为前置）+拆卡 156-IMPL-CARRY / 156-IMPL-NAMED / 156-REG-RESCHEDULE+§11 五项回归要求+T10 边界（两 ledger schema 不统一，停点写死 DESIGN §4）。
  - 停止条件未触发；锚点清单=RESULTS.md §①/§② 两张事实-锚点表。
- 验收：2026-10-21 晚 / GLM 主管决策流 / **整卡 pass**：两缺口归因各有代码锚点+工件回指——缺口① F1-F9（frontier=三扫描视图 top-k band 行派生+守卫/G6/G8 三重封锁，工件回指 prompt_render 并集无 1032 实证）+缺口② L1-L5（active-only merge switch+空 ledger 畅通）；抽查锚点 freeStateCandidateScanViewIDs(audio_closure_controller.go:1270) 与 free_state_reasoning_loop.go:265-270 switch 属实；新发现「观察供应非平稳」（同请求相差 73 秒两份持久观察）在卷；三案对比+A2 否决理由（诚实边界契约）+T10 停止条件细则+§11 五项齐备；**C4/C5 主管裁定=认可**（C4 用户指名+机械名字解析=意图搬运非发明，fail-open 边界按设计稿；C5 G5 措辞扩展同步），实现卡 156-IMPL-NAMED/156-IMPL-CARRY/156-REG-RESCHEDULE 按设计稿 §3 开卡（NAMED 先行，CARRY 同文件串行随后）；验收 commit=本提交

# FIX-FS-SETTLE-DEADCHAIN-1：空态 settle 拒收后已准入实验死链修复（108 收口腿解锁）

- 发卡：GLM 主管决策侧 / 2026-10-13 深夜（依据=[TIMEFX-D1-EXIT0-DEEPSEEK-1 blocked 上交](../blocked/2026-10-13-TIMEFX-D1-EXIT0-DEEPSEEK-1.md)+[FORENSICS.md §四](../../runs/TIMEFX-D1-EXIT0-DEEPSEEK-1/FORENSICS.md)修复菜单裁定）
- 派发确认：已确认（主管依已核实上交证据发卡）
- 验收负责人：GLM 主管决策流
- 池序 109；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / 108 适配已合入 main（本卡收口腿即 108 遗留）
- 模型分级：L2 / GLM 执行会话（PC，agent 面+真栈腿）

## 缺陷机制（已核实，FORENSICS §三）

deepseek-chat 末轮提案 JSON 同时携带空态 `experiment_materiality={state:none,evaluation:not_ready}` 且 `final=true`（模型幻觉"已应用"，工程实际零插件）→ `free_state_experiment_runtime.go:649` 按字段在场判结算企图拒收 → `goalrunner_chat.go:3119` owed 扣留后不再 arm（lifecycle=none）→ 已准入实验（reverb_decay_adjust）物化/装载/执行链不发生 → interventions=0 → exit 1 "exactly one forward mutation"。两轮同断点（run 231930/232817，工件在 main）。

## 修复方向（裁定=B 主修，seam 由执行侧取证后二选一并写明依据）

- **B-i（谓词精化）**：`settleReportCarried` 不把空态 materiality（state=none 且 evaluation=not_ready 且无 target_response/round_decision）计为结算企图——字段在场≠结算声明。
- **B-ii（恢复臂）**：owed 扣留后对已准入未应用实验 arm 一次物化 continuation。
- **硬约束**：不得弱化证据门——真实 settle 仍需非空 materiality；幻觉 settle 拒收行为保持；只允许"死链→驱动已准入实验执行"的恢复语义。
- 可选硬化腿 C（若并入须注明概率性且不触盲法）：提案轮提示明令禁填 settle 三字段。A（驱动器 nudge）不并入，B 落地后评估。

## 任务

1. 取证复核（短）：读两锚点代码定 seam，B-i/B-ii 二选一并写设计依据。
2. 修复+回归测试：空态 final=true 反例——链不死（interventions≠0 路径恢复），真实空态 settle 拒收语义有测试钉住。
3. 真栈收口：`run_free_state_d1_smoke.ps1 -PromptFlavor reverb -ExpectDomain reverb` **exit 0**（建议连过 2 轮：销账+体验场次稳定性）；delay 族可选加分。需登记栈占用；LLM=deepseek-chat（勿改 flash）。
4. `go test ./... -count=1` 0 FAIL+gofmt 净（LF blob 口径）。

## 文件域

agent/internal/free_state_experiment_runtime.go + agent/internal/goalrunner（goalrunner_chat.go）+ 同包回归测试；可选提示组装面（C 腿）；领取时核对 doing 卡。

## 验收标准

diff 亲读+seam 设计依据+回归测试+95 包 0 FAIL+D1 reverb exit 0（工件含 console+summary+模型口径行）；销三笔挂账（M2-TIMEFX-1 111841/M2-TIMEFX-2 195448/FIX-106）由决策侧复核后补记。

## 停止条件

B 两半均与证据门语义冲突（修不动）→ 带设计分析上交；域外新发现→上交不扩域。

- 领取：2026-10-13 23:54 +0800 / origin/main=f937a84d（doing/ 空、无文件域冲突；栈空闲已核）/ GLM-5.3 L2 执行会话（PC，ZCode）/ port/fix-fs-settle-deadchain-1 / D:/Vit_DAW_wt_fsdc1 / 本提交（coord-only 领取提交）
- 回执：（commit / run ID / seam 选择依据 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

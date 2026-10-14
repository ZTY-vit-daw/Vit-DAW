# FIX-SHADOW-BINDING-IDENTITY-1：VSP 快照缺身份时 shadow 保留最后已知工程身份（109 线硬阻断修复）

- 发卡：GLM 主管决策侧 / 2026-10-14 晚窗（依据=[FORENSICS-LEG-E.md](../../runs/FIX-FS-SETTLE-DEADCHAIN-1/FORENSICS-LEG-E.md) §二 run9 子形态实锤+腿 E 上交裁定；109 收口腿 exit 0 前置）
- 派发确认：已确认（主管依已核实取证证据发卡）
- 验收负责人：GLM 主管决策流
- 池序 112；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：**P1（109 硬阻断）** / 0.5 天 / 无（与 113 文件域不相交可并行）
- 模型分级：L1 / GLM 或 flash 执行会话（agent 面）

## 缺陷机制（已核实，FORENSICS-LEG-E §二.2 锚点链）

mutation 后某次 VSP state.snapshot 回复未带身份三元组（revision/epoch/snapshot_hash 全空，graph_revision 计数器替代形态，run9 mix session latest_change before=11→缺失实录）→ `harness.go:1844 vspLegacyStateWithBinding` 仅在 reply 带身份时写入，refresh 走 `shadow.Initialize` **空身份整体覆盖** → 组装链 mixboard/project_package.go:10→catalog.go:1017→harness/ccb_observation.go:134-157 产出身份空包 → binding unbound（capabilitycontext/free_state_observation.go:522）→ `freeStatePostActionObservationEligible`（chat/free_state_reasoning_loop.go:1275）要求 revision 非空 → 观察永不记 post_action → D1 断言结构性不可满足。对照组 run6 同路径 revision='7' 正常=时序相关间歇缺陷。

## 任务

1. 修复（agent 侧优先）：VSP 快照 reply 缺身份三元组（全空）且 shadow 已有非空身份时**保留最后已知身份**（不空覆盖），并记 telemetry/日志标记（如 identity_preserved_on_incomplete_snapshot）供复跑核验；shadow 本就无身份（从未绑定）→ 维持 unbound 现状（fail-closed 不变）。
2. 单测钉三形态：①reply 带新身份→更新（现语义）；②reply 缺身份+shadow 有→保留旧值+标记；③reply 缺身份+shadow 无→unbound 不变。
3. `go test ./... -count=1` 0 FAIL+gofmt 净（LF blob）。

## 文件域

agent/internal/harness.go（vspLegacyStateWithBinding/shadow 刷新一带）+同包或就近测试；**不得动** mixboard/capabilitycontext/chat 组装链（那是后果不是病因）；kernel/VSP 侧恒带身份为后备方案不在本卡（agent 侧修复后若真栈仍复现身份丢失再议 C++ 面）。

## 验收标准

diff 亲读+三形态单测亲跑+95 包 0 FAIL；修复后由决策侧安排 109 rerun5（真栈复跑归 109 线，本卡不占栈）。

## 停止条件

保留语义与 shadow 既有 checkpoint/回滚语义冲突（保留旧身份会污染别的读方）→ 带冲突读方清单上交。

- 领取：2026-10-14 19:05 +0800 / origin/main=8291d42a（todo 在位、派发已确认；doing 三卡域=experiment_runtime+goalrunner / 前端仓焦点日志 / agentloop free_state_reasoning+free_state_gate+chat prompt，均与本卡 harness 域不相交） / GLM-5.3-Flash L1 执行会话（PC Windows，ZCode） / port/fix-shadow-binding-identity-1 / D:/Vit_DAW_wt_shadowbind1 / 领取提交=本提交
- 回执：（commit / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

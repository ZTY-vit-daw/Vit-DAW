# MIXLAYER-M1-1：黑板持久层+settle 供血桥（PC 混音线 M1 五序之首）

- 发卡：GLM 主管决策侧 / 2026-10-11 早窗（依据=用户 /morning 裁定"PC 混音线首批立卡"；排序依据=MIXLAYER-RECON-1 M1 表+MIX-CAP-RECON-1 §3"仍成立"结论）
- 派发确认：已确认（用户 2026-10-11 /morning 选定 PC 混音线首批立卡）
- 验收负责人：GLM 主管决策流
- 池序 64；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 1.5-2 天 / 无（M1 五序公共前置；M1-2/M1-5 依赖本卡合入）
- 模型分级：L1 / GLM 或 flash 执行会话（PC）
- 晚窗参考序：①（早窗即可领；晚窗若仍未领则为晚窗起步卡）

## 目标（MIXLAYER Q4 五件事之①-④ + 黑板报告改读持久层）

1. **键控桥**：conversation→project_uuid 映射。锚点：`ensureFreeStateExperimentCheckpoint`（agent/internal/agentloop/free_state_experiment_runtime.go:322）已从 `loop.LatestProjectChange` 取 checkpoint_ref/commit_id——工程历史检查点锚已存在于自由态 loop 内，直接作桥接键。
2. **持久化层**：settle 结论（含用户人耳判定 outcome）落工程级存储（账本 `decisions/` 同级或黑板专属目录，落点执行侧定但须工程级可寻址），跨会话/跨天可读（现状：Turn 只活内存，settle 按 conversation_id 键控、账本/黑板按 project_uuid 键控——身份轴错位即本卡要解的题）。
3. **结论单元投影**：settle 报告→黑板可读单元；与账本 impact 契约对齐（writes/recheck_on 维度）或作独立"自由态结论"黑板块；needs_review 联动沿用账本既有机制。
4. **黑板消费**：`agentloop/project_blackboard_report.go` 增读该持久层（现状只读 project.state+会话内存，读不到任何跨会话状态）。

## 文件域

agent/internal/agentloop/（blackboard report+settle 供血面）+ 新持久层落点（工程级目录）+ 对应单测；越域停下上报（AGENTS §11）。

## 并行与资源

与 M1-3/M1-4（chat capability seam）文件域不相交，可并行；PC 真栈烟测按 coord/resources/PC-RUNTIME-STACK.md 串行占用。

## 验收标准

- 单测：旧工程无持久层文件→黑板报告缺省语义明确+旧记录反序列化/往返测试（AGENTS §11）；settle→持久→跨会话读回断言。
- 真栈烟测（AGENTS §5，扩展现有 ps1 体系不另起）：两次会话/重启间 settle 结论存活且黑板报告可见，exit 0。
- `go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

checkpoint_ref/commit_id 桥接锚点与卡面不符（上游已变）→ 锚点上交。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 跨会话存活证据 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

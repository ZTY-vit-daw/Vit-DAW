# MIXLAYER-M1-2：mix_workflow_queue.v0 视图化（黑板投影，非独立存储）

- 发卡：GLM 主管决策侧 / 2026-10-11 早窗（同 M1-1 依据；视图化路径=MIXLAYER M1 表降本裁定 1-2 天→0.5-1 天）
- 派发确认：已确认（用户 2026-10-11 /morning 选定 PC 混音线首批立卡）
- 验收负责人：GLM 主管决策流
- 池序 65；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / **MIXLAYER-M1-1 合入后领取**
- 模型分级：L1 / GLM 或 flash 执行会话（PC）

## 目标

`mix_workflow_queue.v0`（权威节点集=docs/agent_action_workflow_v1_master_plan.md §6.1：A1-A5/B1-B4/C1-C5/D1-D4/E1-E4/F1）实现为 **M1-1 黑板持久层之上的投影视图**：节点队列+9 态状态机+agent 读写面；**不为 queue 另建独立持久存储**。

- 现状锚点：mix_workflow_queue 全仓零命中（纯设计，MIXLAYER/MIX-CAP 双证）；A-F 各节点实际状态以 MIX-CAP-RECON-1 §3 对照矩阵为准——queue 视图按节点真实状态渲染，**本卡不做能力补齐**（能力开发属 M1-3/M1-4/后续批次）。
- **红线（F=固定编排淘汰方向，A-F 纪律序）**：queue 不得成为新的固定编排执行入口——它是黑板投影视图：agent 只读驱动+节点状态回写；执行仍走自由态循环内能力。

## 文件域

chat/ 或 agentloop/ 新增 queue 视图文件+黑板持久层读取；与 M1-3/M1-4 潜在交集=capability_routing.go（若 queue 视图需注册路由一行，先核对 doing 卡文件域，PROTOCOL §2.1）。

## 并行与资源

依赖 M1-1；与 M1-3/M1-4 并行时先核对上述交集面；PC 真栈串行占用（PC-RUNTIME-STACK.md）。

## 验收标准

- 单测：9 态状态机迁移合法性+旧工程无 queue 视图时的缺省语义（AGENTS §11）。
- 真栈烟测：自由态循环内 agent 读 queue 视图+状态回写往返，exit 0；A4 零命中纪律不破（fs_largeproject_smoke 现成断言面）。
- `go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

黑板持久层接口与 M1-1 合入形态不符（卡面描述过期）→ 上交对齐，不擅改 M1-1 文件域。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

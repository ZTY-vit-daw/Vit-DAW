# Decision：A-F 能力层与 harness 关系裁定+M 线映射（2026-10-04 晚）

用户问询两轮收敛（"harness 更新后还需要固定编排能力层吗/还需要这么细吗"+"A-F 应该挺快，只有时域和自动化是额外开发项"）；ground truth=docs/agent_action_workflow_v1_master_plan.md §6+docs/static_mix_capability_contract_v0.md。

## 裁定 1：能力层与 harness 的关系——层保留，编排退位（三层拆分）

1. **线性门禁/固定顺序：不需要**——文档已自我去编排化（"capability family""不表示门禁""按目标非线性调用"），不回退。
2. **能力契约：保留，性质降格为领域词汇表/目录**——不约束 agent 怎么走，只保证每次调用同名能力的输入/输出/证据/状态形态一致（observed→suggested→pending→applied 枚举=自主 agent 落点规范）。工具箱不规定取用顺序，但工具规格书必须稳定。
3. **工程黑板+决策账本：保留且更重要**——灵活上下文解决单会话容量，不解决跨会话/跨天状态持久化；自由态 settle 报告=黑板数据源（供血关系非替代关系）。自主性越强，外部化状态/证据账本越关键。

## 裁定 2：粒度拆分

族粒度（A-F）保留=用户语言与黑板一级视图（"只做 B3""跳过 D"）；子编号（A1-A5…E1-E4）降为契约目录结构，不绑黑板必填节点粒度；粒度随使用演化（B5 并 B2 先例），不做一次性重划；D/E 母带线契约保留、实现后置。

## 裁定 3：A-F 是 M 线的产品视图，不是第二条大线（用户"挺快"判断采纳+精化）

- 观察侧投影底子已厚（TIM/TOM/COM/masking/RLM/SEG/clip 族命令全链），A-F 对它们=契约接线（编排设计类）。
- **真开发项=M2 时域+M3 自动化**（=master_plan §6.3 已列 automation 写入缺口）。
- 两个精化注脚：①typed route 小缺口清单（声像/宽度/bus-send 结构化读写/采样率管理/export variants）=开发不是编排，单项小随节点补（§6.3"节点需要时补最小"原则）；②黑板/账本/queue/mix.report 实现度未盘点——"编排类剩余量"待盘点定数。
- **并入关系**：A-F 编排类工作（契约接线+黑板投影）并入 M1 harness 更新批次。

## 排程动作

- 立只读盘点卡 MIXLAYER-RECON-1（黑板/账本/queue/mix.report 实现度+A-F 契约接线现状+typed route 缺口核对，0.25 天 flash 级）——盘完 A-F 剩余量全为已知数，M1 内的 A-F 工作拆卡有据。
- v2 排序 M 线不变；M2/M3=C 族节点实现+DAV capability 缺口补齐的产品视图注记已补。

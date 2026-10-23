# UX-AB-AUDITION-RESIDUAL-1：剩余旁路 audition 覆盖（P2–P6 待命卡）

- 发卡：GLM 主管决策流 / 2026-10-23 晚窗二（依据=177 取证 P2–P6 清单，FORENSICS §2.2 锚点表已随卡合入）
- 派发确认：已确认（177 验收裁定=待命卡入库，不占首排）
- 验收负责人：GLM 主管决策流
- 池序 181；目标仓库=D:\Vit_DAW；**P3 待命**（非首排；触发后按 P1 处理）/ 0.5-1 天（触发后）；模型分级：L2 / GLM 执行会话

## 背景（177 取证结论）

177 已修 P1（plugin_grabber bus 两腿=场次⑤实证行使面）。剩余五条旁路均有可挂点、场次⑤未行使：

| # | 路径 | 锚点（FORENSICS §2.2） |
|---|---|---|
| P2 | 单轨 insert 装载（decisions 执行期） | server.go executeDecisions / full_access executeDecisions；挂点=executeDecisions 前 Workflow==pluginGrabberLoadCommand 分支 |
| P3 | sendTopologyConnectWorkflow 直连 | server.go finishSendTopologyConnectWorkflow |
| P4 | C1/B4/C3/C2 运行时装载 | c1_frequency_cleanup_runtime.go:361 / b4_eq_runtime.go:254 / c2_dynamic_plugin_load.go:123 / c3_space_depth_runtime.go |
| P5 | goalrunner 装载面 | goalrunner_chat.go plugin.load_to_rack 族 |
| P6 | agentloop 原始工具直发 | message_loop.go 工具面（与 P1 命令面重叠、入口不同） |

## 触发条件（满足其一即领取）

①任一路径在真实会话/用户手测出现效果类突变零 audition 形态（取证在卷）；②M2 收口后主动批量收口（一次覆盖 P2–P6 防回归钉，挂点模式=P1 同构 semanticTopologyAuditionPlan+bracket 包裹）。

## 验收标准（触发后）

每条路径：单测防回归钉+（有行使形态时）真栈腿 audition 在场；全量 0 FAIL；工件入 coord/runs/UX-AB-AUDITION-RESIDUAL-1/。

## 停止条件

单条路径 before render 时机不可逆且无等价挂点 → 锚点上交（177 已预勘=无此形态，预期不触发）。

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（覆盖清单 / commit / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

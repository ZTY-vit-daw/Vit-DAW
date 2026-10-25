# M3-ENVELOPE-LANE-RECON-1：包络车道 UI+机架状态面实锚勘察（M3 自动化批次③前置）

- 发卡：GLM 主管决策流 / 2026-10-25 晚窗（依据=decisions/2026-10-04-phase-ordering-v2.md §M3 步骤 3「包络车道 UI（时间线主场）+机架状态面（徽章/珠/播放头联动）」+2026-10-24 gate §5 ④+decisions/2026-10-24-pc-mixing-mac-generation.md PC=混音线）
- 派发确认：已确认（gate 2026-10-24 §5 首排④列明决策侧开勘察卡）
- 验收负责人：GLM 主管决策流
- 池序 186；勘察对象=**前端仓 D:\Godot\project\vit-daw-frontend**（主检出只读）；报告落主仓 coord/runs/M3-ENVELOPE-LANE-RECON-1/
- 归属：PC 混音线 M3 批次步骤 3 前置；P2 / 0.5 天 / 依赖=M3-KERNEL-1 合入（**已满足**：automation.get_curve/set_points/clear_region 已在主仓 main CommandDispatcher.cpp:3131-3136 亲核）
- 模型分级：L1 / 闲时任务或 flash（纯只读勘察，零代码零真栈零探针；与 180/185 在飞可并行——零写面天然互斥）
- 设计输入：[runs/M3-RECON-1/RECON.md](../../runs/M3-RECON-1/RECON.md)（引擎侧锚点权威：AutomationCurve 点三要素 t·v·c/paramID 绑定/TimeBase time|beats）+ [M3-KERNEL-1 done 卡](../done/2026-10-12-M3-KERNEL-1-automation-commands.md)（六裁定：time 秒主形态/base_revision=plugin state 级/宏与曲线道不暴露/曲线回流=DAD 扩型不做新 peer/披露面解锁随步骤 4）

## 勘察五问

1. **时间线主场结构实锚**：`app/timeline/` 车道体系——轨道行渲染归属文件、车道类型的枚举/工厂挂载点（新增「包络车道」的最少侵入面）、播放头↔视图既有同步链、车道高度/布局管理、`TimelineInputArbitrator` 对车道内点选/拖动的既有分派形态。逐条给文件+行号锚点。
2. **数据通道实锚**：自动化曲线数据到前端的最短路径——前端既有 VitIpcClient→4445→agent 通道族中哪条能承载曲线读数（既有 repository/拓扑回执面可复用 vs 需新请求面，两案并列）；paramID↔轨道/rack 成员映射的既有面（176/183/185 消费的拓扑回执同源可复用性）；推送 vs 轮询形态建议（给倾向不越权定案）。
3. **机架状态面实锚**：`app/rack/Vit_Graph_Rack.gd` 对卡呈现邻域——「参数有包络」的徽章/珠挂载点（185 FX 湿链徽标同位竞争面）+播放头时间→卡面联动既有钩子（若有）；无则登记为新建面。
4. **写回链实锚**：前端编辑包络→`automation.set_points` 的调用路径——前端触发 agent 工具的既有模式（183 track.send.set_amount drag_ended 恰一笔先例）可移植性；拖动中零发包+释放一笔+回执回显收敛（183 vit_fader_dragging 门先例）移植面；base_revision 取用面（plugin state 读回既有面）。
5. **实施卡拆分建议**：按 1-4 锚点给步骤 3 实施卡的拆分建议（车道 UI/数据通道/机架徽章/写回链的串并行与文件域划分），供主管裁定，不自行定案。

## 文件域

零代码；报告落主仓 coord/runs/M3-ENVELOPE-LANE-RECON-1/（RECON.md+必要 grep 附录）。

## 验收标准

五问齐+逐条亲核锚点（文件:行号）；第 2/5 问给建议不越权定案；附实施卡文件域冲突评估（185 同文件 Vit_Graph_Rack.gd 时序单列；180/185 在飞串行链不断插）。

## 停止条件

时间线主场结构与既有卡面先例大面积不符（TimelineEditStore/输入仲裁已重构）→ 锚点上交再裁。

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（报告路径 / 五问结论摘要 / 实施卡拆分建议）
- 验收：（裁定文件 / 验收 commit）

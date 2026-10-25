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

- 领取：2026-10-25 晚窗 / 主仓 origin/main=e6456b6d（前端仓勘察基线 main=5da6360f 只读零改动）/ owner=GLM-5.3-flash 勘察会话 @ PC（D:\Vit_DAW_wt_m3envrec1）/ 分支 port/m3-envelope-recon-1 / worktree D:/Vit_DAW_wt_m3envrec1 / 领取提交=本提交（报告+卡面领取/回执一并，勘察卡零代码无前置实现提交）
- 回执：[runs/M3-ENVELOPE-LANE-RECON-1/RECON.md](../../runs/M3-ENVELOPE-LANE-RECON-1/RECON.md)（五问逐条锚点+grep 附录）/ 五问结论摘要：①主场=虚拟行容器（drop_router :657-688）→track_row.tscn→Track_Row_Controller→TrackLane2D，车道工厂单点 `_ensure_lane_2d_layer`（Track_Row_Controller.gd:1033-1063）+双层视图枚举（lane :11-12/行 :28-29），仲裁器 hit-kind 词表（TimelineViewMapping.gd:116-228）+match 分派（TimelineInputArbitrator.gd:503-550）+4px 手势状态机可复用；②通道=legacy UDP 4445 面零新请求面可承载（get_curve 已入只读白名单；建议案 A 直连+按需拉取，案 B VSP 信封 CAS 并列，推送非必要——delta_update 覆盖 plugin state 未证）；③rack 徽章挂点=Vit_Graph_Rack.gd:2034-2036 徽标 chip 邻域+:2331-2350 工厂，**播放头联动 app/rack/ 零既有钩子（登记新建面）**，「参数有包络」在场性 get_plugin_parameters 回执缺字段（标注点②）；④183 三段式（Vit_Graph_Rack.gd:3609-3619 拖动零发包/:3650-3659 恰一笔/:1993-1997 回显门）可整体移植，语义差=set_points 区域替换需前端持工作副本；**标注点①：已合入 set_points 无 base_revision 参数，现役 CAS=VSP 信封级（VspKernelReference.cpp:2507-2530），游标=project_store.gd vsp_revision**；⑤建议 4 卡（C1 数据通道新文件/C2 车道 UI/C3 机架徽章/C4 写回闭环）——C1/C2/C4 与 180/185 文件域不相交，**唯 C3 与 185 同文件 Vit_Graph_Rack.gd 时序单列（185 合入后领取）**；停止条件未触发（TimelineEditStore/仲裁器/183 先例与卡面假设一致）
- 验收：（裁定文件 / 验收 commit）

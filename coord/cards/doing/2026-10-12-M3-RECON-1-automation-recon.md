# M3-RECON-1：tracktion automation 层级/快照能力/VSP 暴露形态实锚勘察（M3 自动化批次①）

- 发卡：GLM 主管决策侧 / 2026-10-12 早窗（依据=decisions/2026-10-04-phase-ordering-v2.md §M3 步骤 1「前置实锚勘察」；MIX-CAP-RECON §2「自动化曲线面不存在（引擎原生未暴露，agent 观测恒 0）」；用户 2026-10-10 勘误口径=自动化属 C4 段落批次）
- 派发确认：已确认（用户 2026-10-12 早窗 /morning 勾选「M3 自动化拆卡」里程碑）
- 验收负责人：GLM 主管决策流
- 池序 93；目标仓库=D:\Vit_DAW
- 归属：PC 混音线 M3 批次（M2-TIMEFX 在飞，本卡非当日阻塞）
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无依赖
- 模型分级：L1 / 闲时任务或 flash（纯只读勘察，零代码零真栈零探针）
- 设计输入：runs/FREESTATE-REGION-GOAL-1/DESIGN.md（锚点地图 D1-D7+范围 goal 协议）+ runs/FREESTATE-REGION-IMPL-1-STOP/（单轮形态可复用面：range_goal 字段/武装/测试骨架，两轮逻辑届时重写）+ runs/MIX-CAP-RECON-1/RECON.md §2（底盘现状）

## 勘察四问

1. **tracktion automation 层级实锚**：内核所携 tracktion 引擎的 automation 数据模型——参数/轨/宏各层级存储形态（编辑工程 XML 面）、修改 API、快照能力（AutomationCurve/快照节点取用路径）在不在；给出 C++ 符号+文件+行号锚点（VitApp 源与 include 面全查）。
2. **VitApp 侧现状复核**：内核是否已有 automation 读写/应用路径（grep automation/envelope/curve 全仓）；复核 MIX-CAP-RECON「引擎原生未暴露」结论；给出原生能力到 VitApp 服务层的最短暴露路径（可挂载面：CommandDispatcher/ProcessorService/插件参数面，逐个评估）。
3. **VSP 暴露形态建议**：automation 若暴露走什么通道——既有 set_plugin_param（readback+base_revision）可否承载连续写，还是需新命令族（automation.set_points/get_curve/clear 之类）；白名单/CAS/幂等面评估（对照 vsphub 幂等协调既有形态）。**给「建议命令面清单」供主管裁定，不自行定案。**
4. **agent 侧消费面**：观察面（FXM 差分采集缺生产者，automation 曲线若回流→落 DAD 证据层还是新 peer，给两案文件域）、意图层（包络指令词表现状）、goal 编舞（IMPL-1-STOP 保全实现可复用面清点）三处现状+接入点锚点。

## 文件域

零代码；报告落 coord/runs/M3-RECON-1/（RECON.md+必要 grep 附录）。

## 验收标准

四问齐+逐条亲核锚点（文件:行号）；第 3 问建议清单不越权定案；第 4 问附文件域冲突评估（M2-TIMEFX 在飞：域表/写回链冲突面单列）。

## 停止条件

tracktion automation 源不在仓内且无头文件可考 → 上交依赖清单，不硬凑。

## M3 批次骨架登记（拆卡续作依据，步骤 2-5 待本卡回执后拆）

2=内核 automation 写入命令面（依赖问 3 裁定）；3=包络车道 UI（时间线主场）+机架状态面（徽章/珠/播放头联动）；4=agent 范围 goal 编舞单轮终态（原 FREESTATE IMPL-1/2 并入，DESIGN 保留为输入）+意图层包络指令；5=后段快照统摄层（节点+过渡；先曲线后骨架再统摄）。

- 领取：2026-10-12 早窗（/morning 后同会话代跑） / origin/main=f9a0d7cd057568784c354f574a4ca08bb370b96d / owner=闲时任务·主管会话代跑（GLM-5.3 主会话 sess_cf9df7ab） / 分支=main·coord-only 直推 / worktree=主工作树 D:\Vit_DAW（写面仅本卡+coord/runs/M3-RECON-1/）。并行流在场（M2-TIMEFX-1 worktree D:\Vit_DAW_wt_m2_timefx1@coord/close-m2-timefx1、LOUDNESS worktree D:\Vit_DAW_wt_loudness_watch_1@91ce26fd）——本卡零代码零触碰，写面互斥天然成立。注：闲时任务 offpeak-69cc8895（队列位 186）由本会话直接代跑，完成即核销，勿重复执行。
- 回执：（报告路径 / 四问结论摘要 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

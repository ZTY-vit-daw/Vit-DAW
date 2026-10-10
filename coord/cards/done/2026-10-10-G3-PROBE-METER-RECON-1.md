# G3-PROBE-METER-RECON-1：probe 物理成本计量源勘察（只读，G3 重开条件①输入）

- 发卡：GLM 主管决策侧（/morning 会话）/ 2026-10-10
- 派发确认：已确认（用户 2026-10-10 /morning 裁定标准日+机会面清理（gate 建议序④））
- 验收负责人：GLM 主管决策流
- 池序 42（来源=[G3 终裁](../../rulings/2026-10-09-G3-FINAL-ruling.md) 重开条件①+§遗留④"probe 物理成本计量源（归因卡①如实披露项）——机会卡"；G3-ATTRIB-1 报告披露=pull 账本 probe 成本项 unknown 如实）；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 1-2h / 无；**只读勘察，零代码**
- **晚窗参考序 2**（2026-10-10 早窗 /morning 标注：首选闲时车道——本会话闲时位被 BOUNDARY-PERSIST-1 占用，空出后转排；或任意引擎/任意窗口领取）
- 模型分级：L0 / **任意引擎（含闲时）**

## 背景与问题

- G3 终裁维持 push 生产缺省，**唯一实质差距=轮级调用 17v6**；重开条件①="probe 计量源接入后真实成本对比"。当前 pull 账本中 probe 物理成本记 unknown（计量源缺失），成本对比的物理层证据面是空的。
- 需要回答：**存不存在可接入的 probe 物理成本计量源**（渲染时长/CPU/IO 等物理量，而非 LLM token 口径）。

## 目标（报告三节，只读）

1. **unknown 构成清单**：pull 账本（G3-ATTRIB-1 run 20261009_214138_harnessab 工件+相关遥测字段定义处）probe 成本项 unknown 的逐项构成——哪些记账位、各自为何 unknown。
2. **可计量源盘点**：内核侧（VitApp 遥测/VSP 回执字段）、agent 侧（harness/probeaudio/FXM 面的既有时钟或资源事件）是否已有 probe 物理成本观测量——逐项给锚点（文件:行）与字段形态；区分"已有但未接线"与"根本没有"。
3. **重开条件①边界结论**：若有源 → 列最小接入面清单（供后续立卡）；若无源 → 如实登记物理边界（成本对比只能停留在 LLM 口径，G3 重开条件①的可行域收窄），不臆造方案。

## 文件域

零代码改动；工件目录 [coord/runs/G3-PROBE-METER-RECON-1/](../../runs/G3-PROBE-METER-RECON-1/)（report.md+可复跑 grep/读取命令附录，M8 报告附录风格）。

## 验收标准

报告三节齐+每项结论有锚点或 grep 实证可回指；"无源"结论必须附全库盘点口径（搜了什么、命中什么）。

## 停止条件

勘察发现成本口径设计争议（如"物理成本"定义本身存疑）→ 如实记录两口径上交，不选边。

## 并行与资源

只读零占用；与全部在池卡并行安全。

- 领取：2026-10-10 闲时车道 / origin/main f0dc8e77 / owner=闲时任务·GLM-5.3-Flash（ZCode 主管会话 sess_1c9ec412 派发）·PC / 只读无分支（零代码勘察卡）/ 领取提交=4b339b7b+86fc935f
- 回执：[coord/runs/G3-PROBE-METER-RECON-1/report.md](../../runs/G3-PROBE-METER-RECON-1/report.md)——**结论=有源，最小接入面成立**：①unknown 构成=账本位齐备（pullLedger.probeSpent/probeCostKnown+budget.go 账户/执法/披露面）唯缺逐笔成本生产者（execRecord 九字段无计量键、InvokeResponse 无耗时字段）；**接线点已预声明**（pull_session.go:411-430 settleBatch 注释"有真实计量源后在此接入"）。②可计量源盘点：agent 侧三源现成（Harness.Invoke 墙钟 harness.go:879-892 / message_loop.tool 墙钟 message_loop.go:66 均日志形态；**CollectL2RenderProbeBatch.ElapsedMS l2_probe_batch.go:38 结构化 probe 级测量但组合层丢弃**）+内核侧两处日志形态计时（L3AcousticAnalyzer:952-961/WaveformEnvelopeBaker:672-693）、VSP 回执结构化时长面=零（反向确认 grep 在案）。③边界结论：三层口径递进无争议（(a) agent 工具墙钟充分层≈2 文件改动可支撑重开条件①对比/(b) probe 级结构化精化层/(c) 内核渲染遥测扩展层非必要）——不触发停止条件；诚实边界=层(a)含调度噪音，未来纯内核口径走层(c)勿冒称。
- 验收：（待主管终审——本卡执行侧自验完成，报告即验收材料）

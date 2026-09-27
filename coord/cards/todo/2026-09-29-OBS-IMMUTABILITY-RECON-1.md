# OBS-IMMUTABILITY-RECON-1：观察产物共享引用穿透勘察（架构级缺陷的影响面与修复评估）

- 优先级 / 预估 / 依赖：P1 / 0.4 天 / MAT-D3 取证：compactFeatureSnapshot→compactFeatureRow 嵌套值浅引用（mixboard.go:2764/2892），finalize 后遥测 ingest 原地更新穿透到 obs.GlobalSummary["feature_snapshot"]（三角重放实证 replay≠obs）
- 模型分级：L1 / flash 可接（纯只读勘察+报告）
- **执行侧（PC 会话）**
- 目标：
  1. **穿透路径全景**：ObservationPacket 产物从 finalize 到消费的全生命周期——哪些持有共享引用（GlobalSummary/feature snapshot 嵌套行/MixPackage 派生视图）、哪些链路会在 finalize 后原地写入这些引用（遥测 ingest 四分支/观察链内部后续步骤/其他——逐一锚点+时序）；MAT-D3 候选机制的"遥测穿透 vs strip 触发"未定论部分定案。
  2. **受影响消费方清单**：读观察产物的所有面（obs JSON 落盘重读/mix_read/ContextPack/CCB/事件流投递/journal——落盘时机在穿透前后决定文件内容是否被污染；逐面判定"读到的是 finalize 态还是穿透后态"）。
  3. **修复方案评估**：深拷贝（每轮观察 compact 时全深拷 vs 嵌套行写时复制 vs 遥测 ingest 改写前克隆）——各方案的改动点+性能量级评估（观察轮频率参照 MAT-0 数据）+行为影响（是否有消费方**依赖**穿透后的新值=意外行为耦合）。
  4. 报告落 `coord/runs/OBS-IMMUTABILITY-RECON-1/`。
- 约束：零代码改动；锚点带文件:行；性能评估给量级与方法不编造精确数。
- 验收：①穿透路径全景（含未定论定案）②消费方清单带"读到哪个态"判定 ③三方案评估 ④报告入库
- 停止条件：常规勘察止损

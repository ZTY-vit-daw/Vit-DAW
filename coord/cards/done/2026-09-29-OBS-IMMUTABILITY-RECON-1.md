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
- 领取：2026-09-27 PC 执行侧（与 MAT-D4 同批勘察；报告未走独立 port 分支，随验收提交直入 main）
- 回执（2026-09-28 mac 决策会话补记）：报告 `coord/runs/OBS-IMMUTABILITY-RECON-1/report.md`（22.4KB，随 d91ba51 入 main）；穿透路径全景（compactFeatureSnapshot→compactFeatureRow 浅引用 mixboard.go:2764/2892+遥测四分支+尾挂 State B）+消费方态判定+两态分叉 W1/W2 定位（persistence_v2.go:13-75）+三修复方案（R1 完整态证据回溯/R2/R3）+关键锚点索引表；结论：本卡不推进 G2-D 闭环，R1 为推荐修复方向。
- 验收：**pass（2026-09-27 PC 决策会话 @ d91ba51；2026-09-28 mac 决策会话补记归档）**——"OBS 五条取证逐条吻合为定案功臣"（State A/B 两态机制全案定案的实证基础）；R1 修复方向已兑现为 MAT-E0（执行+验收 pass，2026-09-29 链路闭环）。本条为簿记补齐：原验收提交漏挪本卡（todo 残留），卡面状态机于本提交闭合。

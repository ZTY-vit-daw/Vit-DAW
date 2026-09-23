# FIX-PCA-AUTOSWEEP-1：全库批量认证扫描——一次扫描分类+认证+派生全部可用效果器

- 优先级 / 预估 / 依赖：P1 / 1-1.5 天 / PORT-PCA-FULL-CANDIDATES-1 的构建器入仓（本卡复用其派生层）；**用户裁定 2026-09-23"应设计一套认证就能过滤清楚所有能用效果器的机制"**
- 模型分级：L2 / GLM-5.3（分类器规则=族轴拓扑定义，核心设计面）
- **目标：把"哪个插件可用"从人工逐个认证变成机器可回答**——一条命令扫全库：分类 → 批量确定性认证 → 白名单自动派生 → 例外如实报告。
- **背景**：现管线三层割裂——扫描层全自动（语义索引全量在库：PC 719 Waves 主体+PA/FabFilter）、认证层手工逐主体（PCA job：PC 24/mac 27，冰山一角）、派生层刚自动化（FULL-CANDIDATES 构建器）。瓶颈=认证层的手工性；EQ-1 已证明**零 LLM 确定性认证通道**可行（phase3_live_smoke 收据+pcactl import，Q10 即此路径认证）。
- **设计冻结**：
  1. **族分类器（确定性，零 LLM，零名称猜测）**：输入=probe 参数面拓扑+厂商分类；规则=每族实验轴的结构特征——static_eq：≥3 组（频率锚,增益参数）对；broadband：threshold(+ratio/makeup) 单频段；limiter：ceiling 参数存在且无多频段阈值；gate_expander：threshold+range+展开向参数；de_esser：threshold+sibilance 频段结构；transient_shaper：attack+独立 sustain 参数；multiband：≥3 组频段阈值/分频点。一主体可属多族（如实多列）；不匹配任何族轴=非实验类效果器（混响/延迟/仪表等）如实排除。**分类只用结构性参数拓扑，不用厂商命名**（名称仅作 ambiguity 提示不作判据）。
  2. **批量认证扫（零 LLM，复用 EQ-1 确定性通道）**：对分类命中主体逐个跑——identifier 实例化（PLUGIDENT 纪律）→ 该族轴覆盖写（upsert/modify/disable/undo）→ readback → 快照比对 → phase3_live_smoke 式收据 → pass 自动 pcactl import 晋升；fail/歧义进**例外队列**（证据+原因，不认证不硬凑）。幂等可重跑；批次 RUN_LEDGER。
  3. **派生层**：现有构建器（FULL-CANDIDATES 规则冻结版：promoted ∧ 族轴覆盖 ∧ 四源参数锚点）消费扩大后的晋升集 → v6 白名单全候选。
  4. **产物**：`sweep_report` = 全库 → 分类结果/认证通过/例外 三桶清单 + 各族候选计数表——"Waves 包各族到底有多少能用"从此是跑一次就有准数的问题。
- 文件域：`scripts/`（扫描驱动+分类器+报告器，与构建器同域）+ 机器本地工件；零 agent 代码改动预期（认证/装载/白名单机制全复用）
- 验收：①PC 全库扫（719+ 主体）：sweep_report 三桶齐全+各族计数表；②抽样 ≥3 主体覆盖收据核验（含 ≥1 个 Waves 壳内主体走 identifier 实例化）；③派生白名单 overlay 全族回归；④≥1 个新增多候选族真栈自选腿五环证据链；⑤例外队列无静默丢弃（每条有因）；⑥零手编零 LLM 全程（收据可证）
- 停止条件：某族轴分类规则需厂商命名判据才能达可用精度 → 上交裁定（content-blind 边界问题）；认证扫需 LLM 参与才能过覆盖 → 上交（违反零 LLM 冻结）
- 领取：
- 回执：
- 验收：

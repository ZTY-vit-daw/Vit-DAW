# 设计①：触发机制（主案=处理后触发）

LOUDNESS-WATCH-DESIGN-1 设计件 1/6。编译器类比定位：处理=写代码，响度检查=编译/lint，越界=告警。
本件回答"什么时候跑检查"。零生产代码；锚点均为领取基线（origin/main 8e66e674）实测。

## 1. "处理后"的语义定义

用户原意：**工程音频被 agent 或用户处理后**，后台校验响度未越设定范围。"处理后"在现有架构上有三个可锚定的完成事实（三类来源各对应一个已有钩子，不新建内核依赖）：

| # | 处理来源 | 完成事实 | 已有钩子锚点 |
|---|---|---|---|
| T1 | agent 自由态处理（含 settle 裁决） | settle 结论落盘 | `agent/internal/chat/free_state_conclusion_bridge.go:65` `recordFreeStateSettlement`——六漏斗（settle/stop/judgment×3/adoption/park-continuation/closure，kind 枚举见 `agent/internal/mixboard/free_state_conclusion_store.go:33-35`）唯一持久化入口，`:75` 调 `mixboard.Store.RecordFreeStateConclusion` |
| T2 | agent mix 动作（gain/EQ/限幅等） | 动作后观察流完成且 loudness_summary 新鲜 | `agent/internal/harness/harness.go:4279` `requestMixObservationFeaturesBackground`（后台特征请求已存在）；观察就位门 `harness.go:3796` `ensureMixObservationReadyGate`（同步面，默认只对 `mom.IntentActionPreflightObservation` 生效，`harness.go:3844-3853`） |
| T3 | 用户在 Godot 前端手动处理 | 内核项目变更到达 agent | `agent/internal/chat/free_state_reasoning_loop.go:83` `loop.LatestProjectChange`（携带 project_revision/project_uuid/project_path；消费先例 `chat/audition_events.go:604-609`） |

## 2. 主案接线：三个钩子点

### 2.1 主钩子 = T1 settle 完成钩子

- **接线点**：`recordFreeStateSettlement` 内，`RecordFreeStateConclusion` 成功返回后（`free_state_conclusion_bridge.go:75` 之后）触发响度 watch 检查。
- **理由**：settle 是自由态循环已权威确认"一段处理完成"的时刻；M1-1 桥已在六漏斗全部收口，单点接线覆盖面最大。桥自身携带 ProjectUUID/CheckpointRef/ProjectRevision（`:25-45`），watch 的 dedupe 键与工程身份直接可得。
- **纪律**：桥现有语义是"持久化失败只 WARN 不回滚 settle"（`:66-68` 注释）。watch 钩子同纪律——**watch 失败只记日志，绝不影响 settle 本身与回执**；watch 是观察面寄生，不是流程关卡。

### 2.2 次钩子 = T2 动作观察流搭车（piggyback）

- **接线点**：`requestMixObservationFeaturesBackground`（`harness.go:4279`）返回的特征快照中，当 loudness_summary 行新鲜（freshness 判据复用 acousticpackage status，`agent/internal/acousticpackage/status.go:350,378` 的 loudness_summary 特征行）时，对受影响 clip 跑比对。
- **理由**：特征 bake 已经发生，watch 比对是**纯函数消费已有行，零额外内核成本**。不接 `ensureMixObservationReadyGate` 同步路径（`harness.go:3796`）——那是"观察不齐阻塞 LLM 回复"的关卡，watch 挂上去会把告警面变成阻塞面，违反后台定位。
- **范围**：只比对本次观察覆盖的 clip（特征 bake 是 clip 域的，见设计④ §3 缺口 5），不虚构母线级结论。

### 2.3 辅钩子 = T3 项目变更钩子

- **接线点**：`loop.LatestProjectChange` 更新处（自由态循环上下文装配，`free_state_reasoning_loop.go:83` 与 `:362-363` overlay 复制处）。
- **诚实边界**：LatestProjectChange 的更新时机受循环节奏约束——用户纯手动编辑（无 agent 会话活动）不保证即时触发。v1 如实声明：**用户处理的即时检查依赖 agent 循环观察到位，或用户主动调 watch status**；即时性缺口由周期备选（§3）兜底，内核侧推送钩子列为远期候选不进本轮实施卡。

### 2.4 触发后流程（三钩子共用）

```
钩子命中 → 读配置（设计②；关=静默退出，零成本）
        → 取测量行（新鲜复用；陈旧且需检查时才发 bake——T2 搭车路径永不发新 bake）
        → 纯函数 compare(measurements, config) → verdict（三态：pass / breach / unmeasurable）
        → dedupe：同 project_revision 同 verdict 形态只发一次（键=project_uuid+project_revision+verdict_kind）
        → 告警双面分发（设计③）
```

- verdict 记录 schema：`loudness_watch.verdict.v0`（字段：schema_version/project_uuid/project_revision/trigger(kind+ref)/measurement_grade/measured(逐 clip：clip_id/integrated_lufs/peak_dbfs/approximate 标注)/config_digest/verdict/breach_details(逐条：metric/measured/threshold/delta_db)/checked_at/unmeasurable_reason(可选)）。
- **unmeasurable 是一等公民态**：特征行缺失/陈旧不可用/口径不支持（如 realtime 模式无生产者）时如实返回原因，**不产 pass 也不产 breach**（证据等级纪律，AGENTS §7）。

## 3. 备选触发三态

| 态 | v1 判定 | 依据 |
|---|---|---|
| 处理后触发 | **主案**，默认 | 三钩子全部有既有锚点；增量成本最小（见 06 性能件） |
| 周期触发 | 可配选项，默认关 | 接线点复用 pull 后台探针/背景观察节奏（`harness.go:4279` 同族后台 fill）；interval 默认 300s、下限 60s（配置校验拒绝更小值）；周期模式运行在 pull 会话内时其 bake 成本按 probe 分级入 probeSpent 账本（`agentloop/pull_session.go:440-463`） |
| 实时监听 | v1 不可达，配置可存、激活报 unavailable | 内核无实时表头生产者：L3AcousticAnalyzer/AudioFeatureService 全部是离线 bake 请求-应答面（`VitApp/Source/Service/AudioFeatureService.cpp:158` requestBake；发布面 `:128` audio_feature_bake_status），无流式母线计量。内核补齐面列设计④ §4，实时态在配置校验时接受枚举值但状态面报 `status=unavailable reason=no_realtime_producer`，不产 verdict 噪音 |

用户原话"实时若影响性能则退后两者"的落地：实时态不是性能问题而是**生产能力缺失**问题，比性能理由更强——如实按能力缺口处理，不做伪实时（轮询 bake 冒充实时）。

## 4. 非目标与红线

- watch **不是编译器硬门**：类比是 lint/告警，不是阻断构建——任何触发点都不阻塞 settle、动作回执或用户操作（对照：ready-gate 才是阻塞式观察关卡，watch 明确不挂那里）。
- watch **不写工程状态**、不改 mix 决策、不自动提修复提案——修复提案只由模型/用户消费告警后经既有 mix 控制面（PCA 准入）走正常路径。
- 同一触发点单次检查内 bake 发起次数有上限（实现卡 B 定为 ≤4 clip/次触发，超限记 unmeasurable_reason=scope_capped）——防止大工程 settle 时 bake 风暴。

# 投影物化与增量失效设计（MATERIALIZATION_V1_DESIGN，L1-2 / D4）

> **状态：规划（未实现）**。本文档是路线图 [AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md](AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md) L1-2 段 / D4 组件（投影物化 + 增量失效，make 模型）的设计，卡 `coord/cards/todo/2026-09-28-L1-2-DESIGN-1.md`；G2 质量门（失效正确性专项）的被测对象即本设计。零代码改动；全部接口为设计态签名，落地按 §8 排程拆卡。
>
> 规格锚：G1 裁定 [coord/decisions/2026-09-27-g1-ref-schema-ruling.md](../coord/decisions/2026-09-27-g1-ref-schema-ruling.md)（ref=物化主键；L1 结构化键=ObservationKey 扩段，排 L1-2 开工时——本设计 §6.3 承接）。读口契约锚：[QUERY_ENGINE_V1_DESIGN.md](QUERY_ENGINE_V1_DESIGN.md) §5（MaterializedStore 三函数，本设计 §6 细化实现侧）。事实基线：[coord/runs/L1-2-RECON-1/PROJECTION_PIPELINE_INVENTORY.md](../coord/runs/L1-2-RECON-1/PROJECTION_PIPELINE_INVENTORY.md)（零事件驱动现状 / 17 依赖边 / 19 事件目录 / 10 失效种子——下文简称 RECON，按 §/M/E/# 编号引用）。拓扑权威：[OBSERVATION_PROJECTION_MANIFEST.md](OBSERVATION_PROJECTION_MANIFEST.md)（九投影 peer，非链）。
>
> 一句话定位：物化层是观察域的 **make**——事件**急切标脏**（不漏标是生命线）、重算**按需触发**（读时 lazy 为主），产出以 L0 ref 为主键的行集与增量变更流；查询引擎（L1-3）与 CCB 是它的两个只读消费者。

---

## 0. 设计输入与既有事实（不自创现状）

事实全部来自 RECON 报告与本次亲验锚点；本设计只在其上做裁决。

| # | 事实 | 出处（亲验） |
|---|---|---|
| F1 | 九投影全部请求驱动惰性现算，无事件驱动重算；内核事件只刷输入缓存（feature snapshot / shadow 镜像） | RECON §1.1/§1.3；`agent/internal/bridge/bridge.go:400-430` → `harness.go:3890-3906` |
| F2 | 投影计算单点收口：`FinalizeObservationContext`（dom/mom/tim/fxm/com 顺序编排+digest/catalog） | `agent/internal/mixboard/catalog.go:58-84`（亲验）；仅两个调用方（`RequestObservation` `mixboard.go:1089` / `BuildObservation` :1290） |
| F3 | 脏传播雏形已存在：shadow `addChangeScopes` 把参数路径映射到观察域（gain/level/mute/solo→track.level+mix.\*+headroom 等），free-state 台账消费 `affected_scopes` 把 view 行打 `stale+invalidated_by_change_id` | `shadow/change.go:200-222`（亲验）；`chat/free_state_reasoning_loop.go:1725-1759`（亲验）= RECON M2/M3 |
| F4 | 物化种子现存：evidence CAS（`PutEvidence` 内容寻址 sha256 + pin）、acousticpackage Store（落盘+原子替换+同 target 异 revision 交叉失效）、F5 铃行级 freshness 三分（current/material_reuse/stale）、观察票落盘 | `projectstore/evidence.go:49-75`（亲验）；RECON M1/M4/M7 |
| F5 | L0 ref 文法已定版有实现：`vit://kind/scope/window@snapshot#hash`，window/hash 双段强制；注册 kind 现仅 {dom,fxm,com,rlm}，扩展卡（REFSCHEMA-L0-2）已入池待做 | `agentprotocol/refschema.go`（main c7bbcc0b，亲验）；G1 ruling |
| F6 | 读口契约已冻结（查询引擎只读物化层，失效与新鲜度归物化层管）：`SnapshotView`/`Resolve`/`Subscribe` 三函数 | QUERY_ENGINE §5 |
| F7 | 事件目录 19 项：真候选 14（#1-#14 携带工程/render 变更语义），#15-#19 为 UI 分发面可排除；`l2_render_probe_ready`（#9）走临时 SUB 不经统一入口（现存分叉点） | RECON §3 |
| F8 | 两处"漏标=静默错误数据"活样本：E8（rlm `mergeSourceRows` 字段级覆盖合并无 revision 比较，`rlm/projection.go:170-192`）、E16（B1 pack 同 run 建后不失效，`static_mix_gain_staging_context.go:21-23`） | RECON §2/§5.3——G2 红测素材 |
| F9 | FXM/COM paired 输入随请求携带（测量/双 tap 工件），不是工程状态——不可事件驱动预计算 | RECON E5/E7；`mixboard.go:3972-3978` |
| F10 | 禁丢弃工作树、测试不写源码树、真实栈烟测 exit 0 门槛 | AGENTS §5/§12 |

**宪法（继承裁定）**：D2 统一寻址不统一存储引擎；D5 冷热分离与成本分级；投影 peer 非链；缺失/partial/stale 原样保留不升级 readiness。

---

## 1. 总体形态：一库两流三读端

```
 内核事件（RECON §3，真候选 14 项）
   │  ZMQ SUB / VSP 应答（既有通路，不新增订阅）
   ▼
 ┌────────────────────────────────────────────────────────────┐
 │ 事件接入（§3.1）：shadow ChangeReceipt + 特征到达事件        │
 │  invalidate 型（状态变更）│ arrive 型（证据到达）            │
 └──────────────┬─────────────────────────────────────────────┘
                ▼
 ┌────────────────────────────────────────────────────────────┐
 │ 依赖图与脏传播（§4）：scope 域 → kind → 行集（ref 坐标匹配）  │
 │  急切标脏（eager invalidate）——不漏标是生命线                │
 └──────────────┬─────────────────────────────────────────────┘
                ▼
 ┌────────────────────────────────────────────────────────────┐
 │ 物化库（§2）：内存代际索引（权威读路径，copy-on-write）        │
 │  + evidence CAS 内容层 + manifest 落盘（崩溃重建）            │
 │  重算 lazy：读端触发 / observe 回填；整投影重算+行级 diff      │
 └──┬──────────────────┬───────────────────┬──────────────────┘
    │ SnapshotView     │ Resolve           │ Subscribe（增量流）
    ▼                  ▼                   ▼
 查询引擎中央索引    Expand/diff 句柄      台账/审计/未来 memory
 （L1-3，已冻结）   （L1-3）             （M2 台账可升级为消费者）
```

三条边界纪律（与 QUERY_ENGINE §1 对偶）：

1. **物化层不订阅第二份 ZMQ**——事件源是 shadow ChangeReceipt（已按序）与 `IngestKernelTelemetry` 既有入口的特征事件；单一事实源，避免双订阅乱序（§3.1）。
2. **物化层不装配披露、不做预算**——LLMContext/裁剪/审计是 CCB 职责；物化层只管内容与新鲜度（QUERY_ENGINE §5 原文）。
3. **物化层不猜测输入语义**——行状态判定只用写侧标注与依赖图命中（F5 铃的"读侧零推断"原则，M1），消费时校验（M6 revision 门）归读端既有逻辑，物化层不改写。

---

## 2. 物化存储设计（验收①：形态裁决带依据）

### 2.1 裁决 R1：三层存储——内存代际索引（权威读路径）+ evidence CAS（内容层）+ manifest（可重建清单）

**裁决**：不引入新存储引擎（否决 SQLite/独立数据库方案），采用三层既有设施组合：

| 层 | 形态 | 复用对象 | 依据 |
|---|---|---|---|
| 内存索引 | `materialize.Store` 路径级进程内单例，代际（generation）整表 copy-on-write 原子交换 | M4 模式（`acousticpackage/status.go:106-251` 单例+原子替换）；QUERY_ENGINE §3.4 已为中央索引裁定同构形态 | v1 规模 ≤10⁵ ref 行（QUERY_ENGINE §6.1 规模锚），整表交换微秒级、读侧无锁；查询引擎不自带存储，物化层也不需要 |
| 内容层 | 重算产物 `projectstore.PutEvidence`（kind=`materialized.<kind>`），`evidence://<sha256>` 句柄 + pin 不可逐 | M7（`projectstore/evidence.go:49-75`，亲验：sha256 内容寻址天然去重） | ref hash 段（`#sha256:16hex`）与 CAS 句柄同源——物化行内容身份即存储身份，零翻译；观察票已走此路径（`persistence_v2.go:13-75`），单写者纪律沿用 |
| 清单 | `.vit_agent/<uuid>/materialization/manifest.json`：generation、行坐标→hash→CAS 句柄、kind 注册状态；tmp+rename 原子写 | `harness.go:7155-7174` 原子写模式（F5 铃同款） | 崩溃恢复：内存索引可从 manifest+CAS 句柄全量重建；不持久化 dirty 集合——恢复后所有存量行降级 `material_reuse`，等首个权威快照事件（#14）重建基准（宁多标不漏标，§4.4） |

**否决独立存储引擎的依据**：① D2 裁定统一的是寻址语言不是存储引擎——物化层自带查询引擎（SQLite FTS 等）是对 D2 的越权；② 新引擎=新部署面+新失败模式（Windows 文件锁、版本迁移），而 CAS 已解决去重/pin/审计三件事；③ 查询职责已在 L1-3 中央索引+局部索引分层，物化层重复建索引是第二套真相。

**物化行的主键与值**（G1 裁定"ref=物化主键"的落地）：

```go
// 坐标（主键）= ref 五段中的四段；hash 是值不是键——重算前 hash 仍是旧内容身份。
// materialRow（agent/internal/materialize，规划态）
type materialRow struct {
    Ref        agentprotocol.Ref // 五段完整；Hash=当前内容的 sha256:16hex
    Freshness  string            // current / material_reuse / stale（§2.3 状态机）
    Payload    map[string]any    // 投影声明的标量（PayloadFieldSpec 对齐，QUERY_ENGINE §3.1）
    Handle     string            // evidence://<sha256>（PutEvidence 产物）
    Recomputed int64             // 重算代际计数（G2 断言证据，§5）
}
```

- **snapshot 段的承载**按 QUERY_ENGINE §3.3 已给对照执行：dom=内容 ID 型、mom/tim=observation_id、acp=rev_hex8（M4 revision 指纹直接映射）；**generation（物化层内部提交代）与 snapshot 段（投影自身版本轴）是两个轴**：generation 是 SnapshotView 的原子提交单位，snapshot 段随行内容由投影自身身份填充——`SnapshotPredicate{latest}` 由物化层解析为"当前 generation 视图"（QUERY_ENGINE §5.1.2 已裁定归物化层）。
- **不可物化输入的投影**（F9）：fxm（测量随请求携带，E7）、com paired/change_delta（工件采集，E5/E6）在物化层是**登记型**——观察/执行产物落盘时 upsert 行（added/replaced），不参与事件重算，不进依赖图。这与 QUERY_ENGINE §3.3 把它们列入索引种子不矛盾：索引吃的是"已物化的历史结果"，物化层对登记型的职责只是登记与失效（render_revision 变化时标 stale，走 M6 消费时校验兜底）。

### 2.2 裁决 R2：写入时机——事件驱动标脏（eager）+ 读时重算（lazy）+ observe 回填；事件分四批接入

**make 模型的忠实落地**：make 的语义是"依赖变更→目标过期标记→请求目标时重建"。急切的是**标脏**（不漏标是生命线——G2 存在的理由），按需的是**重算**（不为没有读者的投影付费）。否决"事件直达重算"（eager recompute）：① 现状触发者分布未知（RECON §5.6：观察请求频率无基线），eager 重算可能在无人消费的投影上空转；② E9 类"输入必缺后补"（导入后 DAD 声学 pending）会让 eager 重算产出 missing 占位行再返工；③ lazy 与 QUERY_ENGINE §1 边界 3（query 永不触发计算、miss 如实报 deferred）天然一致。

三个重算触发点：

| 触发点 | 时机 | 行为 |
|---|---|---|
| **observe 读端**（主路径，MAT-E 切换） | `RequestObservation` 装配前询问物化层 | 命中 current 行集→直接装配（`recomputed=0`）；miss/stale→走现状 `FinalizeObservationContext` 现算并**回填**物化层（upsert+计 miss）——响应带 `actual_cost_class`+`recomputed`（QUERY_ENGINE §2.6 已定） |
| **query 读端** | 永不重算 | stale/miss 原样透传（引擎边界，不赘述） |
| **pre-warm 后台队列**（v1.x，非 v1） | 脏投影低优先级后台重算 | acousticpackage 已有后台补采先例（`harness.go:4205`）；v1 不做——单写者纪律（AGENTS §9）+真实栈所有权，先让 lazy+回填在真实栈上站稳 |

**事件接入批次**（19 项目录 RECON §3 的裁剪，"工程/render 变更语义事件优先，UI 分发面已证可排除"的落地）：

| 批 | 事件（RECON #） | 事件型 | 物化层动作 | 接入位置 |
|---|---|---|---|---|
| **批 1（P0，v1 必接）** | #1 delta_update、#12 resync/resync_hint、#14 权威快照（get_project_state 应答） | invalidate | scope 解析→脏传播（§4）；#12/#14 全量基准重建：所有行按依赖域重估，存量降级/标脏 | 消费 shadow ChangeReceipt（`shadow.ApplyDelta` 与 `Initialize` 之后挂通知，不重订 ZMQ）；#12 的 stale 打点复用 `mixboard.go:2262` 同源语义 |
| **批 2（P1，v1 必接）** | #4 waveform_envelope、#5 tile_ready、#6-#8 L3 特征族（band_energy/stereo_relation/loudness） | arrive | 输入缓存更新（既有 `writeMixboardReady*Snapshot` 不动）+ 依赖该输入域的投影行标脏（arrive 型脏=现在可重算了） | `IngestKernelTelemetry`（`harness.go:3890-3906`）旁挂 observer——四分支扩展为通知，不改既有写路径 |
| **批 3（P2，v1 接）** | #2 recording_stopped、#3 render_done/render_failed | invalidate（工程级） | 全 kind 保守标脏（粗粒度：take 落地/render 换代使旧测量整体失真）；#3 同时是 com paired 登记行的 render_revision 失效源 | #2 走 `refreshShadow` 完成回调（`bridge.go:417-422` 链）；#3 走 `ingestKernelRenderTelemetry` 后挂 |
| **批 4（P3，v1 收编/记账）** | #9 l2_render_probe_ready、#11/#13 VSP delta/命令 revision、#10 audition.\* | arrive / 记账 | #9：收编进统一入口（现存分叉点 RECON §5.5——临时 SUB `harness.go:5426-5431` 保留兼容，新增经 IngestKernelTelemetry 的通知路径）；#11 同 #1（ChangeSourceExecutorDelta 同表）；#13 只记 generation 帐不标脏；#10 v1 不接（试听会话事件对观察投影是间接消费者，登记开放问题 OQ-5） | 同上 |
| **排除** | #15-#19（UI/hub 分发面） | — | 不接 | RECON §3.3 已证对观察投影无独立响应 |

批内优先序的依据：批 1 是状态轴（一切失效判定的基准）；批 2 是证据轴（dom/mom/tim/acp 的实质输入）；批 3 是粗粒度兜底（宁可全标脏不可漏）；批 4 是收边。

### 2.3 行状态机（F5 铃三分语义的物化版）

```
                 ┌────────── 依赖域命中（标脏） ──────────┐
                 ▼                                       │
  upsert/回填 → current ──输入域再变更──→ stale ──重算提交──→ current
     ▲              │
     │   存量行（历史观察登记、未经本代事件重算、未被标脏）
     └──────── material_reuse（材料身份在，复用前代产物）
```

- **current**：本 generation 内已重算（或登记）且输入域此后未再变更。
- **stale**：脏传播命中，未重算——`Subscribe` 发 `marked_stale`（只改状态不删行，对齐 M2 台账语义与 QUERY_ENGINE §5.1.1）。
- **material_reuse**：未被任何事件标脏的存量行（含崩溃恢复后的全部存量行，§2.1）。
- **判定规则全部在写侧**（标脏事件/重算提交/降级），读侧零推断——M1 铃原则的移植；M8 sanitize（`harness.go:3860-3888`）保留为 belt-and-suspenders 兜底，不作为物化层判定输入。

---

## 3. 事件接入与统一入口（改造面清单）

### 3.1 单一事实源原则

物化层**不新增 ZMQ 订阅**。三个既有入口挂 observer：

| 入口 | 挂点 | 交付给物化层的内容 |
|---|---|---|
| shadow 变更 | `shadow.ApplyDelta`/`Initialize` 完成、ChangeReceipt 生成后 | `ChangeReceipt{ChangeID, Source, AffectedScopes, ChangedEntities, From/ToProject revision/epoch}`（`shadow/change.go:22-43`，亲验）——物化层吃这张收据，天然获得已按序、含 scope 的失效信号，且与台账（M2）消费同一事实源 |
| 遥测特征 | `IngestKernelTelemetry` 四分支（render/waveform/spectral/L3）写完 feature snapshot 后 | 事件类型+携带信息（track_id/clip_id/source_identity/render_revision，RECON §3.1 表"携带信息"列） |
| render 作业 | `ingestKernelRenderTelemetry` 唤醒 waiters 前 | job_id/status/file_path——render 换代信号 |

改造面形态：`harness` 持有 `materialize.Notifier` 接口（物化层实现），既有函数尾挂 `if n != nil { n.NotifyXxx(...) }`——**零行为改动的纯通知**，nil 时与现状逐字节一致（影子模式的编译期安全）。#9 收编同理：`CollectL2RenderProbeBatch` 的临时 SUB 路径保留，另经 `IngestKernelTelemetry` 加 `l2_render_probe_ready` 通知分支（RECON §5.5 指认的分叉点在此闭合）。

### 3.2 事件型二分：invalidate 与 arrive

- **invalidate 型**（#1/#2/#3/#11/#12/#14）：工程状态变了→依赖该状态的投影行**作废**（stale）。
- **arrive 型**（#4-#9）：新证据到了→输入缓存更新（既有路径不动）→依赖该证据的投影行标脏但语义是"**可重算**"（如 E9：DAD 声学到达后 TIM 行从 acoustic missing 升级可替换——现状 `messageLoopTIMProjectionShouldReplace` `message_loop.go:7830-7849` 的覆盖度比较替换逻辑，物化层把它泛化为依赖图上的就绪门，§4.5）。

两类在脏传播算法里同构（都进 dirty 集合），差别只在重算收益方向——设计上无需分叉数据结构，只在 metrics 里分开计数（§5 断言证据）。

---

## 4. 依赖图与脏传播设计（验收②）

### 4.1 裁决 R3：依赖声明=静态注册表（数据表），实例匹配=构造时坐标——两段式

对照 17 条依赖边的现状（RECON §2）：真正的投影→投影持续依赖只有 E1（TOM←TIM digest，**可选**引用）与 E2（EPM←TIM/TOM，**仅 import 装配一次性传值**）；其余 15 条全是"投影←输入源"（feature snapshot 行、acousticpackage 状态、shadow 域、请求携带测量、双 tap 工件）。**依赖图的主体是"投影 kind ↔ 输入域"，不是投影间网**——这由现状数据决定，不由偏好决定。

```go
// agent/internal/materialize/registry.go（规划态）——kind 级依赖静态注册表
type kindDependency struct {
    Kind            string   // 注册 kind（refschema 注册表成员）
    InputScopes     []string // 消费的输入域（§4.2 词表）
    Compute         string   // "precomputable"（事件可重算） | "registered"（登记型，F9）
    Recompute       func(ctx context.Context, deps DepInputs) (ProjectionResult, error)
    // ^ v1 逐 kind 适配：precomputable 才有 Recompute；registered 的写入只有登记路径
}
```

- **静态注册 vs 构造时声明的裁决**：kind→输入域映射是**静态数据表**（编译期+启动期校验：所有 InputScopes ∈ 域词表、所有 Kind ∈ refschema 注册表）；**行级（实例）匹配在传播时做**（变更的 scope 值 vs 行的 ref 坐标）。依据：① 输入域集合是投影的本质属性（Build 读什么代码说了算），静态表可被 G2 表驱动测试锁定；② shadow `addChangeScopes` 同为静态 switch（`change.go:200-222`），风格同构便于对照审计；③ 17 条边无一需要运行时动态声明（无自省型依赖）。
- **E1/E2 peer 边 v1 不进脏传播**：TOM 对 TIM 是 digest 可选引用（manifest §4：缺失不影响 readiness），E2 是 import 装配传值（`message_loop.go:8592-8593`）非持续依赖——装配时快照传值不是物化依赖。v2 若 TIM/TOM 均物化且 TOM 改读物化行，升级为图边（登记 OQ-3）。**这不是漏标**：E1/E2 的消费端读的是装配时已固化的值，物化层不承诺其新鲜度。

### 4.2 域词表与 scope→kind 映射表（shadow 表的改造面）

脏传播的映射是**两张表串联，均不改 shadow 输出**：

```
事件/收据 ──(表 A：既有，不动)──> 观察域 scopes ──(表 B：新增)──> kind 集合 ──(行坐标匹配)──> ref 行集
```

- **表 A（既有）**：`addChangeScopes`（`shadow/change.go:200-222`）path→域。物化层直接消费 ChangeReceipt.AffectedScopes，**零改造**。
- **表 B（新增，数据表+表驱动测试锁定）**：观察域→kind。初值（对照 RECON §1.1 触发链矩阵与 QUERY_ENGINE §3.3 逐 kind 推）：

| 观察域（表 A 词表） | precomputable kinds | registered（登记）kinds |
|---|---|---|
| track.level | dom, mom, tim | rlm（半物化，§4.5） |
| track.stereo_space | dom, mom | — |
| track.basic_energy / time_dynamics / timbre_frequency | dom, tim | — |
| mix.multitrack_relationship / frequency_relationship / masking_relationship | mom | — |
| project.headroom | dom | rlm |
| project.structure | tom, tim, epm | — |
| processor.identity_and_controls / behavior / change_delta | — | com（paired/change_delta 登记行，topology 失效走 §2.1 登记型规则） |
| comparison.before_after | — | fxm |
| project.state（表 A 默认兜底） | **全部 precomputable kind 保守标脏** | 全部登记行标脏 |

- **宁多勿漏原则**：未知域、空域、词表外 path 一律走 project.state 兜底全标脏——失效正确性的第一敌人是漏标，过标只是浪费一次重算（lazy 下甚至不浪费）。这是 fail-loud 的失效版。
- **arrive 型事件的域**：#4/#5→`feature.waveform`+`feature.spectral`（→dom/tim）；#6/#7/#8→`acoustic.l3.<family>`（→mom/acp upsert+dom/tim 标脏）。arrive 域直接进表 B 同一结构（域字符串按携带信息拼装：track_id 作为 scope_value 参与行匹配——#6 只标脏 `scope(track,T3)` 的行，不碰 T7）。

### 4.3 传播算法（事件→scope 解析→受影响投影集合）

```go
// 传播伪码（实现于 materialize.Store.HandleReceipt / HandleFeatureArrival）
1. 解析输入：ChangeReceipt.AffectedScopes（invalidate）或事件携带的 track/clip/render 身份（arrive）
2. 域→kind：查表 B；未知/空 → 全 kind 兜底
3. kind→行集：内存索引按 kind 选行，再做 scope 坐标匹配：
   - 收据带 ChangedEntities(track T3) 时，scope(track,T3) 行命中；mix.*/project.* 域行全命中（这些投影天然全曲域）
   - window 段 v1 不参与匹配（整投影重算，§4.4）
4. 标脏：命中行 stale + RecomputedDirty 计数；发 MaterializedChange{Op: marked_stale}（不删行）
5. 记账：ChangeID 记入行（对齐台账 invalidated_by_change_id，M2 语义）——失效证据可追溯（路线图 §5 风险缓解项）
```

**与 shadow/台账的冲突检查（停止条件预检，亲验）**：`addChangeScopes` 是纯函数、输出仅观察域字符串；台账消费方式是"scope 命中→view 行打 stale"（`free_state_reasoning_loop.go:1725-1759`）；物化层是**同一收据的新消费者**，不修改 shadow 输出、不抢占台账职责（台账管观察披露面，物化层管投影实例，两个粒度并存）。无不可调和冲突——停止条件未触发。台账未来可改为 Subscribe 消费者之一（OQ-4）。

### 4.4 裁决 R4：失效粒度——v1 整投影重算 + 行级 diff 出增量流；窗口级局部重算留 v2

**裁决**：脏的单位=投影（kind+scope 坐标全行集）；重算的单位=整投影（Build 全量函数）；**变更流的单位=行**（重算后逐行 hash 对比，仅变更行发 replaced/added）。

依据：

1. **Build 无窗口参数化是现状硬约束**：`dom.Build`/`mom.Build` 吃整包输入（feature snapshot 全量行+acoustic 整包），仅 fxm 随请求携带（F9，登记型）；"局部窗口重算"需要输入按窗分片——特征快照行按窗分片尚不存在（RECON §1.3：遥测写的是整 track/clip 行）。
2. **G2 第一敌人是漏标，粒度越细漏标面越大**：窗口级失效要求"变更→受影响窗口"的精确映射（参数 automation 的时间段求交），这是新一类正确性风险；v1 先证"整投影不漏"，v2 在正确地基上细化。
3. **增量收益不靠缩小重算单位也能拿到**：行级 hash diff 使 Subscribe 流量天然增量（未变行不发事件、查询引擎不重建整表）；物化命中率收益主要来自"无关投影不重算"（脏传播已解决），而非"投影内无关窗不重算"（v2 目标）。
4. **浪费上界可控**：v1 规模 ≤100 轨，单投影 Build 属 compile 级（亚秒，D5 分级），lazy 语义下只有被读的投影付费。

v2 升级路径（登记不承诺）：特征快照行按窗分片后，window 段进传播匹配（§4.3 步骤 3 解锁）；E10 before_after 类跨观察 delta 边天然按 observation_id 对齐，不受影响。

### 4.5 半物化投影：rlm 与 B1 pack（E8/E16 的处置）

rlm 的输入是三源工具结果记忆（`state.executed`，E8/E16 同族"记忆型输入无新鲜度门"），不是工程状态——**v1 物化层对 rlm 只登记不重算**（表 B 中标 registered 半物化）：capability preflight 产物落盘时 upsert 行，freshness 按 M6 消费时校验透传。理由：把 rlm 输入域改接到 shadow/feature snapshot 是行为变更（动 `buildReferenceRows` 三源合并，`rlm/projection.go:128-134`），超出物化层基建卡边界；E8 的修复（三源 revision 比较）单开修复卡，G2 红测先行锁定现状（§5.4）。

**依赖图上的就绪门泛化**：E9 的"等上游→覆盖度比较替换"（`message_loop.go:7548-7593`/`7830-7849`）目前绑定在 stems import 回复路径——物化层把它泛化为 arrive 型事件的通用语义：arrive 只标脏不阻塞，重算时 Build 自带 missing 占位行（`tim/import.go:58-64` 同款），下一次 arrive 再触发重算收敛。**不做集中式就绪门**（不阻塞事件流、不等待上游）——收敛性由"arrive 会再触发"保证，其正确性由 G2-B 对拍锁定（增量轨迹与全量轨迹收敛态一致）。

---

## 5. G2 失效正确性测试设计（验收③：可写测试级）

G2 质量门对象=本设计 §3/§4 的实现。测试四组+先行观测卡，全部给到"测试函数签名+fixture 构造+断言证据"级。

### 5.1 断言证据的三个抓手

| 证据 | 形态 | 说明 |
|---|---|---|
| **重算计数器** | `materialize.Metrics`（per-kind：`Invalidations` / `Arrivals` / `Recomputes` / `RowsUpserted` / `RowsUnchanged` / `Reads` / `Hits` / `Misses`） | 测试与影子模式对账共用同一仪表（§7）；注入事件后直接读计数断言 |
| **ref 的 hash/snapshot 变化** | 物化行 `Hash` 段（内容 ID 变了）与 `Recomputed` 代际字段 | 不依赖计数器黑盒：内容身份变化是白盒证据 |
| **Subscribe 流序列** | 订阅 `marked_stale→replaced` 事件序列断言 | 锁定"不删行+顺序"语义 |

### 5.2 G2-A：多级链级联失效（G2 门原文的字面落地）

**链 fixture 构造**（两层，各含一条真实链）：

- **合成链（可控）**：`testInput → TestKindA → TestKindB`——materialize 包内测试专用注册（表 B 测试条目：TestKindB 声明依赖 TestKindA 的输出域），Build 为确定性函数（输入 hash 混入计数），排除真实 Build 的噪声。
- **真实链（证接线）**：E9 型——`L3 声学 arrive 事件 → tim 行标脏 → 重算`（acoustic missing→ready 收敛）；E1 型对照（**不级联**：tim 重算不得触发 tom 重算——E1 边 v1 不在图内，负向断言防过度传播）。

**注入方式**：直接调 `store.HandleReceipt(合成 ChangeReceipt{AffectedScopes:...})` / `store.HandleFeatureArrival(合成 #6 事件)`——不经真实 ZMQ（单测层）；接线正确性由 G2-D 烟测覆盖（§5.5）。

**测试函数（签名级）**：

```go
func TestInvalidationCascadesThroughMultiLevelChain(t *testing.T)
  // 步骤：物化 A、B（各自 current）→ 注入根输入域 invalidate → 断言：
  // ① B 行 Subscribe 收到 marked_stale（即使变更只指向 A 的输入域——级联经表 B 传播）
  // ② 触发 B 重算（读端/显式 Recompute）→ Metrics.Recomputes["TestKindB"]+1
  // ③ 重算后 B 行 Hash 变化、Recomputed 代际推进
func TestInvalidationScopePrecision(t *testing.T)
  // 变更 track:T3 → scope(track,T3) 行 stale；scope(track,T7) 行 freshness 不变；
  // mix.* 域行（全曲域）stale——域语义按表 B 锁定
func TestUnknownScopeFailsLoudAllDirty(t *testing.T)
  // 未知域/空 scopes → 全部 precomputable kind 行 stale（宁多勿漏，§4.2）
func TestStaleNotDeletedNotUpgraded(t *testing.T)
  // marked_stale 后 SnapshotView 行仍在、Freshness=stale；任何路径不得回写 current
  // （对齐 QUERY_ENGINE T6 与 M2 旧行保留审计）
```

### 5.3 G2-B：漏标检测——全量重算 vs 增量对拍（D4 验收重点的测试形态）

**Oracle**：`FullRebuild()`——物化层自带的全量重算入口（无视 dirty 集合，全部 precomputable kind 强制重算）。两轨同事件序列跑，收敛态逐行对拍：

```go
func TestIncrementalMatchesFullRebuild(t *testing.T)
  // 构造：合成工程（≥2 轨+插件链，复用 QUERY_ENGINE §6.1 生成器同源）→ 双 Store 实例
  // 轨 I：正常脏传播+lazy 重算；轨 F：每事件后 FullRebuild
  // 断言：事件序列结束后两轨 SnapshotView 逐行 deep-equal（ref+freshness+payload+handle）
  //   ——hash 相等是内容相等；freshness 允许 current≡current / stale≡stale 严格相等
func TestFuzzEventSequenceIncrementalEquivalence(f *testing.F)
  // 固定 corpus seed 起步；随机事件序列（invalidate/arrive 混合、乱序 seq、重复投递）
  // 每步后对拍收敛态；fuzz 发现分歧即实现 bug，禁改 oracle 凑绿（AGENTS §10）
func TestDuplicateAndReorderedEventsConverge(t *testing.T)
  // 同事件重复投递不产生重复代际抖动（generation 不前进或幂等前进，断言其一锁定）；
  // #1 与 #14 乱序到达（delta 先于权威快照）→ 收敛态与正序一致
```

**对拍的可信度前提**：FullRebuild 与增量轨共享同一 Build 函数与输入缓存读取路径——对拍检测的是**传播漏标**（dirty 集合计算错误），不是 Build 自身错误（后者归各投影单测）。此边界写进测试注释。

### 5.4 G2-C：既有隐患红测（E8/E16，RECON §5.3 指定素材）

```go
func TestRLMThreeSourceMergeRevisionMismatchIsVisible(t *testing.T)  // E8 红测
  // 构造：三源 rows 带 revision 不一致（mix.observe 后工程再变更的时序，RECON E8 时序证据）
  // 断言（现状锁定）：rlm.Build 成功但物化行 freshness 注记 revision_mismatch_risk=visible
  //   ——v1 诚实透传（QUERY_ENGINE §3.3 rlm 行"stale 风险按现状透传，修复归 G2"的落地）；
  //   修复卡（mergeSourceRows 加 revision 比较）合入后本测试升级断言：异 revision 源触发 stale
func TestB1PackSurvivesProjectChangeInRun(t *testing.T)  // E16 红测
  // 构造：run 内建 pack → 注入工程变更 → 断言（现状锁定）：pack 不重建（复现 E16）+ 登记号
  //   （OQ-6：B1 pack 输入域改造卡）；此测试是"记忆型输入无失效钩子"的活样本看门狗
```

红测纪律（AGENTS §10"不得弱化期望凑通过"）：两测试锁定**现状行为**并显式标注"已知隐患，修复卡 XXX 后转绿升级断言"——它们的存在使隐患可见、修复可验，不是豁免。

### 5.5 G2-D：接线、并发与恢复（端侧收口）

```go
// 单测层
func TestNotifierNilIsByteIdentical(t *testing.T)      // Notifier=nil 时 IngestKernelTelemetry/ApplyDelta 路径与现状逐字节一致（§3.1 纯通知承诺）
func TestRecoveryFromManifestRebuildsIndex(t *testing.T) // manifest+CAS 句柄重建 → 存量行全 material_reuse；首个 #14 后基准重建
func TestReadCommittedPerGeneration(t *testing.T)       // SnapshotView 遍历中并发提交新代 → 已开始的结果集不受影响（QUERY_ENGINE §3.4 同款）
// 端侧烟测（AGENTS §5 门槛，MAT-D/E 收口）
// dev_agent_smoke.ps1 体系加物化场景：拉起三件套 → 开测试工程 → 影子模式跑 observe
// → 断言影子对账零分歧（§7.2）→ 退出码 0；只读白名单模式参照 g_runtime_readonly_smoke.ps1
```

### 5.6 先行观测卡（RECON §5.6 建议，MAT-0）

物化命中率在现状下无基线数据。MAT-0（最小卡，先于一切物化实现）：`FinalizeObservationContext` 各 finalize 加只读计数（per-kind finalize 调用频率分布），跑真实工程观察会话采一天数据——为 §7 切换顺序与 pre-warm（v1.x）是否值得提供数据。零行为改动（计数器+结构化日志）。

---

## 6. 与 L1-3 读口对接：三函数落点与签名细化（验收⑤）

### 6.1 裁决 R5：契约类型下沉 `agentprotocol`（materialstore.go 新文件）

QUERY_ENGINE §5 把 `MaterializedStore` 定义在 queryengine 语境，§5.1.4 定了单向依赖原则。实现侧细化：**契约接口与伴生类型（MaterializedStore/MaterializedRow/MaterializedChange/ResolvedEvidence/SnapshotSelector）定义在 `agentprotocol`**（协议共享包），queryengine 与 materialize 都只 import 协议包。依据：① Subscribe 的消费面不止 queryengine——台账（OQ-4）、审计、未来 memory 检索都是潜在订阅者，契约留在引擎包会迫使非引擎消费者 import 引擎；② G1 裁定 #4 已把前缀注册表挂在 agentprotocol（协议归属惯例）；③ 双向解耦后"物化层不知道引擎存在"从运行时语义升级为编译期事实。QUERY_ENGINE §5 签名**零改动**，仅包位置迁移（其 §5.1 契约要点四条全部保留有效）。

### 6.2 三函数的实现落点（`agent/internal/materialize`，规划态）

```go
// SnapshotView：当前代际整表的不可变视图（copy-on-write 语义天然支持——直接返回代指针）
//   selector 解析：latest=当前 generation；exact/at_or_before=按行 Snapshot 段值过滤
//   （v1 只需 latest 正确；exact/at_or_before 随历史代保留策略走，登记 OQ-2）
func (s *Store) SnapshotView(ctx context.Context, sel agentprotocol.SnapshotSelector) ([]agentprotocol.MaterializedRow, error)

// Resolve：内存索引行→Handle（物化写入时已 PutEvidence pin，此处零 CAS 调用）；
//   未 CAS 化历史产物→工件路径句柄+freshness 注记（对应 ref #hash="-" 显式语义，QUERY_ENGINE §5.1.3）
func (s *Store) Resolve(ctx context.Context, ref agentprotocol.Ref) (agentprotocol.ResolvedEvidence, error)

// Subscribe：变更流 fan-out（每订阅者独立缓冲 channel，慢消费者丢弃+计数，不阻塞写侧）；
//   事件序：单写者串行产出（§7.4），订阅者见到的序列=提交序
func (s *Store) Subscribe(ctx context.Context) (<-chan agentprotocol.MaterializedChange, func(), error)
```

写侧入口（queryengine 不知道的第四个函数，物化层自有）：`Upsert(rows []materialRow) error`——重算/登记的提交点，内部做行级 hash diff→发 `added/replaced`→generation 原子推进。

### 6.3 ObservationKey 扩段（G1 ruling #5 作业的承接）

G1 排程钩子："L1 结构化键（ObservationKey 扩段）排 L1-2 物化段开工时"。落地设计：

- **演化的对象**：`audioclosure.ObservationKey`（`audioclosure/types.go:132-141`，现八字段：ProjectUUID/ProjectRevision/Scope/TargetRef/ViewIDs/ObservationMode/Tap/TimeWindow）——不新建类型（G1 #5 裁定"新建=两套真相"）。
- **扩段内容**：`RefKind string`（物化 kind，对齐 refschema 注册表）+ `RefHash string`（内容身份）——使"观察记录→物化行"可互查（observation_id 在 ObservationRecord 已有，扩段后 ObservationKey 与 L0 ref 五段形成完整映射：Kind↔RefKind、Scope↔scope、TimeWindow↔window、ProjectRevision↔snapshot 语义轴）。
- **落卡**：MAT-A 内实现（agentprotocol 契约下沉同卡，都是零行为纯新增）；旧状态缺省语义：新字段 `omitempty`，旧记录反序列化缺省空串=未物化（AGENTS §11 兼容纪律），补一条旧记录往返测试。

---

## 7. 新旧并存与切换策略（验收④）

### 7.1 裁决 R6：环境变量 flag + kind 白名单，三态运行

```bash
VIT_DAW_MATERIALIZATION=off|shadow|on     # 默认 off；harness 既有 VIT_DAW_* 环境变量惯例（如 VIT_DAW_AUDITION_BLIND）
VIT_DAW_MATERIALIZED_KINDS=tom,acp,dom    # 白名单：flag≠off 时参与物化/切换的 kind 子集
```

| 态 | 物化层行为 | 读端（RequestObservation）行为 | 回退动作 |
|---|---|---|---|
| **off**（现状） | Notifier=nil，逐字节现状（G2-D 锁定） | 现算 | — |
| **shadow**（影子） | 全速物化：事件接线+脏传播+lazy 重算+变更流，但**读端不消费** | 现算照旧；每轮 observe 后**对账**：现算产物 vs 物化行逐行 hash 比，分歧计数入 Metrics（`ShadowDivergences`） | flag 回 off |
| **on**（切换） | 同 shadow | 白名单 kind：命中 current 行集→直接装配；miss/stale→现算+回填（`recomputed`/`actual_cost_class` 如实上报） | flag 回 shadow/off——**读端切换不改 FinalizeObservationContext 现路径一行**，回退零成本 |

**FinalizeObservationContext 原路径零改动**是回退路径的设计核心：物化层是旁路读端+回填写端，on 态的"命中"发生在它之前（装配输入就绪后直接跳过 finalize），miss 时走原路径。一张卡内不删不改既有函数（AGENTS §12：现路径是权威输入）。

### 7.2 切换顺序：三个先切 kind 与依据

| 序 | kind | 依据 |
|---|---|---|
| 1 | **tom** | 输入域最简（shadow 镜像 project.structure，无特征依赖）；query 面高频起点（"哪些轨存在"，QUERY_ENGINE §3.3 建议 v1 优先接）；失效语义最浅（结构变更才脏） |
| 2 | **acp**（acousticpackage） | 物化层最成熟种子（M4 store：Upsert 交叉失效+rev_hex8 指纹已实现十年等价物）——物化层大半是给既有 store 换 ref 主键+接事件 |
| 3 | **dom** | 观察主链第一站（catalog.go:66-79 两处调用）；内容 ID 语义最正（hash 段语义最干净，QUERY_ENGINE §3.3）；输入域=feature snapshot+F5 铃（M1 标注直接映射行 freshness）——覆盖 invalidate+arrive 两类事件的完整链路 |

**刻意不进 v1 首切**：mom（依赖 acp+特征+shadow 三域，等 acp 站稳后第二波）；tim（双路径：观察+导入回执，DAD gate 收敛语义要先在 G2-B 证稳）；rlm（半物化，§4.5）；fxm/com（登记型，随观察落盘自动进，无"切换"概念）。

**切换闸门**：shadow 态对账 `ShadowDivergences==0` 连续 N 个真实观察会话（N 由决策侧按会话量裁，建议 ≥3）→ 单 kind on → 真实栈烟测（G2-D）→ 逐步扩白名单。每步都留 flag 回退。

### 7.3 存量与并发纪律

- **单写者**：物化层写入（标脏/重算提交）全部在 Notifier 调用链（主循环 goroutine）或读端回填的同步段——v1 无并发写者；generation 原子交换是唯一跨 goroutine 可见点（§5.5 TestReadCommittedPerGeneration 锁定）。
- **存量迁移**：无。物化层从空库起步（off→shadow 时冷启动），历史观察票不回填（v0 bootstrap 扫描器已服务 L1-3 起步，QUERY_ENGINE §5.2——两轨在 shadow 对账期并存互证）。

---

## 8. 实现卡排程建议（验收⑥）

| 卡 | 内容 | 验收线 | 依赖 |
|---|---|---|---|
| **MAT-0** | 观察请求计数器（per-kind finalize 频率，§5.6） | 计数入结构化日志+一条单测；零行为改动（Notifier-nil 同款锁定） | 无（先行小卡，可立即领） |
| **MAT-A** | 契约类型下沉 agentprotocol（§6.1）+ materialize 包骨架：内存代际索引+三函数+Upsert 行级 diff+Metrics（§5.1）+ ObservationKey 扩段（§6.3） | 契约测试（G2-C 的 T 系列：stale 不删行/读提交/generation 原子性）+旧记录往返测试；纯新增包全绿 | REFSCHEMA-L0-2（acp/tom 等 kind 注册，硬前置） |
| **MAT-B** | 事件接线：Notifier 三挂点+#9 收编（§3.1）+表 B 域→kind 映射+脏传播（§4.2/§4.3） | G2-A 四测试绿（多级链级联/scope 精确/宁多勿漏/stale 不升级）+Notifier-nil 逐字节锁定 | MAT-A |
| **MAT-C** | 三投影适配：tom/acp/dom 的 Recompute 包装+登记路径（fxm/com 随观察落盘）+影子模式对账（§7.1） | G2-B 对拍+fuzz 绿；影子态 `ShadowDivergences==0` 于合成工程；E8/E16 红测入库（§5.4） | MAT-B |
| **MAT-D** | **G2 收口**：崩溃恢复+乱序收敛（§5.5）+真实栈烟测场景（dev_agent_smoke 扩展） | G2 质量门裁定材料：全部 G2 测试+烟测 exit 0；决策侧验收 | MAT-C |
| **MAT-E** | 读端切换：on 路径（命中装配/miss 回填）+回退演练 | 真实栈烟测（on 态）exit 0+回退演练记录；`recomputed/actual_cost_class` 响应字段接线（与 CCB-PARAM 联合断言，QUERY_ENGINE T11） | MAT-D（G2 通过后） |

顺序与并行：MAT-0 独立先行；A→B→C→D 串行（每步是下一步的被测对象）；E 等 G2 门。MAT-C 期间 CCB-PARAM（QUERY_ENGINE §9）可并行——两者在 observe 响应字段（T11）汇合。文件域：MAT-A/B 集中 `agent/internal/materialize`+`agentprotocol`（新文件）；MAT-C 触 `harness.go`/`mixboard.go` 挂点（通知尾挂，diff 极小）；MAT-E 触 `RequestObservation` 装配前置——按 AGENTS §12 依序提交。

---

## 9. 开放问题与边界（上交决策侧）

| # | 开放问题 | 建议 | 影响 |
|---|---|---|---|
| OQ-1 | 历史 generation 保留策略（SnapshotExact/AtOrBefore 需要旧代视图；v1 只保当前代） | v1 只 latest；历史代走"旧观察票+evidence CAS 可重放"既有路径，物化层不做代际 GC 复杂度 | QUERY_ENGINE diff.at_or_before 类查询 v1 降级，已在其 §2.3 声明 v1 只放 latest |
| OQ-2 | 台账（M2）是否改为物化层 Subscribe 消费者（统一失效面） | v2 收敛：物化层行级 stale 流可推导台账 view 行失效，消除双份 scope 匹配逻辑 | 不阻塞 v1；改前台账行为不变 |
| OQ-3 | E1/E2 peer 边 v2 升级（TOM 改读 TIM 物化行） | 待 TIM/TOM 均物化且 TOM Build 支持外部注入 TIM 行后裁 | v1 按装配时快照传值，不构成物化依赖（§4.1） |
| OQ-4 | rlm 输入域改造（E8 修复+B1 pack 失效钩子，E16） | 独立修复卡，G2-C 红测先行锁定现状（§5.4）；改造=把三源接到 shadow/feature snapshot 域 | 物化层 v1 对 rlm 只登记（§4.5） |
| OQ-5 | audition.\*（#10）是否进物化事件源 | v1 不接（试听会话事件对观察投影是间接消费者）；若试听 A/B 结算需要投影失效再裁 | 无阻塞 |
| OQ-6 | pre-warm 后台队列（v1.x）启动时机 | 由 MAT-0 频率数据裁：若 observe miss 率高且集中于固定 kind，值回票价 | 无阻塞 |

**边界申报（诚实）**：① 本设计的事件批 1/批 2 接线正确性最终依赖真实栈时序（ZMQ 到达序、F5 铃并发写行为——RECON §6 已申明动态时序未验证），G2-D 烟测是唯一收口；② 表 B 初值由静态推演（触发链矩阵+§3.3 表），真实工程的 scope 命中分布要靠 MAT-0 数据校准——错配项在影子模式对账期暴露（宁多勿漏原则保证错配只浪费不静默）；③ 内核 C++ 发射侧全集未盘（RECON §6），#9 的 REQ 应答+SUB 双通路发射条件依赖内核侧勘察卡（已在 RECON 边界登记）。

---

## 10. 与既有纪律的对照自检

| 纪律 | 本设计的遵守方式 |
|---|---|
| 投影 peer 非链（AGENTS §5） | 依赖图主体是"kind↔输入域"（§4.1，由 17 边现状数据决定）；仅有的两条 peer 边（E1/E2）v1 不进图——不制造 DOM→MOM 式管线 |
| stale 原样保留不升级 | 行状态机（§2.3）只降不升；G2-A `TestStaleNotDeletedNotUpgraded` 锁定 |
| 读侧零推断（M1 铃原则） | freshness 判定全在写侧（标脏/提交/降级三事件）；M8 sanitize 保留为兜底不进判定（§2.3） |
| 失效证据可追溯（路线图 §5） | ChangeID 记入物化行（§4.3 步骤 5，台账同款） |
| D2 统一寻址不统一存储引擎 | 存储三层全复用既有设施（§2.1）；物化层不带查询引擎 |
| D5 成本分级 | lazy 重算+observe 回填的 `recomputed/actual_cost_class` 如实上报（§7.1） |
| 测试不写源码树（AGENTS §10） | 合成工程 fixture 走 `t.TempDir()`/专用工作区；真实产物只读复制 |
| 新旧并存可回退（§12 现路径是权威输入） | FinalizeObservationContext 零改动，on/off/shadow 三态 flag（§7.1） |
| 持久化兼容（AGENTS §11） | ObservationKey 扩段 omitempty+旧记录缺省"未物化"+往返测试（§6.3）；manifest 原子写（§2.1） |

---

## 11. 验收对照（卡面七项）

| 卡面验收项 | 本设计 | 状态 |
|---|---|---|
| ①存储形态裁决带依据 | §2.1 R1（三层复用，否决独立引擎给三条依据）+§2.2 R2（eager 标脏+lazy 重算，事件四批接入表） | ✅ |
| ②传播算法+失效粒度 v1 建议 | §4.3（两张表串联传播算法）+§4.4 R4（整投影重算+行级 diff，四条依据；窗口级留 v2） | ✅ |
| ③G2 测试矩阵可写测试级 | §5 全部到函数签名+fixture 构造+注入方式+断言证据级（四组+先行观测卡） | ✅ |
| ④并存策略+切换顺序 | §7 R6（三态 flag+kind 白名单+FinalizeObservationContext 零改动回退）+§7.2（tom/acp/dom 三先切+依据+闸门） | ✅ |
| ⑤三函数落点 | §6（契约下沉 agentprotocol 裁决+实现签名+ObservationKey 扩段承接 G1 #5） | ✅ |
| ⑥实现卡排程 | §8 六卡（MAT-0~E）依赖序+验收线+文件域 | ✅ |
| ⑦文档入库 | 本文件，port/l1-2-design-1-materialization-v1 分支 | ✅ |
| 停止条件：依赖图与 shadow 受影响域不可调和冲突 | 未触发——亲验结论见 §4.3（物化层是同一收据的新消费者，shadow 输出零改造） | ✅ |

---

*本文档由 L1-2-DESIGN-1 卡（PC 执行侧，GLM-5.3/L2）产出；事实锚 L1-2-RECON（@9b85b9f）+本次亲验（HEAD=6be92e49：catalog.go:58-84 / shadow/change.go:200-222 / evidence.go:49-75 / free_state_reasoning_loop.go:1725-1759 / audioclosure/types.go:132-141 / refschema.go 全文）；规格锚 G1 ruling + QUERY_ENGINE_V1_DESIGN §5。零代码改动。*

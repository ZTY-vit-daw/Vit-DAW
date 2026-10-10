# L1-1-REFSCHEMA-1：G1 终审定版评审记录（2026-10-07，决策侧亲自）

- 卡：中枢 `queue/todo→done/2026-10-05-L1-1-REFSCHEMA-1-ref-schema-g1-review.md`（决策侧自任，路线图线 1 首段）
- 评审 HEAD：`412820cc`（main）；工作树仅运行时 XML 与 coord/runs 未跟踪工件
- 定版产物：[docs/REF_SCHEMA_V1.md](../../../docs/REF_SCHEMA_V1.md)（现行索引已登记）
- 输入证据链：G1 ruling 2026-09-27（八裁决+L0 文法基线）→ REFSCHEMA-L0-1/L0-2（实现+kind 注册补全，09-27）→ REFSCHEMA-D1/D2（内核 D 类 5/5 迁移收官，09-28）→ L1-3 queryengine IMPL-A/B/C/D（kind 命名与中央索引消费面落地，10-06 收官）→ 本终审。

## 1. 定版裁定

**裁定：统一 ref schema V1 定版通过。** 五元组语义、BNF+canonical form、双注册表（14 legacy 条目+4 prefix-less kind）、时间三层、对象类型轴与 memory 预留、CAS 对接约定、六不变式、三态解析策略——全部落 `docs/REF_SCHEMA_V1.md`，与 Go 权威实现/内核镜像逐条对得上（本审亲核 refschema.go 644 行全文 + RefSchema.h/cpp 回执 + QUERY_ENGINE §3.3）。

关键裁定点（本终审新增，超出 09-27 ruling 八条的收口项）：

| # | 裁定点 | 裁定 | 依据 |
|---|---|---|---|
| F1 | 定版文档落位 | `docs/REF_SCHEMA_V1.md` + CURRENT-STATE 现行索引 | ruling 尾注承诺兑现；此前规格只活在代码注释与 ruling 里 |
| F2 | 对象类型轴承载 | 对象族进 kind（新对象=新 kind 注册）；地址性对象进 scope_kind 词汇表（v1 五现役+四预留）；memory 预留 `memory.entry`/`memory.distilled` 命名空间 | D2"坐标 vs 对象"分判；charset 已容纳零文法改动 |
| F3 | hash 截断 64bit | 维持 16hex；定位=地址提示+完整性抽检，非存储主键；全量 digest 归物化层 | ≤10⁵ refs 生日碰撞 ≈2.7e-10；QUERY_ENGINE §3.4 规模 |
| F4 | 时间归一归属 | 消费边界义务（生成/查询谓词侧）；ref 内只存采样点；采样率绑定快照 | D3 三层；五元组不加字段 |
| F5 | 迁移与 L1-2 关系 | 中央索引以 legacy 翻译条目种子即可开工，**生成点迁移非 L1-2 硬前置** | QUERY_ENGINE §3.3"DAD 迁移前以 legacy 翻译条目进中央索引"先例 |
| F6 | 实例/内容身份分判 | 按 kind 注记（dom/com 内容身份，fxm/rlm 实例身份），diff Changed 判定按注记 | 盘点 I1；§3.3 fxm 行 |

## 2. 反例测试清单

**已覆盖（锚点亲核，双端）**：

- Go `agentprotocol/refschema_test.go`（11 项）：合法表/拒绝表（缺段、malformed、非 hex 转义、截断转义、裸保留字符、前导零、溢出、大写 hex、错 hash 长度）/三态/legacy 翻译/注册表初值/Format 黄金/Format 拒绝/转义往返/WARN-once+计数/nil 静默/并发计数。
- 内核 `VitApp/Tests/RefSchemaTests.cpp`：双配置 10/10（D1 红绿 9 FAIL 形态实测；T5 dual_tap 真实采样窗黄金串）。
- 消费面：com 双形态校验（legacy+vit:// 同帧兼容、错 kind/PairID 拒）——REFSCHEMA-D2 红先行实测。

**待排缺口（不阻塞定版，分别挂靠后续卡验收）**：

| 缺口 | 挂靠 | 形态 |
|---|---|---|
| CAS 重拉一致性（INV2 后半） | L1-2 物化实现卡 | 同 hash 段 ref 双次 resolve 字节一致断言 |
| 时间三标尺归一（秒/bar:beat→采样点） | L1-3 工具面接线卡 | 边界归一单测+无 tempo map 拒绝路径 |
| 双端黄金向量形式化 | 下一张 refschema 触碰卡 | 共享向量文件（Go FormatRef vs C++ builder 同集输出逐字节一致） |

## 3. 现状核对（生成面迁移余额）

亲核（2026-10-07 HEAD）：agent 侧生成点**仍发 legacy 格式**——`dom_`（dom/projection.go:530-537 实读）、`acoustic_package_status:`/`observation:`（mom/evidence.go:23-39 实读）等；内核 D 类 5 处已迁 vit://（D1/D2 卡）。**注册表缺口**：`observation:`（mom C3 高频族，与 `audio_observation:` 不同物）未注册——现落 opaque（WARN 计数积累中）。~~盘点 §6 两处未覆盖（Godot 前端写者/PCA 层）维持待勘~~。**后续落账（同日验收）**：M1 已补 `observation:` 条目（REFSCHEMA-M1 验收合入 00f96200，注册表现 15 条）；M8 勘察已排闲时任务（offpeak-3b571695）。

## 4. 迁移计划（类 × 动作 × 触发 × 优先级）

原则：渐进归一（F5）；新生成强制 vit://+hash；存量不重写（INV1）。**执行卡从本表取材，逐卡立卡不整批**。

| 序 | 类/面 | 现状 | 归一动作 | 触发 | 优先级/引擎 |
|---|---|---|---|---|---|
| M1 | 注册表补全：`observation:` + opaque 计数头 | 缺口，落 opaque | legacyPrefixRegistry 追加（族=evidence_scheme_uri、slot=snapshot、TargetKind 留空待 L1-2 结构化键）+ 测试 | 立即可做 | P3 / flash（零行为追加，L0-2 同型） |
| M2 | mom C 类构造器（mix.read:/acoustic_package_status:/observation:） | legacy | 迁 vit://mom 形态（scope 承载数据键族；snapshot=observation_id） | M1 后；痛点②③④根源 | P2 / flash（锚点齐） |
| M3 | A 类四包（dom/fxm/com/rlm stableProjectionID） | legacy（哈希已在手） | 生成 vit:// 带 sha256 段（内容哈希现成）；snapshot 承载按 F6 注记 | M2 后，或随 L1-2 种子需要 | P2 / flash |
| M4 | B 类身份族（obs_/ccbobs_/rel_/cap_pack_/c2_plan_） | legacy | 进 EvidenceRefs 边界处归一 | 随各域触碰机会；**capabilitycontext 批次 2026-10-10 落地（部分完结，见下方 M4 行批注）** | P3 / 机会 |
| M5 | E 类回执（semantic_eq_batch/c2.dynamic_plugin_load_batch） | legacy | 同 M4 | 随功能触碰 | P3 / 机会 |
| M6 | G 类快照族（mixboard_/kernel_prepared_+Godot 第三写者） | legacy | 族迁移+GDScript 常量导出 | **BELL/F5 链下次触碰**（ruling #7 原钩子保留） | P2 / 届时定 |
| M7 | C5/C6/C7 数据源 ref（project.state: 族/project_package.*/裸常量） | legacy | 随 L1-2 物化种子逐面升 | L1-2 各面 IMPL 卡内嵌 | 随卡 |
| M8 | Godot 写者+PCA 层勘察补腿 | 未盘点 | 两小节勘察（≤0.5h） | 机会卡或 M6 前 | P3 / 闲时可 |

已完结：内核 D 类 5/5（REFSCHEMA-D1/D2，09-28）。

> **M4 行批注（2026-10-10，REFSCHEMA-M4 执行回执回写，实现待决策验收）**：capabilitycontext 批次落地——`ccbobs_`/`ccbobs_rejected_`/`ccbobs_batch_`/`cap_pack_` 四词条注册进 legacyPrefixRegistry（15→19）：identity 族核实成立（remainder=compactID 对 obs_ 观察 id 的清洗透传、含时序戳；cap_pack_ 的 sha256 种子含 generatedAt=实例身份非内容指纹）→ slot=snapshot、TargetKind 留空，family=evidence_scheme_uri 对齐 `observation:` 先例（M4 卡承载预裁定）。实现 `port/refschema-m4@cf080a9a`（消费链=纯响应面零解析消费，capabilitycontext 生产代码零改动；回执 coord/runs/REFSCHEMA-M4/receipt.md）。
>
> **升记录缺口（ccbr_ 族待重裁）**：`ccbr_`/`ccbr_rejected_`/`ccbr_batch_`（M8 报告 D4 移交面，free_state_observation.go:567/:439/:1636）触发 M4 卡停止条件——实读 remainder 与 ccbobs_ 族同构（obs_ 身份透传，**实含时间戳+随机量**，webui trace fixture 实录 `ccbr_obs_20260911T115419_a2ef6022376c`），与"compactID=观察 id+请求 id 哈希短串、内容寻址→slot=hash"预裁定矛盾（compactID 只取首个非空输入，非哈希）。缓注册维持 opaque 透传（fail-visible 测试钉住），重裁前不入注册表；若重裁，实读证据指向与 ccbobs_ 族对齐（identity→slot=snapshot）。余量 obs_/rel_/c2_plan_ 维持"随域触碰"不变。
>
> **缺口闭合（ccbr_ 族，2026-10-10 REFSCHEMA-M4B 落地回写）**：重裁生效（[2026-10-10 MORNING-BATCH rulings §3](../../../rulings/2026-10-10-MORNING-BATCH-rulings.md)）——裁定 `ccbr_`/`ccbr_rejected_`/`ccbr_batch_` 按 identity 族承载（slot=snapshot、TargetKind 留空，family=evidence_scheme_uri），对齐 ccbobs_ 同款；M8 报告 D4 行"哈希短串"描述失准记注（历史工件不改，行为证据优先）。落地：三词条注册进 legacyPrefixRegistry（19→22）+ 钉住测试翻面（`TestRefSchemaM4CCBRFamilyStaysUnregistered` → `TestRefSchemaM4BCCBRFamilyRegistered`）+ 初值表测试扩 22；capabilitycontext 生产代码零改动（锚点测试保持）。实现 `port/refschema-m4b@a62f7125`（待决策验收合入）；回执 coord/runs/REFSCHEMA-M4B/receipt.md。capabilitycontext 域 7 生成点至此全部注册在案；M4 余量 obs_/rel_/c2_plan_ 维持"随域触碰"不变。

## 5. 解锁声明（2026-10-07 午间勘误修订）

- **L1-4 IMPL 解锁维持**（退场机制靠句柄重拉，schema 已定版）——IMPL-A 卡已入池（coord/cards/todo/2026-10-07-L1-4-IMPL-A.md）；
- ~~L1-2 物化索引 IMPL 解锁~~ **勘误（同日午间）**：早班本记录撰写时误标 L1-2 为"IMPL 待入池"——核对 done 卡池与 git 史后证实 **L1-2 已于 2026-09-29 完工**（MAT 十一卡全链；G2 门 pass ruling 2026-09-29-G2-materialization-pass.md 含 E0 附条件闭环；MAT-E 读端切换验收 pass @ 3f17f52e，裁定行 2026-10-07 已补挂回卡身）。错误成因=只读设计文档与 CURRENT-STATE 的 L1-3 视角条目、未对 done 卡池核账——与本记录 §1 所批评的 10-05 重复立卡同型，已同步修正 roadmap/CURRENT-STATE 并记 decision-log。L1-2 无解锁需要。
- L1-1 路线图状态标注已更新为"定版"；M1-M8 为 L1-1 迁移余量的排程表，不重开 L1-1。

## 6. 边界

- 本卡 docs+标注+记录产出，零生产代码改动（refschema.go 等实现属已验收的 L0-1/L0-2/D1/D2 卡）；
- F5 语义下 L1-2 不等 M2-M7；若 L1-2 实现中发现翻译条目承载力不足，回本记录升 M 序优先级而非改 schema；
- 时间归一/CAS resolve 的测试锚点在对应实现卡落定前，INV2 后半与 D3 消费侧属"规格已定、验收未布"状态（§2 缺口表）。

# 统一 Ref Schema V1（定版规格）

> **状态：现行·定版**。G1 质量门两段闭合：文法定版 ruling 2026-09-27（[coord/decisions/2026-09-27-g1-ref-schema-ruling.md](../coord/decisions/2026-09-27-g1-ref-schema-ruling.md)，八裁决）→ G1 终审定版 2026-10-07（[coord/runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md](../coord/runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md)）。本文档兑现 ruling 承诺的"完整正式规格落 docs/ 并登记现行索引"。
> **权威实现**：`agent/internal/agentprotocol/refschema.go`（Go 权威源）+ `refschema_test.go`；内核镜像 `VitApp/Source/Service/RefSchema.h/.cpp` + `VitApp/Tests/RefSchemaTests.cpp`（注释指向 Go 权威源）。文法/注册表歧义时以 Go 实现为准，镜像偏离即缺陷。
> **盘点基线**：[coord/runs/L1-1-RECON-1/EVIDENCE_REFS_INVENTORY.md](../coord/runs/L1-1-RECON-1/EVIDENCE_REFS_INVENTORY.md)（31 生成点 / 8 类 / 7 组不一致 / 4 组痛点）。
> 路线图锚点：AGENTIC_OBSERVATION_ROADMAP §2 D1/D2/D3/D15（本卡为线 1 首段 L1-1 的定版产物）。

---

## 1. 定位与边界

所有观察产物、问题、决策、变更、memory 条目（预留）共用的**全局唯一、不可变、可重放**的寻址语言。语义等价 coding 域的 `read(file@commit)`。

三条边界裁定（沿 D2/ruling，定版重申）：

1. **统一寻址语言，不统一存储引擎**。各投影层局部索引（倒排/立方体各随其便）不动；查询引擎面向 ref 谓词跨层路由（同构 grep/git/LSP 多引擎共存）。
2. **字符串层规范，不是类型替换**。消费面 `EvidenceRefs []string`（agentprotocol/audioclosure/journal/trajectory/pendingmanager 等全部）零类型改动；refschema 是生成与解析的规范层。
3. **渐进归一，不炸消费面**。legacy 引用按三态解析桥接（§9），迁移按排程推进（§11），任何时刻存量引用可被消费。

## 2. 五元组字段语义

```
ref := (projection_kind, scope_id, time_window, snapshot_id, content_hash)
```

| 五元组字段 | 文法段 | 语义 | 唯一性来源 | 不变性义务 |
|---|---|---|---|---|
| `projection_kind` | kind | 谁产出/什么对象：投影类型或对象族（§6） | 注册表（§4） | 注册后不得改义、不得复用旧名 |
| `scope_id` | `scope_kind:scope_value` | 数据面地址：寻址坐标（哪条轨/哪个频段/哪张表），不判定语义角色（D11） | 各域 ID 体系 | 拓扑版本变化走 snapshot，scope 字面量不重写 |
| `time_window` | `t=all` \| `t=s..e` | 观察覆盖的采样窗（D3 权威层，§5） | 采样点区间 | 同一观察窗一经发出不变 |
| `snapshot_id` | `@snapshot` | 数据版本代：钉住拓扑与数据状态（request_id / render_revision / observation_id / pair_id 等 legacy 身份的归一座） | 各生成域版本体系 | 代内容不可变（F5 snapshot freshness 方向） |
| `content_hash` | `#hash` | 内容身份：CAS 对接位（§7） | payload 规范序列化哈希 | hash 在场即承诺字节级重拉一致 |

## 3. 文法（BNF）与 canonical form

```
ref      := "vit://" kind "/" scope "/" window "@" snapshot "#" hash
kind     := 注册表收录的 projection_kind（charset [a-z0-9._]，未收录→三态 opaque，§9）
scope    := scope_kind ":" scope_value（段内保留字符百分号转义）
window   := "t=all" | "t=" sampleStart ".." sampleEnd   （十进制非负采样点，权威层）
snapshot := 数据版本标识（渐进期允许 legacy 值直填；空非法）
hash     := "sha256:" 16hex | "-"                        （"-" = 显式未 CAS 化）
```

ruling #2/#3 收紧（定版维持）：**window 与 hash 两段禁止省略**——全时间窗显式 `t=all`，未 CAS 化显式 `#-`。缺段即解析拒绝（隐式缺段=歧义温床）。

**保留字符与转义**：保留集 = 文法结构分隔符 `% / @ # :`。段内（scope_kind/scope_value/snapshot）出现即百分号转义（`%25 %2F %40 %23 %3A`，大写 hex）；其余字符（含空格、Unicode）逐字节直传。**解码严格闭合**：`%` 后必须两位十六进制；段内未转义保留字符按非法拒绝。**canonical 唯一性**：每结构化 ref 恰有一个合法串形态——非规范形态（未转义保留字符、小写 hex 转义、冗余转义）拒绝而不静默归一。

**采样窗边界**：十进制数字、无前导零、int64 域、`SampleStart ≤ SampleEnd` 语义由生成方保证（文法层仅拒负值与前导零）。

## 4. kind 注册表

注册表是权威清单不是代码架构（ruling #4）：Go 侧 `agentprotocol` 常量表起步，内核 C++ 侧编译期镜像。**新增 kind = 注册表追加 + 注册义注释（生成点锚点）**，不改文法。

**表一：legacy 前缀翻译条目**（`legacyPrefixRegistry`，15 条；匹配取最长前缀）：

| legacy 前缀 | 族 | TargetKind | 槽位 | 生成点锚点 |
|---|---|---|---|---|
| `dom_` | projection_content_id | dom | hash | L1-1 §2 A1 |
| `fxm_` | projection_content_id | fxm | hash | L1-1 §2 A2 |
| `com_` | projection_content_id | com | hash | L1-1 §2 A3 |
| `rlm_` | projection_content_id | rlm | hash | L1-1 §2 A4 |
| `mixboard_` | snapshot_request_id | — | snapshot | L1-1 §2 G1 |
| `kernel_prepared_` | snapshot_request_id | — | snapshot | L1-1 §2 G2 |
| `acoustic_package_status:` | evidence_scheme_uri | acp | scope | L1-1 §2 C2 |
| `dad.l3.` | evidence_scheme_uri | dad.l3 | scope | L1-1 §2 D3-D5 |
| `dad.l2_render_probe:` | evidence_scheme_uri | dad.l2_render_probe | snapshot | L1-1 §2 C4/D1 |
| `dad.compressor_dual_tap:` | evidence_scheme_uri | dad.compressor_dual_tap | snapshot | L1-1 §2 D2 |
| `dad.frequency_evidence:` | evidence_scheme_uri | dad.frequency_evidence | snapshot | L1-1 §2 C6 |
| `audio_observation:` | observation_fingerprint | — | hash | L1-1 §2 B2 |
| `observation:` | evidence_scheme_uri | — | snapshot | L1-1 §2 C3（REFSCHEMA-M1 补入 2026-10-07，00f96200；mom 观察票加头，身份族非内容哈希） |
| `fci_` / `fcp_` | observation_fingerprint | — | hash | L1-1 §2 B7 |

（族语义：A 类投影内容 ID→hash 槽；G 类快照 request_id→snapshot 槽；C/D 类 URI 式→scope 或 snapshot 槽；B 类内容指纹→hash 槽。身份族无对应 kind 时 TargetKind 留空，待 L1-2 结构化键定承载。）

**表二：无 legacy 前缀的 kind**（`projectionKindRegistry`，4 条）：`mom` / `tim` / `tom` / `epm`——四包无自生成 ID（身份=外部 observation_id，进 snapshot 段；QUERY_ENGINE §3.3 裁定）。

## 5. 时间坐标三层（D3）

| 层 | 形态 | 地位 | 归一规则 |
|---|---|---|---|
| 采样点/帧 | int64 非负 | **权威底座**：ref 内部唯一存储形态 | — |
| 秒 | float/十进制串 | 人类通用层（无网格工程默认标尺） | `samples = floor(seconds × sample_rate + 0.5)`（非负域半上取整，边界一次完成） |
| 小节:拍 | `bar:beat[.sub]` | 条件启用覆盖层 | 仅工程有可靠 tempo map 时启用；沿 tempo map 积分（拍内位置经拍号映射）；无可靠网格时**明确拒绝**，不猜测回退 |

- **采样率绑定快照**：ref 五元组不携带采样率——seconds/bar:beat 归一与 samples 语义解读都在持有快照工程状态的边界完成（snapshot 段钉住含采样率在内的数据代）。跨采样率解读 ref = 未定义行为。
- 归一是**消费边界义务**（生成侧与查询谓词侧），归一后进 ref/索引的只有采样点。查询侧三标尺输入接线归 L1-3 工具面（实现状态见 G1 终审记录）。

## 6. 对象类型轴与 memory 预留（D2）

对象类型（轨道/段落/问题/决策/变更/memory 条目）的承载裁定：

- **kind 承载"对象族"**：观察产物以投影类型为 kind（§4 两表）；问题/决策/变更等新对象族未来以新 kind 注册（命名随对象系统落地定，charset `[a-z0-9._]` 已容纳，无需改文法）。
- **memory 预留**（D14）：`memory.entry`（原始记忆流）/ `memory.distilled`（蒸馏文档）为预留命名空间——本定版只占名，不实现；收尾段拼装时注册即用，反序零返工（路线图排程依据）。
- **地址性对象不占 kind**：轨道/段落/总线/频段是**坐标**不是对象族——归 scope_kind 词汇。
- **scope_kind 词汇表 v1**（开放集+登记义务：新增须随注册表注释登记）：现役 `track` / `project` / `feature` / `band` / `pair`；现役数据键族（mom 域，scope 承载数据键——REFSCHEMA-M1/M2 落地，登记见 `agent/internal/agentprotocol/refschema.go` 注释）`mix.read` / `acoustic_package_status` / `observation`；预留 `bus` / `segment` / `send_path` / `sidechain`（D2 拓扑轴细化时启用）。
- **语义角色零判定**（D11 红线）：scope_value 用各域原生 ID；"像 hook""是低音轨"类解释永不进 ref。

## 7. 内容寻址与 CAS 对接约定

- **hash 语义**：`sha256:` + payload 规范序列化字节流的全量 sha256 的**前 16 hex（64 bit 截断）**。
- **截断理由**：v1 规模 ≤10⁵ refs（QUERY_ENGINE §3.4），生日碰撞 ≈2.7×10⁻¹⁰；ref 长度预算友好。**hash 段是地址提示与完整性抽检位，不是存储主键**——全量 digest 由物化/存储层保留。
- **resolve 契约**（物化层 L1-2 实现面）：`resolve(ref)` 成功 ⇔ 存在内容 C 使 `full_sha256(C)` 前 16 hex == ref.hash 且 C 属 ref.snapshot 代。**同一 hash 段 ref 任意时刻重拉得到字节一致内容**（CAS 语义，vsphub 既有能力方向）。
- **`#-` 语义**：显式未 CAS 化（ruling #3 渐进归一）。resolve 走各层局部索引按 (kind, scope, snapshot) 定位，**不承诺字节一致**。
- **canonical 序列化义务归生成方**：同内容同 hash 的前提是序列化稳定（Go json.Marshal 字段序稳定 / 内核 JUCE 序列化约定）。生成方变更序列化格式 = 内容身份变更（hash 必变，旧 ref 继续指向旧内容，属可接受演进）。
- **实例身份 vs 内容身份按 kind 注记**（盘点 I1 的裁定落点）：`dom`/`com` 哈希种子置空时间戳=内容身份；`fxm`/`rlm` 时间戳参与=实例身份。diff 的 Changed 判定按 kind 语义注记（QUERY_ENGINE §3.3 fxm 行先例）。

## 8. 不变式（六条，验收与反例测试的总纲）

| # | 不变式 | 内容 |
|---|---|---|
| INV1 | 不可变 | ref 字符串一经发出即冻结语义；hash 在场时其指向内容字节不变 |
| INV2 | 可重放 | `parse(format(r)) == r`（canonical 闭环）；CAS 化 ref 重拉字节一致 |
| INV3 | 全局唯一 | 五元组全局唯一标识一个可寻址单元；legacy 翻译条目使 legacy 字面量到槽位的映射唯一 |
| INV4 | canonical 唯一 | 每结构化 ref 恰有一个合法串形态；非规范形态拒绝不归一 |
| INV5 | 显式段 | window/hash 禁省略（ruling #2/#3） |
| INV6 | 渐进兼容 | 三态解析下 legacy/opaque 不阻断消费面；新生成强制 vit:// 带 hash |

## 9. 解析三态与宽容策略

| 态 | 触发 | 行为 |
|---|---|---|
| `parsed` | 合法 `vit://` 且 kind 已收录 | 结构化消费 |
| `legacy` | 命中前缀注册表 | 翻译到槽位（不重写字符串），按族语义消费 |
| `opaque` | 未收录 scheme/kind | WARN 一次 + 原样透传，**不构成错误**（ruling #8：31 生成点渐进期炸消费面不可接受） |

- WARN 走 `RefSchemaWarnLogger` 注入点（host 接线 logx；nil 保持纯函数静默）；**计数按 scheme 头聚合**——opaque 计数即注册表补全的优先级排序信号。
- 内核侧镜像同规则（RefSchema.h 常量+builder 族）。

## 10. 双端实现与测试锚点

- **Go 权威源**：`agent/internal/agentprotocol/refschema.go`（644 行：文法+两注册表+三态解析+转义闭环+WARN 记账）。测试 `refschema_test.go` 11 项：Valid 表 / Rejects 表 / 三态 / legacy 翻译表 / 注册表初值 / Format 黄金 / Format 拒绝 / 转义往返 / WARN-once 与计数 / nil 静默 / 并发计数。
- **内核镜像**：`VitApp/Source/Service/RefSchema.h/.cpp` + `VitApp/Tests/RefSchemaTests.cpp`（双配置 10/10；D 类迁移红绿与黄金样例 T1-T9，含 dual_tap 真实采样窗黄金串）。
- **生产消费面（已接线，回归义务面）**：`com`（双形态校验 evidence.go/paired.go）、`queryengine`（engine/predicates/index/bootstrap）、`materialize`（manifest/recompute）、`harness`（ref_query.go/ref_diff.go 工具面）。涉 ref 行为的改动跑对应包测试 + `go test ./... -count=1` 全量。
- **时间归一 / CAS resolve** 属消费边界义务：锚点随 L1-3 工具面接线与 L1-2 物化实现卡落（当前状态见 G1 终审记录）。

## 11. 迁移策略总则

- **渐进归一**：中央索引以 legacy 翻译条目种子（三态 legacy 态），生成点迁移后升 parsed——**迁移不是 L1-2 硬前置**，按痛点优先级机会推进。
- **新生成强制**：新代码路径发出的 evidence ref 一律 vit:// + hash（ruling #3）；存量 legacy 引用不重写（INV1）。
- **细目排程**（类×动作×触发×优先级 + 注册表缺口清单）见 [coord/runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md](../coord/runs/L1-1-REFSCHEMA-1/G1_FINAL_REVIEW.md) §4——迁移执行卡从该表取材。

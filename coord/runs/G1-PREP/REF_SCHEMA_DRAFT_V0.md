# D1 统一 Ref Schema 草案 v0（G1 评审输入，2026-09-27 决策侧起草）

> 状态：**草案 v0 骨架——非定版**。G1 质量门正式评审（决策侧亲自）以本文件为起点。事实输入：`coord/runs/L1-1-RECON-1/EVIDENCE_REFS_INVENTORY.md`（31 生成点/8 类/7 组不一致）+ L1-2 管线盘点 + 决策侧两项补查（2026-09-27）。定版后正式规格落 `docs/`并登记现行索引。

## 0. 设计约束（全部来自盘点事实，不是偏好）

| # | 约束 | 事实来源 |
|---|---|---|
| C1 | 31 生成点/8 类方案**不能一次性迁移**——schema 必须允许渐进归一（新写新格式、旧读兼容旧格式） | L1-1 §2 |
| C2 | 写者跨四种语言运行时（agent Go / harness Go / 内核 C++ / 前端 GDScript）——schema 规范形态必须语言无关，各端自行序列化；**现状靠隐式字符串约定对齐（`kernel_prepared_*` 前缀族 Go/GDScript 双写者共享、无共享常量定义）** | 补查一（telemetry_manager.gd:1963 与 harness.go:7337 同族） |
| C3 | 消费主面是 `EvidenceRefs []string`（agentprotocol:131 等 8+ 处透传）且 `Validate()` 只查非空——**第一阶段产物必须是规范字符串文法**（不是立即改结构体），结构体是第二阶段 | L1-1 §1 |
| C4 | 两个结构化孤岛是演化锚点：`audioclosure.ObservationKey`（六段：project/scope/target/views/tap/time_window）与 `processorattestation.EvidenceRef`（**全库唯一带 SHA256 的引用结构**） | L1-1 §4 + 补查二（types.go:67-73） |
| C5 | `snapshot_id` 字段名全库非测试代码零命中——**可直接启用新名，零迁移冲突**；现状由 request_id/render_revision/observation_id/pair_id 分头扮演 | L1-1 §3-I6（grep 复验） |
| C6 | vsphub CAS 既有——content_hash 的后端现成，不新建存储 | 路线图 D1 |

## 1. 草案核心：三层 schema，分阶段落地

- **L0 规范字符串文法**（v0 定版对象）：全库 ref 字面量的统一格式——所有生成点的目标格式，`[]string` 消费面零改动即可承接；
- **L1 结构化键**（v1 演化对象）：ObservationKey 的扩展（补 projection_kind/snapshot_id/content_hash 段），生成侧内部使用、序列化为 L0 输出；
- **前缀注册表**：8 类 legacy 格式的登记处（legacy 格式字面量→所属类→迁移路径→目标格式），**新旧并存期的翻译层**（解析时 legacy 格式按注册表识别，不 fail）。

## 2. L0 文法草案（两案并列，G1 裁决点①）

```
案 A（URI 风格）：
ref := "vit://" kind "/" scope [ "/" window ] "@" snapshot [ "#" hash ]

案 B（定界符串）：
ref := kind ":" scope ":" window ":" snapshot ":" hash
```

- 两案共同：五元组各一段；段省略必须**文法显式**（`@-` / 空段 `::`），禁止隐式吞段——"省略"本身是信息（未 CAS 化的旧产物无 hash、全曲观察无 window）；
- 案 A 优势：与既有 legacy 前缀（`dad.l3.` / `dom_` / `rlm_`）零冲突（`vit://` 开头即新版）；案 B 优势：更短、与 ObservationKey 字段序直译；
- 分隔符冲突核验（L1-1 §3-I5 三方案在用）：两案的段内禁止出现 `:` `/` `@` `#`（scope 含轨道路径时先做百分号转义——转义规则入文法定义）。

## 3. 五元组逐字段 v0 建议

| 字段 | v0 建议 | 依据 |
|---|---|---|
| projection_kind | 以前缀注册表为种子（`dad.l3`/`dom`/`rlm`…4 包现成），未注册 kind 解析 fail-closed；**注册表是权威清单，不在文法里硬编码 kind 枚举** | I1（仅 4 包有 kind 段）+ 可增长性 |
| scope_id | 序列化 `scope_kind:scope_value`（如 `track:1007`/`bus:master`）；规范结构=ObservationKey.Scope（Kind+ID）——唯一结构化现物 | I3（轨道 ID 三种位置） |
| time_window | **权威层采样点**（D3 裁定）：`t=<sampleStart>..<sampleEnd>`；全曲=显式 `t=all`；秒/小节标尺是查询语言层的输入转换，不进 ref | D3 + I2（时间窗四态） |
| snapshot_id | 统一新名（C5 零冲突）；语义=钉数据版本；v0 允许 legacy 值直填（request_id 等按原值入段），注册表记其族属 | I6 + I4（snapshot 身份四载体） |
| content_hash | `sha256:<16hex>`（短哈希，CAS 全哈希为后端映射）；**新生成强制、旧引用可无（显式空段）**；裁决点③ | C6 + I7（哈希有无两态） |

## 4. 迁移路径草图（8 类各自一行，G1 评审细化）

| 类 | 迁移触发 | 步骤 |
|---|---|---|
| A 投影 ID 四变体 | 各投影物化改造时（L1-2 段） | stableProjectionID 输出改 L0 格式（前缀→注册表 kind） |
| B 观察/会话身份 ID | 新 harness 骨架（L1-5） | 身份 ID 与 ref 分离：ID 留原位，引用改 L0 |
| C URI 字面量族 | 同 A | 字面量直接按注册表翻译 |
| D 内核 C++ 5 处 | 内核腿排卡（TIM-GAPS 报告输入） | C++ 侧 L0 序列化函数+注册表常量镜像 |
| E 执行回执 | receipt 结构下次触碰时 | ReceiptID 入 ref 的 snapshot 段 |
| F CCB view 键 | view 参数化（L1-3） | view 键保持，披露的 refs 走 L0 |
| G request_id 族（含 GDScript 第三写者） | F5/BELL 链下次触碰 | 前缀注册表登记+前端常量表导出共享（C2 的解） |
| H ObservationKey | L1 结构化键演化（本 schema L1 层） | Key 扩段，序列化输出 L0 |

## 5. G1 正式评审的裁决清单

1. 案 A vs 案 B（文法形态）；
2. time_window 省略语义：显式 `t=all` vs 允许整段省略；
3. content_hash 强制等级：新生成强制（建议）vs 全部强制 vs 全部可选；
4. 前缀注册表归属：agentprotocol 常量表 vs 独立 `refschema` 包 vs schema 文档即权威；
5. L1 结构化键：演化 ObservationKey（建议）vs 新建；
6. 内核 C++ 5 处迁移排期（与 TIM 内核腿并卡 or 独立卡）；
7. GDScript 写者的 schema 共享机制（常量导出 vs 文档契约）；
8. 渐进期判定：注册表未收录的 legacy ref——宽容解析+告警（建议）vs fail-closed。

## 6. 补查记录（2026-09-27 决策侧亲读）

- 前端第三写者：`telemetry_manager.gd:1963` `kernel_prepared_waveform_envelope_%s`——与 agent 侧 harness.go:7337 判族前缀同源，**跨语言共享隐式约定、无共享常量定义**（C2 的实证）；`mixboard_feature_request.v1` schema_version 存在但仅前端单方使用；
- PCA 层：`processorattestation/types.go:67-73` EvidenceRef{ReceiptID/Kind/SHA256/ObservedAt/CorpusRecord}——全库唯一结构化带哈希引用（C4）；pluginsemantics index 的 `Source: "plugin_list_available"`（index.go:153）是数据源标注不是 ref，不人册。

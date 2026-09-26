# 决策记录：G1 质量门——D1 统一 Ref Schema 裁决定版（2026-09-27，决策侧亲自）

G1 是路线图四质量门之首（"ref schema 定版失误→全线返工"），评审由决策侧亲自执行。输入：`coord/runs/G1-PREP/REF_SCHEMA_DRAFT_V0.md`（草案 v0+两项补查）+五份盘点报告。八裁决点逐条裁定如下；完整正式规格（文法 BNF+注册表初值全集）由实现卡 `refschema-L0-1` 产出后落 `docs/` 并登记现行索引。

## 八裁决

| # | 裁决点 | 裁定 | 依据 |
|---|---|---|---|
| 1 | 文法形态 | **案 A（URI：`vit://`）** | 与 31 处 legacy 前缀（`dad.l3.`/`dom_`/`rlm_` 等）零冲突是硬优势——案 B 冒号段的转义面更大（scope 含路径、legacy 值含冒号）；长度代价由机器消费面消化 |
| 2 | time_window 省略 | **显式 `t=all`，禁止整段省略** | "省略必须是文法显式"的一致性原则；隐式缺段=解析歧义温床（L1-1 盘点 I2 时间窗四态就是缺显式规范的结果） |
| 3 | content_hash 强制等级 | **新生成强制、旧引用显式空段（`#-`）** | 渐进归一（C1 约束）的唯一自洽解：新的从第一天带哈希，旧的显式标注未 CAS 化而不是假装有 |
| 4 | 前缀注册表归属 | **agentprotocol 常量表起步**（独立 `refschema` 包留 L1 结构化键阶段再评估） | 最小面启动；注册表是权威清单不是代码架构，先挂最通用的协议包 |
| 5 | L1 结构化键 | **演化 `audioclosure.ObservationKey`**（扩段），不新建 | C4 锚点：六段现成结构+全库唯一；新建=两套真相 |
| 6 | 内核 C++ 5 处（D 类）迁移 | **独立小卡，排在 refschema-L0-1 落地之后** | 不与内核腿新功能混卡（保持一张卡一个可回滚逻辑变更）；迁移依赖 L0 文法先在 |
| 7 | GDScript 写者共享 | **文档契约先行；常量导出随 G 类迁移做** | 前端写者只有一个族（request_id 前缀族），为它先建导出机制过度；G 类迁移（BELL/F5 链下次触碰）时一并做 |
| 8 | 未收录 legacy ref | **宽容解析+WARN 一次（opaque 透传）** | 渐进期炸消费面不可接受（31 生成点的生产链路）；WARN 计数供注册表补全的优先级排序 |

## L0 文法定版要点（实现卡规格基线）

```
ref      := "vit://" kind "/" scope [ "/" window ] "@" snapshot [ "#" hash ]
kind     := 前缀注册表收录的 projection_kind（未收录→宽容解析规则 #8）
scope    := scope_kind ":" scope_value     （段内保留字符百分号转义）
window   := "t=all" | "t=" sampleStart ".." sampleEnd    （采样点权威层，D3）
snapshot := 数据版本标识（v0 允许 legacy 值直填：request_id/render_revision 等）
hash     := "sha256:" 16hex | "-"           （"-"=显式未 CAS 化）
```

- 注册表初值来源：L1-1 报告 §2 的 A 类四变体 kind（`dom`/`mom`/`rlm`/`dad.l3` 等）+ G 类前缀族登记；
- 解析器三态：`parsed(valid) / legacy(registered，可翻译) / opaque(未收录，WARN+透传)`；
- §11 兼容：`EvidenceRefs []string` 消费面零改动——L0 是字符串层的规范，不是类型替换。

## 后续排程钩子

- `refschema-L0-1`（文法+注册表+解析器，agent 侧纯新增+测试门）→ 入夜池；
- D 类内核 5 处迁移卡、G 类前端族迁移（含常量导出）分别排在 L0 落地后、BELL/F5 链下次触碰时；
- L1 结构化键（ObservationKey 扩段）排 L1-2 物化段开工时。

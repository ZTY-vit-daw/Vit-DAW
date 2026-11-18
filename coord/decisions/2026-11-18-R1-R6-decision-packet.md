# G0-LEGAME-CONTRACT-1：R1–R6 用户裁定单（2026-11-18 整理，待讨论）

- 来源：`docs/LEGAME_DESIGN_CONTRACT_V0.md` §7（370 G0-LEGAME-CONTRACT-1 产出，2026-11-15 冻结草案）。
- 状态：**R1–R6 仍未裁定**（11-17 gate 记录“R1-R6 裁定单仍候用户（W01 前置）”）；裁定回填契约 §10 后契约冻结，才能发 W01 实施卡（三张草案见 `coord/runs/G0-LEGAME-CONTRACT-1/impl-cards-draft.md`）。
- 影响：W01（最小设计源格式与工程绑定）是展演教学运行时（W02/W19 等）的前置；这是目前展演线最硬的未开工块。

## 逐项（草案立场仅参考，不是裁定）

| # | 事项 | 草案立场 | 可选项 | 我的建议 | 影响面 |
|---|---|---|---|---|---|
| R1 | 非乐局意图文档的 schema_version 保留值（`vit.project_intent.v0`）与 L0 字段集是否=乐局字段子集 | 保留该值；L0=子集 | A. 按草案保留+子集；B. 暂不保留非乐局格式（只有乐局）；C. 其他值/字段集 | **A**：保留值+子集，成本最低，不阻塞乐局；非乐局注入本就是 M2+，不占关键路径 | M2+ 非乐局链路 |
| R2 | 判层机制=仅 schema_version；“字段全可选”是否不含 schema_version 本身 | 是（工程必需解读） | A. 按草案（schema_version 必填、其余全可选）；B. 增加 kind/type 等第二判层键 | **A**：单一判层键最简单，fail-closed 处理未知 schema_version；需要更多文档类型时再扩 | 解析器主入口 |
| R3 | design.tracks→runtime_slots 的 track_index 指派方式 | 声明序 1..n + manifest 记录映射 | A. 序号指派（草案）；B. 设计源新增显式 index 字段；C. 混合（有 index 用 index，否则序号） | **A**：少一个字段、专家手写成本低；映射记录在 manifest，冲突可诊断。显式 index 可作后续 schema 扩展 | 编译器 T-12；内核轨序 |
| R4 | objectives 多 evidence_checks → 卡单 check 的映射 | 拆分为多卡目标 id#k（保全证据钩） | A. 拆分（草案）；B. 选主检查+余项 advisory；C. 合并为一条汇总检查 | **A**：每条证据检查保留独立目标/判定，W02 每轮注入与报告可回指；卡数增加但可控 | 编译器 T-15；W02 证据注入 |
| R5 | budget/tempo 空值编译策略 | 覆盖参数必填否则 E（fail-closed） | A. 按草案 E（必须显式覆盖参数）；B. 固定缺省注入表（tempo/budget 默认值）；C. tempo E、budget 默认 | **A**：不静默替用户/设计者定速度与预算；缺项用诊断 E 暴露，符合“不伪造”原则。真需要默认值时由设计源显式写 | 编译器 T-8/T-20/T-18 |
| R6 | key_signature 词表封闭度；locator.music_region 单/复值归一；材料核验状态是否进设计源 schema | 三小项草案见下 | R6a 词表：A 自由 token `^[A-G][#b]?$` / B 15 大调封闭枚举 / C 含小调；R6b music_region：A 接受单/复两态编译期归一为 list / B 只接受 list；R6c 核验状态：A 不提升（留 catalog）/ B 提升进 schema | **R6a=B（15 大调封闭枚举）**：本课材料只用 F/C/G/D 等大调，封闭词表能 fail-closed 防拼写错误/幽灵调；小调未来如需要单独扩枚举。**R6b=A**：接受两态归一 list，作者手写宽容、编译期统一。**R6c=A**：核验状态属材料目录/catalog 事实，不混入设计源 schema（避免设计源依赖材料审计进度） | §4.6 timing；§4.4 locator；§6.3 装配 |

## 非裁定的解释备案（主管可自行决定，用户可否决）

1. §15 “regions” 视为 workspace.regions，不提升顶层字段（草案）。
2. 卡 kind 映射表 T-12 的 `assist_generated → blank_to_fill` 为提案值；W01-C 落实时若语义冲突，回契约文档修订。

## 裁定回填后的动作

1. 把结果写回 `docs/LEGAME_DESIGN_CONTRACT_V0.md` §7/§10，契约冻结生效；
2. 按 `coord/runs/G0-LEGAME-CONTRACT-1/impl-cards-draft.md` 发 W01 三张实施卡（源 schema/编译映射/工程绑定与缓存失效）；
3. W01 首版子范围优先（非乐局/完整迁移路径月后可收），与 W02/W19 依赖串行。

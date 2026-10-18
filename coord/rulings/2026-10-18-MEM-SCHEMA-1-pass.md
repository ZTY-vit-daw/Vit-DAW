# 2026-10-18 MEM-SCHEMA-1 设计评审裁定（pass）

- 验收会话：12:00-18:00 决策流（DSH PC，本机）；裁定来源=用户 2026-10-18「可以出卡，放入池中」授权出卡（[decisions/2026-10-18-plugin-memory-and-boundary-cards.md](../decisions/2026-10-18-plugin-memory-and-boundary-cards.md)）+ 卡面验收负责人=主管决策流（PC）。
- 交付：分支 **port/mem-schema-1@b6ce75d8**（8 个新增文件：`docs/PLUGIN_MEMORY_SCHEMA_V1.md` 1502 行 + `coord/runs/MEM-SCHEMA-1/` 7 项）；已 cherry-pick 入 main=**c6de193e**（本裁定提交后 main=__MAIN_AFTER__）。
- 执行侧：DSH 团队执行流 exec-memschema（task-7）；协调面由 12:00-18:00 决策流单写。

## 一、决策侧评审（亲做）

1. **范围/域界亲核**：提交仅 `docs/PLUGIN_MEMORY_SCHEMA_V1.md` + `coord/runs/MEM-SCHEMA-1/`；`agent/|VitApp/|scripts/` 路径 **零命中**；worktree clean；未 push main、未写共享主工作树。✅
2. **卡面目标 8 点逐项亲读**：
   - §1 定位与键：D14 三记忆三键表 + `env_key = env1_<16hex>` 公式 + 指纹失配→降权待复验/不跨设备/不删不迁移 + 人键/工程键分离禁混用；✅
   - §2 六条目类型逐类「允许字段/禁止内容/trust 允许」+ 类型×trust 矩阵 + 通用禁可执行载荷；✅
   - §3 16 字段总表 + `entry_ref` 对齐 REF_SCHEMA §6 `memory.entry`/`memory.distilled` + trust 五态语义与迁移 + 证据 ref 文法 + 版本封闭集 + raw/物化两层语义；✅
   - §4 写入策略：T1–T4 capture 触发 + 反触发 / W1–W4 写者与 trust 上限 / 矛盾保留四规则（Q1↔C4 冲突示例）/ 过期合并字段（执行细节留 D14）/ 禁止写入清单；✅
   - §5 消费契约非权威：允许面（planner/LLM 上下文+只读工具）+ 红线逐条（非执行权威/非参数映射来源/非 credential/不产生可调度路径/不复活 VPS-Profile-SPAL）+ executor 五项 fresh inspect 义务 + live 证据优先冲突表 + A1–A7 测试断言建议（A2=notes 不得新增可调用控制路径）；✅
   - §6 13 条样例（Pro-Q 3×5 / Pro-C 2×5 / ValhallaSupermassive×3），全部带 2–3 条 `evidence_refs`，覆盖写序、`Mix` 0–200%、非有限端点、通道/立体声联动、unsupported 边界、矛盾保留；✅
   - §7 既有资产关系：ADR 逐项不复活 VPS/Profile/SPAL 执行权威；与 `COMPRESSOR_REFERENCE_PRO_C_2.md` human reference 区别；无可执行映射/alias/credential 三重保障；✅
   - §8 D14 接入点：raw append-only vs distilled markdown 职责表、查询/consolidate/画像归 D14、本卡只定义 schema/policy、开放项 O1–O8；✅
3. **自校验亲核**：`VALIDATION.txt` 13/13 entries PASS（必填字段/枚举与 trust 允许集/confidence 域/evidence 词法/env_key 可重算/content 白名单/禁可执行载荷）；WARN 项=样例身份字段缺失且已标 `[待用户确认]`（诚实，未编造）；`SELF_CHECK_8POINTS.md` 8 点自查在卷。✅
4. **待用户确认 5 类**（Pro-C 2 identity/version 与原始测量工件缺失、ValhallaSupermassive version、Valhalla dry/mix/wet 绑定、外部报告 Used/Enabled 先写与实测 activation-last 冲突并存）均写入条目 limitations/EVIDENCE_INDEX，未伪造结论。✅
5. **第三样例 ValhallaSupermassive 替代 Valhalla Plate**：卡面允许「或同族 reverb」；理由=本机无 Plate 参数面工件、Supermassive 有真实 inspect/surface 证据；未编造——**采信**。
6. **停止条件核查**：均未触发（无 D14/REF_SCHEMA 前提冲突——仅使用 §6 预留命名空间，`scope_kind`/`entry_env` 候选按开放集+登记义务列 D14 O1；未实现存储/检索/巩固；无法核实证据已标 `[待用户确认]`；无「notes 参与执行判定」设计，§2.3/§5.2/§5.3 反向写死并引 ADR）。✅

## 二、评审结论

- **设计评审 pass**。文档状态由「待决策侧设计评审」更新为「决策侧评审 pass（2026-10-18）」；分支 cherry-pick 入 main；卡 135 归 done。
- 评审意见（不阻塞 pass，落实于后续实现/D14 卡）：①`entry_env`/`scope_kind` 候选名待 D14 登记（开放项 O1）；②样例 Pro-Q 3 capture 为 VPS 退役前工件，后续实现引用时须复核不引用 profile/skill/virtual_controls 残留（EVIDENCE_INDEX 已声明）；③本卡只是 spec——存储/检索/巩固/画像归 D14 排程，不得据此扩 runtime 或执行权威。
- 边界：本卡纯文档域、零真栈、零生产代码、零 runtime；手测不适用；5 类 `[待用户确认]` 项由用户在后续实现/D14 卡时确认。

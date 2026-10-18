# MEM-SCHEMA-1 卡面 8 点覆盖与验收自查

- 文档：`docs/PLUGIN_MEMORY_SCHEMA_V1.md`（唯一生产文档；本页 + `EVIDENCE_INDEX.md` + 样例 JSONL/校验报告为辅助工件）
- 基线：`origin/main = a99117a0`；分支 `port/mem-schema-1`；worktree `D:/Vit_DAW_wt_memschema1`
- 自查时间：2026-10-18

## 1. 卡面「目标」8 点逐项覆盖

| # | 卡面要求 | 文档位置 | 覆盖证据 |
|---|---|---|---|
| 1 | 定位与键：D14 三种记忆之「客观经验」类；聚合键=环境（机器+插件 identity/version/format+参数面指纹）；指纹失配→降权待复验、不跨设备同步；用户偏好/工程事实分走人/工程键 | §1.1–§1.5（含环境键公式、失配语义、混合禁区） | D14 三键表（§1.1）；`env_key = env1_<16hex>` 可重算（§1.2/§3.6）；失配→降权待复验四条（§1.3）；偏好/工程事实边界（§1.4） |
| 2 | 条目类型：`semantic_hint` / `gotcha` / `write_order_precondition` / `user_approved_setting` / `ambiguity_resolution` / `unsupported_boundary`；逐类允许字段+禁止内容 | §2.1–§2.7（六类小节 + 类型×trust 矩阵） | 每类四段：用途/允许字段/禁止内容/trust 允许；§2.0 通用禁止；§2.7 矩阵 |
| 3 | 字段：`schema_version`、`memory.entry` ref、trust 五态、confidence、evidence_count、evidence_refs、context_tags、created/updated/expires、plugin_identity+surface_fingerprint、content、limitations | §3.1–§3.11 + 附录 A JSON Schema | 16 字段总表（§3.1）；`entry_ref` 模板与 REF_SCHEMA §6 对齐（§3.2）；trust 五态语义/迁移（§3.3）；证据文法（§3.4）；时间/过期（§3.7）；封闭集与版本（§3.10） |
| 4 | 写入策略：capture 触发（用户纠正/失败重试/实测 gotcha/用户认可设置）、谁可写、矛盾保留（不覆盖、并存带上下文）、过期/合并字段（执行细节留 D14） | §4.1–§4.6 | T1–T4 表 + 反触发（§4.1）；writer 类与 trust 上限（§4.2）；冲突保留四规则 + Q1/C4 冲突示例（§4.3）；过期/合并字段行为 + "执行细节留 D14"（§4.4/§3.11）；禁止写入清单（§4.6） |
| 5 | 消费契约（非权威）：只进 planner/LLM 上下文或只读工具；不是执行权威/不是参数映射来源/不是 credential/不产生可调度路径；typed executor 必须 fresh inspect 重新推导；冲突时 live 证据优先；附「notes 不得新增可调用控制路径」测试断言建议 | §5.1–§5.7 | 允许面（§5.1）；红线逐条对照 ADR（§5.2）；executor 五项义务（§5.3）；冲突规则表（§5.4）；A1–A7 断言建议（§5.6，A2 即"notes 不得新增可调用控制路径"） |
| 6 | 样例：Pro-Q 3 / Pro-C 2 / Valhalla Plate（或同族 reverb）各 ≥2 条、标 evidence_refs；覆盖写序、`Mix` 0–200%、非有限端点、通道/立体声联动等真实 gotcha 至少各一例；证据不能核实→标 `[待用户确认]`，不得编造 | §6.1–§6.3 + `SAMPLE_ENTRIES.jsonl` / `EVIDENCE_INDEX.md` | 13 条（5/5/3）；覆盖表 §6.3；每条带 evidence_refs（2–3 条）；C1/C2/C4/V3 及 Pro-C 2/Valhalla 身份缺口显式 `[待用户确认]`（§5/EVIDENCE_INDEX §5/§6） |
| 7 | 与既有资产关系：明确不复用/不复活 VPS/Profile/SPAL 执行权威；与 `COMPRESSOR_REFERENCE_PRO_C_2.md`（human reference，非运行时）区别；不写可执行参数映射/alias/credential | §7.1–§7.6 | ADR 逐项对照 + 退役工具测试锚点（§7.1）；human reference vs entry 对比表（§7.2）；context pack weak hint 关系（§7.3）；三重保障（§7.6） |
| 8 | D14 接入点：raw append-only stream vs distilled markdown 职责；查询/consolidate/画像由 D14 提供；本卡只定义域内 schema/policy 与未来接入约定 | §8.1–§8.5 | 职责划分表（§8.1）；接入约定 6 条（§8.2）；明确不做（§8.3）；开放项 O1–O8（§8.4）；升级/停止条件（§8.5） |

## 2. 卡面「验收标准」自查

| 验收项 | 结果 | 证据 |
|---|---|---|
| 文档存在且逐项覆盖 §目标 8 点 | ✅ PASS | 上表 8/8；文档 `docs/PLUGIN_MEMORY_SCHEMA_V1.md`（约 1500 行） |
| 三个插件样例 ≥6 条 entries | ✅ PASS（13 条：5/5/3；第三件为卡面允许的「同族 reverb」ValhallaSupermassive，见文档 §6.1 说明） | §6.2.1 内联 + `SAMPLE_ENTRIES.jsonl` 13 行 |
| 字段与声明 schema 一致 | ✅ PASS | `VALIDATION.txt`：13/13 entries PASS；含必填字段、枚举、content 白名单、entry_ref 摘要自洽、env_key 可重算 |
| `evidence_refs` 可回指（或明确标 `[待用户确认]`） | ✅ PASS | tracked 证据 @ `a99117a0`（blob hash 表）；workspace 证据带 SHA256；缺失项标 `[待用户确认]`（`EVIDENCE_INDEX.md` §2/§3/§5） |
| 显式给出 D14/REF_SCHEMA 对齐 | ✅ PASS | §1.1（D14 三键）、§3.2（`memory.entry`/`memory.distilled` 预留、scope_kind 登记义务）、§8 |
| 显式 non-authoritative 边界 + 冲突规则 + 测试断言建议 | ✅ PASS | §5.2 红线逐条、§5.4 冲突表、§5.6 A1–A7 |
| 决策侧设计评审 pass | ⏳ 待评审 | 本卡交付后由主管决策流评审 |
| `git diff --stat` 仅 docs/coord，零生产代码 | ✅ PASS | 见回执 `git status --short` / `git show --stat`（仅 `docs/PLUGIN_MEMORY_SCHEMA_V1.md` + `coord/runs/MEM-SCHEMA-1/`） |

## 3. 停止条件核查（均未触发）

| 停止条件 | 核查结果 |
|---|---|
| D14/REF_SCHEMA 前提冲突（memory 对象类型轴/命名空间被改） | **无冲突**：只使用 §6 已预留 `memory.entry`/`memory.distilled` 命名空间；新增 `scope_kind` 候选按"开放集+登记义务"登记为 D14 开放项（O1），未改文法、未改对象类型轴 |
| 要求本卡顺便实现存储/检索/巩固 | **未要求、未实现**：§8.3 明确不做；`coord/runs/MEM-SCHEMA-1/make_samples.py` 仅为设计期样例生成/校验工具，不进生产链路 |
| 样例证据无法核实 → 标 `[待用户确认]`，不得编造 | **已执行**：Pro-C 2 identity/version/原始工件缺失、Valhalla version、Valhalla dry/mix/wet 绑定、C4 外部写序报告共 5 类显式标注（`EVIDENCE_INDEX.md` §6）；无编造证据 |
| 任何让 notes 参与执行判定的设计要求 | **无此要求**；文档 §2.3/§5.2/§5.3 反向写死"memory 不参与执行判定"，并引用 `SPAL_VPS_RETIREMENT_ADR.md`；A2 断言专门保障 |

## 4. 端测边界声明

- 本卡为**纯文档域**（`docs/PLUGIN_MEMORY_SCHEMA_V1.md` + `coord/runs/MEM-SCHEMA-1/`）：**零真栈、零生产代码、零 runtime 接线、零持久化 store**。
- 不适用 AGENTS §5"agent 侧改动的端侧烟测"门槛（无 agent 行为改动）；不占用 `coord/resources/PC-RUNTIME-STACK.md`。
- 样例证据的可核实性按 `EVIDENCE_INDEX.md` 逐条声明；schema 一致性由 `make_samples.py` + `VALIDATION.txt` 提供机器校验（可复现）。
- 未修改：`agent/`、`VitApp/`、`scripts/`、共享主工作树 `D:/Vit_DAW`、main 分支；未 push main。

## 5. 留给后续卡的量

| 后续卡 | 内容 | 依赖 |
|---|---|---|
| D14 落地卡 | kind/scope_kind 注册、raw stream envelope、存储/检索/失效/consolidation/画像 | D14 排程（收尾拼装） |
| 消费接线卡 | planner 上下文召回 + 只读 memory 工具 + A1–A7 测试断言落地 | D14 落地后 |
| 证据复验 | Pro-C 2 real inspect、Valhalla binding、C4 外部报告复现 | 真栈/插件环境就绪 + 用户在场 |

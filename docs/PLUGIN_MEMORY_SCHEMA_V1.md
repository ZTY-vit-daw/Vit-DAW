# 插件域 Memory Entry Schema 与 Advisory 策略 V1

> **状态：设计件（spec-only，决策侧设计评审 pass：2026-10-18，[rulings/2026-10-18-MEM-SCHEMA-1-pass.md](../coord/rulings/2026-10-18-MEM-SCHEMA-1-pass.md)）**。日期 2026-10-18。卡：MEM-SCHEMA-1（池序 135，P2，L2 设计件；owner=DSH PC 执行流 exec-memschema）。授权：用户 2026-10-18「可以出卡，放入池中」明确授权（授权记录见 [decisions/2026-10-18-plugin-memory-and-boundary-cards.md](../coord/decisions/2026-10-18-plugin-memory-and-boundary-cards.md)）。
>
> **定位**：D14 Memory 系统的**插件域（客观经验类）前置规格**——只定义 entry schema、写入策略与消费边界；**存储、检索、失效、consolidation、画像机制归 [`AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md`](AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md) D14**。本文**不建 store、不写 Go 代码、不接 runtime、不改变 PCA/typed executor 准入门槛**。
>
> **依据**：[`REF_SCHEMA_V1.md`](REF_SCHEMA_V1.md) §6（`memory.entry` / `memory.distilled` 预留命名空间）；[`AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md`](AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md) D14（三种记忆三种键、raw append-only + 蒸馏文档、画像数学）；[`SPAL_VPS_RETIREMENT_ADR.md`](SPAL_VPS_RETIREMENT_ADR.md)；[`COMPRESSOR_REFERENCE_PRO_C_2.md`](COMPRESSOR_REFERENCE_PRO_C_2.md)；[`AUDITION_AB_AND_USER_JUDGMENT_V1.md`](AUDITION_AB_AND_USER_JUDGMENT_V1.md) §11；调研结论 `coord/reports/2026-10-18-reaper-mcp-plugin-param-research.md` §5.3/§5.4（PC 工作区、**未入库**；见 §6 证据状态）。
>
> **样例与证据工件**：[`coord/runs/MEM-SCHEMA-1/`](../coord/runs/MEM-SCHEMA-1/)（13 条样例 entries、证据索引、schema 自校验 PASS 报告）。本文只是设计规格，不产生任何执行权威。

---

## 0. 一页摘要（TL;DR）

- 本文定义**插件域客观经验 memory**（D14 三种记忆之一）的 entry schema；聚合键是**环境**（机器 + 插件 identity/version/format + 参数面指纹），环境条目**机器本地、不跨设备同步**（§1）。
- **六种条目类型**：`semantic_hint` / `gotcha` / `write_order_precondition` / `user_approved_setting` / `ambiguity_resolution` / `unsupported_boundary`，逐类字段白名单与禁止内容见 §2。
- **字段**：`schema_version`、`entry_type`、`memory.entry` ref、`environment`（机器+env 键）、`trust` 五态、`confidence`、`evidence_count`、`evidence_refs`、`context_tags`、`created_at/updated_at/expires_at`、`plugin_identity`、`surface_fingerprint`、`content`、`limitations`（§3）。
- **trust 五态**：`observed` / `user-confirmed` / `agent-inferred` / `conformed` / `unsupported`；可信度只用于 advisory 权重，**不是执行门**（§3.3）。
- **写入**：capture 触发=用户纠正 / 失败重试 / 实测 gotcha / 用户认可设置；矛盾**保留不覆盖**；过期/合并字段就位、执行细节归 D14（§4）。
- **消费契约（非权威）**：只进 planner/LLM 上下文或只读工具；**不是执行权威、不是参数映射来源、不是 credential、不产生可调度路径**；typed executor 必须 fresh inspect 重新推导；**冲突时 live 证据优先**（§5）。
- **测试断言建议**：不新增可调用控制路径、memory 不进 executor、live 优先、指纹失配降权等 7 条，供后续实现卡（§5.6）。
- **样例**：Pro-Q 3 / Pro-C 2 / ValhallaSupermassive 共 13 条，全部标 `evidence_refs`；覆盖写序、`Mix` 0–200%、非有限端点、通道/立体声联动、unsupported 边界、矛盾保留等真实 gotcha，可核实性逐条标注（§6）。
- **与既有资产关系**：不复活 VPS/Profile/SPAL 执行权威；与 `COMPRESSOR_REFERENCE_PRO_C_2.md`（human reference，非运行时）定位不同；不写可执行参数映射/alias/credential（§7）。
- **D14 接入点**：raw append-only stream（`memory.entry`）vs distilled markdown（`memory.distilled`）职责分离；查询/consolidate/画像由 D14 提供；本卡只定义域内 schema/policy 与接入约定（§8）。

---

## 1. 定位、三类记忆与聚合键

### 1.1 三类记忆、三种键（D14）

[`AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md`](AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md) D14（`L147-L162`）把记忆钉成三类三键；**本文件只覆盖其中「客观经验」类**：

| 记忆类型 | 聚合键 | 协议 | 本文范围 |
|---|---|---|---|
| 用户偏好 | 按「人」（跨设备同步） | 质询确权（D13） | 否（归 D13 + D14 用户偏好类） |
| **客观经验（插件 bug、流程踩坑）** | **按「环境」（机器+版本/参数面指纹；`.local` 同构，不跨设备同步）** | 环境指纹失配 → 降权待复验 → 复验通过才退役留痕 | **是** |
| 工程事实 | 按「工程」（工程账本） | append-only | 否（工程账本） |

- 「换台机器就不灵」类故障的根因就是键错位；本 schema 用**环境键**而不是插件名/产品名聚合（同一产品在不同机器、版本、参数面指纹下是不同环境）。
- 条目本身是坐标集对象类型轴上的对象（`REF_SCHEMA_V1.md` §6 D2 预留）；**按需检索，不全文塞前缀**（D14 收尾拼装约定）。

### 1.2 环境聚合键（environment key）

```text
plugin_identity    = {vendor, product, plugin_identifier, plugin_format, plugin_version}
surface_fingerprint = {tool, generation, param_surface_digest, captured_at}
environment        = {machine_key, env_key}

env_key = "env1_" + hex16(sha256(canonical_json({machine_key, plugin_identity, surface_fingerprint})))
```

| 组成 | 语义 | 约束 |
|---|---|---|
| `machine_key` | 机器本地不透明指纹（如 `m_<16hex>`） | 稳定、不可逆；不携带用户名/序列号/凭证；**只在机器本地存储，不进同步面** |
| `plugin_identity.vendor/product` | 厂商与产品名（证据标签） | 只用于分组检索，**不作为拓扑/识别证据**（沿 `COMPRESSOR_CONTROL_V1.md` §Evidence Boundary 口径） |
| `plugin_identity.plugin_identifier` | VST3 identifier（例：`VST3-Pro-Q 3-a1aba170-ba5f7ab2`） | 生产捕获必填；设计样例允许 null 且必须标 `[待用户确认]` |
| `plugin_identity.plugin_format` | `VST3` 等格式 | 格式不同即不同环境 |
| `plugin_identity.plugin_version` | live 发现的版本（例：`3.2.3.0`） | 版本不同即不同环境；未知时标 `[待用户确认]` |
| `surface_fingerprint.tool` | 产生指纹的 inspect 工具（`plugin_grabber.inspect_eq` 等） | 工具不同即不同参数面语义 |
| `surface_fingerprint.generation` | inspect 返回的实例级 topology generation（例：`eqt1_1c011654…`） | **实例级、每次 inspect 可能变化**；不得跨会话当权威 |
| `surface_fingerprint.param_surface_digest` | canonical 参数面摘要（`sha256:<16hex>` 或显式未 CAS 的 `sha256:-`） | 生产捕获必填；设计样例可用 `sha256:-`（对齐 REF_SCHEMA `#-` 渐进语义） |
| `surface_fingerprint.captured_at` | 指纹采集时间 | 生产必填；设计样例允许 null+`[待用户确认]` |

- **参数面摘要口径（设计期约定，D14 可升级）**：对参数面 `param_id|name|display domain|discrete|enum labels` 排序后做 UTF-8 canonical 串的 sha256 截 16 hex；示例计算见 [`coord/runs/MEM-SCHEMA-1/make_samples.py`](../coord/runs/MEM-SCHEMA-1/make_samples.py)（非生产代码）。
- **generation 与 digest 同等重要**：generation 说明"哪个实例的哪代拓扑"，digest 说明"参数面本身"；两者任一失配都按 §1.3 降权。

### 1.3 指纹失配语义（降权待复验、不跨设备同步）

1. **失配 = 降权待复验**：live inspect 的 generation/digest 与条目环境不匹配时，条目在 D14 物化视图中被标记 re-verify pending、有效权重下调；**不删除、不静默改写、不自动迁移到新指纹**。复验通过才追加 `conformed` 证据；复验失败则追加纠正/`unsupported` 证据并留痕（D14 执行）。
2. **版本/格式升级 = 新环境**：新版本产生新 `env_key`，旧条目保留为历史审计，不做"兼容继承"。
3. **不跨设备同步**：环境类条目机器本地（`.local` 同构），同步机制（若未来存在）只服务用户偏好类；把环境条目同步到另一台机器即制造"换台机器就不灵"故障。
4. **generation 禁止缓存为权威**：样例 V2 记录两次 run 的 generation 不同（`dl1t1_f8c727…` / `dl1t1_9fc850…`），证明 generation 是实例级；执行期绑定必须 fresh inspect（§5.3）。

### 1.4 与其它两类记忆的边界（禁止混用）

- **用户偏好**（按人、跨设备）：一律走 D13 质询确权 + D14 用户偏好类；本 schema 不定义、不聚合、不同步。**不把"用户喜欢这个设置"写成客观经验**。
- **工程事实**（按工程、append-only）：工程账本负责；本 schema 不承载工程状态。
- **`user_approved_setting` 的定位**：只记录**客观可复核的设置事实**（哪个环境/上下文、用户显式请求/认可了什么物理值、证据在哪），**不承载偏好泛化**、不成为默认值、不跨工程复用（§2.4）。A/B 主观判断的原始证据归属 [`AUDITION_AB_AND_USER_JUDGMENT_V1.md`](AUDITION_AB_AND_USER_JUDGMENT_V1.md) §11（raw judgment 是 source of truth，偏好蒸馏是未来层），不在本文。

### 1.5 机器本地与隐私

- 条目与证据 refs **不得包含** credential/token/serial/用户名/密钥；`plugin_path` 只用于证据索引（工作区工件路径），**不进 `plugin_identity`**（用 identifier/version/format 定位）。
- `machine_key` 只在本机持久化；不得上传或写入共享文件。
- 证据 ref 指向的工作区工件可能含本机路径，**随机器本地存储**，不属于跨设备同步面。

---

## 2. 条目类型（逐类允许字段与禁止内容）

### 2.0 通用规则（对六类均适用）

1. `content` 是**结构化 JSON**，键闭合（等价 `additionalProperties: false`）；未列入白名单的键一律拒绝。
2. **禁止任何可执行载荷**：参数映射、alias 注册、normalized↔physical 曲线、credential、命令/tool_call/写计划/调度路径、以及任何"可直接喂给执行器"的结构。文本字段只描述证据、边界与风险。
3. 每条条目**必须**带 ≥1 条 `evidence_refs`；无证据的猜测只能以 `agent-inferred` + 证据引用（"推断所依据的观测"）存在，且 `limitations` 必须写明。
4. 条目类型**不是权限**：任何类型都不产生执行权、不扩展工具目录、不改变 PCA/typed 准入。

### 2.1 `semantic_hint`（模糊参数名的可能含义）

- **用途**：记录参数名/显示名语义歧义的**候选解释**，供 planner 推理与用户解释使用（与 `plugin_grabber.explain_controls` 的 `weak_local_heuristic` 语义提示同族，但持久化、带证据）。
- **允许字段**：`subject`（`control_name`、可选 `display_group`）；`candidates[]`（`meaning`、`confidence`、`basis`）；`caveats[]`。
- **禁止内容**：任何 normalized↔physical 映射/曲线/参数 ID→值公式；alias 定义；把候选解释写成"已验证 DSP 行为"；替代 live inspect 的 role 绑定；preset 值。
- **trust 允许**：`observed` / `user-confirmed` / `agent-inferred`。

### 2.2 `gotcha`（真实踩坑）

- **用途**：记录真实遇到的坑：崩溃风险、静默 no-op、显示/行为不一致、探针副作用、非有限端点、通道歧义、domain 陷阱等。
- **允许字段**：`symptom`、`trigger_conditions[]`、`observed_signature`、`impact`、`safe_policy[]`（**确定性安全约束**，如"必须走 typed 工具/必须 fresh inspect/有限目标不得落在非有限端点"）、`verification`。
- **禁止内容**：绕过准入门/typed 事务的 workaround；对 typed-owned 参数改用通用 `plugin.set_parameter` 的指引；主机 GUI/键鼠自动化（computer use）指引；无界重试；credential 使用；把单一观测写成跨版本永久结论。
- **trust 允许**：`observed` / `user-confirmed` / `conformed` / `agent-inferred`。

### 2.3 `write_order_precondition`（写序/前置条件）

- **用途**：记录某一环境下观察到的角色写入顺序/前置条件（per-plugin gotcha），供规划期风险提示与验证计划参考。
- **允许字段**：`ordered_roles[]`、`condition`、`observed_signature`、`authority_note`（必须写明 advisory-only 与执行权威归属）。
- **禁止内容**：把写序写成执行权威/通用规则；携带 param_id + 目标值的可执行写计划；覆盖 typed 事务的写入顺序/原子性；"先写 A 再写 B 才能工作"式绕过事务校验的指令集。
- **trust 允许**：`observed` / `user-confirmed` / `conformed` / `agent-inferred`（外部报告类通常只能 `agent-inferred`，且须保留冲突）。
- **执行权威重申**：typed executor 的写入顺序由其契约与 live inspect 决定（例：`apply_eq_edits` free_floating 的 activation-last，`eq_band_summary.go:110-111`）；memory 不参与执行判定。

### 2.4 `user_approved_setting`（仅客观可复核的设置事实，不承载偏好）

- **用途**：记录用户在**具体上下文**中显式请求/认可的客观设置（物理值/枚举标签），以及批准证据。
- **允许字段**：`setting.roles[]`（`role`、`target_kind`、`value`、`unit`）、`setting.context`；`approval.event_ref`、`approval.scope`、`approval.approved_at`。
- **禁止内容**：偏好泛化（"用户喜欢 X"）、默认值固化、跨工程/跨上下文自动复用、由单次 A/B 选择推断长期偏好（`AUDITION_AB_AND_USER_JUDGMENT_V1.md` §11/§14）、credential。
- **trust 允许**：仅 `user-confirmed`。

### 2.5 `ambiguity_resolution`（歧义消解记录）

- **用途**：记录某个命名/语义歧义问题如何被消解（问题、结论、被否决的替代解释、依据），例：`Mix` 是 parallel balance 不是压缩强度；产品家族标签不等于控制拓扑。
- **允许字段**：`question`、`resolution`、`rejected_alternatives[]`、`basis`。
- **禁止内容**：可执行映射；把某版本的结论套用到其它版本/格式；覆盖 live 显示标签；替代 inspect 的分类结果。
- **trust 允许**：`observed` / `user-confirmed` / `conformed` / `agent-inferred`。

### 2.6 `unsupported_boundary`（证据支撑的负向能力事实）

- **用途**：记录确定性的"不支持/拒绝"边界（inspect 拒绝码、超出 typed 工具能力的行为、家族标签与拓扑不一致），供能力边界回复与规划避坑。
- **允许字段**：`attempted_tool`、`rejection_code`、`observed_scope`、可选 `family_label_vs_topology`、`retry_policy`。
- **禁止内容**：跨版本永久结论；把边界当 PCA 豁免/准入凭证；发明 fallback；默认开启近似写；主机控制/键鼠自动化绕过。
- **trust 允许**：仅 `unsupported`。

### 2.7 类型 × trust 矩阵

| entry_type | observed | user-confirmed | agent-inferred | conformed | unsupported |
|---|---|---|---|---|---|
| `semantic_hint` | ✓ | ✓ | ✓ | — | — |
| `gotcha` | ✓ | ✓ | ✓ | ✓ | — |
| `write_order_precondition` | ✓ | ✓ | ✓ | ✓ | — |
| `user_approved_setting` | — | ✓ | — | — | — |
| `ambiguity_resolution` | ✓ | ✓ | ✓ | ✓ | — |
| `unsupported_boundary` | — | — | — | — | ✓ |

---

## 3. Entry 字段规范（schema v1，封闭集）

### 3.1 字段总表

| # | 字段 | 类型 | 必填 | 语义与约束 |
|---|---|---|---|---|
| 1 | `schema_version` | string | 是 | 固定 `plugin-memory.entry/v1`；不兼容变更必须升版本 |
| 2 | `entry_type` | enum | 是 | 六类之一（§2）；content 结构判别符 |
| 3 | `entry_ref` | string | 是 | `memory.entry` ref（§3.2）；条目身份 |
| 4 | `environment` | object | 是 | `{machine_key, env_key}`；聚合键（§1.2/§3.6） |
| 5 | `trust` | enum | 是 | `observed/user-confirmed/agent-inferred/conformed/unsupported`（§3.3） |
| 6 | `confidence` | number [0,1] | 是 | advisory 权重，不是执行门（§3.4） |
| 7 | `evidence_count` | integer ≥1 | 是 | 独立证据记录数（≥1） |
| 8 | `evidence_refs` | string[] ≥1 | 是 | 可回指证据（§3.4 文法） |
| 9 | `context_tags` | string[] ≥1 | 是 | 小写点分标签，不允许空格/大写（§3.5） |
| 10 | `created_at` | RFC3339 | 是 | 首次观测/生成时间 |
| 11 | `updated_at` | RFC3339 | 是 | 原始条目 ≡ `created_at`；D14 派生视图可重算（§3.7） |
| 12 | `expires_at` | RFC3339 \| null | 是 | 过期策略字段；null=无 TTL（§3.7） |
| 13 | `plugin_identity` | object | 是 | §1.2；生产捕获子字段非空 |
| 14 | `surface_fingerprint` | object | 是 | §1.2；生产捕获子字段非空 |
| 15 | `content` | object | 是 | 按 `entry_type` 的白名单结构（§2、附录 A） |
| 16 | `limitations` | string[] ≥1 | 是 | 已知边界；未核实项必须含 `[待用户确认]`（§3.9） |

**封闭集说明**：上表 = 卡面字段全集 + 两个 schema 必需补字段——`entry_type`（六类内容判别符）与 `environment`（聚合键，落实 §目标 1 的"聚合键=环境"）。除此之外 v1 **不允许**新增字段；扩展须升 `schema_version`。

**兼容策略（无存量负担）**：D14 落地前本 schema 无生产存量记录；落地后新增字段/枚举值一律 fail-closed（未知即拒绝，不静默降级），旧版本条目按 `schema_version` 分派迁移。

### 3.2 `entry_ref` 与 REF_SCHEMA §6 对齐

[`REF_SCHEMA_V1.md`](REF_SCHEMA_V1.md) §6 预留：`memory.entry`（原始记忆流）/ `memory.distilled`（蒸馏文档）为命名空间，本定版只占名、不实现，收尾段注册即用。本 schema 的条目 ref 模板：

```text
vit://memory.entry/memory.env:<env_key>/t=all@<append_id>#<content_digest>
```

| 段 | 本 schema 取值 | 说明 |
|---|---|---|
| `kind` | `memory.entry` | 使用 §6 预留命名空间；charset `[a-z0-9._]` 已容纳，不改文法 |
| `scope` | `memory.env:<env_key>` | `scope_kind` **扩展候选** `memory.env`（另有 `memory.project` / `memory.user` 候选，分别对应工程/用户类，由 D14 注册时登记；REF_SCHEMA §6 规定新增 scope_kind 走"开放集+登记义务"） |
| `window` | `t=all` | memory 条目没有音频采样窗；本版显式用 `t=all` 满足 INV5（window 禁省略）。D14 若需要专用 window 记号，属其注册范围，不改变本 schema 字段 |
| `snapshot` | `append_id` | 原始流的追加标识（D14 定义，非空）；不得复用可变值 |
| `hash` | `#` 段取 `sha256:<16hex>` 或 `-` | 内容摘要：对条目 payload（除 `entry_ref` 外全部字段）做 canonical JSON（键排序、无空格、UTF-8）sha256 截 16 hex；`-` 表示尚未 CAS 化（对齐 REF_SCHEMA §7 `#-` 渐进语义） |

- **内容摘要自洽**：样例生成器会重算摘要并断言与 `entry_ref` 一致（见 `make_samples.py` / `VALIDATION.txt`），保证"条目身份 ↔ 内容"可校验。
- **命名空间对齐**：`memory.distilled`（蒸馏文档）不承载单条 entry，而是 D14 从原始条目投影出的 markdown/派生文档（§8）；两者通过 `entry_ref` 回链。
- **注册时点**：kind/scope_kind 的实际注册归 D14 落地卡（本卡只定义模板与登记义务，不修改 `agent/internal/agentprotocol/refschema.go`）。

### 3.3 `trust` 五态语义与迁移

| trust | 语义 | 证据要求 | 典型来源 |
|---|---|---|---|
| `observed` | 直接观测到的客观事实（live/记录工件中的 inspect、apply、readback、probe、参数面） | ≥1 条可回指观测证据 | 运行时 capture、真栈烟测工件 |
| `user-confirmed` | 用户显式确认过的事实（D13 式确权，但限客观经验，不是偏好） | 观测证据 + 用户确认事件 | 用户纠正/确认 |
| `agent-inferred` | agent/模型基于证据的推断或假设（含外部报告转述） | ≥1 条推断依据的观测证据 + 明确 caveat | 失败重试分析、外部资料 |
| `conformed` | 已在**同环境**用独立 live 证据复验一致（再 inspect/读回/复现通过） | 原证据 + 复验证据 | D14 复验任务 |
| `unsupported` | 确定性的负向能力事实（稳定拒绝码/重复失败归类） | 拒绝/失败工件 + 分类 | inspect 拒绝、boundary 记录 |

**迁移规则**（追加式，不原地改历史）：

- 任何 trust 提升都必须**追加新证据**（`evidence_count` 增 + refs 增）；只改标签不收证据 = 拒绝。
- `observed → conformed`：独立复验通过；复验失败 → 追加纠正证据（不得静默降级）。
- `agent-inferred → observed/conformed`：live 证据确认后追加新条目/新证据。
- `user-confirmed` 不因模型推理而降级；用户可显式撤销（追加撤销证据）。
- `unsupported` 是独立状态不是"降级"；若后续证据证明支持，追加新条目并保留旧边界条目为历史。
- **环境失配时任何 trust 都不能直接沿用**：先降权待复验（§1.3）。

### 3.4 `confidence` / `evidence_count` / `evidence_refs`

- `confidence` ∈ [0,1]，**advisory 权重**：不用于执行门、不用于 PCA、不用于跳过 inspect。建议区间（非强制）：`agent-inferred` 0.2–0.5；`observed` 0.5–0.85；`conformed` 0.8–0.95；`user-confirmed` ≥0.8（对事实本身）；`unsupported` 0.6–0.95（取决于拒绝证据强度）。
- `evidence_count` = 独立证据记录数（≥1）；复验/新观测递增；与 `evidence_refs` 不要求一一对应（一个 ref 可携带多条证据），但不得虚增。
- `evidence_refs` 字符串文法（三态 + 一种设计期本地形态）：

| 前缀 | 形态 | 语义 |
|---|---|---|
| `vit://` | 合法 REF_SCHEMA ref | 首选；CAS 化后指向字节一致内容 |
| `repo:` | `repo:<相对路径>#<anchor>` | 仓库受控文件（记录分支基线 commit 于证据索引） |
| `run:` | `run:<coord/runs 相对路径>#<anchor>` | 受控 run 工件（同上） |
| `workspace:` | `workspace:<相对路径>#<anchor>#sha256=<64HEX>` | 机器本地/未入库工件（必须带文件 sha256；不跨设备） |

- `anchor` 是**人读锚点**（`L<line>`、`L<a>-L<b>`、`section-...`、`json-path[token]`），不含空格与 `#`；不是机器解析协议。
- 生产捕获的 `evidence_refs` **应**以 `vit://` 为主；`repo:/run:/workspace:` 是设计期与渐进期形态，D14 物化时应转 CAS ref 或按 legacy 降权。

### 3.5 `context_tags`

- 小写点分标签（`^[a-z0-9][a-z0-9._-]*$`），建议族：`domain.*`（eq/compressor/reverb/delay）、`product.*`、`tool.*`（如 `tool.plugin_grabber.apply_eq_edits`）、`context.*`（static-eq/parallel/user-approved/ambiguity）、`risk.*`（write-order/non-finite-endpoint/channel-linkage/capability-boundary/crash/silent-noop）、`source.*`（external-report/workspace-artifact/inspect-run）。
- 标签只用于检索/分组，**不表达权威**；不得用标签编码执行指令。

### 3.6 `environment` / `plugin_identity` / `surface_fingerprint`

- 三者关系与派生见 §1.2；`env_key` **必须**可由三个部分重算（样例校验器强制）。
- `machine_key` 在生产条目中为本机不透明指纹（`m_<16hex>`）；设计样例用 `m_local` 占位（非真实指纹，样例环境键因此只是演示值）。
- **设计样例的 null 例外**：`plugin_identifier` / `plugin_version` / `surface_fingerprint.captured_at` 在样例中可为 null，但**必须**在 `limitations` 出现 `[待用户确认]`；生产捕获一律非空（校验器 sample_mode 区分）。
- `plugin_identity` **不含** `plugin_path`、preset 名、用户名等本机/隐私信息；路径只允许出现在 `evidence_refs` 的 workspace 形态中。

### 3.7 时间字段

| 字段 | 语义 | 规则 |
|---|---|---|
| `created_at` | 首次观测/条目生成时间 | RFC3339（含时区）；设计样例为设计件生成时间 |
| `updated_at` | 原始条目恒等于 `created_at`（append-only，不原地改写）；D14 派生/合并视图表示最近重算时间 | 不得早于 `created_at`；原始条目禁改 |
| `expires_at` | 过期策略字段；null=无 TTL | 晚于 `created_at`；建议环境条目 TTL 90 天（调研结论 §5.3 借 mthines 的 ~90 天过期提及，仅作 D14 默认建议，非本卡强制） |

- **过期执行归 D14**：本文只定义字段与语义；环境指纹失配是比墙钟更早的软过期信号（§1.3）。
- 合并（consolidation）产生的派生条目必须保留来源 `entry_refs`（D14 实现），不得覆盖/删除原始条目。

### 3.8 `content` 结构（按类型，摘要）

- 六类 content 的允许键见 §2 各类；实现可直接用附录 A 的 JSON Schema（`oneOf` by `entry_type`，`additionalProperties: false`）。
- content 内**不允许**嵌套"可执行载荷"键（校验器对 `mapping/alias/credential/token/command/tool_call/control_ref/param_map/apply_plan/schedule/normalized_curve/instructions` 等键名递归拒绝）。

### 3.9 `limitations` 与 `[待用户确认]`

- `limitations` 必填、非空字符串数组；必须诚实记录：证据缺口、版本边界、不可跨插件套用的范围、raw 工件缺失等。
- **无法核实的样例证据必须显式标 `[待用户确认]`**；不得编造。校验器在 sample_mode 下强制"null 身份字段 ⇒ limitations 含 `[待用户确认]`"。
- `[待用户确认]` 的条目**不等于不可用**：作为 advisory 仍可进 planner 上下文，但不得作为执行依据、不得升级 trust、不得出现在"已验证"叙述中。

### 3.10 版本与兼容

- `schema_version` 采用 `plugin-memory.entry/v1`；字段/枚举/语义不兼容变更必须升版本（v2），并给迁移规则。
- 未知 `entry_type` / `trust` / 未知顶层字段：**fail-closed 拒绝**（不静默保留），防止未来字段被旧消费者误读为权威。
- 兼容承诺：同一 `schema_version` 内只做加性、可忽略的语义澄清；条目身份（`entry_ref` 内容摘要）随内容变化自然变化，不承诺跨版本 ref 相同。

### 3.11 与 D14 的两层语义（raw vs 物化视图）

- **原始条目**（本文 schema 的对象，`memory.entry`）：capture 时生成，**append-only、不可改**；纠错/复验/合并都通过**追加新条目/新证据**表达。
- **物化/派生视图**（D14 提供，`memory.distilled` 文档或其查询视图）：从原始条目聚合、降权、合并、生成画像；`updated_at`/`expires_at` 在派生视图中可重算。
- 本卡只定义原始条目的字段与两侧语义；**聚合算法/视图形态是 D14 实现细节**（卡面："过期/合并字段（执行细节留 D14）"）。

---

## 4. 写入策略

### 4.1 capture 触发（T1–T4）

| 触发 | 定义 | 最小证据要求 | 典型 entry_type | trust 起点 |
|---|---|---|---|---|
| **T1 用户纠正** | 用户明确指出某个记忆/判断/操作错误，或确认事实 | 用户确认事件 + 被纠正对象的观测证据 | gotcha / ambiguity_resolution / write_order_precondition | `user-confirmed`（事实确认）或追加纠正证据 |
| **T2 失败重试** | 确定性失败后经修正重试成功/定位出稳定签名 | 失败工件 + 成功/分类证据；单次偶发失败不得触发（按 AGENTS §8 分类） | gotcha / unsupported_boundary | `observed`（有工件）或 `agent-inferred`（仅推断） |
| **T3 实测 gotcha** | inspect/apply/readback/probe 暴露真实坑 | 原始工件（hash/日志/ref） | gotcha / semantic_hint / write_order_precondition / unsupported_boundary | `observed` / `unsupported` |
| **T4 用户认可设置** | 用户显式请求/认可具体设置事实 | 用户请求事件 + typed readback（若执行） | user_approved_setting | `user-confirmed` |

**反触发（禁止 capture）**：

- 单次未分类的偶发失败（不得当 gotcha 固化）；
- 无证据的模型猜测（`agent-inferred` 也必须引用"推断所依据的观测"）；
- 用户偏好/主观判断（走 D13 + D14 用户偏好类，不在本 schema）；
- credential/token/隐私/路径类信息（永不写入）；
- "近似写成功"的经验（默认不写；如确需记录，只能作为 unsupported/风险记录，且不得成为可执行方案）。

### 4.2 谁可写（writer 类与 trust 上限）

| writer | 可写内容 | trust 上限 | 说明 |
|---|---|---|---|
| **W1 运行时 capture hook**（D14 接线，确定性） | 从工具/事务/运行结果自动追加原始条目 | `observed` / `unsupported` / `agent-inferred` | 不得直接写 `user-confirmed`/`conformed`（需用户/复验证据） |
| **W2 用户**（UI/会话） | 纠正、确认、显式设置请求 | `user-confirmed` | 用户编辑同样是追加事件，不覆盖历史 |
| **W3 D14 consolidation/verification 任务** | 追加复验证据、提升 `conformed`、重蒸馏视图 | `conformed`（需独立复验证据） | 不得编造证据；合并/过期在此层执行 |
| **W4 决策流/文档维护者** | 维护 reference/doc；可把人工结论**转写为条目**，但必须带证据 refs | `agent-inferred`（最多） | 防止"文档权威"借 memory 回潮为执行权威；升 trust 需 W2/W3 证据 |
| **禁止 writer** | typed executor 事务内读写 memory；插件/第三方；无证据的模型自由写入；任何把 memory 当控制面的组件 | — | executor 事务不得依赖 memory（§5.2） |

### 4.3 矛盾保留（不覆盖、带上下文并存）

1. **不覆盖**：同一环境/主题出现相反证据时，**保留双方**，各自携带 `context_tags`、`observed_signature`、`evidence_refs`；禁止原地覆写/删除旧条目。
2. **并存带上下文**：冲突对通过相同环境键 + 主题标签关联；D14 consolidate 时按"默认 + 条件化例外"处理（D14 画像数学：例外不覆盖默认、例外挂条件），并显式输出冲突视图供用户可见。
3. **冲突裁决顺序（消费期）**：(a) 当次 live 证据；(b) trust 等级；(c) 证据量与近期权重（D14 数学）；(d) 以上都不可分时**不裁决**，保持双条目、planner 提示不确定性。**执行永远只服从 (a) + typed 契约。**
4. **样例**：Pro-Q 3 写序 Q1（Vit 实测 activation-last）与 Pro-C 2 写序 C4（外部报告 Used/Enabled 先写）在样例中作为**冲突对并存**，各带条件与证据；C4 为 `agent-inferred`/`[待用户确认]`，两者都不改变执行（§6）。

### 4.4 过期、合并与失效字段

- **过期**：`expires_at` 由 D14 策略执行；环境指纹失配是**立即软过期**信号（降权待复验），不依赖墙钟。
- **合并**：D14 可把多条同环境/同主题条目合并为派生视图；必须保留全部来源 `entry_refs`、合并 `evidence_refs`（去重）、`evidence_count` 不重复计数、`limitations` 取并集；原始条目永不删除（D14 "原始流永不删"）。
- **失效**：版本升级/格式变化 → 新环境键；旧条目转为历史；`unsupported` 边界随识别器演进可标记 superseded（追加新条目），不静默翻转。
- **执行细节留 D14**：何时触发过期扫描、TTL 默认值、合并频率、画像数学、冲突视图 UI 均不在本文定义。

### 4.5 写入失败、隐私与容量

- memory 写入是**旁路**：capture/持久化失败不得阻断用户任务或执行事务；失败按 D14 事件记录，不得静默丢弃。
- 隐私：见 §1.5；证据索引中的本机路径/哈希只用于本机审计。
- 容量：D14 负责 raw stream 的保留/归档策略；本 schema 不设容量上限，但建议 D14 对 `agent-inferred` 低价值条目优先降权/归档（非本文执行）。

### 4.6 禁止写入清单（schema 与校验双保险）

- ❌ 可执行参数映射、normalized↔physical 曲线/公式、param_id→role 映射表、alias 注册、preset 映射；
- ❌ credential/token/secret/密码/序列号/用户名；
- ❌ 命令、tool_call、ToolRoute、capability 注册、写计划、调度表、"下一步调用 X"式可执行路径；
- ❌ 主机 GUI 控制/键鼠模拟/computer use 指引；
- ❌ 用户偏好泛化、工程事实、跨设备环境条目；
- ❌ 任何绕过 PCA/typed executor/事务校验的建议。

---

## 5. 消费契约（advisory，非权威）

### 5.1 允许的消费面

1. **planner/LLM 上下文**：D14 查询接口按环境键/插件 identity/标签召回条目，作为**规划提示、风险提示、验证计划参考**进入上下文（前缀只放蒸馏后的高频强条目，按需检索，D14 决定）。
2. **只读工具**：未来的 `memory.query`（或其等价只读接口）返回条目 + evidence refs；只读、可审计、可被用户查看。
3. **用户透明视图**：画像/条目用户可见可删改（D14 透明性底线，"机器写人可改"）。

### 5.2 禁止的消费面（红线，逐条对照 ADR）

记忆条目**不得**：

- 作为**执行权威**（execution authority）——执行权威是 live deterministic topology + typed tools + PCA；
- 作为**参数映射来源**（parameter-mapping source）——角色绑定只能来自 live inspect 的 generation-scoped `control_ref`；
- 作为 **credential/验证凭证/准入证据**；
- 产生**可调度路径**（不新增工具、不注册 capability、不产生 tool route、不改变 readiness/准入）；
- 被 typed executor 在事务中读取或引用；
- 复活任何 VPS/Profile/SPAL 执行权威（[`SPAL_VPS_RETIREMENT_ADR.md`](SPAL_VPS_RETIREMENT_ADR.md)：退役能力不得作为执行权威/参数映射来源/验证凭证/持久化格式；`PluginProbe` 仍只做 health/load/snapshot/render/unload）。

### 5.3 typed executor 的义务（fresh inspect 重新推导）

1. 每次执行**必须 fresh inspect** 获得当前实例的 generation、拓扑与 `control_ref`；**禁止**从 memory 复用 control_ref/param_id/role 绑定。
2. 写入顺序、原子性、readback、rollback、restore 全部由 typed 事务契约决定；memory 的 `write_order_precondition` **不得**改变计划。
3. 拓扑不成立/参数面失配时 fail-closed 并按能力边界返回；不得因 memory 声称"这个参数可用"而放宽。
4. 不得因 memory 文本给参数起 alias/虚拟控制，或走通用 `plugin.set_parameter` 绕过 typed 所有权。
5. memory 写序/风险条目只能作为规划提示与验证项（如"读回时检查 -INF 端点"），不能作为执行依据。

### 5.4 冲突规则（live 证据优先）

| 情形 | 处理 |
|---|---|
| memory 说 X，live 证据说 ¬X | **live 优先**；memory 降权并追加冲突/纠正证据；执行不受 memory 影响 |
| memory 说"不支持"，live inspect 证明支持 | live 优先；旧 `unsupported_boundary` 标记 superseded（追加新条目），保留历史 |
| memory 说"支持/可写"，live inspect 拒绝 | live 优先；记录边界，按 capability boundary 返回 |
| memory 之间冲突 | §4.3 保留并存；planner 提示不确定性；执行仍只服从 live + typed 契约 |
| memory 过期/指纹失配 | 不复用；需要时重新 inspect/复验（§1.3） |

### 5.5 允许用途（明确列举）

- 生成规划假设与候选方向（不直接落参数）；
- 风险提示与验证计划（例如"该端点可能返回 -INF，读回需按命名状态校验"）；
- 触发 D13 质询（"这条写序冲突是否需要用户确认？"）；
- 向用户解释为什么某方案不可行/需要手动指引；
- 为 capability boundary 回复提供上下文（不替代确定性 boundary resolver）。

### 5.6 测试断言建议（供后续实现卡；本卡不实现）

以下断言用于 D14 落地/消费接线卡，直接回应"notes 不得新增可调用控制路径"（调研结论 §5.3 要求）：

| # | 断言（建议测试名） | 内容 | 参考锚点 |
|---|---|---|---|
| A1 | `TestMemoryEntrySchemaRejectsExecutablePayload` | schema 校验拒绝未知字段、禁止键（mapping/alias/credential/command/tool_call/control_ref/param_map/apply_plan/…）与非法 content 结构；`[待用户确认]` 规则生效 | 本文 §3、附录 A；`make_samples.py` 已实现同类校验 |
| A2 | `TestMemoryNotesCannotIntroduceCallableControlPath` | 注入对抗性 memory 条目（点名退役工具/自造 alias/伪 tool route）后：tool catalog、capability 路由、readiness 与注入前**逐项相等**；任何 memory 派生名字都不可解析为可调用路径 | `agent/internal/tools/catalog_test.go:348-361`（退役工具必须不可调用）；`agent/internal/chat/plugin_profile_retirement_test.go:11-29` |
| A3 | `TestExecutorIgnoresMemoryNotes` | 执行请求 DTO 无 memory 字段；executor 不读 memory store；apply 仍强制 current-generation `control_ref`，旧 generation 拒绝 | typed 契约：`docs/COMPRESSOR_CONTROL_V1.md` §Apply Contract；`plugin_profile_retirement_test.go:31-44` |
| A4 | `TestLiveEvidenceOverridesMemoryConflict` | memory 与 live inspect 冲突时 planner 采用 live、memory 降权、冲突保留可查 | 本文 §5.4 |
| A5 | `TestFingerprintMismatchDegradesMemory` | env key/generation/digest 失配 → 条目降权待复验、不被复用；复验通过才 `conformed` | 本文 §1.3/§3.3 |
| A6 | `TestEnvMemoryNeverCrossesDevices` | 环境类条目不出现在任何同步/导出面；机器本地策略可断言 | 本文 §1.3；D14 三种键 |
| A7 | `TestMemoryStoresNoCredentialsOrAliases` | 条目全字段扫描无 credential/secret/alias/路径泄漏模式；写入口拒绝 | 本文 §4.6 |

> 断言是**建议**，具体落点/命名由后续实现卡决定；本卡不在 `agent/` 写测试。

### 5.7 与 PCA、工具目录、准入门槛的关系

- PCA 门（[`PCA_PLUGIN_LOAD_GATE_V1.md`](PCA_PLUGIN_LOAD_GATE_V1.md)）不变：memory 不是 PCA 证据，不能替代 installed fingerprint/coverage/attestation 检查。
- 工具目录不变：memory 不新增工具/alias（`plugin_grabber.apply_control` 等退役名仍被 catalog 拒绝；`plugin_profile_retirement_test.go`）。
- readiness/准入门槛不变：memory 不升级 readiness、不解除 blocked；它最多解释"为什么被拒"。

---

## 6. 样例（Pro-Q 3 / Pro-C 2 / Valhalla 同族 reverb，共 13 条）

### 6.1 样例身份与指纹

> **为什么第三样例是 ValhallaSupermassive 而不是 Valhalla Plate**：卡面允许「Valhalla Plate（或同族 reverb）」。本机 2026-10-16 run 的 61 件时域目标中出现的唯一 Valhalla reverb 是 **ValhallaSupermassive**；仓库/工作区内没有 Valhalla Plate 的参数面或 inspect 工件。为保证样例 `evidence_refs` 可回指、**不编造 Plate 参数面**，本文用同厂同族 reverb ValhallaSupermassive 作为第三样例；Valhalla Plate 参数面留待未来 live inspect 补充（见 `EVIDENCE_INDEX.md` §5）。

| 插件 | plugin_identity（证据来源） | surface_fingerprint | env_key（样例派生） |
|---|---|---|---|
| **Pro-Q 3** | vendor=FabFilter，product=Pro-Q 3，identifier=`VST3-Pro-Q 3-a1aba170-ba5f7ab2`，format=VST3，version=3.2.3.0（`pro_q_3.json` `parameters.plugin_identity`/`plugin_resolution`） | tool=`plugin_grabber.inspect_eq`，generation=`eqt1_1c011654e3fffcaf6d968457`，digest=`sha256:24db54ccf8dc4064`（由 348 参数面重算） | `env1_2c7d9afdbd260293`（`m_local` 样例占位） |
| **Pro-C 2** | vendor=FabFilter，product=Pro-C 2，identifier/version **未在证据工件中记录** → `[待用户确认]`，format=VST3（`COMPRESSOR_REFERENCE_PRO_C_2.md`） | tool=`plugin_grabber.inspect_compressor`，generation=`c2t1_6b96b6b6969df9a2ddbf22af`，digest=`sha256:-`（原始参数面工件不在盘） | `env1_2d1a8c2ea9938d31`（样例占位） |
| **ValhallaSupermassive**（同族 reverb） | vendor=`valhalla`，product=ValhallaSupermassive，identifier=`VST3-ValhallaSupermassive-5cffe1ee-7e8fe5d5`，format=VST3，version **未在工件中记录** → `[待用户确认]`（run3 `matrix_summary.json`） | tool=`plugin_grabber.inspect_delay`，generation=`dl1t1_9fc850d507fac64a9601736e`，digest=`sha256:86160d7b9e3855f8`（18 参数面重算） | `env1_d5b28078fb7084f4`（样例占位） |

### 6.2 样例条目（完整 JSON）

> 以下 13 条是**设计期样例**（不是生产 store 记录）：`machine_key=m_local` 为占位；`created_at` 为设计件生成时间（历史观测时间见 `surface_fingerprint.captured_at` 与 `evidence_refs`）；全部样例通过 [`coord/runs/MEM-SCHEMA-1/VALIDATION.txt`](../coord/runs/MEM-SCHEMA-1/VALIDATION.txt) 的 schema 自校验（13/13 PASS，机器可读版本见 [`SAMPLE_ENTRIES.jsonl`](../coord/runs/MEM-SCHEMA-1/SAMPLE_ENTRIES.jsonl)）。
>
> **证据状态**：13 条中 Pro-C 2 的 identity/version 与原始测量工件、ValhallaSupermassive 的 version 属**无法核实**项，已在各条 `limitations` 标 `[待用户确认]`；其余证据可在仓库/工件中回指（逐条 ref 已内联，索引见 [`coord/runs/MEM-SCHEMA-1/EVIDENCE_INDEX.md`](../coord/runs/MEM-SCHEMA-1/EVIDENCE_INDEX.md)）。

### 6.2.1 逐条目索引与完整 JSON

#### Pro-Q 3（FabFilter Pro-Q 3，VST3 3.2.3.0）

| seq | entry_type | trust | confidence | evidence_count | 一句话 |
|---|---|---|---|---|---|
| q1 | `write_order_precondition` | `observed` | 0.85 | 3 | free_floating（allocatable slot）EQ 的 upsert，经 plugin_grabber. |
| q2 | `gotcha` | `observed` | 0.8 | 2 | Dry Level / Wet Level 在 normalized=0 的显示文本为 `-INF dB`（非有限端点） |
| q3 | `semantic_hint` | `observed` | 0.75 | 3 | Band N Dynamic Range / Dynamics Enabled / Threshold / Extern |
| q4 | `gotcha` | `observed` | 0.75 | 3 | 每个 band 带 Stereo Placement（enum Left/Right/Stereo/Mid/Side）、 |
| q5 | `unsupported_boundary` | `unsupported` | 0.8 | 3 | dynamic_section_excluded / unsupported_dynamic_eq |

##### 样例 Q1

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "write_order_precondition",
  "entry_ref": "vit://memory.entry/memory.env:env1_2c7d9afdbd260293/t=all@memschema-sample-q1#sha256:1b2b8b781b58bbd5",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_2c7d9afdbd260293"
  },
  "trust": "observed",
  "confidence": 0.85,
  "evidence_count": 3,
  "evidence_refs": [
    "workspace:artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json#smoke.result.writes#sha256=95E9CC67765EF2A2ABE4FDA591AB08780947B95978F9D1EBD50D4B3406FCC7C4",
    "repo:agent/internal/workflows/plugingrabber/eq_band_summary.go#L110-L111",
    "repo:docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md#section-8-monotonic-extension-contract"
  ],
  "context_tags": [
    "domain.eq",
    "product.fabfilter.pro-q-3",
    "tool.plugin_grabber.apply_eq_edits",
    "context.static-eq",
    "risk.write-order"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-Q 3",
    "plugin_identifier": "VST3-Pro-Q 3-a1aba170-ba5f7ab2",
    "plugin_format": "VST3",
    "plugin_version": "3.2.3.0"
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_eq",
    "generation": "eqt1_1c011654e3fffcaf6d968457",
    "param_surface_digest": "sha256:24db54ccf8dc4064",
    "captured_at": "2026-07-28T01:17:28+08:00"
  },
  "content": {
    "ordered_roles": [
      "shape",
      "frequency",
      "q",
      "gain",
      "activation"
    ],
    "condition": "free_floating（allocatable slot）EQ 的 upsert，经 plugin_grabber.apply_eq_edits 原子事务执行",
    "observed_signature": "真实 apply 的 writes/actual_readback 序列为 shape→frequency→q→gain→used(activation)，activation 位于末尾且整体 status=exact；参数面含 24 组 Used/Enabled allocatable slot",
    "authority_note": "advisory only：写入顺序的执行权威是 typed executor 契约（activation-last）与 live inspect 绑定；memory 不得覆盖事务计划，也不得据此改成其它写序"
  },
  "limitations": [
    "产品版本 3.2.3.0；非通用规则（其它主机/工具/版本可能不同）",
    "第三方调研报告存在相反写序主张（见 C4 冲突条目），本条与 C4 并存保留、不合并",
    "不得作为执行权威（见文档 §5）"
  ]
}
```

##### 样例 Q2

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "gotcha",
  "entry_ref": "vit://memory.entry/memory.env:env1_2c7d9afdbd260293/t=all@memschema-sample-q2#sha256:70543fdf6490b68b",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_2c7d9afdbd260293"
  },
  "trust": "observed",
  "confidence": 0.8,
  "evidence_count": 2,
  "evidence_refs": [
    "workspace:artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json#parameters.parameters#sha256=95E9CC67765EF2A2ABE4FDA591AB08780947B95978F9D1EBD50D4B3406FCC7C4",
    "repo:docs/COMPRESSOR_CONTROL_V1.md#apply-contract"
  ],
  "context_tags": [
    "domain.eq",
    "product.fabfilter.pro-q-3",
    "risk.non-finite-endpoint",
    "tool.plugin_grabber.apply_eq_edits"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-Q 3",
    "plugin_identifier": "VST3-Pro-Q 3-a1aba170-ba5f7ab2",
    "plugin_format": "VST3",
    "plugin_version": "3.2.3.0"
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_eq",
    "generation": "eqt1_1c011654e3fffcaf6d968457",
    "param_surface_digest": "sha256:24db54ccf8dc4064",
    "captured_at": "2026-07-28T01:17:28+08:00"
  },
  "content": {
    "symptom": "Dry Level / Wet Level 在 normalized=0 的显示文本为 `-INF dB`（非有限端点）；Band N Threshold 在 normalized=1 的显示文本为命名端点 `Auto`",
    "trigger_conditions": [
      "按显示最小/最大值反推 normalized↔physical 线性曲线",
      "把 -INF 或 Auto 当作连续数值端点参与插值"
    ],
    "observed_signature": "display_probe：Dry/Wet Level samples [0→-INF dB, .25→-12.0, .5→-6.0, .75→-2.5, 1→+0.0]；Band 1 Threshold samples [0→-90.0 dB, .25→-45.0, .5→-30.0, .75→-15.0, 1→Auto]",
    "impact": "有限目标值可能被错误解析到非有限/命名端点，或把端点状态当连续域，导致 typed 事务拒绝或语义误写",
    "safe_policy": [
      "一律以 live inspect 的 measured domain 与 display probe 为准",
      "有限写入目标必须落在有限数值区间；-INF/Auto 等只作为特殊状态处理",
      "写入后使用 typed readback（exact/quantized）校验，禁止自行反推曲线"
    ],
    "verification": "apply 前后 full preimage readback；遇到目标落在非有限端点时由 typed 层拒绝"
  },
  "limitations": [
    "端点语义是 Pro-Q 3 3.2.3.0 记录的值；其它版本可能不同，使用前重新 live inspect",
    "只描述显示端点现象，不构成任何 normalized↔physical 映射"
  ]
}
```

##### 样例 Q3

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "semantic_hint",
  "entry_ref": "vit://memory.entry/memory.env:env1_2c7d9afdbd260293/t=all@memschema-sample-q3#sha256:62237ad81acb8f66",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_2c7d9afdbd260293"
  },
  "trust": "observed",
  "confidence": 0.75,
  "evidence_count": 3,
  "evidence_refs": [
    "workspace:artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json#parameters.parameters#sha256=95E9CC67765EF2A2ABE4FDA591AB08780947B95978F9D1EBD50D4B3406FCC7C4",
    "repo:docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md#section-4-1-surfacescope",
    "repo:docs/EQ_AUXILIARY_EXCLUSION_FIX_VALIDATION.md#root-cause-and-fix"
  ],
  "context_tags": [
    "domain.eq",
    "product.fabfilter.pro-q-3",
    "context.static-eq",
    "context.ambiguity"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-Q 3",
    "plugin_identifier": "VST3-Pro-Q 3-a1aba170-ba5f7ab2",
    "plugin_format": "VST3",
    "plugin_version": "3.2.3.0"
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_eq",
    "generation": "eqt1_1c011654e3fffcaf6d968457",
    "param_surface_digest": "sha256:24db54ccf8dc4064",
    "captured_at": "2026-07-28T01:17:28+08:00"
  },
  "content": {
    "subject": {
      "control_name": "Band N Dynamic Range / Dynamics Enabled / Threshold / External Side Chain / Speakers / Ex Band N Speakers",
      "display_group": "Dynamics / Mix 等"
    },
    "candidates": [
      {
        "meaning": "静态 EQ core（Frequency/Gain/Q/Shape/Activation）的同频段辅助字段",
        "confidence": 0.8,
        "basis": "参数面为 24 组同构 Band N 字段；EQ 拓扑研究把 Dynamic/Threshold/External Side Chain 归入 auxiliary bindings，不参与静态 core 写入"
      },
      {
        "meaning": "Ex Band N Speakers 属 external side chain 扬声器配置，不是独立 band",
        "confidence": 0.85,
        "basis": "同 section 辅助字段不改变 main band 归属（EQ 拓扑研究 §4.1）"
      }
    ],
    "caveats": [
      "参数名含 Band/Threshold 不等于主链路控制；任何 role 绑定以 live inspect 结果为准",
      "不得把 Dynamic Range 当作静态 Gain 使用"
    ]
  },
  "limitations": [
    "解释性提示（weak hint 族），不是已验证 DSP 结论",
    "只提示命名歧义，不提供参数绑定或映射"
  ]
}
```

##### 样例 Q4

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "gotcha",
  "entry_ref": "vit://memory.entry/memory.env:env1_2c7d9afdbd260293/t=all@memschema-sample-q4#sha256:fa1b84905d4db84c",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_2c7d9afdbd260293"
  },
  "trust": "observed",
  "confidence": 0.75,
  "evidence_count": 3,
  "evidence_refs": [
    "workspace:artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json#parameters.parameters#sha256=95E9CC67765EF2A2ABE4FDA591AB08780947B95978F9D1EBD50D4B3406FCC7C4",
    "repo:docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md#section-4-2-eqsectionfacts",
    "repo:docs/COM_V1_TEST_CONTRACT.md#L132"
  ],
  "context_tags": [
    "domain.eq",
    "product.fabfilter.pro-q-3",
    "risk.channel-linkage",
    "context.static-eq"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-Q 3",
    "plugin_identifier": "VST3-Pro-Q 3-a1aba170-ba5f7ab2",
    "plugin_format": "VST3",
    "plugin_version": "3.2.3.0"
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_eq",
    "generation": "eqt1_1c011654e3fffcaf6d968457",
    "param_surface_digest": "sha256:24db54ccf8dc4064",
    "captured_at": "2026-07-28T01:17:28+08:00"
  },
  "content": {
    "symptom": "每个 band 带 Stereo Placement（enum Left/Right/Stereo/Mid/Side）、Speakers 与 Ex Band N Speakers 等通道相关字段；全局另有 Output Pan / Output Pan Mode",
    "trigger_conditions": [
      "把全局请求静默落成单通道写入",
      "未证明 channel_contract 就写通道相关或 stereo placement 字段"
    ],
    "observed_signature": "参数面 24×Stereo Placement/Speakers/Ex Band N Speakers；EQ 拓扑研究要求先证明 channel_contract ∈ {shared, linked mirror, unsafe asymmetric} 再决定通道写入方式",
    "impact": "通道语义可能被误改（只改单通道、立体声联动失配），或把全局请求错误扁平化",
    "safe_policy": [
      "全局请求优先 shared binding；通道特定写入仅在 live inspect 证明 linked/shared 后执行",
      "不得把多路径/通道联动当作单一标量控制",
      "写后校验所有被绑定通道的读回"
    ],
    "verification": "typed readback + full preimage restore；channel_link 类事实不得由音频推断（COM 测试契约既有规则）"
  },
  "limitations": [
    "通道契约按实例/版本由 live inspect 给出；本条只是风险提示，不代替 inspect",
    "不同插件族的 channel_contract 不同，禁止跨插件套用"
  ]
}
```

##### 样例 Q5

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "unsupported_boundary",
  "entry_ref": "vit://memory.entry/memory.env:env1_2c7d9afdbd260293/t=all@memschema-sample-q5#sha256:4c8ac8e4605973cb",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_2c7d9afdbd260293"
  },
  "trust": "unsupported",
  "confidence": 0.8,
  "evidence_count": 3,
  "evidence_refs": [
    "repo:docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md#section-6-dynamic-eq-boundary",
    "repo:docs/MULTIBAND_DYNAMICS_CONTROL_V1.md#L64-L74",
    "repo:docs/EQ_AUXILIARY_EXCLUSION_FIX_VALIDATION.md#section-conclusion"
  ],
  "context_tags": [
    "domain.eq",
    "product.fabfilter.pro-q-3",
    "risk.capability-boundary",
    "tool.plugin_grabber.apply_eq_edits"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-Q 3",
    "plugin_identifier": "VST3-Pro-Q 3-a1aba170-ba5f7ab2",
    "plugin_format": "VST3",
    "plugin_version": "3.2.3.0"
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_eq",
    "generation": "eqt1_1c011654e3fffcaf6d968457",
    "param_surface_digest": "sha256:24db54ccf8dc4064",
    "captured_at": "2026-07-28T01:17:28+08:00"
  },
  "content": {
    "attempted_tool": "plugin_grabber.apply_eq_edits（typed 静态 EQ core）",
    "rejection_code": "dynamic_section_excluded / unsupported_dynamic_eq",
    "observed_scope": "Dynamic Range / Threshold / Attack / Release / Ratio / External Side Chain 等动态/侧链行为不进入 typed 静态 EQ 控制范围；辅助字段本身仍可读",
    "family_label_vs_topology": "Pro-Q 3 产品上可做动态段，但 typed 静态 core 只控制静态 Frequency/Gain/Q/Shape/Activation；产品名/家族名不是控制证据",
    "retry_policy": "动态行为请求按 capability boundary 返回或换用支持该行为的处理器；不得用静态 Gain 冒充动态行为，不得让模型自行发明 fallback"
  },
  "limitations": [
    "边界范围随 typed 工具能力演进；本条不是永久结论",
    "不得据此放宽 PCA 或 typed 准入门槛"
  ]
}
```

#### Pro-C 2（FabFilter Pro-C 2，VST3；identity 子字段未记录 → [待用户确认]）

| seq | entry_type | trust | confidence | evidence_count | 一句话 |
|---|---|---|---|---|---|
| c1 | `gotcha` | `observed` | 0.8 | 1 | Pro-C 2 的 Mix 实测物理域为 0–200%，不是通用的 0–100% |
| c2 | `gotcha` | `observed` | 0.8 | 3 | Wet Gain / Dry Gain 的实测域带非有限端点；显示文本可出现 `-INF dB` |
| c3 | `ambiguity_resolution` | `observed` | 0.85 | 2 | `Mix` / `Wet Gain` / `Dry Gain` 是否表示压缩强度？ |
| c4 | `write_order_precondition` | `agent-inferred` | 0.35 | 2 | 第三方 REAPER MCP 报告声称：Pro-Q 3 / Pro-C 2 需先写 Band 的 Used/Enable |
| c5 | `user_approved_setting` | `user-confirmed` | 0.9 | 2 | user-approved setting（threshold=-12 dB / ratio=4:1） |

##### 样例 C1

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "gotcha",
  "entry_ref": "vit://memory.entry/memory.env:env1_e2daa9acfddc933d/t=all@memschema-sample-c1#sha256:c8614d12fb89425d",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_e2daa9acfddc933d"
  },
  "trust": "observed",
  "confidence": 0.8,
  "evidence_count": 1,
  "evidence_refs": [
    "repo:docs/COMPRESSOR_REFERENCE_PRO_C_2.md#measured-reference-surface"
  ],
  "context_tags": [
    "domain.compressor",
    "product.fabfilter.pro-c-2",
    "risk.parameter-domain",
    "context.parallel"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": "2027-01-16T14:30:00+08:00",
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-C 2",
    "plugin_identifier": null,
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_compressor",
    "generation": "c2t1_6b96b6b6969df9a2ddbf22af",
    "param_surface_digest": "sha256:-",
    "captured_at": null
  },
  "content": {
    "symptom": "Pro-C 2 的 Mix 实测物理域为 0–200%，不是通用的 0–100%",
    "trigger_conditions": [
      "按通用假设把 Mix 百分比钳制到 100%",
      "把 Mix 当作压缩强度控制"
    ],
    "observed_signature": "COMPRESSOR_REFERENCE_PRO_C_2 §Measured Reference Surface：`mix` 0 to 200% on this product; never assume 0 to 100%",
    "impact": "parallel balance 设置被错误截断或误读为压缩量",
    "safe_policy": [
      "百分比目标必须落在 live inspect 测得的物理域内",
      "禁止假设 0–100%；域检查以 inspect/apply 的 typed 校验为准"
    ],
    "verification": "typed readback（exact/quantized）+ measured domain 校验"
  },
  "limitations": [
    "[待用户确认] 该测量来自 tracked 参考文档；原始测量工件路径（VitApp/Workspace/Artifacts/smoke/product_path_20260804_094717/…/pro_c_2.json）在 2026-10-18 核查时不在盘，使用前应重新 live inspect",
    "TTL 为 90 天建议值（expires_at 示例），实际过期执行归 D14"
  ]
}
```

##### 样例 C2

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "gotcha",
  "entry_ref": "vit://memory.entry/memory.env:env1_e2daa9acfddc933d/t=all@memschema-sample-c2#sha256:30b3b107caf76c9b",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_e2daa9acfddc933d"
  },
  "trust": "observed",
  "confidence": 0.8,
  "evidence_count": 3,
  "evidence_refs": [
    "repo:docs/COMPRESSOR_REFERENCE_PRO_C_2.md#measured-reference-surface",
    "repo:docs/COMPRESSOR_CONTROL_V1.md#apply-contract",
    "repo:docs/PLUGIN_ALLIANCE_COMPRESSOR_BLIND_V1_ROUND1.md#failure-taxonomy"
  ],
  "context_tags": [
    "domain.compressor",
    "product.fabfilter.pro-c-2",
    "risk.non-finite-endpoint"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-C 2",
    "plugin_identifier": null,
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_compressor",
    "generation": "c2t1_6b96b6b6969df9a2ddbf22af",
    "param_surface_digest": "sha256:-",
    "captured_at": null
  },
  "content": {
    "symptom": "Wet Gain / Dry Gain 的实测域带非有限端点；显示文本可出现 `-INF dB`",
    "trigger_conditions": [
      "按显示 min/max 线性反推 normalized 曲线",
      "把 -INF 当作连续数值域端点"
    ],
    "observed_signature": "COMPRESSOR_REFERENCE_PRO_C_2：wet_gain/dry_gain `-18 to +36 dB, with a non-finite endpoint`；同类 -INF 命中已在 blind round 记录为拒绝（Zip threshold 目标 -60 dB → 显示 -inf dB，事务被拒并原子回滚）",
    "impact": "有限目标解析到非有限端点 → typed 层拒绝，或读到与预期不一致的端点状态",
    "safe_policy": [
      "有限目标不得映射到非有限端点（typed 契约既有规则）",
      "端点只按命名状态处理；曲线反推以 measured anchors 为准"
    ],
    "verification": "apply rejected 时确认无残留状态；成功时 exact/quantized readback"
  },
  "limitations": [
    "[待用户确认] wet/dry 域名来自 tracked 参考文档，原始测量工件不在盘；使用前重新 live inspect",
    "不要据此条为其它插件推导非有限端点行为"
  ]
}
```

##### 样例 C3

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "ambiguity_resolution",
  "entry_ref": "vit://memory.entry/memory.env:env1_e2daa9acfddc933d/t=all@memschema-sample-c3#sha256:355ee13d8821d27e",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_e2daa9acfddc933d"
  },
  "trust": "observed",
  "confidence": 0.85,
  "evidence_count": 2,
  "evidence_refs": [
    "repo:docs/COMPRESSOR_REFERENCE_PRO_C_2.md#canonical-semantic-axes",
    "repo:docs/COMPRESSOR_REFERENCE_PRO_C_2.md#directional-semantics"
  ],
  "context_tags": [
    "domain.compressor",
    "product.fabfilter.pro-c-2",
    "context.parallel",
    "context.ambiguity"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-C 2",
    "plugin_identifier": null,
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_compressor",
    "generation": "c2t1_6b96b6b6969df9a2ddbf22af",
    "param_surface_digest": "sha256:-",
    "captured_at": null
  },
  "content": {
    "question": "`Mix` / `Wet Gain` / `Dry Gain` 是否表示压缩强度？",
    "resolution": "否。它们属 output/parallel balance 轴：降低 mix 削弱可听湿声贡献，但不削弱压缩器内部 gain reduction；output gain 不是 compression intensity",
    "rejected_alternatives": [
      "把 mix 当作 compression intensity",
      "用 output gain 代替压缩量（output gain 只是电平补偿）"
    ],
    "basis": "COMPRESSOR_REFERENCE §Canonical Semantic Axes 与 §Directional Semantics：parallel balance / output normalization 是独立轴"
  },
  "limitations": [
    "语义澄清，不是参数映射；实际角色绑定以 live inspect 为准",
    "不同压缩器家族的 output 段结构不同，本条不跨插件套用",
    "[待用户确认] Pro-C 2 的 identity/version/identifier 未在证据工件中记录（原始测量工件不在盘）"
  ]
}
```

##### 样例 C4

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "write_order_precondition",
  "entry_ref": "vit://memory.entry/memory.env:env1_e2daa9acfddc933d/t=all@memschema-sample-c4#sha256:6dd6390b1304e9da",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_e2daa9acfddc933d"
  },
  "trust": "agent-inferred",
  "confidence": 0.35,
  "evidence_count": 2,
  "evidence_refs": [
    "workspace:coord/reports/2026-10-18-reaper-mcp-plugin-param-research.md#section-5-3#sha256=3717DF1738D19516E36C5A277A0079A65A9462034C9BE103E4D2003ADDDD4494",
    "repo:docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md#section-8-monotonic-extension-contract"
  ],
  "context_tags": [
    "domain.eq",
    "domain.compressor",
    "product.fabfilter.pro-q-3",
    "product.fabfilter.pro-c-2",
    "risk.write-order",
    "source.external-report"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-C 2",
    "plugin_identifier": null,
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_compressor",
    "generation": "c2t1_6b96b6b6969df9a2ddbf22af",
    "param_surface_digest": "sha256:-",
    "captured_at": null
  },
  "content": {
    "ordered_roles": [
      "used/enable activation",
      "frequency/gain/q"
    ],
    "condition": "第三方 REAPER MCP 报告声称：Pro-Q 3 / Pro-C 2 需先写 Band 的 Used/Enabled 再写 Frequency/Gain，否则参数写了没声音，更严重会触发崩溃",
    "observed_signature": "外部报告（REAPER/host 工具链，未在 Vit-DAW 栈复现）；Vit 侧真实 apply 记录相反（activation-last，见 Q1）",
    "authority_note": "冲突保留、不合并：本条 trust=agent-inferred、仅作规划期风险提示；写入顺序权威=typed executor 契约 + live inspect；任何执行不得据此改变写序，不得据此绕过事务校验"
  },
  "limitations": [
    "[待用户确认] 外部报告未经 Vit 栈复现；主机/工具/版本差异可能解释冲突",
    "不得把本条升级为执行权威或通用写序规则"
  ]
}
```

##### 样例 C5

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "user_approved_setting",
  "entry_ref": "vit://memory.entry/memory.env:env1_e2daa9acfddc933d/t=all@memschema-sample-c5#sha256:8c636a7b056df5f2",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_e2daa9acfddc933d"
  },
  "trust": "user-confirmed",
  "confidence": 0.9,
  "evidence_count": 2,
  "evidence_refs": [
    "repo:docs/COMPRESSOR_CONTROL_V1.md#vit-natural-language-validation",
    "repo:docs/COMPRESSOR_REFERENCE_PRO_C_2.md#worked-projection-examples"
  ],
  "context_tags": [
    "domain.compressor",
    "product.fabfilter.pro-c-2",
    "context.user-approved",
    "tool.plugin_grabber.apply_compressor_controls"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "FabFilter",
    "product": "Pro-C 2",
    "plugin_identifier": null,
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_compressor",
    "generation": "c2t1_6b96b6b6969df9a2ddbf22af",
    "param_surface_digest": "sha256:-",
    "captured_at": null
  },
  "content": {
    "setting": {
      "roles": [
        {
          "role": "threshold",
          "target_kind": "physical_absolute",
          "value": -12.0,
          "unit": "dB"
        },
        {
          "role": "ratio",
          "target_kind": "physical_absolute",
          "value": 4.0,
          "unit": ":1"
        }
      ],
      "context": "Vit natural-language compressor validation：用户明确要求 Threshold=-12 dB、Ratio=4:1；disposable Pro-C 2 实例"
    },
    "approval": {
      "event_ref": "conversation:compressor_nl_final_20260803_2350",
      "scope": "单次会话/单实例的显式设置请求，不跨工程、不成为默认",
      "approved_at": "2026-08-03T23:50:00+08:00"
    }
  },
  "limitations": [
    "只记录客观可复核的设置事实与用户显式请求，不承载偏好、不做默认值、不自动复用",
    "用户偏好类信息必须走 D13 质询确权 + D14 用户偏好类，不在本 schema",
    "[待用户确认] Pro-C 2 的 identity/version/identifier 未在证据工件中记录（原始测量工件不在盘）"
  ]
}
```

#### Valhalla 同族 reverb（ValhallaSupermassive，VST3；version 未记录）

| seq | entry_type | trust | confidence | evidence_count | 一句话 |
|---|---|---|---|---|---|
| v1 | `unsupported_boundary` | `unsupported` | 0.9 | 3 | unsupported_delay |
| v2 | `ambiguity_resolution` | `observed` | 0.85 | 3 | ValhallaSupermassive 应按 reverb 还是 delay 控制？ |
| v3 | `semantic_hint` | `agent-inferred` | 0.45 | 2 | Mix（其余面：DelaySync / DelayNote / Delay_Ms / DelayWarp / Feedb |

##### 样例 V1

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "unsupported_boundary",
  "entry_ref": "vit://memory.entry/memory.env:env1_218efe9c1a66754a/t=all@memschema-sample-v1#sha256:ac65edc7e2208c92",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_218efe9c1a66754a"
  },
  "trust": "unsupported",
  "confidence": 0.9,
  "evidence_count": 3,
  "evidence_refs": [
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/valhallasupermassive.inspects.json#inspects.reverb",
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_204214/valhallasupermassive.inspects.json#inspects.reverb",
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/README.md#L58"
  ],
  "context_tags": [
    "domain.reverb",
    "domain.delay",
    "product.valhalla.supermassive",
    "risk.capability-boundary"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "valhalla",
    "product": "ValhallaSupermassive",
    "plugin_identifier": "VST3-ValhallaSupermassive-5cffe1ee-7e8fe5d5",
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_delay",
    "generation": "dl1t1_9fc850d507fac64a9601736e",
    "param_surface_digest": "sha256:86160d7b9e3855f8",
    "captured_at": "2026-10-16T20:56:39+08:00"
  },
  "content": {
    "attempted_tool": "plugin_grabber.inspect_reverb",
    "rejection_code": "unsupported_delay",
    "observed_scope": "reverb 家族插件无 provable reverb stage/decay 控制（无衰减词证据），inspect_reverb 拒绝；同一实例 inspect_delay 可解析为 feedback_delay",
    "family_label_vs_topology": "family=reverb 来自清单元数据；家族/产品名不是拓扑证据，reverb 词表拒绝不等于产品不可用",
    "retry_policy": "不得据家族名强行按 reverb 控制；按 capability boundary 返回或走手动指引/替换处理器；不得发明 fallback、不得默认近似写"
  },
  "limitations": [
    "两条独立 run 均得到同一拒绝码（run2/run3）；插件版本未在工件中记录",
    "边界随词表/识别器演进；本条不是永久结论",
    "[待用户确认] ValhallaSupermassive 的 version 未在工件中记录"
  ]
}
```

##### 样例 V2

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "ambiguity_resolution",
  "entry_ref": "vit://memory.entry/memory.env:env1_218efe9c1a66754a/t=all@memschema-sample-v2#sha256:77ebee516db5f793",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_218efe9c1a66754a"
  },
  "trust": "observed",
  "confidence": 0.85,
  "evidence_count": 3,
  "evidence_refs": [
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/valhallasupermassive.inspects.json#inspects.delay",
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_204214/valhallasupermassive.inspects.json#inspects.delay",
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/matrix_summary.json#matrix[valhallasupermassive]"
  ],
  "context_tags": [
    "domain.reverb",
    "domain.delay",
    "product.valhalla.supermassive",
    "context.ambiguity"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "valhalla",
    "product": "ValhallaSupermassive",
    "plugin_identifier": "VST3-ValhallaSupermassive-5cffe1ee-7e8fe5d5",
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_delay",
    "generation": "dl1t1_9fc850d507fac64a9601736e",
    "param_surface_digest": "sha256:86160d7b9e3855f8",
    "captured_at": "2026-10-16T20:56:39+08:00"
  },
  "content": {
    "question": "ValhallaSupermassive 应按 reverb 还是 delay 控制？",
    "resolution": "以 live inspect 的分类为准：inspect_reverb 拒绝 unsupported_delay；inspect_delay 返回 feedback_delay（timing/output/feedback 段）。家族标签 reverb 不是控制证据",
    "rejected_alternatives": [
      "按 vendor family=reverb 猜测 reverb 参数",
      "按产品名套用 reverb 词表/写序"
    ],
    "basis": "两次独立 run 的 inspects 工件：reverb 面拒绝、delay 面 resolved=true、classification=feedback_delay、confidence 0.96"
  },
  "limitations": [
    "两条 run 的 generation 不同（dl1t1_f8c727… / dl1t1_9fc850…），说明 generation 是实例级、不可跨会话缓存复用",
    "分类结果随识别器版本变化，使用前重新 inspect",
    "[待用户确认] ValhallaSupermassive 的 version 未在工件中记录"
  ]
}
```

##### 样例 V3

```json
{
  "schema_version": "plugin-memory.entry/v1",
  "entry_type": "semantic_hint",
  "entry_ref": "vit://memory.entry/memory.env:env1_218efe9c1a66754a/t=all@memschema-sample-v3#sha256:33d82309f88e7640",
  "environment": {
    "machine_key": "m_local",
    "env_key": "env1_218efe9c1a66754a"
  },
  "trust": "agent-inferred",
  "confidence": 0.45,
  "evidence_count": 2,
  "evidence_refs": [
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/surfaces_compact.json#valhallasupermassive",
    "run:coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/valhallasupermassive.inspects.json#inspects.delay"
  ],
  "context_tags": [
    "domain.reverb",
    "domain.delay",
    "product.valhalla.supermassive",
    "context.ambiguity"
  ],
  "created_at": "2026-10-18T14:30:00+08:00",
  "updated_at": "2026-10-18T14:30:00+08:00",
  "expires_at": null,
  "plugin_identity": {
    "vendor": "valhalla",
    "product": "ValhallaSupermassive",
    "plugin_identifier": "VST3-ValhallaSupermassive-5cffe1ee-7e8fe5d5",
    "plugin_format": "VST3",
    "plugin_version": null
  },
  "surface_fingerprint": {
    "tool": "plugin_grabber.inspect_delay",
    "generation": "dl1t1_9fc850d507fac64a9601736e",
    "param_surface_digest": "sha256:86160d7b9e3855f8",
    "captured_at": "2026-10-16T20:56:39+08:00"
  },
  "content": {
    "subject": {
      "control_name": "Mix（其余面：DelaySync / DelayNote / Delay_Ms / DelayWarp / Feedback / Density / Width / LowCut / HighCut / ModRate / ModDepth / Mode / Reserved1-4）",
      "display_group": "output"
    },
    "candidates": [
      {
        "meaning": "inspect 的 output 段声明 dry/mix/wet 三个角色，但实测参数面只录到 1 个 Mix（display 50.0 %）",
        "confidence": 0.5,
        "basis": "surfaces_compact 仅 1 个 mix 参数；inspects 工件声明 output=[dry,mix,wet]，两者绑定关系未在工件中给出"
      },
      {
        "meaning": "不得假定 dry/wet 有独立可写 control_ref；角色绑定以 live inspect 结果为准",
        "confidence": 0.8,
        "basis": "同上；dry/wet 可能是 inspect 派生角色而非独立参数"
      }
    ],
    "caveats": [
      "[待用户确认] 需要一次 live inspect 验证 dry/wet 绑定",
      "本条仅为 agent 推断（trust=agent-inferred），不得作为参数映射或写计划来源"
    ]
  },
  "limitations": [
    "[待用户确认] 工件未包含角色→参数绑定明细，推断待 live inspect 复验",
    "不得据此构造 normalized↔physical 映射"
  ]
}
```

### 6.3 样例覆盖自查（卡面 §目标 6）

| 卡面要求 | 样例条目 | 覆盖 |
|---|---|---|
| 写序 gotcha | Q1（Vit 实测 activation-last）、C4（外部报告相反写序，冲突并存） | ✅ |
| `Mix` 0–200% | C1（Pro-C 2 Mix 实测域 0–200%） | ✅ |
| 非有限端点 | Q2（Pro-Q 3 Dry/Wet `-INF`、Threshold `Auto`）、C2（Pro-C 2 wet/dry 非有限端点 + blind round `-inf` 拒绝） | ✅ |
| 通道/立体声联动 | Q4（Pro-Q 3 per-band Stereo Placement/Speakers + channel_contract） | ✅ |
| 三插件各 ≥2 条 | Pro-Q 3 ×5、Pro-C 2 ×5、ValhallaSupermassive ×3 | ✅ |
| evidence_refs 可回指 / `[待用户确认]` | 13/13 带 refs；Pro-C 2 identity/工件缺口、Valhalla version 缺口已标 | ✅ |

---

## 7. 与既有资产的关系（不复用、不复活执行权威）

### 7.1 不复活 VPS / Profile / SPAL

- [`SPAL_VPS_RETIREMENT_ADR.md`](SPAL_VPS_RETIREMENT_ADR.md)：SPAL、VPS、VPSForge、learned Plugin Grabber profiles 已退役，**不是执行权威、不是参数映射来源、不是验证凭证、不是持久化格式**。本 schema 是**新的、非权威的、非可执行的** advisory 记忆，不恢复任何退役能力：
  - 不存 provider/credential/mapping/verification/promotion/staging（ADR "Removed Authority" 逐条）；
  - 不存可执行参数映射/alias（旧 Profile 的核心能力）；
  - 不复活 `plugin_grabber.apply_control` 等退役工具名（`catalog_test.go:348-361`、`plugin_profile_retirement_test.go` 继续保障）；
  - `PluginProbe` 仍只有 health/load/snapshot/render/unload 只读操作，memory 不改变其协议。
- 与调研报告 §5.3 的结论一致：xDarkzx 白名单 = 已退役映射层的极简版；mthines 版本化 md 是 advisory memory 而非控制权威；Vit 现状是 live deterministic topology + typed tools。

### 7.2 与 `COMPRESSOR_REFERENCE_PRO_C_2.md` 的区别

| 维度 | `COMPRESSOR_REFERENCE_PRO_C_2.md`（human reference exemplar） | 本文 memory entries |
|---|---|---|
| 性质 | 人工整理的**参考范例/知识文档**（"canonical worked example"） | 运行时/工件证据抽取的**环境域条目** |
| 作用面 | 人/设计者阅读，帮助理解语义轴与规划规则 | planner/LLM 上下文与只读工具（advisory） |
| 是否运行时 | 否（文档明确非 Profile/VPS/SPAL mapping，不参与运行时） | 是（D14 落地后进入上下文），但**仍非权威** |
| 证据 | 引用测量工件与历史烟测 | 每条必须带 evidence_refs |
| 执行 | 不产生执行 | 不产生执行（§5） |

- 两者不是替代关系：reference 文档可以**作为条目的来源之一**（样例 C1/C2/C3 即引用其测量表），但条目必须独立带证据、环境键与 limitations；文档不会因被引用而获得运行时权威。
- 样例中 Pro-C 2 的原始测量工件（`VitApp/Workspace/Artifacts/smoke/product_path_20260804_094717/…/pro_c_2.json`）在 2026-10-18 核查时**不在盘**，因此相关条目以 tracked 文档记录为证据并标 `[待用户确认]`——这正是 reference 与 runtime evidence 的边界示例。

### 7.3 与 `plugin_grabber.explain_controls` context pack 的 `semantic_hint` 关系

- 现役 context pack 已有 `semantic_hint_policy: {kind:"weak_local_heuristic", caveat:"Hints are prior clues for AI/user reasoning, not verified facts…"}`（`agent/internal/workflows/plugingrabber/context_pack.go:87-91`；测试 `agent/internal/chat/server_test.go:2645-2648`）。本文的 `semantic_hint` 条目**沿用同一"弱提示"哲学**，但：
  - context pack 提示是**即时、单次、无持久化**；memory 提示是**环境域、带证据、可复验**；
  - live context pack（本次 inspect 结果）**优先于** memory 提示；memory 不得覆盖当前 pack 的角色/域；
  - 两者都不得成为参数映射来源。

### 7.4 与 REF_SCHEMA / 观察投影（DAD 等）的关系

- 条目身份使用 `memory.entry` ref（§3.2）；evidence_refs 使用统一 ref schema（或渐进期文件锚点）。
- memory 是**对象族**（D2 对象类型轴），不是观察投影：不进入 DOM/MOM/TIM/TOM/FXM/COM/EPM/RLM 拓扑，不展开 raw package，不产生投影 view。
- 不改变 CCB/Context Runtime 的披露规则；memory 检索由 D14 提供，不与观察投影混层。

### 7.5 与 AUDITION 用户判断、D13 的关系

- [`AUDITION_AB_AND_USER_JUDGMENT_V1.md`](AUDITION_AB_AND_USER_JUDGMENT_V1.md) §11：raw `UserJudgmentEvidence` 是 source of truth；未来蒸馏偏好必须可回链；主观偏好必须确权、不能由一次 A/B 变成永久偏好；§14 非目标明确"不做自动偏好蒸馏"。
- 本 schema 只收**客观经验**；`user_approved_setting` 仅记录客观设置事实与批准证据，不承担偏好蒸馏。用户偏好类的蒸馏与画像归 D13/D14。

### 7.6 不写可执行参数映射 / alias / credential（三重保障）

1. **字段层**：schema 无映射/alias/credential 字段；
2. **content 层**：每类白名单 + 递归禁止键（§2.0/§4.6）；
3. **测试层**：后续实现卡按 §5.6 A1/A2/A7 断言（catalog 无新增、条目无可执行载荷、无凭证）。

---

## 8. D14 接入点（本卡只定义域内 schema/policy 与接入约定）

### 8.1 职责划分：raw append-only stream vs distilled markdown

| 层 | 职责 | 归属 | 本文关系 |
|---|---|---|---|
| **raw append-only stream**（`memory.entry` 族） | 不可变 capture 记录：工具/事务/用户事件 → 条目（本文 schema）+ 原始工件引用 | **D14**（存储/索引/追加） | 本文定义条目 schema 与写入策略；D14 定义 raw envelope（writer/trigger/时间/工件） |
| **distilled markdown**（`memory.distilled` 族） | 从原始流投影出可读蒸馏文档/画像 core；算法升级可对全历史重蒸馏 | **D14** | 本文只约定"蒸馏文档可回链 entry_refs、不覆盖 raw"；格式归 D14 |
| **query 接口** | 按环境键/identity/标签/trust 召回条目 + evidence refs | **D14** | 消费面只允许 planner 上下文/只读工具（§5.1） |
| **consolidate / 画像** | 冲突并存、例外/默认、贝叶斯画像、透明可编辑 | **D14** | 本文提供字段（trust/confidence/evidence/expires/context_tags）与矛盾保留策略；算法归 D14 |
| **失效/过期/复验** | 指纹失配降权、TTL、复验任务 | **D14** | 本文定义语义（§1.3/§3.7/§4.4），执行归 D14 |
| **存储与同步** | `.local` 同构、机器本地、不跨设备同步环境类 | **D14** | 本文定义键与同步禁令（§1.3） |

D14 参考形态（路线图 `L157-L162`）：原始记忆流（append-only、evidence 层）+ 蒸馏文档（投影层）；参考 Hermes Agent 的 markdown 蒸馏产物 / SQLite+FTS5 原始流 / consolidation loop，但**架构不照搬**——其自主巩固仅适用于客观经验类，主观偏好必须确权。本 schema 即"客观经验类"的域内规格。

### 8.2 接入约定（D14 落地时）

1. **命名空间注册**：`memory.entry`（已预留）+ `scope_kind` 候选 `memory.env`（本 schema 用）/`memory.project`/`memory.user`（D14 按对象类型轴登记；REF_SCHEMA §6 开放集+登记义务，不改文法）。
2. **capture 写入**：只追加、由 writer 类（§4.2）触发；raw envelope 由 D14 定义（writer/trigger/at/raw_artifact_refs）；本文条目字段从 envelope 内容派生。
3. **查询/上下文拼装**：D14 提供查询；planner 上下文只注入 distilled 核心 + 按需召回条目；注入内容必须带 `entry_ref`/trust/limitations，供 LLM 识别"这是 advisory"。
4. **物化与 ref**：条目进入 CAS 后 `entry_ref` 的 `#` 段从 `-` 升级为内容摘要；`vit://` evidence refs 可 resolve。
5. **无执行接线**：D14 不得把 memory 读到执行路径（executor 请求无 memory 字段）；消费接线卡必须带 §5.6 断言。
6. **机器本地/隐私**：环境类不跨设备；机器指纹只在本机。

### 8.3 本卡明确不做

- 不建持久化 store（SQLite/FTS5/markdown 都不建）；
- 不实现查询/检索/失效/consolidation/画像；
- 不写 Go 代码、不接 runtime、不加 capture hook；
- 不定义 raw envelope 完整格式（D14 定）；
- 不改变 PCA/typed executor/工具目录/准入门槛；
- 不做消费接线（D14 落地后另卡）。

### 8.4 D14 落地前需确认的开放项

| # | 开放项 | 建议 |
|---|---|---|
| O1 | `scope_kind` 记号（`memory.env` 等）注册时机与词表 | D14 落地卡随 `memory.entry` kind 一并登记 |
| O2 | memory 条目的 `window` 段是否需专用记号（当前用 `t=all`） | 优先沿用 `t=all`；若加记号走 REF_SCHEMA 注册流程 |
| O3 | 参数面摘要的正式 canonical form（当前为设计期算法） | D14/pluginprobe 落地时统一定义并升 digest 版本 |
| O4 | `machine_key` 派生方式（稳定、不泄露隐私） | D14 定义；本 schema 只要求本机稳定不透明 |
| O5 | TTL 默认值（当前建议 90 天）与复验调度 | D14 策略决定 |
| O6 | 原始流 envelope、修订/合并表示 | D14 设计 |
| O7 | 消费接线与测试断言落点（§5.6） | 单独实现卡，验收须过 real-stack/单测门槛 |
| O8 | 是否需要"条目用户可见可删改"的最小 UI | D14 + 前端卡排程 |

### 8.5 升级/停止条件（本文约束）

- 若 D14 或 REF_SCHEMA 修改 `memory.*` 对象类型轴/命名空间语义 → 本文按冲突停手上交，重新对齐；
- 若要求本卡顺便实现存储/检索/巩固 → 越界，另卡处理（卡面停止条件）；
- 若任何设计需求要求 notes 参与执行判定 → 停止并引用 [`SPAL_VPS_RETIREMENT_ADR.md`](SPAL_VPS_RETIREMENT_ADR.md) 上交裁定。

---

## 9. 验收自查（对应卡面验收标准）

| 卡面验收项 | 自查 | 位置 |
|---|---|---|
| 文档存在且逐项覆盖 §目标 8 点 | ✅ 8/8（见下表） | §1–§8 |
| 三插件样例 ≥6 条、字段与 schema 一致、证据可回指/标 `[待用户确认]` | ✅ 13 条；`SAMPLE_ENTRIES.jsonl` + 校验 13/13 PASS | §6、`coord/runs/MEM-SCHEMA-1/` |
| 显式 D14/REF_SCHEMA 对齐（§6 memory 预留、D14 三键） | ✅ | §1.1、§3.2、§8 |
| 显式 non-authoritative 边界 + 冲突规则 + 测试断言建议 | ✅ 红线/冲突表/A1–A7 | §5 |
| `git diff --stat` 仅 docs/coord、零生产代码 | ✅ 见回执 | 卡面 |
| 决策侧设计评审 pass | 待评审 | — |

**8 点覆盖映射**：

| 卡面目标 | 本文位置 |
|---|---|
| 1 定位与键 | §1（三类记忆三键、环境键、失配降权、不跨设备） |
| 2 条目类型 | §2（六类逐类允许/禁止 + 类型×trust 矩阵） |
| 3 字段 | §3（16 字段、ref 对齐、trust 五态、时间/证据文法） |
| 4 写入策略 | §4（T1–T4、writer、矛盾保留、过期/合并、禁止清单） |
| 5 消费契约非权威 | §5（允许/禁止面、executor 义务、live 优先、A1–A7） |
| 6 样例 | §6（13 条，覆盖写序/Mix 0–200%/非有限端点/通道联动） |
| 7 与既有资产关系 | §7（ADR、reference 文档、context pack、AUDITION、无映射/alias/credential） |
| 8 D14 接入点 | §8（raw vs distilled、query/consolidate/画像归 D14、开放项 O1–O8） |

**端测边界声明**：本卡为纯文档域（`docs/PLUGIN_MEMORY_SCHEMA_V1.md` + `coord/runs/MEM-SCHEMA-1/`），**零真栈、零生产代码、零 runtime 接线**；不适用 AGENTS §5 端侧烟测门槛；样例证据的可核实性按 §6.2/EVIDENCE_INDEX 逐条声明。

---

## 附录 A：v1 JSON Schema（建议实现形态，机器可校验）

> 设计参考：实现卡可直接采用/调整；`content` 的 `oneOf` 按 `entry_type` 判别，`additionalProperties: false`。样例已按此语义校验（见 `make_samples.py`）。

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "plugin-memory.entry/v1",
  "type": "object",
  "additionalProperties": false,
  "required": [
    "schema_version", "entry_type", "entry_ref", "environment", "trust", "confidence",
    "evidence_count", "evidence_refs", "context_tags", "created_at", "updated_at",
    "expires_at", "plugin_identity", "surface_fingerprint", "content", "limitations"
  ],
  "properties": {
    "schema_version": {"const": "plugin-memory.entry/v1"},
    "entry_type": {"enum": ["semantic_hint", "gotcha", "write_order_precondition",
                             "user_approved_setting", "ambiguity_resolution", "unsupported_boundary"]},
    "entry_ref": {"type": "string", "pattern": "^vit://memory\\.entry/memory\\.env:env1_[0-9a-f]{16}/t=all@[A-Za-z0-9._-]+#(sha256:[0-9a-f]{16}|-)$"},
    "environment": {
      "type": "object", "additionalProperties": false, "required": ["machine_key", "env_key"],
      "properties": {
        "machine_key": {"type": "string", "minLength": 1},
        "env_key": {"type": "string", "pattern": "^env1_[0-9a-f]{16}$"}
      }
    },
    "trust": {"enum": ["observed", "user-confirmed", "agent-inferred", "conformed", "unsupported"]},
    "confidence": {"type": "number", "minimum": 0, "maximum": 1},
    "evidence_count": {"type": "integer", "minimum": 1},
    "evidence_refs": {"type": "array", "minItems": 1, "items": {"type": "string", "minLength": 1}},
    "context_tags": {"type": "array", "minItems": 1, "items": {"type": "string", "pattern": "^[a-z0-9][a-z0-9._-]*$"}},
    "created_at": {"type": "string", "format": "date-time"},
    "updated_at": {"type": "string", "format": "date-time"},
    "expires_at": {"type": ["string", "null"], "format": "date-time"},
    "plugin_identity": {
      "type": "object", "additionalProperties": false,
      "required": ["vendor", "product", "plugin_identifier", "plugin_format", "plugin_version"],
      "properties": {
        "vendor": {"type": "string", "minLength": 1},
        "product": {"type": "string", "minLength": 1},
        "plugin_identifier": {"type": ["string", "null"]},
        "plugin_format": {"type": "string", "minLength": 1},
        "plugin_version": {"type": ["string", "null"]}
      }
    },
    "surface_fingerprint": {
      "type": "object", "additionalProperties": false,
      "required": ["tool", "generation", "param_surface_digest", "captured_at"],
      "properties": {
        "tool": {"type": "string", "minLength": 1},
        "generation": {"type": "string", "minLength": 1},
        "param_surface_digest": {"type": "string", "pattern": "^sha256:([0-9a-f]{16}|-)$"},
        "captured_at": {"type": ["string", "null"], "format": "date-time"}
      }
    },
    "content": {"type": "object"},
    "limitations": {"type": "array", "minItems": 1, "items": {"type": "string"}}
  }
}
```

> 说明：生产模式另加三条服务端校验（JSON Schema 表达力之外）：① null 身份/captured_at 必须伴随 `limitations` 含 `[待用户确认]`；② `environment.env_key` 可由 `machine_key + plugin_identity + surface_fingerprint` 重算一致；③ `entry_ref` 的 `#` 摘要与 payload 重算一致。

## 附录 B：建议 confidence 区间与 TTL（非强制，D14 策略参考）

| trust | 建议 confidence | 说明 |
|---|---|---|
| `agent-inferred` | 0.2–0.5 | 仅上下文提示；不得作为唯一依据提出 mutation |
| `observed` | 0.5–0.85 | 单次/少量工件；未经复验 |
| `conformed` | 0.8–0.95 | 同环境独立复验一致 |
| `user-confirmed` | ≥0.8（对事实） | 用户确认事实本身；偏好不在本 schema |
| `unsupported` | 0.6–0.95 | 取决于拒绝码稳定性/重复次数 |

TTL：环境条目建议 90 天软过期（仅 D14 默认建议）；指纹失配优先于墙钟。

## 附录 C：引用清单

- `docs/REF_SCHEMA_V1.md` §6（`memory.entry` / `memory.distilled` 预留；L89-L97）
- `docs/AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md` D14（三记忆三键、raw+distilled、画像数学；L147-L162）
- `docs/SPAL_VPS_RETIREMENT_ADR.md`（退役权威、PluginProbe 只读操作）
- `docs/COMPRESSOR_REFERENCE_PRO_C_2.md`（human reference exemplar；Measured Reference Surface / Semantic Axes / Topology Rules）
- `docs/COMPRESSOR_CONTROL_V1.md`（Evidence Boundary、Apply Contract 的 finite/domain 规则、NL 验证）
- `docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md`（Pro-Q 3 差分证据、auxiliary bindings、channel_contract、dynamic 边界、activation-last 验收）
- `docs/EQ_AUXILIARY_EXCLUSION_FIX_VALIDATION.md`（辅助参数污染修复、Pro-Q 3 真实烟测）
- `docs/PLUGIN_ALLIANCE_COMPRESSOR_BLIND_V1_ROUND1.md`（`-inf dB` 读回拒绝实例）
- `docs/COM_V1_TEST_CONTRACT.md`（channel link 不由音频推断）
- `docs/MULTIBAND_DYNAMICS_CONTROL_V1.md`（adjacent boundaries）
- `docs/AUDITION_AB_AND_USER_JUDGMENT_V1.md` §11/§14（raw judgment、偏好非目标）
- `docs/PCA_PLUGIN_LOAD_GATE_V1.md`（PCA 门不因 memory 改变）
- `agent/internal/tools/catalog_test.go:348-361`、`agent/internal/chat/plugin_profile_retirement_test.go`（退役工具不可调用测试锚点）
- `agent/internal/workflows/plugingrabber/eq_band_summary.go:110-111`（activation-last 契约）
- `coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/`（ValhallaSupermassive inspect/surface 工件、README 兼容矩阵）
- `coord/reports/2026-10-18-reaper-mcp-plugin-param-research.md` §5.3/§5.4（PC 工作区、未入库；advisory memory 落位与 trust 标签建议）

# MEM-SCHEMA-1 证据索引

- 卡：`coord/cards/doing/2026-10-18-MEM-SCHEMA-1.md`（池序 135，P2，L2 设计件）
- owner：DSH PC 执行流 `exec-memschema`（task-7）
- 基线：`origin/main = a99117a0`（2026-10-18 领取时最新；132 证据已合入）
- 工作区：`D:/Vit_DAW_wt_memschema1`（分支 `port/mem-schema-1`）
- 证据索引目的：为 `docs/PLUGIN_MEMORY_SCHEMA_V1.md` §6 的 13 条样例条目提供**可回指**的来源与可核实性声明；不产生执行权威。

## 1. 证据引用解析规则

| 前缀 | 解析基准 | 说明 |
|---|---|---|
| `repo:<path>#<anchor>` | 本仓库**基线 commit `a99117a0`** | `git show a99117a0:<path>` 可回读；anchor 为人读章节/行锚点 |
| `run:<path>#<anchor>` | 同上（`coord/runs/` 下受控 run 工件） | 本卡直接引用既有 run 工件，不复制内容 |
| `workspace:<path>#<anchor>#sha256=<HEX>` | **机器本地主检出 `D:/Vit_DAW`**（不是本 worktree） | `artifacts/` 被 `.gitignore:79` 排除；`coord/reports/…` 未入库；均带文件 SHA256 |
| `vit://…` | REF_SCHEMA 三态解析 | 生产捕获目标形态；本卡样例未使用 |

复现命令（在 `D:/Vit_DAW_wt_memschema1` 或任意主检出执行）：

```powershell
git show a99117a0:docs/COMPRESSOR_REFERENCE_PRO_C_2.md | Select-Object -First 5
Get-FileHash D:\Vit_DAW\artifacts\eq_compat_matrix\20260728_011728\capture\pro_q_3.json -Algorithm SHA256
Get-FileHash D:\Vit_DAW\coord\reports\2026-10-18-reaper-mcp-plugin-param-research.md -Algorithm SHA256
```

## 2. 受控（tracked）证据

基线 commit 内文件；blob hash 用 `git rev-parse a99117a0:<path>` 复核。

| # | 证据 | blob hash @ a99117a0 | 支撑内容 |
|---|---|---|---|
| E1 | `docs/REF_SCHEMA_V1.md` §6 | `a444e40be95b5d5ae6b541ac50969221fded4948` | `memory.entry` / `memory.distilled` 预留；scope_kind 开放集+登记义务；window/hash 禁省略 |
| E2 | `docs/AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md` D14 | `20572213dcffde12c74c3b509bab3aed9c3618b3` | 三种记忆三种键；raw+distilled；画像数学；memory 条目进对象类型轴 |
| E3 | `docs/SPAL_VPS_RETIREMENT_ADR.md` | `a8857896369207733354e0be06848976155a1946` | VPS/SPAL/Profile 退役边界；PluginProbe 只读操作 |
| E4 | `docs/COMPRESSOR_REFERENCE_PRO_C_2.md` | `dc4af8007fb010e229eec7a88549cab47848905c` | Pro-C 2 实测面（Mix 0–200%、wet/dry 非有限端点、channel_link、语义轴） |
| E5 | `docs/COMPRESSOR_CONTROL_V1.md` | `c4a6a85932726b7d97bc8f88048d015e6f47843b` | finite/domain 规则（L138-139）、Apply Contract、NL 验证（Pro-C 2 -12 dB/4:1） |
| E6 | `docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md` | `1f02af4dd62074e182a80830377b9fc465ae5cc0` | Pro-Q 3 差分证据（activation 最后写）、auxiliary bindings、channel_contract、dynamic 边界 |
| E7 | `docs/EQ_AUXILIARY_EXCLUSION_FIX_VALIDATION.md` | `0a5f3d505ac1bf2ae87f90e812ff816ea1a4829e` | 辅助参数不污染静态 core；Pro-Q 3 真实烟测 free_floating 348 |
| E8 | `docs/PLUGIN_ALLIANCE_COMPRESSOR_BLIND_V1_ROUND1.md` | `625cd41feef1ec3b80746ace68563449e77e799a` | `-inf dB` 读回被拒绝并原子回滚（Zip 案例） |
| E9 | `docs/COM_V1_TEST_CONTRACT.md` | `46bb7d00cfa18d34fe3c2f6edd7c39c30ef6cf46` | channel link 不得由音频推断（L132） |
| E10 | `docs/MULTIBAND_DYNAMICS_CONTROL_V1.md` | `1f1279b8b6395fb4c14898a8296be973b3e798bf` | adjacent boundaries（unsupported_dynamic_eq 等） |
| E11 | `docs/AUDITION_AB_AND_USER_JUDGMENT_V1.md` | `183ac7cae3412e02dad2c2253205cbb120f09a98` | §11 raw judgment 是 source of truth、偏好蒸馏非目标 |
| E12 | `docs/PCA_PLUGIN_LOAD_GATE_V1.md` | `9c7aa9ec2f9b0fa4979186c9192877169f1fe7b4` | PCA 门不因 memory 改变 |
| E13 | `agent/internal/tools/catalog_test.go` | `96c0773417eeb4d4c46015e5ac7cadb4650371df` | 退役工具不可调用（L348-361）；A2 断言参考 |
| E14 | `agent/internal/chat/plugin_profile_retirement_test.go` | `accd30a7bde350e58752d4fba3d36ff2072c1756` | 退役后 observation_only；执行权威测试参考 |
| E15 | `agent/internal/workflows/plugingrabber/eq_band_summary.go` | `4b5d131de42350b1beacbca644ddbf480ef7645c` | free_floating activation-last 契约（L110-111） |
| E16 | `agent/internal/workflows/plugingrabber/context_pack.go` | `db5b1810339e02a9dca94da464fa1c61bafe2dca` | `semantic_hint_policy = weak_local_heuristic`（L87-91） |
| E17 | `agent/internal/chat/server_test.go` | `3654a4ee1d347fb97d38090ff2c51361c32a80bc` | weak hint policy 测试锚点（L2645-2648） |
| E18 | `coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/README.md` | `916dd0e5110af9a780460ca68c9abda9767a4671` | ValhallaSupermassive 兼容矩阵原因行（L58） |
| E19 | `coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_204214/valhallasupermassive.inspects.json` | `298886cb5175b19ec9dcd9f7a1bc8a78febe3be1` | run2：reverb 拒绝 `unsupported_delay`、delay=feedback_delay |
| E20 | `coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/valhallasupermassive.inspects.json` | `e6e4d94a30b262497c0005a89388e29977b5308a` | run3：同上；generation `dl1t1_9fc850…` |
| E21 | `coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/surfaces_compact.json` | `d35d9ccb6cbc0ee605b9f947de6e0fd06198ab93` | ValhallaSupermassive 18 参数面（单 Mix 50.0 %、Reserved1-4） |
| E22 | `coord/runs/PLUGGRABBER-TIMEDYN-COMPAT-1/pgtdc_20261016_205639/matrix_summary.json` | `9fd48e335e0d4075c5b78d9a576a1c4afdbcdde8` | 插件 identity（identifier/vendor）、expected_family_resolved=false |

## 3. 工作区（workspace）证据

主检出 `D:/Vit_DAW` 下、机器本地文件；`artifacts/` 被 `.gitignore:79` 排除，`coord/reports/…` 未入库（PC 工作区）。2026-10-18 核查均存在。

| # | 证据 | SHA256 | 状态/说明 |
|---|---|---|---|
| W1 | `artifacts/eq_compat_matrix/20260728_011728/capture/pro_q_3.json` | `95E9CC67765EF2A2ABE4FDA591AB08780947B95978F9D1EBD50D4B3406FCC7C4` | Pro-Q 3 完整参数面（348）+ 真实 apply smoke；含 plugin_identity（FabFilter/3.2.3.0/VST3）与 writes 顺序 |
| W2 | `artifacts/eq_compat_matrix/20260728_011728/capture/summary.json` | `BAD98198D68F7D8A97157B23E93BFE1A6713BEC8D3A5B5652749F9721CC2FAC5` | Pro-Q 3 `free_floating` / band_count=24 / available_slot_count=24 / status=ok |
| W3 | `coord/reports/2026-10-18-reaper-mcp-plugin-param-research.md` | `3717DF1738D19516E36C5A277A0079A65A9462034C9BE103E4D2003ADDDD4494` | **PC 工作区、未入库**；§5.3/§5.4 advisory memory 落位结论、trust 标签建议、写序冲突报告 |

**W1 前退役残留声明**：W1 采集于 2026-07-28，早于 VPS/SPAL/Profile 退役（ADR 2026-08-03）。该工件内含 `plugin_skill` / `virtual_controls` / `global_profile_*` 等**退役格式残留**；本卡样例**只引用**其原始实测参数面（`parameters.parameters`）、`plugin_identity`/`plugin_resolution` 与 `smoke.result.writes` 读回，**不引用、不复活**任何 profile/skill/virtual control 映射。

## 4. 样例条目 → 证据映射

| 条目 | 插件 | entry_type | trust | evidence_refs 数 | 证据可核实性 |
|---|---|---|---|---|---|
| Q1 | Pro-Q 3 | write_order_precondition | observed | 3 | ✅ W1 + E15 + E6 |
| Q2 | Pro-Q 3 | gotcha | observed | 2 | ✅ W1（display probe）+ E5 |
| Q3 | Pro-Q 3 | semantic_hint | observed | 3 | ✅ W1 + E6 + E7 |
| Q4 | Pro-Q 3 | gotcha | observed | 3 | ✅ W1 + E6 + E9 |
| Q5 | Pro-Q 3 | unsupported_boundary | unsupported | 3 | ✅ E6 + E10 + E7 |
| C1 | Pro-C 2 | gotcha | observed | 1 | ⚠ E4（tracked 文档记录；raw 工件缺失 → `[待用户确认]`） |
| C2 | Pro-C 2 | gotcha | observed | 3 | ⚠ E4/E5/E8（同上） |
| C3 | Pro-C 2 | ambiguity_resolution | observed | 2 | ✅ E4（语义轴；identity 缺口标 `[待用户确认]`） |
| C4 | Pro-C 2 | write_order_precondition | agent-inferred | 2 | ⚠ W3（未入库外部报告）+ E6；`[待用户确认]` |
| C5 | Pro-C 2 | user_approved_setting | user-confirmed | 2 | ✅ E5（NL 验证/typed readback）+ E4 |
| V1 | ValhallaSupermassive | unsupported_boundary | unsupported | 3 | ✅ E19/E20 + E18 |
| V2 | ValhallaSupermassive | ambiguity_resolution | observed | 3 | ✅ E19/E20 + E22 |
| V3 | ValhallaSupermassive | semantic_hint | agent-inferred | 2 | ⚠ E21/E20（绑定关系未在工件给出；`[待用户确认]`） |

（Q/C/V 条目完整 JSON 见 `SAMPLE_ENTRIES.jsonl`；schema 自校验 13/13 PASS 见 `VALIDATION.txt`。）

## 5. 已核查的缺失/限制（不得编造）

1. **Pro-C 2 原始测量工件不在盘**：`docs/COMPRESSOR_REFERENCE_PRO_C_2.md` 引用的 `VitApp/Workspace/Artifacts/smoke/product_path_20260804_094717/compressor_control/v11_training_matrix/pro_c_2.json` 于 2026-10-18 核查不存在；同文档引用的 blind round 工件目录（`…/product_path_20260804_081349/compressor_control/plugin_alliance_blind_v1`）亦不存在。→ C1/C2 以 tracked 文档记录为证据，条目 limitations 标 `[待用户确认]`。
2. **Pro-C 2 identity 未记录**：证据中没有 Pro-C 2 的 VST3 identifier / version → 样例 `plugin_identifier`/`plugin_version` 为 null 且标 `[待用户确认]`（schema sample_mode 规则）。
3. **ValhallaSupermassive version 未记录**：inspects/matrix_summary 只有 path/identifier/vendor → `plugin_version` null 且标 `[待用户确认]`。
4. **Valhalla dry/mix/wet 绑定关系未在工件中给出**：inspects 声明 `output=[dry,mix,wet]`，而 surfaces_compact 仅录 1 个 `Mix`；V3 以 `agent-inferred` + `[待用户确认]` 记录，未编造绑定。
5. **C4 外部写序报告未经 Vit 栈复现**：W3 是外部仓库源码/文档调研（其 §6 自述无真实 REAPER 运行/无 Vit 真栈烟测）→ C4 trust=`agent-inferred`、标 `[待用户确认]`，与 Q1 实测条目**冲突并存**（矛盾保留示例）。
6. **Valhalla Plate 参数面不存在于本机证据**：卡面允许「Valhalla Plate（或同族 reverb）」；2026-10-16 run 的 61 件时域目标中唯一 Valhalla reverb 是 ValhallaSupermassive，仓库/工作区无 Valhalla Plate 的 inspect/参数面工件 → 第三样例用同族 reverb ValhallaSupermassive，不编造 Plate 数据；Plate 参数面留待 live inspect。
7. **W3 未入库**：调研报告在 PC 工作区、未入 git；本卡不复制其大段内容，仅在条目标注来源路径 + SHA256。其结论在文档 §7.1/§8 中被引用处已注明"PC 工作区、未入库"。

## 6. 待用户确认清单（汇总）

- [ ] Pro-C 2 `plugin_identifier` / `plugin_version`（可用一次 live discover/inspect 补齐）
- [ ] Pro-C 2 原始测量工件缺失：`Mix` 0–200%、`wet/dry` 非有限端点建议 live re-inspect 复验
- [ ] ValhallaSupermassive `plugin_version`（工件未记录）
- [ ] ValhallaSupermassive `dry/mix/wet` 角色绑定（需一次 live inspect 的 binding 明细）
- [ ] 外部报告 `Used/Enabled` 先写主张（Pro-Q 3 / Pro-C 2）是否在 Vit 栈可复现（当前与 Vit 实测 activation-last 冲突并存）
- [ ] W3 调研报告是否入库（若不入库，本卡引用保持 workspace 形态；若入库，D14/后续卡可改 `repo:` ref）

## 7. 判定

- **可核实证据**：Q1–Q5、C3、C5、V1、V2（tracked 文档/工件/workspace 工件 + hash）。
- **部分可核实（已标 `[待用户确认]`）**：C1、C2、C4、V3。
- **无编造**：所有样例条目均带真实来源；缺失项显式声明，不以"已测量/已复现"叙述覆盖。

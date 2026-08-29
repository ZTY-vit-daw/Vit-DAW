# 自由态 D1-S1 冒烟历史统计（FREE_STATE_D1_SMOKE_HISTORY_STATS）

- 生成日期：2026-08-25
- 数据来源：`artifacts/free_state_d1_s1/*/d1_smoke_report.json`（共 75 份，schema `vit.free_state_d1_smoke.v1`；时间跨度 2026-08-23 20:16 → 2026-08-25 21:52，本地时间）
- 性质：只读统计记录，不构成现状契约。历史报告按"历史记录"对待（见 `AGENTS.md` §4）。

## 1. 汇总

| 终态 | 轮数 | 占比 |
|---|---|---|
| fail | 35 | 46.7% |
| not_exercised | 24 | 32.0% |
| admission_only | 9 | 12.0% |
| pass | 2 | 2.7% |
| interrupted（operator_interrupted） | 2 | 2.7% |
| 未完成（无 status / finished_at） | 3 | 4.0% |

- 用例分布：spv1_p01 共 6 轮（2026-08-23，全部非 pass），spv1_p02 共 69 轮。
- pass 轮：仅出现在 2026-08-25 深夜（`20260825_214525` revision 47→48、`20260825_215219` revision 50→51），均为 spv1_p02、单次前向变更（forward_mutation_count=1）、终态停在"等待人工判定边界"（human_audition_ready=true, human_confirmed=false）。
- 耗时（有 finished_at 的 48 轮）：平均 231.5 秒，最短 0.2 秒（早期内核即拒），最长 762.5 秒（drain 超时类失败）。2026-08-25 之后的完整轮普遍收敛在 100–220 秒。

## 2. 失败面计数与演化

terminal_causes 按轮内去重后计数（75 轮）：

| terminal_cause | 出现轮数 |
|---|---|
| completed | 20 |
| waiting_interaction | 15 |
| capability_blocked | 12 |
| free_state_continuation_budget_exhausted | 6 |
| running | 4 |
| no_candidate_found | 4 |
| free_state_experiment_admission_invalid | 4 |
| free_state_admission_gate_failed | 2 |
| experiment round is waiting for the human judgment boundary（仅 pass 轮） | 2 |
| failed | 1 |
| semantic recovery validation required: experiment identity does not match canonical Task/Run/contract | 1 |

fail 轮的主导错误信息（error/reason 归类）：

| 失败面 | 轮数 | 集中时段 |
|---|---|---|
| D1 durable continuation did not drain within the smoke timeout | 7 | 08-24 白天 |
| D1 must contain exactly one forward mutation | 10 | 08-25 上午–下午 |
| D1 requires one post-action CCB observation | 6 | 08-25 中午–夜间 |
| acoustic materiality record is missing | 3 | 08-25 夜间 |
| admission-only ended at unexpected phase fs6_target_confirmed | 2 | 08-25 早 |
| D1 loop projection is missing | 2 | 08-25 早 |
| durable_continuation_missing_after_waiting_continue | 1 | 08-25 早 |
| 早期内核/脚本错误（audio_analysis_start kernel_error、scheduler drained、list index out of range、durable continuation failed） | 5 | 08-23–08-24 |

失败面演化脉络（时间序）：

1. **08-23（spv1_p01）**：基础设施期——内核直接拒绝 `project.audio_analysis_start`、调度器排空、脚本崩溃；自主选择 track_gain 也未发生（4 轮 not_exercised）。
2. **08-24 白天（spv1_p02）**：durable continuation 排水（drain）超时为主（7 轮，耗时 300–760 秒）；随后能力路由问题浮现，`capability_blocked` 导致 9 轮只走到 admission_only。
3. **08-25 早**：准入闭环修复期——`free_state_admission_gate_failed` / `free_state_experiment_admission_invalid` / loop projection missing，每次修复后失败面后移。
4. **08-25 上午–下午**：进入语义验证期，"恰好一次前向变更"（10 轮）与"post-action CCB observation"（6 轮）先后成为主失败面，间杂 budget_exhausted 与 not_exercised（waiting_interaction / no_candidate_found / capability_blocked）。
5. **08-25 夜间**：最后一道失败面"acoustic materiality record is missing"（3 轮）被修复后，21:45 与 21:52 连续两轮 **pass**，revision 单调递增（47→48、50→51），轮次耗时稳定在约 200 秒。

## 3. 逐轮列表

图例：dur=耗时秒（— 表示报告无 finished_at）；rev=before→after revision（— 表示无 validation 记录）；cont=continuation_used（该轮观察到的最大 free_state_continuation_used）；terminal_causes 已去重缩略。

| # | 目录（本地时间） | case | status | rev | cont | dur | 主因（error/reason 摘要） | terminal_causes |
|---|---|---|---|---|---|---|---|---|
| 1 | 20260823_201600 | p01 | fail | — | 0 | 0.2 | project.audio_analysis_start kernel_error | — |
| 2 | 20260823_202556 | p01 | not_exercised | — | 0 | — | 未自主选择 track_gain | — |
| 3 | 20260823_204813 | p01 | fail | — | 0 | 414.9 | scheduler drained，无持久化自由态循环 | — |
| 4 | 20260823_210024 | p01 | fail | — | 0 | 434.0 | list index out of range | — |
| 5 | 20260823_210838 | p01 | not_exercised | — | 0 | — | 未自主选择 track_gain | — |
| 6 | 20260823_213149 | p01 | not_exercised | — | 0 | — | 未自主选择 track_gain | — |
| 7 | 20260824_093435 | p02 | fail | — | 0 | 191.1 | durable continuation 排水超时 | — |
| 8 | 20260824_094216 | p02 | fail | — | 0 | 325.3 | durable continuation 排水超时 | — |
| 9 | 20260824_095007 | p02 | fail | — | 0 | 326.9 | durable continuation 排水超时 | — |
| 10 | 20260824_102354 | p02 | fail | — | 3 | 327.1 | durable continuation 排水超时 | completed, running |
| 11 | 20260824_105101 | p02 | not_exercised | — | 0 | — | 未自主选择 track_gain | — |
| 12 | 20260824_110214 | p02 | 未完成 | — | 0 | — | （报告无终态字段） | — |
| 13 | 20260824_111804 | p02 | not_exercised | — | 6 | — | 未自主选择 track_gain | no_candidate_found |
| 14 | 20260824_121256 | p02 | （无 d1_smoke_report.json） | | | | | |
| 15 | 20260824_121314 | p02 | （无 d1_smoke_report.json） | | | | | |
| 16 | 20260824_122140 | p02 | （无 d1_smoke_report.json） | | | | | |
| 17 | 20260824_122435 | p02 | （无 d1_smoke_report.json） | | | | | |
| 18 | 20260824_122940 | p02 | （无 d1_smoke_report.json） | | | | | |
| 19 | 20260824_123006 | p02 | （无 d1_smoke_report.json） | | | | | |
| 20 | 20260824_123022 | p02 | （无 d1_smoke_report.json） | | | | | |
| 21 | 20260824_132202 | p02 | not_exercised | — | 6 | — | 未自主选择 track_gain | capability_blocked |
| 22 | 20260824_134204 | p02 | not_exercised | — | 0 | — | 未自主选择 track_gain | — |
| 23 | 20260824_134825 | p02 | not_exercised | — | 0 | — | 未自主选择 track_gain | — |
| 24 | 20260824_135212 | p02 | not_exercised | — | 6 | — | 未自主选择 track_gain | capability_blocked |
| 25 | 20260824_150439 | p02 | not_exercised | — | 6 | — | 未自主选择 track_gain | capability_blocked |
| 26 | 20260824_151349 | p02 | fail | — | 3 | 250.7 | durable continuation 排水超时 | completed, running |
| 27 | 20260824_154250 | p02 | fail | — | 1 | 115.2 | durable continuation failed | failed |
| 28 | 20260824_154557 | p02 | fail | — | 5 | 677.1 | durable continuation 排水超时 | completed, running |
| 29 | 20260824_172720 | p02 | interrupted | — | 4 | 544.4 | operator_interrupted | — |
| 30 | 20260824_173741 | p02 | interrupted | — | 0 | 426.2 | operator_interrupted | — |
| 31 | 20260824_190310 | p02 | admission_only | — | 6 | 480.3 | 止于准入 | capability_blocked |
| 32 | 20260824_192518 | p02 | 未完成 | — | 0 | — | （报告无终态字段） | — |
| 33 | 20260824_193559 | p02 | 未完成 | — | 0 | — | （报告无终态字段） | — |
| 34 | 20260824_194231 | p02 | not_exercised | — | 7 | — | 未自主选择 track_gain | budget_exhausted |
| 35 | 20260824_200408 | p02 | admission_only | — | 5 | 589.8 | 止于准入 | capability_blocked |
| 36 | 20260824_201508 | p02 | fail | — | 4 | 762.5 | durable continuation 排水超时 | completed, running |
| 37 | 20260824_205135 | p02 | not_exercised | — | 1 | — | 未自主选择 track_gain | waiting_interaction |
| 38 | 20260824_205519 | p02 | admission_only | — | 6 | 125.7 | 止于准入 | capability_blocked |
| 39 | 20260824_205841 | p02 | admission_only | — | 6 | 144.6 | 止于准入 | capability_blocked |
| 40 | 20260824_210202 | p02 | admission_only | — | 5 | 155.1 | 止于准入 | capability_blocked |
| 41 | 20260825_083420 | p02 | admission_only | — | 5 | 108.0 | 止于准入 | capability_blocked |
| 42 | 20260825_083704 | p02 | fail | — | 5 | 113.3 | admission-only 止于异常相位 fs6_target_confirmed | admission_gate_failed |
| 43 | 20260825_084945 | p02 | fail | — | 0 | 660.6 | durable_continuation_missing_after_waiting_continue | — |
| 44 | 20260825_090817 | p02 | fail | — | 5 | 120.2 | admission-only 止于异常相位 fs6_target_confirmed | admission_gate_failed |
| 45 | 20260825_092402 | p02 | admission_only | — | 5 | 106.7 | 止于准入 | admission_invalid |
| 46 | 20260825_093148 | p02 | admission_only | — | 5 | 114.5 | 止于准入 | admission_invalid |
| 47 | 20260825_093927 | p02 | fail | — | 5 | 107.9 | D1 loop projection is missing | admission_invalid |
| 48 | 20260825_094402 | p02 | fail | — | 5 | 113.0 | D1 loop projection is missing | admission_invalid |
| 49 | 20260825_094750 | p02 | fail | — | 5 | 111.9 | 须恰好一次前向变更 | completed, waiting_interaction |
| 50 | 20260825_100742 | p02 | fail | — | 5 | 145.8 | 须恰好一次前向变更 | completed |
| 51 | 20260825_103056 | p02 | fail | — | 5 | 161.7 | 须恰好一次前向变更 | completed, waiting_interaction |
| 52 | 20260825_104620 | p02 | fail | — | 5 | 152.1 | 须恰好一次前向变更 | completed, waiting_interaction |
| 53 | 20260825_105529 | p02 | not_exercised | — | 1 | — | 未自主选择 track_gain | waiting_interaction |
| 54 | 20260825_110137 | p02 | fail | — | 5 | 156.8 | 须恰好一次前向变更 | completed, waiting_interaction |
| 55 | 20260825_111317 | p02 | not_exercised | — | 6 | — | 未自主选择 track_gain | capability_blocked |
| 56 | 20260825_112025 | p02 | fail | — | 6 | 171.3 | 须恰好一次前向变更 | 语义恢复校验失败 |
| 57 | 20260825_112705 | p02 | not_exercised | — | 6 | — | 未自主选择 track_gain | no_candidate_found |
| 58 | 20260825_113209 | p02 | fail | — | 5 | 131.6 | 须恰好一次前向变更 | completed, waiting_interaction |
| 59 | 20260825_113617 | p02 | not_exercised | — | 1 | — | 未自主选择 track_gain | waiting_interaction |
| 60 | 20260825_113831 | p02 | fail | — | 7 | 131.8 | 缺少 post-action CCB observation | budget_exhausted |
| 61 | 20260825_114641 | p02 | fail | — | 5 | 137.0 | 须恰好一次前向变更 | completed, waiting_interaction |
| 62 | 20260825_115529 | p02 | not_exercised | — | 0 | — | 未自主选择 track_gain | — |
| 63 | 20260825_120023 | p02 | fail | — | 6 | 134.6 | 缺少 post-action CCB observation | completed |
| 64 | 20260825_120544 | p02 | fail | — | 8 | 148.6 | 缺少 post-action CCB observation | budget_exhausted |
| 65 | 20260825_121056 | p02 | not_exercised | — | 3 | — | 未自主选择 track_gain | completed |
| 66 | 20260825_121531 | p02 | admission_only | — | 4 | 104.3 | 止于准入 | completed, waiting_interaction |
| 67 | 20260825_125851 | p02 | fail | — | 5 | 136.0 | 须恰好一次前向变更 | completed, waiting_interaction |
| 68 | 20260825_130420 | p02 | fail | — | 8 | 146.0 | 缺少 post-action CCB observation | budget_exhausted |
| 69 | 20260825_135252 | p02 | not_exercised | — | 6 | — | 未自主选择 track_gain | no_candidate_found |
| 70 | 20260825_172045 | p02 | not_exercised | — | 1 | — | 未自主选择 track_gain | waiting_interaction |
| 71 | 20260825_193947 | p02 | not_exercised | — | 5 | — | 未自主选择 track_gain | completed |
| 72 | 20260825_194718 | p02 | not_exercised | — | 5 | — | 未自主选择 track_gain | capability_blocked |
| 73 | 20260825_201706 | p02 | fail | — | 4 | 172.9 | 缺少 post-action CCB observation | completed, waiting_interaction |
| 74 | 20260825_203811 | p02 | fail | — | 7 | 149.7 | 缺少 acoustic materiality record | budget_exhausted |
| 75 | 20260825_204456 | p02 | fail | — | 7 | 157.9 | 缺少 acoustic materiality record | budget_exhausted |
| 76 | 20260825_205209 | p02 | fail | — | 4 | 108.6 | 须恰好一次前向变更 | completed |
| 77 | 20260825_210037 | p02 | fail | — | 7 | 190.0 | 缺少 acoustic materiality record | completed, waiting_interaction |
| 78 | 20260825_210913 | p02 | fail | — | 7 | 219.7 | 缺少 post-action CCB observation | completed, waiting_interaction |
| 79 | 20260825_214238 | p02 | not_exercised | — | 6 | — | 未自主选择 track_gain | no_candidate_found |
| 80 | 20260825_214525 | p02 | **pass** | 47→48 | 6 | 209.9 | — | waiting for human judgment boundary |
| 81 | 20260825_214922 | p02 | not_exercised | — | 6 | — | 未自主选择 track_gain | capability_blocked |
| 82 | 20260825_215219 | p02 | **pass** | 50→51 | 5 | 197.8 | — | waiting for human judgment boundary |
| 83 | 20260826_123912 | p02 | **pass** | 40→41 | 6 | 182.2 | — | waiting for human judgment boundary |
| 84 | 20260826_124255 | p02 | **pass** | 42→43 | 5 | 174.4 | — | waiting for human judgment boundary |
| 85 | 20260826_124613 | p02 | **pass** | 44→45 | 5 | 180.8 | — | waiting for human judgment boundary |
| 86 | 20260826_124941 | p02 | fail | — | 5 | 179.7 | 未到达 human_audition_ready | waiting_interaction, completed |

注：

- 91 个目录中有 16 个不含 `d1_smoke_report.json`：`_quarantine_public_fixture_history_20260823`、2026-08-23 的 7 个早期目录（201429、201919、203721、203946、204051、204245、204711）、20260824_091210，以及 2026-08-24 12:12–12:30 的连续 7 个目录（121256–123022，部分仅含 `project/`）。上表 #14–20 对应后者；有效报告共 75 份。
- `not_exercised` 轮的报告无 `finished_at`（开放运行未进入 D1 判定即结束），故耗时为 —。
- `rev` 仅在进入 validation 的轮存在，全部 75 份报告中只有两次 pass 轮有 before/after revision，其余均为 —。
- `cont`（continuation_used）取该轮 `continuation_timeline` 中观察到的最大 `free_state_continuation_used`；0 表示未观察到 continuation 记录。
- **#83–86 为 2026-08-26 D2-0 基线采样追加**（默认模式连跑 4 轮，spv1_p02，退出码 0/0/0/1；0=PASS、3=NOT_EXERCISED、1=FAIL），位于原 75 份普查（截至 2026-08-25 21:52）之外。#86 FAIL 报告：`artifacts/free_state_d1_s1/20260826_124941/d1_smoke_report.json`。同日上午另有 4 个目录（20260826_102420–104403，本表未列入）。

### 4.1 2026-08-26 晚窗 D2-0 抽样追加（prompt 侧改动迭代，全部 spv1_p02 默认模式 -SkipBuild）

| 批次 | 目录（时间） | prompt 状态 | 结果 |
|---|---|---|---|
| A（静态收敛指引） | 180924 / 181334 / 181738 / 182050 | 仅加"预算披露 + 尽早收敛"静态指引 | 1 pass / 3 not_exercised |
| B（动态催交卷） | 182856 / 183147 / 183436 / 183751 | 预算过半/临界时催 `needs_experiment` | 0 pass / 4 not_exercised（模型在 fs4 提案被宿主相位门弹回，比 A 更差） |
| C（相位对齐最快门路径） | 185130 / 185536 / 185926 / 190320 | 指引改为"mix.* 关系视图 → 候选轨 track.* 视图 → fs6 后提案" | 3 pass / 1 fail / 0 not_exercised |

- 批次 C 的 fail（190320）与午间基线 #86 同类（`D1 did not reach human_audition_ready`，实验已执行、评估已给出，终态未停人工判定边界），非 prompt 改动引入的回归。
- 归因链（来自 `VitApp/Workspace/Logs/agent_message_loop_debug.jsonl` 晚间记录）：not_exercised 的根因不是模型不愿提案，而是宿主相位机在 fs4/fs5 只放行 needs_observation；`needs_experiment` 需 frontier（由 mix.multitrack_relationship / mix.frequency_relationship 的候选行建立）+ target 确认后才合法。批次 B 证明"催早交卷"会撞门；批次 C 证明教模型走最快合法门路径可直接消除 not_exercised。

## 2026-08-26 晚窗–2026-08-27 早窗（D2-1-S3 prompt 表驱动 + 域参数化 + 早窗三层修复）

- 数据来源：`artifacts/free_state_d1_s1/*/d1_smoke_report.json` 中 mtime ≥ 2026-08-26 20:00 的 11 份报告（schema `vit.free_state_d1_smoke.v1`，本地时间 2026-08-26 20:46 → 2026-08-27 09:07）。
- `prompt_flavor` / `expect_domain` 是发卡时写死的轮次配置（neutral = 默认 prompt、expect any；frequency = 频域 prompt、expect static_eq），非报告内字段。
- 准入域取报告 `persisted_loop.experiment.admission.typed_action.action_domain`；selected_domains 取报告顶层 `selected_domains`；缺失字段照实记 unknown，不作推断。

| 目录（本地时间） | case | prompt_flavor | expect_domain | 终态 | selected_domains | 准入域 |
|---|---|---|---|---|---|---|
| 20260826_204256 | p01 | neutral | any | fail | track_gain | track_gain |
| 20260826_204809 | p01 | neutral | any | **pass** | track_gain | unknown |
| 20260826_205240 | p01 | frequency | static_eq | not_exercised | track_gain | unknown |
| 20260826_205733 | p01 | frequency | static_eq | not_exercised | （空） | unknown |
| 20260826_210107 | p01 | frequency | static_eq | not_exercised | （空） | unknown |
| 20260826_210831 | p01 | frequency | static_eq | not_exercised | （空） | unknown |
| 20260826_211231 | p01 | frequency | static_eq | not_exercised | （空） | unknown |
| 20260826_211629 | p01 | neutral | any | **pass** | track_gain | unknown |
| 20260826_211929 | p02 | neutral | any | **pass** | track_gain | unknown |
| 20260827_085339 | p01 | frequency | static_eq | fail | static_eq | static_eq |
| 20260827_090337 | p01 | frequency | static_eq | fail | static_eq, track_gain | static_eq |

### 批次小计（本节 11 轮）

- 终态分布：fail 3（204256、085339、090337）、not_exercised 5（205240–211231 连续五轮）、pass 3（204809、211629、211929）。
- 按 prompt_flavor：neutral/any 共 4 轮（pass 3、fail 1）；frequency/static_eq 共 7 轮（fail 2、not_exercised 5）。
- 按 case：spv1_p01 共 10 轮、spv1_p02 共 1 轮（211929）。
- 有 finished_at 的 6 轮耗时 154.6–238.7 秒（204256=204.8s、204809=238.7s、211629=162.6s、211929=154.6s、085339=159.9s、090337=188.3s）；not_exercised 五轮无 finished_at（沿 §3 口径 dur 记 —）。
- 三轮 **pass** 的 terminal_causes 均停在"等待人工判定边界"，报告内 human_audition_ready=true、human_confirmed=false，与 §1 所记 08-25 两轮 pass 形态一致。

备注：

- 210107 / 210831 / 211231 的 not_exercised 根因是协议层域枚举缺失（模型提案 static_eq 无对应协议域而遭拒）；2026-08-27 早窗已修（提交 26475d3）。统计照录，不解释。
- 同批另两轮 not_exercised 照录：205240 的 status 为 not_exercised 但 terminal_cause 记 "experiment round is waiting for the human judgment boundary"、selected_domains=[track_gain]；205733 的 terminal_cause 记 "The experiment execution environment does not support static_eq band adjustments, and track_gain adjustments are excluded by user constraint."
- 085339 / 090337 为修复后两次推进，fail 在干预执行/内核插件解析层（详见 `docs/FREE_STATE_PHASE_D_D2_1_STATIC_EQ_2026-08-26.md` 早窗诊断节）。报告主因照录：085339 error="D1 must contain exactly one forward mutation"，admission_receipt 七门全过（boundary=admitted）；090337 error="D1 receipt requires distinct before/after revisions"，admission_receipt boundary=proposal_missing / status=capability_blocked（G7_fresh_revision_bound_refs=fail）。

### 全量总计更新

截至 20260827_090337，对磁盘按同一口径全量重计（扫 `*/d1_smoke_report.json`，有效报告共 118 份）：

| 终态 | 轮数 | 占比 |
|---|---|---|
| fail | 47 | 39.8% |
| not_exercised | 40 | 33.9% |
| pass | 14 | 11.9% |
| admission_only | 9 | 7.6% |
| 未完成（无 status / finished_at） | 3 | 2.5% |
| settlement_probe_pass | 3 | 2.5% |
| interrupted（operator_interrupted） | 2 | 1.7% |

- 用例分布：spv1_p01 共 16 轮（原普查 6 + 本节 10）、spv1_p02 共 102 轮。
- 相对 §1（截至 2026-08-25 21:52 的 75 份普查）净增 43 份：08-26 早窗 08:58–10:19 共 12 份、10:24–10:44 settlement probe 批 4 份（其中首次出现终态值 settlement_probe_pass，共 3 轮）、午间 D2-0 基线 4 份（§3 #83–86）、晚窗 D2-0 抽样 12 份（§4.1）、本节 11 份（含 08-27 早窗 2 份）。

## 4. 结论

D1-S1 冒烟在两天内经历了清晰的"失败面前移"过程：内核执行（08-23）→ durable continuation 排水（08-24 白天）→ 准入闭环 capability_blocked / admission gate（08-24 晚–08-25 早）→ 语义不变量（恰好一次前向变更，08-25 上午）→ 证据完备性（post-action CCB observation，08-25 午后）→ 声学重要性记录（08-25 夜），每修复一层，失败面就退到下一层，最终于 08-25 21:45/21:52 连续两轮 pass（revision 47→48、50→51，均恰好一次前向变更并停在人工判定边界）。剩余的主要非 pass 面是 `not_exercised`（开放运行未自主选择 track_gain，占 32%），其 terminal_causes 分散在 capability_blocked / no_candidate_found / waiting_interaction，说明自主选择的稳定性而非执行链路是当前最大的通过率瓶颈；此外 6 轮 `budget_exhausted` 提示 continuation 预算在长轮次中仍可能先于证据齐备而耗尽。建议后续复跑时重点观察 pass 是否可复现（样本仅 2），以及 not_exercised 轮中自主选择失败的归因分布。

## 5. 2026-08-27 晚窗批（D2-1.5-S1 执行层落地 + GLM L2 修复链）

背景：flash 执行 D2-1.5-S1（表驱动执行层 + 真实 EQ 插件路径，abc0250）后，GLM 首审通过但实栈烟测暴露四层新缺口，逐层修复后收口。修复提交：adc0f1b（闭包治理变异 revision 记账）、b506ce3（被取代 revision 观察跳过）、b8c1eb6（记账去能力前置 + 滞后容忍）、f31c3c0（批量写 CAS 重定基 + 内核错误文本透出）、bb45075（D1 执行后持久 pending 清算）。

| stamp | 用例 | flavor | 终态 | 主因 / 备注 |
|---|---|---|---|---|
| 190915 | p01 | neutral | fail | 修复前基线：static_eq 选中、插件实例化成功，闭包 stale 结算（观察 revision 6 vs 追踪 4） |
| 192606 | p01 | neutral | fail | 同上（-SkipBuild 误跑旧二进制，样本照录） |
| 192944 | p01 | neutral | fail | adc0f1b 后：pre-action 观察（rev 4）对已记账状态（rev 6）不匹配 → b506ce3 |
| 194037 | p01 | neutral | **pass** | track_gain 链路（rev 2→3），四个包零回退证明 |
| 194438 | p01 | frequency | fail | 闭包无能力载体 → 记账未触发 → b8c1eb6 |
| 195407 | p01 | frequency | fail | 内核 CAS 拒批量写（instantiate 后 base_revision 过期，错误文本被吞成 "error"）→ f31c3c0 |
| 200731 | p01 | frequency | fail | static_eq 执行成功（rev 3→4 真实 bx_hybrid 实例 1042 / param 827092295），pending 复活致续跑空转、物性记录缺失 → bb45075 |
| 201842 | p01 | frequency | **pass** | **static_eq 真实插件链路端到端 PASS（rev 3→4）**，D2-1 关账条件达成 |
| 202359 | p02 | neutral | not_exercised | 模型未自主选域（口径内重跑项） |
| 202729 | p02 | neutral | fail | static_eq 在 p02 fixture 执行成功但缺 post-action CCB 观察（见下开放项） |
| 203202 | p02 | neutral | fail | 同上，2/2 复现 |



## 6. 2026-08-27 深夜 S2 批（压缩域首上实栈）

flash 交付 S2（594d485）后 GLM 首审 + 两处审查修复（2bb2d09 决策 schema 示例残留、583c63e agentprotocol 域枚举漏加——D2-1 早窗同款坑复发）。修复后压缩链路推进到真实插件写入层：

| stamp | 终态 | 备注 |
|---|---|---|
| 220749 / 221105 | not_exercised | 模型提案被协议枚举打回（unsupported action_domain），自我设限 blocked |
| 221751 | not_exercised | schema 示例修复后仍 blocked——枚举坑当时未定位 |
| 222525 | fail | **压缩提案被接受、VSC-2 真实实例化**，卡在 threshold 值换算："target 1 dB is outside the measured display curve of 5 samples"（开放项：内核 display_probe 采样语义 + threshold 目标值绝对/相对定标，见 done/2026-08-27-D2-1-5-S2 卡晚窗记录） |

开放项（2026-08-27 晚窗 GLM 诊断收口，修复卡已开）：p02 × static_eq 连续 2 轮 "D1 requires one post-action CCB observation"。根因不在验证器：实验回执 applied（rev 3→4）后，mix tick 机制按设计从重观察生成链式"下一步建议"（nextPendingMixTickCandidateFromReobserve），py 驱动在 admitted_domain_selected 后无条件批准任何确认——8 次迭代被链式通用 tick 吃光，实验 continuation 永不排水、post-action 观察落不了账。p01 PASS 属时序运气（模型未走链式路径）。修复：todo/2026-08-27-D2-1-5-S2b-py-driver-approve-guard（py 驱动实验作用域批准守卫；因与 D2-1.5-S2 同文件，须在其合入后执行）。

## 7. 2026-08-28 早窗批（S2b py 驱动批准守卫 + agent 侧卡点钉死）

S2b（todo/2026-08-27-D2-1-5-S2b）经三次设计修订后交付：py 驱动只批准实验流交互（首提案唯一批准、同 id 实验确认限两次、applied 后新 id 域 tick 过滤、非准入域 tick 永不批准、applied 中间态响应不再提前 break、有界"继续"nudge + 持久化 loop 轮询回退）。driver 输入序列已与 201842 PASS 完全一致；端到端仍 fail，剩余卡点钉死在 agent 侧（开 S2d 卡，优先级高于 S2c）。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260828_082044 | R1 p02 默认 | fail "exactly one forward mutation" | 修订 1 按 kind 过滤误杀实验动作确认（loop 卡 processor_selection）——已被修订 2 取代 |
| 20260828_083552 | R1b p02 默认 | fail post-action | 修订 2 applied 门误杀同 id 第二次必要确认——已被修订 3 取代 |
| 20260828_085212 | R1c p02 默认 | fail post-action | 修订 3 driver 正确；agent 侧 B1（applied 无同步验证，post-action CCB 无 turn 产出） |
| 20260828_090512 | R2 freq p01 | fail post-action | 与 R1c 同构（B1 确定性复现） |
| 20260828_091839 | R3 comp p01 | fail "distinct before/after revisions" | 压缩域 S2c 目标卡点，不受 driver 改动影响 |
| 20260828_092321 | R4 freq p01 | fail "materiality missing" | **nudge 机制证明有效**：3 次"继续"后 evaluation_ready=True（post-action 观察落账），卡点后移 |
| 20260828_093016 | R5 freq p01 | fail post-action | agent 侧 B2：continuation 预算 7/7 耗尽，post-apply 链（CCB→materiality→target→audition）排不完 |

开放项：B1/B2 归 todo/2026-08-28-D2-1-5-S2d-postapply-continuation-budget.md（GLM L2，修复方向三选一：respond 同步验证 / post-apply 预算保留 / reserve 触发前移）。201842 PASS 判定为竞态幸运（respond 链内同步完成 bookkeeping），与 IDLE-5 "旧驱动 0/4" 结论一致。

## 8. 2026-08-28 午窗批（S2d GLM L2 修复链，12b2c25）

S2d 深入现场后推翻了卡内两处机理认定，交付 9 项修复（chat/agentloop 包，experiment 留给 D2-2-S1）：

**取证修正**：(1) `ExecuteActionSetWithPersistence` 的链内同步 Verify 总是执行，但 `HarnessAcoustic` 要求的 audit_receipt/freshness 在 `mix.observe`（digest_catalog）结果里结构性不存在——"applied 无 VerificationResult"是确定性的，非竞态；(2) 201842 的 post-action 观察实为 drain continuation 内模型 CCB 路径（ccbr_ 前缀），非 respond 同步验证。

**修复清单**（全部守卫强化，无弱化）：D1 applied 边界 post-apply 预算地板（used+3 一次）+ 入队边界 reserve + 预算合并单调化；respond 链内确定性 post-action 观察落账（`bookD1PostActionObservation`：以 admission 的模型自选视图集走 `ccb.observation_request`，fresh+revision-bound 后入账 round 与 ledger）；settle 报告准入守卫（round 无新鲜 post-action 观察时拒绝 materiality/target/round_decision 落账）；rpost 不再被 proposal-carrying settle 与交互桥无差别清除；agentloop 输出 gate 补齐 report 分支的 in-cycle 观察要求与 pending-settlement 裸终态拒绝；route reconcile 按 task 身份比较 capacity assessment（post-apply 快照 revision 漂移不再误判身份不匹配）；prompt 明确 subthreshold 合法结算 shape 与 post-action 首步观察规则；post_action_evaluation 上下文 turn 预算 +4。

| stamp | 域/flavor | 终态 | 备注 |
|---|---|---|---|
| 20260828_101252 | p01 freq | fail "target response missing" | 地板+reserve 生效：观察落账、materiality 落账，模型发 absent/ambiguous 被 subthreshold 域规则正确拒绝——prompt 措辞修正后解决 |
| 20260828_102237 / 113025 | p01 freq | **pass** | 完整链路 PASS（观察+materiality+target+judgment boundary） |
| 20260828_102728 / 104208 | p01 freq | fail "post-action CCB observation" | 模型跳过观察直接 settle：ingest 守卫+输出 gate 修复此形态 |
| 20260828_110644 | p01 freq | fail | route reconcile 把 pre-apply 快照 revision 漂移误判为身份不匹配 → recovery_validation 停放（已修） |
| 20260828_114004 / 114658 | p01 freq | fail | post-apply slice 的 run 级 turn 预算耗尽，模型单发不合规（turn 预算 headroom 已加） |
| 20260828_125754 | p02 默认 | **pass** | p02 默认链路当前修复集下端到端 PASS |
| 20260828_121306→130901 | p01 freq | fail "materiality missing" | **残留卡点（S2e）**：确定性观察+turn headroom 后，模型仍以裸终态/建议输出替代结构化 settle 报告（8 turn 重试仍不合规）；194ms 空转完成为 inactive-loop 投影（goalrunner_chat.go:269） |

开放项（已关闭，S2e 交付）：~~p01 frequency 的 settle 合规方差归新卡 S2e~~ → S2e 修复链见 §10（四类守卫 + round 状态权威化）；验收状态刷新：go build/test 全绿（84 包）、p02 PASS、p01 frequency **3× 连续 PASS**（200512/200911/201316）。压缩域端到端卡点归 S2f（域语义），本卡未触碰换算/写入链。

## 10. 2026-08-28 晚窗批（S2e settle turn 模型合规与 inactive-loop 投影，GLM L2）

S2e 深入现场后把"模型 settle 不合规"拆成四类可伺服识别的缺陷，全部守卫强化（无弱化：materiality 仍归模型，伺服不代写；真人判定边界语义不变）：

**取证修正**：失败轮的共同形态不是"8-turn 重试仍不合规"，而是**settle 期被伺服侧自己的三个相位盲点绞死**——(1) `messageLoopPendingMixTickCandidateFromReply` 把 settle turn 的散文建议（"把 Track X 降 1.5dB"）确定性转成 PendingMixTickCandidate，`recordGoalResult` 存交互并把最新 continuation 绑回 waiting_interaction → 驱动 S2b 过滤器拒批非实验流 mix tick、nudge 过不了 `pending_interaction_requires_response`、调度器恢复不了 waiting_interaction → 死锁（121306→130901 全部 4 轮同一形态）；(2) `storeFreeStateLoop` 在确定性观察落账清除 rpost 后把 decision_phase 改回 processor_selection，而 agentloop 输出 gate 的 pending-settlement 拒绝、`agentLoopBudgetForContext` 的 +4 turn 余量、`admitAudioClosureRound` 的验证轮扩展**全部以 phase/rpost 为键**——194942 轮：applied+落账后 closure 边界（"closure observation round boundary reached"）在 settle 切片跑之前就把 loop 结成 capability_blocked，裸 capability_blocked 也在 processor_selection 相位下穿透 gate；(3) inactive-loop 投影（goalrunner_chat.go:251-275）对欠结算 round 的 loop 强制 settle closure + goal 置 completed——194ms 无 LLM 空转"完成"，settle 永久丢失。

**修复清单**（裁定写入卡内）：① round 状态权威化——chat 侧新增 `freeStateLoopRoundPendingSettlement`（experiment running + 当前 round 有 fresh post-action 观察 + 无 decision），不信任可改写的 DecisionPhase；② settle 期第二次 mutation 伺服拒绝——agentloop 四个确定性 pending 合成函数（mix tick from reply / vocal clarification tick / mix treatment / conservative headroom）在 round 欠结算时返回 nil，chat 侧 recordGoalResult 拒绝持久存储、chatResponseFromAgentLoopResult 拒绝交互面覆盖、handlePendingMixTickChat 入口拒绝确认（含 recovered-expired 同 id 重批通道）；③ 输出 gate 收口——needs_experiment 在 round 欠结算时无报告字段=非法二次准入（该形态正是 chat 层映射 capability_blocked 的裸终态泄漏源）；settle 报告缺 preserved proposal 也拒绝；pending-settlement 反馈消息直接内嵌 settle 报告 JSON 形状示例（`freeStateSettleReportExample`）；④ 三个相位盲点改 round 键——agentloop gate 弃 phase 检查、closure 验证轮扩展覆盖 booked-evidence 窗口、turn +4 余量覆盖欠结算 round；⑤ inactive-loop 投影对欠结算 round 不再强制 settle closure / 不再置 completed，保持 waiting_continue（不复活：loop 终态原样保留）。

| stamp | 域/flavor | 终态 | 备注 |
|---|---|---|---|
| 20260828_194942 | p01 freq | fail "materiality missing" | 修复中间态取证轮：死锁已消（nudge 不再被 pending_interaction 拒绝），暴露 closure 边界 + gate 的 phase 盲点 → 修复 ②④ |
| 20260828_200512 / 200911 / 201316 | p01 freq | **pass ×3 连续** | 完整链路 PASS（观察+materiality+target+judgment boundary），S2e 验收达成 |
| 20260828_201719 | p02 默认 | **pass** | p02 默认链路无回归 |

开放项：无（S2e 验收关闭）。压缩域端到端仍归 S2f；D2-2-S2/S3 多轮链路未在本批回归（改动未触碰多轮准入，multi-round 卡自有验收）。

## 9. 2026-08-28 晚窗批（S2c 压缩域换算定标 + S2 合入后接线）

S2c 诊断定案（temp/s2c_probe/curve_dump.jsonl）：VSC-2 threshold 的 display probe **结构性退化**——read_only_value_to_string 在全部 5 个合成 normalized 点返回当前文本（5×"+11.8"，span=0），曲线反演确定性不可用（与目标值无关）。修复（b0c2ebd + 11e9183 三层）：experiment 域表 TargetSemantics 声明（broadband_compression=delta_db）、executionports 事务探测 delta 机器（单次可逆探测斜率、三段 CAS 重定基、冻结显示/超程 fail-closed 无净移动、0.15dB 物理读回门、static_eq 绝对路径逐字节不变有封存测试）、chat plan 一行透传（等 S2 合入后加）。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260828_181603 | comp p01（诊断） | fail "outside the measured display curve" | 曲线转储定案退化（旧卡点复现取证） |
| 20260828_185138 | comp p01（验收） | fail "delta target 12.8 dB (current 11.8 + 1) is outside the reachable normalized range" | **换算机器实栈验证通过**：探测取斜率成功、目标=current+delta 正确、顶格不可达诚实拒绝——旧换算卡点消除，新卡点为域语义（见 S2f） |
| 20260828_185457 | freq p01（回归） | fail "materiality missing"（S2e 方差） | **static_eq 零回归**：写入链 applied/readback(-1.5)/evaluation_ready 全绿，失败在 S2e 已知 settle 合规方差（S2d 戳表同款） |

开放项：压缩域端到端 PASS 的剩余卡点是**实例绑定语义**——绑定解析按 D2-1 static_eq 同款设计实例化全新 VSC-2，其 threshold 默认即物理顶格（normalized 1.0=+11.8dB），模型"过压→抬阈值"的 +1 方向物理无解；p01 的过压源头在工程既有处理链，新实例语义无法承载该修复意图。归新卡 S2f（域设计裁定：绑定既有实例 vs 新实例语义 vs prompt 方向引导）。

## 11. 2026-08-28 晚窗批（D2-2-S3 多轮探针收口：py 判定契约对齐 + Go 侧执行桥缺口钉死）

S3 诊断定案（卡面待查两点均闭合）：(1) **py 探针判定 bug**——`validate_d2_multi_round` 要求两 dose scope 的 `max_action_attempts >= 2` 才允许多轮，与 Go 侧 S1 冻结契约直接矛盾：`ValidateD2MultiRound`（experiment/d2_multiround.go:80-84）与 §8 裁定 4 都要求 `== 1`（每轮单变更），跨轮续行只由 `experiment_budget ∈ 2..MaxD2MultiRoundBudget` 表达。已修（fix(d1-smoke)）：改判 `== 1`，默认路径 `validate_d1` 不动。(2) **env 注入通道无恙**——ps1 无需任何透传改动，shell `$env:` 经 run_free_state_d1_smoke.ps1 → dev_agent_smoke.ps1 → Start-Process 继承链到达 agent（同 `VIT_PARAM_CURVE_DUMP` 机制）；三份产物 admission 均 `experiment_budget: 2` 且带服务端 baseline_fingerprint，`ResolveD2MultiRoundBudget` 接收确认。

**新发现（Go 侧，超出 S3 只动 scripts 的边界）**：修复后探针不再误判 fail，但三份产物（202855/204911/205244）同病——D2-2 admission 建成、round 1 开出后，chat 执行桥 plan builder 无条件调 `ValidateD1S1()` 拒绝 budget>1 admission（`d1PluginParamPlanWithBinding` free_state_d1_plan_table.go:292-297；track_gain 同构 `d1TrackGainPlan` free_state_d1_runtime.go:88-93），报错原文 `D1-S1 experiment_budget must be 1`，round 1 零干预、continuation 反复同错耗尽。**D2-2 档位下任何 admitted 域的干预都无法执行**，多轮续行（ ruling 1 的 next_round）永不可达，探针只能诚实 NOT_EXERCISED。修复方向已写入新卡 D2-2-S4：plan builder 用现成 `freeStateAdmissionRunsSingleRound`（free_state_experiment_runtime.go:336）分档校验，另需处理 experiment 级 sessionID/round 级 before-render 的每轮语义。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260828_202855 | p01 freq + 注入2 + MultiRoundProbe | fail "max_action_attempts (1) admits no multi-round continuation" | **py 判定 bug 首证**：budget=2 注入已生效（budget 检查通过才轮到该断言），误判在 admission 边界 |
| 20260828_204453 | p01 freq 默认路径回归 | **pass** | py 修复后默认路径行为零变化红线守住 |
| 20260828_204911 / 205244 | p01 freq + 注入2 + MultiRoundProbe | NOT_EXERCISED（exit 3）×2 | 判定修复生效：不再误判 fail；卡点后移至 Go 侧 plan builder 拒批（round 1 零干预，见上文钉死） |

开放项：**D2-2-S4（Go L2）**——chat 执行桥多轮接线（plan builder 分档校验 + 每轮 session/render 语义），S3 验收"注入 2 exit 0"在 S4 合入前结构性不可达，NOT_EXERCISED（exit 3）为其诚实终态。

## 12. 2026-08-29 晚窗批（D2-2-S3d settle 竞态单测隔离修复：修复本体真栈验证成立，探针失败点前移定案）

S3d 以单测确定性复现 S3c 遗留的 settle 报告竞态（20260829_165648 trace 形态：refused settle 与重放 tick 同秒入库 + waiting_interaction 遗留），先 RED 后修，四点落地（fix(d2-2-s3d)）：① refusal 分支在 loop 上落 `settle_refused_round_id` 同轮标记（投影滞后窗口内 spent-mutation/pending-settlement 两谓词皆不成立，守卫此前读的是竞态前旧快照）；② `recordGoalResult` 守卫与 `freeStateRoundPendingSettlementForConversation` 统一为三元条件（+settleRefused）；③ `chatResponseFromAgentLoopResult` 对竞态窗口信封做 settle 轮 execution memory 归零并降级 waiting_continue（`settle_report_refused_awaiting_observation`），continuation 不再 park；④ `improvementProposalResponse` 在标记窗口抑制提案确认面。标记随 settle 报告落地或轮推进自愈；普通提案路径对照单测锁定零变化。

**真栈验证（20260829_175049 trace）**：修复本体行为完全符合设计——refuse 后同一信封 tick 未重存、无 waiting_interaction 绑定（终态 continuation 全 completed，S3c 遗留的 restart-idempotency 卡点消除）、round 1 权威终态完整（interventions=1、decision=next_round、obs=2）。但探针仍 exit 1，失败点**前移**至更早的校验器：`round 1 carries 0 forward interventions`——与权威持久化 loop（round 1 恰 1 干预）直接矛盾，定案两个新层缺口（归新卡 D2-2-S3e）：(a) **round-2 驱动停滞**——判定开出 round 2 后，其首个 nudge 轮携带 settle 报告被拒（round 2 欠干预不欠结算，拒得正确）、降级链耗尽为 completed，后续 3 次 nudge 全部不进 LLM 轮，round 2 的欠账干预永未提出；(b) **校验器投影新鲜度**——`find_d1_loop` 按 `updated_at` 字符串选投影且同刻保留最后遍历副本，响应信封内的过期 loop 副本（round 1=0 干预的滞后投影）压过权威持久化副本，断言打在陈旧数据上。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260829_175049 | p01 freq + 注入2 + MultiRoundProbe | fail "round 1 carries 0 forward interventions" | **竞态修复真栈验证成立**（refuse 轮 `settle_report_refused_awaiting_observation`、无 tick 重存、无 waiting_interaction 遗留）；失败点前移至 round-2 驱动停滞 + 校验器选了过期响应投影（权威 loop round 1=1 干预完整闭合） |
| 20260829_180942 | p01 freq 默认路径回归（-SkipBuild） | **pass** | S3d 修复后默认路径零变化红线守住 |

开放项：**D2-2-S3e（Go L2 + py 一并）**——round-2 欠账干预的驱动链（refused-settle 降级后 nudge 不进 LLM 轮）与探针校验器的投影新鲜度（`find_d1_loop` 同刻/滞后副本胜出）。S3b/S3c/S3d 三卡验收 3 在 S3e 合入前保持未绿，D2-2 exit 0 收口顺延。

## 13. 2026-08-29 晚窗批（D2-2-S3e mix-tick 吞咽解除 + 校验器权威投影优先：两修真栈验证成立，失败点前移至 refused-settle 信封残留）

S3e 以 175049 trace 的 nudge 回复原文（.vit_history commit 逐字命中 `d1_settlement_pending_mix_tick_refused` 文案）定案缺口 A 根因：`messageExplicitMixTickApply` 精确匹配表含裸"继续"，settle 拒绝后结算挂起态（`freeStateLoopRoundSettleRefused` 常驻）把每个驱动 nudge 当显式执行确认短路，永远到不了 `runAgentLoopChat`。RED 单测逐字复现后修（fix(d2-2-s3e) 9cd6f99）：裸继续（`isContinueMessage`）不算显式执行确认，待确认面照旧退役（S2e 不变量，既有拒绝测试同绿）。缺口 B：`find_d1_loop` 加 `authoritative` 参数——持久化副本同刻/缺失时间戳胜出，信封副本仅严格更晚取代；`validate_d1` 无参路径逐字节不变，未放宽任何断言。

**真栈验证（20260829_183540 trace）**：两修本体均成立——nudge #2/#3 已到达 `runAgentLoopChat`（stop=`pending_interaction_requires_response`，mix-tick 吞咽消除）；校验器已看权威数据（报错 "round 1 carries 0 forward interventions" 指 round index 1=round 2 的真实 0 干预，权威终态 rounds=[1,0] 一致）。终验仍 exit 1，失败点**再前移一层**：settle 拒绝降级轮（18:39:46）的 continuation 信封 `free_state_decision` 残留 `user_judgment_pending`+`human_audition_ready`（S3c 的 park 标记中和只覆盖 loop 投影 LatestDecision），`continuationRequiresUserInteraction` 据此把 `cont_3ca4...` park 成 waiting_interaction 且负载为空壳（无 interaction_id/requests）——bare-continue 交互门吞掉全部 nudge 却无可答复交互面，S3c 武装的 round-2 goal continuation 排在门后不可达，round 2 欠账干预永未提出。归新卡 D2-2-S3f。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260829_183540 | p01 freq + 注入2 + MultiRoundProbe | fail "round 1 carries 0 forward interventions" | **两修真栈验证成立**（nudge 过 mix-tick 门到达 agent loop；校验器读权威投影）；失败点前移至 refused-settle 降级的 continuation 信封 decision 残留（空壳 waiting_interaction + bare-continue 门吸收，armed round-2 continuation 不可达） |
| 20260829_185128 | p01 freq 默认路径回归（-SkipBuild） | **pass** | S3e 两修后默认路径零变化红线守住 |

开放项：**D2-2-S3f（Go L2）**——refused-settle 降级的中和须覆盖 result 信封的 `free_state_decision`（或 bare-continue 门对无 interaction_id 的空壳 waiting_interaction 放行）。S3b/S3c/S3d/S3e 四卡验收 3 在 S3f 合入前保持未绿，D2-2 exit 0 收口顺延。

## 14. 2026-08-29 晚窗批（D2-2-S3f 信封边界信号对中和：修复真栈验证成立，失败点前移至 refused-settle 重试轮的 completed 残留 + goal 终态折叠）

S3f 以 183540 trace 定案的机制开卡，但取证发现 S3c 中和只清了判定边界信号对的一半：settle 报告按提示词契约**成对**携带 `experiment_round_decision=user_judgment_pending` 与 `experiment_target_response.outcome=human_audition_ready`，存活的一半经 `runAgentLoopChat` 的 LatestDecision 回写进入 result 信封，`continuationRequiresUserInteraction` 的 decision 双检查（continuation_scheduler.go:148）仍命中 → park 空壳。RED 单测逐字复现（pending 负载与 183540 终态逐字节一致）后修（fix(d2-2-s3f) 9588e3f）：拒绝分支中和完整信号对（任一存在即双清 round decision + target response）；红线对照测试证明真实判定边界（settle 被接受、新鲜 post-action 观察已入账）仍收口 `completed_at_human_judgment_boundary`。

**真栈验证（20260829_192048 trace）**：修复本体成立——判定 POST 后 turn=1（19:24:45，nudge 驱动）的 checkpoint `cont_3f44` 为 pending 且被调度器领取（attempt=1）驱动 turn=2（19:24:56，调度器驱动），全程无 `pending_interaction_requires_response`（park/bare-continue 吞咽消除），S3c 武装的 round-2 continuation 可达且已运行。终验仍 exit 1，失败点**再前移一层**，盘态三环铁证：(a) round-2 欠账轮模型应答 `final:true` settle 报告 + settle 预算余量（S3c +4）使其干净收尾 → turn=2 `res.Status=completed`，拒绝降级只改写 HTTP 响应不改写信封状态；(b) `durableContinuationFromResult`（continuation_scheduler.go:220-229）对 completed 无分支落默认 waiting_interaction，预算入队分支跳过（终态 used=7/8，两轮只入队一次）；(c) goal 被 flip 为 completed 后，下一次调度器重载 `normalizeRestoredDurableContinuation`（:361-363）按终态 goal 把 turn=2 的 `cont_5a13` 折叠为 completed——**出生毫秒戳 attempt=0**（排除 setContinuationStatus/reconcile，均会动 UpdatedAt）——goal_continuations 清空，后续 nudge 走 `no_continuation`（43ms），round 2 欠账干预永未提出。归新卡 D2-2-S3g。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260829_192048 | p01 freq + 注入2 + MultiRoundProbe | fail "round 1 carries 0 forward interventions" | **S3f 修复真栈验证成立**（无 waiting_interaction park、无交互门吞咽，cont_3f44 pending→领取→驱动 turn=2）；失败点前移至 refused-settle 重试轮 res.Status=completed 残留 + goal 终态重载折叠（cont_5a13 出生即 completed，attempt=0） |

旁证：全量 `go test ./...` 三跑中两跑各有一个 continuation_scheduler_test.go 的 durable-slice 观测测试非确定性失败（两次为不同测试、单独重跑均绿、失败路径不经过 S3f 改动分支，基座一跑未复现）——满负载抖动嫌疑，非本批因果；再复现则单开测试稳定性卡。

开放项：**D2-2-S3g（Go L2）**——refused-settle 重试轮的信封 `res.Status` 中和（或 durableContinuationFromResult 保持欠账轮可调度）+ goal 终态折叠的 loop 活性豁免。S3b/S3c/S3d/S3e/S3f 五卡验收 3 在 S3g 合入前保持未绿，D2-2 exit 0 收口顺延。

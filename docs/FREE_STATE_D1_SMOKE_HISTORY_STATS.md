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

## 15. 2026-08-29 晚窗批（D2-2-S3g 重试轮信封中和 + goal 终态折叠豁免：修复真栈验证成立，失败点前移至 settle 重放烧预算 + G7 旧证据拒收 + 第二 goal 交互吞噬）

S3g 以 192048 盘态定案的三连缺口开卡。RED 单测逐字复现全链（refused settle 的 `res.Status=completed` 残留 → `durableContinuationFromResult` 默认 waiting_interaction 空壳落盘 + 预算入队跳过 → goal 翻终态后重载按 `normalizeRestoredDurableContinuation` 折叠为出生毫秒戳 attempt=0 的 completed → bare-continue 门 no_continuation），另附红线对照（loop 终态仍折叠）。修复取 A+C 组合（fix(d2-2-s3g)）：A——`chatResponseFromAgentLoopResult` 的 settle-refused 中和扩展到干净 settle 收尾的 completed 残留（仅 `res.Continuation!=nil` 可降级，无 checkpoint 的真完成保持终态），信封与降级响应对齐为 waiting_continue + 可调度 checkpoint + 自有预算槽；C——goal 终态折叠加 loop 活性豁免（`restoredContinuationOwesActiveRound`：loop 仍 active 且当前轮带 SettleRefusedRoundID 或欠干预时放行，真完成 loop 终态折叠不动）。B 方向（往纯函数灌 loop 状态）由 A 前置覆盖，不做。

**真栈验证（20260829_201003 trace）**：修复本体成立——S3g 诊断的杀链全链消除：refused-settle 重试轮的 checkpoint 全部 attempt=1 被调度器真实领取驱动（12:13:48、12:14:02 两轮重试连续运行，预算槽被真实消耗 6→7→8），无出生戳折叠击杀、无 waiting_interaction 空壳、后续 nudge 无 no_continuation。终验仍 exit 1，失败点**再前移一层**，201003 盘态三环：(a) round-2 重试轮的模型输出持续**重放 round-1 的 settle 报告**（round-2 自身未行动、fresh post-action 观察永不可能入账，每次重放被拒并消耗一个 continuation 预算槽，used 打满 8/8）；(b) 唯一一次 needs_experiment 再提案引用 round-1 时期观察（obs_20260829T121303，早于 round-2 开轮），准入审计拒收（`G7_fresh_revision_bound_refs` + `G1_project_binding`，盘态 admission_receipt 佐证），欠账干预无法进入执行；(c) 预算耗尽后 probe nudge 在同会话开出**第二个 goal**（goal_1cb7e764），其 settle-refused 轮把 checkpoint `cont_a596` 停在 waiting_interaction（attempt=0），后续两次 nudge 均被 `pending_interaction_requires_response`（11ms）吞掉。round 2 终态 0 干预 → 校验 exit 1。归新卡 D2-2-S3h。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260829_201003 | p01 freq + 注入2 + MultiRoundProbe | fail "round 1 carries 0 forward interventions" | **S3g 修复真栈验证成立**（重试轮 attempt=1 连续驱动、无折叠/空壳/no_continuation）；失败点前移至 settle 重放烧预算（used 8/8）+ 旧证据再提案 G7/G1 拒收 + 第二 goal waiting_interaction 吞 nudge |
| 20260829_202637 | p01 freq 默认路径回归（-SkipBuild） | **pass** | S3g 两修后默认路径零变化红线守住 |

旁证：本批全量 `go test ./...` 一次全绿（S3f 批记录的 durable-slice 满负载抖动未复现）。

开放项：**D2-2-S3h（待定引擎档）**——round-2 欠账轮的 settle 重放循环（提示词/契约层：欠干预轮的重试内容应为 observe→propose 而非等待 post-action 观察）+ 再提案的 G7 新鲜证据绑定 + 预算耗尽后同会话第二 goal 的 waiting_interaction 吞噬。S3b/S3c/S3d/S3e/S3f/S3g 六卡验收 3 在 S3h 合入前保持未绿，D2-2 exit 0 收口顺延。

## 16. 2026-08-29 深夜批（D2-2-S3h1 round-2 证据面新鲜度：A+B 组合修复真栈验证成立，失败点前移至欠干预轮模型行为层）

S3h1 以 201003 盘态定案的"目录递过期指针"开卡。RED 单测三件（轮边界基准直出 available_views、merge 防低版本 overlay 拖回 + 同版本/新版本/无版本 overlay 行为保持对照、陈旧 transport echo 端到端不回退 + 单轮边界零变化红线）+ G7 可满足性 walkthrough（agentloop：刷新面呈现的 obs id 在当前 closureRevision 下 G7 判过，rev 旧 ref 与混合引用仍拒——与盘态差异的最终裁决点）。取证阶段另钉死两层隐藏机制：(i) `mergeFreeStateAvailableViewRow` overlay 全键覆盖无新近性比较，判定前序列化的 continuation/transport 回声把已刷新的 available_views 行（及从 ledger 重建的 LatestObservation）拖回 pre-action 指针——"从未刷新"的实际机制是**刷新后被拖回**（d1 记账路径本身有 ledger 投影）；(ii) d1_post_action 收据行 freshness class="post_action" 不在 G7 词表（`freeStateFreshStatus`），即使目录指向新鲜 obs，引用它仍被 receipts 匹配行拒收——S3g 批 (b) obs_121303 再提案被拒的另一半根因。附带：`mergeFreeStateLedgers` 按 receipt_id 去重、空 id 行不去重 → overlay 双向合并指数膨胀（201003 盘态 1024 行）。

修复取 A+B 组合（fix(d2-2-s3h1) b0febf7）：A——merge 版本回退守卫（base 行 project revision 严格高于 overlay 时保留 base，同/新/无版本行为对照测试逐字锁定不变）；B——`bookRecalibrationRoundBaseFromLoop` 记账成功后 `recalibrationRoundBaseRecent` 把基准观察直出 ledger（只重述记账携带事实，不重构 per-view conclusions，缺视图不伪造）+ `supersedeFreeStateRoundBaseReceipts` 把该 obs 收据行重述为 current_observation+rev 绑定（同 id 重复行收敛为一条、赋 `d1_ccb:<obs>` 去重身份——该观察族的收据膨胀随之治愈；其它路径空 receipt_id 膨胀为潜在独立缺口，留档未修）。G7 语义零改动；单轮边界路径有双重门 + 对照测试锁定零变化。

**真栈验证（20260829_205921 trace）**：修复本体成立——available_views 目录呈现 round-2 新鲜基准（`track:1032::track.timbre_frequency → obs_20260829T130227 rev 4 ready current_observation`；201003 同位为 rev 2 pre-action 旧指针），receipts 4 条无膨胀且基准收据带 d1_ccb 去重身份，round-2 开轮带 rev4 基准（S3c 保持），continuation 链 attempt=1 真实领取驱动（S3g 保持）。终验仍 exit 1，失败点**前移至模型行为层**（全部落在已开卡 S3h2/S3h3）：round-2 turn5/6 连续 limit_reached（completed_steps=3、零工具调用）→ turn7 干净收尾输出 settle 报告被拒（settle_refused 落 round-2）→ 后续 nudge settle 重放两连拒（21:03:17/21:03:28）烧尽预算 8/8 → probe 同会话开出第二 goal（goal_f124424dcd609575）checkpoint 停 waiting_interaction，21:05:14 起 nudge 被 `pending_interaction_requires_response` 吞噬。G7 walkthrough 栈上未行使（round-2 模型未到达提案）——S3h1 的目录修复需 S3h2 把模型引到提案后才能端到端检验。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260829_205921 | p01 freq + 注入2 + MultiRoundProbe | fail "round 1 carries 0 forward interventions" | **S3h1 修复真栈验证成立**（目录呈现 rev4 基准 current_observation、收据去重无膨胀、S3c/S3g 行为保持）；失败前移至欠干预轮模型行为（settle 重放烧预算 + 第二 goal 吞噬）→ S3h2/S3h3 |
| 20260829_211900 | p01 freq 默认路径回归（-SkipBuild） | **pass** | S3h1 两修后默认路径零变化红线守住 |

旁证：全量 `go test ./...` 一次唯一失败为 `TestProcessorCertificationStartAcceptsBroadbandCompressorCapability` 的 Windows TempDir 清理竞态（unlinkat 目录非空），单独重跑绿，路径不经过本批改动分支，非因果（S3f 批同款抖动先例）。

开放项：**D2-2-S3h2**（欠干预轮 settle 拒收语义分流 + 准入拒收附新鲜引用——当前 round-2 走不到提案的第一因）+ **D2-2-S3h3**（预算耗尽后第二 goal 边界守卫，205921 复发确认）。S3b/S3c/S3d/S3e×2/S3f/S3g/S3h1 八卡验收 3 在 S3h2/S3h3 合入前保持未绿，D2-2 exit 0 收口顺延。

## 17. 2026-08-29 深夜批（D2-2-S3h2 欠干预轮拒收语义分流：修复真栈验证成立，失败点前移至调度器续跑轮的指引消费层）

S3h2 以 205921 盘态定案的"拒收文案把欠干预轮引向等观察"开卡。取证阶段的关键发现是**分流判据**：`freeStateLoopRoundOwesIntervention` 单用无法区分"真欠结算竞态"（干预已执行、投影零干预因 booking 滞后——S3d/S3c 竞态窗）与"从未执行的欠干预轮"（round-2 刚开），两者投影同形；判据取 `RequiresPostActionObservation` 债务位（applied 边界设、确定性入账清、settle 重放保活——free_state_reasoning_loop.go:878-884 的 S3c 期修复正好保住了它），新谓词 `freeStateLoopRoundNeverActed`。

修复四点（fix(d2-2-s3h2)）：A——拒收日志按轮类型分流；B——improvementProposalResponse 拒收分支分流文案（欠干预轮 settle 重放得到"本轮欠一次有界干预提案；新鲜基准为 <obs_id>@rev<N>，请引用它提出本轮提案"+ stop `round_owes_intervention_proposal`；**欠干预轮的裸提案不再被吞**、落到 S3c 再校准路由——否则指引是谎话；真欠结算轮文案逐字不变）；C——chatResponseFromAgentLoopResult 降级残留 stop reason 同步分流（S3g 机制不动）；D——agentloop G1/G7 准入拒收附带 ledger 当前可引用新鲜引用（available_views 优先、fresh+revision-bound、非版本门对照不携带）。RED 四件先失败于正确原因后全绿；S3d fixture 补债务位、S3g 断言按新分类更新（机制断言不动）；全量 `go test ./...` 一次全绿。

**真栈验证（20260829_215139 trace）**：修复本体成立——4 次拒收全部落 never-acted round-2 且 WARN 带新鲜基准 `obs_20260829T135455@4`（S3h1 目录修复保持）；**指引原文落地会话历史**（`.vit_history` 13:55:36 提交，HTTP 面模型可读通道打通）；round-1 期 final gate 反馈与真欠结算语义零变化。终验仍 exit 1，失败点**前移至调度器续跑层**：21:55:47/52/57 三次 settle 重放来自 executeDurableContinuation 续跑轮（无 HTTP 响应日志、无历史提交——该路径响应从不写历史，server.go:2250 历史提交仅 HTTP handler），指引未进入这些轮的可读面；且 21:55:08 round-1 终门刚教过模型 "emit the settle report"（当时合法），惯性源明确。预算烧尽后 probe nudge 落空确认面（21:57:34/21:59:35"没有找到正在等待确认的混音动作"）。round-2 终态 0 干预；G7/G1 附引用反馈栈上未行使（模型未到达提案）。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260829_215139 | p01 freq + 注入2 + MultiRoundProbe | fail "round 1 carries 0 forward interventions" | **S3h2 修复真栈验证成立**（拒收分类分流生效、指引落地会话历史、新鲜基准 obs@rev4 精确呈现、S3h1/S3c/S3g 行为保持）；失败前移至调度器续跑轮的指引消费层 → S3h4 |
| 20260829_220804 | p01 freq 默认路径回归（-SkipBuild） | **pass** | S3h2 四点改动后默认路径零变化红线守住 |

开放项：**D2-2-S3h4**（调度器续跑轮对拒收指引的消费通道 + settle 惯性切断点——215139 三连重放的归因层）+ **D2-2-S3h3**（预算耗尽后第二 goal 边界守卫）。S3b/S3c/S3d/S3e×2/S3f/S3g/S3h1/S3h2 九卡验收 3 在 S3h4（或其取证结论指向的卡）合入前保持未绿，D2-2 exit 0 收口顺延。

## 18. 2026-08-29 深夜批（D2-2-S3h4 调度器续跑轮的拒收指引消费通道：修复真栈验证成立，失败点前移至 limit 停机 checkpoint 的边界残留误分类层）

S3h4 以 215139 定案的"调度器续跑轮读不到拒收指引"开卡。取证定案（疑点 A 裁定：**不可见**，机制三层）：(i) S3h2 指引只写 conversation store（HTTP handler 独占，server.go:2250；调度器路径 executeDurableContinuation 丢弃响应，215139 的 135547/52/57 三轮无历史提交）；(ii) 续跑轮 prompt 的 History 来自 checkpoint 序列化的**环内**会话（runner.go:765），server 层拒收发生在消息环返回之后，拒收反馈结构性进不了环内会话——215139 三轮环内唯一反馈是 broad-acoustic 遗留闸门的无关文案（messages=3、1:user:154 即该文案 rune 数；neutral-family 投影把 History 压缩为最后一条 final_gate，message_loop.go:3720）；(iii) free-state ledger 白名单（free_state_reasoning.go:393）无欠轮/拒收键。疑点 B（惯性）降为次要——指引从未可达，惯性无从验证。

修复（fix(d2-2-s3h4) 56a01a7）：拒收前移进消息环 final gate 的 settle-report 分支（708/711 之后新增 `messageLoopFreeStateRoundOwesInterventionIssue`）——实验 running + 多轮层（复用 `messageLoopFreeStateMultiRoundTier`，密封单轮层零变化）+ 总干预数 < 预算 + 当前轮零干预/无 decision + `requires_post_action_observation=false`（竞态窗让位 708 闸门）+ 判断边界不挂起（GLM ruling 3，镜像 chat 侧 bookFreeStateRecalibrationRoundBase 守卫）时，settle 报告被拒，文案同口径命名欠的提案与可引用新鲜基准（`messageLoopFreeStateOwedRoundBaseReference` 镜像 chat 侧 freeStateOwedRoundBaseReference 的选择规则：当前轮最新 fresh 非 post-action 观察 `observation_id@project_revision`——两 refusal face 同源 loop 投影，不双写漂移；首版读错键名 `id` 在栈上落了 fallback 文案，已修为 `observation_id` 并以真实键形夹具锁定）。该 `<final_gate>` 反馈进环内会话 → checkpoint 序列化 → 下一续跑轮 History（含 neutral-family 压缩）可见——**消息环反馈即单一事实消费通道**（卡中点名的两通道之一），不另建通道。RED 8 用例（拒绝+引用命名/单轮层豁免/竞态窗豁免且 708 文案归属不变/预算耗尽豁免/欠轮提案放行/无基准 fallback/判断边界豁免）；封印测试 TestJudgmentBoundarySpansRoundsAtFinalGate 首跑打破后以判断边界守卫修复（挂起边界 settle 族照常放行，封印语义保持）。

**真栈验证（20260829_223957 trace）**：修复本体成立——round-2 开轮首轮（HTTP "继续" 驱动 armed continuation）模型重放 settle 报告（raw 引 obs_20260829T144324、携带 user_judgment_pending/human_audition_ready），消息环 final gate 以同口径文案当场拒绝（agent_message_loop_debug.jsonl stage=free_state_final_gate 全文留档），反馈序列化进 checkpoint 会话末位（cont_6205789b7 会话尾部 [assistant settle-replay, user final_gate 欠轮指引]）；server 层 booking 拒收全程未发生（settle_refused_round_id=None、零 WARN），重放仅耗一轮（215139 为三轮烧链）；round-1 的合法 settle（已行动轮）照常放行终局 completed。G7/引用命名的栈上行使受阻（见下）。终验仍 exit 1，失败点**前移至新层**：round-2 limit 停机（audioClosure 切片 MaxTurns=1）后，goalrunner_chat.go:455 把 loop.LatestDecision（round-1 settle 的判断边界残留 user_judgment_pending + human_audition_ready——判断已落地、round-1 decision=next_round、round-2 已开轮，但 LatestDecision 未被刷新）投到 res.FreeStateDecision，`continuationRequiresUserInteraction`（continuation_scheduler.go:127-152）把 limit 停机误分类为交互边界 → durable 停 **waiting_interaction 空壳**（pending_interaction 仅 {status,stop,limit} 无可应答请求，attempt=0 至终局未被领取）→ probe 三次 nudge 全被 `pending_interaction_requires_response`（3ms）吞咽 → round-2 第二 turn 永不运行，欠轮指引在 checkpoint 里未被消费 → round-2 终态 0 干预。与 S3f/S3g 的空壳 waiting_interaction 同形但触发源不同（非 refused-settle 信封、非 goal 折叠，而是上一轮已结算判断的 LatestDecision 残留骑到本轮 limit 停机）。S3h3 的第二 goal 吞噬本轮未到达（链死在更早的 park 层）。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260829_223957 | p01 freq + 注入2 + MultiRoundProbe | fail "round 1 carries 0 forward interventions" | **S3h4 修复真栈验证成立**（环内欠轮拒收精确触发、反馈进 checkpoint 会话、server 拒收/重放烧链消失、round-1 合法 settle 放行）；失败前移至 limit 停机 checkpoint 的 LatestDecision 边界残留误分类（waiting_interaction 空壳 park）→ S3h5 |
| 20260829_230224 | p01 freq 默认路径回归（-SkipBuild） | **pass** | S3h4 改动后默认路径零变化红线守住 |

旁证：全量 `go test ./...` 一次全绿（TestJudgmentBoundarySpansRoundsAtFinalGate 修复后含于其中）。

开放项：**D2-2-S3h5**（limit 停机 checkpoint 被 loop.LatestDecision 的 round-1 判断边界残留误分类为 waiting_interaction 空壳——armed 续跑链死亡层；S3h4 的指引已送达但无轮可读）。S3h3（预算耗尽第二 goal）仍未触达。S3b/S3c/S3d/S3e×2/S3f/S3g/S3h1/S3h2/S3h4 十卡验收 3 在 S3h5 合入前保持未绿，D2-2 exit 0 收口顺延。

## 19. 2026-08-30 早批（D2-2-S3h5 判断落地开轮的 LatestDecision 边界残留中和：修复真栈验证成立，失败点前移至 round-2 欠轮提案的 G1 准入拒收层）

S3h5 以 223957 定案的"limit 停机被 round-1 判断边界残留误分类"开卡。取证闭环：`recordFreeStateDecision` 的 decision==nil 分支（free_state_reasoning_loop.go:676-680）在 limit 停机轮直接返回、LatestDecision 不刷新；goalrunner_chat.go:455 的 loop copy-back 把残留投到 `res.FreeStateDecision`（limit 停机轮的 `state.freeStateDecision` 为 nil，残留是唯一来源）；`continuationRequiresUserInteraction`（continuation_scheduler.go:140-152）的 decision 双检查把 limit 停机分类为交互边界 → `durableContinuationFromResult` 停 waiting_interaction + `pendingInteractionFromResult` 取不到请求 → 空壳。残留源头锁定在 `applyFreeStateJudgmentOutcome` next_round 分支：DecideRound(next_round)+StartRound 开轮时 LatestDecision 未被中和；`freeStateJudgmentBoundary` 读 experiment 轮状态而非 LatestDecision，中和不触碰真实边界语义。

修复（fix(d2-2-s3h5) f470e88）：提取 S3f 内联中和为 `neutralizeFreeStateBoundaryResidue`（剥 LatestDecision 的完整边界信号对），接入两处开轮点——判断落地 recalibration 分支（StartRound 成功后、armed continuation 序列化 loop 之前，保证 armed context 也干净）与 insufficient-dose calibration 分支（S3f 处改为调用同 helper）。方向取 (a)（任务卡三选一）：与 S3f 同族、根因层状态卫生、不依赖 stop_reason 推断；(b) 侵入 scheduler 纯函数签名、(c) 依赖"真实边界轮从不 limit 停机"的脆弱不变量，均弃。RED 三态：判断落地开轮后残留幸存（修复前失败）+ 欠轮 limit 停机 park 空壳（修复前失败，空壳形态与 223957 逐字一致）+ 挂起期间边界分类保留（控制组通过）。

**真栈验证（20260830_085624 trace）**：修复本体全部成立——终态 5 个 continuation 全 completed、零 waiting_interaction 空壳（223957 为 park 死锁）；persisted_loop.latest_decision 无边界残留；round-1 合法判断边界 park（"experiment round is waiting for the human judgment boundary"）与终局折叠照常；**round-2 续跑轮首次真实消费 S3h4 欠轮指引**（checkpoint 环内会话：settle 重放被欠轮文案拒收 → 模型提出 round-2 提案"Track 1027 300Hz -0.5dB" → G1 准入拒收**附新鲜引用 `obs_20260830T005914_bb866ca1bfc6@4`**（S3h2 的 D 修复与引用命名在栈上行使——本卡验收点达成））。终验仍 exit 1，失败点**前移至新层**：round-2 的 needs_experiment 提案被消息环 full admission gate 的 **G1_project_binding 拒收**——`task_contract.project_revision=2`（合同冻结基线）vs `minimal_audio_closure.project_revision=4`（round-1 干预后当前值），gateG1 的 revision 一致性检查（free_state_gate.go:69-82）在首轮干预后结构性失败；模型被 G1 拒收文案导向 needs_observation → 防重复观察闸（"already returned usable evidence"）又拒 → 模型回 settle → 欠轮拒收，三重方向矛盾循环每轮 executed=0 烧预算至 9/8 → loop blocked"free-state reasoning exhausted its 8-continuation budget"。round-2 终态 0 干预；3 次 nudge 落"现在没有可执行的待确认混音动作"。S3h3 的第二 goal 吞噬仍未触达（链死于 gate 矛盾循环）。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260830_085624 | p01 freq + 注入2 + MultiRoundProbe | fail "round 1 carries 0 forward interventions" | **S3h5 修复真栈验证成立**（空壳 park 消除、round-2 链持续可调度、欠轮指引首次被消费、G1 拒收附 obs@4 引用行使、round-1 合法边界照常）；失败前移至 round-2 欠轮提案的 G1 准入拒收（contract rev2 vs closure rev4 结构性不一致 + 三重方向矛盾烧预算）→ S3h6 |
| 20260830_091207 | p01 freq 默认路径回归（-SkipBuild） | **pass** | S3h5 改动后默认路径零变化红线守住 |

旁证：全量 `go test ./...` 一次全绿（S3f 控制组 TestGenuineSettleJudgmentBoundaryStillParks 含于其中）；另 20260830_085151 一轮 NOT_EXERCISED（exit 3）为环境配置缺失的空跑（未带 freq flavor + tier 注入），不计入失败面。

开放项：**D2-2-S3h6**（round-2 欠轮提案的 G1_project_binding 拒收层——needs_experiment 对已 admission 实验的轮内提案走 full admission gate、gateG1 拿合同冻结 revision 对 closure 当前 revision 做一致性检查在干预后结构性失败，且与欠轮指引、防重复观察闸形成方向矛盾循环烧尽预算；含 budget 9/8 超支 1 次的守卫核对）。S3h3（预算耗尽第二 goal）仍未触达。S3b/S3c/S3d/S3e×2/S3f/S3g/S3h1/S3h2/S3h4/S3h5 十一卡验收 3 在 S3h6 合入前保持未绿，D2-2 exit 0 收口顺延。

## 20. 2026-08-30 早批（D2-2-S3h6 欠轮提案的 G1 准入拒收：修复合入并单测锁定，多轮终验失败层前移至判定驱动的 arm 槽位 legacy 空壳——S3h6 豁免本轮栈上不可达）

S3h6 以 085624 定案的"G1 拿合同冻结 revision 对 closure 当前 revision 做一致性检查、首轮干预后结构性恒 false"开卡。取证确认语义本体：round-2 提案是**已 admission 实验的轮内提案**（同一 admission 下 DecideRound+StartRound 开轮），不是新实验 admission；chat 侧 recordFreeStateDecision 已有 carriesExperimentReport 豁免先例（真实路径 internal/chat/free_state_reasoning_loop.go:814-818，卡内误写 agentloop 包）——轮内提案与 settle 报告同属"审计 pre-apply 状态会拒绝已合法演进状态"的形状。方向取 (a) 边界豁免：G1-G7 语义零触碰，动的是"哪些提案过 full gate"的边界。

修复（fix(d2-2-s3h6) a4f158b）两层同步：agentloop 新谓词 `messageLoopFreeStateRunningMultiRoundExperiment`（budget 严格取自序列化 admission、不吃 env tier 回退，防畸形 context 骗豁免）包裹 final gate 的 else-if 分支；chat 侧 `freeStateLoopRunningMultiRoundExperiment`（typed loop）扩展 carriesExperimentReport 同位的审计条件。豁免范围锁定 running + budget 2..max 的 multi-round：新 admission（无 experiment）、密封单轮 tier、已 settle 实验保持全量 gate（三控制组单测锁定）；轮级边界仍守（pending-settlement 拒收先于豁免、judgment boundary、轮单干预 runtime 守卫、VSP base revision）。附带核对项定案：continuation 9/8 超支为 goalrunner_chat.go:2647"递增后检查"的拒收计数器残留（n-th 合法注释明言），非真实超支执行，不触冻结契约。RED→GREEN：agentloop free_state_intra_round_gate_test.go 6 用例（主断言首跑精确复现取证文案）+ chat TestD2MultiRoundOwedRoundProposalSkipsGateAudit（含单轮控制）；全量 `go test ./...` 一次全绿。

**真栈验证（20260830_093308 trace）**：默认路径 -SkipBuild **exit 0**（20260830_095248）——单轮默认路径零变化红线守住。多轮终验 exit 1，但失败层**前移到 S3h6 修复面之下**：round-1 全链完整（budget=2 static_eq 准入、干预 b3→a4、probe 机器判定落地、decision=next_round、round-2 开轮 + 基准 book + grant 6→9 + S3h5 中和全部保持），round-2 却**零模型轮**（0 干预、validate 报 "round 1 carries 0 forward interventions"，index 1 = round 2）——S3h6 豁免在栈上不可达。死亡链：本轮 01:36:06 一次 transient_llm_error 环境抖动多烧一片使 settle 切片恰在 used=6==budget=6 耗尽边界 park（合法）→ 01:36:29 判定 POST 入口 `activateCurrentProjectWorkspace` 触发全量 workspace restore（幂等守卫因 settle 期 session rebind 不成立）→ server.go:6912-6944 legacy 迁移把 settle 期 persisted in-memory continuation 物化为 `legacy_waiting_continue` 空壳 durable 记录并占据 `goalContinuations[goalID]` → `armFreeStateRecalibrationContinuation`"拒绝覆盖"守卫让位 → round-2 无 armed continuation → nudge 全被空壳 pending_interaction_requires_response 吞噬（agent_last.log 判断后零 agent_loop_chat timing 行佐证）。对照：085624（S3h5 run）同判定流程后 round-2 切片正常跑动、S3c 期判定驱动曾完整走通 round-2 执行——judgment 时刻全量 restore 是否为差异入口待 S3h7 取证。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260830_093308 | p01 freq + 注入2 + MultiRoundProbe | fail "round 1 carries 0 forward interventions" | round-1 全链完整、round-2 开轮/基准/grant/中和保持；round-2 零模型轮——判定驱动 arm 槽位被 restore legacy 迁移空壳占据，S3h6 豁免栈上不可达 → S3h7 |
| 20260830_095248 | p01 freq 默认路径回归（-SkipBuild） | **pass** | S3h6 两层豁免改动后默认路径零变化红线守住 |

旁证：全量 `go test ./...` 一次全绿（agentloop 6 新用例 + chat 1 新用例含于其中）。

开放项：**D2-2-S3h7**（判定驱动的 arm 槽位被 workspace restore legacy 迁移空壳占据——round-2 链饿死于首个模型轮之前；S3h6 豁免待该层修复后才能首次栈上行使）。S3h3（预算耗尽第二 goal）仍未触达。S3b/S3c/S3d/S3e×2/S3f/S3g/S3h1/S3h2/S3h4/S3h5/S3h6 十二卡验收 3 在 S3h7 合入前保持未绿，D2-2 exit 0 收口顺延。

## 21. 2026-08-30 早批（D2-2-S3h7 arm 槽位 legacy 空壳：修复真栈验证成立——round-2 全链首次走通、S3h6 豁免首次栈上行使，失败点前移至提案确认 park 孤儿层）

S3h7 以 093308 定案的"判定驱动 arm 槽位被 workspace restore legacy 迁移空壳占据"开卡。取证**修正卡内死亡链两处**：(i) 物化入口不是判定 POST 入口的 `activateCurrentProjectWorkspace`（幂等守卫成立——093308 判断窗口无第二次 "[workspace] activated" 行），而是 **scheduler 周期的 `reloadActiveRuntimeState`**（continuation_scheduler.go:947，静默无日志入口；POST 退出后 invocationsActive 放行即触发）；(ii) 被物化的磁盘 `state.GoalContinuations` 条目不是 settle 切片 recordGoalResult 的写回（该写回经 2570 行必带 durable ID，restore 会走既有匹配分支不产空壳），而是 **arm 自己在判定 POST 内写回的 ID-less armed continuation**（goal 键控空间唯一无 ID 写入者 = arm:1072；终态空壳 ID hash(goal|run|""|"") 与 armed 形态同构互证）。即 round-2 **已被武装并持久化**，scheduler 磁盘重载把武装成果误判为 pre-C legacy checkpoint 物化成 `legacy_waiting_continue` 不可应答空壳挤掉了它——093308 终态 `goal_continuations` 只剩空壳、6 条真实 durable 全 completed 与此完全一致。

修复（fix(d2-2-s3h7) 3903e31）方向 (b)+(a) 组合、(c) 排除（POST 入口守卫本次未触发，收窄无的放矢）：(b) 根因——legacy 迁移对携带 `free_state_internal_resume` 标记的条目（`isArmedInternalResumeContinuation`）不物化 durable 空壳、保留兼容索引槽位，并对同 hash 的非终态不可驱动残留（旧版 restore 物化的 shell）按 arm 同规则迁移时置换（防 latestByGoal 重载再毒化）；(a) 纵深——arm 守卫经命名谓词 `continuationOccupantDrivable` 从"槽位存在即让位"改为"occupant 不可驱动时可置换"，置换簿记 cancelled + `displaced_by_recalibration_arm` 审计标记，终态记录永不改写（restart 幂等）。**CLEAN1 审计判据种子落地（附加要求）**：`continuationOccupantDrivable`（可驱动 ⇔ 无 durable（=armed 形态，nudge 直驱）/pending/claimed/running/waiting_interaction 带可应答交互面；不可应答空壳与终态残留 ⇒ 可置换）+ `legacyWaitingContinuePendingInteraction` + `continuationTerminalStatus`，arm/迁移两处消费同一谓词零内联判断——供 CLEAN1 卡逐点核对"parked continuation 必须可驱动或可置换"不变量。RED→GREEN：free_state_arm_slot_restore_test.go 5 用例（迁移保 armed——首跑物化空壳与 093308 逐字段同形；真 legacy 迁移形状锁定（红线）；arm 置换空壳并过 reload 往返；arm 永不置换可驱动 occupant（红线）；谓词表 9 形态）；全量 `go test ./...` 一次全绿。

**真栈验证（20260830_102243 trace）**：修复本体成立——**round-2 全链首次在栈上走通**：armed continuation 跨 scheduler reload 存活（空壳零物化），round-2 模型轮恢复运行，提案**过 gate**（S3h6 豁免首次栈上行使，无 G1 拒收）、mix-tick 确认（10:28:24 explicit confirmation routed）、干预执行入账（persisted rounds=2、round-2 interventions=1）、post-action 观察新鲜且 revision 绑定、settle 链走通至 round-2 判断边界；validate 的轮数/每轮单干预/跨轮剂量界/持久化投影一致性**全部通过**（历次 "round 1 carries 0 forward interventions" 首次消失）。终验仍 exit 1，失败点**前移至新层**：restart 幂等检出 `cont_030927` waiting_interaction + **waiting_confirmation**（真实可应答面，非空壳）——round-2 提案轮的确认 park 在确认被应答、干预已入账后未被终态化：确认前 nudge 从 ID-less armed continuation 恢复（cont_1520 的 ResumedFromID 为空 → parent-completion 无法链接提案 park），显式确认走 mix-tick pending 面不经 interaction-respond 桥（completePendingInteractionContinuation 不触发）→ 孤儿过 restart。默认路径 -SkipBuild（20260830_103923）被 **D 盘占满环境故障**打断（单轮链健康推进至提案确认段，10:43:29 runtime state save 连续 "There is not enough space on the disk" → durable_checkpoint_persist_failed；改动对单轮路径结构惰性——armed 标记只在多轮 arm 产生、arm 守卫被 owed-intervention 谓词门控、迁移分支只认 armed 标记），磁盘已清理，待补跑一次。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260830_102243 | p01 freq + 注入2 + MultiRoundProbe | fail "restart resurrected non-terminal continuations: waiting_interaction" | **S3h7 修复真栈验证成立**（round-2 全链首次走通、S3h6 豁免首次栈上行使、armed 跨 reload 存活、历次 0-forward-interventions 消失）；失败前移至 round-2 提案确认 park 孤儿（answered confirmation 未终态化）→ S3h8 |
| 20260830_103923 | p01 freq 默认路径回归（-SkipBuild） | fail（环境：D 盘占满） | 链路健康至磁盘满点（runtime state save 磁盘满 → durable_checkpoint_persist_failed），非代码回归；磁盘已清理，随 S3h8 终验一并补跑 |

旁证：全量 `go test ./...` 一次全绿（chat 5 新用例含于其中）。

开放项：**D2-2-S3h8**（round-2 提案确认 park 孤儿——armed 链恢复无 ResumedFromID 链接 + mix-tick 显式确认面不走 completePendingInteractionContinuation，answered confirmation 未终态化过 restart）+ S3h7 默认路径 -SkipBuild 补跑（磁盘清理后）。S3h3（预算耗尽第二 goal）仍未触达。S3b/S3c/S3d/S3e×2/S3f/S3g/S3h1/S3h2/S3h4/S3h5/S3h6/S3h7 十三卡验收 3 在 S3h8 合入前保持未绿，D2-2 exit 0 收口顺延。

## 22. 2026-08-30 午批（D2-2-S3h8 已应答确认 park 孤儿：修复真栈验证成立——**多轮终验首次 exit 0，D2-2 十三卡链收口**）

S3h8 以 102243 定案的孤儿开卡。取证把卡内死亡链精确到**确认面错配**：probe 的 round-2 驱动 nudge（"继续"，`messageExplicitMixTickApply`/`ClassifyConfirmation` 均列为显式接受）在候选已在 `pendingMixTicks` 落位时经 `/agent/chat` → `handlePendingMixTickChat` DecisionAccept 直接执行（agent_last.log 10:28:24 `explicit confirmation routed`，全请求无 `http.interaction_respond` timing 行佐证）——该路径从不触完成桥；而 interaction/respond 面（server.go:4139，S3c 修复）先 `completePendingInteractionContinuation` 再路由。round-1 逃过纯因确认恰好经 respond 面驱动。armed 链 ID-less resume（cont_1520 无 ResumedFromID → parent-completion 链不上 cont_030927）是邻近事实但非闭合点：park 等的是确认本身。

**方向裁定（按"特例变少"判据取 (a)）**：两个确认面经同一完成桥——accept 与 reject 都是"已应答"，都在 `handlePendingMixTickChat` 路由时以 `completeAnsweredMixTickParks` 按 candidate 归属（park pi 为 waiting_confirmation + 带 interaction_id + requests payload 的 observation_id/track_id 与被确认候选一致）反查 park，交 `completePendingInteractionContinuation` 终态化；异源 park 与未应答 park 不动（红线）。**身份纪律种子 (#3) 裁定为不可控成本、留档 deferral**：arm 有意不写 durable 记录（S3h7 设计注释明示），补 durable 身份会向调度器合格性引入新 pending 记录、动摇 S3h7 已验证行为；且 (a) 落地后 ID-less resume 对完成传播家族不再致孤——种子推迟至 CLEAN1 卡（CLEAN1 卡已在 todo）。RED→GREEN：continuation_scheduler_test.go 3 用例（acceptance 闭 park / rejection 闭 park / 异源 park 不误伤）；全量 `go test ./...` 绿（`TestProcessorCertificationStartAcceptsBroadbandCompressorCapability` 出现一次 Windows TempDir 清理竞态 fail，单跑与复跑均绿、域无涉，判环境抖动非代码）。

**真栈验证（20260830_125328 trace）**：**多轮终验首次 exit 0**——`multi_round_probe_pass`、`restart_idempotent=True`、round-1/round-2 各 1 干预（static_eq，跨轮每频段累计 -1.0dB 界内）、budget=2 耗尽、5 条 durable continuation 全 completed；**修复层在孤儿原位栈上触发**：round-2 提案 park 12:57:37 stored → 12:59:20 chat 面显式确认 routed → 同秒 `[mix.tick.pending] answered confirmation closed park`（interaction_69ac1ede…）→ restart 检查零复活。**默认路径回归（20260830_132640，-SkipBuild + freq）exit 0**——单轮路径零变化红线守住，同时补齐 S3h7 磁盘满中断的欠账。

**终验环境修复留档（磁盘清理连锁，两处）**：早间 D 盘清理抹掉 `temp/…smoke-v1/fixtures`（夹具集 + `VitApp/build` 的 CMakeCache）。(i) 夹具重建：`semantic_processor_project_smoke_fixtures.py` 重生成 stems（6 个 sha256 与 manifest 全匹配）+ 从 artifacts 副本恢复 .vit（哈希 `4ba3caa6…`，33 次 run 物化共识）；CMake 重新 configure+build。(ii) **冻结 v2 存储两次阶**：初恢复漏了 `projects/spv1_p01/.vit_agent`（Aug13–26 冻结积累、manifest `degraded.evidence_off=true`（store_total_budget_exhausted））→ 新存储 evidence 开启 → 每笔观察挂 `evidence://` 引用（裸 obs ID 不在 refs）→ 绑定只给模型 URI → 模型照引 → FS6→FS7 准入门（要求裸 ID）确定拒收 → loop blocked → NOT_EXERCISED×2（20260830_121054/122919，环境性非代码，模型忠实引用唯一被呈现的 ref）；恢复冻结存储后解除。二阶：从 102243 副本恢复时漏剔其 journal 运行日记录 → `/agent/actions` 多出 2 条历史 `free_state_d1_s1` 动作 → 默认路径 "D1 journal must contain exactly one forward mutation" fail（20260830_131010 中性味 NOT_EXERCISED 属准入方差、131515 journal 3 条）；按 created_at < 2026-08-27 剔 28 条后默认路径过。**留档后续观察（非本卡家族）**：全新存储（evidence 开）下"v2 持久化 evidence 引用 vs 准入门裸 ID 要求"是真实产品层不一致——早晨系列全绿只因冻结存储 evidence 被预算耗尽关闭；该层属 admission/evidence 家族，待后续卡裁定。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260830_121054 / 122919 | p01 freq + 注入2 + MultiRoundProbe | NOT_EXERCISED（exit 3）×2 | 环境阶 1：夹具 .vit_agent 冻结存储缺失 → evidence:// 引用形态 → 准入门拒收（非代码、非模型方差；121054 附 NOT_EXERCISED 语义剖析） |
| 20260830_125328 | p01 freq + 注入2 + MultiRoundProbe | **pass（multi_round_probe_pass）** | **D2-2 多轮终验首次 exit 0**：S3h8 修复在孤儿原位触发（answered confirmation closed park）、restart_idempotent=True、全 continuation 终态；S3 系列全部断言绿 |
| 20260830_131010 | p01 中性味默认路径（-SkipBuild） | NOT_EXERCISED（exit 3） | 中性味准入方差（S3 系列默认路径回归惯例为 freq 味，本次误用默认值） |
| 20260830_131515 | p01 freq 默认路径（-SkipBuild） | fail "D1 journal must contain exactly one forward mutation" | 环境阶 2：恢复的冻结 journal 带 102243 运行日 2 条 d1 动作史（loop 本体健康：1 轮 1 干预、forward_mutation_count=1、正确停判断边界）；剔运行日记录后解除 |
| 20260830_132640 | p01 freq 默认路径（-SkipBuild） | **pass** | 单轮路径零变化红线守住；S3h7 磁盘满欠账补齐 |

旁证：全量 `go test ./...` 绿（chat 3 新用例含于其中）。

收口：**D2-2 十三卡链（S3b/S3c/S3d/S3e×2/S3f/S3g/S3h1/S3h2/S3h4/S3h5/S3h6/S3h7）以 20260830_125328+132640 双 exit 0 为同一戳连关**（blocked→done 10 张 + 已 done 3 张）；S3h3 层未被终验触达（双链全程单 goal）按裁定分支**关闭留档**（未来再现第二 goal 吞噬凭卡重开）。fix 4fda027 合入，不 push。开放项移交：CLEAN1（持久残留新鲜度审计，含 S3h8 身份种子 deferral 上下文 + 本批冻结存储/evidence_off 取证）、STAB1（durable 切片测试稳定性，本批 TempDir 抖动为其又一例证）。

## 23. 2026-08-30 晚批（D2-2-STAB1 durable-slice 观测测试满负载稳定性：双根因定案，测试隔离/时序修复）

根因一句话结论：`TestFreeStateObservationsSurviveFourDurableSlices` 的满负载抖动是测试侧两个独立缺陷叠加——① 快照 overlay 用测试启动时刻固定的未来时间戳（`now+(index+1)s`）盖 `UpdatedAt`，满负载拖慢后真实时钟反超，`mergeFreeStateLoops` 的 overlay-newer 守卫（S3h1 同款机制）把快照误判为陈旧回显，`CurrentPhase` 停止跨切片前进（"child checkpoint lost durable evidence"）；② `recordGoalResult` 尾部对每个 auto continuation 调 `wakeContinuationScheduler()`，测试一旦配置 `continuationExecutor` 即启动真后台调度器 goroutine，与测试的同步扫描/断言竞态抢同一个刚创建的 pending checkpoint（"slice N did not create a pending child checkpoint"，即卡上 :312 断言族）。

复现/修复证据：
- 机制①：临时探针在 slice 1 注入 2s 延迟（等价满负载拖慢）→ 原代码 slice 2 失败 `phase="fs3_project_scan"`（期望 fs4），修复后同探针通过；
- 机制②：40 个 busy-loop 满载（20 核 ×2）下原代码 `-count=100` 失败 5 次（slice 3/4 pending 缺失），状态转储证实 slice-4 记录在断言前已被后台 worker claim+complete（created 后 ~6ms completed、attempt=1）；修复后同负载 `-count=100` + 探针 200 迭代全绿。

修复（`test(d2-2-stab1)` 43f6909，仅测试隔离/同步/时序层，零生产代码改动、零断言放宽）：快照时间戳改从服务器 loop 自身 `UpdatedAt` 推导（`before.UpdatedAt.Add((index+1)s)`，overlay 恒新于合并基座）；同步驱动后复位 `continuationExecutor=nil`，令 wake 的 executor 门永不成立、后台 worker 永不启动；同机制波及兄弟测试 `TestProductionFreeStateRunnerObservationsSurviveDurableSlices`（S3f 批同族抖动）→ 置 `schedulerWake=nil`（其驱动面完全同步，不需要 wake）。

验证：`go test ./internal/chat -run 'TestFreeStateObservationsSurviveFourDurableSlices' -count=10` 绿；满载 `-count=100` 绿；兄弟测试满载 `-count=30` 绿；全量 `go test ./... -count=1` 三连全绿（2026-08-30 18:15:25 / 18:16:03 / 18:16:45，84 包 ok）。

旁证：S3h8 批 `TestProcessorCertificationStartAcceptsBroadbandCompressorCapability` 的 Windows TempDir 清理竞态与本次无共同机制（前者为测试框架 TempDir 清理 vs 残留句柄竞态，本次为测试自身时序/调度器隔离缺陷），按卡纪律记录不修。

## 24. 2026-08-30 晚批（D2-2-CLEAN1-1 裸 continue 拦截面读侧租约：防御纵深补位，写入侧谓词读侧贯彻）

病灶一句话结论（CLEAN1 审计 F7/L1）：continue nudge 拦截面的扫描函数 `interactionContinuationForConversation`（goalrunner_chat.go:2889）对**任意** waiting_interaction park 无过滤透出——写入侧已在 arm/迁移双点消费 `continuationOccupantDrivable`（S3h7）并经完成桥终态化已应答 park（S3h8），但读侧零过滤；非 armed 的 pre-C legacy 快照经 server.go 兼容分支仍会物化 `legacy_waiting_continue` 不可应答空壳，一旦再现，聊天 nudge 会被永久吞成 `pending_interaction_requires_response`（S3h7 死亡链第 5 步的读侧复刻）。当前栈无触发（D2-2 快照无 pre-C legacy），属防御纵深缺口。

修复（`fix(d2-2-clean1-1)` 70e72b1，扫描循环内一行谓词复用，拦截面分支字节不动）：`interactionContinuationForConversation` 跳过 `!continuationOccupantDrivable(item.Continuation, &item)` 的 park、最新**可驱动**者胜出。取舍记录：① 不可驱动 park 选**放行 fall-through**（与 beginChatGoal 终态 goal 不复用而 BeginGoal 同构），读路径不改写 durable——位移/终态化仍归写入侧所有（终态记录永不改写红线）；② 过滤落在扫描函数而非拦截分支内，使可驱动形态的拦截语义/文案/InteractionRequests 透出**一字不动**（红线最强形式），且"更新的空壳不得遮蔽更早的真实确认"由"最新可驱动胜出"自然成立；③ 驱动面兜底扫描 `goalContinuationForConversation`（空壳仍可作为 pre-C 检查点被显式 continue 驱动，属 pre-C 语义保持）**不在本卡扩面**——该扫描属 CLEAN1-2 的 run 任期领域。

验证（RED 先行）：RED 双例按卡内预言失败——`TestContinueNudgeDoesNotSwallowLegacyWaitingContinueShell`（扫描透出空壳）与 `TestLegacyShellDoesNotShadowDrivableConfirmationPark`（newest-wins 让空壳遮蔽真实确认 park）；对照双例修复前即绿锁定红线——`TestContinueNudgeStillInterceptsDrivableWaitingConfirmation`（拦截 stop reason/精确文案/workflow 数据/可应答 request 透出全不变）与 `TestContinueNudgeIgnoresTerminalResiduePark`（终态残留不达拦截面 + 谓词判非驱动）。定向 `go test ./internal/chat -run 'Test.*(Nudge|Continue|Lease|Drivable)' -count=1` 绿；chat 包全量绿（含 S3h7 谓词 9 形态表/arm 槽位、S3h8 完成桥、park 消费点全部不回归）；全量 `go test ./... -count=1` exit 0。

旁证：默认路径 -SkipBuild 冒测按卡内许可**与 CLEAN1-2 合并一次执行**（同工作树同文件串行，本卡先行合入）。

## 25. 2026-08-30 晚批（D2-2-CLEAN1-2 读侧任期：conversation/goal 键扫描与 restore 再绑定按当前 RunID 门控——CLEAN1 双卡收口）

病灶一句话结论（CLEAN1 审计 F8/L2）：写入侧完成桥已 run 匹配（conversation+goal+run+interaction_id），但 continue 拦截/自动续跑应答/resume 链的四个 conversation/goal 键扫描（`automaticContinuationForConversation` / `interactionContinuationForConversation` / `goalContinuationForConversation` / `goalContinuationForCurrentGoal`）**只按键不校验 run**——同 conversation 开新 goal run 后，旧 run 非终态 durable（未应答 waiting_confirmation park、parked 自动续跑检查点）仍吞新 run 的 nudge 或顶替其 resume（"旧轮残留吞新轮驱动"）。restore 一层同病：`normalizeRestoredDurableContinuation` 无 TaskContract 时 `item.RunID = firstNonEmpty(goal.RunID, item.RunID)` 把旧 run 记录**静默再绑定**为当前 run（有合同时由合同身份校验兜住，无合同时无隔离）。

修复（`fix(d2-2-clean1-2)` 95e2835）：① 四扫描按现任 run 过滤（`currentRunIDForConversationLocked`：conversationGoals→`RuntimeStatus(goalID).RunID`，free-state loop.RunID 回退；`durableContinuationInCurrentRun`：两侧任一身份为空即放行——无权威 run 时不围栏，当前行为零变化）；② `resumeContinuationForChat` 的 `durable_continuation_id` 精确寻址分支**不围栏**（调度器检查点寻址，非键扫描）；③ restore 守卫 `restoredRunBelongsToEarlierGoalRun`：**有效非终态状态 + 双侧 run 身份齐且不等** → `recovery_validation_required` 隔离并**保留真实 run 身份**（完成桥的 run 匹配因此保持可信），不再静默再绑定；终态记录永不改写（mark 原语只动 runnable 态）；无效状态记录保持既有任务快照身份修复（锁定测试 `TestRestoreUsesTaskSnapshotAsIdentityAndIntentAuthority` 的修复方向原样保留——run-corrupt/unknown_status 形态不走隔离分支）。

验证（RED 先行）：RED 四例全部按卡内预言失败——跨 run waiting_confirmation park 不拦截新 run nudge（并落到诚实 no_continuation）、跨 run pending 检查点不以"已在自动续跑队列"应答、跨 run durable 不喂 goal/conversation 双 resume 读源、restore 无 TaskContract 旧 run 非终态被隔离而非再绑定（RunID 保持 run-fence-stale）；对照 `TestRunFenceKeepsCurrentRunBehaviorUnchanged` 锁同 run 红线（拦截文案/envelope/InteractionRequests 与自动队列应答不变）。定向 `go test ./internal/chat -run 'Test.*(RunFence|RunIdentity|StaleRun|CrossRun|Nudge|Continue|Lease|Drivable)'` 绿（含 CLEAN1-1 全部用例）；chat 包全量绿（S3h7 arm 槽位/谓词 9 形态表、S3h8 完成桥、restore 幂等、free_state 全家不回归）；全量 `go test ./... -count=1` exit 0。

真栈回归（CLEAN1-1+CLEAN1-2 合并一次）：默认路径 p01 freq -SkipBuild **exit 0**（20260830_185606）。过程债如实记录：首轮 20260830_185124 PASS 跑的是 12:53 的**旧二进制**（-SkipBuild 复用 `agent\bin\VitAgent.exe`，早于 18:40 合入的修复）——重建二进制（18:56）后经 `-RestartAgent` 重跑，PASS 方为有效回归；未来 -SkipBuild 冒测前须核对二进制 mtime 晚于待验代码（已在本批踩坑，机制留档）。

收口：**CLEAN1 双卡（读侧租约 CLEAN1-1 + 读侧任期 CLEAN1-2）以 70e72b1/95e2835 + 20260830_185606 为同一戳连关**，CLEAN1 审计 F7/F8 两潜伏病灶关闭（F9 留档观察、F10 范围外移交 admission/evidence 家族的裁定不变）。fix 合入，不 push。

## 26. 2026-08-30 晚批（D2-1.5-S2f-2 压缩域正向用例 fixture：证据链修复 + 割线收敛 delta 机器 + spv1_p03 透明链端到端——**压缩域正向链首次 exit 0**）

目标一句话：为 broadband_compression 建立正向端到端用例（模型提案物理可达 → 执行成功 → A/B 证据可信），验收从 p01+compression（概念性 +1 → 物理拒绝，观察项）迁移至新用例。

**设计裁定（GLM L2，卡内定稿）**：载体 = 新 case `spv1_p03`（不用 "raw" 命名，case id 不透明纪律）——stems 复用 p01 公开 stems（同 sha256，manifest 只追加第三 case、既有条目逐字节不动），工程经 product path（`semantic_processor_project_smoke_projects.py`）重建**透明链**（0 插件实例，实测 p03.vit 无 PLUGININSTANCE/SSL）+ fresh `.vit_agent`；正向方向 = 新 prompt flavor `leveling`（动态松散前提，与 frequency/compression 同构）引导负 threshold 增量（新实例化 VSC-2 默认 +11.8 dB 顶格，负增量唯一物理可达，S2c 已验）——家族卡"引导负增量被否决"仅针对过压素材上的声学无意义动作，松散素材上加压缩声学有意义，分层 4 用户裁定口径一致。判据机器 = runner 侧 numpy+soundfile 逐轨机器计算 peak/rms/crest（门槛 rms>-45 每轨、crest≥10 每轨、最优 crest≥14；p01 stems 实测 14.5–21.0），进 report `material_qualification` fail-closed，不预选目标轨不进 agent 上下文。

**证据链修复（S2f-2a 报告 §4 强制项）**：①§4-6 诊断——`freshWaveformRowOrMissing` stub 增补 `judged_track_id/judged_clip_id/judged_request_id/judged_status`（compact 白名单同步放行）；②§4-1 变体——`normalizeTrackWaveformFeatureFreshness` 对 requested_features 非空且不含 waveform_envelope 的 latest_request（跨运行 l2 render probe 形态，冻结 p01 快照实证 `mixboard_l2_render_probe_1032`/status requested）不做 request 身份失效判定，TW 行交给 material/project-state 过滤（kernel 遥测 packet 自带三类 feature 清单不受影响，空/缺列表保持权威 fail-safe）；③防错归位加固——`preserveTargetWaveformTimeSegments` 在请求带具体目标、WE 槽描述其它目标、目标轨无可回填行时置诚实 missing（reason `waveform_row_not_bound_to_requested_target`，带 displaced 行身份）——堵死"静默消费错轨 ready 行"（比 loud rejection 危险）。权威路径行为字节不变。**真栈首发命中**：192556 轮工件 obs_20260830T112631 实录 not_bound stub（目标 1012/drums，displaced vocals 1032/1036 ready 行）——in-run kernel 遥测窗口（waveform-authoritative 基线跨目标）由该加固诚实降级，剩余微节点待后续 D1 工件按 judged_* 字段定案。不采用 §4-4（§4-1 已解根因，冗余改动面）。

**执行层发现链（正向链首次真实写入暴露的三层缺陷，全部卡内目标"执行成功"范围）**：run1 192556——模型自主以 **threshold_db=-1** 准入（leveling 语义生效）并执行到内核，delta 机器 ±0.25 归一化窗口的平均斜率外推在非线性 taper 上失准（目标 10.8 dB 实达 10.1，容差 0.15）→ **割线收敛机**（`refineDeltaChannels`：写入后按最近两实测点投影迭代 ≤4 次，每次 write+rebase+读显示文本，耗尽恢复原值零净移动，trace 进 receipt `delta_refinement`；测试改写 `GateRejectsCurvatureMiss` 为收敛断言 + 新增耗尽恢复测试）；run2 193330——refine 写入被拒：主写入后端口 CAS base 未 rebase（原流程主写入后无后续写入）→ refinement 顶部先 rebase；同时修正成功路径复核快照 `<=` 为 `<`（真内核重读同 revision，假客户端单调队列曾掩盖）；run3 193601——**执行链全绿**（receipt applied、before 5→after 7、readback 10.8 精确、割线 1 次收敛、human_audition_ready ✓），挂在本卡新断言过严（要求 round 全部观察披露 time_dynamics——模型自主选了 basic_energy，D1 契约模型自选视图、服务端注入禁止）→ 断言改形：round 观察 requested ⊆ executed + runner 侧 time_dynamics **可披露探针**（对已准入目标发真实 ccb.observation_request）；run4 193906——探针答 **partial**：取证为健康披露态（COM source_dynamics 全量在场 crest 15.611/4 有效段/decision_support supported、freshness ready/revision 绑定，partial 源于 source_only 设计性限制），contract formal gate 本就是 "ready **or partial** and fresh" → 断言接受 partial + 拒 rejected/missing + freshness 非 stale。

**验收（20260830_194058，含构建禁 -SkipBuild）**：`spv1_p03 -PromptFlavor leveling -ExpectDomain broadband_compression` **exit 0**——模型自主准入 broadband_compression threshold_db=-1（目标轨 1012/drums），执行 receipt applied（before 5→after 7、transaction/idempotency 全、readback **10.8** 精确命中 current 11.8+(-1)、delta_refinement 1 次迭代收敛）、post-action CCB fresh 且 revision 绑定（obs_20260830T114159）、A/B audition 双候选就绪（render/preview revision 互异）、time_dynamics 探针 partial+fresh 且执行视图含 time_dynamics、素材判据 6 轨全过（best crest 20.989/guitar）。RED→GREEN 纪律：mixboard 三新测试（非波形基线保留 ready 行全链到 COM ready / 错轨 WE 槽诚实 stub / §4-6 诊断字段）先行 RED 失败形态与预测一致后修复转绿；全量 `go test ./... -count=1` exit 0。

**p01 freq 回归（-SkipBuild）**：见下表（单轮路径零变化红线 + 二进制 mtime 核对前置执行：agent 19:41 ≥ go 源 19:35、kernel 12:00 ≥ 全部 C++ 源）。

| stamp | 轮 | 终态 | 备注 |
|---|---|---|---|
| 20260830_192556 | p03 leveling 正向 | fail "D1 receipt requires distinct before/after revisions" | 首轮：模型 -1 准入✓、内核写入✓（revision 推进 6）、斜率失准 readback 10.1≠10.8 → applied_unreconciled（细节缺 before/after）；**§4-6/not_bound 诊断同轮真栈首发命中** |
| 20260830_193330 | p03 leveling 正向 | fail 同上（错误前移） | 割线机已进二进制：refine 写入被 CAS 拒（主写入后 base 未 rebase）→ 修复 |
| 20260830_193601 | p03 leveling 正向 | fail "did not disclose track.time_dynamics" | **执行链全绿**（applied 5→7、readback 10.8、audition ready）；断言过严（模型自选 basic_energy）→ 改形为 requested⊆executed + 可披露探针 |
| 20260830_193906 | p03 leveling 正向 | fail "not disclosable: partial" | 探针语义修正：partial=健康披露（contract gate "ready or partial and fresh"），取证 COM 全量在场 |
| 20260830_194058 | p03 leveling 正向 | **pass（exit 0）** | **压缩域正向端到端首次 exit 0**：全链证据见上 |
| 20260830_194243 | p01 freq 默认路径（-SkipBuild） | **pass** | 单轮路径零变化（static_eq before 9→after 10、readback -1）；mixboard 修复在冻结 p01（l2 probe 残留基线）上同栈生效；素材判据机器对既有 case 同样全过 |

留档：①spv1_p03 fixture（manifest 第三 case + 透明链工程 + fresh .vit_agent）建于 temp/ 夹具集，p01/p02 既有条目与 sealed 语义不动；②run5 的 d1_receipt 持久化投影读取为顶层 responses 权威（validation dict）——persisted_loop 快照时间差属已知投影竞争，非缺陷；③p01+compression 观察项维持 8/28 记录不变；④S2f-1 激活问句仍在域表（家族卡 revert 裁定未排，本批 5 轮均携问句跑，正向链不受其扰）。

收口：S2f-2 卡 todo→done；fix 合入 `feat(d2-1.5-s2f2):`，不 push。开放项移交：kernel 遥测型 latest_request 的跨目标 in-run 窗口（preserve 加固已诚实降级 + §4-6 诊断在位，是否豁免待后续 D1 工件定案——本批不裁）。

## 27. 2026-08-30 晚批（D2-EVID1-fix1 绑定面裸观察 ID 透出：裁定 B 单点落地——evidence on/off 引用面统一）

改动摘要：`mixboard/catalog.go observationBinding`（读时构造，:1030）的 `evidence_refs` 由原样透出持久化 refs 改为**前置 `obs.ObservationID` 后去重**（`uniqueObservationRefs`，obsID 为空维持原样）——durable packet 的 `EvidenceRefs` 持久化形状一字不动（`persistence_v2.go` 未碰，实测 v2 观察文件 refs 仍只含 `evidence://<sha256>`），准入门/D1-S1/G7/prompt 语义零改动（红线全守）。效果：evidence 开 `[obsID, evidence://…]`、evidence 关 `[obsID]` 两种存储状态对模型与全部下游（CCB bundle、账本 receipts/available_views、G7、D1-S1、诊断 known-refs）呈现统一引用形态；`lazyFeatureSnapshot` 按 `evidence://` 前缀过滤不受影响（核对未改）。

验证（RED 先行，20260830_195814-195816）：mixboard 三新用例按卡内预言失败——`TestObservationBindingEvidenceRefsPrependBareObservationID`（evidence 开 refs 首项为裸 ID）、`TestObservationBindingEvidenceRefsBareOnlyWhenEvidenceOff`（evidence 关 refs=[obsID]）、`TestObservationBindingEvidenceRefsDeduplicatesBareObservationID`（已含 obsID 不重复）；空 obsID 对照用例修复前即绿（维持原样红线锁）。实现后 `go test ./internal/mixboard -run 'Test.*Binding.*' -count=1` + `go test ./internal/capabilitycontext -run 'Test.*Observation' -count=1` 绿（含新 bundle 透传断言 `TestAssembleFreeStateObservationBundleCarriesBareObservationIDInEvidenceRefs`：AssembleFreeStateObservation 的 EvidenceRefs 首项为裸 ID）。全量 `go test ./... -count=1` exit 0（20260830_195818→195922，84 包 ok，重跑取证戳）。

真栈烟测（**全新存储 evidence 开场景**，20260830_195743→195930，exit 0）：冻结夹具 .vit_agent（evidence_off）会掩盖本路径，故以 staged 全新工程跑——`temp/evid1-fix1-fresh-store-smoke/projects/spv1_p01/`（仅 .vit，无 .vit_agent）+ 指向该工程的 manifest 副本，`run_free_state_d1_smoke.ps1 -PublicCaseId spv1_p01 -PromptFlavor frequency -SkipBuild`；二进制 mtime 核对前置（agent 19:57:01 ≥ 全部 go 源 19:55 窗，-SkipBuild 复用合法）。实测 live 工作区 manifest `degraded.evidence_off=false`、evidence 5 blob/1.6MB、durable 观察 refs 仍只含 URI（持久化红线）；账本 receipts/available_views 的 evidence_refs 实录 `[obs_…, evidence://…]`（裸 ID 首项，绑定面透出生效）；报告 status=pass，D1-S1 无 `free_state_experiment_admission_invalid`、无 loop blocked、未 NOT_EXERCISED，static_eq 提案准入并执行（before revision 12→after 13）终态等待人工判定边界（fs8）。

收口：EVID1-fix1 卡 todo→done（commit `12e028d`，fix 合入不 push）；EVID1 裁定 B 落地，F10 关闭；A 式准入宽容维持观察项（模型只引 URI 的残余风险留档，另行裁定）。

## 28. 2026-08-31 早批（D2-MRREG1：p01 多轮探针"回归"定性为启动器档位环境缺口——非代码回归，四嫌疑提交全部洗清）

**症状与晚关口误读**：8-30 夜间闲时回归 p01 多轮探针连续两次 NOT_EXERCISED（exit 3，`20260830_204218`/`20260830_205128`），gate 据此定性"回归"并锁定 12:53 后四提交（CLEAN1-1 `70e72b1`/CLEAN1-2 `95e2835`/S2f-2 `ec5711e`/fix1 `12e028d`）为嫌疑窗口；gate 另判 20:51 为"未自选期望域"第二形态——**该判读有误**：ps1 exit-3 分支对一切形态打印同一条笼统 "did not autonomously select ..." 消息，两轮 report 的权威 `reason` 均为 "admission granted experiment_budget 1"（同形态，`selected_domains` 均含 static_eq）。

**取证链（125328 vs 204218 vs 205128 admission 阶段对比）**：三轮模型行为同构——域选择均到 static_eq、提案均为 300Hz 单步 -1dB `static_eq_band_adjust`（dose bounds `source=proposal`）；唯一分叉在 `verification_plan.experiment_budget`（2 vs 1 vs 1）。budget 不是模型可影响量：D2-2 档位 env 单源 `VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET`（`agentloop/free_state_d2_multiround.go`，封存 2..4，缺失/畸形 fail-closed 回落 1，模型不可升档，D2-2 勘察文档 §准入档位接线）。125328 agent 日志第 153 行有 "D2-2 multi-round admission tier budget=2 source=VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET"；204218/205128 **无此行**——档位从未启用，admission 只能照模型单步提案发 budget=1。全仓 grep 证实除 Go 常量与 D2-2 勘察文档外无任何脚本/runbook 管理该变量：12:53 通过是会话手工注入，闲时通道裸调 ps1 必然确定性复现。三嫌疑就此洗清：①证据面漂移（fix1 refs 形态）——budget 为服务端 env 单点控制，与模型可见引用面无关；②夹具 journal 累积——夹具 `projects/spv1_p01/.vit_agent` 各 journal 最后写入停在 8-30 13:26，三轮失败 run 走 per-run workdir 未触碰夹具存储（S2f-2a 透明链纪律生效）；③CLEAN1-2 RunID 门控——失败死在准入 tier，多轮驱动链未参与。

**修复（启动器拥有档位注入）**：`run_free_state_d1_smoke.ps1` 三处——①新参 `-MultiRoundBudget`（默认 2，`ValidateRange(2,4)`）：`-MultiRoundProbe` 时无 env 则注入本进程（子链继承）、有 env 须与参数一致否则硬错（拒绝静默覆盖）、畸形/越界 env 硬错；②非 probe 运行发现有效档位 env（2..4）硬错（否则单轮尾 py `experiment_budget must equal one` 断言确定性失败）、无效值仅告警；③exit 3 消息改为打印 report 真实 `reason`（修掉误导 gate 的笼统措辞，读取失败回退原文案）。负例三连实测（全部构建前秒级失败）：`-MultiRoundBudget 5` 被 ValidateRange 拒；env=3 vs 参数 2 冲突拒；非 probe + env=2 污染拒。

**判别+验收二合一重跑（`20260831_085320`，含构建，当前 HEAD 含全部四嫌疑提交）**：单变量 = 档位 env 注入 → **MULTI_ROUND_PROBE PASS，exit 0**（budget=2、rounds_used=2、pending_boundary_round=1、action_domain=static_eq、finished 09:01；tier 注入行见该 run agent 日志 :167）——环境缺口定案与修复生效一次证明。

**p01 默认路径回归（`20260831_090227`，-SkipBuild）**：**pass exit 0**（selected_domains static_eq+track_gain，finished 09:06）——单轮路径零劣化。mtime 纪律注记：agent 二进制 08:53:25 为 HEAD 构建；go 源新于二进制的两条（d1s1_domains.go/whitelist.go）是 FAM1-S1 并行会话在途接线、smoke.py 的 ADMITTED_DOMAIN_KINDS 镜像行同理——均为 flash 线未提交改动，刻意不入本验证面。

收口：MRREG1 卡 doing→done；fix 合入 `fix(d2-mrreg1):`，不 push。观察项修正：晚 gate "两种形态"与"四提交回归窗口"两项判读作废；"夹具 journal 跨 run 累积"维持 S3h8 已知机制留档（本案实证 8-30 晚间各 run 未污染夹具存储）。

## 29. 2026-08-31 早批（D2-FAM1-S1 齿音域探测定锚 + 域表/白名单接线——第 13 域 admitted，单共享参数形态首次接线）

**probe 三问（pluginprobe 实测 FabFilter Pro-DS，观察宿主 loopback + native worker，产物 `%TEMP%\vit_fam1_probe\`）**：① Threshold = 全表面 157 参数中**唯一** threshold 参数，id `"1"`（`vst3_hosted_parameter_id`/stable/automatable/连续，display "-36.00 dB"），拓扑 Stereo In 2ch（另有禁用 side chain bus）→ Stereo Out 2ch——**单共享参数驱动立体声，非 PA 系 ch1/ch2 对**，`WriteBinding.Channels=1`；② 新实例默认 `default_normalized=0.400000005960464` / "-36.00 dB"，unload→reload 全参数表面逐字节一致（确定性），**0.4 严格内点 = 居中 → 双向物理可达**（非 VSC-2 型顶格），GLM 裁定②批准 flavor 按 leveling 模式写减齿音方向（对齐 coverage 轴 sibilance_reduction 与 i03 fault）、不写死唯一方向；③ display 写入响应**观察宿主不可判定**（worker 操作集 load/snapshot/render/unload，write 族按设计缺席）——如实上报，`delta_db` 照抄不回头（勘察 §1.4 机器域无关 + p03 全链），运行时 fail-closed（restore 零净移动）兜底，FAM1-S2 首动作事务性现场检验。载体准入核对：`pcactl query-v2 -subject-key pcs1_06dd3d0a4a46db3c8ac48a19 -axis sibilance_reduction` → eligible/promoted（认证 `pca2_8f262031241647ba834eb809`），认证指纹 `66650d44…ee656` 与现算文件指纹一致 = 磁盘即 2026-08-07 认证份，零新认证。

**接线（commit `922bff7`，10 文件 +824/−17）**：域表第 4 行 de_esser（`de_esser_threshold_adjust`/`_deess`/`static_mix.de_ess.v0`/threshold_db ±2/观察视图 `track.frequency_time_events`/Channels=1/`delta_db`/无 stub form）；白名单 schema **v3** + `DeEsserPlugin` 单 `threshold_param_id` 节；chat `resolveD1DeEsserWhitelistBinding` 分支 + `d1DeEsserAttestationReader`（NewStoreV2）伴生；smoke.py `ADMITTED_DOMAIN_KINDS` 镜像行。GLM 三裁定全落：**①Form A 双派发**——`ValidateDeEsserAdmission(libV2 LibraryV2)` 经 `validateSectionAdmissionV2` 孪生谓词（边界措辞逐字镜像 v1），Form B（谓词自选库）因必改 static_eq/compression 冻结签名被否；**③Preflight 守卫放宽（带两修正）**——normalized-batch 守卫从"plugin_path+param_id_ch2 非空"放宽为仅 plugin_path（ch2 是 PA 载波形状假设非安全属性），Apply 调用点过滤空通道 ID（修正 a：非 delta 分支 `eqPlanGainChannels` 对空 ID 硬错，必须调用点不传实参），单通道动作 args **省略** param_id_ch2 key（修正 b，receipt details 维持空串诚实记录"无第二通道"）。零改动核对：`ccb_model_prompt.go` 表驱动自动派生、`free_state_d1_runtime.go` 泛化执行路径，均只核对未动。

**RED 纪律**：守卫放宽前四测试先行（裁定三形态 + 单通道 delta E2E 附赠），实现前三单通道测试按预言死于旧守卫（"requires plugin_path and param_id_ch2"）、双通道对照绿；实现后 executionports 全绿（一次测试侧期望值笔误：单通道 delta 断言误用绝对式公式，实现输出 23/60 正确，修断言不修实现）。chat 侧新增 `TestResolveD1PluginParamBindingForDeEsser`（六类边界：unconfigured/invalid/unreadable/ineligible/pinned/success，镜像 compression 类结构，v2 reader 独立注入缝）+ `TestD1S1DeEsserPlanEmbedsWhitelistBinding`（args 省略 key 断言 + 压缩行 ch2 key 存活对照）；experimentplugins 侧 v3 round-trip/畸形拒绝/准入五类（promoted/指纹漂移 binary_fingerprint_changed/no_attestation/unconfigured/absent 文件换 os.ErrNotExist 包裹）。**验收全绿**：`go build ./...` + 分包（experiment/experimentplugins/chat/agentprotocol）+ 全量 `go test ./... -count=1` 84 包 exit 0 + py_compile 过。

**机器本地 `~/.vit/free_state_experiment_plugins.json` v3 + de_esser 节**（与代码同批落，窗口最小化——v3 文件与旧 v2 二进制互斥，MRREG1 并行线在途期间文件保持 v2 直到本批）：五元组与认证 subject 逐字一致（name "Pro-DS"/identifier "VST3-Pro-DS-831d2251-cba24108"）+ `threshold_param_id: "1"`。残余风险留档：参数 ID 跨宿主一致性（JUCE probe worker vs Tracktion 内核面）由运行时活表面核对兜底（ID 不符 = 首动作诚实 fail 不误写）。

收口：FAM1-S1 卡 doing→done；不 push；解锁 FAM1-S2（fixture + sibilance flavor + 端到端烟测，flavor 方向按裁定②定稿）。真实栈正向链未验（本卡验收面仅 go test + py_compile，按卡执行）——delta 机器对 Pro-DS 的现场斜率发现是 FAM1-S2 烟测的第一个观察点。

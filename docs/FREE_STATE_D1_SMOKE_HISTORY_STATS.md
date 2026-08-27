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

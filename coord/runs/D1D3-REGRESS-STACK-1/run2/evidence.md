# run2 取证（D2 门组 tracks 面两断言失败：分列=断言预期面漂移，非修复缺陷回归）

- run：`20261108_181759`（18:17:59 起栈→18:22:4x PS1 exit 1）；单 run 授权用尽，未重跑、未现场改断言。
- 被测基线：port/d1d3-regress-stack-r2 860d318d = origin/main 8c68c186 + cherry-pick 272f27e9（D2 断言修正）。274 f413fbca / 292 5c8712a6 / **295 f6d1982e 三修 merge-base 亲证在基线**（run1 归因②消解前提成立）。agent 二进制 SHA256 82A70AA8F9147CBF9F49190BD15BF50F98FE31DC5CD7BBD33FDEE4339DAEA3A7；kernel F9E3DA0A…（run1 同款只读复用，18:14 亲核）。

## 结果总览（PS1 exit 1）

| 段 | 结果 |
|---|---|
| runner 主链 D1-S1 | **PASS**（exit 0；branch=zero_mutation_boundary，validation status=pass，forward/journal mutation 均 0） |
| D1-settle 门组（含 266 D1 门 A 脚本位） | **structurally skipped**（zero-mutation 分支 settle 门结构性不可达，skip-with-reason 在档 d1_settle_gates.json：无 apply→无 watch hook→无 verdict store） |
| D2 门组 | **2 失败**（gate A tracks 面两断言；其余 5 门全绿） |
| D3 门组 | **未到达**（脚本序在 D2 后，D2 throw 先停） |
| LLM 计费 | 9 调用 deepseek-flash，in 98,489 / out 8,274 / cache_read 23,552 tok，**0 error**（run1 首轮 transient 未复现）；估 ≈¥0.1-0.2（预算 ≈¥0.6 内） |

## 失败点原文（d2_gates.json failures）

1. `per-track face counts (considered=6, rows=6, with_metrics=6, without_metrics=0; expected 7/7/6/1 …)`
2. `the no-metric row must be track 1039 classified read-without-metrics (got track_id='', status='')`

其余 D2 断言**全过**：global stereo_status=**present**（274 ground truth）、1007 锚值 corr=**0.689**/balance=**-1.475**（292 单测锚 ±0.0005 命中）、payload tracks_measured=6、check turn 零 mutation、verdict store 在盘、action_face=disclosure_only_v1、D2 node=in_progress。

## 分列结论：断言预期面漂移（272f27e9 预期面按 1-mutation 分支活栈形态书写；本 run 走 0-mutation 分支）

### 证据链

1. **本 run 走 0-mutation 合法分支**：响应链 2 步（resp0=improvement_proposal_confirmation_required，resp1=semantic_treatment_capability_boundary，message=「semantic progressive disclosure observation audit is not successful: partial」）。py runner 分支容差契约（free_state_d1_smoke.py:1386 validate_zero_mutation_boundary，D1-RUNNER-FORENSICS-1/256）**明文在档同形态先例 armrun2 20261105_215624**（提案→确认→semantic-treatment 路由→observation audit "partial"→诚实 blocked，零 mutation 零叙述）——与 run1 的 capability_blocked 形态**不同型**（run1=post-load G7/G8 blocked+settle 尾缺席 py:1770 exit 1；本 run=orchestrator 入口 audit partial+完整 validation pass exit 0）。**止损线（两现）未触发**。
2. **fixture 工程本体只有 6 轨**：spv1_p01.vit 的 TRACK 元素=1007(bass)/1012(drums)/1017(guitar)/1022(other)/1027(piano)/1032(vocals)；**"1039" 在工程文件里是 rack PLUGIN id（`<PLUGIN type="rack" rackType="1037" id="1039">`），不是轨 id**；.vit_derived 分析数据（acoustic_package_status/analysis_manifest/l3_feature_log）同样无 1039。
3. **"track 1039" 的真实来源=1-mutation 分支 runner 新建 FX 轨**：265 reverify1 run2（2026-11-06，PS1 exit 0，1-mutation 主链）的 d2_check_chat.json 实盘 face=**7 行全 missing，含 track_id=1039**——即 265 那轮 runner apply 后工程态 7 轨（6 fixture 轨+新建 FX 轨自动 id 1039，无 stereo 行→诚实 missing）。292 单测注释「265 实盘形状：track 1039 无行」把该**分支专属活栈形态**固化成了预期面；272f27e9 离线双形状干跑用的正是该单测形状（修复后 7/7/6/1 形状全过），故断言从未对 0-mutation 分支的 6 轨基线校准。
4. **本 run 的 6/6/6/0 face 是修复后正确读数**：D2 观察的 track_stereo=6 行全 status=**read** 且带指标（1007/1012/1017/1022/1027/1032，corr 0.689/0.974/0.227/0.225/0.088/0.979）；对照 265-era 同 fixture 同门 face=7 行全 **missing**（pre-292 catalog_key_not_found 形态）——**292 track-key 路由修复在真栈兑现**（恒 missing→全 read+锚值命中）。
5. **274 digest 接线修复同场兑现**：本 run D2 carrier observation loudness_peak_status=**present**+loudness_ranking_rows 6 行（1032 vocals -19.843 …1007 bass -30.255）+stereo_source_status=ready；265-era 同位=missing/None/None。

### 与 run1 归因对照

- run1 归因①（LLM 概率腿 settle 边界停）：本 run 未复现同型（runner exit 0，settle 断言未触发；边界停移前到 orchestrator 入口且为在案合法分支形态）。
- run1 归因②（295 修前提示词面）：已消解——f6d1982e 在基线亲证；本 run 0 LLM error。
- run1 归因③（274/292 回归）：**反向证据**——两修消费面（global stereo present/track-key read+锚值）真栈全过。
- run1 归因④（294 卫生）：无交叠证据变化。

## 交主管裁定项

1. **D2 tracks 面两断言的分支感知修正**（建议）：0-mutation 分支预期=6/6/6/0（fixture 本体 6 轨全 read）；1-mutation 分支预期=7 行含新建 FX 轨 read-无指标行（新建轨 id 非恒 1039，应按「新增无 stereo 行轨」形状断言而非硬编码 id）。属断言块域内修正，是否重跑活栈兑现由主管定。
2. **D3 门组仍未活栈验证**（run1 未到达、run2 未到达）——274/292 回归门收口口径中 D3 面仍空。
3. **D1 门 A（266）脚本位在 0-mutation 分支结构性跳过**：274 目标键（loudness_peak_status）已由本 run D2 carrier 直接实证 present+rankings；是否认可该旁证替代（或需 1-mutation 分支跑 settle 门）由主管定。
4. run1 建议的 capability_blocked 取证卡：**两现条件未满足**（run2 非同型），暂无开卡依据。

## 工件清单（run2/）

RUNBOOK.md（§8 跑前成文）·console_run2.txt·ps1_exit_run2.txt·run2_meta.txt·d1_smoke_report.json（全量）·d1_settle_gates.json·d2_check_chat.json·d2_gates.json·d2_probe_chat.json·d2_queue_view.json·d2_verdict_store.json·agent_last.log·telemetry_run_window.jsonl（9 行全量）·agent_binary_sha256.txt

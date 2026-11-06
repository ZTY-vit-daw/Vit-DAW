# D3-REF-ALIGN-THIN-1 verifyrun1 evidence（单 run 取证，2026-11-07 01:12–01:18）

## 判定

- **PS1 exit=1**（run `20261107_011211`，01:12:11–01:17:54）。按 §8 预写②分类：**门断言失败，取证上交不重跑**（单 run 授权用尽）。
- 失败点=**已收口 D1-settle 门组 gate A 观察面子断言**（`observation loudness_peak_status was 'missing' [expected present]`，ps1:1100-1105）——**非本卡 D3 判定面**（D3 门组在脚本次序上位于 D2 之后，本次未及执行；D2 门组亦未及执行）。
- 其余已跑面全绿：D1 python runner PASS（1-mutation 主链 settle 完成）；LW 门 1-4+E1 全绿（disclosure/verdict store/dedupe/config-off silence/E1 extract）；D1-settle 的 stop_reason/verdict=breach/落卡（mix_tick_confirmation）/取消腿（mix_tick_rejected）/节点 needs_confirmation 全绿——**唯独观察面子 missing**。

## 取证锚点（最小复现链）

1. **kernel digest 自始无 D1 全局面**：`loudness_watch_observe1.json`（本 run 最早一次 mix.observe，01:13 LW gate 1）`result.digest` 键集含 `stereo_status=ready`/`balance_db`/`correlation_estimate` 而 **`peak_dbfs`/`rms_dbfs`/`crest_dbfs` 全缺**——D1 全局面在本 run 第一次 observe 起即缺席，非 settle turn 独有。
2. **settle turn 侧**：`agent_last_log_excerpt.txt` 01:15:11 `harness.invoke tool=mix.observe status=ok`（5.2s 成功返回）；`d1_settle_chat.json` `workflow_data.observation.loudness_peak_status=missing`、`observe_error=null`、global peak/rms/crest=null、limitations 无「mix.observe 未成功返回」无「返回空 digest」——**observe ok 但 digest 无 D1 字段**（d1d3 观察组装按缺失如实保留，行为符合 M1-4 语义）。
3. 判据消费侧无异常：settle 判断走 loudness_summaries 行（verdict=breach 如实产出、落卡/取消腿全绿）——失败仅在观察披露面子断言。

## 归因候选（交主管裁定，本腿不扩查不改码）

- **首次四开关叠加**：-LoudnessWatch+-D1LoudnessSettle+-D2StereoCheck+-D3RefAlign 全叠加为史上首跑（历史 exit 0 run 均为单门组或 -D2StereoCheck 组合，265 reverify 20261106_232316 亦仅 D2 门组）；但锚点 1 表明 D1 面缺席先于任何 LW 门动作，纯叠加次序归因不充分。
- **工程观察状态**：265 reverify（首个 1-mutation 完整收敛 run）当日 D2 门 A 观察到 `stereo_face=missing`；本 run 同 fixture `stereo_status=ready` 而 D1 面缺席——同一 kernel 二进制（wt_228 F9E3DA0A）下 fixture 观察面在不同 run 间漂移，疑与 runner 1-mutation 后工程观察状态/声学证据新鲜度相关（域外，未查证）。
- **agent 侧合入面**：被测=main fe8ab880（269/276/277 合入）+本卡 D3（D3 不触 D1/LW/observe 路径，chat 全包+LW/rlm/mixboard 单测全绿）。
- 域外缺陷定位锚点：`scripts/run_free_state_d1_smoke.ps1:1100-1105`（断言）；`agent/internal/chat/d1d3_premaster_observation_runtime.go` d1d3AssemblePreMasterReport（观察组装，消费不改写）；kernel digest 生产面（VitApp 侧，本腿不可见）。

## 计费（telemetry 原件 telemetry_run_window.jsonl）

- 本 run 窗口 7 调用 deepseek-flash（message_loop×runner 主链+semantic_entry），in+cache ≈22.7k tok，out 字段读 0（该批 section_stats 形态，如实记录）——估 **≈¥0.01-0.05**（预算 ≈¥1-3 远未触及：D3 face P 探针腿未及执行）。

## D3 验证腿状态

- **未验证**（门组未及执行）。判据/runtime/门组代码零改动（零代码改动边界遵守）；零栈面证据（15 单测+chat 全包+build）已在卡面回执。验证腿是否重派（含是否降级为 -D3RefAlign 单门组）交主管裁定。

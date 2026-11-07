# D1D3-REGRESS-STACK-1（296）run1 §8 口径（跑前成文）

- 时间：2026-11-07 晚窗前段；owner=GLM 主会话 L2-worker 子代理（ZCode/PC，会话 296regress）。
- 依据：卡面 296 + [274 裁定](../../../rulings/2026-11-07-D1D3-DIGEST-WIRING-1-pass.md)⑤（真栈回归门=266 D1 门 A+D2/D3 门）+ [292 裁定](../../../rulings/2026-11-07-MIXBOARD-TRACKKEY-1-pass.md)⑥（真栈回归门=265 D2 门组，6/7 轨+1039 诚实 missing+verdict 三态可测分支）。
- 被测 worktree：D:/Vit_DAW_wt_296regress @ port/d1d3-regress-stack-1 = origin/main ae537693 + 本卡脚本断言修正（仅 scripts/run_free_state_d1_smoke.ps1 D2 门组断言块）。**agent 源码零改动红线**：两修已在 main 亲证祖先（f413fbca=274 digest 接线、5c8712a6=292 track-key 路由，git merge-base --is-ancestor 双过）；294 fixture 卫生在权威位换入（WE 裸槽已剔，band_energy/loudness/stereo 三数组 (1007,1055) 休眠行按裁定保留）。观察读数可能随卫生面变化，如实记录。
- 被测 agent：worktree 现场构建 `agent\bin\VitAgent.exe`（SHA256 随 run 工件记）；kernel=D:/Vit_DAW_wt_228/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe SHA256 F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A（225-281 同款只读复用，登记前亲核）；引擎=本机默认无注入（deepseek-flash）。

## 命令

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_296regress\scripts\run_free_state_d1_smoke.ps1 `
  -RepoRoot D:\Vit_DAW_wt_296regress `
  -PublicManifest D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20\fixture_manifest.json `
  -PromptFlavor reverb -ExpectDomain reverb -D1LoudnessSettle -D2StereoCheck -D3RefAlign -SkipBuild `
  -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe
```

（与 265 reverify1 同款命令形+D2 同参；增 -D1LoudnessSettle -D3RefAlign（266 两门）；不带 -LoudnessWatch（LW 门组不在本卡范围，开关收窄对齐 266 裁定的变量收窄口径）。）

## 断言预期（新 ground truth；修正=对准修复后真值，非放松门）

- **D1 门 A（266）**：`loudness_peak_status=present`——ps1:1100-1105 既有断言原样（274 修后由 digest rankings 面驱动满足）；门 B/C/D 既有分支语义不变。
- **D2 门组（265）新增/收紧**（本卡 ps1 修改，锚=292 单测 catalog_trackkey_test.go）：
  - 全局面 `observation.stereo_status=present`（274 digest 视图修后读法；265 时代如实 missing 的旧形态反钉为必败）；
  - tracks 面 7/7/6/1：tracks_considered=7、行数 7、带 correlation 指标行 6（1007/1012/1017/1022/1027/1032 ready）、无指标行恰 1；
  - 1039 诚实 missing：该行 track_id=1039 且 status=read（mix.read 面 reason=stereo_relation_row_missing_for_track≠catalog_key_not_found→chat 读取面记 read-无指标，292 回执解读口径；status=missing=修复前形态必败）；
  - 1007 锚值：correlation=0.689±0.0005、balance_db=-1.475±0.0005（TestMixReadTrackKeyedStereoServesFullProjectSnapshotRows 实盘 round3 值钉死；其余轨值 fixture-dependent 只记录不硬编码）；
  - payload tracks_measured=6（d2AssessStereoRisk 排除 read-无指标行）；
  - verdict 三态取值 fixture-dependent 不预断言（阈值边界由零 LLM 单测钉死；1022 corr 0.12<0.25 预计触发 risk_found，仅记录）。
- **D3 门组（266）**：既有断言不动（三态 verdict+profile=builtin:spotify（intent_named）+grade=approximate+动作面分支+节点点亮）；294 卫生未触 loudness/RLM 面（(1007,1055) loudness 休眠行保留），取值如实。
- present/ready 断言一律带值/带形状，不降级为存在性检查；预期面修正对照入卡面回执。

## 运行次数 / 成功条件

- **单 run 无重跑**：一次起栈一次执行；同一确定性断点失败两次禁止原样重跑（AGENTS §8）。
- 成功 = **PS1 exit 0**：runner 主链 PASS（1-mutation 主链/0-mutation 合法分支皆可，:658 分支检测读出实际分支）+ D1 settle 门组 + D2 门组（含新增 global_stereo/tracks_face 两门）+ D3 门组全绿；face P 探针如实记录不挡门。

## LLM 参与与预算

- **确定性腿零 LLM**：D1 门 A-D、D2 门 A-D、D3 门 A-D（显式检查词法命中先于 semantic_entry）。
- LLM 腿=runner 主链 + D2/D3 两个 face P 探针（deepseek-flash 默认引擎）。预算 ≈¥0.1-1.5（266 verifyrun2 单轮 19 调用 ≈¥0.1-0.3；265 reverify1 run2 健康轮 16 调用 ≈¥0.2-0.5；上沿取 d2run1 单轮 ¥0.8-1.5）。跑后以 telemetry 亲数核对；若现 LLM 行如实分账（含可能出现的默认子腿）。

## 失败分类（预写）

1. **断言失败**：取证分列——修复缺陷回归（274/292 任一修在真栈失效，按门定位）vs 断言预期面漂移（fixture 卫生后读数变化/消费面新形态），不追绿不重跑；
2. **runner 排干族致死**（wait_scheduler_drain throw 形态）：按 265 在案归因记录取证（非高峰窗复现将推翻「晚高峰瞬态」归因，如实上交），不重跑；
3. **环境中断**（端口残留/进程冲突/启动 throw/loopback 10061 形态）：按中断记录拆栈，重建栈至多 1 次（256 先例③，仅限中断类）；
4. **发现必须改 agent 源码才能跑**：停止上交取证（零源码改动红线）。

## 止损线

- 单 run 结束即止：任何门失败→取证入本工件目录→卡面回执上交主管；不自行重跑、不自行改断言凑过、不把失败改写成成功；环境中断类的 1 次重建授权用尽即止损。

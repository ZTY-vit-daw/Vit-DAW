# Observation System v1 Acceptance Note

日期：2026-06-28

## 范围

Observation v1 的封口目标是稳定这条观测链路：

`Godot -> kernel -> feature snapshot -> DAD evidence layer -> Observation Model Layer (MOM/TIM/TOM/DOM projections) -> LLM context`

本次不扩展新能力，只固定观测基础、smoke 入口和防回归契约，避免后续 agent/action workflow 重构破坏 DAD / Observation Model Layer / Godot 的观测面。

## DAD / Observation Model Layer 分层

2026-07-06 架构校准：

- DAD 是 Evidence Layer，负责事实采集、轻量派生、状态标注和 evidence refs；DAD 不直接承担“混音判断”“工程整理建议”或“交付审查”。
- Observation Model Layer 位于 DAD 之上，负责把事实层压缩为面向任务的 compact projection，供 agent / LLM 使用。
- MOM = Mixing Observation Model，继续负责混音关系观察、action preflight 和 AB result 相关 compact context。
- TIM = Technical Integrity Model，负责工程技术完整性检查，例如 source/path/playback 有效性、格式归类、采样率/bit depth/声道覆盖率、DAD acoustic readiness、静音/削波等导入风险。
- TOM = Track Organization Model，后续负责智能轨道整理提案；它应优先基于 ID/命名关联、长度、mono/stereo、格式、声像和轻量波形特征聚类，再谨慎提出角色假设。
- DOM = Delivery Observation Model，后续负责导出/交付/响度/格式审查。
- 每个 model 输出自己的 projection artifact，例如 `mom.projection.v1.x`、`tim.projection.v0`、`tom.projection.v0`；LLM 默认消费 projection/context，不消费 raw waveform、spectrogram payload 或完整工程 dump。

- DAD 负责采集、派生和标注证据。输入来自 kernel/Godot 的 feature snapshot，输出 lightweight acoustic package 与 per-feature 状态。
- L1/L2/L3 是观测层，不是行动层。L1 提供 waveform/peak/RMS/time-energy，L2 提供 realtime/live meter 或 L2 Render Probe，L3 提供 full-song band/stereo/loudness summary。
- `band_energy_summary`、`stereo_relation_summary`、`loudness_summary` 必须带状态和 evidence ref；ready/suspect 都可以被报告，但 suspect 不能被当作可执行依据。
- L2 Render Probe 的 AB result 只在同 tap point、同 render mode、before/after 都 ready、且 render revision 改变时可信。
- MOM v1.4 负责把 DAD/feature snapshot/project package 投影为 LLM 可读的 compact observation。它只输出摘要、质量门、限制、evidence refs，不输出 raw package、waveform arrays、spectrogram tile payload、shared memory、文件路径或 revision 大串。
- FXM v0 = Effects Transformation Model，和 MOM/TIM/TOM/EPM 同级；它比较同源、同时间窗、同路由和同渲染设置下的 bypass/processed 插件链结果，输出电平、频段能量、动态与延迟的 compact transformation delta。FXM 是带条件的派生观察，不是音乐质量判断，也不把 raw render 写入 LLM context。

## 状态规则

- `ready`：目标 intent 所需证据完整、fresh，可用于普通观察；action preflight 还必须通过 trust gate。
- `suspect`：证据存在但质量门失败或来源不可信。可以简短说明限制，不能作为修改依据。
- `stale`：证据 revision/tap/source 身份落后。可以说明“需要刷新观察”，不能包装成 ready。
- `missing`：目标层缺失。回复应短说明缺失项，不反复输出 schema 或 raw JSON。
- `partial/deferred/building`：异步或非目标层未完成。非目标层不得把当前 intent 降成“未完整”。
- `approximate`：只能作为近似提示。当前 `loudness_summary` 的 RMS-derived LUFS 必须标记 approximate，不得冒充正式 integrated LUFS。

## Agent 行为边界

- 用户明确说“不要修改”“只观察”“不要动”等时，agent 只能走只读观察路径，允许 `mix.observe`、`mix.read`、`mix.derive` 和 read/list/project-state 类工具，不得生成 pending，不得加载插件或写参数。
- 用户要求“帮我处理/调整/收一点/降低/应用”等，即使包含“先告诉我依据”，也进入 action preflight：先观察并说明依据，然后生成待确认候选；确认前不得执行。
- 普通观察回复应是自然语言，优先给结论、证据、限制、建议。数据 ready 时不要反复说“未完整”；只有 missing/suspect/stale/approximate 时才简短说明限制。
- pending 只在 action preflight 或明确执行意图后出现。内部 `mix_treatment_pending` / schema marker 必须被隐藏，用户只看到自然语言候选和确认提示。
- 确认后必须走 typed route，并在 reobserve 后检查 AB result；AB 不可信时要明确标记，而不是假装完成可信对比。

## 手测预期

1. `观察当前工程的频段和声像状态，不要修改。`
   - 预期：只读观察；不产生 pending/confirmation；回复不含 raw JSON、schema 大串、revision 大串；ready 时直接给频段和声像结论。
2. `比较一下各轨频段占用和声像关系，不要修改。`
   - 预期：MOM intent 为 `project_multitrack_relation_observation`；使用 compact multitrack relation；不修改工程；missing/suspect track 只作为限制说明。
3. `帮我把低频稍微收一点，但先告诉我依据。`
   - 预期：进入 action preflight；先引用 MOM/DAD 证据说明低频依据；生成待确认候选；确认前不加载插件、不写参数。
4. 确认后检查 AB result。
   - 预期：执行 typed apply 后 reobserve；若 before/after L2 Render Probe 同 tap/mode 且 revision changed，则 AB result ready；否则回复中明确 stale/suspect/missing 限制。

## Smoke 入口

统一验收入口：

```powershell
D:\Vit_DAW\scripts\run_observation_v1_acceptance_smoke.ps1 -RepoRoot D:\Vit_DAW -GodotProjectRoot D:\Godot\project\vit-daw-frontend -GodotExe D:\Godot\Godot_v4.6.1-stable_win64_console.exe
```

该入口串联：

- Godot headless parse
- Go observation regression set
- DAD L3 package smoke
- L2 Render Probe smoke
- L2 realtime observation smoke
- MOM observation/action preflight/raw leakage tests
- TIM technical integrity projection tests
- AB result smoke
- Godot product-path lifecycle smoke

统一 summary 输出到：

`D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\observation_v1_acceptance_<timestamp>\summary.json`

## Backlog

- masking / reference / structure：补完整掩蔽、参考曲线、段落结构观察，不进入 Observation v1 封口范围。
- 完整 LUFS：接入正式 LUFS 分析前，当前 loudness 只能标 approximate。
- action workflow 重构：把 pending、confirmation、resolver、apply/reobserve 流程进一步类型化；不得破坏本 note 中的 observation contract。

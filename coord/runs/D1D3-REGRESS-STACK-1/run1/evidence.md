# run1 失败取证（acoustic materiality record is missing）

- run：`20261107_184225`（18:42:20 起栈→18:54:24 python FAIL→PS1 exit 1）；attempt0=启动腿 bash 反斜杠路径事故（PS1_EXIT=127，零时长，栈未启动亲证 procs/ports=0——run 未消耗，266 栈前自纠先例）。
- 失败点=**runner 主链 D1 验证尾**（free_state_d1_smoke.py:1770 `require(round_row.get("materiality") is not None)`）；ps1:664 throw。**D1-settle/D2/D3 三门组在脚本序中位于 runner 之后，未及执行**——274/292 回归门未验证（非断言失败，是未到达）。

## 响应链（d1_smoke_report.json responses）

| # | stop_reason | 摘要 |
|---|---|---|
| 0 | transient_llm_error | 首轮 LLM 服务瞬态（telemetry error 行=context deadline exceeded），checkpoint 保留 |
| 1 | （提案确认卡） | 「人声轨试一小步短衰减混响，走效果发送（Reverb Bus）」 |
| 2 | plugin_selection_required | Pro-R 推荐（63 混响检索/30 PCA 准入） |
| 3 | （装载确认卡） | Pro-R→新建 FX 轨 Reverb Bus，轨道 1032 send -6dB |
| 4 | semantic_dynamic_confirmation_required | 语义参数控制器就绪 |
| 5 | **capability_blocked** | 「为核实这次改动而请求的时域观察返回缺源数据、投影不可用…重复同一观察集不会带来新证据」 |

对照健康基线（265 reverify1 run2，2026-11-06）：`improvement_proposal_confirmation_required→plugin_selection_required→(确认)→semantic_dynamic_confirmation_required→(确认)→done`，round 携带完整 settle 尾（materiality+target_response+decision=user_judgment_pending+audition_session）。

## round 形状对照

- 本 run round keys：`checkpoint_ref/interventions/number/observations/phase/project_revision/requested_view_ids/round_id/started_at/status/updated_at`——**settle 尾键全缺**（materiality/target_response/decision/audition_session_id 均 null/缺席）。
- 1770 断言**之前**的断言全过：readback_verified=true、恰好 1 条 post-action 观察、post fresh+revision-bound——**应用/回读/post 观察链本身健康**（1-mutation 形态）。

## capability_boundary 载荷（resp5.workflow_data）

```json
{"code":"proposal_missing","class":"free_state","track_id":"1032",
 "reason":"自由态准入门未通过：G7_fresh_revision_bound_refs、G8_target_consistency",
 "blocked_by":["G7_fresh_revision_bound_refs","G8_target_consistency"],
 "unavailable_reasons":["certification_entry_unavailable_for_family","family_unresolved","no_loaded_instance_for_manual_guidance"]}
```

- agent log：18:49:51–18:53:02 六次 ccb.observation_request（每次 ~6s status=ok）=验证观察反复拿不到时域证据面；18:54:23 `stop=capability_blocked completed_steps=9 executed=6`；shadow `tracks=7`（工程态 7 轨，与 292 镜像一致）。
- 8787 VSP 注册拒绝=248 在案既有形态（本栈无 VspHub，非本腿变量）；l2_render_probe_ready telemetry spill 三次在档。

## 归因分列（按可能性排，均未定论——单 run 证据）

1. **LLM 概率腿分支**：自由态 settle 相位在「已应用改动的声学效果不可观察」判定上诚实停边界（AGENTS §8 的 capability_blocked 独立失败类）。支持：拓扑=FX 总线+send（新建 Reverb Bus 轨无已分析素材）+首轮瞬态错误扰动；对照 2026-10-14 FIX-FS-SETTLE-DEADCHAIN-1 在案先例（materiality 教学腿存在合法拒绝形态）。
2. **295 修前提示词面（基线不含 3ff051b0，merge-base 亲证）**：track.{id}.* 误导模板仍在场——验证观察若走了恒 miss 键族，与本失败形态相容；295 impl 已在 main（f6d1982e，本 run 基线 ae537693 之后合入），重派时自然消解该变量。
3. **274/292 回归**：无证据指向——失败面（free-state settle/materiality 链）与两修的消费面（d1d3 digest 读取/mix.read stereo 键路由）零交集；应用链断言（1770 之前）全过。
4. **294 fixture 卫生**：本 run 动作面=1032+新建 FX 轨（非 1007）；卫生只剔 (1007,1055) 遗留行；无直接交叠证据。

## 门组状态与计费

- D1 门 A/D2 门组/D3 门组：**未执行**（脚本序在 runner 后）；D2 门组断言修正（本卡 ps1 改动）在位待后续 run。
- 计费：30 调用 deepseek-flash，input 419,531 / output 61,162 / cache_read 212,604 tok，1 error（超时行）；估 ≈¥0.3-0.6（§8 预算 ≈¥0.1-1.5 内）。telemetry_run_window.jsonl 全量 30 行在档。

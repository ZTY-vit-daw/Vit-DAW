# 144 FIX-STACK-DRAFT-ROOT-ISOLATION-1 审计表：真栈/烟测脚本 draft 根隔离状态

- 审计时间：2026-10-18 晚窗；审计人：144 执行流（GLM ZCode）
- 范围：`scripts/*.ps1`（93 个 + 本次新增 helper 1 个）与 `coord/runs/**/*.ps1`（14 个）
- 判据：脚本是否在启动 VitAgent（直接 / kernel+agent berth / Godot 链）之前设置 `VIT_HISTORY_DRAFT_ROOT` 或等价隔离（`LOCALAPPDATA` 覆盖使默认 draft 根落到 runtime 目录，见 `agent/internal/history/history.go defaultDraftProjectRoot`：env → `os.UserCacheDir()`=Windows %LocalAppData% → TEMP）
- 状态值：**隔离**（修复前已合规）/ **等价隔离**（LOCALAPPDATA 覆盖）/ **未隔离→已修**（本次补）/ **委托**（agent 由 dev_agent_smoke 启动，中心修复覆盖）/ **kernel-only**（不起 agent，不写 draft）/ **N/A**（构建/静态/HTTP-only）

## 一、scripts/*.ps1

### A. 修复前已隔离（16）

| 脚本 | 证据 |
|---|---|
| b13b_frontier_masking_smoke.ps1 | :337-345 设 run 目录 `$AgentDrafts`，启动后恢复 |
| cont_stall_repro_smoke.ps1 | :251-259 同模式 |
| cont_stall_two_goal_smoke.ps1 | :254-262 同模式 |
| d1_audition_gap_smoke.ps1 | :207-215 同模式 |
| d1_stall_repro_smoke.ps1 | :166-174 同模式 |
| fs_largeproject_smoke.ps1 | :506 `Set-Item env:VIT_HISTORY_DRAFT_ROOT $AgentDraftsDir`（pull_env.json 留痕） |
| journey1_demo_journey_smoke.ps1 | :399 |
| m2_timedyn_fixverify.ps1 | :119 |
| reply_gen_toolgate_smoke.ps1 | :172-180 |
| run_b5_workspace_switch_smoke.ps1 | :179-195 |
| run_b6_mixtick_terminal_smoke.ps1 | :179-195 |
| run_chat_chain_e2e_smoke.ps1 | :239-242 |
| run_confirm_chat_1_natural_affirmative_smoke.ps1 | :256-264 |
| run_webui_rendered_dom_smoke.ps1 | :295-303 |
| settle_deliver_smoke.ps1 | :203-211、:457-465（两处 agent 启动） |
| dev_agent_smoke.ps1（仅 journey berth 路径） | :1003-1005 仅 `-Scenario journey_*` 场景设置 → **非旅程路径未隔离，本次中心修复**（见 B-1） |

### B. 未隔离→已修：直接启动 VitAgent 的 berth/脚本（11）

| 脚本 | 修复内容 |
|---|---|
| dev_agent_smoke.ps1 | **中心修复**：任何进程启动前默认设 env（场景模式=run 目录 `agent_drafts`；非场景=%TEMP% 每调用目录；外部 env 优先）；`Assert-DraftRootIsolation` 在主 agent 启动与 harness_ab 重启前断言；run 目录留 `draft_root_isolation.json` 痕 |
| restart_agent.ps1 | dot-source helper，%TEMP% 默认 |
| run_acoustic_source_identity_smoke.ps1 | 同上（共享 artifact 目录非每调用，取 %TEMP%） |
| run_b13c_reply_branch_smoke.ps1 | helper + `-RunDir $runDir` |
| run_compressor_change_delta_smoke.ps1 | helper，%TEMP% 默认 |
| run_compressor_dual_tap_smoke.ps1 | helper，%TEMP% 默认 |
| run_eq_direct_control_smoke.ps1 | helper，%TEMP% 默认 |
| run_eq_nl_agent_smoke.ps1 | helper，%TEMP% 默认 |
| run_eq_three_models_smoke.ps1 | helper，%TEMP% 默认 |
| run_free_state_open_intent_acceptance.ps1 | helper + `-RunDir $runDir` |
| run_vit_product_path_smoke.ps1 | helper + `-RunDir $ArtifactDir` |

### C. 未隔离→已修：Godot 链启动（agent 由 Godot 启动页拉起，继承脚本进程 env）（12）

| 脚本 | 修复内容 |
|---|---|
| run_ccb_observation_product_smoke.ps1 | helper + `-RunDir $runDir`（Godot Start-Process 前注入） |
| run_com_product_path_smoke.ps1 | 同上 |
| run_timefx_probe_smoke.ps1 | 同上 |
| run_static_eq_external_holdout_smoke.ps1 | 同上 |
| run_de_esser_control_validation.ps1 | 同上 |
| run_limiter_control_validation.ps1 | 同上 |
| run_transient_shaper_control_validation.ps1 | 同上 |
| run_eq_compat_matrix_smoke.ps1 | 同上 |
| run_waves_eq_topology_census.ps1 | 同上 |
| run_waves_static_eq_phase3_smoke.ps1 | 同上 |
| run_plugin_alliance_eq_two_level_smoke.ps1 | 同上 |
| run_gui_project_save_matrix_smoke.ps1 | helper，%TEMP% 默认（Godot `--script` 诊断腿；若探针未起 agent 则 env 无消费者，无害） |

### D. 等价隔离（LOCALAPPDATA 覆盖）→ 本次补显式 env（3）

| 脚本 | 原状态 | 修复内容 |
|---|---|---|
| h_g8_start_runtime.ps1 | `-Environment @{ LOCALAPPDATA = <runtime>\localappdata }` → 默认 draft 根落 runtime | helper（`-RunDir $RuntimeRoot`）+ 环境表补显式 `VIT_HISTORY_DRAFT_ROOT` |
| h_g8_start_ox_r29.ps1 | 同上 | 同上 |
| h_g8_start_grok_r30.ps1 | 进程级 `$env:LOCALAPPDATA` 覆盖（kernel+agent 全链） | helper（`-RunDir $root`）显式化 |

### E. 委托：agent 由 dev_agent_smoke 启动，中心修复覆盖（11，脚本本身不改）

materialize_shadow_smoke、pgtdc_compat_verify、run_b15_manual_save_smoke、run_blind_mix_conversation_smoke（kernel 自起+agent 委托）、run_free_state_d1_smoke（Godot 自起+委托）、run_live_material_observation_smoke（kernel 自起+委托）、run_mix_single_tick_e2e（kernel 自起+委托）、run_observation_v1_acceptance_smoke（产品路径腿委托 run_vit_product_path_smoke，后者已修）、seg_primitives_smoke、tim_assert_smoke、krev1_kernel_param_revision_probe（vsphub 自起；agent 委托）

### F. kernel-only berth：不起 agent，不写 draft 根（4）

run_kernel_dad_probe_smoke、run_project_audio_settings_preflight_smoke、run_project_stems_import_smoke、run_render_wedge_evidence

### G. N/A：构建/静态/HTTP-only（其余）

- 构建/安装：assemble_windows_release、build_agent、build_installer_v0.5/0.6/0.61/0.62/0.7/0.71/0.9/0.92/0.93、bundle_python_embed、download_vcredist、test_phase_d_changes（构建 test_agent.exe 后删除，从不运行）
- 静态/诊断（无栈启动）：baker_diag_matrix、sample_vit_import_perf、spectral_dynamics_evidence_audit、h_g8_prepare_r30、clear_acoustic_observation_state、codex_vsp_readonly_probe（只读 HTTP 探针，栈由外部启动）
- HTTP-only（假设外部栈已在跑，起栈方负责隔离）：f4_turn_started_timing_smoke、h1_vit_gate_timing_smoke、tim_assert_smoke（见 E）、seg_primitives_smoke（见 E）

## 二、coord/runs/**/*.ps1（14）

| 脚本 | 状态 |
|---|---|
| FE-START-EMPTY-DRAFT-1/scripts/probe_draft_root.ps1 | 隔离（显式 VIT_HISTORY_DRAFT_ROOT） |
| FE-START-EMPTY-DRAFT-1/scripts/probe_race.ps1 / probe_pristine_race.ps1 | N/A（对已起栈做 HTTP/ZMQ 探测 + 快照，栈由 dev_agent_smoke berth 启动；berth 路径已由中心修复覆盖） |
| FIX-START-DRAFT-SIDE-EFFECT-1/scripts/run_stack_leg_137.ps1 | 隔离（:299-313 run 目录 + 恢复模式） |
| FIX-TEST-DRAFT-ISOLATION-1/scripts/find_polluting_tests.ps1 / map_polluting_tests*.ps1 | N/A（140 卡的测试定位诊断；运行 `go test`，非真栈；140 已在 TestMain 层修复 chat/harness 测试隔离） |
| TRACK-LIST-GAP-1/run_tracklist_gap_probe.ps1（139 腿） | 隔离（事发后补：:299-313 run 目录 `agent_drafts` + 恢复） |
| FIX-MASTERVOL-CALIBRATION-1/run_master_volume_diag.ps1 | kernel-only berth（N/A） |
| KERNEL-RENDERGAIN-1/run_render_gain_diag.ps1 | kernel-only berth（N/A，脚本头注明 agent 面不参与实验） |
| L1-5-IMPL-C-REVIEW-1/diagnose.ps1 / verify.ps1 | N/A（go test/静态校验） |
| L1-5-IMPL-D/20261009_205504_p2_recompute/recompute_p2.ps1 | N/A（离线遥测复算） |
| OPT-IMPL-2/sample_replies.ps1 | 隔离（显式 env） |

## 三、grep 证据（修复后）

- `grep -L 'VIT_HISTORY_DRAFT_ROOT\|vit_stack_draft_isolation' <全部启动 VitAgent 的脚本>` = 空集（等价隔离的 h_g8 三脚本也已补显式 env）。
- 真实根历史残留（139 腿 4 个 draft，18:04-18:07）已由 Lead 移入 `%TEMP%\fedraft1\stack_leg_orphan_drafts\`（未删，决策侧处置）；真实根 10185→10181。

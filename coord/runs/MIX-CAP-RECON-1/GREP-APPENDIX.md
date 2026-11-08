# MIX-CAP-RECON-1 grep 附录

执行环境：Git Bash @ D:\Vit_DAW\agent（除注明外）。每条=命令+关键命中摘录。生成时间 2026-10-10 夜，基线 origin/main=1c1a447a。

## G1. 能力标记全仓分布（§1.1 文件清单来源）

```
$ grep -rn "c1_frequency_cleanup|c2_dynamic_batch|c2_dynamic_control|b4_eq|semantic_eq|static_balance|pan_layout|low_end_relation|strip_silence|clip_fade_gain|gain_staging" --include="*.go" -l
→ internal/tools/catalog_test.go, internal/tools/catalog.go, internal/tools/capability_pack.go,
  internal/staticbalance/types.go, internal/queryengine/route.go, internal/processorregistry/registry.go,
  internal/panlayout/types.go, internal/panlayout/model.go, internal/orchestrationstore_test.go,
  internal/orchestrationruntime/runtime.go, internal/orchestration/types_test.go, internal/orchestration/store_test.go,
  internal/orchestration/registry_test.go, internal/orchestration/owner_policy_test.go, internal/orchestration/owner_policy.go,
  internal/orchestration/context_envelope_test.go, internal/orchestration/builtins.go, internal/mixstyle/style.go,
  internal/mixboard/mixboard.go, internal/mixboard/decision_ledger_test.go …（chat 包大量命中，见 G3）
```

## G2. orchestration/builtins.go 注册面（全文 63 行，声明层六能力）

```
static_mix.static_balance.v0 / static_mix.pan_layout.v0 / static_mix.low_end_relation.v0 /
fine_mix.frequency_cleanup.v1 / fine_mix.dynamic_control.v1 / agent.effect.eq_control.v0
（CapabilityDefinition 六条；无 B5/focus_position、无 gain_staging、无 space_depth）
```

## G3. chat 包固定编排文件清单（ls internal/chat/ 摘录）

```
b4_eq_planner.go / b4_eq_runtime.go / c1_frequency_cleanup_planner.go / c1_frequency_cleanup_runtime.go /
c2_dynamic_batch.go / c2_dynamic_control_runtime.go / c2_dynamic_plugin_load.go / c2_project_treatment.go /
capability_owner.go / capability_routing.go / capability_runtime_canary.go / capability_cutover.go /
low_end_relation_runtime_canary.go / pan_layout_runtime_canary.go / semantic_eq_runtime.go /
mix_session_workflow.go / free_state_d1_*.go / free_state_experiment_runtime.go / goalrunner_chat.go / server.go …
```

## G4. c1/c2 命名与 LLM source 标（固定编排 planner 证据）

```
$ grep -rn '"c1_|c1_frequency|c2_dynamic' internal/chat/ | grep -v _test
→ c1_frequency_cleanup_planner.go:41  Source: "c1_project_treatment_planner"
  c1_frequency_cleanup_planner.go:108 Source: "c1_project_eq_planner"
  c2_project_treatment.go:114         Source: "c2_project_treatment_planner"
  c2_project_treatment.go:579         ccb.observation_request c2-discovery-*
  c2_dynamic_control_runtime.go:214   Source: "c2_project_target_planner"
  c2_dynamic_control_runtime.go:308   Source: "c2_project_candidate_planner"
$ grep -n "b4_project_eq_planner|b4_project_eq_instance_selection|b4_project_treatment_planner" internal/chat/b4_eq_planner.go
→ :180 / :43 / :124
```

## G5. A4 标记定义与断言（scripts/fs_largeproject_smoke.ps1）

```
:1219 $fixedOrchestrationLlmSources = @(   # 8 个 planner source 标
      "b4_project_eq_planner","b4_project_eq_instance_selection","b4_project_treatment_planner",
      "c1_project_eq_planner","c1_project_treatment_planner",
      "c2_project_candidate_planner","c2_project_target_planner","c2_project_treatment_planner")
:1266 $fixedMarkers = @(                    # 6 个文本标记（原始 5+review A note 1 增补第 6）
      "semantic_eq_batch","eq_plugin_load_batch","dynamic_plugin_load_batch",
      "dynamic_parameter_batch","c2_dynamic_plugin_selection","c2.dynamic_plugin_load.governed")
:1314 scope_note = "fastpath preflight (FastPathRouter) is harness plumbing and not in the marker set;
      tool catalogs / prompt advertisement not scanned (capability ads are not runtime activation); …"
:1324 $a4 = $fixedOrchestrationAudit.zero_hits
```

## G6. A4 六标记生产者（governed 命令常量锚点）

```
b4_eq_runtime.go:26            b4LoadBatchCommand = "b4.eq_plugin_load_batch.governed"
b4_eq_runtime.go:618           b4.semantic_eq_batch.governed（refschema.go:202-204 主/reconcile 词条）
c1_frequency_cleanup_runtime.go:27  c1LoadBatchCommand = "c1.eq_plugin_load_batch.governed"
c2_dynamic_batch.go:26         c2DynamicPluginLoadBatchCommand = "c2.dynamic_plugin_load_batch.governed"
c2_dynamic_batch.go:27,471     c2.dynamic_parameter_batch.governed
c2_dynamic_plugin_load.go:20   c2DynamicPluginLoadCommand = "c2.dynamic_plugin_load.governed"
c2_dynamic_control_runtime.go:625   Workflow: "c2_dynamic_plugin_selection"（交互卡）
```

## G7. FastPathRouter 注册清单（fastpath/router.go:79-90，全文）

```
"static_mix_capability_contract","project_blackboard_status","clip_fade_gain_set","clip_fade_gain_read",
"strip_silence_suggest","clip_range_split","stems_folder_import","pending_section_markers_apply",
"natural_mix_observation","static_mix_gain_staging_context_pack"
（挂点：agentloop/message_loop.go:444 一带；SetDiagnosticOnly=CCB 实验边界语义旁路）
```

## G8. 话术→capability 推断（capability_owner.go:206-247 摘录）

```
B2: "static_mix.static_balance","static_mix.focus_position","b2","b5","静态平衡","静态音量","推子平衡",
    "静态主次","建立主次","主次关系","核心元素定位","突出主唱","static balance","focus position"
B3: "static_mix.pan_layout","b3","声像布局","pan layout","panning layout"
B4: "static_mix.low_end_relation","b4","低频关系","low-end relation","低频分析","低频占用","低频遮蔽"
C1: "fine_mix.frequency_cleanup","c1","frequency cleanup","频段清理","频率清理","频谱清理","全工程eq清理"
C2: "fine_mix.dynamic_control","c2"   （仅显式标记——注释：不做选轨动态话术的词法后继）
```

## G9. 段位路由映射与显式阶段话术（capability_routing.go:584-624）

```
explicitCapabilityStagePattern = ([a-f][1-9])   # A1-F9 显式阶段
capabilityIDForStage: A2→project_prep.technical_integrity.v0 / A3→project_prep.track_organization.v0 /
                      C1→fine_mix.frequency_cleanup.v0 / C2→fine_mix.dynamic_control.v0 /
                      C3→fine_mix.space_depth.v0 / default→mixing_capability_stage.<letter>
（注意：C1/C2 此处映射 .v0，与注册面 .v1 版本号不一致——路由记录与 registry ID 的版本书写偏差，勘表面记录）
（C3 仅有映射无 runtime：canary seam 六分支无 C3，全仓 "space_depth" 仅此一处命中）
```

## G10. legacy mix session 门控（mix_session_workflow.go）

```
:464 func legacyMixSessionWorkflowEnabled(ctx) → env VIT_ENABLE_LEGACY_MIX_SESSION_WORKFLOW=1/true 或显式上下文旗标
:129 runMixSessionEntryChat（生产调用者=0，仅 mix_session_workflow_test.go 三处）
```

## G11. B1/B2/B3 goalrunner 话术面（goalrunner_chat.go 摘录）

```
:3952 hasB1: "b1","b 1","static_mix.gain_staging","gain staging","gainstage" + 增益/电平健康词族
:3967 B2: "b2","static_mix.static_balance","static balance","静态平衡" + 全工程×电平/推子平衡
:3977 B3: "b3","static_mix.pan_layout","pan layout","声像布局","声场布局","静态声像"
:4256 agentLoopStaticMixGainStagingTools = project.state, mix.observe/read/derive, clip.gain.set(set_batch),
      track.group.apply_control, project.undo/redo …
:4266 agentLoopStaticMixStaticBalanceTools = …, mix.propose_tick, mix.apply_tick, mix.rollback_tick …
```

## G12. 自动化曲线面反向搜索（§2.C 证据）

```
$ grep -rn "automation" VitApp/Source/Service/CommandDispatcher.cpp        → 4 处注释/旗标，零命令
$ grep -rn "AutomationCurve" tracktion_engine/modules/.../automation/      → te::AutomationCurve.h 等原生族在位（未被 VitApp 调用）
$ grep -n "AutomationLaneCount" agent/internal/chat/capability_routing.go → :36 声明, :496 读取, :545 因子(阈值 24)
（上游 createTrackState :841-870 不产出 automation_* 键 → 实践恒 0）
```

## G13. 三工程登记（§4 证据）

```
docs/mix_benchmark_manifest_v0.md:16-24   sattelites 61轨/3.74GB/48k24b/228.571s（E:\BaiduNetdiskDownload\…）
docs/mix_benchmark_manifest_v0.md:30-37   Weekend Lover 104轨/7.22GB/258.641s（封存集）
docs/AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md:206  用户方向裁定①②③（A–F 纪律序/F=固定编排淘汰/三工程基材+验收口径）
scripts/fs_largeproject_smoke.ps1:6,88,276  TrainingFolder=sattelites；隔离工程名 sattelites_61.vit
coord/runs/FS-LARGEPROJECT-SMOKE-1/fs_largeproject_smoke_20261010_203348/run_report.json
  → import.tracks_created=61/clips_created=61/readable_file_count=61；audition_events=0（A3 not_driven_no_card）
```

## G14. 官方 A-F 定义（§3 权威锚）

```
docs/agent_action_workflow_v1_master_plan.md:217 §6.1 Mix Workflow Queue v0
  #### A. project_prep A1-A5（:23-30 一带，详 project_prep_capability_layer_v0.md）
  #### B. static_mix B1-B4 并列（旧 B5 退役）
  #### C. C1 频段清理/C2 动态控制/C3 空间深度/C4 段落自动化/C5 总线处理
  #### D. D1 响度峰值/D2 翻译/D3 参考曲/D4 版本输出
  #### E. E1 母带 EQ/E2 母带动态响度/E3 母带空间/E4 最终导出质检
  #### F. F1 混音审查
coord/decisions/2026-10-04-capability-layer-harness-ruling.md  裁定 1-3（层保留编排退位/族粒度保留+D,E 后置/A-F=M 线产品视图，真开发项=M2 时域+M3 自动化）
coord/cards/todo/2026-10-04-FREESTATE-REGION-IMPL-1.md:3  推迟裁定+2026-10-10 用户勘误（原 B2 系误记，自动化=C4）
```

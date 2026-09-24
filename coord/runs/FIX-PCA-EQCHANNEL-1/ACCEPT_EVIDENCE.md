# FIX-PCA-EQCHANNEL-1 验收证据与上交材料（PC 执行侧，2026-09-24）

run：FIX-PCA-AUTOSWEEP-1/20260924_005622 续跑（EQ 通道相，state.json 幂等续跑）
代码：scripts/pca_autosweep.py（本卡唯一文件域改动，+~700 行）

## ① EQ 通道收据抽核（全 7 份，超额于 ≥3；零 LLM 全检）

收据根：`C:\Users\timoz\.vit\pca_certifications\eq_channel_20260924_005622\<slug>\summary.json`

| subject | kind | chat | audio_probe | b4 | restored | drift | extra coverage |
|---|---|---|---|---|---|---|---|
| API-550A Stereo（Waves 壳） | waves.static_eq.phase3_live_smoke.v1 | 0 | 0 | 0 | ✓ | [] | modify+undo |
| AudioTrack Mono/Stereo（Waves 壳） | waves.* | 0 | 0 | 0 | ✓ | [] | modify/disable+undo |
| L316 Mono/Stereo（Waves 壳） | waves.* | 0 | 0 | 0 | ✓ | [] | modify/disable+undo |
| Lindell 80 Channel（PA） | plugin_alliance.eq_two_level_live_smoke.v1 | 0 | 0 | 0 | ✓ | [] | modify/disable+undo |
| bx_hybrid V2（PA，legacy 补认证） | plugin_alliance.* | 0 | 0 | 0 | ✓ | [] | modify/disable+undo |

工具面全检：每份收据 agent_tools 仅 {plugin_search, track.add_audio, plugin.load_to_rack, plugin.get_parameters, plugin_grabber.explain_controls, plugin_grabber.apply_eq_edits}（+track.delete 不入收据审计），与 EQ-1 phase3 允许清单同面；natural_language_chat_count=0。
import 合并例证：API-550A Stereo 晋升条目 evidence 同时携带 2026-07-27 legacy 收据与 2026-09-24 新收据，coverage={upsert,modify,undo}×bell。

## ② 白名单 EQ 多候选 + 溯源

- live 白名单 static_eq：1（bx_hybrid S0）→ **6**（+API-550A Stereo/AudioTrack Mono/AudioTrack Stereo/L316 Mono/L316 Stereo），--apply-live 双备份（live_backup_pre_eq.json / live_backup_pre_apply.json）
- 溯源：provenance_table_v6_full.json static_eq 13 行——每条目 bands 行（explain 拓扑 × S3 双周期探测零漂移 stable_id 交叉核对）+ _attestation 行（S2 store + 收据路径）+ _excluded（Lindell 80 Channel：带面无 center/gain 锚，如实不入列）+ _s0_kept（bx_hybrid V2 保留 S0 条目，新收据仅并 store coverage）
- 派生工件：derive/eq_static_eq_derived.json（5 derived + 1 excluded 记因）

## ③ overlay 回归

verify 相：61 检 0 败（含 6 条 static_eq 准入查询 upsert+bell、55 条其它族、3 条 non-member spot 拒绝）。

## ⑤ 例外队列 static_eq 处置（68 命中全分类）

| 处置 | 数量 | 原因 |
|---|---|---|
| certified this sweep | 7 | 收据+import 晋升 |
| load_gate_blocked | 57 | pca_load_gate: 56× no_promoted_current_pca_admission_for_identifier + 1× 跨族歧义 fail-closed（见上交材料） |
| vendor_kind_missing | 3 | FabFilter×2 / Tokyo Dawn Labs×1——v1 导入门只收 waves/PA 两种收据种类（扩展=agent 代码，越域） |
| shape_not_provably_reachable | 1 | explain 拓扑存在但 upsert bell 不可证（诚实失败，主体见 sweep_report.json） |

（certified 7 + 61 例外行 = 68 ✓；上轮 68 条"no runner"滞留已清零，全部转为具体处置）

## 【上交决策侧】load_gate 对未晋升 static_eq 的结构性阻断（57 主体）

**事实**：EQ 认证通道需要对未晋升主体做 identifier 装载；当前 agent 的 pca_load_gate 对 HTTP 工具面调用有三条合法放行路，全部对该场景关闭：

1. certification token 路：token 仅由 `/agent/processor-certification/start` 铸造，而该端点对 static_eq 显式拒绝（`agent/internal/chat/processor_certification_entry.go:419` inspect_only 分支；candidates 面同样标注 `generic_static_eq_certification_runner_unavailable`）。
2. full-access 路：`authorizeFullProjectAccessLoad`（`agent/internal/harness/harness.go:1999`）要求主体已在晋升目录——先有鸡还是先有蛋（认证的目的正是晋升）。
3. 无 source 操作员路：`handleInvoke` 对所有 HTTP invoke 强制 `source="http"`（`server.go:1463`），gate 的 sourceless 豁免在 HTTP 面不存在。

**关键缺口**：harness 的 certification 授权白名单**已显式包含 static_eq**（`harness.go:1956`：`!IsV2Family && != broadband_compressor && != FamilyStaticEQ` 才拒绝）——基础设施已预留 EQ 认证装载，只是没有入口为它铸授权。

**本卡的部分绕行（已执行，如实记录）**：对**任意族已晋升**的 68 命中主体（7 例），经 full-access 路合法装载并完成 EQ 认证；未晋升 57 例如实滞留 load_gate_blocked。

**可选修法（需决策侧授权 agent 文件域）**：processor_certification_entry.go 增设 static_eq 的 token-only 认证授权（consent + 指纹 + 单次消费机制全部现成，harness 白名单已支持），约 40-60 行 + 测试；或产品化 EQ runner 进 agent。修法落地后 `--phase certify --families static_eq` 幂等续跑即可自动吸收 57 例（retry 语义已就位）。

## ④ journey EQ 多候选自选腿（五环 exit 0）

**run4（journey4/，默认演示提示词"帮助贝斯轨道的低频问题，给我方案"）＝ JOURNEY1_VERDICT all_green，exit 0，9/9 断言绿**（a1 不问方向 / a2 装载路径 / a3 实验链 / a4 会话洁净）。五环证据链：

| 环 | 证据 |
|---|---|
| ① 披露 | static_eq 白名单 6 候选>1 → free_state_plugin_candidate_disclosure.v1 多候选披露面激活（结构性字段）；模型假设文本以 EQ 处理成帧（"约 200 Hz 处 1.5 dB 窄带衰减"） |
| ② 模型自选 | agent_runtime_state_snapshot：`improvement_proposal.parameter_bounds.plugin_identifier = VST3-bx_hybrid V2-d0ef306f-c141eb4b`（模型决策显式 pin；6 候选族空 pin 会被拒，此 pin=真实自选） |
| ③ membership/准入 | admission.typed_action 晋升，`a2_pca_load_gate_denials=0`，零 membership 拒绝签名 |
| ④ 绑定 | d1 static_eq 白名单绑定解析 bx_hybrid 条目 → 假设 200Hz 最近带=315Hz（表 63/315/3150/6300/11200），ch1=827092295/ch2=843869511（与白名单条目逐字一致） |
| ⑤ 写链 | trajectory.intervention.applied：plugin_instantiated_by_action=true（plugin_id 1040）、normalized_batch_v1 双通道写 -1.5dB、actual_readback=-1.5 readback_verified=true、revision 3→4、audition.prepare.started→candidate.ready→ready×2、card_mounted=true、mix_tick.pending |

**run1-3 如实记录（§8 概率运行纪律，4 轮全部留工件 journey1/-4/）**：
- run1（EQ 链路顺序提示词）：assertions_red——模型走直接 set_parameter 路线（a1 绿），a2 探针 L316 Stereo 被防歧义闸正确拒绝（双族晋升 fail-closed，harness 设计行为）。
- run2/run3（强化链路/具体动作提示词）：assertions_red **同签名两连**——模型停到 `mix_treatment.pending` 待确认，确认后 resolver 判 `observation_only`。取证结论：`resolveMixTreatment` 的 `plugin_treatment` 分支**设计性退役**（mix_treatment_confirmation.go:279 "Stored mapping treatment execution is retired"），确认路径对 EQ 插件处置是死路；可执行路径=goal 内自主 trajectory 链。
- 据此 run4 改用脚本默认演示提示词（与前三次配置不同形，非 §8 同形重跑）→ all_green。
- a2 直探针 run2-4 均绿：run2/4=API-550A Stereo（本卡新认证 EQ 候选）rack 实例化成功、gate 零拒绝。

## 交付物清单（本卡工件根 coord/runs/FIX-PCA-EQCHANNEL-1/）

- 代码：scripts/pca_autosweep.py（EQ 通道 + EQ 白名单派生 + --apply-live/--families/--eq-subject + ensure_kernel_plugin_list 忙/空分流修正）
- 认证收据（机器本地）：~/.vit/pca_certifications/eq_channel_20260924_005622/（7 份 summary+explain+before/after+apply/undo 全证据链）
- run 账目/报告（入库）：FIX-PCA-AUTOSWEEP-1/20260924_005622/{run_ledger.jsonl,sweep_report.*,derive/*}（EQ 段已并入）
- journey 报告入库副本：journey_report_journey1-4.json + 控制台日志；journey1-4/ 工作区（含二进制/工程副本）留机器本地（惯例）


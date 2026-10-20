# PCA-REVERB-PROMOTION-1 战役结果（卡 150，池序 150）

- 执行：GLM-5.3 L2 PC ZCode 执行会话（port=pca-reverb-promotion-1）；worktree `D:/Vit_DAW_wt_reverb1`（基线 origin/main c08de366 领取后）
- 被测：worktree 构建 VitAgent（=origin/main 代码，零生产代码改动）+ 主检出 staging VitApp.exe（只读复用）；不启 UI/Godot（PLUGGRABBER 先例边界：agent HTTP + 内核 ZMQ 面）
- 终版 run：`pca_reverb_promotion_20261020_191316`（2026-10-20 19:13-19:2x，一 run 一次过零重跑）
- §8 记账：**LLM=0**（全确定性认证作业流+驱动脚本）；失败分类=2 件预期拒绝（spot-reject，矩阵原因行的认证侧确认）；止损线未触发（29/29 completed，无同签名连续失败）

## 1. 战役通道（复用既有认证作业流，零新机制）

每件走 agent HTTP 既有入口 `POST /agent/processor-certification/start {identifier, family:"reverb", confirmed, consent:"temporary_track_apply_readback_restore"}`：
- 三门装载（processor_certification_entry.go:102-136）：一次性 `pca_load_*` token（单次消费+10min 过期+source=pcactl.certify_processor+tool=plugin.load_to_rack+confirmed scope 校验）+ 精确装载身份（track_id+plugin_path+identifier 全等）+ 指纹一致（装载前 FingerprintPath 重算对比）；
- CertifyProcessor（local_processor_certification.go）：临时轨→三门装载→双 inspect（topology generation 稳定性）→`selectV2ReversibleControls` 选 ≤3 个不同轴可逆控制（enum/toggle 走 enum_label 写）→typed apply/readback/restore→全参数快照核对→临时轨清理；
- auto import/promote（receipts_v2.go ImportReceiptsV2→store.PromoteCurrent("deterministic_strong_receipt_passed")），coverage=V2CoverageForRoles(family, selected_roles) **按实生成，机制上不可能虚报轴**。

驱动=新增 `scripts/pca_reverb_promotion_smoke.py` + `dev_agent_smoke.ps1 -PcaReverbPromotionSmoke` 腿（ps1 腿+py 断言体的既有模式；脚本扩展在卡面允许域，非生产代码）。

## 2. 结果

**29/29 reverb 可证件全部 completed+promoted**（store revision **79→108**，reverb promoted 1→30）：

| 族 | 件数 | selected roles（实测） | typed coverage（按实申报） |
|---|---|---|---|
| Abbey Road Plates（Mono/Mono-Stereo/Stereo） | 3 | reverb_mode, output_gain, wet | output_normalization, spatial_mode, wet_balance |
| Abbey Road Chambers ×3 | 3 | reverb_mode, mix, output_gain | 同上 |
| H-Reverb ×9 + H-Reverb long ×9 | 18 | damping, mix, output_gain | 同上 |
| RVerb ×2 | 2 | damping, mix, size | decay_time(size), spatial_mode(damping), wet_balance |
| TrueVerb ×3 | 3 | mix, decay, pre_delay | decay_time, pre_delay, wet_balance |
| Pro-R（既有，rev79 前） | 1 | — | decay_time, output_normalization, wet_balance |

**decay 轴申报纪律（卡面要点 2/3 的如实落地）**：
- Plates 的衰减轴=Damper（plate_damper_family 0.90）——本战役 **未对 Plates 申报 decay_time**：确定性选择器（M2-TIMEFX-1 冻结的三轴界）按 inspect 拓扑遍历序取前三个不同轴，Plates 落在 reverb_mode/output_gain/wet，Damper 未被选中。不虚报=如实未申报；Damper 受控写证据仍在兼容矩阵（pgtdc run3 abbey_road_plates_spot.json）为探针侧证据。
- RVerb 的 decay_time 经 **size** 角色申报、TrueVerb 经 **decay** 直名申报——均为实际写入回读的角色，无一冒充。
- 若主管裁定 Plates 需补 decay_time 轴覆盖：需改 selectV2ReversibleControls 的轴偏好（生产代码改动，超本卡文件域）——本卡按停止条件未越域，如实上交此边界。

**指纹绑定**：29 件 Waves 壳件全部绑定 `WaveShell1-VST3 17.1_x64.vst3` 的 sha256（同一路径指纹）；Pro-R 绑定 FabFilter Pro-R.vst3。壳升级→全量 stale 是已知语义（卡面要点 2），store 内 Pro-R 已有一条 stale 先例同构。

**不可证件原因清单（19 行在卷）**：`non_certifiable_reason_rows.json`（源自 PLUGGRABBER 矩阵 2026-10-16）：Clarity Vx DeReverb ×2 / DeReverb Pro ×2（AI 去混响工具）、IR-L ×4 / IR1 ×4 / IRLive ×3（卷积衰减固化于 IR 采样，参数面无衰减控制）、MannyM Reverb ×3（无衰减时间控制）、ValhallaSupermassive ×1（reverb 面无衰减词）。另跑 **2 件 spot-reject 确认认证侧诚实拒绝**：MannyM Reverb Stereo→`unsupported_broadband_compressor`、IR1 full Stereo→`unresolved_reverb_surface`（job 失败即预期证据，不计入战役失败）。

## 3. 终态验收（真栈腿，脚本 exit 0）

`dev_agent_smoke.ps1 -StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -PcaReverbPromotionSmoke -KernelExe <主检出 staging>` → **exit 0（"dev agent smoke completed"）**，三断言门全绿：

- **G1 candidates 面**：GET `/agent/processor-certification/candidates?family=reverb` promoted+指纹匹配 **30 件**（含 Pro-R + 新晋 29）——即推荐准入面（filterPCAAdmittedPluginRecommendationCandidatesV2→QueryLibraryAdmissionV2 同一 store 同一判定）；
- **G2 推荐候选源**：harness `plugin.semantic_search`（query=reverb/type=reverb/limit=200，plugin_recommendation.go localPluginRecommendationCandidates 的生产调用形态）63 entries，**全部 29 件新晋件在场**；
- **G3 零静默丢件**：31 条记录（29 认证+2 spot-reject）每条终态 completed+promoted 或带原因。

**端测边界声明**：本腿覆盖推荐候选面（族过滤+准入+候选源，即「加混响」语义搜索的候选集来源）；NL「加混响」→processor_type=reverb 的 LLM 路由不在本腿（LLM=0 纪律，路由层属 146 实验域）。语义候选集从 1 件（Pro-R）扩至 30 件后，路由实验（146）与装载链（147）的候选丰富度前提已就位。

## 4. 零改动基线证明

- `go build ./...` exit 0；`go test ./...` **96 包全绿 0 FAIL**（worktree=origin/main+脚本扩展，无生产代码 diff）
- 仓库改动仅：`scripts/pca_reverb_promotion_smoke.py`（新增驱动）、`scripts/dev_agent_smoke.ps1`（+1 腿）、本目录工件

## 5. 工件

- 本目录：`campaign_summary.json`（含 31 条逐件记录：job_id/selected_roles/attestation_id/秒数）、`campaign_records.json`、`ledger.jsonl`、`store_before.json`（rev79）/`store_after.json`（rev108）、`non_certifiable_reason_rows.json`（19 行）、`leg_console.txt`（腿控制台全文）
- 机内认证工件：`~/.vit/pca_certifications/pca_job_*`（本 run 新增 31 个，每件 summary.json+cases/01_*/evidence.json 全链证据：三门装载响应/双 inspect/typed apply+readback+restore/全参数快照对比/临时轨清理）
- 腿烟测工件：worktree `VitApp/Workspace/Artifacts/smoke/pca_reverb_promotion_20261020_191316/`
- 栈占用/释放：`coord/resources/PC-RUNTIME-STACK.md`（19:1x 登记 80a1bd81 / 19:2x 释放复核进程=0、栈端口监听=0；本腿自起 pid VitApp 11456/VitAgent 23136 脚本 best-effort 拆栈残留，已按 145 卡先例手动清杀本腿 pid）

## 6. 偏差如实记录

- 起栈后本腿零重跑一次过；无环境中断。
- 领取过程主检出到期 push 拒绝两次（主管会话并发推进），按协议 §2.1.3 改道 worktree cherry-pick 重放领取提交（c08de366）；主检出留有同内容未推本地副本 57bd3c56（内容一致，主管会话 pull 时会自动去重）。

# PCA-REVERB-PROMOTION-1：时域混响族 PCA 准入晋升战役（Plates 等已验证件入推荐候选面）

- 发卡：GLM 主管决策流 / 2026-10-20 晚窗（依据=M2 体验场次①实录 F7+用户裁定 2026-10-20「肯定要开」：[2026-10-20-m2-experience-session-1.md](../../reports/2026-10-20-m2-experience-session-1.md)）
- 派发确认：已确认（用户 2026-10-20 晚窗原话裁定）
- 验收负责人：GLM 主管决策流
- 池序 150；目标仓库=D:\Vit_DAW；P2 / 战役 0.5-1 天 / 依赖=无硬依赖；**理想时序=在 146 软引导实验跑前落地**（单候选测不出路由偏好，候选丰富度是实验有效性前提）
- 模型分级：L2 / GLM 执行会话（PCA 证据纪律+真栈）

## 缺口（实录 F7 实证）

- 推荐候选面只看 PCA 准入库（`~/.vit/processor_control_attestations.v2.json`，`plugin_recommendation_attestation.go` filterPCAAdmitted…V2 → QueryLibraryAdmissionV2）；reverb 族**仅 Pro-R 1 件 promoted**（rev78/79，2026-10-12 起）。
- 探针兼容面已有 **42 件可证时域插件**（[PLUGGRABBER-TIMEDYN-COMPAT-1 兼容矩阵](../../runs/PLUGGRABBER-TIMEDYN-COMPAT-1/README.md)，2026-10-16 验收 pass）：含 Abbey Road Plates（Waves 板式混响，inspect 放行 conf 0.9/plate_damper_family+Damper enum 受控写 0.2→0.0 exact+readback 在卷）。**探针验证≠准入晋升**——用户点名记忆的「Waves 板式混响」即此断层。
- 既有晋升通道在卷：chat 认证入口（`internal/chat/processor_certification_entry.go`：一次性授权 token+精确装载身份+指纹一致三门）+ 认证作业工件（`~/.vit/pca_certifications/` 210 个 pca_job_*）+ 自动扫工具（`scripts/pca_autosweep.py`，`~/.vit/autosweep`）——限压器 23 件等多族批量即出此道。

## 目标

把兼容矩阵中 reverb 族可证件（**首件=Abbey Road Plates Stereo**，其余按矩阵可证清单）经既有认证作业流晋升为 v2 promoted 准入：每件产出 control case evidence receipts（typed coverage 按实际可证轴申报——reverb 族轴表 `processorattestation/v2.go:87`：decay_time/output_normalization/pre_delay/spatial_mode/wet_balance；Plates 的衰减轴=Damper（plate_damper_family 0.90），**不得虚报轴覆盖**）+ 指纹绑定 + store 晋升；最终推荐面「加混响」语义搜索返回 **≥2 个 reverb 候选**（含 Pro-R）。

## 执行要点

1. 先取证再跑：读 Pro-R/Timeless 3 的 pca_job 工件与 autosweep 用法，复用既有流程，不另造认证机制；
2. Waves 壳现实：WaveShell1-VST3 四版本在场、按路径指纹绑定（Pro-R 已有一条 stale 先例=二进制更新致指纹失效）；晋升后如实登记 stale 生命周期（壳升级需重认证为已知语义）；
3. 不可证件如实留原因行（矩阵 19 原因行先例：IR 系衰减固化于采样/MannyM Reverb 无衰减控制等）——不降证据标准凑数；
4. 战役确定性执行（LLM=0，PLUGGRABBER 先例），真栈占用登记 `coord/resources/PC-RUNTIME-STACK.md`，与 146 实验腿/147 烟测腿串行错峰。

## 文件域

预期零生产代码改动（纯认证战役+工件）；若认证流对 reverb 族有缺口（如 typed coverage 模板不认 plate_damper 证据形态）→ 停在缺口锚点上交，另拆实现卡。脚本扩展（如 autosweep 加族参数）允许，按现有 ps1/py 体系。

## 验收标准

v2 store reverb 族 ≥2 件 promoted（每件 attestation_id+evidence receipts+指纹可回指 job 工件）；推荐面回归：语义搜索/推荐候选含新晋件（单测或真栈腿二选一，真栈腿须 exit 0）；战役工件（run 记录+store 前后 revision）入 `coord/runs/PCA-REVERB-PROMOTION-1/`；不可证件原因清单在卷。`go build ./...`+全量测试 0 FAIL（若零代码改动则跑基线证明零改动）。

## 停止条件

- 认证流要求代码级改动才能接 reverb/板式证据形态 → 缺口锚点上交拆卡，本卡不越域改代码；
- Waves 壳枚举/许可在认证环境下不稳定 → 按环境中断记录，不归因插件能力；
- 指纹验证连续失败（同一签名×2）→ 停，上交壳版本诊断。

<<<<<<<< HEAD:coord/cards/doing/2026-10-20-PCA-REVERB-PROMOTION-1.md
- 领取：2026-10-20 19:00 +0800 / origin/main=0a4488be / owner=GLM-5.3 L2 PC ZCode 执行会话（port=pca-reverb-promotion-1）/ 分支 port/pca-reverb-promotion-1 / worktree D:/Vit_DAW_wt_reverb1 / 领取提交见下（推送后回填）
- 回执：（store revision 前后 / 晋升清单+attestation_id / run 记录与工件路径 / 端测边界声明）
========
- 领取：2026-10-20 19:00 +0800 / origin/main=0a4488be / owner=GLM-5.3 L2 PC ZCode 执行会话（port=pca-reverb-promotion-1）/ 分支 port/pca-reverb-promotion-1 / worktree D:/Vit_DAW_wt_reverb1 / 领取提交 c08de366（worktree 内 cherry-pick 重放后推送；主检出另有未推本地副本 57bd3c56 同内容）
- 回执：store revision **79→108**（reverb promoted 1→30，29 件新晋）。晋升清单=兼容矩阵 reverb 可证件全量 29：Plates×3/Chambers×3/H-Reverb×9/H-Reverb long×9/RVerb×2/TrueVerb×3；首件 **Abbey Road Plates Stereo=pca2_b5a48ef89aa051e0fa6f2184**（coverage=output_normalization/spatial_mode/wet_balance，2 evidence receipts 可回指 job 工件）；全量 attestation_id 见 campaign_summary.json。真栈腿=dev_agent_smoke `-PcaReverbPromotionSmoke` **exit 0**（三门：G1 candidates 面 promoted+指纹 30 件含 Pro-R/G2 plugin.semantic_search 候选源 63 entries 含全部新晋/G3 零静默丢件）；19 行不可证件原因清单+2 件 spot-reject（MannyM=unsupported_broadband_compressor/IR1=unresolved_reverb_surface 认证侧诚实拒绝）在卷。工件=coord/runs/PCA-REVERB-PROMOTION-1/pca_reverb_promotion_20261020_191316/（RESULTS.md）；机内 ~/.vit/pca_certifications/pca_job_*×31。栈占用 80a1bd81 登记/19:2x 释放复核（本腿 pid 残留按 145 先例手动清杀，进程=0 端口=0）。**端测边界**：①真栈腿覆盖推荐候选面（准入过滤+候选源，即「加混响」语义搜索的候选集来源，1→30 件）；NL 意图→reverb 的 LLM 路由不在腿（LLM=0 纪律，146 域）。②typed coverage 全部按实申报——**Plates 未申报 decay_time**（M2-TIMEFX-1 冻结的三轴选择器按遍历序取 reverb_mode/output_gain/wet，Damper 未选中；不虚报；补 decay 轴需改 selectV2ReversibleControls=生产代码改动，按停止条件未越域，交主管裁定是否拆卡）；RVerb decay_time 经 size、TrueVerb 经 decay 直名，无冒充。③零生产代码改动：仓库 diff 仅 scripts/pca_reverb_promotion_smoke.py（新增）+dev_agent_smoke.ps1（+1 腿，ps1+py 既有模式，卡面允许域）；go build exit 0+全量 96 包 0 FAIL。④Waves 壳 29 件同绑 WaveShell1-VST3 17.1 指纹，壳升级→stale 生命周期为已知语义。实现 commit=port/pca-reverb-promotion-1@a5d1933d 待 cherry-pick；LLM=0 一次过零重跑。
>>>>>>>> origin/main:coord/cards/done/2026-10-20-PCA-REVERB-PROMOTION-1.md
- 验收：（裁定文件 / 验收 commit）

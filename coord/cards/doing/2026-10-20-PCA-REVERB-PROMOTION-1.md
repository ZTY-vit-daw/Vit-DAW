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

- 领取：2026-10-20 19:00 +0800 / origin/main=0a4488be / owner=GLM-5.3 L2 PC ZCode 执行会话（port=pca-reverb-promotion-1）/ 分支 port/pca-reverb-promotion-1 / worktree D:/Vit_DAW_wt_reverb1 / 领取提交见下（推送后回填）
- 回执：（store revision 前后 / 晋升清单+attestation_id / run 记录与工件路径 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

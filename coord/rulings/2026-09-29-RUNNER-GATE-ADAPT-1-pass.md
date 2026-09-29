# Ruling：RUNNER-GATE-ADAPT-1 pass（2026-09-29，决策侧）

- **裁定：pass**。实现合入 main（cherry-pick 2932cf57）；PILOT p01+p07 真栈 exit 0 采信——**PAPER-EXP-ADAPT-1 目标 4 闭环，PILOT 前置就绪达成**。
- **决策侧核验（非转述）**：
  - **diff 审**（scripts/free_state_d1_smoke.py 114 行）：门作用域化实现合规——manifest 顶层 `qualification_profile`；未声明=默认 spv1_synthetic 原常量零变化（向后兼容）；未知名 ValueError fail-closed（AGENTS §11 unknown-enum 策略注释明确）；pv1_real_stems 参数与推导注释一致（per-track RMS −100/crest 6、**best crest 14 不变**、sibilance/transient per-track 撤销+best 级保留 6/14）；拒绝信息带 profile 名可审计；flavor 子门（pan/limiter/gate/multiband）保持 spv1 校准不静默重校准（pv1 flavored 留正式轮设计——边界诚实）。
  - **验证矩阵核读**：pv1 7/7 PASS+spv1 回归 8/8 PASS（p02 缺本地工程=既有 temp fixture 状态已记录）+反例 3/3 REJECTED（数字静音/重度削波/平稳噪声各被对应门拒）+未知 profile fail-closed+**pv1 在默认 spv1 口径下仍被拒**（放行来自 manifest 声明而非全局放宽——门的纪律性证明）。
  - **决策侧复算**（我复跑，worktree 分支版脚本+材料仓清单）：pv1_p01@pv1 passed（best_crest 25.355 与工件一致）/pv1_p01@默认口径 REJECTED/未知 profile ValueError/pv1_p05@默认口径 passed（交叉验证 ADAPT-1"仅 p05 过旧门"结论，两卡互证）/pv1_p07@pv1 passed——五格全符。
  - **PILOT 工件**：p01 exit 0（track_gain，mutation rev 3→4，读回 −1.0dB，validation=pass；轮 1 NOT_EXERCISED 模型分支分类记录）；p07 exit 0（static_eq VST3 550A，mutation rev 9→10，读回 −3，validation=pass；轮 1 capability_blocked——static_eq 已知方差面 2026-09-28 已有 fail 先例，非本卡引入，归因合规）。重试协议 §8 预声明（≤4 轮）合规；泊位声明规范（起栈核查+二进制双哈希记录+收栈三进程停三端口空+authority 全程 manual_confirmation）。
  - **材料仓 d7b686b**：仅 fixture_manifest.json +3−1（profile 声明），stems/projects 零改动——不修素材纪律遵守。
- **纪律边界核验**：六字段判据/盲法契约/配对设计零改动（diff 仅门作用域）；tag experiment-baseline-2026-09-29 未动（VitApp.exe 主树构建哈希复核后复制=tag 等价）。
- **注记**：正式轮（EXP-VIT-MAIN）脚本化序列已由 ADAPT-1 目标 3 备案（物化副本→project.open→同身份切 full→--reuse-existing-project 直跑 py）；pv1 flavored 子门口径为正式轮设计输入（非本卡范围）。

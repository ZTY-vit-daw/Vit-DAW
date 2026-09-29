# RUNNER-GATE-ADAPT-1：runner 材料合格门 fixture-set 作用域化+pv1 profile 重校准（PILOT 解锁前置）

- 池序 2（P0，解锁 PILOT）；来源=PAPER-EXP-ADAPT-1 blocked 处置（rulings/2026-09-29-PAPER-EXP-ADAPT-1-disposition.md，采纳上交选项①）
- 优先级 / 预估 / 依赖：P0 / 0.5 天 / PAPER-EXP-ADAPT-1 目标 1/2/3 已合入（七例工程+适配清单在材料仓 258b1ba）；证据基线=coord/runs/paper-exp/offline-qualification/20260929T2010/all7_qualify_results.json（全七例门复算）+主报告 §4
- 模型分级：L1 / GLM 首选（实验纪律相关断言变更）
- 目标：
  1. **门作用域化**：`qualify_material`（scripts/free_state_d1_smoke.py ~L234-259，现全 flavor 无条件）改为 fixture-set 作用域——manifest 侧声明 qualification profile（如 `qualification_profile: "spv1_synthetic" | "pv1_real_stems"`），spv1 保留现行门参数（零行为变化）；pv1 profile 按 all7_qualify_results.json 实测分布重校准。未声明 profile 的 manifest 走现行门（默认=spv1 口径，向后兼容）。
  2. **pv1 门参数推导**（不是拍脑袋放宽）：按七例实测分布定参数——安静轨 RMS 地板必须放宽（真实混音形态）；逐轨 crest/瞬态对比门的去留逐门裁定并记录依据（哪些门保留真实约束防不合格材料、哪些参数随材料类型变）；参数表与推导过程写入工件（可回指）。
  3. **门不形同虚设**：pv1 profile 仍须能拒绝真不合格材料（至少构造一组反例验证——如全静音轨/削波轨被拒），证明门在 pv1 语义下仍有区分度。
  4. **PILOT 闭环**（本卡验收=原 ADAPT-1 目标 4）：`run_free_state_d1_smoke.ps1 -PublicManifest <适配清单> -PublicCaseId pv1_p01`（及 p07）**exit 0**，工件落 coord/runs/paper-exp/vit-pilot/。
  5. **spv1 回归**：原 spv1 fixture 走原门全过（证明作用域化零破坏）——离线复算即可。
- 文件域：scripts/free_state_d1_smoke.py + scripts/run_free_state_d1_smoke.ps1（如需参数透传）+ 材料仓 experiments/out/suite_v1_lm_runner/fixture_manifest.json（profile 声明）。**不改六字段判据/盲法契约/配对设计**（56b 定稿纪律边界——改=停止上交）。
- 约束：真栈泊位声明必附（PILOT 两轮）；tag experiment-baseline-2026-09-29 不动（阻断在 python 侧，被测对象未变）；不修素材。
- 验收标准：PILOT p01+p07 exit 0（真栈工件可回指）+ spv1 回归全过 + 门反例区分度证明 + 参数推导工件入库。
- 停止条件：发现门与 D1 契约本体存在耦合（非独立素材前置）→ 实证上交，不擅拆契约。
- 领取：2026-09-29 21:05 / origin/main acf5c52c / `port/runner-gate-adapt-1`（worktree `D:/Vit_DAW_worktrees/runner-gate-adapt-1`，基于 tag `experiment-baseline-2026-09-29`=4b695354——PILOT 被测对象与 tag 一致，runner 两脚本自 tag 以来零改动，分支 diff=纯门变更）；领取时主树 HEAD=acf5c52c、工作树仅既有运行时残留 `M VitApp/Workspace/default_project.xml`
- 回执：（commit hash / PILOT run ID 与退出码 / 泊位声明 / HEAD）
- 验收：（裁定文件 / 验收 commit）

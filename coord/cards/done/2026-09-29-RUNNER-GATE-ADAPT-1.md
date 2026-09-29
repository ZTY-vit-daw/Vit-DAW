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
- 回执（执行侧自验，2026-09-29 21:10）：
  - **实现 commit**：`port/runner-gate-adapt-1` = **2932cf57**（已推 origin；基于 tag experiment-baseline-2026-09-29=4b695354，diff=scripts/free_state_d1_smoke.py 114 行+工件）；**材料仓 commit** = `毕业设计` 仓 **d7b686b**（仅 fixture_manifest.json 声明 profile，stems/projects 零改动）；ps1 零改动（profile 走 manifest，无需参数透传）
  - **目标 1/2（作用域化+推导）**：manifest 顶层 `qualification_profile`（未声明=spv1 原口径零变化；未知名 ValueError fail-closed）；pv1 参数按七例实测分布推导——RMS −45→−100、逐轨 crest 10→6、best crest 14 不变、逐轨 sibilance 撤销+best 14→6、逐轨瞬态撤销+best 14 不变；逐门裁定依据与削波判别边界实测扫描在 `coord/runs/paper-exp/gate-adapt/GATE_DERIVATION.md`（+pv1_all_metrics.json 全指标）
  - **目标 3（反例区分度）**：静音轨（RMS+crest 双拒）/重度削波轨（crest 拒）/平稳噪声集（sibilance best 0.51<6 拒）3/3 REJECTED；另证 p01 在默认 spv1 口径下仍被拒（放行由 manifest 声明选择非全局放宽）——`verify_profiles_results.json`
  - **目标 5（spv1 回归）**：原 fixture 未声明→默认口径 8/8 PASS（p02 缺本地工程为 temp fixture 既有状态，已记录未修）
  - **目标 4（PILOT 闭环）**：**p01 exit 0 + p07 exit 0**（各第 2 轮；§8 预声明重试 ≤4 轮）。p01：run 20260929_202650，track_gain，mutation 1（rev 3→4），读回 −1.0 dB，validation=pass；轮 1 NOT_EXERCISED（模型分支：单响应只读诊断）已分类记录。p07：run 20260929_203751，static_eq（VST3 550A），mutation 1（rev 9→10），读回 −3，validation=pass；轮 1 capability_blocked（插件参数读回不可确认→D1 回执缺失断言拒；历史面=static_eq 已知方差，2026-09-28 已有两轮 fail 先例，非本卡引入）。工件 `coord/runs/paper-exp/vit-pilot/20260929T2100_{p01,p07}/`，原始轮目录复制至主树 `artifacts/free_state_d1_s1/{20260929_202240,202650,203032,203751}/`（未覆盖）
  - **泊位声明（AGENTS §9）**：本会话 2026-09-29 20:22–20:44 独占真实栈；起栈前 5555/5556/7878 全空、无 VitApp/VitAgent/Godot 残留（初查 4 个"Godot"为查询命令自匹配误报）；PILOT 全栈泊位于 worktree `D:/Vit_DAW_worktrees/runner-gate-adapt-1`（内核=主树 tag 等价 exe 哈希复核后复制，SHA256 78dc4889…；agent=worktree tag 源码构建 2d80a782…；runner=分支版）；收栈后 VitAgent 14700/VitApp 12008/Godot 17556 全停、三端口复空、authority 全程 manual_confirmation；测试时 HEAD=port/runner-gate-adapt-1 2932cf57 前身（tag+门变更）；-SkipBuild 使用已按 §9 记录二进制哈希
  - 端测边界声明（AGENTS §5）：本卡改动为 runner python 侧（非 agent/webui 渲染面），验证=真实三件套端侧烟测（PILOT 两轮 exit 0）+离线验证矩阵；无 webui 改动
- 验收：（裁定文件 / 验收 commit）
- 验收：pass（2026-09-29，决策侧）——rulings/2026-09-29-RUNNER-GATE-ADAPT-1-pass.md；diff 亲核（fail-closed/best 级门保留/flavor 子门不静默重校准）+验证矩阵核读+决策侧复算五格全符（含 p05@默认口径两卡互证）+PILOT 双例真栈 exit 0 工件亲核（重试协议合规+泊位规范）；合入=cherry-pick 2932cf57；PAPER-EXP-ADAPT-1 目标 4 闭环、PILOT 前置就绪。

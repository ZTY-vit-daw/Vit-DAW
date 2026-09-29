# PILOT p01 真栈轮（RUNNER-GATE-ADAPT-1 门适配后）

- 命令：powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run_free_state_d1_smoke.ps1 -RepoRoot "D:/Vit_DAW_worktrees/runner-gate-adapt-1" -PublicManifest "C:/Users/timoz/Documents/毕业设计/experiments/out/suite_v1_lm_runner/fixture_manifest.json" -PublicCaseId pv1_p01 -PromptFlavor neutral -SkipBuild
- 最终退出码：**0**（D1-S1 PASS: real-stack public-only smoke completed）
- 材料：manifest 声明 `qualification_profile: "pv1_real_stems"`（材料仓本卡提交）；`material_qualification.qualification_profile=pv1_real_stems` 已入报告
- 结果实质：自主选域 **track_gain**；正向 mutation 1 次（revision 3→4）；读回 `actual_readback_db=-1.000000476837158`；audition ready；`validation.status=pass`
- 原始工件目录（主树，未覆盖）：`D:\Vit_DAW\artifacts\free_state_d1_s1\20260929_202650\`（含 project/ 持久化回路与 agent_last.log）

## 重试协议记录（AGENTS §8，预声明：每例 ≤4 轮、成功=exit 0、断言/崩溃/环境类先诊断、止损=4 轮 blocked 上交）

| 轮 | 原始目录 | 退出码 | 分类 |
|---|---|---|---|
| 1 | artifacts\free_state_d1_s1\20260929_202240\ | 3（NOT_EXERCISED） | 模型分支：单响应只读诊断后 goal=completed，未入 D1 工作流（报告见 attempt1_not_exercised_d1_smoke_report.json） |
| 2 | artifacts\free_state_d1_s1\20260929_202650\ | **0** | PASS（上表"结果实质"） |

- 测试时代码：worktree `D:/Vit_DAW_worktrees/runner-gate-adapt-1`，分支 port/runner-gate-adapt-1（基于 tag experiment-baseline-2026-09-29=4b695354）+ 本卡门作用域化改动；VitApp.exe SHA256 `78dc4889921204472ddca365ffa1915287ef8c5d69bd7ba8c91c68463ad0631f`（主树 2026-09-29 19:37:42 构建原样复制进 worktree，复算哈希一致=tag 等价二进制）；VitAgent.exe 为 worktree 内 tag 源码构建（SHA256 `2d80a78220f11d275039ad52bfdaca6ded8212b51943de86c213a435da3c217e`，20:22:16）；runner py 为 worktree 分支版（含门变更）
- 无 -SkipBuild 以外的特殊开关；VIT_AGENT_DOMAIN_ROUTES / VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET 未设
- 栈泊位：2026-09-29 20:22–20:44 独占（起栈前 5555/5556/7878 全空、无 VitApp/VitAgent/Godot 残留；收栈后三进程停、三端口空）

# PILOT p07 真栈轮（RUNNER-GATE-ADAPT-1 门适配后）

- 命令：powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run_free_state_d1_smoke.ps1 -RepoRoot "D:/Vit_DAW_worktrees/runner-gate-adapt-1" -PublicManifest "C:/Users/timoz/Documents/毕业设计/experiments/out/suite_v1_lm_runner/fixture_manifest.json" -PublicCaseId pv1_p07 -PromptFlavor neutral -SkipBuild
- 最终退出码：**0**（D1-S1 PASS: real-stack public-only smoke completed）
- 材料：manifest 声明 `qualification_profile: "pv1_real_stems"`；`material_qualification.qualification_profile=pv1_real_stems` 已入报告
- 结果实质：自主选域 **static_eq**（VST3-API-550A 频段增益）；正向 mutation 1 次（revision 9→10）；读回 `actual_readback_value=-3`；audition ready；`validation.status=pass`
- 原始工件目录（主树，未覆盖）：`D:\Vit_DAW\artifacts\free_state_d1_s1\20260929_203751\`（含 project/ 持久化回路与 agent_last.log）

## 重试协议记录（AGENTS §8，预声明同 p01）

| 轮 | 原始目录 | 退出码 | 分类 |
|---|---|---|---|
| 1 | artifacts\free_state_d1_s1\20260929_203032\ | 1（断言失败） | **capability_blocked**：static_eq 准入 G1–G8 全过、插件实例已加载（PLUGININSTANCE node_added、revision 7、plugin_count 1），但参数读回无法确认（agent 诚实终态"已应用的改动无法通过回读确认"；一项混音级视图不可用），D1 回执缺失 → runner 断言 "D1 receipt requires distinct before/after revisions"。历史面核：static_eq 插件链在 spv1 上大量 PASS 亦有大量 fail（含 2026-09-28 两轮 fail，早于本卡）＝已知方差面，非本卡门改动引入（报告见 attempt1_assertion_fail_d1_smoke_report.json） |
| 2 | artifacts\free_state_d1_s1\20260929_203751\ | **0** | PASS（同域 static_eq 完整闭环，上表"结果实质"） |

- 测试时代码/二进制/泊位：与 p01 run_manifest 同（同一 worktree、同 VitApp/VitAgent 哈希、同栈泊位窗口 20:22–20:44）

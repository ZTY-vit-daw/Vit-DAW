# D1D3-REGRESS-STACK-1（296）run2 §8 口径（跑前成文）

- 时间：2026-11-08 晚窗；owner=GLM 主会话 L2-worker 子代理（ZCode/PC，会话 296r2）。
- 依据：卡面 296 验收节 rework 裁定四条（基线=现 main 重跑一次非原样重跑；capability_blocked 两现止损；排栈队列 300 S0/P 之后；预算沿用 ≈¥0.6/run）+ run1 工件（run1/evidence.md 归因分列）。
- 与 run1 唯一口径差：**基线版本已变**——run2 worktree=D:/Vit_DAW_wt_296r2 @ port/d1d3-regress-stack-r2 = origin/main **8c68c186** + cherry-pick 272f27e9（D2 门组断言修正，主管预收 pass）→ 860d318d。祖先亲证 f413fbca(274)+5c8712a6(292)+**f6d1982e(295，run1 归因②候选变量——误导性 track.{id}.* 逐轨稳定键模板移除)** 三修全在场。**agent 源码零改动红线**（diff vs origin/main 仅 ps1 D2 断言块 3 hunk+run1 RUNBOOK 工件）。
- 被测 agent：worktree 现场构建 `agent\bin\VitAgent.exe`（SHA256 随 run 工件记）；kernel=D:/Vit_DAW_wt_228/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe SHA256 **F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A**（2026-11-08 18:14 亲核与 run1 只读复用同款）；引擎=本机默认无注入（deepseek-flash）；Godot 4.6.1 默认发现。

## 命令

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_296r2\scripts\run_free_state_d1_smoke.ps1 `
  -RepoRoot D:\Vit_DAW_wt_296r2 `
  -PublicManifest D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20\fixture_manifest.json `
  -PromptFlavor reverb -ExpectDomain reverb -D1LoudnessSettle -D2StereoCheck -D3RefAlign -SkipBuild `
  -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe
```

（run1 同款命令形；工件目录=coord/runs/D1D3-REGRESS-STACK-1/run2/ 不覆盖 run1；脚本自管起栈三件套 dev_agent_smoke.ps1 -RestartAgent -StartKernel -StartUI，agent 二进制先手构建+记 SHA256。）

## 断言预期（与 run1 RUNBOOK 同面，272f27e9 断言块原样在场）

- **D1 门 A（266）**：loudness_peak_status=present（ps1:1100-1105 既有断言，274 digest 视图修后驱动）；门 B/C/D 分支语义不变。
- **D2 门组（265，本卡 272f27e9 断言）**：全局面 stereo_status=present；tracks 面 7/7/6/1（7 考虑/7 行/6 带 correlation/1 无指标）；1039 行 track_id=1039 且 status=read；1007 锚值 corr=0.689±0.0005/balance=-1.475±0.0005；payload tracks_measured=6；verdict 三态 fixture-dependent 只记录。
- **D3 门组（266）**：既有断言不动（三态 verdict+profile/grade/动作面分支/节点点亮）。
- present/ready 断言带值/带形状不降级；预期面修正对照见卡面 run1 回执。

## 运行次数 / 成功条件

- **单 run 无重跑**（rework 授权一次重跑=本 run2；用尽即止）。
- 成功 = **PS1 exit 0**：runner 主链 PASS（1-mutation 主链/0-mutation 合法分支皆可）+ D1 settle 门组 + D2 门组（含 global_stereo/tracks_face 两门）+ D3 门组全绿 = 274/292 回归门收口（两卡闭环，D2 断言修正活栈兑现）。

## LLM 参与与预算

- 确定性腿（D1/D2/D3 门 A-D）零 LLM；LLM 腿=runner 主链+D2/D3 face P 探针（deepseek-flash）。预算 **≈¥0.6/run**（rework 裁定④沿用；run1 实耗 30 调用 ≈¥0.3-0.6 同量级）。跑后以 telemetry 亲数核对。

## 失败分类（预写）

1. **exit 0**=全 PASS→回归门收口（两卡闭环）；
2. **capability_blocked 形态再现**（run1 同型：runner 主链 settle 尾 materiality/target_response 缺席→py:1770 断言→PS1 exit 1）=**两现止损**：停、取证（响应链/round 形状/capability_boundary 载荷/telemetry）、上交主管开取证卡，不重跑；与 run1 归因对照（同型=LLM 概率腿系统性；不同型=记录差异面）；
3. **门断言失败**（runner 过、D1A/D2/D3 任一断言挂）：取证分列——修复缺陷回归（274/292 任一修真栈失效，按门定位）vs 断言预期面漂移（fixture 卫生/295 键宣传修后读数形态变化）；不追绿不重跑不现场改断言；
4. **环境中断**（端口残留/进程冲突/启动 throw/loopback 10061/LLM 服务中断类）：按 §9 只记不当作功能结论；中断类栈重建至多 1 次（run1 RUNBOOK 同款授权）；
5. **发现必须改 agent 源码才能跑**：停止上交取证（零源码改动红线）。

## 止损线

- 本 run2 即 rework 授权的最后一次主链运行：capability_blocked 再现=两现即停（上交开取证卡）；断言失败=取证上交不重跑；环境中断类 1 次重建授权用尽即止损。任何结局先拆栈净空再写回执。

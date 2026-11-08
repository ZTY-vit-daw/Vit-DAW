# D1D3-REGRESS-STACK-1（296）run3 §8 口径（跑前成文）

- 时间：2026-11-08 晚窗；owner=GLM 主会话 L2-worker 子代理（ZCode/PC，会话 296r3）。
- 依据：卡面 R2 验收四裁定（①D2 断言分支感知修正采纳——0-mutation=6/6/6/0，1-mutation=7 行按形状断言禁硬编码 id；②R3 收尾腿授权=断言修正+单 run，D2 通过即 D3 门组首次活栈执行；栈空闲窗口先行、310 就绪即让栈；预算 ≈¥0.6；同型确定性断点再败→止损禁第三轮；③D1 门 A 旁证认可不强制；④capability_blocked 取证卡不开）。
- worktree：复用 R2 的 D:/Vit_DAW_wt_296r2 @ port/d1d3-regress-stack-r2，基线 HEAD=f281d887（=origin/main 8c68c186 merge-base + 272f27e9 断言块 cherry-pick + run2 工件）+ 本腿断言分支感知修正（仅 scripts/run_free_state_d1_smoke.ps1 D2 断言块 +76/-25）。**agent 源码零改动红线**（274 f413fbca/292 5c8712a6/295 f6d1982e 三修祖先亲证在基线）。
- 被测 agent：worktree 现场构建 `agent\bin\VitAgent.exe`（SHA256 随 run 工件记）；kernel=D:/Vit_DAW_wt_228/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe（run1/run2 同款只读复用，SHA256 F9E3DA0A…，登记前亲核）；引擎=本机默认无注入（deepseek-flash）；Godot 4.6.1 默认发现。

## 断言修正（裁定①兑现，run 前离线亲证）

- **修正内容**：D2 门组 tracks 面两断言改分支感知——0-mutation 分支（$zeroMutationBranch，源自 validation.branch=zero_mutation_boundary，ps1:679-694 既有分支检测）：counts 6/6/6/0+六行全 status=read+零无指标行；1-mutation 分支：counts 7/7/6/1+唯一无指标行=非 fixture 六轨 id（runner 新建轨 id 自动分配，**不硬编码**）且 status=read。gate_results.tracks_face 相应分支化；其余断言（global stereo present/1007 锚值 ±0.0005/payload tracks_measured=6/门 B-D）两分支共用零改动。
- **PARSE_OK**：修正后整脚本 Parser::ParseFile 零错误（13915 tokens）。
- **离线双形状干跑**（run3/dryrun_console.txt，272f27e9 预收同模式）：从真实脚本切出修正块原文（1502-1582 行 81 行）在 StrictMode Latest 下喂 8 构造形状——正形状×3 全过（T1=0-mutation+run2 实盘逐字 face 6/6/6/0；T2=1-mutation 7/7/6/1 无指标行 id=1039；**T3=同形状但 id=1043 过**（禁硬编码实证））；反形状×5 必败（T4=0-mutation 喂 7/7/6/1；T5=0-mutation 喂 pre-292 全 missing；T6=1-mutation 喂 6/6/6/0；T7=无指标行 status=missing（pre-292 形态）；T8=无指标行 track_id=1007（fixture 轨行丢失=回归形态））。DRYRUN_VERDICT: ALL_SHAPES_AS_EXPECTED。

## 命令

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_296r2\scripts\run_free_state_d1_smoke.ps1 `
  -RepoRoot D:\Vit_DAW_wt_296r2 `
  -PublicManifest D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20\fixture_manifest.json `
  -PromptFlavor reverb -ExpectDomain reverb -D1LoudnessSettle -D2StereoCheck -D3RefAlign -SkipBuild `
  -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe
```

（run1/run2 同款命令形；工件目录=coord/runs/D1D3-REGRESS-STACK-1/run3/ 不覆盖旧 run；脚本自管起栈三件套 dev_agent_smoke.ps1 -RestartAgent -StartKernel -StartUI，agent 二进制先手构建+记 SHA256。）

## 断言预期

- **D1 门 A（266）**：loudness_peak_status=present（0-mutation 分支脚本位结构性 skip 时由 D2 carrier 旁证，裁定③认可；1-mutation 分支 settle 门自然执行）。
- **D2 门组（265，本腿分支感知断言）**：global stereo_status=present；tracks 面**按实际分支**——0-mutation=6/6/6/0 全 read；1-mutation=7/7/6/1+runner 新建轨 read-无指标行（形状断言）；1007 锚值 corr=0.689±0.0005/balance=-1.475±0.0005；payload tracks_measured=6；verdict 三态 fixture-dependent 只记录。
- **D3 门组（266，本 run 首次活栈执行）**：既有断言不动——stop_reason=d3_ref_align_checked+三态 verdict（取值 fixture-dependent 不预断言，卡面 266 口径）+profile=builtin:spotify（basis 非空）+grade=approximate+零 mutation+verdict store 在盘+动作面分支（gain_card_v1→mix_tick_confirmation 卡+取消腿 mix_tick_rejected；none_needed/disclosure_only_v1→零卡）+D3 节点点亮；face P 探针如实记录不挡门。
- present/ready 断言带值/带形状不降级；分支感知=对准两分支各自真值，非放松门。

## 运行次数 / 成功条件

- **单 run 无重跑**（R3 收尾腿授权=最后一轮；用尽即止）。
- 成功 = **PS1 exit 0**：runner 主链 PASS（两合法分支皆可）+ D1 settle 门组（或 0-mutation 结构性 skip-with-reason）+ D2 门组全绿（分支感知断言活栈兑现）+ **D3 门组全绿（首次活栈覆盖）** = 274/292 回归门收口+D3 覆盖缺口补齐。

## LLM 参与与预算

- 确定性腿（D1/D2/D3 门 A-D）零 LLM；LLM 腿=runner 主链+D2/D3 face P 探针（deepseek-flash）。预算 **≈¥0.6/run**（裁定②沿用；run1 实耗 30 调用 ≈¥0.3-0.6、run2 实耗 9 调用 ≈¥0.1-0.2）。跑后以 telemetry 亲数核对。

## 失败分类（预写）

1. **exit 0**=全 PASS→整卡收口（D3 三态处置照卡面：全绿即记 274/292/D3 三面闭环）；
2. **同型确定性断点再败**（与 run2 同型=D2 tracks 面在新断言下仍失败，或 D1/D3 门确定性断言失败）：**止损取证上交，禁第三轮重跑**（裁定②明文）；取证分列——修复缺陷回归 vs 断言预期面漂移 vs D3 门首达新形态（D3 断言若失败属首次活栈形态，按卡面三态处置原文：取证分列上交，不追绿）；
3. **capability_blocked 形态再现**（对 run1 两现，裁定④语境）：停止取证上交（run1 归因记录在案）；
4. **环境中断**（端口残留/进程冲突/启动 throw/loopback 10061/LLM 服务中断类）：按 §9 只记不当作功能结论；中断类栈重建至多 1 次（run1/run2 RUNBOOK 同款授权）；
5. **发现必须改 agent 源码才能跑**：停止上交取证（零源码改动红线）。

## 止损线

- 本 run3 即 R3 授权的最后一次主链运行：同型确定性断点再败=止损取证（禁第三轮）；断言失败=取证上交不重跑；环境中断类 1 次重建授权用尽即止损。任何结局先拆栈净空（TEARDOWN_ALL_ZERO 亲核）再写回执。310 A-pure 就绪即让栈（登记前重读栈记录，若 310 已占栈则等待）。

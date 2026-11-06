# D3-REF-ALIGN-THIN-1 verifyrun1 RUNBOOK（§8 跑前口径，run 前成文）

- 2026-11-07 01:1x +0800 / owner=ZCode GLM-5.3 主会话 L2-worker 子代理（266impl，验证腿主管派发）
- 被测=wt_266impl @ port/d3-ref-align-thin-1 **49a85a25**（impl 30877292 + origin/main fe8ab880 合入[276 整卡+269 终验+277 段1]零冲突——ps1 两行 -Encoding UTF8 与 D3 门组区域不相交自动合并亲证；合并后 `go build ./...` exit 0 + chat 全包 `-count=1` 122.1s ok 亲证）
- 被测 agent=worktree 现场构建 `go build -o ./bin/VitAgent.exe ./cmd/vitagent`，SHA256 **F4B5A2150CB0936A19D37A9A5E13D632CA3825E1E79CB5AA63A303BF3EC7603D**（2026-11-07 01:09:37，含本卡全部改动——SkipBuild 前提满足）
- kernel=wt_228 `D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe` SHA256 **F9E3DA0A66263F0B6E3FED5CDA0FB214C460FCE8053234240C470D515ED0D67A**（01:0x 亲核，与 269/276 同款只读复用）
- 引擎=本机默认无注入：`~/.vit/config.json` defaultModel=deepseek-flash 亲读；VIT_AGENT_LLM_MODEL / VIT_AGENT_DOMAIN_ROUTES 均 UNSET 亲证；不传 -LlmModelOverride

## 命令（单 run 无重跑）

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_266impl\scripts\run_free_state_d1_smoke.ps1 `
  -RepoRoot D:\Vit_DAW_wt_266impl `
  -PublicManifest D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20\fixture_manifest.json `
  -KernelExe D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe `
  -SkipBuild `
  -LoudnessWatch -D1LoudnessSettle -D2StereoCheck -D3RefAlign
```

- 门组叠加（266 派发令第 4 条）：-SkipBuild（kernel 用 wt_228 只读复用不重建；agent 用 worktree 预构建二进制）+四门组串联（LW→D1 settle→D2 stereo→D3 ref-align，按脚本既定次序 runner 后追加）。全门组全断言（LW/D1/D2 为已收口门组的零回退面，D3 为本卡判定面）。
- 成功条件=**PS1 exit 0**。

## 预算（跑前预写，不看结果倒推）

- D3 face P 裸话术探针 1 腿 LLM（≈¥0.1-0.5）；D1 runner 主链 1-mutation 腿（≈¥0.5-2.5，非高峰窗晚跑前例）；LW/D1/D2/D3 门 A-D 全部零 LLM。合计估 **≈¥1-3**（卡面口径）；telemetry 原件 `D:\Vit_DAW\VitApp\Workspace\Logs\agent_llm_telemetry.jsonl` run 后快照入工件。

## 失败分类（预写）

- ① **PASS**：exit 0，全门组绿（D3 门 A-D + face P 记录面）。
- ② **门 A-D 断言失败**：逐门取证（console + d3_gates.json + 对应证据件），分类=本卡面缺陷（判据/runtime/脚本管线）或已收口门组回归（LW/D1/D2 面）；**取证上交不重跑**（AGENTS §8：同版本同断点两败禁原样重跑；本卡授权单 run）。
- ③ **环境通用类**：排干族瞬态（254/265 前例：wait_scheduler_drain loopback 拒绝/排干 throw）、端口/进程残留、Godot 冷启动超时——仅当原始日志在盘可证时记环境中断；按 269 先例**允许 owner 亲拆重建一次**（重建后仍算同一次授权 run 的重试面，第二次失败=取证上交）。
- ④ **face P 探针未过 semantic_entry**：不挡门（记录面，交主管裁定是否建词表——263 教训证据面）。
- 止损线：单 run 授权用尽即停；两败取证上交；不把未发生的成功改写成成功。

## 纪律

- 零代码改动（发现必须改=停止上交）；§10 防污染（fixture 只读经 -PublicManifest 引用，工件全部落脚本自建 artifactDir）；占用/释放按 PROTOCOL §2.2（登记推送成功→≤60s 即时复核→启栈；收口 TEARDOWN_ALL_ZERO 亲核+释放登记推 main）。

## verifyrun2 重跑节（§8 追加，2026-11-07 01:3x 跑前成文）

- **重派授权依据**：主管验证腿裁定（卡面 doing/ 权威本，main 30c3cd02）——原 run 失败在已收口 D1-settle 门组且本卡门组未及执行，**不算断言失败重跑**；授权**单门组变量收窄**。
- **命令（单开关，其余参数同 verifyrun1）**：`run_free_state_d1_smoke.ps1 -RepoRoot wt_266impl -PublicManifest（同上）-KernelExe wt_228 -SkipBuild -D3RefAlign`——不加 -LoudnessWatch/-D1LoudnessSettle/-D2StereoCheck（四开关叠加=上轮归因候选①，本轮收窄排除）。
- **单 run 无重跑**（本次为第二次也是最后一次授权 run，主管明示不再授权第三次）；预算沿用 **≈¥1-3**（D3 face P 1 腿 LLM+runner 主链 LLM 腿）。
- 失败分类沿用①-④；②取证上交后**不再有第三次 run**。
- 被测不变：port 分支 fc0790ef 工作树（agent 代码=49a85a25 态，agent 二进制 F4B5A215 复核未变；kernel wt_228 F9E3DA0A 复核未变）；引擎=本机默认无注入。
- **挂账观察项（主管裁定：记录不追查）**：D1 观察面选择性缺失形态（kernel digest 无 peak/rms/crest 而 stereo 在场）是否再现——本轮单开关 run 的 D3 check turn 观察面若再现该形态，记录证据即可（工件+d1 观察 JSON），不定性不扩查。

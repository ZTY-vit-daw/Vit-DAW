# 271 CLOSURE-ROUTE-ADAPT-1：独立真实栈验证跑前口径

- 2026-11-17；执行 owner=Codex `/root/closure271`（Sol/high，PC），最终验收归主管 `/root`。
- 本文件成文时仅完成**零栈装置准备**。PC 栈仍归 `/root/entry388`；未启动/探测/重启 VitApp、agent、Godot、VitShell，未调用其端口。主管另行放行及占用登记前不得执行下列命令。
- 唯一写入 worktree=`C:/Users/timoz/.codex/worktrees/closure-validation-271/Vit_DAW`，branch=`codex/closure-validation-271`。领取基线 HEAD=`e274536921e6689ace370c044babb0ec85fc1073`，初始 status/diff 均空；领取协调提交 `0790ecef` 已推共享 main、owner 已由主管核验。
- 被测生产修复=`cf22db7f`，已确认是本 worktree HEAD 祖先。旧 276 rerun1 被测 agent 含 `2f61434d`、尚无 `cf22db7f`；其 `closure.mode=diagnostic`、空 frontier、零 executed、`limit_type=max_turns` 为历史失败证据。**本次是271修复版本的一次独立验证，不是旧276第三次原样重跑。**

## 文件域与待证锚点

仅改两个既有装置脚本：`scripts/dev_agent_smoke.ps1` 和 `scripts/edu_transform_smoke.py`；本 RUNBOOK 为配套工件。不改 Go、模型 prompt、semantic route、closure 准入、golden 或治理门。

- `audio_closure_controller.go:463`：诊断判断读 closure 自身 mode；`:497`：budget 分支保留 caller base，仍钳制工具调用≤6，治疗/legacy及automation分支另有原契约。
- `goalrunner_chat.go:862`：`free_state_diagnostic_only` caller base=5 turns/6 calls；无 flag 的mode base可不同。因此运行判据为**实际同身份slice `max_turns > 1`**，不把5写死，也不凭源码代替运行证据。
- `server.go` 的 `GET /agent/runtime/status` 披露 `task.run.slices`；`runtime/task_runtime.go:58` 的 `InvocationSlice` 自带 `max_turns/max_tool_calls/turns_used/tool_calls_used`。保存整个只读响应；task/goal/run 与 chat 响应一致，且 slice_id 一致，才认可budget证据。身份漂移/字段缺失记未证实，不改 Go 补锚。
- `edu_transform_smoke.py` 保留276原有同形用户消息、42音小星星构造及全部golden。每次run由git-clean默认XML复制件启动，fixture仅在本run的新project.vit构造，不打开历史276工件，不发“继续”，不注入私有closure/route字段，不改变话术追命中。

## 唯一正式场景与判据

Phase A零LLM：既有ZMQ fixture构造和回读。Phase B仅一次含LLM的真实 `/agent/chat`（包含semantic-entry、capacity等前置及同回合工具往返；既有ps1基线的确定性 `/smoke range_context` 不计作模型场景，全部实际telemetry仍计费核对）。Phase C：golden与271额外证据判定。

PASS必须同时满足：

1. 真实内核、现场构建agent、显式独立前端Godot三件套在场；`godot_boot.json`有真实Godot进程+`Godot Engine`日志锚，`stack_before_chat.json`记录三进程及内核/agent SHA256；Godot在chat后仍存活。
2. 生产响应 `workflow_data.minimal_audio_closure.mode == diagnostic`；**自然路由未命中为 `not_exercised`（py exit5），不能记PASS或换话术再跑。**
3. 只读runtime四身份匹配，对应slice实际 `max_turns > 1`；响应未在 `limit_reached/max_turns` 停泊。若有max_turns终止，保留实际budget，不能不查数值便归因为旧1-turn clamp。
4. `executed_kernel_reply`真实含 `midi.analyze_transformations`（或命令名）的status=ok结果。工具日志仅为链路进度，不代替 executed 行。
5. 既有golden全成立：42 notes；diatonic proto8→match16，shift2/steps1，label「模进（调内，上移 2 半音）」和bars3–4→5–6；repeat proto16→match24、shift0、label「原样重复」、无sequence_quality、bars5–6→7–8。
6. PS1整体exit0；recents原字节/原absence+SHA256还原、源XML和fixture生成器前后hash一致、`TEARDOWN_ALL_ZERO`均成立；最终全部telemetry未越账面预算。

报告必须分别列：diagnostic命中、budget证据、工具进度、golden、最终验收。一次exit0仅证明存在成功路径，不声明稳定性。WebUI渲染及用户完整旅程不在本卡覆盖面；Godot只证明真实前端在场与boot，不声称GUI交互验收。

## 次数、账面预算与止损

- 正式场景**唯一1次**，无随机分支重跑、无用户/自动“继续”驱动。环境中断有原始端口/进程退出/传输证据才允许至多1次重建；重建另开全新run目录、记录原run，不覆盖工件，不凭不命中或LLM未达申请重建。
- LLM账面上限：**全run完成调用≤8，累计input≤250000 tokens，output≤16000 tokens**，包括semantic-entry/capacity/message-loop/scheduler等全部来源，不只统计目标conversation。目标conversation与全run均记账；finally拆栈后再次核全run telemetry。超过任一项为 `budget_exceeded`（py exit6或最终d1报告非零），立即停止后续投入。
- 本上限为**请求返回/拆栈后账面核对，非运行中硬熔断**；不承诺自动在第8次或¥2中断。软成本止损目标≤¥2；现有脚本不读取供应商价格/密钥，无法保证精确货币硬上限。成本以真实model、usage/cache原始telemetry交主管核算，不假设历史deepseek-flash配置仍现行。
- chat HTTP timeout=300s；生产诊断slice自带210s等原预算。timeout不代表服务器已停；外层finally立即拆本run精确进程路径再核零残留。全场景墙钟软止损20min，运行owner如发现卡住即停止本装置进程并执行同一拆栈/还原，不另发chat。
- 有明确失败即停；模型路由未命中、no_candidate_found、capability_blocked、漏key/不调工具均不自动重跑；确定性断点不原样凑成功。

分类：py0=PASS；py2=environment（需原始栈/传输/退出证据复核，fixture回读不符不能据此直接归因模型）；py3=LLM行为未达（`failure_kind`保留no_candidate_found/capability_blocked）；py4=golden/assertion或budget证据未达（有真实max_turns数值再判断预算缺陷）；py5=not_exercised；py6=budget_exceeded。崩溃保留原始退出/日志，未获证据的外层失败记unclassified_failure；recents/拆栈卫生失败强制非零、归environment_interrupt。不得把工具调用发生/局部golden数据或单测通过写作真栈终验。

## 未来运行命令（当前禁止执行）

主管须先提供**独立已部署内核运行时**绝对路径与**独立前端检出**绝对路径；不得启动主检出的内核/前端，也不解析/运行Godot作准备检查。首次执行前把真实路径和完整命令补入本run证据，确认前端HEAD/status及内核SHA256。脚本拒绝缺显式Godot参数、共享前端默认路径、SkipBuild、已有run目录、继承的project/history/user-config目录覆盖和无关冒测开关。

```powershell
# $kernelRuntimeExe / $frontendWorktree / $godotExe 由主管放行时填入已确认绝对路径。
$repo271 = 'C:/Users/timoz/.codex/worktrees/closure-validation-271/Vit_DAW'
$run271 = Join-Path $repo271 ('coord/runs/CLOSURE-ROUTE-ADAPT-1/validation-20261117/run_' + (Get-Date -Format 'yyyyMMdd_HHmmss'))
powershell -NoProfile -ExecutionPolicy Bypass -File "$repo271/scripts/dev_agent_smoke.ps1" `
  -RepoRoot $repo271 -StartKernel -KernelExe $kernelRuntimeExe `
  -RestartAgent -StartUI -GodotProjectRoot $frontendWorktree -GodotExe $godotExe `
  -NoChatSmoke -NoStripSilenceSmoke -ClosureDiagnosticValidation -RunArtifactsDir $run271
$code271 = $LASTEXITCODE
```

占栈前按PROTOCOL：主管确认388释放→fetch重读资源owner→进程/端口净空→占用登记push共享main→启动前≤60秒再fetch重读owner及即时净空。脚本的preflight亦拒绝任何已有Vit/Godot进程或相关监听，绝不复用/擅杀外来栈。`-StartKernel`后既有`-StartUI`会no-op，271子腿另显式Hidden启动真实Godot，`VIT_SKIP_DEV_AUTOSTART=1`只给该子进程，防其另起agent/kernel。官方console Godot的同目录非console兄弟exe列入精确拆栈路径，不做全机宽泛kill。

## 工件与卫生

每run新目录，outer wrapper在任何进程启动前快照 `%APPDATA%/Vit-DAW/global_project_config.json` 到 `recents.before.json`（若原不存在以before.meta记录absence）；拆进程后finally按原字节或absence还原，核hash。默认XML源与Python fixture生成器前后SHA256均入d1报告。源有变或还原/零残留失败使装置非零，禁止静默。

保留：`console_ps1.txt`、`d1_smoke_report.json`（HEAD/status/diffstat/全部实参/退出码/清理/终账）、`godot_boot.json`+原始Godot log、`stack_before_chat.json`（二进制hash）、`kernel_last.log`（若部署内核生成）、`logs/agent_last.log`、`agent_llm_telemetry.jsonl`、`agent_prompt_render.jsonl`、`chat_request.json`、`chat_response.json`、`runtime_status_after_chat.json`、`closure_evidence.json`、`agent_chat_delta.log`、`edu_transform_summary.json`/console、默认XML独立副本、fixture project与`fixture_notes.json`。不读取/记录API key或config密钥，不因诊断把raw package注入模型。

拆栈后owner还需亲核TEARDOWN_ALL_ZERO与recents hash，才交主管更新资源释放。若finally残留报错，保留owner、待处置，不自动清理其他路径。不修改coord卡态，不push main。

## 已完成的零栈装置QA

- Python AST解析（`python -c "import ast,pathlib; ast.parse(pathlib.Path('scripts/edu_transform_smoke.py').read_text(encoding='utf-8')); print('PY_SYNTAX_OK')"`）exit0。
- PowerShell `[System.Management.Automation.Language.Parser]::ParseFile(...)` 无错误、exit0；`git diff --check` exit0。
- 本地数据适配的完整Python评估器6案exit0：现有历史direct-invoke golden+diagnostic/budget5→py0；无路由→5；foreign task→4；旧single-turn/max_turns→4；budget1但无stop→4；9调用→6。历史rerun1 telemetry核对2 calls、input14731/output254/cache4096；同时修正既有Python漏识别Go usage input_tokens/output_tokens/cache_read_tokens的计费统计。**以上仅评估器回归，历史数据不当现状成功。**
- PowerShell AST抽取真实outer finally，仅将child入口换成本地“写坏recents然后抛错”的失败适配器，进程/端口提供器均空mock。原始二进制bytes与原absence两案均恢复亲证；测试命令 `powershell -NoProfile -ExecutionPolicy Bypass -File C:/Users/timoz/AppData/Local/Temp/vit271-hygiene-3ad5264c428f414aa7c2fcb94bbc40f4.ps1` exit0；临时工件 `C:/Users/timoz/AppData/Local/Temp/vit271-hygiene-67bf8c05a1384b8e89158eaa975d725b`。最初离线测试命令因嵌套here-string被PowerShell解析拒绝，未执行测试体；修正本地测试包装后上述两案通过，非产品失败/真栈运行。
- Go未改，不跑Go或编译真栈；不把上述QA视为271终验。装置提交后由主管复核，再决定是否放行唯一正式场景。

## 启栈前守卫修订与部署披露（仍未放行）

- 主管复核 `fb424394` 后要求补长构建窗口：271现在在 `go build` 返回后、`Stop-AgentForPaths`之前fetch origin/main，UTF8读取共享资源最新首条事件，要求占用marker和owner `/root/closure271`；中文marker以Unicode codepoint构成，避免PS5.1无BOM源码ANSI误解。随后净空检查；`StartKernel`紧前再次净空，owner证明超过60秒则重新fetch重读。别owner、释放事件、已有进程/监听均停止，不静默复用或终止外来进程。
- VspHub无需启动的依据：`agent/cmd/vitagent/main.go:114–127` 仅 `runtimehost.EnabledFromEnv()` 为真才启runtimehost；`runtimehost/runtimehost.go:102` 仅 `VIT_AGENT_SPAWN_RUNTIME=1/true/on/yes` 为真；`main.go:160` 的 `startVSPHubRegistration` 只是HTTP注册重试，无spawn。271 wrapper拒绝继承该变量真值；Godot仍设置子进程专属 `VIT_SKIP_DEV_AUTOSTART=1`，所以本次内核/Godot/agent三件套不缺hub依赖。最终净空依然要求hub=0。
- finally清理进一步限定为本run `owned_processes.jsonl` 登记的PID+绝对路径+启动时间三项一致，且路径在明确允许列表中；官方Godot console包装器的同目录GUI兄弟仅在父PID匹配时登记。未登记外来进程不杀，残留使报告非零。无泛化全机进程树kill。
- 主管部署已确认：内核到本worktree `VitApp/build/VitApp_artefacts/Release/VitApp.exe`，来源主仓A5947361同款二进制（本卡零C++改动）；Settings独立复制到本worktree `VitApp/Workspace/Settings/Settings.xml`，SHA256前缀 `72F0F511`、尾 `EDFBB8`。该Settings是已知运行库修改，保留不纳代码commit，不还原为仓库旧值。A5947361未含378 D3，披露为边界，不触本卡轮预算/音符分析。运行时仍必须记录完整SHA256。
- 未来独立前端参数确定为 `C:/Users/timoz/.codex/worktrees/godot-runtime-388/vit-daw-frontend` @`d05043e1`；资源/扩展/缓存由主管确认已水合。未含394/402，不借本卡结论判焦点卡。当前仅记录路径，无Godot解析/运行。
- 修订QA：PS5.1抽取真实guard函数，git/进程/端口全为本地空/模拟提供器；owner271+占用+净空通过、owner388拒绝、最新释放marker拒绝、外来agent拒绝，exit0。语法和diff检查exit0。无真实fetch/端口探测/进程操作；正式场景仍待主管登记放行。

## 正式启动记录（跑前）

- 放行与资源登记：主管明确放行，origin/main f83cca4f资源首条owner /root/closure271已亲核；当前HEAD=e79740dc，唯一M=主管部署Settings。
- run：C:\Users\timoz\.codex\worktrees\closure-validation-271\Vit_DAW\coord\runs\CLOSURE-ROUTE-ADAPT-1\validation-20261117\run_20261117_122151
- 完整命令：powershell -NoProfile -ExecutionPolicy Bypass -File C:/Users/timoz/.codex/worktrees/closure-validation-271/Vit_DAW/scripts/dev_agent_smoke.ps1 -RepoRoot C:/Users/timoz/.codex/worktrees/closure-validation-271/Vit_DAW -StartKernel -KernelExe C:/Users/timoz/.codex/worktrees/closure-validation-271/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe -RestartAgent -StartUI -GodotProjectRoot C:/Users/timoz/.codex/worktrees/godot-runtime-388/vit-daw-frontend -GodotExe D:/Godot/Godot_v4.6.1-stable_win64_console.exe -NoChatSmoke -NoStripSilenceSmoke -ClosureDiagnosticValidation -RunArtifactsDir C:\Users\timoz\.codex\worktrees\closure-validation-271\Vit_DAW\coord\runs\CLOSURE-ROUTE-ADAPT-1\validation-20261117\run_20261117_122151
- 内核SHA256=A59473614152882B21433DCEE684F8B13AE0BD5B632A97CD5E2C8765BB18348E；Settings=72F0F5112EF926ED77F59F015E6CE492142F8FDF1828E5082E970AA949EDFBB8；源XML=947E4D00886F146F7DB082215BC8A69E1727A41D5EA339FA064DEE814E0D521C。

### 装置轮122151（零LLM、非有效模型样本）

- build通过后guard因误要求前导斜杠而拒绝真实资源首条，outer exit1；真实共享格式为时间后直接接占用marker。原始d1/console/recents/fixture hash均保留；源资源f83cca4f字节快照保留resource_f83cca4f_raw.txt。进程/TCP/UDP亲核0，recents还原及亲核807226F4一致。属于装置断言格式错误，不是产品断言或模型随机环境中断。
- 修正仅移除marker前想象斜杠，并新增启动守卫原始资源落盘。PS5.1离线测试从本次真实f83cca4f首条UTF8快照取行，配对验证owner271通过、替换owner388拒绝、替换release marker拒绝及外来进程拒绝，exit0。原始测试文件C:/Users/timoz/AppData/Local/Temp/vit271-guard-a98c14844b7f413ea02818798d83d393.ps1。新run沿用冻结场景和全部预算，不continue、不改话术。

# 388 / 401 真实栈 RUNBOOK（待主管放行）

本轮只验388/401+既有旅程。稳定前端d05043e1不含394/402，不能据此判定新窗口焦点行为通过。人工手测仍由用户从Godot启动，以下仅自动化装置。

## 跑前冻结

- owner `/root/entry388`，主仓worktree `C:/Users/timoz/.codex/worktrees/converge-entry-388/Vit_DAW`，分支codex/converge-entry-388；运行前存实际HEAD/status/diffstat。
- 前端 `C:/Users/timoz/.codex/worktrees/godot-runtime-388/vit-daw-frontend` @d05043e1；主管已水合LFS spectrum_data.exr，已知M仅测试资产，不提交。扩展DLL已隔离复制。
- fixture源 `D:/Vit_DAW/temp/f3_empty_project.vit`，已确认存在，跑前SHA256=`AC0CE2E81C12E6F10E3D691CF062CD3E568365C274F12F2B963694AE3B48B41D`；每腿复制到run/work目录，仅副本可写。源哈希前后必须一致。
- 先主管登记共享栈owner并推main→fetch重读资源与即时（≤60秒）进程/端口净空；空闲不是复用许可。7878/7879/9333/5555/5556/8787及Vit进程须无其他owner。
- 两腿各1次有效运行，顺序串行；不声明稳定性。LLM供应商预算0：主旅程保持`VIT_AGENT_LLM_BASE_URL=http://127.0.0.1:1`，既有L3仅验证诚实失败交付；新管理局部区间必须telemetry零新增、零chat请求。ui_entry2不发送聊天，另设dead endpoint。
- 成功=对应脚本exit0+fixture/recents亲证+原始工件完备。分类分别记录assertion/崩溃/capability_blocked/no_candidate_found/环境中断（后两类不能凭猜测排除）；本卡确定性装置不应出现模型候选分支。同版本同断点2次失败后停止原样重跑，先锚点诊断；不新增预算、不删失败工件。

## 腿A：VitShell真实主旅程（含管理往返）

从本主仓worktree根目录，运行时将唯一run ID替换为实际时间；禁止SkipBuild首跑。

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/vitshell_full_smoke.ps1 -FrontendWorktree 'C:/Users/timoz/.codex/worktrees/godot-runtime-388/vit-daw-frontend' -Webview2SdkDir 'D:/Godot/project/vit-daw-frontend/extension/third_party/webview2' -GodotExe 'D:/Godot/Godot_v4.6.1-stable_win64_console.exe' -FixtureSource 'D:/Vit_DAW/temp/f3_empty_project.vit' -CardId 'CONVERGE-ENTRY-1' -Rounds 1 -EntryManagementChecks -RunRoot 'C:/Users/timoz/.codex/worktrees/converge-entry-388/Vit_DAW/coord/runs/CONVERGE-ENTRY-1/stack_A_20261117_XXXX'
```

装置默认仍跑既有failure/legacy腿（未弱化）；`-EntryManagementChecks`默认off，显式开才新增L2m。在fixture_a工程ready后、L3聊天前插入管理段：

- 真GET启动/管理两态最近列表；405拒绝POST；存在性/治理directive；两态左下settings打开、Escape与焦点归还，零config写。
- 查看启动页仍显示真实当前工程名，隐藏自由composer/建议词；new/open/磁带卡三个真实入口均可用且经原治理取消；返回工程输入保留。
- 独立GET503仅为synthetic failure injection，证明错误呈现不冒充空态；解除route后必须真实GET恢复。不得把该注入说成真内核故障。
- 局部before/after状态要求agent/Godot PID、VitApp/VspHub PID、绑定path/uuid、launch_count、window_hwnd/current_url不变。d05043e状态schema没有native navigate_count，替代证据为CDP主frame导航事件0+不变sentinel+HWND/current_url；null不可通过。
- telemetry通过显式env写本run独立jsonl，局部前后bytes必须相同；两份快照保留。后续既有L3允许dead endpoint失败telemetry，不能混算局部零新增。
- 394/402采样插入点需在触发open前开始，本段不验加载焦点；不触碰窗口原生实现。

finally沿用脚本既有整树拆栈、端口释放、recents bytes/absence还原和SHA256亲证、两root运行状态还原、fixture哈希核对。工件至少meta.json、main/frontend HEAD/status、构建日志与二进制hash、每阶段CDP JSON、管理截图/局部state/status/telemetry/agent原日志；归档完整run并记录最终退出码。该脚本无d1_smoke_report.json，使用原生meta.json与阶段JSON，不能伪造D1报告。

## 腿B：ui_entry2旧Godot宿主独立回归

该脚本的旧finally仅copy recents、fixture变化仅WARN；禁止裸跑。以下外层卫生命令在新run目录强制亲证recents还原（含原文件不存在）及fixture源不变；外层失败也为非零环境中断。它不替代腿A管理语义。先重新登记/核净空再运行。

```powershell
$entryRepo = 'C:/Users/timoz/.codex/worktrees/converge-entry-388/Vit_DAW'
$entryRun = Join-Path $entryRepo 'coord/runs/CONVERGE-ENTRY-1/stack_B_20261117_XXXX'
New-Item -ItemType Directory -Path $entryRun -ErrorAction Stop | Out-Null
$entryRecents = Join-Path $env:APPDATA 'Vit-DAW/global_project_config.json'
$entryFixture = 'D:/Vit_DAW/temp/f3_empty_project.vit'
$entryHadRecents = Test-Path -LiteralPath $entryRecents
$entryBeforeHash = if ($entryHadRecents) { (Get-FileHash -LiteralPath $entryRecents -Algorithm SHA256).Hash } else { 'ABSENT' }
if ($entryHadRecents) { Copy-Item -LiteralPath $entryRecents -Destination (Join-Path $entryRun 'recents_before.json') }
$entryFixtureHash = (Get-FileHash -LiteralPath $entryFixture -Algorithm SHA256).Hash
$entryExit = 1
try {
  $env:VIT_AGENT_LLM_BASE_URL = 'http://127.0.0.1:1'
  & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $entryRepo 'scripts/ui_entry2_smoke.ps1') -RepoRoot $entryRepo -GodotProjectRoot 'C:/Users/timoz/.codex/worktrees/godot-runtime-388/vit-daw-frontend' -GodotExe 'D:/Godot/Godot_v4.6.1-stable_win64.exe' -KernelExe (Join-Path $entryRepo 'VitApp/build/VitApp_artefacts/Release/VitApp.exe') -ProjectSource $entryFixture -RunRoot (Join-Path $entryRun 'device')
  $entryExit = $LASTEXITCODE
} finally {
  Remove-Item Env:VIT_AGENT_LLM_BASE_URL -ErrorAction SilentlyContinue
  if ($entryHadRecents) { Copy-Item -LiteralPath (Join-Path $entryRun 'recents_before.json') -Destination $entryRecents -Force }
  elseif (Test-Path -LiteralPath $entryRecents) { Remove-Item -LiteralPath $entryRecents -Force }
  $entryAfterHash = if (Test-Path -LiteralPath $entryRecents) { (Get-FileHash -LiteralPath $entryRecents -Algorithm SHA256).Hash } else { 'ABSENT' }
  $entryFixtureAfter = (Get-FileHash -LiteralPath $entryFixture -Algorithm SHA256).Hash
  @{ before=$entryBeforeHash; after=$entryAfterHash; fixture_before=$entryFixtureHash; fixture_after=$entryFixtureAfter; device_exit=$entryExit } | ConvertTo-Json | Set-Content (Join-Path $entryRun 'hygiene.json')
  if ($entryAfterHash -ne $entryBeforeHash -or $entryFixtureAfter -ne $entryFixtureHash) { throw 'ENV-INTERRUPT: recents/fixture hygiene mismatch' }
}
if ($entryExit -ne 0) { throw "ui_entry2 device exit $entryExit" }
```

保留该腿report/原日志/退出码和hygiene.json；与腿A分开汇报。ui_entry2是否仍适配稳定前端，以实际退出证据为准，不把旧链失败归给新管理视图。

## 当前零栈结果

401定向Go/chat全包/go build与API测试通过；388全量前端test/build/render通过，补new/open/card正断言后的render3同样PASS。两个装置脚本仅做PowerShell AST parse与node --check，未启栈，不以syntax通过声称真实旅程已通过。最终组合394/402窗口行为需另run重验。

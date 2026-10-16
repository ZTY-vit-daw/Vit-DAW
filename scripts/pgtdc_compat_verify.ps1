# PLUGGRABBER-TIMEDYN-COMPAT-1 真栈验证（worktree 专用，随 port 分支）
#
# 场景=Waves+FabFilter 时域件批量兼容 pass 验证（AGENTS §5 端侧烟测门槛）：
#   (1) 兼容矩阵：在场目标件（FabFilter Pro-R/Timeless 3 + Waves 壳内时域件
#       + ValhallaSupermassive 顺手项）逐件真实参数面 + inspect_reverb/
#       inspect_delay 判定（可证/拒绝+原因），零遗漏或原因行；
#   (2) Abbey Road Plates 抽验：inspect_reverb 放行（tail_evidence=
#       plate_damper_family，confidence 0.90）+ Damper 受控写入落地
#       （apply actual_readback + 独立参数 readback 在卷）；
#   (3) FabFilter Pro-R 抽验：inspect_reverb 放行（direct_wording）+ 一例
#       受控写入 readback 在卷。
#
# AGENTS §8 声明（运行前写定，不得事后改）：
#   LLM 轮次：0（全确定性工具面：plugin_search/load/inspect/apply/参数读）。
#   成功条件：矩阵目标件全覆盖（captured 或原因行）+ 两抽验写入 readback
#   变化 + exit 0。
#   失败分类：environment_interrupt(栈/端口故障，不计有效轮)/assertion_fail(
#   inspect 语义或写入 readback 不符)/capability_blocked(装载失败=许可或壳
#   枚举问题→按停止条件拆卡，不硬凑)。
#   止损线：同签名 assertion_fail 连续 2 次 → 停止上交，禁止原样重跑。
#
# 用法（仓库根=本卡 worktree D:\Vit_DAW_wt_pgtdc_compat）：
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\pgtdc_compat_verify.ps1
#   可选 -SkipBuild（须先确认 agent\bin\VitAgent.exe 含本卡改动）

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [int]$WaitSeconds = 90,
    [switch]$SkipBuild,
    [switch]$MatrixOnly
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($RepoRoot)) { $RepoRoot = (Get-Location).Path }
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$RunStamp = Get-Date -Format "yyyyMMdd_HHmmss"
$RunRoot = Join-Path $RepoRoot ("coord\runs\PLUGGRABBER-TIMEDYN-COMPAT-1\pgtdc_" + $RunStamp)
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$report = [ordered]@{
    card = "PLUGGRABBER-TIMEDYN-COMPAT-1"
    leg = "compat-matrix-verify"
    run_id = $RunStamp
    repo_head = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim()
    branch = (git -C $RepoRoot rev-parse --abbrev-ref HEAD 2>$null | Out-String).Trim()
    started_at = (Get-Date).ToString("o")
    sec8_decl = "zero LLM (deterministic tool faces only); success=full matrix coverage + both spot write readbacks changed + exit 0; failure classes environment_interrupt/assertion_fail/capability_blocked; stop-loss 2 same-signature assertion_fail"
}

function Write-Step([string]$m) { Write-Host ("[pgtdc] " + $m) -ForegroundColor Cyan }
function Write-Ok([string]$m)   { Write-Host ("  ok: " + $m) -ForegroundColor Green }
function Write-Warn2([string]$m){ Write-Host ("  warn: " + $m) -ForegroundColor Yellow }
function Write-Fail([string]$m) { Write-Host ("  FAIL: " + $m) -ForegroundColor Red }
function Save-Json($o, [string]$name) {
    $o | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath (Join-Path $RunRoot $name) -Encoding UTF8
}
function Test-StackGone {
    $out = @()
    foreach ($p in 7878, 5555, 5556, 8787) {
        if (Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue) { $out += "port $p listening" }
    }
    foreach ($n in "VitApp", "VitAgent", "vsphub", "godot") {
        $procs = Get-Process -Name $n -ErrorAction SilentlyContinue
        if ($procs) { $out += ("process " + $n + " pid=" + ($procs.Id -join ",")) }
    }
    return $out
}
function Get-StackPids {
    $pids = @{}
    foreach ($n in "VitApp", "VitAgent") {
        $procs = Get-Process -Name $n -ErrorAction SilentlyContinue
        if ($procs) { $pids[$n] = @($procs.Id) }
    }
    return $pids
}

try {
# ---------------- phase 0: clear field
Write-Step "Phase 0: clear-field preflight"
$residual = @(Test-StackGone)
if ($residual.Count -gt 0) {
    foreach ($r in $residual) { Write-Fail $r }
    $report.verdict = "stack_occupied"; $report.environment_interrupt = ($residual -join "; ")
    $report.finished_at = (Get-Date).ToString("o"); Save-Json $report "run_report.json"; exit 2
}
Write-Ok "ports 7878/5555/5556/8787 free, no stack processes"

# ---------------- phase 1: bring up stack via dev_agent_smoke (kernel + agent, no UI)
Write-Step "Phase 1: stack bring-up (dev_agent_smoke -StartKernel, no UI, worktree build)"
$KernelExe = "D:\Vit_DAW\Export\staging\runtime\VitApp.exe"
if (-not (Test-Path -LiteralPath $KernelExe)) { throw ("kernel exe missing: " + $KernelExe) }
$report.kernel_exe = $KernelExe
$report.kernel_sha256 = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
$devSmoke = Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"
$devParams = @{ StartKernel = $true; NoChatSmoke = $true; NoStripSilenceSmoke = $true; Strict = $true; WaitSeconds = $WaitSeconds; RepoRoot = $RepoRoot; KernelExe = $KernelExe }
if ($SkipBuild) { $devParams["SkipBuild"] = $true }
$devConsoleLog = Join-Path $RunRoot "dev_agent_smoke_console.log"
& $devSmoke @devParams 2>&1 | Tee-Object -FilePath $devConsoleLog | Out-Null
if ($LASTEXITCODE -ne 0) { throw ("dev_agent_smoke bring-up failed exit=" + $LASTEXITCODE + " (see dev_agent_smoke_console.log)") }
$stackPids = Get-StackPids
$report.stack_pids = $stackPids
try { $report.agent_sha256 = (Get-FileHash -LiteralPath (Join-Path $RepoRoot "agent\bin\VitAgent.exe") -Algorithm SHA256).Hash } catch { $report.agent_sha256 = "unavailable" }
Write-Ok ("stack up pids=" + (($stackPids.GetEnumerator() | ForEach-Object { $_.Key + "=" + ($_.Value -join ",") }) -join " "))

# agent health
$state = $null
$healthDeadline = (Get-Date).AddSeconds(45)
while ((Get-Date) -lt $healthDeadline -and ($null -eq $state -or [string]$state.status -ne "ok")) {
    try { $state = Invoke-RestMethod -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10 } catch { Start-Sleep -Seconds 3 }
    if ($null -eq $state) { Start-Sleep -Seconds 3 }
}
if ($null -eq $state -or [string]$state.status -ne "ok") { throw "agent health check failed" }
Write-Ok ("agent ok tool_count=" + [string]$state.tool_count)

# ---------------- phase 2: matrix + spot driver (zero LLM)
Write-Step "Phase 2: compat matrix + spot driver (pgtdc_compat_matrix.py)"
$driverArgs = @((Join-Path $RepoRoot "scripts\pgtdc_compat_matrix.py"),
    "--repo-root", $RepoRoot, "--agent-http", $AgentHttp, "--output-dir", $RunRoot)
if ($MatrixOnly) { $driverArgs += "--skip-spot" }
$driverLog = Join-Path $RunRoot "pgtdc_matrix_console.log"
& python @driverArgs 2>&1 | Tee-Object -FilePath $driverLog | Out-Null
$report.driver_exit = $LASTEXITCODE
$report.run_root = $RunRoot
if ($LASTEXITCODE -ne 0) { throw ("matrix driver failed exit=" + $LASTEXITCODE + " (see pgtdc_matrix_console.log / matrix_summary.json)") }
Write-Ok "matrix driver exit 0"
$report.verdict = "compat_verify_pass"
}
catch {
    $report.verdict = "compat_verify_fail"
    $report.error = $_.Exception.Message
    try { $_ | Out-String | Set-Content -LiteralPath (Join-Path $RunRoot "script_exception.txt") -Encoding UTF8 } catch { }
}
finally {
    # ---------------- teardown
    try {
        $pids = Get-StackPids
        foreach ($name in $pids.Keys) {
            foreach ($p in $pids[$name]) { try { Stop-Process -Id $p -Force -ErrorAction Stop } catch { } }
        }
        Start-Sleep -Seconds 3
        $remain = @(Test-StackGone)
        $report.teardown = @{ remaining = $remain; torn_down = ($remain.Count -eq 0) }
        if ($remain.Count -eq 0) { Write-Ok "stack torn down (ports free, processes gone)" }
        else { Write-Warn2 ("teardown residue: " + ($remain -join "; ")) }
    } catch { }
    $report.finished_at = (Get-Date).ToString("o")
    Save-Json $report "run_report.json"
    Write-Host ("[pgtdc] run_root: " + $RunRoot)
    Write-Host ("[pgtdc] verdict: " + $report.verdict)
}
if ($report.verdict -eq "compat_verify_pass") { exit 0 } else { exit 1 }

<#
SEG_PRIMITIVES real-stack smoke (card L2-2-SEG-SMOKE-1).

Proves the SEG-1 SegmentationPrimitives feature on the real three-piece stack
(VitApp kernel + Godot UI + Go agent) against the freshly built kernel:

  1. bring the stack up via dev_agent_smoke.ps1 -StartKernel -StartUI with an
     EXPLICIT -KernelExe pointing at the incrementally built
     cmake-build-pcverify1 Release binary (Export/staging demo binaries are
     never touched), with the kernel/UI wait budget widened to >= 60s
     (TIM-SMOKE lesson: cold plugin-table load can exceed the 20s default);
  2. run scripts/seg_primitives_probe.py against the live stack: it imports
     the real stems training folder through the same command chain as
     project_stems_import_probe.py, then requests two segmentation_primitives
     bakes of the same imported stem file and persists the published payloads;
  3. four assertion groups (all must pass for exit 0):
     A1 dad_l3_segmentation_primitives.v1 payload present for both rounds,
     A2 three sub-structures legal (onset counters / density frames /
        novelty frames + summaries),
     A3 two-round payloads byte-identical excluding updated_at/request_id,
     A4 exit 0 (this script's own exit code).

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\seg_primitives_smoke.ps1
  powershell ... -ReuseStack    # stack already running (kernel+UI+agent)
  powershell ... -SkipBuild     # reuse the installed agent binary

Exit codes: 0 = PASS, 1 = assertion/environment failure, 2 = stack bring-up
failure or stack occupied. Artifacts land under
coord\runs\L2-2-SEG-SMOKE-1\seg_primitives_smoke_<stamp>\ (new dir per run).
#>

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$KernelExe = "",
    [string]$TrainingFolder = "E:\BaiduNetdiskDownload\yingge - sattelites tracks out",
    [string]$PythonExe = "python",
    [string]$ZmqReqPort = "5555",
    [string]$ZmqSubPort = "5556",
    # TIM-SMOKE lesson: the kernel cold start (994-entry plugin table) can
    # exceed dev_agent_smoke's 20s default UI/port budget - keep >= 60s.
    [int]$WaitSeconds = 90,
    [int]$BakeTimeoutSeconds = 900,
    [int]$BackgroundAnalysisTimeoutSeconds = 600,
    [int]$BackgroundQuietTimeoutSeconds = 300,
    [int]$MergeWindowSeconds = 30,
    # 0 imports the whole training folder (61 stems queue one serialized L3
    # bake each and starve the probe bakes for tens of minutes); a small
    # staged subset keeps the same command chain with a bounded queue.
    [int]$MaxImportFiles = 3,
    [int]$ImportReqTimeoutMs = 300000,
    [switch]$SkipBuild,
    [switch]$ReuseStack,
    [switch]$KeepStack
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) {
        return (Resolve-Path -LiteralPath $Explicit).Path
    }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

function Write-Step { param([string]$Message) Write-Host "" ; Write-Host ("== " + $Message + " ==") -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("   [ok] " + $Message) -ForegroundColor Green }
function Write-WarnLine { param([string]$Message) Write-Host ("   [warn] " + $Message) -ForegroundColor Yellow }
function Write-FailLine { param([string]$Message) Write-Host ("   [FAIL] " + $Message) -ForegroundColor Red }

function Get-OptionalProperty {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $adapted = $Object -as [System.Management.Automation.PSObject]
    if ($null -ne $adapted) {
        foreach ($prop in $adapted.PSObject.Properties) {
            if ($prop.Name -eq $Name) { return $prop.Value }
        }
    }
    return $null
}

# Tear down the stack this run owns (kernel via REQ port, agent via HTTP port,
# the Godot frontend by its project path). Best effort: never masks the verdict.
function Clear-OwnedStack {
    foreach ($port in @(@{ port = 7878; label = "agent" }, @{ port = 5555; label = "kernel" })) {
        try {
            $listener = Get-NetTCPConnection -LocalPort $port.port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($null -ne $listener) {
                Stop-Process -Id $listener.OwningProcess -Force -ErrorAction SilentlyContinue
                Write-Host ("   [cleanup] stopped " + $port.label + " pid=" + $listener.OwningProcess)
            }
        }
        catch { Write-WarnLine ("cleanup " + $port.label + " failed: " + $_.Exception.Message) }
    }
    try {
        $godotProcs = Get-CimInstance Win32_Process -Filter "Name LIKE 'Godot%'" -ErrorAction SilentlyContinue |
            Where-Object { $_.CommandLine -like "*vit-daw-frontend*" }
        foreach ($proc in @($godotProcs)) {
            Stop-Process -Id $proc.ProcessId -Force -ErrorAction SilentlyContinue
            Write-Host ("   [cleanup] stopped godot pid=" + $proc.ProcessId)
        }
    }
    catch { Write-WarnLine ("cleanup godot failed: " + $_.Exception.Message) }
}

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    # SEG-1 lives only in the incrementally built pcverify1 tree; the staging
    # demo binary is deliberately NOT a candidate (card constraint).
    $KernelExe = Join-Path $RepoRoot "VitApp\cmake-build-pcverify1\VitApp_artefacts\Release\VitApp.exe"
}
$KernelExe = (Resolve-Path -LiteralPath $KernelExe).Path
if (-not (Test-Path -LiteralPath $KernelExe)) {
    throw "kernel exe not found: $KernelExe"
}
$TrainingFolder = (Resolve-Path -LiteralPath $TrainingFolder).Path

$KernelSha256 = ""
try { $KernelSha256 = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash } catch { $KernelSha256 = "unavailable" }
$KernelMtime = (Get-Item -LiteralPath $KernelExe).LastWriteTimeUtc.ToString("o")

$RunStamp = Get-Date -Format "yyyyMMdd_HHmmss"
$RunRoot = Join-Path $RepoRoot ("coord\runs\L2-2-SEG-SMOKE-1\seg_primitives_smoke_" + $RunStamp)
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$TempProjectDir = Join-Path $RunRoot "temp_project"
New-Item -ItemType Directory -Force -Path $TempProjectDir | Out-Null
$ProbeOutput = Join-Path $RunRoot "seg_probe_summary.json"
$ProbeLog = Join-Path $RunRoot "seg_probe_run.log"

$failureReasons = New-Object System.Collections.Generic.List[string]
function Add-Failure { param([string]$Message) $script:failureReasons.Add($Message); Write-FailLine $Message }

$report = [ordered]@{
    run_id = "seg_primitives_smoke_" + $RunStamp
    card = "L2-2-SEG-SMOKE-1"
    started_at = (Get-Date).ToUniversalTime().ToString("o")
    command_line = ($MyInvocation.Line)
    repo_head = ""
    kernel_exe = $KernelExe
    kernel_sha256 = $KernelSha256
    kernel_mtime_utc = $KernelMtime
    stack_mode = ""
    probe_output = $ProbeOutput
    assertions_block = $null
    verdict = ""
}
try { $report.repo_head = (git -C $RepoRoot rev-parse HEAD 2>$null | Out-String).Trim() } catch { $report.repo_head = "unavailable" }

function Write-Report {
    $report | ConvertTo-Json -Depth 10 | Out-File -FilePath (Join-Path $RunRoot "run_report.json") -Encoding utf8
}

Write-Step "SEG primitives real-stack smoke"
Write-Host ("run_root: " + $RunRoot)
Write-Host ("repo HEAD: " + $report.repo_head)
Write-Host ("kernel: " + $KernelExe)
Write-Host ("kernel sha256: " + $KernelSha256)
Write-Host ("kernel mtime(utc): " + $KernelMtime)

# ---------------------------------------------------------------- stack bring-up
if (-not $ReuseStack) {
    # AGENTS.md section 9 single-owner rule: refuse to build on a stack that
    # may belong to another session.
    $agentListener = Get-NetTCPConnection -LocalPort 7878 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    $kernelListener = Get-NetTCPConnection -LocalPort ([int]$ZmqReqPort) -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $agentListener -or $null -ne $kernelListener) {
        Write-FailLine ("stack already running (agent pid=" + $(if ($agentListener) { $agentListener.OwningProcess } else { "-" }) + " kernel pid=" + $(if ($kernelListener) { $kernelListener.OwningProcess } else { "-" }) + "); clean it up or pass -ReuseStack")
        $report.verdict = "stack_occupied"
        Write-Report
        exit 2
    }
    Write-Step "Bring up real stack via dev_agent_smoke (kernel + Godot UI + agent)"
    $devSmoke = Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"
    $devParams = @{
        StartKernel = $true
        StartUI = $true
        NoChatSmoke = $true
        NoStripSilenceSmoke = $true
        Strict = $true
        WaitSeconds = $WaitSeconds
        RepoRoot = $RepoRoot
        KernelExe = $KernelExe
    }
    if ($SkipBuild) { $devParams["SkipBuild"] = $true }
    $devExit = 0
    try {
        & $devSmoke @devParams
        if (-not $?) { $devExit = 1 }
    }
    catch {
        $devExit = 1
        Write-FailLine ("dev_agent_smoke failed: " + $_.Exception.Message)
    }
    if ($devExit -ne 0) {
        Write-FailLine "dev_agent_smoke stack bring-up failed"
        $report.verdict = "stack_bringup_failed"
        Write-Report
        if (-not $KeepStack) { Clear-OwnedStack }
        exit 2
    }
    $report.stack_mode = "dev_agent_smoke_started"
}
else {
    $report.stack_mode = "reused_existing_stack"
}

# ---------------------------------------------------------------- agent health
Write-Step "Agent health"
$state = $null
$healthDeadline = (Get-Date).AddSeconds(45)
while ((Get-Date) -lt $healthDeadline) {
    try {
        $state = Invoke-RestMethod -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10
        if ($null -ne $state -and [string](Get-OptionalProperty -Object $state -Name "status") -eq "ok") { break }
    }
    catch { Start-Sleep -Seconds 3 }
    if ($null -eq $state) { Start-Sleep -Seconds 3 }
}
if ($null -eq $state -or [string](Get-OptionalProperty -Object $state -Name "status") -ne "ok") {
    Add-Failure "GET /agent/state did not return ok (agent not reachable)"
}
else {
    Write-Ok ("agent ok tool_count=" + [string](Get-OptionalProperty -Object $state -Name "tool_count"))
}

# ---------------------------------------------------------------- probe (stems import + two bakes + A1/A2/A3)
Write-Step "Run seg_primitives_probe (stems import + two segmentation_primitives bakes)"
$probe = Join-Path $RepoRoot "scripts\seg_primitives_probe.py"
$probeArgs = @(
    $probe,
    "--req-url", ("tcp://127.0.0.1:" + $ZmqReqPort),
    "--sub-url", ("tcp://127.0.0.1:" + $ZmqSubPort),
    "--training-folder", $TrainingFolder,
    "--project-path", (Join-Path $TempProjectDir "seg_primitives_smoke.vit"),
    "--output", $ProbeOutput,
    "--bake-timeout-sec", ([string]$BakeTimeoutSeconds),
    "--background-analysis-timeout-sec", ([string]$BackgroundAnalysisTimeoutSeconds),
    "--background-quiet-timeout-sec", ([string]$BackgroundQuietTimeoutSeconds),
    "--merge-window-sec", ([string]$MergeWindowSeconds),
    "--max-import-files", ([string]$MaxImportFiles),
    "--req-timeout-ms", ([string]$ImportReqTimeoutMs)
)
& $PythonExe @probeArgs *>&1 | Tee-Object -FilePath $ProbeLog
$probeExit = $LASTEXITCODE
$report.probe_exit_code = $probeExit
if ($probeExit -ne 0) {
    Add-Failure ("seg_primitives_probe.py exited " + $probeExit + "; log=" + $ProbeLog)
}

# ---------------------------------------------------------------- assertion review (A1-A4)
Write-Step "Review assertion groups A1-A4"
# Review must never throw past this point: run 20260926_224515 died inside
# ConvertFrom-Json (ANSI read of UTF-8) and skipped both the FAIL verdict and
# the stack cleanup. Keep every step non-throwing and record what happened.
$a1 = $false; $a2 = $false; $a3 = $false
if (Test-Path -LiteralPath $ProbeOutput) {
    try {
        # PS 5.1 Get-Content defaults to ANSI on this codepage and mangles the
        # UTF-8 probe JSON; read as UTF-8 explicitly.
        $probeReport = Get-Content -LiteralPath $ProbeOutput -Raw -Encoding UTF8 | ConvertFrom-Json
    }
    catch {
        Add-Failure ("probe summary unreadable: " + $_.Exception.Message)
        $probeReport = $null
    }
    if ($null -ne $probeReport) {
        $report.assertions_block = Get-OptionalProperty -Object $probeReport -Name "assertions"
        $report.probe_status = [string](Get-OptionalProperty -Object $probeReport -Name "status")
        $report.probe_error = [string](Get-OptionalProperty -Object $probeReport -Name "error")
        $report.readable_file_count = Get-OptionalProperty -Object $probeReport -Name "readable_file_count"
        $report.tracks_created = Get-OptionalProperty -Object $probeReport -Name "tracks_created"
        $report.observed_publish_traffic = Get-OptionalProperty -Object $probeReport -Name "observed_publish_traffic"
        $report.import_folder = [string](Get-OptionalProperty -Object $probeReport -Name "import_folder")
        $assertions = Get-OptionalProperty -Object $probeReport -Name "assertions"
        $a1 = [bool](Get-OptionalProperty -Object $assertions -Name "A1_payload_present")
        $a2 = [bool](Get-OptionalProperty -Object $assertions -Name "A2_structure_legal")
        $a3 = [bool](Get-OptionalProperty -Object $assertions -Name "A3_determinism")
        if (-not $a1) { Add-Failure "A1 payload present: FAIL (see probe failures)" }
        else { Write-Ok "A1 dad_l3_segmentation_primitives.v1 payload present in both rounds" }
        if (-not $a2) { Add-Failure "A2 three sub-structures: FAIL (see probe failures)" }
        else { Write-Ok "A2 onset_events/onset_density/energy_novelty structures legal" }
        if (-not $a3) { Add-Failure "A3 two-round determinism: FAIL (functional red line)" }
        else {
            Write-Ok ("A3 two-round payloads byte-identical excluding updated_at/request_id (canonical bytes=" + [string](Get-OptionalProperty -Object $probeReport -Name "canonical_bytes") + ")")
        }
    }
}
else {
    Add-Failure ("probe summary missing: " + $ProbeOutput)
}

$report.finished_at = (Get-Date).ToUniversalTime().ToString("o")
if ($failureReasons.Count -eq 0 -and $probeExit -eq 0) {
    # A4: exit 0 is this script's own exit code.
    $report.verdict = "PASS"
    Write-Report
    Write-Step "PASS"
    Write-Ok ("SEG primitives real-stack smoke passed (four assertion groups). run_root=" + $RunRoot)
    if (-not $KeepStack) { Clear-OwnedStack }
    exit 0
}
$report.verdict = "FAIL"
$report.failures = @($failureReasons)
Write-Report
Write-Step "FAIL"
foreach ($reason in $failureReasons) { Write-FailLine $reason }
Write-Host ("run_root=" + $RunRoot)
if (-not $KeepStack) { Clear-OwnedStack }
exit 1

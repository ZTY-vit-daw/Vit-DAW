<#
GEN-YUE-DIALECT-1 real-stack smoke (card 278, registration chain #3):
yue_local dialect on the real VitApp stack (kernel + agent, HTTP chat face)
plus the local YuE2 engine (127.0.0.1:8087) as an INDEPENDENT process.
Script-stack mode follows reply_gen_toolgate_smoke.ps1; assertion body lives
in scripts/yue_dialect_smoke.py (leg0 registration / leg1 chat-driven
generation / leg2 a-cappella re-verification with SheetSage2 readback).

Engine ownership note: the YuE2 engine is started and stopped BY THIS SCRIPT
around the driver run and is recorded in the run artifacts only — it is NOT
registered in coord/resources/PC-RUNTIME-STACK.md (that record governs the
VitApp/Godot/agent three-piece stack). The VitApp stack itself follows the
normal stack protocol: the operator registers occupancy in
PC-RUNTIME-STACK.md before invoking this script (six-step: clean-check,
register+push, re-check <=60s, start, test, TEARDOWN_ALL_ZERO).

Probabilistic-run protocol (AGENTS.md §8, declared before execution):
  runs        : 1 full-chain attempt. Generation legs are never retried
                in-run; a second full attempt only for a classified
                environment interruption (port busy / engine start failure /
                kernel start failure), never a blind rerun of a functional
                failure.
  LLM budget  : leg1 is the only LLM-participating leg — one chat goal
                (single-turn tool plan + final reply). Estimated cost is the
                usual one-goal magnitude (< CNY 0.1). Zero LLM in leg0/leg2.
  wall clock  : two generations at 21-40s each (60s song, ~0.55x realtime,
                YUE2-PC-DEPLOY-1) + SheetSage2 transcription (~30-60s) +
                chat-chain latency; budget below is the stop-loss ceiling.
  success     : exit 0 — all legs pass (see yue_dialect_smoke.py header).
  fail classes: leg0_registration, leg1_chat_goal, leg1_no_artifacts,
                leg1_bad_artifacts, leg2_invoke, leg2_transcribe,
                leg2_instrumental_detected, leg2_shape_mismatch, timeout,
                env_failure (ports busy, engine unreachable, kernel/agent
                start failure), vram_over_budget (>6.3GB engine peak —
                environment interruption, escalate, do not rerun).
  stop-loss   : same-shape failure twice across runs -> stop, keep artifacts,
                escalate to the decision side. No assertion weakening.

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_yue_dialect_smoke.ps1 `
      -KernelExe D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe
  (Run from the implementation worktree; the agent binary is built from the
   worktree so it contains the yue_local dialect. The kernel is a prebuilt
   unchanged binary — hash and mtime are recorded per AGENTS §9.)
#>

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = "",
    [switch]$SkipBuild,
    [string]$AgentBin = "",
    [string]$KernelExe = "",
    [string]$EngineRoot = "E:\AI\YuE2",
    [int]$WaitSeconds = 30,
    [int]$KernelDwellSeconds = 30,
    [int]$EngineStartBudgetSeconds = 300,
    [int]$ChatBudgetSeconds = 420,
    [int]$Leg2InvokeTimeoutSeconds = 300,
    [int]$Leg2JobBudgetSeconds = 300,
    [switch]$SkipLeg1,
    [double]$VramPeakLimitMiB = 6451  # 6.3GB in MiB, YUE2-PC-DEPLOY-1 measured peak ceiling
)

$ErrorActionPreference = "Stop"

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) { return (Resolve-Path -LiteralPath $Explicit).Path }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}
function Write-Step { param([string]$Message) Write-Host ""; Write-Host ("== " + $Message) -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("ok: " + $Message) -ForegroundColor Green }
function Write-Bad { param([string]$Message) Write-Host ("fail: " + $Message) -ForegroundColor Red }

function Get-TcpListener { param([int]$Port) return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1 }
function Wait-PortListen {
    param([int]$Port, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Get-TcpListener -Port $Port) { return $true }
        Start-Sleep -Milliseconds 500
    }
    return $false
}
function Wait-HttpReady {
    param([string]$BaseUrl, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri ($BaseUrl.TrimEnd("/") + "/health") -TimeoutSec 3
            if ($resp.StatusCode -eq 200) { return $true }
        } catch { }
        Start-Sleep -Milliseconds 1000
    }
    return $false
}
function Get-VramUsedMiB {
    try {
        $line = (& nvidia-smi --query-gpu=memory.used --format=csv,noheader,nounits | Select-Object -First 1)
        return [double]::Parse(([string]$line).Trim())
    } catch { return -1 }
}

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("coord\runs\GEN-YUE-DIALECT-1\run_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$DownloadDir = Join-Path $RunRoot "downloads"
New-Item -ItemType Directory -Force -Path $DownloadDir | Out-Null
$AgentDrafts = Join-Path $RunRoot "agent_drafts"
New-Item -ItemType Directory -Force -Path $AgentDrafts | Out-Null
if ([string]::IsNullOrWhiteSpace($AgentBin)) { $AgentBin = Join-Path $RunRoot "bin\VitAgent.yuedialect.exe" }
$AgentLog = Join-Path $RunRoot "agent_last.log"
$AgentDir = Join-Path $RepoRoot "agent"
$GenapiConfig = Join-Path $RunRoot "genapi_providers.json"
$PrereqPath = Join-Path $RunRoot "prereq.txt"
$VramLog = Join-Path $RunRoot "vram_samples.log"
if ([string]::IsNullOrWhiteSpace($KernelExe)) { $KernelExe = "D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe" }

$prereq = New-Object System.Collections.Generic.List[string]
function Add-Prereq { param([string]$Line) $script:prereq.Add($Line); Write-Host $Line }

$kernelProcId = $null
$agentProcId = $null
$engineProcId = $null
$vramJob = $null
$outcome = "env_failure"
$driverExit = $null
$detail = ""
$vramPeakMiB = -1

try {
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("head=" + (& git -C $RepoRoot rev-parse HEAD))
    Add-Prereq ("git_status=" + ((& git -C $RepoRoot status --short) -join " ; "))

    Write-Step "Prereq: ports 7878/5555/5556 (VitApp stack) and 8087 (YuE2 engine) must be free"
    foreach ($port in 7878, 5555, 5556, 8087) {
        if (Get-TcpListener -Port $port) { throw ("port " + $port + " is already listening; aborting before any start (stack ownership rule)") }
        Add-Prereq ("port_free=" + $port)
    }

    Write-Step "VRAM precheck (desktop baseline; engine needs ~3.4GB, task peak ceiling 6.3GB)"
    $vramBefore = Get-VramUsedMiB
    Add-Prereq ("vram_before_mib=" + $vramBefore)

    if (-not $SkipBuild) {
        Write-Step "Build VitAgent from the worktree (must contain the yue_local dialect)"
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $AgentBin) | Out-Null
        Push-Location $AgentDir
        try {
            & go build -o $AgentBin .\cmd\vitagent
            if ($LASTEXITCODE -ne 0) { throw ("go build failed with exit code " + $LASTEXITCODE) }
        }
        finally { Pop-Location }
    }
    elseif (-not (Test-Path -LiteralPath $AgentBin -PathType Leaf)) { throw "-SkipBuild given but isolated agent binary missing" }
    $agentItem = Get-Item -LiteralPath $AgentBin
    Add-Prereq ("agent_binary=" + $AgentBin + " size=" + $agentItem.Length + " mtime=" + $agentItem.LastWriteTime.ToString("o"))

    Write-Step "Run-scoped genapi provider config (never touches ~/.vit)"
    $configJSON = @'
{
  "providers": [
    {
      "id": "yue-local",
      "dialect": "yue_local",
      "base_url": "http://127.0.0.1:8087",
      "api_key": "",
      "download_dir": "__DOWNLOAD_DIR__",
      "extra_params": { "seed": 11 }
    }
  ]
}
'@ -replace "__DOWNLOAD_DIR__", ($DownloadDir -replace "\\", "\\")
    # BOM-less UTF8: Go json.Unmarshal rejects a leading BOM.
    [System.IO.File]::WriteAllText($GenapiConfig, $configJSON, (New-Object System.Text.UTF8Encoding($false)))
    Add-Prereq ("genapi_config=" + $GenapiConfig)

    Write-Step "Start kernel (prebuilt unchanged binary; hash recorded per AGENTS 9)"
    if (-not (Test-Path -LiteralPath $KernelExe -PathType Leaf)) { throw ("kernel binary missing: " + $KernelExe) }
    $kernelItem = Get-Item -LiteralPath $KernelExe
    Add-Prereq ("kernel_binary=" + $KernelExe + " size=" + $kernelItem.Length + " mtime=" + $kernelItem.LastWriteTime.ToString("o") + " sha256=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $KernelExe).Hash)
    $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -WindowStyle Hidden -PassThru
    $kernelProcId = $kernelProc.Id
    Add-Prereq ("kernel_pid=" + $kernelProcId)
    if (-not (Wait-PortListen -Port 5555 -TimeoutSeconds $WaitSeconds)) { throw "kernel ZMQ REQ port 5555 did not listen" }
    Write-Ok ("kernel up, pid=" + $kernelProcId)

    Write-Step "Start agent (isolated draft root + run-scoped genapi config)"
    $priorDraftRoot = [Environment]::GetEnvironmentVariable("VIT_HISTORY_DRAFT_ROOT", "Process")
    $priorDevRoot = [Environment]::GetEnvironmentVariable("VIT_DAW_DEV_ROOT", "Process")
    $priorGenapi = [Environment]::GetEnvironmentVariable("VIT_GENAPI_CONFIG", "Process")
    $env:VIT_HISTORY_DRAFT_ROOT = $AgentDrafts
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    $env:VIT_GENAPI_CONFIG = $GenapiConfig
    try {
        $agentProc = Start-Process -FilePath $AgentBin -ArgumentList @("-http", "127.0.0.1:7878", "-last-log-path", $AgentLog, "-keep-last-log-lines", "9000") -WorkingDirectory $AgentDir -WindowStyle Hidden -PassThru
    }
    finally {
        if ($null -eq $priorDraftRoot) { Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue } else { $env:VIT_HISTORY_DRAFT_ROOT = $priorDraftRoot }
        if ($null -eq $priorDevRoot) { Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue } else { $env:VIT_DAW_DEV_ROOT = $priorDevRoot }
        if ($null -eq $priorGenapi) { Remove-Item Env:VIT_GENAPI_CONFIG -ErrorAction SilentlyContinue } else { $env:VIT_GENAPI_CONFIG = $priorGenapi }
    }
    $agentProcId = $agentProc.Id
    Add-Prereq ("agent_pid=" + $agentProcId)
    if (-not (Wait-HttpReady -BaseUrl "http://127.0.0.1:7878" -TimeoutSeconds $WaitSeconds)) { throw "agent HTTP did not become ready" }
    Write-Ok ("agent up, pid=" + $agentProcId)

    if ($KernelDwellSeconds -gt 0) {
        Write-Step ("Kernel warm-up dwell " + $KernelDwellSeconds + "s")
        Start-Sleep -Seconds $KernelDwellSeconds
    }

    Write-Step "Full project access (auto-apply; gen.generate is RiskConfirm)"
    $auth = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:7878/agent/authority" -ContentType "application/json" -Body (@{ authority_mode = "full_project_access" } | ConvertTo-Json -Compress) -TimeoutSec 30
    Add-Prereq ("authority_mode=" + $auth.authority_mode)
    if ($auth.authority_mode -ne "full_project_access") { throw ("authority switch refused: " + ($auth | ConvertTo-Json -Compress)) }

    Write-Step "Start YuE2 engine (independent process; self-managed per card 278)"
    # Launch yue-server DIRECTLY (verbatim mirror of E:\AI\YuE2\start_engine.ps1
    # arguments). Never `& powershell -File start_engine.ps1 | Out-Null`: the
    # engine child inherits the pipe write handle and the pipeline never sees
    # EOF — the wrapper blocks forever (attempt 1, 2026-11-07 19:26 run,
    # zero legs executed, teardown all-zero).
    $engineExe = Join-Path $EngineRoot "engine\yue-server.exe"
    if (-not (Test-Path -LiteralPath $engineExe -PathType Leaf)) { throw ("engine binary missing: " + $engineExe) }
    $engineLogs = Join-Path $EngineRoot "logs"
    New-Item -ItemType Directory -Force -Path $engineLogs | Out-Null
    $engineProc = Start-Process -FilePath $engineExe -ArgumentList @(
        '--model', (Join-Path $EngineRoot 'models\YuE2-3B-Q8_0.gguf'),
        '--vae', (Join-Path $EngineRoot 'models\YuE2-Vae-F32.gguf'),
        '--transcriber', (Join-Path $EngineRoot 'models\SheetSage2-Q8_0.gguf'),
        '--tokenizer', (Join-Path $EngineRoot 'models\yue2-mothersuperior-realaudio-tokenizer-v4-Q8_0.gguf'),
        '--host', '127.0.0.1', '--port', '8087', '--max-seq', '24576'
    ) -WorkingDirectory (Split-Path -Parent $engineExe) -WindowStyle Hidden `
        -RedirectStandardOutput (Join-Path $engineLogs "engine.log") `
        -RedirectStandardError (Join-Path $engineLogs "engine.err.log") -PassThru
    if ($engineProc.HasExited) { throw ("yue-server exited immediately with code " + $engineProc.ExitCode) }
    if (-not (Wait-HttpReady -BaseUrl "http://127.0.0.1:8087" -TimeoutSeconds $EngineStartBudgetSeconds)) { throw "YuE2 engine /health did not become ready" }
    $engineProcId = $engineProc.Id
    Add-Prereq ("engine_pid=" + $engineProcId)
    $vramEngineUp = Get-VramUsedMiB
    Add-Prereq ("vram_engine_up_mib=" + $vramEngineUp)
    Write-Ok ("engine up, pid=" + $engineProcId + " vram=" + $vramEngineUp + "MiB")

    Write-Step "VRAM sampler (2s interval) while the driver runs"
    $vramJob = Start-Job -ScriptBlock {
        param([string]$LogPath)
        $deadline = (Get-Date).AddMinutes(30)
        while ((Get-Date) -lt $deadline) {
            try {
                $line = (& nvidia-smi --query-gpu=memory.used,utilization.gpu --format=csv,noheader,nounits | Select-Object -First 1)
                Add-Content -LiteralPath $LogPath -Value ((Get-Date -Format "o") + " " + $line)
            } catch { }
            Start-Sleep -Seconds 2
        }
    } -ArgumentList $VramLog

    Write-Step "Run the smoke driver (leg0 registration / leg1 chat / leg2 a-cappella re-verify)"
    $driverArgs = @(
        "-I", (Join-Path $RepoRoot "scripts\yue_dialect_smoke.py"),
        "--agent-base", "http://127.0.0.1:7878",
        "--engine-base", "http://127.0.0.1:8087",
        "--run-root", $RunRoot,
        "--download-dir", $DownloadDir,
        "--chat-budget", [string]$ChatBudgetSeconds,
        "--leg2-invoke-timeout", [string]$Leg2InvokeTimeoutSeconds,
        "--leg2-job-budget", [string]$Leg2JobBudgetSeconds
    )
    if ($SkipLeg1) { $driverArgs += "--skip-leg1" }
    & python @driverArgs
    $driverExit = $LASTEXITCODE
    Add-Prereq ("driver_exit=" + $driverExit)

    if ($null -ne $vramJob) { Stop-Job -Job $vramJob -ErrorAction SilentlyContinue; Remove-Job -Job $vramJob -Force -ErrorAction SilentlyContinue; $vramJob = $null }
    if (Test-Path -LiteralPath $VramLog) {
        $peak = 0
        foreach ($line in (Get-Content -LiteralPath $VramLog)) {
            if ($line -match "\d{4}-\d{2}-\d{2}T.*?,(\d+)") { $v = [int]$Matches[1]; if ($v -gt $peak) { $peak = $v } }
        }
        $vramPeakMiB = $peak
        Add-Prereq ("vram_peak_mib=" + $vramPeakMiB)
        if ($vramPeakMiB -gt $VramPeakLimitMiB) {
            $outcome = "vram_over_budget"
            throw ("engine VRAM peak " + $vramPeakMiB + "MiB exceeds the 6.3GB ceiling — environment interruption, escalate")
        }
    }
    if ($driverExit -eq 0) { $outcome = "finished" } else { $outcome = "driver_failed" }
}
catch {
    $detail = $_.Exception.Message
    Add-Prereq ("fatal=" + $detail)
    Write-Bad ("fatal: " + $detail)
    if ($outcome -eq "env_failure" -and $detail -notlike "*port*" -and $detail -notlike "*listen*" -and $detail -notlike "*ready*" -and $detail -notlike "*vram*") { $outcome = "chain_failed" }
}
finally {
    Write-Step "Teardown (agent, kernel, YuE2 engine — all independent stops)"
    if ($agentProcId) { Stop-Process -Id $agentProcId -Force -ErrorAction SilentlyContinue; Add-Prereq ("stopped_agent_pid=" + $agentProcId) }
    if ($kernelProcId) { Stop-Process -Id $kernelProcId -Force -ErrorAction SilentlyContinue; Add-Prereq ("stopped_kernel_pid=" + $kernelProcId) }
    if ($null -ne $vramJob) { Stop-Job -Job $vramJob -ErrorAction SilentlyContinue; Remove-Job -Job $vramJob -Force -ErrorAction SilentlyContinue }
    # Engine stop: mirror of E:\AI\YuE2\stop_engine.ps1 (Stop-Process yue-server).
    $engineProc = Get-Process -Name yue-server -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($engineProc) { Stop-Process -Id $engineProc.Id -Force -ErrorAction SilentlyContinue; Add-Prereq ("stopped_engine_pid=" + $engineProc.Id) }
    Start-Sleep -Seconds 3
    foreach ($port in 7878, 5555, 5556, 8087) {
        if (Get-TcpListener -Port $port) { Add-Prereq ("port_still_busy=" + $port) } else { Add-Prereq ("port_released=" + $port) }
    }
    $vramAfter = Get-VramUsedMiB
    Add-Prereq ("vram_after_mib=" + $vramAfter)
    # Engine log slice into the run dir (engine.log is append-mode across runs).
    try {
        $engineLog = Join-Path $EngineRoot "logs\engine.log"
        if (Test-Path -LiteralPath $engineLog) { Copy-Item -LiteralPath $engineLog -Destination (Join-Path $RunRoot "engine_log_copy.txt") -Force }
    } catch { Add-Prereq ("engine_log_copy_failed=" + $_.Exception.Message) }
    Add-Prereq ("finished=" + (Get-Date -Format "o"))
    $prereq | Out-File -FilePath $PrereqPath -Encoding utf8

    $report = [ordered]@{
        schema_version = "run_yue_dialect_smoke.v1"
        run_root       = $RunRoot
        outcome        = $outcome
        driver_exit    = $driverExit
        detail         = $detail
        vram_peak_mib  = $vramPeakMiB
        prereq         = @($prereq)
    }
    $report | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $RunRoot "stack_report.json") -Encoding UTF8
}

Write-Host ""
Write-Host ("YUE_DIALECT_STACK_VERDICT " + $outcome + " driver_exit=" + $driverExit)
switch ($outcome) {
    "finished" { exit 0 }
    "vram_over_budget" { exit 2 }
    "env_failure" { exit 2 }
    default { exit 1 }
}

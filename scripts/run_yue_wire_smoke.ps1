<#
GEN-YUE-WIRE-1 DG1-C real-stack smoke (card 372, DG1-C per
docs/STACKGATE_DG1_DG3_V1.md §3.3): student score -> YuE2 arrangement
end-to-end on the real VitApp stack (kernel + agent berth form, no UI) with
the YuE2 engine brought up BY THE AGENT'S OWN engine supervision face (C7 —
the script deliberately does NOT prestart the engine).

Assertion body lives in scripts/yue_wire_smoke.py (legs W0-W9, DG1-C C1-C8).

Engine ownership note (mirrors run_yue_dialect_smoke.ps1): the YuE2 engine
is an INDEPENDENT process, started by the agent's engine supervisor (zero
manual service) and stopped by this script's teardown as a safety net; it is
NOT registered in coord/resources/PC-RUNTIME-STACK.md (that record governs
the VitApp/Godot/agent three-piece stack). The VitApp stack itself follows
the normal six-step stack protocol BEFORE invoking this script.

Probabilistic-run protocol (AGENTS.md §8 + STACKGATE §1.6, declared before
execution):
  runs        : 1 full-chain attempt. Real generations are never retried
                in-run; one rebuild re-attempt only for a classified
                environment interruption (port busy / kernel or agent start
                failure), never a blind rerun of a functional failure.
  generation  : exactly 2 (version v1 + v2, one job each — C5) + 1
                SheetSage2 transcription (C3 readback, probabilistic class
                per GEN-YUE-DIALECT-1 ruling B: pre-declared
                legW9_vocal_detected failure class, same-shape twice across
                runs -> stop, escalate).
  LLM budget  : ZERO — every leg drives the deterministic /agent/invoke
                tool face; no chat goal is ever created. The agent log is
                archived for the stayed-zero audit.
  wall clock  : 2 generations (21-60s each) + transcription (~30-60s) +
                engine cold start (~40-120s model load); stop-loss ceiling
                below (parameters).
  success     : exit 0 — all legs pass (see yue_wire_smoke.py header).
  fail classes: legW0_registration, legW1_fixture, legW2_start,
                legW2_status, legW7_engine_bringup, legW3_roundtrip,
                legW3_vocal_not_rested, legW4_style_not_mapped,
                legW4_hygiene, legW5_not_two_jobs, legW6_blocking,
                legW8_artifacts, legW9_transcribe, legW9_vocal_detected,
                timeout, env_failure (ports busy, engine unreachable after
                supervisor bring-up, kernel/agent start failure),
                vram_over_budget (>6451 MiB engine peak — environment
                interruption, escalate, do not rerun).
  stop-loss   : same-shape failure twice across runs -> stop, keep
                artifacts, escalate. No assertion weakening (STACKGATE §1.7).

Usage (run from the implementation worktree; the agent binary is built from
the worktree so it contains the GEN-YUE-WIRE-1 six-item wiring; the kernel
is a prebuilt unchanged binary — hash and mtime recorded per AGENTS §9):
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_yue_wire_smoke.ps1 `
      -KernelExe D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe
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
    [int]$KernelDwellSeconds = 20,
    [int]$EngineReadyBudgetSeconds = 300,
    [int]$JobBudgetSeconds = 420,
    [switch]$SkipLegW9,   # transcription readback leg (probabilistic class)
    [double]$VramPeakLimitMiB = 6451  # YUE2-PC-DEPLOY-1 measured ceiling
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
    $RunRoot = Join-Path $RepoRoot ("coord\runs\GEN-YUE-WIRE-1\run_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null
$DownloadDir = Join-Path $RunRoot "downloads"
New-Item -ItemType Directory -Force -Path $DownloadDir | Out-Null
$JobsDir = Join-Path $RunRoot "genapi_jobs"
New-Item -ItemType Directory -Force -Path $JobsDir | Out-Null
$AgentDrafts = Join-Path $RunRoot "agent_drafts"
New-Item -ItemType Directory -Force -Path $AgentDrafts | Out-Null
$EngineStdout = Join-Path $RunRoot "engine_supervisor_stdout.log"
$EngineStderr = Join-Path $RunRoot "engine_supervisor_stderr.log"
if ([string]::IsNullOrWhiteSpace($AgentBin)) { $AgentBin = Join-Path $RunRoot "bin\VitAgent.yuewire.exe" }
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
$vramJob = $null
$outcome = "env_failure"
$driverExit = $null
$detail = ""
$vramPeakMiB = -1

try {
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("head=" + (& git -C $RepoRoot rev-parse HEAD))
    Add-Prereq ("git_status=" + ((& git -C $RepoRoot status --short) -join " ; "))

    Write-Step "Prereq: ports 7878/5555/5556 (VitApp stack) and 8087 (YuE2 engine) must be free — engine must be DOWN so the agent supervisor bring-up is real (C7)"
    foreach ($port in 7878, 5555, 5556, 8087) {
        if (Get-TcpListener -Port $port) { throw ("port " + $port + " is already listening; aborting before any start (stack ownership rule)") }
        Add-Prereq ("port_free=" + $port)
    }

    Write-Step "VRAM precheck (desktop baseline; engine needs ~3.4GB, task peak ceiling 6451MiB)"
    $vramBefore = Get-VramUsedMiB
    Add-Prereq ("vram_before_mib=" + $vramBefore)

    if (-not $SkipBuild) {
        Write-Step "Build VitAgent from the worktree (must contain the GEN-YUE-WIRE-1 wiring)"
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

    Write-Step "Run-scoped genapi provider config: engine block (agent-supervised bring-up) + POLLUTING extra_params (C4 hygiene probe)"
    # The abc/style extra_params are deliberate pollution: the wiring must
    # protect the student score / user style from the fixed overlay. seed
    # stays a legal engine knob.
    $engineExe = Join-Path $EngineRoot "engine\yue-server.exe"
    if (-not (Test-Path -LiteralPath $engineExe -PathType Leaf)) { throw ("engine binary missing: " + $engineExe) }
    $dlWin = $DownloadDir -replace "\\", "\\"
    $cfg = @"
{
  "providers": [
    {
      "id": "yue-local",
      "dialect": "yue_local",
      "base_url": "http://127.0.0.1:8087",
      "api_key": "",
      "download_dir": "$dlWin",
      "extra_params": { "seed": 11, "abc": "POLLUTED-ABC", "style": "polluted-style" },
      "engine": {
        "command": "$($engineExe -replace '\\', '\\')",
        "args": [
          "--model", "$($EngineRoot -replace '\\', '\\')\\models\\YuE2-3B-Q8_0.gguf",
          "--vae", "$($EngineRoot -replace '\\', '\\')\\models\\YuE2-Vae-F32.gguf",
          "--transcriber", "$($EngineRoot -replace '\\', '\\')\\models\\SheetSage2-Q8_0.gguf",
          "--tokenizer", "$($EngineRoot -replace '\\', '\\')\\models\\yue2-mothersuperior-realaudio-tokenizer-v4-Q8_0.gguf",
          "--host", "127.0.0.1", "--port", "8087", "--max-seq", "24576"
        ],
        "working_dir": "$(($EngineRoot + '\engine') -replace '\\', '\\')",
        "stdout_log": "$($EngineStdout -replace '\\', '\\')",
        "stderr_log": "$($EngineStderr -replace '\\', '\\')",
        "ready_timeout_seconds": $EngineReadyBudgetSeconds,
        "poll_interval_seconds": 2
      }
    }
  ]
}
"@
    # BOM-less UTF8: Go json.Unmarshal rejects a leading BOM.
    [System.IO.File]::WriteAllText($GenapiConfig, $cfg, (New-Object System.Text.UTF8Encoding($false)))
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

    Write-Step "Start agent (isolated drafts + run-scoped genapi config + run-scoped jobs archive)"
    $priorDraftRoot = [Environment]::GetEnvironmentVariable("VIT_HISTORY_DRAFT_ROOT", "Process")
    $priorDevRoot = [Environment]::GetEnvironmentVariable("VIT_DAW_DEV_ROOT", "Process")
    $priorGenapi = [Environment]::GetEnvironmentVariable("VIT_GENAPI_CONFIG", "Process")
    $priorJobs = [Environment]::GetEnvironmentVariable("VIT_GENAPI_JOBS_DIR", "Process")
    $env:VIT_HISTORY_DRAFT_ROOT = $AgentDrafts
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    $env:VIT_GENAPI_CONFIG = $GenapiConfig
    $env:VIT_GENAPI_JOBS_DIR = $JobsDir
    try {
        $agentProc = Start-Process -FilePath $AgentBin -ArgumentList @("-http", "127.0.0.1:7878", "-last-log-path", $AgentLog, "-keep-last-log-lines", "9000") -WorkingDirectory $AgentDir -WindowStyle Hidden -PassThru
    }
    finally {
        if ($null -eq $priorDraftRoot) { Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue } else { $env:VIT_HISTORY_DRAFT_ROOT = $priorDraftRoot }
        if ($null -eq $priorDevRoot) { Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue } else { $env:VIT_DAW_DEV_ROOT = $priorDevRoot }
        if ($null -eq $priorGenapi) { Remove-Item Env:VIT_GENAPI_CONFIG -ErrorAction SilentlyContinue } else { $env:VIT_GENAPI_CONFIG = $priorGenapi }
        if ($null -eq $priorJobs) { Remove-Item Env:VIT_GENAPI_JOBS_DIR -ErrorAction SilentlyContinue } else { $env:VIT_GENAPI_JOBS_DIR = $priorJobs }
    }
    $agentProcId = $agentProc.Id
    Add-Prereq ("agent_pid=" + $agentProcId)
    if (-not (Wait-HttpReady -BaseUrl "http://127.0.0.1:7878" -TimeoutSeconds $WaitSeconds)) { throw "agent HTTP did not become ready" }
    Write-Ok ("agent up, pid=" + $agentProcId)

    if ($KernelDwellSeconds -gt 0) {
        Write-Step ("Kernel warm-up dwell " + $KernelDwellSeconds + "s")
        Start-Sleep -Seconds $KernelDwellSeconds
    }

    Write-Step "Full project access (auto-apply; gen.jobs.start is RiskConfirm)"
    $auth = Invoke-RestMethod -Method Post -Uri "http://127.0.0.1:7878/agent/authority" -ContentType "application/json" -Body (@{ authority_mode = "full_project_access" } | ConvertTo-Json -Compress) -TimeoutSec 30
    Add-Prereq ("authority_mode=" + $auth.authority_mode)
    if ($auth.authority_mode -ne "full_project_access") { throw ("authority switch refused: " + ($auth | ConvertTo-Json -Compress)) }

    Write-Step "VRAM sampler (2s interval) while the driver runs (engine comes up mid-run via the agent supervisor)"
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

    Write-Step "Run the DG1-C driver (W0 registration / W1 fixture / W2 job v1 + engine bring-up / W3 roundtrip / W4 mapping+hygiene / W5 two jobs / W6 non-blocking / W8 artifacts / W9 readback)"
    $driverArgs = @(
        "-I", (Join-Path $RepoRoot "scripts\yue_wire_smoke.py"),
        "--agent-base", "http://127.0.0.1:7878",
        "--engine-base", "http://127.0.0.1:8087",
        "--kernel-zmq", "tcp://127.0.0.1:5555",
        "--run-root", $RunRoot,
        "--download-dir", $DownloadDir,
        "--jobs-dir", $JobsDir,
        "--job-budget", [string]$JobBudgetSeconds
    )
    if ($SkipLegW9) { $driverArgs += "--skip-legw9" }
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
            throw ("engine VRAM peak " + $vramPeakMiB + "MiB exceeds the 6451MiB ceiling — environment interruption, escalate")
        }
    }
    if ($driverExit -eq 0) { $outcome = "finished" } else { $outcome = "driver_failed" }
}
catch {
    $detail = $_.Exception.Message
    Add-Prereq ("fatal=" + $detail)
    Write-Bad ("fatal: " + $detail)
    if ($outcome -eq "env_failure" -and $detail -notlike "*port*" -and $detail -notlike "*listen*" -and $detail -notlike "*ready*" -and $detail -notlike "*vram*" -and $detail -notlike "*binary*") { $outcome = "chain_failed" }
}
finally {
    Write-Step "Teardown (agent, kernel, engine safety net — all independent stops)"
    if ($agentProcId) { Stop-Process -Id $agentProcId -Force -ErrorAction SilentlyContinue; Add-Prereq ("stopped_agent_pid=" + $agentProcId) }
    if ($kernelProcId) { Stop-Process -Id $kernelProcId -Force -ErrorAction SilentlyContinue; Add-Prereq ("stopped_kernel_pid=" + $kernelProcId) }
    if ($null -ne $vramJob) { Stop-Job -Job $vramJob -ErrorAction SilentlyContinue; Remove-Job -Job $vramJob -Force -ErrorAction SilentlyContinue }
    # Engine safety net: the agent supervisor owns its process, but a
    # force-killed agent leaves the engine orphaned — stop it so the stack
    # releases 8087 (recorded; not a product behavior).
    $engineProc = Get-Process -Name yue-server -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($engineProc) { Stop-Process -Id $engineProc.Id -Force -ErrorAction SilentlyContinue; Add-Prereq ("stopped_engine_pid=" + $engineProc.Id) }
    Start-Sleep -Seconds 3
    foreach ($port in 7878, 5555, 5556, 8087) {
        if (Get-TcpListener -Port $port) { Add-Prereq ("port_still_busy=" + $port) } else { Add-Prereq ("port_released=" + $port) }
    }
    $vramAfter = Get-VramUsedMiB
    Add-Prereq ("vram_after_mib=" + $vramAfter)
    Add-Prereq ("finished=" + (Get-Date -Format "o"))
    $prereq | Out-File -FilePath $PrereqPath -Encoding utf8

    $report = [ordered]@{
        schema_version = "run_yue_wire_smoke.v1"
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
Write-Host ("YUE_WIRE_STACK_VERDICT " + $outcome + " driver_exit=" + $driverExit)
switch ($outcome) {
    "finished" { exit 0 }
    "vram_over_budget" { exit 2 }
    "env_failure" { exit 2 }
    default { exit 1 }
}

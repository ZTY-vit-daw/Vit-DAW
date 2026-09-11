<#
B6: mix-tick terminal delivery quality smoke (real stack).

Drives the 2026-09-11 hand-test-3 attempt-#2 mix-tick shape end to end with
full_project_access active: the free-state improvement chain proposes a
bounded native adjustment on the mantest3 forensics project (copy inside the
run dir; original untouched). Expected after the B6 fix: the mix_tick.pending
display uses the direct-apply wording (no confirmation promise), and the
chain terminal is actionable without logs (five elements: target track,
parameter, signed change + readback, targeted finding, where to look) with no
internal codes and no hollow pending-judgment claim; the judgment path is
honest (answerable park carries a servicable entry, settled carries none).

Stack shape: same as run_b5_workspace_switch_smoke.ps1 (kernel from repo
Release artefacts with the repo VitApp workspace, agent binary built into the
run dir, agent conversation drafts redirected into the run dir via
VIT_HISTORY_DRAFT_ROOT).

Exit codes: 0 = all segments green; 1 = assertion failure; 2 = environment
failure.
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = "",
    [switch]$SkipBuild,
    [string]$KernelExe = "",
    [int]$WaitSeconds = 30,
    [int]$KernelDwellSeconds = 120,
    [int]$TerminalBudgetSeconds = 420,
    [string]$Message = "请从混音角度检查当前工程有什么可以改善的地方，并做一个小步尝试"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) {
        return (Resolve-Path -LiteralPath $Explicit).Path
    }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host ("== " + $Message) -ForegroundColor Cyan
}

function Get-TcpListener {
    param([int]$Port)
    return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
        Select-Object -First 1
}

function Wait-PortListen {
    param([int]$Port, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        if (Get-TcpListener -Port $Port) {
            return $true
        }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

function Wait-HttpReady {
    param([string]$BaseUrl, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri ($BaseUrl.TrimEnd("/") + "/health") -TimeoutSec 2
            if ($resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500) {
                return $true
            }
        }
        catch {
            Start-Sleep -Milliseconds 500
        }
    } while ((Get-Date) -lt $deadline)
    return $false
}

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
$AgentDir = Join-Path $RepoRoot "agent"
$ScriptsDir = Join-Path $RepoRoot "scripts"
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("artifacts\B6\" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
if (Test-Path -LiteralPath $RunRoot) {
    throw "run dir already exists (no-overwrite rule): $RunRoot"
}
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
$AgentDrafts = Join-Path $RunRoot "agent_drafts"
$AgentBin = Join-Path $RunRoot "bin\VitAgent.b6.exe"
$AgentLog = Join-Path $RunRoot "agent_last.log"
$PrereqPath = Join-Path $RunRoot "prereq.txt"
$Project = Join-Path $RunRoot "project\spv1_p01_b6.vit"
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"
}
else {
    $KernelExe = (Resolve-Path -LiteralPath $KernelExe).Path
}
$ForensicsSource = Join-Path $RepoRoot "artifacts\mantest3\20260911\project\spv1_p01.vit"
$DefaultProjectHashBefore = (Get-FileHash -LiteralPath $RepoDefaultProject -Algorithm SHA256).Hash

$prereq = New-Object System.Collections.Generic.List[string]
function Add-Prereq {
    param([string]$Line)
    $script:prereq.Add($Line)
    Write-Host $Line
}

$envFailure = $false
$driverExit = -1
$AgentProcId = 0
$KernelProcId = 0

try {
    Write-Step "B6 run root: $RunRoot"
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("started=" + (Get-Date -Format o))
    Add-Prereq ("head=" + (git -C $RepoRoot rev-parse HEAD))
    Add-Prereq ("git_status=" + ((git -C $RepoRoot status --short) -join " ; "))

    Write-Step "Prereq: ports 7878/5555/5556 must be free"
    foreach ($port in @(7878, 5555, 5556)) {
        $listener = Get-TcpListener -Port $port
        if ($null -ne $listener) {
            Add-Prereq ("port_busy=" + $port + " pid=" + $listener.OwningProcess)
            throw "port $port is already listening (stack ownership rule); aborting before any start"
        }
        Add-Prereq ("port_free=" + $port)
    }

    if (-not (Test-Path -LiteralPath $ForensicsSource -PathType Leaf)) {
        throw "forensics source project missing: $ForensicsSource"
    }
    New-Item -ItemType Directory -Path (Split-Path -Parent $Project) -Force | Out-Null
    Copy-Item -LiteralPath $ForensicsSource -Destination $Project
    Add-Prereq ("project=" + $Project + " (copy of mantest3 forensics project; original untouched)")

    if (-not $SkipBuild) {
        Write-Step "Build VitAgent (isolated binary in run dir)"
        New-Item -ItemType Directory -Path (Split-Path -Parent $AgentBin) -Force | Out-Null
        Push-Location $AgentDir
        try {
            & go build -o $AgentBin .\cmd\vitagent
            if ($LASTEXITCODE -ne 0) {
                throw "go build failed with exit code $LASTEXITCODE"
            }
        }
        finally {
            Pop-Location
        }
    }
    elseif (-not (Test-Path -LiteralPath $AgentBin -PathType Leaf)) {
        throw "-SkipBuild given but isolated agent binary missing: $AgentBin"
    }
    $agentBinaryItem = Get-Item -LiteralPath $AgentBin
    Add-Prereq ("agent_binary=" + $AgentBin + " size=" + $agentBinaryItem.Length + " mtime=" + $agentBinaryItem.LastWriteTime.ToString("o"))

    Write-Step "Start kernel (repo workspace, E2E1-comparable shape)"
    $kernelBinaryItem = Get-Item -LiteralPath $KernelExe
    Add-Prereq ("kernel_binary=" + $KernelExe + " size=" + $kernelBinaryItem.Length + " mtime=" + $kernelBinaryItem.LastWriteTime.ToString("o"))
    $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -WindowStyle Hidden -PassThru
    $KernelProcId = $kernelProc.Id
    Add-Prereq ("kernel_pid=" + $KernelProcId)
    if (-not (Wait-PortListen -Port 5555 -TimeoutSeconds $WaitSeconds)) {
        throw "kernel ZMQ REQ port 5555 did not listen within $WaitSeconds s"
    }
    Write-Host ("ok: kernel up, pid=" + $KernelProcId) -ForegroundColor Green

    Write-Step "Start agent (isolated draft root inside run dir)"
    $priorDraftRoot = [Environment]::GetEnvironmentVariable("VIT_HISTORY_DRAFT_ROOT", "Process")
    $priorDevRoot = [Environment]::GetEnvironmentVariable("VIT_DAW_DEV_ROOT", "Process")
    $env:VIT_HISTORY_DRAFT_ROOT = $AgentDrafts
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $agentProc = Start-Process -FilePath $AgentBin -ArgumentList @(
            "-http", "127.0.0.1:7878",
            "-last-log-path", $AgentLog,
            "-keep-last-log-lines", "6000"
        ) -WorkingDirectory $AgentDir -WindowStyle Hidden -PassThru
    }
    finally {
        if ($null -eq $priorDraftRoot) {
            Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue
        }
        else {
            $env:VIT_HISTORY_DRAFT_ROOT = $priorDraftRoot
        }
        if ($null -eq $priorDevRoot) {
            Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue
        }
        else {
            $env:VIT_DAW_DEV_ROOT = $priorDevRoot
        }
    }
    $AgentProcId = $agentProc.Id
    Add-Prereq ("agent_pid=" + $AgentProcId)
    Add-Prereq ("agent_env=VIT_HISTORY_DRAFT_ROOT=" + $AgentDrafts + " VIT_DAW_DEV_ROOT=" + $RepoRoot)
    if (-not (Wait-HttpReady -BaseUrl "http://127.0.0.1:7878" -TimeoutSeconds $WaitSeconds)) {
        throw "agent HTTP did not become ready at 127.0.0.1:7878"
    }

    if ($KernelDwellSeconds -gt 0) {
        Write-Step ("Kernel warm-up dwell " + $KernelDwellSeconds + "s (fresh kernels hold the engine-busy flag; same exposure as E2E1)")
        Start-Sleep -Seconds $KernelDwellSeconds
        $dwellState = Invoke-RestMethod -Uri "http://127.0.0.1:7878/agent/state" -TimeoutSec 10
        Add-Prereq ("post_dwell_shadow_initialized=" + $dwellState.shadow.initialized + " track_count=" + $dwellState.shadow.track_count)
        if ($dwellState.shadow.initialized -ne $true) {
            throw "shadow lost initialization after dwell (kernel wedged?)"
        }
    }

    Write-Step "B6 driver (M1-M6)"
    & python (Join-Path $ScriptsDir "b6_mixtick_terminal_forensics.py") `
        --agent-http "http://127.0.0.1:7878" `
        --kernel-req "tcp://127.0.0.1:5555" `
        --out-dir (Join-Path $RunRoot "driver") `
        --run-root $RunRoot `
        --agent-log $AgentLog `
        --project $Project `
        --message $Message `
        --terminal-budget ([double]$TerminalBudgetSeconds) 2>&1 | Tee-Object -FilePath (Join-Path $RunRoot "driver_console.txt")
    $driverExit = $LASTEXITCODE
    Add-Prereq ("driver_exit=" + $driverExit)
}
catch {
    Add-Prereq ("fatal=" + $_.Exception.Message)
    $envFailure = $true
    Write-Host ("fatal: " + $_.Exception.Message) -ForegroundColor Yellow
}
finally {
    Write-Step "Teardown: stop self-started stack, verify ports"
    foreach ($pair in @(@("agent", $AgentProcId), @("kernel", $KernelProcId))) {
        $name = $pair[0]
        $procId = [int]$pair[1]
        if ($procId -gt 0) {
            try {
                Stop-Process -Id $procId -Force -ErrorAction Stop
                Wait-Process -Id $procId -Timeout 20 -ErrorAction SilentlyContinue
                Add-Prereq ("stopped_" + $name + "_pid=" + $procId)
            }
            catch {
                Add-Prereq ("stop_failed_" + $name + "_pid=" + $procId + " " + $_.Exception.Message)
            }
        }
    }
    Start-Sleep -Seconds 1
    foreach ($port in @(7878, 5555, 5556)) {
        $listener = Get-TcpListener -Port $port
        if ($null -ne $listener) {
            Add-Prereq ("PORT_STILL_BUSY=" + $port + " pid=" + $listener.OwningProcess)
            $envFailure = $true
        }
        else {
            Add-Prereq ("port_released=" + $port)
        }
    }
    $DefaultProjectHashAfter = (Get-FileHash -LiteralPath $RepoDefaultProject -Algorithm SHA256).Hash
    Add-Prereq ("default_project_hash_before=" + $DefaultProjectHashBefore)
    Add-Prereq ("default_project_hash_after=" + $DefaultProjectHashAfter)
    Add-Prereq ("finished=" + (Get-Date -Format o))
    $prereq | Out-File -FilePath $PrereqPath -Encoding utf8

    $summary = [ordered]@{
        run_root     = $RunRoot
        driver_exit  = $driverExit
        env_failure  = $envFailure
    }
    ($summary | ConvertTo-Json) | Out-File -FilePath (Join-Path $RunRoot "b6_summary.json") -Encoding utf8
    Write-Host ("summary: " + ($summary | ConvertTo-Json -Compress))

    if ($envFailure) {
        Write-Host "B6_EXIT 2" -ForegroundColor Red
        Set-Location $RepoRoot
        exit 2
    }
    if ($driverExit -eq 0) {
        Write-Host "B6_EXIT 0 (all segments green)" -ForegroundColor Green
        Set-Location $RepoRoot
        exit 0
    }
    Write-Host "B6_EXIT 1 (segment failure - see driver report)" -ForegroundColor Yellow
    Set-Location $RepoRoot
    exit 1
}

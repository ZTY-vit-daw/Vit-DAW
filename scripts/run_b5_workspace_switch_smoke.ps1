<#
B5: workspace activation-switch orphan smoke (real stack).

Replicates the 2026-09-11 hand-test-3 attempt-#1 timeline: the agent boots on
the default workspace, a chat turn parks an in-flight continuation chain, then
the kernel opens another project (the Godot "open project" user action) and
the agent's workspace activation switch fires mid-chain. Expected after the
B5 fix (explicit settle-then-switch): the old chain is explicitly closed
(cancelled record + turn.stopped notice + notice node in the old workspace
graph), and a fresh chain on the new workspace delivers and lands its
terminal (attempt-#2 regression).

Stack shape: same as run_chat_chain_e2e_smoke.ps1 (kernel from repo Release
artefacts with the repo VitApp workspace, agent binary built into the run
dir, agent conversation drafts redirected into the run dir via
VIT_HISTORY_DRAFT_ROOT). The switch-target project is a copy of the mantest3
forensics project placed inside the run dir (no writes to original forensics
artefacts).

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
    [switch]$CardApprove
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
    $RunRoot = Join-Path $RepoRoot ("artifacts\B5\" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
if (Test-Path -LiteralPath $RunRoot) {
    throw "run dir already exists (no-overwrite rule): $RunRoot"
}
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
$AgentDrafts = Join-Path $RunRoot "agent_drafts"
$AgentBin = Join-Path $RunRoot "bin\VitAgent.b5.exe"
$AgentLog = Join-Path $RunRoot "agent_last.log"
$PrereqPath = Join-Path $RunRoot "prereq.txt"
$NextProject = Join-Path $RunRoot "project\spv1_p01_b5.vit"
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"
}
else {
    $KernelExe = (Resolve-Path -LiteralPath $KernelExe).Path
}
$ForensicsSource = Join-Path $RepoRoot "artifacts\mantest3\20260911\project\spv1_p01.vit"

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
    Write-Step "B5 run root: $RunRoot"
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
    New-Item -ItemType Directory -Path (Split-Path -Parent $NextProject) -Force | Out-Null
    Copy-Item -LiteralPath $ForensicsSource -Destination $NextProject
    Add-Prereq ("next_project=" + $NextProject + " (copy of mantest3 forensics project; original untouched)")

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

    Write-Step "W1 stack gate"
    $state = Invoke-RestMethod -Uri "http://127.0.0.1:7878/agent/state" -TimeoutSec 10
    if ($state.status -ne "ok") {
        throw "GET /agent/state did not return ok"
    }
    Add-Prereq ("w1_shadow_initialized=" + $state.shadow.initialized + " track_count=" + $state.shadow.track_count)
    if ($state.shadow.initialized -ne $true) {
        throw "shadow not initialized after stack start"
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

    Write-Step "B5 driver (W1-W6)"
    $driverFlags = @()
    if ($CardApprove) {
        $driverFlags += @("--card-approve")
    }
    & python (Join-Path $ScriptsDir "b5_workspace_switch_forensics.py") `
        --agent-http "http://127.0.0.1:7878" `
        --kernel-req "tcp://127.0.0.1:5555" `
        --out-dir (Join-Path $RunRoot "driver") `
        --run-root $RunRoot `
        --agent-log $AgentLog `
        --next-project $NextProject `
        --terminal-budget ([double]$TerminalBudgetSeconds) `
        @driverFlags 2>&1 | Tee-Object -FilePath (Join-Path $RunRoot "driver_console.txt")
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
    Add-Prereq ("finished=" + (Get-Date -Format o))
    $prereq | Out-File -FilePath $PrereqPath -Encoding utf8

    $summary = [ordered]@{
        run_root     = $RunRoot
        driver_exit  = $driverExit
        env_failure  = $envFailure
    }
    ($summary | ConvertTo-Json) | Out-File -FilePath (Join-Path $RunRoot "b5_summary.json") -Encoding utf8
    Write-Host ("summary: " + ($summary | ConvertTo-Json -Compress))

    if ($envFailure) {
        Write-Host "B5_EXIT 2" -ForegroundColor Red
        Set-Location $RepoRoot
        exit 2
    }
    if ($driverExit -eq 0) {
        Write-Host "B5_EXIT 0 (all segments green)" -ForegroundColor Green
        Set-Location $RepoRoot
        exit 0
    }
    Write-Host "B5_EXIT 1 (segment failure - see driver report)" -ForegroundColor Yellow
    Set-Location $RepoRoot
    exit 1
}

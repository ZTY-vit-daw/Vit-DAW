<#
E2E-1: chat-chain user-loop unattended E2E smoke (two-layer bed, RED-baseline capable).

Layer A (HTTP contract, S1-S12 + stand-in judgment layer C): python driver
        scripts/e2e_chat_chain_layer_a.py
Layer B (real browser rendering/latency, R1-R7 + T1-T6, playwright headless
        chromium against the agent-served WebUI): scripts/e2e_chat_chain_layer_b.py

Stack shape (B1-DIAG comparable): kernel from the repo Release artefacts with
the repo VitApp workspace (waveform cache keys are workspace-path bound; a
cold isolated workspace re-bakes and wedges the kernel message thread - dbg1
through dbg3 evidence 2026-09-09), agent binary built into the run dir, agent
conversation drafts redirected into the run dir via VIT_HISTORY_DRAFT_ROOT.
The repo default_project.xml hash is recorded before/after; its mutation is
the same whitelisted runtime increment every smoke run (incl. B1-DIAG) makes.

Exit codes: 0 = bed delivered with expected assertion shape (RED baseline shape
when -ExpectRed, post-F1 shape when -ExpectF1Fixed — layer A requires S8/S11
green with S9/S10 observed honestly and F2-owned segments unchanged, layer B
keeps its F2/F3-owned red table; post-F2+F3 combined shape when -ExpectF2Fixed
— layer A requires S5/S6/S7 green on the scheduled variant on top of the F1
shape, layer B requires R4 and R5 (both halves: terminal survival via the F2
conversation-graph node, dead-card guard via the F3 interaction guard) green
in both variants with R3 recorded as B3-owned; all-green otherwise); 1 = shape
mismatch or
assertion failure of the expected kind; 2 = environment/bed failure (see run
dir prereq.txt).
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = "",
    [switch]$SkipLayerB,
    [switch]$SkipBuild,
    [string]$KernelExe = "",
    [string]$JudgmentBranch = "A",
    [switch]$ExpectRed,
    [switch]$ExpectF1Fixed,
    [switch]$ExpectF2Fixed,
    [switch]$B9FreeStateChat,
    [int]$WaitSeconds = 30,
    [int]$KernelDwellSeconds = 120,
    [int]$ApprovePacingMs = 5000,
    [int]$ChatBudgetSeconds = 0
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

function Write-Ok {
    param([string]$Message)
    Write-Host ("ok: " + $Message) -ForegroundColor Green
}

function Write-WarnLine {
    param([string]$Message)
    Write-Host ("warn: " + $Message) -ForegroundColor Yellow
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

function Get-FileHashText {
    param([string]$Path)
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return "missing"
    }
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash
}

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
$AgentDir = Join-Path $RepoRoot "agent"
$ScriptsDir = Join-Path $RepoRoot "scripts"
$RepoDefaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("artifacts\E2E1\" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
if (Test-Path -LiteralPath $RunRoot) {
    throw "run dir already exists (no-overwrite rule): $RunRoot"
}
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
$StackDir = Join-Path $RunRoot "stack"
$AgentDrafts = Join-Path $RunRoot "agent_drafts"
$AgentBin = Join-Path $RunRoot "bin\VitAgent.e2e.exe"
$AgentLog = Join-Path $RunRoot "agent_last.log"
$PrereqPath = Join-Path $RunRoot "prereq.txt"
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"
}
else {
    $KernelExe = (Resolve-Path -LiteralPath $KernelExe).Path
}

$prereq = New-Object System.Collections.Generic.List[string]
function Add-Prereq {
    param([string]$Line)
    $script:prereq.Add($Line)
    Write-Host $Line
}

$envFailure = $false
$layerAExit = -1
$layerBExit = -1
$layerBStatus = "skipped-by-switch"
$layerB9Exit = -1
$AgentProcId = 0
$KernelProcId = 0
$repoProjectHashBefore = ""
$repoProjectHashAfter = ""

try {
    Write-Step "E2E-1 run root: $RunRoot"
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("started=" + (Get-Date -Format o))
    $psArgsText = ($PSBoundParameters.GetEnumerator() | ForEach-Object { "$($_.Key)=$($_.Value)" }) -join " "
    Add-Prereq ("ps_args=" + $psArgsText)
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

    $godotEditor = Get-CimInstance Win32_Process -Filter "Name like 'Godot%'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -match "--editor" }
    Add-Prereq ("godot_editor_processes=" + ($(if ($godotEditor) { ($godotEditor | ForEach-Object { "$($_.ProcessId):$($_.CommandLine)" }) -join " ; " } else { "none" })))
    Add-Prereq "note=user-owned Godot editor processes are never touched by this bed"

    if (-not $SkipLayerB) {
        Write-Step "Playwright precheck (layer B)"
        $probeOut = Join-Path $RunRoot "playwright_probe.txt"
        $probeCode = "import sys; sys.path.insert(0, r'" + $ScriptsDir + "'); import e2e_chat_chain_layer_b as m; import json; ok = m.probe_browser(); print('PLAYWRIGHT_OK' if ok else 'PLAYWRIGHT_FAIL'); sys.exit(0 if ok else 1)"
        try {
            & python -c $probeCode 2>&1 | Tee-Object -FilePath $probeOut
            if ($LASTEXITCODE -ne 0) {
                throw "playwright probe failed"
            }
            Add-Prereq "playwright=ok"
            Write-Ok "playwright headless chromium available (channel fallback chain)"
        }
        catch {
            Add-Prereq ("playwright=FAILED " + $_.Exception.Message)
            Write-WarnLine "playwright environment unavailable -> layer B will be skipped (layer A single-layer delivery, explicitly not a full PASS)"
            $SkipLayerB = $true
            $layerBStatus = "env-unavailable"
        }
    }

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

    Write-Step "Start kernel (repo workspace, B1-comparable shape)"
    $kernelBinaryItem = Get-Item -LiteralPath $KernelExe
    Add-Prereq ("kernel_binary=" + $KernelExe + " size=" + $kernelBinaryItem.Length + " mtime=" + $kernelBinaryItem.LastWriteTime.ToString("o"))
    if (-not (Test-Path -LiteralPath $RepoDefaultProject -PathType Leaf)) {
        throw "repo default project missing: $RepoDefaultProject"
    }
    $repoProjectHashBefore = Get-FileHashText $RepoDefaultProject
    Add-Prereq ("repo_default_project_sha256_before=" + $repoProjectHashBefore)
    $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -WindowStyle Hidden -PassThru
    $KernelProcId = $kernelProc.Id
    Add-Prereq ("kernel_pid=" + $KernelProcId)
    Add-Prereq ("kernel_workspace=repo VitApp Workspace (waveform cache keys are path-bound; isolated workspaces re-bake and wedge the message thread - dbg1..dbg3 2026-09-09)")
    if (-not (Wait-PortListen -Port 5555 -TimeoutSeconds $WaitSeconds)) {
        throw "kernel ZMQ REQ port 5555 did not listen within $WaitSeconds s"
    }
    Write-Ok ("kernel up, pid=" + $KernelProcId)

    Write-Step "Start agent (isolated draft root inside run dir; COM root = repo)"
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
    Write-Ok ("agent up, pid=" + $AgentProcId)

    Write-Step "S1 stack gate (health + shadow init)"
    $state = Invoke-RestMethod -Uri "http://127.0.0.1:7878/agent/state" -TimeoutSec 10
    if ($state.status -ne "ok") {
        throw "GET /agent/state did not return ok"
    }
    Add-Prereq ("s1_shadow_initialized=" + $state.shadow.initialized + " track_count=" + $state.shadow.track_count)
    if ($state.shadow.initialized -ne $true) {
        throw "shadow not initialized after stack start"
    }
    Write-Ok ("shadow initialized, track_count=" + $state.shadow.track_count)

    if ($KernelDwellSeconds -gt 0) {
        Write-Step ("Kernel warm-up dwell " + $KernelDwellSeconds + "s (fresh kernels hold the engine-busy flag while the initial waveform bake drains; B1's kernel was 8 min old at flow start)")
        Start-Sleep -Seconds $KernelDwellSeconds
        $dwellState = Invoke-RestMethod -Uri "http://127.0.0.1:7878/agent/state" -TimeoutSec 10
        Add-Prereq ("post_dwell_shadow_initialized=" + $dwellState.shadow.initialized + " track_count=" + $dwellState.shadow.track_count)
        if ($dwellState.shadow.initialized -ne $true) {
            throw "shadow lost initialization after dwell (kernel wedged?)"
        }
    }

    if ($ExpectRed -and $ExpectF1Fixed) {
        throw "-ExpectRed and -ExpectF1Fixed are mutually exclusive shape modes"
    }
    if ($ExpectF2Fixed -and ($ExpectRed -or $ExpectF1Fixed)) {
        throw "-ExpectF2Fixed is mutually exclusive with -ExpectRed/-ExpectF1Fixed shape modes"
    }
    # Layer B's red table is F2/F3-owned (terminal delivery / refresh recovery)
    # and is unaffected by the F1 judgment-boundary exemption, so -ExpectF1Fixed
    # still evaluates layer B against the RED baseline table.
    $layerAFlags = @()
    $layerBFlags = @()
    if ($ExpectRed) {
        $layerAFlags = @("--expect-red")
        $layerBFlags = @("--expect-red")
    }
    if ($ExpectF1Fixed) {
        $layerAFlags = @("--f1-fixed")
        $layerBFlags = @("--expect-red")
    }
    if ($ExpectF2Fixed) {
        # Post-F2+F3 combined shape: layer A requires S5/S6/S7 green on the
        # scheduled variant on top of the F1 shape; layer B requires R4 and R5
        # (both halves) green in both variants and R3 recorded as B3-owned
        # (F3 receipt §5 pre-declared R5's dead-card half going green).
        $layerAFlags = @("--f2-fixed")
        $layerBFlags = @("--f2-fixed")
    }

    # Layer order: B first, then A. A fresh browser session hydrates the
    # server's current conversation (messages + pending cards) with no
    # conversation scoping, and a RED-parked conversation leaves a stale
    # pending card that blocks idle authority switches and suppresses the
    # next session's own card rendering (official run 20260909_213930).
    # Layer B therefore runs on the clean stack; layer A's HTTP card
    # discovery is goal-scoped and immune to the leftover pending state.
    if (-not $SkipLayerB) {
        Write-Step "Layer B: browser rendering/latency bed (playwright headless chromium)"
        $layerBConsole = Join-Path $RunRoot "layer_b_console.txt"
        # -ChatBudgetSeconds must also widen layer B's T2 budget (approve1 ->
        # card2 wait rides the same LLM-window latency as layer A's chat budget;
        # known-issues #17-2: the 90s default nearly saturated in run 220854
        # with T2=84.6s).
        if ($ChatBudgetSeconds -gt 0) {
            $layerBFlags += @("--t2-budget", ([string]$ChatBudgetSeconds))
        }
        & python (Join-Path $ScriptsDir "e2e_chat_chain_layer_b.py") `
            --webui-url "http://127.0.0.1:7878/app/" `
            --agent-http "http://127.0.0.1:7878" `
            --out-dir (Join-Path $RunRoot "layer_b") `
            @layerBFlags 2>&1 | Tee-Object -FilePath $layerBConsole
        $layerBExit = $LASTEXITCODE
        # known-issues #17-1: record the truth after an actual run - the init
        # value "skipped-by-switch" used to leak into e2e1_summary.json even
        # when layer B ran (run 220854 ran layer B but the summary claimed a
        # skip; layer reports were the only source of truth).
        $layerBStatus = "ran"
        Add-Prereq ("layer_b_exit=" + $layerBExit)
    }
    else {
        Add-Prereq ("layer_b=" + $layerBStatus)
    }

    Write-Step "Layer A: HTTP contract bed (S1-S12 + stand-in judgment)"
    $layerAConsole = Join-Path $RunRoot "layer_a_console.txt"
    # S2's card wait rides the scheduler chain behind the first chat slice; the
    # driver's default budget absorbs the calibrated round-1 latency (~52s) and
    # -ChatBudgetSeconds widens it for slower LLM windows (latency variance is
    # an environment class, not a segment shape).
    $layerABudgetFlags = @()
    if ($ChatBudgetSeconds -gt 0) {
        $layerABudgetFlags = @("--chat-budget", ([string]$ChatBudgetSeconds))
    }
    & python (Join-Path $ScriptsDir "e2e_chat_chain_layer_a.py") `
        --agent-http "http://127.0.0.1:7878" `
        --out-dir (Join-Path $RunRoot "layer_a") `
        --branch $JudgmentBranch `
        --approve-pacing ([double]($ApprovePacingMs / 1000.0)) `
        --kernel-project $RepoDefaultProject `
        @layerABudgetFlags `
        @layerAFlags 2>&1 | Tee-Object -FilePath $layerAConsole
    $layerAExit = $LASTEXITCODE
    Add-Prereq ("layer_a_exit=" + $layerAExit)

    # B9 layer (card B9, 2026-09-11): free-state default chat chain trace
    # unified-surface scenario (manual test-3 form replay). Must run AFTER
    # layer A: this layer switches authority to full access (mix_tick applies
    # directly, replaying the 19:54 manual form); running it earlier would
    # starve layer A of its manual confirmation cards. After layer A's
    # stand-in judgment settles, the stack should be idle and the authority
    # switch should not hit 409; a busy switch is recorded as env failure.
    if ($B9FreeStateChat) {
        Write-Step "Layer B9: free-state chat trace unified surface (playwright)"
        $layerB9Console = Join-Path $RunRoot "layer_b9_console.txt"
        $layerB9BudgetFlags = @()
        if ($ChatBudgetSeconds -gt 0) {
            $layerB9BudgetFlags = @("--chat-budget", ([string]$ChatBudgetSeconds))
        }
        & python (Join-Path $ScriptsDir "b9_free_state_chat_trace_smoke.py") `
            --webui-url "http://127.0.0.1:7878/app/" `
            --agent-http "http://127.0.0.1:7878" `
            --out-dir (Join-Path $RunRoot "layer_b9") `
            @layerB9BudgetFlags 2>&1 | Tee-Object -FilePath $layerB9Console
        $layerB9Exit = $LASTEXITCODE
        Add-Prereq ("layer_b9_exit=" + $layerB9Exit)
    }
}
catch {
    Add-Prereq ("fatal=" + $_.Exception.Message)
    $envFailure = $true
    Write-WarnLine ("fatal: " + $_.Exception.Message)
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
    $repoProjectHashAfter = Get-FileHashText $RepoDefaultProject
    Add-Prereq ("repo_default_project_sha256_after=" + $repoProjectHashAfter)
    if ($repoProjectHashBefore -and ($repoProjectHashBefore -ne $repoProjectHashAfter)) {
        Add-Prereq "note=repo default_project.xml mutated by the kernel during the run (whitelisted runtime increment, same exposure as dev_agent_smoke.ps1/B1-DIAG; git diff to be recorded in the receipt)"
    }
    $godotEditorAfter = Get-CimInstance Win32_Process -Filter "Name like 'Godot%'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -match "--editor" }
    Add-Prereq ("godot_editor_after=" + ($(if ($godotEditorAfter) { ($godotEditorAfter | ForEach-Object { $_.ProcessId }) -join "," } else { "none" })))

    Add-Prereq ("finished=" + (Get-Date -Format "o"))
    $prereq | Out-File -FilePath $PrereqPath -Encoding utf8

    $summary = [ordered]@{
        run_root         = $RunRoot
        expect_red       = [bool]$ExpectRed
        expect_f1_fixed  = [bool]$ExpectF1Fixed
        layer_a_exit     = $layerAExit
        layer_b_exit     = $layerBExit
        layer_b_status   = $layerBStatus
        layer_b9_exit    = $layerB9Exit
        env_failure      = $envFailure
    }
    ($summary | ConvertTo-Json) | Out-File -FilePath (Join-Path $RunRoot "e2e1_summary.json") -Encoding utf8
    Write-Host ""
    Write-Host ("summary: " + ($summary | ConvertTo-Json -Compress))

    if ($envFailure) {
        Write-Host "E2E1_EXIT 2" -ForegroundColor Red
        Set-Location $RepoRoot
        exit 2
    }
    $overallOk = ($layerAExit -eq 0)
    if (-not $SkipLayerB) {
        $overallOk = $overallOk -and ($layerBExit -eq 0)
    }
    if ($B9FreeStateChat) {
        $overallOk = $overallOk -and ($layerB9Exit -eq 0)
    }
    if ($overallOk) {
        Write-Host "E2E1_EXIT 0 (expected shape confirmed)" -ForegroundColor Green
        Set-Location $RepoRoot
        exit 0
    }
    Write-Host "E2E1_EXIT 1 (assertion/shape mismatch - see layer reports)" -ForegroundColor Yellow
    Set-Location $RepoRoot
    exit 1
}

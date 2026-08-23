#Requires -Version 5.1
# Free-state open-intent acceptance smoke (ADR docs/FREE_STATE_MINIMUM_IMPROVEMENT_WORKFLOW_UPDATE_2026-08-22.md §13 artifact 6).
# Starts the real three-piece stack (VitApp kernel + Godot frontend + rebuilt Go agent) using the
# same build/start sequence as scripts/dev_agent_smoke.ps1 -StartKernel -StartUI (kernel exe, Godot
# project UI, agent HTTP), then runs scripts/free_state_open_intent_acceptance.py against the live
# stack. The dev_agent_smoke extra smokes are not re-run here; this script owns its own PASS exit code.
# Exit code 0 = PASS.
[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    [string]$GodotProject = "D:\Godot\project\vit-daw-frontend",
    [int]$TimeoutSeconds = 900,
    [int]$WaitSeconds = 60
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
function Fail([string]$Message) { throw $Message }
function Wait-HttpReady([string]$Uri, [int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    do {
        try { if ((Invoke-WebRequest -UseBasicParsing -Uri $Uri -TimeoutSec 3).StatusCode -eq 200) { return $true } } catch { }
        Start-Sleep -Milliseconds 400
    } while ((Get-Date) -lt $deadline)
    return $false
}
function Wait-Port([int]$Port, [int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    do {
        if (Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue) { return $true }
        Start-Sleep -Milliseconds 400
    } while ((Get-Date) -lt $deadline)
    return $false
}
function Wait-UdpPort([int]$Port, [int]$Seconds) {
    $deadline = (Get-Date).AddSeconds($Seconds)
    do {
        if (Get-NetUDPEndpoint -LocalPort $Port -ErrorAction SilentlyContinue) { return $true }
        Start-Sleep -Milliseconds 400
    } while ((Get-Date) -lt $deadline)
    return $false
}
function Assert-PortOwner([int]$Port, [int]$ExpectedPid, [bool]$Udp = $false) {
    $owner = if ($Udp) {
        Get-NetUDPEndpoint -LocalPort $Port -ErrorAction SilentlyContinue | Select-Object -First 1
    } else {
        Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue | Select-Object -First 1
    }
    if ($null -eq $owner -or [int]$owner.OwningProcess -ne $ExpectedPid) {
        $actual = if ($null -eq $owner) { "none" } else { [string]$owner.OwningProcess }
        Fail "port $Port is not owned by this run (expected pid=$ExpectedPid, actual=$actual)"
    }
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$GodotExe = (Resolve-Path -LiteralPath $GodotExe).Path
$GodotProject = (Resolve-Path -LiteralPath $GodotProject).Path
foreach ($port in @(5555, 5556, 7878, 8787)) {
    if (Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue) {
        Fail "Port $port is already in use; refusing to disturb an existing product session"
    }
}
foreach ($port in @(4445)) {
    if (Get-NetUDPEndpoint -LocalPort $port -ErrorAction SilentlyContinue) {
        Fail "UDP port $port is already in use; refusing to reuse an existing Agent"
    }
}

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$runDir = Join-Path $RepoRoot ("artifacts\free_state_open_intent_acceptance\" + $stamp)
$null = New-Item -ItemType Directory -Force -Path $runDir
$report = Join-Path $runDir "free_state_open_intent_acceptance.json"
$journal = Join-Path $runDir "agent_journal.json"
$agentDir = Join-Path $RepoRoot "agent"
$agentExe = Join-Path $agentDir "bin\VitAgent.exe"
$vspHubExe = Join-Path $agentDir "bin\VspHub.exe"
$kernelExe = Join-Path $RepoRoot "Export\staging\runtime\VitApp.exe"
$agentLog = Join-Path $runDir "agent_last.log"
$godotStdout = Join-Path $runDir "godot.stdout.log"
$godotStderr = Join-Path $runDir "godot.stderr.log"
$provenanceReport = Join-Path $runDir "process_provenance.json"
$godot = $null
$kernel = $null
$vspHub = $null
$agent = $null
$priorJournal = $env:VIT_AGENT_JOURNAL_PATH
$priorLauncher = $env:VIT_FROM_LAUNCHER
$startupError = $null
try {
    $env:VIT_AGENT_JOURNAL_PATH = $journal
    # Godot's startup page autostarts local kernel/VspHub/VitAgent unless it
    # is launcher-managed. This script owns all three processes explicitly.
    $env:VIT_FROM_LAUNCHER = "1"

    # Build agent + VspHub (same sequence as scripts/dev_agent_smoke.ps1 / run_ccb_observation_product_smoke.ps1).
    Push-Location $agentDir
    try {
        & go build -o $agentExe ./cmd/vitagent
        if ($LASTEXITCODE -ne 0) { Fail "VitAgent build failed" }
        & go build -o $vspHubExe ./cmd/vsphub
        if ($LASTEXITCODE -ne 0) { Fail "VspHub build failed" }
    }
    finally { Pop-Location }

    # Start kernel + Godot frontend + VspHub + agent (same sequence as dev_agent_smoke -StartKernel -StartUI,
    # plus VspHub so the observation path is not spuriously capability_blocked).
    if (-not (Test-Path -LiteralPath $kernelExe)) { Fail "kernel exe not found: $kernelExe" }
    $kernel = Start-Process -FilePath $kernelExe -WorkingDirectory (Split-Path -Parent $kernelExe) -WindowStyle Hidden -PassThru
    if ($kernel.HasExited) { Fail "kernel exited immediately with code $($kernel.ExitCode)" }
    if (-not (Wait-Port 5555 $WaitSeconds)) { Fail "kernel command port 5555 did not become ready" }
    Assert-PortOwner 5555 $kernel.Id
    $godot = Start-Process -FilePath $GodotExe -ArgumentList @("--path", $GodotProject) `
        -WorkingDirectory $GodotProject -WindowStyle Hidden -RedirectStandardOutput $godotStdout `
        -RedirectStandardError $godotStderr -PassThru
    if (-not (Wait-Port 5556 $WaitSeconds)) { Fail "kernel event port 5556 did not become ready" }
    Assert-PortOwner 5556 $kernel.Id
    $vspHub = Start-Process -FilePath $vspHubExe -ArgumentList @("-last-log-path", (Join-Path $runDir "vsp_hub_last.log")) `
        -WorkingDirectory $agentDir -WindowStyle Hidden -PassThru
    if ($vspHub.HasExited) { Fail "VspHub exited immediately with code $($vspHub.ExitCode)" }
    if (-not (Wait-Port 8787 $WaitSeconds)) { Fail "VspHub port 8787 did not become ready" }
    Assert-PortOwner 8787 $vspHub.Id
    $agent = Start-Process -FilePath $agentExe -ArgumentList @("-http", "127.0.0.1:7878", "-last-log-path", $agentLog) `
        -WorkingDirectory $agentDir -WindowStyle Hidden -PassThru
    if ($agent.HasExited) { Fail "VitAgent exited immediately with code $($agent.ExitCode)" }
    if (-not (Wait-HttpReady "http://127.0.0.1:7878/health" $WaitSeconds)) { Fail "VitAgent HTTP did not become ready" }
    Assert-PortOwner 7878 $agent.Id
    if (-not (Wait-UdpPort 4445 $WaitSeconds)) { Fail "VitAgent UDP command port 4445 did not become ready" }
    Assert-PortOwner 4445 $agent.Id $true

    $provenance = [ordered]@{
        schema_version = "free_state_process_provenance.v1"
        repo_root = $RepoRoot
        verified_at = (Get-Date).ToUniversalTime().ToString("o")
        process_ownership_verified = $true
        binaries = @(
            [ordered]@{ role = "kernel"; path = $kernelExe; sha256 = (Get-FileHash -LiteralPath $kernelExe -Algorithm SHA256).Hash; pid = $kernel.Id },
            [ordered]@{ role = "godot"; path = $GodotExe; sha256 = (Get-FileHash -LiteralPath $GodotExe -Algorithm SHA256).Hash; pid = $godot.Id },
            [ordered]@{ role = "vsp_hub"; path = $vspHubExe; sha256 = (Get-FileHash -LiteralPath $vspHubExe -Algorithm SHA256).Hash; pid = $vspHub.Id },
            [ordered]@{ role = "agent"; path = $agentExe; sha256 = (Get-FileHash -LiteralPath $agentExe -Algorithm SHA256).Hash; pid = $agent.Id }
        )
        ports = @(
            [ordered]@{ protocol = "tcp"; port = 5555; pid = $kernel.Id },
            [ordered]@{ protocol = "tcp"; port = 5556; pid = $kernel.Id },
            [ordered]@{ protocol = "tcp"; port = 8787; pid = $vspHub.Id },
            [ordered]@{ protocol = "tcp"; port = 7878; pid = $agent.Id },
            [ordered]@{ protocol = "udp"; port = 4445; pid = $agent.Id }
        )
    }
    $provenance | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $provenanceReport -Encoding UTF8

    & python (Join-Path $RepoRoot "scripts\free_state_open_intent_acceptance.py") `
        --journal $journal --output $report --timeout-sec $TimeoutSeconds `
        --agent-log $agentLog --agent-log (Join-Path $RepoRoot "VitApp\Workspace\Logs\agent_last.log")
    if ($LASTEXITCODE -ne 0) { Fail "free-state open-intent acceptance failed; inspect $runDir" }
    Write-Host ("PASS: " + $report)
}
catch {
    $startupError = $_.Exception.Message
    $failure = [ordered]@{
        schema_version = "free_state_open_intent_acceptance.v1"
        status = "infrastructure_failed"
        error = $startupError
        process_ownership_verified = $false
        mutation_performed = $false
    }
    $failure | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $report -Encoding UTF8
    throw
}
finally {
    if ($null -ne $priorJournal) { $env:VIT_AGENT_JOURNAL_PATH = $priorJournal } else { Remove-Item Env:VIT_AGENT_JOURNAL_PATH -ErrorAction SilentlyContinue }
    if ($null -ne $priorLauncher) { $env:VIT_FROM_LAUNCHER = $priorLauncher } else { Remove-Item Env:VIT_FROM_LAUNCHER -ErrorAction SilentlyContinue }
    # Tear down only processes this run started (ports were verified free up front).
    Start-Sleep -Milliseconds 800
    if ($null -ne $godot) {
        try { $null = $godot.CloseMainWindow() } catch { }
        try { if (-not $godot.WaitForExit(12000)) { Stop-Process -Id $godot.Id -Force -ErrorAction SilentlyContinue } } catch { }
    }
    foreach ($process in @($agent, $vspHub, $kernel)) {
        if ($null -ne $process) {
            try { if (-not $process.HasExited) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue } } catch { }
        }
    }
    foreach ($port in @(5555, 5556, 7878, 8787)) {
        $listener = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($null -eq $listener) { continue }
        $process = Get-CimInstance Win32_Process -Filter ("ProcessId=" + $listener.OwningProcess) -ErrorAction SilentlyContinue
        if ($null -ne $process -and $process.ExecutablePath -like ($RepoRoot + "*")) {
            Stop-Process -Id $listener.OwningProcess -Force -ErrorAction SilentlyContinue
        }
    }
    $udpListener = Get-NetUDPEndpoint -LocalPort 4445 -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $udpListener -and $null -ne $agent -and [int]$udpListener.OwningProcess -eq $agent.Id) {
        Stop-Process -Id $agent.Id -Force -ErrorAction SilentlyContinue
    }
    Write-Host ("artifacts: " + $runDir)
}

#Requires -Version 5.1
# B13-C real-stack probe: consecutive free-state questions on one project must
# produce the branch-appropriate human settlement reply (supply trimmed by the
# disclosure budget vs. genuinely no evidence-backed hypothesis).
#
# Starts the real three-piece stack (VitApp kernel + Godot frontend + rebuilt Go
# agent, plus VspHub so the observation path is not spuriously
# capability_blocked) using the same build/start sequence as
# scripts/run_free_state_open_intent_acceptance.ps1 and
# scripts/dev_agent_smoke.ps1 -StartKernel -StartUI, then runs
# scripts/b13c_reply_branch_smoke.py against the live stack.
#
# Exit codes: 0 = probe ran and the report was written (the branch verdict is
# data, reported honestly); 2 = infrastructure failure (see prereq.json).
[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64.exe",
    [string]$GodotProject = "D:\Godot\project\vit-daw-frontend",
    [string]$KernelExe = "",
    [int]$TimeoutSeconds = 900,
    [int]$ChainTimeoutSeconds = 300,
    [int]$WaitSeconds = 90,
    [switch]$SkipBuild,
    [string[]]$Message = @(),
    [string]$ConversationId = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
function Fail([string]$Text) { throw $Text }
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
function Get-HashOrEmpty([string]$Path) {
    if (Test-Path -LiteralPath $Path) { return (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash }
    return ""
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$GodotExe = (Resolve-Path -LiteralPath $GodotExe).Path
$GodotProject = (Resolve-Path -LiteralPath $GodotProject).Path
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"
}
if (-not (Test-Path -LiteralPath $KernelExe)) { Fail "kernel exe not found: $KernelExe" }

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

$head = (& git -C $RepoRoot rev-parse HEAD | Out-String).Trim()
$statusLines = @(& git -C $RepoRoot status --short | ForEach-Object { [string]$_ })

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$runDir = Join-Path $RepoRoot ("artifacts\b13c_reply_branch\" + $stamp)
$null = New-Item -ItemType Directory -Force -Path $runDir
$report = Join-Path $runDir "b13c_reply_branch_report.json"
$journal = Join-Path $runDir "agent_journal.json"
$agentDir = Join-Path $RepoRoot "agent"
$agentExe = Join-Path $agentDir "bin\VitAgent.b13c.exe"
$vspHubExe = Join-Path $agentDir "bin\VspHub.b13c.exe"
$agentLog = Join-Path $runDir "agent_last.log"
$godotStdout = Join-Path $runDir "godot.stdout.log"
$godotStderr = Join-Path $runDir "godot.stderr.log"
$provenanceReport = Join-Path $runDir "process_provenance.json"
$defaultProject = Join-Path $RepoRoot "VitApp\Workspace\default_project.xml"
$godot = $null; $kernel = $null; $vspHub = $null; $agent = $null
$priorJournal = $env:VIT_AGENT_JOURNAL_PATH
$priorLauncher = $env:VIT_FROM_LAUNCHER
$priorDevRoot = $env:VIT_DAW_DEV_ROOT

try {
    [ordered]@{
        schema_version = "b13c_prereq.v1"
        recorded_at = (Get-Date).ToUniversalTime().ToString("o")
        head = $head
        git_status_short = $statusLines
        repo_default_project_sha256_before = (Get-HashOrEmpty $defaultProject)
        kernel_exe = $KernelExe
        kernel_sha256 = (Get-HashOrEmpty $KernelExe)
    } | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $runDir "prereq.json") -Encoding UTF8

    $env:VIT_AGENT_JOURNAL_PATH = $journal
    $env:VIT_FROM_LAUNCHER = "1"
    $env:VIT_DAW_DEV_ROOT = $RepoRoot

    if (-not $SkipBuild) {
        Push-Location $agentDir
        try {
            & go build -o $agentExe ./cmd/vitagent
            if ($LASTEXITCODE -ne 0) { Fail "VitAgent build failed" }
            & go build -o $vspHubExe ./cmd/vsphub
            if ($LASTEXITCODE -ne 0) { Fail "VspHub build failed" }
        }
        finally { Pop-Location }
    }

    $kernel = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -WindowStyle Hidden -PassThru
    if ($kernel.HasExited) { Fail "kernel exited immediately with code $($kernel.ExitCode)" }
    if (-not (Wait-Port 5555 $WaitSeconds)) { Fail "kernel command port 5555 did not become ready" }
    Assert-PortOwner 5555 $kernel.Id
    $godotArgs = @("--path", $GodotProject)
    $godot = Start-Process -FilePath $GodotExe -ArgumentList $godotArgs -WorkingDirectory $GodotProject -WindowStyle Hidden -RedirectStandardOutput $godotStdout -RedirectStandardError $godotStderr -PassThru
    if (-not (Wait-Port 5556 $WaitSeconds)) { Fail "kernel event port 5556 did not become ready" }
    Assert-PortOwner 5556 $kernel.Id
    $vspHubArgs = @("-last-log-path", (Join-Path $runDir "vsp_hub_last.log"))
    $vspHub = Start-Process -FilePath $vspHubExe -ArgumentList $vspHubArgs -WorkingDirectory $agentDir -WindowStyle Hidden -PassThru
    if ($vspHub.HasExited) { Fail "VspHub exited immediately with code $($vspHub.ExitCode)" }
    if (-not (Wait-Port 8787 $WaitSeconds)) { Fail "VspHub port 8787 did not become ready" }
    Assert-PortOwner 8787 $vspHub.Id
    $agentArgs = @("-http", "127.0.0.1:7878", "-last-log-path", $agentLog)
    $agent = Start-Process -FilePath $agentExe -ArgumentList $agentArgs -WorkingDirectory $agentDir -WindowStyle Hidden -PassThru
    if ($agent.HasExited) { Fail "VitAgent exited immediately with code $($agent.ExitCode)" }
    if (-not (Wait-HttpReady "http://127.0.0.1:7878/health" $WaitSeconds)) { Fail "VitAgent HTTP did not become ready" }
    Assert-PortOwner 7878 $agent.Id
    if (-not (Wait-UdpPort 4445 $WaitSeconds)) { Fail "VitAgent UDP command port 4445 did not become ready" }
    Assert-PortOwner 4445 $agent.Id $true

    [ordered]@{
        schema_version = "b13c_process_provenance.v1"
        repo_root = $RepoRoot
        verified_at = (Get-Date).ToUniversalTime().ToString("o")
        process_ownership_verified = $true
        binaries = @(
            [ordered]@{ role = "kernel"; path = $KernelExe; sha256 = (Get-HashOrEmpty $KernelExe); pid = $kernel.Id },
            [ordered]@{ role = "godot"; path = $GodotExe; sha256 = (Get-HashOrEmpty $GodotExe); pid = $godot.Id },
            [ordered]@{ role = "vsp_hub"; path = $vspHubExe; sha256 = (Get-HashOrEmpty $vspHubExe); pid = $vspHub.Id },
            [ordered]@{ role = "agent"; path = $agentExe; sha256 = (Get-HashOrEmpty $agentExe); pid = $agent.Id }
        )
        ports = @(
            [ordered]@{ protocol = "tcp"; port = 5555; pid = $kernel.Id },
            [ordered]@{ protocol = "tcp"; port = 5556; pid = $kernel.Id },
            [ordered]@{ protocol = "tcp"; port = 8787; pid = $vspHub.Id },
            [ordered]@{ protocol = "tcp"; port = 7878; pid = $agent.Id },
            [ordered]@{ protocol = "udp"; port = 4445; pid = $agent.Id }
        )
    } | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $provenanceReport -Encoding UTF8

    $driverArgs = @(
        (Join-Path $RepoRoot "scripts\b13c_reply_branch_smoke.py"),
        "--agent-http", "http://127.0.0.1:7878",
        "--out-dir", $runDir,
        "--chat-timeout", "$TimeoutSeconds",
        "--chain-timeout", "$ChainTimeoutSeconds"
    )
    if (-not [string]::IsNullOrWhiteSpace($ConversationId)) { $driverArgs += @("--conversation-id", $ConversationId) }
    foreach ($line in $Message) { $driverArgs += @("--message", $line) }
    & python @driverArgs
    $driverExit = $LASTEXITCODE
    Write-Host "B13C_DRIVER_EXIT=$driverExit"
    if ($driverExit -ne 0) { Fail "b13c reply-branch probe failed; inspect $runDir" }
    $projectAfter = Join-Path $runDir "post_project_sha256.txt"
    (Get-HashOrEmpty $defaultProject) | Set-Content -LiteralPath $projectAfter -Encoding UTF8
    Write-Host ("B13C_PASS: " + $report)
    Write-Host ("B13C_ARTIFACTS: " + $runDir)
    $script:exitCode = 0
}
catch {
    Write-Host ("B13C_INFRA_FAILURE: " + $_.Exception.Message)
    $script:exitCode = 2
}
finally {
    if ($null -ne $priorJournal) { $env:VIT_AGENT_JOURNAL_PATH = $priorJournal } else { Remove-Item Env:VIT_AGENT_JOURNAL_PATH -ErrorAction SilentlyContinue }
    if ($null -ne $priorLauncher) { $env:VIT_FROM_LAUNCHER = $priorLauncher } else { Remove-Item Env:VIT_FROM_LAUNCHER -ErrorAction SilentlyContinue }
    if ($null -ne $priorDevRoot) { $env:VIT_DAW_DEV_ROOT = $priorDevRoot } else { Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue }
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
}
exit $script:exitCode

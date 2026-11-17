# 388 hygiene wrapper over the existing A/B smoke mechanisms. No product probes here.
[CmdletBinding()]
param(
    [ValidateSet('A','B','C','Guard')][string]$Leg,
    [Parameter(Mandatory=$true)][string]$RunRoot,
    [string]$FrontendWorktree = 'C:/Users/timoz/.codex/worktrees/godot-runtime-388/vit-daw-frontend',
    [string]$FixtureSource = 'D:/Vit_DAW/temp/f3_empty_project.vit',
    [int]$AllowedRootPid = 0,
    [int]$AllowedKernelPid = 0,
    [int]$InjectedBoundAgentPort = 0,
    [int]$InjectionOwnerPid = 0,
    [ValidateSet('CONVERGE-ENTRY-1','SHELL-WIN-BEHAVIOR-1')][string]$CardId = 'CONVERGE-ENTRY-1',
    [switch]$WindowBehaviorChecks,
    [ValidateSet('normal','delayed_success','scene_failed','snapshot_failed','workspace_timeout','exit_before_ready')][string]$PresentationScenario = 'normal',
    [string]$PreviousLaunchId = ''
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot

function Assert-EntryLeaseAndStack {
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    & git -C $repo fetch origin main 2>&1 | Out-File (Join-Path $RunRoot 'lease_fetch.log') -Append -Encoding utf8
    $fetchExit = $LASTEXITCODE
    $lease = @(& git -C $repo show origin/main:coord/resources/PC-RUNTIME-STACK.md)
    $showExit = $LASTEXITCODE
    $ErrorActionPreference = $previousPreference
    if ($fetchExit -ne 0 -or $showExit -ne 0) { throw 'ENV-INTERRUPT: lease fetch/read failed' }
    $leaseHead = ($lease | Select-Object -First 4) -join "`n"
    $leaseHead | Out-File (Join-Path $RunRoot 'lease_head.log') -Append -Encoding utf8
    $occupiedMarker = [string][char]0x5360 + [char]0x7528
    if ($leaseHead -notmatch ($occupiedMarker + '.*' + [regex]::Escape($CardId)) -or $leaseHead -notmatch 'owner=Codex /root/entry388') { throw 'ENV-INTERRUPT: current remote stack owner/card is not this execution' }
    $processes = @(Get-CimInstance Win32_Process)
    $allowed = @{}
    if ($AllowedRootPid -gt 0) {
        $rootProcess = $processes | Where-Object ProcessId -eq $AllowedRootPid
        $expectedExe = [IO.Path]::GetFullPath((Join-Path $repo 'agent/bin/VitAgent.exe'))
        if ($null -eq $rootProcess -or "$($rootProcess.ExecutablePath)" -ne $expectedExe) { throw 'ENV-INTERRUPT: allowed agent PID does not match this worktree binary' }
        $allowed[$AllowedRootPid] = $true
        do {
            $changed = $false
            foreach ($process in $processes) {
                if ($allowed.ContainsKey([int]$process.ParentProcessId) -and -not $allowed.ContainsKey([int]$process.ProcessId)) { $allowed[[int]$process.ProcessId] = $true; $changed = $true }
            }
        } while ($changed)
    }
    if ($AllowedKernelPid -gt 0) {
        $kernelProcess = $processes | Where-Object ProcessId -eq $AllowedKernelPid
        $expectedKernel = [IO.Path]::GetFullPath((Join-Path $repo 'VitApp/build/VitApp_artefacts/Release/VitApp.exe'))
        if ($null -eq $kernelProcess -or "$($kernelProcess.ExecutablePath)" -ne $expectedKernel) { throw 'ENV-INTERRUPT: allowed kernel PID does not match this worktree binary' }
        $allowed[$AllowedKernelPid] = $true
    }
    $foreign = @($processes | Where-Object { $_.Name -match '^(VitShell|VitAgent|VitApp|VspHub|Godot.*)\.exe$' -and -not $allowed.ContainsKey([int]$_.ProcessId) })
    if ($foreign.Count) { throw ('ENV-INTERRUPT: foreign Vit stack found, refusing reuse/kill: ' + (($foreign | ForEach-Object { "$($_.Name):$($_.ProcessId)" }) -join ',')) }
    $listeners = @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -in @(7878,7879,8787,5555,5556,9333) })
    foreach ($listener in $listeners) { if (-not $allowed.ContainsKey([int]$listener.OwningProcess)) { throw "ENV-INTERRUPT: TCP $($listener.LocalPort) already listening pid=$($listener.OwningProcess)" } }
    foreach ($udp in @(Get-NetUDPEndpoint -LocalPort 4445 -ErrorAction SilentlyContinue)) { if (-not $allowed.ContainsKey([int]$udp.OwningProcess)) { throw "ENV-INTERRUPT: UDP4445 occupied pid=$($udp.OwningProcess)" } }
    if ($InjectedBoundAgentPort -gt 0) {
        $bound = @(Get-NetTCPConnection -LocalPort $InjectedBoundAgentPort -ErrorAction SilentlyContinue | Where-Object { $_.State -eq 'Bound' })
        if ($InjectionOwnerPid -le 0 -or $bound.Count -eq 0 -or @($bound | Where-Object { [int]$_.OwningProcess -ne $InjectionOwnerPid }).Count) { throw 'ENV-INTERRUPT: F2 socket is not the known script-owned bound injection' }
    }
    $snapshot = @{ at=(Get-Date).ToString('o'); allowed_root_pid=$AllowedRootPid; listeners=@($listeners | Select-Object LocalPort,OwningProcess); foreign_count=$foreign.Count }
    $snapshot | ConvertTo-Json -Depth 4 | Out-File (Join-Path $RunRoot 'immediate_guard.jsonl') -Append -Encoding utf8
}

if ($Leg -eq 'Guard') {
    New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
    Assert-EntryLeaseAndStack
    return
}
if (Test-Path -LiteralPath $RunRoot) { throw 'RunRoot already exists; use a new run directory' }
New-Item -ItemType Directory -Path $RunRoot | Out-Null
Assert-EntryLeaseAndStack # Refuse foreign ownership before taking restorable runtime snapshots.
$snapshots = @()
$paths = @((Join-Path $env:APPDATA 'Vit-DAW/global_project_config.json'))
if ($WindowBehaviorChecks) { $paths += Join-Path $env:APPDATA 'Godot/app_userdata/Vit_DAW_Frontend/logs/godot.log' }
foreach ($base in @($repo, 'D:/Vit_DAW')) { foreach ($relative in @('VitApp/Workspace/Settings/Settings.xml', 'VitApp/Workspace/default_project.xml')) { $paths += Join-Path $base $relative } }
foreach ($path in $paths) {
    $exists = Test-Path -LiteralPath $path
    $backup = Join-Path $RunRoot ("before_$($snapshots.Count).bin")
    $hash = 'ABSENT'
    if ($exists) { Copy-Item -LiteralPath $path -Destination $backup; $hash = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash }
    $snapshots += @{ path=$path; existed=$exists; backup=$backup; before_hash=$hash }
}
$fixtureHash = (Get-FileHash -LiteralPath $FixtureSource -Algorithm SHA256).Hash
$snapshots | ConvertTo-Json -Depth 4 | Set-Content (Join-Path $RunRoot 'before_manifest.json') -Encoding utf8
$innerExit = -1
$wrapperError = ''
$restoreErrors = @()
try {
    Assert-EntryLeaseAndStack
    $env:VIT_AGENT_LLM_BASE_URL = 'http://127.0.0.1:1'
    $device = Join-Path $RunRoot 'device'
    # Native stderr is evidence, not a PowerShell terminating NativeCommandError.
    $ErrorActionPreference = 'Continue'
    if ($Leg -eq 'A') {
        $deviceArgs = @('-NoProfile','-ExecutionPolicy','Bypass','-File',(Join-Path $PSScriptRoot 'vitshell_full_smoke.ps1'),'-FrontendWorktree',$FrontendWorktree,'-FixtureSource',$FixtureSource,'-CardId',$CardId,'-Rounds','1','-EntryManagementChecks','-RunRoot',$device)
        if ($WindowBehaviorChecks) {
            $deviceArgs += @('-WindowBehaviorChecks','-PresentationScenario',$PresentationScenario)
            if ($PreviousLaunchId) { $deviceArgs += @('-PreviousLaunchId',$PreviousLaunchId) }
        }
        & powershell @deviceArgs *> (Join-Path $RunRoot 'stdout.log')
    } elseif ($Leg -eq 'B') {
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'ui_entry2_smoke.ps1') -RepoRoot $repo -GodotProjectRoot $FrontendWorktree -ProjectSource $FixtureSource -KernelExe (Join-Path $repo 'VitApp/build/VitApp_artefacts/Release/VitApp.exe') -EntryManagementChecks -RunRoot $device *> (Join-Path $RunRoot 'stdout.log')
    } else {
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'ui_entry_smoke.ps1') -RepoRoot $repo -GodotProjectRoot $FrontendWorktree -ProjectSource $FixtureSource -KernelExe (Join-Path $repo 'VitApp/build/VitApp_artefacts/Release/VitApp.exe') -AgentBinary (Join-Path $repo 'agent/bin/VitAgent.exe') -EntryManagementChecks -EntryCardId $CardId -RunRoot $device *> (Join-Path $RunRoot 'stdout.log')
    }
    $innerExit = $LASTEXITCODE
    $ErrorActionPreference = 'Stop'
} catch { $wrapperError = $_.Exception.Message }
finally {
    $ErrorActionPreference = 'Stop'
    Remove-Item Env:VIT_AGENT_LLM_BASE_URL -ErrorAction SilentlyContinue
    foreach ($snapshot in $snapshots) {
        try {
            if ($WindowBehaviorChecks -and $snapshot.path -like '*Godot*logs*godot.log' -and (Test-Path -LiteralPath $snapshot.path)) {
                Copy-Item -LiteralPath $snapshot.path -Destination (Join-Path $RunRoot 'godot_before_restore.log') -Force
            }
            if ($snapshot.existed) { Copy-Item -LiteralPath $snapshot.backup -Destination $snapshot.path -Force }
            elseif (Test-Path -LiteralPath $snapshot.path) { Remove-Item -LiteralPath $snapshot.path -Force }
            $after = if (Test-Path -LiteralPath $snapshot.path) { (Get-FileHash -LiteralPath $snapshot.path -Algorithm SHA256).Hash } else { 'ABSENT' }
            $snapshot.after_hash = $after
            if ($after -ne $snapshot.before_hash) { throw "restore mismatch $($snapshot.path)" }
        } catch { $restoreErrors += $_.Exception.Message }
    }
    $fixtureAfter = (Get-FileHash -LiteralPath $FixtureSource -Algorithm SHA256).Hash
    if ($fixtureAfter -ne $fixtureHash) { $restoreErrors += 'fixture source changed; not restored or hidden' }
    @{ leg=$Leg; inner_exit=$innerExit; wrapper_error=$wrapperError; restore_errors=$restoreErrors; snapshots=$snapshots; fixture_before=$fixtureHash; fixture_after=$fixtureAfter } | ConvertTo-Json -Depth 6 | Set-Content (Join-Path $RunRoot 'hygiene.json') -Encoding utf8
}
if ($innerExit -ne 0 -or $wrapperError -or $restoreErrors.Count) { Write-Error "FAIL: inner=$innerExit wrapper=$wrapperError restore=$($restoreErrors -join ';')"; exit 1 }
Write-Output "PASS: $Leg inner exit0, outer hygiene verified"
exit 0

# run_tracklist_fix_probe.ps1 — FIX-TRACKLIST-SHADOW-FRESHNESS-1 (143) fix-acceptance stack berth.
#
# Berth discipline copied verbatim from the 139 runner (run_tracklist_gap_probe.ps1):
#   - one stack owner: refuse if any stack port/process is already live;
#   - run-dir kernel workspace with VitPaths root markers + VIT_PROJECT_XML redirect so
#     Workspace/Logs/Cache never land in the shared deployment tree;
#   - kernel + vsphub + agent (no Godot UI), VSP hub on the default 8787 endpoint so the
#     agent's canonical command path is the real hub->kernel path;
#   - tear down only the pids this run started, then re-verify ports/processes = 0.
# New script (does not modify existing scripts).
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\coord\runs\FIX-TRACKLIST-SHADOW-FRESHNESS-1\run_tracklist_fix_probe.ps1 `
#       -RepoRoot D:\Vit_DAW_wt_trackfix1 -KernelExe D:\Vit_DAW\Export\staging\runtime\VitApp.exe

[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW_wt_trackfix1",
    [string]$KernelExe = "D:\Vit_DAW\Export\staging\runtime\VitApp.exe",
    [string]$SettingsSource = "D:\Vit_DAW\Export\staging\runtime\Workspace\Settings",
    [string]$RunDir = "",
    [string]$Python = "python",
    [switch]$SkipBuild,
    [switch]$AllowOutOfScopeGodot,
    [switch]$PreflightOnly,
    [int]$KernelStartTimeoutSeconds = 180,
    [int]$AgentStartTimeoutSeconds = 120
)

$ErrorActionPreference = "Stop"

$stackPorts = @(5555, 5556, 5557, 7878, 8787, 4445)
$agentExe = Join-Path $RepoRoot "agent\bin\VitAgent.exe"
$hubExe = Join-Path $RepoRoot "agent\bin\vsphub.exe"

function Write-Step([string]$text) { Write-Host ("`n== " + $text) }
function Write-Ok([string]$text) { Write-Host ("ok: " + $text) }
function Write-WarnLine([string]$text) { Write-Host ("warn: " + $text) -ForegroundColor Yellow }

function Get-StackListeners {
    $found = @()
    foreach ($port in $stackPorts) {
        try {
            $listeners = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction Stop
            foreach ($listener in $listeners) {
                $found += [pscustomobject]@{ port = $port; pid = $listener.OwningProcess }
            }
        }
        catch { }
    }
    return $found
}

function Get-StackProcesses {
    $rows = @()
    $cims = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
            $_.Name -eq "VitApp.exe" -or $_.Name -eq "VitAgent.exe" -or $_.Name -eq "vsphub.exe" -or $_.Name -like "Godot*"
        })
    foreach ($c in $cims) {
        $isGodot = ($c.Name -like "Godot*")
        # Out-of-scope Godot helpers: the user's editor window, and headless/import helper
        # processes (asset import, never binds the stack ports). A windowed DAW frontend
        # (not --editor / --headless / --import) stays a hard stack blocker.
        $outOfScopeGodot = $isGodot -and ($c.CommandLine -imatch "--editor" -or
                                          $c.CommandLine -imatch "--headless" -or
                                          $c.CommandLine -imatch "--import")
        $rows += [pscustomobject]@{
            Id               = [int]$c.ProcessId
            ProcessName      = $c.Name
            CommandLine      = $c.CommandLine
            IsEditor         = ($isGodot -and $c.CommandLine -imatch "--editor")
            OutOfScopeGodot  = $outOfScopeGodot
        }
    }
    return @($rows | Sort-Object Id -Unique)
}

function Test-PortListening([int]$Port, [int]$TimeoutMs = 500) {
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $async = $client.BeginConnect("127.0.0.1", $Port, $null, $null)
        if (-not $async.AsyncWaitHandle.WaitOne($TimeoutMs)) { return $false }
        $client.EndConnect($async)
        return $true
    }
    catch { return $false }
    finally { $client.Close() }
}

function Stop-OwnPid([string]$Name, [Nullable[int]]$ProcId) {
    if ($null -eq $ProcId -or $ProcId -le 0) { return }
    try {
        $proc = Get-Process -Id $ProcId -ErrorAction Stop
        Stop-Process -Id $ProcId -Force -ErrorAction Stop
        $proc.WaitForExit(10000) | Out-Null
        Write-Ok ("stopped " + $Name + " pid=" + $ProcId)
    }
    catch {
        Write-WarnLine ("could not stop " + $Name + " pid=" + $ProcId + ": " + $_.Exception.Message)
    }
}

function New-BlankStartupProject([string]$Path) {
    $xml = @"
<?xml version="1.0" encoding="UTF-8"?>
<EDIT appVersion="Unknown" projectID="0/tlprobe0" creationTime="0" modifiedBy="tracklist_probe">
  <TRANSPORT endToEnd="1" position="0.0"/>
  <TEMPOSEQUENCE>
    <TEMPO startBeat="0.0" bpm="120.0" curve="1.0"/>
    <TIMESIG numerator="4" denominator="4" startBeat="0.0"/>
  </TEMPOSEQUENCE>
  <VIDEO/>
  <CLICKTRACK level="0.6000000238418579"/>
  <MASTERVOLUME>
    <PLUGIN type="volume" id="1001" enabled="1" volume="0.7408182206817179">
      <MODIFIERASSIGNMENTS/>
    </PLUGIN>
  </MASTERVOLUME>
  <RACKS/>
  <MASTERPLUGINS/>
  <INPUTDEVICES>
    <INPUTDEVICE deviceID="all_midi_in" name="All MIDI Ins"/>
  </INPUTDEVICES>
  <TRACKCOMPS/>
  <ARRANGERTRACK name="Arranger" id="1002"/>
  <CHORDTRACK name="Chord" id="1003"/>
  <MARKERTRACK name="Marker" id="1004"/>
  <TEMPOTRACK name="Global" id="1005">
    <MODIFIERS/>
  </TEMPOTRACK>
  <MASTERTRACK name="Master" id="1006">
    <MODIFIERS/>
  </MASTERTRACK>
  <SCENES/>
</EDIT>
"@
    [System.IO.File]::WriteAllText($Path, $xml, (New-Object System.Text.UTF8Encoding($false)))
}

$exitCode = 0
$kernelProc = $null
$agentProc = $null
$hubProc = $null
$kernelPid = $null
$agentPid = $null
$hubPid = $null
$RunDir = ""

try {
    Write-Step "Preflight: ports and processes (AGENTS.md section 9 / PC-RUNTIME-STACK)"
    if (-not (Test-Path -LiteralPath $KernelExe)) { throw "kernel exe not found: $KernelExe" }
    $KernelExe = (Resolve-Path -LiteralPath $KernelExe).Path

    $listeners = Get-StackListeners
    if ($listeners.Count -gt 0) {
        $listeners | Format-Table -AutoSize | Out-String | Write-Host
        throw "stack ports already listening; another owner holds the stack (AGENTS.md section 9)"
    }
    $procs = Get-StackProcesses
    $blockingProcs = @($procs | Where-Object { -not $_.OutOfScopeGodot })
    $editorProcs = @($procs | Where-Object { $_.IsEditor })
    $outOfScopeGodot = @($procs | Where-Object { $_.OutOfScopeGodot })
    if ($blockingProcs.Count -gt 0) {
        $blockingProcs | Select-Object Id, ProcessName, CommandLine | Format-Table -AutoSize | Out-String | Write-Host
        throw "stack processes already running; another owner holds the stack (AGENTS.md section 9)"
    }
    if ($outOfScopeGodot.Count -gt 0) {
        $outOfScopeGodot | Select-Object Id, ProcessName, CommandLine | Format-Table -AutoSize | Out-String | Write-Host
        if (-not $AllowOutOfScopeGodot) {
            throw ("out-of-scope Godot helper processes present (editor/headless/import, no stack ports): pid=" + (($outOfScopeGodot | ForEach-Object { $_.Id }) -join ",") + "; re-run with -AllowOutOfScopeGodot or wait for them to finish")
        }
        Write-WarnLine ("out-of-scope Godot helper processes allowed by -AllowOutOfScopeGodot (never touched): pid=" + (($outOfScopeGodot | ForEach-Object { $_.Id }) -join ","))
    }
    Write-Ok "ports 5555/5556/5557/7878/8787/4445 listening=0; VitApp/VitAgent/vsphub/Godot-frontend=0"
    if ($PreflightOnly) {
        Write-Ok "preflight-only mode: preflight passed; NO build, NO stack start, NO teardown needed"
        $preflightOnlyMode = $true
        return
    }

    # ---- build agent + vsp hub (worktree binaries) ---------------------
    if (-not $SkipBuild -or -not (Test-Path -LiteralPath $agentExe) -or -not (Test-Path -LiteralPath $hubExe)) {
        Write-Step "Build VitAgent + VspHub"
        New-Item -ItemType Directory -Path (Join-Path $RepoRoot "agent\bin") -Force | Out-Null
        Push-Location (Join-Path $RepoRoot "agent")
        try {
            & go build -o $agentExe .\cmd\vitagent
            if ($LASTEXITCODE -ne 0) { throw "go build vitagent failed with exit code $LASTEXITCODE" }
            & go build -o $hubExe .\cmd\vsphub
            if ($LASTEXITCODE -ne 0) { throw "go build vsphub failed with exit code $LASTEXITCODE" }
            Write-Ok ("built " + $agentExe + " and " + $hubExe)
        }
        finally { Pop-Location }
    }

    # ---- run dir + isolated kernel workspace ---------------------------
    if ([string]::IsNullOrWhiteSpace($RunDir)) {
        $RunDir = Join-Path $RepoRoot ("coord\runs\FIX-TRACKLIST-SHADOW-FRESHNESS-1\run_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
    }
    if (-not [System.IO.Path]::IsPathRooted($RunDir)) {
        $RunDir = [System.IO.Path]::GetFullPath((Join-Path (Get-Location).Path $RunDir))
    }
    if (Test-Path -LiteralPath $RunDir) { throw "run artifacts directory already exists: $RunDir" }
    $KernelWorkspace = Join-Path $RunDir "kernel_workspace"
    $KernelWorkspaceSub = Join-Path $KernelWorkspace "Workspace"
    foreach ($dir in @($RunDir, $KernelWorkspace, $KernelWorkspaceSub,
                       (Join-Path $KernelWorkspaceSub "Settings"),
                       (Join-Path $KernelWorkspaceSub "Logs"),
                       (Join-Path $KernelWorkspace "Source"),
                       (Join-Path $RunDir "agent_drafts"),
                       (Join-Path $RunDir "BridgeReplies"),
                       (Join-Path $RunDir "raw"))) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    New-Item -ItemType File -Path (Join-Path $KernelWorkspace "CMakeLists.txt") -Force | Out-Null
    if (Test-Path -LiteralPath $SettingsSource) {
        $settingsTargetDir = Join-Path $KernelWorkspace "Workspace\Settings"
        Get-ChildItem -LiteralPath $SettingsSource -File | ForEach-Object {
            Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $settingsTargetDir $_.Name) -Force
        }
        Write-Ok ("copied kernel Settings cache from " + $SettingsSource)
    }
    $startupProject = Join-Path $RunDir "startup_default_project.xml"
    New-BlankStartupProject -Path $startupProject
    Write-Ok ("run dir: " + $RunDir)

    $kernelSha = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
    $agentSha = if (Test-Path -LiteralPath $agentExe) { (Get-FileHash -LiteralPath $agentExe -Algorithm SHA256).Hash } else { "" }
    $hubSha = if (Test-Path -LiteralPath $hubExe) { (Get-FileHash -LiteralPath $hubExe -Algorithm SHA256).Hash } else { "" }
    [pscustomobject]@{
        started = (Get-Date).ToString("o")
        repo_root = $RepoRoot
        kernel_exe = $KernelExe
        kernel_sha256 = $kernelSha
        agent_exe = $agentExe
        agent_sha256 = $agentSha
        vsphub_exe = $hubExe
        vsphub_sha256 = $hubSha
        worktree_head = (& git -C $RepoRoot rev-parse HEAD 2>$null)
        worktree_status_short = (& git -C $RepoRoot status --short 2>$null | Out-String)
        python = $Python
        machine = $env:COMPUTERNAME
        stack_ports = $stackPorts -join ","
        out_of_scope_godot_editor_pids = (($editorProcs | ForEach-Object { $_.Id }) -join ",")
        out_of_scope_godot_helper_pids = (($outOfScopeGodot | ForEach-Object { $_.Id }) -join ",")
    } | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath (Join-Path $RunDir "run_meta.json") -Encoding UTF8

    # ---- start kernel --------------------------------------------------
    Write-Step "Start kernel"
    $kernelStdout = Join-Path $RunDir "kernel_stdout.log"
    $kernelStderr = Join-Path $RunDir "kernel_stderr.log"
    $priorProjectXml = [System.Environment]::GetEnvironmentVariable("VIT_PROJECT_XML", "Process")
    $env:VIT_PROJECT_XML = $startupProject
    try {
        $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory $KernelWorkspace `
            -WindowStyle Hidden -PassThru `
            -RedirectStandardOutput $kernelStdout -RedirectStandardError $kernelStderr
    }
    finally {
        if ([string]::IsNullOrWhiteSpace($priorProjectXml)) { Remove-Item Env:VIT_PROJECT_XML -ErrorAction SilentlyContinue }
        else { $env:VIT_PROJECT_XML = $priorProjectXml }
    }
    $kernelPid = $kernelProc.Id
    Write-Ok ("started kernel pid=" + $kernelPid + " workdir=" + $KernelWorkspace)
    $deadline = (Get-Date).AddSeconds($KernelStartTimeoutSeconds)
    while ($true) {
        if ($kernelProc.HasExited) { throw ("kernel exited early with code " + $kernelProc.ExitCode) }
        if (Test-PortListening 5555) { break }
        if ((Get-Date) -gt $deadline) { throw "kernel 5555 not listening within timeout" }
        Start-Sleep -Milliseconds 500
    }
    Write-Ok "kernel ZMQ 5555 listening"

    # ---- start vsphub --------------------------------------------------
    Write-Step "Start VspHub"
    $hubStdout = Join-Path $RunDir "vsphub_stdout.log"
    $hubStderr = Join-Path $RunDir "vsphub_stderr.log"
    $hubProc = Start-Process -FilePath $hubExe -WorkingDirectory $RunDir -WindowStyle Hidden -PassThru `
        -ArgumentList @("-listen", "127.0.0.1:8787",
                        "-kernel-req", "tcp://127.0.0.1:5555",
                        "-kernel-sub", "tcp://127.0.0.1:5556",
                        "-last-log-path", (Join-Path $RunDir "vsphub_last.log")) `
        -RedirectStandardOutput $hubStdout -RedirectStandardError $hubStderr
    $hubPid = $hubProc.Id
    Write-Ok ("started vsphub pid=" + $hubPid)
    $deadline = (Get-Date).AddSeconds(60)
    while ($true) {
        if ($hubProc.HasExited) { throw ("vsphub exited early with code " + $hubProc.ExitCode) }
        if (Test-PortListening 8787) { break }
        if ((Get-Date) -gt $deadline) { throw "vsphub 8787 not listening within timeout" }
        Start-Sleep -Milliseconds 300
    }
    Write-Ok "vsphub HTTP 8787 listening"

    # ---- start agent ---------------------------------------------------
    Write-Step "Start VitAgent"
    $agentStdout = Join-Path $RunDir "agent_stdout.log"
    $agentStderr = Join-Path $RunDir "agent_stderr.log"
    $priorDraftRoot = [System.Environment]::GetEnvironmentVariable("VIT_HISTORY_DRAFT_ROOT", "Process")
    $env:VIT_HISTORY_DRAFT_ROOT = (Join-Path $RunDir "agent_drafts")
    try {
        $agentProc = Start-Process -FilePath $agentExe -WorkingDirectory (Join-Path $RepoRoot "agent") -WindowStyle Hidden -PassThru `
            -ArgumentList @("-http", "127.0.0.1:7878",
                            "-zmq-req-url", "tcp://127.0.0.1:5555",
                            "-zmq-sub-url", "tcp://127.0.0.1:5556",
                            "-vsp-hub-url", "http://127.0.0.1:8787/vsp",
                            "-last-log-path", (Join-Path $RunDir "agent_last.log"),
                            "-file-reply-dir", (Join-Path $RunDir "BridgeReplies")) `
            -RedirectStandardOutput $agentStdout -RedirectStandardError $agentStderr
    }
    finally {
        if ([string]::IsNullOrWhiteSpace($priorDraftRoot)) { Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue }
        else { $env:VIT_HISTORY_DRAFT_ROOT = $priorDraftRoot }
    }
    $agentPid = $agentProc.Id
    Write-Ok ("started agent pid=" + $agentPid)
    $deadline = (Get-Date).AddSeconds($AgentStartTimeoutSeconds)
    $agentReady = $false
    while ((Get-Date) -lt $deadline) {
        if ($agentProc.HasExited) { throw ("agent exited early with code " + $agentProc.ExitCode + "; see " + $agentStdout) }
        try {
            $health = Invoke-RestMethod -Uri "http://127.0.0.1:7878/health" -TimeoutSec 2
            if ($null -ne $health) { $agentReady = $true; break }
        }
        catch { }
        Start-Sleep -Milliseconds 500
    }
    if (-not $agentReady) { throw "agent /health not ready within timeout" }
    Write-Ok "agent HTTP 7878 ready"
    Start-Sleep -Seconds 2  # let the VSP hub registration settle

    # ---- diagnostic probe ---------------------------------------------
    Write-Step "track.list same-frame probe (python)"
    $probeScript = Join-Path $PSScriptRoot "tracklist_fix_probe.py"
    if (-not (Test-Path -LiteralPath $probeScript)) { throw "probe script missing: $probeScript" }
    $summaryPath = Join-Path $RunDir "probe_summary.json"
    $consolePath = Join-Path $RunDir "probe_console.txt"
    & $Python -u $probeScript --output $summaryPath --kernel-pid $kernelPid --startup-project $startupProject 2>&1 |
        Tee-Object -FilePath $consolePath
    $probeExit = $LASTEXITCODE
    Write-Ok ("probe exit code: " + $probeExit)
    if ($probeExit -ne 0) { $exitCode = 1 }
}
catch {
    Write-Host ("FAIL: " + $_.Exception.Message) -ForegroundColor Red
    $exitCode = 1
}
finally {
    Write-Step "Teardown (only the pids this run started)"
    Stop-OwnPid -Name "agent" -ProcId $agentPid
    Stop-OwnPid -Name "vsphub" -ProcId $hubPid
    Stop-OwnPid -Name "kernel" -ProcId $kernelPid
    Start-Sleep -Seconds 2
    for ($i = 0; $i -lt 20; $i++) {
        if (-not (Test-PortListening 5555) -and -not (Test-PortListening 7878) -and -not (Test-PortListening 8787)) { break }
        Start-Sleep -Milliseconds 500
    }
    $remainingListeners = Get-StackListeners
    $remainingProcs = @(Get-StackProcesses | Where-Object { -not $_.OutOfScopeGodot })
    $remainingEditors = @(Get-StackProcesses | Where-Object { $_.OutOfScopeGodot })
    if ($remainingListeners.Count -gt 0) {
        $remainingListeners | Format-Table -AutoSize | Out-String | Write-Host
        Write-WarnLine "stack ports still listening after teardown"
        $exitCode = 1
    }
    else { Write-Ok "ports 5555/5556/5557/7878/8787/4445 listening=0 after teardown" }
    if ($remainingProcs.Count -gt 0) {
        $remainingProcs | Select-Object Id, ProcessName | Format-Table -AutoSize | Out-String | Write-Host
        Write-WarnLine "stack processes still present after teardown"
        $exitCode = 1
    }
    else { Write-Ok "VitApp/VitAgent/vsphub/Godot-frontend processes=0 after teardown" }
    if ($remainingEditors.Count -gt 0) {
        Write-Ok ("user Godot editor still present (out of scope, untouched): pid=" + (($remainingEditors | ForEach-Object { $_.Id }) -join ","))
    }

    # ---- post-run evidence collection ---------------------------------
    if (-not [string]::IsNullOrWhiteSpace($RunDir) -and (Test-Path -LiteralPath $RunDir)) {
        $kernelLog = Get-ChildItem (Join-Path $RunDir "kernel_workspace\Workspace\Logs") -Filter "VitHeadlessServer*.log" -File -ErrorAction SilentlyContinue |
            Sort-Object LastWriteTime | Select-Object -Last 1
        if ($null -ne $kernelLog) {
            $vspLines = @(Select-String -Path $kernelLog.FullName -Pattern 'vsp_version' -ErrorAction SilentlyContinue)
            $legacyTrackLines = @(Select-String -Path $kernelLog.FullName -Pattern '"cmd": "list_tracks"' -ErrorAction SilentlyContinue)
            $renderLines = @()
            [pscustomobject]@{
                kernel_session_log = $kernelLog.FullName
                vsp_envelope_lines = $vspLines.Count
                legacy_list_tracks_lines = $legacyTrackLines.Count
            } | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath (Join-Path $RunDir "kernel_vsp_evidence.json") -Encoding UTF8
            Write-Ok ("kernel VSP evidence: vsp_envelope_lines=" + $vspLines.Count + " legacy_list_tracks_lines=" + $legacyTrackLines.Count)
        }
        [pscustomobject]@{
            finished = (Get-Date).ToString("o")
            kernel_pid = $kernelPid
            vsphub_pid = $hubPid
            agent_pid = $agentPid
            exit_code = $exitCode
            probe_summary = (Join-Path $RunDir "probe_summary.json")
            remaining_listeners = $remainingListeners.Count
            remaining_processes = $remainingProcs.Count
            out_of_scope_godot_editors = @($remainingEditors | ForEach-Object { $_.Id })
        } | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath (Join-Path $RunDir "teardown.json") -Encoding UTF8
        Write-Ok ("artifacts: " + $RunDir)
    }
}

exit $exitCode

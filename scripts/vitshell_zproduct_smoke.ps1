# SHELL-WIN-ZPRODUCT-1 companion to vitshell_open_chain_smoke.ps1.
# Explicit opt-in, one natural foreground condition, one open, zero chat/LLM.
# Preparation never calls this runtime leg. PASS applies only to stated samples
# and the existing own-thread subclass interval, never to rendered frames.
param(
    [switch]$RuntimeAuthorized,
    [string]$LeaseOwner,
    [ValidateSet('shell','external')][string]$Condition = 'shell',
    [string]$FrontendWorktree,
    [string]$StageRoot,
    [string]$GodotProjectStage,
    [string]$ExpectedExtensionSha256,
    [string]$GodotExe = 'D:/Godot/Godot_v4.6.1-stable_win64.exe',
    [string]$FixtureSource = 'D:/Vit_DAW/temp/f3_empty_project.vit',
    [string]$RunRoot,
    [ValidateRange(0,180)][int]$NaturalConditionWaitSec = 0,
    [ValidateRange(0,2147483647)][int]$ExpectedExternalPid = 0,
    [ValidateRange(10,240)][int]$TimeoutSec = 120
)
$ErrorActionPreference = 'Stop'
$MainRoot = Split-Path -Parent $PSScriptRoot
if (-not $RuntimeAuthorized) { throw '409 runtime not authorized: explicit -RuntimeAuthorized and synced unique runtime lease required' }
foreach ($required in @('LeaseOwner','FrontendWorktree','StageRoot','ExpectedExtensionSha256','RunRoot')) {
    if ([string]::IsNullOrWhiteSpace((Get-Variable $required -ValueOnly))) { throw "missing -$required" }
}
if (Test-Path -LiteralPath $RunRoot) { throw 'RunRoot must be new (no overwrite/retry)' }
New-Item -ItemType Directory -Path $RunRoot | Out-Null
$RunRoot = (Resolve-Path -LiteralPath $RunRoot).Path
$report = [ordered]@{ card='SHELL-WIN-ZPRODUCT-1'; run_id=(Split-Path -Leaf $RunRoot); condition=$Condition
    command_line=[Environment]::CommandLine; invocation=$MyInvocation.Line; parameters=$PSBoundParameters
    started_utc=[DateTime]::UtcNow.ToString('o'); result='environment_failure'; llm_calls='not_yet_verified'
    coverage='DOM open chain, bound native first-show/correction, existing own-thread message interval, 10ms polling (not frames; gaps unobserved)' }
$snapshots = @(); $savedEnv = @{}; $shell = $null; $lastStatus = $null; $observerStarted = $false; $owned = @{}; $teardownErrors = @(); $exitCode = 2
function Save-Json($Value,$Name) { ConvertTo-Json -InputObject $Value -Depth 20 | Out-File -LiteralPath (Join-Path $RunRoot $Name) -Encoding utf8 }
function Hash($Path) { if (Test-Path -LiteralPath $Path -PathType Leaf) { (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash } else { 'absent' } }
function Git-Read([string[]]$Arguments) {
    $previous=$ErrorActionPreference; $ErrorActionPreference='Continue'
    $previousEncoding=[Console]::OutputEncoding
    try { [Console]::OutputEncoding=[Text.UTF8Encoding]::new($false); $output = & git @Arguments 2>&1; $gitExit=$LASTEXITCODE }
    finally { $ErrorActionPreference=$previous; [Console]::OutputEncoding=$previousEncoding }
    if ($gitExit -ne 0) { throw "git $Arguments failed (exit $gitExit): $output" }
    return ($output -join "`n")
}
function Assert-Lease {
    # Fetch only; never checkout/merge/push or acquire a lease on the caller's behalf.
    $null = Git-Read -Arguments @('-C',$MainRoot,'fetch','origin','main')
    $remoteHead = Git-Read -Arguments @('-C',$MainRoot,'rev-parse','origin/main')
    $resource = Git-Read -Arguments @('-C',$MainRoot,'show','origin/main:coord/resources/PC-RUNTIME-STACK.md')
    $resource | Out-File -LiteralPath (Join-Path $RunRoot 'remote_resource.md') -Encoding utf8
    $latest = ($resource -split "`n" | Where-Object { $_ -match '^- ' } | Select-Object -First 1)
    # The action belongs to the timestamped event header, not its teardown prose.
    $eventHeader = ($latest -split '/',2)[0]
    if ($eventHeader -notmatch '^-\s+\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}(?::\d{2})?\s+[+-]\d{2}:?\d{2}\s+(预留|占用)\s*$' -or $latest -match '无.*启动权' -or
        $latest -notmatch 'SHELL-WIN-ZPRODUCT-1' -or $latest -notmatch ('owner=.*'+[regex]::Escape($LeaseOwner)+'[ `；;,（(]') -or ([regex]::Matches($latest,'owner=')).Count -ne 1) {
        throw 'latest remote resource event does not grant this exact 409 runtime owner'
    }
    $report.remote_head = $remoteHead; $report.lease = $latest
    return [DateTime]::UtcNow
}
function Get-Clearance {
    $processes = @(Get-CimInstance Win32_Process | Where-Object { $_.Name -match '^(Vit|VspHub|Godot)' } |
        Select-Object ProcessId,ParentProcessId,Name,ExecutablePath,CreationDate)
    # Query all endpoints, then filter: query failure is fatal, never silently clean.
    $tcp = @(Get-NetTCPConnection | Where-Object { $_.LocalPort -in @(7878,7879,8787,5555,5556,9333) } |
        Select-Object LocalAddress,LocalPort,State,OwningProcess)
    # Keep TIME_WAIT/other endpoint evidence; only Listen/Bound block ownership.
    $tcpBlockers = @($tcp | Where-Object { [string]$_.State -in @('Listen','Bound') })
    $udp = @(Get-NetUDPEndpoint | Where-Object { $_.LocalPort -eq 4445 } | Select-Object LocalAddress,LocalPort,OwningProcess)
    return @{ at_utc=[DateTime]::UtcNow.ToString('o'); processes=$processes; tcp=$tcp; tcp_active_blockers=$tcpBlockers; udp=$udp; clean=($processes.Count+$tcpBlockers.Count+$udp.Count -eq 0) }
}
function Snapshot($Path,$Name) {
    $entry = @{ path=$Path; hash=(Hash $Path); backup=(Join-Path $RunRoot ($Name+'.before.json')) }
    if ($entry.hash -ne 'absent') { Copy-Item -LiteralPath $Path -Destination $entry.backup }
    $script:snapshots += $entry
}
function Set-RunEnv($Name,$Value) {
    $script:savedEnv[$Name] = [Environment]::GetEnvironmentVariable($Name,'Process')
    [Environment]::SetEnvironmentVariable($Name,$Value,'Process')
}
function Status {
    Invoke-RestMethod -Uri 'http://127.0.0.1:7879/status' -TimeoutSec 2
}
function Format-HostArguments([string[]]$Arguments) {
    if ($Arguments.Count % 2) { throw 'host arguments must be flag/value pairs' }
    for ($i=0; $i -lt $Arguments.Count; $i+=2) {
        if ($Arguments[$i] -notmatch '^--[a-z][a-z0-9-]*$' -or $Arguments[$i+1].Contains('"') -or $Arguments[$i+1] -match '[\r\n]') { throw 'invalid host flag or embedded quote/newline in value' }
        $Arguments[$i]
        '"'+$Arguments[$i+1]+'"'
    }
}
function Read-ShellExit([System.Diagnostics.Process]$Process) {
    # Caller pins .Handle immediately after Start-Process (PS5.1 ExitCode cache).
    if (-not $Process.WaitForExit(5000)) { return @{known=$false; code=$null; reason='process has not exited'} }
    $Process.WaitForExit()
    $code=$Process.ExitCode
    if ($null -eq $code) { return @{known=$false; code=$null; reason='ExitCode unavailable after WaitForExit'} }
    return @{known=$true; code=[int]$code; reason='WaitForExit with retained handle'}
}
function Cdp($Phase) {
    $out = Join-Path $RunRoot ("cdp_$Phase.json")
    & node (Join-Path $PSScriptRoot 'vitshell_open_chain_cdp.mjs') --cdp-port 9333 --phase $Phase --out $out --webui-dir (Join-Path $MainRoot 'agent/webui') 2>&1 |
        Out-File -LiteralPath ($out+'.log') -Encoding utf8
    if ($LASTEXITCODE -ne 0) { throw "CDP $Phase failed (exit $LASTEXITCODE)" }
    return (Get-Content -LiteralPath $out -Raw -Encoding UTF8 | ConvertFrom-Json)
}
function Remember-Tree {
    param([object[]]$ProcessSnapshot)
    if ($null -eq $shell) { return }
    $all = if ($PSBoundParameters.ContainsKey('ProcessSnapshot')) { @($ProcessSnapshot) } else { @(Get-CimInstance Win32_Process) }
    $roots = @($all | Where-Object { [int]$_.ProcessId -eq [int]$shell.Id })
    # CIM CreationDate has microsecond precision; compare original StartTime
    # at that precision, then retain the actual CIM birth for exact cleanup.
    $birthFormat='yyyyMMddHHmmss.ffffff'
    if ($roots.Count -ne 1 -or $roots[0].CreationDate.ToUniversalTime().ToString($birthFormat) -ne $shell.StartTime.ToUniversalTime().ToString($birthFormat)) { return }
    $visited = @{}
    $pending = @([int]$shell.Id)
    while ($pending.Count) {
        $next = @()
        foreach ($currentId in $pending) {
            if ($visited.ContainsKey($currentId)) { continue }
            $visited[$currentId]=$true
            $matches=@($all | Where-Object { [int]$_.ProcessId -eq $currentId })
            if ($matches.Count -ne 1) { continue }
            $current=$matches[0]
            if ($script:owned.ContainsKey($currentId) -and $script:owned[$currentId].ToUniversalTime().ToString($birthFormat) -ne $current.CreationDate.ToUniversalTime().ToString($birthFormat)) { continue }
            $script:owned[$currentId]=$current.CreationDate
            # Every valid current descendant is traversed on every snapshot,
            # even when previously owned; reused PIDs are pruned, never adopted.
            foreach ($child in $all) {
                if ([int]$child.ParentProcessId -eq $currentId -and $child.CreationDate -ge $current.CreationDate -and -not $visited.ContainsKey([int]$child.ProcessId)) { $next += [int]$child.ProcessId }
            }
        }
        $pending = $next
    }
}
function Wait-StageRuntime([datetime]$Deadline) {
    $roles=@('VitAgent.exe','VspHub.exe','VitApp.exe')
    $portRoles=@{7878='VitAgent.exe';8787='VspHub.exe';5555='VitApp.exe';5556='VitApp.exe'}
    while ($true) {
        # Read endpoints first so a process born after the endpoint snapshot is
        # simply not ready yet. Use the same process snapshot for tree ownership.
        $endpoints=@(Get-NetTCPConnection | Where-Object { $_.LocalPort -in @($portRoles.Keys) } | ForEach-Object {
            @{LocalAddress=[string]$_.LocalAddress;LocalPort=[int]$_.LocalPort;State=[string]$_.State;OwningProcess=[int]$_.OwningProcess}
        })
        $all=@(Get-CimInstance Win32_Process)
        Remember-Tree -ProcessSnapshot $all
        $processes=@($all | Where-Object { $_.Name -in $roles })
        $light=@($processes | ForEach-Object { @{ProcessId=[int]$_.ProcessId;ParentProcessId=[int]$_.ParentProcessId;Name=[string]$_.Name;ExecutablePath=[string]$_.ExecutablePath;CreationDate=$_.CreationDate.ToUniversalTime().ToString('o')} })
        $missing=@();$problems=@()
        foreach ($p in $processes) {
            if ([string]::IsNullOrWhiteSpace($p.ExecutablePath) -or -not $p.ExecutablePath.StartsWith($runtime+'\',[StringComparison]::OrdinalIgnoreCase)) { $problems += "runtime outside stage or unknown path: $($p.Name) pid=$($p.ProcessId)" }
            if (-not $owned.ContainsKey([int]$p.ProcessId) -or $owned[[int]$p.ProcessId] -ne $p.CreationDate) { $problems += "runtime has unknown PID/birth owner: $($p.Name) pid=$($p.ProcessId)" }
        }
        foreach ($role in $roles) {
            $matches=@($processes | Where-Object { $_.Name -eq $role })
            if ($matches.Count -eq 0) { $missing += "process:$role" }
            elseif ($matches.Count -ne 1) { $problems += "duplicate runtime process:$role count=$($matches.Count)" }
        }
        foreach ($port in $portRoles.Keys) {
            $roleProcess=@($processes | Where-Object { $_.Name -eq $portRoles[$port] })
            $active=@($endpoints | Where-Object { $_.LocalPort -eq $port -and $_.State -in @('Listen','Bound') })
            foreach ($endpoint in $active) {
                if ($roleProcess.Count -ne 1 -or $endpoint.OwningProcess -ne $roleProcess[0].ProcessId) { $problems += "unknown/wrong endpoint owner: port=$port pid=$($endpoint.OwningProcess)" }
            }
            if (@($active | Where-Object { $_.State -eq 'Listen' }).Count -eq 0) { $missing += "listener:$port ($($portRoles[$port]))" }
        }
        if ($shell.HasExited) { $problems += 'VitShell exited while waiting for runtime' }
        $evidence=@{at_utc=[DateTime]::UtcNow.ToString('o');processes=$light;endpoints=$endpoints;missing=$missing;problems=$problems;state='waiting'}
        if ($problems.Count) { $evidence.state='failure' }
        elseif (-not $missing.Count) { $evidence.state='ready' }
        elseif ([DateTime]::UtcNow -ge $Deadline) { $evidence.state='timeout' }
        ($evidence | ConvertTo-Json -Depth 6 -Compress) | Add-Content -LiteralPath (Join-Path $RunRoot 'runtime_wait_trace.jsonl') -Encoding utf8
        if ($evidence.state -ne 'waiting') {
            Save-Json $evidence 'runtime_readiness.json'; Save-Json $light 'runtime_processes.json'
            if ($evidence.state -eq 'failure') { throw ('runtime readiness rejected: '+($problems -join '; ')) }
            if ($evidence.state -eq 'timeout') { throw ('runtime readiness timeout; missing '+($missing -join '; ')) }
            return
        }
        Start-Sleep -Milliseconds 200
    }
}
function Wait-OwnedExit([datetime]$Deadline) {
    while ($true) {
        $all=@(Get-CimInstance Win32_Process)
        Remember-Tree -ProcessSnapshot $all
        $remaining=@($all | Where-Object { $owned.ContainsKey([int]$_.ProcessId) -and $owned[[int]$_.ProcessId] -eq $_.CreationDate })
        if (-not $remaining.Count -or [DateTime]::UtcNow -ge $Deadline) { return $remaining }
        Start-Sleep -Milliseconds 200
    }
}
function Capture-BoundGodotLog($Source,$BeforeHash,$OpenTick,$GodotPid,$ShellPid,$ReferenceHwnd,$PollSamples) {
    $capture=@{source=$Source;accepted=$false;reason='log missing';native=$null;guard_ok=$false}
    if (-not (Test-Path -LiteralPath $Source)) { return $capture }
    $raw=Join-Path $RunRoot 'godot-captured.raw.log'
    Copy-Item -LiteralPath $Source -Destination $raw
    $capture.raw=$raw;$capture.sha256=Hash $raw;$capture.last_write_utc=(Get-Item -LiteralPath $Source).LastWriteTimeUtc.ToString('o')
    $capture.reason='unchanged/stale log'
    if ($capture.sha256 -eq $BeforeHash) { return $capture }
    $lines=@(Get-Content -LiteralPath $raw -Encoding UTF8 | Where-Object { $_.StartsWith('[ShellZ409.snapshot] ') })
    $capture.reason='missing/ambiguous native snapshot'
    if ($lines.Count -ne 1) { return $capture }
    try { $native=$lines[0].Substring('[ShellZ409.snapshot] '.Length) | ConvertFrom-Json } catch { return $capture }
    $capture.reason='native snapshot not bound to this launch'
    if (-not $native.samples.Count -or $native.message_interval.start_tick_ms -lt $OpenTick) { return $capture }
    $targetHwnd=$native.samples[0].hwnd
    foreach ($sample in $native.samples) {
        if (-not $sample.binding_valid -or $sample.pid -ne $GodotPid -or $sample.reference_pid -ne $ShellPid -or $sample.reference_hwnd -ne $ReferenceHwnd -or $sample.hwnd -ne $targetHwnd -or $sample.tick_ms -lt $OpenTick) { return $capture }
    }
    if (-not @($PollSamples | ForEach-Object { $_.engines } | Where-Object { $_.pid -eq $GodotPid -and $_.hwnd -eq $targetHwnd }).Count) { return $capture }
    $capture.accepted=$true;$capture.reason='changed raw log and current PID/reference/HWND/tick binding';$capture.native=$native
    $rawText=Get-Content -LiteralPath $raw -Raw -Encoding UTF8
    $snapshotEnd=$rawText.IndexOf($lines[0],[StringComparison]::Ordinal)+$lines[0].Length
    $capture.guard_ok=$rawText.Substring($snapshotEnd) -match 'launch_result.post guard_ok=true'
    return $capture
}
try {
    $StageRoot = (Resolve-Path -LiteralPath $StageRoot).Path
    $tempBase = [IO.Path]::GetFullPath($env:TEMP).TrimEnd('\')+'\'
    if (-not $StageRoot.StartsWith($tempBase,[StringComparison]::OrdinalIgnoreCase)) { throw 'stage must be a dedicated TEMP copy, outside all checkouts' }
    if (Test-Path -LiteralPath (Join-Path $StageRoot '.shell409-used')) { throw 'stage already used; create a fresh isolated stage' }
    if (@(Get-ChildItem -LiteralPath $StageRoot -Recurse -Force | Where-Object { $_.Attributes -band [IO.FileAttributes]::ReparsePoint }).Count) { throw 'stage must not contain borrowed junctions/symlinks' }
    $runtime = Join-Path $StageRoot 'runtime'
    $project = if ([string]::IsNullOrWhiteSpace($GodotProjectStage)) { Join-Path $StageRoot 'frontend' } else { (Resolve-Path -LiteralPath $GodotProjectStage).Path }
    if (-not ([IO.Path]::GetFullPath($project)).StartsWith($tempBase,[StringComparison]::OrdinalIgnoreCase) -or (Test-Path -LiteralPath (Join-Path $project '.git'))) { throw 'GodotProjectStage must be an isolated TEMP copy, not a source checkout' }
    if (Test-Path -LiteralPath (Join-Path $project '.shell409-used')) { throw 'Godot project stage already used; one attempt per fresh stage' }
    if (@(Get-ChildItem -LiteralPath $project -Recurse -Force | Where-Object { $_.Attributes -band [IO.FileAttributes]::ReparsePoint }).Count) { throw 'Godot project stage contains junction/symlink' }
    if (Test-Path -LiteralPath (Join-Path $runtime '.git')) { throw 'runtime stage must not be a checkout' }
    $shellExe = Join-Path $StageRoot 'host/VitShell.exe'
    $agentExe = Join-Path $runtime 'agent/bin/VitAgent.exe'
    $kernelExe = Join-Path $runtime 'VitApp/build/VitApp_artefacts/Release/VitApp.exe'
    $hubExe = Join-Path $runtime 'agent/bin/VspHub.exe'
    $extension = Join-Path $project 'extension/bin/vit_extension.windows.template_debug.x86_64.dll'
    foreach ($file in @($shellExe,$agentExe,$kernelExe,$hubExe,$extension,$GodotExe,(Join-Path $runtime 'agent/webui/dist/index.html'),(Join-Path $project 'project.godot'))) {
        if (-not (Test-Path -LiteralPath $file -PathType Leaf)) { throw "stage input missing: $file" }
    }
    if (-not (Test-Path -LiteralPath (Join-Path $runtime 'VitApp/CMakeLists.txt')) -or -not (Test-Path -LiteralPath (Join-Path $runtime 'VitApp/Source') -PathType Container)) { throw 'stage lacks VitPaths root markers (would resolve Workspace incorrectly)' }
    if ((Hash $extension) -ne $ExpectedExtensionSha256 -or (Hash $extension) -eq (Hash (Join-Path $FrontendWorktree 'extension/bin/vit_extension.windows.template_debug.x86_64.dll'))) { throw 'stage requires the freshly built candidate DLL; tracked old DLL forbidden' }
    foreach ($rel in @('project.godot','app/startup/start_page.gd','app/startup/vitshell_window_entry.gd')) {
        if ((Hash (Join-Path $project $rel)) -ne (Hash (Join-Path $FrontendWorktree $rel))) { throw "stage/source mismatch: $rel" }
    }
    # runtimehost sorts candidates by freshness including canonical fallback.
    # Staged copies must win; never alter canonical timestamps or binaries.
    foreach ($pair in @(@($kernelExe,'D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe'),@($kernelExe,'D:/Vit_DAW/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe'),@($hubExe,'D:/Vit_DAW/agent/bin/VspHub.exe'),@($hubExe,'D:/Vit_DAW/agent/VspHub.exe'))) {
        if ((Test-Path -LiteralPath $pair[1]) -and (Get-Item -LiteralPath $pair[0]).LastWriteTimeUtc -le (Get-Item -LiteralPath $pair[1]).LastWriteTimeUtc) { throw 'staged runtime freshness must exceed canonical fallback (touch only staged copies)' }
    }
    $report.main_head = Git-Read -Arguments @('-C',$MainRoot,'rev-parse','HEAD')
    $report.main_status = Git-Read -Arguments @('-C',$MainRoot,'status','--short')
    $report.frontend_head = Git-Read -Arguments @('-C',$FrontendWorktree,'rev-parse','HEAD')
    $report.frontend_status = Git-Read -Arguments @('-C',$FrontendWorktree,'status','--short')
    (Git-Read -Arguments @('-C',$MainRoot,'diff','--stat')) | Out-File -LiteralPath (Join-Path $RunRoot 'main_diffstat.txt') -Encoding utf8
    (Git-Read -Arguments @('-C',$FrontendWorktree,'diff','--stat')) | Out-File -LiteralPath (Join-Path $RunRoot 'frontend_diffstat.txt') -Encoding utf8
    $report.binaries = @(@($shellExe,$agentExe,$kernelExe,$hubExe,$extension,$GodotExe) | ForEach-Object { @{path=$_; sha256=(Hash $_); modified_utc=(Get-Item -LiteralPath $_).LastWriteTimeUtc.ToString('o')} })
    $report.source_fixture_sha256 = Hash $FixtureSource
    if ($report.source_fixture_sha256 -eq 'absent') { throw 'fixture missing' }
    $report.stage_fixture_inputs = @{ stage=$StageRoot; godot_project_stage=$project; frontend_source=$FrontendWorktree; source=$FixtureSource; project_sha256=(Hash (Join-Path $project 'project.godot')); webui_index_sha256=(Hash (Join-Path $runtime 'agent/webui/dist/index.html')) }
    Snapshot (Join-Path $env:APPDATA 'Vit-DAW/global_project_config.json') 'recents'
    Snapshot $FixtureSource 'fixture-source'
    foreach ($base in @($MainRoot,'D:/Vit_DAW')) {
        foreach ($rel in @('VitApp/Workspace/Settings/Settings.xml','VitApp/Workspace/default_project.xml')) {
            Snapshot (Join-Path $base $rel) ('authority-'+$snapshots.Count)
        }
    }
    Save-Json $snapshots 'authority_snapshots.json'
    $fixture = Join-Path $RunRoot 'fixture_a.vit'; Copy-Item -LiteralPath $FixtureSource -Destination $fixture
    $report.fixture_copy_before_sha256=Hash $fixture
    $godotLog = Join-Path $RunRoot 'godot.log'
    $report.raw_logs=@{godot=$godotLog; agent=(Join-Path $RunRoot 'agent.log'); hub=(Join-Path $RunRoot 'hub.log'); kernel=(Join-Path $runtime 'VitApp/Workspace/Logs'); bridge=(Join-Path $RunRoot 'bridge.log')}
    # Explicit isolated logger override; no production source/config mutations.
    $override = Join-Path $project 'override.cfg'
    if (Test-Path -LiteralPath $override) { throw 'stage override.cfg must be absent; preserve it for review rather than overwrite' }
    "[debug]`nfile_logging/enable_file_logging=true`nfile_logging/log_path=`"$($godotLog.Replace('\','/'))`"`n" | Out-File -LiteralPath $override -Encoding utf8
    Add-Type -Path (Join-Path $PSScriptRoot 'vitshell_zproduct_observer.cs')
    Set-RunEnv 'WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS' '--remote-debugging-port=9333'
    Set-RunEnv 'VIT_DAW_DEV_ROOT' $runtime; Set-RunEnv 'VIT_DAW_ROOT' $runtime
    Set-RunEnv 'VIT_DAW_USER_CONFIG_DIR' (Join-Path $RunRoot 'user-config')
    Set-RunEnv 'VIT_AGENT_LAST_LOG_PATH' (Join-Path $RunRoot 'agent.log')
    Set-RunEnv 'VIT_VSP_HUB_LAST_LOG_PATH' (Join-Path $RunRoot 'hub.log')
    Set-RunEnv 'VIT_AGENT_LLM_TELEMETRY_PATH' (Join-Path $RunRoot 'llm.jsonl')
    Set-RunEnv 'VIT_AGENT_MESSAGE_LOOP_DEBUG_PATH' (Join-Path $RunRoot 'messages.jsonl')
    Set-RunEnv 'VIT_AGENT_WINDOW_MODE' ''; Set-RunEnv 'VIT_FROM_LAUNCHER' ''
    $leaseAt = Assert-Lease
    $clear = Get-Clearance; Save-Json $clear 'prelaunch_clearance.json'
    $clearanceAt=[DateTime]::UtcNow
    if (-not $clear.clean -or ($clearanceAt-$leaseAt).TotalSeconds -gt 60) { throw 'runtime lease/clearance is not fresh and clean' }
    $arguments = @('--agent-exe',$agentExe,'--dev-root',$runtime,'--agent-url','http://127.0.0.1:7878','--agent-http-addr','127.0.0.1:7878',
        '--url','http://127.0.0.1:7878/app/?surface=launcher','--status-port','7879','--godot-exe',$GodotExe,'--godot-project-dir',$project,
        '--smoke-file-dialog',$fixture,'--bridge-log',(Join-Path $RunRoot 'bridge.log'))
    # The actual host parser requires bare flags; only values are quoted.
    $quoted = @(Format-HostArguments -Arguments $arguments)
    Save-Json @{exe=$shellExe; arguments=$arguments; command=($shellExe+' '+($quoted -join ' ')); cwd=$runtime} 'launch_command.json'
    New-Item -ItemType File -Path (Join-Path $StageRoot '.shell409-used') | Out-Null
    New-Item -ItemType File -Path (Join-Path $project '.shell409-used') | Out-Null
    if (([DateTime]::UtcNow-$leaseAt).TotalSeconds -gt 60 -or ([DateTime]::UtcNow-$clearanceAt).TotalSeconds -gt 60) { throw 'lease/clearance recheck exceeded 60 seconds before spawn' }
    $shell = Start-Process -FilePath $shellExe -ArgumentList $quoted -WorkingDirectory $runtime -PassThru -WindowStyle Normal -RedirectStandardError (Join-Path $RunRoot 'shell.stderr.log') -RedirectStandardOutput (Join-Path $RunRoot 'shell.stdout.log')
    $shellHandle = $shell.Handle
    $report.shell_pid = $shell.Id; $owned[[int]$shell.Id] = $shell.StartTime
    $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSec); $ready = $null
    do {
        Remember-Tree
        try { $ready=Status } catch { $ready=$null }
        if ($null -ne $ready) { $lastStatus=$ready }
        if ($null -ne $ready -and $ready.phase -eq 'error') { Save-Json $ready 'status_failure.json'; $report.status_error=[string]$ready.error; throw ('VitShell phase=error: '+$ready.error) }
        if ($null -ne $ready -and $ready.phase -eq 'ready') { break }
        if ($shell.HasExited) { throw 'VitShell exited before ready' }
        Start-Sleep -Milliseconds 200
    } while ([DateTime]::UtcNow -lt $deadline)
    if ($null -eq $ready -or $ready.phase -ne 'ready') { throw 'ready timeout' }
    Save-Json $ready 'status_ready.json'
    Wait-StageRuntime -Deadline ([DateTime]::UtcNow.AddSeconds($TimeoutSec))
    $probe = Cdp 'probe-launcher'
    $shell.Refresh(); $reference = $shell.MainWindowHandle.ToInt64()
    if (-not $reference) { throw 'missing VitShell main HWND' }
    $conditionDeadline=[DateTime]::UtcNow.AddSeconds($NaturalConditionWaitSec)
    $stableExternalHwnd=0L;$stableExternalSince=$null
    do {
    $before = [Shell409Observer]::Capture($reference)
    $conditionMet = if ($Condition -eq 'shell') { $before.foreground_hwnd -eq $reference -and $before.foreground_pid -eq $shell.Id } else { $before.foreground_hwnd -ne 0 -and $before.foreground_pid -ne $shell.Id -and $before.foreground_pid -ne $PID -and $before.foreground_pid -notin @($owned.Keys) }
    if ($Condition -eq 'external' -and $ExpectedExternalPid -gt 0) {
        if ($conditionMet -and $before.foreground_pid -eq $ExpectedExternalPid) {
            if ($stableExternalHwnd -ne $before.foreground_hwnd) { $stableExternalHwnd=$before.foreground_hwnd;$stableExternalSince=$before.tick_ms }
            $conditionMet=($null -ne $stableExternalSince -and $before.tick_ms-$stableExternalSince -ge 2000 -and [DateTime]::UtcNow -le $conditionDeadline)
        } else { $stableExternalHwnd=0L;$stableExternalSince=$null;$conditionMet=$false }
    }
    if ($conditionMet -or [DateTime]::UtcNow -ge $conditionDeadline) { break }
    Start-Sleep -Milliseconds 200
    } while ($true)
    Save-Json $before 'natural_condition.json'
    if ($Condition -eq 'external' -and $ExpectedExternalPid -gt 0) { $report.expected_external_pid=$ExpectedExternalPid;$report.expected_external_hwnd=$stableExternalHwnd;$report.expected_external_stable_since_tick=$stableExternalSince;$report.expected_external_ready=$conditionMet }
    if (-not $conditionMet) { $report.result='not_exercised'; $report.reason='natural foreground absent; no focus forcing'; $exitCode=3 }
    else {
        $defaultGodotLog=Join-Path $env:APPDATA 'Godot/app_userdata/Vit_DAW_Frontend/logs/godot.log'
        $defaultBefore=@{source=$defaultGodotLog;sha256=(Hash $defaultGodotLog);captured_utc=[DateTime]::UtcNow.ToString('o')}
        if (Test-Path -LiteralPath $defaultGodotLog) { $defaultBefore.last_write_utc=(Get-Item -LiteralPath $defaultGodotLog).LastWriteTimeUtc.ToString('o'); Copy-Item -LiteralPath $defaultGodotLog -Destination (Join-Path $RunRoot 'godot-default.before.log') }
        Save-Json $defaultBefore 'godot-default.before.json'
        [Shell409Observer]::Start($reference); $observerStarted=$true
        Start-Sleep -Milliseconds 50
        $openTick = [Shell409Observer]::GetTickCount64()
        $null = Cdp 'click-open'
        $deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSec); $opened=$null
        do {
            Remember-Tree
            $opened=Status
            $lastStatus=$opened
            if ($opened.chain_state -eq 'failed' -or $opened.phase -eq 'error') { Save-Json $opened 'status_failure.json'; $report.status_error=[string]$opened.error; throw ('PRODUCT: product open chain failed: '+$opened.error) }
            if ([int]$opened.launch_count -eq 1 -and $opened.chain_state -eq 'opened') { break }
            Start-Sleep -Milliseconds 100
        } while ([DateTime]::UtcNow -lt $deadline)
        Save-Json $opened 'status_opened.json'
        if ([int]$opened.launch_count -ne 1 -or $opened.chain_state -ne 'opened' -or -not $opened.godot_running) { throw 'PRODUCT: single product open timeout' }
        $dom = Cdp 'probe-project'
        Start-Sleep -Milliseconds 2200
        $samples=[Shell409Observer]::Stop(); $observerStarted=$false
        Save-Json $samples 'native_poll_samples.json'
        $logSource=$godotLog;$beforeHash='absent'
        if (-not (Test-Path -LiteralPath $logSource)) { $logSource=$defaultGodotLog;$beforeHash=$defaultBefore.sha256 }
        $capture=Capture-BoundGodotLog $logSource $beforeHash $openTick ([int]$opened.godot_pid) $shell.Id $reference $samples
        Save-Json $capture 'godot_log_capture.json'
        $native=$capture.native
        $state = Invoke-RestMethod 'http://127.0.0.1:7878/agent/state' -TimeoutSec 5; Save-Json $state 'agent_state.json'
        $data = @{ condition=$Condition; condition_met=$conditionMet; shell_pid=$shell.Id; shell_hwnd=$reference; godot_pid=[int]$opened.godot_pid
            expected_foreground_hwnd=$before.foreground_hwnd; expected_foreground_pid=$before.foreground_pid
            open_tick_ms=$openTick; native=$native; samples=$samples; observer_error=[Shell409Observer]::Error
            chain_opened=($opened.chain_state -eq 'opened' -and ([string]$state.current_project.path).Replace('\','/') -eq $fixture.Replace('\','/'))
            guard_ok=$capture.guard_ok
            project_dom_ok=($dom.ok -and $dom.launcher_root_gone -and $dom.sentinel -eq $probe.sentinel -and $dom.url -eq $probe.url) }
        Save-Json $data 'classification_input.json'
        & node (Join-Path $PSScriptRoot 'vitshell_zproduct_verdict.mjs') (Join-Path $RunRoot 'classification_input.json') (Join-Path $RunRoot 'verdict.json') 2>&1 | Out-File -LiteralPath (Join-Path $RunRoot 'verdict.log') -Encoding utf8
        $exitCode=$LASTEXITCODE; $report.classifier_exit=$exitCode
        $verdict=Get-Content -LiteralPath (Join-Path $RunRoot 'verdict.json') -Raw -Encoding UTF8 | ConvertFrom-Json
        $report.result=$verdict.classification; $report.verdict=$verdict
    }
} catch {
    $report.error=$_.Exception.Message; $report.error_record=($_ | Out-String); $exitCode=2
    if ($null -ne $lastStatus) { try { Save-Json $lastStatus 'status_last.json' } catch { $report.status_evidence_error=$_.Exception.Message } }
    if ($_.Exception.Message.StartsWith('PRODUCT:')) { $report.result='product_failure'; $exitCode=1 }
    elseif ($null -ne $shell) { $report.result='harness_failure' }
} finally {
    if ($observerStarted) { try { Save-Json ([Shell409Observer]::Stop()) 'native_poll_samples.json' } catch { $teardownErrors += $_.Exception.Message } }
    if ($null -ne $shell) {
        try { Remember-Tree; Save-Json @($owned.GetEnumerator() | ForEach-Object { @{pid=[int]$_.Key; birth=$_.Value} }) 'owned_processes.json' } catch { $teardownErrors += $_.Exception.Message }
        # Only address our still-live shell. Never close an unrelated listener.
        try {
            $live=Get-Process -Id $shell.Id -ErrorAction SilentlyContinue
            if ($null -ne $live -and $live.StartTime -eq $shell.StartTime) { $null=Invoke-RestMethod -Method Post 'http://127.0.0.1:7879/close' -Body '{}' -ContentType 'application/json' -TimeoutSec 3 }
        } catch { $report.close_error=$_.Exception.Message }
        $until=[DateTime]::UtcNow.AddSeconds(20)
        $remaining=@(Wait-OwnedExit -Deadline $until)
        Save-Json @($owned.GetEnumerator() | ForEach-Object { @{pid=[int]$_.Key; birth=$_.Value} }) 'owned_processes.json'
        # Exact PID + birth-time only; job containment normally exits the tree.
        foreach ($ownedId in @($owned.Keys)) {
            $p=Get-CimInstance Win32_Process -Filter "ProcessId=$ownedId"
            if ($null -ne $p -and $p.CreationDate -eq $owned[$ownedId]) {
                $teardownErrors += "owned process required forced cleanup: $ownedId"
                try { Stop-Process -Id $ownedId -Force; Wait-Process -Id $ownedId -Timeout 5 -ErrorAction SilentlyContinue } catch { $teardownErrors += $_.Exception.Message }
            }
        }
        try {
            $observedExit=Read-ShellExit -Process $shell
            $report.shell_exit_observation=$observedExit
            if ($observedExit.known) { $report.shell_exit=$observedExit.code; if ($observedExit.code -ne 0) { $teardownErrors += "shell exit $($observedExit.code)" } }
            else { $report.shell_exit='unknown'; $teardownErrors += ('shell exit unknown: '+$observedExit.reason) }
        } catch { $report.shell_exit='unknown'; $teardownErrors += ('shell exit unknown: '+$_.Exception.Message) }
        try { $clear=Get-Clearance; Save-Json $clear 'teardown_clearance.json'; if (-not $clear.clean) { $teardownErrors += 'process/port residue' } } catch { $teardownErrors += $_.Exception.Message }
    }
    # Finally restores both present-before and absent-before state; hash proof.
    foreach ($snap in $snapshots) {
        try {
            $beforeRestore=Hash $snap.path
            if ($beforeRestore -ne $snap.hash -and $snap.path -ne (Join-Path $env:APPDATA 'Vit-DAW/global_project_config.json')) { $teardownErrors += "authority changed: $($snap.path)"; if ($beforeRestore -ne 'absent') { Copy-Item -LiteralPath $snap.path -Destination ($snap.backup+'.polluted') } }
            if ($beforeRestore -ne $snap.hash) {
                if ($snap.hash -ne 'absent') { Copy-Item -LiteralPath $snap.backup -Destination $snap.path -Force }
                elseif (Test-Path -LiteralPath $snap.path) { Remove-Item -LiteralPath $snap.path -Force }
            }
            $snap.restored_hash=Hash $snap.path
            if ($snap.restored_hash -ne $snap.hash) { $teardownErrors += "restore hash mismatch: $($snap.path)" }
        } catch { $teardownErrors += $_.Exception.Message }
    }
    Save-Json $snapshots 'authority_restoration.json'
    if (Get-Variable fixture -ErrorAction SilentlyContinue) { $report.fixture_copy_after_sha256=Hash $fixture }
    $telemetry=Join-Path $RunRoot 'llm.jsonl'
    $report.llm_calls=0
    if ((Test-Path -LiteralPath $telemetry) -and (Get-Item -LiteralPath $telemetry).Length -gt 0) { $report.llm_calls=@(Get-Content -LiteralPath $telemetry -Encoding UTF8).Count; $teardownErrors += 'zero-LLM assertion failed: telemetry nonempty' }
    foreach ($name in $savedEnv.Keys) { [Environment]::SetEnvironmentVariable($name,$savedEnv[$name],'Process') }
    if ($teardownErrors.Count) { $report.assertion_result_before_teardown=$report.result; $report.result='environment_failure'; $report.teardown_errors=$teardownErrors; $exitCode=2 }
    $report.exit_code=$exitCode; $report.finished_utc=[DateTime]::UtcNow.ToString('o')
    Save-Json $report 'd1_smoke_report.json'
}
Write-Host "$($report.result): $RunRoot (exit $exitCode)"
exit $exitCode

# run_render_wedge_evidence.ps1 — WEDGE-2A diagnostic entry point.
#
# Card: 2026-09-05-WEDGE-2A-completion-baseline-and-cancel-evidence.md
# Rebuilds the bare-kernel render completion baseline (raw request vs D1-same-param
# request) on project copies, captures A-phase thread stacks BEFORE the render
# watchdog deadline and B-phase stacks if the message thread dies after it.
# Zero production code changes; all evidence goes under -OutputRoot\<run-id>\.
#
# Exit codes (honest diagnostic semantics, do NOT normalize):
#   0 = both groups rendered truly healthy (terminal success + second same-param
#       render also completed)
#   1 = anomaly observed (no progress / watchdog / message thread death / error
#       terminal) with evidence saved
#   2 = environment / protocol / tooling failure made the experiment undecidable

param(
    [string]$Mode = "BaselineAndWatchdog",
    [string]$OutputRoot = ".\artifacts\render_wedge_evidence",
    [string]$KernelExe = "",
    # WEDGE-2B1 ParamSweep mode: path to a sweep spec JSON with a trials array
    # ({label, project_source, request, watchdog_seconds?, a_stack_at?, skip_if?}).
    [string]$SweepSpec = "",
    [int]$WatchdogSeconds = 140,
    [int]$AStackAt = 105,
    [int]$PingIntervalSeconds = 10,
    [int]$ObserveAfterWatchdogSeconds = 45,
    [int]$MaxSecondsPerGroup = 420,
    [switch]$SkipCleanup
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$runStamp = Get-Date -Format "yyyyMMdd-HHmmss"

function Write-Step([string]$m) { Write-Host "== $m" }
function Write-Ok([string]$m)   { Write-Host "   OK: $m" }
function Write-Warn2([string]$m){ Write-Host "   WARN: $m" -ForegroundColor Yellow }

function Write-Result([hashtable]$result, [int]$code, [string]$runDir) {
    $result["exit_code"] = $code
    $result["finished_at"] = (Get-Date).ToString("o")
    $result | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $runDir "result.json") -Encoding UTF8
    exit $code
}

$runDir = Join-Path (Join-Path $repoRoot $OutputRoot) "run_$runStamp"
if (-not (Test-Path $runDir)) { New-Item -ItemType Directory -Path $runDir -Force | Out-Null }
Write-Host "WEDGE-2A render wedge evidence run: $runDir"

if ($Mode -ne "BaselineAndWatchdog" -and $Mode -ne "ParamSweep") {
    Write-Result @{ error = "unsupported Mode '$Mode' (BaselineAndWatchdog or ParamSweep)" } 2 $runDir
}

# --- environment ------------------------------------------------------------
Write-Step "Record environment"
$env_ = @{
    started_at   = (Get-Date).ToString("o")
    mode         = $Mode
    repo_root    = $repoRoot
    watch_dog_formula = "kRenderWatchdogBaseSeconds(120) + range_length; 20s probes -> ~140s deadline"
}

$gitHead = (git -C $repoRoot rev-parse HEAD) 2>$null
$gitStatus = (git -C $repoRoot status --porcelain) 2>$null
$env_["git_head"] = $gitHead
$env_["git_status_porcelain"] = @($gitStatus)
Write-Ok "git HEAD $gitHead"

if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $repoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"
}
if (-not (Test-Path -LiteralPath $KernelExe)) {
    Write-Result @{ error = "kernel exe not found: $KernelExe"; environment = $env_ } 2 $runDir
}
$kernelItem = Get-Item -LiteralPath $KernelExe
$kernelHash = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
$env_["kernel_exe"] = $KernelExe
$env_["kernel_exe_sha256"] = $kernelHash
$env_["kernel_exe_mtime"] = $kernelItem.LastWriteTimeUtc.ToString("o")
# running binary version is recorded explicitly, never inferred from source HEAD
Write-Ok ("kernel exe: $KernelExe sha256=$($kernelHash.Substring(0,16))... mtime=$($kernelItem.LastWriteTime)")

$pyExe = (Get-Command python -ErrorAction SilentlyContinue).Source
if (-not $pyExe) {
    Write-Result @{ error = "python not on PATH"; environment = $env_ } 2 $runDir
}
$pyVer = (& $pyExe --version) 2>&1
$pyZmqOk = (& $pyExe -c "import zmq;print(zmq.__version__)") 2>$null
if (-not $pyZmqOk) {
    Write-Result @{ error = "pyzmq unavailable"; environment = $env_ } 2 $runDir
}
$env_["python_exe"] = $pyExe
$env_["python_version"] = "$pyVer"
$env_["pyzmq_version"] = "$pyZmqOk"
$probeScript = Join-Path $repoRoot "scripts\render_wedge_probe.py"
if (-not (Test-Path -LiteralPath $probeScript)) {
    Write-Result @{ error = "probe script missing: $probeScript"; environment = $env_ } 2 $runDir
}

# --- port ownership ----------------------------------------------------------
Write-Step "Check ZMQ port ownership (5555/5556/5557)"
$portOwners = @{}
foreach ($port in 5555, 5556, 5557) {
    $conn = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue |
        Select-Object -First 1
    if ($conn) {
        $p = Get-Process -Id $conn.OwningProcess -ErrorAction SilentlyContinue
        $portOwners["$port"] = @{
            pid  = $conn.OwningProcess
            name = if ($p) { $p.ProcessName } else { "?" }
            path = if ($p) { $p.Path } else { "?" }
        }
    }
}
$env_["port_owners"] = $portOwners
if ($portOwners.Count -gt 0) {
    Write-Warn2 ("ports busy: " + (($portOwners.GetEnumerator() | ForEach-Object { "$($_.Key)=$($_.Value.name)($($_.Value.pid))" }) -join ", "))
    Write-Result @{
        error       = "ZMQ ports already owned by other processes; per card discipline, do not kill them"
        port_owners = $portOwners
        environment = $env_
    } 2 $runDir
}
Write-Ok "all three ports free"

# --- project copies / sweep spec ----------------------------------------------
if ($Mode -eq "BaselineAndWatchdog") {
Write-Step "Prepare project copies"
$projSource = Join-Path $repoRoot "temp\wedge_probe\clean_p01.vit"
if (-not (Test-Path -LiteralPath $projSource)) {
    Write-Result @{ error = "project source missing: $projSource"; environment = $env_ } 2 $runDir
}
$projSourceHash = (Get-FileHash -LiteralPath $projSource -Algorithm SHA256).Hash
$requests = @{
    group1 = @{
        label   = "raw_bare_request"
        request = [ordered]@{ cmd = "l2_render_probe"; track_id = "1007"; tap_point = "track_post_fader" }
    }
    group2 = @{
        label   = "d1_same_param_request"
        request = [ordered]@{
            cmd         = "l2_render_probe"
            track_id    = "1007"
            clip_id     = "1011"
            range       = @(0, 20)
            tail_seconds = 0
            tap_point   = "track_post_fader"
            render_mode = "offline_probe"
        }
    }
}
$groupDirs = @{}
foreach ($g in "group1", "group2") {
    $gdir = Join-Path $runDir $g
    New-Item -ItemType Directory -Path $gdir -Force | Out-Null
    $groupDirs[$g] = $gdir
    $copy = Join-Path $gdir "clean_p01.vit"
    Copy-Item -LiteralPath $projSource -Destination $copy -Force
    $copyHash = (Get-FileHash -LiteralPath $copy -Algorithm SHA256).Hash
    $reqPath = Join-Path $gdir "request.json"
    $requests[$g]["request"] | ConvertTo-Json | Set-Content -LiteralPath $reqPath -Encoding UTF8
    $requests[$g]["copy"] = $copy
    $requests[$g]["copy_sha256"] = $copyHash
    $requests[$g]["request_file"] = $reqPath
    $requests[$g]["copy_matches_source"] = ($copyHash -eq $projSourceHash)
    Write-Ok ("$g copy: $copy (hash match source: $($copyHash -eq $projSourceHash))")
}
}
else {
Write-Step "Load sweep spec (ParamSweep)"
if ([string]::IsNullOrWhiteSpace($SweepSpec) -or -not (Test-Path -LiteralPath $SweepSpec)) {
    Write-Result @{ error = "ParamSweep requires -SweepSpec pointing to an existing spec file"; environment = $env_ } 2 $runDir
}
$spec = Get-Content -LiteralPath $SweepSpec -Raw -Encoding UTF8 | ConvertFrom-Json
$specTrials = @($spec.trials)
if ($specTrials.Count -eq 0) {
    Write-Result @{ error = "sweep spec contains no trials"; environment = $env_ } 2 $runDir
}
Copy-Item -LiteralPath $SweepSpec -Destination (Join-Path $runDir "sweep_spec.json") -Force
foreach ($t in $specTrials) {
    if ([string]::IsNullOrWhiteSpace($t.label) -or [string]::IsNullOrWhiteSpace($t.project_source)) {
        Write-Result @{ error = "sweep trial missing label or project_source"; environment = $env_ } 2 $runDir
    }
    if (-not (Test-Path -LiteralPath $t.project_source)) {
        Write-Result @{ error = "sweep trial '$($t.label)' project source missing: $($t.project_source)"; environment = $env_ } 2 $runDir
    }
}
$env_["sweep_spec"] = $SweepSpec
$env_["sweep_trial_count"] = $specTrials.Count
Write-Ok ("sweep spec loaded: $($specTrials.Count) trials")
}

function Get-KernelLogs([datetime]$since) {
    $logsDir = Join-Path $repoRoot "VitApp\Workspace\Logs"
    Get-ChildItem -LiteralPath $logsDir -Filter "VitHeadlessServer*.log" -ErrorAction SilentlyContinue |
        Where-Object { $_.LastWriteTime -ge $since } |
        Sort-Object LastWriteTime
}

# Run the python probe without letting native stderr (tracebacks etc.) trigger
# ErrorActionPreference=Stop, and always capture the console transcript.
function Invoke-Probe {
    param([string[]]$ArgumentList, [string]$ConsoleLog)
    $prev = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $out = & $pyExe $probeScript @ArgumentList 2>&1
        $code = $LASTEXITCODE
    }
    finally {
        $ErrorActionPreference = $prev
    }
    $out | Set-Content -LiteralPath $ConsoleLog -Encoding UTF8
    return $code
}

function Get-ThreadStates([int]$procId) {
    try {
        $proc = Get-Process -Id $procId -ErrorAction Stop
        return @{
            thread_count = $proc.Threads.Count
            wait_reasons = @($proc.Threads | Group-Object WaitReason | ForEach-Object {
                "{0} x{1}" -f $_.Name, $_.Count
            })
        }
    }
    catch { return @{ error = "process $procId not queryable" } }
}

function Start-ExperimentKernel {
    $startedAt = Get-Date
    $proc = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) `
        -WindowStyle Hidden -PassThru
    $deadline = (Get-Date).AddSeconds(30)
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 500
        if ($proc.HasExited) { break }
        if (Get-NetTCPConnection -LocalPort 5555 -State Listen -ErrorAction SilentlyContinue) {
            return @{ pid = $proc.Id; started_at = $startedAt.ToString("o"); listening = $true }
        }
    }
    return @{ pid = $proc.Id; started_at = $startedAt.ToString("o"); listening = $false; exited = $proc.HasExited }
}

function Stop-ExperimentKernel([int]$procId, [hashtable]$record) {
    # Only ever touch the PID this card started, and only if it is VitApp.exe.
    try {
        $p = Get-Process -Id $procId -ErrorAction Stop
        if ($p.ProcessName -ne "VitApp") {
            $record["cleanup_error"] = "refusing to stop pid ${procId}: process name is $($p.ProcessName), not VitApp"
            Write-Warn2 $record["cleanup_error"]
            return
        }
        if (-not $SkipCleanup) {
            Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
            $record["kernel_stopped_at"] = (Get-Date).ToString("o")
            Write-Ok "stopped experiment kernel pid=$procId"
        }
        else {
            $record["kernel_stopped_at"] = $null
            Write-Warn2 "-SkipCleanup given; leaving kernel pid=$procId running"
        }
    }
    catch {
        $record["kernel_stopped_at"] = $null
        Write-Warn2 "kernel pid=$procId already gone"
    }
    $freed = $false
    $deadline = (Get-Date).AddSeconds(15)
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 500
        if (-not (Get-NetTCPConnection -LocalPort 5555 -State Listen -ErrorAction SilentlyContinue)) {
            $freed = $true; break
        }
    }
    $record["port_5555_freed"] = $freed
}

$groupResults = @()
$trialResults = @()
$overallClass = 0  # 0 healthy, 1 anomaly, 2 tooling
$currentKernelPid = -1
$currentGroupResult = $null

try {
    if ($Mode -eq "BaselineAndWatchdog") {
    foreach ($g in "group1", "group2") {
        Write-Step "Run $g ($($requests[$g]['label']))"
        $gdir = $groupDirs[$g]
        $startRec = Start-ExperimentKernel
        $currentKernelPid = $startRec["pid"]
        $gresult = @{
            group         = $g
            label         = $requests[$g]["label"]
            kernel_pid    = $startRec["pid"]
            kernel_started_at = $startRec["started_at"]
            kernel_listening_5555 = $startRec["listening"]
            project_copy  = $requests[$g]["copy"]
            project_copy_matches_source = $requests[$g]["copy_matches_source"]
            request       = $requests[$g]["request"]
            exit_code     = $null
            verdict       = $null
        }
        $currentGroupResult = $gresult
        if (-not $startRec["listening"]) {
            $gresult["verdict"] = "kernel_start_failed"
            $gresult["exit_code"] = 2
            $groupResults += $gresult
            $overallClass = 2
            Stop-ExperimentKernel $startRec["pid"] $gresult
            break
        }
        Write-Ok "kernel started pid=$($startRec['pid'])"

    # warmup: cache system PDBs and take a healthy baseline stack snapshot
    Write-Step "Warmup stack capture (also a baseline snapshot)"
    $warmLog = Join-Path $gdir "warmup_console.log"
    $null = Invoke-Probe -ArgumentList @("stacks", "--pid", "$($startRec['pid'])", "--out", (Join-Path $gdir "warmup_baseline_stacks.json")) -ConsoleLog $warmLog
    $warmJson = Join-Path $gdir "warmup_baseline_stacks.json"
    if (Test-Path $warmJson) {
        $wj = Get-Content $warmJson -Raw | ConvertFrom-Json
        Write-Ok ("baseline captured: {0} threads" -f $wj.threads.Count)
    }
    else {
        Write-Warn2 "warmup capture failed; A-phase captures may be slower"
    }

    Write-Step "Monitor probe loop"
    $consoleLog = Join-Path $gdir "monitor_console.log"
    $monExit = Invoke-Probe -ArgumentList @(
        "monitor",
        "--out-dir", $gdir,
        "--project-copy", $requests[$g]["copy"],
        "--request-file", $requests[$g]["request_file"],
        "--kernel-pid", "$($startRec['pid'])",
        "--group", $g,
        "--watchdog-seconds", "$WatchdogSeconds",
        "--a-stack-at", "$AStackAt",
        "--ping-interval", "$PingIntervalSeconds",
        "--observe-after-watchdog", "$ObserveAfterWatchdogSeconds",
        "--max-seconds", "$MaxSecondsPerGroup"
    ) -ConsoleLog $consoleLog
    Get-Content -LiteralPath $consoleLog | Write-Host
    $gresult["exit_code"] = $monExit

    $grPath = Join-Path $gdir "group_result.json"
    if (Test-Path $grPath) {
        $gr = Get-Content $grPath -Raw | ConvertFrom-Json
        $gresult["verdict"] = $gr.verdict
        $gresult["requests"] = $gr.requests
        $gresult["watchdog_triggered"] = $gr.watchdog_triggered
        $gresult["watchdog_event"] = $gr.watchdog_event
        $gresult["a_stacks"] = $gr.a_stacks
        $gresult["b_stacks"] = $gr.b_stacks
        $gresult["missing_evidence"] = $gr.missing_evidence
        $gresult["pub_broken"] = $gr.pub_broken
    }
    else {
        $gresult["verdict"] = "monitor_produced_no_group_result"
    }
    $gresult["thread_states_post"] = Get-ThreadStates $startRec["pid"]

    # copy the kernel log written during this group's window
    $since = [datetime]::Parse($startRec["started_at"]).AddMinutes(-1)
    $logs = Get-KernelLogs $since
    if ($logs -and $logs.Count -gt 0) {
        $klog = $logs | Sort-Object LastWriteTime -Descending | Select-Object -First 1
        Copy-Item -LiteralPath $klog.FullName -Destination (Join-Path $gdir $klog.Name) -Force
        $gresult["kernel_log"] = Join-Path $gdir $klog.Name
        Write-Ok ("kernel log captured: $($klog.Name)")
    }
    else {
        Write-Warn2 "no kernel log found for this group's window"
    }

    Stop-ExperimentKernel $startRec["pid"] $gresult

    $class = if ($gresult["verdict"] -in @("kernel_start_failed", "monitor_produced_no_group_result")) { 2 }
             elseif ($monExit -eq 2) { 2 }
             elseif ($monExit -eq 1) { 1 }
             elseif ($monExit -eq 0) { 0 }
             else { 2 }
    if ($class -gt $overallClass) { $overallClass = $class }
    $groupResults += $gresult
    $currentKernelPid = -1
    $currentGroupResult = $null
    }
    }
    else {
    # WEDGE-2B1 ParamSweep: one independent clean kernel per trial; the render
    # watchdog firing is a valid discriminating outcome (collect and stop), so
    # the probe runs in --stop-at-watchdog mode and no B-phase forensics run.
    $validVerdicts = @("healthy", "watchdog_wedge", "watchdog_wedge_second_request",
                       "ping_dead_no_forensics", "render_error_terminal")
    foreach ($t in $specTrials) {
        $label = $t.label
        $skipRules = @()
        if ($null -ne $t.skip_if) { $skipRules += $t.skip_if }
        if ($null -ne $t.skip_if_any) { $skipRules += @($t.skip_if_any) }
        $skipReason = ""
        foreach ($rule in $skipRules) {
            $prior = $trialResults | Where-Object { -not $_.skipped -and $_.label -eq $rule.label }
            if ($prior -and $prior.verdict -eq $rule.verdict) {
                $skipReason = "prior trial '$($rule.label)' verdict=$($rule.verdict)"
                break
            }
        }
        if ($skipReason) {
            Write-Step "Skip $label ($skipReason)"
            $trialResults += @{ label = $label; skipped = $true; skip_reason = $skipReason }
            continue
        }
        Write-Step "Run sweep trial $label"
        $tdir = Join-Path $runDir $label
        New-Item -ItemType Directory -Path $tdir -Force | Out-Null
        $srcHash = (Get-FileHash -LiteralPath $t.project_source -Algorithm SHA256).Hash
        $copy = Join-Path $tdir (Split-Path -Leaf $t.project_source)
        Copy-Item -LiteralPath $t.project_source -Destination $copy -Force
        $copyHash = (Get-FileHash -LiteralPath $copy -Algorithm SHA256).Hash
        $reqPath = Join-Path $tdir "request.json"
        $t.request | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $reqPath -Encoding UTF8
        $wdSec = 140.0
        if ($t.watchdog_seconds) { $wdSec = [double]$t.watchdog_seconds }
        $trialAStackAt = [Math]::Max(60.0, $wdSec - 45.0)
        if ($t.a_stack_at) { $trialAStackAt = [double]$t.a_stack_at }
        $maxSec = 2 * $wdSec + 120

        $startRec = Start-ExperimentKernel
        $currentKernelPid = $startRec["pid"]
        $tresult = @{
            trial         = $label
            label         = $label
            project_source = $t.project_source
            project_source_sha256 = $srcHash
            project_copy  = $copy
            project_copy_sha256 = $copyHash
            copy_matches_source = ($copyHash -eq $srcHash)
            request       = $t.request
            request_file  = $reqPath
            watchdog_seconds = $wdSec
            a_stack_at    = $trialAStackAt
            kernel_pid    = $startRec["pid"]
            kernel_started_at = $startRec["started_at"]
            kernel_listening_5555 = $startRec["listening"]
            exit_code     = $null
            verdict       = $null
            skipped       = $false
        }
        $currentGroupResult = $tresult
        if (-not $startRec["listening"]) {
            $tresult["verdict"] = "kernel_start_failed"
            $tresult["exit_code"] = 2
            $tresult["data_class"] = "tooling_invalid"
            $trialResults += $tresult
            Stop-ExperimentKernel $startRec["pid"] $tresult
            $currentKernelPid = -1
            $currentGroupResult = $null
            continue
        }
        Write-Ok "kernel started pid=$($startRec['pid'])"

        # warmup: warm the symbol cache and take a healthy baseline snapshot
        $warmLog = Join-Path $tdir "warmup_console.log"
        $null = Invoke-Probe -ArgumentList @("stacks", "--pid", "$($startRec['pid'])", "--out", (Join-Path $tdir "warmup_baseline_stacks.json")) -ConsoleLog $warmLog

        Write-Step "Monitor probe loop (stop-at-watchdog)"
        $consoleLog = Join-Path $tdir "monitor_console.log"
        $monExit = Invoke-Probe -ArgumentList @(
            "monitor",
            "--out-dir", $tdir,
            "--project-copy", $copy,
            "--request-file", $reqPath,
            "--kernel-pid", "$($startRec['pid'])",
            "--group", $label,
            "--watchdog-seconds", "$wdSec",
            "--a-stack-at", "$trialAStackAt",
            "--ping-interval", "$PingIntervalSeconds",
            "--observe-after-watchdog", "10",
            "--max-seconds", "$maxSec",
            "--stop-at-watchdog"
        ) -ConsoleLog $consoleLog
        Get-Content -LiteralPath $consoleLog | Write-Host
        $tresult["exit_code"] = $monExit

        $grPath = Join-Path $tdir "group_result.json"
        if (Test-Path $grPath) {
            $gr = Get-Content $grPath -Raw | ConvertFrom-Json
            $tresult["verdict"] = $gr.verdict
            $tresult["watchdog_triggered"] = [bool]$gr.watchdog_triggered
            $tresult["watchdog_event_class"] = if ($gr.watchdog_event) { $gr.watchdog_event._class } else { $null }
            $tresult["pub_broken"] = [bool]$gr.pub_broken
            $tresult["missing_evidence"] = $gr.missing_evidence
            $tresult["requests"] = $gr.requests
            if ($gr.final_render_files) { $tresult["final_render_files"] = @($gr.final_render_files) }
        }
        else {
            $tresult["verdict"] = "monitor_produced_no_group_result"
        }
        $tresult["thread_states_post"] = Get-ThreadStates $startRec["pid"]

        # copy the kernel log written during this trial's window
        $since = [datetime]::Parse($startRec["started_at"]).AddMinutes(-1)
        $logs = Get-KernelLogs $since
        if ($logs -and $logs.Count -gt 0) {
            $klog = $logs | Sort-Object LastWriteTime -Descending | Select-Object -First 1
            Copy-Item -LiteralPath $klog.FullName -Destination (Join-Path $tdir $klog.Name) -Force
            $tresult["kernel_log"] = Join-Path $tdir $klog.Name
            Write-Ok ("kernel log captured: $($klog.Name)")
        }
        else {
            Write-Warn2 "no kernel log found for this trial's window"
        }

        Stop-ExperimentKernel $startRec["pid"] $tresult

        if ($tresult["verdict"] -in $validVerdicts) { $tresult["data_class"] = "discriminative" }
        elseif ($tresult["verdict"] -in @("kernel_start_failed", "monitor_produced_no_group_result")) { $tresult["data_class"] = "tooling_invalid" }
        else { $tresult["data_class"] = "invalid" }
        $trialResults += $tresult
        $currentKernelPid = -1
        $currentGroupResult = $null
    }
    }
}
catch {
    Write-Warn2 ("orchestrator exception: {0}" -f $_.Exception.Message)
    if ($currentKernelPid -gt 0) {
        $cleanupRec = @{}
        if ($null -ne $currentGroupResult) { $cleanupRec = $currentGroupResult }
        Stop-ExperimentKernel $currentKernelPid $cleanupRec
    }
    Write-Result @{
        error       = "orchestrator exception: $($_.Exception.Message)"
        environment = $env_
        groups      = $groupResults
    } 2 $runDir
}

if ($Mode -eq "ParamSweep") {
    $matrix = @()
    foreach ($tr in $trialResults) {
        if ($tr.skipped) {
            $matrix += [ordered]@{ trial = $tr.label; skipped = $true; params = ""; outcome = "skipped" }
            continue
        }
        $progressCount = 0
        $finalWav = 0
        $evPath = Join-Path (Join-Path $runDir $tr.label) "monitor_events.jsonl"
        if (Test-Path $evPath) {
            foreach ($line in Get-Content -LiteralPath $evPath) {
                if ($line -notmatch '"kind": "pub_event"') { continue }
                try { $ev = $line | ConvertFrom-Json } catch { continue }
                if ($ev.cls -eq "render_progress") { $progressCount++ }
            }
        }
        if ($tr.final_render_files) {
            foreach ($f in @($tr.final_render_files)) { if ([int64]$f.size -gt $finalWav) { $finalWav = [int64]$f.size } }
        }
        $outcome = switch ($tr.verdict) {
            "healthy" { "pass" }
            { $_ -in @("watchdog_wedge", "watchdog_wedge_second_request", "ping_dead_no_forensics") } { "wedge" }
            "render_error_terminal" { "error" }
            default { "invalid($($tr.verdict))" }
        }
        $matrix += [ordered]@{
            trial            = $tr.label
            skipped          = $false
            project          = Split-Path -Leaf $tr.project_source
            params           = ($tr.request | ConvertTo-Json -Compress -Depth 6)
            verdict          = $tr.verdict
            outcome          = $outcome
            watchdog_triggered = [bool]$tr.watchdog_triggered
            progress_events  = $progressCount
            final_wav_bytes  = $finalWav
            data_class       = $tr.data_class
        }
    }
    $matrix | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $runDir "sweep_matrix.json") -Encoding UTF8
    $md = @("# WEDGE-2B1 sweep matrix ($runStamp)", "",
        "| trial | project | params | verdict | outcome | watchdog | progress_events | final_wav_bytes |",
        "|---|---|---|---|---|---|---|---|")
    foreach ($row in $matrix) {
        $md += "| $($row.trial) | $(if ($row.skipped) { '- (skipped)' } else { $row.project }) | $($row.params) | $(if ($row.skipped) { '-' } else { $row.verdict }) | $($row.outcome) | $(if ($row.skipped) { '-' } else { $row.watchdog_triggered }) | $(if ($row.skipped) { '-' } else { $row.progress_events }) | $(if ($row.skipped) { '-' } else { $row.final_wav_bytes }) |"
    }
    $md | Set-Content -LiteralPath (Join-Path $runDir "sweep_matrix.md") -Encoding UTF8
    Write-Host "== Sweep matrix =="
    $md | ForEach-Object { Write-Host $_ }

    $dataTrials = @($trialResults | Where-Object { -not $_.skipped })
    $validCount = @($dataTrials | Where-Object { $_.data_class -eq "discriminative" }).Count
    if ($validCount -eq 0) { $overallClass = 2 }
    elseif ($validCount -lt $dataTrials.Count) { $overallClass = 1 }
    else { $overallClass = 0 }
}

$finalCode = $overallClass
$result = @{
    environment = $env_
    groups      = $groupResults
    interpretation = @{
        exit_0_means = "both groups truly completed render + second same-param render"
        exit_1_means = "anomaly observed with evidence (diagnostic delivery, not a failure of the card)"
        exit_2_means = "environment/protocol/tooling failure; experiment undecidable"
    }
}
if ($Mode -eq "ParamSweep") {
    $result["trials"] = $trialResults
    $result["matrix"] = $matrix
    $result["interpretation"] = @{
        exit_0_means = "every executed trial produced a discriminative outcome (wedge / pass / error terminal are all data)"
        exit_1_means = "some executed trials were invalid (tooling/environment); see data_class per trial"
        exit_2_means = "no trial produced data; the matrix is undecidable"
    }
}
Write-Result $result $finalCode $runDir

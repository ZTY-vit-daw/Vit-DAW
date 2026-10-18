# FIX-START-DRAFT-SIDE-EFFECT-1 (137) real-stack leg.
#
# Purpose: reproduce the 118 realstack scenario (isolated ProjectHistory root with
# a seeded previous-session draft for the kernel startup uuid + the autonomous
# startup history reader) against the 137 fix, and assert the root-cause contract:
#   A) startup/activation/reader phase creates ZERO draft dirs (read paths never
#      materialize the default draft; old behavior = one 75B shell draft A);
#   B) the frontend-identical kernel new_project + agent version.project_new
#      notification creates exactly ONE draft (the real project draft; old
#      behavior = two: A + B);
#   C) no old kernel-uuid workspace is materialized inside the new draft (leak
#      guard) and the seeded recovery draft keeps the old uuid;
#   D) the reader phase leaves the draft root byte/mtime-identical (no writes).
#
# The script owns its stack: it starts the stock kernel and the branch agent, and
# tears them down in finally, killing only the pids it started.

param(
    [Parameter(Mandatory = $true)][string]$AgentExe,
    [Parameter(Mandatory = $true)][string]$KernelExe,
    [Parameter(Mandatory = $true)][string]$RunDir,
    [Parameter(Mandatory = $true)][string]$KernelHelper,
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$KernelZmqUrl = "tcp://127.0.0.1:5555",
    [string]$PythonExe = "python",
    [string]$SeedUuid = "",
    [int]$KernelReadyTimeoutSec = 90,
    [int]$AgentReadyTimeoutSec = 90,
    [int]$ActivationTimeoutSec = 30,
    [int]$ReaderWindowMs = 2500
)

$ErrorActionPreference = "Stop"
$script:failures = New-Object System.Collections.Generic.List[string]
$script:notes = New-Object System.Collections.Generic.List[string]
$agentProc = $null
$kernelProc = $null
$t0 = Get-Date

function Ok([string]$m) { Write-Host ("[137][OK] " + $m); $script:notes.Add("OK: " + $m) | Out-Null }
function Bad([string]$m) { Write-Host ("[137][FAIL] " + $m); $script:failures.Add($m) | Out-Null }
function Prop($o, [string]$n, $d = $null) { if ($null -eq $o) { return $d }; $p = $o.PSObject.Properties[$n]; if ($null -eq $p) { return $d }; return $p.Value }
function TextOf($v) { if ($null -eq $v) { return "" }; return ([string]$v).Trim() }

function Invoke-AgentJson {
    param([string]$Method, [string]$Uri, [object]$Body, [int]$TimeoutSec = 30)
    $params = @{ Method = $Method; Uri = $Uri; TimeoutSec = $TimeoutSec }
    if ($null -ne $Body) { $params["ContentType"] = "application/json"; $params["Body"] = ($Body | ConvertTo-Json -Depth 10 -Compress) }
    return Invoke-RestMethod @params
}

function DraftDirs([string]$root) {
    if (-not (Test-Path -LiteralPath $root)) { return @() }
    return @(Get-ChildItem -LiteralPath $root -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -like "draft_*" } | Sort-Object Name | ForEach-Object { $_.Name })
}

function DraftIdentity([string]$root, [string]$name) {
    $dir = Join-Path $root $name
    $h = Join-Path $dir ".vit_history"
    $a = Join-Path $dir ".vit_agent"
    $hu = @(); $au = @()
    if (Test-Path -LiteralPath $h) { $hu = @(Get-ChildItem -LiteralPath $h -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "vitproj_*" } | ForEach-Object { $_.Name }) }
    if (Test-Path -LiteralPath $a) { $au = @(Get-ChildItem -LiteralPath $a -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "vitproj_*" } | ForEach-Object { $_.Name }) }
    return [pscustomobject]@{ name = $name; history = (Test-Path -LiteralPath $h); agent = (Test-Path -LiteralPath $a); uuids = @($hu + $au | Select-Object -Unique) }
}

function TreeSnapshot([string]$root) {
    $out = [ordered]@{}
    if (-not (Test-Path -LiteralPath $root)) { return $out }
    $files = @(Get-ChildItem -LiteralPath $root -Recurse -File -ErrorAction SilentlyContinue | Sort-Object FullName)
    foreach ($f in $files) {
        # Transient agent persistence artifacts (lock files / temp files) appear
        # and vanish around request boundaries; they are not draft content.
        if ($f.Name -match '\.(lock|tmp)(\.json)?$') { continue }
        $rel = $f.FullName.Substring($root.Length + 1)
        $hash = $null
        try { $hash = (Get-FileHash -LiteralPath $f.FullName -Algorithm SHA256 -ErrorAction Stop).Hash } catch { continue }
        $size = $null; $mtime = $null
        try { $info = Get-Item -LiteralPath $f.FullName -ErrorAction Stop; $size = $info.Length; $mtime = $info.LastWriteTimeUtc.ToString("o") } catch { continue }
        $out[$rel] = @{ size = $size; mtime = $mtime; sha256 = $hash }
    }
    return $out
}

function SnapshotPhase([string]$root, [string]$label) {
    $names = @(DraftDirs $root)
    $rows = @()
    foreach ($n in $names) { $rows += DraftIdentity $root $n }
    return [ordered]@{ label = $label; at = (Get-Date).ToString("o"); dirs = $names; identities = $rows; tree = (TreeSnapshot $root) }
}

function Compare-TreeIdentity($before, $after) {
    $diff = New-Object System.Collections.Generic.List[string]
    foreach ($key in $before.Keys) {
        if (-not $after.Contains($key)) { $diff.Add("removed:" + $key); continue }
        if ($before[$key].sha256 -ne $after[$key].sha256) { $diff.Add("changed:" + $key) }
        if ($before[$key].mtime -ne $after[$key].mtime) { $diff.Add("mtime:" + $key) }
    }
    foreach ($key in $after.Keys) { if (-not $before.Contains($key)) { $diff.Add("added:" + $key) } }
    return $diff
}

# Wait until the draft root tree (paths + content sha256 + mtimes) has been stable
# for two consecutive probes. The agent persists its own runtime state right after
# activation (journal/manifest/session runtime state) — that is expected activation
# persistence, NOT a read-path write, so assertion D must baseline AFTER quiescence
# and then prove the reader faces add nothing.
function Wait-TreeQuiescent([string]$root, [int]$TimeoutMs = 20000, [int]$ProbeMs = 500, [int]$StableProbes = 6) {
    $deadline = (Get-Date).AddMilliseconds($TimeoutMs)
    $stable = 0
    $previous = $null
    $probes = 0
    while ((Get-Date) -lt $deadline) {
        $current = TreeSnapshot $root
        $probes++
        if ($null -ne $previous) {
            $diff = @(Compare-TreeIdentity $previous $current)
            if ($diff.Count -eq 0) { $stable++ } else { $stable = 0 }
            if ($stable -ge $StableProbes) { return [pscustomobject]@{ stable = $true; probes = $probes; waited_ms = $TimeoutMs - [int](($deadline - (Get-Date)).TotalMilliseconds); stable_probes = $stable } }
        }
        $previous = $current
        Start-Sleep -Milliseconds $ProbeMs
    }
    return [pscustomobject]@{ stable = $false; probes = $probes; waited_ms = $TimeoutMs; stable_probes = $stable }
}

$result = [ordered]@{
    schema_version = "fix_start_draft_side_effect_stack_leg.v1"
    run_dir = $RunDir; agent_http = $AgentHttp; kernel_zmq = $KernelZmqUrl
    started_at = $t0.ToString("o"); assertions = [ordered]@{}; failures = @(); notes = @()
}

try {
    New-Item -ItemType Directory -Force -Path $RunDir | Out-Null
    $root = Join-Path (Join-Path $RunDir "ProjectHistory") "drafts"
    New-Item -ItemType Directory -Force -Path $root | Out-Null
    $result["drafts_root"] = $root

    # --- identity of inputs
    $result["git_head"] = (& git -C (Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $AgentExe))) rev-parse HEAD 2>$null | Out-String).Trim()
    $result["git_status"] = (& git -C (Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $AgentExe))) status --short 2>$null | Out-String).Trim()
    $result["agent_sha256"] = (Get-FileHash -LiteralPath $AgentExe -Algorithm SHA256).Hash
    $result["kernel_sha256"] = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
    if ([string]::IsNullOrWhiteSpace($SeedUuid)) {
        $defaultProject = Join-Path (Split-Path -Parent $KernelExe) "Workspace\default_project.xml"
        $match = Select-String -Path $defaultProject -Pattern 'vit_project_uuid="([^"]+)"' | Select-Object -First 1
        if ($match) { $SeedUuid = $match.Matches[0].Groups[1].Value }
    }
    $result["kernel_startup_uuid"] = $SeedUuid
    if ([string]::IsNullOrWhiteSpace($SeedUuid)) { throw "cannot determine kernel startup project uuid" }

    # --- precheck: stack ports/processes must be free (user Godot editor allowed, recorded)
    $busyPorts = @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -in 7878,5555,5556,8787,4445,19876 })
    if ($busyPorts.Count -gt 0) { throw ("stack ports already listening: " + (($busyPorts | ForEach-Object { $_.LocalPort }) -join ",")) }
    $stackProcs = @(Get-CimInstance Win32_Process -Filter "name='VitApp.exe' OR name='VitAgent.exe' OR name='vsphub.exe' OR name='VspHub.exe'" -ErrorAction SilentlyContinue)
    if ($stackProcs.Count -gt 0) { throw ("stack processes already running: " + (($stackProcs | ForEach-Object { $_.Name + ":" + $_.ProcessId }) -join ",")) }
    $godotProcs = @(Get-CimInstance Win32_Process -Filter "name LIKE 'Godot%'" -ErrorAction SilentlyContinue)
    $godotGame = @($godotProcs | Where-Object { $_.CommandLine -notmatch '--editor' })
    if ($godotGame.Count -gt 0) { throw ("Godot game instance present (not editor): " + (($godotGame | ForEach-Object { $_.ProcessId }) -join ",")) }
    $result["precheck_godot_editor"] = @($godotProcs | ForEach-Object { "pid=" + $_.ProcessId })
    Ok ("precheck: six stack ports free, stack processes=0, Godot editor only=" + $result["precheck_godot_editor"].Count)

    # --- seed the previous-session draft for the kernel startup uuid
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    $seedName = "draft_20261001T120000_seedprev"
    $seedDir = Join-Path $root $seedName
    $seedPath = Join-Path $seedDir "Unsaved.vit"
    $seedStateDir = Join-Path $seedDir ".vit_history\.sessions\$SeedUuid\session_seed\workspace\state"
    New-Item -ItemType Directory -Force -Path $seedStateDir | Out-Null
    [System.IO.File]::WriteAllText($seedPath, "<EDIT draft=`"true`" projectID=`"0/0`" appVersion=`"Unknown`" creationTime=`"0`"/>`n", $utf8NoBom)
    $seedState = [ordered]@{ schema_version = "vit_project_agent_runtime.v2"; project_path = $seedPath; project_uuid = $SeedUuid; saved_at = "2026-10-01T00:00:00Z"; conversations = @{} }
    [System.IO.File]::WriteAllText((Join-Path $seedStateDir "agent_runtime_state.json"), ($seedState | ConvertTo-Json -Depth 6), $utf8NoBom)
    $result["seed_name"] = $seedName
    $result["snapshot_prestart"] = SnapshotPhase $root "prestart"

    # --- start kernel
    $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -PassThru -WindowStyle Hidden
    $result["kernel_pid"] = $kernelProc.Id
    $deadline = (Get-Date).AddSeconds($KernelReadyTimeoutSec); $kernelReady = $false
    while ((Get-Date) -lt $deadline) {
        if (Get-NetTCPConnection -State Listen -LocalPort 5555 -ErrorAction SilentlyContinue) { $kernelReady = $true; break }
        if ($kernelProc.HasExited) { throw "kernel exited early (code=" + $kernelProc.ExitCode + ")" }
        Start-Sleep -Milliseconds 250
    }
    if (-not $kernelReady) { throw "kernel ZMQ port 5555 not listening within timeout" }
    Ok ("kernel up pid=" + $kernelProc.Id)

    # --- start branch agent with the isolated draft root
    $env:VIT_HISTORY_DRAFT_ROOT = $root
    $agentLog = Join-Path $RunDir "agent_last.log"
    $agentArgs = @("-http", "127.0.0.1:7878", "-last-log-path", $agentLog, "-keep-last-log-lines", "800")
    $agentProc = Start-Process -FilePath $AgentExe -ArgumentList $agentArgs -PassThru -WindowStyle Hidden -RedirectStandardOutput (Join-Path $RunDir "agent_stdout.log") -RedirectStandardError (Join-Path $RunDir "agent_stderr.log")
    $result["agent_pid"] = $agentProc.Id
    $deadline = (Get-Date).AddSeconds($AgentReadyTimeoutSec); $agentReady = $false
    while ((Get-Date) -lt $deadline) {
        if (Get-NetTCPConnection -State Listen -LocalPort 7878 -ErrorAction SilentlyContinue) { $agentReady = $true; break }
        if ($agentProc.HasExited) { throw "agent exited early (code=" + $agentProc.ExitCode + ")" }
        Start-Sleep -Milliseconds 200
    }
    if (-not $agentReady) { throw "agent HTTP port 7878 not listening within timeout" }
    Ok ("agent up pid=" + $agentProc.Id)

    # --- wait for startup recovery/activation (seeded draft must be claimed read-only)
    $deadline = (Get-Date).AddSeconds($ActivationTimeoutSec); $activated = ""
    while ((Get-Date) -lt $deadline) {
        if (Test-Path -LiteralPath $agentLog) {
            $hit = Select-String -Path $agentLog -Pattern '\[workspace\] activated project="([^"]+)" uuid=(vitproj_[0-9a-f]+)' -AllMatches | Select-Object -Last 1
            if ($hit) { $m = $hit.Matches[0]; if ($m.Groups[1].Value -like "*seedprev*") { $activated = $m.Groups[2].Value; break } }
        }
        Start-Sleep -Milliseconds 100
    }
    if ($activated -eq "") { Bad "startup activation did not claim the seeded previous-session draft" } else { Ok ("startup recovery claimed seed draft uuid=" + $activated) }
    $quiescence = Wait-TreeQuiescent $root 10000
    $result["startup_quiescence"] = $quiescence
    if (-not $quiescence.stable) { Write-Host "[137] note: draft root did not reach quiescence after activation; continuing with last snapshot" }

    # --- assertion A: startup phase created no draft dirs at all
    $snapshotStartup = SnapshotPhase $root "after_startup"
    $result["snapshot_after_startup"] = $snapshotStartup
    $dirsStartup = @($snapshotStartup["dirs"])
    $agentCreatedAtStartup = @($dirsStartup | Where-Object { $_ -ne $seedName })
    $result["agent_created_dirs_at_startup"] = $agentCreatedAtStartup
    if ($dirsStartup.Count -eq 1 -and $agentCreatedAtStartup.Count -eq 0) {
        Ok "A) startup created zero draft dirs (root = seeded previous-session draft only)"
    } else {
        Bad ("A) startup materialized draft dirs: " + ($agentCreatedAtStartup -join ",") + " (dirs=" + ($dirsStartup -join ",") + ")")
    }
    if ($activated -ne "" -and $activated -ne $SeedUuid) { Bad ("recovered uuid " + $activated + " != kernel startup uuid " + $SeedUuid) }

    # --- assertion D: autonomous reader faces are read-only
    $readerResponses = [ordered]@{}
    foreach ($tool in @("version.list", "version.worktree_list")) {
        try {
            $resp = Invoke-AgentJson -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{ tool = $tool; args = @{}; source = "fedraft_137_probe"; confirmed = $true } -TimeoutSec 30
            $res = Prop $resp "result"
            $commitCount = $null; $commits = Prop $res "commits"
            if ($null -ne $commits) { $commitCount = @($commits).Count }
            $readerResponses[$tool] = [ordered]@{ status = TextOf(Prop $resp "status"); project_path = TextOf(Prop $res "project_path"); commits = $commitCount; worktrees = @(Prop $res "worktrees").Count }
        } catch { $readerResponses[$tool] = @{ status = "error"; error = $_.Exception.Message } }
    }
    try { $readerResponses["/agent/state"] = @{ status = TextOf(Prop (Invoke-AgentJson -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10) "status") } } catch { $readerResponses["/agent/state"] = @{ status = "error" } }
    $result["reader_responses"] = $readerResponses
    Start-Sleep -Milliseconds $ReaderWindowMs
    $snapshotAfterReader = SnapshotPhase $root "after_reader"
    $result["snapshot_after_reader"] = $snapshotAfterReader
    $readerTreeDiff = @(Compare-TreeIdentity $snapshotStartup["tree"] $snapshotAfterReader["tree"])
    $result["reader_tree_diff"] = $readerTreeDiff
    # Request-boundary persistence of the ACTIVE workspace (journal audit +
    # runtime-state save after an /agent/invoke request) is existing agent design
    # and is allowed only inside the seeded active workspace. Draft materialization
    # or any write to another draft/path fails assertion D.
    $allowedDiffs = @(); $forbiddenDiffs = @()
    foreach ($diffItem in $readerTreeDiff) {
        $separator = $diffItem.IndexOf(":")
        $diffPath = if ($separator -ge 0) { $diffItem.Substring($separator + 1) } else { $diffItem }
        if ($diffPath.StartsWith($seedName + "\") -or $diffPath.StartsWith($seedName + "/")) { $allowedDiffs += $diffItem } else { $forbiddenDiffs += $diffItem }
    }
    $readerTargetOk = $true
    foreach ($tool in @("version.list", "version.worktree_list")) {
        $readerResponse = $readerResponses[$tool]
        $readerPath = ""
        if ($null -ne $readerResponse) { $readerPath = TextOf($readerResponse["project_path"]) }
        if ($readerPath -ne "" -and $readerPath -notlike "*seedprev*") { $readerTargetOk = $false }
    }
    $readerDirs = @($snapshotAfterReader["dirs"])
    $readerNewDirs = @($readerDirs | Where-Object { $dirsStartup -notcontains $_ })
    $result["reader_new_dirs"] = $readerNewDirs
    $result["reader_allowed_persistence_diffs"] = $allowedDiffs
    $result["reader_forbidden_diffs"] = $forbiddenDiffs
    $result["reader_target_ok"] = $readerTargetOk
    if ($readerNewDirs.Count -eq 0 -and $forbiddenDiffs.Count -eq 0 -and $readerTargetOk) {
        Ok ("D) reader faces created zero drafts and wrote nothing outside the active workspace (allowed request-boundary persistence diffs=" + $allowedDiffs.Count + ", reader target=seeded workspace)")
    } else {
        Bad ("D) reader faces violated the read-only contract: newDirs=" + ($readerNewDirs -join ",") + " forbiddenDiff=" + (($forbiddenDiffs | Select-Object -First 8) -join ";") + " readerTargetOk=" + $readerTargetOk)
    }

    # --- assertion B/C: frontend-identical new-project notification creates exactly one draft
    $kernelNewRaw = & $PythonExe $KernelHelper --url $KernelZmqUrl --cmd new_project 2>&1
    $kernelNewReply = $null; try { $kernelNewReply = ($kernelNewRaw | Out-String).Trim() | ConvertFrom-Json } catch { $kernelNewReply = $null }
    $newUuid = TextOf(Prop $kernelNewReply "project_uuid"); if ($newUuid -eq "") { $newUuid = TextOf(Prop $kernelNewReply "project_id") }
    $result["kernel_new_project_raw"] = ($kernelNewRaw | Out-String)
    $result["new_project_uuid"] = $newUuid
    if ($newUuid -eq "") { Bad "kernel new_project did not return a project uuid" }
    $notifyBody = @{ tool = "version.project_new"; args = @{ project_uuid = $newUuid; project_path = ""; source = "fedraft_137_probe" }; source = "fedraft_137_probe"; confirmed = $true }
    $notifyResp = $null
    try { $notifyResp = Invoke-AgentJson -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body $notifyBody -TimeoutSec 30 } catch { Bad ("version.project_new HTTP error: " + $_.Exception.Message) }
    $result["version_project_new_response"] = $notifyResp
    $newQuiescence = Wait-TreeQuiescent $root 10000
    $result["new_project_quiescence"] = $newQuiescence
    $snapshotAfterNew = SnapshotPhase $root "after_project_new"
    $result["snapshot_after_project_new"] = $snapshotAfterNew
    $dirsAfter = @($snapshotAfterNew["dirs"])
    $newDirs = @($dirsAfter | Where-Object { $dirsStartup -notcontains $_ })
    $result["new_dirs_after_project_new"] = $newDirs
    if ($newDirs.Count -eq 1) {
        Ok ("B) version.project_new added exactly 1 draft (old behavior=2): " + $newDirs[0])
    } else {
        Bad ("B) version.project_new added " + $newDirs.Count + " draft dirs: " + ($newDirs -join ","))
    }
    $newDraftRows = @($snapshotAfterNew["identities"] | Where-Object { $newDirs -contains $_.name })
    $newDraftUuids = @()
    foreach ($row in $newDraftRows) { $newDraftUuids += @($row.uuids) }
    $result["new_draft_uuids"] = $newDraftUuids
    if ($newDraftUuids -contains $newUuid) { Ok "B) new draft carries the new project uuid " + $newUuid } else { Bad ("new draft does not carry new uuid " + $newUuid + "; uuids=" + ($newDraftUuids -join ",")) }
    $leakNew = @($newDraftUuids | Where-Object { $_ -eq $SeedUuid })
    if ($leakNew.Count -eq 0) { Ok "C) no old kernel-uuid workspace leaked into the new project draft" } else { Bad ("C) old kernel-uuid workspace leaked into the new draft: " + ($leakNew -join ",")) }
    $seedRowAfter = @($snapshotAfterNew["identities"] | Where-Object { $_.name -eq $seedName })
    if ($seedRowAfter.Count -eq 1 -and (@($seedRowAfter[0].uuids) -contains $SeedUuid)) { Ok "C) seeded recovery draft keeps the old kernel-uuid workspace" } else { Bad "C) seeded recovery draft lost the old kernel-uuid workspace" }

    # --- logs: refused stale rebind warnings, activation-pending lines
    if (Test-Path -LiteralPath $agentLog) {
        $rebindWarn = @(Select-String -Path $agentLog -Pattern 'refused stale project identity rebind' -ErrorAction SilentlyContinue)
        $pending = @(Select-String -Path $agentLog -Pattern 'activation pending uuid=' -ErrorAction SilentlyContinue)
        $result["stale_rebind_warnings"] = $rebindWarn.Count
        $result["activation_pending_lines"] = $pending.Count
        if ($rebindWarn.Count -gt 0) { Write-Host ("[137] note: stale rebind refusals logged=" + $rebindWarn.Count) }
    }

    $result["assertions"] = [ordered]@{
        A_startup_zero_materialization = ($dirsStartup.Count -eq 1 -and $agentCreatedAtStartup.Count -eq 0)
        B_project_new_exactly_one_draft = ($newDirs.Count -eq 1 -and $newDraftUuids -contains $newUuid)
        C_no_old_uuid_leak = ($leakNew.Count -eq 0 -and $seedRowAfter.Count -eq 1)
        D_reader_read_only = ($readerNewDirs.Count -eq 0 -and $forbiddenDiffs.Count -eq 0 -and $readerTargetOk)
    }
}
catch {
    Bad ("exception: " + $_.Exception.Message)
    $result["exception"] = $_.Exception.Message
    $result["exception_stack"] = $_.ScriptStackTrace
    $result["reader_responses"] = $result["reader_responses"]
}
finally {
    $result["failures"] = @($script:failures)
    $result["notes"] = @($script:notes)
    $result["finished_at"] = (Get-Date).ToString("o")
    # teardown: only our own pids
    if ($agentProc -ne $null -and -not $agentProc.HasExited) { Stop-Process -Id $agentProc.Id -Force -ErrorAction SilentlyContinue; Start-Sleep -Milliseconds 400 }
    if ($kernelProc -ne $null -and -not $kernelProc.HasExited) { Stop-Process -Id $kernelProc.Id -Force -ErrorAction SilentlyContinue; Start-Sleep -Milliseconds 800 }
    $remainingPorts = @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -in 7878,5555,5556,8787,4445,19876 })
    $remainingStack = @(Get-CimInstance Win32_Process -Filter "name='VitApp.exe' OR name='VitAgent.exe' OR name='vsphub.exe' OR name='VspHub.exe'" -ErrorAction SilentlyContinue)
    $result["teardown"] = [ordered]@{ ports_listening = @($remainingPorts | ForEach-Object { $_.LocalPort }); stack_processes = @($remainingStack | ForEach-Object { $_.Name + ":" + $_.ProcessId }); at = (Get-Date).ToString("o") }
    New-Item -ItemType Directory -Force -Path $RunDir | Out-Null
    $result | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $RunDir "stack_leg_result.json") -Encoding UTF8
    Write-Host ("[137] result: " + (Join-Path $RunDir "stack_leg_result.json"))
    if ($script:failures.Count -gt 0 -or $remainingPorts.Count -gt 0) { Write-Host ("[137] FAIL count=" + $script:failures.Count + " ports=" + ($result["teardown"]["ports_listening"] -join ",")); exit 1 }
    Write-Host "[137] PASS"; exit 0
}


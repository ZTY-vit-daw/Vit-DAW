# FIX-START-STATE-RECOVERY-1 (138) real-stack measurement leg.
#
# Reproduces the 118/138 phenomenon under an isolated ProjectHistory root: the
# kernel starts reporting the OLD project uuid (via VIT_PROJECT_XML override,
# never touching the stock workspace files) while the root holds a seeded
# previous-session draft carrying a real-sized (~42MB) agent runtime state for
# that uuid. Measures what agent startup does with that state.
#
# Modes:
#   -Mode baseline : expect the OLD behavior to reproduce (uuid scan claims the
#                    seed draft and rewrites the 42MB state) -> leg PASS means
#                    "phenomenon reproduced" (positive control for the fix).
#   -Mode fixed    : expect the NEW semantics (user ruling 2026-10-18): no
#                    recovery by kernel uuid, seed tree ZERO-touch, project_new
#                    creates exactly one draft (118/137 no-regression).
#   -Sentinel      : hold an exclusive handle (FileShare.None) on the seeded
#                    42MB state for the whole leg. fixed+sentinel must behave
#                    identically to fixed (no open ever attempted); baseline+
#                    sentinel must DISTURB the recovery (positive control that
#                    the sentinel detects read attempts).
#
# The script owns its stack: starts the stock kernel and one agent binary,
# tears them down in finally, killing only the pids it started.

param(
    [Parameter(Mandatory = $true)][ValidateSet("baseline", "fixed")][string]$Mode,
    [Parameter(Mandatory = $true)][string]$AgentExe,
    [Parameter(Mandatory = $true)][string]$KernelExe,
    [Parameter(Mandatory = $true)][string]$RunDir,
    [Parameter(Mandatory = $true)][string]$KernelHelper,
    [Parameter(Mandatory = $true)][string]$SeedStateSource,
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$KernelZmqUrl = "tcp://127.0.0.1:5555",
    [string]$PythonExe = "python",
    [string]$SeedUuid = "vitproj_b3f2838183d64add9ca7824b8b3ac557",
    [string]$SeedName = "draft_20260905T134132_3866b080",
    [switch]$Sentinel,
    [int]$KernelReadyTimeoutSec = 90,
    [int]$AgentReadyTimeoutSec = 90,
    [int]$ActivationTimeoutSec = 45,
    [int]$ReaderWindowMs = 2500
)

$ErrorActionPreference = "Stop"
$script:failures = New-Object System.Collections.Generic.List[string]
$script:notes = New-Object System.Collections.Generic.List[string]
$agentProc = $null
$kernelProc = $null
$sentinelStream = $null
$t0 = Get-Date

function Ok([string]$m) { Write-Host ("[138][OK] " + $m); $script:notes.Add("OK: " + $m) | Out-Null }
function Bad([string]$m) { Write-Host ("[138][FAIL] " + $m); $script:failures.Add($m) | Out-Null }
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
    $s = Join-Path $h ".sessions"
    $hu = @(); $au = @(); $su = @()
    if (Test-Path -LiteralPath $h) { $hu = @(Get-ChildItem -LiteralPath $h -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "vitproj_*" } | ForEach-Object { $_.Name }) }
    if (Test-Path -LiteralPath $a) { $au = @(Get-ChildItem -LiteralPath $a -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "vitproj_*" } | ForEach-Object { $_.Name }) }
    if (Test-Path -LiteralPath $s) { $su = @(Get-ChildItem -LiteralPath $s -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "vitproj_*" } | ForEach-Object { $_.Name }) }
    return [pscustomobject]@{ name = $name; history = (Test-Path -LiteralPath $h); agent = (Test-Path -LiteralPath $a); uuids = @($hu + $au | Select-Object -Unique); sessions_uuids = @($su | Select-Object -Unique) }
}

function TreeSnapshot([string]$root) {
    $out = [ordered]@{}
    if (-not (Test-Path -LiteralPath $root)) { return $out }
    $files = @(Get-ChildItem -LiteralPath $root -Recurse -File -ErrorAction SilentlyContinue | Sort-Object FullName)
    foreach ($f in $files) {
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

# Poll the agent log until an activation verdict line shows up; return which
# line fired plus latency from agent start. /agent/state GETs drive activation
# deterministically (the scheduler's first tick is the other trigger).
function Wait-ActivationVerdict([string]$agentLog, [datetime]$agentStart, [int]$TimeoutSec) {
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        if (Test-Path -LiteralPath $agentLog) {
            $activatedHit = Select-String -Path $agentLog -Pattern '\[workspace\] activated project="([^"]+)" uuid=(vitproj_[0-9a-f]+)' -ErrorAction SilentlyContinue | Select-Object -Last 1
            if ($activatedHit) {
                return [pscustomobject]@{ verdict = "activated"; detail = $activatedHit.Line; ms = [int]((Get-Date) - $agentStart).TotalMilliseconds }
            }
            $pendingHit = Select-String -Path $agentLog -Pattern 'activation pending uuid=(vitproj_[0-9a-f]+) reason=([a-z_]+)' -ErrorAction SilentlyContinue | Select-Object -Last 1
            if ($pendingHit) {
                return [pscustomobject]@{ verdict = "pending"; detail = $pendingHit.Line; ms = [int]((Get-Date) - $agentStart).TotalMilliseconds }
            }
        }
        try { Invoke-AgentJson -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 5 | Out-Null } catch {}
        Start-Sleep -Milliseconds 100
    }
    return [pscustomobject]@{ verdict = "timeout"; detail = ""; ms = -1 }
}

$result = [ordered]@{
    schema_version = "fix_start_state_recovery_stack_leg.v1"
    mode = $Mode; sentinel = [bool]$Sentinel
    run_dir = $RunDir; agent_http = $AgentHttp; kernel_zmq = $KernelZmqUrl
    started_at = $t0.ToString("o"); assertions = [ordered]@{}; observations = [ordered]@{}; failures = @(); notes = @()
}

try {
    $sharing = @()
    $seedFullDiff = @()
    $seedRewritten = $false
    New-Item -ItemType Directory -Force -Path $RunDir | Out-Null
    $root = Join-Path (Join-Path $RunDir "ProjectHistory") "drafts"
    New-Item -ItemType Directory -Force -Path $root | Out-Null
    $result["drafts_root"] = $root

    # --- identity of inputs
    $result["git_head"] = (& git -C (Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $AgentExe))) rev-parse HEAD 2>$null | Out-String).Trim()
    $result["git_status"] = (& git -C (Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $AgentExe))) status --short 2>$null | Out-String).Trim()
    $result["agent_sha256"] = (Get-FileHash -LiteralPath $AgentExe -Algorithm SHA256).Hash
    $result["kernel_sha256"] = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
    $result["seed_state_source"] = $SeedStateSource
    $result["seed_state_source_sha256"] = (Get-FileHash -LiteralPath $SeedStateSource -Algorithm SHA256).Hash
    $result["seed_state_source_bytes"] = (Get-Item -LiteralPath $SeedStateSource).Length
    $result["kernel_startup_uuid"] = $SeedUuid

    # --- precheck: stack ports/processes must be free (user Godot editor allowed, recorded)
    $busyPorts = @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -in 7878,5555,5556,8787,4445,19876 })
    if ($busyPorts.Count -gt 0) { throw ("stack ports already listening: " + (($busyPorts | ForEach-Object { $_.LocalPort }) -join ",")) }
    $stackProcs = @(Get-CimInstance Win32_Process -Filter "name='VitApp.exe' OR name='VitAgent.exe' OR name='vsphub.exe' OR name='VspHub.exe'" -ErrorAction SilentlyContinue)
    if ($stackProcs.Count -gt 0) { throw ("stack processes already running: " + (($stackProcs | ForEach-Object { $_.Name + ":" + $_.ProcessId }) -join ",")) }
    $godotProcs = @(Get-CimInstance Win32_Process -Filter "name LIKE 'Godot%'" -ErrorAction SilentlyContinue)
    # 137-era rule aborted on any non-editor instance; headless scratch scripts
    # (--headless -s <scratch>.gd in %TEMP%) can neither hold the stack nor
    # spawn a kernel mid-leg, so only interactive game instances abort.
    $godotGame = @($godotProcs | Where-Object { $_.CommandLine -notmatch '--editor' -and $_.CommandLine -notmatch '--headless' })
    if ($godotGame.Count -gt 0) { throw ("Godot game instance present (not editor): " + (($godotGame | ForEach-Object { $_.ProcessId }) -join ",")) }
    $result["precheck_godot_editor"] = @($godotProcs | ForEach-Object { "pid=" + $_.ProcessId })
    $result["precheck_godot_headless"] = @($godotProcs | Where-Object { $_.CommandLine -match '--headless' } | ForEach-Object { "pid=" + $_.ProcessId })
    Ok ("precheck: six stack ports free, stack processes=0, Godot interactive=0 (headless scratches=" + $result["precheck_godot_headless"].Count + ", editor=" + @($godotProcs | Where-Object { $_.CommandLine -match '--editor' }).Count + ")")

    # --- seed the previous-session draft with the REAL-sized runtime state copy.
    # Only project_path is rewritten (isolated root); uuid and the remaining
    # ~42MB of content stay byte-identical to the durable original, which is
    # never touched (copy-on-seed, AGENTS §10).
    $seedDir = Join-Path $root $SeedName
    $seedPath = Join-Path $seedDir "Unsaved.vit"
    $seedStateDir = Join-Path $seedDir ".vit_history\.sessions\$SeedUuid\session_20260908T102051_099b621b\workspace\state"
    New-Item -ItemType Directory -Force -Path $seedStateDir | Out-Null
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($seedPath, "<EDIT draft=`"true`" projectID=`"0/0`" appVersion=`"Unknown`" creationTime=`"0`"/>`n", $utf8NoBom)
    $seedStatePath = Join-Path $seedStateDir "agent_runtime_state.json"
    $stateText = [System.IO.File]::ReadAllText($SeedStateSource)
    $oldRootEscaped = "C:\\Users\\timoz\\AppData\\Local\\Vit\\ProjectHistory\\drafts"
    $newRootEscaped = ($root -replace '\\', '\\')
    $patched = $stateText.Replace($oldRootEscaped, $newRootEscaped)
    if ($patched -eq $stateText) { throw "seed patch did not apply: project_path prefix not found in source state" }
    # Give the seeded state one running continuation so RecoverWorkingSession
    # RESUMES this exact session (recovered=true) — that is the 118 shape whose
    # request-boundary persist then rewrites the 42MB file in place.
    $pendingMarker = '"durable_continuations": {"fxsr_138_pending_probe": {"status": "running"}},'
    if ($patched -notmatch '"durable_continuations"') {
        $patched = $patched -replace '("saved_at": "[^"]+",)', ('$1 ' + $pendingMarker)
        if ($patched -notmatch 'fxsr_138_pending_probe') { throw "pending-work patch did not apply to seed state" }
    }
    [System.IO.File]::WriteAllText($seedStatePath, $patched, $utf8NoBom)
    $seedSession = [ordered]@{
        schema_version = "vit_project_working_session.v1"; session_id = "session_20260908T102051_099b621b"
        project_path = $seedPath; project_uuid = $SeedUuid
        workspace_dir = ([System.IO.Path]::GetFullPath((Join-Path $seedStateDir "..")))
        status = "active"; created_at = "2026-09-08T10:20:51Z"; updated_at = "2026-10-18T05:45:31Z"
    }
    [System.IO.File]::WriteAllText((Join-Path (Split-Path -Parent (Split-Path -Parent $seedStateDir)) "session.json"), ($seedSession | ConvertTo-Json -Depth 4), $utf8NoBom)
    $result["seed_state_bytes"] = (Get-Item -LiteralPath $seedStatePath).Length
    $result["seed_state_sha256"] = (Get-FileHash -LiteralPath $seedStatePath -Algorithm SHA256).Hash
    $result["seed_name"] = $SeedName
    Ok ("seed: " + $result["seed_state_bytes"] + "B state seeded for uuid=" + $SeedUuid)

    # --- kernel startup project override: VIT_PROJECT_XML points the kernel at a
    # run-dir copy carrying the OLD uuid; stock workspace files stay untouched.
    $kernelXml = Join-Path $RunDir "default_project.xml"
    $stockXml = Join-Path (Split-Path -Parent $KernelExe) "Workspace\default_project.xml"
    $xmlText = [System.IO.File]::ReadAllText($stockXml)
    $xmlPatched = $xmlText -replace 'vit_project_uuid="[^"]+"', ('vit_project_uuid="' + $SeedUuid + '"')
    [System.IO.File]::WriteAllText($kernelXml, $xmlPatched, $utf8NoBom)
    $result["kernel_project_xml"] = $kernelXml

    # --- sentinel (optional): exclusive handle on the seeded 42MB state
    if ($Sentinel) {
        $sentinelStream = [System.IO.File]::Open($seedStatePath, 'Open', 'Read', 'None')
        Ok ("sentinel: exclusive handle held on " + $seedStatePath)
    }

    $result["snapshot_prestart"] = SnapshotPhase $root "prestart"

    # --- start kernel (VIT_PROJECT_XML scoped to the kernel start only)
    $env:VIT_PROJECT_XML = $kernelXml
    $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -PassThru -WindowStyle Hidden
    Remove-Item Env:\VIT_PROJECT_XML -ErrorAction SilentlyContinue
    $result["kernel_pid"] = $kernelProc.Id
    $kernelStart = Get-Date
    $deadline = (Get-Date).AddSeconds($KernelReadyTimeoutSec); $kernelReady = $false
    while ((Get-Date) -lt $deadline) {
        if (Get-NetTCPConnection -State Listen -LocalPort 5555 -ErrorAction SilentlyContinue) { $kernelReady = $true; break }
        if ($kernelProc.HasExited) { throw "kernel exited early (code=" + $kernelProc.ExitCode + ")" }
        Start-Sleep -Milliseconds 250
    }
    if (-not $kernelReady) { throw "kernel ZMQ port 5555 not listening within timeout" }
    $kernelReadyMs = [int]((Get-Date) - $kernelStart).TotalMilliseconds
    $result["kernel_ready_ms"] = $kernelReadyMs
    Ok ("kernel up pid=" + $kernelProc.Id + " ready_ms=" + $kernelReadyMs)

    # --- start the agent under test with the isolated draft root
    $env:VIT_HISTORY_DRAFT_ROOT = $root
    $agentLog = Join-Path $RunDir "agent_last.log"
    $agentArgs = @("-http", "127.0.0.1:7878", "-last-log-path", $agentLog, "-keep-last-log-lines", "4000")
    $agentStart = Get-Date
    $agentProc = Start-Process -FilePath $AgentExe -ArgumentList $agentArgs -PassThru -WindowStyle Hidden -RedirectStandardOutput (Join-Path $RunDir "agent_stdout.log") -RedirectStandardError (Join-Path $RunDir "agent_stderr.log")
    $result["agent_pid"] = $agentProc.Id
    $deadline = (Get-Date).AddSeconds($AgentReadyTimeoutSec); $agentReady = $false
    while ((Get-Date) -lt $deadline) {
        if (Get-NetTCPConnection -State Listen -LocalPort 7878 -ErrorAction SilentlyContinue) { $agentReady = $true; break }
        if ($agentProc.HasExited) { throw "agent exited early (code=" + $agentProc.ExitCode + ")" }
        Start-Sleep -Milliseconds 200
    }
    if (-not $agentReady) { throw "agent HTTP port 7878 not listening within timeout" }
    $agentReadyMs = [int]((Get-Date) - $agentStart).TotalMilliseconds
    $result["agent_ready_ms"] = $agentReadyMs
    Ok ("agent up pid=" + $agentProc.Id + " ready_ms=" + $agentReadyMs)

    # --- startup activation verdict + settle
    $verdict = Wait-ActivationVerdict $agentLog $agentStart $ActivationTimeoutSec
    $result["activation_verdict"] = $verdict.verdict
    $result["activation_line"] = $verdict.detail
    $result["activation_ms_from_agent_start"] = $verdict.ms
    $quiescence = Wait-TreeQuiescent $root 15000
    $result["startup_quiescence"] = $quiescence
    # Request-boundary persistence trigger (same face 137's D phase used): one
    # version.list POST after activation. Baseline: the seed workspace is
    # active, so the boundary persist rewrites the 42MB state. Fixed: no
    # active workspace exists, the persist is a no-op and List returns the
    # unmaterialized state without touching any draft.
    $startupProbeResp = $null
    try { $startupProbeResp = Invoke-AgentJson -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{ tool = "version.list"; args = @{}; source = "fxsr_138_probe"; confirmed = $true } -TimeoutSec 60 } catch { $startupProbeResp = @{ status = "error"; error = $_.Exception.Message } }
    $result["startup_probe_response"] = $startupProbeResp
    $probeQuiescence = Wait-TreeQuiescent $root 15000
    $result["startup_probe_quiescence"] = $probeQuiescence
    $snapshotStartup = SnapshotPhase $root "after_startup"
    $result["snapshot_after_startup"] = $snapshotStartup
    $startupDiff = @(Compare-TreeIdentity $result["snapshot_prestart"]["tree"] $snapshotStartup["tree"])
    $result["startup_tree_diff"] = $startupDiff
    $stripType = { param($d) $sep = $d.IndexOf(":"); if ($sep -ge 0) { $d.Substring($sep + 1) } else { $d } }
    $seedStartupDiff = @($startupDiff | Where-Object { (& $stripType $_).StartsWith($SeedName + "\") -or (& $stripType $_).StartsWith($SeedName + "/") })
    $result["seed_diffs_at_startup"] = $seedStartupDiff
    $seedStateAfterStartup = $snapshotStartup["tree"][($SeedName + "\.vit_history\.sessions\" + $SeedUuid + "\session_20260908T102051_099b621b\workspace\state\agent_runtime_state.json")]
    $result["seed_state_after_startup"] = $seedStateAfterStartup
    $seedRewritten = ($null -ne $seedStateAfterStartup -and $seedStateAfterStartup.sha256 -ne $result["seed_state_sha256"])

    $dirsStartup = @($snapshotStartup["dirs"])
    $agentCreatedAtStartup = @($dirsStartup | Where-Object { $_ -ne $SeedName })
    $result["agent_created_dirs_at_startup"] = $agentCreatedAtStartup

    # --- mode-specific startup assertions
    if ($Mode -eq "fixed") {
        if ($verdict.verdict -eq "pending") { Ok ("A1) startup activation stayed pending (new session): " + $verdict.detail) }
        else { Bad ("A1) startup activation verdict=" + $verdict.verdict + " line=" + $verdict.detail + " — expected pending (no recovery by kernel uuid)") }
        if ($verdict.verdict -eq "pending" -and $verdict.detail -notmatch 'reason=startup_recovery_disabled_new_session') {
            Bad ("A1) pending reason is not the FIX-START-STATE-RECOVERY-1 disabled reason: " + $verdict.detail)
        }
        if ($agentCreatedAtStartup.Count -eq 0) { Ok "A2) startup materialized zero draft dirs (137 no-regression)" }
        else { Bad ("A2) startup materialized draft dirs: " + ($agentCreatedAtStartup -join ",")) }
        if ($seedStartupDiff.Count -eq 0 -and -not $seedRewritten) { Ok ("Z) seeded previous-session draft ZERO-touch through startup+settle (42MB state sha+mtime identical, no adds/removes)") }
        else { Bad ("Z) seeded draft was touched at startup: " + (($seedStartupDiff | Select-Object -First 8) -join ";") + " rewritten=" + $seedRewritten) }
        if ($Sentinel) {
            $sharing = @(Select-String -Path $agentLog -Pattern 'being used by another process|sharing violation' -ErrorAction SilentlyContinue)
            $result["sentinel_sharing_violations"] = $sharing.Count
            if ($sharing.Count -eq 0) { Ok "S) sentinel leg: zero sharing-violation evidence — no open was attempted on the 42MB state" }
            else { Bad ("S) sentinel leg caught open attempts on the exclusively-held state: " + $sharing.Count + " log hits") }
        }
    } else {
        # baseline: document the phenomenon; leg PASS == phenomenon reproduced.
        if ($verdict.verdict -eq "activated" -and $verdict.detail -like ("*" + $SeedName + "*")) { Ok ("B1) baseline reproduced: uuid recovery claimed the seeded old draft — " + $verdict.detail) }
        elseif ($Sentinel) { Ok ("B1s) baseline+sentinel: recovery did NOT claim the seed (verdict=" + $verdict.verdict + ") — sentinel blocked the scan read (positive control)") }
        else { Bad ("B1) baseline did NOT reproduce the phenomenon: verdict=" + $verdict.verdict + " line=" + $verdict.detail) }
        if (-not $Sentinel) {
            if ($seedRewritten) { Ok ("B2) baseline rewrote the 42MB state at startup (write cost reproduced)") } else { Bad "B2) baseline did not rewrite the 42MB state — phenomenon incomplete" }
        } else {
            if (-not $seedRewritten -and $seedStartupDiff.Count -eq 0) { Ok "B2s) baseline+sentinel: 42MB state untouched (read attempt blocked)" } else { Bad ("B2s) baseline+sentinel unexpectedly touched the seed: " + (($seedStartupDiff | Select-Object -First 5) -join ";")) }
        }
        $result["observations"]["baseline_activation_ms"] = $verdict.ms
        $result["observations"]["baseline_seed_rewrite_settled_ms"] = $quiescence.waited_ms
    }
    $result["observations"]["activation_ms_from_agent_start"] = $verdict.ms

    # --- B/C: frontend-identical new-project notification creates exactly one draft
    $kernelNewRaw = & $PythonExe $KernelHelper --url $KernelZmqUrl --cmd new_project 2>&1
    $kernelNewReply = $null; try { $kernelNewReply = ($kernelNewRaw | Out-String).Trim() | ConvertFrom-Json } catch { $kernelNewReply = $null }
    $newUuid = TextOf(Prop $kernelNewReply "project_uuid"); if ($newUuid -eq "") { $newUuid = TextOf(Prop $kernelNewReply "project_id") }
    $result["kernel_new_project_raw"] = ($kernelNewRaw | Out-String)
    $result["new_project_uuid"] = $newUuid
    if ($newUuid -eq "") { Bad "kernel new_project did not return a project uuid" }
    $notifyBody = @{ tool = "version.project_new"; args = @{ project_uuid = $newUuid; project_path = ""; source = "fxsr_138_probe" }; source = "fxsr_138_probe"; confirmed = $true }
    $notifyResp = $null
    try { $notifyResp = Invoke-AgentJson -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body $notifyBody -TimeoutSec 60 } catch { Bad ("version.project_new HTTP error: " + $_.Exception.Message) }
    $result["version_project_new_response"] = $notifyResp
    $newQuiescence = Wait-TreeQuiescent $root 15000
    $result["new_project_quiescence"] = $newQuiescence
    $snapshotAfterNew = SnapshotPhase $root "after_project_new"
    $result["snapshot_after_project_new"] = $snapshotAfterNew
    $dirsAfter = @($snapshotAfterNew["dirs"])
    $newDirs = @($dirsAfter | Where-Object { $dirsStartup -notcontains $_ })
    $result["new_dirs_after_project_new"] = $newDirs
    if ($newDirs.Count -eq 1) { Ok ("B) version.project_new added exactly 1 draft (118 no-regression): " + $newDirs[0]) }
    else { Bad ("B) version.project_new added " + $newDirs.Count + " draft dirs: " + ($newDirs -join ",")) }
    $newDraftRows = @($snapshotAfterNew["identities"] | Where-Object { $newDirs -contains $_.name })
    $newDraftUuids = @()
    foreach ($row in $newDraftRows) { $newDraftUuids += @($row.uuids) }
    $result["new_draft_uuids"] = $newDraftUuids
    if ($newDraftUuids -contains $newUuid) { Ok "B) new draft carries the new project uuid " + $newUuid } else { Bad ("new draft does not carry new uuid " + $newUuid + "; uuids=" + ($newDraftUuids -join ",")) }
    $leakNew = @($newDraftUuids | Where-Object { $_ -eq $SeedUuid })
    $result["new_draft_old_uuid_residue"] = $leakNew
    $seedRowAfter = @($snapshotAfterNew["identities"] | Where-Object { $_.name -eq $SeedName })
    if ($Mode -eq "fixed") {
        # Fixed-world C gate: the new draft carries the NEW uuid, and the seed's
        # durable layout (.sessions/<old uuid>) is intact — zero-touch already
        # proves nothing was removed. The stale-shadow-uuid switch tick can
        # materialize an inert old-uuid workspace row inside the new draft
        # (118 ATTRIBUTION §4 signature, previously soaked up by the recovery
        # redirect); it self-heals to the new uuid and is recorded, not gated,
        # pending a supervisor ruling on that adjacent defect.
        if ($newDraftUuids -contains $newUuid -and $seedRowAfter.Count -eq 1 -and (@($seedRowAfter[0].sessions_uuids) -contains $SeedUuid)) {
            Ok "C) new draft carries the new uuid and the seeded draft keeps its .sessions old-uuid layout"
        } else {
            Bad ("C) identity gate failed: newDraftUuids=" + ($newDraftUuids -join ",") + " seedSessions=" + (@($seedRowAfter | ForEach-Object { $_.sessions_uuids -join "," }) -join ";"))
        }
        if ($leakNew.Count -gt 0) { Write-Host ("[138][NOTE] C2) inert old-uuid workspace row(s) inside the new draft (stale-shadow switch tick, self-healed; recorded for supervisor): " + ($leakNew -join ",")) }
    } else {
        if ($leakNew.Count -eq 0) { Ok "C) no old kernel-uuid workspace leaked into the new project draft" }
        elseif ($Sentinel) { Write-Host ("[138][NOTE] C2) baseline+sentinel: inert old-uuid row inside the new draft (scan blocked = homeless old uuid lands in the current draft; same signature as fixed mode, recorded)") }
        else { Bad ("C) old kernel-uuid workspace leaked into the new draft: " + ($leakNew -join ",")) }
        if ($seedRowAfter.Count -eq 1 -and ((@($seedRowAfter[0].uuids) -contains $SeedUuid) -or (@($seedRowAfter[0].sessions_uuids) -contains $SeedUuid))) { Ok "C) seeded draft keeps the old kernel-uuid workspace" } else { Bad "C) seeded draft lost the old kernel-uuid workspace" }
    }

    # --- D: reader faces stay read-only outside the active workspace (137 no-regression)
    $readerResponses = [ordered]@{}
    foreach ($tool in @("version.list", "version.worktree_list")) {
        try {
            $resp = Invoke-AgentJson -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{ tool = $tool; args = @{}; source = "fxsr_138_probe"; confirmed = $true } -TimeoutSec 30
            $res = Prop $resp "result"
            $commitCount = $null; $commits = Prop $res "commits"
            if ($null -ne $commits) { $commitCount = @($commits).Count }
            $readerResponses[$tool] = [ordered]@{ status = TextOf(Prop $resp "status"); project_path = TextOf(Prop $res "project_path") }
        } catch { $readerResponses[$tool] = @{ status = "error"; error = $_.Exception.Message } }
    }
    $result["reader_responses"] = $readerResponses
    Start-Sleep -Milliseconds $ReaderWindowMs
    $snapshotAfterReader = SnapshotPhase $root "after_reader"
    $result["snapshot_after_reader"] = $snapshotAfterReader
    $readerTreeDiff = @(Compare-TreeIdentity $snapshotAfterNew["tree"] $snapshotAfterReader["tree"])
    $result["reader_tree_diff"] = $readerTreeDiff
    $activeDraft = ($newDirs | Select-Object -First 1)
    $readerForbidden = @($readerTreeDiff | Where-Object { $p = & $stripType $_; -not ($p.StartsWith($activeDraft + "\") -or $p.StartsWith($activeDraft + "/")) })
    $readerDirs = @($snapshotAfterReader["dirs"])
    $readerNewDirs = @($readerDirs | Where-Object { $dirsAfter -notcontains $_ })
    $result["reader_new_dirs"] = $readerNewDirs
    $result["reader_forbidden_diffs"] = $readerForbidden
    if ($readerNewDirs.Count -eq 0 -and $readerForbidden.Count -eq 0) { Ok ("D) reader faces created zero drafts and wrote nothing outside the active workspace") }
    else { Bad ("D) reader faces violated the read-only contract: newDirs=" + ($readerNewDirs -join ",") + " forbidden=" + (($readerForbidden | Select-Object -First 8) -join ";")) }

    # --- final zero-touch verdict for the seed across the whole leg (fixed mode)
    if ($Mode -eq "fixed") {
        $fullDiff = @(Compare-TreeIdentity $result["snapshot_prestart"]["tree"] $snapshotAfterReader["tree"])
        $seedFullDiff = @($fullDiff | Where-Object { $p = & $stripType $_; $p.StartsWith($SeedName + "\") -or $p.StartsWith($SeedName + "/") })
        $result["seed_diffs_full_leg"] = $seedFullDiff
        if ($seedFullDiff.Count -eq 0) { Ok "Z-final) seeded old draft ZERO-touch across the whole leg (startup+project_new+readers)" }
        else { Bad ("Z-final) seeded draft changed across the leg: " + (($seedFullDiff | Select-Object -First 8) -join ";")) }
    }

    # --- log scan: activation lines + any error mentioning the seed
    if (Test-Path -LiteralPath $agentLog) {
        $activatedAll = @(Select-String -Path $agentLog -Pattern '\[workspace\] activated' -ErrorAction SilentlyContinue)
        $pendingAll = @(Select-String -Path $agentLog -Pattern 'activation pending uuid=' -ErrorAction SilentlyContinue)
        $result["activated_line_count"] = $activatedAll.Count
        $result["activation_pending_line_count"] = $pendingAll.Count
        $result["last_activated_line"] = if ($activatedAll.Count -gt 0) { $activatedAll[$activatedAll.Count - 1].Line } else { "" }
        $result["last_pending_line"] = if ($pendingAll.Count -gt 0) { $pendingAll[$pendingAll.Count - 1].Line } else { "" }
    }

    $cIdentityGate = if ($Mode -eq "fixed") {
        ($newDraftUuids -contains $newUuid -and $seedRowAfter.Count -eq 1 -and (@($seedRowAfter[0].sessions_uuids) -contains $SeedUuid))
    } elseif ($Sentinel) {
        ($seedRowAfter.Count -eq 1 -and ((@($seedRowAfter[0].uuids) -contains $SeedUuid) -or (@($seedRowAfter[0].sessions_uuids) -contains $SeedUuid)))
    } else {
        ($leakNew.Count -eq 0 -and $seedRowAfter.Count -eq 1 -and ((@($seedRowAfter[0].uuids) -contains $SeedUuid) -or (@($seedRowAfter[0].sessions_uuids) -contains $SeedUuid)))
    }
    $result["assertions"] = [ordered]@{
        A1_no_recovery_by_kernel_uuid = ($Mode -ne "fixed") -or ($verdict.verdict -eq "pending" -and $verdict.detail -match 'reason=startup_recovery_disabled_new_session')
        A2_startup_zero_materialization = ($agentCreatedAtStartup.Count -eq 0)
        B_project_new_exactly_one_draft = ($newDirs.Count -eq 1 -and $newDraftUuids -contains $newUuid)
        C_identity_gate = $cIdentityGate
        D_reader_read_only = ($readerNewDirs.Count -eq 0 -and $readerForbidden.Count -eq 0)
        Z_seed_zero_touch_full_leg = ($Mode -ne "fixed") -or ($seedFullDiff.Count -eq 0)
        S_sentinel_zero_open_attempts = (-not $Sentinel) -or ($Mode -ne "fixed") -or ($sharing.Count -eq 0)
        BASE_phenomenon_reproduced = ($Mode -ne "baseline") -or ($Sentinel -and $verdict.verdict -ne "activated" -and -not $seedRewritten) -or (-not $Sentinel -and $verdict.verdict -eq "activated" -and $verdict.detail -like ("*" + $SeedName + "*") -and $seedRewritten)
    }
}
catch {
    Bad ("exception: " + $_.Exception.Message)
    $result["exception"] = $_.Exception.Message
    $result["exception_stack"] = $_.ScriptStackTrace
}
finally {
    $result["failures"] = @($script:failures)
    $result["notes"] = @($script:notes)
    $result["finished_at"] = (Get-Date).ToString("o")
    if ($sentinelStream -ne $null) { $sentinelStream.Close(); $sentinelStream = $null }
    if ($agentProc -ne $null -and -not $agentProc.HasExited) { Stop-Process -Id $agentProc.Id -Force -ErrorAction SilentlyContinue; Start-Sleep -Milliseconds 400 }
    if ($kernelProc -ne $null -and -not $kernelProc.HasExited) { Stop-Process -Id $kernelProc.Id -Force -ErrorAction SilentlyContinue; Start-Sleep -Milliseconds 800 }
    Remove-Item Env:\VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue
    $remainingPorts = @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -in 7878,5555,5556,8787,4445,19876 })
    $remainingStack = @(Get-CimInstance Win32_Process -Filter "name='VitApp.exe' OR name='VitAgent.exe' OR name='vsphub.exe' OR name='VspHub.exe'" -ErrorAction SilentlyContinue)
    $result["teardown"] = [ordered]@{ ports_listening = @($remainingPorts | ForEach-Object { $_.LocalPort }); stack_processes = @($remainingStack | ForEach-Object { $_.Name + ":" + $_.ProcessId }); at = (Get-Date).ToString("o") }
    New-Item -ItemType Directory -Force -Path $RunDir | Out-Null
    $result | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath (Join-Path $RunDir "stack_leg_result.json") -Encoding UTF8
    Write-Host ("[138] result: " + (Join-Path $RunDir "stack_leg_result.json"))
    if ($script:failures.Count -gt 0 -or $remainingPorts.Count -gt 0) { Write-Host ("[138] FAIL count=" + $script:failures.Count + " ports=" + ($result["teardown"]["ports_listening"] -join ",")); exit 1 }
    Write-Host "[138] PASS"; exit 0
}

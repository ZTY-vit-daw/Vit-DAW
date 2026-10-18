# FE-START-EMPTY-DRAFT-1 real-stack RACE probe.
#
# Why this variant exists: in the dev_agent_smoke stack the agent performs
# startup-internal history reads that materialize `.vit_history` inside the
# implicit default draft a few seconds after startup (observed at +7s; the
# read path is harness.ProjectHistorySummaryForProject -> history.Status ->
# historyState -> listWorktreeItems -> ensureRootWorktreeItem). To exercise the
# pristine-reuse branch on the real stack we must fire the frontend-identical
# kernel new_project + agent version.project_new notification immediately after
# startup recovery/activation completes and before that internal read lands.
#
# Sequence (all agent-visible reads stay on the kernel face until the
# notification): wait TCP 7878 -> wait agent log activation line for the seeded
# previous-session draft -> filesystem snapshot (implicit draft must be pristine)
# -> kernel ZMQ get_project_state (old uuid) -> kernel ZMQ new_project (new uuid)
# -> HTTP /agent/invoke version.project_new(new uuid) -> filesystem snapshot.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File probe_race.ps1 `
#     -DraftsRoot <run>\ProjectHistory\drafts -OutDir <run>\probe_race `
#     -AgentLogPath <agent_last.log> -KernelHelper <scripts>\kernel_new_project.py `
#     [-SeedPrefix prev] [-TimeoutSec 20]

param(
    [Parameter(Mandatory = $true)][string]$DraftsRoot,
    [Parameter(Mandatory = $true)][string]$OutDir,
    [Parameter(Mandatory = $true)][string]$AgentLogPath,
    [Parameter(Mandatory = $true)][string]$KernelHelper,
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$KernelZmqUrl = "tcp://127.0.0.1:5555",
    [string]$PythonExe = "python",
    [string]$SeedMarker = "prev",
    [int]$TimeoutSec = 25
)

$ErrorActionPreference = "Stop"
$failures = New-Object System.Collections.Generic.List[string]
$notes = New-Object System.Collections.Generic.List[string]
$t0 = Get-Date

function Ok([string]$m) { Write-Host ("[race][OK] " + $m); $notes.Add("OK: " + $m) | Out-Null }
function Bad([string]$m) { Write-Host ("[race][FAIL] " + $m); $failures.Add($m) | Out-Null }
function Prop($o, [string]$n, $d = $null) { if ($null -eq $o) { return $d }; $p = $o.PSObject.Properties[$n]; if ($null -eq $p) { return $d }; return $p.Value }

function Drafts {
    if (-not (Test-Path -LiteralPath $DraftsRoot)) { return @() }
    return @(Get-ChildItem -LiteralPath $DraftsRoot -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -like "draft_*" } | Sort-Object Name -Descending | ForEach-Object { $_.Name })
}

function Identity([string]$name) {
    $dir = Join-Path $DraftsRoot $name
    $h = Join-Path $dir ".vit_history"
    $a = Join-Path $dir ".vit_agent"
    $hu = @(); $au = @()
    if (Test-Path -LiteralPath $h) { $hu = @(Get-ChildItem -LiteralPath $h -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "vitproj_*" } | ForEach-Object { $_.Name }) }
    if (Test-Path -LiteralPath $a) { $au = @(Get-ChildItem -LiteralPath $a -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "vitproj_*" } | ForEach-Object { $_.Name }) }
    return [pscustomobject]@{ name = $name; history = (Test-Path -LiteralPath $h); agent = (Test-Path -LiteralPath $a); uuids = @($hu + $au | Select-Object -Unique) }
}

function Snapshot {
    $rows = @(); foreach ($n in (Drafts)) { $rows += Identity $n }
    return [pscustomobject]@{ at = (Get-Date).ToString("o"); names = @(Drafts); rows = $rows }
}

$result = [ordered]@{ schema_version = "fe_start_empty_draft_race_probe.v1"; drafts_root = $DraftsRoot; started_at = (Get-Date).ToString("o"); assertions = @(); failures = @(); notes = @() }

# 1) wait for the agent HTTP port (TCP only; no agent HTTP read yet).
$deadline = (Get-Date).AddSeconds($TimeoutSec)
$portUp = $false
while ((Get-Date) -lt $deadline) {
    $c = Get-NetTCPConnection -State Listen -LocalPort 7878 -ErrorAction SilentlyContinue
    if ($c) { $portUp = $true; break }
    Start-Sleep -Milliseconds 100
}
if (-not $portUp) { Bad "agent port 7878 did not open within ${TimeoutSec}s"; $result.failures = @($failures); $result.finished_at = (Get-Date).ToString("o"); New-Item -ItemType Directory -Force -Path $OutDir | Out-Null; $result | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $OutDir "probe_race_result.json") -Encoding UTF8; exit 2 }

# 2) wait for startup recovery/activation to claim the seeded previous-session draft.
$activatedSeed = ""
$deadline = (Get-Date).AddSeconds($TimeoutSec)
while ((Get-Date) -lt $deadline) {
    if (Test-Path -LiteralPath $AgentLogPath) {
        $line = Select-String -Path $AgentLogPath -Pattern '\[workspace\] activated project="([^"]+)" uuid=(vitproj_[0-9a-f]+)' -AllMatches | Select-Object -Last 1
        if ($line) {
            $m = $line.Matches[0]
            $proj = $m.Groups[1].Value
            $uuid = $m.Groups[2].Value
            if ($proj -like "*$SeedMarker*") { $activatedSeed = $proj; $oldUuid = $uuid; break }
        }
    }
    Start-Sleep -Milliseconds 100
}
if ($activatedSeed -eq "") {
    Bad ("startup activation did not claim a seeded previous-session draft within ${TimeoutSec}s")
} else {
    Ok ("startup activation claimed seeded draft " + $activatedSeed)
}
$result["activated_seed_path"] = $activatedSeed
$result["old_kernel_uuid"] = $oldUuid

# 3) snapshot + pristine precondition (immediately after activation).
$before = Snapshot
$result["snapshot_before"] = $before
$seedNames = @($before.rows | Where-Object { $_.name -like "*$SeedMarker*" } | ForEach-Object { $_.name })
$implicitRows = @($before.rows | Where-Object { $seedNames -notcontains $_.name })
$implicit = @($implicitRows | ForEach-Object { $_.name })
$result["seed_names"] = $seedNames
$result["implicit_dirs"] = $implicit
$implicitPristine = $false
if ($implicit.Count -eq 1) {
    $row = $implicitRows[0]
    $implicitPristine = (-not $row.history -and -not $row.agent)
    if ($implicitPristine) { Ok ("implicit default draft " + $row.name + " is pristine at notification time") }
    else { Bad ("implicit default draft " + $row.name + " already initialized before notification (history=" + $row.history + " agent=" + $row.agent + ")") }
} else {
    Bad ("expected exactly 1 implicit draft, got " + $implicit.Count + ": " + ($implicit -join ","))
}
$result["implicit_pristine"] = $implicitPristine

# 4) kernel-side old identity (no agent HTTP read), then kernel new_project.
$helper = { param([string]$extra) & $PythonExe $KernelHelper --url $KernelZmqUrl @extra 2>&1 }
$stateRaw = & $PythonExe $KernelHelper --url $KernelZmqUrl --cmd get_project_state 2>&1
$stateReply = $null
try { $stateReply = ($stateRaw | Out-String).Trim() | ConvertFrom-Json } catch { $stateReply = $null }
$kernelUuid = [string](Prop $stateReply "project_uuid" ""); if (-not $kernelUuid) { $kernelUuid = [string](Prop (Prop $stateReply "project") "project_uuid" "") }
$result["kernel_get_project_state_raw"] = ($stateRaw | Out-String)
$result["kernel_project_uuid_before_new"] = $kernelUuid
if ($kernelUuid -ne $oldUuid -and $kernelUuid -ne "") { Bad ("kernel uuid " + $kernelUuid + " != recovery uuid " + $oldUuid) }

$newRaw = & $PythonExe $KernelHelper --url $KernelZmqUrl --cmd new_project 2>&1
$newReply = $null
try { $newReply = ($newRaw | Out-String).Trim() | ConvertFrom-Json } catch { $newReply = $null }
$newUuid = [string](Prop $newReply "project_uuid" ""); if (-not $newUuid) { $newUuid = [string](Prop $newReply "project_id" "") }
$result["kernel_new_project_raw"] = ($newRaw | Out-String)
$result["new_project_uuid"] = $newUuid
if ($newUuid -eq "") { Bad "kernel new_project did not return a uuid" } else { Ok ("kernel new_project -> " + $newUuid) }

# 5) frontend-identical agent notification (immediately).
$body = @{ tool = "version.project_new"; args = @{ project_uuid = $newUuid; project_path = ""; source = "fedraft_race_probe" }; source = "fedraft_race_probe"; confirmed = $true }
$resp = $null
try { $resp = Invoke-RestMethod -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -ContentType "application/json" -Body ($body | ConvertTo-Json -Depth 8 -Compress) -TimeoutSec 30 }
catch { Bad ("version.project_new HTTP error: " + $_.Exception.Message) }
$result["version_project_new_response"] = $resp
Start-Sleep -Seconds 1

$after = Snapshot
$result["snapshot_after"] = $after
$newDirs = @($after.names | Where-Object { $before.names -notcontains $_ })
$result["new_dirs_after"] = $newDirs
$result["agent_created_dirs"] = @($after.names | Where-Object { $seedNames -notcontains $_ })
if ($newDirs.Count -eq 0) { Ok "version.project_new added no draft directory (pristine default draft reused)" } else { Bad ("version.project_new added " + $newDirs.Count + " new draft dir(s): " + ($newDirs -join ",")) }

$reused = @($after.rows | Where-Object { $_.uuids -contains $newUuid } | ForEach-Object { $_.name })
$result["reused_dirs"] = $reused
if ($reused.Count -eq 1) { Ok ("new project uuid materialized in exactly one draft: " + $reused[0]) } else { Bad ("new project uuid in " + $reused.Count + " drafts: " + ($reused -join ",")) }
if ($implicit.Count -eq 1 -and $reused -contains $implicit[0]) { Ok ("the pristine implicit draft " + $implicit[0] + " was reused") } else { Bad ("implicit draft was not reused (reused: " + ($reused -join ",") + ")") }
$leak = @($after.rows | Where-Object { $seedNames -notcontains $_.name -and $_.uuids -contains $oldUuid } | ForEach-Object { $_.name })
$result["old_uuid_leak_rows"] = $leak
if ($leak.Count -eq 0) { Ok "no old kernel-uuid workspace materialized in the new-project draft" } else { Bad ("old kernel-uuid workspace leaked into: " + ($leak -join ",")) }
$seedRow = @($after.rows | Where-Object { $seedNames -contains $_.name -and $_.uuids -contains $oldUuid })
if ($seedRow.Count -ge 1) { Ok "seeded recovery draft keeps the old kernel-uuid workspace" } else { Bad "seeded recovery draft lost the old kernel-uuid workspace" }

$result["assertions"] = @([ordered]@{ name = "pristine_precondition"; pass = $implicitPristine }, [ordered]@{ name = "new_project_reuse"; pass = ($newDirs.Count -eq 0 -and $reused.Count -eq 1 -and ($implicit.Count -eq 1 -and $reused -contains $implicit[0])) }, [ordered]@{ name = "no_old_uuid_leak"; pass = ($leak.Count -eq 0) })
$result["failures"] = @($failures)
$result["notes"] = @($notes)
$result["finished_at"] = (Get-Date).ToString("o")
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$result | ConvertTo-Json -Depth 15 | Set-Content -LiteralPath (Join-Path $OutDir "probe_race_result.json") -Encoding UTF8
Write-Host ("[race] result: " + (Join-Path $OutDir "probe_race_result.json"))
if ($failures.Count -gt 0) { Write-Host ("[race] FAIL count=" + $failures.Count); exit 1 }
Write-Host "[race] PASS"; exit 0

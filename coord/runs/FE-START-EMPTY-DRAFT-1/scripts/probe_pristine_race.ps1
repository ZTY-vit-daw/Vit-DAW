# FE-START-EMPTY-DRAFT-1 real-stack pristine-reuse probe (deterministic variant).
#
# The stock agent has an autonomous startup reader that materializes
# `.vit_history` inside the implicit default draft ~1.5-2s after startup
# activation (observed: history list/tool invokes right after activation; this is
# the read-path side effect tracked as FIX-START-DRAFT-SIDE-EFFECT-1). The
# minimal fix under test is scoped to the ProjectNew decision: "if the cached
# default draft is still pristine when the new-project notification arrives,
# reuse it instead of creating a second directory".
#
# This probe therefore cancels ONLY that startup reader's materialization (it
# removes the freshly created .vit_history/.vit_agent scaffolding from the
# implicit draft, recording exactly what was removed), then immediately drives
# the frontend-identical sequence: kernel ZMQ new_project -> agent
# /agent/invoke version.project_new. If the fix works, the implicit draft is
# reused (no second directory, only the new uuid's workspace materialized) and
# the seeded previous-session draft keeps the old kernel uuid.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File probe_pristine_race.ps1 `
#     -DraftsRoot <run>\ProjectHistory\drafts -OutDir <run>\probe_final `
#     -AgentLogPath <agent_last.log> -KernelHelper <scripts>\kernel_new_project.py `
#     -SeedMarker prev -TimeoutSec 25

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

function Ok([string]$m) { Write-Host ("[final][OK] " + $m); $notes.Add("OK: " + $m) | Out-Null }
function Bad([string]$m) { Write-Host ("[final][FAIL] " + $m); $failures.Add($m) | Out-Null }
function Prop($o, [string]$n, $d = $null) { if ($null -eq $o) { return $d }; $p = $o.PSObject.Properties[$n]; if ($null -eq $p) { return $d }; return $p.Value }
function DraftNames { @(Get-ChildItem -LiteralPath $DraftsRoot -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "draft_*" } | Sort-Object Name | ForEach-Object { $_.Name }) }
function Identity([string]$name) {
    $dir = Join-Path $DraftsRoot $name
    $h = Join-Path $dir ".vit_history"; $a = Join-Path $dir ".vit_agent"
    $hu = @(); $au = @()
    if (Test-Path -LiteralPath $h) { $hu = @(Get-ChildItem -LiteralPath $h -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "vitproj_*" } | ForEach-Object { $_.Name }) }
    if (Test-Path -LiteralPath $a) { $au = @(Get-ChildItem -LiteralPath $a -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -like "vitproj_*" } | ForEach-Object { $_.Name }) }
    [pscustomobject]@{ name = $name; history = (Test-Path -LiteralPath $h); agent = (Test-Path -LiteralPath $a); uuids = @($hu + $au | Select-Object -Unique) }
}
function Snapshot { $rows = @(); foreach ($n in (DraftNames)) { $rows += Identity $n }; [pscustomobject]@{ at = (Get-Date).ToString("o"); names = @(DraftNames); rows = $rows } }

$result = [ordered]@{ schema_version = "fe_start_empty_draft_pristine_race.v1"; drafts_root = $DraftsRoot; started_at = (Get-Date).ToString("o"); failures = @(); notes = @(); canceled_reader_artifacts = @() }

# 1) wait for agent port (TCP only).
$deadline = (Get-Date).AddSeconds($TimeoutSec); $portUp = $false
while ((Get-Date) -lt $deadline) { if (Get-NetTCPConnection -State Listen -LocalPort 7878 -ErrorAction SilentlyContinue) { $portUp = $true; break }; Start-Sleep -Milliseconds 80 }
if (-not $portUp) { Bad "agent port 7878 did not open"; $result.failures = @($failures); $result.finished_at = (Get-Date).ToString("o"); New-Item -ItemType Directory -Force -Path $OutDir | Out-Null; $result | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $OutDir "probe_final_result.json") -Encoding UTF8; exit 2 }

# 2) wait for activation to claim the seeded previous-session draft.
$activated = ""; $oldUuid = ""
$deadline = (Get-Date).AddSeconds($TimeoutSec)
while ((Get-Date) -lt $deadline) {
    if (Test-Path -LiteralPath $AgentLogPath) {
        $hit = Select-String -Path $AgentLogPath -Pattern '\[workspace\] activated project="([^"]+)" uuid=(vitproj_[0-9a-f]+)' -AllMatches | Select-Object -Last 1
        if ($hit) { $m = $hit.Matches[0]; $proj = $m.Groups[1].Value; if ($proj -like "*$SeedMarker*") { $activated = $proj; $oldUuid = $m.Groups[2].Value; break } }
    }
    Start-Sleep -Milliseconds 80
}
$result["activated_seed_path"] = $activated; $result["old_kernel_uuid"] = $oldUuid
if ($activated -eq "") { Bad "startup activation did not claim a seeded draft" } else { Ok ("activation claimed seeded draft (old uuid " + $oldUuid + ")") }

# 3) cancel the autonomous startup reader's materialization inside the implicit draft.
$beforeCancel = Snapshot
$result["snapshot_before_cancel"] = $beforeCancel
$seedNames = @($beforeCancel.names | Where-Object { $_ -like "*$SeedMarker*" })
$implicit = @($beforeCancel.names | Where-Object { $seedNames -notcontains $_ })
$result["seed_names"] = $seedNames; $result["implicit_dirs"] = $implicit
if ($implicit.Count -ne 1) { Bad ("expected exactly 1 implicit draft, got " + $implicit.Count + ": " + ($implicit -join ",")) }
$implicitName = if ($implicit.Count -ge 1) { $implicit[0] } else { "" }
if ($implicitName -ne "") {
    $implicitDir = Join-Path $DraftsRoot $implicitName
    foreach ($sub in @(".vit_history", ".vit_agent")) {
        $p = Join-Path $implicitDir $sub
        if (Test-Path -LiteralPath $p) {
            $files = @(Get-ChildItem -LiteralPath $p -Recurse -File -ErrorAction SilentlyContinue)
            $result["canceled_reader_artifacts"] += [pscustomobject]@{ path = $p; files = $files.Count; bytes = [long](($files | Measure-Object -Property Length -Sum).Sum) }
            Remove-Item -LiteralPath $p -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
    $remaining = @(Get-ChildItem -LiteralPath $implicitDir -Force -ErrorAction SilentlyContinue)
    $pristine = ($remaining.Count -eq 1 -and $remaining[0].Name -eq "Unsaved.vit")
    $result["implicit_after_cancel"] = [pscustomobject]@{ name = $implicitName; entries = @($remaining | ForEach-Object { $_.Name }); pristine = $pristine }
    if ($pristine) { Ok ("implicit draft " + $implicitName + " restored to pristine before notification (canceled reader artifacts recorded)") } else { Bad ("implicit draft " + $implicitName + " not pristine after cancel: " + (@($remaining | ForEach-Object { $_.Name }) -join ",")) }
}

# 4) kernel new_project -> agent version.project_new (frontend-identical order).
$newRaw = & $PythonExe $KernelHelper --url $KernelZmqUrl --cmd new_project 2>&1
$newReply = $null; try { $newReply = ($newRaw | Out-String).Trim() | ConvertFrom-Json } catch { $newReply = $null }
$newUuid = [string](Prop $newReply "project_uuid" ""); if (-not $newUuid) { $newUuid = [string](Prop $newReply "project_id" "") }
$result["kernel_new_project_raw"] = ($newRaw | Out-String); $result["new_project_uuid"] = $newUuid
if ($newUuid -eq "") { Bad "kernel new_project did not return a uuid" } else { Ok ("kernel new_project -> " + $newUuid) }
$body = @{ tool = "version.project_new"; args = @{ project_uuid = $newUuid; project_path = ""; source = "fedraft_pristine_race"; }; source = "fedraft_pristine_race"; confirmed = $true }
$resp = $null
try { $resp = Invoke-RestMethod -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -ContentType "application/json" -Body ($body | ConvertTo-Json -Depth 8 -Compress) -TimeoutSec 30 } catch { Bad ("version.project_new HTTP error: " + $_.Exception.Message) }
$result["version_project_new_response"] = $resp
$result["snapshot_immediately_after_new"] = Snapshot
Start-Sleep -Milliseconds 1500
$after = Snapshot
$result["snapshot_after"] = $after
$newDirs = @($after.names | Where-Object { $beforeCancel.names -notcontains $_ })
$result["new_dirs_after_new"] = $newDirs
if ($newDirs.Count -eq 0) { Ok "version.project_new added no draft directory (implicit draft reused)" } else { Bad ("version.project_new added draft(s): " + ($newDirs -join ",")) }
$reused = @($after.rows | Where-Object { $_.uuids -contains $newUuid } | ForEach-Object { $_.name })
$result["reused_dirs"] = $reused
if ($reused.Count -eq 1 -and $implicitName -ne "" -and $reused -contains $implicitName) { Ok ("implicit draft " + $implicitName + " reused as the new project draft (uuid " + $newUuid + ")") } else { Bad ("implicit draft not reused; new uuid in: " + ($reused -join ",")) }
$leak = @($after.rows | Where-Object { $seedNames -notcontains $_.name -and $_.uuids -contains $oldUuid } | ForEach-Object { $_.name })
$result["old_uuid_leak_rows"] = $leak
if ($leak.Count -eq 0) { Ok "no old kernel-uuid workspace materialized outside the recovery target" } else { Bad ("old kernel-uuid workspace leaked into: " + ($leak -join ",")) }
$reusedRow = @($after.rows | Where-Object { $_.name -eq $implicitName })
if ($reusedRow.Count -eq 1 -and ($reusedRow[0].uuids -notcontains $oldUuid)) { Ok "reused draft carries only the new project uuid" } else { Bad ("reused draft carries unexpected uuids: " + (@($reusedRow[0].uuids) -join ",")) }
$seedRow = @($after.rows | Where-Object { $seedNames -contains $_.name -and $_.uuids -contains $oldUuid })
if ($seedRow.Count -ge 1) { Ok "seeded recovery draft keeps the old kernel-uuid workspace" } else { Bad "seeded recovery draft lost the old kernel-uuid workspace" }

$result["assertions"] = @([ordered]@{ name = "pristine_precondition"; pass = ($implicitName -ne "" -and $result["implicit_after_cancel"].pristine) }, [ordered]@{ name = "new_project_reuse"; pass = ($newDirs.Count -eq 0 -and $reused.Count -eq 1 -and $reused -contains $implicitName) }, [ordered]@{ name = "no_old_uuid_leak"; pass = ($leak.Count -eq 0) })
$result["failures"] = @($failures); $result["notes"] = @($notes); $result["finished_at"] = (Get-Date).ToString("o")
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$result | ConvertTo-Json -Depth 15 | Set-Content -LiteralPath (Join-Path $OutDir "probe_final_result.json") -Encoding UTF8
Write-Host ("[final] result: " + (Join-Path $OutDir "probe_final_result.json"))
if ($failures.Count -gt 0) { Write-Host ("[final] FAIL count=" + $failures.Count); exit 1 }
Write-Host "[final] PASS"; exit 0

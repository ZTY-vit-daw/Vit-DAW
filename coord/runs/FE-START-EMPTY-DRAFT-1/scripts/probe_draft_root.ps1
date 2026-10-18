# FE-START-EMPTY-DRAFT-1 real-stack probe: startup draft-root lifecycle assertions.
# All draft activity is confined to -DraftsRoot (the VIT_HISTORY_DRAFT_ROOT the
# agent was started with); the user's real ProjectHistory is never touched.
#
# The isolated root must contain a "ProjectHistory\drafts" path segment so that
# history.IsDraftProjectPath() recognizes its drafts exactly like the user's real
# root (otherwise the startup recovery scan skips every candidate).
#
# Modes:
#   naive  : empty root; records the stock fresh-machine behavior.
#   seeded : user-equivalent. Pre-created "previous session" draft(s) exist in the
#            root for the kernel startup project uuid; startup recovery claims the
#            matching seed, leaving the implicit default draft pristine, and the
#            frontend-identical version.project_new notification must REUSE that
#            pristine draft (no second directory).
#
# Node add -KernelNewProjectFirst to first switch the live kernel to a new project
# over ZMQ (:5555) and then notify the agent with the kernel-issued uuid 鈥?exactly
# the Godot start-page sequence (kernel first, agent notification second).
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File probe_draft_root.ps1 `
#     -DraftsRoot <run>\ProjectHistory\drafts -OutDir <run> -Mode seeded `
#     -SeedNames draft_a_prev,draft_b_prev -KernelNewProjectFirst `
#     -KernelHelper <scripts>\kernel_new_project.py

param(
    [Parameter(Mandatory = $true)][string]$DraftsRoot,
    [Parameter(Mandatory = $true)][string]$OutDir,
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [ValidateSet("naive", "seeded")][string]$Mode = "naive",
    [string[]]$SeedNames = @(),
    [string]$KernelUuid = "",
    [switch]$KernelNewProjectFirst,
    [string]$KernelZmqUrl = "tcp://127.0.0.1:5555",
    [string]$KernelHelper = "",
    [string]$PythonExe = "python",
    [int]$ReadyTimeoutSec = 180
)

$ErrorActionPreference = "Stop"
$script:failures = New-Object System.Collections.Generic.List[string]
$script:notes = New-Object System.Collections.Generic.List[string]
$seedArgNames = @($SeedNames | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }); if ($seedArgNames.Count -eq 1 -and $seedArgNames[0].Contains(",")) { $seedArgNames = @($seedArgNames[0] -split ",") }; $seedSet = @($seedArgNames | ForEach-Object { $_.Trim() } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })

function Write-StepLine([string]$Message) { Write-Host ("[probe] " + $Message) }
function Write-ProbeOk([string]$Message) { Write-Host ("[probe][OK] " + $Message); $script:notes.Add("OK: " + $Message) | Out-Null }
function Write-ProbeFail([string]$Message) { Write-Host ("[probe][FAIL] " + $Message); $script:failures.Add($Message) | Out-Null }

function Invoke-AgentJson {
    param([string]$Method, [string]$Uri, [object]$Body, [int]$TimeoutSec = 30)
    $params = @{ Method = $Method; Uri = $Uri; TimeoutSec = $TimeoutSec }
    if ($null -ne $Body) {
        $params["ContentType"] = "application/json"
        $params["Body"] = ($Body | ConvertTo-Json -Depth 10 -Compress)
    }
    return Invoke-RestMethod @params
}

function Get-Prop {
    param([object]$Object, [string]$Name, $Default = $null)
    if ($null -eq $Object) { return $Default }
    $p = $Object.PSObject.Properties[$Name]
    if ($null -eq $p) { return $Default }
    return $p.Value
}

function Get-DraftDirs {
    if (-not (Test-Path -LiteralPath $DraftsRoot)) { return @() }
    return @(Get-ChildItem -LiteralPath $DraftsRoot -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -like "draft_*" } | Sort-Object Name | ForEach-Object { $_.Name })
}

function Get-DraftIdentity {
    param([string]$Name)
    $dir = Join-Path $DraftsRoot $Name
    $historyUuids = @()
    $agentUuids = @()
    $historyRoot = Join-Path $dir ".vit_history"
    if (Test-Path -LiteralPath $historyRoot) {
        $historyUuids = @(Get-ChildItem -LiteralPath $historyRoot -Directory -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -like "vitproj_*" } | ForEach-Object { $_.Name })
    }
    $agentRoot = Join-Path $dir ".vit_agent"
    if (Test-Path -LiteralPath $agentRoot) {
        $agentUuids = @(Get-ChildItem -LiteralPath $agentRoot -Directory -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -like "vitproj_*" } | ForEach-Object { $_.Name })
    }
    return [ordered]@{
        name          = $Name
        has_history   = (Test-Path -LiteralPath $historyRoot)
        has_agent     = (Test-Path -LiteralPath $agentRoot)
        history_uuids = $historyUuids
        agent_uuids   = $agentUuids
        all_uuids     = @($historyUuids + $agentUuids | Select-Object -Unique)
    }
}

function Get-Snapshot {
    $names = Get-DraftDirs
    $rows = @()
    foreach ($n in $names) { $rows += [pscustomobject](Get-DraftIdentity -Name $n) }
    return [ordered]@{ captured_at = (Get-Date).ToString("o"); dirs = $names; identities = $rows }
}

# 1) wait for agent HTTP readiness.
$deadline = (Get-Date).AddSeconds($ReadyTimeoutSec)
$ready = $false
while ((Get-Date) -lt $deadline) {
    try {
        $tools = Invoke-AgentJson -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/tools") -TimeoutSec 5
        if ($null -ne $tools) { $ready = $true; break }
    }
    catch { Start-Sleep -Milliseconds 500 }
}
if (-not $ready) { Write-ProbeFail "agent /agent/tools not ready within $ReadyTimeoutSec sec" }

$result = [ordered]@{
    schema_version = "fe_start_empty_draft_probe.v2"
    mode           = $Mode
    drafts_root    = $DraftsRoot
    agent_http     = $AgentHttp
    started_at     = (Get-Date).ToString("o")
    seed_names     = $seedSet
    assertions     = [ordered]@{}
    failures       = @()
    notes          = @()
}
if (-not $ready) {
    $result["failures"] = @($script:failures)
    $result["finished_at"] = (Get-Date).ToString("o")
    New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
    $result | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $OutDir "probe_result.json") -Encoding UTF8
    exit 2
}

# 2) let startup identity binding + recovery activation settle, then read the
# KERNEL-side startup project identity. Do NOT use /agent/state here: an agent
# HTTP read before the new-project notification materializes history inside the
# pristine implicit draft and destroys the reuse precondition (observed live in
# the first probe iteration). Kernel reads do not touch the agent draft root.
Start-Sleep -Seconds 2
$kernelStateReply = $null
if (-not [string]::IsNullOrWhiteSpace($KernelHelper) -and (Test-Path -LiteralPath $KernelHelper)) {
    $kernelStateRaw = & $PythonExe $KernelHelper --url $KernelZmqUrl --cmd get_project_state 2>&1
    $result["kernel_get_project_state_raw"] = ($kernelStateRaw | Out-String)
    try { $kernelStateReply = ($kernelStateRaw | Out-String).Trim() | ConvertFrom-Json } catch { $kernelStateReply = $null }
}
$kernelUuidObserved = [string](Get-Prop $kernelStateReply "project_uuid" "")
if ([string]::IsNullOrWhiteSpace($kernelUuidObserved)) { $kernelUuidObserved = [string](Get-Prop (Get-Prop $kernelStateReply "project") "project_uuid" "") }
$kernelPathObserved = [string](Get-Prop $kernelStateReply "project_path" "")
if ([string]::IsNullOrWhiteSpace($kernelPathObserved)) { $kernelPathObserved = [string](Get-Prop (Get-Prop $kernelStateReply "project") "project_path" "") }
$oldKernelUuid = if (-not [string]::IsNullOrWhiteSpace($KernelUuid)) { $KernelUuid } else { $kernelUuidObserved }
$result["kernel_project_uuid_observed"] = $kernelUuidObserved
$result["kernel_project_path_observed"] = $kernelPathObserved
$result["old_kernel_uuid_used"] = $oldKernelUuid

$snapshotBefore = Get-Snapshot
$dirsBefore = @($snapshotBefore["dirs"])
$result["snapshot_before_invoke"] = $snapshotBefore
$implicitDirsBefore = @($dirsBefore | Where-Object { $seedSet -notcontains $_ })
$result["implicit_dirs_before_invoke"] = $implicitDirsBefore

# 2b) seeded mode: startup recovery must have claimed the seed matching the
# observed kernel startup uuid, leaving the implicit default draft pristine.
$claimedSeed = ""
if ($Mode -eq "seeded") {
    $seedRowsBefore = @($snapshotBefore["identities"] | Where-Object { $seedSet -contains $_.name })
    $claimedSeedRows = @($seedRowsBefore | Where-Object { $oldKernelUuid -ne "" -and $_.all_uuids -contains $oldKernelUuid })
    if ($claimedSeedRows.Count -eq 1) {
        $claimedSeed = $claimedSeedRows[0].name
        Write-ProbeOk ("startup recovery claimed seeded previous-session draft " + $claimedSeed + " (kernel uuid " + $oldKernelUuid + ")")
    } else {
        Write-ProbeFail ("startup recovery claimed " + $claimedSeedRows.Count + " seeded draft(s) for kernel uuid " + $oldKernelUuid + "; seeds=" + ($seedSet -join ","))
    }
    $result["claimed_seed"] = $claimedSeed
    if ($implicitDirsBefore.Count -eq 1) {
        $implicitRow = @($snapshotBefore["identities"] | Where-Object { $_.name -eq $implicitDirsBefore[0] })[0]
        $pristine = (-not $implicitRow.has_history -and -not $implicitRow.has_agent)
        if ($pristine) {
            Write-ProbeOk ("implicit default draft " + $implicitRow.name + " is still pristine before the new-project notification")
        } else {
            Write-ProbeFail ("implicit default draft " + $implicitRow.name + " was initialized before the notification (reuse precondition lost)")
        }
        $result["assertions"]["pristine_precondition"] = [ordered]@{ implicit_draft = $implicitRow.name; pristine = $pristine; pass = $pristine }
    } else {
        Write-ProbeFail ("expected exactly 1 implicit (non-seed) draft before the notification, got " + $implicitDirsBefore.Count + ": " + ($implicitDirsBefore -join ", "))
        $result["assertions"]["pristine_precondition"] = [ordered]@{ implicit_draft = ""; pristine = $false; pass = $false }
    }
} else {
    $result["claim_seed_skipped"] = "naive mode"
}

# 3) assertion 鈶?鈶? frontend-identical new-project lifecycle.
if ($KernelNewProjectFirst) {
    if ([string]::IsNullOrWhiteSpace($KernelHelper) -or -not (Test-Path -LiteralPath $KernelHelper)) {
        Write-ProbeFail ("kernel new_project helper missing: " + $KernelHelper)
        $newUuid = "vitproj_probe" + (Get-Date -Format "yyyyMMddHHmmss")
    } else {
        $kernelNewRaw = & $PythonExe $KernelHelper --url $KernelZmqUrl 2>&1
        $kernelNewExit = $LASTEXITCODE
        $result["kernel_new_project_raw"] = ($kernelNewRaw | Out-String)
        $kernelNewReply = $null
        try { $kernelNewReply = ($kernelNewRaw | Out-String).Trim() | ConvertFrom-Json } catch { $kernelNewReply = $null }
        $result["kernel_new_project_reply"] = $kernelNewReply
        $newUuid = [string](Get-Prop $kernelNewReply "project_uuid" "")
        if ([string]::IsNullOrWhiteSpace($newUuid)) { $newUuid = [string](Get-Prop $kernelNewReply "project_id" "") }
        if ([string]::IsNullOrWhiteSpace($newUuid)) {
            Write-ProbeFail ("kernel new_project did not return a project uuid (exit=" + $kernelNewExit + ")")
            $newUuid = "vitproj_probe" + (Get-Date -Format "yyyyMMddHHmmss")
        } else {
            Write-ProbeOk ("kernel new_project switched the live project to uuid " + $newUuid + " (exit=" + $kernelNewExit + ")")
            $result["kernel_new_project_uuid"] = $newUuid
            $result["kernel_new_project_path"] = [string](Get-Prop $kernelNewReply "project_path" "")
        }
        Start-Sleep -Seconds 1
    }
} else {
    $newUuid = "vitproj_probe" + (Get-Date -Format "yyyyMMddHHmmss")
}
$result["new_project_uuid"] = $newUuid
$invokeBody = @{
    tool      = "version.project_new"
    args      = @{ project_uuid = $newUuid; project_path = ""; source = "fedraft_probe" }
    source    = "fedraft_probe"
    confirmed = $true
}
$newResp = Invoke-AgentJson -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body $invokeBody -TimeoutSec 30
$result["version_project_new_response"] = $newResp
Start-Sleep -Seconds 2
$snapshotAfter = Get-Snapshot
$dirsAfter = @($snapshotAfter["dirs"])
$result["snapshot_after_new"] = $snapshotAfter

$newDirs = @($dirsAfter | Where-Object { $dirsBefore -notcontains $_ })
$result["new_dirs_after_new"] = $newDirs
$agentCreated = @($dirsAfter | Where-Object { $seedSet -notcontains $_ })
$result["agent_created_dirs"] = $agentCreated
if ($newDirs.Count -eq 0) {
    Write-ProbeOk "version.project_new added no draft directory (the pristine default draft was reused)"
} else {
    Write-ProbeFail ("version.project_new added " + $newDirs.Count + " new draft dir(s): " + ($newDirs -join ", "))
}
if ($agentCreated.Count -eq 1) {
    Write-ProbeOk ("agent-created draft dirs = 1 (expected 1; old behavior = 2)")
} else {
    Write-ProbeFail ("agent-created draft dirs = " + $agentCreated.Count + " (expected 1): " + ($agentCreated -join ", "))
}
$reusedRows = @($snapshotAfter["identities"] | Where-Object { $_.all_uuids -contains $newUuid })
$reusedNames = @($reusedRows | ForEach-Object { $_.name })
$result["reused_draft_dirs"] = $reusedNames
if ($reusedNames.Count -eq 1) {
    Write-ProbeOk ("new project uuid materialized in exactly one draft: " + $reusedNames[0])
} else {
    Write-ProbeFail ("new project uuid materialized in " + $reusedNames.Count + " draft(s): " + ($reusedNames -join ", "))
}
foreach ($row in $reusedRows) {
    $leaked = @($row.all_uuids | Where-Object { $_ -ne $newUuid -and $_ -eq $oldKernelUuid })
    if ($leaked.Count -eq 0) {
        Write-ProbeOk ("no old kernel-uuid workspace leaked into the new project draft " + $row.name)
    } else {
        Write-ProbeFail ("old kernel-uuid workspace " + ($leaked -join ",") + " leaked into the new project draft " + $row.name)
    }
}
if ($Mode -eq "seeded" -and $implicitDirsBefore.Count -eq 1) {
    if ($reusedNames -contains $implicitDirsBefore[0]) {
        Write-ProbeOk ("the pristine startup draft " + $implicitDirsBefore[0] + " was reused as the new project draft")
    } else {
        Write-ProbeFail ("new project uuid did not land in the pristine startup draft " + $implicitDirsBefore[0] + " (reused: " + ($reusedNames -join ",") + ")")
    }
}
$result["assertions"]["new_project_reuse"] = [ordered]@{
    new_dirs           = $newDirs
    agent_created_dirs = $agentCreated
    reused_draft_dirs  = $reusedNames
    pass               = ($newDirs.Count -eq 0 -and $reusedNames.Count -eq 1)
}

# 4) assertion 鈶? opening an existing project must not add a draft / empty shell.
if ($Mode -eq "seeded" -and $claimedSeed -ne "") {
    $openPath = Join-Path (Join-Path $DraftsRoot $claimedSeed) "Unsaved.vit"
    $openUuid = [string](@($snapshotBefore["identities"] | Where-Object { $_.name -eq $claimedSeed })[0].all_uuids | Select-Object -First 1)
    $openLabel = "seeded existing draft " + $claimedSeed
} elseif ($reusedNames.Count -ge 1) {
    $openPath = Join-Path (Join-Path $DraftsRoot $reusedNames[0]) "Unsaved.vit"
    $openUuid = $newUuid
    $openLabel = "new-project draft " + $reusedNames[0]
} else {
    $openPath = Join-Path (Join-Path $DraftsRoot $dirsAfter[0]) "Unsaved.vit"
    $openUuid = $oldKernelUuid
    $openLabel = "first draft " + $dirsAfter[0]
}
$dirsBeforeOpen = @(Get-DraftDirs)
$openBody = @{
    tool      = "version.project_opened"
    args      = @{ project_path = $openPath; project_uuid = $openUuid; source = "fedraft_probe" }
    source    = "fedraft_probe"
    confirmed = $true
}
$openResp = $null
try { $openResp = Invoke-AgentJson -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body $openBody -TimeoutSec 60 }
catch {
    $result["version_project_opened_error"] = $_.Exception.Message
    Write-ProbeFail ("version.project_opened HTTP error: " + $_.Exception.Message)
}
$result["version_project_opened_response"] = $openResp
Start-Sleep -Seconds 1
$dirsAfterOpen = @(Get-DraftDirs)
$newDirsOpen = @($dirsAfterOpen | Where-Object { $dirsBeforeOpen -notcontains $_ })
$result["new_dirs_after_open"] = $newDirsOpen
$result["open_target"] = [ordered]@{ label = $openLabel; project_path = $openPath; project_uuid = $openUuid }
if ($null -ne $openResp) {
    $openStatus = [string](Get-Prop $openResp "status" "")
    if ($newDirsOpen.Count -eq 0 -and $dirsAfterOpen.Count -eq $dirsBeforeOpen.Count) {
        Write-ProbeOk ("version.project_opened added no draft/shell (status=" + $openStatus + ")")
    } else {
        Write-ProbeFail ("version.project_opened added draft(s): " + ($newDirsOpen -join ", "))
    }
    $result["assertions"]["existing_project_open"] = [ordered]@{ new_dirs = $newDirsOpen; status = $openStatus; pass = ($newDirsOpen.Count -eq 0) }
} else {
    $result["assertions"]["existing_project_open"] = [ordered]@{ new_dirs = $newDirsOpen; status = "error"; pass = $false }
}

# 5) informational: list_tracks face.
$trackList = $null
try { $trackList = Invoke-AgentJson -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{ tool = "list_tracks"; args = @{}; source = "fedraft_probe"; confirmed = $true } -TimeoutSec 30 }
catch { $result["list_tracks_error"] = $_.Exception.Message }
$result["list_tracks_response_summary"] = if ($null -ne $trackList) {
    [ordered]@{ status = [string](Get-Prop $trackList "status" ""); track_count = (Get-Prop (Get-Prop $trackList "result") "track_count" $null) }
} else { $null }

# 6) final leak scan across the whole isolated root (seeds excluded).
$finalSnapshot = Get-Snapshot
$result["snapshot_final"] = $finalSnapshot
$leakRows = @()
foreach ($row in $finalSnapshot["identities"]) {
    if ($seedSet -contains $row.name) { continue }
    $leaked = @($row.all_uuids | Where-Object { $_ -eq $oldKernelUuid })
    if ($leaked.Count -gt 0) { $leakRows += [pscustomobject]@{ name = $row.name; uuids = $leaked } }
}
$result["old_uuid_leak_rows"] = $leakRows
if ($leakRows.Count -eq 0) {
    Write-ProbeOk "no old kernel-uuid workspace materialized outside the recovery target"
} else {
    Write-ProbeFail ("old kernel-uuid workspace leaked outside recovery target: " + (($leakRows | ForEach-Object { $_.name }) -join ", "))
}

$result["failures"] = @($script:failures)
$result["notes"] = @($script:notes)
$result["finished_at"] = (Get-Date).ToString("o")
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$result | ConvertTo-Json -Depth 15 | Set-Content -LiteralPath (Join-Path $OutDir "probe_result.json") -Encoding UTF8
Write-Host ("[probe] result: " + (Join-Path $OutDir "probe_result.json"))
if ($script:failures.Count -gt 0) { Write-Host ("[probe] FAIL count=" + $script:failures.Count); exit 1 }
Write-Host "[probe] PASS"; exit 0


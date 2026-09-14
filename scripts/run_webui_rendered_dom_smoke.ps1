<#
E2E-WEBUI-1 (2026-09-13): rendered-surface end-side smoke for the webui.

Purpose (AGENTS.md section 5, "rendered surface and user journey gate", added
2026-09-13 by the user): the existing smoke layer only asserts the agent's
HTTP/ZMQ surface. It cannot see what the browser renders, which is exactly how
UI-FOLLOW-1 passed vitest+build and still shipped a visible defect to the user.
This script closes that gap: it starts a real VitAgent, opens /app/ in a real
browser, and asserts the rendered DOM. Exit code 0 is the delivery gate for
webui changes.

Shape of the run (all three are real; nothing is mocked):
  * agent   -- VitAgent.exe built from the working tree, started with an
              isolated draft root (VIT_HISTORY_DRAFT_ROOT) and its own log, on
              its own HTTP port. It never touches a stack another session owns
              (AGENTS.md section 9).
  * browser -- Chromium (Playwright), headless: bundled build if installed,
              else the system Edge channel. See docs in the report for why no
              headed window is needed.
  * events  -- the archived real event stream of conversation webui_mtzba6wf
              (artifacts/_forensic_ev.json, 2026-09-13 12:26-12:28) is replayed
              at the network layer for GET /agent/events, so no LLM call is
              needed and the run is deterministic. The archived conversation
              graph is seeded into the isolated draft history so the webui
              hydrates its real transcript (Project History path) instead of an
              optimistic client message.

Assertions and the report schema live in webui_rendered_dom_smoke.mjs.

Usage:
  powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_webui_rendered_dom_smoke.ps1
  powershell ... -SkipBuild            # reuse the already-built binary + dist
  powershell ... -HttpPort 7897 -KeepStack

This file is deliberately pure ASCII (Windows PowerShell 5.1 reads BOM-less
.ps1 with the ANSI code page; literal CJK would corrupt the parse).
#>
[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = "",
    [string]$Affix = "e2e_webui1",
    [int]$HttpPort = 7897,
    [switch]$SkipBuild,
    [string]$AgentBinary = "",
    [string]$ConversationId = "webui_mtzba6wf",
    [string]$GraphFixture = "",
    [string]$EventsFixture = "",
    [string]$CommitDirs = "",
    [string]$PwModule = "",
    [int]$StartupSeconds = 30,
    [int]$TimeoutSeconds = 600,
    [switch]$KeepStack,
    [switch]$Headed
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ("== " + $Message) -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("ok: " + $Message) -ForegroundColor Green }
function Write-Bad { param([string]$Message) Write-Host ("fail: " + $Message) -ForegroundColor Red }
function Write-Note { param([string]$Message) Write-Host ("   " + $Message) }

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) { return (Resolve-Path -LiteralPath $Explicit).Path }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

function Get-TcpListener { param([int]$Port) return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1 }

function Wait-HttpReady {
    param([string]$BaseUrl, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri ($BaseUrl.TrimEnd("/") + "/health") -TimeoutSec 2
            if ($resp.StatusCode -eq 200) { return $true }
        }
        catch { }
        Start-Sleep -Milliseconds 500
    }
    return $false
}

# The archived graph references real commit objects; the agent's history reader
# silently drops nodes whose commit object is missing, so the smoke must hand
# them over. The objects it needs are named by the graph fixture itself.
# Read as UTF-8 explicitly: Windows PowerShell 5.1 decodes a BOM-less file with
# the ANSI code page, which mangles the fixture's CJK node texts badly enough to
# break ConvertFrom-Json. The commit ids themselves are plain ASCII, so they are
# extracted with a regex over the decoded text (no JSON parser involved).
function Get-GraphCommitIds {
    param([string]$GraphFixture)
    $ids = New-Object System.Collections.Generic.List[string]
    if ([string]::IsNullOrWhiteSpace($GraphFixture) -or -not (Test-Path -LiteralPath $GraphFixture)) { return $ids }
    $raw = Get-Content -LiteralPath $GraphFixture -Raw -Encoding UTF8
    if ([string]::IsNullOrWhiteSpace($raw)) { return $ids }
    foreach ($match in [regex]::Matches($raw, '"commit_id"\s*:\s*"([^"]+)"')) {
        $commitId = $match.Groups[1].Value.Trim()
        if ($commitId -ne "" -and -not $ids.Contains($commitId)) { $ids.Add($commitId) }
    }
    return $ids
}

# TRAJ-IMPL-1 (2026-09-14): "newest non-empty commits directory" is NOT the right
# pick. Any later session directory holding different commit objects makes every
# archived node get skipped, and the smoke then dies on its own hydration
# precondition (exit 2 -- an environment failure, not a finding about the webui).
# Resolve by content first: the directory that actually holds the fixture's own
# commit ids wins; recency is only the fallback (and the whole fallback is the
# old behaviour when no required id is known).
function Find-SessionCommitDir {
    param([string]$HistoryRoot, [string[]]$RequiredCommitIds = @())
    if ([string]::IsNullOrWhiteSpace($HistoryRoot) -or -not (Test-Path -LiteralPath $HistoryRoot)) { return "" }
    $candidates = Get-ChildItem -LiteralPath $HistoryRoot -Recurse -Directory -Filter "commits" -ErrorAction SilentlyContinue |
        Where-Object { $_.FullName -like "*\.sessions\*" } |
        Sort-Object LastWriteTime -Descending
    $best = ""
    $bestHits = 0
    foreach ($candidate in $candidates) {
        $count = @(Get-ChildItem -LiteralPath $candidate.FullName -File -ErrorAction SilentlyContinue).Count
        if ($count -eq 0) { continue }
        if ($RequiredCommitIds.Count -eq 0) { return $candidate.FullName }
        $hits = 0
        foreach ($commitId in $RequiredCommitIds) {
            if (Test-Path -LiteralPath (Join-Path $candidate.FullName ($commitId + ".json"))) { $hits += 1 }
        }
        if ($hits -gt $bestHits) { $bestHits = $hits; $best = $candidate.FullName }
    }
    if ($bestHits -gt 0) { return $best }
    foreach ($candidate in $candidates) {
        $count = @(Get-ChildItem -LiteralPath $candidate.FullName -File -ErrorAction SilentlyContinue).Count
        if ($count -gt 0) { return $candidate.FullName }
    }
    return ""
}

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $RunRoot = Join-Path $RepoRoot ("artifacts\" + $Affix + "\" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
if (Test-Path -LiteralPath $RunRoot) { throw ("run root already exists; each run needs a fresh artifact directory: " + $RunRoot) }
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null

$AgentDir = Join-Path $RepoRoot "agent"
$WebUIDir = Join-Path $AgentDir "webui"
$AgentDrafts = Join-Path $RunRoot "agent_drafts"
New-Item -ItemType Directory -Force -Path $AgentDrafts | Out-Null
$AgentLog = Join-Path $RunRoot "agent_last.log"
$PrereqPath = Join-Path $RunRoot "prereq.txt"
$SmokeScript = Join-Path $RepoRoot "scripts\webui_rendered_dom_smoke.mjs"
$FixtureDir = Join-Path $RepoRoot "scripts\fixtures\webui_rendered_dom"
if ([string]::IsNullOrWhiteSpace($GraphFixture)) { $GraphFixture = Join-Path $FixtureDir "webui_rendered_dom_graph.fixture.json" }
if ([string]::IsNullOrWhiteSpace($EventsFixture)) { $EventsFixture = Join-Path $FixtureDir "webui_rendered_dom_events.fixture.json" }
# The user's own project history is READ ONLY here: the archived graph and its
# commit objects are the evidence this smoke replays, never a write target.
$UserHistoryRoot = "D:\Godot\project\vit-daw-frontend\.vit_history"

$prereq = New-Object System.Collections.Generic.List[string]
function Add-Prereq { param([string]$Line) $script:prereq.Add($Line); Write-Host $Line }

$agentProcId = $null
$outcome = "env_failure"
$smokeExit = $null

try {
    Add-Prereq ("run_root=" + $RunRoot)
    Add-Prereq ("affix=" + $Affix)
    Add-Prereq ("head=" + (& git -C $RepoRoot rev-parse HEAD))
    Add-Prereq ("git_status=" + ((& git -C $RepoRoot status --short) -join " ; "))
    Add-Prereq ("http_port=" + $HttpPort)
    # AGENTS.md section 9: another session's stack owns the default bridge ports
    # (ZMQ 5555/5556, Godot UDP 4444/4445) whenever it runs the real three-piece
    # stack. The agent refuses to start when it cannot bind its command UDP port,
    # so a gate run must record which ports it will actually own. The overrides
    # come from the documented VIT_AGENT_* environment variables and are unset by
    # default (agent defaults below).
    $bridgeEnv = @()
    foreach ($bridgeName in @("VIT_AGENT_ZMQ_REQ_URL", "VIT_AGENT_ZMQ_SUB_URL", "VIT_AGENT_UDP_TO_GODOT", "VIT_AGENT_UDP_FROM_GODOT")) {
        $bridgeValue = [Environment]::GetEnvironmentVariable($bridgeName, "Process")
        if (-not [string]::IsNullOrWhiteSpace($bridgeValue)) { $bridgeEnv += ($bridgeName + "=" + $bridgeValue) }
    }
    if ($bridgeEnv.Count -eq 0) {
        Add-Prereq "bridge_ports=agent defaults (zmq 5555/5556, udp 4444/4445)"
    } else {
        Add-Prereq ("bridge_ports=" + ($bridgeEnv -join " "))
    }
    Add-Prereq ("conversation_id=" + $ConversationId)
    Add-Prereq ("started=" + (Get-Date -Format "o"))

    Write-Step ("Port " + $HttpPort + " must be free (stack ownership, AGENTS.md section 9)")
    if (Get-TcpListener -Port $HttpPort) { throw ("port " + $HttpPort + " is already listening; refusing to reuse another session's stack") }
    Add-Prereq ("port_free=" + $HttpPort)

    if ([string]::IsNullOrWhiteSpace($AgentBinary)) { $AgentBinary = Join-Path $RunRoot "bin\VitAgent.e2ewebui1.exe" }
    if (-not $SkipBuild) {
        Write-Step "Build VitAgent (isolated binary in the run directory)"
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $AgentBinary) | Out-Null
        Push-Location $AgentDir
        try {
            & go build -o $AgentBinary .\cmd\vitagent
            if ($LASTEXITCODE -ne 0) { throw ("go build failed with exit code " + $LASTEXITCODE) }
        }
        finally { Pop-Location }
        Write-Ok "VitAgent built"

        Write-Step "Build webui dist (the bundle the browser will load)"
        Push-Location $WebUIDir
        try {
            # npm writes harmless npm-warning lines to stderr; with
            # $ErrorActionPreference = "Stop" a native stderr line surfaces as a
            # terminating RemoteException, so relax it for the call and judge
            # the build by its exit code only.
            $npmErr = Join-Path $RunRoot "npm_build.stderr.txt"
            $savedEap = $ErrorActionPreference
            $ErrorActionPreference = "Continue"
            try { & npm run build 2>$npmErr | Out-Host; $npmExit = $LASTEXITCODE }
            finally { $ErrorActionPreference = $savedEap }
            if (Test-Path -LiteralPath $npmErr) {
                $stderrText = (Get-Content -LiteralPath $npmErr -Raw)
                if (-not [string]::IsNullOrWhiteSpace($stderrText)) { Add-Prereq ("npm_build_stderr=" + (($stderrText -split "?
" | Where-Object { $_ -ne "" }) -join " | ")) }
            }
            if ($npmExit -ne 0) { throw ("npm run build failed with exit code " + $npmExit) }
        }
        finally { Pop-Location }
        Write-Ok "webui dist built"
    }
    elseif (-not (Test-Path -LiteralPath $AgentBinary -PathType Leaf)) { throw "-SkipBuild given but the isolated agent binary is missing" }

    $agentItem = Get-Item -LiteralPath $AgentBinary
    Add-Prereq ("agent_binary=" + $AgentBinary + " size=" + $agentItem.Length + " mtime=" + $agentItem.LastWriteTime.ToString("o") + " sha256=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $AgentBinary).Hash)
    $indexPath = Join-Path $WebUIDir "dist\index.html"
    if (-not (Test-Path -LiteralPath $indexPath)) { throw ("webui bundle missing: " + $indexPath) }
    $assets = Get-ChildItem -LiteralPath (Join-Path $WebUIDir "dist\assets") -File -ErrorAction SilentlyContinue
    Add-Prereq ("webui_index_sha256=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $indexPath).Hash)
    foreach ($asset in $assets) { Add-Prereq ("webui_asset=" + $asset.Name + " size=" + $asset.Length + " sha256=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $asset.FullName).Hash) }

    Write-Step "Check the archived evidence inputs"
    foreach ($fixture in @($GraphFixture, $EventsFixture, $SmokeScript)) {
        if (-not (Test-Path -LiteralPath $fixture)) { throw ("missing smoke input: " + $fixture) }
    }
    Add-Prereq ("graph_fixture=" + $GraphFixture + " sha256=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $GraphFixture).Hash)
    Add-Prereq ("events_fixture=" + $EventsFixture + " sha256=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $EventsFixture).Hash)
    Add-Prereq ("smoke_script=" + $SmokeScript + " sha256=" + (Get-FileHash -Algorithm SHA256 -LiteralPath $SmokeScript).Hash)

    $graphCommitIds = @(Get-GraphCommitIds -GraphFixture $GraphFixture)
    if ([string]::IsNullOrWhiteSpace($CommitDirs)) {
        $sessionCommits = Find-SessionCommitDir -HistoryRoot $UserHistoryRoot -RequiredCommitIds $graphCommitIds
        if (-not [string]::IsNullOrWhiteSpace($sessionCommits)) { $CommitDirs = $sessionCommits }
    }
    if ([string]::IsNullOrWhiteSpace($CommitDirs)) {
        Write-Note "no archived session commit directory found; the smoke will report how many graph nodes it had to skip"
    } else {
        Add-Prereq ("commit_dirs=" + $CommitDirs)
        $commitHits = 0
        foreach ($commitId in $graphCommitIds) {
            if (Test-Path -LiteralPath (Join-Path $CommitDirs ($commitId + ".json"))) { $commitHits += 1 }
        }
        Add-Prereq ("graph_commit_ids=" + $graphCommitIds.Count + " matched_in_commit_dirs=" + $commitHits)
        Write-Ok ("archived commit objects: " + $CommitDirs + " (" + $commitHits + "/" + $graphCommitIds.Count + " required objects present)")
    }

    # Playwright may already be on the host through npx even though the repo
    # has no package.json entry for it: the npm cache keeps
    # _npx\<hash>\node_modules\playwright-core. Prefer the newest one there
    # (and honour an explicit -PwModule / $env:PW_MODULE).
    if ([string]::IsNullOrWhiteSpace($PwModule)) {
        $pwCandidates = New-Object System.Collections.Generic.List[string]
        $pwCandidates.Add((Join-Path $WebUIDir "node_modules\playwright-core"))
        $pwCandidates.Add((Join-Path $WebUIDir "node_modules\playwright"))
        $pwCandidates.Add((Join-Path $RepoRoot "node_modules\playwright-core"))
        $pwCandidates.Add((Join-Path $RepoRoot "node_modules\playwright"))
        $npxRoots = @()
        if (-not [string]::IsNullOrWhiteSpace($env:npm_config_cache)) { $npxRoots += (Join-Path $env:npm_config_cache "_npx") }
        if (-not [string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) { $npxRoots += (Join-Path $env:LOCALAPPDATA "npm-cache\_npx") }
        if (-not [string]::IsNullOrWhiteSpace($env:APPDATA)) { $npxRoots += (Join-Path $env:APPDATA "npm-cache\_npx") }
        foreach ($npxRoot in $npxRoots) {
            if (-not (Test-Path -LiteralPath $npxRoot)) { continue }
            $found = Get-ChildItem -LiteralPath $npxRoot -Directory -ErrorAction SilentlyContinue |
                ForEach-Object { Join-Path $_.FullName "node_modules\playwright-core" } |
                Where-Object { Test-Path -LiteralPath (Join-Path $_ "package.json") } |
                Sort-Object { (Get-Item -LiteralPath $_).LastWriteTime } -Descending
            foreach ($item in $found) { if (-not $pwCandidates.Contains($item)) { $pwCandidates.Add($item) } }
        }
        foreach ($candidate in $pwCandidates) {
            if (Test-Path -LiteralPath (Join-Path $candidate "package.json")) { $PwModule = $candidate; break }
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($PwModule)) { Add-Prereq ("playwright_module=" + $PwModule) }

    Write-Step "Start the isolated agent (own draft root, own log, own port)"
    $priorDraftRoot = [Environment]::GetEnvironmentVariable("VIT_HISTORY_DRAFT_ROOT", "Process")
    $priorDevRoot = [Environment]::GetEnvironmentVariable("VIT_DAW_DEV_ROOT", "Process")
    $env:VIT_HISTORY_DRAFT_ROOT = $AgentDrafts
    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    try {
        $agentProc = Start-Process -FilePath $AgentBinary -ArgumentList @("-http", ("127.0.0.1:" + $HttpPort), "-last-log-path", $AgentLog, "-keep-last-log-lines", "4000") -WorkingDirectory $AgentDir -WindowStyle Hidden -PassThru
    }
    finally {
        if ($null -eq $priorDraftRoot) { Remove-Item Env:VIT_HISTORY_DRAFT_ROOT -ErrorAction SilentlyContinue } else { $env:VIT_HISTORY_DRAFT_ROOT = $priorDraftRoot }
        if ($null -eq $priorDevRoot) { Remove-Item Env:VIT_DAW_DEV_ROOT -ErrorAction SilentlyContinue } else { $env:VIT_DAW_DEV_ROOT = $priorDevRoot }
    }
    $agentProcId = $agentProc.Id
    Add-Prereq ("agent_pid=" + $agentProcId)
    $base = "http://127.0.0.1:" + $HttpPort
    if (-not (Wait-HttpReady -BaseUrl $base -TimeoutSeconds $StartupSeconds)) { throw "agent HTTP did not become ready" }
    Write-Ok ("agent up, pid=" + $agentProcId + " at " + $base)

    Write-Step "Run the rendered DOM smoke (Playwright)"
    $nodeArgs = @(
        $SmokeScript,
        "--agent-base", $base,
        "--conversation-id", $ConversationId,
        "--graph-fixture", $GraphFixture,
        "--events-fixture", $EventsFixture,
        "--out-dir", $RunRoot
    )
    if (-not [string]::IsNullOrWhiteSpace($CommitDirs)) { $nodeArgs += @("--commit-dirs", $CommitDirs) }
    if (-not [string]::IsNullOrWhiteSpace($PwModule)) { $nodeArgs += @("--playwright-module", $PwModule) }
    if ($Headed) { $nodeArgs += @("--headed", "1") }

    $nodeErr = Join-Path $RunRoot "smoke.stderr.txt"
    $savedNodeEap = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try { & node @nodeArgs 2>$nodeErr | Out-Host; $smokeExit = $LASTEXITCODE }
    finally { $ErrorActionPreference = $savedNodeEap }
    if (Test-Path -LiteralPath $nodeErr) {
        $nodeStderr = (Get-Content -LiteralPath $nodeErr -Raw)
        if (-not [string]::IsNullOrWhiteSpace($nodeStderr)) {
            Add-Prereq ("smoke_stderr=" + (($nodeStderr -split "?
" | Where-Object { $_ -ne "" }) -join " | "))
            $nodeStderr | Out-File -FilePath (Join-Path $RunRoot "smoke_stderr.log") -Encoding utf8
        }
    }
    Add-Prereq ("smoke_exit=" + $smokeExit)
    if ($smokeExit -eq 0) { Write-Ok "rendered DOM smoke PASS" } else { Write-Bad ("rendered DOM smoke exit " + $smokeExit) }

    $reportPath = Join-Path $RunRoot "webui_rendered_dom_report.json"
    if (Test-Path -LiteralPath $reportPath) {
        try {
            # UTF-8 explicitly, and never fatal: the smoke's own exit code is the
            # delivery decision. Windows PowerShell 5.1 decodes a BOM-less file
            # with the ANSI code page, which corrupts the CJK assertion notes and
            # can make them unparseable (observed 2026-09-14: smoke_exit=0 with
            # report verdict=pass, yet the wrapper reported env_failure because
            # this very read threw). A report that cannot be read is recorded as
            # lost evidence, not as a verdict.
            $report = Get-Content -LiteralPath $reportPath -Raw -Encoding UTF8 | ConvertFrom-Json
            Add-Prereq ("verdict=" + $report.verdict)
            Add-Prereq ("failed_groups=" + (($report.failed_groups) -join ","))
            Add-Prereq ("browser=" + $report.browser + " " + $report.browser_version)
        }
        catch {
            Add-Prereq ("report_read_error=" + $_.Exception.Message)
        }
    }
    Write-Host ""
    Write-Host ("ARTIFACT_RUN_ROOT " + $RunRoot)

    switch ($smokeExit) {
        0 { $outcome = "pass" }
        1 { $outcome = "smoke_failed" }
        default { $outcome = "smoke_error" }
    }
}
catch {
    $detail = $_.Exception.Message
    Add-Prereq ("fatal=" + $detail)
    Add-Prereq ("fatal_type=" + $_.Exception.GetType().FullName)
    Write-Bad ("fatal: " + $detail)
}
finally {
    Write-Step "Teardown"
    if ($agentProcId -and -not $KeepStack) {
        Stop-Process -Id $agentProcId -Force -ErrorAction SilentlyContinue
        Add-Prereq ("stopped_agent_pid=" + $agentProcId)
        Start-Sleep -Seconds 2
    }
    elseif ($agentProcId) { Add-Prereq ("stack_kept_pid=" + $agentProcId) }
    if (Get-TcpListener -Port $HttpPort) { Add-Prereq ("port_still_busy=" + $HttpPort) } else { Add-Prereq ("port_released=" + $HttpPort) }
    Add-Prereq ("finished=" + (Get-Date -Format "o"))
    $prereq | Out-File -FilePath $PrereqPath -Encoding utf8
}

Write-Host ""
Write-Host ("WEBUI_RENDERED_DOM_VERDICT " + $outcome)
switch ($outcome) {
    "pass" { exit 0 }
    "smoke_failed" { exit 1 }
    default { exit 2 }
}

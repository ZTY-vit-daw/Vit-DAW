#Requires -Version 5.1
[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$PublicManifest = "",
    [string]$PublicCaseId = "spv1_p01",
    [string]$AgentHttp = "http://127.0.0.1:7878",
    [string]$GodotProjectRoot = "D:\Godot\project\vit-daw-frontend",
    [string]$GodotExe = "",
    [string]$KernelExe = "",
    [int]$TimeoutSeconds = 600,
    [switch]$SkipBuild,
    [switch]$AdmissionOnly,
    [ValidateSet("retain", "rollback", "ambiguous")]
    [string]$SettlementProbe = "",
    [ValidateSet("neutral", "frequency", "compression", "leveling", "sibilance", "transient", "pan", "limiter", "gate", "multiband", "reverb", "delay")]
    [string]$PromptFlavor = "neutral",
    [ValidateSet("any", "track_gain", "static_eq", "broadband_compression", "de_esser", "transient_shaper", "pan", "limiter", "gate_expander", "multiband_dynamics", "reverb", "delay")]
    [string]$ExpectDomain = "any",
    # D2-2-S3 multi-round probe switch: passes --multi-round-probe to the
    # runner, which swaps the single-round D1 tail for the independent
    # validate_d2_multi_round assertions. TODO(D2-2-S2): the deterministic
    # multi-round drive (memory gear-on + dose-calibration scenario injection)
    # waits on the D2-2-S2 parameter-injection channel; until it merges an
    # open-prompt run usually reports NOT_EXERCISED (exit 3), which is a
    # recorded acceptable outcome, not something to paper over.
    [switch]$MultiRoundProbe,
    # MRREG1 (2026-08-31): the D2-2 admission tier is env-sourced
    # (VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET, sealed 2..4; missing or malformed
    # fails closed to 1 and the model cannot upgrade it), so a -MultiRoundProbe
    # run without an in-range tier can never exercise multi-round continuation.
    # The launcher owns the injection: default 2, and a conflicting caller env
    # is a hard error instead of a silent override.
    [ValidateRange(2, 4)]
    [int]$MultiRoundBudget = 2,
    # D2-REG3 A face (2026-09-02): passes --expect-honest-refusal to the
    # runner, which swaps the applied-path D1 tail for the four
    # honest-refusal requirements (valid proposal in an admitted domain,
    # fail-closed refusal, zero forward mutation / zero unplanned side
    # effects, REG2 honest terminal with the StopProjectRevisionStale
    # misattribution explicitly rejected). Any requirement not met is an
    # honest FAIL, not a retry loop.
    [switch]$ExpectHonestRefusal,
    # FS-PARK-TURNFAIL-1 (2026-09-30 用户裁定): with the round parked at the
    # human-judgment boundary, send a NEW user message through /agent/chat and
    # assert the default-adoption settlement — adopted_by_continuation, zero
    # judgment evidence, fresh goal/run/turn identity, reply landed, applied
    # state kept. Pre-fix live failure (2026-09-30 22:07): turn.failed.
    [switch]$ContinuationAdoptionProbe,
    # FS-STOP-APPLY-1 (2026-10-01 M1 活栈取证): with the round parked at the
    # human-judgment boundary, POST /agent/turn/stop and assert the honest
    # stopped goal status, the audio-closure settle with the controller-owner
    # release, zero further interventions, the stop request-arrival log anchor,
    # and an unwedged conversation (a NEW user message runs on a fresh goal).
    # Pre-fix live failure (2026-10-01 18:35): the next message failed with
    # audio_closure_controller_failure "conversation is already owned by ...".
    [switch]$StopSemanticsProbe,
    # FIX-TURNSTOP-WEDGE-1 (2026-10-15 M2-EXPERIENCE-1 取证): fire the user
    # stop immediately after the first ask — before any boundary — and assert
    # the M2 wedge is gone: goal/task/slice converge out of active/running
    # (settle-window fallback covers a stalled runner), the second stop stays
    # idempotent, and the conversation can start a new turn afterwards.
    # Pre-fix live failure (2026-10-15 10:05): task=active/slice=running frozen
    # 10 minutes; second stop returned ok while converging nothing.
    [switch]$ImmediateStopProbe,
    # LOUDNESS-B-TRIGGER revision leg (BATCH1 2026-10-17 §二 案①, dispatched
    # 2026-10-18): the loudness-watch real-stack proof moves onto this D1
    # settle leg because only the D1 flow deterministically drives the
    # free-state triggers that lazily register the watch hook (T3
    # project_change after the applied action; T1 settle where a settle lands).
    # Off by default = zero behavior change: the runner stays synchronous and
    # no extra HTTP calls are made. On: the runner becomes a background
    # process, the watch is configured while the D1 project binding is live
    # (before the applied action), and the post-run LW gates assert disclosure,
    # verdict store + round-borne trigger, dedupe, config-off silence and the
    # E1 cost extraction.
    [switch]$LoudnessWatch,
    # TIM-SMOKE lesson (run2 fs_largeproject_smoke_20261017_192734): a cold
    # kernel/plugin-table start can sail past the dev_agent_smoke default 20s
    # UI gate; the D1 launcher now owns and passes a >=90s budget through.
    [int]$UiPortWaitSeconds = 90,
    # AGENT-1/AGENT-2 (2026-09-05): per-domain processor-selection route
    # switch, e.g. "broadband_compression=processor_selection". The launcher
    # owns the injection into the child agent process env
    # (VIT_AGENT_DOMAIN_ROUTES); a conflicting caller env is a hard error
    # instead of a silent override. Default (empty) keeps every domain on
    # legacy_native.
    [string]$DomainRoute = "",
    # AGENT-2 A3: evaluator-side sealed target check. Resolves the sealed
    # truth artifact next to the public manifest and passes it to the smoke;
    # the smoke opens it only after the admission target is frozen and never
    # sends its values to the agent.
    [switch]$SealedTargetCheck,
    # AGENT-1 milestone (walked by AGENT-2): require a processor_selection.v1
    # record for this action domain plus the [domain-route] processor_selection
    # log lines (observability of the new selection route).
    [ValidateSet("", "track_gain", "static_eq", "broadband_compression", "de_esser", "transient_shaper", "pan", "limiter", "gate_expander", "multiband_dynamics")]
    [string]$ExpectProcessorSelection = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
if ([string]::IsNullOrWhiteSpace($PublicManifest)) {
    $PublicManifest = Join-Path $RepoRoot "temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20\fixture_manifest.json"
}
$PublicManifest = (Resolve-Path -LiteralPath $PublicManifest).Path
if (($PublicManifest -split '[\\/]') -contains 'sealed') {
    throw "D1 smoke refuses manifest paths containing a sealed segment"
}
if ($MultiRoundProbe -and ($AdmissionOnly -or $SettlementProbe -ne "")) {
    throw "-MultiRoundProbe owns the run tail and cannot be combined with -AdmissionOnly or -SettlementProbe"
}
if ($ExpectHonestRefusal -and ($MultiRoundProbe -or $AdmissionOnly -or $SettlementProbe -ne "")) {
    throw "-ExpectHonestRefusal owns the run tail and cannot be combined with -MultiRoundProbe, -AdmissionOnly, or -SettlementProbe"
}
if ($ContinuationAdoptionProbe -and ($MultiRoundProbe -or $AdmissionOnly -or $SettlementProbe -ne "" -or $ExpectHonestRefusal)) {
    throw "-ContinuationAdoptionProbe owns the run tail and cannot be combined with -MultiRoundProbe, -AdmissionOnly, -SettlementProbe, or -ExpectHonestRefusal"
}
if ($StopSemanticsProbe -and ($MultiRoundProbe -or $AdmissionOnly -or $SettlementProbe -ne "" -or $ExpectHonestRefusal -or $ContinuationAdoptionProbe)) {
    throw "-StopSemanticsProbe owns the run tail and cannot be combined with -MultiRoundProbe, -AdmissionOnly, -SettlementProbe, -ExpectHonestRefusal, or -ContinuationAdoptionProbe"
}
if ($ImmediateStopProbe -and ($MultiRoundProbe -or $AdmissionOnly -or $SettlementProbe -ne "" -or $ExpectHonestRefusal -or $ContinuationAdoptionProbe -or $StopSemanticsProbe)) {
    throw "-ImmediateStopProbe owns the whole run (it stops the first ask before any boundary) and cannot be combined with -MultiRoundProbe, -AdmissionOnly, -SettlementProbe, -ExpectHonestRefusal, -ContinuationAdoptionProbe, or -StopSemanticsProbe"
}
# The LW post-run gates only run after the default D1 runner exits 0, so the
# tail-owning probe switches (which either exit before the settle or replace
# the run tail) are mutually exclusive with -LoudnessWatch.
if ($LoudnessWatch -and ($MultiRoundProbe -or $AdmissionOnly -or $SettlementProbe -ne "" -or $ExpectHonestRefusal -or $ContinuationAdoptionProbe -or $StopSemanticsProbe -or $ImmediateStopProbe -or $ExpectProcessorSelection -ne "")) {
    throw "-LoudnessWatch owns the post-run tail (LW gates run after a default D1 runner exits 0) and cannot be combined with -MultiRoundProbe, -AdmissionOnly, -SettlementProbe, -ExpectHonestRefusal, -ContinuationAdoptionProbe, -StopSemanticsProbe, -ImmediateStopProbe, or -ExpectProcessorSelection"
}
$multiroundTierEnv = [string]$env:VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET
# AGENT-1/AGENT-2 domain-route env discipline: the launcher owns
# VIT_AGENT_DOMAIN_ROUTES. An explicit -DomainRoute must be the only source
# (caller conflicts are a hard error); without the parameter the caller env is
# cleared for the child so a stale shell cannot silently flip routes.
$domainRouteEnv = [string]$env:VIT_AGENT_DOMAIN_ROUTES
if (-not [string]::IsNullOrWhiteSpace($DomainRoute)) {
    foreach ($entry in ($DomainRoute -split ',')) {
        $pair = $entry.Trim()
        if ($pair -eq "") { continue }
        $parts = $pair -split '=', 2
        if ($parts.Count -ne 2 -or [string]::IsNullOrWhiteSpace($parts[0]) -or
            ($parts[1].Trim() -ne "processor_selection" -and $parts[1].Trim() -ne "legacy_native")) {
            throw ("DomainRoute entry '" + $pair + "' is malformed; expected domain=processor_selection|legacy_native")
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($domainRouteEnv) -and $domainRouteEnv.Trim() -ne $DomainRoute.Trim()) {
        throw ("caller env VIT_AGENT_DOMAIN_ROUTES='" + $domainRouteEnv + "' conflicts with -DomainRoute '" + $DomainRoute + "'; align them instead of relying on a silent override")
    }
    $env:VIT_AGENT_DOMAIN_ROUTES = $DomainRoute
}
else {
    if (-not [string]::IsNullOrWhiteSpace($domainRouteEnv)) {
        Write-Warning ("clearing caller env VIT_AGENT_DOMAIN_ROUTES='" + $domainRouteEnv + "' for this run; pass -DomainRoute to enable a route explicitly")
        Remove-Item Env:VIT_AGENT_DOMAIN_ROUTES -ErrorAction SilentlyContinue
    }
}
if ($ExpectProcessorSelection -ne "" -and [string]::IsNullOrWhiteSpace($DomainRoute)) {
    throw "-ExpectProcessorSelection requires the processor_selection route; pass -DomainRoute with '<domain>=processor_selection'"
}
if ($ExpectProcessorSelection -ne "" -and -not $DomainRoute.Contains(($ExpectProcessorSelection + "=processor_selection"))) {
    throw ("-ExpectProcessorSelection " + $ExpectProcessorSelection + " is not enabled by -DomainRoute '" + $DomainRoute + "'")
}
$sealedTruthPath = ""
if ($SealedTargetCheck) {
    $sealedTruthPath = Join-Path (Split-Path -Parent $PublicManifest) "sealed\sealed_truth.json"
    if (-not (Test-Path -LiteralPath $sealedTruthPath -PathType Leaf)) {
        throw ("sealed truth artifact is missing at " + $sealedTruthPath)
    }
}
$agentLogPath = Join-Path $RepoRoot "VitApp\Workspace\Logs\agent_last.log"
if ($MultiRoundProbe) {
    if (-not [string]::IsNullOrWhiteSpace($multiroundTierEnv)) {
        $callerTier = 0
        if (-not [int32]::TryParse($multiroundTierEnv.Trim(), [ref]$callerTier) -or $callerTier -lt 2 -or $callerTier -gt 4) {
            throw ("VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET='" + $multiroundTierEnv + "' is malformed or outside the sealed 2..4 tier range; the agent would fail closed to a single-round tier and the multi-round probe could never pass. Unset it or pass -MultiRoundBudget within 2..4.")
        }
        if ($callerTier -ne $MultiRoundBudget) {
            throw ("caller env VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET=" + $callerTier + " conflicts with -MultiRoundBudget " + $MultiRoundBudget + "; align them instead of relying on a silent override")
        }
    }
    else {
        $env:VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET = [string]$MultiRoundBudget
    }
}
elseif (-not [string]::IsNullOrWhiteSpace($multiroundTierEnv)) {
    $callerTier = 0
    if ([int32]::TryParse($multiroundTierEnv.Trim(), [ref]$callerTier) -and $callerTier -ge 2 -and $callerTier -le 4) {
        throw ("VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET=" + $callerTier + " would raise every admission to a multi-round tier and deterministically break the single-round D1 tail (experiment_budget must equal one); unset it for non-probe runs")
    }
    Write-Warning ("ignoring VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET='" + $multiroundTierEnv + "': not an in-range tier, the agent fails closed to single-round anyway")
}

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$artifactDir = Join-Path $RepoRoot ("artifacts\free_state_d1_s1\" + $stamp)
New-Item -ItemType Directory -Path $artifactDir -Force | Out-Null
$report = Join-Path $artifactDir "d1_smoke_report.json"

# ---------------------------------------------------------------- LW helpers
# Tolerant JSON helpers for the opt-in -LoudnessWatch leg (off by default, so
# the default D1 flow is byte-for-byte unchanged). Tool-face responses carry
# their failure semantics in the JSON body, and Windows PowerShell 5.1 drains
# the Invoke-WebRequest error stream before catch blocks can read it
# (dev_agent_smoke / fs_largeproject_smoke Invoke-ToolJson precedent); the
# /agent/state polling must never throw the run out of its cleanup path.
function Get-D1JsonProperty {
    param($Object, [string]$Name)
    if ($null -eq $Object) { return $null }
    $adapted = $Object -as [System.Management.Automation.PSObject]
    if ($null -ne $adapted) {
        foreach ($prop in $adapted.PSObject.Properties) {
            if ($prop.Name -eq $Name) { return $prop.Value }
        }
    }
    return $null
}

function Invoke-D1JsonGet {
    param([string]$Uri, [int]$TimeoutSec = 10)
    try {
        $resp = Invoke-WebRequest -UseBasicParsing -Method GET -Uri $Uri -TimeoutSec $TimeoutSec
        if ([string]::IsNullOrWhiteSpace($resp.Content)) { return $null }
        return $resp.Content | ConvertFrom-Json
    }
    catch { return $null }
}

function Invoke-D1ToolJson {
    param([string]$Uri, [object]$Body, [int]$TimeoutSec = 30)
    Add-Type -AssemblyName System.Net.Http | Out-Null
    $client = New-Object System.Net.Http.HttpClient
    $client.Timeout = [TimeSpan]::FromSeconds($TimeoutSec)
    try {
        $json = $Body | ConvertTo-Json -Depth 20 -Compress
        $content = New-Object System.Net.Http.StringContent ($json, [System.Text.Encoding]::UTF8, "application/json")
        $post = $client.PostAsync($Uri, $content).GetAwaiter().GetResult()
        $text = $post.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        if ([string]::IsNullOrWhiteSpace($text)) { return $null }
        return $text | ConvertFrom-Json
    }
    catch { return $null }
    finally { $client.Dispose() }
}

function Save-D1JsonEvidence {
    param($Object, [string]$Path)
    if ($null -eq $Object -or [string]::IsNullOrWhiteSpace($Path)) { return }
    try { $Object | ConvertTo-Json -Depth 24 | Set-Content -LiteralPath $Path -Encoding UTF8 } catch { }
}

function Read-D1LwVerdictStore {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) { return $null }
    try {
        if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) { return $null }
        $text = [string](Get-Content -LiteralPath $Path -Raw -Encoding UTF8)
        if ([string]::IsNullOrWhiteSpace($text)) { return $null }
        return $text | ConvertFrom-Json
    }
    catch { return $null }
}

# /agent/state nests the live project binding (the python smoke resolves it
# with the same recursive walk, free_state_d1_smoke.py project_path_from_state):
# collect every project_path/current_project_path value so the caller can match
# the artifact project without assuming a top-level key shape.
function Get-D1StateProjectPaths {
    param($Node, $Accumulator)
    if ($null -eq $Node) { return }
    if ($Node -is [string]) { return }
    if ($Node -is [System.Management.Automation.PSCustomObject]) {
        foreach ($prop in $Node.PSObject.Properties) {
            if ($prop.Name -eq "project_path" -or $prop.Name -eq "current_project_path") {
                $value = [string]$prop.Value
                if (-not [string]::IsNullOrWhiteSpace($value) -and $value -ne "<nil>") { $Accumulator.Add($value) | Out-Null }
            }
        }
        foreach ($prop in $Node.PSObject.Properties) {
            Get-D1StateProjectPaths -Node $prop.Value -Accumulator $Accumulator
        }
        return
    }
    if ($Node -is [System.Collections.IEnumerable]) {
        foreach ($child in $Node) {
            Get-D1StateProjectPaths -Node $child -Accumulator $Accumulator
        }
    }
}

if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"
}
if (-not $SkipBuild) {
    $kernelListener = Get-NetTCPConnection -LocalPort 5555 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $kernelListener) {
        $kernelProcess = Get-CimInstance Win32_Process -Filter ("ProcessId=" + $kernelListener.OwningProcess) -ErrorAction SilentlyContinue
        if ($null -eq $kernelProcess -or $kernelProcess.Name -ne "VitApp.exe" -or -not ([string]$kernelProcess.ExecutablePath).StartsWith($RepoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
            throw "port 5555 is owned by a process outside the Vit-DAW workspace"
        }
        Stop-Process -Id $kernelListener.OwningProcess -Force
        Start-Sleep -Milliseconds 800
    }
    & cmake --build (Join-Path $RepoRoot "VitApp\build") --config Release --target VitApp --parallel 4
    if ($LASTEXITCODE -ne 0) {
        throw "current VitApp Release build failed"
    }
}
$KernelExe = (Resolve-Path -LiteralPath $KernelExe).Path

$devArgs = @(
    "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", (Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1"),
    "-RepoRoot", $RepoRoot, "-AgentHttp", $AgentHttp, "-RestartAgent", "-StartKernel", "-KernelExe", $KernelExe, "-StartUI",
    "-GodotProjectRoot", $GodotProjectRoot, "-NoChatSmoke", "-NoStripSilenceSmoke", "-WaitSeconds", ([string]$TimeoutSeconds),
    # Cold-start lesson: hand dev_agent_smoke an explicit >=90s UI gate instead
    # of leaving it on the 20s default (BATCH1 2026-10-17 §二 ③).
    "-UiPortWaitSeconds", ([string]$UiPortWaitSeconds)
)
if (-not [string]::IsNullOrWhiteSpace($GodotExe)) {
    $devArgs += @("-GodotExe", $GodotExe)
}
if ($SkipBuild) {
    $devArgs += "-SkipBuild"
}
& powershell @devArgs
if ($LASTEXITCODE -ne 0) {
    throw "real-stack startup failed with exit code $LASTEXITCODE"
}

$kernelListener = Get-NetTCPConnection -LocalPort 5555 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
if ($null -eq $kernelListener) {
    throw "real VitApp Kernel is not listening on port 5555"
}
$agentListener = Get-NetTCPConnection -LocalPort 7878 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
if ($null -eq $agentListener) {
    throw "real Go agent is not listening on port 7878"
}
$godot = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
    Where-Object { [string]$_.CommandLine -like ("*" + $GodotProjectRoot + "*") } | Select-Object -First 1
if ($null -eq $godot) {
    $godotCandidates = @($GodotExe, (Join-Path (Split-Path -Parent $GodotProjectRoot) "Godot_v4.6.1-stable_win64.exe"), "D:\Godot\Godot_v4.6.1-stable_win64.exe")
    $resolvedGodot = $godotCandidates | Where-Object { -not [string]::IsNullOrWhiteSpace($_) -and (Test-Path -LiteralPath $_ -PathType Leaf) } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace([string]$resolvedGodot)) {
        throw "real Godot frontend process is absent and no Godot executable could be resolved"
    }
    # The D1 smoke owns Kernel/agent startup. Keep the Godot fallback from
    # autostarting a competing local runtime; scope the setting to this child.
    $priorSkipDevAutostart = [Environment]::GetEnvironmentVariable("VIT_SKIP_DEV_AUTOSTART", "Process")
    $env:VIT_SKIP_DEV_AUTOSTART = "1"
    try {
        $godotProcess = Start-Process -FilePath $resolvedGodot -ArgumentList @("--path", $GodotProjectRoot) -WorkingDirectory $GodotProjectRoot -WindowStyle Hidden -PassThru
    }
    finally {
        if ($null -eq $priorSkipDevAutostart) {
            Remove-Item Env:VIT_SKIP_DEV_AUTOSTART -ErrorAction SilentlyContinue
        }
        else {
            $env:VIT_SKIP_DEV_AUTOSTART = $priorSkipDevAutostart
        }
    }
    Start-Sleep -Seconds 3
    if ($godotProcess.HasExited) {
        throw "real Godot frontend exited during D1 smoke startup"
    }
}

# ------------------------------------------- LW opt-in setup (default: skipped)
# Deterministic configure ordering (revision-leg run1 evidence): chat's
# handleInvoke preamble takes schedulerExecutionMu, so an /agent/invoke issued
# while the D1 python runner is mid-turn blocks behind the scheduler (run1
# 20261018_120243: the launcher's 30s configure timed out client-side while the
# request itself landed minutes later; T3 still caught it, but the ordering is
# not deterministic). The deterministic path is: reproduce the runner's
# materialize_public_case copy in the artifact dir, open that project while the
# agent is idle, configure the watch (the config persists against the live D1
# project binding), then run the python smoke against the already bound project
# with --reuse-existing-project. The watch hook is still built lazily on its
# first trigger (T3 project_change after the applied action / T1 settle).
$lwSetup = @{
    enabled              = [bool]$LoudnessWatch
    ui_port_wait_seconds = $UiPortWaitSeconds
    prepared             = $false
    project_open_ok      = $false
    project_path_seen    = ""
    project_poll_seconds = -1
    configure_ok         = $false
    config_digest        = ""
    config_error         = ""
    runner_stdout_log    = ""
    kernel_exe           = $KernelExe
    kernel_sha256        = ""
    agent_exe            = ""
    agent_sha256         = ""
}
if ($LoudnessWatch) {
    try { $lwSetup.kernel_sha256 = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash } catch { $lwSetup.kernel_sha256 = "unavailable" }
    $lwAgentExe = Join-Path $RepoRoot "agent\bin\VitAgent.exe"
    $lwSetup.agent_exe = $lwAgentExe
    try { $lwSetup.agent_sha256 = (Get-FileHash -LiteralPath $lwAgentExe -Algorithm SHA256).Hash } catch { $lwSetup.agent_sha256 = "unavailable" }
    $lwSetup | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $artifactDir "lw_binary_face.json") -Encoding UTF8
    Write-Host ("[lw] binary face: kernel=" + $lwSetup.kernel_sha256 + " agent=" + $lwSetup.agent_sha256)

    $lwStateUri = $AgentHttp.TrimEnd("/") + "/agent/state"
    $lwInvokeUri = $AgentHttp.TrimEnd("/") + "/agent/invoke"
    $lwProjectDir = Join-Path $artifactDir "project"
    try {
        $lwManifest = Get-Content -LiteralPath $PublicManifest -Raw | ConvertFrom-Json
        $lwCase = @($lwManifest.cases) | Where-Object { [string]$_.public_case_id -eq $PublicCaseId } | Select-Object -First 1
        if ($null -eq $lwCase -or [string]::IsNullOrWhiteSpace([string]$lwCase.project_path)) {
            $lwSetup.config_error = "public case " + $PublicCaseId + " not found in the manifest; watch not configured"
        }
        else {
            $lwSourceProject = [System.IO.Path]::GetFullPath([string]$lwCase.project_path)
            $lwSourceDir = Split-Path -Parent $lwSourceProject
            $lwTargetProject = Join-Path $lwProjectDir (Split-Path -Leaf $lwSourceProject)
            # Reproduce free_state_d1_smoke.py materialize_public_case exactly:
            # copytree of the case project's parent dir, .vit_history ignored.
            New-Item -ItemType Directory -Path $lwProjectDir -Force | Out-Null
            Get-ChildItem -LiteralPath $lwSourceDir -Force | Where-Object { $_.Name -ne ".vit_history" } | ForEach-Object {
                Copy-Item -LiteralPath $_.FullName -Destination $lwProjectDir -Recurse -Force
            }
            if (-not (Test-Path -LiteralPath $lwTargetProject -PathType Leaf)) {
                $lwSetup.config_error = "materialized D1 project missing at " + $lwTargetProject
            }
            else {
                $lwOpenResp = Invoke-D1ToolJson -Uri $lwInvokeUri -TimeoutSec 180 -Body @{
                    tool      = "project.open"
                    args      = @{ file_path = $lwTargetProject; project_path = $lwTargetProject }
                    confirmed = $true
                    source    = "free_state_d1_smoke"
                }
                Save-D1JsonEvidence -Object $lwOpenResp -Path (Join-Path $artifactDir "lw_project_open.json")
                $lwProjectNormalized = ([System.IO.Path]::GetFullPath($lwProjectDir)).TrimEnd([char[]]@('\', '/'))
                $lwWaitStarted = Get-Date
                $lwWaitDeadline = $lwWaitStarted.AddSeconds(120)
                while ((Get-Date) -lt $lwWaitDeadline) {
                    $lwState = Invoke-D1JsonGet -Uri $lwStateUri -TimeoutSec 10
                    if ($null -ne $lwState) {
                        $lwStatePaths = New-Object System.Collections.Generic.List[string]
                        Get-D1StateProjectPaths -Node $lwState -Accumulator $lwStatePaths
                        foreach ($lwRawPath in $lwStatePaths) {
                            try { $lwCandidate = [System.IO.Path]::GetFullPath($lwRawPath) } catch { $lwCandidate = $lwRawPath }
                            $lwCandidate = (($lwCandidate -replace '/', '\').TrimEnd([char[]]@('\', '/')))
                            if ($lwCandidate.StartsWith($lwProjectNormalized, [System.StringComparison]::OrdinalIgnoreCase)) {
                                $lwSetup.project_path_seen = $lwCandidate
                                break
                            }
                        }
                    }
                    if (-not [string]::IsNullOrWhiteSpace($lwSetup.project_path_seen)) { break }
                    Start-Sleep -Seconds 1
                }
                $lwSetup.project_poll_seconds = [int]((Get-Date) - $lwWaitStarted).TotalSeconds
                if ([string]::IsNullOrWhiteSpace($lwSetup.project_path_seen)) {
                    $lwSetup.config_error = "the materialized D1 project was not the live agent binding within 120s; watch not configured"
                }
                else {
                    $lwSetup.project_open_ok = $true
                    Write-Host ("[lw] D1 project binding live after " + [string]$lwSetup.project_poll_seconds + "s: " + $lwSetup.project_path_seen)
                    $lwConfigureBody = @{
                        tool      = "loudness_watch_configure"
                        args      = @{ enabled = $true; profile_source = "builtin:ebu_r128" }
                        confirmed = $true
                        source    = "free_state_d1_smoke"
                    }
                    for ($lwAttempt = 1; $lwAttempt -le 2; $lwAttempt++) {
                        $lwConfigureResp = Invoke-D1ToolJson -Uri $lwInvokeUri -Body $lwConfigureBody -TimeoutSec 90
                        Save-D1JsonEvidence -Object $lwConfigureResp -Path (Join-Path $artifactDir "loudness_watch_configure.json")
                        if ($null -ne $lwConfigureResp) {
                            $lwCfgResult = Get-D1JsonProperty -Object $lwConfigureResp -Name "result"
                            $lwCfgOk = ((([string](Get-D1JsonProperty -Object $lwConfigureResp -Name "status")) -eq "ok") -and ($null -ne $lwCfgResult))
                            if ($lwCfgOk) {
                                $lwSetup.configure_ok = $true
                                $lwSetup.config_digest = [string](Get-D1JsonProperty -Object $lwCfgResult -Name "config_digest")
                                break
                            }
                            else {
                                $lwSetup.config_error = [string](Get-D1JsonProperty -Object $lwConfigureResp -Name "error")
                            }
                        }
                        else {
                            $lwSetup.config_error = "no response from loudness_watch_configure"
                        }
                        Start-Sleep -Seconds 2
                    }
                }
            }
        }
    }
    catch {
        $lwSetup.config_error = "LW pre-open/configure failed: " + $_.Exception.Message
    }
    if ($lwSetup.configure_ok) {
        $lwSetup.prepared = $true
        $lwDigestShort = $lwSetup.config_digest
        if ($lwDigestShort.Length -gt 12) { $lwDigestShort = $lwDigestShort.Substring(0, 12) + "..." }
        Write-Host ("[lw] watch configured before the run (digest=" + $lwDigestShort + "); runner will reuse the bound project") -ForegroundColor Green
    }
    else {
        Write-Host ("[lw] configure FAILED/skipped: " + $lwSetup.config_error + "; the LW gates will fail honestly") -ForegroundColor Red
        # A partially materialized project dir would make the python runner
        # refuse to materialize a fresh copy; drop it so the default flow keeps
        # its own fresh-copy semantics (gates then fail on the missing hook).
        if (Test-Path -LiteralPath $lwProjectDir) {
            try { Remove-Item -LiteralPath $lwProjectDir -Recurse -Force } catch { }
        }
    }
    $lwSetup | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $artifactDir "lw_prepare.json") -Encoding UTF8
}
$runnerStdoutLog = ""

try {
    $smokeArgs = @(
        (Join-Path $RepoRoot "scripts\free_state_d1_smoke.py"),
        "--public-manifest", $PublicManifest, "--public-case-id", $PublicCaseId,
        "--agent-http", $AgentHttp, "--timeout-sec", ([string]$TimeoutSeconds),
        "--project-workdir", (Join-Path $artifactDir "project"), "--output", $report
    )
    if ($AdmissionOnly) {
        $smokeArgs += "--admission-only"
    }
    if ($SettlementProbe -ne "") {
        $smokeArgs += @("--settlement-probe", $SettlementProbe)
    }
    if ($ContinuationAdoptionProbe) {
        $smokeArgs += "--continuation-adoption-probe"
    }
    if ($StopSemanticsProbe) {
        # The probe asserts the stop request-arrival anchor line, so it needs
        # the agent log path the way the processor-selection check does.
        $smokeArgs += @("--stop-semantics-probe", "--agent-log", $agentLogPath)
    }
    if ($ImmediateStopProbe) {
        # FIX-TURNSTOP-WEDGE-1: the probe asserts the stop request-arrival
        # anchor line (and records the settle-window fallback anchor), so it
        # needs the agent log path the same way.
        $smokeArgs += @("--immediate-stop-probe", "--agent-log", $agentLogPath)
    }
    if ($PromptFlavor -ne "neutral") {
        $smokeArgs += @("--prompt-flavor", $PromptFlavor)
    }
    if ($ExpectDomain -ne "any") {
        $smokeArgs += @("--expect-domain", $ExpectDomain)
    }
    if ($MultiRoundProbe) {
        # TODO(D2-2-S2): the deterministic multi-round drive (gear-on + dose
        # calibration scenario injection) is added here once the D2-2-S2
        # parameter-injection channel design merges.
        $smokeArgs += "--multi-round-probe"
    }
    if ($ExpectHonestRefusal) {
        $smokeArgs += "--expect-honest-refusal"
    }
    if ($SealedTargetCheck) {
        $smokeArgs += @("--sealed-truth", $sealedTruthPath)
    }
    if ($ExpectProcessorSelection -ne "") {
        $smokeArgs += @("--agent-log", $agentLogPath, "--expect-processor-selection", $ExpectProcessorSelection)
    }
    if ($LoudnessWatch) {
        # ------------------- LW: run the python smoke on the pre-bound project
        # The watch was configured before the run (deterministic ordering); the
        # runner now reuses the already bound project so prepare_project does
        # not reopen/replace the runtime state (a reopen would restore the
        # project's persisted config record and drop the in-memory one).
        $runnerStdoutLog = Join-Path $artifactDir "runner_stdout.log"
        $lwSetup.runner_stdout_log = $runnerStdoutLog
        if ($lwSetup.prepared) { $smokeArgs += "--reuse-existing-project" }
        Write-Host ("[lw] running the D1 python runner (reuse_existing_project=" + [string]$lwSetup.prepared + ", log=" + $runnerStdoutLog + ")")
        $lwRunnerLines = & python @smokeArgs 2>&1
        $runnerExit = $LASTEXITCODE
        $lwRunnerLines | Tee-Object -FilePath $runnerStdoutLog | Out-Host
        Write-Host ("[lw] runner exit=" + [string]$runnerExit)
    }
    else {
        & python @smokeArgs
        $runnerExit = $LASTEXITCODE
    }
}
finally {
    $agentLog = Join-Path $RepoRoot "VitApp\Workspace\Logs\agent_last.log"
    if (Test-Path -LiteralPath $agentLog -PathType Leaf) {
        Copy-Item -LiteralPath $agentLog -Destination (Join-Path $artifactDir "agent_last.log") -Force
    }
}
if ($runnerExit -eq 3) {
    # MRREG1 (2026-08-31): print the report's own NOT_EXERCISED reason; the old
    # fixed "did not autonomously select ..." wording misdescribed the
    # multi-round budget form and misled the 2026-08-30 nightly triage.
    $notExercisedReason = ""
    try {
        $notExercisedReason = [string]((Get-Content -LiteralPath $report -Raw) | ConvertFrom-Json).reason
    }
    catch {
    }
    if ([string]::IsNullOrWhiteSpace($notExercisedReason)) {
        $notExercisedReason = ($PublicCaseId + " did not autonomously select the expected admitted domain (expect=" + $ExpectDomain + ")")
    }
    Write-Host ("D1-S1 NOT_EXERCISED: " + $notExercisedReason + "; report=" + $report) -ForegroundColor Yellow
    exit 3
}
if ($runnerExit -ne 0) {
    throw "D1-S1 smoke failed; report=$report"
}
# ------------------- LW post-run gates (opt-in; only after runner exit 0)
# LOUDNESS-B-TRIGGER revision leg (BATCH1 2026-10-17 §二 案①): the D1 settle
# leg is the deterministic trigger bed for the watch hook. All gates are
# deterministic /agent/invoke calls (no LLM) and never assert fixture audio
# values (the verdict kind is fixture-dependent and stays unasserted):
#   1. observe #1 -> additive loudness_watch disclosure with a non-empty
#      verdicts_path (empty path = immediate FAIL, no deadline spin);
#   2. verdict store on disk with the pinned schema and at least one
#      settle/project_change trigger record (T1/T3 wiring);
#   3. observe #2 -> checks_total advances, record count stays stable and the
#      dedupe keys stay unique (same revision+kind rechecks, never duplicates);
#   4. configure off -> observe #3 -> checks_total does not move (silent zero
#      cost); A4 fixed-orchestration marker scan; E1 cost extraction.
if ($LoudnessWatch) {
    Write-Host ""
    Write-Host "== Phase LW: loudness watch post-settle gates ==" -ForegroundColor Cyan
    $lwInvokeUri = $AgentHttp.TrimEnd("/") + "/agent/invoke"
    $lwPost = @{
        enabled                = $true
        disclosure_present     = $false
        verdicts_path          = ""
        verdict_file_exists    = $false
        verdict_schema_ok      = $false
        verdict_schema_version = ""
        round_trigger_record   = $false
        round_trigger_kinds    = @()
        records_after_run      = -1
        checks_before          = -1
        checks_after           = -1
        records_before         = -1
        records_after          = -1
        checks_after_off       = -1
        dedupe_stable          = $false
        keys_unique            = $false
        config_off_silent      = $false
        observe_files          = @()
        marker_hits            = @()
        e1_extract_file        = ""
        e1_output              = @()
        e1_settle_sample_count = -1
        pre_observe_snapshot   = ""
    }
    $lwFailures = New-Object System.Collections.Generic.List[string]
    $lwFixedMarkers = @(
        "semantic_eq_batch",
        "eq_plugin_load_batch",
        "dynamic_plugin_load_batch",
        "dynamic_parameter_batch",
        "c2_dynamic_plugin_selection",
        "c2.dynamic_plugin_load.governed"
    )
    # Race guard: the first post-run observe itself fires a T2 piggyback check,
    # and the dedupe path relabels a matched record's trigger_kind to the latest
    # trigger. Snapshot the store the runner left behind BEFORE that observe so
    # the round-borne trigger evidence (T1 settle / T3 project_change) cannot be
    # relabelled out from under the gate. The snapshot is discovered on disk
    # because the disclosure path is only known after observe #1.
    $lwPreObservePath = Join-Path $artifactDir "loudness_watch_verdicts_preobserve.json"
    try {
        $lwFoundStore = @(Get-ChildItem -LiteralPath (Join-Path $artifactDir "project") -Filter "loudness_watch_verdicts.json" -File -Recurse -ErrorAction SilentlyContinue) | Select-Object -First 1
        if ($null -ne $lwFoundStore) {
            Copy-Item -LiteralPath $lwFoundStore.FullName -Destination $lwPreObservePath -Force
            $lwPost.pre_observe_snapshot = $lwPreObservePath
        }
    }
    catch { }
    # Gate 1: the observation carrier discloses the watch status additively.
    Write-Host "[lw] gate 1: observe #1 mix_request_observation scope=full_project"
    $lwObs1 = Invoke-D1ToolJson -Uri $lwInvokeUri -Body @{
        tool      = "mix_request_observation"
        args      = @{ scope = "full_project" }
        confirmed = $true
        source    = "free_state_d1_smoke"
    } -TimeoutSec 180
    $lwObs1File = Join-Path $artifactDir "loudness_watch_observe1.json"
    Save-D1JsonEvidence -Object $lwObs1 -Path $lwObs1File
    $lwPost.observe_files += $lwObs1File
    $lwObs1Result = Get-D1JsonProperty -Object $lwObs1 -Name "result"
    $lwDisclosure = Get-D1JsonProperty -Object $lwObs1Result -Name "loudness_watch"
    if ($null -ne $lwDisclosure) {
        $lwPost.disclosure_present = $true
        $lwPost.verdicts_path = [string](Get-D1JsonProperty -Object $lwDisclosure -Name "verdicts_path")
    }
    if ($lwPost.disclosure_present) { Write-Host "   [ok] observe #1 carries the loudness_watch disclosure" -ForegroundColor Green }
    else {
        $lwFailures.Add("LW FAIL: loudness_watch disclosure missing from the mix_request_observation response (watch hook never registered: no settle/project_change trigger fired during the run)")
        Write-Host ("   [FAIL] " + $lwFailures[$lwFailures.Count - 1]) -ForegroundColor Red
    }
    # Empty-path guard: must FAIL fast and must never fall through into three
    # 30s spin loops or a Test-Path on "" (the run3
    # fs_largeproject_smoke_20261017_193105 script_exception).
    if ([string]::IsNullOrWhiteSpace($lwPost.verdicts_path)) {
        $lwFailures.Add("LW FAIL: loudness_watch verdicts_path is empty; verdict-store/dedupe/config-off gates skipped by the empty-path guard (no deadline spin)")
        Write-Host ("   [FAIL] " + $lwFailures[$lwFailures.Count - 1]) -ForegroundColor Red
    }
    else {
        # Gate 2: verdict store on disk, pinned schema, round-borne trigger.
        Write-Host "[lw] gate 2: verdict store on disk (pinned schema + round-borne trigger record)"
        $lwStoreForRound = $null
        if (-not [string]::IsNullOrWhiteSpace($lwPost.pre_observe_snapshot)) {
            $lwStoreForRound = Read-D1LwVerdictStore -Path $lwPost.pre_observe_snapshot
        }
        if ($null -eq $lwStoreForRound) {
            $lwStoreForRound = Read-D1LwVerdictStore -Path $lwPost.verdicts_path
        }
        $lwFileDeadline = (Get-Date).AddSeconds(45)
        while ($null -eq $lwStoreForRound -and (Get-Date) -lt $lwFileDeadline) {
            $lwStoreForRound = Read-D1LwVerdictStore -Path $lwPost.verdicts_path
            if ($null -eq $lwStoreForRound) { Start-Sleep -Seconds 2 }
        }
        if ($null -eq $lwStoreForRound) {
            $lwFailures.Add("LW FAIL: verdict store file missing at " + $lwPost.verdicts_path)
            Write-Host ("   [FAIL] " + $lwFailures[$lwFailures.Count - 1]) -ForegroundColor Red
        }
        else {
            $lwPost.verdict_file_exists = (Test-Path -LiteralPath $lwPost.verdicts_path -PathType Leaf)
            $lwPost.verdict_schema_version = [string](Get-D1JsonProperty -Object $lwStoreForRound -Name "schema_version")
            $lwPost.verdict_schema_ok = ($lwPost.verdict_schema_version -eq "loudness_watch_verdicts.v0")
            $lwVerdictsRaw = @()
            $lwRawVerdicts = Get-D1JsonProperty -Object $lwStoreForRound -Name "verdicts"
            if ($null -ne $lwRawVerdicts) { $lwVerdictsRaw = @($lwRawVerdicts) }
            $lwPost.records_after_run = $lwVerdictsRaw.Count
            foreach ($lwRow in $lwVerdictsRaw) {
                $lwRowKind = [string](Get-D1JsonProperty -Object $lwRow -Name "trigger_kind")
                if ($lwRowKind -eq "settle" -or $lwRowKind -eq "project_change") {
                    $lwPost.round_trigger_kinds += $lwRowKind
                }
            }
            $lwPost.round_trigger_record = ($lwPost.round_trigger_kinds.Count -ge 1)
            if ($lwPost.verdict_schema_ok -and $lwPost.round_trigger_record) {
                Write-Host ("   [ok] verdict store schema pinned, round-trigger record(s)=" + ($lwPost.round_trigger_kinds -join ",") + " (records=" + [string]$lwPost.records_after_run + ")") -ForegroundColor Green
            }
            else {
                $lwFailures.Add("LW FAIL: verdict store missing, foreign schema, or no settle/project_change record (schema=" + $lwPost.verdict_schema_version + ", records=" + [string]$lwPost.records_after_run + ")")
                Write-Host ("   [FAIL] " + $lwFailures[$lwFailures.Count - 1]) -ForegroundColor Red
            }
            # Gate 3: dedupe. Baseline waits for the observe #1 piggyback check
            # to land and the counter to go stable (two equal reads 1s apart).
            Write-Host "[lw] gate 3: dedupe (observe #2 advances checks_total, record count stable, keys unique)"
            $lwStableDeadline = (Get-Date).AddSeconds(45)
            while ((Get-Date) -lt $lwStableDeadline) {
                $lwStoreA = Read-D1LwVerdictStore -Path $lwPost.verdicts_path
                if ($null -ne $lwStoreA) {
                    Start-Sleep -Seconds 1
                    $lwStoreB = Read-D1LwVerdictStore -Path $lwPost.verdicts_path
                    if ($null -ne $lwStoreB) {
                        $lwChecksA = Get-D1JsonProperty -Object $lwStoreA -Name "checks_total"
                        $lwChecksB = Get-D1JsonProperty -Object $lwStoreB -Name "checks_total"
                        if ($null -ne $lwChecksA -and $null -ne $lwChecksB -and ([long]$lwChecksA -eq [long]$lwChecksB)) {
                            $lwPost.checks_before = [long]$lwChecksB
                            $lwRecordsB = Get-D1JsonProperty -Object $lwStoreB -Name "verdicts"
                            if ($null -eq $lwRecordsB) { $lwPost.records_before = 0 } else { $lwPost.records_before = @($lwRecordsB).Count }
                            break
                        }
                    }
                }
                Start-Sleep -Seconds 1
            }
            $lwObs2 = Invoke-D1ToolJson -Uri $lwInvokeUri -Body @{
                tool      = "mix_request_observation"
                args      = @{ scope = "full_project" }
                confirmed = $true
                source    = "free_state_d1_smoke"
            } -TimeoutSec 180
            $lwObs2File = Join-Path $artifactDir "loudness_watch_observe2.json"
            Save-D1JsonEvidence -Object $lwObs2 -Path $lwObs2File
            $lwPost.observe_files += $lwObs2File
            $lwDedupeDeadline = (Get-Date).AddSeconds(45)
            while ((Get-Date) -lt $lwDedupeDeadline -and $lwPost.checks_before -ge 0) {
                $lwStoreC = Read-D1LwVerdictStore -Path $lwPost.verdicts_path
                if ($null -ne $lwStoreC) {
                    $lwChecksC = Get-D1JsonProperty -Object $lwStoreC -Name "checks_total"
                    if ($null -ne $lwChecksC -and ([long]$lwChecksC -gt $lwPost.checks_before)) {
                        $lwPost.checks_after = [long]$lwChecksC
                        $lwRecordsC = Get-D1JsonProperty -Object $lwStoreC -Name "verdicts"
                        if ($null -eq $lwRecordsC) { $lwPost.records_after = 0 } else { $lwPost.records_after = @($lwRecordsC).Count }
                        break
                    }
                }
                Start-Sleep -Seconds 1
            }
            if ($lwPost.checks_after -gt $lwPost.checks_before -and $lwPost.checks_before -ge 0) {
                $lwPost.dedupe_stable = ($lwPost.records_after -eq $lwPost.records_before)
                $lwKeys = @()
                $lwStoreFinal = Read-D1LwVerdictStore -Path $lwPost.verdicts_path
                if ($null -ne $lwStoreFinal) {
                    $lwRecordsFinal = Get-D1JsonProperty -Object $lwStoreFinal -Name "verdicts"
                    if ($null -ne $lwRecordsFinal) {
                        foreach ($lwRec in @($lwRecordsFinal)) { $lwKeys += [string](Get-D1JsonProperty -Object $lwRec -Name "key") }
                    }
                }
                $lwPost.keys_unique = ($lwKeys.Count -eq @($lwKeys | Select-Object -Unique).Count)
            }
            if ($lwPost.dedupe_stable -and $lwPost.keys_unique) {
                Write-Host ("   [ok] dedupe held (checks " + [string]$lwPost.checks_before + "->" + [string]$lwPost.checks_after + ", records " + [string]$lwPost.records_before + "=" + [string]$lwPost.records_after + ", keys unique)") -ForegroundColor Green
            }
            else {
                $lwFailures.Add("LW FAIL: dedupe broken (checks " + [string]$lwPost.checks_before + "->" + [string]$lwPost.checks_after + ", records " + [string]$lwPost.records_before + "->" + [string]$lwPost.records_after + ", keys_unique=" + [string]$lwPost.keys_unique + ")")
                Write-Host ("   [FAIL] " + $lwFailures[$lwFailures.Count - 1]) -ForegroundColor Red
            }
            # Gate 4: disabled watch is silent zero cost.
            Write-Host "[lw] gate 4: configure off -> observe #3 must not advance checks_total (silent zero cost)"
            $lwOff = Invoke-D1ToolJson -Uri $lwInvokeUri -Body @{
                tool      = "loudness_watch_configure"
                args      = @{ enabled = $false }
                confirmed = $true
                source    = "free_state_d1_smoke"
            } -TimeoutSec 30
            Save-D1JsonEvidence -Object $lwOff -Path (Join-Path $artifactDir "loudness_watch_configure_off.json")
            $lwObs3 = Invoke-D1ToolJson -Uri $lwInvokeUri -Body @{
                tool      = "mix_request_observation"
                args      = @{ scope = "full_project" }
                confirmed = $true
                source    = "free_state_d1_smoke"
            } -TimeoutSec 180
            $lwObs3File = Join-Path $artifactDir "loudness_watch_observe3_disabled.json"
            Save-D1JsonEvidence -Object $lwObs3 -Path $lwObs3File
            $lwPost.observe_files += $lwObs3File
            Start-Sleep -Seconds 3
            $lwStoreD = Read-D1LwVerdictStore -Path $lwPost.verdicts_path
            if ($null -ne $lwStoreD -and $lwPost.checks_after -ge 0) {
                $lwChecksD = Get-D1JsonProperty -Object $lwStoreD -Name "checks_total"
                if ($null -ne $lwChecksD) {
                    $lwPost.checks_after_off = [long]$lwChecksD
                    $lwPost.config_off_silent = ([long]$lwChecksD -eq $lwPost.checks_after)
                }
            }
            if ($lwPost.config_off_silent) { Write-Host ("   [ok] config-off silent (checks_total stays " + [string]$lwPost.checks_after + ")") -ForegroundColor Green }
            else {
                $lwFailures.Add("LW FAIL: disabled watch still moved checks_total (" + [string]$lwPost.checks_after + "->" + [string]$lwPost.checks_after_off + ")")
                Write-Host ("   [FAIL] " + $lwFailures[$lwFailures.Count - 1]) -ForegroundColor Red
            }
            # Evidence: the verdict store snapshot rides the artifact root; E1
            # extraction runs over that copy.
            if (Test-Path -LiteralPath $lwPost.verdicts_path -PathType Leaf) {
                Copy-Item -LiteralPath $lwPost.verdicts_path -Destination (Join-Path $artifactDir "loudness_watch_verdicts.json") -Force
            }
            $lwVerdictCopy = Join-Path $artifactDir "loudness_watch_verdicts.json"
            $lwE1Script = Join-Path $RepoRoot "coord\runs\LOUDNESS-B-TRIGGER-1\extract_e1.py"
            if ((Test-Path -LiteralPath $lwVerdictCopy -PathType Leaf) -and (Test-Path -LiteralPath $lwE1Script -PathType Leaf)) {
                Write-Host "[lw] E1 extraction: extract_e1.py <verdict store copy>"
                $lwPost.e1_extract_file = Join-Path $artifactDir "e1_extract.txt"
                $lwE1Lines = @()
                $lwE1Exit = 1
                try {
                    $lwE1Lines = @(& python $lwE1Script $lwVerdictCopy 2>&1 | ForEach-Object { [string]$_ })
                    $lwE1Exit = $LASTEXITCODE
                }
                catch { $lwE1Exit = 1 }
                $lwE1Text = ($lwE1Lines -join [Environment]::NewLine)
                try { [System.IO.File]::WriteAllText($lwPost.e1_extract_file, $lwE1Text + [Environment]::NewLine, (New-Object System.Text.UTF8Encoding($false))) } catch { }
                $lwPost.e1_output = $lwE1Lines
                if ($lwE1Exit -eq 0) {
                    $lwE1Match = [regex]::Match($lwE1Text, '\[settle/project_change\] n=(\d+)')
                    if ($lwE1Match.Success) { $lwPost.e1_settle_sample_count = [int]$lwE1Match.Groups[1].Value }
                    if ($lwPost.e1_settle_sample_count -ge 1) {
                        Write-Host ("   [ok] E1 extract: settle/project_change samples=" + [string]$lwPost.e1_settle_sample_count) -ForegroundColor Green
                    }
                    else { $lwFailures.Add("LW FAIL: E1 extraction carries no settle/project_change sample row") }
                }
                else {
                    $lwFailures.Add("LW FAIL: extract_e1.py exited " + [string]$lwE1Exit)
                    Write-Host ("   [FAIL] " + $lwFailures[$lwFailures.Count - 1]) -ForegroundColor Red
                }
            }
            else {
                $lwFailures.Add("LW FAIL: E1 extraction inputs missing (verdict copy or extract_e1.py)")
                Write-Host ("   [FAIL] " + $lwFailures[$lwFailures.Count - 1]) -ForegroundColor Red
            }
        }
        # A4 discipline: every LW leg response surface stays clean of the
        # fixed-orchestration marker families.
        $lwAuditSurfaces = @($lwPost.observe_files)
        foreach ($lwCfgSurface in @((Join-Path $artifactDir "loudness_watch_configure.json"), (Join-Path $artifactDir "loudness_watch_configure_off.json"))) {
            if (Test-Path -LiteralPath $lwCfgSurface -PathType Leaf) { $lwAuditSurfaces += $lwCfgSurface }
        }
        $lwHits = New-Object System.Collections.Generic.List[object]
        foreach ($lwSurface in $lwAuditSurfaces) {
            if ([string]::IsNullOrWhiteSpace([string]$lwSurface) -or -not (Test-Path -LiteralPath $lwSurface -PathType Leaf)) { continue }
            $lwText = ""
            try { $lwText = [string](Get-Content -LiteralPath $lwSurface -Raw -Encoding UTF8) } catch { $lwText = "" }
            if ([string]::IsNullOrWhiteSpace($lwText)) { continue }
            foreach ($lwMarker in $lwFixedMarkers) {
                if ($lwText.Contains($lwMarker)) { $lwHits.Add([pscustomobject]@{ marker = $lwMarker; surface = $lwSurface }) }
            }
        }
        $lwPost.marker_hits = @($lwHits.ToArray())
        if ($lwHits.Count -eq 0) { Write-Host "   [ok] LW response surfaces clean of fixed-orchestration markers" -ForegroundColor Green }
        else {
            foreach ($lwHit in @($lwPost.marker_hits)) { $lwFailures.Add("LW FAIL: marker '" + $lwHit.marker + "' hit on " + $lwHit.surface) }
            Write-Host ("   [FAIL] fixed-orchestration marker hit(s): " + [string]$lwHits.Count) -ForegroundColor Red
        }
    }
    $lwGatesPath = Join-Path $artifactDir "loudness_watch_gates.json"
    $lwGatesReport = @{
        schema_version = "loudness_watch_d1_gates.v0"
        run_stamp      = $stamp
        runner_exit    = $runnerExit
        setup          = $lwSetup
        post           = $lwPost
        a4_markers     = $lwFixedMarkers
        failures       = @($lwFailures.ToArray())
        gate_results   = @{
            disclosure                  = $lwPost.disclosure_present
            verdict_store_round_trigger = ($lwPost.verdict_schema_ok -and $lwPost.round_trigger_record)
            dedupe                      = ($lwPost.dedupe_stable -and $lwPost.keys_unique)
            config_off_silent           = $lwPost.config_off_silent
            a4_zero_markers             = (@($lwPost.marker_hits).Count -eq 0)
            e1_samples                  = $lwPost.e1_settle_sample_count
        }
    }
    try { $lwGatesReport | ConvertTo-Json -Depth 16 | Set-Content -LiteralPath $lwGatesPath -Encoding UTF8 } catch { }
    if ($lwFailures.Count -gt 0) {
        throw ("LOUDNESS-WATCH gates failed (" + [string]$lwFailures.Count + " failure(s)); evidence=" + $lwGatesPath)
    }
    Write-Host ("[lw] PASS: disclosure + verdict store/round trigger + dedupe + config-off silence + E1 all green; gates=" + $lwGatesPath) -ForegroundColor Green
}
if ($AdmissionOnly) {
    Write-Host ("D1-S1 ADMISSION_ONLY PASS: real-stack proposal/admission smoke completed without Apply; report=" + $report) -ForegroundColor Green
    exit 0
}
if ($SettlementProbe -ne "") {
    # Restart-consistency phase: settle first, then restart only the agent on
    # the same workspace and re-verify the persisted settled projection.
    Write-Host ("D1-S1 SETTLEMENT(" + $SettlementProbe + ") PASS: settled; restarting agent for restart verification") -ForegroundColor Green
    & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $RepoRoot "scripts\dev_agent_smoke.ps1") `
        -RepoRoot $RepoRoot -AgentHttp $AgentHttp -RestartAgent -SkipBuild -WaitSeconds ([string]$TimeoutSeconds)
    if ($LASTEXITCODE -ne 0) {
        throw "agent restart for settlement verification failed with exit code $LASTEXITCODE"
    }
    & python (Join-Path $RepoRoot "scripts\free_state_d1_smoke.py") `
        --public-manifest $PublicManifest --public-case-id $PublicCaseId `
        --agent-http $AgentHttp --timeout-sec ([string]$TimeoutSeconds) `
        --project-workdir (Join-Path $artifactDir "project") --output $report `
        --verify-settled $report
    if ($LASTEXITCODE -ne 0) {
        throw "D1-S1 settlement restart verification failed; report=$report"
    }
    Write-Host ("D1-S1 SETTLEMENT(" + $SettlementProbe + ") PASS: real-stack settlement + restart consistency verified; report=" + $report) -ForegroundColor Green
    exit 0
}
if ($ExpectHonestRefusal) {
    Write-Host ("D1-S1 HONEST_REFUSAL PASS: fail-closed refusal verified honest (four requirements, no stale misattribution); report=" + $report) -ForegroundColor Green
    exit 0
}
if ($ImmediateStopProbe) {
    Write-Host ("D1-S1 IMMEDIATE_STOP PASS: real-stack ask-then-immediate-stop converged goal/task/slice, second stop idempotent, conversation can start a new turn; report=" + $report) -ForegroundColor Green
    exit 0
}
if ($ExpectProcessorSelection -ne "") {
    Write-Host ("D1-S1 PASS: processor_selection route walked end to end (domain=" + $ExpectProcessorSelection + "; selection record + routing log verified); report=" + $report) -ForegroundColor Green
    exit 0
}
Write-Host ("D1-S1 PASS: real-stack public-only smoke completed; report=" + $report) -ForegroundColor Green

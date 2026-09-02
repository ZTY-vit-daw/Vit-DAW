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
    [ValidateSet("neutral", "frequency", "compression", "leveling", "sibilance", "transient", "pan", "limiter", "gate")]
    [string]$PromptFlavor = "neutral",
    [ValidateSet("any", "track_gain", "static_eq", "broadband_compression", "de_esser", "transient_shaper", "pan", "limiter", "gate_expander")]
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
    [int]$MultiRoundBudget = 2
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
$multiroundTierEnv = [string]$env:VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET
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
    "-GodotProjectRoot", $GodotProjectRoot, "-NoChatSmoke", "-NoStripSilenceSmoke", "-WaitSeconds", ([string]$TimeoutSeconds)
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
    & python @smokeArgs
    $runnerExit = $LASTEXITCODE
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
Write-Host ("D1-S1 PASS: real-stack public-only smoke completed; report=" + $report) -ForegroundColor Green

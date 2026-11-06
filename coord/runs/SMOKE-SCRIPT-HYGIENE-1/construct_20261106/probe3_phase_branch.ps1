# probe3_phase_branch.ps1 — SMOKE-SCRIPT-HYGIENE-1 (260) item (4) constructed verification.
#
# Zero real stack, zero LLM. Each leg gets a fresh stub repo (dev_agent_smoke
# + D1 runner + agent HTTP face stubs) and runs the D1 launcher against it:
#
#   leg 1  post-fix, VIT260_STUB_BRANCH=zero_mutation, -LoudnessWatch -D1LoudnessSettle
#          -> exit 0; loudness_watch_gates.json AND d1_settle_gates.json both
#             written as skip-with-reason faces (phase_reachable=false, every
#             gate "skipped_branch_unreachable", failures empty); run reaches
#             the D1-S1 PASS tail. (The 258 verifyrun1 form, now branchified.)
#   leg 2  post-fix, applied branch, -LoudnessWatch
#          -> exit 1; LW gates RUN and fail honestly against the
#             no-disclosure stub (gate semantics unchanged on 1-mutation).
#   leg 3  post-fix, applied branch, -D1LoudnessSettle
#          -> exit 1; settle gates RUN and fail honestly (A/B/D red, C ok).
#   leg 4  pre-fix, zero_mutation, -LoudnessWatch -D1LoudnessSettle
#          -> exit 1; LW gates fail with NO branch awareness and the throw
#             cuts the run: d1_settle_gates.json is ABSENT (258 form).
param(
    [string]$ArtifactDir,
    [string]$StubsDir,
    [string]$PreFixScript,
    [string]$PostFixScript
)

$ErrorActionPreference = 'Stop'
$tempBase = Join-Path ([System.IO.Path]::GetTempPath()) ('vit260_probe3_' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tempBase -Force | Out-Null

foreach ($port in @(5555, 5556, 5557, 7878, 8787, 4445)) {
    $busy = @(Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue)
    if ($busy.Count -gt 0) { Write-Host ("ENV-BLOCKED: port $port already listening"); exit 3 }
}
if (-not [string]::IsNullOrWhiteSpace($env:VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET)) { throw "VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET is set; unset it for this probe" }
if (-not [string]::IsNullOrWhiteSpace($env:VIT_AGENT_DOMAIN_ROUTES)) { throw "VIT_AGENT_DOMAIN_ROUTES is set; unset it for this probe" }

function New-D1StubRepo {
    param([string]$Path, [string]$LauncherScript)
    foreach ($rel in @('scripts', 'godot_frontend_stub', 'VitApp\Workspace\Logs', 'temp\semantic-processor-agent-project-smoke-v1\fixtures\vit260_stub\case_project')) {
        New-Item -ItemType Directory -Path (Join-Path $Path $rel) -Force | Out-Null
    }
    Copy-Item -LiteralPath (Join-Path $StubsDir 'dev_agent_smoke_stub.ps1') -Destination (Join-Path $Path 'scripts\dev_agent_smoke.ps1') -Force
    Copy-Item -LiteralPath (Join-Path $StubsDir 'free_state_d1_smoke.py') -Destination (Join-Path $Path 'scripts\free_state_d1_smoke.py') -Force
    Copy-Item -LiteralPath (Join-Path $StubsDir 'stub_agent_face.py') -Destination (Join-Path $Path 'scripts\stub_agent_face.py') -Force
    Copy-Item -LiteralPath $LauncherScript -Destination (Join-Path $Path 'scripts\run_free_state_d1_smoke.ps1') -Force
    Set-Content -LiteralPath (Join-Path $Path 'VitApp\Workspace\Logs\agent_last.log') -Value 'vit260 stub agent log' -Encoding ASCII
    Set-Content -LiteralPath (Join-Path $Path 'kernel_dummy.exe') -Value 'vit260 stub kernel (Resolve-Path only, never executed)' -Encoding ASCII
    Set-Content -LiteralPath (Join-Path $Path 'temp\semantic-processor-agent-project-smoke-v1\fixtures\vit260_stub\case_project\case.vit') -Value 'vit260 stub project bytes' -Encoding ASCII
    $manifest = @{
        fixture_set_id = 'vit260_stub'
        cases          = @(@{ public_case_id = 'spv1_p01'; project_path = (Join-Path $Path 'temp\semantic-processor-agent-project-smoke-v1\fixtures\vit260_stub\case_project\case.vit') })
    }
    $manifest | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $Path 'temp\semantic-processor-agent-project-smoke-v1\fixtures\vit260_stub\fixture_manifest.json') -Encoding UTF8
    $Path
}

function Stop-StubServer {
    param([string]$StubRepo)
    $pidFile = Join-Path $StubRepo 'stub_server.pid'
    if (Test-Path -LiteralPath $pidFile) {
        $serverPid = [int](Get-Content -LiteralPath $pidFile -Raw).Trim()
        try { Stop-Process -Id $serverPid -Force -ErrorAction Stop } catch { }
    }
    $deadline = (Get-Date).AddSeconds(15)
    while ((Get-Date) -lt $deadline) {
        $k = @(Get-NetTCPConnection -LocalPort 5555 -State Listen -ErrorAction SilentlyContinue)
        $a = @(Get-NetTCPConnection -LocalPort 7878 -State Listen -ErrorAction SilentlyContinue)
        if ($k.Count -eq 0 -and $a.Count -eq 0) { return }
        Start-Sleep -Milliseconds 300
    }
    Write-Warning "stub server ports did not drain within 15s"
}

function Invoke-D1Leg {
    param([string]$Name, [string]$Branch, [string]$LauncherScript, [string[]]$Switches)
    $stub = New-D1StubRepo -Path (Join-Path $tempBase $Name) -LauncherScript $LauncherScript
    $console = Join-Path $ArtifactDir ($Name + '_console.txt')
    $prior = $env:VIT260_STUB_BRANCH
    $env:VIT260_STUB_BRANCH = $Branch
    try {
        $args = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $stub 'scripts\run_free_state_d1_smoke.ps1'),
                  '-RepoRoot', $stub, '-SkipBuild', '-KernelExe', (Join-Path $stub 'kernel_dummy.exe'),
                  '-PublicManifest', (Join-Path $stub 'temp\semantic-processor-agent-project-smoke-v1\fixtures\vit260_stub\fixture_manifest.json'),
                  '-GodotProjectRoot', (Join-Path $stub 'godot_frontend_stub'),
                  '-TimeoutSeconds', '60', '-UiPortWaitSeconds', '30') + $Switches
        # Relax EAP for the child call: legs 2-4 end in the launcher's honest
        # gate throw on stderr (expected evidence, not a driver failure).
        $priorEap = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try { & powershell @args *> $console }
        finally { $ErrorActionPreference = $priorEap }
        $code = $LASTEXITCODE
    }
    finally {
        if ($null -eq $prior) { Remove-Item Env:VIT260_STUB_BRANCH -ErrorAction SilentlyContinue }
        else { $env:VIT260_STUB_BRANCH = $prior }
        Stop-StubServer -StubRepo $stub
    }
    # Harvest the launcher's artifact dir (newest run stamp) into the evidence dir.
    $artRoot = Join-Path $stub 'artifacts\free_state_d1_s1'
    $runDir = $null
    if (Test-Path -LiteralPath $artRoot) { $runDir = @(Get-ChildItem -LiteralPath $artRoot -Directory | Sort-Object Name | Select-Object -Last 1)[0] }
    $lwGates = $null; $settleGates = $null; $reportCopy = $null
    if ($null -ne $runDir) {
        if (Test-Path -LiteralPath (Join-Path $runDir.FullName 'loudness_watch_gates.json')) {
            $lwGates = Join-Path $ArtifactDir ($Name + '_loudness_watch_gates.json')
            Copy-Item -LiteralPath (Join-Path $runDir.FullName 'loudness_watch_gates.json') -Destination $lwGates -Force
        }
        if (Test-Path -LiteralPath (Join-Path $runDir.FullName 'd1_settle_gates.json')) {
            $settleGates = Join-Path $ArtifactDir ($Name + '_d1_settle_gates.json')
            Copy-Item -LiteralPath (Join-Path $runDir.FullName 'd1_settle_gates.json') -Destination $settleGates -Force
        }
        if (Test-Path -LiteralPath (Join-Path $runDir.FullName 'd1_smoke_report.json')) {
            $reportCopy = Join-Path $ArtifactDir ($Name + '_d1_smoke_report.json')
            Copy-Item -LiteralPath (Join-Path $runDir.FullName 'd1_smoke_report.json') -Destination $reportCopy -Force
        }
    }
    [pscustomobject]@{
        leg          = $Name
        branch       = $Branch
        exit_code    = $code
        console_path = $console
        lw_gates     = $lwGates
        settle_gates = $settleGates
        report_copy  = $reportCopy
    }
}

function Read-JsonFile {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) { return $null }
    (Get-Content -LiteralPath $Path -Raw -Encoding UTF8) | ConvertFrom-Json
}

try {
    $leg1 = Invoke-D1Leg -Name 'leg1_postfix_zero_mutation' -Branch 'zero_mutation' -LauncherScript $PostFixScript -Switches @('-LoudnessWatch', '-D1LoudnessSettle')
    $leg2 = Invoke-D1Leg -Name 'leg2_postfix_one_mutation_lw' -Branch 'applied' -LauncherScript $PostFixScript -Switches @('-LoudnessWatch')
    $leg3 = Invoke-D1Leg -Name 'leg3_postfix_one_mutation_settle' -Branch 'applied' -LauncherScript $PostFixScript -Switches @('-D1LoudnessSettle')
    $leg4 = Invoke-D1Leg -Name 'leg4_prefix_zero_mutation' -Branch 'zero_mutation' -LauncherScript $PreFixScript -Switches @('-LoudnessWatch', '-D1LoudnessSettle')

    # ---- leg 1: the branchified zero-mutation judgment surface.
    $l1lw = Read-JsonFile -Path $leg1.lw_gates
    $l1st = Read-JsonFile -Path $leg1.settle_gates
    $l1SkipVals = @('disclosure', 'verdict_store_round_trigger', 'dedupe', 'config_off_silent', 'a4_zero_markers', 'e1_samples')
    $l1lwAllSkip = ($null -ne $l1lw) -and ($l1lw.phase_reachable -eq $false) -and ($l1lw.d1_branch -eq 'zero_mutation_boundary') -and (@($l1lw.failures).Count -eq 0)
    foreach ($k in $l1SkipVals) { if ($null -ne $l1lw -and $l1lw.gate_results.$k -ne 'skipped_branch_unreachable') { $l1lwAllSkip = $false } }
    $l1stAllSkip = ($null -ne $l1st) -and ($l1st.phase_reachable -eq $false) -and ($l1st.d1_branch -eq 'zero_mutation_boundary') -and (@($l1st.failures).Count -eq 0) -and
                   ($l1st.gate_results.settle_turn -eq 'skipped_branch_unreachable') -and ($l1st.gate_results.verdict_on_disk -eq 'skipped_branch_unreachable') -and
                   ($l1st.gate_results.action_face -eq 'skipped_branch_unreachable') -and ($l1st.gate_results.d1_node_lit -eq 'skipped_branch_unreachable')
    $l1Tail = (Select-String -LiteralPath $leg1.console_path -Pattern 'D1-S1 PASS: real-stack public-only smoke completed' -Quiet)
    $l1Skips = ((Select-String -LiteralPath $leg1.console_path -Pattern '\[lw\] BRANCH-SKIP' -Quiet) -and (Select-String -LiteralPath $leg1.console_path -Pattern '\[d1-settle\] BRANCH-SKIP' -Quiet))

    # ---- leg 2: 1-mutation LW gates run and fail honestly.
    $l2lw = Read-JsonFile -Path $leg2.lw_gates
    $l2GatesRan = ($null -ne $l2lw) -and ($null -eq ($l2lw.PSObject.Properties['phase_reachable'])) -and
                  ($l2lw.gate_results.disclosure -eq $false) -and (@($l2lw.failures).Count -ge 2)
    $l2Throw = (Select-String -LiteralPath $leg2.console_path -Pattern 'LOUDNESS-WATCH gates failed' -Quiet)

    # ---- leg 3: 1-mutation settle gates run and fail honestly.
    $l3st = Read-JsonFile -Path $leg3.settle_gates
    $l3GatesRan = ($null -ne $l3st) -and ($null -eq ($l3st.PSObject.Properties['phase_reachable'])) -and
                  ($l3st.gate_results.settle_turn -eq $false) -and ($l3st.gate_results.verdict_on_disk -eq $false) -and ($l3st.gate_results.d1_node_lit -eq $false) -and
                  (@($l3st.failures).Count -ge 3)
    $l3Throw = (Select-String -LiteralPath $leg3.console_path -Pattern 'D1-LOUDNESS-SETTLE gates failed' -Quiet)

    # ---- leg 4: pre-fix throws and cuts the settle phase (258 form).
    $l4lw = Read-JsonFile -Path $leg4.lw_gates
    $l4Cut = ($null -ne $l4lw) -and ($null -eq ($l4lw.PSObject.Properties['phase_reachable'])) -and (@($l4lw.failures).Count -ge 1) -and
             ([string]::IsNullOrWhiteSpace([string]$leg4.settle_gates)) -and
             (Select-String -LiteralPath $leg4.console_path -Pattern 'LOUDNESS-WATCH gates failed' -Quiet) -and
             (-not (Select-String -LiteralPath $leg4.console_path -Pattern 'BRANCH-SKIP' -Quiet))

    $summary = [pscustomobject]@{
        leg1_zero_mutation_postfix = [pscustomobject]@{ exit = $leg1.exit_code; lw_skip_face = $l1lwAllSkip; settle_skip_face = $l1stAllSkip; branch_skip_lines = [bool]$l1Skips; pass_tail = [bool]$l1Tail }
        leg2_one_mutation_lw       = [pscustomobject]@{ exit = $leg2.exit_code; gates_ran_honest_fail = $l2GatesRan; throw_present = [bool]$l2Throw }
        leg3_one_mutation_settle   = [pscustomobject]@{ exit = $leg3.exit_code; gates_ran_honest_fail = $l3GatesRan; throw_present = [bool]$l3Throw }
        leg4_zero_mutation_prefix  = [pscustomobject]@{ exit = $leg4.exit_code; lw_fail_no_branch_awareness = [bool]($null -ne $l4lw); settle_gates_absent_cut_off = $l4Cut }
    }
    $summary | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $ArtifactDir 'probe3_summary.json') -Encoding UTF8
    $summary | ConvertTo-Json -Depth 4 | Write-Host

    $ok1 = ($leg1.exit_code -eq 0) -and $l1lwAllSkip -and $l1stAllSkip -and $l1Skips -and $l1Tail
    $ok2 = ($leg2.exit_code -eq 1) -and $l2GatesRan -and $l2Throw
    $ok3 = ($leg3.exit_code -eq 1) -and $l3GatesRan -and $l3Throw
    $ok4 = ($leg4.exit_code -eq 1) -and $l4Cut
    Write-Host ("probe3 leg1 zero-mutation post-fix complete skip faces + exit 0:   " + $ok1)
    Write-Host ("probe3 leg2 1-mutation LW gates unchanged (run+throw):            " + $ok2)
    Write-Host ("probe3 leg3 1-mutation settle gates unchanged (run+throw):        " + $ok3)
    Write-Host ("probe3 leg4 pre-fix 258 form (throw cuts settle phase):           " + $ok4)
    $allOk = $ok1 -and $ok2 -and $ok3 -and $ok4
    Write-Host ("probe3 ALL PASS: " + $allOk)
    if (-not $allOk) { exit 1 }
    exit 0
}
finally {
    if (Test-Path -LiteralPath $tempBase) { Remove-Item -LiteralPath $tempBase -Recurse -Force -ErrorAction SilentlyContinue }
}

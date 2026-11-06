# probe_268.ps1 — D1GATE-SCRIPT-HYGIENE-1 (268) constructed verification.
#
# Zero real stack, zero LLM (pattern per the 260 three-probe harness). Each
# leg gets a fresh stub repo (dev_agent_smoke + D1 runner + agent HTTP face
# stubs) in %TEMP% and runs the D1 launcher against it, pre-fix script from
# the claim baseline vs post-fix from the worktree:
#
#   leg1  post-fix, applied branch, scenario=settle_pass  -> exit 0; settle
#         gates all green through the -Encoding UTF8 store read (fix 1), the
#         mix.report queue view (fix 2) and the no-card face (fix 3).
#   leg2  pre-fix,  same scenario                          -> exit 1; the ANSI
#         store read folds a good store into "no d1_settle record" (fix 1
#         evidence) and the mix_queue_view guess crashes on the missing
#         result property (fix 2 evidence).
#   leg3  post-fix, scenario=breach_response               -> exit 0; the
#         breach card is seen on the chat response face.
#   leg4  post-fix, scenario=breach_durable                -> exit 0; the
#         breach card is seen through the durable-face walk
#         (continuations[].pending_interaction, fix 3 evidence).
#   leg5  pre-fix,  scenario=breach_durable                -> exit 1; the bare
#         property access dies on the missing needs_confirmation key (265
#         run1 StrictMode lesson) or misses the card outright.
#   leg6  post-fix, zero_mutation branch, UTF-8 report     -> exit 0; branch
#         detected through the UTF-8 read (fix 4), skip-with-reason face,
#         260 branch semantics intact.
#   leg7  pre-fix,  zero_mutation branch, UTF-8 report     -> exit 1; the
#         branch read dies to the ANSI read and the run is misrouted into
#         the full-gate path (fix 4 evidence: gates run on a zero-mutation
#         run and fail structurally).
#   leg8  pre-fix,  zero_mutation branch, ASCII report     -> exit 0; same
#         pre-fix code, only the report charset differs — isolates the
#         encoding variable for legs 6/7.
param(
    [string]$ArtifactDir,
    [string]$StubsDir,
    [string]$PreFixScript,
    [string]$PostFixScript
)

$ErrorActionPreference = 'Stop'
$tempBase = Join-Path ([System.IO.Path]::GetTempPath()) ('vit268_probe_' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tempBase -Force | Out-Null

foreach ($port in @(5555, 7878)) {
    $busy = @(Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue)
    if ($busy.Count -gt 0) { Write-Host ("ENV-BLOCKED: port $port already listening"); exit 3 }
}
if (-not [string]::IsNullOrWhiteSpace($env:VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET)) { throw "VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET is set; unset it for this probe" }

function New-D1StubRepo {
    param([string]$Path, [string]$LauncherScript)
    foreach ($rel in @('scripts', 'godot_frontend_stub', 'VitApp\Workspace\Logs', 'temp\semantic-processor-agent-project-smoke-v1\fixtures\vit268_stub\case_project')) {
        New-Item -ItemType Directory -Path (Join-Path $Path $rel) -Force | Out-Null
    }
    Copy-Item -LiteralPath (Join-Path $StubsDir 'dev_agent_smoke_stub.ps1') -Destination (Join-Path $Path 'scripts\dev_agent_smoke.ps1') -Force
    Copy-Item -LiteralPath (Join-Path $StubsDir 'free_state_d1_smoke_268_stub.py') -Destination (Join-Path $Path 'scripts\free_state_d1_smoke.py') -Force
    Copy-Item -LiteralPath (Join-Path $StubsDir 'stub_agent_face_268.py') -Destination (Join-Path $Path 'scripts\stub_agent_face_268.py') -Force
    Copy-Item -LiteralPath $LauncherScript -Destination (Join-Path $Path 'scripts\run_free_state_d1_smoke.ps1') -Force
    Set-Content -LiteralPath (Join-Path $Path 'VitApp\Workspace\Logs\agent_last.log') -Value 'vit268 stub agent log' -Encoding ASCII
    Set-Content -LiteralPath (Join-Path $Path 'kernel_dummy.exe') -Value 'vit268 stub kernel (Resolve-Path only, never executed)' -Encoding ASCII
    Set-Content -LiteralPath (Join-Path $Path 'temp\semantic-processor-agent-project-smoke-v1\fixtures\vit268_stub\case_project\case.vit') -Value 'vit268 stub project bytes' -Encoding ASCII
    $manifest = @{
        fixture_set_id = 'vit268_stub'
        cases          = @(@{ public_case_id = 'spv1_p01'; project_path = (Join-Path $Path 'temp\semantic-processor-agent-project-smoke-v1\fixtures\vit268_stub\case_project\case.vit') })
    }
    $manifest | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $Path 'temp\semantic-processor-agent-project-smoke-v1\fixtures\vit268_stub\fixture_manifest.json') -Encoding UTF8
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
    param([string]$Name, [string]$Branch, [string]$Scene, [bool]$AsciiReport, [string]$LauncherScript)
    $stub = New-D1StubRepo -Path (Join-Path $tempBase $Name) -LauncherScript $LauncherScript
    $console = Join-Path $ArtifactDir ($Name + '_console.txt')
    $prior = @{ branch = $env:VIT268_STUB_BRANCH; scene = $env:VIT268_STUB_SCENARIO; ascii = $env:VIT268_STUB_REPORT_ASCII }
    $env:VIT268_STUB_BRANCH = $Branch
    $env:VIT268_STUB_SCENARIO = $Scene
    if ($AsciiReport) { $env:VIT268_STUB_REPORT_ASCII = '1' } else { Remove-Item Env:VIT268_STUB_REPORT_ASCII -ErrorAction SilentlyContinue }
    try {
        $args = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $stub 'scripts\run_free_state_d1_smoke.ps1'),
                  '-RepoRoot', $stub, '-SkipBuild', '-KernelExe', (Join-Path $stub 'kernel_dummy.exe'),
                  '-PublicManifest', (Join-Path $stub 'temp\semantic-processor-agent-project-smoke-v1\fixtures\vit268_stub\fixture_manifest.json'),
                  '-GodotProjectRoot', (Join-Path $stub 'godot_frontend_stub'),
                  '-TimeoutSeconds', '60', '-UiPortWaitSeconds', '30', '-D1LoudnessSettle')
        # Relax EAP for the child call: the pre-fix legs end in the launcher's
        # honest gate throw / StrictMode crash on stderr (expected evidence,
        # not a driver failure).
        $priorEap = $ErrorActionPreference
        $ErrorActionPreference = 'Continue'
        try { & powershell @args *> $console }
        finally { $ErrorActionPreference = $priorEap }
        $code = $LASTEXITCODE
    }
    finally {
        if ($null -eq $prior.branch) { Remove-Item Env:VIT268_STUB_BRANCH -ErrorAction SilentlyContinue } else { $env:VIT268_STUB_BRANCH = $prior.branch }
        if ($null -eq $prior.scene) { Remove-Item Env:VIT268_STUB_SCENARIO -ErrorAction SilentlyContinue } else { $env:VIT268_STUB_SCENARIO = $prior.scene }
        if ($null -eq $prior.ascii) { Remove-Item Env:VIT268_STUB_REPORT_ASCII -ErrorAction SilentlyContinue } else { $env:VIT268_STUB_REPORT_ASCII = $prior.ascii }
        Stop-StubServer -StubRepo $stub
    }
    # Harvest the launcher's artifact dir (newest run stamp) into the evidence dir.
    $artRoot = Join-Path $stub 'artifacts\free_state_d1_s1'
    $runDir = $null
    if (Test-Path -LiteralPath $artRoot) { $runDir = @(Get-ChildItem -LiteralPath $artRoot -Directory | Sort-Object Name | Select-Object -Last 1)[0] }
    $harvest = @{}
    if ($null -ne $runDir) {
        foreach ($fileName in @('d1_settle_gates.json', 'd1_smoke_report.json', 'd1_settle_chat.json', 'd1_settle_cancel.json', 'd1_settle_queue_view.json', 'd1_settle_verdict_store.json')) {
            $src = Join-Path $runDir.FullName $fileName
            if (Test-Path -LiteralPath $src) {
                $dst = Join-Path $ArtifactDir ($Name + '_' + $fileName)
                Copy-Item -LiteralPath $src -Destination $dst -Force
                $harvest[$fileName] = $dst
            }
        }
        # The raw verdict store under the project workdir: proves the store was
        # on disk when gate B folded a parse throw into "no record" (leg2).
        $rawStore = Join-Path $runDir.FullName 'project\.vit_agent\loudness_watch_verdicts.json'
        if (Test-Path -LiteralPath $rawStore) {
            $dst = Join-Path $ArtifactDir ($Name + '_loudness_watch_verdict_store_raw.json')
            Copy-Item -LiteralPath $rawStore -Destination $dst -Force
            $harvest['raw_store'] = $dst
        }
    }
    [pscustomobject]@{
        leg            = $Name
        branch         = $Branch
        scene          = $Scene
        exit_code      = $code
        console_path   = $console
        settle_gates   = $harvest['d1_settle_gates.json']
        report_copy    = $harvest['d1_smoke_report.json']
        raw_store_path = $harvest['raw_store']
    }
}

function Read-JsonFile {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) { return $null }
    (Get-Content -LiteralPath $Path -Raw -Encoding UTF8) | ConvertFrom-Json
}

function Assert-SettleGatesAllGreen {
    param([string]$Path)
    $g = Read-JsonFile -Path $Path
    if ($null -eq $g) { return $false }
    return (($g.gate_results.settle_turn -eq $true) -and ($g.gate_results.verdict_on_disk -eq $true) -and
            ($g.gate_results.action_face -eq $true) -and ($g.gate_results.d1_node_lit -eq $true) -and
            (@($g.failures).Count -eq 0))
}

function Assert-SettleGatesSkipFace {
    param([string]$Path)
    $g = Read-JsonFile -Path $Path
    if ($null -eq $g) { return $false }
    $allSkip = ($g.phase_reachable -eq $false) -and ($g.d1_branch -eq 'zero_mutation_boundary') -and (@($g.failures).Count -eq 0)
    foreach ($k in @('settle_turn', 'verdict_on_disk', 'action_face', 'd1_node_lit')) {
        if ($g.gate_results.$k -ne 'skipped_branch_unreachable') { $allSkip = $false }
    }
    return $allSkip
}

try {
    $leg1 = Invoke-D1Leg -Name 'leg1_postfix_settle_pass'       -Branch 'applied'       -Scene 'settle_pass'    -AsciiReport $false -LauncherScript $PostFixScript
    $leg2 = Invoke-D1Leg -Name 'leg2_prefix_settle_pass'        -Branch 'applied'       -Scene 'settle_pass'    -AsciiReport $false -LauncherScript $PreFixScript
    $leg3 = Invoke-D1Leg -Name 'leg3_postfix_breach_response'   -Branch 'applied'       -Scene 'breach_response' -AsciiReport $false -LauncherScript $PostFixScript
    $leg4 = Invoke-D1Leg -Name 'leg4_postfix_breach_durable'    -Branch 'applied'       -Scene 'breach_durable' -AsciiReport $false -LauncherScript $PostFixScript
    $leg5 = Invoke-D1Leg -Name 'leg5_prefix_breach_durable'     -Branch 'applied'       -Scene 'breach_durable' -AsciiReport $false -LauncherScript $PreFixScript
    $leg6 = Invoke-D1Leg -Name 'leg6_postfix_zero_mut_utf8'     -Branch 'zero_mutation' -Scene 'settle_pass'    -AsciiReport $false -LauncherScript $PostFixScript
    $leg7 = Invoke-D1Leg -Name 'leg7_prefix_zero_mut_utf8'      -Branch 'zero_mutation' -Scene 'settle_pass'    -AsciiReport $false -LauncherScript $PreFixScript
    $leg8 = Invoke-D1Leg -Name 'leg8_prefix_zero_mut_ascii'     -Branch 'zero_mutation' -Scene 'settle_pass'    -AsciiReport $true  -LauncherScript $PreFixScript

    # ---- leg 1/3/4: post-fix green faces.
    $l1ok = ($leg1.exit_code -eq 0) -and (Assert-SettleGatesAllGreen -Path $leg1.settle_gates) -and
            (Select-String -LiteralPath $leg1.console_path -Pattern 'non-breach verdict parked no action card' -Quiet) -and
            (Select-String -LiteralPath $leg1.console_path -Pattern '\[ok\] D1 node state=in_progress' -Quiet) -and
            (Select-String -LiteralPath $leg1.console_path -Pattern 'D1-S1 PASS: real-stack public-only smoke completed' -Quiet)
    $l3ok = ($leg3.exit_code -eq 0) -and (Assert-SettleGatesAllGreen -Path $leg3.settle_gates) -and
            (Select-String -LiteralPath $leg3.console_path -Pattern 'face=chat_response_interaction_requests' -Quiet) -and
            (Select-String -LiteralPath $leg3.console_path -Pattern 'mix_tick_rejected' -Quiet)
    $l4ok = ($leg4.exit_code -eq 0) -and (Assert-SettleGatesAllGreen -Path $leg4.settle_gates) -and
            (Select-String -LiteralPath $leg4.console_path -Pattern 'face=runtime_status_pending_interaction' -Quiet)

    # ---- leg 2: pre-fix ANSI store read + mix_queue_view guess. The PS 5.1
    # StrictMode error text is localized; PropertyNotFoundStrict (the
    # FullyQualifiedErrorId) is the locale-independent anchor.
    $l2GateBFail = Select-String -LiteralPath $leg2.console_path -Pattern 'no d1_settle trigger record found' -Quiet
    $l2StoreOnDisk = (-not [string]::IsNullOrWhiteSpace([string]$leg2.raw_store_path)) -and (Test-Path -LiteralPath $leg2.raw_store_path)
    $l2StrictOrD = Select-String -LiteralPath $leg2.console_path -Pattern 'PropertyNotFoundStrict' -Quiet
    $l2ok = ($leg2.exit_code -ne 0) -and $l2GateBFail -and $l2StoreOnDisk -and $l2StrictOrD -and
            (-not (Select-String -LiteralPath $leg2.console_path -Pattern 'D1-S1 PASS: real-stack public-only smoke completed' -Quiet))

    # ---- leg 5: pre-fix durable-card face.
    $l5Miss = (Select-String -LiteralPath $leg5.console_path -Pattern 'PropertyNotFoundStrict' -Quiet) -or
              (Select-String -LiteralPath $leg5.console_path -Pattern 'must park a governed confirmation card' -Quiet)
    $l5ok = ($leg5.exit_code -ne 0) -and $l5Miss -and
            (-not (Select-String -LiteralPath $leg5.console_path -Pattern 'face=runtime_status_pending_interaction' -Quiet)) -and
            (-not (Select-String -LiteralPath $leg5.console_path -Pattern 'D1-S1 PASS: real-stack public-only smoke completed' -Quiet))

    # ---- legs 6/7/8: zero-mutation regression + encoding isolation.
    $l6ok = ($leg6.exit_code -eq 0) -and (Assert-SettleGatesSkipFace -Path $leg6.settle_gates) -and
            (Select-String -LiteralPath $leg6.console_path -Pattern '\[d1-settle\] BRANCH-SKIP' -Quiet) -and
            (Select-String -LiteralPath $leg6.console_path -Pattern 'D1-S1 PASS: real-stack public-only smoke completed' -Quiet)
    $l7ok = ($leg7.exit_code -ne 0) -and
            (Select-String -LiteralPath $leg7.console_path -Pattern '\[d1-settle\] gate A: explicit check chat turn' -Quiet) -and
            (-not (Select-String -LiteralPath $leg7.console_path -Pattern 'BRANCH-SKIP' -Quiet)) -and
            (-not (Select-String -LiteralPath $leg7.console_path -Pattern 'D1-S1 PASS: real-stack public-only smoke completed' -Quiet))
    $l8ok = ($leg8.exit_code -eq 0) -and (Assert-SettleGatesSkipFace -Path $leg8.settle_gates) -and
            (Select-String -LiteralPath $leg8.console_path -Pattern '\[d1-settle\] BRANCH-SKIP' -Quiet)

    $summary = [pscustomobject]@{
        leg1_postfix_settle_pass      = [pscustomobject]@{ exit = $leg1.exit_code; gates_all_green = [bool]$l1ok }
        leg2_prefix_settle_pass       = [pscustomobject]@{ exit = $leg2.exit_code; gate_b_ansi_fail = [bool]$l2GateBFail; store_on_disk = [bool]$l2StoreOnDisk; strict_crash = [bool]$l2StrictOrD; leg_ok = [bool]$l2ok }
        leg3_postfix_breach_response  = [pscustomobject]@{ exit = $leg3.exit_code; gates_all_green = [bool]$l3ok }
        leg4_postfix_breach_durable   = [pscustomobject]@{ exit = $leg4.exit_code; gates_all_green = [bool]$l4ok }
        leg5_prefix_breach_durable    = [pscustomobject]@{ exit = $leg5.exit_code; card_miss_or_crash = [bool]$l5Miss; leg_ok = [bool]$l5ok }
        leg6_postfix_zero_mut_utf8    = [pscustomobject]@{ exit = $leg6.exit_code; skip_face = [bool]$l6ok }
        leg7_prefix_zero_mut_utf8     = [pscustomobject]@{ exit = $leg7.exit_code; full_gate_misroute = [bool]$l7ok }
        leg8_prefix_zero_mut_ascii    = [pscustomobject]@{ exit = $leg8.exit_code; skip_face = [bool]$l8ok }
    }
    $summary | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $ArtifactDir 'probe_268_summary.json') -Encoding UTF8
    $summary | ConvertTo-Json -Depth 4 | Write-Host

    Write-Host ("leg1 post-fix settle pass all green exit 0:                       " + $l1ok)
    Write-Host ("leg2 pre-fix ANSI store read + mix_queue_view honest fail:       " + $l2ok)
    Write-Host ("leg3 post-fix breach card on chat response face exit 0:          " + $l3ok)
    Write-Host ("leg4 post-fix breach card via durable-face walk exit 0:          " + $l4ok)
    Write-Host ("leg5 pre-fix durable-card face dies (StrictMode/miss):           " + $l5ok)
    Write-Host ("leg6 post-fix zero-mutation UTF-8 report skip face exit 0:       " + $l6ok)
    Write-Host ("leg7 pre-fix zero-mutation UTF-8 report full-gate misroute:      " + $l7ok)
    Write-Host ("leg8 pre-fix zero-mutation ASCII report control skip face:       " + $l8ok)
    $allOk = $l1ok -and $l2ok -and $l3ok -and $l4ok -and $l5ok -and $l6ok -and $l7ok -and $l8ok
    Write-Host ("probe_268 ALL PASS: " + $allOk)
    if (-not $allOk) { exit 1 }
    exit 0
}
finally {
    if (Test-Path -LiteralPath $tempBase) { Remove-Item -LiteralPath $tempBase -Recurse -Force -ErrorAction SilentlyContinue }
}

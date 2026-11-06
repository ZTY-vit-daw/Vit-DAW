# probe2_rundir.ps1 — SMOKE-SCRIPT-HYGIENE-1 (260) item (2) constructed verification.
#
# Zero stack, zero LLM. Exercises the -RunDir parameter plumbing of
# run_tracklist_fix_probe.ps1 up to (and just past) the kernel start:
# -SkipBuild with pre-created dummy agent/hub binaries, a dummy kernel exe
# (copy of cmd.exe — never a real VitApp), and a 3s kernel-start timeout so
# the leg dies honestly at "kernel 5555 not listening" / an early-exit dummy.
# The RunDir semantics are observable BEFORE that point: run dir creation,
# run_meta.json, and teardown.json all land under the RunDir.
#
#   leg A  pre-fix  + -RunDir <passed_dir> -> passed dir NOT created (defect: cleared at ps1:147)
#   leg B  post-fix + -RunDir <passed_dir> -> passed dir created; run_meta.json + teardown.json inside
#   leg C  post-fix, no -RunDir            -> default run_<ts> under the card's coord\runs dir still works
#
#   leg D  post-fix + -PreflightOnly       -> port/proc guard face intact (no dirs, no processes)
param(
    [string]$ArtifactDir,
    [string]$PreFixScript,
    [string]$PostFixScript
)

$ErrorActionPreference = 'Stop'
$tempBase = Join-Path ([System.IO.Path]::GetTempPath()) ('vit260_probe2_' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tempBase -Force | Out-Null

# Abort as env-blocked (do not fake a functional verdict) if any stack port is held.
foreach ($port in @(5555, 5556, 5557, 7878, 8787, 4445)) {
    $busy = @(Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue)
    if ($busy.Count -gt 0) { Write-Host ("ENV-BLOCKED: port $port already listening"); exit 3 }
}

function New-RunDirStubRepo {
    param([string]$Path)
    $binDir = Join-Path $Path 'agent\bin'
    New-Item -ItemType Directory -Path $binDir -Force | Out-Null
    # Dummy binaries: only Test-Path is ever called on them (-SkipBuild); the
    # kernel dummy below never survives past the 3s listen timeout.
    Set-Content -LiteralPath (Join-Path $binDir 'VitAgent.exe') -Value 'vit260 stub binary (never executed)' -Encoding ASCII
    Set-Content -LiteralPath (Join-Path $binDir 'vsphub.exe') -Value 'vit260 stub binary (never executed)' -Encoding ASCII
    New-Item -ItemType Directory -Path (Join-Path $Path 'coord\runs\FIX-TRACKLIST-SHADOW-FRESHNESS-1') -Force | Out-Null
    Copy-Item -LiteralPath $env:ComSpec -Destination (Join-Path $Path 'kernel_dummy.exe') -Force
    # run_meta.json embeds `git -C RepoRoot rev-parse HEAD` / `status --short`;
    # an empty (or non-) git repo makes those emit stderr, which the script's
    # Stop-mode EAP turns terminating before run_meta.json lands. One commit
    # keeps the stub on the happy path the real repo is always on.
    & git -C $Path init -q
    if ($LASTEXITCODE -ne 0) { throw "git init failed for stub $Path" }
    & git -C $Path add -A
    if ($LASTEXITCODE -ne 0) { throw "git add failed for stub $Path" }
    & git -C $Path -c user.name=vit260stub -c user.email=vit260stub@local commit -q -m 'vit260 probe2 stub repo'
    if ($LASTEXITCODE -ne 0) { throw "git commit failed for stub $Path" }
    $Path
}

function Invoke-RunDirLeg {
    param([string]$Name, [string]$StubRepo, [string]$ScriptSource, [string[]]$ExtraArgs)
    $console = Join-Path $ArtifactDir ($Name + '_console.txt')
    $args = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $ScriptSource,
              '-RepoRoot', $StubRepo,
              '-KernelExe', (Join-Path $StubRepo 'kernel_dummy.exe'),
              '-SkipBuild', '-AllowOutOfScopeGodot', '-KernelStartTimeoutSeconds', '3') + $ExtraArgs
    # Relax EAP for the child call: the expected kernel-timeout throw arrives
    # on the child's stderr (evidence, not a driver failure).
    $priorEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try { & powershell @args *> $console }
    finally { $ErrorActionPreference = $priorEap }
    [pscustomobject]@{
        leg          = $Name
        exit_code    = $LASTEXITCODE
        console_path = $console
        stub_repo    = $StubRepo
    }
}

try {
    $stubA = New-RunDirStubRepo -Path (Join-Path $tempBase 'stubA_prefix')
    $stubB = New-RunDirStubRepo -Path (Join-Path $tempBase 'stubB_postfix_passed')
    $stubC = New-RunDirStubRepo -Path (Join-Path $tempBase 'stubC_postfix_default')
    $stubD = New-RunDirStubRepo -Path (Join-Path $tempBase 'stubD_postfix_preflight')

    $passedDirA = Join-Path $stubA 'passed_rundir_A'
    $passedDirB = Join-Path $stubB 'passed_rundir_B'

    $legA = Invoke-RunDirLeg -Name 'legA_prefix_ignored_rundir' -StubRepo $stubA -ScriptSource $PreFixScript -ExtraArgs @('-RunDir', $passedDirA)
    $legB = Invoke-RunDirLeg -Name 'legB_postfix_passed_rundir' -StubRepo $stubB -ScriptSource $PostFixScript -ExtraArgs @('-RunDir', $passedDirB)
    $legC = Invoke-RunDirLeg -Name 'legC_postfix_default_rundir' -StubRepo $stubC -ScriptSource $PostFixScript -ExtraArgs @()
    $legD = Invoke-RunDirLeg -Name 'legD_postfix_preflight_only' -StubRepo $stubD -ScriptSource $PostFixScript -ExtraArgs @('-PreflightOnly')

    # leg A: the passed dir was ignored (defect): no passed_rundir_A, and a
    # default run_<ts> dir appeared under the card's coord\runs path instead.
    $defaultDirsA = @(Get-ChildItem -LiteralPath (Join-Path $stubA 'coord\runs\FIX-TRACKLIST-SHADOW-FRESHNESS-1') -Directory -ErrorAction SilentlyContinue)
    # leg B: the passed dir is honored end to end.
    $runMetaB = Test-Path -LiteralPath (Join-Path $passedDirB 'run_meta.json') -PathType Leaf
    $teardownB = Test-Path -LiteralPath (Join-Path $passedDirB 'teardown.json') -PathType Leaf
    $passedDirBEscaped = [regex]::Escape($passedDirB)
    $runDirLineB = (Select-String -LiteralPath $legB.console_path -Pattern $passedDirBEscaped -Quiet)
    # leg C: default timestamped dir still created when -RunDir is omitted.
    $defaultDirsC = @(Get-ChildItem -LiteralPath (Join-Path $stubC 'coord\runs\FIX-TRACKLIST-SHADOW-FRESHNESS-1') -Directory -ErrorAction SilentlyContinue)
    # leg D: preflight-only leaves no run dirs at all.
    $dirsAfterD = @(Get-ChildItem -LiteralPath (Join-Path $stubD 'coord\runs\FIX-TRACKLIST-SHADOW-FRESHNESS-1') -Directory -ErrorAction SilentlyContinue)

    $summary = [pscustomobject]@{
        legA = [pscustomobject]@{ exit = $legA.exit_code; passed_dir_created = (Test-Path -LiteralPath $passedDirA); default_dirs_instead = $defaultDirsA.Count }
        legB = [pscustomobject]@{ exit = $legB.exit_code; passed_dir_created = (Test-Path -LiteralPath $passedDirB); run_meta_inside = $runMetaB; teardown_inside = $teardownB; console_names_dir = [bool]$runDirLineB }
        legC = [pscustomobject]@{ exit = $legC.exit_code; default_dirs_created = ($defaultDirsC.Count -ge 1) }
        legD = [pscustomobject]@{ exit = $legD.exit_code; run_dirs_after = $dirsAfterD.Count; preflight_ok_line = [bool](Select-String -LiteralPath $legD.console_path -Pattern 'preflight passed' -Quiet) }
    }
    $summary | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $ArtifactDir 'probe2_summary.json') -Encoding UTF8
    $summary | ConvertTo-Json -Depth 4 | Write-Host

    $okA = (-not (Test-Path -LiteralPath $passedDirA)) -and ($defaultDirsA.Count -ge 1)
    $okB = ((Test-Path -LiteralPath $passedDirB) -and $runMetaB -and $teardownB -and $runDirLineB -and ($legB.exit_code -eq 1))
    $okC = ($defaultDirsC.Count -ge 1) -and ($legC.exit_code -eq 1)
    $okD = ($dirsAfterD.Count -eq 0) -and ($legD.exit_code -eq 0)
    Write-Host ("probe2 legA defect reproduced (passed dir ignored): " + $okA)
    Write-Host ("probe2 legB passed RunDir honored end-to-end:        " + $okB)
    Write-Host ("probe2 legC default RunDir path unchanged:          " + $okC)
    Write-Host ("probe2 legD preflight-only face intact:             " + $okD)
    $allOk = $okA -and $okB -and $okC -and $okD
    Write-Host ("probe2 ALL PASS: " + $allOk)
    if (-not $allOk) { exit 1 }
    exit 0
}
finally {
    # Make sure no dummy-kernel copy of cmd.exe from a leg survived the
    # script's own teardown before dropping the staging tree.
    foreach ($p in @(Get-Process -Name kernel_dummy -ErrorAction SilentlyContinue)) { Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue }
    if (Test-Path -LiteralPath $tempBase) { Remove-Item -LiteralPath $tempBase -Recurse -Force -ErrorAction SilentlyContinue }
}

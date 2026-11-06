# probe1_c1_exitcode.ps1 — SMOKE-SCRIPT-HYGIENE-1 (260) item (1) constructed verification.
#
# Zero stack, zero LLM. Builds isolated stub repos under %TEMP% (never the
# source tree, AGENTS.md section 10) and runs the c1 smoke script against
# them, comparing pre-fix vs post-fix exit codes:
#
#   leg A  pre-fix  script + FAIL stub go test -> expect exit 0   (the false green, 257 G3)
#   leg B  post-fix script + FAIL stub go test -> expect exit <>0 (throw names the failing group)
#   leg C  post-fix script + PASS stub go test -> expect exit 0   (no regression on green)
#
# The script under test derives its repo root from its own location, so each
# stub repo gets the script copied to <stub>\scripts\.
param(
    [string]$ArtifactDir,
    [string]$PreFixScript,
    [string]$PostFixScript
)

$ErrorActionPreference = 'Stop'
$tempBase = Join-Path ([System.IO.Path]::GetTempPath()) ('vit260_probe1_' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tempBase -Force | Out-Null

$pkgDirs = @('frequencycleanup', 'capabilitycontext', 'capabilityadapters',
             'orchestration', 'orchestrationruntime', 'chat', 'harness', 'mixboard')

function New-C1StubRepo {
    param([string]$Path, [bool]$FailingFirstGroup)
    New-Item -ItemType Directory -Path (Join-Path $Path 'scripts') -Force | Out-Null
    $agentDir = Join-Path $Path 'agent'
    New-Item -ItemType Directory -Path $agentDir -Force | Out-Null
    Set-Content -LiteralPath (Join-Path $agentDir 'go.mod') -Value @('module vit260c1stub', '', 'go 1.21') -Encoding ASCII
    foreach ($pkg in $pkgDirs) {
        $dir = Join-Path $agentDir ('internal\' + $pkg)
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        Set-Content -LiteralPath (Join-Path $dir ('pkg.go')) -Value ('package ' + $pkg) -Encoding ASCII
    }
    $testBody = if ($FailingFirstGroup) {
        @('package frequencycleanup', '', 'import "testing"', '', 'func TestStubConstructedFail(t *testing.T) {',
          '	t.Error("SMOKE-SCRIPT-HYGIENE-1 probe1 constructed FAIL stub")', '}')
    }
    else {
        @('package frequencycleanup', '', 'import "testing"', '', 'func TestStubConstructedPass(t *testing.T) {',
          '	t.Log("SMOKE-SCRIPT-HYGIENE-1 probe1 constructed PASS stub")', '}')
    }
    Set-Content -LiteralPath (Join-Path $agentDir 'internal\frequencycleanup\stub_test.go') -Value $testBody -Encoding ASCII
    # `git -C <root> diff --check` must succeed after the go test legs: an
    # empty repo with only untracked files diffs clean.
    & git -C $Path init -q
    if ($LASTEXITCODE -ne 0) { throw "git init failed for stub $Path" }
}

function Invoke-C1Leg {
    param([string]$Name, [string]$StubRepo, [string]$ScriptSource)
    Copy-Item -LiteralPath $ScriptSource -Destination (Join-Path $StubRepo 'scripts\run_c1_frequency_cleanup_smoke.ps1') -Force
    $console = Join-Path $ArtifactDir ($Name + '_console.txt')
    # Relax EAP for the child call: a FAIL leg's throw arrives on the child's
    # stderr, and Stop-mode would kill this driver on the evidence it asked for.
    $priorEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try { & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $StubRepo 'scripts\run_c1_frequency_cleanup_smoke.ps1') *> $console }
    finally { $ErrorActionPreference = $priorEap }
    $code = $LASTEXITCODE
    $throwSeen = (Select-String -LiteralPath $console -Pattern 'failed with exit code' -Quiet)
    [pscustomobject]@{
        leg          = $Name
        exit_code    = $code
        throw_line   = [bool]$throwSeen
        stub_repo    = $StubRepo
        console_path = $console
    }
}

try {
    $stubFailA = Join-Path $tempBase 'stubA_prefix_fail'
    $stubFailB = Join-Path $tempBase 'stubB_postfix_fail'
    $stubPassC = Join-Path $tempBase 'stubC_postfix_pass'
    New-C1StubRepo -Path $stubFailA -FailingFirstGroup $true
    New-C1StubRepo -Path $stubFailB -FailingFirstGroup $true
    New-C1StubRepo -Path $stubPassC -FailingFirstGroup $false

    $results = @(
        Invoke-C1Leg -Name 'legA_prefix_failstub'    -StubRepo $stubFailA -ScriptSource $PreFixScript
        Invoke-C1Leg -Name 'legB_postfix_failstub'   -StubRepo $stubFailB -ScriptSource $PostFixScript
        Invoke-C1Leg -Name 'legC_postfix_passstub'   -StubRepo $stubPassC -ScriptSource $PostFixScript
    )
    $summaryPath = Join-Path $ArtifactDir 'probe1_summary.json'
    $results | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $summaryPath -Encoding UTF8

    # Assertions (pre-declared by the card): FAIL-stub post-fix exit <> 0;
    # pre-fix exit 0 (false green reproduced); post-fix PASS stub exit 0.
    $a = $results[0]; $b = $results[1]; $c = $results[2]
    $verdicts = @(
        ("legA pre-fix FAIL stub exit=0 false-green reproduced: " + ($a.exit_code -eq 0)),
        ("legB post-fix FAIL stub exit<>0 check present: " + (($b.exit_code -ne 0) -and $b.throw_line)),
        ("legC post-fix PASS stub exit=0 no regression: " + ($c.exit_code -eq 0))
    )
    $verdicts | ForEach-Object { Write-Host $_ }
    $allOk = (($a.exit_code -eq 0) -and ($b.exit_code -ne 0) -and $b.throw_line -and ($c.exit_code -eq 0))
    Write-Host ("probe1 ALL PASS: " + $allOk)
    if (-not $allOk) { exit 1 }
    exit 0
}
finally {
    if (Test-Path -LiteralPath $tempBase) { Remove-Item -LiteralPath $tempBase -Recurse -Force -ErrorAction SilentlyContinue }
}

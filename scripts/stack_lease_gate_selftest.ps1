# STACKLEASE-OWNER-PARAM-1 (442) self-test: deterministic zero-stack assertions
# for the stack-lease gate changes in scripts/.
#
# What it pins (per card acceptance: "最小自测 … 无惯例则 dry-run 模式断言"):
#   1. Parse check: all five changed scripts parse with zero errors
#      (System.Management.Automation.Language.Parser, PS 5.1 compatible).
#   2. Gate behavior: the Get-StackLeaseVerdict function is extracted VERBATIM
#      (AST) from each of the three owner-gate scripts and run against a
#      synthetic ledger case table (current ledger format, ISO/space timestamps,
#      quoted/bare owners, full-width parens, whitespace/CRLF, release-latest,
#      stale-event head-window regression, ambiguous owner, fail-closed empty
#      owner). All copies must agree and match the expected verdict.
#   3. Latest-event locator: Get-LatestOccupancyEvent extracted from the two
#      head-window scripts (vitshell_full / vitshell_open_chain) and asserted.
#   4. Hardcoded-owner scan: 'owner=Codex', '/root/entry388', '/root/restore403'
#      must not appear anywhere in scripts/*.ps1 (grep evidence).
#
# No real stack, no network: everything runs on in-memory synthetic ledgers.
# Runtime state writes only under -RunRoot (default: %TEMP% subfolder).
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\stack_lease_gate_selftest.ps1
# PASS = exit 0.

[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$RunRoot = ""
)

$ErrorActionPreference = "Stop"

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) { return (Resolve-Path -LiteralPath $Explicit).Path }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

function Write-Step { param([string]$Message) Write-Host ""; Write-Host ("== " + $Message) -ForegroundColor Cyan }
function Write-Ok { param([string]$Message) Write-Host ("ok: " + $Message) -ForegroundColor Green }
function Write-Fail { param([string]$Message) Write-Host ("FAIL: " + $Message) -ForegroundColor Red }

$repo = Resolve-RepoRoot -Explicit $RepoRoot
$scriptsDir = Join-Path $repo "scripts"
if (-not (Test-Path -LiteralPath $scriptsDir)) { Write-Fail "scripts dir not found under $repo"; exit 2 }

if ([string]::IsNullOrWhiteSpace($RunRoot)) {
    $stamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $RunRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("stack_lease_gate_selftest_" + $stamp)
}
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
$RunRoot = (Resolve-Path -LiteralPath $RunRoot).Path
Write-Host "selftest run root: $RunRoot"

$failures = @()

# ---------------------------------------------------------------------------
# Step 1: parse check (zero errors) on every changed script.
# ---------------------------------------------------------------------------
$changedScripts = @(
    "run_entry_management_acceptance.ps1",
    "vitshell_smoke.ps1",
    "vitshell_zproduct_smoke.ps1",
    "vitshell_full_smoke.ps1",
    "vitshell_open_chain_smoke.ps1"
)
Write-Step "1/4 parse check (Parser::ParseFile, zero errors required)"
$parsed = @{}
foreach ($name in $changedScripts) {
    $path = Join-Path $scriptsDir $name
    if (-not (Test-Path -LiteralPath $path)) { $failures += ("missing changed script: " + $name); continue }
    $tokens = $null; $errors = $null
    $null = [System.Management.Automation.Language.Parser]::ParseFile($path, [ref]$tokens, [ref]$errors)
    if ($errors -and $errors.Count -gt 0) {
        $msgs = ($errors | ForEach-Object { ($_.Extent.StartLineNumber.ToString()) + ": " + $_.Message }) -join " | "
        $failures += ("parse errors in " + $name + ": " + $msgs)
        Write-Fail ("parse errors in " + $name + ": " + $msgs)
    } else {
        Write-Ok ("parse clean: " + $name)
    }
    $parsed[$name] = $true
}

# ---------------------------------------------------------------------------
# AST helper: extract a named function definition verbatim from a script.
# ---------------------------------------------------------------------------
function Get-ExtractedFunction {
    param([string]$Path, [string]$FunctionName)
    $tokens = $null; $errors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($Path, [ref]$tokens, [ref]$errors)
    if ($errors -and $errors.Count -gt 0) { throw ("parse errors in " + $Path + " during extraction") }
    $found = $ast.FindAll({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $FunctionName }, $true)
    if (-not $found -or $found.Count -eq 0) { return $null }
    return [scriptblock]::Create($found[0].Extent.Text)
}

# ---------------------------------------------------------------------------
# Step 2: Get-StackLeaseVerdict case table (synthetic ledgers, newest-first).
# ---------------------------------------------------------------------------
Write-Step "2/4 Get-StackLeaseVerdict case table (3 extracted copies must agree)"

# Expected fields: Pass, State (grant cases), Owner (grant cases), ReasonContains (reject cases).
$verdictCases = @(
    @{ Name="grant_current_format"; Lines=@("# PC 真实运行栈占用记录", "- 2026-11-20 23:50 +0800 预留 / **CONVERGE-ENTRY-1(388) 真栈腿**：owner=Codex /root/entry388；berth=x；teardown 全零；栈空闲后释放。"); Owner="Codex /root/entry388"; Card="CONVERGE-ENTRY-1(388)"; Pass=$true; State="预留"; OwnerValue="Codex /root/entry388" },
    @{ Name="grant_iso_timestamp"; Lines=@("# hdr", "- 2026-11-20T23:50:00+08:00 占用 / **SIM-MUSIC-WIDGET-1(441) 真栈手测**：owner=lead；单次轮。"); Owner="lead"; Card="SIM-MUSIC-WIDGET-1(441)"; Pass=$true; State="占用"; OwnerValue="lead" },
    @{ Name="grant_no_seconds_no_tz"; Lines=@("# hdr", "- 2026-11-20 23:50 预留 / **CARD-X(442)**：owner=lead"); Owner="lead"; Card="CARD-X(442)"; Pass=$true; State="预留"; OwnerValue="lead" },
    @{ Name="grant_bare_owner_paren_suffix"; Lines=@("# hdr", "- 2026-11-20 20:06 +0800 预留 / **AUDIO-CPP-INSTRUMENTAL-SPIKE-1(439) GPU 占用**：owner=audiocpp439（ZCode 执行子代理，lead 派发）；恰 1 次生成。"); Owner="audiocpp439"; Card="AUDIO-CPP-INSTRUMENTAL-SPIKE-1(439)"; Pass=$true; State="预留"; OwnerValue="audiocpp439" },
    @{ Name="grant_fullwidth_parens_card"; Lines=@("# hdr", "- 2026-11-20 23:50 +0800 预留 / **CARD-X（442）真栈腿**：owner=lead。"); Owner="lead"; Card="CARD-X(442)"; Pass=$true; State="预留"; OwnerValue="lead" },
    @{ Name="grant_backtick_owner"; Lines=@("# hdr", '- 2026-11-21 00:01 +0800 占用 / **CARD-Z(403)**：owner=Codex `pc442-a`；单轮。'); Owner="pc442-a"; Card="CARD-Z(403)"; Pass=$true; State="占用"; OwnerValue="pc442-a" },
    @{ Name="grant_double_quoted_owner"; Lines=@("# hdr", '- 2026-11-21 00:02 +0800 占用 / **CARD-Z(403)**：owner="session-A"；单轮。'); Owner="session-A"; Card="CARD-Z(403)"; Pass=$true; State="占用"; OwnerValue="session-A" },
    @{ Name="grant_leading_whitespace_and_crlf"; Lines=@("# hdr", ("-   2026-11-20 23:50 +0800 预留 / **CARD-X(442)**：owner=lead" + [string][char]13)); Owner="lead"; Card="CARD-X(442)"; Pass=$true; State="预留"; OwnerValue="lead" },
    @{ Name="grant_field_order_shuffled"; Lines=@("# hdr", "- 预留：owner=lead；berth=b1 / **CARD-X(442)** 真栈腿 2026-11-20"); Owner="lead"; Card="CARD-X(442)"; Pass=$true; State="预留"; OwnerValue="lead" },
    @{ Name="release_latest_rejected_stale_grant_ignored"; Lines=@("# hdr", "- 2026-11-20 23:50 +0800 释放 / **CARD-X(442) 收口**：栈空闲。", "- 2026-11-19 10:00 +0800 预留 / **CARD-X(442)**：owner=lead；"); Owner="lead"; Card="CARD-X(442)"; Pass=$false; ReasonContains="释放" },
    @{ Name="stale_event_head_window_regression"; Lines=@("# hdr", "- 2026-11-20 23:50 +0800 预留 / **OTHER-CARD(999)**：owner=other；", "- 2026-11-19 10:00 +0800 预留 / **CARD-X(442)**：owner=lead"); Owner="lead"; Card="CARD-X(442)"; Pass=$false; ReasonContains="owner mismatch" },
    @{ Name="wrong_owner_rejected"; Lines=@("# hdr", "- 2026-11-20 23:50 +0800 预留 / **CARD-X(442)**：owner=audiocpp439；"); Owner="lead"; Card="CARD-X(442)"; Pass=$false; ReasonContains="owner mismatch" },
    @{ Name="wrong_card_rejected"; Lines=@("# hdr", "- 2026-11-20 23:50 +0800 预留 / **CARD-Y(1)**：owner=lead；"); Owner="lead"; Card="CARD-X(442)"; Pass=$false; ReasonContains="does not name expected card" },
    @{ Name="no_owner_field_rejected"; Lines=@("# hdr", "- 2026-11-20 23:50 +0800 预留 / **CARD-X(442)**：berth=x；"); Owner="lead"; Card="CARD-X(442)"; Pass=$false; ReasonContains="no owner= field" },
    @{ Name="ambiguous_owner_rejected"; Lines=@("# hdr", "- 2026-11-20 23:50 +0800 预留 / **CARD-X(442)**：owner=lead；owner=other；"); Owner="lead"; Card="CARD-X(442)"; Pass=$false; ReasonContains="ambiguous owner field" },
    @{ Name="no_expected_owner_fail_closed"; Lines=@("# hdr", "- 2026-11-20 23:50 +0800 预留 / **CARD-X(442)**：owner=lead；"); Owner=""; Card="CARD-X(442)"; Pass=$false; ReasonContains="no expected owner passed" },
    @{ Name="no_event_line_rejected"; Lines=@("# PC 真实运行栈占用记录", "some prose without events"); Owner="lead"; Card="CARD-X(442)"; Pass=$false; ReasonContains="no event line found" },
    @{ Name="no_startup_rights_rejected"; Lines=@("# hdr", "- 2026-11-20 23:50 +0800 预留 / **CARD-X(442)**：owner=lead；无启动权期间禁止启动。"); Owner="lead"; Card="CARD-X(442)"; Pass=$false; ReasonContains="无启动权" }
)

$verdictScriptNames = @(
    "run_entry_management_acceptance.ps1",
    "vitshell_smoke.ps1",
    "vitshell_zproduct_smoke.ps1"
)
$perCopyJson = @{}   # copy name -> array of verdict JSON strings (for cross-copy sync check)

foreach ($copyName in $verdictScriptNames) {
    $path = Join-Path $scriptsDir $copyName
    $fn = Get-ExtractedFunction -Path $path -FunctionName "Get-StackLeaseVerdict"
    if ($null -eq $fn) { $failures += ("Get-StackLeaseVerdict not found in " + $copyName); Write-Fail ("Get-StackLeaseVerdict not found in " + $copyName); continue }
    . $fn
    $jsons = @()
    foreach ($case in $verdictCases) {
        $verdict = $null
        try { $verdict = Get-StackLeaseVerdict -LeaseLines $case.Lines -ExpectedOwner $case.Owner -ExpectedCard $case.Card } catch {
            $failures += ($case.Name + " [" + $copyName + "] threw: " + $_.Exception.Message); Write-Fail ($case.Name + " [" + $copyName + "] threw: " + $_.Exception.Message); continue
        }
        $jsons += (ConvertTo-Json -InputObject $verdict -Depth 5 -Compress)
        if ($verdict.Pass -ne $case.Pass) {
            $failures += ($case.Name + " [" + $copyName + "] Pass=" + $verdict.Pass + " expected " + $case.Pass + "; reasons=" + (($verdict.Reasons) -join " | "))
            Write-Fail ($case.Name + " [" + $copyName + "] Pass=" + $verdict.Pass + " expected " + $case.Pass)
            continue
        }
        if ($case.Pass) {
            if (-not [string]::IsNullOrEmpty($case.State) -and $verdict.State -ne $case.State) {
                $failures += ($case.Name + " [" + $copyName + "] State=" + $verdict.State + " expected " + $case.State); Write-Fail ($case.Name + " [" + $copyName + "] state mismatch"); continue
            }
            if (-not [string]::IsNullOrEmpty($case.OwnerValue) -and $verdict.Owner -cne $case.OwnerValue) {
                $failures += ($case.Name + " [" + $copyName + "] Owner=[" + $verdict.Owner + "] expected [" + $case.OwnerValue + "]"); Write-Fail ($case.Name + " [" + $copyName + "] owner extract mismatch"); continue
            }
            if (-not $verdict.CardHit) {
                $failures += ($case.Name + " [" + $copyName + "] CardHit=false on a grant case"); Write-Fail ($case.Name + " [" + $copyName + "] CardHit=false"); continue
            }
        } else {
            if (-not (($verdict.Reasons) -join " | ").Contains($case.ReasonContains)) {
                $failures += ($case.Name + " [" + $copyName + "] reasons missing needle [" + $case.ReasonContains + "]: " + (($verdict.Reasons) -join " | "))
                Write-Fail ($case.Name + " [" + $copyName + "] reason needle missing"); continue
            }
        }
    }
    $perCopyJson[$copyName] = $jsons
}

# cross-copy sync: all three copies must produce identical verdicts per case
# (excluding the fail-closed case, whose hint names each script's own parameter).
$referenceCopy = "run_entry_management_acceptance.ps1"
if ($perCopyJson.ContainsKey($referenceCopy) -and $perCopyJson[$referenceCopy].Count -eq $verdictCases.Count) {
    foreach ($copyName in $verdictScriptNames) {
        if ($copyName -eq $referenceCopy) { continue }
        if (-not $perCopyJson.ContainsKey($copyName) -or $perCopyJson[$copyName].Count -ne $verdictCases.Count) { continue }
        for ($i = 0; $i -lt $verdictCases.Count; $i++) {
            if ($verdictCases[$i].Name -eq "no_expected_owner_fail_closed") { continue }
            if ($perCopyJson[$copyName][$i] -cne $perCopyJson[$referenceCopy][$i]) {
                $failures += ("verdict drift between " + $referenceCopy + " and " + $copyName + " on case " + $verdictCases[$i].Name)
                Write-Fail ("verdict drift on " + $verdictCases[$i].Name + " between copies")
            }
        }
    }
    Write-Ok ("Get-StackLeaseVerdict case table executed on " + $verdictScriptNames.Count + " extracted copies (" + $verdictCases.Count + " cases each, cross-copy sync checked)")
} else {
    $failures += "case table incomplete on reference copy (a case threw); sync check skipped"
}

# ---------------------------------------------------------------------------
# Step 3: Get-LatestOccupancyEvent (head-window scripts) case table.
# ---------------------------------------------------------------------------
Write-Step "3/4 Get-LatestOccupancyEvent case table (2 extracted copies must agree)"
$locatorCases = @(
    @{ Name="latest_first_newest_first"; Lines=@("# hdr", "- 2026-11-20 23:50 +0800 预留 / **NEW(2)**：owner=a", "- 2026-11-19 10:00 +0800 预留 / **OLD(1)**：owner=b"); Expected="- 2026-11-20 23:50 +0800 预留 / **NEW(2)**：owner=a" },
    @{ Name="leading_whitespace_trimmed"; Lines=@("# hdr", "  - 2026-11-20 23:50 预留 / **NEW(2)**：owner=a"); Expected="- 2026-11-20 23:50 预留 / **NEW(2)**：owner=a" },
    @{ Name="crlf_stripped"; Lines=@("# hdr", ("- 2026-11-20 23:50 预留 / **NEW(2)**：owner=a" + [string][char]13)); Expected="- 2026-11-20 23:50 预留 / **NEW(2)**：owner=a" },
    @{ Name="prose_dash_ignored_no_space"; Lines=@("# hdr", "-prose-ish line", "- 2026-11-20 23:50 预留 / **NEW(2)**：owner=a"); Expected="- 2026-11-20 23:50 预留 / **NEW(2)**：owner=a" },
    @{ Name="no_event_returns_null"; Lines=@("# hdr", "prose only"); Expected=$null }
)
foreach ($copyName in @("vitshell_full_smoke.ps1", "vitshell_open_chain_smoke.ps1")) {
    $path = Join-Path $scriptsDir $copyName
    $fn = Get-ExtractedFunction -Path $path -FunctionName "Get-LatestOccupancyEvent"
    if ($null -eq $fn) { $failures += ("Get-LatestOccupancyEvent not found in " + $copyName); Write-Fail ("Get-LatestOccupancyEvent not found in " + $copyName); continue }
    . $fn
    foreach ($case in $locatorCases) {
        $got = Get-LatestOccupancyEvent -LedgerLines $case.Lines
        if ($case.Expected -eq $null) {
            if ($null -ne $got) { $failures += ($case.Name + " [" + $copyName + "] expected null, got: " + $got); Write-Fail ($case.Name + " [" + $copyName + "] expected null") }
        } elseif ($got -cne $case.Expected) {
            $failures += ($case.Name + " [" + $copyName + "] got [" + $got + "] expected [" + $case.Expected + "]"); Write-Fail ($case.Name + " [" + $copyName + "] locator mismatch")
        }
    }
    Write-Ok ("Get-LatestOccupancyEvent cases executed on " + $copyName)
}

# ---------------------------------------------------------------------------
# Step 4: hardcoded-owner scan across scripts/*.ps1 (grep evidence).
# The selftest itself is excluded: its synthetic ledger fixtures and needle list
# legitimately contain owner records as TEST DATA; the scan targets device/gate
# scripts, which must carry zero hardcoded owner strings.
# ---------------------------------------------------------------------------
Write-Step "4/4 hardcoded-owner scan (owner=Codex / /root/entry388 / /root/restore403 must be absent in device scripts)"
$needles = @('owner=Codex', '/root/entry388', '/root/restore403')
$scanTargets = @(Get-ChildItem -LiteralPath $scriptsDir -Filter "*.ps1" | Where-Object { $_.Name -ne "stack_lease_gate_selftest.ps1" })
$hits = @($scanTargets | Select-String -SimpleMatch -Pattern $needles)
if ($hits.Count -gt 0) {
    foreach ($hit in $hits) { $failures += ("hardcoded owner string at " + $hit.Path + ":" + $hit.LineNumber + ": " + $hit.Line.Trim()); Write-Fail ($hit.Path + ":" + $hit.LineNumber) }
} else {
    Write-Ok ("hardcoded owner strings: zero hits in scripts/*.ps1 (selftest excluded: " + $scanTargets.Count + " device scripts scanned)")
}

# ---------------------------------------------------------------------------
# Report + exit code.
# ---------------------------------------------------------------------------
$report = [ordered]@{
    schema_version = "stack_lease_gate_selftest.v1"
    card = "STACKLEASE-OWNER-PARAM-1"
    run_root = $RunRoot
    changed_scripts = $changedScripts
    verdict_cases = $verdictCases.Count
    locator_cases = $locatorCases.Count
    hardcoded_scan_needles = $needles
    failures = $failures
    status = $(if ($failures.Count -eq 0) { "passed" } else { "failed" })
    finished_at = (Get-Date).ToString("o")
}
$report | ConvertTo-Json -Depth 5 | Out-File -LiteralPath (Join-Path $RunRoot "selftest_report.json") -Encoding utf8

Write-Host ""
if ($failures.Count -eq 0) {
    Write-Ok "stack lease gate selftest PASSED (exit 0)"
    exit 0
}
Write-Fail ("stack lease gate selftest FAILED with " + $failures.Count + " failure(s):")
foreach ($failure in $failures) { Write-Host ("  - " + $failure) -ForegroundColor Red }
exit 1

# 296 R3 D2 assertion branch-aware dry run (offline, no stack)
# Slices the VERBATIM shipped assertion block out of the real ps1 and feeds it
# constructed branch shapes under Set-StrictMode -Version Latest (same as the
# live script). Pass shapes must pass both checks; negative shapes must fail.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$scriptPath = 'D:\Vit_DAW_wt_296r2\scripts\run_free_state_d1_smoke.ps1'
$lines = Get-Content -LiteralPath $scriptPath
$startIdx = -1; $endIdx = -1
for ($i = 0; $i -lt $lines.Count; $i++) {
    if ($lines[$i] -eq '    $d2TrackRows = @()') { $startIdx = $i }
    if ($lines[$i] -eq '    $d2Row1007 = $null') { $endIdx = $i; break }
}
if (($startIdx -lt 0) -or ($endIdx -lt 0)) { throw "slice markers not found (start=$startIdx end=$endIdx)" }
$slice = ($lines[$startIdx..($endIdx - 1)]) -join "`n"
Write-Output ("slice: lines " + ($startIdx + 1) + ".." + $endIdx + " (" + ($endIdx - $startIdx) + " lines) from " + $scriptPath)

function New-Row([string]$id, [string]$status, $corr, $balance) {
    $o = [ordered]@{ track_id = $id; status = $status }
    if ($null -ne $corr) { $o['correlation_estimate'] = $corr }
    if ($null -ne $balance) { $o['balance_db'] = $balance }
    return ($o | ConvertTo-Json | ConvertFrom-Json)
}
function New-Obs($considered, $rows) {
    $payload = @{ tracks_considered = $considered; track_stereo = $rows } | ConvertTo-Json -Depth 6
    return ($payload | ConvertFrom-Json)
}
$sixReady = @(
    (New-Row '1007' 'read' 0.689 -1.475), (New-Row '1012' 'read' 0.974 0.039),
    (New-Row '1017' 'read' 0.227 0.676), (New-Row '1022' 'read' 0.225 -0.597),
    (New-Row '1027' 'read' 0.088 0.887), (New-Row '1032' 'read' 0.979 -0.085)
)

# T1: zero-mutation + run2 verbatim face (loaded from the run2 evidence file).
$run2Obs = (Get-Content -LiteralPath 'D:\Vit_DAW_wt_296r2\coord\runs\D1D3-REGRESS-STACK-1\run2\d2_check_chat.json' -Raw -Encoding UTF8 | ConvertFrom-Json).workflow_data.observation
# T2/T3: 1-mutation pass shapes (7/7/6/1, no-metric row = runner-created track, id auto-assigned).
$seven1039 = New-Obs 7 ($sixReady + @((New-Row '1039' 'read' $null $null)))
$seven1043 = New-Obs 7 ($sixReady + @((New-Row '1043' 'read' $null $null)))
# T4: zero-mutation fed the 7/7/6/1 face (branch mismatch).
# T5: zero-mutation fed the pre-292 face (6 rows, all missing, no metrics).
$sixMissing = @(
    (New-Row '1007' 'missing' $null $null), (New-Row '1012' 'missing' $null $null),
    (New-Row '1017' 'missing' $null $null), (New-Row '1022' 'missing' $null $null),
    (New-Row '1027' 'missing' $null $null), (New-Row '1032' 'missing' $null $null)
)
$obsSixMissing = New-Obs 6 $sixMissing
# T6: 1-mutation fed 6/6/6/0 (runner track row absent).
# T7: 1-mutation fed 7/7/6/1 but no-metric row status=missing (pre-292 shape).
$sevenMissingStatus = New-Obs 7 ($sixReady + @((New-Row '1039' 'missing' $null $null)))
# T8: 1-mutation fed 7/7/6/1 but no-metric row is a FIXTURE id (regression shape).
$sevenFixtureNoMetric = New-Obs 7 ($sixReady + @((New-Row '1007' 'read' $null $null)))

$cases = @(
    @{ name = 'T1 zero-mutation + run2-verbatim 6/6/6/0 all-read'; expect = 'PASS'; zero = $true;  branch = 'zero_mutation_boundary'; obs = $run2Obs },
    @{ name = 'T2 1-mutation 7/7/6/1 no-metric id=1039 status=read'; expect = 'PASS'; zero = $false; branch = ''; obs = $seven1039 },
    @{ name = 'T3 1-mutation 7/7/6/1 no-metric id=1043 status=read (non-1039)'; expect = 'PASS'; zero = $false; branch = ''; obs = $seven1043 },
    @{ name = 'T4 zero-mutation fed 7/7/6/1 face (branch mismatch)'; expect = 'FAIL'; zero = $true;  branch = 'zero_mutation_boundary'; obs = $seven1039 },
    @{ name = 'T5 zero-mutation fed pre-292 face (6 rows all missing)'; expect = 'FAIL'; zero = $true;  branch = 'zero_mutation_boundary'; obs = $obsSixMissing },
    @{ name = 'T6 1-mutation fed 6/6/6/0 (runner track row absent)'; expect = 'FAIL'; zero = $false; branch = ''; obs = $run2Obs },
    @{ name = 'T7 1-mutation no-metric row status=missing (pre-292)'; expect = 'FAIL'; zero = $false; branch = ''; obs = $sevenMissingStatus },
    @{ name = 'T8 1-mutation no-metric row track_id=1007 (fixture id)'; expect = 'FAIL'; zero = $false; branch = ''; obs = $sevenFixtureNoMetric }
)

$verdicts = @()
foreach ($case in $cases) {
    Write-Output ("== " + $case.name + " (expect " + $case.expect + ") ==")
    $d2Observation = $case.obs
    $d2Report = @{}
    $d2Failures = New-Object System.Collections.Generic.List[string]
    $zeroMutationBranch = [bool]$case.zero
    $d1ValidationBranch = [string]$case.branch
    $d2CountsOK = $false; $d2ShapeOK = $false; $d2TracksFaceOK = $false
    Invoke-Expression $slice
    $result = if ($d2TracksFaceOK -and ($d2Failures.Count -eq 0)) { 'PASS' } elseif ((-not $d2TracksFaceOK) -and ($d2Failures.Count -gt 0)) { 'FAIL' } else { 'INCONSISTENT' }
    Write-Output ("   countsOK=" + $d2CountsOK + " shapeOK=" + $d2ShapeOK + " tracksFaceOK=" + $d2TracksFaceOK + " failures=" + $d2Failures.Count + " => " + $result)
    foreach ($f in $d2Failures) { Write-Output ("   [failure] " + $f) }
    $verdicts += ,@($case.name, $case.expect, $result)
}
Write-Output ""
$allOk = $true
foreach ($v in $verdicts) {
    $ok = ($v[1] -eq $v[2])
    if (-not $ok) { $allOk = $false }
    Write-Output (($v[0]) + "  expect=" + $v[1] + " got=" + $v[2] + "  " + $(if ($ok) { 'OK' } else { 'MISMATCH' }))
}
Write-Output ("DRYRUN_VERDICT: " + $(if ($allOk) { 'ALL_SHAPES_AS_EXPECTED' } else { 'SHAPE_MISMATCH' }))

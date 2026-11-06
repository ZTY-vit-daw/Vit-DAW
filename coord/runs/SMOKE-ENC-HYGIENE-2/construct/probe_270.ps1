# probe_270.ps1 -- SMOKE-ENC-HYGIENE-2 construct validation
# Card: two residual unencoded Get-Content -Raw reads in
#   scripts/run_free_state_d1_smoke.ps1 (:445 LW manifest, :630 NOT_EXERCISED reason).
# Legs: per read point, pre-fix line (extracted from baseline snapshot) vs post-fix line
#   (extracted from worktree snapshot) against the SAME UTF-8-no-BOM Chinese stub file.
# Stub contents are built from explicit UTF-8 byte arrays so this probe file stays
# pure ASCII (no dependence on the encoding powershell.exe assumes for this file).
# Zero real stack, zero LLM, offline files only. Idempotent: each run writes a fresh
# out dir supplied by the caller.
#
# Exit code: 0 = ALL PASS, 1 = any leg failed.
param(
    [Parameter(Mandatory = $true)][string]$OutDir,
    [Parameter(Mandatory = $true)][string]$PreScript,
    [Parameter(Mandatory = $true)][string]$PostScript
)

$ErrorActionPreference = "Stop"
$failures = New-Object System.Collections.Generic.List[string]
$legResults = New-Object System.Collections.Generic.List[object]

function New-Utf8NoBomFile {
    param([string]$Path, [byte[]]$Bytes)
    $dir = Split-Path -Parent $Path
    if (-not (Test-Path -LiteralPath $dir)) { New-Item -ItemType Directory -Path $dir -Force | Out-Null }
    [System.IO.File]::WriteAllBytes($Path, $Bytes)
}

function Get-ScriptLine {
    # Extract the exact single source line from a script snapshot (byte-identical text
    # of what the script executes; both target lines are pure ASCII so extraction is
    # encoding-safe).
    param([string]$ScriptPath, [string]$Pattern, [string]$Label)
    $hits = @(Select-String -LiteralPath $ScriptPath -Pattern $Pattern)
    if ($hits.Count -ne 1) {
        throw ("line extraction failed for {0}: pattern '{1}' matched {2} lines (expected exactly 1)" -f $Label, $Pattern, $hits.Count)
    }
    return $hits[0].Line.Trim()
}

# ---- stub byte construction (UTF-8, no BOM) ----
# project_path value: "D:/工程桩/样例_混音_测试.json"
$zhPathBytes = [byte[]](
    0x44,0x3A,0x2F,
    0xE5,0xB7,0xA5, 0xE7,0xA8,0x8B, 0xE6,0xA1,0x81, 0x2F,
    0xE6,0xA0,0xB7, 0xE4,0xBE,0x8B, 0x5F,
    0xE6,0xB7,0xB7, 0xE9,0x9F,0xB3, 0x5F,
    0xE6,0xB5,0x8B, 0xE8,0xAF,0x95,
    0x2E,0x6A,0x73,0x6F,0x6E
)
# reason value: "预算上限已到：第 2 轮未通过"
$zhReasonBytes = [byte[]](
    0xE9,0xA2,0x84, 0xE7,0xAE,0x97, 0xE4,0xB8,0x8A, 0xE9,0x99,0x90, 0xE5,0xB7,0xB2, 0xE5,0x88,0xB0,
    0xEF,0xBC,0x9A, 0xE7,0xAC,0xAC, 0x20,0x32,0x20,
    0xE8,0xBD,0xAE, 0xE6,0x9C,0xAA, 0xE9,0x80,0x9A, 0xE8,0xBF,0x87
)
$u8 = New-Object System.Text.UTF8Encoding($false)
$expectedPath   = [string]$u8.GetString($zhPathBytes)
$expectedReason = [string]$u8.GetString($zhReasonBytes)

# manifest stub: {"cases":[{"public_case_id":"case_zh_enc_270","project_path":"<zh>"}]}
$manHead = [System.Text.Encoding]::ASCII.GetBytes('{"cases":[{"public_case_id":"case_zh_enc_270","project_path":"')
$manTail = [System.Text.Encoding]::ASCII.GetBytes('"}]}')
$manifestBytes = New-Object System.Collections.Generic.List[byte]
$manifestBytes.AddRange($manHead); $manifestBytes.AddRange($zhPathBytes); $manifestBytes.AddRange($manTail)

# reason stub: {"branch":"","reason":"<zh>"}  (d1_smoke_report.json NOT_EXERCISED face)
$rHead = [System.Text.Encoding]::ASCII.GetBytes('{"branch":"","reason":"')
$rTail = [System.Text.Encoding]::ASCII.GetBytes('"}')
$reportBytes = New-Object System.Collections.Generic.List[byte]
$reportBytes.AddRange($rHead); $reportBytes.AddRange($zhReasonBytes); $reportBytes.AddRange($rTail)

# ---- line extraction from snapshots ----
# Patterns anchor on the assignment target: $report is also read (with -Encoding) at
# :665 by a different statement, so a bare 'Get-Content -LiteralPath $report' pattern
# is ambiguous. Anchoring on the assignment makes the extracted line the exact full
# source statement, which Invoke-Expression then executes verbatim.
$preManifestLine  = Get-ScriptLine -ScriptPath $PreScript  -Pattern '\$lwManifest = Get-Content -LiteralPath \$PublicManifest -Raw' -Label "pre :445"
$postManifestLine = Get-ScriptLine -ScriptPath $PostScript -Pattern '\$lwManifest = Get-Content -LiteralPath \$PublicManifest -Raw' -Label "post :445"
$preReasonLine    = Get-ScriptLine -ScriptPath $PreScript  -Pattern '\$notExercisedReason = \[string\]\(\(Get-Content -LiteralPath \$report -Raw' -Label "pre :630"
$postReasonLine   = Get-ScriptLine -ScriptPath $PostScript -Pattern '\$notExercisedReason = \[string\]\(\(Get-Content -LiteralPath \$report -Raw' -Label "post :630"

Write-Host ("[probe] pre  :445 line: " + $preManifestLine)
Write-Host ("[probe] post :445 line: " + $postManifestLine)
Write-Host ("[probe] pre  :630 line: " + $preReasonLine)
Write-Host ("[probe] post :630 line: " + $postReasonLine)

if ($preManifestLine -eq $postManifestLine) { throw "pre/post :445 lines identical - snapshot mixup" }
if ($preReasonLine -eq $postReasonLine)     { throw "pre/post :630 lines identical - snapshot mixup" }
if ($postManifestLine -notmatch '-Encoding\s+UTF8') { throw "post :445 line lacks -Encoding UTF8" }
if ($postReasonLine   -notmatch '-Encoding\s+UTF8') { throw "post :630 line lacks -Encoding UTF8" }
if ($preManifestLine -match '-Encoding')  { throw "pre :445 line unexpectedly has -Encoding" }
if ($preReasonLine   -match '-Encoding')  { throw "pre :630 line unexpectedly has -Encoding" }

function Add-Leg {
    param([string]$Name, [bool]$Pass, [string]$Form, [string]$Detail)
    $script:legResults.Add([pscustomobject]@{ leg = $Name; pass = $Pass; form = $Form; detail = $Detail })
    # Console detail uses Unicode code-point escapes so the transcript stays pure
    # ASCII and free of console-codepage display ambiguity; the summary object
    # (ConvertTo-Json) additionally carries the raw strings.
    if ($Pass) { Write-Host ("[PASS] {0}  form={1}  {2}" -f $Name, $Form, (To-Utf16Escape $Detail)) }
    else {
        Write-Host ("[FAIL] {0}  form={1}  {2}" -f $Name, $Form, (To-Utf16Escape $Detail))
        $script:failures.Add($Name)
    }
}

function To-Utf16Escape {
    param([string]$s)
    $sb = New-Object System.Text.StringBuilder
    foreach ($ch in $s.ToCharArray()) {
        if ([int]$ch -gt 126 -or [int]$ch -lt 32) { [void]$sb.Append(("\u{0:x4}" -f [int]$ch)) }
        else { [void]$sb.Append($ch) }
    }
    return $sb.ToString()
}

# ---- leg1: manifest read, pre-fix line, UTF-8 no-BOM Chinese stub ----
# Expected defect: parse throw (script swallows it into config_error at :528) or
# mojibake project_path. Leg passes if the observed value differs from the stub source.
$legDir = Join-Path $OutDir "leg1_manifest_prefix"
New-Item -ItemType Directory -Path $legDir -Force | Out-Null
$stubManifest1 = Join-Path $legDir "fixture_manifest.json"
New-Utf8NoBomFile -Path $stubManifest1 -Bytes $manifestBytes.ToArray()
$PublicManifest = $stubManifest1
$lwManifest = $null
$gotPath = ""
$form1 = ""
try {
    Invoke-Expression $preManifestLine
    $gotPath = [string](@($lwManifest.cases)[0].project_path)
} catch {
    $form1 = "parse_throw(" + $_.Exception.GetType().Name + ")"
    $gotPath = ""
}
Copy-Item -LiteralPath $stubManifest1 -Destination (Join-Path $legDir "stub_bytes_copy.json") -Force
if ($form1 -eq "") { $form1 = "mojibake_or_wrong_value" }
$defectSeen1 = ($gotPath -ne $expectedPath)
Add-Leg -Name "leg1_manifest_prefix" -Pass $defectSeen1 -Form $form1 `
    -Detail ("got=[" + $gotPath + "] expected=[" + $expectedPath + "]")

# ---- leg2: manifest read, post-fix line, same stub ----
$legDir = Join-Path $OutDir "leg2_manifest_postfix"
New-Item -ItemType Directory -Path $legDir -Force | Out-Null
$stubManifest2 = Join-Path $legDir "fixture_manifest.json"
New-Utf8NoBomFile -Path $stubManifest2 -Bytes $manifestBytes.ToArray()
$PublicManifest = $stubManifest2
$lwManifest = $null
$gotPath = ""
$form2 = ""
try {
    Invoke-Expression $postManifestLine
    $gotPath = [string](@($lwManifest.cases)[0].project_path)
} catch {
    $form2 = "parse_throw(" + $_.Exception.GetType().Name + ")"
    $gotPath = ""
}
if ($form2 -eq "") { $form2 = "clean_parse" }
$pass2 = (($form2 -ne "mojibake_or_wrong_value") -and ($gotPath -eq $expectedPath))
Add-Leg -Name "leg2_manifest_postfix" -Pass $pass2 -Form $form2 `
    -Detail ("got=[" + $gotPath + "] expected=[" + $expectedPath + "]")

# ---- leg3: NOT_EXERCISED reason read, pre-fix line, same-shaped stub ----
# Mirrors script shape :629-:633 exactly: try { <line> } catch { }  -- the script's
# empty catch swallows a ConvertFrom-Json throw, leaving the reason blank.
$legDir = Join-Path $OutDir "leg3_reason_prefix"
New-Item -ItemType Directory -Path $legDir -Force | Out-Null
$stubReport3 = Join-Path $legDir "d1_smoke_report.json"
New-Utf8NoBomFile -Path $stubReport3 -Bytes $reportBytes.ToArray()
$report = $stubReport3
$notExercisedReason = ""
$form3 = ""
try {
    Invoke-Expression $preReasonLine
} catch {
    $form3 = "swallowed_by_empty_catch"
}
Copy-Item -LiteralPath $stubReport3 -Destination (Join-Path $legDir "stub_bytes_copy.json") -Force
if ($form3 -eq "") { $form3 = "mojibake_or_wrong_value" }
$defectSeen3 = ([string]$notExercisedReason -ne $expectedReason)
Add-Leg -Name "leg3_reason_prefix" -Pass $defectSeen3 -Form $form3 `
    -Detail ("got=[" + [string]$notExercisedReason + "] expected=[" + $expectedReason + "]")

# ---- leg4: NOT_EXERCISED reason read, post-fix line, same stub, same try/catch shape ----
$legDir = Join-Path $OutDir "leg4_reason_postfix"
New-Item -ItemType Directory -Path $legDir -Force | Out-Null
$stubReport4 = Join-Path $legDir "d1_smoke_report.json"
New-Utf8NoBomFile -Path $stubReport4 -Bytes $reportBytes.ToArray()
$report = $stubReport4
$notExercisedReason = ""
$form4 = ""
try {
    Invoke-Expression $postReasonLine
} catch {
    $form4 = "swallowed_by_empty_catch"
}
if ($form4 -eq "") { $form4 = "clean_parse" }
$pass4 = (($form4 -eq "clean_parse") -and ([string]$notExercisedReason -eq $expectedReason))
Add-Leg -Name "leg4_reason_postfix" -Pass $pass4 -Form $form4 `
    -Detail ("got=[" + [string]$notExercisedReason + "] expected=[" + $expectedReason + "]")

# ---- leg5: static no-regression sweep (covers 268's four fixes + this card's two) ----
# Every Get-Content line in the post-fix script must carry -Encoding.
$legDir = Join-Path $OutDir "leg5_static_encoding_coverage"
New-Item -ItemType Directory -Path $legDir -Force | Out-Null
$allGc = @(Select-String -LiteralPath $PostScript -Pattern 'Get-Content')
$bad = @($allGc | Where-Object { $_.Line -notmatch '-Encoding\s' })
$allGc | ForEach-Object { ($_.Line.Trim()) } | Set-Content -LiteralPath (Join-Path $legDir "all_getcontent_lines_postfix.txt") -Encoding UTF8
$preBad = @((Select-String -LiteralPath $PreScript -Pattern 'Get-Content') | Where-Object { $_.Line -notmatch '-Encoding\s' })
$pass5 = (($bad.Count -eq 0) -and ($preBad.Count -eq 2))
Add-Leg -Name "leg5_static_encoding_coverage" -Pass $pass5 `
    -Form ("postfix_unencoded=" + $bad.Count + " prefix_unencoded=" + $preBad.Count) `
    -Detail ("total Get-Content lines postfix=" + $allGc.Count + "; expected exactly 2 unencoded in pre (268 four + this card two covered by postfix=0)")

# ---- summary ----
$summary = [pscustomobject]@{
    probe             = "probe_270"
    card              = "SMOKE-ENC-HYGIENE-2"
    date              = (Get-Date -Format "yyyy-MM-dd HH:mm:ss")
    pre_baseline      = $PreScript
    post_snapshot     = $PostScript
    stub_encoding     = "UTF-8 no BOM (byte-level constructed)"
    legs              = $legResults
    all_pass          = ($failures.Count -eq 0)
    failure_count     = $failures.Count
}
$summaryPath = Join-Path $OutDir "probe_270_summary.json"
$summary | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $summaryPath -Encoding UTF8
Write-Host ("[probe] summary: " + $summaryPath)

if ($failures.Count -gt 0) {
    Write-Host ("[probe] FAILED legs: " + ($failures -join ", "))
    exit 1
}
Write-Host "[probe] ALL PASS (5/5)"
exit 0

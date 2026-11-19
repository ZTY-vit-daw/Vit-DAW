[CmdletBinding()]
param(
    # Reset the W02-A1 hand-test fixture: <HandtestRoot>/pristine -> <HandtestRoot>/live.
    # Card: W02-A1-HANDTEST-V0 (pool 434). ASCII only so Windows PowerShell 5.1
    # decodes it even without a BOM.
    #
    # Scope (lead adjudication B, 2026-11-19): the HAND TEST is same-stack
    # (-SameStackPrep), and its reset is "re-run prep" in the running stack -
    # NOT this script. This file restore is only for the legacy reopen-mode
    # fixture, or for a manually placed <HandtestRoot>/live directory. A file
    # restore never brings the lesson session back (PROJ-OPEN-RESUME-1), so the
    # same-stack flow must re-run prep after any close/reopen.
    [Parameter(Mandatory = $true)][string]$HandtestRoot,
    [switch]$DryRun
)

# Exit codes: 0 = reset verified, 2 = environment/usage error (nothing silently half-done).
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Fail-Env {
    param([string]$Message)
    Write-Host ("error: " + $Message) -ForegroundColor Red
    exit 2
}

if ([string]::IsNullOrWhiteSpace($HandtestRoot)) { Fail-Env "-HandtestRoot is required." }
if (-not (Test-Path -LiteralPath $HandtestRoot)) { Fail-Env ("HandtestRoot does not exist: " + $HandtestRoot) }
$root = (Resolve-Path -LiteralPath $HandtestRoot).Path
if (-not (Test-Path -LiteralPath $root -PathType Container)) { Fail-Env ("HandtestRoot is not a directory: " + $root) }
# Refuse drive roots and any path that is not a real fixture root.
if ($root.Length -le 3 -or $root -match '^[A-Za-z]:\\?$') { Fail-Env ("refusing to reset a drive root: " + $root) }

$live = Join-Path $root 'live'
$pristine = Join-Path $root 'pristine'
foreach ($target in @($live, $pristine)) {
    $parent = Split-Path -Parent $target
    if ($parent -ne $root) { Fail-Env ("refusing to operate outside the fixture root: " + $target) }
}
if (-not (Test-Path -LiteralPath $pristine -PathType Container)) { Fail-Env ("pristine fixture missing: " + $pristine) }

$manifestPath = Join-Path $root 'handtest.json'
$projectFile = ''
if (Test-Path -LiteralPath $manifestPath) {
    try {
        $manifest = Get-Content -LiteralPath $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
        $projectFile = [string]$manifest.project_file
    }
    catch { $projectFile = '' }
}
if ([string]::IsNullOrWhiteSpace($projectFile)) {
    $vitFiles = @(Get-ChildItem -LiteralPath $pristine -File -Filter '*.vit' -ErrorAction SilentlyContinue)
    if ($vitFiles.Count -ne 1) { Fail-Env ("cannot determine the fixture project: pristine has " + [string]$vitFiles.Count + " .vit file(s) and handtest.json is absent/invalid") }
    $projectFile = $vitFiles[0].Name
}
$pristineProject = Join-Path $pristine $projectFile
if (-not (Test-Path -LiteralPath $pristineProject)) { Fail-Env ("pristine project missing: " + $pristineProject) }

$pristineSha = (Get-FileHash -Algorithm SHA256 -LiteralPath $pristineProject).Hash
$pristineFiles = @(Get-ChildItem -LiteralPath $pristine -Recurse -File -Force)
$pristineBytes = [long]0
foreach ($file in $pristineFiles) { $pristineBytes += [long]$file.Length }

Write-Host ("fixture root : " + $root)
Write-Host ("pristine     : " + $pristine + " (" + [string]$pristineFiles.Count + " files, " + [string]$pristineBytes + " bytes)")
Write-Host ("pristine .vit: " + $pristineProject + " sha256=" + $pristineSha.Substring(0, 12) + "..")
Write-Host ("live         : " + $live + " (exists=" + [string](Test-Path -LiteralPath $live) + ")")

if ($DryRun) {
    Write-Host "dry run: would delete live/ and re-copy pristine/ -> live/; nothing was changed." -ForegroundColor Yellow
    exit 0
}

# The stack (VitShell/DAW/agent) must be closed: a live project that is still
# open keeps handles and the delete fails. That failure is reported, not hidden.
if (Test-Path -LiteralPath $live) {
    try {
        Remove-Item -LiteralPath $live -Recurse -Force
    }
    catch {
        Fail-Env ("could not delete the live fixture (close VitShell / the DAW window and the prep stack first): " + $_.Exception.Message)
    }
}
New-Item -ItemType Directory -Path $live -Force | Out-Null
foreach ($entry in @(Get-ChildItem -LiteralPath $pristine -Force)) {
    Copy-Item -LiteralPath $entry.FullName -Destination (Join-Path $live $entry.Name) -Recurse -Force
}

# Verify: same .vit hash, same file count, same per-file digest.
$liveProject = Join-Path $live $projectFile
if (-not (Test-Path -LiteralPath $liveProject)) { Fail-Env ("live project missing after reset: " + $liveProject) }
$liveSha = (Get-FileHash -Algorithm SHA256 -LiteralPath $liveProject).Hash
$liveFiles = @(Get-ChildItem -LiteralPath $live -Recurse -File -Force)
$liveBytes = [long]0
foreach ($file in $liveFiles) { $liveBytes += [long]$file.Length }

function Get-TreeDigest {
    param([string]$Path)
    $rootPath = [System.IO.Path]::GetFullPath($Path)
    $lines = @()
    foreach ($file in @(Get-ChildItem -LiteralPath $rootPath -Recurse -File -Force | Sort-Object FullName)) {
        $relative = $file.FullName.Substring($rootPath.Length).TrimStart([char[]]@([char]92, [char]47))
        $lines += ($relative + ":" + (Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash)
    }
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        $hash = $sha.ComputeHash([System.Text.Encoding]::UTF8.GetBytes(($lines -join ([string][char]10))))
        return [System.BitConverter]::ToString($hash).Replace("-", "")
    }
    finally { $sha.Dispose() }
}

$pristineDigest = Get-TreeDigest -Path $pristine
$liveDigest = Get-TreeDigest -Path $live
if ($liveSha -ne $pristineSha) { Fail-Env ("live .vit hash mismatch after reset: " + $liveSha + " != " + $pristineSha) }
if ($liveFiles.Count -ne $pristineFiles.Count) { Fail-Env ("live/pristine file count mismatch: " + [string]$liveFiles.Count + " != " + [string]$pristineFiles.Count) }
if ($liveDigest -ne $pristineDigest) { Fail-Env ("live/pristine tree digest mismatch: " + $liveDigest + " != " + $pristineDigest) }

Write-Host ("reset ok: live=" + $live + " files=" + [string]$liveFiles.Count + " bytes=" + [string]$liveBytes + " .vit sha256=" + $liveSha.Substring(0, 12) + "..") -ForegroundColor Green
Write-Host "Next: start VitShell (product entry), open the live .vit, select the PREP conversation (not a new conversation), select the second clip, say 写好了."
Write-Host "Reminder: same-stack hand test (-SameStackPrep) resets by RE-RUNNING prep; a file restore does not restore the lesson session."
exit 0

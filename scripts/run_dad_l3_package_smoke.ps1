[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$KernelExe = "",
    [string]$MaterialPath = "",
    [string]$PythonExe = "python",
    [string]$ZmqReqPort = "5555",
    [string]$ZmqSubPort = "5556",
    [switch]$ReuseKernel,
    [switch]$KeepProcess,
    [int]$WaitSeconds = 45
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$probeRoot = Join-Path $RepoRoot "VitApp\Workspace\Artifacts\dad_probe"
$before = @{}
if (Test-Path -LiteralPath $probeRoot) {
    Get-ChildItem -LiteralPath $probeRoot -Filter dad_probe_summary.json -Recurse -ErrorAction SilentlyContinue | ForEach-Object {
        $before[$_.FullName] = $true
    }
}

$kernelSmoke = Join-Path $RepoRoot "scripts\run_kernel_dad_probe_smoke.ps1"
$probeArgs = @{
    RepoRoot = $RepoRoot
    PythonExe = $PythonExe
    ZmqReqPort = $ZmqReqPort
    ZmqSubPort = $ZmqSubPort
    Features = "waveform_envelope,spectral_field,l3_acoustic_summary"
    WaitSeconds = $WaitSeconds
}
if (-not [string]::IsNullOrWhiteSpace($KernelExe)) {
    $probeArgs["KernelExe"] = $KernelExe
}
if (-not [string]::IsNullOrWhiteSpace($MaterialPath)) {
    $probeArgs["MaterialPath"] = $MaterialPath
}
if ($ReuseKernel) {
    $probeArgs["ReuseKernel"] = $true
}
if ($KeepProcess) {
    $probeArgs["KeepProcess"] = $true
}

& $kernelSmoke @probeArgs
if ($LASTEXITCODE -ne 0) {
    throw "kernel DAD probe smoke failed with exit code $LASTEXITCODE"
}

$summary = Get-ChildItem -LiteralPath $probeRoot -Filter dad_probe_summary.json -Recurse -ErrorAction SilentlyContinue |
    Where-Object { -not $before.ContainsKey($_.FullName) } |
    Sort-Object LastWriteTime -Descending |
    Select-Object -First 1
if ($null -eq $summary) {
    $summary = Get-ChildItem -LiteralPath $probeRoot -Filter dad_probe_summary.json -Recurse -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1
}
if ($null -eq $summary) {
    throw "dad_probe_summary.json was not produced"
}

$report = Get-Content -LiteralPath $summary.FullName -Raw | ConvertFrom-Json
$l3 = @($report.features | Where-Object { $_.feature_type -eq "l3_acoustic_summary" } | Select-Object -First 1)
if ($l3.Count -eq 0) {
    throw "l3_acoustic_summary feature report missing: $($summary.FullName)"
}
if ($l3[0].status -notin @("ready", "suspect")) {
    throw "l3_acoustic_summary status is not ready/suspect: $($l3[0].status)"
}
$observed = @($l3[0].observed_features)
foreach ($required in @("band_energy_summary", "stereo_relation_summary", "loudness_summary")) {
    if ($observed -notcontains $required) {
        throw "missing L3 feature $required in $($summary.FullName)"
    }
    $featureProperty = $l3[0].per_feature.PSObject.Properties[$required]
    $feature = if ($null -ne $featureProperty) { $featureProperty.Value } else { $null }
    if ($null -eq $feature -or [string]::IsNullOrWhiteSpace([string]$feature.evidence_ref)) {
        throw "missing evidence_ref for $required in $($summary.FullName)"
    }
}

# The product-path band payload is the upstream source for the bounded DOM
# evidence. Keep these fields in the probe contract so a summary projection
# cannot silently hide a producer regression.
$bandEvents = @($l3[0].events | ForEach-Object { $_.event } | Where-Object { $_.feature_type -eq "band_energy_summary" })
if ($bandEvents.Count -eq 0) {
    throw "no band_energy_summary event payload was retained in $($summary.FullName)"
}
$bandEvent = $bandEvents[0]
foreach ($requiredBoundedFact in @("noise_floor_evidence", "frequency_time_events", "transient_events", "band_dynamics")) {
    if (-not ($bandEvent.PSObject.Properties.Name -contains $requiredBoundedFact)) {
        throw "missing bounded producer fact $requiredBoundedFact in $($summary.FullName)"
    }
}

Write-Host ("ok: DAD L3 package smoke passed; summary=" + $summary.FullName) -ForegroundColor Green

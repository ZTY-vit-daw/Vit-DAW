[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [switch]$IncludeSmokeObservationCache
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function New-Row {
    param(
        [string]$Path,
        [string]$Kind,
        [string]$Status,
        [string]$Message = ""
    )
    [pscustomobject]@{
        path = $Path
        kind = $Kind
        status = $Status
        message = $Message
    }
}

$artifactRoot = Join-Path $RepoRoot "VitApp\Workspace\Artifacts"
$targets = @(
    @{ Path = Join-Path $artifactRoot "mixboard_feature_snapshot.json"; Kind = "file" },
    @{ Path = Join-Path $artifactRoot "acoustic_package_status.json"; Kind = "file" },
    @{ Path = Join-Path $artifactRoot "mixboard"; Kind = "directory" }
)

if ($IncludeSmokeObservationCache) {
    $targets += @{ Path = Join-Path $artifactRoot "smoke\acoustic_source_identity"; Kind = "directory" }
}

$rows = New-Object System.Collections.Generic.List[object]
foreach ($target in $targets) {
    $path = [string]$target.Path
    $kind = [string]$target.Kind
    if ([string]::IsNullOrWhiteSpace($path)) {
        continue
    }
    $resolvedRoot = [System.IO.Path]::GetFullPath($artifactRoot)
    $resolvedPath = [System.IO.Path]::GetFullPath($path)
    if (-not $resolvedPath.StartsWith($resolvedRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
        $rows.Add((New-Row -Path $path -Kind $kind -Status "skipped" -Message "outside_artifact_root"))
        continue
    }
    if (-not (Test-Path -LiteralPath $resolvedPath)) {
        $rows.Add((New-Row -Path $resolvedPath -Kind $kind -Status "missing"))
        continue
    }
    if ($PSCmdlet.ShouldProcess($resolvedPath, "Remove generated acoustic observation cache")) {
        Remove-Item -LiteralPath $resolvedPath -Force -Recurse
        $rows.Add((New-Row -Path $resolvedPath -Kind $kind -Status "removed"))
    }
}

[pscustomobject]@{
    schema_version = "acoustic_observation_cleanup.v0"
    repo_root = [System.IO.Path]::GetFullPath($RepoRoot)
    artifact_root = [System.IO.Path]::GetFullPath($artifactRoot)
    include_smoke_observation_cache = [bool]$IncludeSmokeObservationCache
    updated_at = (Get-Date).ToUniversalTime().ToString("o")
    rows = $rows
} | ConvertTo-Json -Depth 8

param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$Artifact = "D:\Vit_DAW\VitApp\Workspace\Artifacts\com_evidence\com2_b22af9ec7b5d41e18043db06\com2_b22af9ec7b5d41e18043db06.json",
    [string]$Output = ""
)

$ErrorActionPreference = "Stop"
$resolvedRepo = (Resolve-Path -LiteralPath $RepoRoot).Path
$resolvedArtifact = (Resolve-Path -LiteralPath $Artifact).Path
if ([string]::IsNullOrWhiteSpace($Output)) {
    $Output = Join-Path (Split-Path -Parent $resolvedArtifact) "com3_derivation_report.json"
}

python (Join-Path $resolvedRepo "scripts\compressor_behavior_derivation_smoke.py") `
    --repo-root $resolvedRepo `
    --artifact $resolvedArtifact `
    --output $Output
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}

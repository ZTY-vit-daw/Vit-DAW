param()

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$agentRoot = Join-Path $repoRoot 'agent'
$tempBase = [System.IO.Path]::GetTempPath()
$smokeRoot = Join-Path $tempBase ('vit-c1-frequency-cleanup-smoke-' + [guid]::NewGuid().ToString('N'))
$priorMixboardRoot = $env:VIT_MIXBOARD_ROOT

New-Item -ItemType Directory -Path $smokeRoot | Out-Null
try {
    $env:VIT_MIXBOARD_ROOT = $smokeRoot
    Push-Location $agentRoot
    try {
        go test ./internal/frequencycleanup ./internal/capabilitycontext ./internal/capabilityadapters
        go test ./internal/orchestration ./internal/orchestrationruntime
        go test ./internal/chat ./internal/harness ./internal/mixboard
    }
    finally {
        Pop-Location
    }
    git -C $repoRoot diff --check
}
finally {
    $env:VIT_MIXBOARD_ROOT = $priorMixboardRoot
    $resolvedSmoke = (Resolve-Path -LiteralPath $smokeRoot -ErrorAction SilentlyContinue)
    if ($null -ne $resolvedSmoke -and $resolvedSmoke.Path.StartsWith($tempBase, [System.StringComparison]::OrdinalIgnoreCase)) {
        Remove-Item -LiteralPath $resolvedSmoke.Path -Recurse -Force
    }
}

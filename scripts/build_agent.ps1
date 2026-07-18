#requires -Version 5.1

[CmdletBinding()]
param(
    [string]$OutDir = ""
)

$RepoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$AgentDir = Join-Path $RepoRoot "agent"
if ([string]::IsNullOrWhiteSpace($OutDir)) {
    $OutDir = Join-Path $AgentDir "bin"
}

if (-not (Test-Path -LiteralPath (Join-Path $AgentDir "go.mod"))) {
    throw "Missing agent go.mod: $AgentDir"
}

New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
Push-Location $AgentDir
try {
    go test ./...
    if ($LASTEXITCODE -ne 0) {
        throw "go test failed with exit code $LASTEXITCODE"
    }

    $outExe = Join-Path $OutDir "VitAgent.exe"
    go build -o $outExe .\cmd\vitagent
    if ($LASTEXITCODE -ne 0) {
        throw "VitAgent go build failed with exit code $LASTEXITCODE"
    }
    Write-Host "Built VitAgent: $outExe"

    $hubExe = Join-Path $OutDir "VspHub.exe"
    go build -o $hubExe .\cmd\vsphub
    if ($LASTEXITCODE -ne 0) {
        throw "VspHub go build failed with exit code $LASTEXITCODE"
    }
    Write-Host "Built VspHub: $hubExe"
}
finally {
    Pop-Location
}

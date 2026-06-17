[CmdletBinding()]
param(
    [string]$RepoRoot = "",
    [string]$OutRoot = "",
    [string]$SessionId = "mixlab_full_project",
    [double]$SegmentSeconds = 2.0,
    [switch]$SkipBuild
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Resolve-RepoRoot {
    param([string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) {
        return (Resolve-Path -LiteralPath $Explicit).Path
    }
    return (Split-Path -Parent (Split-Path -Parent $MyInvocation.ScriptName))
}

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host ("== " + $Message) -ForegroundColor Cyan
}

$RepoRoot = Resolve-RepoRoot -Explicit $RepoRoot
$AgentDir = Join-Path $RepoRoot "agent"
$ExePath = Join-Path $AgentDir "bin\mixlab.exe"

Write-Step "Mix acoustic lab"
Write-Host ("repo: " + $RepoRoot)

if (-not $SkipBuild) {
    Write-Step "Build mixlab"
    New-Item -ItemType Directory -Path (Split-Path -Parent $ExePath) -Force | Out-Null
    Push-Location $AgentDir
    try {
        & go build -o $ExePath .\cmd\mixlab
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed with exit code $LASTEXITCODE"
        }
    }
    finally {
        Pop-Location
    }
    Write-Host ("ok: built " + $ExePath) -ForegroundColor Green
}
elseif (-not (Test-Path -LiteralPath $ExePath)) {
    throw "Missing mixlab binary: $ExePath. Run without -SkipBuild first."
}

Write-Step "Run lab"
$args = @(
    "-repo-root", $RepoRoot,
    "-session-id", $SessionId,
    "-segment-seconds", [string]$SegmentSeconds
)
if (-not [string]::IsNullOrWhiteSpace($OutRoot)) {
    $args += @("-out-root", $OutRoot)
}
& $ExePath @args
if ($LASTEXITCODE -ne 0) {
    throw "mixlab failed with exit code $LASTEXITCODE"
}

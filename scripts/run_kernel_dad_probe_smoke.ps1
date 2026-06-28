[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$KernelExe = "",
    [string]$MaterialPath = "",
    [string]$PythonExe = "python",
    [string]$ZmqReqPort = "5555",
    [string]$ZmqSubPort = "5556",
    [string]$Features = "waveform_envelope,spectral_field,l3_acoustic_summary,l2_render_probe",
    [switch]$ReuseKernel,
    [switch]$KeepProcess,
    [switch]$WaitAllTiles,
    [int]$WaitSeconds = 45
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host ("== " + $Message) -ForegroundColor Cyan
}

function Write-Ok {
    param([string]$Message)
    Write-Host ("ok: " + $Message) -ForegroundColor Green
}

function Fail {
    param([string]$Message)
    throw $Message
}

function Get-TcpListener {
    param([int]$Port)
    return Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
}

function Wait-TcpListener {
    param([int]$Port, [int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $listener = Get-TcpListener -Port $Port
        if ($null -ne $listener) { return $listener }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    return $null
}

function Resolve-FirstExistingPath {
    param([string[]]$Candidates, [string]$Label)
    foreach ($candidate in $Candidates) {
        if (-not [string]::IsNullOrWhiteSpace($candidate) -and (Test-Path -LiteralPath $candidate)) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }
    Fail ("Could not find " + $Label + ". Tried: " + ($Candidates -join "; "))
}

function Resolve-KernelExe {
    param([string]$Root, [string]$Explicit)
    if (-not [string]::IsNullOrWhiteSpace($Explicit)) {
        return (Resolve-Path -LiteralPath $Explicit).Path
    }
    return Resolve-FirstExistingPath -Label "kernel exe" -Candidates @(
        (Join-Path $Root "VitApp\build_release\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $Root "VitApp\build\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $Root "VitApp\build_release\VitApp.exe"),
        (Join-Path $Root "Export\_build\Vit_DAW_v0.9_release\kernel\VitApp.exe"),
        (Join-Path $Root "Export\staging\runtime\VitApp.exe")
    )
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$KernelExe = Resolve-KernelExe -Root $RepoRoot -Explicit $KernelExe
if ([string]::IsNullOrWhiteSpace($MaterialPath)) {
    $MaterialPath = Resolve-FirstExistingPath -Label "probe material" -Candidates @(
        (Join-Path $RepoRoot "Paper Crown.mp3"),
        (Join-Path $RepoRoot "test_100hz_10s.wav")
    )
} else {
    $MaterialPath = (Resolve-Path -LiteralPath $MaterialPath).Path
}

$artifactDir = Join-Path $RepoRoot ("VitApp\Workspace\Artifacts\dad_probe\" + (Get-Date -Format "yyyyMMdd_HHmmss"))
$summaryPath = Join-Path $artifactDir "dad_probe_summary.json"
New-Item -ItemType Directory -Force -Path $artifactDir | Out-Null

$startedKernel = $null
try {
    Write-Step "Prepare VitApp kernel"
    $listener = Get-TcpListener -Port ([int]$ZmqReqPort)
    if ($null -ne $listener) {
        Write-Ok ("reusing kernel command port " + $ZmqReqPort + " pid=" + $listener.OwningProcess)
    } else {
        if ($ReuseKernel) {
            Fail ("ReuseKernel was set but no kernel command listener exists on " + $ZmqReqPort)
        }
        $startedKernel = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -PassThru -WindowStyle Hidden
        $listener = Wait-TcpListener -Port ([int]$ZmqReqPort) -TimeoutSeconds $WaitSeconds
        if ($null -eq $listener) {
            Fail ("Kernel command port did not become ready: " + $ZmqReqPort)
        }
        Write-Ok ("started kernel pid=" + $startedKernel.Id)
    }
    $subListener = Wait-TcpListener -Port ([int]$ZmqSubPort) -TimeoutSeconds 5
    if ($null -eq $subListener) {
        Write-Host ("warn: kernel publish port not listening yet: " + $ZmqSubPort) -ForegroundColor Yellow
    }

    Write-Step "Run DAD probe"
    $probe = Join-Path $RepoRoot "scripts\dad_probe.py"
    $args = @(
        $probe,
        "--req-url", ("tcp://127.0.0.1:" + $ZmqReqPort),
        "--sub-url", ("tcp://127.0.0.1:" + $ZmqSubPort),
        "--material-path", $MaterialPath,
        "--features", $Features,
        "--timeout-sec", ([string]$WaitSeconds),
        "--output", $summaryPath
    )
    if ($WaitAllTiles) {
        $args += "--wait-all-tiles"
    }
    & $PythonExe @args
    if ($LASTEXITCODE -ne 0) {
        Fail ("dad_probe.py failed with exit code " + $LASTEXITCODE)
    }
    Write-Ok ("DAD probe passed; summary=" + $summaryPath)
}
finally {
    if ($null -ne $startedKernel -and -not $KeepProcess) {
        Stop-Process -Id $startedKernel.Id -Force -ErrorAction SilentlyContinue
    }
}

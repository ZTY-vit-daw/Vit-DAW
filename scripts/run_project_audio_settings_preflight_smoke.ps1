#requires -Version 5.1

[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$KernelExe = "",
    [string]$TrainingFolder = "E:\BaiduNetdiskDownload\yingge - sattelites tracks out",
    [string]$PythonExe = "python",
    [string]$ZmqReqPort = "5555",
    [switch]$ReuseKernel,
    [switch]$KeepProcess,
    [int]$TimeoutSeconds = 45
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
        if ($null -ne $listener) {
            return $listener
        }
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
        (Join-Path $Root "VitApp\build\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $Root "VitApp\build_release\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $Root "VitApp\build_release\VitApp.exe"),
        (Join-Path $Root "Export\_build\Vit_DAW_v0.9_release\kernel\VitApp.exe"),
        (Join-Path $Root "Export\staging\runtime\VitApp.exe")
    )
}

function ConvertTo-JsonFile {
    param(
        [object]$Value,
        [string]$Path,
        [int]$Depth = 24
    )
    $Value | ConvertTo-Json -Depth $Depth | Set-Content -LiteralPath $Path -Encoding UTF8
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$KernelExe = Resolve-KernelExe -Root $RepoRoot -Explicit $KernelExe
$TrainingFolder = (Resolve-Path -LiteralPath $TrainingFolder).Path

$stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$artifactDir = Join-Path $RepoRoot ("VitApp\Workspace\Artifacts\smoke\project_audio_preflight_" + $stamp)
$summaryPath = Join-Path $artifactDir "summary.json"
$probeOutput = Join-Path $artifactDir "project_audio_preflight_probe.json"
$probeLog = Join-Path $artifactDir "project_audio_preflight_probe.log"
$tempProjectDir = Join-Path $artifactDir "temp_project"
$tempProjectPath = Join-Path $tempProjectDir "project_audio_settings_roundtrip.vit"
New-Item -ItemType Directory -Force -Path $artifactDir | Out-Null
New-Item -ItemType Directory -Force -Path $tempProjectDir | Out-Null

$summary = [ordered]@{
    schema_version = "project_audio_settings_preflight_smoke_wrapper.v1"
    status = "running"
    started_at = (Get-Date).ToString("o")
    repo_root = $RepoRoot
    kernel_exe = $KernelExe
    training_folder = $TrainingFolder
    artifact_dir = $artifactDir
    temp_project_path = $tempProjectPath
    probe_output = $probeOutput
    steps = @()
}
ConvertTo-JsonFile -Value $summary -Path $summaryPath

$startedKernel = $null
try {
    Write-Step "Prepare VitApp kernel"
    $listener = Get-TcpListener -Port ([int]$ZmqReqPort)
    if ($null -ne $listener) {
        if (-not $ReuseKernel) {
            $proc = Get-Process -Id $listener.OwningProcess -ErrorAction SilentlyContinue
            $runningPath = if ($null -ne $proc) { [string]$proc.Path } else { "" }
            if (-not [string]::IsNullOrWhiteSpace($runningPath) -and
                [System.IO.Path]::GetFullPath($runningPath).Equals([System.IO.Path]::GetFullPath($KernelExe), [System.StringComparison]::OrdinalIgnoreCase)) {
                Write-Ok ("desired kernel already listening pid=" + $listener.OwningProcess)
            }
            else {
                Fail ("kernel command port " + $ZmqReqPort + " is already in use by pid=" + $listener.OwningProcess + "; rerun with -ReuseKernel if that is intentional")
            }
        }
        else {
            Write-Ok ("reusing kernel command port " + $ZmqReqPort + " pid=" + $listener.OwningProcess)
        }
    }
    else {
        $startedKernel = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path -Parent $KernelExe) -PassThru -WindowStyle Hidden
        $listener = Wait-TcpListener -Port ([int]$ZmqReqPort) -TimeoutSeconds $TimeoutSeconds
        if ($null -eq $listener) {
            Fail ("Kernel command port did not become ready: " + $ZmqReqPort)
        }
        Write-Ok ("started kernel pid=" + $startedKernel.Id)
    }
    $summary["steps"] += @([ordered]@{
        name = "Prepare VitApp kernel"
        status = "passed"
        pid = if ($null -ne $listener) { $listener.OwningProcess } elseif ($null -ne $startedKernel) { $startedKernel.Id } else { $null }
    })
    ConvertTo-JsonFile -Value $summary -Path $summaryPath

    Write-Step "Run project audio settings/preflight probe"
    $probe = Join-Path $RepoRoot "scripts\project_audio_preflight_probe.py"
    $probeArgs = @(
        $probe,
        "--req-url", ("tcp://127.0.0.1:" + $ZmqReqPort),
        "--training-folder", $TrainingFolder,
        "--project-path", $tempProjectPath,
        "--output", $probeOutput,
        "--req-timeout-ms", ([string]([Math]::Max(30000, $TimeoutSeconds * 1000)))
    )
    & $PythonExe @probeArgs *>&1 | Tee-Object -FilePath $probeLog
    if ($LASTEXITCODE -ne 0) {
        Fail ("project_audio_preflight_probe.py failed with exit code " + $LASTEXITCODE + "; log=" + $probeLog)
    }
    $probeReport = Get-Content -LiteralPath $probeOutput -Raw | ConvertFrom-Json
    if ([string]$probeReport.status -ne "passed") {
        Fail ("project audio preflight probe failed: " + [string]$probeReport.error)
    }
    $summary["steps"] += @([ordered]@{
        name = "Run project audio settings/preflight probe"
        status = "passed"
        log_path = $probeLog
        output_path = $probeOutput
    })
    $summary["probe_assertions"] = $probeReport.assertions
    $summary["status"] = "passed"
    $summary["ended_at"] = (Get-Date).ToString("o")
    ConvertTo-JsonFile -Value $summary -Path $summaryPath
    Write-Ok ("project audio settings/preflight smoke passed; summary=" + $summaryPath)
}
catch {
    $summary["status"] = "failed"
    $summary["ended_at"] = (Get-Date).ToString("o")
    $summary["error"] = $_.Exception.Message
    ConvertTo-JsonFile -Value $summary -Path $summaryPath
    throw
}
finally {
    if ($null -ne $startedKernel -and -not $KeepProcess) {
        Stop-Process -Id $startedKernel.Id -Force -ErrorAction SilentlyContinue
    }
}

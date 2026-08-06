[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$TrackId = "1007",
    [string]$PluginId = "1021",
    [string]$Material = "",
    [int]$TimeoutSeconds = 90,
    [switch]$KeepProcesses
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($Material)) {
    $Material = Join-Path $RepoRoot "PluginProbe\assets\probe_audio_v1\transient_burst_48k_10s.wav"
}
$kernel = Join-Path $RepoRoot "VitApp\build_release\VitApp_artefacts\Release\VitApp.exe"
$agent = Join-Path $RepoRoot "agent\bin\VitAgent.exe"
$stdout = Join-Path $RepoRoot "VitApp\Workspace\Logs\com2_smoke_kernel_stdout.log"
$stderr = Join-Path $RepoRoot "VitApp\Workspace\Logs\com2_smoke_kernel_stderr.log"
$started = @()
try {
    if (-not (Get-NetTCPConnection -State Listen -LocalPort 5555 -ErrorAction SilentlyContinue)) {
        $p = Start-Process -FilePath $kernel -WorkingDirectory (Join-Path $RepoRoot "VitApp") -WindowStyle Hidden -RedirectStandardOutput $stdout -RedirectStandardError $stderr -PassThru
        $started += $p
    }
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while (-not (Get-NetTCPConnection -State Listen -LocalPort 5555 -ErrorAction SilentlyContinue)) {
        if ((Get-Date) -ge $deadline) { throw "kernel did not open port 5555" }
        Start-Sleep -Milliseconds 300
    }
    if (-not (Get-NetTCPConnection -State Listen -LocalPort 7878 -ErrorAction SilentlyContinue)) {
        $p = Start-Process -FilePath $agent -WorkingDirectory (Join-Path $RepoRoot "agent") -WindowStyle Hidden -PassThru
        $started += $p
    }
    while (-not (Get-NetTCPConnection -State Listen -LocalPort 7878 -ErrorAction SilentlyContinue)) {
        if ((Get-Date) -ge $deadline) { throw "agent did not open port 7878" }
        Start-Sleep -Milliseconds 300
    }
    & python (Join-Path $RepoRoot "scripts\compressor_dual_tap_smoke.py") --track-id $TrackId --plugin-id $PluginId --material $Material --workspace (Join-Path $RepoRoot "VitApp\Workspace") --timeout-sec $TimeoutSeconds
    if ($LASTEXITCODE -ne 0) { throw "compressor dual-tap product smoke failed" }
}
finally {
    if (-not $KeepProcesses) {
        foreach ($process in $started) {
            Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue
        }
    }
}

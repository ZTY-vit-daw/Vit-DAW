param([int]$OldPid)
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$AgentExe = "D:\Vit_DAW\agent\bin\VitAgent.exe"

if ($OldPid -gt 0) {
    $proc = Get-Process -Id $OldPid -ErrorAction SilentlyContinue
    if ($null -ne $proc) {
        Write-Host "Stopping old VitAgent PID=$OldPid"
        Stop-Process -Id $OldPid -Force
        Start-Sleep -Milliseconds 800
    }
}

Write-Host "Starting new VitAgent: $AgentExe"
Start-Process -FilePath $AgentExe -WindowStyle Hidden

# Wait up to 20s for port 7878
$deadline = (Get-Date).AddSeconds(20)
while ((Get-Date) -lt $deadline) {
    Start-Sleep -Milliseconds 500
    $l = Get-NetTCPConnection -LocalPort 7878 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $l) {
        Write-Host "ok: VitAgent ready on port 7878 (PID=$($l.OwningProcess))"
        exit 0
    }
}
Write-Host "FAIL: VitAgent did not come up on port 7878 within 20s"
exit 1

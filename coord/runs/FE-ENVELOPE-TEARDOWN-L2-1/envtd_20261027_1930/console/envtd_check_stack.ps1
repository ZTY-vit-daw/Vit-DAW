param()
$ErrorActionPreference = 'Continue'
$procs = Get-Process VitApp,VitAgent,VspHub,Godot -ErrorAction SilentlyContinue
if ($procs) {
    Write-Output ("PROCS_PRESENT: " + (($procs | ForEach-Object { $_.Id.ToString() + "=" + $_.ProcessName }) -join ","))
} else {
    Write-Output "PROCS_ZERO"
}
foreach ($pt in @(5555,5556,7878,8787,18787)) {
    $l = Get-NetTCPConnection -LocalPort $pt -State Listen -ErrorAction SilentlyContinue
    if ($l) { Write-Output ("TCP " + $pt + " LISTENING") } else { Write-Output ("tcp " + $pt + " free") }
}
foreach ($pt in @(4444,4445)) {
    $u = Get-NetUDPEndpoint -LocalPort $pt -ErrorAction SilentlyContinue
    if ($u) { Write-Output ("UDP " + $pt + " BOUND") } else { Write-Output ("udp " + $pt + " free") }
}
Write-Output ("CHECK_AT " + (Get-Date -Format 'HH:mm:ss'))

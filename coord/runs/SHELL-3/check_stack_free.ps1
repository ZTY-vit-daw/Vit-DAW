# check_stack_free.ps1 - S2.2 physical pre-check (read-only)
$c = 0
foreach ($p in @('VitShell','VitAgent','VitApp','VspHub','Godot')) {
    $c += @(Get-Process -Name $p -ErrorAction SilentlyContinue).Count
}
Write-Host "VIT_PROC_COUNT=$c"
if ($c -gt 0) {
    Get-Process -Name VitShell,VitAgent,VitApp,VspHub,Godot -ErrorAction SilentlyContinue |
        Select-Object Name,Id | Format-Table | Out-String | Write-Host
}
foreach ($port in @(7878,7879,8787)) {
    $l = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
    if ($l) { Write-Host "PORT_${port}_LISTENING pid=$($l[0].OwningProcess)" }
    else { Write-Host "PORT_${port}_FREE" }
}
$udp = Get-NetUDPEndpoint -LocalPort 4445 -ErrorAction SilentlyContinue
if ($udp) { Write-Host "UDP_4445_BOUND" } else { Write-Host "UDP_4445_FREE" }

# micro_bind.ps1 - does Socket.Close() on a bound socket release instantly?
$occ = [System.Net.Sockets.Socket]::new([System.Net.Sockets.AddressFamily]::InterNetwork, [System.Net.Sockets.SocketType]::Stream, [System.Net.Sockets.ProtocolType]::Tcp)
$occ.Bind([System.Net.IPEndPoint]::new([System.Net.IPAddress]::Loopback, 7878))
Write-Host "bound"
$probe1 = $false
try { $l = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 7878); $l.Start(); $l.Stop(); $probe1 = $true } catch { Write-Host ("probe-while-bound: blocked (expected): " + $_.Exception.InnerException.Message) }
Write-Host "probe1=$probe1"
$occ.Close()
Write-Host "closed"
for ($i = 1; $i -le 5; $i++) {
    $ok = $false
    try { $l = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 7878); $l.Start(); $l.Stop(); $ok = $true } catch { $e = $_.Exception.InnerException.Message }
    Write-Host ("probe after close t=" + $i + "s ok=" + $ok + $(if (-not $ok) { " err=" + $e }))
    if ($ok) { break }
    Start-Sleep -Seconds 1
}
# fixed-probe pattern (finally-dispose) during a hold
$occ2 = [System.Net.Sockets.Socket]::new([System.Net.Sockets.AddressFamily]::InterNetwork, [System.Net.Sockets.SocketType]::Stream, [System.Net.Sockets.ProtocolType]::Tcp)
$occ2.Bind([System.Net.IPEndPoint]::new([System.Net.IPAddress]::Loopback, 7878))
$fixedProbeBlocked = $false
$l2 = $null
try { $l2 = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 7878); $l2.Start() } catch { $fixedProbeBlocked = $true } finally { if ($null -ne $l2) { try { $l2.Stop() } catch {} } }
Write-Host "fixed-probe-while-bound blocked=$fixedProbeBlocked"
# immediately probe again - leaked?
$ok2 = $false
try { $l3 = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 7878); $l3.Start(); $l3.Stop(); $ok2 = $true } catch { $e2 = $_.Exception.InnerException.Message }
Write-Host ("probe-after-fixed-probe ok=" + $ok2 + $(if (-not $ok2) { " err=" + $e2 }))
$occ2.Close()
Write-Host "MICRO_DONE"

# repro_f2hold.ps1 - bound-only socket occupier + VitShell error leg; dump who
# holds 7878 after Close() and how long until bindable.
$ErrorActionPreference = "Continue"
$MainRoot = "D:/Vit_DAW_wt_shell3"
$FrontendWorktree = "D:/Godot/project/vit-daw-frontend_wt_shell3"
$GodotExe = "D:/Godot/Godot_v4.6.1-stable_win64_console.exe"
$VitShellExe = Join-Path $FrontendWorktree "VitShell/build/Release/VitShell.exe"
$AgentExe = Join-Path $MainRoot "agent/bin/VitAgent.exe"
$work = Join-Path $MainRoot "coord/runs/SHELL-3/repro_f2hold"
New-Item -ItemType Directory -Path $work -Force | Out-Null

function Dump-Port {
    param([int]$Port, [string]$Tag)
    $conns = Get-NetTCPConnection -LocalPort $Port -ErrorAction SilentlyContinue
    if (-not $conns) { Write-Host ("  [" + $Tag + "] port " + $Port + ": (no entries)") }
    foreach ($c in $conns) {
        $pn = try { (Get-Process -Id $c.OwningProcess -ErrorAction Stop).ProcessName } catch { "?" }
        Write-Host ("  [" + $Tag + "] port " + $Port + " state=" + $c.State + " pid=" + $c.OwningProcess + " proc=" + $pn)
    }
}

$env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = "--remote-debugging-port=9333"
$env:VIT_WEBVIEW_DEBUG = "1"
$occ = [System.Net.Sockets.Socket]::new([System.Net.Sockets.AddressFamily]::InterNetwork, [System.Net.Sockets.SocketType]::Stream, [System.Net.Sockets.ProtocolType]::Tcp)
$occ.Bind([System.Net.IPEndPoint]::new([System.Net.IPAddress]::Loopback, 7878))
Write-Host "occupier bound (no listen)"
$p = Start-Process -FilePath $VitShellExe `
    -ArgumentList @("--agent-exe", $AgentExe, "--dev-root", $MainRoot,
        "--agent-url", "http://127.0.0.1:7878", "--agent-http-addr", "127.0.0.1:7878",
        "--url", "http://127.0.0.1:7878/app/?surface=launcher", "--status-port", "7879",
        "--godot-exe", $GodotExe, "--godot-project-dir", $FrontendWorktree,
        "--agent-ready-timeout-ms", "15000",
        "--bridge-log", (Join-Path $work "f2_bridge.log")) `
    -RedirectStandardError (Join-Path $work "f2_stderr.log") -PassThru -WindowStyle Hidden
Remove-Item Env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS -ErrorAction SilentlyContinue
Remove-Item Env:VIT_WEBVIEW_DEBUG -ErrorAction SilentlyContinue
Write-Host ("VitShell pid=" + $p.Id)
for ($i = 0; $i -lt 40; $i++) {
    Start-Sleep -Milliseconds 800
    try {
        $r = Invoke-WebRequest -Uri "http://127.0.0.1:7879/status" -UseBasicParsing -TimeoutSec 2
        $j = $r.Content | ConvertFrom-Json
        if ($j.phase -eq "error") { Write-Host ("error at t=" + $i + ": " + $j.error); break }
    } catch {}
}
Dump-Port -Port 7878 -Tag "at error phase"
$occ.Close()
Write-Host "occupier closed"
for ($t = 1; $t -le 20; $t++) {
    Start-Sleep -Seconds 1
    $ok = $false
    $s2 = $null
    try { $s2 = [System.Net.Sockets.Socket]::new([System.Net.Sockets.AddressFamily]::InterNetwork, [System.Net.Sockets.SocketType]::Stream, [System.Net.Sockets.ProtocolType]::Tcp); $s2.Bind([System.Net.IPEndPoint]::new([System.Net.IPAddress]::Loopback, 7878)); $ok = $true } catch {}
    finally { if ($null -ne $s2) { try { $s2.Close() } catch {} } }
    if ($ok) { Write-Host ("7878 bindable after " + $t + "s"); break }
    if ($t -eq 5 -or $t -eq 10 -or $t -eq 15) { Dump-Port -Port 7878 -Tag ("t+" + $t + "s") }
}
if (-not $ok) { Write-Host "7878 NOT bindable after 20s"; Dump-Port -Port 7878 -Tag "final" }
# also check what the agent process is doing
Get-Process -Name VitAgent -ErrorAction SilentlyContinue | ForEach-Object { Write-Host ("VitAgent still alive: pid=" + $_.Id) }
try { $null = Invoke-WebRequest -Uri "http://127.0.0.1:7879/close" -Method POST -UseBasicParsing -TimeoutSec 3 } catch {}
Start-Sleep -Seconds 4
Get-Process -Name VitShell,VitAgent -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Write-Host "F2HOLD_DONE"

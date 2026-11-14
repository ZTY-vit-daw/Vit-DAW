# repro_port.ps1 - focused repro of F2->R1 port lingering with full diagnostics.
$ErrorActionPreference = "Continue"
$MainRoot = "D:/Vit_DAW_wt_shell3"
$FrontendWorktree = "D:/Godot/project/vit-daw-frontend_wt_shell3"
$GodotExe = "D:/Godot/Godot_v4.6.1-stable_win64_console.exe"
$VitShellExe = Join-Path $FrontendWorktree "VitShell/build/Release/VitShell.exe"
$AgentExe = Join-Path $MainRoot "agent/bin/VitAgent.exe"
$work = Join-Path $MainRoot "coord/runs/SHELL-3/repro_port"
New-Item -ItemType Directory -Path $work -Force | Out-Null

function Dump-Port {
    param([int]$Port, [string]$Tag)
    Write-Host "--- port $Port dump ($Tag) ---"
    $conns = Get-NetTCPConnection -LocalPort $Port -ErrorAction SilentlyContinue
    foreach ($c in $conns) {
        $procName = try { (Get-Process -Id $c.OwningProcess -ErrorAction Stop).ProcessName } catch { "?" }
        Write-Host ("  state=" + $c.State + " pid=" + $c.OwningProcess + " proc=" + $procName + " local=" + $c.LocalAddress + ":" + $c.LocalPort + " remote=" + $c.RemoteAddress + ":" + $c.RemotePort)
    }
    if (-not $conns) { Write-Host "  (no connections)" }
    $b = try {
        $l = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $Port)
        $l.Start(); $l.Stop(); "BINDABLE"
    } catch { "NOT_BINDABLE: " + $_.Exception.Message }
    Write-Host ("  bindprobe=" + $b)
}

function Start-VS {
    param([string]$Tag, [string]$AgentExePath = "", [int]$ReadyMs = 15000)
    $env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = "--remote-debugging-port=9335"
    $args1 = @("--agent-exe", $(if ($AgentExePath) { $AgentExePath } else { $AgentExe }),
        "--dev-root", $MainRoot, "--agent-url", "http://127.0.0.1:7878",
        "--agent-http-addr", "127.0.0.1:7878", "--url", "http://127.0.0.1:7878/app/?surface=launcher",
        "--status-port", "7879", "--godot-exe", $GodotExe, "--godot-project-dir", $FrontendWorktree,
        "--agent-ready-timeout-ms", "$ReadyMs", "--bridge-log", (Join-Path $work "$Tag`_bridge.log"))
    $p = Start-Process -FilePath $VitShellExe -ArgumentList $args1 `
        -RedirectStandardError (Join-Path $work "$Tag`_stderr.log") -PassThru -WindowStyle Hidden
    Remove-Item Env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS -ErrorAction SilentlyContinue
    return $p
}

Write-Host "=== STEP 1: hold 7878, launch VitShell (F2 style) ==="
$listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 7878)
$listener.Start()
$p = Start-VS -Tag "f2"
# wait for error phase
$errPhase = $false
for ($i = 0; $i -lt 40; $i++) {
    Start-Sleep -Milliseconds 800
    try {
        $r = Invoke-WebRequest -Uri "http://127.0.0.1:7879/status" -UseBasicParsing -TimeoutSec 2
        $j = $r.Content | ConvertFrom-Json
        if ($j.phase -eq "error") { $errPhase = $true; Write-Host ("error phase at t=" + $i + ": " + $j.error); break }
    } catch {}
}
Write-Host "errPhase=$errPhase"
Write-Host "=== STEP 2: stop listener, check bindable with dump ==="
$listener.Stop()
Start-Sleep -Seconds 2
Dump-Port -Port 7878 -Tag "after listener stop"
Dump-Port -Port 7879 -Tag "before VitShell close"

Write-Host "=== STEP 3: close VitShell ==="
try { $null = Invoke-WebRequest -Uri "http://127.0.0.1:7879/close" -Method POST -UseBasicParsing -TimeoutSec 3 } catch { Write-Host ("close failed: " + $_.Exception.Message) }
Start-Sleep -Seconds 4
Get-Process -Name VitShell,VitAgent -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 3
Dump-Port -Port 7878 -Tag "after VitShell close"
Dump-Port -Port 7879 -Tag "after VitShell close"

Write-Host "=== STEP 4: launch round-style VitShell and watch status ==="
$env:VIT_AGENT_LLM_BASE_URL = "http://127.0.0.1:1"
$p2 = Start-VS -Tag "r1" -ReadyMs 120000
Remove-Item Env:VIT_AGENT_LLM_BASE_URL -ErrorAction SilentlyContinue
for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Seconds 2
    $alive = $null -ne (Get-Process -Id $p2.Id -ErrorAction SilentlyContinue)
    $st = "no-response"
    try {
        $r = Invoke-WebRequest -Uri "http://127.0.0.1:7879/status" -UseBasicParsing -TimeoutSec 2
        $j = $r.Content | ConvertFrom-Json
        $st = "phase=" + $j.phase + " agent_running=" + $j.agent_running
    } catch {
        $st = "no-response"
    }
    Write-Host ("t=" + (($i+1)*2) + "s alive=$alive status=$st")
    if ($st -match "phase=(ready|error)") { break }
    if (-not $alive) { break }
}
Dump-Port -Port 7879 -Tag "after round launch"
Dump-Port -Port 7878 -Tag "after round launch"
Get-Content (Join-Path $MainRoot "VitApp/Workspace/Logs/agent_last.log") -Tail 6 -ErrorAction SilentlyContinue | ForEach-Object { Write-Host ("agentlog: " + $_) }
Get-Process -Name VitShell,VitAgent,VitApp,VspHub,Godot -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Write-Host "REPRO_PORT_DONE"

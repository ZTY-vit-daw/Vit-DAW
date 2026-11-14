# repro_seq.ps1 - replicate formal-run sequence F1->F2->F3->R1 with
# VIT_WEBVIEW_DEBUG=1 stderr traces, msedgewebview2 tracking, port dumps.
$ErrorActionPreference = "Continue"
$MainRoot = "D:/Vit_DAW_wt_shell3"
$FrontendWorktree = "D:/Godot/project/vit-daw-frontend_wt_shell3"
$GodotExe = "D:/Godot/Godot_v4.6.1-stable_win64_console.exe"
$VitShellExe = Join-Path $FrontendWorktree "VitShell/build/Release/VitShell.exe"
$AgentExe = Join-Path $MainRoot "agent/bin/VitAgent.exe"
$work = Join-Path $MainRoot "coord/runs/SHELL-3/repro_seq"
New-Item -ItemType Directory -Path $work -Force | Out-Null

function Dump-WV {
    param([string]$Tag)
    $wv = @(Get-Process -Name msedgewebview2 -ErrorAction SilentlyContinue | Where-Object { $_.StartTime -gt (Get-Date).AddMinutes(-30) })
    Write-Host ("  [wv] " + $Tag + ": fresh msedgewebview2 count=" + $wv.Count + " pids=" + (($wv | Select-Object -First 5 -ExpandProperty Id) -join ","))
}

function Start-VS {
    param([string]$Tag, [string]$AgentExePath = "", [int]$ReadyMs = 15000)
    $env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = "--remote-debugging-port=9333"
    $env:VIT_WEBVIEW_DEBUG = "1"
    $args1 = @("--agent-exe", $(if ($AgentExePath) { $AgentExePath } else { $AgentExe }),
        "--dev-root", $MainRoot, "--agent-url", "http://127.0.0.1:7878",
        "--agent-http-addr", "127.0.0.1:7878", "--url", "http://127.0.0.1:7878/app/?surface=launcher",
        "--status-port", "7879", "--godot-exe", $GodotExe, "--godot-project-dir", $FrontendWorktree,
        "--agent-ready-timeout-ms", "$ReadyMs", "--bridge-log", (Join-Path $work "$Tag`_bridge.log"))
    $p = Start-Process -FilePath $VitShellExe -ArgumentList $args1 `
        -RedirectStandardError (Join-Path $work "$Tag`_stderr.log") -PassThru -WindowStyle Hidden
    Remove-Item Env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS -ErrorAction SilentlyContinue
    Remove-Item Env:VIT_WEBVIEW_DEBUG -ErrorAction SilentlyContinue
    Write-Host ("[" + $Tag + "] launched pid=" + $p.Id)
    return $p
}

function Wait-Phase {
    param([string]$Expect, [int]$TimeoutSec)
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Milliseconds 600
        try {
            $r = Invoke-WebRequest -Uri "http://127.0.0.1:7879/status" -UseBasicParsing -TimeoutSec 2
            $j = $r.Content | ConvertFrom-Json
            if ($j.phase -eq $Expect) { return $j }
        } catch {}
    }
    return $null
}

function Kill-Tree {
    try { $null = Invoke-WebRequest -Uri "http://127.0.0.1:7879/close" -Method POST -UseBasicParsing -TimeoutSec 2 } catch {}
    Start-Sleep -Seconds 3
    Get-Process -Name VitShell,VitAgent,VitApp,VspHub,Godot -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
}

Write-Host "=== F1 (bad agent exe) ==="
$p = Start-VS -Tag "f1" -AgentExePath "D:/nonexistent_shell3_badagent/VitAgent.exe"
$s = Wait-Phase -Expect "error" -TimeoutSec 30
Write-Host ("  f1 phase=" + $(if ($s) { $s.phase } else { "TIMEOUT" }))
Dump-WV -Tag "f1 post"
Kill-Tree
Dump-WV -Tag "post f1 teardown"

Write-Host "=== F2 (port occupied) ==="
$listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 7878)
$listener.Start()
$p = Start-VS -Tag "f2"
$s = Wait-Phase -Expect "error" -TimeoutSec 60
Write-Host ("  f2 phase=" + $(if ($s) { $s.phase } else { "TIMEOUT" }))
Start-Sleep -Seconds 2
$listener.Stop()
Write-Host "  listener.Stop() called"
$bindable = $false
for ($i = 0; $i -lt 24; $i++) {
    Start-Sleep -Milliseconds 1000
    try { $l2 = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 7878); $l2.Start(); $l2.Stop(); $bindable = $true; Write-Host ("  7878 bindable after " + ($i+1) + "s"); break } catch {}
}
if (-not $bindable) {
    Write-Host "  7878 STILL NOT BINDABLE after 24s"
    Get-NetTCPConnection -LocalPort 7878 -ErrorAction SilentlyContinue | ForEach-Object {
        $pn = try { (Get-Process -Id $_.OwningProcess -ErrorAction Stop).ProcessName } catch { "?" }
        Write-Host ("    state=" + $_.State + " pid=" + $_.OwningProcess + " proc=" + $pn)
    }
}
Kill-Tree
Dump-WV -Tag "post f2 teardown"

Write-Host "=== F3 (webview2 off) ==="
$wv2Dead = Join-Path $work "wv2dead"
New-Item -ItemType Directory -Path $wv2Dead -Force | Out-Null
$env:WEBVIEW2_BROWSER_EXECUTABLE_FOLDER = $wv2Dead
$p = Start-VS -Tag "f3"
Remove-Item Env:WEBVIEW2_BROWSER_EXECUTABLE_FOLDER -ErrorAction SilentlyContinue
Start-Sleep -Seconds 12
$alive = $null -ne (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)
Write-Host ("  f3 alive=" + $alive)
Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2
Dump-WV -Tag "post f3"

Write-Host "=== R1 (round style) ==="
$env:VIT_AGENT_LLM_BASE_URL = "http://127.0.0.1:1"
$p = Start-VS -Tag "r1" -ReadyMs 120000
Remove-Item Env:VIT_AGENT_LLM_BASE_URL -ErrorAction SilentlyContinue
Dump-WV -Tag "r1 +2s"
Start-Sleep -Seconds 3
Dump-WV -Tag "r1 +5s"
$s = Wait-Phase -Expect "ready" -TimeoutSec 100
Write-Host ("  r1 phase=" + $(if ($s) { $s.phase } else { "TIMEOUT-UNREACHABLE" }))
Dump-WV -Tag "r1 end"
Get-Content (Join-Path $work "r1_stderr.log") -Tail 15 -ErrorAction SilentlyContinue | ForEach-Object { Write-Host ("  r1stderr: " + $_) }
Kill-Tree
Write-Host "REPRO_SEQ_DONE"

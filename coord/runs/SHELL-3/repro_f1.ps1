# repro_f1.ps1 - exact F1 leg reproduction with instrumented polling.
$ErrorActionPreference = "Stop"
$MainRoot = "D:/Vit_DAW_wt_shell3"
$FrontendWorktree = "D:/Godot/project/vit-daw-frontend_wt_shell3"
$GodotExe = "D:/Godot/Godot_v4.6.1-stable_win64_console.exe"
$VitShellExe = Join-Path $FrontendWorktree "VitShell/build/Release/VitShell.exe"
$AgentExe = Join-Path $MainRoot "agent/bin/VitAgent.exe"
$RunRoot = Join-Path $MainRoot "coord/runs/SHELL-3/repro_f1"
New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
$StatusPort = 7879
$AgentPort = 7878
$CdpPort = 9333
$fixtureA = Join-Path $RunRoot "fixture_a.vit"
$fixtureB = Join-Path $RunRoot "fixture_b.vit"
Copy-Item "D:/Vit_DAW/temp/f3_empty_project.vit" $fixtureA -Force
Copy-Item "D:/Vit_DAW/temp/f3_empty_project.vit" $fixtureB -Force
$StatusUrl = "http://127.0.0.1:$StatusPort"

$env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = "--remote-debugging-port=$CdpPort"
$p = Start-Process -FilePath $VitShellExe `
    -ArgumentList (@(
        "--agent-exe", $AgentExe,
        "--dev-root", $MainRoot,
        "--agent-url", "http://127.0.0.1:$AgentPort",
        "--agent-http-addr", "127.0.0.1:$AgentPort",
        "--url", "http://127.0.0.1:$AgentPort/app/?surface=launcher",
        "--status-port", "$StatusPort",
        "--godot-exe", $GodotExe,
        "--godot-project-dir", $FrontendWorktree,
        "--bridge-log", (Join-Path $RunRoot "f1_bridge.log")
    ) + @("--agent-exe", "D:/nonexistent_shell3_badagent/VitAgent.exe", "--smoke-file-dialog", $fixtureA, "--smoke-file-dialog-b", $fixtureB)) `
    -RedirectStandardError (Join-Path $RunRoot "f1_bridge.log.stderr.log") -PassThru -WindowStyle Hidden
Remove-Item Env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS -ErrorAction SilentlyContinue
Write-Host "pid=$($p.Id)"

for ($i = 1; $i -le 20; $i++) {
    Start-Sleep -Milliseconds 1500
    $alive = $null -ne (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)
    $r = $null
    $err = ""
    try {
        $resp = Invoke-WebRequest -Uri $StatusUrl -UseBasicParsing -TimeoutSec 3
        $r = $resp.Content
    } catch {
        $err = $_.Exception.Message
        if ($err.Length -gt 90) { $err = $err.Substring(0, 90) }
    }
    Write-Host ("poll $i alive=$alive statusBody=" + $(if ($null -ne $r) { $r.Substring(0, [Math]::Min(80, $r.Length)) } else { "NONE err=$err" }))
    if ($null -ne $r) { break }
    if (-not $alive) { Write-Host "PROCESS DIED"; break }
}
Get-Process -Name VitShell,VitAgent -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Write-Host "REPRO_DONE"

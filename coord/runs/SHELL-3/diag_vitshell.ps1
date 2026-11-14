# diag_vitshell.ps1 - manual diagnostic: launch VitShell with F1-style args,
# poll status, report.
$ErrorActionPreference = "Continue"
$VitShellExe = "D:/Godot/project/vit-daw-frontend_wt_shell3/VitShell/build/Release/VitShell.exe"
$AgentExe = "D:/Vit_DAW_wt_shell3/agent/bin/VitAgent.exe"
$MainRoot = "D:/Vit_DAW_wt_shell3"
$FrontendWorktree = "D:/Godot/project/vit-daw-frontend_wt_shell3"
$GodotExe = "D:/Godot/Godot_v4.6.1-stable_win64_console.exe"
$work = "D:/Vit_DAW_wt_shell3/coord/runs/SHELL-3/diag_manual"
New-Item -ItemType Directory -Path $work -Force | Out-Null

$env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = "--remote-debugging-port=9334"
$env:VIT_AGENT_LLM_BASE_URL = "http://127.0.0.1:1"
$p = Start-Process -FilePath $VitShellExe `
    -ArgumentList @(
        "--agent-exe", "D:/Vit_DAW_wt_shell3/agent/bin/VitAgent.exe", "--agent-exe", "D:/nonexistent_shell3_badagent/VitAgent.exe",
        "--dev-root", $MainRoot,
        "--agent-url", "http://127.0.0.1:7878",
        "--agent-http-addr", "127.0.0.1:7878",
        "--url", "http://127.0.0.1:7878/app/?surface=launcher",
        "--status-port", "7879",
        "--godot-exe", $GodotExe,
        "--godot-project-dir", $FrontendWorktree,
        "--bridge-log", (Join-Path $work "bridge.log")
    ) `
    -RedirectStandardError (Join-Path $work "stderr.log") -PassThru -WindowStyle Hidden
Remove-Item Env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS -ErrorAction SilentlyContinue
Remove-Item Env:VIT_AGENT_LLM_BASE_URL -ErrorAction SilentlyContinue
Write-Host "pid=$($p.Id)"

for ($i = 0; $i -lt 20; $i++) {
    Start-Sleep -Seconds 2
    $alive = $null -ne (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)
    $status = "no-response"
    try {
        $r = Invoke-WebRequest -Uri "http://127.0.0.1:7879/status" -UseBasicParsing -TimeoutSec 2
        $status = $r.Content.Substring(0, [Math]::Min(200, $r.Content.Length))
    } catch {
        $status = "no-response (" + $_.Exception.Message.Substring(0, [Math]::Min(60, $_.Exception.Message.Length)) + ")"
    }
    $listen = Get-NetTCPConnection -LocalPort 7879 -State Listen -ErrorAction SilentlyContinue
    $agentProc = @(Get-Process -Name VitAgent -ErrorAction SilentlyContinue).Count
    Write-Host ("t=" + (($i+1)*2) + "s alive=$alive listen7879=" + ($null -ne $listen) + " agentProc=$agentProc status=$status")
    if ($status -notmatch "no-response") { break }
    if (-not $alive) { break }
}
Get-Content (Join-Path $work "bridge.log") -ErrorAction SilentlyContinue | ForEach-Object { Write-Host ("bridge: " + $_) }
Get-Process -Name VitShell,VitAgent -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Write-Host "DIAG_DONE"

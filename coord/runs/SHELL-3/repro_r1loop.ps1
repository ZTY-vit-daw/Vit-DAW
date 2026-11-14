# repro_r1loop.ps1 - R1-exact launch config in a tight loop to catch the
# pre-trace instant death; per-iteration alive/trace/port evidence.
$ErrorActionPreference = "Continue"
$MainRoot = "D:/Vit_DAW_wt_shell3"
$FrontendWorktree = "D:/Godot/project/vit-daw-frontend_wt_shell3"
$GodotExe = "D:/Godot/Godot_v4.6.1-stable_win64_console.exe"
$VitShellExe = Join-Path $FrontendWorktree "VitShell/build/Release/VitShell.exe"
$AgentExe = Join-Path $MainRoot "agent/bin/VitAgent.exe"
$work = Join-Path $MainRoot "coord/runs/SHELL-3/repro_r1loop"
New-Item -ItemType Directory -Path $work -Force | Out-Null
$fixtureA = Join-Path $work "fixture_a.vit"
$fixtureB = Join-Path $work "fixture_b.vit"
Copy-Item "D:/Vit_DAW/temp/f3_empty_project.vit" $fixtureA -Force
Copy-Item "D:/Vit_DAW/temp/f3_empty_project.vit" $fixtureB -Force

for ($i = 1; $i -le 6; $i++) {
    $env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS = "--remote-debugging-port=9333"
    $env:VIT_WEBVIEW_DEBUG = "1"
    $env:VIT_AGENT_LLM_BASE_URL = "http://127.0.0.1:1"
    $p = Start-Process -FilePath $VitShellExe `
        -ArgumentList @(
            "--agent-exe", $AgentExe, "--dev-root", $MainRoot,
            "--agent-url", "http://127.0.0.1:7878", "--agent-http-addr", "127.0.0.1:7878",
            "--url", "http://127.0.0.1:7878/app/?surface=launcher", "--status-port", "7879",
            "--godot-exe", $GodotExe, "--godot-project-dir", $FrontendWorktree,
            "--smoke-file-dialog", $fixtureA, "--smoke-file-dialog-b", $fixtureB,
            "--bridge-log", (Join-Path $work "loop${i}_bridge.log")
        ) `
        -RedirectStandardError (Join-Path $work "loop${i}_stderr.log") -PassThru -WindowStyle Hidden
    Remove-Item Env:WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS -ErrorAction SilentlyContinue
    Remove-Item Env:VIT_WEBVIEW_DEBUG -ErrorAction SilentlyContinue
    Remove-Item Env:VIT_AGENT_LLM_BASE_URL -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 5
    $alive = $null -ne (Get-Process -Id $p.Id -ErrorAction SilentlyContinue)
    $st = ""
    try {
        $r = Invoke-WebRequest -Uri "http://127.0.0.1:7879/status" -UseBasicParsing -TimeoutSec 2
        $j = $r.Content | ConvertFrom-Json
        $st = "phase=" + $j.phase
    } catch { $st = "unreachable" }
    $stderrSize = (Get-Item (Join-Path $work "loop${i}_stderr.log") -ErrorAction SilentlyContinue).Length
    Write-Host ("iter $i pid=" + $p.Id + " alive5s=$alive status=$st stderrBytes=$stderrSize")
    if (-not $alive) {
        try { Write-Host ("  ExitCode=" + $p.ExitCode) } catch { Write-Host "  ExitCode unavailable" }
    }
    # teardown
    try { $null = Invoke-WebRequest -Uri "http://127.0.0.1:7879/close" -Method POST -UseBasicParsing -TimeoutSec 2 } catch {}
    Start-Sleep -Seconds 3
    Get-Process -Name VitShell,VitAgent,VitApp,VspHub,Godot -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 4
    # wait our webview gone
    for ($w = 0; $w -lt 30; $w++) {
        $n = 0
        Get-CimInstance Win32_Process -Filter "Name = 'msedgewebview2.exe'" -ErrorAction SilentlyContinue | ForEach-Object { if ($_.CommandLine -match "VitDAW") { $n++ } }
        if ($n -eq 0) { break }
        Start-Sleep -Milliseconds 500
    }
}
Write-Host "LOOP_DONE"

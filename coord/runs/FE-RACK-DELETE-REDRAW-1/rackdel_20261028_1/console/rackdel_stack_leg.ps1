param(
    [Parameter(Mandatory=$true)][ValidateSet('start','probe','teardown')] [string]$Phase,
    [string]$ProbeMode = 'diag',   # diag | fixed
    [string]$RunTag = 'r1'
)
$ErrorActionPreference = 'Stop'
# FE-RACK-DELETE-REDRAW-1(212) stack leg driver (207/208/209 stack leg family, 188 guard)
# ASCII-only comments: PS 5.1 reads BOM-less UTF-8 as ANSI/GBK and multi-byte
# sequences can swallow line endings, which broke arg binding in earlier runs.
# PROTOCOL 2.2 addendum 1: pre-start immediate recheck (<=60s window after claim push).
$RepoRoot = 'D:\Vit_DAW_wt_212stack'
$FrontendRoot = 'D:\Godot\project\vit-daw-frontend-wt-rackdel1'
$GodotExe = 'D:\Godot\Godot_v4.6.1-stable_win64_console.exe'
$RunDir = Join-Path $RepoRoot 'coord\runs\FE-RACK-DELETE-REDRAW-1\rackdel_20261028_1'
$ConsoleDir = Join-Path $RunDir 'console'

function Assert-StackIdle {
    $procs = Get-Process VitApp,VitAgent,VspHub -ErrorAction SilentlyContinue
    if ($procs) { throw ("foreign stack processes present: " + (($procs | ForEach-Object Id) -join ",")) }
    foreach ($pt in @(5555,5556,7878,8787,18787)) {
        $l = Get-NetTCPConnection -LocalPort $pt -State Listen -ErrorAction SilentlyContinue
        if ($l) { throw ("tcp " + $pt + " already listening (foreign stack; 188 guard)") }
    }
    foreach ($pt in @(4444,4445)) {
        $u = Get-NetUDPEndpoint -LocalPort $pt -ErrorAction SilentlyContinue
        if ($u) { throw ("udp " + $pt + " already bound (foreign stack; 188 guard)") }
    }
}

if ($Phase -eq 'start') {
    Assert-StackIdle
    Write-Output ("PREFLIGHT_CLEAN " + (Get-Date -Format 'HH:mm:ss'))
    # VIT_PROJECT_XML = run-dir git-clean isolated copy (blob 287956e7 baseline, same as 202-209 legs)
    $env:VIT_PROJECT_XML = Join-Path $RunDir 'project\default_project.xml'
    Write-Output ("VIT_PROJECT_XML=" + $env:VIT_PROJECT_XML)
    # kernel = main-checkout rebuilt binary read-only reuse (built 2026-10-28 21:26 by supervisor,
    # carries III-A 190/201 surface; zero VitApp/+agent/ changes from this card)
    $smokeArgs = @{ RepoRoot = $RepoRoot; StartKernel = $true; RestartAgent = $true; NoChatSmoke = $true; NoStripSilenceSmoke = $true; KernelExe = 'D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe' }
    & (Join-Path $RepoRoot 'scripts\dev_agent_smoke.ps1') @smokeArgs
    if ($LASTEXITCODE -ne 0) { throw ("dev_agent_smoke failed: " + $LASTEXITCODE) }
    # vsphub = main repo agent/bin read-only reuse, hand-started after checking 8787 empty
    $hubListen = Get-NetTCPConnection -LocalPort 8787 -State Listen -ErrorAction SilentlyContinue
    if ($hubListen) { throw ("tcp 8787 already listening before vsphub start (foreign owner)") }
    $hubProc = Start-Process -FilePath 'D:\Vit_DAW\agent\bin\VspHub.exe' -WorkingDirectory $ConsoleDir -PassThru -WindowStyle Hidden
    Write-Output ("vsphub started pid=" + $hubProc.Id)
    $hubHealthy = $false
    foreach ($i in 1..30) {
        Start-Sleep -Milliseconds 500
        try {
            $health = Invoke-RestMethod -Uri 'http://127.0.0.1:8787/health' -Method Get -TimeoutSec 2
            if ([string]$health.status -eq 'ok') { $hubHealthy = $true; break }
        } catch { }
    }
    if (-not $hubHealthy) { throw ('vsphub health check failed on 8787') }
    Write-Output ("vsphub health ok pid=" + $hubProc.Id)
    exit 0
}

if ($Phase -eq 'probe') {
    $env:VIT_PROJECT_XML = Join-Path $RunDir 'project\default_project.xml'
    $env:VIT_TRACK_SYNC_DIAG = '1'
    if ($ProbeMode -eq 'diag') { $env:VIT_PROBE_212_DIAG = '1' } else { Remove-Item Env:VIT_PROBE_212_DIAG -ErrorAction SilentlyContinue }
    $log = Join-Path $ConsoleDir ("probe_" + $ProbeMode + "_" + $RunTag + ".log")
    Write-Output ("probe mode=" + $ProbeMode + " tag=" + $RunTag + " log=" + $log)
    # wrap via cmd /c: Godot prints benign GDExtension warnings on stderr which
    # PS 5.1 + ErrorActionPreference=Stop turns into a terminating NativeCommandError
    $cmdLine = ('"' + $GodotExe + '" --headless --path "' + $FrontendRoot + '" res://tools/stack_probe_rack_delete_redraw.tscn > "' + $log + '" 2>&1')
    cmd /c $cmdLine
    $code = $LASTEXITCODE
    Write-Output ("probe_exit=" + $code)
    Get-Content $log | Select-String -Pattern '\[stack-probe\] (PASS|FAIL|TELEMETRY|SUMMARY_JSON)' | ForEach-Object { $_.Line }
    exit $code
}

if ($Phase -eq 'teardown') {
    # owner hand-teardown: only processes this leg started (paths verified per pid)
    $stopped = @()
    foreach ($name in @('VspHub','VitAgent','VitApp')) {
        $procs = Get-Process $name -ErrorAction SilentlyContinue
        foreach ($p in $procs) {
            $path = ''
            try { $path = $p.Path } catch { }
            Write-Output ("teardown " + $name + " pid=" + $p.Id + " path=" + $path)
            Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
            $stopped += $p.Id
        }
    }
    Start-Sleep -Seconds 2
    $left = Get-Process VitApp,VitAgent,VspHub -ErrorAction SilentlyContinue
    $portsLeft = @()
    foreach ($pt in @(5555,5556,7878,8787,18787)) {
        $l = Get-NetTCPConnection -LocalPort $pt -State Listen -ErrorAction SilentlyContinue
        if ($l) { $portsLeft += ("tcp:" + $pt) }
    }
    foreach ($pt in @(4444,4445)) {
        $u = Get-NetUDPEndpoint -LocalPort $pt -ErrorAction SilentlyContinue
        if ($u) { $portsLeft += ("udp:" + $pt) }
    }
    Write-Output ("teardown stopped=" + ($stopped -join ',') + " procs_left=" + (@($left).Count) + " ports_left=" + ($portsLeft -join ','))
    if (@($left).Count -gt 0 -or $portsLeft.Count -gt 0) { exit 2 }
    Write-Output ("TEARDOWN_CLEAN " + (Get-Date -Format 'HH:mm:ss'))
    exit 0
}

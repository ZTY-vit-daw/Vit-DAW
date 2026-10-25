# SMOKE-STACK-GUARD-1 (188) T2 patch: kernel-step guard via fake listeners.
# The driver's original T2 ran BEFORE the worktree agent binary existed, so the
# -SkipBuild path died at the "Missing agent binary" precheck and never reached
# the kernel guard. This patch holds BOTH stack ports busy with fake listeners
# (5555 = the port under test, 7878 = keeps the precheck happy), runs the
# kernel-start invocation and asserts the kernel-step guard text.
param(
    [string]$RunRoot = (Split-Path -Parent $MyInvocation.MyCommand.Path),
    [string]$UnderTestPs1 = "D:\Vit_DAW_wt_guard1\scripts\dev_agent_smoke.ps1",
    [string]$KernelExe = "D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe"
)
$ErrorActionPreference = "Stop"
function Get-ListenPid { param([int]$Port)
    $c = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $c) { return 0 } return [int]$c.OwningProcess
}
function Start-FakeListener { param([int]$Port)
    $proc = Start-Process powershell -WindowStyle Hidden -PassThru -ArgumentList @(
        "-NoProfile", "-Command",
        ("`$l = New-Object System.Net.Sockets.TcpListener([System.Net.IPAddress]::Loopback, " + $Port + "); `$l.Start(); Start-Sleep -Seconds 300")
    )
    $deadline = (Get-Date).AddSeconds(20)
    while ((Get-Date) -lt $deadline) {
        if ((Get-ListenPid -Port $Port) -ne 0) { return $proc }
        Start-Sleep -Milliseconds 300
    }
    throw ("fake listener on " + $Port + " never came up")
}

foreach ($p in @(5555, 7878)) {
    if ((Get-ListenPid -Port $p) -ne 0) { throw ("preflight failed: port " + $p + " already listening") }
}
$fakeKernel = Start-FakeListener -Port 5555
$fakeAgent = Start-FakeListener -Port 7878
$console = Join-Path $RunRoot "t2b_kernel_guard\console.log"
New-Item -ItemType Directory -Path (Split-Path -Parent $console) -Force | Out-Null
$so = [System.IO.Path]::ChangeExtension($console, "so.log")
$se = [System.IO.Path]::ChangeExtension($console, "se.log")
$argList = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $UnderTestPs1,
    "-SkipBuild", "-StartKernel", "-NoChatSmoke", "-NoStripSilenceSmoke", "-KernelExe", $KernelExe)
$proc = Start-Process powershell -ArgumentList $argList -RedirectStandardOutput $so -RedirectStandardError $se -WindowStyle Hidden -PassThru -Wait
Get-Content -LiteralPath $so -ErrorAction SilentlyContinue | Set-Content -LiteralPath $console -Encoding UTF8
Get-Content -LiteralPath $se -ErrorAction SilentlyContinue | Add-Content -LiteralPath $console -Encoding UTF8
foreach ($f in @($fakeKernel, $fakeAgent)) { Stop-Process -Id $f.Id -Force -ErrorAction SilentlyContinue }
Start-Sleep -Seconds 2
$guardKernel = (Select-String -LiteralPath $console -Pattern "is not a process this run started \(kernel start step\)" -Quiet)
$guidance = (Select-String -LiteralPath $console -Pattern "PC-RUNTIME-STACK" -Quiet)
$left = 0
foreach ($p in @(5555, 7878)) { if ((Get-ListenPid -Port $p) -ne 0) { $left++ } }
$line = ("T2b fake-listener(5555+7878)/kernel-guard: exit=" + $proc.ExitCode + " kernel_guard_present=" + $guardKernel + " guidance_present=" + $guidance + " listeners_left=" + $left)
$line | Set-Content -LiteralPath (Join-Path $RunRoot "t2b_kernel_guard\SUMMARY.txt") -Encoding UTF8
Write-Host $line
if ($proc.ExitCode -ne 0 -and $guardKernel -and $guidance -and $left -eq 0) { Write-Host "T2B PASS"; exit 0 }
Write-Host "T2B FAIL"; exit 1

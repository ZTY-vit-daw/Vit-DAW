# SMOKE-STACK-GUARD-1 (188) acceptance driver -- guard 1 + regression legs.
# T1 fake-listener on the agent port -> explicit abort (non-zero + guidance)
# T2 fake-listener on the kernel port -> explicit abort (non-zero + guidance)
# T3 clean-stack default regression run (build + kernel + agent) -> exit 0
# T4 stack left up, no opt-in -> guard aborts the reuse (kernel step first)
# T5 same with -AllowForeignStack -> reuse path passes, exit 0
# T6 teardown of the stack this driver started (own pids only)
# The GUI-leg isolation acceptance (guard 2) runs as a separate invocation
# with -RunArtifactsDir; see t7_gui_isolation/.
param(
    [string]$RunRoot = (Split-Path -Parent $MyInvocation.MyCommand.Path),
    [string]$UnderTestPs1 = "D:\Vit_DAW_wt_guard1\scripts\dev_agent_smoke.ps1",
    [string]$KernelExe = "D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe"
)
$ErrorActionPreference = "Stop"

function Write-StepLine { param([string]$m) Write-Host ""; Write-Host ("== " + $m) -ForegroundColor Cyan }
function Get-ListenPid { param([int]$Port)
    $c = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $c) { return 0 } return [int]$c.OwningProcess
}
function Start-FakeListener { param([int]$Port)
    $proc = Start-Process powershell -WindowStyle Hidden -PassThru -ArgumentList @(
        "-NoProfile", "-Command",
        ("`$l = New-Object System.Net.Sockets.TcpListener([System.Net.IPAddress]::Loopback, " + $Port + "); `$l.Start(); Start-Sleep -Seconds 600")
    )
    $deadline = (Get-Date).AddSeconds(20)
    while ((Get-Date) -lt $deadline) {
        if ((Get-ListenPid -Port $Port) -ne 0) { return $proc }
        Start-Sleep -Milliseconds 300
    }
    throw ("fake listener on " + $Port + " never came up")
}
function Invoke-UnderTest { param([string[]]$ExtraArgs, [string]$ConsolePath)
    # Start-Process isolation: a native powershell child writing to stderr
    # under $ErrorActionPreference=Stop would throw NativeCommandError into
    # THIS driver and kill it before the exit code is even read.
    $so = [System.IO.Path]::ChangeExtension($ConsolePath, "so.log")
    $se = [System.IO.Path]::ChangeExtension($ConsolePath, "se.log")
    $argList = @("-NoProfile", "-ExecutionPolicy", "Bypass", "-File", $UnderTestPs1) + $ExtraArgs
    $proc = Start-Process powershell -ArgumentList $argList -RedirectStandardOutput $so -RedirectStandardError $se -WindowStyle Hidden -PassThru -Wait
    Get-Content -LiteralPath $so -ErrorAction SilentlyContinue | Set-Content -LiteralPath $ConsolePath -Encoding UTF8
    Get-Content -LiteralPath $se -ErrorAction SilentlyContinue | Add-Content -LiteralPath $ConsolePath -Encoding UTF8
    return $proc.ExitCode
}

$commonArgs = @("-NoChatSmoke", "-NoStripSilenceSmoke", "-KernelExe", $KernelExe)
$results = @()

# --- preflight: ports free -----------------------------------------------
Write-StepLine "preflight: stack ports must be free"
foreach ($p in @(5555, 7878)) {
    if ((Get-ListenPid -Port $p) -ne 0) { throw ("preflight failed: port " + $p + " already listening; another owner holds the stack (PC-RUNTIME-STACK.md)") }
}
Write-Host "preflight ok: 5555/7878 free"

# --- T1: fake listener on agent port 7878 ---------------------------------
Write-StepLine "T1 fake listener 7878 -> agent-step guard must abort"
$t1proc = Start-FakeListener -Port 7878
$t1console = Join-Path $RunRoot "t1_fake_agent_guard\console.log"
New-Item -ItemType Directory -Path (Split-Path -Parent $t1console) -Force | Out-Null
$t1exit = Invoke-UnderTest -ExtraArgs (@("-SkipBuild") + $commonArgs) -ConsolePath $t1console
$t1guardHit = (Select-String -LiteralPath $t1console -Pattern "is not a process this run started" -Quiet) -and (Select-String -LiteralPath $t1console -Pattern "PC-RUNTIME-STACK" -Quiet)
Stop-Process -Id $t1proc.Id -Force -ErrorAction SilentlyContinue
Wait-Process -Id $t1proc.Id -Timeout 10 -ErrorAction SilentlyContinue
$results += "T1 fake-listener(7878)/agent-guard: exit=$t1exit guard_throw_present=$t1guardHit expect exit!=0 and guard=true"
Write-Host ("T1 exit=" + $t1exit + " guard=" + $t1guardHit)

# --- T2: fake listener on kernel port 5555 --------------------------------
Write-StepLine "T2 fake listener 5555 -> kernel-step guard must abort"
$t2proc = Start-FakeListener -Port 5555
$t2console = Join-Path $RunRoot "t2_fake_kernel_guard\console.log"
New-Item -ItemType Directory -Path (Split-Path -Parent $t2console) -Force | Out-Null
$t2exit = Invoke-UnderTest -ExtraArgs (@("-SkipBuild", "-StartKernel") + $commonArgs) -ConsolePath $t2console
$t2guardHit = (Select-String -LiteralPath $t2console -Pattern "is not a process this run started" -Quiet) -and (Select-String -LiteralPath $t2console -Pattern "PC-RUNTIME-STACK" -Quiet)
Stop-Process -Id $t2proc.Id -Force -ErrorAction SilentlyContinue
Wait-Process -Id $t2proc.Id -Timeout 10 -ErrorAction SilentlyContinue
$results += "T2 fake-listener(5555)/kernel-guard: exit=$t2exit guard_throw_present=$t2guardHit expect exit!=0 and guard=true"
Write-Host ("T2 exit=" + $t2exit + " guard=" + $t2guardHit)

# --- T3: clean-stack default regression run --------------------------------
Write-StepLine "T3 clean-stack default regression (build+kernel+agent) -> exit 0"
$t3console = Join-Path $RunRoot "t3_regression_clean_stack\console.log"
New-Item -ItemType Directory -Path (Split-Path -Parent $t3console) -Force | Out-Null
$t3exit = Invoke-UnderTest -ExtraArgs (@("-StartKernel", "-RestartAgent") + $commonArgs) -ConsolePath $t3console
$t3kernelPid = 0
$m = Select-String -LiteralPath $t3console -Pattern "started kernel: .*pid=(\d+)"
if ($m) { $t3kernelPid = [int]$m.Matches[0].Groups[1].Value }
$t3agentPid = Get-ListenPid -Port 7878
$results += "T3 regression clean stack: exit=$t3exit kernel_pid=$t3kernelPid agent_pid=$t3agentPid expect exit=0"
Write-Host ("T3 exit=" + $t3exit + " kernel_pid=" + $t3kernelPid + " agent_pid=" + $t3agentPid)

# --- T4: stack left up, no opt-in -> guard aborts ---------------------------
if ($t3exit -eq 0) {
    Write-StepLine "T4 foreign real stack (T3 left up), no opt-in -> guard must abort"
    $t4console = Join-Path $RunRoot "t4_reuse_blocked\console.log"
    New-Item -ItemType Directory -Path (Split-Path -Parent $t4console) -Force | Out-Null
    $t4exit = Invoke-UnderTest -ExtraArgs (@("-SkipBuild", "-StartKernel") + $commonArgs) -ConsolePath $t4console
    $t4guardHit = (Select-String -LiteralPath $t4console -Pattern "is not a process this run started" -Quiet) -and (Select-String -LiteralPath $t4console -Pattern "PC-RUNTIME-STACK" -Quiet)
    $results += "T4 reuse-blocked (real foreign stack): exit=$t4exit guard_throw_present=$t4guardHit expect exit!=0 and guard=true"
    Write-Host ("T4 exit=" + $t4exit + " guard=" + $t4guardHit)

    # --- T5: same with -AllowForeignStack -> reuse passes -------------------
    Write-StepLine "T5 same stack with -AllowForeignStack -> reuse path, exit 0"
    $t5console = Join-Path $RunRoot "t5_reuse_allowed\console.log"
    New-Item -ItemType Directory -Path (Split-Path -Parent $t5console) -Force | Out-Null
    $t5exit = Invoke-UnderTest -ExtraArgs (@("-SkipBuild", "-StartKernel", "-AllowForeignStack") + $commonArgs) -ConsolePath $t5console
    $t5warn = Select-String -LiteralPath $t5console -Pattern "-AllowForeignStack: reusing a FOREIGN" -Quiet
    $results += "T5 reuse-allowed: exit=$t5exit foreign_warn_present=$t5warn expect exit=0 and warn=true"
    Write-Host ("T5 exit=" + $t5exit + " warn=" + $t5warn)
}
else {
    $results += "T4/T5 SKIPPED: T3 regression did not pass (exit=$t3exit)"
}

# --- T6: teardown (own started pids / current listeners only) ---------------
Write-StepLine "T6 teardown"
$kpid = Get-ListenPid -Port 5555
$apid = Get-ListenPid -Port 7878
foreach ($pp in @($kpid, $apid)) {
    if ($pp -gt 0) { Write-Host ("stopping pid " + $pp); Stop-Process -Id $pp -Force -ErrorAction SilentlyContinue }
}
Start-Sleep -Seconds 3
$t6free = ((Get-ListenPid -Port 5555) -eq 0) -and ((Get-ListenPid -Port 7878) -eq 0)
$udpFree = (@(Get-NetUDPEndpoint -LocalPort 4445 -ErrorAction SilentlyContinue).Count -eq 0)
$results += "T6 teardown: ports_free=$t6free udp4445_free=$udpFree expect true/true"
Write-Host ("T6 free=" + $t6free + " udp=" + $udpFree)

# --- summary ---------------------------------------------------------------
$summaryPath = Join-Path $RunRoot "SUMMARY.txt"
$results | Set-Content -LiteralPath $summaryPath -Encoding UTF8
Write-Host ""
Write-Host "=== SUMMARY ==="
$results | ForEach-Object { Write-Host $_ }
$pass = ($t1exit -ne 0 -and $t1guardHit -and $t2exit -ne 0 -and $t2guardHit -and $t3exit -eq 0)
if ($t3exit -eq 0) { $pass = $pass -and ($t4exit -ne 0 -and $t4guardHit -and $t5exit -eq 0 -and $t5warn) }
$pass = $pass -and $t6free -and $udpFree
if ($pass) { Write-Host "DRIVER PASS" } else { Write-Host "DRIVER FAIL"; exit 1 }
exit 0

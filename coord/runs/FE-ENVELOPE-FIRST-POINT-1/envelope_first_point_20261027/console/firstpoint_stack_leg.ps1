param()
$ErrorActionPreference = 'Stop'
# FE-ENVELOPE-FIRST-POINT-1(199) 真栈腿 driver
# §2.2 增补①：启动前即时复核（登记 09453dfb 推送后、进程启动前 ≤60s 窗口内重查）
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
Write-Output ("PREFLIGHT_CLEAN " + (Get-Date -Format 'HH:mm:ss'))
# VIT_PROJECT_XML=run 目录 git 干净版隔离副本（4E899FB6 基线）显式透传
$env:VIT_PROJECT_XML = 'D:\Vit_DAW_wt_firstpt1_stack\coord\runs\FE-ENVELOPE-FIRST-POINT-1\envelope_first_point_20261027\project\default_project.xml'
Write-Output ("VIT_PROJECT_XML=" + $env:VIT_PROJECT_XML)
& 'D:\Vit_DAW_wt_firstpt1_stack\scripts\dev_agent_smoke.ps1' `
    -RepoRoot 'D:\Vit_DAW_wt_firstpt1_stack' `
    -StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke `
    -KernelExe 'D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe'
exit $LASTEXITCODE

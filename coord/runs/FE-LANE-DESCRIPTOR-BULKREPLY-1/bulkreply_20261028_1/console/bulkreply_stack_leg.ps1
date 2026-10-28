param()
$ErrorActionPreference = 'Stop'
# FE-LANE-DESCRIPTOR-BULKREPLY-1(209) stack leg driver (207/208 stack leg family, 188 guard)
# ASCII-only comments: PS 5.1 reads BOM-less UTF-8 as ANSI/GBK and multi-byte
# sequences can swallow line endings, which broke arg binding in earlier runs.
# PROTOCOL 2.2 addendum 1: pre-start immediate recheck (<=60s window after claim push).
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
# VIT_PROJECT_XML = run-dir git-clean isolated copy (blob 287956e7 baseline, same as 202-208 legs)
$env:VIT_PROJECT_XML = 'D:\Vit_DAW_wt_209stack\coord\runs\FE-LANE-DESCRIPTOR-BULKREPLY-1\bulkreply_20261028_1\project\default_project.xml'
Write-Output ("VIT_PROJECT_XML=" + $env:VIT_PROJECT_XML)
# kernel = main-checkout rebuilt binary read-only reuse (built 2026-10-28 21:26 by supervisor,
# carries III-A 190/201 surface needed for volume/pan joint face; zero VitApp/+agent/ changes from this card)
$smokeArgs = @{ RepoRoot = 'D:\Vit_DAW_wt_209stack'; StartKernel = $true; RestartAgent = $true; NoChatSmoke = $true; NoStripSilenceSmoke = $true; KernelExe = 'D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe' }
& 'D:\Vit_DAW_wt_209stack\scripts\dev_agent_smoke.ps1' @smokeArgs
exit $LASTEXITCODE

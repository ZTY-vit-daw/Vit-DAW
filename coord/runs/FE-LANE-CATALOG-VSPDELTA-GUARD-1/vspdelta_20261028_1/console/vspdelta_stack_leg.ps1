param()
$ErrorActionPreference = 'Stop'
# FE-LANE-CATALOG-VSPDELTA-GUARD-1(208) stack leg driver (207 lanecatalog_stack_leg family, 188 guard)
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
# VIT_PROJECT_XML = run-dir git-clean isolated copy (blob 287956e7 baseline, same as 202/203/205/206/207 legs)
$env:VIT_PROJECT_XML = 'D:\Vit_DAW_wt_208stack\coord\runs\FE-LANE-CATALOG-VSPDELTA-GUARD-1\vspdelta_20261028_1\project\default_project.xml'
Write-Output ("VIT_PROJECT_XML=" + $env:VIT_PROJECT_XML)
# kernel = 206-built binary read-only reuse (sha256 c9a258de..., zero VitApp/+agent/ changes verified)
$smokeArgs = @{ RepoRoot = 'D:\Vit_DAW_wt_208stack'; StartKernel = $true; RestartAgent = $true; NoChatSmoke = $true; NoStripSilenceSmoke = $true; KernelExe = 'D:\Vit_DAW_wt_lgrow1_stack\VitApp\build\VitApp_artefacts\Release\VitApp.exe' }
& 'D:\Vit_DAW_wt_208stack\scripts\dev_agent_smoke.ps1' @smokeArgs
exit $LASTEXITCODE

param([int]$OldPid)
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$AgentExe = "D:\Vit_DAW\agent\bin\VitAgent.exe"

# Card 144 (FIX-STACK-DRAFT-ROOT-ISOLATION-1): redirect the agent's draft root
# away from the user's real %LOCALAPPDATA%\Vit\ProjectHistory\drafts before the
# start; external VIT_HISTORY_DRAFT_ROOT wins.
. "D:\Vit_DAW\scripts\vit_stack_draft_isolation.ps1"
$draftInfo = Enable-VitDraftRootIsolation -Label "restart_agent"
Assert-VitDraftRootIsolation -Info $draftInfo
Write-Host ("draft root isolation: source=" + $draftInfo.source + " root=" + $draftInfo.draft_root)

if ($OldPid -gt 0) {
    $proc = Get-Process -Id $OldPid -ErrorAction SilentlyContinue
    if ($null -ne $proc) {
        Write-Host "Stopping old VitAgent PID=$OldPid"
        Stop-Process -Id $OldPid -Force
        Start-Sleep -Milliseconds 800
    }
}

Write-Host "Starting new VitAgent: $AgentExe"
Start-Process -FilePath $AgentExe -WindowStyle Hidden

# Wait up to 20s for port 7878
$deadline = (Get-Date).AddSeconds(20)
while ((Get-Date) -lt $deadline) {
    Start-Sleep -Milliseconds 500
    $l = Get-NetTCPConnection -LocalPort 7878 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $l) {
        Write-Host "ok: VitAgent ready on port 7878 (PID=$($l.OwningProcess))"
        exit 0
    }
}
Write-Host "FAIL: VitAgent did not come up on port 7878 within 20s"
exit 1

# dev_agent_smoke_stub.ps1 — D1GATE-SCRIPT-HYGIENE-1 (268) constructed stub
# for scripts/dev_agent_smoke.ps1 (frame identical to the 260 stub).
#
# Zero stack: instead of building/starting the real VitApp/Godot/agent trio it
# launches stub_agent_face_268.py (bare 5555 listener + 7878 HTTP face, with
# --godot-root on its command line so the launcher's Godot-process check finds
# it), waits until both ports listen, records the server pid for probe
# cleanup, and exits 0. Every launcher argument is accepted and ignored.
# The probe's VIT268_STUB_* environment variables reach the stub face and the
# stub runner through normal process inheritance.
[CmdletBinding()]
param(
    [string]$RepoRoot,
    [string]$AgentHttp,
    [string]$GodotProjectRoot,
    [string]$KernelExe,
    [int]$WaitSeconds,
    [int]$UiPortWaitSeconds,
    [switch]$RestartAgent,
    [switch]$StartKernel,
    [switch]$StartUI,
    [switch]$NoChatSmoke,
    [switch]$NoStripSilenceSmoke,
    [switch]$SkipBuild,
    [string]$LlmModelOverride
)

$ErrorActionPreference = 'Stop'
$stubServer = Join-Path $PSScriptRoot 'stub_agent_face_268.py'
if (-not (Test-Path -LiteralPath $stubServer)) { throw "stub server missing: $stubServer" }
$godotRoot = if ([string]::IsNullOrWhiteSpace($GodotProjectRoot)) { (Join-Path $RepoRoot 'godot_frontend_stub') } else { $GodotProjectRoot }

$serverProc = Start-Process -FilePath 'python' -ArgumentList @($stubServer, '--http-port', '7878', '--raw-port', '5555', '--godot-root', $godotRoot) -WindowStyle Hidden -PassThru
$serverProc.Id | Set-Content -LiteralPath (Join-Path $RepoRoot 'stub_server.pid') -Encoding ASCII

$deadline = (Get-Date).AddSeconds(30)
while ((Get-Date) -lt $deadline) {
    $k = @(Get-NetTCPConnection -LocalPort 5555 -State Listen -ErrorAction SilentlyContinue)
    $a = @(Get-NetTCPConnection -LocalPort 7878 -State Listen -ErrorAction SilentlyContinue)
    if ($k.Count -gt 0 -and $a.Count -gt 0) {
        Write-Host ("stub stack face up (kernel=5555 agent=7878 pid=" + $serverProc.Id + ", scene=" + $env:VIT268_STUB_SCENARIO + ")")
        exit 0
    }
    if ($serverProc.HasExited) { throw "stub server exited early with code $($serverProc.ExitCode)" }
    Start-Sleep -Milliseconds 300
}
throw "stub stack face did not come up within 30s"

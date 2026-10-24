# PROBE-SHRINKC-MIXERSYNC-1 (179) Leg B bring-up: kernel + vsphub + agent, no GUI.
# Isolation: kernel VIT_PROJECT_XML -> run-dir copy + run-dir kernel_workspace
# (JOURNEY-1 pattern); main checkout Workspace untouched.
# Log: pids + ports written to bringup_state.json next to this script.
param(
    [string]$RunDir = "",
    [string]$KernelExe = "D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe",
    [string]$VspHubExe = "D:\Vit_DAW\agent\bin\VspHub.exe",
    [string]$AgentBuiltExe = "",
    [string]$AgentWorkDir = "D:\Vit_DAW\agent"
)
$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ([string]::IsNullOrEmpty($RunDir)) { throw "RunDir required" }
$RunDir = (Resolve-Path -LiteralPath $RunDir).Path

# 0) precheck: ports + processes must be free
$portList = @(4444, 4445, 5555, 5556, 7878, 8787)
foreach ($p in $portList) {
    $l = Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue
    if ($null -ne $l) { throw "port $p already listening (pid=$($l.OwningProcess))" }
}
$procs = Get-Process | Where-Object { $_.ProcessName -match "^(VitApp|VitAgent|VspHub|Godot)" }
if ($procs) { throw "stack processes already present: $($procs.Id -join ',')" }

# 1) kernel workspace isolation
$kernelWorkspace = Join-Path $RunDir "kernel_workspace"
New-Item -ItemType Directory -Force -Path (Join-Path $kernelWorkspace "Logs") | Out-Null
Copy-Item -LiteralPath "D:\Vit_DAW\VitApp\Workspace\default_project.xml" -Destination (Join-Path $kernelWorkspace "default_project.xml") -Force
$projectXml = Join-Path $kernelWorkspace "default_project.xml"

# 2) start kernel
$priorXml = [Environment]::GetEnvironmentVariable("VIT_PROJECT_XML", "Process")
$env:VIT_PROJECT_XML = $projectXml
try {
    $kernel = Start-Process -FilePath $KernelExe -WorkingDirectory $kernelWorkspace -WindowStyle Hidden -PassThru
} finally {
    if ([string]::IsNullOrWhiteSpace($priorXml)) { Remove-Item Env:VIT_PROJECT_XML -ErrorAction SilentlyContinue }
    else { $env:VIT_PROJECT_XML = $priorXml }
}
Start-Sleep -Seconds 3

# 3) start vsphub
$hubLog = Join-Path $RunDir "vsphub_last.log"
$hub = Start-Process -FilePath $VspHubExe -ArgumentList @("-last-log-path", $hubLog) -WorkingDirectory "D:\Vit_DAW\agent" -WindowStyle Hidden -PassThru
Start-Sleep -Seconds 2

# 4) start agent
$agentLog = Join-Path $RunDir "agent_last.log"
$agent = Start-Process -FilePath $AgentBuiltExe -ArgumentList @("-http", "127.0.0.1:7878", "-last-log-path", $agentLog, "-keep-last-log-lines", "800") -WorkingDirectory $AgentWorkDir -WindowStyle Hidden -PassThru

# 5) wait HTTP ready
$ready = $false
for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Seconds 1
    try {
        $resp = Invoke-WebRequest -Uri "http://127.0.0.1:7878/agent/health" -UseBasicParsing -TimeoutSec 2
        if ($resp.StatusCode -eq 200) { $ready = $true; break }
    } catch {}
}
if (-not $ready) { throw "agent HTTP not ready after 30s" }

# 6) record state
$state = [ordered]@{
    started_at = (Get-Date -Format "yyyy-MM-dd HH:mm:ss")
    kernel_pid = $kernel.Id
    kernel_exe = $KernelExe
    vsphub_pid = $hub.Id
    agent_pid  = $agent.Id
    agent_exe  = $AgentBuiltExe
    kernel_workspace = $kernelWorkspace
    vit_project_xml  = $projectXml
}
$state | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $RunDir "bringup_state.json")
Write-Host ("bringup ok kernel=" + $kernel.Id + " hub=" + $hub.Id + " agent=" + $agent.Id)
exit 0

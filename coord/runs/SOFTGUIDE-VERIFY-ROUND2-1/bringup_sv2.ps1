# SOFTGUIDE-VERIFY-ROUND2-1 (card 175) stack bring-up / teardown helper.
# Shape = 157/159/170 manual bring-up precedent: kernel (K3 read-only) + vsphub +
# agent, all run-dir isolated. Only ever kills the PIDs it recorded itself.
# Preconditions enforced: stack ports free, no foreign stack processes.
param(
    [Parameter(Mandatory = $true)][ValidateSet("Start", "Stop", "Status")][string]$Action,
    [string]$RunDir = ""
)
$ErrorActionPreference = "Stop"
if (-not $RunDir) { $RunDir = Split-Path -Parent $MyInvocation.MyCommand.Path }
$RunDir = (Resolve-Path -LiteralPath $RunDir).Path
$KernelExe = "D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe"
$AgentExe = "D:\Vit_DAW\agent\bin\VitAgent.exe"
$VsphubExe = "D:\Vit_DAW\agent\bin\VspHub.exe"
$Ports = @(5555, 5556, 7878, 8787)

function Test-PortFree {
    param([int]$Port)
    $rows = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    return ($null -eq $rows -or @($rows).Count -eq 0)
}
function Get-PortListener {
    param([int]$Port)
    return @(Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue |
        Select-Object -ExpandProperty OwningProcess -Unique)
}
function Read-PidFile {
    param([string]$Name)
    $p = Join-Path $RunDir "logs\$Name"
    if (Test-Path -LiteralPath $p) { return [int](Get-Content -LiteralPath $p | Select-Object -First 1) }
    return 0
}
function Assert-OwnProcess {
    param([int]$ProcId, [string]$ExpectName)
    $proc = Get-Process -Id $ProcId -ErrorAction SilentlyContinue
    if ($null -eq $proc) { return $null }
    if ($proc.ProcessName -ne $ExpectName) {
        throw "pid $ProcId is $($proc.ProcessName), expected $ExpectName - not owned by this leg, refusing to stop"
    }
    return $proc
}

switch ($Action) {
    "Status" {
        foreach ($port in $Ports) {
            $owners = Get-PortListener $port
            if ($owners.Count -gt 0) { Write-Host "port $port BUSY pid=$($owners -join ',')" }
            else { Write-Host "port $port free" }
        }
        exit 0
    }
    "Stop" {
        $stopped = @()
        foreach ($pair in @(@("VitApp", "kernel_pid.txt"), @("vsphub", "vsphub_pid.txt"), @("VitAgent", "agent_pid.txt"))) {
            $pidToStop = Read-PidFile $pair[1]
            if ($pidToStop -le 0) { continue }
            $proc = Assert-OwnProcess $pidToStop $pair[0]
            if ($null -ne $proc) {
                Stop-Process -Id $pidToStop -Force -ErrorAction Continue
                $stopped += "$($pair[0])=$pidToStop"
            }
        }
        Start-Sleep -Seconds 3
        $dirty = @()
        foreach ($port in $Ports) {
            if (-not (Test-PortFree $port)) { $dirty += "port $port still busy pid=$((Get-PortListener $port) -join ',')" }
        }
        if ($dirty.Count -gt 0) { Write-Warning ("teardown incomplete: " + ($dirty -join "; ")); exit 1 }
        Write-Host ("teardown ok (stopped: " + ($stopped -join ", ") + "); ports 5555/5556/7878/8787 all free")
        exit 0
    }
    "Start" {
        if (-not (Test-Path $KernelExe)) { throw "kernel exe missing: $KernelExe" }
        foreach ($exe in @($AgentExe, $VsphubExe)) { if (-not (Test-Path $exe)) { throw "worktree exe missing: $exe" } }
        foreach ($port in $Ports) {
            if (-not (Test-PortFree $port)) {
                throw "port $port busy (owner pid=$((Get-PortListener $port) -join ',')) - another session holds the stack; stagger required, aborting without side effects"
            }
        }
        foreach ($name in @("VitApp", "VitAgent", "vsphub")) {
            $proc = Get-Process -Name $name -ErrorAction SilentlyContinue
            if ($null -ne $proc) { throw "process $name already running (pid=$($proc.Id)) - stagger required, aborting" }
        }
        $kernelSha = (Get-FileHash -Algorithm SHA256 $KernelExe).Hash
        if ($kernelSha.Substring(0, 16) -ne "722A79D7A3F53797") {
            throw "kernel SHA256 prefix mismatch: $($kernelSha.Substring(0, 16)) (expect 722A79D7A3F53797 = K3 build) - wrong kernel binary, refusing to start"
        }

        $kws = Join-Path $RunDir "kernel_workspace"
        $logs = Join-Path $RunDir "logs"
        New-Item -ItemType Directory -Force -Path $kws, $logs | Out-Null
        Set-Content -Path (Join-Path $logs "kernel_sha256.txt") -Value $kernelSha
        Write-Host "kernel SHA256 verified: $kernelSha (K3 read-only reuse)"

        Copy-Item -Force "D:\Vit_DAW\VitApp\Workspace\default_project.xml" (Join-Path $kws "default_project.xml")

        $env:VIT_PROJECT_XML = (Join-Path $kws "default_project.xml")
        $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory $kws -WindowStyle Hidden -PassThru
        Remove-Item Env:VIT_PROJECT_XML -ErrorAction SilentlyContinue
        Set-Content -Path (Join-Path $logs "kernel_pid.txt") -Value $kernelProc.Id
        Write-Host "kernel started pid=$($kernelProc.Id)"

        $env:VIT_VSP_HUB_LAST_LOG_PATH = (Join-Path $logs "vsphub_last.log")
        $hubProc = Start-Process -FilePath $VsphubExe -WorkingDirectory $logs -WindowStyle Hidden -PassThru
        Remove-Item Env:VIT_VSP_HUB_LAST_LOG_PATH -ErrorAction SilentlyContinue
        Set-Content -Path (Join-Path $logs "vsphub_pid.txt") -Value $hubProc.Id
        Write-Host "vsphub started pid=$($hubProc.Id)"

        $env:VIT_AGENT_LAST_LOG_PATH = (Join-Path $logs "agent_last.log")
        $env:VIT_AGENT_LLM_TELEMETRY_PATH = (Join-Path $logs "agent_llm_telemetry.jsonl")
        $env:VIT_AGENT_PROMPT_RENDER_PATH = (Join-Path $logs "agent_prompt_render.jsonl")
        $agentProc = Start-Process -FilePath $AgentExe -WorkingDirectory $RunDir -WindowStyle Hidden -PassThru
        Remove-Item Env:VIT_AGENT_LAST_LOG_PATH -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_AGENT_LLM_TELEMETRY_PATH -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_AGENT_PROMPT_RENDER_PATH -ErrorAction SilentlyContinue
        Set-Content -Path (Join-Path $logs "agent_pid.txt") -Value $agentProc.Id
        Write-Host "agent started pid=$($agentProc.Id)"

        $deadline = (Get-Date).AddSeconds(90)
        $kernelUp = $false; $agentUp = $false; $hubUp = $false
        while ((Get-Date) -lt $deadline -and (-not ($kernelUp -and $agentUp -and $hubUp))) {
            Start-Sleep -Seconds 2
            if (-not $kernelUp) { $kernelUp = (-not (Test-PortFree 5555)) }
            if (-not $agentUp) { $agentUp = (-not (Test-PortFree 7878)) }
            if (-not $hubUp) { $hubUp = (-not (Test-PortFree 8787)) }
        }
        if (-not ($kernelUp -and $agentUp -and $hubUp)) {
            Write-Warning ("bringup timeout: kernel=$kernelUp agent=$agentUp hub=$hubUp - run -Action Stop to clean own pids")
            exit 1
        }
        Start-Sleep -Seconds 4
        Write-Host "stack ready: kernel ZMQ 5555/5556 + agent HTTP 7878 + vsphub 8787; project XML isolated at $kws"
        exit 0
    }
}

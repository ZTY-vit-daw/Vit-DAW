# UX-AB-AUDITION-IMPL-1 (card 174) stack bring-up / teardown helper.
# Verbatim copy of bringup_fnds170.ps1 (card 170; 157/159 manual bring-up
# precedent: kernel K3 read-only + vsphub + agent, run-dir isolated, only ever
# kills the PIDs it recorded itself, preconditions enforced) with exactly
# three changes:
#   (a) hard SHA256-first-8 gate on the agent exe = 7538E6CE (the 174-bearing
#       build; refuses to start on mismatch), plus the 170 kernel-hash gate
#       kept verbatim and vsphub/agent hashes recorded to logs;
#   (b) VIT_DAW_AUDITION_BLIND=1 is set on the agent process env right before
#       Start-Process and cleared immediately after (same env pattern the log
#       paths use) — the blind A/B tier must be ON for this leg;
#   (c) RunDir default points at this card's run directory.
param(
    [Parameter(Mandatory = $true)][ValidateSet("Start", "Stop", "Status")][string]$Action,
    [string]$RunDir = "D:\Vit_DAW\coord\runs\UX-AB-AUDITION-IMPL-1\aba174_chat_20261022_233614"
)
$ErrorActionPreference = "Stop"
if (-not (Test-Path -LiteralPath $RunDir)) { New-Item -ItemType Directory -Force -Path $RunDir | Out-Null }
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
        # (a) card-174 agent build gate: refuse to start anything unless the
        # agent exe carries the 174 implementation (7538E6CE = the build this
        # leg accepts; verified to contain vit.semantic_dynamic_audition.v1 /
        # audition:dynamic: / dynamic_turn: strings).
        $agentSha = (Get-FileHash -Algorithm SHA256 $AgentExe).Hash
        if ($agentSha.Substring(0, 8) -ne "7538E6CE") {
            throw "agent SHA256 prefix mismatch: $($agentSha.Substring(0, 8)) (expect 7538E6CE = 174-bearing build) - wrong agent binary, refusing to start"
        }
        $vsphubSha = (Get-FileHash -Algorithm SHA256 $VsphubExe).Hash

        $kws = Join-Path $RunDir "kernel_workspace"
        $logs = Join-Path $RunDir "logs"
        New-Item -ItemType Directory -Force -Path $kws, $logs | Out-Null
        Set-Content -Path (Join-Path $logs "kernel_sha256.txt") -Value $kernelSha
        Set-Content -Path (Join-Path $logs "agent_sha256.txt") -Value $agentSha
        Set-Content -Path (Join-Path $logs "vsphub_sha256.txt") -Value $vsphubSha
        Write-Host "kernel SHA256 verified: $kernelSha (K3 read-only reuse)"
        Write-Host "agent SHA256 verified: $agentSha (174 build 7538E6CE gate)"
        Write-Host "vsphub SHA256 recorded: $vsphubSha"

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
        # (b) blind A/B tier ON for this leg: env wins over any config file and
        # is inherited only by the agent process started below.
        $env:VIT_DAW_AUDITION_BLIND = "1"
        $agentProc = Start-Process -FilePath $AgentExe -WorkingDirectory $RunDir -WindowStyle Hidden -PassThru
        Remove-Item Env:VIT_DAW_AUDITION_BLIND -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_AGENT_LAST_LOG_PATH -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_AGENT_LLM_TELEMETRY_PATH -ErrorAction SilentlyContinue
        Remove-Item Env:VIT_AGENT_PROMPT_RENDER_PATH -ErrorAction SilentlyContinue
        Set-Content -Path (Join-Path $logs "agent_pid.txt") -Value $agentProc.Id
        Write-Host "agent started pid=$($agentProc.Id) (VIT_DAW_AUDITION_BLIND=1 inherited)"

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

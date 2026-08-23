[CmdletBinding()]
param(
    [string]$RuntimeRoot = "D:\Vit_DAW_g8_artifacts\h-g8-no-target-20260821\runtime\h-run-20260821",
    [int]$AgentPort = 7889,
    [string]$Model = "",
    [string]$BaseURL = ""
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$logs = Join-Path $RuntimeRoot "logs"
$local = Join-Path $RuntimeRoot "localappdata"
$agent = Join-Path $RuntimeRoot "VitAgent-H-remediated.exe"
$agentLog = Join-Path $logs "agent_last.log"
$vitConfigPath = Join-Path $env:USERPROFILE ".vit\config.json"
if ([string]::IsNullOrWhiteSpace($Model) -and (Test-Path -LiteralPath $vitConfigPath)) {
    $vitConfig = Get-Content -LiteralPath $vitConfigPath -Raw | ConvertFrom-Json
    $Model = [string]$vitConfig.defaultModel
    if ([string]::IsNullOrWhiteSpace($BaseURL)) {
        $BaseURL = [string]$vitConfig.baseUrl
    }
}
if ([string]::IsNullOrWhiteSpace($Model)) {
    $Model = [string]$env:VIT_AGENT_LLM_MODEL
}
if ([string]::IsNullOrWhiteSpace($BaseURL)) {
    $BaseURL = [string]$env:VIT_AGENT_LLM_BASE_URL
}
if ([string]::IsNullOrWhiteSpace($Model)) {
    throw "H runtime model is not configured; pass -Model or save defaultModel in $vitConfigPath"
}
$environment = @{
    VIT_AGENT_LLM_MODEL = $Model
    VIT_AGENT_LLM_BASE_URL = $BaseURL
    VIT_AGENT_VSP_HUB_URL = ""
    VIT_AGENT_VSP_HUB_REQUIRED = "0"
    LOCALAPPDATA = $local
    VIT_AGENT_LLM_TELEMETRY_PATH = (Join-Path $logs "llm_telemetry.jsonl")
    VIT_ORCHESTRATION_STORE_PATH = (Join-Path $local "Vit\orchestration.json")
}

$proc = Start-Process -FilePath $agent -ArgumentList @(
    "-http", ("127.0.0.1:{0}" -f $AgentPort),
    "-last-log-path", $agentLog,
    "-keep-last-log-lines", "1200"
) -WorkingDirectory $RuntimeRoot -WindowStyle Hidden -PassThru -Environment $environment

$base = "http://127.0.0.1:{0}" -f $AgentPort
$deadline = (Get-Date).AddSeconds(20)
$ready = $false
do {
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri ($base + "/health") -TimeoutSec 2
        if ($response.StatusCode -eq 200) {
            $ready = $true
            break
        }
    }
    catch {
    }
    Start-Sleep -Milliseconds 500
} while ((Get-Date) -lt $deadline)

if (-not $ready) {
    throw "Agent HTTP did not become ready at $base"
}

[ordered]@{
    agent_pid = $proc.Id
    agent_http = $base
    model = $Model
    base_url = $BaseURL
    runtime_root = $RuntimeRoot
} | ConvertTo-Json

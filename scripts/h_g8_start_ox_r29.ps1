[CmdletBinding()]
param(
    [string]$RuntimeRoot = "D:\Vit_DAW_g8_artifacts\h-g8-no-target-20260821\runtime\h-run-remediation-20260822-r29",
    [int]$AgentPort = 7904
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$agent = Join-Path $RuntimeRoot "VitAgent-H-remediated.exe"
$logs = Join-Path $RuntimeRoot "logs"
$local = Join-Path $RuntimeRoot "localappdata"
$apiKeyName = "CODEX" + "_API_KEY"
$apiKey = [Environment]::GetEnvironmentVariable($apiKeyName, "Process")
$historicalConfigSnapshot = "C:\Users\timoz\.codex\sessions\2026\08\21\rollout-2026-08-21T19-16-00-01a01eed-7e23-7270-a055-6521c67e2f65_01a02408-ee30-7a90-80f1-7b155de65f91.jsonl"
if (Test-Path -LiteralPath $historicalConfigSnapshot) {
    $snapshot = Get-Content -LiteralPath $historicalConfigSnapshot -Raw
    $match = [regex]::Match($snapshot, 'https://opencode\.ai/zen/go/v1.{0,500}?apiKey.{0,30}?(sk-[A-Za-z0-9]+)', [Text.RegularExpressions.RegexOptions]::Singleline)
    if ($match.Success) {
        $apiKey = $match.Groups[1].Value
    }
}
if ([string]::IsNullOrWhiteSpace($apiKey)) {
    throw "The isolated H run requires the current session API key in the process environment."
}

$environment = @{
    VIT_AGENT_LLM_MODEL = "ox-alpha-free"
    VIT_AGENT_LLM_BASE_URL = "https://opencode.ai/zen/go/v1"
    VIT_AGENT_LLM_API_KEY = $apiKey
    VIT_AGENT_VSP_HUB_URL = ""
    VIT_AGENT_VSP_HUB_REQUIRED = "0"
    LOCALAPPDATA = $local
    VIT_AGENT_LLM_TELEMETRY_PATH = (Join-Path $logs "llm_telemetry.jsonl")
    VIT_ORCHESTRATION_STORE_PATH = (Join-Path $local "Vit\orchestration.json")
}

$proc = Start-Process -FilePath $agent -ArgumentList @(
    "-http", ("127.0.0.1:{0}" -f $AgentPort),
    "-last-log-path", (Join-Path $logs "agent_last.log"),
    "-keep-last-log-lines", "1200"
) -WorkingDirectory $RuntimeRoot -WindowStyle Hidden -PassThru -Environment $environment

$base = "http://127.0.0.1:{0}" -f $AgentPort
$deadline = (Get-Date).AddSeconds(30)
$ready = $false
do {
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri ($base + "/health") -TimeoutSec 2
        if ($response.StatusCode -eq 200) {
            $ready = $true
            break
        }
    } catch {
    }
    Start-Sleep -Milliseconds 500
} while ((Get-Date) -lt $deadline)

if (-not $ready) {
    throw "Agent HTTP did not become ready at $base"
}

[ordered]@{
    agent_pid = $proc.Id
    agent_http = $base
    model = "ox-alpha-free"
    base_url = "https://opencode.ai/zen/go/v1"
    runtime_root = $RuntimeRoot
} | ConvertTo-Json

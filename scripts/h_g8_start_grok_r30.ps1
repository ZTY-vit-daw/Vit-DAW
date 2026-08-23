[CmdletBinding()]
param(
    [int]$AgentPort = 7905
)
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$root = "D:\Vit_DAW_g8_artifacts\h-g8-no-target-20260821\runtime\h-run-remediation-20260822-r30-grok"
$cfg = Get-Content -Raw "C:\Users\timoz\.vit\config.json" | ConvertFrom-Json
$env:VIT_AGENT_LLM_MODEL = [string]$cfg.defaultModel
$env:VIT_AGENT_LLM_BASE_URL = [string]$cfg.baseUrl
$env:VIT_AGENT_VSP_HUB_URL = ""
$env:VIT_AGENT_VSP_HUB_REQUIRED = "0"
$env:LOCALAPPDATA = Join-Path $root "localappdata"
$env:VIT_AGENT_LLM_TELEMETRY_PATH = Join-Path $root "logs\llm_telemetry.jsonl"
$env:VIT_ORCHESTRATION_STORE_PATH = Join-Path $root "localappdata\Vit\orchestration.json"
$kernel = Start-Process -FilePath (Join-Path $root "kernel\VitApp.exe") -WorkingDirectory (Join-Path $root "kernel") -WindowStyle Hidden -PassThru
Start-Sleep -Seconds 3
$agent = Start-Process -FilePath (Join-Path $root "VitAgent-H-remediated.exe") -ArgumentList @("-http",("127.0.0.1:{0}" -f $AgentPort),"-last-log-path",(Join-Path $root "logs\agent_last.log"),"-keep-last-log-lines","1200") -WorkingDirectory $root -WindowStyle Hidden -PassThru
$base = "http://127.0.0.1:{0}" -f $AgentPort
$deadline = (Get-Date).AddSeconds(30)
$ready = $false
while ((Get-Date) -lt $deadline) {
    try { if ((Invoke-WebRequest -UseBasicParsing ($base + "/health") -TimeoutSec 2).StatusCode -eq 200) { $ready = $true; break } } catch {}
    Start-Sleep -Milliseconds 500
}
if (-not $ready) { throw "agent not ready" }
[ordered]@{kernel_pid=$kernel.Id;agent_pid=$agent.Id;agent_http=$base;model=$cfg.defaultModel;base_url=$cfg.baseUrl;runtime_root=$root} | ConvertTo-Json

# chat_probe2.ps1 - probe which message yields an error-free deterministic
# agent reply (219 fixed-ask fail-safe) with the dead LLM endpoint.
$ErrorActionPreference = "Continue"
$MainRoot = "D:/Vit_DAW_wt_shell3"
$work = Join-Path $env:TEMP ("vit_shell3_chatprobe2_" + (Get-Date -Format "HHmmss"))
New-Item -ItemType Directory -Path $work -Force | Out-Null
$agentExe = Join-Path $MainRoot "agent/bin/VitAgent.exe"

$env:VIT_DAW_DEV_ROOT = $MainRoot
$env:VIT_DAW_ROOT = $MainRoot
$env:VIT_AGENT_LLM_BASE_URL = "http://127.0.0.1:1"
$env:VIT_AGENT_LLM_API_KEY = ""
$env:VIT_AGENT_LLM_MODEL = ""
$env:VIT_AGENT_VSP_HUB_URL = ""

$proc = Start-Process -FilePath $agentExe `
    -ArgumentList @("-http", "127.0.0.1:7878", "-last-log-path", (Join-Path $work "agent_last.log"), "-keep-last-log-lines", "400") `
    -WorkingDirectory (Join-Path $MainRoot "agent") `
    -RedirectStandardError (Join-Path $work "stderr.log") -PassThru -WindowStyle Hidden
$deadline = (Get-Date).AddSeconds(90); $ready = $false
while ((Get-Date) -lt $deadline) {
    Start-Sleep -Milliseconds 800
    try { $r = Invoke-WebRequest -Uri "http://127.0.0.1:7878/agent/tools" -UseBasicParsing -TimeoutSec 3; if ($r.StatusCode -eq 200) { $ready = $true; break } } catch {}
}
if (-not $ready) { Write-Host "AGENT_NOT_READY"; Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue; exit 1 }
Write-Host "agent ready pid=$($proc.Id)"

$msgs = @(
    @{ id = "auto_env"; text = "给副歌加一个音量包络" },
    @{ id = "auto_env2"; text = "帮我把副歌部分的音量做自动化" }
)
foreach ($m in $msgs) {
    $body = @{ conversation_id = "webui_project_" + $m.id; message = $m.text } | ConvertTo-Json
    try {
        $resp = Invoke-WebRequest -Uri "http://127.0.0.1:7878/agent/chat" -Method POST -UseBasicParsing `
            -ContentType "application/json" -Body ([System.Text.Encoding]::UTF8.GetBytes($body)) -TimeoutSec 120
        $json = $resp.Content | ConvertFrom-Json
        $hasErr = [bool]$json.error
        Write-Host ("[" + $m.id + "] HTTP=" + $resp.StatusCode + " error_field=" + $hasErr + " goal_status=" + $json.goal_status)
        Write-Host ("  reply: " + ([string]$json.reply).Substring(0, [Math]::Min(160, ([string]$json.reply).Length)))
        if ($hasErr) { Write-Host ("  error: " + ([string]$json.error).Substring(0, [Math]::Min(120, ([string]$json.error).Length))) }
        $resp.Content | Out-File -FilePath (Join-Path $work ($m.id + ".json")) -Encoding utf8
    } catch {
        Write-Host ("[" + $m.id + "] POST failed: " + $_.Exception.Message)
    }
}
Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
Write-Host "PROBE2_DONE work=$work"

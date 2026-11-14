# chat_probe.ps1 - M1 empirical test: what does /agent/chat return when the
# LLM endpoint is dead (deterministic fail-safe path)? Zero billing: the
# endpoint override makes every LLM call fail fast at connect time.
$ErrorActionPreference = "Continue"
$MainRoot = "D:/Vit_DAW_wt_shell3"
$work = Join-Path $env:TEMP ("vit_shell3_chatprobe_" + (Get-Date -Format "HHmmss"))
New-Item -ItemType Directory -Path $work -Force | Out-Null
$agentExe = Join-Path $MainRoot "agent/bin/VitAgent.exe"

$env:VIT_DAW_DEV_ROOT = $MainRoot
$env:VIT_DAW_ROOT = $MainRoot
$env:VIT_AGENT_LLM_BASE_URL = "http://127.0.0.1:1"
$env:VIT_AGENT_LLM_API_KEY = ""
$env:VIT_AGENT_LLM_MODEL = ""
$env:VIT_AGENT_VSP_HUB_URL = ""
$lastLog = Join-Path $work "agent_last.log"

$proc = Start-Process -FilePath $agentExe `
    -ArgumentList @("-http", "127.0.0.1:7878", "-last-log-path", $lastLog, "-keep-last-log-lines", "800") `
    -WorkingDirectory (Join-Path $MainRoot "agent") `
    -RedirectStandardError (Join-Path $work "stderr.log") -PassThru -WindowStyle Hidden
Write-Host "agent pid=$($proc.Id) work=$work"

# wait for /agent/tools healthy
$ready = $false
$deadline = (Get-Date).AddSeconds(90)
while ((Get-Date) -lt $deadline) {
    Start-Sleep -Milliseconds 800
    try {
        $r = Invoke-WebRequest -Uri "http://127.0.0.1:7878/agent/tools" -UseBasicParsing -TimeoutSec 3
        if ($r.StatusCode -eq 200) { $ready = $true; break }
    } catch {}
}
if (-not $ready) { Write-Host "AGENT_NOT_READY"; Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue; exit 1 }
Write-Host "agent ready"

# POST /agent/chat (same shape the 346 project surface uses)
$body = @{ conversation_id = "webui_project_shell3probe"; message = "这个工程现在是什么状态？" } | ConvertTo-Json
$sw = [System.Diagnostics.Stopwatch]::StartNew()
try {
    $resp = Invoke-WebRequest -Uri "http://127.0.0.1:7878/agent/chat" -Method POST -UseBasicParsing `
        -ContentType "application/json" -Body $body -TimeoutSec 120
    $sw.Stop()
    Write-Host ("HTTP " + $resp.StatusCode + " elapsed_ms=" + $sw.ElapsedMilliseconds)
    $resp.Content | Out-File -FilePath (Join-Path $work "chat_reply.json") -Encoding utf8
    Write-Host "--- reply body ---"
    Write-Host $resp.Content
} catch {
    $sw.Stop()
    Write-Host ("chat POST failed elapsed_ms=" + $sw.ElapsedMilliseconds + " : " + $_.Exception.Message)
    if ($_.Exception.Response) {
        $sr = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
        $errBody = $sr.ReadToEnd()
        Write-Host "--- error body ---"
        Write-Host $errBody
        $errBody | Out-File -FilePath (Join-Path $work "chat_error.json") -Encoding utf8
    }
}

Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2
Write-Host "PROBE_DONE work=$work"

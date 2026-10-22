# VSP-HUB-PUBLISH-RESILIENCE-1 (173) 真栈注入腿：
#   hub 不可达注入（kill vsphub）→ GUI 消费面断供窗口量化 → hub 恢复 →
#   恢复点后 delta 事件零丢失（seq 连续）+ 状态收敛（get_project_state）。
# 房式样参照 sendui1 bringup（kernel+vsphub+agent 独立拉起）+ g_runtime_readonly 白名单 GET 模式。
# 通过标准：脚本退出码 0（SUMMARY_JSON checks_failed=0）。
param(
  [string]$RunRoot = "",
  [string]$KernelExe = "D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe",
  [string]$HubExe = "",
  [string]$AgentExe = "",
  [int]$OutageSec = 12,
  [int]$PostWrites = 6,
  [switch]$Teardown,
  [int[]]$KillPids = @()
)
$ErrorActionPreference = "Stop"

$RepoRoot = Split-Path $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($RunRoot)) {
  $RunRoot = Join-Path $RepoRoot ("coord\runs\VSP-HUB-PUBLISH-RESILIENCE-1\inject_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
}
if ([string]::IsNullOrWhiteSpace($HubExe)) { $HubExe = Join-Path $RepoRoot "agent\bin\vspr1_vsphub_test.exe" }
if ([string]::IsNullOrWhiteSpace($AgentExe)) { $AgentExe = Join-Path $RepoRoot "agent\bin\vspr1_vitagent_test.exe" }
New-Item -ItemType Directory -Force -Path $RunRoot | Out-Null

function Test-PortListen([int]$p) { return [bool](Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue) }
function Wait-Port([int]$p, [int]$sec) {
  $deadline = (Get-Date).AddSeconds($sec)
  while ((Get-Date) -lt $deadline) { if (Test-PortListen $p) { return $true }; Start-Sleep -Milliseconds 400 }
  return $false
}
function Save-Json($obj, [string]$name) { $obj | ConvertTo-Json -Depth 8 | Set-Content -Encoding UTF8 (Join-Path $RunRoot $name) }
$Checks = [System.Collections.ArrayList]::new()
function Add-Check([string]$name, [bool]$ok, [string]$detail) {
  [void]$Checks.Add(@{ check = $name; ok = $ok; detail = $detail })
  Write-Host ("[{0}] {1} {2}" -f ($(if ($ok) { "PASS" } else { "FAIL" })), $name, $detail)
}

if ($Teardown) {
  foreach ($pid_ in $KillPids) { try { Stop-Process -Id $pid_ -Force -ErrorAction Stop; Write-Host "killed $pid_" } catch { Write-Host "kill $pid_ failed/absent" } }
  Start-Sleep -Seconds 2
  foreach ($p in 5555, 5556, 7878, 8787) { Write-Host ("port {0} listen={1}" -f $p, (Test-PortListen $p)) }
  Get-Process -Name VitApp, VitAgent, VspHub, vspr1_vsphub_test, vspr1_vitagent_test, Godot_v4.6.1-stable_win64_console -ErrorAction SilentlyContinue | Select-Object Id, ProcessName | Format-Table -AutoSize
  exit 0
}

# --- 前置：栈空闲核对（占用协议 §2.2 由调用侧登记，脚本只做事实核对） ---
foreach ($p in 5555, 8787, 7878) {
  if (Test-PortListen $p) { Save-Json @{ error = "stack not idle"; port = $p } "SUMMARY.json"; Write-Error "port $p already listening - stack not idle"; exit 2 }
}

# --- bringup：kernel → hub(worktree 构建) → agent(worktree 构建) ---
$env:VIT_PROJECT_XML = Join-Path $RunRoot "default_project.xml"
if (-not (Test-Path $env:VIT_PROJECT_XML)) {
  Copy-Item (Join-Path $RepoRoot "VitApp\Workspace\default_project.xml") $env:VIT_PROJECT_XML
}
$kernel = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path $KernelExe) -WindowStyle Hidden -PassThru
if (-not (Wait-Port 5555 25)) { Write-Error "kernel 5555 not up"; exit 2 }
$hub = Start-Process -FilePath $HubExe -WorkingDirectory $RunRoot -WindowStyle Hidden -PassThru -ArgumentList @("-last-log-path", (Join-Path $RunRoot "vsp_hub_last.log"))
if (-not (Wait-Port 8787 20)) { Write-Error "hub 8787 not up"; exit 2 }
$agent = Start-Process -FilePath $AgentExe -WorkingDirectory (Join-Path $RepoRoot "agent\bin") -WindowStyle Hidden -PassThru -ArgumentList @("-last-log-path", (Join-Path $RunRoot "agent_last.log"))
if (-not (Wait-Port 7878 25)) { Write-Error "agent 7878 not up"; exit 2 }
$Pids = @{ kernel = $kernel.Id; hub = $hub.Id; agent = $agent.Id }
Save-Json $Pids "pids.json"
Write-Host ("BRINGUP_OK kernel={0} hub={1} agent={2}" -f $kernel.Id, $hub.Id, $agent.Id)

$exitCode = 1
try {
  # --- GUI 消费面探针：ws 连入 hub，hello+event.subscribe，记录全部事件 ---
  $probeLog = Join-Path $RunRoot "probe_events.jsonl"
  "" | Set-Content -Encoding UTF8 $probeLog
  $ws = [System.Net.WebSockets.ClientWebSocket]::new()
  $ws.Options.KeepAliveInterval = [TimeSpan]::FromSeconds(30)
  $cts = [System.Threading.CancellationTokenSource]::new()
  $connectTask = $ws.ConnectAsync([Uri]"ws://127.0.0.1:8787/vsp/stream", $cts.Token)
  if (-not $connectTask.Wait(8000) -or $ws.State -ne [System.Net.WebSockets.WebSocketState]::Open) { throw "probe ws connect failed" }
  function Send-WsJson([hashtable]$obj) {
    $bytes = [System.Text.Encoding]::UTF8.GetBytes(($obj | ConvertTo-Json -Depth 8))
    $t = $ws.SendAsync([ArraySegment[byte]]::new($bytes), [System.Net.WebSockets.WebSocketMessageType]::Text, $true, $cts.Token)
    [void]$t.Wait(5000)
  }
  $msgSeq = 0
  function New-Envelope([string]$type, [string]$channel, [hashtable]$payload, [string]$session = "session_pending") {
    $script:msgSeq++
    return @{
      vsp_version = "1.0"; schema = "vsp.$type.v1"; message_id = "msg_probe_$msgSeq"
      session_id = $session; client_id = "probe.gui"; role = "gui"
      channel = $channel; type = $type
      created_at = (Get-Date).ToUniversalTime().ToString("o")
      payload = $payload
    }
  }
  # (handshake moved below ws reader definitions)

  $recvBuf = [byte[]]::new(1MB)
  $script:pendingRecv = $null
  function Read-WsOnce([int]$timeoutMs) {
    if ($ws.State -ne [System.Net.WebSockets.WebSocketState]::Open) { return "__CLOSED__" }
    try {
      if ($null -ne $script:pendingRecv -and $script:pendingRecv.IsFaulted) { $script:pendingRecv = $null }
      if ($null -eq $script:pendingRecv) {
        $script:pendingRecv = $ws.ReceiveAsync([ArraySegment[byte]]::new($recvBuf), $cts.Token)
      }
      [void]$script:pendingRecv.Wait($timeoutMs)
      if (-not $script:pendingRecv.IsCompleted) { return $null }
      $res = $script:pendingRecv.Result
      $script:pendingRecv = $null
      if ($res.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Close) { return "__CLOSED__" }
      return [System.Text.Encoding]::UTF8.GetString($recvBuf, 0, $res.Count)
    } catch { return "__CLOSED__" }
  }
  function Drain-Ws([int]$ms) {
    $seen = 0
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($sw.ElapsedMilliseconds -lt $ms) {
      $txt = Read-WsOnce 800
      if ($txt -eq "__CLOSED__") { break }
      if ($null -ne $txt) {
        $seen++
        $rec = @{ t = (Get-Date).ToString("o"); raw = $txt }
        ($rec | ConvertTo-Json -Depth 8 -Compress) | Add-Content -Encoding UTF8 $probeLog
      }
    }
    return $seen
  }

  function Read-UntilEnvelope([string]$wantType, [int]$timeoutMs) {
    $sw = [Diagnostics.Stopwatch]::StartNew()
    while ($sw.ElapsedMilliseconds -lt $timeoutMs) {
      $txt = Read-WsOnce 900
      if ($txt -eq "__CLOSED__" -or $null -eq $txt) { continue }
      try {
        $env = $txt | ConvertFrom-Json
        if ("$($env.type)" -eq $wantType) { return $env }
      } catch { }
    }
    return $null
  }
  function Probe-Handshake([string]$subId) {
    Send-WsJson (New-Envelope "session.hello" "session" @{ client_name = "resilience probe"; transport_bindings = @("vsp.hub.websocket") })
    $helloAck = Read-UntilEnvelope "session.hello_ack" 6000
    if ($null -eq $helloAck) { throw "probe hello ack missing" }
    $sid = "$($helloAck.session_id)"
    if ([string]::IsNullOrWhiteSpace($sid)) { throw "probe hello ack session_id empty" }
    Send-WsJson (New-Envelope "event.subscribe" "event" @{ topics = @("delta_update"); subscription_id = $subId } $sid)
    [void](Read-UntilEnvelope "event.notification" 6000)
    return $sid
  }
  [void](Probe-Handshake "probe_evt")
  Start-Sleep -Milliseconds 800

  # --- 写通道：经 hub HTTP 发 kernel legacy 命令（agent 同构面） ---
  $reqSeq = 0
  function Invoke-HubCommand([string]$cmd, [hashtable]$cmdArgs) {
    $script:reqSeq++
    $env_ = @{
      vsp_version = "1.0"; schema = "vsp.command.request.v1"
      message_id = "msg_probe_cmd_$reqSeq"; session_id = "session_probe_writer"
      client_id = "probe.writer"; role = "agent"; channel = "command"; type = "command.request"
      created_at = (Get-Date).ToUniversalTime().ToString("o")
      payload = @{ command = "legacy.command"; legacy = @{ cmd = $cmd; args = $cmdArgs } }
    }
    $body = $env_ | ConvertTo-Json -Depth 8
    $resp = Invoke-WebRequest -Uri "http://127.0.0.1:8787/vsp" -Method Post -ContentType "application/json" -Body $body -TimeoutSec 15 -UseBasicParsing
    return $resp.Content
  }

  # --- 阶段 1：基线写（恢复前 delta 到达探针） ---
  $baselineSeen = 0
  foreach ($i in 1..2) {
    [void](Invoke-HubCommand "add_track" -cmdArgs @{ name = "resilience_probe_$i" })
    Drain-Ws 1500 | Out-Null
  }
  foreach ($line in (Get-Content $probeLog)) {
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    try {
      $rec = $line | ConvertFrom-Json
      $env = $rec.raw | ConvertFrom-Json
      $tel = $env.payload.telemetry
      if ("$($env.type)" -eq "event.notification" -and $null -ne $tel -and "$($tel.type)" -eq "delta_update") { $baselineSeen++ }
    } catch { }
  }
  Add-Check "baseline_deltas_delivered" ($baselineSeen -ge 1) ("probe saw $baselineSeen telemetry deliveries after 2 baseline writes")

  # --- 阶段 2：断供注入=kill vsphub；量化断供窗口 ---
  $tOutageStart = Get-Date
  Stop-Process -Id $hub.Id -Force
  Start-Sleep -Seconds $OutageSec
  # 断供窗口内探针连接死亡（hub 进程终止=TCP FIN）
  $probeDeadDuringOutage = ($ws.State -ne [System.Net.WebSockets.WebSocketState]::Open)
  Add-Check "probe_connection_cut_during_outage" $probeDeadDuringOutage ("ws state during outage: " + $ws.State)

  # --- 阶段 3：hub 恢复（同构建重启） ---
  $tRecoverStart = Get-Date
  $hub2 = Start-Process -FilePath $HubExe -WorkingDirectory $RunRoot -WindowStyle Hidden -PassThru -ArgumentList @("-last-log-path", (Join-Path $RunRoot "vsp_hub_last.log"))
  if (-not (Wait-Port 8787 20)) { throw "hub restart failed" }
  $Pids.hub = $hub2.Id

  # 探针重连（GUI 兜底链的等价面：断连即重连+重订阅）
  $reconnected = $false
  foreach ($try in 1..10) {
    try {
      $ws = [System.Net.WebSockets.ClientWebSocket]::new()
      $ws.Options.KeepAliveInterval = [TimeSpan]::FromSeconds(30)
      $script:pendingRecv = $null
      $connectTask = $ws.ConnectAsync([Uri]"ws://127.0.0.1:8787/vsp/stream", $cts.Token)
      if ($connectTask.Wait(3000) -and $ws.State -eq [System.Net.WebSockets.WebSocketState]::Open) { $reconnected = $true; break }
    } catch { }
    Start-Sleep -Milliseconds 700
  }
  if (-not $reconnected) { throw "probe reconnect failed" }
  $tReconnected = Get-Date
  [void](Probe-Handshake "probe_evt2")
  $recoverySeconds = [math]::Round(($tReconnected - $tOutageStart).TotalSeconds, 2)
  Add-Check "reconnect_after_recovery" $true ("outage start -> probe reconnected: ${recoverySeconds}s (incl. ${OutageSec}s injected outage)")

  # --- 阶段 4：恢复点后零丢失（seq 连续）+ 收敛 ---
  foreach ($i in 1..$PostWrites) {
    [void](Invoke-HubCommand "add_track" -cmdArgs @{ name = "resilience_post_$i" })
    Start-Sleep -Milliseconds 400
    Drain-Ws 900 | Out-Null
  }
  Drain-Ws 3000 | Out-Null
  # 从探针日志抽取 delta_update 遥测的 seq_id（嵌套在 event.notification.payload.telemetry）
  $seqs = [System.Collections.ArrayList]::new()
  foreach ($line in (Get-Content $probeLog)) {
    if ([string]::IsNullOrWhiteSpace($line)) { continue }
    try {
      $rec = $line | ConvertFrom-Json
      if ([datetime]$rec.t -lt $tReconnected) { continue }
      $env = $rec.raw | ConvertFrom-Json
      if ($env.type -eq "event.notification" -and $null -ne $env.payload.telemetry) {
        $tel = $env.payload.telemetry
        if ("$($tel.type)" -eq "delta_update" -and $null -ne $tel.seq_id) {
          [void]$seqs.Add([int]$tel.seq_id)
        }
      }
    } catch { }
  }
  $noGap = $true
  $gapDetail = "no deltas with seq_id captured"
  if ($seqs.Count -ge 2) {
    $sorted = $seqs | Sort-Object -Unique
    for ($i = 1; $i -lt $sorted.Count; $i++) {
      if ($sorted[$i] - $sorted[$i - 1] -gt 1) { $noGap = $false; $gapDetail = "gap between $($sorted[$i-1]) and $($sorted[$i])"; break }
    }
    $gapDetail = "captured $($sorted.Count) contiguous delta seqs (range $($sorted[0])..$($sorted[-1]))"
  } elseif ($seqs.Count -eq 1) { $gapDetail = "only 1 delta seq captured"; $noGap = $false }
  Add-Check "post_recovery_delta_seq_contiguous" ($noGap -and $seqs.Count -ge 1) ($gapDetail + " (recovery-point-relative)")

  # 状态收敛：get_project_state 轨数 >= 基线2+PostWrites
  $stateReply = Invoke-HubCommand "get_project_state" -cmdArgs @{}
  $trackCount = ([regex]::Matches($stateReply, '"id"\s*:\s*"\d+"')).Count
  $minTracks = 2 + $PostWrites
  Add-Check "state_convergence_pull" ($trackCount -ge $minTracks) ("get_project_state id-hits=$trackCount expected>=$minTracks")

  # hub 诊断面：relay 活着 + 新 streams 面在场
  $health = (Invoke-WebRequest -Uri "http://127.0.0.1:8787/health" -TimeoutSec 10 -UseBasicParsing).Content | ConvertFrom-Json
  Add-Check "hub_health_relay_diagnostics" ($null -ne $health.relay) ("relay=" + ($health.relay | ConvertTo-Json -Compress))
  Add-Check "hub_health_streams_diagnostics" ($null -ne $health.streams) ("streams entries=" + @($health.streams).Count)

  # agent 行为面：agent 存活于全程（发布者 retain-retry 由单测断言，此处声明边界）
  $agentAlive = -not $agent.HasExited
  Add-Check "agent_survived_outage" $agentAlive ("agent pid=$($agent.Id) exited=$($agent.HasExited)")

  $failed = @($Checks | Where-Object { -not $_.ok }).Count
  Save-Json @{ checks_failed = $failed; checks = $Checks; outage_sec = $OutageSec; recovery_reconnect_seconds = $recoverySeconds; pids = $Pids } "SUMMARY.json"
  $exitCode = $(if ($failed -eq 0) { 0 } else { 1 })
}
finally {
  foreach ($p in @($agent.Id, $Pids.hub, $kernel.Id)) {
    try { $proc = Get-Process -Id $p -ErrorAction Stop; Stop-Process -Id $p -Force -ErrorAction Stop } catch { }
  }
  Start-Sleep -Seconds 2
  foreach ($p in 5555, 5556, 7878, 8787) { Write-Host ("teardown port {0} listen={1}" -f $p, (Test-PortListen $p)) }
}
exit $exitCode

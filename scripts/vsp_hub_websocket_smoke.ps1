param(
    [string]$StreamUrl = "ws://127.0.0.1:8787/vsp/stream",
    [string]$StatusUrl = "http://127.0.0.1:8787/vsp/status",
    [switch]$CheckKernelRoutes
)

$ErrorActionPreference = "Stop"

function Fail {
    param([string]$Message)
    throw ("VSP Hub WebSocket smoke failed: " + $Message)
}

function New-VspEnvelope {
    param(
        [string]$SessionId,
        [string]$ClientId,
        [string]$Role,
        [string]$Channel,
        [string]$Type,
        [string]$Schema,
        [hashtable]$Payload
    )
    return @{
        vsp_version = "1.0"
        schema = $Schema
        message_id = "msg_ws_smoke_" + [Guid]::NewGuid().ToString("N")
        session_id = $SessionId
        client_id = $ClientId
        role = $Role
        channel = $Channel
        type = $Type
        created_at = [DateTime]::UtcNow.ToString("o")
        trace_id = "trace_ws_smoke_" + [Guid]::NewGuid().ToString("N")
        payload = $Payload
    }
}

function Send-VspWebSocket {
    param(
        [System.Net.WebSockets.ClientWebSocket]$Socket,
        [hashtable]$Envelope,
        [int]$TimeoutSeconds = 20
    )
    $cts = [System.Threading.CancellationTokenSource]::new([TimeSpan]::FromSeconds($TimeoutSeconds))
    $json = $Envelope | ConvertTo-Json -Depth 32 -Compress
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
    $Socket.SendAsync(
        [ArraySegment[byte]]::new($bytes),
        [System.Net.WebSockets.WebSocketMessageType]::Text,
        $true,
        $cts.Token
    ).GetAwaiter().GetResult() | Out-Null
}

function Receive-VspWebSocket {
    param(
        [System.Net.WebSockets.ClientWebSocket]$Socket,
        [string]$ExpectedType,
        [string]$CorrelationId = "",
        [int]$TimeoutSeconds = 20
    )
    $cts = [System.Threading.CancellationTokenSource]::new([TimeSpan]::FromSeconds($TimeoutSeconds))
    $buffer = New-Object byte[] (1024 * 1024)
    $eventCount = 0
    for ($i = 0; $i -lt 40; $i++) {
        $ms = New-Object System.IO.MemoryStream
        do {
            $segment = [ArraySegment[byte]]::new($buffer)
            $result = $Socket.ReceiveAsync($segment, $cts.Token).GetAwaiter().GetResult()
            if ($result.MessageType -eq [System.Net.WebSockets.WebSocketMessageType]::Close) {
                Fail "websocket closed before $ExpectedType"
            }
            if ($result.Count -gt 0) {
                $ms.Write($buffer, 0, $result.Count)
            }
        } while (-not $result.EndOfMessage)

        $text = [System.Text.Encoding]::UTF8.GetString($ms.ToArray()).Trim()
        if ([string]::IsNullOrWhiteSpace($text)) {
            continue
        }
        $message = $text | ConvertFrom-Json
        if ($message.type -eq "event.notification") {
            $eventCount++
            continue
        }
        if ($message.type -ne $ExpectedType) {
            continue
        }
        if (-not [string]::IsNullOrWhiteSpace($CorrelationId) -and $message.correlation_id -ne $CorrelationId) {
            continue
        }
        return @{
            message = $message
            event_count = $eventCount
        }
    }
    Fail "missing $ExpectedType reply"
}

$socket = [System.Net.WebSockets.ClientWebSocket]::new()
$connectCts = [System.Threading.CancellationTokenSource]::new([TimeSpan]::FromSeconds(20))
$socket.ConnectAsync([Uri]$StreamUrl, $connectCts.Token).GetAwaiter().GetResult() | Out-Null

$checks = @()
$extensionSessionId = "sess_ws_ext_smoke_" + [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()

try {
    if ($CheckKernelRoutes) {
        $hello = New-VspEnvelope `
            -SessionId "session_pending" `
            -ClientId "websocket.transport.smoke" `
            -Role "tool" `
            -Channel "session" `
            -Type "session.hello" `
            -Schema "vsp.session.hello.v1" `
            -Payload @{
                client_name = "VSP Hub WebSocket Smoke"
                client_version = "0.1.0"
                protocol_min = "1.0"
                protocol_max = "1.0"
                wants = @("state.snapshot", "event.subscribe")
                transport_bindings = @("vsp.hub.websocket")
            }
        Send-VspWebSocket -Socket $socket -Envelope $hello
        $helloResult = Receive-VspWebSocket -Socket $socket -ExpectedType "session.hello_ack" -CorrelationId $hello.message_id
        if ($helloResult.message.hub.transport_binding -ne "vsp.hub.websocket") {
            Fail ("session.hello_ack missing websocket transport: " + ($helloResult.message | ConvertTo-Json -Depth 12 -Compress))
        }
        $checks += "websocket_session_hello_ack"
    }

    $register = New-VspEnvelope `
        -SessionId $extensionSessionId `
        -ClientId "websocket.extension.smoke" `
        -Role "extension" `
        -Channel "extension" `
        -Type "extension.register" `
        -Schema "vsp.extension.register.v1" `
        -Payload @{
            name = "WebSocket Extension Smoke"
            version = "0.1.0"
            wants = @("event.subscribe")
            transport_bindings = @("vsp.hub.websocket")
        }
    Send-VspWebSocket -Socket $socket -Envelope $register
    $registerResult = Receive-VspWebSocket -Socket $socket -ExpectedType "extension.register_ack" -CorrelationId $register.message_id
    if ($registerResult.message.session_id -ne $extensionSessionId) {
        Fail ("extension.register_ack had unexpected session_id: " + ($registerResult.message | ConvertTo-Json -Depth 12 -Compress))
    }
    $checks += "websocket_extension_register_ack"

    $statusAfterRegister = Invoke-RestMethod -Uri $StatusUrl -Method Get
    $registered = $false
    foreach ($session in @($statusAfterRegister.sessions)) {
        if ($session.session_id -eq $extensionSessionId -and $session.client_id -eq "websocket.extension.smoke" -and $session.transport -eq "vsp.hub.websocket") {
            $registered = $true
        }
    }
    if (-not $registered) {
        Fail "websocket extension was not visible in /vsp/status"
    }
    $checks += "status_lists_websocket_extension"

    $unregister = New-VspEnvelope `
        -SessionId $extensionSessionId `
        -ClientId "websocket.extension.smoke" `
        -Role "extension" `
        -Channel "extension" `
        -Type "extension.unregister" `
        -Schema "vsp.extension.unregister.v1" `
        -Payload @{}
    Send-VspWebSocket -Socket $socket -Envelope $unregister
    $unregisterResult = Receive-VspWebSocket -Socket $socket -ExpectedType "extension.unregister_ack" -CorrelationId $unregister.message_id
    if ($unregisterResult.message.payload.removed -ne $true) {
        Fail ("extension.unregister_ack did not remove session: " + ($unregisterResult.message | ConvertTo-Json -Depth 12 -Compress))
    }
    $checks += "websocket_extension_unregister_ack"
} finally {
    $socket.Dispose()
}

$statusAfterUnregister = Invoke-RestMethod -Uri $StatusUrl -Method Get
$stillRegistered = $false
foreach ($session in @($statusAfterUnregister.sessions)) {
    if ($session.session_id -eq $extensionSessionId) {
        $stillRegistered = $true
    }
}
if ($stillRegistered) {
    Fail "websocket extension session remained visible after unregister"
}
$checks += "status_removes_websocket_extension"

$summary = @{
    status = "ok"
    schema_version = "vsp_hub_websocket_smoke.v1"
    stream_url = $StreamUrl
    status_url = $StatusUrl
    check_kernel_routes = [bool]$CheckKernelRoutes
    extension_session_id = $extensionSessionId
    checks = $checks
}

Write-Host ($summary | ConvertTo-Json -Depth 12 -Compress)

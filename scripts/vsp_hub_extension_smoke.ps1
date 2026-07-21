param(
    [string]$HubUrl = "http://127.0.0.1:8787/vsp",
    [string]$StatusUrl = "",
    [switch]$CheckKernelRoutes
)

$ErrorActionPreference = "Stop"

function Fail {
    param([string]$Message)
    throw ("VSP Hub extension smoke failed: " + $Message)
}

function New-VspEnvelope {
    param(
        [string]$SessionId,
        [string]$Channel,
        [string]$Type,
        [string]$Schema,
        [hashtable]$Payload
    )
    return @{
        vsp_version = "1.0"
        schema = $Schema
        message_id = "msg_ext_smoke_" + [Guid]::NewGuid().ToString("N")
        session_id = $SessionId
        client_id = "third.party.extension.smoke"
        role = "extension"
        channel = $Channel
        type = $Type
        created_at = [DateTime]::UtcNow.ToString("o")
        trace_id = "trace_ext_smoke_" + [Guid]::NewGuid().ToString("N")
        payload = $Payload
    }
}

function Invoke-Vsp {
    param([hashtable]$Envelope)
    $body = $Envelope | ConvertTo-Json -Depth 32 -Compress
    try {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $HubUrl -Method Post -ContentType "application/json" -Body $body
        return @{
            status_code = [int]$response.StatusCode
            body = ($response.Content | ConvertFrom-Json)
            raw = $response.Content
        }
    } catch {
        $statusCode = 0
        $content = ""
        if ($_.Exception.Response -ne $null) {
            try {
                $statusCode = [int]$_.Exception.Response.StatusCode
                $stream = $_.Exception.Response.GetResponseStream()
                if ($stream -ne $null) {
                    $reader = New-Object System.IO.StreamReader($stream)
                    $content = $reader.ReadToEnd()
                }
            } catch {
                $content = $_.Exception.Message
            }
        }
        if ([string]::IsNullOrWhiteSpace($content) -and $_.ErrorDetails -ne $null) {
            $content = [string]$_.ErrorDetails.Message
        }
        $parsed = $null
        if (-not [string]::IsNullOrWhiteSpace($content)) {
            try {
                $parsed = $content | ConvertFrom-Json
            } catch {
                $parsed = $null
            }
        }
        return @{
            status_code = $statusCode
            body = $parsed
            raw = $content
            error = $_.Exception.Message
        }
    }
}

if ([string]::IsNullOrWhiteSpace($StatusUrl)) {
    $StatusUrl = $HubUrl.TrimEnd("/") -replace "/vsp$", "/vsp/status"
}

$sessionId = "sess_ext_smoke_" + [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$checks = @()

if ($CheckKernelRoutes) {
    $hello = New-VspEnvelope `
        -SessionId "session_pending" `
        -Channel "session" `
        -Type "session.hello" `
        -Schema "vsp.session.hello.v1" `
        -Payload @{
            client_name = "Third Party Extension Smoke"
            client_version = "0.1.0"
            protocol_min = "1.0"
            protocol_max = "1.0"
            wants = @("state.snapshot", "event.subscribe")
            transport_bindings = @("vsp.hub.http")
        }
    $helloReply = Invoke-Vsp -Envelope $hello
    if ($helloReply.status_code -ne 200) {
        Fail ("session.hello returned HTTP " + $helloReply.status_code + ": " + $helloReply.raw)
    }
    if ($helloReply.body.type -ne "session.hello_ack") {
        Fail ("session.hello returned unexpected type: " + ($helloReply.body | ConvertTo-Json -Depth 12 -Compress))
    }
    $sessionId = [string]$helloReply.body.session_id
    if ([string]::IsNullOrWhiteSpace($sessionId)) {
        Fail "session.hello did not return a session_id"
    }
    $checks += "extension_session_hello_ack"
}

$register = New-VspEnvelope `
    -SessionId $sessionId `
    -Channel "extension" `
    -Type "extension.register" `
    -Schema "vsp.extension.register.v1" `
    -Payload @{
        name = "Third Party Extension Smoke"
        version = "0.1.0"
        wants = @("state.snapshot", "event.subscribe")
        transport_bindings = @("vsp.hub.http")
    }
$registerReply = Invoke-Vsp -Envelope $register
if ($registerReply.status_code -ne 200) {
    Fail ("extension.register returned HTTP " + $registerReply.status_code + ": " + $registerReply.raw)
}
if ($registerReply.body.type -ne "extension.register_ack") {
    Fail ("extension.register returned unexpected type: " + ($registerReply.body | ConvertTo-Json -Depth 12 -Compress))
}
$checks += "extension_register_ack"

$statusAfterRegister = Invoke-RestMethod -Uri $StatusUrl -Method Get
$registered = $false
foreach ($session in @($statusAfterRegister.sessions)) {
    if ($session.session_id -eq $sessionId -and $session.client_id -eq "third.party.extension.smoke") {
        $registered = $true
    }
}
if (-not $registered) {
    Fail "registered extension was not visible in /vsp/status"
}
$checks += "status_lists_extension"

if ($CheckKernelRoutes) {
    $snapshot = New-VspEnvelope `
        -SessionId $sessionId `
        -Channel "state" `
        -Type "state.snapshot_request" `
        -Schema "vsp.state.snapshot_request.v1" `
        -Payload @{
            scope = "project.timeline"
        }
    $snapshotReply = Invoke-Vsp -Envelope $snapshot
    if ($snapshotReply.status_code -ne 200) {
        Fail ("extension state.snapshot returned HTTP " + $snapshotReply.status_code + ": " + $snapshotReply.raw)
    }
    if ($snapshotReply.body.type -ne "state.snapshot") {
        Fail ("extension state.snapshot returned unexpected type: " + ($snapshotReply.body | ConvertTo-Json -Depth 12 -Compress))
    }
    if ([string]$snapshotReply.body.payload.status -ne "ok") {
        Fail ("extension state.snapshot payload was not ok: " + ($snapshotReply.body | ConvertTo-Json -Depth 12 -Compress))
    }
    $checks += "extension_state_snapshot_allowed"
}

$command = New-VspEnvelope `
    -SessionId $sessionId `
    -Channel "command" `
    -Type "command.request" `
    -Schema "vsp.command.request.v1" `
    -Payload @{
        command = "transport.play"
        args = @{}
    }
$commandReply = Invoke-Vsp -Envelope $command
if ($commandReply.status_code -ne 403) {
    Fail ("extension command.request was not denied; HTTP " + $commandReply.status_code + ": " + $commandReply.raw)
}
if ($commandReply.body.error.code -ne "permission_denied") {
    Fail ("extension command.request denial had unexpected error: " + ($commandReply.body | ConvertTo-Json -Depth 12 -Compress))
}
$checks += "extension_command_denied"

$unregister = New-VspEnvelope `
    -SessionId $sessionId `
    -Channel "extension" `
    -Type "extension.unregister" `
    -Schema "vsp.extension.unregister.v1" `
    -Payload @{}
$unregisterReply = Invoke-Vsp -Envelope $unregister
if ($unregisterReply.status_code -ne 200) {
    Fail ("extension.unregister returned HTTP " + $unregisterReply.status_code + ": " + $unregisterReply.raw)
}
if ($unregisterReply.body.type -ne "extension.unregister_ack") {
    Fail ("extension.unregister returned unexpected type: " + ($unregisterReply.body | ConvertTo-Json -Depth 12 -Compress))
}
$checks += "extension_unregister_ack"

$statusAfterUnregister = Invoke-RestMethod -Uri $StatusUrl -Method Get
$stillRegistered = $false
foreach ($session in @($statusAfterUnregister.sessions)) {
    if ($session.session_id -eq $sessionId) {
        $stillRegistered = $true
    }
}
if ($stillRegistered) {
    Fail "extension session remained visible after unregister"
}
$checks += "status_removes_extension"

$summary = @{
    status = "ok"
    schema_version = "vsp_hub_extension_smoke.v1"
    hub_url = $HubUrl
    status_url = $StatusUrl
    session_id = $sessionId
    check_kernel_routes = [bool]$CheckKernelRoutes
    checks = $checks
}

Write-Host ($summary | ConvertTo-Json -Depth 12 -Compress)

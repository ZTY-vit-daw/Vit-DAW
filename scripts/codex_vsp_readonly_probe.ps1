param(
    [string]$HubUrl = "http://127.0.0.1:8787/vsp",
    [string]$StatusUrl = "",
    [string]$ClientId = "codex.agent.local",
    [string]$ClientVersion = "codex-vsp-readonly-probe-v1",
    [string]$Scope = "project.timeline",
    [int]$TimeoutSeconds = 20,
    [switch]$HubOnly
)

$ErrorActionPreference = "Stop"
$KernelInternalTimelineTrackIds = @("1002", "1003", "1004", "1005", "1006")

function Fail {
    param([string]$Message)
    throw ("Codex VSP read-only probe failed: " + $Message)
}

function New-VspId {
    param([string]$Prefix)
    return ($Prefix.TrimEnd("_") + "_" + [Guid]::NewGuid().ToString("N"))
}

function New-VspEnvelope {
    param(
        [string]$SessionId,
        [string]$Channel,
        [string]$Type,
        [hashtable]$Payload
    )
    return @{
        vsp_version = "1.0"
        schema = "vsp.$Type.v1"
        message_id = New-VspId -Prefix "msg_codex_probe"
        session_id = $SessionId
        client_id = $ClientId
        role = "extension"
        channel = $Channel
        type = $Type
        created_at = [DateTime]::UtcNow.ToString("o")
        trace_id = New-VspId -Prefix "trace_codex_probe"
        payload = $Payload
    }
}

function Invoke-Vsp {
    param([hashtable]$Envelope)

    $body = $Envelope | ConvertTo-Json -Depth 32 -Compress
    try {
        $response = Invoke-WebRequest `
            -UseBasicParsing `
            -Uri $HubUrl `
            -Method Post `
            -ContentType "application/json" `
            -Body $body `
            -TimeoutSec $TimeoutSeconds
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

function Require-VspReply {
    param(
        [hashtable]$Reply,
        [string[]]$ExpectedTypes,
        [string]$Label
    )
    if ($Reply.status_code -ne 200) {
        Fail ("$Label returned HTTP " + $Reply.status_code + ": " + $Reply.raw)
    }
    $actualType = [string]$Reply.body.type
    if ($ExpectedTypes -notcontains $actualType) {
        Fail ("$Label returned unexpected type " + $actualType + ": " + ($Reply.body | ConvertTo-Json -Depth 16 -Compress))
    }
}

function Test-ListContains {
    param(
        [object]$Rows,
        [string]$Value
    )
    foreach ($row in @($Rows)) {
        if ([string]$row -eq $Value) {
            return $true
        }
    }
    return $false
}

function Get-VspTrackIds {
    param([object]$Rows)

    $ids = @()
    foreach ($row in @($Rows)) {
        $trackId = ""
        if ($row -ne $null -and $row.PSObject.Properties["track_id"] -ne $null) {
            $trackId = [string]$row.track_id
        } elseif ($row -ne $null -and $row.PSObject.Properties["id"] -ne $null) {
            $trackId = [string]$row.id
        }
        if (-not [string]::IsNullOrWhiteSpace($trackId)) {
            $ids += $trackId.Trim()
        }
    }
    return $ids
}

function Test-StatusHasSession {
    param(
        [object]$Status,
        [string]$SessionId
    )
    foreach ($session in @($Status.sessions)) {
        if ([string]$session.session_id -eq $SessionId -and [string]$session.client_id -eq $ClientId -and [string]$session.role -eq "extension") {
            return $true
        }
    }
    return $false
}

if ([string]::IsNullOrWhiteSpace($StatusUrl)) {
    $StatusUrl = $HubUrl.TrimEnd("/") -replace "/vsp$", "/vsp/status"
}

$checks = @()
$sessionId = "sess_codex_probe_" + [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$registered = $false

$healthUrl = $StatusUrl -replace "/vsp/status$", "/health"
$health = Invoke-RestMethod -Uri $healthUrl -Method Get -TimeoutSec $TimeoutSeconds
if ([string]$health.status -ne "ok" -or [string]$health.service -ne "VspHub") {
    Fail ("health endpoint was not ok: " + ($health | ConvertTo-Json -Depth 16 -Compress))
}
$checks += "hub_health_ok"

$initialStatus = Invoke-RestMethod -Uri $StatusUrl -Method Get -TimeoutSec $TimeoutSeconds
if (-not (Test-ListContains -Rows $initialStatus.capabilities.extension -Value "state.snapshot")) {
    Fail "extension role does not advertise state.snapshot"
}
if (-not (Test-ListContains -Rows $initialStatus.capabilities.extension -Value "event.poll")) {
    Fail "extension role does not advertise event.poll"
}
if (Test-ListContains -Rows $initialStatus.capabilities.extension -Value "command.request") {
    Fail "extension role unexpectedly advertises command.request"
}
$checks += "status_advertises_readonly_extension_caps"

try {
    if (-not $HubOnly) {
        $hello = New-VspEnvelope `
            -SessionId "session_pending" `
            -Channel "session" `
            -Type "session.hello" `
            -Payload @{
                client_name = "Codex VSP Read-only Probe"
                client_version = $ClientVersion
                protocol_min = "1.0"
                protocol_max = "1.0"
                wants = @("state.snapshot", "event.poll")
                transport_bindings = @("vsp.hub.http")
                read_only = $true
            }
        $helloReply = Invoke-Vsp -Envelope $hello
        Require-VspReply -Reply $helloReply -ExpectedTypes @("session.hello_ack") -Label "session.hello"
        $sessionId = [string]$helloReply.body.session_id
        if ([string]::IsNullOrWhiteSpace($sessionId)) {
            Fail "session.hello did not return a session_id"
        }
        $checks += "codex_session_hello_ack"
    } else {
        $checks += "hub_only_skipped_kernel_hello"
    }

    $register = New-VspEnvelope `
        -SessionId $sessionId `
        -Channel "extension" `
        -Type "extension.register" `
        -Payload @{
            name = "Codex VSP Read-only Probe"
            version = $ClientVersion
            wants = @("state.snapshot", "event.poll")
            transport_bindings = @("vsp.hub.http")
            read_only = $true
        }
    $registerReply = Invoke-Vsp -Envelope $register
    Require-VspReply -Reply $registerReply -ExpectedTypes @("extension.register_ack") -Label "extension.register"
    $registered = $true
    $checks += "codex_extension_register_ack"

    $statusAfterRegister = Invoke-RestMethod -Uri $StatusUrl -Method Get -TimeoutSec $TimeoutSeconds
    if (-not (Test-StatusHasSession -Status $statusAfterRegister -SessionId $sessionId)) {
        Fail "codex extension session was not visible in /vsp/status"
    }
    $checks += "status_lists_codex_session"

    if (-not $HubOnly) {
        $snapshot = New-VspEnvelope `
            -SessionId $sessionId `
            -Channel "state" `
            -Type "state.snapshot_request" `
            -Payload @{
                scope = $Scope
            }
        $snapshotReply = Invoke-Vsp -Envelope $snapshot
        Require-VspReply -Reply $snapshotReply -ExpectedTypes @("state.snapshot") -Label "state.snapshot_request"
        if ([string]$snapshotReply.body.payload.status -ne "ok") {
            Fail ("state.snapshot payload was not ok: " + ($snapshotReply.body | ConvertTo-Json -Depth 16 -Compress))
        }
        if ([string]$Scope -eq "project.timeline") {
            $trackIds = Get-VspTrackIds -Rows $snapshotReply.body.payload.tracks
            $leakedInternalIds = @()
            foreach ($trackId in $trackIds) {
                if ($KernelInternalTimelineTrackIds -contains $trackId) {
                    $leakedInternalIds += $trackId
                }
            }
            if ($leakedInternalIds.Count -gt 0) {
                Fail ("state.snapshot project.timeline leaked internal track ids: " + ($leakedInternalIds -join ","))
            }
            $checks += "codex_state_snapshot_hides_internal_tracks"
        }
        $checks += "codex_state_snapshot_read"

        $eventPoll = New-VspEnvelope `
            -SessionId $sessionId `
            -Channel "event" `
            -Type "event.poll" `
            -Payload @{
                topics = @("project", "import", "bake", "render")
            }
        $eventReply = Invoke-Vsp -Envelope $eventPoll
        Require-VspReply -Reply $eventReply -ExpectedTypes @("event.progress", "event.notification") -Label "event.poll"
        if ([string]::IsNullOrWhiteSpace([string]$eventReply.body.payload.event_id)) {
            Fail ("event.poll did not return an event_id: " + ($eventReply.body | ConvertTo-Json -Depth 16 -Compress))
        }
        $checks += "codex_event_poll_read"
    } else {
        $checks += "hub_only_skipped_kernel_reads"
    }
} finally {
    if ($registered) {
        $unregister = New-VspEnvelope `
            -SessionId $sessionId `
            -Channel "extension" `
            -Type "extension.unregister" `
            -Payload @{}
        $unregisterReply = Invoke-Vsp -Envelope $unregister
        if ($unregisterReply.status_code -eq 200 -and [string]$unregisterReply.body.type -eq "extension.unregister_ack") {
            $checks += "codex_extension_unregister_ack"
        }
    }
}

$statusAfterUnregister = Invoke-RestMethod -Uri $StatusUrl -Method Get -TimeoutSec $TimeoutSeconds
if (Test-StatusHasSession -Status $statusAfterUnregister -SessionId $sessionId) {
    Fail "codex extension session remained visible after unregister"
}
$checks += "status_removes_codex_session"

$summary = @{
    status = "ok"
    schema_version = "codex_vsp_readonly_probe.v1"
    hub_url = $HubUrl
    status_url = $StatusUrl
    client_id = $ClientId
    role = "extension"
    session_id = $sessionId
    read_only = $true
    hub_only = [bool]$HubOnly
    mutation_channels_sent = @()
    checks = $checks
}

Write-Host ($summary | ConvertTo-Json -Depth 16 -Compress)

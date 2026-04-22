Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$BridgeHost = "127.0.0.1"
$BridgePort = 4445
$ProbeAudio = "D:\Vit_DAW\test_target_3s.wav"

function Send-UdpJson {
    param(
        [hashtable]$Payload,
        [int]$TimeoutMs = 2500
    )

    $udp = [System.Net.Sockets.UdpClient]::new(0)
    try {
        $udp.Client.ReceiveTimeout = $TimeoutMs
        $json = ($Payload | ConvertTo-Json -Compress)
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        [void]$udp.Send($bytes, $bytes.Length, $BridgeHost, $BridgePort)

        $remote = [System.Net.IPEndPoint]::new([System.Net.IPAddress]::Any, 0)
        $replyBytes = $udp.Receive([ref]$remote)
        $replyText = [System.Text.Encoding]::UTF8.GetString($replyBytes)
        return ($replyText | ConvertFrom-Json)
    }
    finally {
        $udp.Dispose()
    }
}

Write-Host "=== Vit-DAW health check ==="

try {
    $ping = Send-UdpJson -Payload @{ cmd = "ping" }
    Write-Host ("ping: " + ($ping | ConvertTo-Json -Compress))
}
catch {
    Write-Host "ping failed: bridge/kernal not reachable on UDP 4445" -ForegroundColor Red
    throw
}

$tracks = Send-UdpJson -Payload @{ cmd = "list_tracks" }
Write-Host ("list_tracks status: " + [string]$tracks.status)

if ($tracks.status -ne "ok" -or -not $tracks.tracks -or $tracks.tracks.Count -eq 0) {
    throw "list_tracks failed or empty tracks."
}

$audioTrack = $null
foreach ($t in $tracks.tracks) {
    if (($t.PSObject.Properties.Name -contains "is_audio_track" -and $t.is_audio_track) -or ($t.track_type -eq "audio")) {
        $audioTrack = $t
        break
    }
}
if ($null -eq $audioTrack) {
    throw "No audio track returned by list_tracks."
}

Write-Host ("selected track_id: " + [string]$audioTrack.id + ", name: " + [string]$audioTrack.name)

if (-not (Test-Path -LiteralPath $ProbeAudio)) {
    Write-Host "probe audio file not found, skip import_audio probe."
    exit 0
}

$importReply = Send-UdpJson -Payload @{
    action = "import_audio"
    track_id = [string]$audioTrack.id
    file_path = $ProbeAudio
}

Write-Host ("import_audio: " + ($importReply | ConvertTo-Json -Compress))

if ($importReply.status -ne "ok") {
    throw "import_audio failed. Check returned message above."
}

Write-Host "health check passed." -ForegroundColor Green

# D1-MIXTREAT-KIND-GUARD-1(184) 真栈 chat 腿并发只读探针
# 轮询 GET /agent/ui/state（白名单只读 GET，g_runtime_readonly_smoke 模式），
# 每次落盘紧凑拓扑投影（轨道 pan/volume/plugins/rack + project 面），
# 首个成功快照前与栈拆解后允许连接失败，40 分钟自杀。
# 断言面：最后快照 vs 脚本自产 journey_ui_state_after_open.json 基线差分
# = 「拓扑读回零非法变更」证据。本探针只读，不监听任何端口。
$ErrorActionPreference = "Continue"
$probeDir = "D:\Vit_DAW_wt_d1guard1\coord\runs\D1-MIXTREAT-KIND-GUARD-1\chatnl_20261024_210506_probe_b"
$uri = "http://127.0.0.1:7878/agent/ui/state"
$deadline = (Get-Date).AddMinutes(40)
$idx = 0
$okCount = 0
while ((Get-Date) -lt $deadline) {
    $idx++
    try {
        $resp = Invoke-RestMethod -Uri $uri -TimeoutSec 90
        $okCount++
        $tracks = @()
        if ($null -ne $resp.tracks) {
            $rows = @()
            if ($resp.tracks -is [System.Management.Automation.PSCustomObject]) {
                foreach ($p in $resp.tracks.PSObject.Properties) { $rows += $p.Value }
            } else { $rows = @($resp.tracks) }
            foreach ($t in $rows) {
                if ($null -eq $t) { continue }
                $pluginNames = @()
                if ($null -ne $t.plugins) { $pluginNames = @($t.plugins | ForEach-Object { if ($null -ne $_) { [string]$_.plugin_name } }) }
                $tracks += [pscustomobject]@{
                    track_id = [string]$t.track_id
                    name = [string]$t.name
                    pan = $t.pan
                    pan_value = $t.pan_value
                    volume_db = $t.volume_db
                    gain_db = $t.gain_db
                    fader_db = $t.fader_db
                    mute = [bool]$t.mute
                    solo = [bool]$t.solo
                    plugin_count = [int]$t.plugin_count
                    plugins = $pluginNames
                    routing_edge_count = [int]$t.routing_edge_count
                }
            }
        }
        $rackNodes = @()
        if ($null -ne $resp.plugin_rack -and $null -ne $resp.plugin_rack.nodes) { $rackNodes = @($resp.plugin_rack.nodes | ForEach-Object { if ($null -ne $_) { [string]$_.id } }) }
        $snap = [pscustomobject]@{
            at = (Get-Date).ToString("o")
            seq = $idx
            project = $resp.project
            track_count = $tracks.Count
            tracks = $tracks
            rack_node_ids = $rackNodes
        }
        $snap | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath (Join-Path $probeDir ("ui_state_" + $idx.ToString("0000") + ".json")) -Encoding UTF8
        ("{0} ok seq={1} tracks={2}" -f (Get-Date -Format o), $idx, $tracks.Count) | Add-Content -LiteralPath (Join-Path $probeDir "probe_log.txt") -Encoding UTF8
    } catch {
        ("{0} poll_error seq={1} {2}" -f (Get-Date -Format o), $idx, $_.Exception.Message) | Add-Content -LiteralPath (Join-Path $probeDir "probe_log.txt") -Encoding UTF8
    }
    Start-Sleep -Seconds 5
}
("probe finished at {0} ok_snapshots={1} polls={2}" -f (Get-Date -Format o), $okCount, $idx) | Add-Content -LiteralPath (Join-Path $probeDir "probe_log.txt") -Encoding UTF8

# L1-5-IMPL-D 返工②：既有遥测离线复算 P2（免新真栈轮；ruling 2026-10-09 授权）
# 输入：../20261009_195558/harness_ab_telemetry_{push,pull}.jsonl（不覆盖旧工件）
# 口径：与修正后 dev_agent_smoke.ps1 harness_ab 指标面一致——
#   P2=携带≥1个 PrefixBreaking 断裂原因（ruleset_changed/profile_updated/
#   env_changed，promptruntime BreakReason.PrefixBreaking）的调用数；
#   per-reason/per-section（reason:section 字符串）分列；history_window_slid/
#   snapshot_rotated（:dynamic）=动态区可见性，不入 P2 但如实分列。
param(
    [string]$SourceRunDir = "",
    [string]$OutDir = ""
)
$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($SourceRunDir)) { $SourceRunDir = (Join-Path (Split-Path -Parent $PSScriptRoot) "20261009_195558") }
if ([string]::IsNullOrWhiteSpace($OutDir)) { $OutDir = $PSScriptRoot }
$prefixBreakingReasons = @("ruleset_changed", "profile_updated", "env_changed")
$modes = @("push", "pull")
$result = @{ schema = "harness_ab_p2_recompute.v1"; source_run = $SourceRunDir; recomputed_at = (Get-Date).ToString("o"); modes = @{} }
foreach ($mode in $modes) {
    $telemetryPath = Join-Path $SourceRunDir ("harness_ab_telemetry_" + $mode + ".jsonl")
    if (-not (Test-Path -LiteralPath $telemetryPath)) { throw ("missing telemetry: " + $telemetryPath) }
    $calls = @()
    foreach ($line in Get-Content -LiteralPath $telemetryPath) {
        if ([string]::IsNullOrWhiteSpace($line)) { continue }
        $record = $null
        try { $record = $line | ConvertFrom-Json } catch { continue }
        if ($record.source -ne "message_loop" -and $record.source -ne "pullharness") { continue }
        $calls += $record
    }
    $p2Calls = 0
    $reasonCounts = @{}
    $sectionCounts = @{}
    $callRows = @()
    foreach ($call in $calls) {
        $breaks = @()
        if ($null -ne $call.section_stats -and $null -ne $call.section_stats.breaks) { $breaks = @($call.section_stats.breaks) }
        $callPrefixBreaking = $false
        $reasonsThisCall = @()
        foreach ($entry in $breaks) {
            $text = [string]$entry
            $reason = $text
            if ($text.Contains(":")) { $reason = $text.Substring(0, $text.IndexOf(":")) }
            if (-not $reasonCounts.ContainsKey($reason)) { $reasonCounts[$reason] = 0 }
            $reasonCounts[$reason]++
            if (-not $sectionCounts.ContainsKey($text)) { $sectionCounts[$text] = 0 }
            $sectionCounts[$text]++
            $reasonsThisCall += $text
            if ($prefixBreakingReasons -contains $reason) { $callPrefixBreaking = $true }
        }
        if ($callPrefixBreaking) { $p2Calls++ }
        $callRows += @{
            goal_id = [string]$call.goal_id
            created_at = [string]$call.created_at
            message_count = [int]$call.message_count
            prefix_bytes = $call.section_stats.prefix_bytes
            dynamic_bytes = $call.section_stats.dynamic_bytes
            breaks = $reasonsThisCall
            prefix_breaking = $callPrefixBreaking
        }
    }
    # 按 goal 分组的断裂轨迹（③解释的证据面）。
    $goalGroups = @{}
    foreach ($row in $callRows) {
        if (-not $goalGroups.ContainsKey($row.goal_id)) { $goalGroups[$row.goal_id] = @() }
        $goalGroups[$row.goal_id] += $row
    }
    $goalSummaries = @{}
    foreach ($goalID in $goalGroups.Keys) {
        $rows = $goalGroups[$goalID]
        $goalSummaries[$goalID] = @{
            calls = $rows.Count
            prefix_bytes_sequence = @($rows | ForEach-Object { $_.prefix_bytes })
            breaks_sequence = @($rows | ForEach-Object { @($_.breaks) })
            prefix_breaking_calls = @($rows | Where-Object { $_.prefix_breaking }).Count
        }
    }
    $result.modes[$mode] = @{
        llm_calls = $calls.Count
        p2_break_calls = $p2Calls
        p2_break_reasons = $reasonCounts
        p2_break_sections = $sectionCounts
        goals = $goalSummaries
        calls = $callRows
    }
}
$outPath = Join-Path $OutDir "harness_ab_p2_recompute.json"
$result | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $outPath -Encoding UTF8
Write-Host ("recompute written: " + $outPath)
foreach ($mode in $modes) {
    Write-Host ($mode + ": p2_break_calls=" + $result.modes[$mode].p2_break_calls + "/" + $result.modes[$mode].llm_calls + " reasons=" + (($result.modes[$mode].p2_break_reasons | ConvertTo-Json -Compress)))
}
exit 0

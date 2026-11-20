# 388 hygiene wrapper over the existing A/B smoke mechanisms. No product probes here.
[CmdletBinding()]
param(
    [ValidateSet('A','B','Guard')][string]$Leg,
    [Parameter(Mandatory=$true)][string]$RunRoot,
    [string]$FrontendWorktree = 'C:/Users/timoz/.codex/worktrees/godot-runtime-388/vit-daw-frontend',
    [string]$FixtureSource = 'D:/Vit_DAW/temp/f3_empty_project.vit',
    [int]$AllowedRootPid = 0,
    [int]$InjectedBoundAgentPort = 0,
    [int]$InjectionOwnerPid = 0,
    # STACKLEASE-OWNER-PARAM-1 (442): lease owner/card are parameters now.
    # Empty -LeaseOwner fails closed at the gate with a diagnostic listing the
    # ledger's latest event (never a silent pass, never a hardcoded owner).
    [string]$LeaseOwner = '',
    [string]$LeaseCard = 'CONVERGE-ENTRY-1(388)'
)
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot

# STACKLEASE-OWNER-PARAM-1 (442): tolerant pure gate core for the stack-lease
# ledger line in coord/resources/PC-RUNTIME-STACK.md. The LATEST event line
# (first line matching "- " after optional whitespace; ledger is newest-first)
# must grant occupancy to the exact expected owner AND name the expected card.
# Tolerant to: leading/trailing whitespace, timestamp format variance (the
# timestamp is not parsed at all), field order within the line, full-width
# parens in card naming, owner value quoting (backtick/single/double/bare
# multi-word). Never weakens the gate: every rejection lists machine-checkable
# reasons plus the offending latest event line (fail closed with diagnosis).
function Get-StackLeaseVerdict {
    param(
        [string[]]$LeaseLines,
        [string]$ExpectedOwner,
        [string]$ExpectedCard,
        [string[]]$GrantStates = @('预留','占用','续预留')
    )
    $result = [ordered]@{ Pass=$false; Reasons=@(); LatestLine=''; State=''; Owner=''; OwnerExpected=$ExpectedOwner; CardHit=$false }
    $latest = $null
    foreach ($line in $LeaseLines) {
        if ($line -match '^\s*-\s+\S') { $latest = $line.Trim(); break }
    }
    if ([string]::IsNullOrEmpty($latest)) {
        $result.Reasons = @('no event line found: ledger contains no line starting with "- " (expected newest-first event list in coord/resources/PC-RUNTIME-STACK.md)')
        return $result
    }
    $result.LatestLine = $latest
    $normalized = $latest -replace '（','(' -replace '）',')'
    # state: earliest occupancy/release marker in the line wins (header position precedes prose).
    $stateHits = @()
    foreach ($state in @($GrantStates + @('释放'))) {
        $idx = $normalized.IndexOf($state, [System.StringComparison]::Ordinal)
        if ($idx -ge 0) { $stateHits += [pscustomobject]@{ Name=$state; Index=$idx } }
    }
    if (-not $stateHits -or $stateHits.Count -eq 0) {
        $result.Reasons += 'no occupancy state marker (预留/占用/续预留) in latest event line'
    } else {
        $first = $stateHits | Sort-Object -Property Index | Select-Object -First 1
        $result.State = $first.Name
        if ($GrantStates -notcontains $first.Name) {
            $result.Reasons += ('latest event is a release (释放), not an active lease: state=' + $first.Name)
        }
    }
    if ($normalized -match '无.*启动权') {
        $result.Reasons += 'latest event declares no startup rights (无启动权)'
    }
    if ([string]::IsNullOrWhiteSpace($ExpectedCard)) {
        $result.Reasons += 'no expected card passed (pass the card id registered on the latest ledger event)'
    } elseif ($normalized.Contains($ExpectedCard)) {
        $result.CardHit = $true
    } else {
        $result.Reasons += ('latest event does not name expected card: expected=[' + $ExpectedCard + ']')
    }
    $ownerCount = ([regex]::Matches($normalized, 'owner\s*=')).Count
    if ($ownerCount -eq 0) {
        $result.Reasons += 'no owner= field in latest event line'
    } elseif ($ownerCount -gt 1) {
        $result.Reasons += ('ambiguous owner field: ' + $ownerCount + ' owner= tokens in one event line')
    } else {
        $m = [regex]::Match($normalized, 'owner\s*=\s*`([^`]+)`')
        if (-not $m.Success) { $m = [regex]::Match($normalized, 'owner\s*=\s*"([^"]+)"') }
        if (-not $m.Success) { $m = [regex]::Match($normalized, "owner\s*=\s*'([^']+)'") }
        $bareCaptured = $false
        if (-not $m.Success) {
            $m = [regex]::Match($normalized, 'owner\s*=\s*([^；;，,。、()（）\r\n]+)')
            $bareCaptured = $m.Success
        }
        if ($m.Success) {
            $value = $m.Groups[1].Value.Trim()
            # bare value may embed a quoted session id (ledger style: owner=Name `<id>`);
            # the quoted id is the canonical comparable owner (pre-442 gate contract).
            if ($bareCaptured) {
                $q = [regex]::Match($value, '`([^`]+)`')
                if ($q.Success) { $value = $q.Groups[1].Value.Trim() }
            }
            $result.Owner = $value
            if ([string]::IsNullOrWhiteSpace($ExpectedOwner)) {
                $result.Reasons += ('no expected owner passed: ledger latest event owner=[' + $result.Owner + '] (pass -LeaseOwner matching this value)')
            } elseif ($result.Owner -cne $ExpectedOwner.Trim()) {
                $result.Reasons += ('owner mismatch: ledger latest event owner=[' + $result.Owner + '], expected=[' + $ExpectedOwner.Trim() + ']')
            }
        } else {
            $result.Reasons += 'owner= field present but value not extractable (unterminated quote?)'
        }
    }
    $result.Pass = ($result.Reasons.Count -eq 0)
    return $result
}

function Assert-EntryLeaseAndStack {
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    & git -C $repo fetch origin main 2>&1 | Out-File (Join-Path $RunRoot 'lease_fetch.log') -Append -Encoding utf8
    $fetchExit = $LASTEXITCODE
    $lease = @(& git -C $repo show origin/main:coord/resources/PC-RUNTIME-STACK.md)
    $showExit = $LASTEXITCODE
    $ErrorActionPreference = $previousPreference
    if ($fetchExit -ne 0 -or $showExit -ne 0) { throw 'ENV-INTERRUPT: lease fetch/read failed' }
    $leaseHead = ($lease | Select-Object -First 4) -join "`n"
    $leaseHead | Out-File (Join-Path $RunRoot 'lease_head.log') -Append -Encoding utf8
    # STACKLEASE-OWNER-PARAM-1 (442): format-tolerant latest-event gate with
    # per-condition diagnosis (was: hardcoded owner/card against a head-4 window,
    # the 408 precheck false-reject shape). Verdict artifact: lease_verdict.json.
    $verdict = Get-StackLeaseVerdict -LeaseLines $lease -ExpectedOwner $LeaseOwner -ExpectedCard $LeaseCard
    $verdict | ConvertTo-Json -Depth 4 | Out-File (Join-Path $RunRoot 'lease_verdict.json') -Append -Encoding utf8
    if (-not $verdict.Pass) {
        throw ('ENV-INTERRUPT: stack lease gate rejected: ' + (($verdict.Reasons) -join ' | ') + ' [latest event: ' + $verdict.LatestLine + ']')
    }
    $processes = @(Get-CimInstance Win32_Process)
    $allowed = @{}
    if ($AllowedRootPid -gt 0) {
        $rootProcess = $processes | Where-Object ProcessId -eq $AllowedRootPid
        $expectedExe = [IO.Path]::GetFullPath((Join-Path $repo 'agent/bin/VitAgent.exe'))
        if ($null -eq $rootProcess -or "$($rootProcess.ExecutablePath)" -ne $expectedExe) { throw 'ENV-INTERRUPT: allowed agent PID does not match this worktree binary' }
        $allowed[$AllowedRootPid] = $true
        do {
            $changed = $false
            foreach ($process in $processes) {
                if ($allowed.ContainsKey([int]$process.ParentProcessId) -and -not $allowed.ContainsKey([int]$process.ProcessId)) { $allowed[[int]$process.ProcessId] = $true; $changed = $true }
            }
        } while ($changed)
    }
    $foreign = @($processes | Where-Object { $_.Name -match '^(VitShell|VitAgent|VitApp|VspHub|Godot.*)\.exe$' -and -not $allowed.ContainsKey([int]$_.ProcessId) })
    if ($foreign.Count) { throw ('ENV-INTERRUPT: foreign Vit stack found, refusing reuse/kill: ' + (($foreign | ForEach-Object { "$($_.Name):$($_.ProcessId)" }) -join ',')) }
    $listeners = @(Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object { $_.LocalPort -in @(7878,7879,8787,5555,5556,9333) })
    foreach ($listener in $listeners) { if (-not $allowed.ContainsKey([int]$listener.OwningProcess)) { throw "ENV-INTERRUPT: TCP $($listener.LocalPort) already listening pid=$($listener.OwningProcess)" } }
    foreach ($udp in @(Get-NetUDPEndpoint -LocalPort 4445 -ErrorAction SilentlyContinue)) { if (-not $allowed.ContainsKey([int]$udp.OwningProcess)) { throw "ENV-INTERRUPT: UDP4445 occupied pid=$($udp.OwningProcess)" } }
    if ($InjectedBoundAgentPort -gt 0) {
        $bound = @(Get-NetTCPConnection -LocalPort $InjectedBoundAgentPort -ErrorAction SilentlyContinue | Where-Object { $_.State -eq 'Bound' })
        if ($InjectionOwnerPid -le 0 -or $bound.Count -eq 0 -or @($bound | Where-Object { [int]$_.OwningProcess -ne $InjectionOwnerPid }).Count) { throw 'ENV-INTERRUPT: F2 socket is not the known script-owned bound injection' }
    }
    $snapshot = @{ at=(Get-Date).ToString('o'); allowed_root_pid=$AllowedRootPid; listeners=@($listeners | Select-Object LocalPort,OwningProcess); foreign_count=$foreign.Count }
    $snapshot | ConvertTo-Json -Depth 4 | Out-File (Join-Path $RunRoot 'immediate_guard.jsonl') -Append -Encoding utf8
}

if ($Leg -eq 'Guard') {
    New-Item -ItemType Directory -Path $RunRoot -Force | Out-Null
    Assert-EntryLeaseAndStack
    return
}
if (Test-Path -LiteralPath $RunRoot) { throw 'RunRoot already exists; use a new run directory' }
New-Item -ItemType Directory -Path $RunRoot | Out-Null
$snapshots = @()
$paths = @((Join-Path $env:APPDATA 'Vit-DAW/global_project_config.json'))
foreach ($base in @($repo, 'D:/Vit_DAW')) { foreach ($relative in @('VitApp/Workspace/Settings/Settings.xml', 'VitApp/Workspace/default_project.xml')) { $paths += Join-Path $base $relative } }
foreach ($path in $paths) {
    $exists = Test-Path -LiteralPath $path
    $backup = Join-Path $RunRoot ("before_$($snapshots.Count).bin")
    $hash = 'ABSENT'
    if ($exists) { Copy-Item -LiteralPath $path -Destination $backup; $hash = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash }
    $snapshots += @{ path=$path; existed=$exists; backup=$backup; before_hash=$hash }
}
$fixtureHash = (Get-FileHash -LiteralPath $FixtureSource -Algorithm SHA256).Hash
$snapshots | ConvertTo-Json -Depth 4 | Set-Content (Join-Path $RunRoot 'before_manifest.json') -Encoding utf8
$innerExit = -1
$wrapperError = ''
$restoreErrors = @()
try {
    Assert-EntryLeaseAndStack
    $env:VIT_AGENT_LLM_BASE_URL = 'http://127.0.0.1:1'
    $device = Join-Path $RunRoot 'device'
    # Native stderr is evidence, not a PowerShell terminating NativeCommandError.
    $ErrorActionPreference = 'Continue'
    if ($Leg -eq 'A') {
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'vitshell_full_smoke.ps1') -FrontendWorktree $FrontendWorktree -FixtureSource $FixtureSource -CardId CONVERGE-ENTRY-1 -Rounds 1 -EntryManagementChecks -RunRoot $device *> (Join-Path $RunRoot 'stdout.log')
    } else {
        & powershell -NoProfile -ExecutionPolicy Bypass -File (Join-Path $PSScriptRoot 'ui_entry2_smoke.ps1') -RepoRoot $repo -GodotProjectRoot $FrontendWorktree -ProjectSource $FixtureSource -KernelExe (Join-Path $repo 'VitApp/build/VitApp_artefacts/Release/VitApp.exe') -EntryManagementChecks -RunRoot $device *> (Join-Path $RunRoot 'stdout.log')
    }
    $innerExit = $LASTEXITCODE
    $ErrorActionPreference = 'Stop'
} catch { $wrapperError = $_.Exception.Message }
finally {
    $ErrorActionPreference = 'Stop'
    Remove-Item Env:VIT_AGENT_LLM_BASE_URL -ErrorAction SilentlyContinue
    foreach ($snapshot in $snapshots) {
        try {
            if ($snapshot.existed) { Copy-Item -LiteralPath $snapshot.backup -Destination $snapshot.path -Force }
            elseif (Test-Path -LiteralPath $snapshot.path) { Remove-Item -LiteralPath $snapshot.path -Force }
            $after = if (Test-Path -LiteralPath $snapshot.path) { (Get-FileHash -LiteralPath $snapshot.path -Algorithm SHA256).Hash } else { 'ABSENT' }
            $snapshot.after_hash = $after
            if ($after -ne $snapshot.before_hash) { throw "restore mismatch $($snapshot.path)" }
        } catch { $restoreErrors += $_.Exception.Message }
    }
    $fixtureAfter = (Get-FileHash -LiteralPath $FixtureSource -Algorithm SHA256).Hash
    if ($fixtureAfter -ne $fixtureHash) { $restoreErrors += 'fixture source changed; not restored or hidden' }
    @{ leg=$Leg; inner_exit=$innerExit; wrapper_error=$wrapperError; restore_errors=$restoreErrors; snapshots=$snapshots; fixture_before=$fixtureHash; fixture_after=$fixtureAfter } | ConvertTo-Json -Depth 6 | Set-Content (Join-Path $RunRoot 'hygiene.json') -Encoding utf8
}
if ($innerExit -ne 0 -or $wrapperError -or $restoreErrors.Count) { Write-Error "FAIL: inner=$innerExit wrapper=$wrapperError restore=$($restoreErrors -join ';')"; exit 1 }
Write-Output "PASS: $Leg inner exit0, outer hygiene verified"
exit 0

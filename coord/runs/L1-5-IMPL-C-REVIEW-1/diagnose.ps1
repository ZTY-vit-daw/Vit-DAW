$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../../..')).Path
Set-Location (Join-Path $repo 'agent')
$cases = @(
    @{ Name = 'chat-isolated'; Args = @('test', './internal/chat', '-run', '^TestAutomaticProposalTextConfirmationKeepsTaskIdentityAndPendingProjection$', '-count=1', '-v') },
    @{ Name = 'chat-package'; Args = @('test', './internal/chat', '-count=1') }
)
$results = @()
foreach ($case in $cases) {
    $start = Get-Date
    $arguments = $case.Args
    & go @arguments *> (Join-Path $PSScriptRoot ($case.Name + '.log'))
    $code = $LASTEXITCODE
    $results += [ordered]@{ command = ('go ' + ($arguments -join ' ')); exit_code = $code; started = $start.ToString('o'); finished = (Get-Date).ToString('o') }
    $results | ConvertTo-Json -Depth 4 | Set-Content -Encoding utf8 (Join-Path $PSScriptRoot 'diagnostic-commands.json')
    Write-Output ($case.Name + ' exit=' + $code)
}
if (($results | Where-Object { $_.exit_code -ne 0 }).Count -gt 0) { exit 1 }
exit 0

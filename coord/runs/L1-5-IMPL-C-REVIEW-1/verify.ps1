$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '../../..')).Path
Set-Location (Join-Path $repo 'agent')
$cases = @(
    @{ Name = 'build'; Args = @('build', './...') },
    @{ Name = 'fastpath'; Args = @('test', './internal/fastpath', '-count=1', '-v') },
    @{ Name = 'agentloop'; Args = @('test', './internal/agentloop', '-count=1') },
    @{ Name = 'full'; Args = @('test', './...', '-count=1') }
)
$results = @()
foreach ($case in $cases) {
    $start = Get-Date
    $arguments = $case.Args
    & go @arguments *> (Join-Path $PSScriptRoot ($case.Name + '.log'))
    $code = $LASTEXITCODE
    $results += [ordered]@{ command = ('go ' + ($arguments -join ' ')); exit_code = $code; started = $start.ToString('o'); finished = (Get-Date).ToString('o') }
    $results | ConvertTo-Json -Depth 4 | Set-Content -Encoding utf8 (Join-Path $PSScriptRoot 'commands.json')
    Write-Output ($case.Name + ' exit=' + $code)
    if ($code -ne 0) { exit $code }
}
exit 0

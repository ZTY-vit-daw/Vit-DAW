param([string]$OutPath, [int]$HoldSeconds = 10)
$rows = Get-ChildItem env: | ForEach-Object { "$($_.Name)=$($_.Value)" }
$rows | Set-Content -LiteralPath $OutPath -Encoding UTF8
Start-Sleep -Seconds $HoldSeconds

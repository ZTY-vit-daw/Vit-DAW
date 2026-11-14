$since = (Get-Date).AddMinutes(-45)
Write-Host "--- Application log: VitShell/WER mentions ---"
Get-WinEvent -FilterHashtable @{ LogName = "Application"; StartTime = $since } -ErrorAction SilentlyContinue |
    Where-Object { $_.Message -match "VitShell" } |
    Select-Object -First 10 TimeCreated, Id, ProviderName, LevelDisplayName |
    Format-Table -AutoSize | Out-String -Width 200 | Write-Host
Write-Host "--- Application log: .NET/Crash errors (any) ---"
Get-WinEvent -FilterHashtable @{ LogName = "Application"; Level = 2; StartTime = $since } -MaxEvents 15 -ErrorAction SilentlyContinue |
    Select-Object TimeCreated, ProviderName |
    Format-Table -AutoSize | Out-String -Width 200 | Write-Host

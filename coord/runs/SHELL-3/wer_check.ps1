$since = (Get-Date).AddMinutes(-20)
Write-Host "--- WER events (20min) mentioning VitShell ---"
Get-WinEvent -FilterHashtable @{ LogName = "Application"; StartTime = $since } -ErrorAction SilentlyContinue |
    Where-Object { $_.Message -match "VitShell" } |
    Select-Object -First 6 TimeCreated, Id, ProviderName |
    Format-Table -AutoSize | Out-String -Width 180 | Write-Host
Write-Host "--- CrashDumps ---"
Get-ChildItem "$env:LOCALAPPDATA\CrashDumps" -ErrorAction SilentlyContinue | Where-Object { $_.LastWriteTime -gt $since } | Select-Object Name, LastWriteTime | Format-Table | Out-String | Write-Host
Write-Host "--- WER ReportArchive recent ---"
Get-ChildItem "$env:PROGRAMDATA\Microsoft\Windows\WER\ReportArchive" -Directory -ErrorAction SilentlyContinue | Where-Object { $_.Name -match "VitShell" -and $_.LastWriteTime -gt $since } | Select-Object Name, LastWriteTime | Format-Table | Out-String | Write-Host

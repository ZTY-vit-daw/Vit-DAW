$errs = $null
$null = [System.Management.Automation.PSParser]::Tokenize((Get-Content "D:/Vit_DAW_wt_shell3/scripts/vitshell_full_smoke.ps1" -Raw), [ref]$errs)
Write-Host ("ERRCOUNT=" + $errs.Count)

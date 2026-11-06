# diag_270.ps1 -- diagnose ArgumentException on manifest stub parse (leg2)
param([string]$Stub)
$ErrorActionPreference = "Continue"
Write-Host "--- file bytes head ---"
$b = [System.IO.File]::ReadAllBytes($Stub)
$head = @(); for ($i = 0; $i -lt [Math]::Min(70, $b.Length); $i++) { $head += $b[$i].ToString("X2") }
Write-Host ($head -join " ")
Write-Host "--- A: Get-Content -Encoding UTF8 | ConvertFrom-Json ---"
try {
    $m = Get-Content -LiteralPath $Stub -Raw -Encoding UTF8 | ConvertFrom-Json
    $p = ""
    if ($null -ne $m -and $null -ne @($m.cases)[0]) { $p = [string](@($m.cases))[0].project_path }
    Write-Host ("OK cases=" + @($m.cases).Count + " path=[" + $p + "]")
} catch {
    Write-Host ("THREW: " + $_.Exception.GetType().FullName + " :: " + $_.Exception.Message)
    Write-Host ($_.Exception.StackTrace)
    if ($_.Exception.InnerException) { Write-Host ("INNER: " + $_.Exception.InnerException.ToString()) }
}
Write-Host "--- B: .NET UTF8 text | ConvertFrom-Json ---"
try {
    $u8 = New-Object System.Text.UTF8Encoding($false)
    $t = $u8.GetString([System.IO.File]::ReadAllBytes($Stub))
    Write-Host ("text=[" + $t + "]")
    $m2 = $t | ConvertFrom-Json
    $p2 = ""
    if ($null -ne $m2 -and $null -ne @($m2.cases)[0]) { $p2 = [string](@($m2.cases))[0].project_path }
    Write-Host ("OK2 path=[" + $p2 + "]")
} catch {
    Write-Host ("THREW2: " + $_.Exception.GetType().FullName + " :: " + $_.Exception.Message)
    if ($_.Exception.InnerException) { Write-Host ("INNER2: " + $_.Exception.InnerException.ToString()) }
}
Write-Host "--- C: Invoke-Expression the post line verbatim ---"
$postLine = '$lwManifest = Get-Content -LiteralPath $PublicManifest -Raw -Encoding UTF8 | ConvertFrom-Json'
$PublicManifest = $Stub
$lwManifest = $null
try {
    Invoke-Expression $postLine
    $p3 = ""
    if ($null -ne $lwManifest -and $null -ne @($lwManifest.cases)[0]) { $p3 = [string](@($lwManifest.cases))[0].project_path }
    Write-Host ("OK3 path=[" + $p3 + "]")
} catch {
    Write-Host ("THREW3: " + $_.Exception.GetType().FullName + " :: " + $_.Exception.Message)
    Write-Host ($_.Exception.StackTrace)
    if ($_.Exception.InnerException) { Write-Host ("INNER3: " + $_.Exception.InnerException.ToString()) }
}
Write-Host ("--- PSVersion=" + $PSVersionTable.PSVersion.ToString() + " Culture=" + (Get-Culture).Name)

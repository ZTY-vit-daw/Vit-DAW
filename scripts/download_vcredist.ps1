#requires -Version 5.1
# Downloads MSVC 2015-2022 x64 redist for Inno Setup (silent install during setup).
# Run once, then compile Vit_DAW_setup.iss — the installer will bundle and run it.
param(
    [string]$OutDir = ""
)
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($OutDir)) {
    $repo = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
    $OutDir = Join-Path $repo "Export\installer\redist"
}
$outFile = Join-Path $OutDir "vc_redist.x64.exe"
New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
$url = "https://aka.ms/vs/17/release/vc_redist.x64.exe"
Write-Host "Downloading $url ..."
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest -Uri $url -OutFile $outFile -UseBasicParsing
Write-Host "Saved: $outFile"
Get-Item -LiteralPath $outFile | Select-Object FullName, Length

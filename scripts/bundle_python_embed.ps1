#requires -Version 5.1
<#
    Bundle Windows embeddable Python + pyzmq into a release folder (no system Python required).

    Run once before zipping / building the Inno installer:
      powershell -ExecutionPolicy Bypass -File .\scripts\bundle_python_embed.ps1

    Default target: D:\Vit_DAW\Export\Vit_DAW_V0.1\python_embed
#>
[CmdletBinding()]
param(
    [string]$TargetDir = "",
    [string]$PythonVersion = "3.12.8",
    [switch]$Force
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($TargetDir)) {
    $repoRoot = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
    $TargetDir = Join-Path $repoRoot "Export\Vit_DAW_V0.1\python_embed"
}

$embedName = "python-$PythonVersion-embed-amd64.zip"
$embedUrl = "https://www.python.org/ftp/python/$PythonVersion/$embedName"
$getPipUrl = "https://bootstrap.pypa.io/get-pip.py"

function Test-ZmqOk {
    param([string]$PyExe)
    & $PyExe -c "import zmq; print('ok')" 2>$null
    return ($LASTEXITCODE -eq 0)
}

if ((Test-Path -LiteralPath (Join-Path $TargetDir "python.exe")) -and -not $Force) {
    if (Test-ZmqOk (Join-Path $TargetDir "python.exe")) {
        Write-Host "Already bundled with pyzmq: $TargetDir"
        exit 0
    }
}

New-Item -ItemType Directory -Path $TargetDir -Force | Out-Null
$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("vit_py_embed_" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempRoot -Force | Out-Null

try {
    $zipPath = Join-Path $tempRoot $embedName
    Write-Host "Downloading $embedUrl ..."
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri $embedUrl -OutFile $zipPath -UseBasicParsing

    Write-Host "Extracting embeddable Python to $TargetDir ..."
    Expand-Archive -LiteralPath $zipPath -DestinationPath $TargetDir -Force

    # Embed package uses python312._pth (extension is "._pth", not ".pth")
    $pth = Get-ChildItem -LiteralPath $TargetDir -File | Where-Object { $_.Name -like '*._pth' } | Select-Object -First 1
    if ($null -eq $pth) { throw "No *._pth file found in embed package." }
    $pthLines = @(Get-Content -LiteralPath $pth.FullName)
    $out = New-Object System.Collections.Generic.List[string]
    $hasSite = $false
    foreach ($line in $pthLines) {
        if ($line -match '^\s*import\s+site\s*$') {
            $out.Add("import site")
            $hasSite = $true
        }
        elseif ($line -match '^\s*#\s*import\s+site\s*$') {
            $out.Add("import site")
            $hasSite = $true
        }
        else {
            $out.Add($line)
        }
    }
    if (-not $hasSite) {
        $out.Add("import site")
    }
    Set-Content -LiteralPath $pth.FullName -Value $out.ToArray() -Encoding ASCII

    $getPip = Join-Path $tempRoot "get-pip.py"
    Write-Host "Downloading get-pip.py ..."
    Invoke-WebRequest -Uri $getPipUrl -OutFile $getPip -UseBasicParsing

    $py = Join-Path $TargetDir "python.exe"
    Write-Host "Installing pip ..."
    & $py $getPip --no-warn-script-location
    if ($LASTEXITCODE -ne 0) { throw "get-pip failed with exit code $LASTEXITCODE" }

    Write-Host "Installing pyzmq (this may take a minute) ..."
    & $py -m pip install --no-warn-script-location --disable-pip-version-check pyzmq
    if ($LASTEXITCODE -ne 0) { throw "pip install pyzmq failed with exit code $LASTEXITCODE" }

    if (-not (Test-ZmqOk $py)) { throw "pyzmq import check failed after install." }

    Write-Host "Done. Bundled Python: $TargetDir"
}
finally {
    Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}

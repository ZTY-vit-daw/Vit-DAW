param(
    [int] $IntervalMs = 250,
    [string] $OutputPath = "",
    [string[]] $ProcessName = @(
        "Godot_v4.6.1-stable_win64",
        "Godot_v4.6.1-stable_win64_console",
        "Vit DAW v0.7",
        "VitApp",
        "python",
        "pythonw"
    )
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($OutputPath)) {
    $stamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $OutputPath = Join-Path $env:TEMP "vit_import_perf_$stamp.csv"
}

$parent = Split-Path -Parent $OutputPath
if (-not [string]::IsNullOrWhiteSpace($parent) -and -not (Test-Path -LiteralPath $parent)) {
    New-Item -ItemType Directory -Path $parent -Force | Out-Null
}

$logicalCores = [Math]::Max(1, [Environment]::ProcessorCount)
$previous = @{}

function Convert-ToCsvCell([object] $Value) {
    if ($null -eq $Value) {
        return '""'
    }
    $text = [string] $Value
    return '"' + $text.Replace('"', '""') + '"'
}

function Get-OptionalDouble([object] $Object, [string] $Name) {
    try {
        $prop = $Object.PSObject.Properties[$Name]
        if ($null -ne $prop -and $null -ne $prop.Value) {
            return [double] $prop.Value
        }
    } catch {
    }
    return -1.0
}

function Test-ProcessNameMatch([string] $Name, [string[]] $Wanted) {
    foreach ($item in $Wanted) {
        if ($Name -eq $item) {
            return $true
        }
        if ($Name -like "*$item*") {
            return $true
        }
    }
    return $false
}

$writer = [System.IO.StreamWriter]::new($OutputPath, $false, [System.Text.UTF8Encoding]::new($false))
$writer.WriteLine("timestamp,process_name,pid,cpu_percent,working_set_mb,private_mb,virtual_mb,threads,handles,io_read_mb,io_write_mb")
$writer.Flush()

Write-Host "Vit import perf sampler writing: $OutputPath"
Write-Host "Press Ctrl+C after reproducing the lag."

try {
    while ($true) {
        $now = Get-Date
        $processes = Get-Process | Where-Object { Test-ProcessNameMatch $_.ProcessName $ProcessName }
        foreach ($p in $processes) {
            $pid = [int] $p.Id
            $cpuMs = [double] $p.TotalProcessorTime.TotalMilliseconds
            $cpuPercent = 0.0
            if ($previous.ContainsKey($pid)) {
                $last = $previous[$pid]
                $elapsedMs = [Math]::Max(1.0, ($now - $last.Time).TotalMilliseconds)
                $cpuDeltaMs = [Math]::Max(0.0, $cpuMs - [double] $last.CpuMs)
                $cpuPercent = ($cpuDeltaMs / $elapsedMs) * 100.0 / $logicalCores
            }
            $previous[$pid] = [pscustomobject] @{
                Time = $now
                CpuMs = $cpuMs
            }

            $workingSetMb = [Math]::Round(([double] $p.WorkingSet64) / 1MB, 2)
            $privateMb = [Math]::Round(([double] $p.PrivateMemorySize64) / 1MB, 2)
            $virtualMb = [Math]::Round(([double] $p.VirtualMemorySize64) / 1MB, 2)
            $ioRead = Get-OptionalDouble $p "IOReadBytes"
            $ioWrite = Get-OptionalDouble $p "IOWriteBytes"
            $ioReadMb = if ($ioRead -ge 0.0) { [Math]::Round($ioRead / 1MB, 2) } else { -1 }
            $ioWriteMb = if ($ioWrite -ge 0.0) { [Math]::Round($ioWrite / 1MB, 2) } else { -1 }

            $cells = @(
                (Convert-ToCsvCell $now.ToString("o")),
                (Convert-ToCsvCell $p.ProcessName),
                (Convert-ToCsvCell $pid),
                (Convert-ToCsvCell ([Math]::Round($cpuPercent, 2))),
                (Convert-ToCsvCell $workingSetMb),
                (Convert-ToCsvCell $privateMb),
                (Convert-ToCsvCell $virtualMb),
                (Convert-ToCsvCell $p.Threads.Count),
                (Convert-ToCsvCell $p.HandleCount),
                (Convert-ToCsvCell $ioReadMb),
                (Convert-ToCsvCell $ioWriteMb)
            )
            $writer.WriteLine(($cells -join ","))
        }
        $writer.Flush()
        Start-Sleep -Milliseconds ([Math]::Max(50, $IntervalMs))
    }
} finally {
    $writer.Flush()
    $writer.Close()
}

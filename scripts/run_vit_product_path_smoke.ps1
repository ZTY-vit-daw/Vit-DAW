[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW",
    [string]$GodotExe = "",
    [string]$GodotProjectRoot = "",
    [string]$UIExe = "",
    [string]$KernelExe = "",
    [string]$AgentExe = "",
    [string]$VspHubExe = "",
    [switch]$ReuseGodot,
    [switch]$ReuseUI,
    [switch]$ReuseAgent,
    [switch]$ReuseHub,
    [switch]$ReuseKernel,
    [switch]$SkipBuild,
    [switch]$KeepProcesses,
    [switch]$ClipFadeGainAgentOnly,
    [switch]$B2StaticBalanceAgentOnly,
    [string]$B2StemsFolder = "",
    [switch]$B3PanLayoutAgentOnly,
    [string]$B3StemsFolder = "",
    [switch]$B4LowEndRelationAgentOnly,
    [int]$TimeoutSeconds = 60
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$AgentHttp = "http://127.0.0.1:7878"
$AgentHttpAddr = "127.0.0.1:7878"
$AgentHttpPort = 7878
$VspHubHttp = "http://127.0.0.1:8787"
$VspHubPort = 8787
$ZmqReqPort = 5555
$ZmqSubPort = 5556

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host ("== " + $Message) -ForegroundColor Cyan
}

function Write-Ok {
    param([string]$Message)
    Write-Host ("ok: " + $Message) -ForegroundColor Green
}

function Write-WarnLine {
    param([string]$Message)
    Write-Host ("warn: " + $Message) -ForegroundColor Yellow
}

function Fail {
    param([string]$Message)
    throw $Message
}

function Join-UnicodeChars {
    param([int[]]$CodePoints)
    $builder = New-Object System.Text.StringBuilder
    foreach ($codePoint in $CodePoints) {
        [void]$builder.Append([char]$codePoint)
    }
    return $builder.ToString()
}

function Get-OptionalProperty {
    param(
        [object]$Object,
        [string]$Name
    )
    if ($null -eq $Object -or [string]::IsNullOrWhiteSpace($Name)) {
        return $null
    }
    if ($Object -is [System.Collections.IDictionary] -and $Object.Contains($Name)) {
        return $Object[$Name]
    }
    $prop = $Object.PSObject.Properties[$Name]
    if ($null -eq $prop) {
        return $null
    }
    return $prop.Value
}

function ConvertTo-JsonFile {
    param(
        [object]$Value,
        [string]$Path,
        [int]$Depth = 24
    )
    $json = $Value | ConvertTo-Json -Depth $Depth
    Set-Content -LiteralPath $Path -Value $json -Encoding UTF8
}

function Get-TcpListener {
    param([int]$Port)
    foreach ($line in @(& netstat -ano -p tcp 2>$null)) {
        $text = ([string]$line).Trim()
        if (-not $text.StartsWith("TCP", [System.StringComparison]::OrdinalIgnoreCase)) {
            continue
        }
        $parts = @($text -split "\s+" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
        if ($parts.Count -lt 5) {
            continue
        }
        $local = [string]$parts[1]
        $state = [string]$parts[3]
        if ($state -ine "LISTENING") {
            continue
        }
        $colon = $local.LastIndexOf(":")
        if ($colon -lt 0 -or $colon -ge ($local.Length - 1)) {
            continue
        }
        $portText = $local.Substring($colon + 1)
        $parsedPort = 0
        if (-not [int]::TryParse($portText, [ref]$parsedPort)) {
            continue
        }
        if ($parsedPort -eq $Port) {
            $address = $local.Substring(0, $colon).Trim("[", "]")
            return [pscustomobject]@{
                LocalAddress = $address
                LocalPort = $parsedPort
                OwningProcess = [int]$parts[4]
            }
        }
    }
    return $null
}

function Wait-TcpListener {
    param(
        [int]$Port,
        [int]$TimeoutSeconds
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $listener = Get-TcpListener -Port $Port
        if ($null -ne $listener) {
            return $listener
        }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    return $null
}

function Wait-HttpReady {
    param(
        [string]$BaseUrl,
        [int]$TimeoutSeconds
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $resp = Invoke-WebRequest -UseBasicParsing -Uri ($BaseUrl.TrimEnd("/") + "/health") -TimeoutSec 2
            if ($resp.StatusCode -ge 200 -and $resp.StatusCode -lt 500) {
                return $true
            }
        }
        catch {
            Start-Sleep -Milliseconds 500
        }
    } while ((Get-Date) -lt $deadline)
    return $false
}

function Invoke-Json {
    param(
        [ValidateSet("GET", "POST")]
        [string]$Method,
        [string]$Uri,
        [object]$Body = $null,
        [int]$TimeoutSec = 30
    )
    if ($Method -eq "GET") {
        $resp = Invoke-WebRequest -UseBasicParsing -Method GET -Uri $Uri -TimeoutSec $TimeoutSec
    }
    else {
        $json = $Body | ConvertTo-Json -Depth 24 -Compress
        $bytes = [System.Text.Encoding]::UTF8.GetBytes($json)
        $resp = Invoke-WebRequest -UseBasicParsing -Method POST -Uri $Uri -Body $bytes -ContentType "application/json; charset=utf-8" -TimeoutSec $TimeoutSec
    }
    if ([string]::IsNullOrWhiteSpace($resp.Content)) {
        return $null
    }
    return $resp.Content | ConvertFrom-Json
}

function Resolve-FirstExistingPath {
    param(
        [string[]]$Candidates,
        [string]$Label
    )
    foreach ($candidate in $Candidates) {
        if (-not [string]::IsNullOrWhiteSpace($candidate) -and (Test-Path -LiteralPath $candidate)) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }
    Fail ("Could not find " + $Label + ". Tried: " + ($Candidates -join "; "))
}

function Get-ProcessInfo {
    param([int]$ProcessID)
    $proc = Get-Process -Id $ProcessID -ErrorAction SilentlyContinue
    if ($null -eq $proc) {
        return @{ pid = $ProcessID; found = $false }
    }
    return @{
        pid = $proc.Id
        name = $proc.ProcessName
        path = [string]$proc.Path
        start_time = if ($null -ne $proc.StartTime) { $proc.StartTime.ToString("o") } else { "" }
    }
}

function Get-ListenersSnapshot {
    $rows = @()
    foreach ($port in @($AgentHttpPort, $VspHubPort, $ZmqReqPort, $ZmqSubPort)) {
        $listener = Get-TcpListener -Port $port
        if ($null -eq $listener) {
            $rows += [pscustomobject]@{
                port = $port
                listening = $false
                pid = $null
                process = $null
                path = $null
            }
        }
        else {
            $proc = Get-Process -Id $listener.OwningProcess -ErrorAction SilentlyContinue
            $rows += [pscustomobject]@{
                port = $port
                listening = $true
                pid = $listener.OwningProcess
                process = if ($null -ne $proc) { $proc.ProcessName } else { $null }
                path = if ($null -ne $proc) { [string]$proc.Path } else { $null }
            }
        }
    }
    return $rows
}

function Find-ProcessByPath {
    param([string]$Path)
    $full = [System.IO.Path]::GetFullPath($Path)
    return Get-Process -ErrorAction SilentlyContinue | Where-Object {
        try {
            -not [string]::IsNullOrWhiteSpace($_.Path) -and
                [System.IO.Path]::GetFullPath($_.Path).Equals($full, [System.StringComparison]::OrdinalIgnoreCase)
        }
        catch {
            $false
        }
    }
}

function Normalize-ComparablePath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    $normalized = $Path.Trim().Trim('"')
    try {
        $normalized = [System.IO.Path]::GetFullPath($normalized)
    }
    catch {
    }
    return $normalized.Replace("/", "\").TrimEnd("\").ToLowerInvariant()
}

function Get-CommandLinePathArg {
    param(
        [string]$CommandLine,
        [string]$Name
    )
    if ([string]::IsNullOrWhiteSpace($CommandLine) -or [string]::IsNullOrWhiteSpace($Name)) {
        return ""
    }
    $pattern = "(?i)(?:^|\s)" + [regex]::Escape($Name) + "\s+(""([^""]+)""|'([^']+)'|([^\s]+))"
    $match = [regex]::Match($CommandLine, $pattern)
    if (-not $match.Success) {
        return ""
    }
    foreach ($index in @(2, 3, 4)) {
        if ($match.Groups[$index].Success -and -not [string]::IsNullOrWhiteSpace($match.Groups[$index].Value)) {
            return $match.Groups[$index].Value
        }
    }
    return ""
}

function Find-GodotProjectProcesses {
    param(
        [string]$ProjectRoot,
        [bool]$RuntimeOnly = $false
    )
    $projectKey = Normalize-ComparablePath -Path $ProjectRoot
    $rows = @()
    $processes = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        $_.Name -like "Godot*.exe"
    }
    foreach ($proc in @($processes)) {
        $cmd = [string]$proc.CommandLine
        if ([string]::IsNullOrWhiteSpace($cmd)) {
            continue
        }
        $pathArg = Get-CommandLinePathArg -CommandLine $cmd -Name "--path"
        if ([string]::IsNullOrWhiteSpace($pathArg)) {
            continue
        }
        if ((Normalize-ComparablePath -Path $pathArg) -ne $projectKey) {
            continue
        }
        $isEditor = $cmd -match "(?i)(?:^|\s)--editor(?:\s|$)"
        if ($RuntimeOnly -and $isEditor) {
            continue
        }
        $rows += [pscustomobject]@{
            pid = [int]$proc.ProcessId
            parent_pid = [int]$proc.ParentProcessId
            name = [string]$proc.Name
            executable_path = [string]$proc.ExecutablePath
            command_line = $cmd
            project_path = $pathArg
            is_editor = [bool]$isEditor
        }
    }
    return $rows
}

function Group-GodotRuntimeProcesses {
    param([object[]]$Processes)
    $byPid = @{}
    foreach ($proc in @($Processes)) {
        $byPid[[int]$proc.pid] = $proc
    }
    $groups = @{}
    foreach ($proc in @($Processes)) {
        $root = $proc
        $seen = @{}
        while ($null -ne $root -and $byPid.ContainsKey([int]$root.parent_pid) -and -not $seen.ContainsKey([int]$root.pid)) {
            $seen[[int]$root.pid] = $true
            $root = $byPid[[int]$root.parent_pid]
        }
        if ($null -eq $root) {
            $root = $proc
        }
        $key = [string]([int]$root.pid)
        if (-not $groups.ContainsKey($key)) {
            $groups[$key] = [System.Collections.ArrayList]::new()
        }
        [void]$groups[$key].Add($proc)
    }
    $out = @()
    foreach ($key in @($groups.Keys)) {
        $members = @($groups[$key])
        $root = $members | Where-Object { [int]$_.pid -eq [int]$key } | Select-Object -First 1
        if ($null -eq $root) {
            $root = $members | Select-Object -First 1
        }
        $out += [pscustomobject]@{
            root_pid = [int]$root.pid
            processes = $members
        }
    }
    return $out
}

function Resolve-GodotProjectRoot {
    param([string]$RequestedProjectRoot)
    if (-not [string]::IsNullOrWhiteSpace($RequestedProjectRoot)) {
        $resolved = Resolve-Path -LiteralPath $RequestedProjectRoot -ErrorAction Stop
        return $resolved.Path
    }
    $godotProcesses = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        $_.Name -like "Godot*.exe" -and -not [string]::IsNullOrWhiteSpace($_.CommandLine)
    }
    foreach ($proc in @($godotProcesses)) {
        $pathArg = Get-CommandLinePathArg -CommandLine ([string]$proc.CommandLine) -Name "--path"
        if ([string]::IsNullOrWhiteSpace($pathArg)) {
            continue
        }
        $candidate = $pathArg.Replace("/", "\")
        if (Test-Path -LiteralPath (Join-Path $candidate "project.godot")) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }
    $fallback = "D:\Godot\project\vit-daw-frontend"
    if (Test-Path -LiteralPath (Join-Path $fallback "project.godot")) {
        return (Resolve-Path -LiteralPath $fallback).Path
    }
    Fail "Godot project root was not supplied and no running Godot --path project.godot was found"
}

function Resolve-GodotExecutable {
    param(
        [string]$RequestedGodotExe,
        [string]$ProjectRoot
    )
    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($RequestedGodotExe)) {
        $candidates += $RequestedGodotExe.Trim().Trim('"')
    }
    foreach ($proc in @(Find-GodotProjectProcesses -ProjectRoot $ProjectRoot -RuntimeOnly $false)) {
        if (-not [string]::IsNullOrWhiteSpace($proc.executable_path)) {
            $candidates += [string]$proc.executable_path
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($env:GODOT)) {
        $candidates += $env:GODOT.Trim().Trim('"')
    }
    $candidates += @(
        "D:\Godot\Godot_v4.6.1-stable_win64_console.exe",
        "D:\Godot\Godot_v4.6.1-stable_win64.exe",
        "${env:ProgramFiles}\Godot\Godot.exe",
        "${env:LocalAppData}\Programs\Godot\Godot.exe"
    )
    foreach ($candidate in $candidates) {
        if (-not [string]::IsNullOrWhiteSpace($candidate) -and (Test-Path -LiteralPath $candidate)) {
            return (Resolve-Path -LiteralPath $candidate).Path
        }
    }
    Fail ("Could not find Godot executable. Tried: " + ($candidates -join "; "))
}

function Resolve-GodotLaunchExecutable {
    param([string]$GodotExe)
    $resolved = (Resolve-Path -LiteralPath $GodotExe).Path
    if ($resolved -match "(?i)_console\.exe$") {
        return $resolved
    }
    $dir = Split-Path -Parent $resolved
    $name = [System.IO.Path]::GetFileName($resolved)
    $consoleName = $name -replace "\.exe$", "_console.exe"
    $consolePath = Join-Path $dir $consoleName
    if (Test-Path -LiteralPath $consolePath) {
        return (Resolve-Path -LiteralPath $consolePath).Path
    }
    $consoleSibling = Get-ChildItem -LiteralPath $dir -Filter "Godot*console*.exe" -ErrorAction SilentlyContinue |
        Select-Object -First 1
    if ($null -ne $consoleSibling) {
        return $consoleSibling.FullName
    }
    return $resolved
}

function Stop-ProcessByID {
    param([int]$ProcessID)
    if ($ProcessID -le 0) {
        return
    }
    $proc = Get-Process -Id $ProcessID -ErrorAction SilentlyContinue
    if ($null -ne $proc) {
        Stop-Process -Id $ProcessID -Force
    }
}

function Build-AgentAndHubIfNeeded {
    param(
        [string]$RepoRoot,
        [string]$AgentPath,
        [string]$VspHubPath,
        [bool]$SkipAgentBuild,
        [bool]$SkipHubBuild
    )
    if ($SkipAgentBuild) {
        if (-not (Test-Path -LiteralPath $AgentPath)) {
            Fail ("Missing agent exe while agent build is skipped: " + $AgentPath)
        }
    }
    if ($SkipHubBuild) {
        if (-not (Test-Path -LiteralPath $VspHubPath)) {
            Fail ("Missing VSP Hub exe while hub build is skipped: " + $VspHubPath)
        }
    }
    if ($SkipAgentBuild -and $SkipHubBuild) {
        return
    }
    $agentDir = Join-Path $RepoRoot "agent"
    $buildExe = Join-Path $agentDir "bin\VitAgent.product-smoke.exe"
    $hubBuildExe = Join-Path $agentDir "bin\VspHub.product-smoke.exe"
    New-Item -ItemType Directory -Path (Join-Path $agentDir "bin") -Force | Out-Null
    Push-Location $agentDir
    try {
        if (-not $SkipAgentBuild) {
            & go build -o $buildExe .\cmd\vitagent
            if ($LASTEXITCODE -ne 0) {
                Fail ("VitAgent go build failed with exit code " + $LASTEXITCODE)
            }
        }
        if (-not $SkipHubBuild) {
            & go build -o $hubBuildExe .\cmd\vsphub
            if ($LASTEXITCODE -ne 0) {
                Fail ("VspHub go build failed with exit code " + $LASTEXITCODE)
            }
        }
    }
    finally {
        Pop-Location
    }
    if (-not $SkipAgentBuild) {
        New-Item -ItemType Directory -Path (Split-Path -Parent $AgentPath) -Force | Out-Null
        Copy-Item -LiteralPath $buildExe -Destination $AgentPath -Force
    }
    if (-not $SkipHubBuild) {
        New-Item -ItemType Directory -Path (Split-Path -Parent $VspHubPath) -Force | Out-Null
        Copy-Item -LiteralPath $hubBuildExe -Destination $VspHubPath -Force
    }
}

function Stop-AgentIfNeededBeforeBuild {
    param([bool]$ReuseAgent)
    $listener = Get-TcpListener -Port $AgentHttpPort
    if ($null -eq $listener) {
        return
    }
    if ($ReuseAgent) {
        Write-WarnLine "reusing the running agent; skipping agent rebuild to avoid replacing an active executable"
        return
    }
    Write-WarnLine ("stopping existing agent before rebuild pid=" + $listener.OwningProcess)
    Stop-ProcessByID -ProcessID $listener.OwningProcess
    Start-Sleep -Milliseconds 500
}

function Stop-HubIfNeededBeforeBuild {
    param([bool]$ReuseHub)
    $listener = Get-TcpListener -Port $VspHubPort
    if ($null -eq $listener) {
        return
    }
    if ($ReuseHub) {
        Write-WarnLine "reusing the running VSP Hub; skipping hub rebuild to avoid replacing an active executable"
        return
    }
    Write-WarnLine ("stopping existing VSP Hub before rebuild pid=" + $listener.OwningProcess)
    Stop-ProcessByID -ProcessID $listener.OwningProcess
    Start-Sleep -Milliseconds 500
}

function Start-Or-Reuse-Agent {
    param(
        [string]$RepoRoot,
        [string]$AgentPath,
        [string]$AgentLog,
        [bool]$ReuseAgent,
        [int]$TimeoutSeconds
    )
    $listener = Get-TcpListener -Port $AgentHttpPort
    if ($null -ne $listener) {
        if ($ReuseAgent) {
            Write-Ok ("reusing agent HTTP pid=" + $listener.OwningProcess)
            return @{ pid = $listener.OwningProcess; started = $false; reused = $true }
        }
        Write-WarnLine ("stopping existing agent HTTP pid=" + $listener.OwningProcess)
        Stop-ProcessByID -ProcessID $listener.OwningProcess
        Start-Sleep -Milliseconds 500
    }
    $agentDir = Join-Path $RepoRoot "agent"
    $args = @(
        "-http", $AgentHttpAddr,
        "-last-log-path", $AgentLog,
        "-keep-last-log-lines", "1200"
    )
    $proc = Start-Process -FilePath $AgentPath -ArgumentList $args -WorkingDirectory $agentDir -WindowStyle Hidden -PassThru
    if (-not (Wait-HttpReady -BaseUrl $AgentHttp -TimeoutSeconds $TimeoutSeconds)) {
        Fail ("VitAgent HTTP did not become ready at " + $AgentHttp)
    }
    return @{ pid = $proc.Id; started = $true; reused = $false }
}

function Start-Or-Reuse-Kernel {
    param(
        [string]$KernelPath,
        [bool]$ReuseKernel,
        [int]$TimeoutSeconds
    )
    $desired = [System.IO.Path]::GetFullPath($KernelPath)
    $listener = Get-TcpListener -Port $ZmqReqPort
    if ($null -ne $listener) {
        $proc = Get-Process -Id $listener.OwningProcess -ErrorAction SilentlyContinue
        $runningPath = if ($null -ne $proc) { [string]$proc.Path } else { "" }
        if ($ReuseKernel) {
            Write-Ok ("reusing kernel command port pid=" + $listener.OwningProcess)
            return @{ pid = $listener.OwningProcess; started = $false; reused = $true }
        }
        if (-not [string]::IsNullOrWhiteSpace($runningPath) -and
            [System.IO.Path]::GetFullPath($runningPath).Equals($desired, [System.StringComparison]::OrdinalIgnoreCase)) {
            Write-Ok ("desired kernel already listening pid=" + $listener.OwningProcess)
            return @{ pid = $listener.OwningProcess; started = $false; reused = $true }
        }
        Write-WarnLine ("stopping existing kernel pid=" + $listener.OwningProcess + " path=" + $runningPath)
        Stop-ProcessByID -ProcessID $listener.OwningProcess
        Start-Sleep -Milliseconds 500
    }
    $proc = Start-Process -FilePath $KernelPath -WorkingDirectory (Split-Path -Parent $KernelPath) -WindowStyle Hidden -PassThru
    $ready = Wait-TcpListener -Port $ZmqReqPort -TimeoutSeconds $TimeoutSeconds
    if ($null -eq $ready) {
        Fail ("Kernel command port did not become ready: " + $ZmqReqPort)
    }
    [void](Wait-TcpListener -Port $ZmqSubPort -TimeoutSeconds 5)
    return @{ pid = $proc.Id; started = $true; reused = $false }
}

function Start-Or-Reuse-UI {
    param(
        [string]$UIPath,
        [bool]$ReuseUI
    )
    $existing = @(Find-ProcessByPath -Path $UIPath)
    if ($existing.Count -gt 0) {
        Write-Ok ("reusing product UI pid=" + $existing[0].Id)
        return @{ pid = $existing[0].Id; started = $false; reused = $true }
    }
    if ($ReuseUI) {
        Write-WarnLine ("-ReuseUI was set but no process matched " + $UIPath + "; starting it")
    }
    $proc = Start-Process -FilePath $UIPath -WorkingDirectory (Split-Path -Parent $UIPath) -PassThru
    Start-Sleep -Seconds 2
    return @{ pid = $proc.Id; started = $true; reused = $false }
}

function Stop-PortOwnerIfNeeded {
    param(
        [int]$Port,
        [string]$Label,
        [bool]$Reuse
    )
    $listener = Get-TcpListener -Port $Port
    if ($null -eq $listener) {
        return
    }
    if ($Reuse) {
        Write-WarnLine ("reusing existing " + $Label + " pid=" + $listener.OwningProcess + "; Godot autostart evidence may be external")
        return
    }
    Write-WarnLine ("stopping existing " + $Label + " pid=" + $listener.OwningProcess + " before Godot project launch")
    Stop-ProcessByID -ProcessID $listener.OwningProcess
    Start-Sleep -Milliseconds 500
}

function Read-LogText {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
        return ""
    }
    try {
        $stream = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
        try {
            $reader = [System.IO.StreamReader]::new($stream, [System.Text.Encoding]::UTF8, $true)
            try {
                return $reader.ReadToEnd()
            }
            finally {
                $reader.Dispose()
            }
        }
        finally {
            $stream.Dispose()
        }
    }
    catch {
        return ""
    }
}

function Get-GodotAutostartEvidence {
    param([string]$LogPath)
    $rows = @()
    $text = Read-LogText -Path $LogPath
    if ([string]::IsNullOrWhiteSpace($text)) {
        return $rows
    }
    foreach ($line in @($text -split "\r?\n")) {
        $match = [regex]::Match($line, "VitIpcClient:\s+registered autostart child\s+(\w+)\s+pid=(\d+)\s+path=(.+)$")
        if (-not $match.Success) {
            continue
        }
        $rows += [pscustomobject]@{
            role = $match.Groups[1].Value
            pid = [int]$match.Groups[2].Value
            path = $match.Groups[3].Value.Trim()
            line = $line
        }
    }
    return $rows
}

function Test-LogContains {
    param(
        [string[]]$LogPaths,
        [string]$Pattern
    )
    foreach ($logPath in $LogPaths) {
        $text = Read-LogText -Path $logPath
        if (-not [string]::IsNullOrWhiteSpace($text) -and $text.Contains($Pattern)) {
            return $true
        }
    }
    return $false
}

function Get-ProcessPathByID {
    param([int]$ProcessID)
    $proc = Get-Process -Id $ProcessID -ErrorAction SilentlyContinue
    if ($null -eq $proc) {
        return ""
    }
    return [string]$proc.Path
}

function Get-ExecutableEvidence {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path)) {
        return @{
            path = $Path
            exists = $false
        }
    }
    $item = Get-Item -LiteralPath $Path
    $hash = Get-FileHash -LiteralPath $Path -Algorithm SHA256
    return @{
        path = $item.FullName
        exists = $true
        length = [int64]$item.Length
        last_write_time_utc = $item.LastWriteTimeUtc.ToString("o")
        sha256 = $hash.Hash
    }
}

function Wait-GodotAutostartEvidence {
    param(
        [string]$LogPath,
        [int]$TimeoutSeconds
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $evidence = @(Get-GodotAutostartEvidence -LogPath $LogPath)
        $hasKernel = $false
        $hasHub = $false
        $hasAgent = $false
        foreach ($row in $evidence) {
            if ([string]$row.role -eq "kernel") {
                $hasKernel = $true
            }
            if ([string]$row.role -eq "hub") {
                $hasHub = $true
            }
            if ([string]$row.role -eq "agent") {
                $hasAgent = $true
            }
        }
        if ($hasKernel -and $hasHub -and $hasAgent) {
            return $evidence
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
    return @(Get-GodotAutostartEvidence -LogPath $LogPath)
}

function Wait-GodotLifecycleEvidence {
    param(
        [string]$StdoutLog,
        [string]$AlternateLog,
        [int]$TimeoutSeconds
    )
    $logPaths = @($StdoutLog, $AlternateLog)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $evidence = @()
        foreach ($logPath in $logPaths) {
            $evidence += @(Get-GodotAutostartEvidence -LogPath $logPath)
        }
        $hasKernel = $false
        $hasHub = $false
        $hasAgent = $false
        foreach ($row in $evidence) {
            if ([string]$row.role -eq "kernel") {
                $hasKernel = $true
            }
            if ([string]$row.role -eq "hub") {
                $hasHub = $true
            }
            if ([string]$row.role -eq "agent") {
                $hasAgent = $true
            }
        }
        $agentListener = Get-TcpListener -Port $AgentHttpPort
        $hubListener = Get-TcpListener -Port $VspHubPort
        $kernelReqListener = Get-TcpListener -Port $ZmqReqPort
        $kernelSubListener = Get-TcpListener -Port $ZmqSubPort
        $hubReadyLog = Test-LogContains -LogPaths $logPaths -Pattern "start_page: VSP Hub ready."
        $agentReadyLog = Test-LogContains -LogPaths $logPaths -Pattern "start_page: Agent v0.5 tools ready."
        $portsReady = ($null -ne $agentListener -and $null -ne $hubListener -and $null -ne $kernelReqListener -and $null -ne $kernelSubListener)
        if ($hasKernel -and $hasHub -and $hasAgent -and $portsReady -and $hubReadyLog -and $agentReadyLog) {
            return @{
                mode = "godot_autostart_log"
                autostart = $evidence
                agent_tools_ready_log = $true
                vsp_hub_ready_log = $hubReadyLog
            }
        }
        if ($portsReady -and $hubReadyLog -and $agentReadyLog) {
            return @{
                mode = "godot_runtime_self_check_ports"
                autostart = $evidence
                agent_tools_ready_log = $true
                vsp_hub_ready_log = $true
            }
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
    $finalEvidence = @()
    foreach ($logPath in $logPaths) {
        $finalEvidence += @(Get-GodotAutostartEvidence -LogPath $logPath)
    }
    return @{
        mode = "missing_godot_lifecycle_evidence"
        autostart = $finalEvidence
        agent_tools_ready_log = Test-LogContains -LogPaths $logPaths -Pattern "start_page: Agent v0.5 tools ready."
        vsp_hub_ready_log = Test-LogContains -LogPaths $logPaths -Pattern "start_page: VSP Hub ready."
    }
}

function Get-GodotAutostartChild {
    param(
        [object[]]$Evidence,
        [string]$Role
    )
    for ($i = $Evidence.Count - 1; $i -ge 0; $i--) {
        if ([string]$Evidence[$i].role -eq $Role) {
            return $Evidence[$i]
        }
    }
    return $null
}

function Assert-GodotOwnedPort {
    param(
        [int]$Port,
        [object]$Child,
        [string]$Label,
        [bool]$AllowExisting = $false,
        [string]$LifecycleMode = "",
        [string]$ExpectedPath = "",
        [bool]$CleanupProcess = $true
    )
    $listener = Get-TcpListener -Port $Port
    if ($null -eq $listener) {
        Fail ($Label + " port is not listening: " + $Port)
    }
    $ownerPath = Get-ProcessPathByID -ProcessID ([int]$listener.OwningProcess)
    if ($null -eq $Child) {
        if ($LifecycleMode -eq "godot_runtime_self_check_ports" -and
            -not [string]::IsNullOrWhiteSpace($ExpectedPath) -and
            (Normalize-ComparablePath -Path $ownerPath) -eq (Normalize-ComparablePath -Path $ExpectedPath)) {
            return @{
                pid = [int]$listener.OwningProcess
                started = $CleanupProcess
                reused = (-not $CleanupProcess)
                via = "godot_runtime_self_check_ports"
                path = $ownerPath
                expected_path = $ExpectedPath
            }
        }
        if ($AllowExisting) {
            return @{
                pid = [int]$listener.OwningProcess
                started = $false
                reused = $true
                via = "existing_port"
                path = $ownerPath
            }
        }
        Fail ("Missing Godot autostart evidence for " + $Label)
    }
    if ([int]$listener.OwningProcess -ne [int]$Child.pid) {
        if ($LifecycleMode -eq "godot_runtime_self_check_ports" -and
            -not [string]::IsNullOrWhiteSpace($ExpectedPath) -and
            (Normalize-ComparablePath -Path $ownerPath) -eq (Normalize-ComparablePath -Path $ExpectedPath)) {
            return @{
                pid = [int]$listener.OwningProcess
                started = $CleanupProcess
                reused = (-not $CleanupProcess)
                via = "godot_runtime_self_check_ports"
                path = $ownerPath
                expected_path = $ExpectedPath
                godot_autostart_pid = [int]$Child.pid
            }
        }
        if ($AllowExisting) {
            return @{
                pid = [int]$listener.OwningProcess
                started = $false
                reused = $true
                via = "existing_port_with_godot_evidence_mismatch"
                godot_autostart_pid = [int]$Child.pid
                path = $ownerPath
            }
        }
        Fail ($Label + " port owner pid=" + $listener.OwningProcess + " does not match Godot autostart pid=" + $Child.pid)
    }
    return @{
        pid = [int]$Child.pid
        started = $CleanupProcess
        reused = (-not $CleanupProcess)
        via = "godot_autostart"
        path = [string]$Child.path
    }
}

function Wait-VspHubAgentSession {
    param([int]$TimeoutSeconds)
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    $lastStatus = $null
    do {
        try {
            $status = Invoke-Json -Method GET -Uri ($VspHubHttp.TrimEnd("/") + "/vsp/status") -TimeoutSec 5
            $lastStatus = $status
            foreach ($session in @($status.sessions)) {
                if ([string]$session.role -eq "agent" -and [string]$session.client_id -eq "vit.agent.official") {
                    return $status
                }
            }
        }
        catch {
        }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    $detail = if ($null -ne $lastStatus) { $lastStatus | ConvertTo-Json -Depth 12 -Compress } else { "<no status>" }
    Fail ("VSP Hub did not report VitAgent session before timeout. last_status=" + $detail)
}

function Start-Or-Reuse-GodotProject {
    param(
        [string]$GodotLaunchExe,
        [string]$ProjectRoot,
        [string]$StdoutLog,
        [string]$StderrLog,
        [bool]$ReuseGodot,
        [int]$TimeoutSeconds
    )
    $runtimeProcesses = @(Find-GodotProjectProcesses -ProjectRoot $ProjectRoot -RuntimeOnly $true)
    if ($runtimeProcesses.Count -gt 0 -and $ReuseGodot) {
        Write-Ok ("reusing Godot project runtime pid=" + $runtimeProcesses[0].pid)
        return @{
            pid = [int]$runtimeProcesses[0].pid
            started = $false
            reused = $true
            exe = $runtimeProcesses[0].executable_path
            launch_exe = $GodotLaunchExe
            project_root = $ProjectRoot
            command_line = $runtimeProcesses[0].command_line
            stdout_log = $StdoutLog
            stderr_log = $StderrLog
        }
    }
    if ($ReuseGodot -and $runtimeProcesses.Count -eq 0) {
        Write-WarnLine "-ReuseGodot was set but no runtime process exists for this project; starting one"
    }
    $beforePids = @{}
    foreach ($proc in @(Find-GodotProjectProcesses -ProjectRoot $ProjectRoot -RuntimeOnly $true)) {
        $beforePids[[int]$proc.pid] = $true
    }
    $args = @("--path", $ProjectRoot, "--log-file", $StdoutLog)
    $proc = Start-Process -FilePath $GodotLaunchExe -ArgumentList $args -WorkingDirectory $ProjectRoot -RedirectStandardError $StderrLog -PassThru
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $current = @(Find-GodotProjectProcesses -ProjectRoot $ProjectRoot -RuntimeOnly $true | Where-Object {
                [int]$_.pid -eq [int]$proc.Id -or -not $beforePids.ContainsKey([int]$_.pid)
            })
        if ($current.Count -gt 0) {
            return @{
                pid = [int]$current[0].pid
                started = $true
                reused = $false
                exe = [string]$current[0].executable_path
                launch_exe = $GodotLaunchExe
                project_root = $ProjectRoot
                command_line = [string]$current[0].command_line
                log_capture = "godot_log_file"
                stdout_log = $StdoutLog
                stderr_log = $StderrLog
            }
        }
        if ($proc.HasExited) {
            $late = @(Find-GodotProjectProcesses -ProjectRoot $ProjectRoot -RuntimeOnly $true | Where-Object {
                    -not $beforePids.ContainsKey([int]$_.pid)
                })
            if ($late.Count -gt 0) {
                return @{
                    pid = [int]$late[0].pid
                    started = $true
                    reused = $false
                    exe = [string]$late[0].executable_path
                    launch_exe = $GodotLaunchExe
                    project_root = $ProjectRoot
                    command_line = [string]$late[0].command_line
                    log_capture = "godot_log_file"
                    stdout_log = $StdoutLog
                    stderr_log = $StderrLog
                }
            }
            Fail ("Godot project process exited early with code " + $proc.ExitCode)
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
    return @{
        pid = [int]$proc.Id
        started = $true
        reused = $false
        exe = $GodotLaunchExe
        launch_exe = $GodotLaunchExe
        project_root = $ProjectRoot
        command_line = ""
        stdout_log = $StdoutLog
        stderr_log = $StderrLog
    }
}

function Stop-GodotProjectRuntimeIfNeeded {
    param(
        [string]$ProjectRoot,
        [bool]$ReuseGodot
    )
    $runtimeProcesses = @(Find-GodotProjectProcesses -ProjectRoot $ProjectRoot -RuntimeOnly $true)
    if ($runtimeProcesses.Count -eq 0) {
        return
    }
    if ($ReuseGodot) {
        Write-WarnLine ("reusing existing Godot project runtime pid=" + $runtimeProcesses[0].pid)
        return
    }
    foreach ($group in @(Group-GodotRuntimeProcesses -Processes $runtimeProcesses)) {
        $pids = @($group.processes | ForEach-Object { [int]$_.pid } | Sort-Object -Unique)
        $childPids = @($pids | Where-Object { $_ -ne [int]$group.root_pid })
        $message = "stopping existing Godot project runtime pid=" + [string]$group.root_pid
        if ($childPids.Count -gt 0) {
            $message += " child_pids=" + ($childPids -join ",")
        }
        $message += " before smoke launch"
        Write-WarnLine $message
        foreach ($processID in $pids) {
            Stop-ProcessByID -ProcessID ([int]$processID)
        }
    }
    Start-Sleep -Milliseconds 500
}

function Invoke-AgentTool {
    param(
        [string]$Tool,
        [object]$ToolArgs = $null,
        [bool]$Confirmed = $true
    )
    if ($null -eq $ToolArgs) {
        $ToolArgs = @{}
    }
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/invoke") -Body @{
        tool = $Tool
        args = $ToolArgs
        confirmed = $Confirmed
        source = "vit_product_path_smoke"
    } -TimeoutSec 120
}

function Invoke-AgentChat {
    param(
        [string]$ConversationID,
        [string]$Message,
        [object]$ExtraContext = $null
    )
    $context = @{
        agent_mode = "chat"
        interaction_path = "agent_http_after_godot_project_lifecycle"
        product_path_smoke = $true
        product_lifecycle = "godot_project"
    }
    if ($null -ne $ExtraContext) {
        if ($ExtraContext -is [System.Collections.IDictionary]) {
            foreach ($key in $ExtraContext.Keys) {
                $context[[string]$key] = $ExtraContext[$key]
            }
        }
        else {
            foreach ($prop in $ExtraContext.PSObject.Properties) {
                $context[$prop.Name] = $prop.Value
            }
        }
    }
    return Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/chat") -Body @{
        conversation_id = $ConversationID
        message = $Message
        context = $context
    } -TimeoutSec 240
}

function Assert-StatusOk {
    param(
        [object]$Response,
        [string]$Label
    )
    $status = [string](Get-OptionalProperty -Object $Response -Name "status")
    if ($status -ne "ok") {
        $errorText = [string](Get-OptionalProperty -Object $Response -Name "error")
        Fail ($Label + " failed: status=" + $status + " error=" + $errorText)
    }
}

function Tool-Names {
    param([object]$Rows)
    $out = @()
    foreach ($row in @($Rows)) {
        if ($row -is [string]) {
            $out += $row
            continue
        }
        foreach ($key in @("tool", "command_name", "command")) {
            $name = [string](Get-OptionalProperty -Object $row -Name $key)
            if (-not [string]::IsNullOrWhiteSpace($name)) {
                $out += $name
            }
        }
    }
    return $out
}

function Compact-ToolResult {
    param([object]$Row)
    $result = Get-OptionalProperty -Object $Row -Name "result"
    $tool = [string](Get-OptionalProperty -Object $Row -Name "tool")
    $command = [string](Get-OptionalProperty -Object $Row -Name "command_name")
    $compact = [ordered]@{
        tool = $tool
        command_name = $command
        tool_status = [string](Get-OptionalProperty -Object $Row -Name "status")
        error = [string](Get-OptionalProperty -Object $Row -Name "error")
    }
    $resultStatus = Get-OptionalProperty -Object $result -Name "status"
    if ($null -ne $resultStatus -and -not [string]::IsNullOrWhiteSpace([string]$resultStatus)) {
        $compact["result_status"] = $resultStatus
    }
    foreach ($key in @("operation", "track_id", "track_name", "delta_db", "before_db", "after_db", "tick_id", "requires_confirmation", "requires_refresh", "observation_id")) {
        $value = Get-OptionalProperty -Object $result -Name $key
        if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) {
            $compact[$key] = $value
        }
    }
    $kernelReply = Get-OptionalProperty -Object $result -Name "kernel_reply"
    if ($null -ne $kernelReply) {
        $compact["kernel_reply"] = [ordered]@{}
        foreach ($key in @("status", "track_id", "track_name", "volume_db", "gain_db", "fader_db", "db")) {
            $value = Get-OptionalProperty -Object $kernelReply -Name $key
            if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) {
                $compact["kernel_reply"][$key] = $value
            }
        }
    }
    $resolvedTarget = Get-OptionalProperty -Object $result -Name "resolved_target"
    if ($null -ne $resolvedTarget) {
        $compact["resolved_target"] = [ordered]@{}
        foreach ($key in @("track_id", "track_name", "clip_id", "clip_name", "duration_seconds", "file_path", "source")) {
            $value = Get-OptionalProperty -Object $resolvedTarget -Name $key
            if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) {
                $compact["resolved_target"][$key] = $value
            }
        }
    }
    $acoustic = Get-OptionalProperty -Object $result -Name "acoustic_digest"
    if ($null -ne $acoustic) {
        $compact["acoustic_digest"] = [ordered]@{}
        foreach ($key in @("track_name", "clip_name", "peak_dbfs", "rms_dbfs", "headroom_db", "crest_db", "status")) {
            $value = Get-OptionalProperty -Object $acoustic -Name $key
            if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) {
                $compact["acoustic_digest"][$key] = $value
            }
        }
        $waveform = Get-OptionalProperty -Object $acoustic -Name "waveform"
        if ($null -ne $waveform) {
            foreach ($key in @("peak_dbfs", "rms_dbfs", "headroom_db", "crest_db", "status")) {
                $value = Get-OptionalProperty -Object $waveform -Name $key
                if ($null -ne $value -and -not [string]::IsNullOrWhiteSpace([string]$value)) {
                    $compact["acoustic_digest"][$key] = $value
                }
            }
        }
    }
    return $compact
}

function Compact-BeforeAfter {
	param([object]$Confirm)
	$rows = @(Get-OptionalProperty -Object $Confirm -Name "executed_kernel_reply")
    $compactRows = @()
    foreach ($row in $rows) {
        $compactRows += Compact-ToolResult -Row $row
    }
    return [ordered]@{
        reply = [string](Get-OptionalProperty -Object $Confirm -Name "reply")
        executed = $compactRows
        full_response_file = "chat_confirm.json"
	}
}

function Number-Value {
	param([object]$Value)
	if ($null -eq $Value) {
		return $null
	}
	$text = ([string]$Value).Trim()
	if ([string]::IsNullOrWhiteSpace($text) -or $text -eq "<nil>") {
		return $null
	}
	$dummy = 0.0
	if (-not [double]::TryParse($text, [System.Globalization.NumberStyles]::Float, [System.Globalization.CultureInfo]::InvariantCulture, [ref]$dummy)) {
		return $null
	}
	return $dummy
}

function Assert-NearNumber {
	param(
		[object]$Actual,
		[double]$Expected,
		[string]$Label,
		[double]$Tolerance = 0.001
	)
	$value = Number-Value -Value $Actual
	if ($null -eq $value -or [Math]::Abs(([double]$value) - $Expected) -gt $Tolerance) {
		Fail ($Label + " expected " + [string]$Expected + " got " + [string]$Actual)
	}
}

function Read-JsonFile {
	param([string]$Path)
	if (-not (Test-Path -LiteralPath $Path)) {
		return $null
	}
	$content = Get-Content -LiteralPath $Path -Raw -ErrorAction Stop
	if ([string]::IsNullOrWhiteSpace($content)) {
		return $null
	}
	return $content | ConvertFrom-Json
}

function Assert-RequestedFeature {
	param(
		[object]$LatestRequest,
		[string]$FeatureType
	)
	$found = $false
	foreach ($row in @((Get-OptionalProperty -Object $LatestRequest -Name "requested_features"))) {
		if ([string](Get-OptionalProperty -Object $row -Name "feature_type") -eq $FeatureType) {
			$found = $true
			break
		}
	}
	if (-not $found) {
		Fail ("latest_request.requested_features missing " + $FeatureType)
	}
}

function Assert-BridgeSnapshotRow {
	param(
		[object]$Snapshot,
		[string]$Name,
		[string]$ExpectedRequestID,
		[string]$Label
	)
	$row = Get-OptionalProperty -Object $Snapshot -Name $Name
	$status = [string](Get-OptionalProperty -Object $row -Name "status")
	if ([string]::IsNullOrWhiteSpace($status)) {
		Fail ($Label + " acoustic feature " + $Name + " missing status")
	}
	$rowRequestID = [string](Get-OptionalProperty -Object $row -Name "request_id")
	if (-not [string]::IsNullOrWhiteSpace($ExpectedRequestID) -and -not [string]::IsNullOrWhiteSpace($rowRequestID) -and $rowRequestID -ne $ExpectedRequestID) {
		Fail ($Label + " acoustic feature " + $Name + " request_id mismatch: got=" + $rowRequestID + " expected=" + $ExpectedRequestID)
	}
	if ($status -in @("ready", "partial")) {
		if ([string]::IsNullOrWhiteSpace($rowRequestID)) {
			Fail ($Label + " ready acoustic feature " + $Name + " missing request_id")
		}
		$source = [string](Get-OptionalProperty -Object $row -Name "source")
		if ([string]::IsNullOrWhiteSpace($source)) {
			Fail ($Label + " ready acoustic feature " + $Name + " missing source")
		}
		if ($Name -eq "spectrogram_tiles") {
			if ([string](Get-OptionalProperty -Object $row -Name "feature_type") -ne "spectral_field") {
				Fail ($Label + " spectrogram_tiles feature_type is not spectral_field")
			}
			if ($source -notin @("kernel_tile_ready_direct_collector", "kernel_tile_ready_godot_bridge")) {
				Fail ($Label + " spectrogram_tiles source is not accepted: " + $source)
			}
			$seen = Number-Value -Value (Get-OptionalProperty -Object $row -Name "tile_count_seen")
			$expected = Number-Value -Value (Get-OptionalProperty -Object $row -Name "tile_count_expected")
			if ($null -eq $seen -or $seen -lt 1 -or $null -eq $expected) {
				Fail ($Label + " spectrogram_tiles missing tile counts: " + ($row | ConvertTo-Json -Depth 8 -Compress))
			}
		}
		return
	}
	if ($status -in @("building", "requested", "pending", "missing", "blocked", "unavailable", "invalid", "stale", "suspect", "deferred", "failed")) {
		$reason = [string](Get-OptionalProperty -Object $row -Name "reason")
		$progress = Get-OptionalProperty -Object $row -Name "progress"
		$progressReason = [string](Get-OptionalProperty -Object $progress -Name "reason")
		if ([string]::IsNullOrWhiteSpace($reason) -and [string]::IsNullOrWhiteSpace($progressReason)) {
			Fail ($Label + " acoustic feature " + $Name + " status " + $status + " missing explicit reason/progress")
		}
		return
	}
	Fail ($Label + " acoustic feature " + $Name + " has unexpected status " + $status)
}

function Assert-FeatureSnapshotAcousticBridgeReadiness {
	param(
		[object]$Snapshot,
		[string]$Label
	)
	$latest = Get-OptionalProperty -Object $Snapshot -Name "latest_request"
	$requestID = [string](Get-OptionalProperty -Object $latest -Name "request_id")
	if ([string]::IsNullOrWhiteSpace($requestID)) {
		Fail ($Label + " feature_snapshot.latest_request.request_id missing")
	}
	Assert-RequestedFeature -LatestRequest $latest -FeatureType "waveform_envelope"
	Assert-RequestedFeature -LatestRequest $latest -FeatureType "spectral_field"
	foreach ($name in @("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary")) {
		Assert-BridgeSnapshotRow -Snapshot $Snapshot -Name $name -ExpectedRequestID $requestID -Label $Label
	}
}

function Assert-ObservationAcousticBridgeReadiness {
	param(
		[object]$Observation,
		[string]$Label
	)
	$global = Get-OptionalProperty -Object $Observation -Name "global_summary"
	$snapshot = Get-OptionalProperty -Object $global -Name "feature_snapshot"
	Assert-FeatureSnapshotAcousticBridgeReadiness -Snapshot $snapshot -Label $Label
	$mixPackage = Get-OptionalProperty -Object $Observation -Name "mix_package"
	$currentMetrics = Get-OptionalProperty -Object $mixPackage -Name "current_metrics"
	foreach ($name in @("band_energy", "stereo_relation")) {
		$row = Get-OptionalProperty -Object $currentMetrics -Name $name
		$status = [string](Get-OptionalProperty -Object $row -Name "status")
		if ($status -eq "missing" -and [string]::IsNullOrWhiteSpace([string](Get-OptionalProperty -Object $row -Name "reason"))) {
			Fail ($Label + " current_metrics." + $name + " missing explicit reason")
		}
	}
}

function Assert-ResponseAcousticBridgeReadiness {
	param(
		[object]$Response,
		[string]$Label
	)
	foreach ($row in @((Get-OptionalProperty -Object $Response -Name "executed_kernel_reply"))) {
		$result = Get-OptionalProperty -Object $row -Name "result"
		$observation = Get-OptionalProperty -Object $result -Name "observation"
		if ($null -ne $observation) {
			Assert-ObservationAcousticBridgeReadiness -Observation $observation -Label $Label
			return
		}
	}
	Fail ($Label + " did not expose a mix observation result")
}

function Get-FirstObservationFromResponse {
	param([object]$Response)
	foreach ($row in @((Get-OptionalProperty -Object $Response -Name "executed_kernel_reply"))) {
		$result = Get-OptionalProperty -Object $row -Name "result"
		$observation = Get-OptionalProperty -Object $result -Name "observation"
		if ($null -ne $observation) {
			return $observation
		}
	}
	return $null
}

function Assert-ResponseMOMMultitrackObservation {
	param(
		[object]$Response,
		[string]$Label,
		[string]$ExpectedGoalText = ""
	)
	$observation = Get-FirstObservationFromResponse -Response $Response
	if ($null -eq $observation) {
		Fail ($Label + " did not expose an observation")
	}
	$target = Get-OptionalProperty -Object $observation -Name "target_ref"
	$targetKind = [string](Get-OptionalProperty -Object $target -Name "kind")
	$targetID = [string](Get-OptionalProperty -Object $target -Name "id")
	if ($targetKind -ne "project" -or $targetID -ne "current") {
		Fail ($Label + " expected project target, got kind=" + $targetKind + " id=" + $targetID)
	}
	$listenScope = Get-OptionalProperty -Object $observation -Name "listen_scope"
	$listenSource = Get-OptionalProperty -Object $listenScope -Name "source"
	$listenMode = [string](Get-OptionalProperty -Object $listenSource -Name "mode")
	if ($listenMode -ne "full_project") {
		Fail ($Label + " expected listen_scope.source.mode=full_project, got " + $listenMode)
	}
	$mom = Get-OptionalProperty -Object $observation -Name "mom_projection"
	$intentPolicy = Get-OptionalProperty -Object $mom -Name "intent_policy"
	$intent = [string](Get-OptionalProperty -Object $intentPolicy -Name "name")
	if ([string]::IsNullOrWhiteSpace($intent)) {
		$intent = [string](Get-OptionalProperty -Object $mom -Name "intent")
	}
	if ($intent -ne "project_multitrack_relation_observation") {
		Fail ($Label + " expected MOM project_multitrack_relation_observation, got " + $intent)
	}
	$relation = Get-OptionalProperty -Object $mom -Name "multitrack_relation"
	$relationStatus = [string](Get-OptionalProperty -Object $relation -Name "status")
	if ($relationStatus -eq "not_applicable_single_track") {
		Fail ($Label + " unexpectedly downgraded to not_applicable_single_track")
	}
	$trust = Get-OptionalProperty -Object $mom -Name "trust_quality"
	$coverage = Get-OptionalProperty -Object $trust -Name "coverage"
	$compared = Number-Value -Value (Get-OptionalProperty -Object $coverage -Name "compared_track_count")
	if ($null -eq $compared -or $compared -lt 2) {
		Fail ($Label + " expected compared_track_count>=2, got " + [string]$compared)
	}
	$projectPackage = Get-OptionalProperty -Object $observation -Name "project_package"
	$projectTracks = @((Get-OptionalProperty -Object $projectPackage -Name "tracks"))
	if ($projectTracks.Count -lt 2) {
		Fail ($Label + " expected project_package.tracks>=2, got " + [string]$projectTracks.Count)
	}
	$llmContext = Get-OptionalProperty -Object $mom -Name "llm_context"
	if ([bool](Get-OptionalProperty -Object $llmContext -Name "do_not_include_raw_package") -ne $true) {
		Fail ($Label + " MOM llm_context did not block raw package")
	}
	if (-not [string]::IsNullOrWhiteSpace($ExpectedGoalText)) {
		$mixPackage = Get-OptionalProperty -Object $observation -Name "mix_package"
		$goalText = [string](Get-OptionalProperty -Object $mixPackage -Name "goal_text")
		if ($goalText -ne $ExpectedGoalText) {
			Fail ($Label + " goal_text mismatch: got=" + $goalText + " expected=" + $ExpectedGoalText)
		}
	}
}

function Get-AcousticPackageStatusFromResponse {
	param([object]$Response)
	foreach ($event in @((Get-OptionalProperty -Object $Response -Name "typed_events"))) {
		$state = Get-OptionalProperty -Object $event -Name "state"
		if ([string](Get-OptionalProperty -Object $state -Name "schema_version") -eq "acoustic_package_status.v0") {
			return $state
		}
		if ($null -ne (Get-OptionalProperty -Object $state -Name "package_layers")) {
			return $state
		}
	}
	foreach ($row in @((Get-OptionalProperty -Object $Response -Name "executed_kernel_reply"))) {
		$result = Get-OptionalProperty -Object $row -Name "result"
		$status = Get-OptionalProperty -Object $result -Name "acoustic_package_status"
		if ($null -ne $status) {
			return $status
		}
		$digest = Get-OptionalProperty -Object $result -Name "acoustic_digest"
		$status = Get-OptionalProperty -Object $digest -Name "acoustic_package_status"
		if ($null -ne $status) {
			return $status
		}
	}
	return $null
}

function Test-AcousticPackageDeepIncomplete {
	param([object]$Status)
	if ($null -eq $Status) {
		return $false
	}
	$layers = Get-OptionalProperty -Object $Status -Name "package_layers"
	$l3 = Get-OptionalProperty -Object $layers -Name "l3_deep"
	$l3Status = ([string](Get-OptionalProperty -Object $l3 -Name "status")).ToLowerInvariant()
	if (-not [string]::IsNullOrWhiteSpace($l3Status) -and $l3Status -ne "ready") {
		return $true
	}
	$features = Get-OptionalProperty -Object $l3 -Name "features"
	foreach ($name in @("spectrogram_tiles", "band_energy_summary", "stereo_relation_summary")) {
		$feature = Get-OptionalProperty -Object $features -Name $name
		$statusText = ([string](Get-OptionalProperty -Object $feature -Name "status")).ToLowerInvariant()
		if ($statusText -ne "ready") {
			return $true
		}
	}
	return $false
}

function Test-ResponseAcousticPackageDeepIncomplete {
	param([object]$Response)
	$status = Get-AcousticPackageStatusFromResponse -Response $Response
	if (Test-AcousticPackageDeepIncomplete -Status $status) {
		return $true
	}
	$reply = [string](Get-OptionalProperty -Object $Response -Name "reply")
	return ($reply -match "L3|深度|spectrogram") -and ($reply -match "building|partial|未完整|未完成|不可靠|正在构建|还在构建")
}

function Assert-AuthoritativeFeatureSnapshotPath {
	param(
		[string]$RepoRoot,
		[string]$GodotProjectRoot,
		[string]$ArtifactDir,
		[datetime]$RunStartedAt
	)
	$authoritative = Join-Path $RepoRoot "VitApp\Workspace\Artifacts\mixboard_feature_snapshot.json"
	$split = Join-Path $GodotProjectRoot "VitApp\Workspace\Artifacts\mixboard_feature_snapshot.json"
	if (-not (Test-Path -LiteralPath $authoritative)) {
		Fail ("authoritative mixboard feature snapshot missing: " + $authoritative)
	}
	$snapshot = Read-JsonFile -Path $authoritative
	Assert-FeatureSnapshotAcousticBridgeReadiness -Snapshot $snapshot -Label "authoritative snapshot"
	Copy-Item -LiteralPath $authoritative -Destination (Join-Path $ArtifactDir "authoritative_mixboard_feature_snapshot.json") -Force
	$out = [ordered]@{
		authoritative_path = $authoritative
		authoritative_last_write_utc = (Get-Item -LiteralPath $authoritative).LastWriteTimeUtc.ToString("o")
		godot_project_split_path = $split
		godot_project_split_exists = $false
	}
	if (Test-Path -LiteralPath $split) {
		$item = Get-Item -LiteralPath $split
		$out["godot_project_split_exists"] = $true
		$out["godot_project_split_last_write_utc"] = $item.LastWriteTimeUtc.ToString("o")
		if ($item.LastWriteTimeUtc -ge $RunStartedAt.ToUniversalTime().AddSeconds(-2)) {
			Fail ("Godot project split mixboard feature snapshot was updated during this run: " + $split)
		}
	}
	return $out
}

function Assert-ToolPresent {
	param(
        [string[]]$Tools,
        [string[]]$Aliases,
        [string]$Label
    )
    foreach ($alias in $Aliases) {
        if ($Tools -contains $alias) {
            return
        }
    }
    Fail ("Expected " + $Label + " in route. route=" + ($Tools -join " -> "))
}

function Assert-ToolAbsent {
    param(
        [string[]]$Tools,
        [string[]]$Aliases,
        [string]$Label
    )
    foreach ($alias in $Aliases) {
        if ($Tools -contains $alias) {
            Fail ("Unexpected " + $Label + " in route. route=" + ($Tools -join " -> "))
        }
    }
}

function Count-ExecutedToolGroup {
    param(
        [object]$Rows,
        [string[]]$Aliases
    )
    $count = 0
    foreach ($row in @($Rows)) {
        $matched = $false
        foreach ($key in @("tool", "command_name", "command")) {
            $name = [string](Get-OptionalProperty -Object $row -Name $key)
            if ($Aliases -contains $name) {
                $matched = $true
            }
        }
        if ($matched) {
            $count++
        }
    }
    return $count
}

function Wait-LogPattern {
    param(
        [string]$LogPath,
        [string]$Pattern,
        [int]$TimeoutSeconds
    )
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        if (Test-Path -LiteralPath $LogPath) {
            $hit = Select-String -Path $LogPath -Pattern $Pattern -SimpleMatch -ErrorAction SilentlyContinue | Select-Object -First 1
            if ($null -ne $hit) {
                return $hit.Line
            }
        }
        Start-Sleep -Milliseconds 250
    } while ((Get-Date) -lt $deadline)
    Fail ("Timed out waiting for log pattern: " + $Pattern)
}

function Reset-FixtureProject {
    $newProject = Invoke-AgentTool -Tool "project.new" -ToolArgs @{} -Confirmed $true
    if ([string](Get-OptionalProperty -Object $newProject -Name "status") -ne "ok") {
        Write-WarnLine ("project.new unavailable: " + [string](Get-OptionalProperty -Object $newProject -Name "error"))
    }
    $clear = Invoke-AgentTool -Tool "project.clear" -ToolArgs @{} -Confirmed $true
    Assert-StatusOk -Response $clear -Label "project.clear"
}

function Resolve-TrackID {
    param([object]$Response)
    $result = Get-OptionalProperty -Object $Response -Name "result"
    $trackID = [string](Get-OptionalProperty -Object $result -Name "track_id")
    if ([string]::IsNullOrWhiteSpace($trackID)) {
        $trackID = [string](Get-OptionalProperty -Object $result -Name "id")
    }
    return $trackID
}

function Resolve-ClipID {
    param([object]$Response)
    $result = Get-OptionalProperty -Object $Response -Name "result"
    $clipID = [string](Get-OptionalProperty -Object $result -Name "clip_id")
    if ([string]::IsNullOrWhiteSpace($clipID)) {
        $clipID = [string](Get-OptionalProperty -Object $result -Name "id")
    }
    if ([string]::IsNullOrWhiteSpace($clipID)) {
        $clipID = [string](Get-OptionalProperty -Object $Response -Name "clip_id")
    }
    return $clipID
}

function Set-AgentUIContext {
    param([hashtable]$Context)
    $resp = Invoke-Json -Method POST -Uri ($AgentHttp.TrimEnd("/") + "/agent/ui/context") -Body $Context -TimeoutSec 10
    if ($null -eq $resp -or [string](Get-OptionalProperty -Object $resp -Name "status") -ne "ok") {
        Fail ("agent ui context update failed: " + ($resp | ConvertTo-Json -Depth 12 -Compress))
    }
    return $resp
}

function Find-ExecutedToolResult {
    param(
        [object]$Rows,
        [string[]]$Aliases
    )
    foreach ($row in @($Rows)) {
        foreach ($key in @("tool", "command_name", "command")) {
            $name = [string](Get-OptionalProperty -Object $row -Name $key)
            if ($Aliases -contains $name) {
                return Get-OptionalProperty -Object $row -Name "result"
            }
        }
    }
    return $null
}

function Import-AudioFixture {
    param(
        [string]$TrackID,
        [string]$FilePath
    )
    $import = Invoke-AgentTool -Tool "clip.import_media_to_track" -ToolArgs @{
        track_id = $TrackID
        file_path = $FilePath
        start_time = 0
        media_type = "audio"
        mode = "non_destructive"
    } -Confirmed $true
    if ([string](Get-OptionalProperty -Object $import -Name "status") -eq "ok") {
        return $import
    }
    return Invoke-AgentTool -Tool "clip.import_audio" -ToolArgs @{
        track_id = $TrackID
        file_path = $FilePath
        offset_time = 0
    } -Confirmed $true
}

$RepoRoot = (Resolve-Path -LiteralPath $RepoRoot).Path
$WorkspaceDir = Join-Path $RepoRoot "VitApp\Workspace"
$LogsDir = Join-Path $WorkspaceDir "Logs"
$SmokeRoot = Join-Path $WorkspaceDir "Artifacts\smoke"
$Stamp = Get-Date -Format "yyyyMMdd_HHmmss"
$ArtifactDir = Join-Path $SmokeRoot ("product_path_" + $Stamp)
New-Item -ItemType Directory -Path $ArtifactDir -Force | Out-Null
New-Item -ItemType Directory -Path $LogsDir -Force | Out-Null

$AgentLog = Join-Path $LogsDir "agent_last.log"
$GodotRuntimeStdout = Join-Path $ArtifactDir "godot_runtime_stdout.log"
$GodotRuntimeStderr = Join-Path $ArtifactDir "godot_runtime_stderr.log"
$Track1Path = Resolve-FirstExistingPath -Label "track 1 fixture" -Candidates @((Join-Path $RepoRoot "test_100hz_10s.wav"))
$Track2Path = Resolve-FirstExistingPath -Label "track 2 fixture" -Candidates @((Join-Path $RepoRoot "test_target_3s.wav"))
if ([string]::IsNullOrWhiteSpace($KernelExe)) {
    $KernelExe = Resolve-FirstExistingPath -Label "kernel exe" -Candidates @(
        (Join-Path $RepoRoot "VitApp\build_release\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $RepoRoot "VitApp\build\VitApp_artefacts\Release\VitApp.exe"),
        (Join-Path $RepoRoot "VitApp\build_release\VitApp.exe"),
        (Join-Path $RepoRoot "Export\staging\runtime\VitApp.exe")
    )
}
else {
    $KernelExe = (Resolve-Path -LiteralPath $KernelExe).Path
}
$GodotProjectRoot = Resolve-GodotProjectRoot -RequestedProjectRoot $GodotProjectRoot
$GodotExe = Resolve-GodotExecutable -RequestedGodotExe $GodotExe -ProjectRoot $GodotProjectRoot
$GodotLaunchExe = Resolve-GodotLaunchExecutable -GodotExe $GodotExe
if (-not [string]::IsNullOrWhiteSpace($UIExe)) {
    $UIExe = (Resolve-Path -LiteralPath $UIExe).Path
}
if ([string]::IsNullOrWhiteSpace($AgentExe)) {
    $AgentExe = Join-Path $RepoRoot "agent\bin\VitAgent.exe"
}
$AgentExe = [System.IO.Path]::GetFullPath($AgentExe)
if ([string]::IsNullOrWhiteSpace($VspHubExe)) {
    $VspHubExe = Join-Path $RepoRoot "agent\bin\VspHub.exe"
}
$VspHubExe = [System.IO.Path]::GetFullPath($VspHubExe)

$RunStartedAt = Get-Date
$summary = [ordered]@{
	schema_version = "vit_product_path_smoke.v1"
	created_at = $RunStartedAt.ToString("o")
    repo_root = $RepoRoot
    artifact_dir = $ArtifactDir
    interaction_path = "agent_http_after_godot_project_lifecycle"
    product_lifecycle = "godot_project"
    godot_exe = $GodotExe
    godot_launch_exe = $GodotLaunchExe
    godot_project_root = $GodotProjectRoot
    ui_exe_override = $UIExe
    kernel_exe = $KernelExe
    vsp_hub_exe = $VspHubExe
    agent_exe = $AgentExe
    godot_runtime_stdout = $GodotRuntimeStdout
    godot_runtime_stderr = $GodotRuntimeStderr
    godot_autostart = @()
    conversation_id = $null
    observation_id = $null
    pending_candidate = $null
    tool_route = @()
    before_after = $null
    stop_reasons = @{}
	mix_loop_v1 = @{}
	focus_relationship = @{}
	vocal_clarification_loop = @{}
    snapshot_paths = @{}
	processes = @{}
    binary_evidence = @{}
    ports = @()
    status = "running"
}

$started = @{
    agent = $null
    hub = $null
    kernel = $null
    godot = $null
    ui_override = $null
}
$godotAutostartEvidence = @()
$oldDevRoot = $env:VIT_DAW_DEV_ROOT
$oldVspHubLastLogPath = $env:VIT_VSP_HUB_LAST_LOG_PATH
$oldAgentVspHubUrl = $env:VIT_AGENT_VSP_HUB_URL
$oldAgentLastLogPath = $env:VIT_AGENT_LAST_LOG_PATH
$oldAgentKeepLogLines = $env:VIT_AGENT_KEEP_LAST_LOG_LINES
$oldSkipDevAutostart = $env:VIT_SKIP_DEV_AUTOSTART
$observeEventSeq = 0

try {
    Write-Step "Product-path lifecycle smoke"
    Write-Host ("repo: " + $RepoRoot)
    Write-Host ("artifact_dir: " + $ArtifactDir)
    Write-Host ("interaction_path=agent_http_after_godot_project_lifecycle")
    Write-Host ("godot: " + $GodotExe)
    Write-Host ("godot_launch: " + $GodotLaunchExe)
    Write-Host ("godot_project: " + $GodotProjectRoot)
    if (-not [string]::IsNullOrWhiteSpace($UIExe)) {
        Write-Host ("ui_override: " + $UIExe)
    }
    Write-Host ("expected kernel: " + $KernelExe)
    Write-Host ("vsp_hub: " + $VspHubExe)
    Write-Host ("agent: " + $AgentExe)

    Write-Step "Prepare agent + VSP Hub"
    if (-not $SkipBuild) {
        Stop-AgentIfNeededBeforeBuild -ReuseAgent ([bool]$ReuseAgent)
        Stop-HubIfNeededBeforeBuild -ReuseHub ([bool]$ReuseHub)
    }
    Build-AgentAndHubIfNeeded -RepoRoot $RepoRoot -AgentPath $AgentExe -VspHubPath $VspHubExe -SkipAgentBuild ([bool]($SkipBuild -or $ReuseAgent)) -SkipHubBuild ([bool]($SkipBuild -or $ReuseHub))
    $summary["binary_evidence"]["agent_expected"] = Get-ExecutableEvidence -Path $AgentExe
    $summary["binary_evidence"]["hub_expected"] = Get-ExecutableEvidence -Path $VspHubExe
    ConvertTo-JsonFile -Value ($summary["binary_evidence"]["agent_expected"]) -Path (Join-Path $ArtifactDir "agent_expected_binary.json")
    ConvertTo-JsonFile -Value ($summary["binary_evidence"]["hub_expected"]) -Path (Join-Path $ArtifactDir "hub_expected_binary.json")

    Write-Step "Start or reuse Godot project lifecycle"
    $summary["processes"]["godot_editors"] = @(Find-GodotProjectProcesses -ProjectRoot $GodotProjectRoot -RuntimeOnly $false | Where-Object { [bool]$_.is_editor })
    Stop-GodotProjectRuntimeIfNeeded -ProjectRoot $GodotProjectRoot -ReuseGodot ([bool]$ReuseGodot)
    Stop-PortOwnerIfNeeded -Port $AgentHttpPort -Label "agent HTTP" -Reuse ([bool]$ReuseAgent)
    Stop-PortOwnerIfNeeded -Port $VspHubPort -Label "VSP Hub HTTP" -Reuse ([bool]$ReuseHub)
    Stop-PortOwnerIfNeeded -Port $ZmqReqPort -Label "kernel command" -Reuse ([bool]$ReuseKernel)
    Stop-PortOwnerIfNeeded -Port $ZmqSubPort -Label "kernel event" -Reuse ([bool]$ReuseKernel)

    if (-not $ReuseAgent -and -not $ReuseKernel) {
        $ownedFeatureSnapshots = @(
            (Join-Path $RepoRoot "VitApp\Workspace\Artifacts\mixboard_feature_snapshot.json"),
            (Join-Path $GodotProjectRoot "VitApp\Workspace\Artifacts\mixboard_feature_snapshot.json")
        )
        foreach ($snapshotPath in $ownedFeatureSnapshots) {
            if (Test-Path -LiteralPath $snapshotPath) {
                Remove-Item -LiteralPath $snapshotPath -Force
            }
        }
    }

    $env:VIT_DAW_DEV_ROOT = $RepoRoot
    $env:VIT_VSP_HUB_LAST_LOG_PATH = Join-Path $ArtifactDir "vsp_hub_last.log"
    $env:VIT_AGENT_VSP_HUB_URL = ($VspHubHttp.TrimEnd("/") + "/vsp")
    $env:VIT_AGENT_LAST_LOG_PATH = $AgentLog
    $env:VIT_AGENT_KEEP_LAST_LOG_LINES = "1200"
    $env:VIT_SKIP_DEV_AUTOSTART = "0"
    if (Test-Path -LiteralPath (Join-Path $GodotProjectRoot "godot_runtime.log")) {
        Copy-Item -LiteralPath (Join-Path $GodotProjectRoot "godot_runtime.log") -Destination (Join-Path $ArtifactDir "godot_runtime_previous.log") -Force
        Remove-Item -LiteralPath (Join-Path $GodotProjectRoot "godot_runtime.log") -Force
    }
    if (Test-Path -LiteralPath $AgentLog) {
        Copy-Item -LiteralPath $AgentLog -Destination (Join-Path $ArtifactDir "agent_last_previous.log") -Force
        Remove-Item -LiteralPath $AgentLog -Force
    }

    if (-not [string]::IsNullOrWhiteSpace($UIExe)) {
        $started.ui_override = Start-Or-Reuse-UI -UIPath $UIExe -ReuseUI ([bool]$ReuseUI)
        $summary["processes"]["ui_override"] = $started.ui_override
    }
    $started.godot = Start-Or-Reuse-GodotProject -GodotLaunchExe $GodotLaunchExe -ProjectRoot $GodotProjectRoot -StdoutLog $GodotRuntimeStdout -StderrLog $GodotRuntimeStderr -ReuseGodot ([bool]$ReuseGodot) -TimeoutSeconds $TimeoutSeconds
    $summary["processes"]["godot_runtime"] = $started.godot

    $lifecycleEvidence = Wait-GodotLifecycleEvidence -StdoutLog $GodotRuntimeStdout -AlternateLog (Join-Path $GodotProjectRoot "godot_runtime.log") -TimeoutSeconds $TimeoutSeconds
    if ([string]$lifecycleEvidence.mode -eq "missing_godot_lifecycle_evidence") {
        Fail "Missing Godot lifecycle evidence: no autostart log and no Hub/agent self-check with live ports"
    }
    $summary["godot_lifecycle_evidence"] = @{
        mode = [string]$lifecycleEvidence.mode
        agent_tools_ready_log = [bool]$lifecycleEvidence.agent_tools_ready_log
        vsp_hub_ready_log = [bool]$lifecycleEvidence.vsp_hub_ready_log
    }
    $godotAutostartEvidence = @($lifecycleEvidence.autostart)
    $summary["godot_autostart"] = $godotAutostartEvidence
    ConvertTo-JsonFile -Value $godotAutostartEvidence -Path (Join-Path $ArtifactDir "godot_autostart.json")

    $kernelChild = Get-GodotAutostartChild -Evidence $godotAutostartEvidence -Role "kernel"
    $hubChild = Get-GodotAutostartChild -Evidence $godotAutostartEvidence -Role "hub"
    $agentChild = Get-GodotAutostartChild -Evidence $godotAutostartEvidence -Role "agent"
    $cleanupGodotChildren = -not ([bool]$ReuseGodot -or [bool]$ReuseAgent -or [bool]$ReuseHub -or [bool]$ReuseKernel)
    $started.agent = Assert-GodotOwnedPort -Port $AgentHttpPort -Child $agentChild -Label "agent HTTP" -AllowExisting ([bool]$ReuseAgent) -LifecycleMode ([string]$lifecycleEvidence.mode) -ExpectedPath $AgentExe -CleanupProcess $cleanupGodotChildren
    $started.hub = Assert-GodotOwnedPort -Port $VspHubPort -Child $hubChild -Label "VSP Hub HTTP" -AllowExisting ([bool]$ReuseHub) -LifecycleMode ([string]$lifecycleEvidence.mode) -ExpectedPath $VspHubExe -CleanupProcess $cleanupGodotChildren
    $started.kernel = Assert-GodotOwnedPort -Port $ZmqReqPort -Child $kernelChild -Label "kernel command" -AllowExisting ([bool]$ReuseKernel) -LifecycleMode ([string]$lifecycleEvidence.mode) -ExpectedPath $KernelExe -CleanupProcess $cleanupGodotChildren
    $kernelEventOwner = Assert-GodotOwnedPort -Port $ZmqSubPort -Child $kernelChild -Label "kernel event" -AllowExisting ([bool]$ReuseKernel) -LifecycleMode ([string]$lifecycleEvidence.mode) -ExpectedPath $KernelExe -CleanupProcess $cleanupGodotChildren
    $started.kernel["event_port_pid"] = [int]$kernelEventOwner.pid
    $summary["processes"]["agent"] = $started.agent
    $summary["processes"]["hub"] = $started.hub
    $summary["processes"]["kernel"] = $started.kernel
    $runningHubPath = Get-ProcessPathByID -ProcessID ([int]$started.hub.pid)
    if ((Normalize-ComparablePath -Path $runningHubPath) -ne (Normalize-ComparablePath -Path $VspHubExe)) {
        Fail ("Godot product path is not using expected VSP Hub binary. expected=" + $VspHubExe + " actual=" + $runningHubPath)
    }
    $summary["binary_evidence"]["hub_running"] = Get-ExecutableEvidence -Path $runningHubPath
    if ([string]$summary["binary_evidence"]["hub_expected"].sha256 -ne [string]$summary["binary_evidence"]["hub_running"].sha256) {
        Fail "Running VSP Hub binary hash does not match expected Hub binary"
    }
    Write-Ok ("Godot product path VSP Hub binary verified sha256=" + [string]$summary["binary_evidence"]["hub_running"].sha256)
    $runningAgentPath = Get-ProcessPathByID -ProcessID ([int]$started.agent.pid)
    if ((Normalize-ComparablePath -Path $runningAgentPath) -ne (Normalize-ComparablePath -Path $AgentExe)) {
        Fail ("Godot product path is not using expected agent binary. expected=" + $AgentExe + " actual=" + $runningAgentPath)
    }
    $summary["binary_evidence"]["agent_running"] = Get-ExecutableEvidence -Path $runningAgentPath
    if ([string]$summary["binary_evidence"]["agent_expected"].sha256 -ne [string]$summary["binary_evidence"]["agent_running"].sha256) {
        Fail "Running agent binary hash does not match expected agent binary"
    }
    Write-Ok ("Godot product path agent binary verified sha256=" + [string]$summary["binary_evidence"]["agent_running"].sha256)

    $summary["ports"] = @(Get-ListenersSnapshot)
    ConvertTo-JsonFile -Value ($summary["ports"]) -Path (Join-Path $ArtifactDir "ports.json")
    ConvertTo-JsonFile -Value ($summary["processes"]) -Path (Join-Path $ArtifactDir "processes.json")

    Write-Step "Health checks"
    $hubHealth = Invoke-Json -Method GET -Uri ($VspHubHttp.TrimEnd("/") + "/health") -TimeoutSec 10
    ConvertTo-JsonFile -Value $hubHealth -Path (Join-Path $ArtifactDir "vsp_hub_health.json")
    if ($null -eq $hubHealth -or [string]$hubHealth.status -ne "ok" -or [string]$hubHealth.service -ne "VspHub") {
        Fail "GET /health did not return VspHub ok"
    }
    $hubStatus = Wait-VspHubAgentSession -TimeoutSeconds 20
    ConvertTo-JsonFile -Value $hubStatus -Path (Join-Path $ArtifactDir "vsp_hub_status.json")
    $transports = @($hubStatus.transports)
    if (-not ($transports -contains "vsp.hub.http")) {
        Fail "VSP Hub status did not advertise vsp.hub.http"
    }
    Write-Ok "VSP Hub health/status passed with VitAgent session"

    $state = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/state") -TimeoutSec 10
    ConvertTo-JsonFile -Value $state -Path (Join-Path $ArtifactDir "agent_state.json")
    if ($null -eq $state -or [string]$state.status -ne "ok") {
        Fail "GET /agent/state did not return ok"
    }
    Assert-StatusOk -Response (Invoke-AgentTool -Tool "project.state" -ToolArgs @{} -Confirmed $false) -Label "project.state"


    if ($B2StaticBalanceAgentOnly) {
        Write-Step "B2 static-balance Agent smoke through Godot-owned lifecycle"
        $b2Script = Join-Path $RepoRoot "scripts\b2_static_balance_agent_smoke.py"
        if (-not (Test-Path -LiteralPath $b2Script)) {
            Fail ("Missing B2 smoke script: " + $b2Script)
        }
        $b2Output = Join-Path $ArtifactDir "b2_static_balance_stdout.json"
        if ([string]::IsNullOrWhiteSpace($B2StemsFolder) -or -not (Test-Path -LiteralPath $B2StemsFolder -PathType Container)) {
            Fail "B2 focused smoke requires -B2StemsFolder with at least two representative stems"
        }
        $b2StemsDir = (Resolve-Path -LiteralPath $B2StemsFolder).Path
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        & python $b2Script --repo-root $RepoRoot --agent-http $AgentHttp --timeout-sec ([Math]::Max(240, $TimeoutSeconds)) --dad-timeout-sec ([Math]::Max(240, $TimeoutSeconds)) --prepare-stems-folder $b2StemsDir --decision approve 2>&1 |
            Tee-Object -FilePath $b2Output
        $b2ExitCode = $LASTEXITCODE
        $ErrorActionPreference = $previousErrorActionPreference
        if ($b2ExitCode -ne 0) {
            Fail ("B2 static-balance Agent smoke failed with exit code " + $b2ExitCode)
        }
        $b2Artifact = Get-ChildItem -LiteralPath (Join-Path $RepoRoot "VitApp\Workspace\Artifacts\smoke") -Directory -Filter "b2_static_balance_*" |
            Sort-Object LastWriteTimeUtc -Descending |
            Select-Object -First 1
        if ($null -eq $b2Artifact -or -not (Test-Path -LiteralPath (Join-Path $b2Artifact.FullName "summary.json"))) {
            Fail "B2 static-balance smoke did not produce summary.json"
        }
        $b2Summary = Get-Content -LiteralPath (Join-Path $b2Artifact.FullName "summary.json") -Raw -Encoding UTF8 | ConvertFrom-Json
        if ([string]$b2Summary.status -ne "ok") {
            Fail ("B2 static-balance smoke summary is not ok: " + ($b2Summary | ConvertTo-Json -Depth 16 -Compress))
        }
        $summary["b2_static_balance_agent"] = $b2Summary
        $summary["conversation_id"] = [string]$b2Summary.conversation_id
        $summary["tool_route"] = @($b2Summary.preflight_tools)
        $summary["status"] = "passed"
        Write-Ok "focused B2 static-balance Godot product-path smoke passed"
        return
    }

    if ($B4LowEndRelationAgentOnly) {
        Write-Step "B4 low-end relation Agent smoke through Godot-owned lifecycle"
        $b4Script = Join-Path $RepoRoot "scripts\b4_low_end_relation_agent_smoke.py"
        if (-not (Test-Path -LiteralPath $b4Script)) {
            Fail ("Missing B4 smoke script: " + $b4Script)
        }
        $b4Output = Join-Path $ArtifactDir "b4_low_end_relation_stdout.json"
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        & python $b4Script --repo-root $RepoRoot --agent-http $AgentHttp --timeout-sec ([Math]::Max(180, $TimeoutSeconds)) --dad-timeout-sec ([Math]::Max(240, $TimeoutSeconds)) 2>&1 |
            Tee-Object -FilePath $b4Output
        $b4ExitCode = $LASTEXITCODE
        $ErrorActionPreference = $previousErrorActionPreference
        if ($b4ExitCode -ne 0) {
            Fail ("B4 low-end relation Agent smoke failed with exit code " + $b4ExitCode)
        }
        $b4Artifact = Get-ChildItem -LiteralPath (Join-Path $RepoRoot "VitApp\Workspace\Artifacts\smoke") -Directory -Filter "b4_low_end_relation_*" |
            Sort-Object LastWriteTimeUtc -Descending |
            Select-Object -First 1
        if ($null -eq $b4Artifact -or -not (Test-Path -LiteralPath (Join-Path $b4Artifact.FullName "summary.json"))) {
            Fail "B4 low-end relation smoke did not produce summary.json"
        }
        $b4Summary = Get-Content -LiteralPath (Join-Path $b4Artifact.FullName "summary.json") -Raw -Encoding UTF8 | ConvertFrom-Json
        if ([string]$b4Summary.status -ne "ok") {
            Fail ("B4 low-end relation smoke summary is not ok: " + ($b4Summary | ConvertTo-Json -Depth 16 -Compress))
        }
        $summary["b4_low_end_relation_agent"] = $b4Summary
        $summary["conversation_id"] = [string]$b4Summary.conversation_id
        $summary["tool_route"] = @($b4Summary.turns.analysis.tools)
        $summary["status"] = "passed"
        Write-Ok "focused B4 low-end relation Godot product-path smoke passed"
        return
    }

    if ($B3PanLayoutAgentOnly) {
        Write-Step "B3 pan-layout Agent smoke through Godot-owned lifecycle"
        $b3Script = Join-Path $RepoRoot "scripts\b3_pan_layout_agent_smoke.py"
        if (-not (Test-Path -LiteralPath $b3Script)) {
            Fail ("Missing B3 smoke script: " + $b3Script)
        }
        $b3Output = Join-Path $ArtifactDir "b3_pan_layout_stdout.json"
        if ([string]::IsNullOrWhiteSpace($B3StemsFolder) -or -not (Test-Path -LiteralPath $B3StemsFolder -PathType Container)) {
            Fail "B3 focused smoke requires -B3StemsFolder with representative analyzed stems"
        }
        $b3StemsDir = (Resolve-Path -LiteralPath $B3StemsFolder).Path
        $previousErrorActionPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        & python $b3Script --repo-root $RepoRoot --agent-http $AgentHttp --timeout-sec ([Math]::Max(240, $TimeoutSeconds)) --dad-timeout-sec ([Math]::Max(240, $TimeoutSeconds)) --prepare-stems-folder $b3StemsDir 2>&1 |
            Tee-Object -FilePath $b3Output
        $b3ExitCode = $LASTEXITCODE
        $ErrorActionPreference = $previousErrorActionPreference
        if ($b3ExitCode -ne 0) {
            Fail ("B3 pan-layout Agent smoke failed with exit code " + $b3ExitCode)
        }
        $b3Artifact = Get-ChildItem -LiteralPath (Join-Path $RepoRoot "VitApp\Workspace\Artifacts\smoke") -Directory -Filter "b3_pan_layout_*" |
            Sort-Object LastWriteTimeUtc -Descending |
            Select-Object -First 1
        if ($null -eq $b3Artifact -or -not (Test-Path -LiteralPath (Join-Path $b3Artifact.FullName "summary.json"))) {
            Fail "B3 pan-layout smoke did not produce summary.json"
        }
        $b3Summary = Get-Content -LiteralPath (Join-Path $b3Artifact.FullName "summary.json") -Raw -Encoding UTF8 | ConvertFrom-Json
        if ([string]$b3Summary.status -ne "ok") {
            Fail ("B3 pan-layout smoke summary is not ok: " + ($b3Summary | ConvertTo-Json -Depth 16 -Compress))
        }
        $summary["b3_pan_layout_agent"] = $b3Summary
        $summary["conversation_id"] = [string]$b3Summary.conversation_id
        $summary["tool_route"] = @($b3Summary.decision_tools)
        $summary["status"] = "passed"
        Write-Ok "focused B3 pan-layout Godot product-path smoke passed"
        return
    }

    Write-Step "Create fixture project through agent + live kernel"
    Reset-FixtureProject
    $track1 = Invoke-AgentTool -Tool "track.add_audio" -ToolArgs @{ name = "Lead Vocal" } -Confirmed $true
    Assert-StatusOk -Response $track1 -Label "track.add_audio Lead Vocal"
    $track1ID = Resolve-TrackID -Response $track1
    $track2 = Invoke-AgentTool -Tool "track.add_audio" -ToolArgs @{ name = "Track 2" } -Confirmed $true
    Assert-StatusOk -Response $track2 -Label "track.add_audio Track 2"
    $track2ID = Resolve-TrackID -Response $track2
    if ([string]::IsNullOrWhiteSpace($track1ID) -or [string]::IsNullOrWhiteSpace($track2ID)) {
        Fail ("Could not resolve fixture track IDs: " + $track1ID + " / " + $track2ID)
    }
    $import1 = Import-AudioFixture -TrackID $track1ID -FilePath $Track1Path
    Assert-StatusOk -Response $import1 -Label "import Track 1"
    $import2 = Import-AudioFixture -TrackID $track2ID -FilePath $Track2Path
    Assert-StatusOk -Response $import2 -Label "import Track 2"
    $clip1ID = Resolve-ClipID -Response $import1
    $clip2ID = Resolve-ClipID -Response $import2
    if ([string]::IsNullOrWhiteSpace($clip1ID)) {
        Fail ("Could not resolve imported clip ID for Track 1: " + ($import1 | ConvertTo-Json -Depth 12 -Compress))
    }
    ConvertTo-JsonFile -Value @{
        track1_id = $track1ID
        track2_id = $track2ID
        clip1_id = $clip1ID
        clip2_id = $clip2ID
        track1_name = "Lead Vocal"
        track1_path = $Track1Path
        track2_path = $Track2Path
        import1 = $import1
        import2 = $import2
    } -Path (Join-Path $ArtifactDir "fixture.json")
    Start-Sleep -Milliseconds 750

    Write-Step "Clip fade/gain agent closed-loop smoke"
    $uiContext = @{
        selected_track_id = $track1ID
        selected_track_name = "Lead Vocal"
        selected_clip_id = $clip1ID
        selected_clip_ids = @($clip1ID)
        selected_clip_track_id = $track1ID
        selected_clip_name = "Lead Vocal clip"
        current_playhead_seconds = 0
    }
    $uiContextResp = Set-AgentUIContext -Context $uiContext
    ConvertTo-JsonFile -Value $uiContextResp -Path (Join-Path $ArtifactDir "clip_fade_gain_ui_context.json")

    $clipConversationID = "product_path_clip_fade_gain_" + $Stamp
    $setClipGain = Invoke-AgentChat -ConversationID $clipConversationID -Message "set current clip gain to -3 dB" -ExtraContext $uiContext
    ConvertTo-JsonFile -Value $setClipGain -Path (Join-Path $ArtifactDir "chat_clip_gain_pending.json")
    $setRows = @(Get-OptionalProperty -Object $setClipGain -Name "executed_kernel_reply")
    $setTools = Tool-Names -Rows $setRows
    $setStop = [string](Get-OptionalProperty -Object $setClipGain -Name "stop_reason")
    $setNeedsConfirmation = [bool](Get-OptionalProperty -Object $setClipGain -Name "needs_confirmation")
    if (-not $setNeedsConfirmation) {
        Fail ("clip gain set setup did not create a confirmation. stop_reason=" + $setStop + " tools=" + ($setTools -join " -> "))
    }
    Assert-ToolAbsent -Tools $setTools -Aliases @("mix.propose_tick", "mix_propose_tick", "mix.apply_tick", "mix_apply_tick", "track.volume", "track_volume", "set_volume") -Label "mix/track gain route during clip gain setup"

    $readClipStateMessage = Join-UnicodeChars @(0x8BFB, 0x53D6, 0x5F53, 0x524D, 0x9009, 0x4E2D, 0x20, 0x63, 0x6C, 0x69, 0x70, 0x20, 0x7684, 0x20, 0x66, 0x61, 0x64, 0x65, 0x20, 0x548C, 0x20, 0x67, 0x61, 0x69, 0x6E, 0x20, 0x72B6, 0x6001)
    $readClipState = Invoke-AgentChat -ConversationID $clipConversationID -Message $readClipStateMessage -ExtraContext $uiContext
    ConvertTo-JsonFile -Value $readClipState -Path (Join-Path $ArtifactDir "chat_clip_fade_gain_read.json")
    $readRows = @(Get-OptionalProperty -Object $readClipState -Name "executed_kernel_reply")
    $readTools = Tool-Names -Rows $readRows
    $readStop = [string](Get-OptionalProperty -Object $readClipState -Name "stop_reason")
    $readNeedsConfirmation = [bool](Get-OptionalProperty -Object $readClipState -Name "needs_confirmation")
    if ($readNeedsConfirmation -or [string](Get-OptionalProperty -Object $readClipState -Name "goal_status") -eq "waiting_confirmation" -or -not [string]::IsNullOrWhiteSpace([string](Get-OptionalProperty -Object $readClipState -Name "plan_id"))) {
        Fail ("clip fade/gain read was intercepted by stale confirmation: " + ($readClipState | ConvertTo-Json -Depth 16 -Compress))
    }
    if ($readStop -ne "done") {
        Fail ("clip fade/gain read stop_reason=" + $readStop + " reply=" + [string](Get-OptionalProperty -Object $readClipState -Name "reply"))
    }
    Assert-ToolPresent -Tools $readTools -Aliases @("clip.fade.read", "clip_fade_read") -Label "clip.fade.read"
    Assert-ToolPresent -Tools $readTools -Aliases @("clip.gain.read", "clip_gain_read") -Label "clip.gain.read"
    Assert-ToolAbsent -Tools $readTools -Aliases @("clip.gain.set", "clip_gain_set", "mix.propose_tick", "mix_propose_tick", "mix.apply_tick", "mix_apply_tick", "track.volume", "track_volume", "set_volume") -Label "stale write or mix route during clip read"
    $fadeResult = Find-ExecutedToolResult -Rows $readRows -Aliases @("clip.fade.read", "clip_fade_read")
    $gainResult = Find-ExecutedToolResult -Rows $readRows -Aliases @("clip.gain.read", "clip_gain_read")
    if ([string](Get-OptionalProperty -Object $fadeResult -Name "clip_id") -ne $clip1ID) {
        Fail ("clip.fade.read used wrong clip_id: " + ($fadeResult | ConvertTo-Json -Depth 12 -Compress))
    }
    if ([string](Get-OptionalProperty -Object $gainResult -Name "clip_id") -ne $clip1ID) {
        Fail ("clip.gain.read used wrong clip_id: " + ($gainResult | ConvertTo-Json -Depth 12 -Compress))
    }
    $clipReply = [string](Get-OptionalProperty -Object $readClipState -Name "reply")
    if ($clipReply -notmatch "Fade" -or $clipReply -notmatch "Clip gain") {
        Fail ("clip fade/gain read reply did not expose fade/gain state: " + $clipReply)
    }

    $fadeSetMessage = Join-UnicodeChars @(0x628A, 0x5F53, 0x524D, 0x9009, 0x4E2D, 0x20, 0x63, 0x6C, 0x69, 0x70, 0x20, 0x7684, 0x20, 0x66, 0x61, 0x64, 0x65, 0x20, 0x69, 0x6E, 0x20, 0x8BBE, 0x7F6E, 0x4E3A, 0x20, 0x30, 0x2E, 0x31, 0x35, 0x20, 0x79D2, 0xFF0C, 0x66, 0x61, 0x64, 0x65, 0x20, 0x6F, 0x75, 0x74, 0x20, 0x8BBE, 0x7F6E, 0x4E3A, 0x20, 0x30, 0x2E, 0x32, 0x35, 0x20, 0x79D2)
    $fadeSet = Invoke-AgentChat -ConversationID $clipConversationID -Message $fadeSetMessage -ExtraContext $uiContext
    ConvertTo-JsonFile -Value $fadeSet -Path (Join-Path $ArtifactDir "chat_clip_fade_set_pending.json")
    $fadeSetRows = @(Get-OptionalProperty -Object $fadeSet -Name "executed_kernel_reply")
    $fadeSetTools = Tool-Names -Rows $fadeSetRows
    $fadeSetNeedsConfirmation = [bool](Get-OptionalProperty -Object $fadeSet -Name "needs_confirmation")
    if (-not $fadeSetNeedsConfirmation) {
        Fail ("clip fade set did not request confirmation: " + ($fadeSet | ConvertTo-Json -Depth 16 -Compress))
    }
    Assert-ToolPresent -Tools $fadeSetTools -Aliases @("clip.fade.set", "clip_fade_set") -Label "clip.fade.set pending"
    Assert-ToolAbsent -Tools $fadeSetTools -Aliases @("clip.fade.read", "clip_fade_read", "clip.gain.read", "clip_gain_read", "mix.propose_tick", "mix_propose_tick", "mix.apply_tick", "mix_apply_tick", "track.volume", "track_volume", "set_volume") -Label "read or mix route during clip fade set"

    $clipConfirmMessage = Join-UnicodeChars @(0x53EF, 0x4EE5, 0x6267, 0x884C)
    $fadeConfirm = Invoke-AgentChat -ConversationID $clipConversationID -Message $clipConfirmMessage -ExtraContext $uiContext
    ConvertTo-JsonFile -Value $fadeConfirm -Path (Join-Path $ArtifactDir "chat_clip_fade_set_confirm.json")
    $fadeConfirmRows = @(Get-OptionalProperty -Object $fadeConfirm -Name "executed_kernel_reply")
    $fadeConfirmTools = Tool-Names -Rows $fadeConfirmRows
    if ([bool](Get-OptionalProperty -Object $fadeConfirm -Name "needs_confirmation")) {
        Fail ("clip fade set confirmation still needs confirmation: " + ($fadeConfirm | ConvertTo-Json -Depth 16 -Compress))
    }
    Assert-ToolPresent -Tools $fadeConfirmTools -Aliases @("clip.fade.set", "clip_fade_set") -Label "confirmed clip.fade.set"
    Assert-ToolAbsent -Tools $fadeConfirmTools -Aliases @("mix.propose_tick", "mix_propose_tick", "mix.apply_tick", "mix_apply_tick", "track.volume", "track_volume", "set_volume") -Label "mix route during confirmed clip fade set"

    $readAfterFadeSet = Invoke-AgentChat -ConversationID $clipConversationID -Message $readClipStateMessage -ExtraContext $uiContext
    ConvertTo-JsonFile -Value $readAfterFadeSet -Path (Join-Path $ArtifactDir "chat_clip_fade_gain_read_after_set.json")
    $readAfterRows = @(Get-OptionalProperty -Object $readAfterFadeSet -Name "executed_kernel_reply")
    $readAfterTools = Tool-Names -Rows $readAfterRows
    Assert-ToolPresent -Tools $readAfterTools -Aliases @("clip.fade.read", "clip_fade_read") -Label "clip.fade.read after fade set"
    Assert-ToolPresent -Tools $readAfterTools -Aliases @("clip.gain.read", "clip_gain_read") -Label "clip.gain.read after fade set"
    $fadeAfterResult = Find-ExecutedToolResult -Rows $readAfterRows -Aliases @("clip.fade.read", "clip_fade_read")
    Assert-NearNumber -Actual (Get-OptionalProperty -Object $fadeAfterResult -Name "fade_in_seconds") -Expected 0.15 -Label "fade_in_seconds after agent fade set"
    Assert-NearNumber -Actual (Get-OptionalProperty -Object $fadeAfterResult -Name "fade_out_seconds") -Expected 0.25 -Label "fade_out_seconds after agent fade set"

    $summary["clip_fade_gain_agent"] = [ordered]@{
        conversation_id = $clipConversationID
        selected_track_id = $track1ID
        selected_clip_id = $clip1ID
        pending_setup_needs_confirmation = $setNeedsConfirmation
        pending_setup_stop_reason = $setStop
        pending_setup_route = $setTools
        read_stop_reason = $readStop
        read_needs_confirmation = $readNeedsConfirmation
        read_route = $readTools
        fade_set_pending_route = $fadeSetTools
        fade_set_confirm_route = $fadeConfirmTools
        read_after_fade_set_route = $readAfterTools
        reply = $clipReply
        full_response_files = @("chat_clip_gain_pending.json", "chat_clip_fade_gain_read.json", "chat_clip_fade_set_pending.json", "chat_clip_fade_set_confirm.json", "chat_clip_fade_gain_read_after_set.json")
    }
    Write-Ok "clip fade/gain agent read cleared stale confirmation and fade set round-tripped through typed clip tools"

    if ($ClipFadeGainAgentOnly) {
        $summary["conversation_id"] = $clipConversationID
        $summary["tool_route"] = $readTools
        $summary["status"] = "passed"
        Write-Ok "focused clip fade/gain agent product-path smoke passed"
        return
    }

    Write-Step "Ask product-path mix question through agent HTTP"
    $conversationID = "product_path_mix_" + $Stamp
    $summary["conversation_id"] = $conversationID
    $observeMessage = Join-UnicodeChars @(0x5E2E, 0x6211, 0x770B, 0x6574, 0x4F53, 0x6DF7, 0x97F3)
    $observe = Invoke-AgentChat -ConversationID $conversationID -Message $observeMessage
    ConvertTo-JsonFile -Value $observe -Path (Join-Path $ArtifactDir "chat_observe.json")
    $summary["stop_reasons"]["observe"] = [string](Get-OptionalProperty -Object $observe -Name "stop_reason")
    if ([string]$observe.stop_reason -ne "done") {
        Fail ("observe turn stop_reason=" + [string]$observe.stop_reason)
    }
    $events = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/events?conversation_id=" + [uri]::EscapeDataString($conversationID) + "&since=0&limit=20") -TimeoutSec 10
    ConvertTo-JsonFile -Value $events -Path (Join-Path $ArtifactDir "events_after_observe.json")
    $observeEventSeq = [int64](Get-OptionalProperty -Object $events -Name "next_seq")
    $observePendingEvents = @()
    foreach ($event in @($events.events)) {
        if ([string]$event.type -eq "mix_tick.pending") {
            $observePendingEvents += $event
        }
    }
    $observeNeedsConfirmation = [bool](Get-OptionalProperty -Object $observe -Name "needs_confirmation")
    $summary["read_only_observation"] = [ordered]@{
        message = $observeMessage
        stop_reason = [string]$observe.stop_reason
        needs_confirmation = $observeNeedsConfirmation
        pending_event_count = $observePendingEvents.Count
    }
    if ($observePendingEvents.Count -ne 0) {
        Fail "read-only product observe emitted mix_tick.pending"
    }
    if ($observeNeedsConfirmation) {
        Fail "read-only product observe requested confirmation"
    }
	Write-Ok "product observe stayed read-only with no pending event or confirmation"
	Assert-ResponseAcousticBridgeReadiness -Response $observe -Label "product observe"
	$summary["snapshot_paths"]["after_observe"] = Assert-AuthoritativeFeatureSnapshotPath -RepoRoot $RepoRoot -GodotProjectRoot $GodotProjectRoot -ArtifactDir $ArtifactDir -RunStartedAt $RunStartedAt

	Write-Step "Chinese multitrack MOM observation scope"
	$multitrackConversationID = "product_path_multitrack_" + $Stamp
	$multitrackMessage = Join-UnicodeChars @(0x6BD4, 0x8F83, 0x4E00, 0x4E0B, 0x5404, 0x8F68, 0x9891, 0x6BB5, 0x5360, 0x7528, 0x548C, 0x58F0, 0x50CF, 0x5173, 0x7CFB, 0xFF0C, 0x4E0D, 0x8981, 0x4FEE, 0x6539, 0x3002)
	$multitrack = Invoke-AgentChat -ConversationID $multitrackConversationID -Message $multitrackMessage
	ConvertTo-JsonFile -Value $multitrack -Path (Join-Path $ArtifactDir "chat_multitrack_observe.json")
	$summary["stop_reasons"]["multitrack_observe"] = [string](Get-OptionalProperty -Object $multitrack -Name "stop_reason")
	if ([string]$multitrack.stop_reason -ne "done") {
		Fail ("multitrack observe turn stop_reason=" + [string]$multitrack.stop_reason)
	}
	Assert-ResponseAcousticBridgeReadiness -Response $multitrack -Label "product Chinese multitrack observe"
	Assert-ResponseMOMMultitrackObservation -Response $multitrack -Label "product Chinese multitrack observe" -ExpectedGoalText $multitrackMessage
	$multitrackEvents = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/events?conversation_id=" + [uri]::EscapeDataString($multitrackConversationID) + "&since=0&limit=20") -TimeoutSec 10
	ConvertTo-JsonFile -Value $multitrackEvents -Path (Join-Path $ArtifactDir "events_after_multitrack_observe.json")
	$multitrackPendingEvents = @()
	foreach ($event in @($multitrackEvents.events)) {
		if ([string]$event.type -eq "mix_tick.pending") {
			$multitrackPendingEvents += $event
		}
	}
	$multitrackNeedsConfirmation = [bool](Get-OptionalProperty -Object $multitrack -Name "needs_confirmation")
	$summary["read_only_multitrack_observation"] = [ordered]@{
		conversation_id = $multitrackConversationID
		message = $multitrackMessage
		stop_reason = [string]$multitrack.stop_reason
		needs_confirmation = $multitrackNeedsConfirmation
		pending_event_count = $multitrackPendingEvents.Count
	}
	if ($multitrackPendingEvents.Count -ne 0) {
		Fail "read-only multitrack observe emitted mix_tick.pending"
	}
	if ($multitrackNeedsConfirmation) {
		Fail "read-only multitrack observe requested confirmation"
	}
	Write-Ok "Chinese multitrack observe used project MOM scope with no pending event or confirmation"

    $confirmMessage = Join-UnicodeChars @(0x53EF, 0x4EE5, 0x6267, 0x884C)
	if ($null -ne $summary["pending_candidate"]) {
		Write-Step "Confirm pending mix tick"
		$confirm = Invoke-AgentChat -ConversationID $conversationID -Message $confirmMessage
		ConvertTo-JsonFile -Value $confirm -Path (Join-Path $ArtifactDir "chat_confirm.json")
		$summary["stop_reasons"]["confirm"] = [string](Get-OptionalProperty -Object $confirm -Name "stop_reason")
		if ([string]$confirm.stop_reason -ne "mix_tick_applied_reobserved") {
			Fail ("confirm turn stop_reason=" + [string]$confirm.stop_reason)
		}
		$confirmRows = @(Get-OptionalProperty -Object $confirm -Name "executed_kernel_reply")
		$tools = Tool-Names -Rows $confirmRows
		$summary["tool_route"] = $tools
		Assert-ToolPresent -Tools $tools -Aliases @("mix.propose_tick", "mix_propose_tick") -Label "mix.propose_tick"
		Assert-ToolPresent -Tools $tools -Aliases @("mix.apply_tick", "mix_apply_tick") -Label "mix.apply_tick"
		Assert-ToolPresent -Tools $tools -Aliases @("mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation") -Label "reobserve"
		Assert-ToolAbsent -Tools $tools -Aliases @("daw.invoke", "daw_invoke") -Label "daw.invoke"
		Assert-ToolAbsent -Tools $tools -Aliases @("track.volume", "track_volume") -Label "track.volume"
		$toolCounts = [ordered]@{
			propose = Count-ExecutedToolGroup -Rows $confirmRows -Aliases @("mix.propose_tick", "mix_propose_tick")
			apply = Count-ExecutedToolGroup -Rows $confirmRows -Aliases @("mix.apply_tick", "mix_apply_tick")
			observe = Count-ExecutedToolGroup -Rows $confirmRows -Aliases @("mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation")
			daw_invoke = Count-ExecutedToolGroup -Rows $confirmRows -Aliases @("daw.invoke", "daw_invoke")
			track_volume = Count-ExecutedToolGroup -Rows $confirmRows -Aliases @("track.volume", "track_volume")
		}
		if ([int]$toolCounts.propose -ne 1 -or [int]$toolCounts.apply -ne 1 -or [int]$toolCounts.observe -ne 1) {
			Fail ("Expected exactly one propose/apply/reobserve in confirmation turn. counts=" + ($toolCounts | ConvertTo-Json -Compress))
		}
		if ([int]$toolCounts.daw_invoke -ne 0 -or [int]$toolCounts.track_volume -ne 0) {
			Fail ("Confirmation bypassed typed mix tools. counts=" + ($toolCounts | ConvertTo-Json -Compress))
		}
		[void](Wait-LogPattern -LogPath $AgentLog -Pattern ("[mix.tick.pending] explicit confirmation routed conversation=" + $conversationID) -TimeoutSeconds 15)
		[void](Wait-LogPattern -LogPath $AgentLog -Pattern ("[mix.tick.pending] applied and reobserved conversation=" + $conversationID) -TimeoutSeconds 15)
		$confirmReply = [string](Get-OptionalProperty -Object $confirm -Name "reply")
		if ($confirmReply -notmatch "AB Result") {
			Fail ("confirmation reply did not include AB Result: " + $confirmReply)
		}
		$summary["before_after"] = Compact-BeforeAfter -Confirm $confirm
		Assert-ResponseAcousticBridgeReadiness -Response $confirm -Label "product confirm reobserve"
		$summary["snapshot_paths"]["after_confirm"] = Assert-AuthoritativeFeatureSnapshotPath -RepoRoot $RepoRoot -GodotProjectRoot $GodotProjectRoot -ArtifactDir $ArtifactDir -RunStartedAt $RunStartedAt
		$eventsAfterConfirm = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/events?conversation_id=" + [uri]::EscapeDataString($conversationID) + "&since=" + $observeEventSeq + "&limit=40") -TimeoutSec 10
		ConvertTo-JsonFile -Value $eventsAfterConfirm -Path (Join-Path $ArtifactDir "events_after_confirm.json")
		$postConfirmPendingEvents = @()
		foreach ($event in @($eventsAfterConfirm.events)) {
			if ([string]$event.type -eq "mix_tick.pending") {
				$postConfirmPendingEvents += $event
			}
		}
		$postConfirmPendingCandidate = $null
		if ($postConfirmPendingEvents.Count -gt 0) {
			$postConfirmPendingCandidate = $postConfirmPendingEvents[$postConfirmPendingEvents.Count - 1].payload
		}
		$autoSecondApplyDetected = ([int]$toolCounts.propose -gt 1 -or [int]$toolCounts.apply -gt 1)
		$summary["mix_loop_v1"] = [ordered]@{
			tool_counts = $toolCounts
			post_confirm_pending_event_count = $postConfirmPendingEvents.Count
			post_confirm_pending_candidate = $postConfirmPendingCandidate
			next_pending_requires_confirmation = ($null -ne $postConfirmPendingCandidate)
			auto_second_apply_detected = $autoSecondApplyDetected
		}
		if ($autoSecondApplyDetected) {
			Fail "Mix Loop v1 auto-executed more than one tick in a single confirmation turn"
		}
	}

    Write-Step "No-pending confirmation guard"
    $guardConversationID = "product_path_no_pending_" + $Stamp
    $guard = Invoke-AgentChat -ConversationID $guardConversationID -Message $confirmMessage
    ConvertTo-JsonFile -Value $guard -Path (Join-Path $ArtifactDir "chat_no_pending.json")
    $summary["stop_reasons"]["no_pending"] = [string](Get-OptionalProperty -Object $guard -Name "stop_reason")
    if ([string]$guard.stop_reason -ne "no_pending_mix_tick_candidate") {
        Fail ("no-pending guard stop_reason=" + [string]$guard.stop_reason)
    }

    Write-Step "Vocal focus relationship observation"
    $focusConversationID = "product_path_vocal_focus_" + $Stamp
    $focusMessage = "make the lead vocal more forward"
    $focus = Invoke-AgentChat -ConversationID $focusConversationID -Message $focusMessage
    ConvertTo-JsonFile -Value $focus -Path (Join-Path $ArtifactDir "chat_vocal_focus.json")
    $summary["stop_reasons"]["vocal_focus"] = [string](Get-OptionalProperty -Object $focus -Name "stop_reason")
    $focusStopReason = [string]$focus.stop_reason
    $acceptedFocusStopReason = @("done", "needs_clarification", "needs_confirmation") -contains $focusStopReason
    if (-not $acceptedFocusStopReason) {
        Fail ("vocal focus turn stop_reason=" + [string]$focus.stop_reason)
    }
    $focusPendingCandidateCount = 0
    foreach ($typedEvent in @($focus.typed_events)) {
        if ([string]$typedEvent.event_type -eq "PendingCandidate") {
            $focusPendingCandidateCount += 1
        }
    }
    $focusRows = @(Get-OptionalProperty -Object $focus -Name "executed_kernel_reply")
    $focusTools = Tool-Names -Rows $focusRows
    $focusCounts = [ordered]@{
        observe = Count-ExecutedToolGroup -Rows $focusRows -Aliases @("mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation")
        derive = Count-ExecutedToolGroup -Rows $focusRows -Aliases @("mix.derive", "mix_derive")
        propose = Count-ExecutedToolGroup -Rows $focusRows -Aliases @("mix.propose_tick", "mix_propose_tick")
        apply = Count-ExecutedToolGroup -Rows $focusRows -Aliases @("mix.apply_tick", "mix_apply_tick")
        daw_invoke = Count-ExecutedToolGroup -Rows $focusRows -Aliases @("daw.invoke", "daw_invoke")
        track_volume = Count-ExecutedToolGroup -Rows $focusRows -Aliases @("track.volume", "track_volume")
    }
    if ([int]$focusCounts.observe -lt 1) {
        Fail ("Expected vocal focus route to include mix.observe. route=" + ($focusTools -join " -> "))
    }
    if ($focusStopReason -eq "needs_confirmation") {
        if (-not [bool](Get-OptionalProperty -Object $focus -Name "needs_confirmation")) {
            Fail "Vocal focus returned needs_confirmation stop reason without needs_confirmation=true"
        }
        if ($focusPendingCandidateCount -lt 1) {
            Fail "Vocal focus needs_confirmation did not include a PendingCandidate typed event"
        }
        if ([int]$focusCounts.derive -lt 1) {
            Fail ("Expected vocal focus pending route to include mix.derive. route=" + ($focusTools -join " -> "))
        }
    }
    if ($focusStopReason -eq "done" -and [int]$focusCounts.derive -lt 1) {
        $focusReply = [string](Get-OptionalProperty -Object $focus -Name "reply")
        $safeNoopDone = ($focusReply -match "can't|cannot|not reliably|No mix action|no mix action|not safe|not identified|partial")
        if (-not $safeNoopDone) {
            Fail ("Expected completed vocal focus route to include mix.derive unless it is an explicit no-op/clarification reply. route=" + ($focusTools -join " -> "))
        }
    }
    if ([int]$focusCounts.apply -ne 0 -or [int]$focusCounts.daw_invoke -ne 0 -or [int]$focusCounts.track_volume -ne 0) {
        Fail ("Vocal focus route mutated the project unexpectedly. counts=" + ($focusCounts | ConvertTo-Json -Compress))
    }
    $summary["focus_relationship"] = [ordered]@{
        conversation_id = $focusConversationID
        message = $focusMessage
        stop_reason = $focusStopReason
        accepted_stop_reason = $acceptedFocusStopReason
        tool_route = $focusTools
        tool_counts = $focusCounts
        pending_candidate_count = $focusPendingCandidateCount
        full_response_file = "chat_vocal_focus.json"
    }

    Write-Step "Vocal clarification loop"
    Reset-FixtureProject
    $genericTrack1 = Invoke-AgentTool -Tool "track.add_audio" -ToolArgs @{ name = "Track 1" } -Confirmed $true
    Assert-StatusOk -Response $genericTrack1 -Label "track.add_audio generic Track 1"
    $genericTrack1ID = Resolve-TrackID -Response $genericTrack1
    $genericTrack2 = Invoke-AgentTool -Tool "track.add_audio" -ToolArgs @{ name = "Track 2" } -Confirmed $true
    Assert-StatusOk -Response $genericTrack2 -Label "track.add_audio generic Track 2"
    $genericTrack2ID = Resolve-TrackID -Response $genericTrack2
    if ([string]::IsNullOrWhiteSpace($genericTrack1ID) -or [string]::IsNullOrWhiteSpace($genericTrack2ID)) {
        Fail ("Could not resolve generic vocal clarification track IDs: " + $genericTrack1ID + " / " + $genericTrack2ID)
    }
    Assert-StatusOk -Response (Import-AudioFixture -TrackID $genericTrack1ID -FilePath $Track1Path) -Label "import generic Track 1"
    Assert-StatusOk -Response (Import-AudioFixture -TrackID $genericTrack2ID -FilePath $Track2Path) -Label "import generic Track 2"
    Start-Sleep -Milliseconds 750

    $clarifyConversationID = "product_path_vocal_clarify_" + $Stamp
    $vocalForwardMessage = Join-UnicodeChars @(0x8BA9, 0x4E3B, 0x5531, 0x66F4, 0x9760, 0x524D)
    $clarifyAsk = Invoke-AgentChat -ConversationID $clarifyConversationID -Message $vocalForwardMessage
    ConvertTo-JsonFile -Value $clarifyAsk -Path (Join-Path $ArtifactDir "chat_vocal_clarify_ask.json")
    $clarifyAskStop = [string](Get-OptionalProperty -Object $clarifyAsk -Name "stop_reason")
    if ($clarifyAskStop -ne "needs_clarification") {
        Fail ("vocal clarification ask stop_reason=" + $clarifyAskStop + " reply=" + [string](Get-OptionalProperty -Object $clarifyAsk -Name "reply"))
    }
    $clarifyAskReply = [string](Get-OptionalProperty -Object $clarifyAsk -Name "reply")
    $whichTrackText = Join-UnicodeChars @(0x54EA, 0x6761)
    $whichOneMeasureText = Join-UnicodeChars @(0x54EA, 0x4E00, 0x6761)
    $whichOneTrackText = Join-UnicodeChars @(0x54EA, 0x4E00, 0x8F68)
    if (($clarifyAskReply -notmatch $whichTrackText) -and ($clarifyAskReply -notmatch $whichOneMeasureText) -and ($clarifyAskReply -notmatch $whichOneTrackText) -and ($clarifyAskReply.ToLowerInvariant() -notmatch "which track")) {
        Fail ("vocal clarification ask did not ask which track is vocal: " + $clarifyAskReply)
    }
    $clarifyAskEvents = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/events?conversation_id=" + [uri]::EscapeDataString($clarifyConversationID) + "&since=0&limit=20") -TimeoutSec 10
    ConvertTo-JsonFile -Value $clarifyAskEvents -Path (Join-Path $ArtifactDir "events_vocal_clarify_after_ask.json")
    foreach ($event in @($clarifyAskEvents.events)) {
        if ([string]$event.type -eq "mix_tick.pending") {
            Fail "vocal clarification ask stored pending before the vocal track was identified"
        }
    }

    $vocalAnswerMessage = "Track 1 " + (Join-UnicodeChars @(0x662F, 0x4E3B, 0x5531))
    $clarifyAnswer = Invoke-AgentChat -ConversationID $clarifyConversationID -Message $vocalAnswerMessage
    ConvertTo-JsonFile -Value $clarifyAnswer -Path (Join-Path $ArtifactDir "chat_vocal_clarify_answer.json")
    $clarifyAnswerStop = [string](Get-OptionalProperty -Object $clarifyAnswer -Name "stop_reason")
    $acceptedClarifyAnswerStop = @("done", "needs_confirmation") -contains $clarifyAnswerStop
    if (-not $acceptedClarifyAnswerStop) {
        Fail ("vocal clarification answer stop_reason=" + $clarifyAnswerStop + " reply=" + [string](Get-OptionalProperty -Object $clarifyAnswer -Name "reply"))
    }
    if ($clarifyAnswerStop -eq "needs_confirmation" -and -not [bool](Get-OptionalProperty -Object $clarifyAnswer -Name "needs_confirmation")) {
        Fail "vocal clarification answer returned needs_confirmation stop reason without needs_confirmation=true"
    }
    [void](Wait-LogPattern -LogPath $AgentLog -Pattern ("[mix.tick.pending] stored conversation=" + $clarifyConversationID) -TimeoutSeconds 15)
    $clarifyAnswerEvents = Invoke-Json -Method GET -Uri ($AgentHttp.TrimEnd("/") + "/agent/events?conversation_id=" + [uri]::EscapeDataString($clarifyConversationID) + "&since=0&limit=40") -TimeoutSec 10
    ConvertTo-JsonFile -Value $clarifyAnswerEvents -Path (Join-Path $ArtifactDir "events_vocal_clarify_after_answer.json")
    $clarifyPendingCandidate = $null
    foreach ($event in @($clarifyAnswerEvents.events)) {
        if ([string]$event.type -eq "mix_tick.pending") {
            $clarifyPendingCandidate = $event.payload
        }
    }
    if ($null -eq $clarifyPendingCandidate) {
        Fail "vocal clarification answer did not produce a pending mix tick"
    }

    $clarifyConfirm = Invoke-AgentChat -ConversationID $clarifyConversationID -Message $confirmMessage
    ConvertTo-JsonFile -Value $clarifyConfirm -Path (Join-Path $ArtifactDir "chat_vocal_clarify_confirm.json")
    $clarifyConfirmStop = [string](Get-OptionalProperty -Object $clarifyConfirm -Name "stop_reason")
    if ($clarifyConfirmStop -ne "mix_tick_applied_reobserved") {
        Fail ("vocal clarification confirm stop_reason=" + $clarifyConfirmStop)
    }
    $clarifyConfirmRows = @(Get-OptionalProperty -Object $clarifyConfirm -Name "executed_kernel_reply")
    $clarifyTools = Tool-Names -Rows $clarifyConfirmRows
    Assert-ToolPresent -Tools $clarifyTools -Aliases @("mix.propose_tick", "mix_propose_tick") -Label "vocal clarification mix.propose_tick"
    Assert-ToolPresent -Tools $clarifyTools -Aliases @("mix.apply_tick", "mix_apply_tick") -Label "vocal clarification mix.apply_tick"
    Assert-ToolPresent -Tools $clarifyTools -Aliases @("mix.observe", "mix_observe", "mix.request_observation", "mix_request_observation") -Label "vocal clarification reobserve"
    Assert-ToolAbsent -Tools $clarifyTools -Aliases @("daw.invoke", "daw_invoke") -Label "vocal clarification daw.invoke"
    Assert-ToolAbsent -Tools $clarifyTools -Aliases @("track.volume", "track_volume") -Label "vocal clarification track.volume"
    $clarifyConfirmReply = [string](Get-OptionalProperty -Object $clarifyConfirm -Name "reply")
    if ($clarifyConfirmReply -notmatch "AB Result") {
        Fail ("vocal clarification confirmation reply did not include AB Result: " + $clarifyConfirmReply)
    }
    $summary["stop_reasons"]["vocal_clarification_ask"] = $clarifyAskStop
    $summary["stop_reasons"]["vocal_clarification_answer"] = $clarifyAnswerStop
    $summary["stop_reasons"]["vocal_clarification_confirm"] = $clarifyConfirmStop
    $summary["vocal_clarification_loop"] = [ordered]@{
        conversation_id = $clarifyConversationID
        ask_message = $vocalForwardMessage
        answer_message = $vocalAnswerMessage
        ask_stop_reason = $clarifyAskStop
        answer_stop_reason = $clarifyAnswerStop
        confirm_stop_reason = $clarifyConfirmStop
        pending_candidate = $clarifyPendingCandidate
        tool_route = $clarifyTools
        before_after = Compact-BeforeAfter -Confirm $clarifyConfirm
        full_response_files = @("chat_vocal_clarify_ask.json", "chat_vocal_clarify_answer.json", "chat_vocal_clarify_confirm.json")
    }

    $summary["status"] = "passed"
    Write-Ok "product-path lifecycle + mix smoke passed"
}
catch {
    $summary["status"] = "failed"
    $summary["error"] = $_.Exception.Message
    Write-WarnLine ("failed: " + $_.Exception.Message)
    throw
}
finally {
    $summary["ports"] = @(Get-ListenersSnapshot)
    if (Test-Path -LiteralPath $AgentLog) {
        Get-Content -LiteralPath $AgentLog -Tail 120 -ErrorAction SilentlyContinue |
            Set-Content -LiteralPath (Join-Path $ArtifactDir "agent_log_tail.txt") -Encoding UTF8
    }
    ConvertTo-JsonFile -Value $summary -Path (Join-Path $ArtifactDir "summary.json")

    if (-not $KeepProcesses) {
        if ($null -ne $started.agent -and [bool]$started.agent.started) {
            Stop-ProcessByID -ProcessID ([int]$started.agent.pid)
        }
        if ($null -ne $started.hub -and [bool]$started.hub.started) {
            Stop-ProcessByID -ProcessID ([int]$started.hub.pid)
        }
        if ($null -ne $started.kernel -and [bool]$started.kernel.started) {
            Stop-ProcessByID -ProcessID ([int]$started.kernel.pid)
        }
        if ($null -ne $started.godot -and [bool]$started.godot.started) {
            Stop-ProcessByID -ProcessID ([int]$started.godot.pid)
        }
        if ($null -ne $started.ui_override -and [bool]$started.ui_override.started) {
            Stop-ProcessByID -ProcessID ([int]$started.ui_override.pid)
        }
    }
    $env:VIT_DAW_DEV_ROOT = $oldDevRoot
    $env:VIT_VSP_HUB_LAST_LOG_PATH = $oldVspHubLastLogPath
    $env:VIT_AGENT_VSP_HUB_URL = $oldAgentVspHubUrl
    $env:VIT_AGENT_LAST_LOG_PATH = $oldAgentLastLogPath
    $env:VIT_AGENT_KEEP_LAST_LOG_LINES = $oldAgentKeepLogLines
    $env:VIT_SKIP_DEV_AUTOSTART = $oldSkipDevAutostart
}

Write-Step "Summary"
Write-Host ("artifact_dir: " + $ArtifactDir)
Write-Host ("conversation_id: " + [string]($summary["conversation_id"]))
Write-Host ("observation_id: " + [string]($summary["observation_id"]))
Write-Host ("route: " + (($summary["tool_route"]) -join " -> "))
Write-Host ("interaction_path: " + [string]($summary["interaction_path"]))

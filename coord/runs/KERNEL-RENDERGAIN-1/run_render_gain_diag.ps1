# run_render_gain_diag.ps1 — KERNEL-RENDERGAIN-1 (132) kernel-stack berth runner.
#
# Same berth discipline as scripts/dev_agent_smoke.ps1 (unchanged production script):
#   - one stack owner at a time: refuse if any stack port/process is already live;
#   - run-dir kernel workspace (Settings/Logs/default project copied into the run dir),
#     VIT_PROJECT_XML redirected to the run dir so nothing under VitApp\Workspace is
#     written by this run (journey-berth isolation contract, dev_agent_smoke.ps1:981-1020);
#   - kernel only (no Godot UI, no agent): this leg drives the kernel ZMQ command face
#     directly, so the agent HTTP/LLM faces are not part of the experiment;
#   - teardown in finally: stop only the pid this run started, then re-verify ports/processes.
# New script (the card allows it); existing scripts are not modified.
#
# Usage:
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\coord\runs\KERNEL-RENDERGAIN-1\run_render_gain_diag.ps1 `
#       -RepoRoot D:\Vit_DAW_wt_rendergain1 `
#       -KernelExe D:\Vit_DAW\Export\staging\runtime\VitApp.exe `
#       -RunDir D:\Vit_DAW_wt_rendergain1\coord\runs\KERNEL-RENDERGAIN-1\run_20261018_HHmmss

[CmdletBinding()]
param(
    [string]$RepoRoot = "D:\Vit_DAW_wt_rendergain1",
    [string]$KernelExe = "D:\Vit_DAW\Export\staging\runtime\VitApp.exe",
    [string]$SettingsSource = "D:\Vit_DAW\Export\staging\runtime\Workspace\Settings",
    [string]$RunDir = "",
    [string]$Python = "python",
    [int]$KernelStartTimeoutSeconds = 180
)

$ErrorActionPreference = "Stop"

$stackPorts = @(5555, 5556, 5557, 7878, 8787, 4445)

function Write-Step([string]$text) { Write-Host ("`n== " + $text) }
function Write-Ok([string]$text) { Write-Host ("ok: " + $text) }
function Write-WarnLine([string]$text) { Write-Host ("warn: " + $text) -ForegroundColor Yellow }

function Get-StackListeners {
    $found = @()
    foreach ($port in $stackPorts) {
        try {
            $listeners = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction Stop
            foreach ($listener in $listeners) {
                $found += [pscustomobject]@{ port = $port; pid = $listener.OwningProcess }
            }
        }
        catch { }
    }
    return $found
}

function Get-StackProcesses {
    # VitApp / VitAgent / vsphub are always stack blockers. Godot is split:
    # a running DAW frontend blocks the berth, while the user's Godot *editor*
    # (--editor, no stack ports) is a documented out-of-scope process
    # (PC-RUNTIME-STACK precedent: not the hand-test stack, zero port conflict).
    $rows = @()
    $cims = @(Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
            $_.Name -eq "VitApp.exe" -or $_.Name -eq "VitAgent.exe" -or $_.Name -eq "vsphub.exe" -or $_.Name -like "Godot*"
        })
    foreach ($c in $cims) {
        $rows += [pscustomobject]@{
            Id          = [int]$c.ProcessId
            ProcessName = $c.Name
            CommandLine = $c.CommandLine
            IsEditor    = ($c.Name -like "Godot*" -and $c.CommandLine -imatch "--editor")
        }
    }
    return @($rows | Sort-Object Id -Unique)
}

function Test-PortListening([int]$Port) {
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $async = $client.BeginConnect("127.0.0.1", $Port, $null, $null)
        if (-not $async.AsyncWaitHandle.WaitOne(500)) { return $false }
        $client.EndConnect($async)
        return $true
    }
    catch { return $false }
    finally { $client.Close() }
}

$exitCode = 0
$kernelProc = $null
$kernelPid = $null
$tornDown = $false

try {
    Write-Step "Preflight: ports and processes (AGENTS.md section 9 / PC-RUNTIME-STACK)"
    if (-not (Test-Path -LiteralPath $KernelExe)) { throw "kernel exe not found: $KernelExe" }
    $KernelExe = (Resolve-Path -LiteralPath $KernelExe).Path

    $listeners = Get-StackListeners
    if ($listeners.Count -gt 0) {
        $listeners | Format-Table -AutoSize | Out-String | Write-Host
        throw "stack ports already listening; another owner holds the stack (AGENTS.md section 9)"
    }
    $procs = Get-StackProcesses
    $blockingProcs = @($procs | Where-Object { -not $_.IsEditor })
    $editorProcs = @($procs | Where-Object { $_.IsEditor })
    if ($blockingProcs.Count -gt 0) {
        $blockingProcs | Select-Object Id, ProcessName, CommandLine | Format-Table -AutoSize | Out-String | Write-Host
        throw "stack processes already running; another owner holds the stack (AGENTS.md section 9)"
    }
    if ($editorProcs.Count -gt 0) {
        $editorProcs | Select-Object Id, ProcessName, CommandLine | Format-Table -AutoSize | Out-String | Write-Host
        Write-WarnLine ("user Godot editor present (out of scope: no stack ports, not the hand-test stack; PC-RUNTIME-STACK precedent): pid=" + (($editorProcs | ForEach-Object { $_.Id }) -join ","))
    }
    Write-Ok "ports 5555/5556/5557/7878/8787/4445 listening=0; VitApp/VitAgent/vsphub/Godot-frontend processes=0"

    if ([string]::IsNullOrWhiteSpace($RunDir)) {
        $RunDir = Join-Path $RepoRoot ("coord\runs\KERNEL-RENDERGAIN-1\run_" + (Get-Date -Format "yyyyMMdd_HHmmss"))
    }
    if (-not [System.IO.Path]::IsPathRooted($RunDir)) {
        $RunDir = [System.IO.Path]::GetFullPath((Join-Path (Get-Location).Path $RunDir))
    }
    if (Test-Path -LiteralPath $RunDir) { throw "run artifacts directory already exists; each run needs a fresh directory: $RunDir" }
    $KernelWorkspace = Join-Path $RunDir "kernel_workspace"
    foreach ($dir in @($RunDir, $KernelWorkspace, (Join-Path $KernelWorkspace "Settings"), (Join-Path $KernelWorkspace "Logs"))) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
    Write-Ok ("run dir: " + $RunDir)

    # ---- provenance -----------------------------------------------------
    $kernelSha = (Get-FileHash -LiteralPath $KernelExe -Algorithm SHA256).Hash
    $head = (& git -C $RepoRoot rev-parse HEAD 2>$null)
    $statusShort = (& git -C $RepoRoot status --short 2>$null | Out-String)
    [pscustomobject]@{
        started = (Get-Date).ToString("o")
        repo_root = $RepoRoot
        kernel_exe = $KernelExe
        kernel_sha256 = $kernelSha
        worktree_head = $head
        worktree_status_short = $statusShort
        python = $Python
        machine = $env:COMPUTERNAME
        stack_ports = $stackPorts -join ","
        out_of_scope_godot_editor_pids = (($editorProcs | ForEach-Object { $_.Id }) -join ",")
    } | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath (Join-Path $RunDir "run_meta.json") -Encoding UTF8
    Write-Ok ("kernel sha256: " + $kernelSha)
    Write-Ok ("worktree HEAD: " + $head)

    # ---- run-dir kernel workspace (journey-berth isolation) -------------
    Copy-Item -LiteralPath (Join-Path $RepoRoot "VitApp\Workspace\default_project.xml") `
              -Destination (Join-Path $RunDir "startup_default_project.xml") -Force
    if (Test-Path -LiteralPath $SettingsSource) {
        Get-ChildItem -LiteralPath $SettingsSource -File | ForEach-Object {
            Copy-Item -LiteralPath $_.FullName -Destination (Join-Path $KernelWorkspace "Settings" $_.Name) -Force
        }
        Write-Ok ("copied kernel Settings cache from " + $SettingsSource)
    }
    else {
        Write-WarnLine ("Settings source missing, kernel will cold-initialise: " + $SettingsSource)
    }

    # ---- start kernel (kernel-only leg; no agent, no UI) ----------------
    Write-Step "Start kernel"
    $kernelStdout = Join-Path $RunDir "kernel_stdout.log"
    $kernelStderr = Join-Path $RunDir "kernel_stderr.log"
    $priorProjectXml = [System.Environment]::GetEnvironmentVariable("VIT_PROJECT_XML", "Process")
    $env:VIT_PROJECT_XML = (Join-Path $RunDir "startup_default_project.xml")
    try {
        $kernelProc = Start-Process -FilePath $KernelExe -WorkingDirectory $KernelWorkspace `
            -WindowStyle Hidden -PassThru `
            -RedirectStandardOutput $kernelStdout -RedirectStandardError $kernelStderr
    }
    finally {
        if ([string]::IsNullOrWhiteSpace($priorProjectXml)) { Remove-Item Env:VIT_PROJECT_XML -ErrorAction SilentlyContinue }
        else { $env:VIT_PROJECT_XML = $priorProjectXml }
    }
    $kernelPid = $kernelProc.Id
    Write-Ok ("started kernel: " + $KernelExe + " pid=" + $kernelPid + " workdir=" + $KernelWorkspace)

    $deadline = (Get-Date).AddSeconds($KernelStartTimeoutSeconds)
    $ready = $false
    while ((Get-Date) -lt $deadline) {
        if ($kernelProc.HasExited) { throw ("kernel exited early with code " + $kernelProc.ExitCode + "; see " + $kernelStdout) }
        if (Test-PortListening 5555) { $ready = $true; break }
        Start-Sleep -Milliseconds 500
    }
    if (-not $ready) { throw ("kernel command port 5555 not listening within " + $KernelStartTimeoutSeconds + "s") }
    Write-Ok "kernel ZMQ command port 5555 listening"

    # ---- differential diagnostic --------------------------------------
    Write-Step "render gain differential diagnostic (python)"
    $diagScript = Join-Path $PSScriptRoot "render_gain_diag.py"
    if (-not (Test-Path -LiteralPath $diagScript)) { throw "diag script missing: $diagScript" }
    $summaryPath = Join-Path $RunDir "diag_summary.json"
    $consolePath = Join-Path $RunDir "diag_console.txt"
    & $Python $diagScript --url "tcp://127.0.0.1:5555" --output $summaryPath 2>&1 | Tee-Object -FilePath $consolePath
    $diagExit = $LASTEXITCODE
    Write-Ok ("diag exit code: " + $diagExit)
    if ($diagExit -ne 0) { $exitCode = 1 }
}
catch {
    Write-Host ("FAIL: " + $_.Exception.Message) -ForegroundColor Red
    $exitCode = 1
}
finally {
    Write-Step "Teardown (only the pid this run started)"
    if ($null -ne $kernelPid -and $kernelPid -gt 0) {
        try {
            $proc = Get-Process -Id $kernelPid -ErrorAction Stop
            Stop-Process -Id $kernelPid -Force -ErrorAction Stop
            $proc.WaitForExit(10000) | Out-Null
            Write-Ok ("stopped kernel pid=" + $kernelPid)
            $tornDown = $true
        }
        catch {
            Write-WarnLine ("could not stop kernel pid=" + $kernelPid + ": " + $_.Exception.Message)
        }
    }
    Start-Sleep -Seconds 2
    for ($i = 0; $i -lt 20; $i++) {
        if (-not (Test-PortListening 5555)) { break }
        Start-Sleep -Milliseconds 500
    }
    $remainingListeners = Get-StackListeners
    $remainingProcs = @(Get-StackProcesses | Where-Object { -not $_.IsEditor })
    $remainingEditors = @(Get-StackProcesses | Where-Object { $_.IsEditor })
    if ($remainingListeners.Count -gt 0) {
        $remainingListeners | Format-Table -AutoSize | Out-String | Write-Host
        Write-WarnLine "stack ports still listening after teardown"
        $exitCode = 1
    }
    else { Write-Ok "ports 5555/5556/5557/7878/8787/4445 listening=0 after teardown" }
    if ($remainingProcs.Count -gt 0) {
        $remainingProcs | Select-Object Id, ProcessName | Format-Table -AutoSize | Out-String | Write-Host
        Write-WarnLine "stack processes still present after teardown"
        $exitCode = 1
    }
    else {
        Write-Ok "VitApp/VitAgent/vsphub/Godot-frontend processes=0 after teardown"
        if ($remainingEditors.Count -gt 0) {
            Write-Ok ("user Godot editor still present (out of scope, untouched): pid=" + (($remainingEditors | ForEach-Object { $_.Id }) -join ","))
        }
    }
    if (-not [string]::IsNullOrWhiteSpace($RunDir) -and (Test-Path -LiteralPath $RunDir)) {
        [pscustomobject]@{
            finished = (Get-Date).ToString("o")
            kernel_pid = $kernelPid
            torn_down = $tornDown
            exit_code = $exitCode
            diag_summary = (Join-Path $RunDir "diag_summary.json")
            remaining_listeners = $remainingListeners.Count
            remaining_processes = $remainingProcs.Count
            out_of_scope_godot_editors = @($remainingEditors | ForEach-Object { $_.Id })
        } | ConvertTo-Json -Depth 3 | Set-Content -LiteralPath (Join-Path $RunDir "teardown.json") -Encoding UTF8
        Write-Ok ("artifacts: " + $RunDir)
    }
}

exit $exitCode

param([switch]$Teardown, [int[]]$KillPids = @())
$ErrorActionPreference = "Stop"
$RunDir = "D:\Vit_DAW\coord\runs\FE-SEND-UI-LIVE-CHAIN-1\sendui1_final_20261022_2210"
$KernelExe = "D:\Vit_DAW\VitApp\build\VitApp_artefacts\Release\VitApp.exe"
$HubExe = "D:\Vit_DAW\agent\bin\VspHub.exe"
$AgentExe = "D:\Vit_DAW\agent\bin\VitAgent.exe"
$GodotExe = "D:\Godot\Godot_v4.6.1-stable_win64_console.exe"
$GodotProject = "D:\Godot\project\project-vit-daw-frontend-senduichain1"
function Test-PortListen([int]$p) { return [bool](Get-NetTCPConnection -LocalPort $p -State Listen -ErrorAction SilentlyContinue) }
function Wait-Port([int]$p, [int]$sec) {
  $deadline = (Get-Date).AddSeconds($sec)
  while ((Get-Date) -lt $deadline) { if (Test-PortListen $p) { return $true }; Start-Sleep -Milliseconds 400 }
  return $false
}
if ($Teardown) {
  foreach ($pid_ in $KillPids) { try { Stop-Process -Id $pid_ -Force -ErrorAction Stop; Write-Host "killed $pid_" } catch { Write-Host "kill $pid_ failed/absent" } }
  Start-Sleep -Seconds 2
  foreach ($p in 5555,5556,7878,8787) { Write-Host ("port {0} listen={1}" -f $p, (Test-PortListen $p)) }
  Get-Process -Name VitApp,VitAgent,VspHub,Godot_v4.6.1-stable_win64_console -ErrorAction SilentlyContinue | Select-Object Id,ProcessName | Format-Table -AutoSize
  exit 0
}
if (Test-PortListen 5555) { throw "kernel port 5555 already listening - stack not idle" }
$env:VIT_PROJECT_XML = Join-Path $RunDir "default_project.xml"
$kernel = Start-Process -FilePath $KernelExe -WorkingDirectory (Split-Path $KernelExe) -WindowStyle Hidden -PassThru
if (-not (Wait-Port 5555 20)) { throw "kernel 5555 not up" }
Write-Host ("kernel pid=" + $kernel.Id)
if (-not (Test-PortListen 8787)) {
  $hub = Start-Process -FilePath $HubExe -WorkingDirectory (Split-Path $HubExe) -WindowStyle Hidden -PassThru
  if (-not (Wait-Port 8787 20)) { throw "hub 8787 not up" }
  Write-Host ("hub pid=" + $hub.Id)
} else { Write-Host "hub already up?" }
$agent = Start-Process -FilePath $AgentExe -WorkingDirectory (Split-Path $AgentExe) -WindowStyle Hidden -PassThru
if (-not (Wait-Port 7878 25)) { throw "agent 7878 not up" }
Write-Host ("agent pid=" + $agent.Id)
$env:VIT_SKIP_DEV_AUTOSTART = "1"
$env:VIT_SENDUI1_REPRO = "1"
$env:VIT_SENDUI1_OUT = Join-Path $RunDir "verdict.json"
$env:VIT_UI_LOG = "1"
$env:VIT_TRACK_SYNC_DIAG = "1"
$godotArgs = @("--path", $GodotProject, "--log-file", (Join-Path $RunDir "godot_repro.log"))
$godot = Start-Process -FilePath $GodotExe -ArgumentList $godotArgs -WorkingDirectory $GodotProject -PassThru
Write-Host ("godot pid=" + $godot.Id)
@{ kernel = $kernel.Id; agent = $agent.Id; godot = $godot.Id } | ConvertTo-Json | Set-Content (Join-Path $RunDir "pids.json")
Write-Host "BRINGUP_OK"

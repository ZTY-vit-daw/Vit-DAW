$ErrorActionPreference = "Stop"
$RunDir = "D:\Vit_DAW_wt_382reach\coord\runs\SCORE-ENTRY-REACH-1\forensic_zmq_20261116_2140"
$env:VIT_PROJECT_XML = Join-Path $RunDir "kernel_runtime\Workspace\default_project.xml"
$p = Start-Process -FilePath (Join-Path $RunDir "kernel_runtime\VitApp.exe") -WorkingDirectory (Join-Path $RunDir "kernel_runtime") -WindowStyle Hidden -PassThru
Write-Output ("KERNEL_PID=" + $p.Id)

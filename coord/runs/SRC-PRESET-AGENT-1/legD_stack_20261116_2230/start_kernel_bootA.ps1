$ErrorActionPreference = "Stop"
$RunDir = "D:\Vit_DAW_wt_presetagent\coord\runs\SRC-PRESET-AGENT-1\legD_stack_20261116_2230"
$env:VIT_PROJECT_XML = Join-Path $RunDir "kernel_runtime\Workspace\default_project.xml"
$env:VIT_DUMP_CLEAR_XML = "1"
$p = Start-Process -FilePath (Join-Path $RunDir "kernel_runtime\VitApp.exe") -WorkingDirectory (Join-Path $RunDir "kernel_runtime") -WindowStyle Hidden -PassThru
Write-Output ("KERNEL_PID=" + $p.Id)

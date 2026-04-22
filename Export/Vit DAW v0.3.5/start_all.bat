@echo off
setlocal
powershell.exe -WindowStyle Hidden -ExecutionPolicy Bypass -File "%~dp0start_all.ps1"
set ERR=%ERRORLEVEL%
if %ERR% neq 0 (
  powershell.exe -WindowStyle Hidden -ExecutionPolicy Bypass -Command "try { Add-Type -AssemblyName PresentationFramework; [System.Windows.MessageBox]::Show('Vit DAW could not start. Check runtime\bridge_stderr.log in the install folder.','Vit DAW','OK','Error') | Out-Null } catch { }"
)
exit /b %ERR%

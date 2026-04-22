@echo off
setlocal

set "ROOT=%~dp0"
set "KERNEL=%ROOT%VitApp.exe"
if not exist "%KERNEL%" set "KERNEL=%ROOT%runtime\VitApp.exe"
set "UI=%ROOT%Vit_DAW.exe"

if not exist "%KERNEL%" (
  echo [ERROR] VitApp.exe not found.
  echo Tried:
  echo   "%ROOT%VitApp.exe"
  echo   "%ROOT%runtime\VitApp.exe"
  pause
  exit /b 1
)

if not exist "%UI%" (
  echo [ERROR] Vit_DAW.exe not found: "%UI%"
  pause
  exit /b 1
)

start "" "%KERNEL%"
timeout /t 1 /nobreak >nul
start "" "%UI%"

exit /b 0

@echo off
setlocal
powershell -ExecutionPolicy Bypass -File "%~dp0start_all.ps1"
if errorlevel 1 (
  echo.
  echo Launch failed. Press any key to exit.
  pause >nul
)

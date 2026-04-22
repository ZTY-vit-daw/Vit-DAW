@echo off
setlocal
set "SCRIPT_DIR=%~dp0"
set "REPO_ROOT=%SCRIPT_DIR%.."
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%REPO_ROOT%\scripts\build_installer_v0.5.ps1"
if errorlevel 1 (
  echo.
  echo Build failed. Press any key to exit.
  pause >nul
  exit /b 1
)
echo.
echo Build complete. Press any key to exit.
pause >nul

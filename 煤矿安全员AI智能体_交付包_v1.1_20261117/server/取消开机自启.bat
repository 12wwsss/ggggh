@echo off
cd /d "%~dp0"
echo.
echo   Removing auto-start...
schtasks /Delete /TN "CoalMineAI" /F >nul 2>nul
del "%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\CoalMineAI.vbs" >nul 2>nul
echo   [OK] Removed
echo.
pause >nul

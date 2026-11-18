@echo off
cd /d "%~dp0"
echo.
echo   Installing auto-start for Coal Mine AI backend...
echo.

schtasks /Create /TN "CoalMineAI" /TR "wscript.exe \"%~dp0start-hidden.vbs\"" /SC ONLOGON /RL HIGHEST /F >nul 2>nul
if errorlevel 1 goto fallback

echo   [OK] Registered scheduled task "CoalMineAI" (runs at logon, no window)
goto startit

:fallback
echo   [!] Task Scheduler not available, using Startup folder instead
copy /Y "start-hidden.vbs" "%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup\CoalMineAI.vbs" >nul
if errorlevel 1 (
  echo   [X] Failed. Please run as administrator.
  pause
  exit /b 1
)
echo   [OK] Copied to Startup folder

:startit
echo.
echo   Starting now...
wscript.exe "%~dp0start-hidden.vbs"
timeout /t 3 /nobreak >nul
start "" explorer.exe http://127.0.0.1:8789/login
echo.
echo   Done. Backend will start automatically every time Windows boots.
echo   Stop: run  stop-service.bat
echo.
pause >nul

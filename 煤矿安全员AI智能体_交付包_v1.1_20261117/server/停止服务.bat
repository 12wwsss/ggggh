@echo off
cd /d "%~dp0"
echo.
echo   Stopping backend...
if not exist "data" mkdir "data"
echo stop > "data\stop.flag"
for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":8789" ^| findstr "LISTENING"') do taskkill /F /PID %%a >nul 2>nul
timeout /t 1 /nobreak >nul
echo   [OK] Stopped
echo.
pause >nul

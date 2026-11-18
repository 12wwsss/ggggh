@echo off
cd /d "%~dp0"
set "HERE=%~dp0"
set "NODE=node"
if exist "%ProgramFiles%\nodejs\node.exe" set "NODE=%ProgramFiles%\nodejs\node.exe"
set "URL=http://127.0.0.1:8789/login"

set "RUNNING=0"
for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":8789" ^| findstr "LISTENING"') do set "RUNNING=1"

if "%RUNNING%"=="1" goto open

echo.
echo   Backend not running, starting...
powershell -NoProfile -WindowStyle Hidden -Command "Start-Process -FilePath '%NODE%' -ArgumentList 'index.js' -WorkingDirectory '%HERE%' -WindowStyle Hidden"
timeout /t 4 /nobreak >nul

:open
echo   Opening: %URL%
start "" "%URL%"
timeout /t 1 /nobreak >nul

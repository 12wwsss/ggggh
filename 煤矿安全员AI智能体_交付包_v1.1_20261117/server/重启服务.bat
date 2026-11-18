@echo off
cd /d "%~dp0"
chcp 65001 >nul
set "HERE=%~dp0"
set "NODE=node"
if exist "%ProgramFiles%\nodejs\node.exe" set "NODE=%ProgramFiles%\nodejs\node.exe"

echo.
echo   [1/4] Killing old process on port 8789 ...
for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":8789" ^| findstr "LISTENING"') do (
  taskkill /F /PID %%a >nul 2>nul
)
timeout /t 1 /nobreak >nul

echo   [2/4] Killing leftover node.exe ...
taskkill /F /IM node.exe >nul 2>nul
timeout /t 2 /nobreak >nul

"%NODE%" --version >nul 2>nul
if errorlevel 1 (
  echo.
  echo   Node.js not found. Install Node.js 18+ from https://nodejs.org
  echo   Then run this script again.
  pause
  exit /b 1
)

echo   [3/4] Starting new backend ...
powershell -NoProfile -WindowStyle Hidden -Command "Start-Process -FilePath '%NODE%' -ArgumentList 'index.js' -WorkingDirectory '%HERE%' -WindowStyle Hidden"
timeout /t 4 /nobreak >nul

echo   [4/4] Checking version ...
echo.
set "OK=0"
for /f "delims=" %%v in ('curl -s http://127.0.0.1:8789/api/health') do (
  echo   %%v
  echo %%v | findstr "kbChunks" >nul && set "OK=1"
)
echo.
if "%OK%"=="1" (
  echo   ============================================
  echo    SUCCESS - backend is up to date
  echo   ============================================
  start "" explorer.exe http://127.0.0.1:8789/login
) else (
  echo   ============================================
  echo    FAILED - old process still running
  echo.
  echo    Please do this manually:
  echo     1^) Ctrl+Shift+Esc to open Task Manager
  echo     2^) Details tab - find node.exe - End task
  echo     3^) Run this script again
  echo   ============================================
)
echo.
pause >nul

@echo off
cd /d "%~dp0"
chcp 65001 >nul
set "HERE=%~dp0"
set "NODE=node"
if exist "%ProgramFiles%\nodejs\node.exe" set "NODE=%ProgramFiles%\nodejs\node.exe"

echo.
echo   ==========================================
echo    Coal Mine AI Agent - Backend
echo   ==========================================
echo.

for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":8789" ^| findstr "LISTENING"') do (
  taskkill /F /PID %%a >nul 2>nul
)
timeout /t 1 /nobreak >nul

"%NODE%" --version >nul 2>nul
if errorlevel 1 (
  echo.
  echo   Node.js not found. Install Node.js 18+ from https://nodejs.org
  echo   Then run this script again.
  pause
  exit /b 1
)

echo   Starting server on port 8789 ...
echo.

"%NODE%" index.js
echo.
echo   Server stopped. Press any key to close.
pause >nul

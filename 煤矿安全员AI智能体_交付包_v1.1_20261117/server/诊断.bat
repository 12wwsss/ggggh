@echo off
cd /d "%~dp0"
chcp 65001 >nul
echo.
echo   ===== Diagnostic =====
echo.
echo   Port 8789:
netstat -ano ^| findstr ":8789" ^| findstr "LISTENING"
echo.
echo   Health:
curl -s http://127.0.0.1:8789/api/health
echo.
echo.
echo   Bind test (expect 400, means interface exists):
curl -s -o nul -w "HTTP %%{http_code}" -X POST http://127.0.0.1:8789/api/bind -H "Content-Type: application/json" -d "{}"
echo.
echo.
echo   Files:
dir /b index.js
echo.
pause >nul

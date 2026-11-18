@echo off
cd /d "%~dp0"
chcp 65001 >nul

set "NODE=node"
where node >nul 2>nul
if not errorlevel 1 goto loop
  set "NODE=%BUNDLED%"
  goto loop
)
exit /b 1

:loop
if not exist "data" mkdir "data"
if exist "data\stop.flag" (
  del "data\stop.flag" >nul 2>nul
  echo [%date% %time%] stopped by flag>> "data\server.out.log"
  exit /b 0
)
echo [%date% %time%] starting>> "data\server.out.log"
"%NODE%" index.js >> "data\server.out.log" 2>&1
echo [%date% %time%] process exited, restart in 5s>> "data\server.out.log"
timeout /t 5 /nobreak >nul
goto loop

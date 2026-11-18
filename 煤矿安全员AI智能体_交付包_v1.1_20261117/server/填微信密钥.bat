@echo off
cd /d "%~dp0"
set "NODE=node"
if exist "%ProgramFiles%\nodejs\node.exe" set "NODE=%ProgramFiles%\nodejs\node.exe"

echo.
echo   ============================================
echo    WeChat login - fill appSecret
echo   ============================================
echo.
echo   Get it here:
echo   mp.weixin.qq.com - Develop - Development Settings
echo   - Mini Program Secret - Generate - copy the 32-char key
echo.
set /p SEC=" Paste appSecret then press Enter: "

if "%SEC%"=="" echo   Nothing entered, skipped. & timeout /t 3 >nul & exit /b

"%NODE%" -e "const fs=require('fs');const c=JSON.parse(fs.readFileSync('config.json','utf8'));c.appSecret=process.argv[1];fs.writeFileSync('config.json',JSON.stringify(c,null,2),'utf8');console.log('   appSecret saved, length=' + c.appSecret.length);" "%SEC%"
echo.
echo   Restart backend to take effect (run StartService bat)
echo.
pause >nul

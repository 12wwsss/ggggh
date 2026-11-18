@echo off
cd /d "%~dp0"
chcp 936 >nul
set "DESK=%USERPROFILE%\Desktop"
if not exist "%DESK%" set "DESK=%PUBLIC%\Desktop"
set "LNK=%DESK%\煤矿安全智能体.lnk"

powershell -NoProfile -Command "$w=New-Object -ComObject WScript.Shell; $s=$w.CreateShortcut('%LNK%'); $s.TargetPath='%~dp0打开后台.bat'; $s.WorkingDirectory='%~dp0'; $s.IconLocation='%SystemRoot%\System32\shell32.dll,13'; $s.Description='煤矿安全智能体 · 管理后台'; $s.Save()"

if exist "%LNK%" (
  echo.
  echo   OK - desktop shortcut created:
  echo   %LNK%
  goto done
)

set "URLF=%DESK%\煤矿安全智能体.url"
> "%URLF%" echo [InternetShortcut]
>> "%URLF%" echo URL=http://127.0.0.1:8789/login
>> "%URLF%" echo IconIndex=0
if exist "%URLF%" (echo   OK - fallback .url created) else (echo   FAILED - create shortcut manually)

:done
echo.
timeout /t 3 >nul

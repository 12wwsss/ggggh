@echo off
cd /d "%~dp0"
title Fill in DeepSeek API Key
chcp 65001 >nul

echo.
echo    Notepad will open config.json
echo.
echo    BEFORE :  "apiKey": "",
echo    AFTER  :  "apiKey": "sk-your-key-here",
echo.
echo    Tips:
echo      - keep the double quotes, no spaces around the key
echo      - the key starts with sk- and is shown only once
echo      - press Ctrl+S to save, then close notepad
echo      - restart the service afterwards (run the start script again)
echo.
pause
notepad "%~dp0config.json"

@echo off
REM Justmart print connector launcher (portable). Keeps the window open for logs.
cd /d "%~dp0"
justmart-connector.exe
echo.
echo Connector stopped. Press any key to close.
pause >nul

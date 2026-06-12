@echo off
REM Justmart print connector launcher. Keeps the window open so you can read logs.
cd /d "%~dp0"
justmart-connector.exe
echo.
echo Connector exited. Press any key to close.
pause >nul

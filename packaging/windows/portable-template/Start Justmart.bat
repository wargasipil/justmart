@echo off
REM Portable launcher: run Justmart from this folder (SQLite, no install).
cd /d "%~dp0"
echo ============================================================
echo  Justmart (portable, SQLite)
echo  A browser will open at http://localhost:__PORT__
echo  Keep this window open. Close it to stop Justmart.
echo ============================================================
REM Open the browser a few seconds after the server starts (detached).
start "" /min cmd /c "timeout /t 3 >nul & start "" http://localhost:__PORT__"
justmart.exe
echo.
echo Justmart has stopped. Press any key to close this window.
pause >nul

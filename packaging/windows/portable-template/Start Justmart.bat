@echo off
REM Portable launcher: run Justmart from this folder (SQLite, no install).
cd /d "%~dp0"
REM Apply a staged update (from Settings -> Updates "Update now"): Windows can't
REM overwrite a running exe, so the swap happens here, before launch. The old
REM exe is kept as justmart.exe.bak for manual rollback.
if exist "justmart.exe.new" (
  echo Applying downloaded update...
  if exist "justmart.exe" move /y "justmart.exe" "justmart.exe.bak" >nul
  move /y "justmart.exe.new" "justmart.exe" >nul
)
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

@echo off
REM Portable launcher: run Justmart from this folder (SQLite, no install).
cd /d "%~dp0"

REM Re-invoked detached (minimized) by the "start" below, to wait for the server
REM and open the browser. Checked before anything else so the helper copy does
REM not re-run the update swap. See :waitopen.
if /i "%~1"=="--wait-open" goto waitopen

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
echo  A browser will open at http://localhost:__PORT__ once ready
__LICENSE_LAUNCHER_NOTE__
echo  Keep this window open. Close it to stop Justmart.
echo ============================================================
start "" /min "%~f0" --wait-open
justmart.exe
echo.
echo Justmart has stopped. Press any key to close this window.
pause >nul
goto :eof

:waitopen
REM Open the browser only once Justmart actually answers.
REM
REM A fixed delay is wrong here. On the LICENSED build the first run stops at the
REM activation page and does NOT bind this port until a licence is accepted, so a
REM timed open lands on a dead port - while the activation page the app opens
REM itself is on a different, ephemeral port. Polling also fixes the same race on
REM the free build, where a slow first boot (migrations) can outlast any guess.
where curl.exe >nul 2>&1 || goto waitopen_delay
REM ~1 hour: long enough for someone to be talked through activation, bounded so
REM this helper window can never linger forever.
for /l %%i in (1,1,3600) do (
  REM -f so only a real 2xx counts: without it any stray server (or an error
  REM page from something else already on this port) reads as "ready".
  curl -sf -o nul --max-time 2 http://localhost:__PORT__/healthz && goto waitopen_now
  timeout /t 1 >nul
)
goto :eof

:waitopen_delay
REM No curl.exe (Windows older than 10 1803): fall back to the old fixed guess.
timeout /t 3 >nul

:waitopen_now
start "" http://localhost:__PORT__
goto :eof

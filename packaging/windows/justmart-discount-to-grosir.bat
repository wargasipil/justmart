@echo off
REM Justmart: convert qty-gated product discounts into grosir (wholesale) price
REM tiers. A discount subtracts an amount from the line; a tier REPLACES the unit
REM price once the quantity is reached. Converting is a one-way change: the old
REM discount is deleted.
REM
REM DRY RUN BY DEFAULT. Nothing is written unless you pass "apply". Every run
REM writes a CSV report - converted rows AND skipped rows with the reason - into
REM the reports\ folder, so run it once without "apply", open the CSV, and only
REM then re-run with "apply".
REM
REM Usage:
REM   justmart-discount-to-grosir.bat                  preview + CSV report
REM   justmart-discount-to-grosir.bat apply            perform it (asks first)
REM   justmart-discount-to-grosir.bat linefixed        also convert whole-line
REM                                                    FIXED discounts (the
REM                                                    amount is divided by the
REM                                                    quantity threshold)
REM   justmart-discount-to-grosir.bat apply linefixed  both
REM   ... nopause                                      no "press any key" at the
REM                                                    end (Task Scheduler)

setlocal

REM --- Locate justmart.exe -----------------------------------------------------
REM Works for BOTH Windows flavors: portable (this script sits next to the exe)
REM and installed (this script lives in {app}\scripts\, exe one level up).
set "EXE="
if exist "%~dp0justmart.exe"      set "EXE=%~dp0justmart.exe"
if not defined EXE if exist "%~dp0..\justmart.exe" set "EXE=%~dp0..\justmart.exe"
if not defined EXE if exist "%ProgramFiles%\Justmart\justmart.exe" set "EXE=%ProgramFiles%\Justmart\justmart.exe"
if not defined EXE (
  echo ERROR: justmart.exe not found next to this script, one folder up, or in
  echo        "%ProgramFiles%\Justmart".
  goto fail
)

REM --- Locate config.yaml ------------------------------------------------------
REM Portable keeps it beside the exe; the installer puts it in ProgramData. The
REM report goes next to whichever config we end up using, so the two flavors
REM never write into each other's folders.
set "CFG="
if exist "%~dp0config.yaml"       set "CFG=%~dp0config.yaml"
if not defined CFG if exist "%~dp0..\config.yaml" set "CFG=%~dp0..\config.yaml"
if not defined CFG if exist "%ProgramData%\Justmart\config.yaml" set "CFG=%ProgramData%\Justmart\config.yaml"
if not defined CFG (
  echo ERROR: config.yaml not found next to this script, one folder up, or in
  echo        "%ProgramData%\Justmart".
  goto fail
)
for %%F in ("%CFG%") do set "DATADIR=%%~dpF"
set "REPORTS=%DATADIR%reports"
if not exist "%REPORTS%" mkdir "%REPORTS%"

REM --- Arguments ---------------------------------------------------------------
set "APPLY="
set "LINEFIXED="
set "NOPAUSE="
:parseargs
if "%~1"=="" goto parsed
if /i "%~1"=="apply"                set "APPLY=--apply"
if /i "%~1"=="--apply"              set "APPLY=--apply"
if /i "%~1"=="linefixed"            set "LINEFIXED=--include-line-fixed"
if /i "%~1"=="--include-line-fixed" set "LINEFIXED=--include-line-fixed"
if /i "%~1"=="nopause"              set "NOPAUSE=1"
shift
goto parseargs
:parsed

REM Timestamp for the report filename. PowerShell rather than wmic: wmic is
REM removed on recent Windows 11 builds, PowerShell is always there.
for /f %%T in ('powershell -NoProfile -Command "Get-Date -Format yyyyMMdd_HHmmss"') do set "STAMP=%%T"
set "REPORT=%REPORTS%\discount-to-grosir_%STAMP%.csv"

REM --- Confirm a destructive run ----------------------------------------------
REM Not inside an if-block: a %VAR% read in the same parenthesized block as the
REM set /p that fills it expands to the OLD value, so the check would never fire.
if not defined APPLY goto run
echo.
echo   WARNING - this will DELETE every discount it converts.
echo   Run this script with no arguments first, open the CSV report, and check
echo   the conversions before continuing.
echo.
set "CONFIRM="
set /p "CONFIRM=Type YES to continue: "
if /i not "%CONFIRM%"=="YES" goto cancelled

:run
echo.
set "JUSTMART_CONFIG=%CFG%"
"%EXE%" discount-to-grosir %APPLY% %LINEFIXED% --csv "%REPORT%"
if errorlevel 1 goto fail
echo.
echo Report: %REPORT%
if not defined APPLY echo Nothing was changed. Re-run with "apply" to perform the conversion.
goto end

:cancelled
echo Cancelled - nothing was changed.
goto end

:fail
echo.
echo FAILED - nothing was changed.
if not defined NOPAUSE pause
endlocal
exit /b 1

:end
if not defined NOPAUSE (
  echo.
  pause
)
endlocal

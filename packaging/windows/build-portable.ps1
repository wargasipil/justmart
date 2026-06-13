<#
  Builds the PORTABLE Justmart distribution (SQLite, no installer).

  Output: a self-contained folder + zip under dist\ that an end user can unzip
  and run with no PostgreSQL, no services, and no install step:

    dist\justmart-portable-<version>\
      justmart.exe          (SPA + migrations embedded)
      config.yaml           (SQLite; host 127.0.0.1; generated JWT secret)
      Start Justmart.bat    (launcher: runs the exe, opens the browser)
      README.txt
    dist\justmart-portable-<version>.zip

  Prerequisites:
    - Go 1.25+ and Node 20+ to build justmart.exe, OR pass -SkipExeBuild when a
      prebuilt dist\justmart.exe already exists.
    - No Inno Setup, no internet access required.

  Usage:
    powershell -ExecutionPolicy Bypass -File packaging\windows\build-portable.ps1 `
      -AppVersion 0.1.0 -Port 8080

  NOTE: keep this file ASCII-only (Windows PowerShell 5.1 reads -File as Windows-1252).
#>
[CmdletBinding()]
param(
  [string] $AppVersion    = "0.1.0",
  [int]    $Port          = 8080,
  [string] $OwnerEmail    = "owner@justmart.local",
  [string] $OwnerPassword = "change-me-now",
  [switch] $SkipExeBuild
)

$ErrorActionPreference = "Stop"
$here     = $PSScriptRoot
$root     = (Resolve-Path (Join-Path $here "..\..")).Path
$dist     = Join-Path $root "dist"
$template = Join-Path $here "portable-template"
$embedDir = Join-Path $root "backend\internal\web\dist"

New-Item -ItemType Directory -Force -Path $dist | Out-Null

# --- 1. Build justmart.exe (SPA + migrations embedded) ------------------------
$exe = Join-Path $dist "justmart.exe"
if ($SkipExeBuild -and (Test-Path $exe)) {
  Write-Host "Reusing existing $exe (-SkipExeBuild)."
} else {
  Write-Host "Building frontend + Windows binary..."
  # NOTE: $ErrorActionPreference="Stop" only catches cmdlet errors, NOT native-exe
  # exit codes. Check $LASTEXITCODE after each tool so a failed build can never be
  # silently packaged as if it succeeded.
  Push-Location (Join-Path $root "frontend")
  npm ci
  if ($LASTEXITCODE -ne 0) { Pop-Location; throw "npm ci failed (exit $LASTEXITCODE). If this is an EPERM unlink on a rollup *.node file, stop any running 'make web' / Vite dev server first -- it locks node_modules on Windows." }
  npm run build
  if ($LASTEXITCODE -ne 0) { Pop-Location; throw "npm run build failed (exit $LASTEXITCODE)." }
  Pop-Location
  Remove-Item -Recurse -Force (Join-Path $embedDir "assets") -ErrorAction SilentlyContinue
  Copy-Item -Recurse -Force (Join-Path $root "frontend\dist\*") $embedDir
  Push-Location (Join-Path $root "backend")
  $env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
  go build -ldflags "-s -w" -o $exe ./cmd/server
  $goExit = $LASTEXITCODE
  Remove-Item Env:\GOOS, Env:\GOARCH, Env:\CGO_ENABLED
  Pop-Location
  if ($goExit -ne 0) { throw "go build failed (exit $goExit)." }
}
if (-not (Test-Path $exe)) { throw "justmart.exe not found at $exe" }

# --- 1b. Build the print connector (Windows-only spooler dep, isolated) -------
# Separate exe shipped in connector\; the shop runs it to print to a USB / local
# printer. CGO off + GOOS=windows like the server (the alexbrainman/printer dep
# is behind //go:build windows, so this is the only target that links it).
$connExe = Join-Path $dist "justmart-connector.exe"
if (-not ($SkipExeBuild -and (Test-Path $connExe))) {
  Write-Host "Building print connector..."
  Push-Location (Join-Path $root "backend")
  $env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
  go build -ldflags "-s -w" -o $connExe ./cmd/connector
  $connExit = $LASTEXITCODE
  Remove-Item Env:\GOOS, Env:\GOARCH, Env:\CGO_ENABLED
  Pop-Location
  if ($connExit -ne 0) { throw "connector build failed (exit $connExit)." }
}
if (-not (Test-Path $connExe)) { throw "justmart-connector.exe not found at $connExe" }

# --- 2. Generate a per-build JWT secret (32 random bytes, hex) ----------------
function New-Secret([int]$bytes = 32) {
  $b = New-Object byte[] $bytes
  [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b)
  -join ($b | ForEach-Object { $_.ToString("x2") })
}
$jwt = New-Secret 32

# --- 3. Assemble the portable folder -----------------------------------------
$outName = "justmart-portable-$AppVersion"
$outDir  = Join-Path $dist $outName
Remove-Item -Recurse -Force $outDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

Copy-Item $exe (Join-Path $outDir "justmart.exe")

# Render each template with literal placeholder substitution, write as ASCII (no BOM).
function Write-Rendered([string]$srcName, [string]$dstName) {
  $text = Get-Content -Raw (Join-Path $template $srcName)
  $text = $text.Replace('__PORT__', "$Port").
                Replace('__JWT_SECRET__', $jwt).
                Replace('__OWNER_EMAIL__', $OwnerEmail).
                Replace('__OWNER_PASSWORD__', $OwnerPassword)
  Set-Content -Path (Join-Path $outDir $dstName) -Value $text -Encoding ascii -NoNewline
}
Write-Rendered "config.yaml"        "config.yaml"
Write-Rendered "Start Justmart.bat" "Start Justmart.bat"
Write-Rendered "README.txt"         "README.txt"

# Connector subfolder: exe + config + launcher + setup tutorial.
$connDir = Join-Path $outDir "connector"
New-Item -ItemType Directory -Force -Path $connDir | Out-Null
Copy-Item $connExe (Join-Path $connDir "justmart-connector.exe")
Write-Rendered "connector-config.yaml" "connector\config.yaml"
Copy-Item (Join-Path $template "Start Connector.bat") (Join-Path $connDir "Start Connector.bat")
Copy-Item (Join-Path $template "CONNECTOR-SETUP.txt")  (Join-Path $connDir "CONNECTOR-SETUP.txt")

# --- 4. Zip for distribution -------------------------------------------------
$zip = Join-Path $dist "$outName.zip"
Remove-Item -Force $zip -ErrorAction SilentlyContinue
Compress-Archive -Path $outDir -DestinationPath $zip -Force

Write-Host ""
Write-Host "Portable build complete:"
Write-Host "  folder: $outDir"
Write-Host "  zip:    $zip"
Write-Host "  login:  $OwnerEmail / $OwnerPassword   (port $Port)"
Write-Host "  connector bundled in connector\ (see connector\CONNECTOR-SETUP.txt)"

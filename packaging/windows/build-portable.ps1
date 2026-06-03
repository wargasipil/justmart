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
  Push-Location (Join-Path $root "frontend")
  npm ci
  npm run build
  Pop-Location
  Remove-Item -Recurse -Force (Join-Path $embedDir "assets") -ErrorAction SilentlyContinue
  Copy-Item -Recurse -Force (Join-Path $root "frontend\dist\*") $embedDir
  Push-Location (Join-Path $root "backend")
  $env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
  go build -ldflags "-s -w" -o $exe ./cmd/server
  Remove-Item Env:\GOOS, Env:\GOARCH, Env:\CGO_ENABLED
  Pop-Location
}
if (-not (Test-Path $exe)) { throw "justmart.exe not found at $exe" }

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

# --- 4. Zip for distribution -------------------------------------------------
$zip = Join-Path $dist "$outName.zip"
Remove-Item -Force $zip -ErrorAction SilentlyContinue
Compress-Archive -Path $outDir -DestinationPath $zip -Force

Write-Host ""
Write-Host "Portable build complete:"
Write-Host "  folder: $outDir"
Write-Host "  zip:    $zip"
Write-Host "  login:  $OwnerEmail / $OwnerPassword   (port $Port)"

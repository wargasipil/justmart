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

  LICENSED FLAVOR (-Licensed):
    Same folder + zip, but justmart.exe is compiled with -tags license, so it
    runs the getresolved licence guard: on first start it opens a local
    activation page and refuses to serve until a licence id is entered, then
    re-verifies periodically and stops if the licence lapses. Output is named
    justmart-portable-licensed-<version> so it can never overwrite the free
    build sitting beside it in dist\.

      ... build-portable.ps1 -Licensed
      ... build-portable.ps1 -Licensed -LicenceId GR-XXXX-XXXX-XXXX-XXXX

    -LicenceId pre-seeds config.yaml so an unattended install activates itself
    and the operator never sees the page. Leave it empty for a build handed to a
    customer who will paste their own id.

  NOTE: keep this file ASCII-only (Windows PowerShell 5.1 reads -File as Windows-1252).
#>
[CmdletBinding()]
param(
  [string] $AppVersion     = "0.1.0",
  [int]    $Port           = 8080,
  [string] $OwnerEmail     = "owner@justmart.local",
  [string] $OwnerPassword  = "change-me-now",
  [switch] $SkipExeBuild,
  [switch] $Licensed,
  [string] $LicenceId      = "",
  [string] $LicenseBaseUrl = ""
)

$ErrorActionPreference = "Stop"
$here     = $PSScriptRoot
$root     = (Resolve-Path (Join-Path $here "..\..")).Path
$dist     = Join-Path $root "dist"
$template = Join-Path $here "portable-template"
$embedDir = Join-Path $root "backend\internal\web\dist"

New-Item -ItemType Directory -Force -Path $dist | Out-Null

# --- 1. Build justmart.exe (SPA + migrations embedded) ------------------------
# The licensed flavor is a genuinely different binary: the `license` build tag
# swaps internal/licensegate's no-op stubs for the real guard (and is the only
# thing that links the getresolved SDK at all). So it gets its own output name --
# sharing justmart.exe would let -SkipExeBuild package a free binary into a
# licensed zip, or ship a guard to customers who did not buy one, with nothing
# in the filename to tell them apart.
$tagArgs = @()
$exeName = "justmart.exe"
if ($Licensed) {
  $tagArgs = @("-tags", "license")
  $exeName = "justmart-licensed.exe"
}
$exe = Join-Path $dist $exeName
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
  go build @tagArgs -ldflags "-s -w -X main.version=$AppVersion" -o $exe ./cmd/server
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

# --- 2b. Licence block rendered into config.yaml + README ---------------------
# Both placeholders render to an empty string for the free build, so ONE set of
# templates serves both flavors and they cannot drift apart.
$licenseBlock    = ""
$licenseReadme   = ""
$licenseLauncher = ""
if ($Licensed) {
  # One extra line in the launcher's console banner. The console window is the
  # only place a first-run operator is told what the waiting is for -- the app
  # port stays closed until the licence is accepted, so there is nothing to see
  # in the browser tab the launcher would otherwise have opened.
  $licenseLauncher = "echo  First run: activate this PC in the page that opens."

  $licBase = if ($LicenseBaseUrl) { $LicenseBaseUrl } else { "https://api.getresolved.id" }
  $licenseBlock = @"

# --- Licence (this is the LICENSED edition) -----------------------------------
# Justmart verifies its licence at startup and will not serve until this PC is
# activated. On first run it opens an activation page in your browser; paste the
# licence id you were given.
license:
  base_url: $licBase
  # Paste a licence id here to activate without the page (optional).
  id: "$LicenceId"
  # Where the activation credential and the signed licence are kept. Keep this
  # folder with justmart.exe -- deleting it means activating this PC again.
  cache_dir: ./license
  # How long Justmart keeps working while the licence server is UNREACHABLE.
  # This never extends a licence past the date it was paid through.
  grace: 72h
  # How often a running Justmart re-checks its licence.
  recheck_interval: 6h
  # true = never open a browser at activation; the page address is printed in
  # the console window instead (for a PC with no desktop).
  headless: false
"@

  $licenseReadme = @"

LICENCE / ACTIVATION
--------------------
This is the licensed edition. The first time you start Justmart it opens an
activation page in your browser and waits there: paste the licence id you were
given (it looks like GR-XXXX-XXXX-XXXX-XXXX). Justmart starts as soon as it is
accepted, and every later start goes straight through.

- The licence is tied to THIS PC. Moving this folder to another PC needs a new
  activation - ask us to release the old one first.
- Justmart keeps working for up to 3 days if it cannot reach the licence server,
  so a dropped internet connection is not a problem. It does stop when the
  licence itself expires.
- The  license\  folder next to justmart.exe holds the activation. Keep it with
  the rest of the folder; deleting it means activating this PC again.
- No page appeared? The address is printed in the black console window
  (something like http://127.0.0.1:53112) - open it in a browser yourself.


"@
}

# --- 3. Assemble the portable folder -----------------------------------------
$outName = if ($Licensed) { "justmart-portable-licensed-$AppVersion" } else { "justmart-portable-$AppVersion" }
$outDir  = Join-Path $dist $outName
Remove-Item -Recurse -Force $outDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $outDir | Out-Null

Copy-Item $exe (Join-Path $outDir "justmart.exe")
# One-off pricing migration (discounts -> grosir tiers). No placeholders to
# render; it finds justmart.exe + config.yaml next to itself.
Copy-Item (Join-Path $here "justmart-discount-to-grosir.bat") (Join-Path $outDir "justmart-discount-to-grosir.bat")

# Render each template with literal placeholder substitution, write as ASCII (no BOM).
function Write-Rendered([string]$srcName, [string]$dstName) {
  $text = Get-Content -Raw (Join-Path $template $srcName)
  $text = $text.Replace('__PORT__', "$Port").
                Replace('__JWT_SECRET__', $jwt).
                Replace('__OWNER_EMAIL__', $OwnerEmail).
                Replace('__OWNER_PASSWORD__', $OwnerPassword).
                Replace('__LICENSE_BLOCK__', $licenseBlock).
                Replace('__LICENSE_README__', $licenseReadme).
                Replace('__LICENSE_LAUNCHER_NOTE__', $licenseLauncher)
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
if ($Licensed) {
  Write-Host "Portable build complete (LICENSED - guard compiled in):"
} else {
  Write-Host "Portable build complete:"
}
Write-Host "  folder: $outDir"
Write-Host "  zip:    $zip"
Write-Host "  login:  $OwnerEmail / $OwnerPassword   (port $Port)"
Write-Host "  connector bundled in connector\ (see connector\CONNECTOR-SETUP.txt)"

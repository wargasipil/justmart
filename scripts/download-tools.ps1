# Download third-party tools for WargaPOS development
# Run from repo root: .\scripts\download-tools.ps1

$ErrorActionPreference = "Stop"

$binDir = Join-Path $PSScriptRoot "..\thirdparties\bin"
$binDir = [System.IO.Path]::GetFullPath($binDir)

if (-not (Test-Path $binDir)) {
    New-Item -ItemType Directory -Path $binDir -Force | Out-Null
    Write-Host "Created $binDir"
}

# ESC/POS Virtual Printer Emulator
# https://github.com/Garletz/escpos-virtual-printer-emulator
$emulatorUrl  = "https://github.com/Garletz/escpos-virtual-printer-emulator/releases/download/windobe/escpos_emulator.exe"
$emulatorDest = Join-Path $binDir "escpos_emulator.exe"

if (Test-Path $emulatorDest) {
    Write-Host "escpos_emulator.exe already exists, skipping."
} else {
    Write-Host "Downloading escpos_emulator.exe ..."
    Invoke-WebRequest -Uri $emulatorUrl -OutFile $emulatorDest -UseBasicParsing
    Write-Host "Saved to $emulatorDest"
}


Write-Host "Done. Tools are in thirdparties\bin\"

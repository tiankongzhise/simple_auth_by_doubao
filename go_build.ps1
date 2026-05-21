# go_build.ps1
# Build Go binary as static, stripped binary for Baota (Linux amd64 by default)

param(
    [string]$OutputName = "auth-service",
    [string]$GoOS = "linux",
    [string]$GoArch = "amd64",
    [switch]$CompressWithUpx,
    [string]$LdFlags = "-s -w"
)

Set-Location $PSScriptRoot

if (-not $env:GOCACHE) {
    $env:GOCACHE = Join-Path $PSScriptRoot ".gocache"
}
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null

Write-Host "[1/4] Building Go binary for $GoOS/$GoArch ..." -ForegroundColor Cyan

Write-Host "[2/4] Tidying go modules..."
go mod tidy
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: go mod tidy failed" -ForegroundColor Red
    exit 1
}

$env:GOOS = $GoOS
$env:GOARCH = $GoArch
$env:CGO_ENABLED = "0"

$buildArgs = @("build", "-ldflags=$LdFlags", "-o", $OutputName, ".")
Write-Host "[3/4] Running: go $($buildArgs -join ' ')" -ForegroundColor Gray
& go @buildArgs

if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Build failed" -ForegroundColor Red
    exit 1
}

Write-Host "[4/4] Build succeeded: $OutputName" -ForegroundColor Green

$fileInfo = Get-Item $OutputName
Write-Host "File size: $([math]::Round($fileInfo.Length / 1MB, 2)) MB" -ForegroundColor Yellow

if ($CompressWithUpx) {
    if (Get-Command upx -ErrorAction SilentlyContinue) {
        Write-Host "Compressing with UPX..."
        upx --best --lzma $OutputName
        $compressedInfo = Get-Item $OutputName
        Write-Host "Compressed size: $([math]::Round($compressedInfo.Length / 1MB, 2)) MB" -ForegroundColor Green
    } else {
        Write-Host "Warning: UPX not found. Skipping compression." -ForegroundColor Yellow
    }
}

Write-Host "Done. Binary: $OutputName" -ForegroundColor Cyan

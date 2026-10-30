$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
New-Item -ItemType Directory -Force 'build/bin' | Out-Null
$env:CGO_ENABLED = '0'
$env:GOOS = 'windows'
$env:GOARCH = 'amd64'
go -C scripts/winicon run . ../../frontend/appicon.ico ../../rsrc_windows_amd64.syso
if ($LASTEXITCODE -ne 0) { throw 'Windows icon resource generation failed' }
go build -trimpath -tags 'desktop,production' -ldflags '-s -w -H=windowsgui' -o 'build/bin/Clothesline-windows-amd64.exe' .
if ($LASTEXITCODE -ne 0) { throw 'Windows build failed' }
Write-Output 'Built: build/bin/Clothesline-windows-amd64.exe'

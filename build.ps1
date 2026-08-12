$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

go test ./internal/github ./internal/zapret ./internal/app ./internal/hosts ./internal/version
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

go run ./tools/genresources
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$exeName = (go run ./tools/exename).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($exeName)) {
  Write-Error "failed to resolve exe name"
  exit 1
}

go build -trimpath -buildvcs=false -ldflags "-H windowsgui -s -w" -o $exeName ./cmd/zapret-manager
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Built $exeName"

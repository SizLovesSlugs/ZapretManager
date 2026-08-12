$ErrorActionPreference = "Stop"
try {
  [Console]::OutputEncoding = [System.Text.Encoding]::UTF8
} catch {}
$OutputEncoding = [System.Text.Encoding]::UTF8
try {
  $Host.UI.RawUI.WindowTitle = "Zapret Manager — сборка"
} catch {}

function Write-Step([string]$Message) {
  Write-Host "  $Message" -ForegroundColor Cyan
}
function Write-Ok([string]$Message) {
  Write-Host "  $Message" -ForegroundColor Green
}
function Write-Fail([string]$Message) {
  Write-Host "  $Message" -ForegroundColor Red
}

$failed = $false
function Fail([string]$Message) {
  Write-Fail $Message
  $script:failed = $true
}

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $root

$goVersion = "1.25.0"
$goZipName = "go$goVersion.windows-amd64.zip"
$goUrl = "https://go.dev/dl/$goZipName"
$goUrlFallback = "https://dl.google.com/go/$goZipName"
$tools = Join-Path $root ".tools"
$goRoot = Join-Path $tools "go"
$goExe = Join-Path $goRoot "bin\go.exe"
$zipPath = Join-Path $env:TEMP $goZipName
$exeName = "Zapret Manager 1.0 Beta.exe"

Write-Host ""
Write-Host "  Zapret Manager — сборка" -ForegroundColor White
Write-Host "  Папка: $root" -ForegroundColor DarkGray
Write-Host ""

function Test-GoReady {
  return (Test-Path -LiteralPath $goExe)
}

function Get-GoArchive {
  param([string]$Url, [string]$Destination)
  if (Get-Command curl.exe -ErrorAction SilentlyContinue) {
    & curl.exe -L --fail --retry 3 -o $Destination $Url
    return ($LASTEXITCODE -eq 0 -and (Test-Path -LiteralPath $Destination))
  }
  try {
    Invoke-WebRequest -Uri $Url -OutFile $Destination -UseBasicParsing
    return (Test-Path -LiteralPath $Destination)
  } catch {
    return $false
  }
}

if (Test-GoReady) {
  Write-Step "Проверяю portable Go..."
  Write-Ok "Go $goVersion уже есть в .tools — повторно не скачиваю."
} else {
  Write-Step "Go не найден. Скачиваю portable SDK $goVersion..."
  New-Item -ItemType Directory -Force -Path $tools | Out-Null
  if (Test-Path -LiteralPath $zipPath) {
    Remove-Item -LiteralPath $zipPath -Force
  }

  Write-Step "Качаю архив с go.dev..."
  $downloaded = Get-GoArchive -Url $goUrl -Destination $zipPath
  if (-not $downloaded) {
    Write-Step "Основное зеркало не ответило. Пробую dl.google.com..."
    $downloaded = Get-GoArchive -Url $goUrlFallback -Destination $zipPath
  }

  if (-not $downloaded) {
    Fail "Не удалось скачать Go. Проверьте интернет и запустите сборку ещё раз."
  } else {
    Write-Ok "Архив скачан."
    Write-Step "Распаковываю Go в .tools\go..."
    if (Test-Path -LiteralPath $goRoot) {
      Remove-Item -LiteralPath $goRoot -Recurse -Force
    }
    if (-not (Get-Command tar.exe -ErrorAction SilentlyContinue)) {
      Fail "Не найден tar.exe — распаковать архив Go не получилось."
    } else {
      & tar.exe -xf $zipPath -C $tools
      if ($LASTEXITCODE -ne 0 -or -not (Test-GoReady)) {
        Fail "Не удалось распаковать архив Go."
      } else {
        Write-Ok "Go распакован и готов к сборке."
      }
    }
    if (Test-Path -LiteralPath $zipPath) {
      Remove-Item -LiteralPath $zipPath -Force -ErrorAction SilentlyContinue
    }
  }
}

if (-not $failed) {
  if (-not (Test-GoReady)) {
    Fail "go.exe не найден: $goExe"
  } else {
    $env:GOROOT = $goRoot
    $env:GOPATH = Join-Path $tools "gopath"
    $env:GOCACHE = Join-Path $tools "gocache"
    $env:GOMODCACHE = Join-Path $tools "pkgmod"
    $env:CGO_ENABLED = "0"
    $env:GOTOOLCHAIN = "local"
    $env:PATH = "$(Join-Path $goRoot 'bin');$env:PATH"

    $resolvedName = (& $goExe run ./tools/exename 2>$null)
    if ($resolvedName) { $exeName = $resolvedName.Trim() }
    Write-Step "Собираю «$exeName»..."
    & $goExe build -trimpath -buildvcs=false -ldflags "-H windowsgui -s -w" -o $exeName ./cmd/zapret-manager
    if ($LASTEXITCODE -ne 0) {
      Fail "Сборка не удалась. Смотрите сообщения компилятора выше."
    } else {
      $exePath = Join-Path $root $exeName
      Write-Host ""
      Write-Ok "Готово: $exePath"
      $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
      if (-not $isAdmin) {
        Write-Ok "Запускайте от имени администратора."
        Write-Ok "Нужен Microsoft Edge WebView2 Runtime."
      }
      Write-Host ""
    }
  }
}

if ($failed) {
  Write-Host ""
  Write-Fail "Сборка остановлена из-за ошибки."
  Write-Host ""
  exit 1
}
exit 0
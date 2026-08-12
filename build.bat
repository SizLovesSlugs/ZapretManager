@echo off
setlocal EnableExtensions EnableDelayedExpansion
chcp 65001 >nul
cd /d "%~dp0"

set "GO_VERSION=1.25.0"
set "GO_ZIP=go%GO_VERSION%.windows-amd64.zip"
set "GO_URL=https://go.dev/dl/%GO_ZIP%"
set "GO_URL_FALLBACK=https://dl.google.com/go/%GO_ZIP%"
set "TOOLS=%~dp0.tools"
set "GOROOT=%TOOLS%\go"
set "GOPATH=%TOOLS%\gopath"
set "GOCACHE=%TOOLS%\gocache"
set "GOMODCACHE=%TOOLS%\gopath\pkg\mod"
set "GOEXE=%GOROOT%\bin\go.exe"
set "EXE_NAME=Zapret Manager 0.1.exe"

echo.
echo  Zapret Manager — сборка
echo  Каталог: %CD%
echo.

if not exist "%GOEXE%" (
  echo  Go %GO_VERSION% не найден, скачиваю portable SDK...
  if not exist "%TOOLS%" mkdir "%TOOLS%"
  set "ZIP=%TEMP%\%GO_ZIP%"
  del /f /q "!ZIP!" 2>nul

  curl.exe -L --fail --retry 3 -o "!ZIP!" "%GO_URL%"
  if errorlevel 1 (
    echo  Основное зеркало недоступно, пробую запасное...
    curl.exe -L --fail --retry 3 -o "!ZIP!" "%GO_URL_FALLBACK%"
  )
  if errorlevel 1 (
    echo  curl не сработал, пробую PowerShell...
    powershell -NoProfile -ExecutionPolicy Bypass -Command "Invoke-WebRequest -Uri '%GO_URL%' -OutFile '!ZIP!'"
  )
  if not exist "!ZIP!" (
    echo  Не удалось скачать Go. Проверьте интернет и повторите.
    exit /b 1
  )

  echo  Распаковываю Go в .tools\go ...
  if exist "%GOROOT%" rmdir /s /q "%GOROOT%"
  tar.exe -xf "!ZIP!" -C "%TOOLS%"
  if errorlevel 1 (
    echo  Не удалось распаковать архив Go.
    exit /b 1
  )
  del /f /q "!ZIP!" 2>nul
)

if not exist "%GOEXE%" (
  echo  go.exe не найден после установки: %GOEXE%
  exit /b 1
)

set "PATH=%GOROOT%\bin;%PATH%"
set "CGO_ENABLED=0"
set "GOTOOLCHAIN=local"

echo  Компилирую %EXE_NAME% ...
"%GOEXE%" build -trimpath -buildvcs=false -ldflags "-H windowsgui -s -w" -o "%EXE_NAME%" ./cmd/zapret-manager
if errorlevel 1 (
  echo.
  echo  Сборка не удалась.
  exit /b 1
)

echo.
echo  Готово: %CD%\%EXE_NAME%
echo  Запускайте от имени администратора. Нужен Microsoft Edge WebView2 Runtime.
echo.
endlocal
exit /b 0

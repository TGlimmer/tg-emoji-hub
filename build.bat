@echo off
chcp 65001 >nul

REM tg-emoji-hub one-click build script
REM Usage:
REM   build.bat            build Linux backend + frontend (DEPLOY mode, default)
REM   build.bat win        build Windows server.exe (for local debug)
REM   build.bat backend    same as default (Linux backend only)
REM   build.bat frontend   frontend only (web/dist)

setlocal enabledelayedexpansion
set ROOT=%~dp0
cd /d "%ROOT%"

if "%1"=="win"      goto :win
if "%1"=="windows"  goto :win
if "%1"=="backend"  goto :backend
if "%1"=="linux"    goto :backend
if "%1"=="frontend" goto :frontend
if "%1"==""         goto :all
echo Unknown arg: %1
echo Supported: (empty) / backend / frontend / win / linux
exit /b 1

:all
call :backend || exit /b 1
call :frontend || exit /b 1
goto :done

:backend
echo.
echo === [1/2] Build backend (Linux amd64, for server deploy) ===
if not exist dist mkdir dist
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o dist\server-linux .\cmd\server
if errorlevel 1 (
  echo [FAIL] Linux backend build failed
  exit /b 1
)
for %%I in (dist\server-linux) do echo [OK] dist\server-linux  ^(%%~zI bytes^)
exit /b 0

:win
echo.
echo === Build Windows backend (server.exe, for local debug) ===
if not exist dist mkdir dist
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w" -o dist\server.exe .\cmd\server
if errorlevel 1 (
  echo [FAIL] Windows backend build failed
  exit /b 1
)
for %%I in (dist\server.exe) do echo [OK] dist\server.exe  ^(%%~zI bytes^)
exit /b 0

:frontend
echo.
echo === [2/2] Build frontend (web/dist) ===
cd web
if not exist node_modules (
  echo [INFO] node_modules not found, running npm install
  call npm install
  if errorlevel 1 (
    echo [FAIL] npm install failed
    cd ..
    exit /b 1
  )
)
call npm run build
if errorlevel 1 (
  echo [FAIL] frontend build failed
  cd ..
  exit /b 1
)
cd ..
echo [OK] web\dist\
exit /b 0

:done
echo.
echo === All Done ===
echo Backend: dist\server-linux   (scp to server for deploy)
echo Frontend: web\dist\          (scp the whole dir for deploy)
echo.
echo Tip: local Windows debug -- run 'build.bat win' to get server.exe
echo.
endlocal

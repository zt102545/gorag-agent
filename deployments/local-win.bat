@echo off
setlocal

set "ROOT=%~dp0.."
set "BACKEND_PORT=8080"
set "FRONTEND_PORT=5173"
set "ACTION=%~1"
if "%ACTION%"=="" set "ACTION=restart"

if /I "%ACTION%"=="start" goto start
if /I "%ACTION%"=="stop" goto stop
if /I "%ACTION%"=="restart" goto restart
if /I "%ACTION%"=="status" goto status

echo Usage: %~nx0 [start^|stop^|restart^|status]
exit /b 1

:restart
call :stop_services
call :start_services
exit /b %ERRORLEVEL%

:stop
call :stop_services
exit /b %ERRORLEVEL%

:start
call :start_services
exit /b %ERRORLEVEL%

:status
call :print_status
exit /b %ERRORLEVEL%

:stop_services
echo Stopping services on ports %BACKEND_PORT% and %FRONTEND_PORT%...
powershell -NoProfile -ExecutionPolicy Bypass -Command "$ports=@(%BACKEND_PORT%,%FRONTEND_PORT%); $targets=@{}; foreach($port in $ports){ Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue | ForEach-Object { if ($_.OwningProcess -gt 0) { $targets[[int]$_.OwningProcess]=$true } } }; foreach($pid in @($targets.Keys)){ try { $proc=Get-CimInstance Win32_Process -Filter ('ProcessId=' + $pid); if ($proc -and $proc.ParentProcessId -gt 0) { $targets[[int]$proc.ParentProcessId]=$true } } catch {} }; foreach($pid in @($targets.Keys)){ Stop-Process -Id ([int]$pid) -Force -ErrorAction SilentlyContinue }"
timeout /t 1 /nobreak >nul
call :print_status
exit /b 0

:start_services
echo Starting backend on http://127.0.0.1:%BACKEND_PORT% ...
start "Agent CS Backend" /min cmd /k "pushd ""%ROOT%"" && set ""APP_ENV=local"" && go run ./cmd/server"

if not exist "%ROOT%\web\node_modules" (
  echo Installing frontend dependencies...
  pushd "%ROOT%\web"
  call npm install
  popd
)

echo Starting frontend on http://127.0.0.1:%FRONTEND_PORT% ...
start "Agent CS Frontend" /min cmd /k "pushd ""%ROOT%\web"" && npm run dev -- --host 127.0.0.1"

echo.
echo Services are starting. Wait a few seconds, then open:
echo   Backend health: http://127.0.0.1:%BACKEND_PORT%/health
echo   GM frontend:    http://127.0.0.1:%FRONTEND_PORT%
exit /b 0

:print_status
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-NetTCPConnection -LocalPort %BACKEND_PORT%,%FRONTEND_PORT% -State Listen -ErrorAction SilentlyContinue | Select-Object LocalAddress,LocalPort,State,OwningProcess | Format-Table -AutoSize"
exit /b 0

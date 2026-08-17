@echo off
rem ============================================================================
rem  Timmypanel one-click build. Double-click me, or:
rem
rem    build.bat                 frontend + Windows exe  -> dist\timmypanel.exe
rem    build.bat linux           Linux amd64 binary (for the VPS)
rem    build.bat all             both
rem    build.bat -SkipFrontend   Go only, reuse last frontend build
rem
rem  The real build logic lives in build.ps1 -- this is only an entry point for
rem  when you'd rather not open PowerShell. Change build steps there, not here.
rem
rem  ASCII ONLY IN THIS FILE. cmd.exe reads a .bat using the *console* code page,
rem  which is 65001 in PowerShell and 936 in a stock cmd window. Chinese text
rem  here decodes differently under the two, and cmd then re-seeks by byte offset
rem  and starts executing garbage (measured: comments turned into commands).
rem  chcp on line 1 does not save you -- the file is already being read.
rem  Same family of trap as the UTF-8 BOM requirement on build.ps1, see CLAUDE.md.
rem ============================================================================

setlocal
cd /d "%~dp0"

rem A single bare word is treated as the target platform, so you don't have to
rem type -Target. Anything else is passed through to build.ps1 untouched.
set "PSARGS=%*"
if /i "%~1"=="windows" set "PSARGS=-Target windows"
if /i "%~1"=="linux"   set "PSARGS=-Target linux"
if /i "%~1"=="all"     set "PSARGS=-Target all"

where go >nul 2>nul
if errorlevel 1 (
    echo [ERROR] go not found. Install Go and put it on PATH.
    goto :fail
)

rem npm is only needed when the frontend is actually being built.
echo %* | find /i "skipfrontend" >nul
if errorlevel 1 (
    where npm >nul 2>nul
    if errorlevel 1 (
        echo [ERROR] npm not found. Install Node.js, or run: build.bat -SkipFrontend
        goto :fail
    )
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0build.ps1" %PSARGS%
if errorlevel 1 goto :fail

echo.
echo Done. Binaries are in dist\. On first run timmypanel.exe creates
echo data\config.yaml and prints the initial admin password to the log.
call :maybe_pause
exit /b 0

:fail
echo.
echo BUILD FAILED - scroll up to the first error.
call :maybe_pause
exit /b 1

rem When double-clicked, cmd's own command line contains /c and the window closes
rem the moment we exit, so hold it open. When run from an existing terminal, don't.
:maybe_pause
echo "%cmdcmdline%" | find /i "/c" >nul
if not errorlevel 1 pause
goto :eof

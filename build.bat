@echo off
setlocal
set GOARCH=amd64
set GOOS=windows
echo Building Windows binary...
go build -ldflags "-s -w -H windowsgui" -trimpath -o out/ExcelSplitter.exe
endlocal
if not %errorlevel%==0 (
	echo Build failed
	exit /b %errorlevel%
) else (
	echo Build successful
)

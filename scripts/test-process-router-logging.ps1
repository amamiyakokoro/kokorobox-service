param(
    [Parameter(Mandatory = $true)]
    [string]$ProxyBridgeIncludePath,
    [string]$OutputDirectory = (Join-Path ([IO.Path]::GetTempPath()) "kokorobox-rule-logging-tests")
)
$ErrorActionPreference = "Stop"
if (-not (Test-Path (Join-Path $ProxyBridgeIncludePath "ProxyBridge.h"))) {
    throw "The pinned ProxyBridge SDK header is required."
}
New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
$Source = Join-Path $PSScriptRoot "../processrouter/native/tests/test_rule_logging.c"
$TestPath = Join-Path $OutputDirectory "test_rule_logging.exe"
$VsWhere = "${env:ProgramFiles(x86)}\Microsoft Visual Studio\Installer\vswhere.exe"
$VsPath = & $VsWhere -latest -products * -requires Microsoft.VisualStudio.Component.VC.Tools.x86.x64 -property installationPath
if (-not $VsPath) { throw "Visual Studio C++ x64 tools are unavailable." }
$VcVars = Join-Path $VsPath "VC\Auxiliary\Build\vcvarsall.bat"
$CompileArgs = "/nologo /utf-8 /O2 /W4 /wd4100 /wd4189 /wd4996 /D_WIN32_WINNT=0x0601 " +
    "/I`"$ProxyBridgeIncludePath`" `"$Source`" /link shell32.lib /OUT:`"$TestPath`""
cmd /c "`"$VcVars`" x64 >nul && cd /d `"$OutputDirectory`" && cl.exe $CompileArgs"
if ($LASTEXITCODE -ne 0) { throw "Process Router logging regression test build failed." }
& $TestPath
if ($LASTEXITCODE -ne 0) { throw "Process Router logging regression test failed." }

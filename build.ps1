# Timmypanel 一键构建脚本
#
#   .\build.ps1                 # 构建前端 + 当前平台（Windows）二进制
#   .\build.ps1 -Target linux   # 构建前端 + Linux amd64 二进制（部署到 VPS 用）
#   .\build.ps1 -Target all     # 两个都要
#   .\build.ps1 -SkipFrontend   # 只重新编译 Go，前端沿用上次的产物
#
# 后端用的是纯 Go 的 sqlite 驱动（modernc），所以 CGO_ENABLED=0 就能交叉编译，
# 不需要在 Windows 上装 gcc。

param(
    [ValidateSet('windows', 'linux', 'all')]
    [string]$Target = 'windows',
    [switch]$SkipFrontend,
    [string]$Version = ''
)

$ErrorActionPreference = 'Stop'
$root = $PSScriptRoot
$distDir = Join-Path $root 'dist'

if (-not $Version) {
    $Version = Get-Date -Format 'yyyy.MM.dd'
}

function Invoke-Step($name, $script) {
    Write-Host "==> $name" -ForegroundColor Cyan
    & $script
    if ($LASTEXITCODE -ne 0) { throw "$name 失败（退出码 $LASTEXITCODE）" }
}

# ---- 前端 ----
if (-not $SkipFrontend) {
    Push-Location (Join-Path $root 'frontend')
    try {
        if (-not (Test-Path 'node_modules')) {
            Invoke-Step '安装前端依赖' { npm install }
        }
        Invoke-Step '构建前端' { npm run build }
    } finally {
        Pop-Location
    }
} else {
    Write-Host '==> 跳过前端构建' -ForegroundColor DarkGray
}

# ---- 后端 ----
New-Item -ItemType Directory -Force -Path $distDir | Out-Null
$ldflags = "-s -w -X main.Version=$Version"

Push-Location (Join-Path $root 'backend')
try {
    $env:CGO_ENABLED = '0'

    if ($Target -eq 'windows' -or $Target -eq 'all') {
        $env:GOOS = 'windows'; $env:GOARCH = 'amd64'
        $out = Join-Path $distDir 'timmypanel.exe'
        Invoke-Step '编译 windows/amd64' { go build -trimpath -ldflags $ldflags -o $out . }
    }

    if ($Target -eq 'linux' -or $Target -eq 'all') {
        $env:GOOS = 'linux'; $env:GOARCH = 'amd64'
        $out = Join-Path $distDir 'timmypanel-linux-amd64'
        Invoke-Step '编译 linux/amd64' { go build -trimpath -ldflags $ldflags -o $out . }
    }
} finally {
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}

Write-Host ''
Write-Host "构建完成（版本 $Version）：" -ForegroundColor Green
Get-ChildItem $distDir | ForEach-Object {
    '{0,-28} {1,8:N1} MB' -f $_.Name, ($_.Length / 1MB)
}

# 构建 file-ledger 单 exe（产物输出到仓库外：项目目录不放 exe）
# 用法：.uild.ps1 [-OutDir <目录>] [-WithWeb]
param(
    [string]$OutDir = 'D:\Development\build\file-ledger',
    [switch]$WithWeb
)

$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $MyInvocation.MyCommand.Path

# Go 工具链（本机放在 D 盘；PATH 里没有也能跑）
$go = 'D:\Development\go\bin\go.exe'
if (-not (Test-Path $go)) { $go = 'go' }

# 本机 Go 环境变量（D 盘）；已有就不覆盖
if (-not $env:GOPATH) { $env:GOPATH = 'D:\Development\gopath' }
if (-not $env:GOMODCACHE) { $env:GOMODCACHE = 'D:\Development\gopath\pkg\mod' }
if (-not $env:GOCACHE) { $env:GOCACHE = 'D:\Development\gocache' }

if ($WithWeb) {
    Write-Host '== 构建前端 web/dist =='
    Push-Location (Join-Path $repo 'web')
    try {
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "前端构建失败：退出码 $LASTEXITCODE" }
    } finally { Pop-Location }
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$exe = Join-Path $OutDir 'file-ledger.exe'

Write-Host "== 编译 Go -> $exe"
Push-Location $repo
try {
    & $go build -o $exe .
    if ($LASTEXITCODE -ne 0) { throw "go build 失败：退出码 $LASTEXITCODE" }
} finally { Pop-Location }

Write-Host "构建完成：$exe（config.json 首次运行在该目录生成）"

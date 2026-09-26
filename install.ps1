#Requires -Version 5.1
<#
.SYNOPSIS
    Daybook 一键安装脚本（Windows）
.DESCRIPTION
    1. 自动识别系统架构（amd64 / arm64）
    2. 优先使用脚本所在目录 dist\ 下的本地产物，找不到再从 GitHub Release 下载
    3. 安装 daybook.exe，并按需初始化数据目录（daybook.yaml / vault / public）
    4. 注册计划任务实现开机自启
       说明：daybook 是普通控制台程序，没有实现 Windows 服务控制协议，
       直接用 sc.exe 注册服务会报 1053 启动失败，因此这里用任务计划程序等效实现常驻运行。
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\install.ps1
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\install.ps1 -DataDir "D:\Daybook"
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\install.ps1 -Uninstall -Purge
#>
[CmdletBinding()]
param(
    [string]$InstallDir,
    [string]$DataDir,
    [switch]$NoService,
    [switch]$Uninstall,
    [switch]$Purge,
    [switch]$Download
)

$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo = "StatIndet/daybook"
$ApiUrl = "https://api.github.com/repos/$Repo/releases/latest"
$DownloadUrlBase = "https://github.com/$Repo/releases/download"
$TaskName = "Daybook"
$Port = 1313

if (-not $InstallDir) { $InstallDir = Join-Path $env:ProgramData "Daybook\bin" }
if (-not $DataDir)    { $DataDir    = Join-Path $env:ProgramData "Daybook" }

function Write-Log  { param([string]$Message) Write-Host "=> $Message" }
function Write-Warn { param([string]$Message) Write-Warning $Message }

function Test-Administrator {
    $principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# ---------- 提权 ----------
if (-not (Test-Administrator)) {
    Write-Log "需要管理员权限，正在请求提权 ..."
    $argList = @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', "`"$PSCommandPath`"")
    if ($PSBoundParameters.ContainsKey('InstallDir')) { $argList += @('-InstallDir', "`"$InstallDir`"") }
    if ($PSBoundParameters.ContainsKey('DataDir'))    { $argList += @('-DataDir', "`"$DataDir`"") }
    if ($NoService) { $argList += '-NoService' }
    if ($Uninstall) { $argList += '-Uninstall' }
    if ($Purge)     { $argList += '-Purge' }
    if ($Download)  { $argList += '-Download' }
    Start-Process -FilePath 'powershell.exe' -Verb RunAs -ArgumentList $argList
    exit
}

# ---------- 平台检测 ----------
$Arch = $env:PROCESSOR_ARCHITEW6432
if (-not $Arch) { $Arch = $env:PROCESSOR_ARCHITECTURE }
switch ($Arch.ToUpperInvariant()) {
    'AMD64' { $ArchName = 'amd64' }
    'ARM64' { $ArchName = 'arm64' }
    default { throw "不支持的架构：$Arch（仅支持 amd64 / arm64）" }
}
$Platform = "windows_$ArchName"
Write-Log "检测到平台：$Platform"

# ---------- 卸载 ----------
if ($Uninstall) {
    Write-Log "正在卸载 Daybook ..."
    if (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue) {
        Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
        Write-Log "已移除计划任务：$TaskName"
    }
    Get-Process -Name 'daybook' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue

    $installedExe = Join-Path $InstallDir 'daybook.exe'
    if (Test-Path $installedExe) { Remove-Item $installedExe -Force }

    if (Get-NetFirewallRule -DisplayName "Daybook $Port" -ErrorAction SilentlyContinue) {
        Remove-NetFirewallRule -DisplayName "Daybook $Port" -ErrorAction SilentlyContinue
    }

    if ($Purge) {
        if (Test-Path $DataDir) { Remove-Item $DataDir -Recurse -Force }
        Write-Log "已删除数据目录：$DataDir"
    } else {
        Write-Log "数据目录保留：$DataDir（如需删除请加 -Purge）"
    }
    exit 0
}

# ---------- 定位安装来源：优先本地，缺失则下载 ----------
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$SrcBin = $null
$TmpDir = $null

if (-not $Download) {
    $candidates = @(
        (Join-Path $ScriptDir "dist\daybook-windows-$ArchName.exe"),
        (Join-Path $ScriptDir "daybook-windows-$ArchName.exe"),
        (Join-Path $ScriptDir "daybook.exe")
    )
    foreach ($candidate in $candidates) {
        if (Test-Path $candidate) {
            $SrcBin = $candidate
            Write-Log "使用本地产物：$candidate"
            break
        }
    }
}

if (-not $SrcBin) {
    Write-Log "查询最新 Release ..."
    try {
        $Release = Invoke-RestMethod -Uri $ApiUrl -ErrorAction Stop
    } catch {
        throw "无法获取最新版本信息：$_"
    }
    $LatestTag = $Release.tag_name
    if (-not $LatestTag) { throw "无法确定最新版本号" }
    $AssetName = "daybook_${LatestTag}_${Platform}.zip"
    $AssetUrl = "$DownloadUrlBase/$LatestTag/$AssetName"
    $ChecksumsUrl = "$DownloadUrlBase/$LatestTag/checksums.txt"
    Write-Log "最新版本：$LatestTag"

    $TmpDir = Join-Path ([IO.Path]::GetTempPath()) ("daybook_" + [Guid]::NewGuid().ToString("N").Substring(0, 8))
    New-Item -ItemType Directory -Force -Path $TmpDir | Out-Null

    try {
        $ZipPath = Join-Path $TmpDir $AssetName
        Write-Log "下载 $AssetName ..."
        Invoke-WebRequest -UseBasicParsing -Uri $AssetUrl -OutFile $ZipPath -ErrorAction Stop

        try {
            $SumsPath = Join-Path $TmpDir 'checksums.txt'
            Invoke-WebRequest -UseBasicParsing -Uri $ChecksumsUrl -OutFile $SumsPath -ErrorAction Stop
            $Expected = $null
            foreach ($line in (Get-Content $SumsPath)) {
                if ($line -match [regex]::Escape($AssetName)) {
                    $Expected = ($line -split '\s+')[0]
                    break
                }
            }
            if ($Expected) {
                $Actual = (Get-FileHash -Path $ZipPath -Algorithm SHA256).Hash
                if ($Expected.ToUpper() -ne $Actual.ToUpper()) {
                    throw "校验和不匹配（期望 $Expected，实际 $Actual）"
                }
                Write-Log "校验和验证通过"
            }
        } catch {
            Write-Warn "跳过校验和：$_"
        }

        Expand-Archive -Path $ZipPath -DestinationPath $TmpDir -Force
        $SrcBin = Join-Path $TmpDir 'daybook.exe'
        if (-not (Test-Path $SrcBin)) { throw "压缩包中未找到 daybook.exe" }
    } catch {
        if (Test-Path $TmpDir) { Remove-Item $TmpDir -Recurse -Force -ErrorAction SilentlyContinue }
        throw
    }
}

$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)

# ---------- 初始化数据目录 ----------
Write-Log "初始化数据目录：$DataDir"
New-Item -ItemType Directory -Force -Path (Join-Path $DataDir 'vault\notes') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $DataDir 'vault\pages') | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $DataDir 'public') | Out-Null

$ConfigPath = Join-Path $DataDir 'daybook.yaml'
if (-not (Test-Path $ConfigPath)) {
    $yaml = @'
site:
  name:
    zh: "我的日记"
    en_US: "My Daybook"
  url: ""
  startedAt: "2026-09-27"
  copyright: "© 2026"
  favicon: ""

profile:
  author:
    name: "博主"
    nameEn: "Owner"
    logoText: "博主"
    avatar: ""
    aboutUrl: "/about"
  social: []
  slogan:
    zh: "记录思考与笔记的个人角落。"
    en_US: "Personal nook for thoughts & notes."

seo:
  homeTitle:
    zh: "我的 Daybook · 随记与记录"
    en_US: "My Daybook"
  homeDescription:
    zh: "欢迎来到我的个人 Daybook。"
    en_US: "Welcome to my personal Daybook."

comment:
  enabled: false
  provider: "waline"
  waline:
    serverURL: ""
    lang: "zh-CN"
    pageSize: 10
    commentSorting: "latest"
    search: false
    imageUploader: false

stats:
  enabled: true

share:
  text: "「{Title}」"
'@
    [IO.File]::WriteAllText($ConfigPath, $yaml, $Utf8NoBom)
    Write-Log "已生成默认配置文件 daybook.yaml"
} else {
    Write-Log "检测到已有 daybook.yaml，跳过初始化配置"
}

$HelloPath = Join-Path $DataDir 'vault\notes\hello.md'
if (-not (Test-Path $HelloPath)) {
    $hello = @'
---
title: 你好，Daybook
date: "2026-09-27"
tags:
  - 随笔
summary: 第一篇示例笔记
lang: zh_CN
---

欢迎使用 Daybook！这是你的第一篇笔记。

打开写作台 http://localhost:1313/admin 即可开始创作。
'@
    [IO.File]::WriteAllText($HelloPath, $hello, $Utf8NoBom)
    Write-Log "已生成示例笔记 vault\notes\hello.md"
}

$AboutPath = Join-Path $DataDir 'vault\pages\about.md'
if (-not (Test-Path $AboutPath)) {
    $about = @'
---
title: 关于
date: "2026-09-27"
summary: 关于我和这个站点
---

你好，我是这里的博主。

这里是记录思考与笔记的个人角落，欢迎随意看看。

如果想联系我，可以直接在文章下方留言。
'@
    [IO.File]::WriteAllText($AboutPath, $about, $Utf8NoBom)
    Write-Log "已生成关于页 vault\pages\about.md"
}

$AboutEnPath = Join-Path $DataDir 'vault\pages\about-en.md'
if (-not (Test-Path $AboutEnPath)) {
    $aboutEn = @'
---
title: About
date: "2026-09-27"
summary: About me and this site
---

Hi, I'm the owner of this site.

A personal nook for thoughts and notes — feel free to look around.

If you'd like to reach me, just leave a comment under any post.
'@
    [IO.File]::WriteAllText($AboutEnPath, $aboutEn, $Utf8NoBom)
    Write-Log "已生成关于页（英文）vault\pages\about-en.md"
}

# ---------- 安装可执行文件 ----------
$installedExe = Join-Path $InstallDir 'daybook.exe'
Write-Log "安装可执行文件到 $installedExe"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Copy-Item -Path $SrcBin -Destination $installedExe -Force

$version = & $installedExe version
if ($LASTEXITCODE -ne 0) { throw "安装后无法运行，请检查文件是否完整" }
Write-Log "安装完成：$version"

# ---------- 首次构建 ----------
Write-Log "生成静态站点产物 ..."
Push-Location $DataDir
try {
    & $installedExe build
    if ($LASTEXITCODE -eq 0) {
        Write-Log "静态站点构建完成"
    } else {
        Write-Warn "首次构建未成功，服务启动后可在写作台重新构建"
    }
} finally {
    Pop-Location
}

# ---------- 注册计划任务（开机自启） ----------
if ($NoService) {
    Write-Log "已跳过服务注册（-NoService）"
} else {
    Write-Log "注册计划任务：$TaskName"
    $action = New-ScheduledTaskAction -Execute $installedExe -Argument 'serve' -WorkingDirectory $DataDir
    $trigger = New-ScheduledTaskTrigger -AtStartup
    $principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
    $settings = New-ScheduledTaskSettingsSet `
        -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
        -MultipleInstances IgnoreNew `
        -ExecutionTimeLimit ([TimeSpan]::Zero) `
        -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)

    Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger `
        -Principal $principal -Settings $settings -Force | Out-Null
    Start-ScheduledTask -TaskName $TaskName

    # 放行防火墙（供局域网访问）
    try {
        if (-not (Get-NetFirewallRule -DisplayName "Daybook $Port" -ErrorAction SilentlyContinue)) {
            New-NetFirewallRule -DisplayName "Daybook $Port" -Direction Inbound -Action Allow `
                -Protocol TCP -LocalPort $Port | Out-Null
            Write-Log "已添加防火墙入站规则：TCP $Port"
        }
    } catch {
        Write-Warn "添加防火墙规则失败，如需局域网访问请手动放行 TCP $Port"
    }
}

# ---------- 清理临时文件 ----------
if ($TmpDir -and (Test-Path $TmpDir)) {
    Remove-Item $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}

# ---------- 完成提示 ----------
$lanIp = (Get-NetIPAddress -AddressFamily IPv4 -ErrorAction SilentlyContinue |
    Where-Object { $_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*' } |
    Select-Object -First 1).IPAddress
if (-not $lanIp) { $lanIp = '<本机IP>' }

Write-Host ""
Write-Log "Daybook 安装完成"
Write-Host "   可执行文件：$installedExe"
Write-Host "   数据目录：  $DataDir"
if (-not $NoService) {
    Write-Host "   计划任务：  $TaskName（已开机自启，以 SYSTEM 身份运行）"
    Write-Host "   站点地址：  http://${lanIp}:$Port/"
    Write-Host "   写作台：    http://${lanIp}:$Port/admin"
    Write-Host "   查看状态：  Get-ScheduledTask -TaskName $TaskName"
    Write-Host "   重启服务：  Stop-ScheduledTask $TaskName; Start-ScheduledTask $TaskName"
} else {
    Write-Host "   启动方式：  cd `"$DataDir`"; & `"$installedExe`" serve"
}
Write-Host ""

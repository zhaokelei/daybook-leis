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

$Repo = "zhaokelei/daybook-leis"
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
    en: "Xiaolei's Daybook"
    zh: "小磊的日记"
  url: ""
  startedAt: "2026-09-26"
  copyright: "© 2026 小磊"
  # Set a path relative to the vault root to use a custom favicon. Empty uses the built-in Daybook favicon.
  favicon: ""

profile:
  author:
    name: "小磊"
    nameEn: "Xiaolei"
    logoText: "小磊"
    avatar: "/avatar.png"
    aboutUrl: "/about"
  social: []
  # - type: github
  #   url: "https://github.com/your-name"
  # - type: youtube
  #   url: "https://youtube.com/@your-channel"

  slogan:
    en_US: "Personal nook for thoughts & notes."
    zh: "记录思考与笔记的个人角落。"

seo:
  homeTitle:
    en: "Notes from Xiaolei's Daybook"
    zh: "小磊的 Daybook · 随记与记录"
  homeDescription:
    en: "Welcome to my personal Daybook."
    zh: "欢迎来到我的个人 Daybook。"

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

$FirstDiaryZh = Join-Path $DataDir 'vault\notes\第一篇日记.md'
if (-not (Test-Path $FirstDiaryZh)) {
    $diaryZh = @'
---
title: 小磊 | 个人简介
date: "2026-09-26"
tags:
    - 随笔
summary: 小磊的个人简介
lang: zh_CN
i18n_key: first-diary
---

# 小磊 / Xiaolei | 个人简介 / Personal Profile

## 👤 基本信息 / Basic Information
| 中文 | English |
| :--- | :--- |
| **姓名**：小磊 | **Name**: Calix |
| **年龄**：26岁 | **Age**: 26 |
| **身份**：计算机爱好者、代码爱好者 | **Identity**: Computer Enthusiast & Coding Lover |
| **状态**：持续学习，深耕所爱，稳步成长 | **Status**: Constantly learning, deeply devoted, steadily growing |

## ✨ 个人标签 / Personal Tags
| 中文标签 | English Tags |
| :--- | :--- |
| `#代码爱好者 #计算机发烧友 #持续深耕 #极简思维 #热爱技术 #终身学习` | `#CodingLover #ComputerEnthusiast #ContinuousExploration #MinimalistThinking #TechPassion #LifelongLearning` |

## 💻 兴趣爱好 / Interests
| 中文 | English |
| :--- | :--- |
| 徜徉于计算机与代码的世界，沉醉于逻辑之美与创造的乐趣。热衷于深挖技术底层原理，反复打磨编码能力，在持续实践与迭代中突破自我边界。<br><br>我始终对互联网、软件开发与计算机底层技术保持赤诚好奇，不止步于表层应用，主动探索新兴技术与框架。每一次编码、调试与优化，皆是沉淀与积累，让热爱成为长期前行的底气与动力。 | I dwell in the world of code and computers, captivated by the beauty of logic and the joy of creation. I enjoy exploring underlying technical principles, refining programming skills, and breaking through limitations through continuous practice and iteration.<br><br>I always retain a sincere curiosity for the internet, software development, and computer fundamentals. Rather than staying at superficial application usage, I actively explore emerging technologies and frameworks. Every coding practice, debugging process, and optimization effort becomes solid accumulation, turning passion into long-term motivation for steady progress. |

## 📝 个人感悟 / Personal Insights
| 中文 | English |
| :--- | :--- |
| 技术从无捷径，所有成长，皆源于日积月累的沉淀与日复一日的坚守。<br><br>二十六岁，守纯粹热爱，持清醒自知，不浮躁、不苟且。热爱代码，不止热爱敲码的过程，更倾心于技术重塑事物、创造价值、赋能美好的力量。<br><br>未来，我将继续深耕技术领域，持续学习、不断迭代，以匠心沉淀自我，以热爱奔赴长远成长。 | Technology bears no shortcuts. All advancement comes from persistent accumulation and quiet perseverance.<br><br>At 26, I uphold pure enthusiasm and sober self-awareness, free from impetuosity and superficiality. My love for code is never limited to the act of programming itself, but lies in the power of technology to reshape reality, deliver value, and bring possibilities to life.<br><br>I will continue to immerse myself in the technical field, keep learning, keep iterating, and grow steadily with devotion and patience. |

## 🎯 个人愿景 / Personal Vision
| 中文 | English |
| :--- | :--- |
| 专注技术，踏实精进，从容前行。<br><br>持续打磨自身技术能力，在热爱的赛道上不断探索，解锁更多技术可能，成长为有思考、有沉淀的专业技术爱好者。愿热爱落地生根，让每一步成长皆清晰可见。 | To stay focused on technology, progress calmly and diligently.<br><br>I will keep polishing my technical capabilities, explore more technological possibilities on this beloved track, and grow into a thoughtful, professional tech enthusiast. Let passion take root, and let every step of growth be clearly visible. |
'@
    [IO.File]::WriteAllText($FirstDiaryZh, $diaryZh, $Utf8NoBom)
    Write-Log "已生成第一篇笔记 vault\notes\第一篇日记.md"
}

$FirstDiaryEn = Join-Path $DataDir 'vault\notes\first-diary.md'
if (-not (Test-Path $FirstDiaryEn)) {
    $diaryEn = @'
---
title: Xiaolei | Personal Profile
date: "2026-09-26"
tags:
    - Essay
summary: Personal profile of Xiaolei
lang: en_US
i18n_key: first-diary
---

# 小磊 / Xiaolei | 个人简介 / Personal Profile

## 👤 基本信息 / Basic Information
| 中文 | English |
| :--- | :--- |
| **姓名**：小磊 | **Name**: Calix |
| **年龄**：26岁 | **Age**: 26 |
| **身份**：计算机爱好者、代码爱好者 | **Identity**: Computer Enthusiast & Coding Lover |
| **状态**：持续学习，深耕所爱，稳步成长 | **Status**: Constantly learning, deeply devoted, steadily growing |

## ✨ 个人标签 / Personal Tags
| 中文标签 | English Tags |
| :--- | :--- |
| `#代码爱好者 #计算机发烧友 #持续深耕 #极简思维 #热爱技术 #终身学习` | `#CodingLover #ComputerEnthusiast #ContinuousExploration #MinimalistThinking #TechPassion #LifelongLearning` |

## 💻 兴趣爱好 / Interests
| 中文 | English |
| :--- | :--- |
| 徜徉于计算机与代码的世界，沉醉于逻辑之美与创造的乐趣。热衷于深挖技术底层原理，反复打磨编码能力，在持续实践与迭代中突破自我边界。<br><br>我始终对互联网、软件开发与计算机底层技术保持赤诚好奇，不止步于表层应用，主动探索新兴技术与框架。每一次编码、调试与优化，皆是沉淀与积累，让热爱成为长期前行的底气与动力。 | I dwell in the world of code and computers, captivated by the beauty of logic and the joy of creation. I enjoy exploring underlying technical principles, refining programming skills, and breaking through limitations through continuous practice and iteration.<br><br>I always retain a sincere curiosity for the internet, software development, and computer fundamentals. Rather than staying at superficial application usage, I actively explore emerging technologies and frameworks. Every coding practice, debugging process, and optimization effort becomes solid accumulation, turning passion into long-term motivation for steady progress. |

## 📝 个人感悟 / Personal Insights
| 中文 | English |
| :--- | :--- |
| 技术从无捷径，所有成长，皆源于日积月累的沉淀与日复一日的坚守。<br><br>二十六岁，守纯粹热爱，持清醒自知，不浮躁、不苟且。热爱代码，不止热爱敲码的过程，更倾心于技术重塑事物、创造价值、赋能美好的力量。<br><br>未来，我将继续深耕技术领域，持续学习、不断迭代，以匠心沉淀自我，以热爱奔赴长远成长。 | Technology bears no shortcuts. All advancement comes from persistent accumulation and quiet perseverance.<br><br>At 26, I uphold pure enthusiasm and sober self-awareness, free from impetuosity and superficiality. My love for code is never limited to the act of programming itself, but lies in the power of technology to reshape reality, deliver value, and bring possibilities to life.<br><br>I will continue to immerse myself in the technical field, keep learning, keep iterating, and grow steadily with devotion and patience. |

## 🎯 个人愿景 / Personal Vision
| 中文 | English |
| :--- | :--- |
| 专注技术，踏实精进，从容前行。<br><br>持续打磨自身技术能力，在热爱的赛道上不断探索，解锁更多技术可能，成长为有思考、有沉淀的专业技术爱好者。愿热爱落地生根，让每一步成长皆清晰可见。 | To stay focused on technology, progress calmly and diligently.<br><br>I will keep polishing my technical capabilities, explore more technological possibilities on this beloved track, and grow into a thoughtful, professional tech enthusiast. Let passion take root, and let every step of growth be clearly visible. |
'@
    [IO.File]::WriteAllText($FirstDiaryEn, $diaryEn, $Utf8NoBom)
    Write-Log "已生成第一篇笔记（英文）vault\notes\first-diary.md"
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

# 覆盖前先停止正在运行的计划任务与进程，否则 exe 被占用会导致复制失败
if (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue) {
    Write-Log "检测到已有计划任务，先停止以释放可执行文件：$TaskName"
    Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
}
Get-Process -Name 'daybook' -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 500

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

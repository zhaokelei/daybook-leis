# daybook-leis
daybook的一个根据个人需求二开的版本

# Daybook

Daybook is a minimalist static blog generator for Go and HTML beginners, featuring native Obsidian Markdown compatibility, zero-framework TypeScript interactions, and a clean, reading-focused design.

[![Deploy to Cloudflare](https://deploy.workers.cloudflare.com/button)](https://deploy.workers.cloudflare.com/?url=https://github.com/StatIndet/daybook-vault)

If you enjoy Daybook, consider supporting its development.

[![Ko-fi](https://img.shields.io/badge/Ko--fi-Support%20Daybook-FF6433?style=for-the-badge&logo=kofi&logoColor=white)](https://ko-fi.com/shizhi)
[![爱发电](https://img.shields.io/badge/爱发电-Support%20Daybook-946CE6?style=for-the-badge&logo=afdian&logoColor=white)](https://afdian.com/a/shi-zhi)

## Desktop

|                           Homepage                           |                            Notes                             |                         Reading Mode                         |                          Footnotes                           |
| :----------------------------------------------------------: | :----------------------------------------------------------: | :----------------------------------------------------------: | :----------------------------------------------------------: |
| ![Daybook homepage](https://raw.githubusercontent.com/StatIndet/picture/main/daybook/%E9%A6%96%E9%A1%B5.png) | ![Daybook notes page](https://raw.githubusercontent.com/StatIndet/picture/main/daybook/%E7%AC%94%E8%AE%B0.png) | ![Daybook reading mode](https://raw.githubusercontent.com/StatIndet/picture/main/daybook/%E9%98%85%E8%AF%BB%E6%A8%A1%E5%BC%8F.png) | ![Daybook footnotes](https://raw.githubusercontent.com/StatIndet/picture/main/daybook/%E6%B3%A8%E9%87%8A.png) |
|                         Attachments                          |                       Knowledge Graph                        |                     Archive & Statistics                     |                            About                             |
| ![Daybook attachments](https://raw.githubusercontent.com/StatIndet/picture/main/daybook/%E9%99%84%E4%BB%B6.png) | ![Daybook knowledge graph](https://raw.githubusercontent.com/StatIndet/picture/main/daybook/%E5%85%B3%E7%B3%BB%E5%9B%BE%E8%B0%B1.png) | ![Daybook archive and statistics](https://raw.githubusercontent.com/StatIndet/picture/main/daybook/%E5%BD%92%E6%A1%A3%E7%95%8C%E9%9D%A2%E4%B8%8E%E7%BB%9F%E8%AE%A1.png) | ![Daybook about page](https://raw.githubusercontent.com/StatIndet/picture/main/daybook/about%E7%95%8C%E9%9D%A2.png) |

## Mobile

|                        Mobile Layout                         |                        Mobile Drawer                         |                       Reading Progress                       |
| :----------------------------------------------------------: | :----------------------------------------------------------: | :----------------------------------------------------------: |
| ![Daybook mobile layout](https://raw.githubusercontent.com/StatIndet/picture/main/daybook/%E7%A7%BB%E5%8A%A8%E7%AB%AF%E5%B8%83%E5%B1%80.png) | ![Daybook mobile drawer](https://raw.githubusercontent.com/StatIndet/picture/main/daybook/%E7%A7%BB%E5%8A%A8%E7%AB%AF%E6%8A%BD%E5%B1%89.png) | ![Daybook reading progress bar](https://raw.githubusercontent.com/StatIndet/picture/main/daybook/%E9%A1%B6%E9%83%A8%E8%BF%9B%E5%BA%A6%E6%9D%A1.png) |



## Architecture

This repository (`StatIndet/daybook`) contains the Daybook CLI source code. It includes:
* **Go runtime**: Core CLI application and build engine.
* **Embedded assets**: Templates, CSS, and generated static files (`internal/embedded/`).
* **Frontend source**: TypeScript source files (`assets/ts/`).
* **Vendor pipeline**: npm-based asset generation for fonts, KaTeX, and Waline.
* **Release workflow**: GitHub Actions for building cross-platform standalone binaries.

### The Vault
Your blog content lives in an independent directory called a **Vault**, which is completely separated from this source repository.

The CLI runs inside your Vault and expects the following structure:
```
my-vault/
├── daybook.yaml
├── notes/
└── attachments/
```

## 一键安装

本仓库（[zhaokelei/daybook-leis](https://github.com/zhaokelei/daybook-leis)）在 Daybook 基础上内置了**后台写作台**，并提供 Windows / Linux 一键安装脚本：自动识别架构、优先使用本地产物（缺失时再从 GitHub Release 下载）、初始化数据目录，并注册系统服务实现开机自启。

### 方式一：在线安装（从 GitHub 直接拉取）

**Linux（amd64 / arm64）**

```sh
curl -fsSL https://raw.githubusercontent.com/zhaokelei/daybook-leis/main/install.sh | sudo bash
```

**Windows（在管理员 PowerShell 中运行）**

```powershell
$u = 'https://raw.githubusercontent.com/zhaokelei/daybook-leis/main/install.ps1'
$f = "$env:TEMP\install-daybook.ps1"
irm $u -OutFile $f
powershell -ExecutionPolicy Bypass -File $f
```

> 在线安装会从本仓库的 [GitHub Releases](https://github.com/zhaokelei/daybook-leis/releases) 下载对应平台的二进制包。
> 如果本仓库还没有发布 Release，请改用「方式二」，或先发布一个 Release。

### 方式二：克隆后本地安装（不依赖 Release）

```bash
git clone https://github.com/zhaokelei/daybook-leis.git
cd daybook-leis

# Linux
sudo ./install.sh
```

```powershell
git clone https://github.com/zhaokelei/daybook-leis.git
cd daybook-leis

# Windows（管理员 PowerShell）
powershell -ExecutionPolicy Bypass -File .\install.ps1
```

脚本会**优先使用仓库 `dist/` 目录下的本地产物**（例如 `dist/daybook-linux-amd64`、`dist/daybook-windows-amd64.exe`），找不到时再从 GitHub Release 下载。

### 安装后的访问地址

| 项目 | 地址 |
| ---- | ---- |
| 站点首页 | `http://<服务器IP>:1313/` |
| 写作台（后台管理） | `http://<服务器IP>:1313/admin` |

### 默认账号密码

| 项目 | 默认值 |
| ---- | ------ |
| 账号 | `admin` |
| 密码 | `admin` |

> **首次登录后请立即在写作台「账号设置」中修改默认密码。**
> 凭据以加盐 SHA-256 保存在数据目录的 `.daybook-admin.json` 中。

### Linux 安装选项

| 项目 | 默认值 |
| ---- | ------ |
| 可执行文件 | `/usr/local/bin/daybook` |
| 数据目录 | `/opt/daybook`（`daybook.yaml`、`vault/`、`public/`） |
| 服务方式 | systemd 服务 `daybook`，开机自启 |
| 服务端口 | `1313` |

```sh
sudo ./install.sh                                       # 默认安装并注册 systemd 服务
sudo ./install.sh --data-dir /opt/daybook --bin-dir /usr/local/bin
sudo ./install.sh --download                            # 强制从 GitHub 下载
sudo ./install.sh --no-service                          # 只安装，不注册服务
sudo ./install.sh --uninstall                           # 卸载（保留数据目录）
sudo ./install.sh --uninstall --purge                   # 卸载并删除数据目录
```

服务管理：

```sh
systemctl status daybook      # 查看状态
systemctl restart daybook     # 重启服务
journalctl -u daybook -f      # 查看日志
```

### Windows 安装选项

| 项目 | 默认值 |
| ---- | ------ |
| 可执行文件 | `%ProgramData%\Daybook\bin\daybook.exe` |
| 数据目录 | `%ProgramData%\Daybook` |
| 自启方式 | 计划任务 `Daybook`（以 SYSTEM 身份开机启动） |
| 服务端口 | `1313`（安装时自动放行防火墙） |

```powershell
powershell -ExecutionPolicy Bypass -File .\install.ps1                      # 默认安装
powershell -ExecutionPolicy Bypass -File .\install.ps1 -DataDir "D:\Daybook"
powershell -ExecutionPolicy Bypass -File .\install.ps1 -Download            # 强制下载
powershell -ExecutionPolicy Bypass -File .\install.ps1 -NoService
powershell -ExecutionPolicy Bypass -File .\install.ps1 -Uninstall
powershell -ExecutionPolicy Bypass -File .\install.ps1 -Uninstall -Purge
```

> daybook 是普通控制台程序，未实现 Windows 服务控制协议（直接用 `sc.exe` 注册服务会报 1053），因此这里用**任务计划程序**实现等效的开机常驻。

服务管理：

```powershell
Get-ScheduledTask -TaskName Daybook                     # 查看状态
Stop-ScheduledTask Daybook; Start-ScheduledTask Daybook # 重启
```

### 平台支持

| 操作系统 | 架构 | 状态 |
| ---- | ---- | ---- |
| Linux | amd64, arm64 | 一键脚本 / 原生支持 |
| Windows | amd64, arm64 | 一键脚本 / 原生支持 |

## 更新

更新只需**重新运行一次安装脚本**，无需卸载。脚本会自动完成：

1. 用新版本**覆盖可执行文件**（`cp` / `Copy-Item -Force`）；
2. 在数据目录重新执行一次 `daybook build`；
3. **重新注册并重启系统服务**（systemd / 计划任务）。

> 你的数据是安全的：脚本对 `daybook.yaml` 与 `vault/` 采用「**存在即跳过**」处理（检测到已有 `daybook.yaml`，跳过初始化配置），因此**更新不会覆盖站点配置与文章/附件**。

### 在线更新（推荐，直接拉取最新 Release）

**Linux**

```sh
curl -fsSL https://raw.githubusercontent.com/zhaokelei/daybook-leis/main/install.sh | sudo bash
```

**Windows（管理员 PowerShell）**

```powershell
$u = 'https://raw.githubusercontent.com/zhaokelei/daybook-leis/main/install.ps1'
$f = "$env:TEMP\install-daybook.ps1"
irm $u -OutFile $f
powershell -ExecutionPolicy Bypass -File $f
```

### 克隆后本地更新

```bash
cd daybook-leis
git pull

# Linux
sudo ./install.sh --download
```

```powershell
cd daybook-leis
git pull

# Windows（管理员 PowerShell）
powershell -ExecutionPolicy Bypass -File .\install.ps1 -Download
```

> 不加 `--download`（Windows 为 `-Download`）时，脚本会**优先使用仓库 `dist/` 目录下的本地产物**；加上后会强制从 [GitHub Releases](https://github.com/zhaokelei/daybook-leis/releases) 下载对应平台的最新版本。

### 手动更新（仅替换二进制）

如果只想换程序、不动其它配置：

```sh
# Linux
sudo cp dist/daybook-linux-amd64 /usr/local/bin/daybook
sudo chmod 0755 /usr/local/bin/daybook
systemctl restart daybook
```

```powershell
# Windows（管理员 PowerShell）
Copy-Item -Path .\dist\daybook-windows-amd64.exe -Destination "$env:ProgramData\Daybook\bin\daybook.exe" -Force
Stop-ScheduledTask Daybook; Start-ScheduledTask Daybook
```

### 更新后确认

```sh
# Linux
daybook version                 # 确认版本
systemctl status daybook        # 确认服务运行
journalctl -u daybook -f        # 查看日志
```

```powershell
# Windows
daybook version
Get-ScheduledTask -TaskName Daybook
```

> **更新前建议先备份数据**：写作台提供「导出数据」功能，可打包下载 `daybook.yaml` 与 `vault/`；也可以直接复制数据目录（Linux `/opt/daybook`，Windows `%ProgramData%\Daybook`）。
> 若更新后站点内容未刷新，可在数据目录中手动再执行一次 `daybook build`。

## CLI Commands

Run these commands inside your Vault directory:

* `daybook build`: Reads `daybook.yaml` and `notes/`, compiles your site, and outputs static HTML to `public/`.
* `daybook serve`: Starts a local web server at `http://localhost:1313` to preview your site.
* `daybook version`: Prints the current CLI version.

## Development

If you are modifying the Daybook CLI itself, follow this workflow:

```bash
# Install npm dependencies
npm ci

# Build first-party TypeScript files
npm run build:js

# Build third-party vendor assets (KaTeX, Waline, Fonts)
npm run build:vendor

# Run Go unit tests
go test ./...

# Build the temporary binary for local testing
go build -o daybook-cli ./cmd/daybook

# Run the complete integrity check suite
./scripts/check.sh
```

> **Note**: Do not modify generated files in `internal/embedded/static/js/` or `internal/embedded/static/vendor/` directly. Always modify the source TypeScript or update the npm package and run the corresponding build scripts.

## Acknowledge

[Retypeset](https://github.com/radishzzz/astro-theme-retypeset)

## License

This project is open-sourced under the [MIT License](LICENSE).
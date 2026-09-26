#!/usr/bin/env bash
#
# Daybook 一键安装脚本（Linux）
#
# 功能：
#   1. 自动识别系统架构（amd64 / arm64）
#   2. 优先使用脚本所在目录 dist/ 下的本地产物，找不到再从 GitHub Release 下载
#   3. 安装可执行文件，并按需初始化数据目录（daybook.yaml / vault / public）
#   4. 注册 systemd 服务并设置开机自启
#
# 用法：
#   sudo ./install.sh
#   sudo ./install.sh --data-dir /opt/daybook --bin-dir /usr/local/bin
#   sudo ./install.sh --download                  # 强制从 GitHub 下载
#   sudo ./install.sh --no-service                # 只安装，不注册服务
#   sudo ./install.sh --uninstall                 # 卸载（保留数据目录）
#   sudo ./install.sh --uninstall --purge         # 卸载并删除数据目录
#
set -eu

REPO="zhaokelei/daybook-leis"
DOWNLOAD_URL_BASE="https://github.com/${REPO}/releases/download"
API_URL="https://api.github.com/repos/${REPO}/releases/latest"

EXE_NAME="daybook"
DEFAULT_BIN_DIR="/usr/local/bin"
DEFAULT_DATA_DIR="/opt/daybook"
SERVICE_NAME="daybook"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
PORT="1313"

BIN_DIR="$DEFAULT_BIN_DIR"
DATA_DIR="$DEFAULT_DATA_DIR"
RUN_USER=""
NO_SERVICE=0
UNINSTALL=0
PURGE=0
FORCE_DOWNLOAD=0

log()  { printf '=> %s\n' "$*"; }
warn() { printf '警告：%s\n' "$*" >&2; }
die()  { printf '错误：%s\n' "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
Daybook 一键安装脚本（Linux）

用法：
  sudo ./install.sh [选项]

选项：
  --bin-dir <目录>    可执行文件安装目录（默认 /usr/local/bin）
  --data-dir <目录>   数据目录，存放 daybook.yaml / vault / public（默认 /opt/daybook）
  --user <用户名>     systemd 服务运行用户（默认当前 sudo 调用的用户）
  --download          强制从 GitHub Release 下载，忽略本地 dist 产物
  --no-service        只安装，不注册 systemd 服务
  --uninstall         卸载（保留数据目录）
  --purge             配合 --uninstall 一并删除数据目录
  -h, --help          显示本帮助
EOF
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

# 需要写 /usr/local/bin 与 /etc/systemd/system，非 root 时通过 sudo 重新执行
if [ "$(id -u)" -ne 0 ]; then
  if command -v sudo >/dev/null 2>&1; then
    log "需要管理员权限，正在通过 sudo 重新执行 ..."
    SELF="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
    exec sudo -E bash "$SELF" "$@"
  fi
  die "请以 root 身份运行（或先安装 sudo）"
fi

while [ $# -gt 0 ]; do
  case "$1" in
    --bin-dir)    BIN_DIR="$2"; shift 2 ;;
    --data-dir)   DATA_DIR="$2"; shift 2 ;;
    --user)       RUN_USER="$2"; shift 2 ;;
    --no-service) NO_SERVICE=1; shift ;;
    --uninstall)  UNINSTALL=1; shift ;;
    --purge)      PURGE=1; shift ;;
    --download)   FORCE_DOWNLOAD=1; shift ;;
    -h|--help)    usage; exit 0 ;;
    *)            die "未知参数：$1" ;;
  esac
done

# ---------- 平台检测 ----------
OS="$(uname -s)"
[ "$OS" = "Linux" ] || die "本脚本仅用于 Linux（当前系统：$OS）。Windows 请使用 install.ps1"

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)   ARCH_NAME="amd64" ;;
  aarch64|arm64)  ARCH_NAME="arm64" ;;
  *)              die "不支持的架构：$ARCH（仅支持 amd64 / arm64）" ;;
esac
PLATFORM="linux_${ARCH_NAME}"
log "检测到平台：${PLATFORM}"

# ---------- 卸载 ----------
if [ "$UNINSTALL" -eq 1 ]; then
  log "正在卸载 Daybook ..."
  if command -v systemctl >/dev/null 2>&1; then
    systemctl disable --now "$SERVICE_NAME" >/dev/null 2>&1 || true
    systemctl daemon-reload >/dev/null 2>&1 || true
  fi
  rm -f "$SERVICE_FILE"
  rm -f "${BIN_DIR}/${EXE_NAME}"
  log "已移除可执行文件与 systemd 服务"
  if [ "$PURGE" -eq 1 ]; then
    rm -rf "$DATA_DIR"
    log "已删除数据目录：$DATA_DIR"
  else
    log "数据目录保留：$DATA_DIR（如需删除请加 --purge）"
  fi
  exit 0
fi

# ---------- 定位安装来源：优先本地，缺失则下载 ----------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SRC_BIN=""

if [ "$FORCE_DOWNLOAD" -eq 0 ]; then
  for cand in \
    "${SCRIPT_DIR}/dist/daybook-linux-${ARCH_NAME}" \
    "${SCRIPT_DIR}/daybook-linux-${ARCH_NAME}" \
    "${SCRIPT_DIR}/daybook"; do
    if [ -f "$cand" ]; then
      SRC_BIN="$cand"
      log "使用本地产物：$cand"
      break
    fi
  done
fi

download_bin() {
  if command -v curl >/dev/null 2>&1; then
    DL="curl -fsSL"
  elif command -v wget >/dev/null 2>&1; then
    DL="wget -qO-"
  else
    die "未找到 curl 或 wget，无法下载"
  fi
  command -v tar >/dev/null 2>&1 || die "未找到 tar，无法解压"

  log "查询最新 Release ..."
  LATEST_TAG="$($DL "$API_URL" | grep '"tag_name":' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')" || true
  [ -n "${LATEST_TAG:-}" ] || die "无法从 GitHub API 获取最新版本号"
  local ASSET="daybook_${LATEST_TAG}_${PLATFORM}.tar.gz"
  log "最新版本：${LATEST_TAG}"

  TMP_DIR="$(mktemp -d)"
  trap 'rm -rf "${TMP_DIR:-}"' EXIT INT TERM

  log "下载 ${ASSET} ..."
  $DL "${DOWNLOAD_URL_BASE}/${LATEST_TAG}/${ASSET}" > "${TMP_DIR}/${ASSET}" || die "下载失败：${ASSET}"

  if $DL "${DOWNLOAD_URL_BASE}/${LATEST_TAG}/checksums.txt" > "${TMP_DIR}/checksums.txt" 2>/dev/null; then
    SUM_CMD=""
    if command -v sha256sum >/dev/null 2>&1; then
      SUM_CMD="sha256sum"
    elif command -v shasum >/dev/null 2>&1; then
      SUM_CMD="shasum -a 256"
    fi
    if [ -n "$SUM_CMD" ]; then
      EXPECT="$(grep "$ASSET" "${TMP_DIR}/checksums.txt" | awk '{print $1}' | head -n1)"
      ACTUAL="$($SUM_CMD "${TMP_DIR}/${ASSET}" | awk '{print $1}')"
      if [ -n "$EXPECT" ] && [ "$EXPECT" = "$ACTUAL" ]; then
        log "校验和验证通过"
      else
        die "校验和不匹配（期望 ${EXPECT:-无}，实际 ${ACTUAL}）"
      fi
    fi
  else
    warn "未获取到 checksums.txt，跳过校验"
  fi

  tar -xzf "${TMP_DIR}/${ASSET}" -C "${TMP_DIR}" "$EXE_NAME" || die "解压失败"
  [ -f "${TMP_DIR}/${EXE_NAME}" ] || die "压缩包中未找到 ${EXE_NAME}"
  SRC_BIN="${TMP_DIR}/${EXE_NAME}"
}

[ -n "$SRC_BIN" ] || download_bin

# ---------- 初始化数据目录 ----------
log "初始化数据目录：${DATA_DIR}"
mkdir -p "${DATA_DIR}/vault/notes" "${DATA_DIR}/vault/pages" "${DATA_DIR}/public"

if [ ! -f "${DATA_DIR}/daybook.yaml" ]; then
  cat > "${DATA_DIR}/daybook.yaml" <<'YAML'
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
YAML
  log "已生成默认配置文件 daybook.yaml"
else
  log "检测到已有 daybook.yaml，跳过初始化配置"
fi

FIRST_DIARY_ZH="${DATA_DIR}/vault/notes/第一篇日记.md"
if [ ! -f "${FIRST_DIARY_ZH}" ]; then
  cat > "${FIRST_DIARY_ZH}" <<'MD'
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
MD
  log "已生成第一篇笔记 vault/notes/第一篇日记.md"
fi

FIRST_DIARY_EN="${DATA_DIR}/vault/notes/first-diary.md"
if [ ! -f "${FIRST_DIARY_EN}" ]; then
  cat > "${FIRST_DIARY_EN}" <<'MD'
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
MD
  log "已生成第一篇笔记（英文）vault/notes/first-diary.md"
fi

if [ ! -f "${DATA_DIR}/vault/pages/about.md" ]; then
  cat > "${DATA_DIR}/vault/pages/about.md" <<'MD'
---
title: 关于
date: "2026-09-27"
summary: 关于我和这个站点
---

你好，我是这里的博主。

这里是记录思考与笔记的个人角落，欢迎随意看看。

如果想联系我，可以直接在文章下方留言。
MD
  log "已生成关于页 vault/pages/about.md"
fi

if [ ! -f "${DATA_DIR}/vault/pages/about-en.md" ]; then
  cat > "${DATA_DIR}/vault/pages/about-en.md" <<'MD'
---
title: About
date: "2026-09-27"
summary: About me and this site
---

Hi, I'm the owner of this site.

A personal nook for thoughts and notes — feel free to look around.

If you'd like to reach me, just leave a comment under any post.
MD
  log "已生成关于页（英文）vault/pages/about-en.md"
fi

# ---------- 安装可执行文件 ----------
log "安装可执行文件到 ${BIN_DIR}/${EXE_NAME}"
mkdir -p "$BIN_DIR" || die "无法创建目录：$BIN_DIR"
cp "$SRC_BIN" "${BIN_DIR}/${EXE_NAME}" || die "复制失败，请检查权限"
chmod 0755 "${BIN_DIR}/${EXE_NAME}"
"${BIN_DIR}/${EXE_NAME}" version >/dev/null || die "安装后无法运行，请检查文件是否完整"
log "安装完成：$("${BIN_DIR}/${EXE_NAME}" version)"

# ---------- 首次构建 ----------
log "生成静态站点产物 ..."
if ( cd "$DATA_DIR" && "${BIN_DIR}/${EXE_NAME}" build ); then
  log "静态站点构建完成"
else
  warn "首次构建未成功，服务启动后可在写作台重新构建"
fi

# ---------- 注册 systemd 服务 ----------
if [ "$NO_SERVICE" -eq 1 ]; then
  log "已跳过服务注册（--no-service）"
else
  command -v systemctl >/dev/null 2>&1 || die "未找到 systemctl，无法注册服务（可加 --no-service 仅安装）"

  if [ -z "$RUN_USER" ]; then
    RUN_USER="${SUDO_USER:-root}"
  fi
  if [ "$RUN_USER" != "root" ] && id "$RUN_USER" >/dev/null 2>&1; then
    chown -R "$RUN_USER" "$DATA_DIR" 2>/dev/null || warn "无法调整数据目录属主，请手动执行：chown -R ${RUN_USER} ${DATA_DIR}"
  else
    RUN_USER="root"
  fi
  log "服务运行用户：${RUN_USER}"

  cat > "$SERVICE_FILE" <<UNIT
[Unit]
Description=Daybook 静态博客服务
After=network.target

[Service]
Type=simple
WorkingDirectory=${DATA_DIR}
ExecStart=${BIN_DIR}/${EXE_NAME} serve
Restart=always
RestartSec=3
User=${RUN_USER}

[Install]
WantedBy=multi-user.target
UNIT

  systemctl daemon-reload
  systemctl enable --now "$SERVICE_NAME"
  sleep 1
  systemctl --no-pager --full status "$SERVICE_NAME" || true
fi

# ---------- 完成提示 ----------
LAN_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[ -n "$LAN_IP" ] || LAN_IP="<服务器IP>"

printf '\n'
log "Daybook 安装完成"
printf '   可执行文件：%s\n' "${BIN_DIR}/${EXE_NAME}"
printf '   数据目录：  %s\n' "${DATA_DIR}"
if [ "$NO_SERVICE" -eq 0 ]; then
  printf '   服务名称：  %s（已开机自启）\n' "$SERVICE_NAME"
  printf '   站点地址：  http://%s:%s/\n' "$LAN_IP" "$PORT"
  printf '   写作台：    http://%s:%s/admin\n' "$LAN_IP" "$PORT"
  printf '   查看日志：  journalctl -u %s -f\n' "$SERVICE_NAME"
  printf '   重启服务：  systemctl restart %s\n' "$SERVICE_NAME"
else
  printf '   启动方式：  cd %s && %s serve\n' "$DATA_DIR" "${BIN_DIR}/${EXE_NAME}"
fi
printf '\n'

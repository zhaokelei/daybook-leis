package admin

const pageTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Daybook 写作台</title>
<script>
(function () {
  var root = document.documentElement;
  var mode = "system";
  var palette = "default";
  try {
    mode = localStorage.getItem("theme-mode") || "system";
    palette = localStorage.getItem("palette") || "default";
  } catch (e) {}
  function apply() {
    var resolved = mode;
    if (mode === "system") {
      resolved = (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) ? "dark" : "light";
    }
    root.setAttribute("data-theme", resolved === "dark" ? "dark" : "light");
    root.setAttribute("data-palette", palette === "warm" ? "warm" : "default");
  }
  apply();
  window.__adminTheme = {
    apply: apply,
    mode: function () { return mode; },
    setMode: function (next) {
      mode = next;
      try { localStorage.setItem("theme-mode", mode); } catch (e) {}
      apply();
    }
  };
})();
</script>
{{FONT_LINKS}}<link rel="stylesheet" href="{{GLOBAL_CSS}}">
<style>
* { box-sizing: border-box; }
html, body { height: 100%; }
body {
  margin: 0;
  min-height: 100vh;
  height: 100vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: var(--color-page);
  color: var(--color-text);
  font-family: var(--font-ui);
  line-height: 1.6;
}
.admin-topbar {
  flex: 0 0 auto;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 0 20px;
  background: var(--color-paper);
  border-bottom: 1px solid var(--color-line);
}
.admin-brand {
  font-family: var(--font-logo);
  font-size: 19px;
  letter-spacing: 0.02em;
  white-space: nowrap;
}
.admin-brand small {
  font-family: var(--font-ui);
  font-size: 12px;
  color: var(--color-muted);
  margin-left: 8px;
  letter-spacing: 0;
}
.admin-actions { display: flex; align-items: center; gap: 8px; }
.admin-btn {
  font-family: var(--font-ui);
  font-size: 13px;
  padding: 7px 15px;
  border-radius: 999px;
  border: 1px solid var(--color-line);
  background: transparent;
  color: var(--color-text);
  cursor: pointer;
  text-decoration: none;
  transition: border-color var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out), background var(--duration-fast) var(--ease-out);
}
.admin-btn:hover { border-color: var(--color-accent); color: var(--color-accent); }
.admin-btn.primary { background: var(--color-accent); border-color: var(--color-accent); color: #fff; }
.admin-btn.primary:hover { opacity: 0.9; color: #fff; }
.admin-icon-btn { padding: 7px 11px; }

.admin-main { flex: 1 1 auto; display: flex; min-height: 0; }
.admin-sidebar {
  flex: 0 0 300px;
  width: 300px;
  display: flex;
  flex-direction: column;
  min-height: 0;
  background: var(--color-paper);
  border-right: 1px solid var(--color-line);
}
.admin-search { flex: 0 0 auto; padding: 12px; border-bottom: 1px solid var(--color-line); }
.admin-search input {
  width: 100%;
  padding: 9px 12px;
  border-radius: 10px;
  border: 1px solid var(--color-line);
  background: var(--color-page);
  color: var(--color-text);
  font-family: var(--font-ui);
  font-size: 13px;
  outline: none;
}
.admin-search input:focus { border-color: var(--color-accent); }
.admin-list { list-style: none; margin: 0; padding: 8px; overflow-y: auto; flex: 1 1 auto; }
.admin-list li {
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid transparent;
  cursor: pointer;
  margin-bottom: 2px;
  transition: background var(--duration-fast) var(--ease-out), border-color var(--duration-fast) var(--ease-out);
}
.admin-list li:hover { background: var(--color-accent-soft); }
.admin-list li.active { background: var(--color-accent-soft-strong); border-color: var(--color-accent); }
.item-title { display: block; font-size: 14px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.item-meta { display: flex; gap: 8px; margin-top: 3px; font-size: 12px; color: var(--color-muted); }
.item-meta .draft-badge { color: var(--color-accent); }
.admin-list .empty { padding: 20px 12px; color: var(--color-muted); font-size: 13px; text-align: center; }

.admin-editor { flex: 1 1 auto; display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.admin-meta {
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px 22px 14px;
  border-bottom: 1px solid var(--color-line);
}
.admin-title-input {
  width: 100%;
  border: none;
  outline: none;
  background: transparent;
  color: var(--color-text);
  font-family: var(--font-serif);
  font-size: 26px;
  font-weight: 700;
  line-height: 1.3;
  padding: 2px 0;
}
.admin-title-input::placeholder { color: var(--color-faint); }
.admin-meta-row { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; }
.admin-field {
  padding: 7px 10px;
  border-radius: 9px;
  border: 1px solid var(--color-line);
  background: var(--color-page);
  color: var(--color-text);
  font-family: var(--font-ui);
  font-size: 13px;
  outline: none;
}
.admin-field:focus { border-color: var(--color-accent); }
.admin-field.grow { flex: 1 1 180px; }
.admin-check { display: inline-flex; align-items: center; gap: 6px; font-size: 13px; color: var(--color-muted); cursor: pointer; }
.admin-toolbar {
  flex: 0 0 auto;
  display: flex;
  flex-wrap: wrap;
  gap: 3px;
  padding: 8px 20px;
  background: var(--color-paper);
  border-bottom: 1px solid var(--color-line);
}
.admin-tool {
  min-width: 34px;
  height: 32px;
  padding: 0 10px;
  border-radius: 8px;
  border: 1px solid transparent;
  background: transparent;
  color: var(--color-muted);
  font-family: var(--font-ui);
  font-size: 13px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: background var(--duration-fast) var(--ease-out), color var(--duration-fast) var(--ease-out);
}
.admin-tool:hover { background: var(--color-accent-soft); color: var(--color-accent); }
.admin-tool-sep { width: 1px; margin: 4px 4px; background: var(--color-line); }

.admin-panes { flex: 1 1 auto; display: flex; min-height: 0; }
.admin-panes .pane { flex: 1 1 50%; min-width: 0; display: flex; flex-direction: column; }
.admin-panes .pane + .pane { border-left: 1px solid var(--color-line); }
.pane-label {
  flex: 0 0 auto;
  padding: 8px 18px 2px;
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--color-faint);
}
#editor {
  flex: 1 1 auto;
  min-height: 0;
  resize: none;
  border: none;
  outline: none;
  background: var(--color-page);
  color: var(--color-text);
  font-family: var(--font-mono);
  font-size: 14px;
  line-height: 1.85;
  padding: 8px 18px 40px;
  tab-size: 2;
}
.admin-preview { flex: 1 1 auto; min-height: 0; overflow-y: auto; padding: 8px 24px 48px; }
.admin-preview.markdown { font-size: var(--note-text-size); }
.admin-footer {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 22px;
  background: var(--color-paper);
  border-top: 1px solid var(--color-line);
}
#status { font-size: 13px; color: var(--color-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
#status.ok { color: #2e7d32; }
#status.err { color: #c62828; }

.admin-modal {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgba(0, 0, 0, 0.35);
}
.admin-modal[hidden] { display: none; }
.admin-modal-card {
  width: 100%;
  max-width: 460px;
  max-height: 88vh;
  overflow-y: auto;
  background: var(--color-paper);
  border: 1px solid var(--color-line);
  border-radius: 16px;
  box-shadow: 0 18px 48px rgba(0, 0, 0, 0.18);
}
.admin-modal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--color-line);
  font-family: var(--font-logo);
  font-size: 16px;
}
.admin-modal-close {
  border: none;
  background: transparent;
  color: var(--color-muted);
  font-size: 20px;
  line-height: 1;
  cursor: pointer;
}
.admin-modal-body { display: flex; flex-direction: column; gap: 18px; padding: 16px 18px 20px; }
.account-block { display: flex; flex-direction: column; gap: 10px; }
.account-block-title { font-size: 12px; letter-spacing: 0.08em; text-transform: uppercase; color: var(--color-faint); }
.avatar-row { display: flex; align-items: center; gap: 16px; }
.avatar-preview {
  flex: 0 0 auto;
  width: 64px;
  height: 64px;
  border-radius: 50%;
  border: 1px solid var(--color-line);
  background-color: var(--color-accent-soft);
  background-size: cover;
  background-position: center;
}
.avatar-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.account-field { display: flex; align-items: center; gap: 10px; }
.account-field > span { flex: 0 0 76px; font-size: 13px; color: var(--color-muted); }
.account-field > input { flex: 1 1 auto; min-width: 0; }
.account-hint { font-size: 12px; color: var(--color-muted); }
.account-hint.ok { color: #2e7d32; }
.account-hint.err { color: #c62828; }
.account-actions { display: flex; justify-content: flex-end; }
.social-list { display: flex; flex-direction: column; gap: 8px; }
.social-row { display: flex; align-items: center; gap: 8px; }
.social-row select { flex: 0 0 130px; }
.social-row input { flex: 1 1 auto; min-width: 0; }
.social-row .social-del { flex: 0 0 auto; padding: 4px 10px; border: 1px solid var(--color-line); background: transparent; color: var(--color-muted); border-radius: 6px; cursor: pointer; }
.social-row .social-del:hover { color: #c62828; border-color: #c62828; }

@media (max-width: 900px) {
  .admin-sidebar { display: none; }
  .admin-panes .pane + .pane { display: none; }
}
</style>
</head>
<body>
<header class="admin-topbar">
  <div class="admin-brand">Daybook 写作台<small>写好即发布</small></div>
  <div class="admin-actions">
    <button type="button" class="admin-btn" id="btn-new">新建文章</button>
    <button type="button" class="admin-btn admin-icon-btn" id="btn-theme" title="切换主题">主题</button>
    <button type="button" class="admin-btn" id="btn-account">账户</button>
    <button type="button" class="admin-btn" id="btn-logout">退出</button>
    <a class="admin-btn" href="/" target="_blank" rel="noopener">查看站点</a>
  </div>
</header>
<main class="admin-main">
  <aside class="admin-sidebar">
    <div class="admin-search"><input id="filter" type="search" placeholder="筛选文章…"></div>
    <ul class="admin-list" id="note-list"></ul>
  </aside>
  <section class="admin-editor">
    <div class="admin-meta">
      <input class="admin-title-input" id="f-title" type="text" placeholder="标题">
      <div class="admin-meta-row">
        <input class="admin-field grow" id="f-slug" type="text" placeholder="链接标识（文件名，可留空自动生成）">
        <input class="admin-field" id="f-date" type="text" placeholder="YYYY-MM-DD" size="12">
        <select class="admin-field" id="f-lang">
          <option value="zh_CN">中文</option>
          <option value="en_US">English</option>
        </select>
        <label class="admin-check"><input type="checkbox" id="f-draft"> 草稿</label>
        <label class="admin-check"><input type="checkbox" id="f-pin"> 置顶</label>
      </div>
      <div class="admin-meta-row">
        <input class="admin-field grow" id="f-tags" type="text" placeholder="标签，用英文逗号分隔">
        <input class="admin-field grow" id="f-summary" type="text" placeholder="摘要（可选）">
      </div>
      <div class="admin-meta-row">
        <input class="admin-field grow" id="f-i18n-key" type="text" placeholder="多语言分组键 i18n_key（中英文版本填相同值即可合并为一篇文章）">
      </div>
    </div>
    <div class="admin-toolbar" id="toolbar">
      <button type="button" class="admin-tool" data-action="h2" title="标题">H2</button>
      <button type="button" class="admin-tool" data-action="h3" title="小标题">H3</button>
      <span class="admin-tool-sep"></span>
      <button type="button" class="admin-tool" data-action="bold" title="加粗"><b>B</b></button>
      <button type="button" class="admin-tool" data-action="italic" title="斜体"><i>I</i></button>
      <button type="button" class="admin-tool" data-action="quote" title="引用">引用</button>
      <span class="admin-tool-sep"></span>
      <button type="button" class="admin-tool" data-action="ul" title="无序列表">列表</button>
      <button type="button" class="admin-tool" data-action="ol" title="有序列表">1.</button>
      <button type="button" class="admin-tool" data-action="code" title="行内代码">代码</button>
      <button type="button" class="admin-tool" data-action="codeblock" title="代码块">代码块</button>
      <span class="admin-tool-sep"></span>
      <button type="button" class="admin-tool" data-action="link" title="链接">链接</button>
      <button type="button" class="admin-tool" data-action="image" title="图片">图片</button>
      <button type="button" class="admin-tool" data-action="hr" title="分隔线">分隔</button>
    </div>
    <div class="admin-panes">
      <div class="pane">
        <div class="pane-label">Markdown</div>
        <textarea id="editor" spellcheck="false" placeholder="在这里用 Markdown 写正文…"></textarea>
      </div>
      <div class="pane">
        <div class="pane-label">预览</div>
        <div class="admin-preview markdown" id="preview"></div>
      </div>
    </div>
    <div class="admin-footer">
      <span id="status">就绪</span>
      <button type="button" class="admin-btn primary" id="btn-save">保存并发布</button>
    </div>
  </section>
</main>
<div class="admin-modal" id="account-modal" hidden>
  <div class="admin-modal-card">
    <div class="admin-modal-head">
      <span>账户设置</span>
      <button type="button" class="admin-modal-close" id="account-close" title="关闭">×</button>
    </div>
    <div class="admin-modal-body">
      <section class="account-block">
        <div class="account-block-title">博主信息</div>
        <label class="account-field"><span>博主名称（中文）</span><input class="admin-field" id="site-name-zh" type="text" placeholder="博主昵称"></label>
        <label class="account-field"><span>博主名称（English）</span><input class="admin-field" id="site-name-en" type="text" placeholder="Owner name"></label>
        <label class="account-field"><span>博主副标题（中文）</span><input class="admin-field" id="site-slogan-zh" type="text" placeholder="一句话简介"></label>
        <label class="account-field"><span>博主副标题（English）</span><input class="admin-field" id="site-slogan-en" type="text" placeholder="One-line intro"></label>
      </section>
      <section class="account-block">
        <div class="account-block-title">站点资料</div>
        <label class="account-field"><span>站点名称（中文）</span><input class="admin-field" id="site-title-zh" type="text" placeholder="用于 SEO 与 RSS，如：小磊的 Daybook"></label>
        <label class="account-field"><span>站点名称（English）</span><input class="admin-field" id="site-title-en" type="text" placeholder="Site name for SEO & RSS"></label>
        <label class="account-field"><span>站点网址</span><input class="admin-field" id="site-url" type="text" placeholder="https://example.com（留空则用相对路径）"></label>
        <label class="account-field"><span>建站日期</span><input class="admin-field" id="site-started" type="text" placeholder="YYYY-MM-DD"></label>
        <label class="account-field"><span>版权信息</span><input class="admin-field" id="site-copyright" type="text" placeholder="© 2026 你的名字"></label>
        <label class="account-field"><span>站点图标</span><input class="admin-field" id="site-favicon" type="text" placeholder="favicon 路径，留空使用默认"></label>
        <label class="account-field"><span>站点标识</span><input class="admin-field" id="site-logo-text" type="text" placeholder="左上角 Logo 文字，留空用英文名"></label>
        <label class="account-field"><span>关于页</span><input class="admin-field" id="site-about-url" type="text" placeholder="/about"></label>
      </section>
      <section class="account-block">
        <div class="account-block-title">首页 SEO</div>
        <label class="account-field"><span>标题（中文）</span><input class="admin-field" id="seo-title-zh" type="text" placeholder="浏览器标签页标题"></label>
        <label class="account-field"><span>标题（English）</span><input class="admin-field" id="seo-title-en" type="text" placeholder="Browser tab title"></label>
        <label class="account-field"><span>描述（中文）</span><input class="admin-field" id="seo-desc-zh" type="text" placeholder="首页描述"></label>
        <label class="account-field"><span>描述（English）</span><input class="admin-field" id="seo-desc-en" type="text" placeholder="Home description"></label>
      </section>
      <section class="account-block">
        <div class="account-block-title">评论与统计</div>
        <label class="admin-check"><input type="checkbox" id="stats-enabled"> 启用访问统计</label>
        <label class="account-field"><span>分享文案</span><input class="admin-field" id="share-text" type="text" placeholder="「{Title}」"></label>
        <label class="admin-check"><input type="checkbox" id="comment-enabled"> 启用评论</label>
        <label class="account-field"><span>评论服务</span><input class="admin-field" id="comment-provider" type="text" placeholder="waline"></label>
        <label class="account-field"><span>服务地址</span><input class="admin-field" id="waline-server" type="text" placeholder="https://waline.example.com"></label>
        <label class="account-field"><span>评论语言</span><input class="admin-field" id="waline-lang" type="text" placeholder="zh-CN"></label>
        <label class="account-field"><span>每页条数</span><input class="admin-field" id="waline-page-size" type="number" min="1" placeholder="10"></label>
        <label class="account-field"><span>评论排序</span><input class="admin-field" id="waline-sorting" type="text" placeholder="latest / oldest"></label>
        <label class="admin-check"><input type="checkbox" id="waline-search"> 启用评论搜索</label>
        <label class="admin-check"><input type="checkbox" id="waline-upload"> 启用图片上传</label>
        <div class="account-hint" id="site-hint"></div>
        <div class="account-actions"><button type="button" class="admin-btn primary" id="btn-save-site">保存全部设置</button></div>
      </section>
      <section class="account-block">
        <div class="account-block-title">社交链接</div>
        <div id="social-list" class="social-list"></div>
        <div class="account-actions">
          <button type="button" class="admin-btn" id="btn-add-social">+ 添加链接</button>
          <button type="button" class="admin-btn primary" id="btn-save-social">保存社交链接</button>
        </div>
        <div class="account-hint" id="social-hint"></div>
      </section>
      <section class="account-block">
        <div class="account-block-title">头像</div>
        <div class="avatar-row">
          <span class="avatar-preview" id="avatar-preview"></span>
          <div class="avatar-actions">
            <input type="file" id="avatar-file" accept="image/*" hidden>
            <button type="button" class="admin-btn" id="btn-choose-avatar">选择图片</button>
            <button type="button" class="admin-btn primary" id="btn-upload-avatar">上传头像</button>
          </div>
        </div>
        <div class="account-hint" id="avatar-hint">支持 png / jpg / gif / webp，建议使用正方形图片</div>
      </section>
      <section class="account-block">
        <div class="account-block-title">账号与密码</div>
        <label class="account-field"><span>账号</span><input class="admin-field" id="acc-username" type="text" autocomplete="username"></label>
        <label class="account-field"><span>当前密码</span><input class="admin-field" id="acc-current" type="password" autocomplete="current-password"></label>
        <label class="account-field"><span>新密码</span><input class="admin-field" id="acc-new" type="password" autocomplete="new-password" placeholder="留空则不修改"></label>
        <label class="account-field"><span>确认新密码</span><input class="admin-field" id="acc-confirm" type="password" autocomplete="new-password"></label>
        <div class="account-hint" id="acc-hint"></div>
        <div class="account-actions"><button type="button" class="admin-btn primary" id="btn-save-account">保存账户设置</button></div>
      </section>
    </div>
  </div>
</div>
<script>
(function () {
  var BT = String.fromCharCode(96);
  var currentSlug = "";
  var allNotes = [];
  var previewTimer = null;

  var el = function (id) { return document.getElementById(id); };
  var editor = el("editor");
  var preview = el("preview");
  var statusEl = el("status");

  function today() {
    var d = new Date();
    var m = ("0" + (d.getMonth() + 1)).slice(-2);
    var day = ("0" + d.getDate()).slice(-2);
    return d.getFullYear() + "-" + m + "-" + day;
  }

  function setStatus(text, kind) {
    statusEl.textContent = text;
    statusEl.className = kind || "";
  }

  function renderList() {
    var keyword = el("filter").value.trim().toLowerCase();
    var listEl = el("note-list");
    var items = allNotes.filter(function (n) {
      if (!keyword) return true;
      return (n.title || "").toLowerCase().indexOf(keyword) >= 0 || (n.slug || "").toLowerCase().indexOf(keyword) >= 0;
    });
    listEl.innerHTML = "";
    if (!items.length) {
      var empty = document.createElement("li");
      empty.className = "empty";
      empty.textContent = allNotes.length ? "没有匹配的文章" : "还没有文章，点右上角新建";
      listEl.appendChild(empty);
      return;
    }
    items.forEach(function (n) {
      var li = document.createElement("li");
      li.setAttribute("data-slug", n.slug);
      if (n.slug === currentSlug) li.className = "active";
      var title = document.createElement("span");
      title.className = "item-title";
      title.textContent = n.title || n.slug;
      var meta = document.createElement("span");
      meta.className = "item-meta";
      var dateSpan = document.createElement("span");
      dateSpan.textContent = n.date || "未定日期";
      meta.appendChild(dateSpan);
      if (n.draft) {
        var draft = document.createElement("span");
        draft.className = "draft-badge";
        draft.textContent = "草稿";
        meta.appendChild(draft);
      }
      li.appendChild(title);
      li.appendChild(meta);
      li.addEventListener("click", function () { loadNote(n.slug); });
      listEl.appendChild(li);
    });
  }

  function loadList() {
    return fetch("/admin/api/list")
      .then(function (r) {
        if (r.status === 401) { location.href = "/admin"; throw new Error("unauthorized"); }
        return r.json();
      })
      .then(function (data) {
        allNotes = (data && data.notes) || [];
        renderList();
        return allNotes;
      })
      .catch(function () { setStatus("读取文章列表失败", "err"); });
  }

  function fillForm(note) {
    el("f-title").value = note.title || "";
    el("f-slug").value = note.slug || "";
    el("f-date").value = note.date || today();
    el("f-lang").value = note.lang || "zh_CN";
    el("f-i18n-key").value = note.i18nKey || "";
    el("f-draft").checked = !!note.draft;
    el("f-pin").checked = !!note.pin;
    el("f-tags").value = (note.tags || []).join(", ");
    el("f-summary").value = note.summary || "";
    editor.value = note.body || "";
    currentSlug = note.isNew ? "" : (note.slug || "");
  }

  function loadNote(slug) {
    fetch("/admin/api/note?slug=" + encodeURIComponent(slug))
      .then(function (r) { return r.json(); })
      .then(function (note) {
        if (note && note.error) { setStatus(note.error, "err"); return; }
        fillForm(note);
        renderList();
        schedulePreview();
        setStatus("已载入：" + (note.title || slug), "");
      })
      .catch(function () { setStatus("载入失败", "err"); });
  }

  function newNote() {
    currentSlug = "";
    fillForm({ isNew: true, title: "", slug: "", date: today(), lang: "zh_CN", i18nKey: "", tags: [], summary: "", body: "", draft: false, pin: false });
    renderList();
    preview.innerHTML = "";
    el("f-title").focus();
    setStatus("新文章 · 填写标题后即可保存", "");
  }

  function schedulePreview() {
    clearTimeout(previewTimer);
    previewTimer = setTimeout(renderPreview, 260);
  }

  function renderPreview() {
    fetch("/admin/api/preview", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ body: editor.value })
    })
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (data && typeof data.html === "string") {
          preview.innerHTML = data.html;
        }
      })
      .catch(function () {});
  }

  function insert(text, back) {
    var start = editor.selectionStart;
    var end = editor.selectionEnd;
    var value = editor.value;
    var selected = value.slice(start, end);
    editor.value = value.slice(0, start) + text + selected + (back || "") + value.slice(end);
    editor.focus();
    var caretStart = start + text.length;
    editor.selectionStart = caretStart;
    editor.selectionEnd = caretStart + selected.length;
    schedulePreview();
  }

  function prefixLines(prefix) {
    var start = editor.selectionStart;
    var end = editor.selectionEnd;
    var value = editor.value;
    var lineStart = value.lastIndexOf("\n", start - 1) + 1;
    var block = value.slice(lineStart, end);
    var lines = block.split("\n");
    var changed = lines.map(function (line) { return prefix + line; }).join("\n");
    editor.value = value.slice(0, lineStart) + changed + value.slice(end);
    editor.focus();
    editor.selectionStart = lineStart;
    editor.selectionEnd = lineStart + changed.length;
    schedulePreview();
  }

  function applyAction(action) {
    switch (action) {
      case "h2": prefixLines("## "); break;
      case "h3": prefixLines("### "); break;
      case "bold": insert("**", "**"); break;
      case "italic": insert("*", "*"); break;
      case "quote": prefixLines("> "); break;
      case "ul": prefixLines("- "); break;
      case "ol": prefixLines("1. "); break;
      case "code": insert(BT, BT); break;
      case "codeblock": insert(BT + BT + BT + "\n", "\n" + BT + BT + BT); break;
      case "link": insert("[", "](https://)"); break;
      case "image": insert("![", "](https://)"); break;
      case "hr": insert("\n---\n", ""); break;
    }
  }

  function collect() {
    var tags = el("f-tags").value.split(",").map(function (s) { return s.trim(); }).filter(function (s) { return s.length > 0; });
    return {
      originalSlug: currentSlug || "",
      slug: el("f-slug").value.trim(),
      title: el("f-title").value.trim(),
      date: el("f-date").value.trim(),
      tags: tags,
      summary: el("f-summary").value.trim(),
      lang: el("f-lang").value,
      i18nKey: el("f-i18n-key").value.trim(),
      draft: el("f-draft").checked,
      pin: el("f-pin").checked,
      body: editor.value
    };
  }

  function save() {
    var payload = collect();
    if (!payload.title) { setStatus("请先填写标题", "err"); el("f-title").focus(); return; }
    setStatus("保存中…", "");
    fetch("/admin/api/save", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    })
      .then(function (r) { return r.json().then(function (d) { return { ok: r.ok, data: d }; }); })
      .then(function (res) {
        if (!res.ok) {
          setStatus((res.data && res.data.error) || "保存失败", "err");
          return;
        }
        currentSlug = res.data.slug;
        el("f-slug").value = currentSlug;
        setStatus("已保存并发布 → " + res.data.url, "ok");
        loadList();
      })
      .catch(function () { setStatus("网络错误，保存失败", "err"); });
  }

  editor.addEventListener("input", schedulePreview);
  el("toolbar").addEventListener("click", function (event) {
    var btn = event.target.closest(".admin-tool");
    if (btn) applyAction(btn.getAttribute("data-action"));
  });
  el("btn-new").addEventListener("click", newNote);
  el("btn-save").addEventListener("click", save);
  el("filter").addEventListener("input", renderList);
  el("btn-theme").addEventListener("click", function () {
    var order = ["system", "light", "dark"];
    var idx = order.indexOf(window.__adminTheme.mode());
    window.__adminTheme.setMode(order[(idx + 1) % order.length]);
    el("btn-theme").textContent = { system: "跟随", light: "浅色", dark: "深色" }[window.__adminTheme.mode()];
  });
  el("btn-theme").textContent = { system: "跟随", light: "浅色", dark: "深色" }[window.__adminTheme.mode()];
  document.addEventListener("keydown", function (event) {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "s") {
      event.preventDefault();
      save();
    }
  });

  var accountModal = el("account-modal");

  function openAccount() {
    accountModal.hidden = false;
    el("acc-current").value = "";
    el("acc-new").value = "";
    el("acc-confirm").value = "";
    el("acc-hint").className = "account-hint";
    el("acc-hint").textContent = "";
    el("avatar-hint").className = "account-hint";
    el("avatar-hint").textContent = "支持 png / jpg / gif / webp，建议使用正方形图片";
    fetch("/admin/api/session")
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (data && data.username) el("acc-username").value = data.username;
      })
      .catch(function () {});
    fetch("/admin/api/avatar")
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (data && data.avatar) {
          el("avatar-preview").style.backgroundImage = "url('" + data.avatar + "')";
        }
      })
      .catch(function () {});
    el("site-hint").className = "account-hint";
    el("site-hint").textContent = "";
    fetch("/admin/api/site")
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (!data) return;
        el("site-name-zh").value = data.name || "";
        el("site-name-en").value = data.nameEn || "";
        el("site-slogan-zh").value = data.sloganZh || "";
        el("site-slogan-en").value = data.sloganEn || "";
        el("site-url").value = data.siteUrl || "";
        el("site-title-zh").value = data.siteTitleZh || "";
        el("site-title-en").value = data.siteTitleEn || "";
        el("site-started").value = data.startedAt || "";
        el("site-copyright").value = data.copyright || "";
        el("site-favicon").value = data.favicon || "";
        el("site-logo-text").value = data.logoText || "";
        el("site-about-url").value = data.aboutUrl || "";
        el("seo-title-zh").value = data.homeTitleZh || "";
        el("seo-title-en").value = data.homeTitleEn || "";
        el("seo-desc-zh").value = data.homeDescZh || "";
        el("seo-desc-en").value = data.homeDescEn || "";
        el("share-text").value = data.shareText || "";
        el("stats-enabled").checked = !!data.statsEnabled;
        el("comment-enabled").checked = !!data.commentEnabled;
        el("comment-provider").value = data.commentProvider || "";
        el("waline-server").value = data.walineServerUrl || "";
        el("waline-lang").value = data.walineLang || "";
        el("waline-page-size").value = data.walinePageSize || "";
        el("waline-sorting").value = data.walineSorting || "";
        el("waline-search").checked = !!data.walineSearch;
        el("waline-upload").checked = !!data.walineImageUploader;
      })
      .catch(function () {});
    el("social-hint").className = "account-hint";
    el("social-hint").textContent = "";
    fetch("/admin/api/social")
      .then(function (r) { return r.json(); })
      .then(function (data) { renderSocial((data && data.links) || []); })
      .catch(function () { renderSocial([]); });
  }

  function closeAccount() { accountModal.hidden = true; }

  el("btn-account").addEventListener("click", openAccount);
  el("account-close").addEventListener("click", closeAccount);
  accountModal.addEventListener("click", function (event) {
    if (event.target === accountModal) closeAccount();
  });

  el("btn-choose-avatar").addEventListener("click", function () { el("avatar-file").click(); });
  el("avatar-file").addEventListener("change", function () {
    var file = el("avatar-file").files && el("avatar-file").files[0];
    if (!file) return;
    var reader = new FileReader();
    reader.onload = function () { el("avatar-preview").style.backgroundImage = "url('" + reader.result + "')"; };
    reader.readAsDataURL(file);
  });

  el("btn-upload-avatar").addEventListener("click", function () {
    var hint = el("avatar-hint");
    var file = el("avatar-file").files && el("avatar-file").files[0];
    if (!file) { hint.className = "account-hint err"; hint.textContent = "请先选择一张图片"; return; }
    var formData = new FormData();
    formData.append("file", file);
    hint.className = "account-hint";
    hint.textContent = "上传中…";
    fetch("/admin/api/avatar", { method: "POST", body: formData })
      .then(function (r) { return r.json().then(function (d) { return { ok: r.ok, data: d }; }); })
      .then(function (res) {
        if (!res.ok) { hint.className = "account-hint err"; hint.textContent = (res.data && res.data.error) || "上传失败"; return; }
        hint.className = "account-hint ok";
        hint.textContent = "头像已更新，站点已重新生成";
        el("avatar-preview").style.backgroundImage = "url('" + res.data.avatar + "?t=" + Date.now() + "')";
      })
      .catch(function () { hint.className = "account-hint err"; hint.textContent = "网络错误，上传失败"; });
  });

  el("btn-save-site").addEventListener("click", function () {
    var hint = el("site-hint");
    hint.className = "account-hint";
    hint.textContent = "保存中…";
    fetch("/admin/api/site", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: el("site-name-zh").value.trim(),
        nameEn: el("site-name-en").value.trim(),
        sloganZh: el("site-slogan-zh").value.trim(),
        sloganEn: el("site-slogan-en").value.trim(),
        siteUrl: el("site-url").value.trim(),
        siteTitleZh: el("site-title-zh").value.trim(),
        siteTitleEn: el("site-title-en").value.trim(),
        startedAt: el("site-started").value.trim(),
        copyright: el("site-copyright").value.trim(),
        favicon: el("site-favicon").value.trim(),
        logoText: el("site-logo-text").value.trim(),
        aboutUrl: el("site-about-url").value.trim(),
        homeTitleZh: el("seo-title-zh").value.trim(),
        homeTitleEn: el("seo-title-en").value.trim(),
        homeDescZh: el("seo-desc-zh").value.trim(),
        homeDescEn: el("seo-desc-en").value.trim(),
        shareText: el("share-text").value.trim(),
        statsEnabled: el("stats-enabled").checked,
        commentEnabled: el("comment-enabled").checked,
        commentProvider: el("comment-provider").value.trim(),
        walineServerUrl: el("waline-server").value.trim(),
        walineLang: el("waline-lang").value.trim(),
        walinePageSize: parseInt(el("waline-page-size").value, 10) || 0,
        walineSorting: el("waline-sorting").value.trim(),
        walineSearch: el("waline-search").checked,
        walineImageUploader: el("waline-upload").checked
      })
    })
      .then(function (r) { return r.json().then(function (d) { return { ok: r.ok, data: d }; }); })
      .then(function (res) {
        if (!res.ok) { hint.className = "account-hint err"; hint.textContent = (res.data && res.data.error) || "保存失败"; return; }
        hint.className = "account-hint ok";
        hint.textContent = "设置已更新";
      })
      .catch(function () { hint.className = "account-hint err"; hint.textContent = "网络错误，保存失败"; });
  });

  var socialPlatforms = [
    { value: "github", label: "GitHub" },
    { value: "x", label: "X (Twitter)" },
    { value: "youtube", label: "YouTube" },
    { value: "bilibili", label: "Bilibili" },
    { value: "bluesky", label: "Bluesky" },
    { value: "discord", label: "Discord" },
    { value: "email", label: "Email" },
    { value: "gitlab", label: "GitLab" },
    { value: "instagram", label: "Instagram" },
    { value: "mastodon", label: "Mastodon" },
    { value: "qq", label: "QQ" },
    { value: "reddit", label: "Reddit" },
    { value: "telegram", label: "Telegram" },
    { value: "threads", label: "Threads" },
    { value: "twitch", label: "Twitch" }
  ];

  function socialOptionsHtml(selected) {
    var opts = '<option value="">选择平台</option>';
    socialPlatforms.forEach(function (p) {
      opts += '<option value="' + p.value + '"' + (selected === p.value ? " selected" : "") + ">" + p.label + "</option>";
    });
    return opts;
  }

  function renderSocial(links) {
    var list = el("social-list");
    list.innerHTML = "";
    if (!links || links.length === 0) {
      addSocialRow();
      return;
    }
    links.forEach(function (l) { addSocialRow(l.type, l.url); });
  }

  function addSocialRow(type, url) {
    var list = el("social-list");
    var row = document.createElement("div");
    row.className = "social-row";
    row.innerHTML =
      '<select class="admin-field">' + socialOptionsHtml(type || "") + "</select>" +
      '<input class="admin-field" type="text" placeholder="链接地址，如 https://github.com/your-name">' +
      '<button type="button" class="social-del" title="删除">×</button>';
    row.querySelector("input").value = url || "";
    row.querySelector(".social-del").addEventListener("click", function () {
      row.remove();
      if (el("social-list").children.length === 0) addSocialRow();
    });
    list.appendChild(row);
  }

  el("btn-add-social").addEventListener("click", function () { addSocialRow(); });

  el("btn-save-social").addEventListener("click", function () {
    var hint = el("social-hint");
    var rows = el("social-list").querySelectorAll(".social-row");
    var links = [];
    rows.forEach(function (row) {
      var t = row.querySelector("select").value.trim();
      var u = row.querySelector("input").value.trim();
      if (t && u) links.push({ type: t, url: u });
    });
    hint.className = "account-hint";
    hint.textContent = "保存中…";
    fetch("/admin/api/social", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ links: links })
    })
      .then(function (r) { return r.json().then(function (d) { return { ok: r.ok, data: d }; }); })
      .then(function (res) {
        if (!res.ok) { hint.className = "account-hint err"; hint.textContent = (res.data && res.data.error) || "保存失败"; return; }
        hint.className = "account-hint ok";
        hint.textContent = "社交链接已更新";
      })
      .catch(function () { hint.className = "account-hint err"; hint.textContent = "网络错误，保存失败"; });
  });

  el("btn-save-account").addEventListener("click", function () {
    var hint = el("acc-hint");
    var username = el("acc-username").value.trim();
    var current = el("acc-current").value;
    var next = el("acc-new").value;
    var confirm = el("acc-confirm").value;
    if (!username) { hint.className = "account-hint err"; hint.textContent = "账号不能为空"; return; }
    if (next !== confirm) { hint.className = "account-hint err"; hint.textContent = "两次输入的新密码不一致"; return; }
    hint.className = "account-hint";
    hint.textContent = "保存中…";
    fetch("/admin/api/account", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: username, currentPassword: current, newPassword: next })
    })
      .then(function (r) { return r.json().then(function (d) { return { ok: r.ok, data: d }; }); })
      .then(function (res) {
        if (!res.ok) { hint.className = "account-hint err"; hint.textContent = (res.data && res.data.error) || "保存失败"; return; }
        hint.className = "account-hint ok";
        hint.textContent = "已保存，下次登录请使用新账号密码";
        el("acc-current").value = "";
        el("acc-new").value = "";
        el("acc-confirm").value = "";
      })
      .catch(function () { hint.className = "account-hint err"; hint.textContent = "网络错误，保存失败"; });
  });

  el("btn-logout").addEventListener("click", function () {
    fetch("/admin/api/logout", { method: "POST" })
      .then(function () { location.href = "/admin"; })
      .catch(function () { location.href = "/admin"; });
  });

  loadList().then(function (notes) {
    if (notes && notes.length) {
      loadNote(notes[0].slug);
    } else {
      newNote();
    }
  });
})();
</script>
</body>
</html>
`

const loginTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Daybook 写作台 · 登录</title>
<script>
(function () {
  var root = document.documentElement;
  var mode = "system";
  var palette = "default";
  try {
    mode = localStorage.getItem("theme-mode") || "system";
    palette = localStorage.getItem("palette") || "default";
  } catch (e) {}
  var resolved = mode;
  if (mode === "system") {
    resolved = (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) ? "dark" : "light";
  }
  root.setAttribute("data-theme", resolved === "dark" ? "dark" : "light");
  root.setAttribute("data-palette", palette === "warm" ? "warm" : "default");
})();
</script>
{{FONT_LINKS}}<link rel="stylesheet" href="{{GLOBAL_CSS}}">
<style>
* { box-sizing: border-box; }
html, body { height: 100%; }
body {
  margin: 0;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-page);
  color: var(--color-text);
  font-family: var(--font-ui);
  line-height: 1.6;
}
.admin-login-card {
  width: 100%;
  max-width: 360px;
  padding: 28px 24px;
  border: 1px solid var(--color-line);
  border-radius: 14px;
  background: var(--color-paper);
  box-shadow: 0 12px 40px rgba(0, 0, 0, 0.12);
}
.admin-login-title { margin: 0 0 4px; font-size: 20px; }
.admin-login-sub { margin: 0 0 20px; font-size: 13px; color: var(--color-muted); }
.admin-login-field { display: block; margin-bottom: 14px; }
.admin-login-field span { display: block; margin-bottom: 6px; font-size: 13px; }
.admin-login-field input {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid var(--color-line);
  border-radius: 8px;
  background: var(--color-page);
  color: var(--color-text);
  font-family: var(--font-ui);
  font-size: 14px;
}
.admin-login-field input:focus { border-color: var(--color-accent); outline: none; }
.admin-login-submit {
  width: 100%;
  margin-top: 6px;
  padding: 10px 12px;
  border: none;
  border-radius: 8px;
  background: var(--color-accent);
  color: #fff;
  font-size: 15px;
  cursor: pointer;
}
.admin-login-submit:disabled { opacity: 0.6; cursor: default; }
.admin-login-error { min-height: 20px; margin-top: 10px; font-size: 13px; color: #e5484d; }
</style>
</head>
<body>
<form class="admin-login-card" id="login-form">
  <h1 class="admin-login-title">Daybook 写作台</h1>
  <p class="admin-login-sub">请登录后继续</p>
  <label class="admin-login-field"><span>账号</span><input id="login-username" type="text" autocomplete="username" autofocus></label>
  <label class="admin-login-field"><span>密码</span><input id="login-password" type="password" autocomplete="current-password"></label>
  <button type="submit" class="admin-login-submit" id="login-submit">登录</button>
  <div class="admin-login-error" id="login-error"></div>
</form>
<script>
(function () {
  var form = document.getElementById("login-form");
  var userInput = document.getElementById("login-username");
  var passInput = document.getElementById("login-password");
  var submit = document.getElementById("login-submit");
  var error = document.getElementById("login-error");
  form.addEventListener("submit", function (e) {
    e.preventDefault();
    error.textContent = "";
    var username = userInput.value.trim();
    var password = passInput.value;
    if (!username || !password) {
      error.textContent = "请输入账号和密码";
      return;
    }
    submit.disabled = true;
    fetch("/admin/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: username, password: password })
    }).then(function (r) {
      return r.json().then(function (data) { return { ok: r.ok, data: data }; });
    }).then(function (res) {
      if (res.ok && res.data && res.data.ok) {
        location.href = "/admin";
        return;
      }
      error.textContent = (res.data && res.data.error) || "账号或密码不正确";
      submit.disabled = false;
    }).catch(function () {
      error.textContent = "网络错误，请重试";
      submit.disabled = false;
    });
  });
})();
</script>
</body>
</html>
`

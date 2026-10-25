// codeview 前端：原生 JS，无需构建。
// 路由：/  /tree/<目录>  /blob/<文件>#L10
(function () {
  "use strict";

  const SITE = document.title;

  // ---------- 多语言 ----------
  // 默认语言由服务端 -lang 决定（写在 <html data-default-lang>），浏览器里切换后记在 localStorage。

  const I18N = {
    en: {
      justNow: "just now", minAgo: "{0} min ago", hoursAgo: "{0} hours ago", daysAgo: "{0} days ago",
      emptyDir: "Empty directory", copyPath: "Copy path", items: "{0} items", lines: "{0} lines",
      changes: "Changes", changesTitle: "Compared with the first time this page opened the file",
      diffToggle: "Highlight", diffToggleTitle: "Highlight changes since this file was first opened (d)",
      diffReset: "Reset", diffResetTitle: "Use the current content as the new baseline",
      preview: "Preview", source: "Source", copyContent: "Copy contents", download: "Download",
      outline: "Outline", outlineTitle: "Show / hide outline (o)",
      tooLarge: "File is too large ({0}) to display.", viewRaw: "View raw file",
      binary: "Binary file, cannot be previewed.",
      gone: "This file was deleted or renamed. Showing its last content.",
      dirGone: "This directory was deleted", updated: "Updated",
      searching: "Searching…", noGrep: "No files contain “{0}”", noMatch: "No matching files. Type /{0} to search file contents",
      toggleTree: "Show / hide file tree (b)", searchPlaceholder: "Go to file (t)  ·  start with / to search contents",
      toggleTheme: "Color theme", theme: "Color theme", themeAuto: "Auto (follow system)", themeDark: "Dark", themeLight: "Light", toggleLang: "切换到中文", langLabel: "中",
      files: "Files", collapseAll: "Collapse all", loading: "Loading…",
    },
    zh: {
      justNow: "刚刚", minAgo: "{0} 分钟前", hoursAgo: "{0} 小时前", daysAgo: "{0} 天前",
      emptyDir: "空目录", copyPath: "复制路径", items: "{0} 项", lines: "{0} 行",
      changes: "改动", changesTitle: "相比本次首次打开时",
      diffToggle: "着色", diffToggleTitle: "着色显示相比本次首次打开时的改动 (d)",
      diffReset: "重置对比", diffResetTitle: "以当前内容作为新的对比基准",
      preview: "预览", source: "源码", copyContent: "复制全部内容", download: "下载",
      outline: "大纲", outlineTitle: "显示/隐藏大纲 (o)",
      tooLarge: "文件太大（{0}），不在线显示。", viewRaw: "查看原始文件",
      binary: "二进制文件，无法预览。",
      gone: "文件已被删除或改名，下面是删除前的内容",
      dirGone: "目录已被删除", updated: "已更新",
      searching: "搜索中…", noGrep: "没有找到包含 “{0}” 的文件", noMatch: "没有匹配的文件，输入 /{0} 可全文搜索",
      toggleTree: "显示/隐藏文件树 (b)", searchPlaceholder: "查找文件 (t)   ·   输入 / 开头做全文搜索",
      toggleTheme: "配色主题", theme: "配色主题", themeAuto: "自动（跟随系统）", themeDark: "深色", themeLight: "浅色", toggleLang: "Switch to English", langLabel: "EN",
      files: "文件", collapseAll: "全部折叠", loading: "加载中…",
    },
  };
  let lang = document.documentElement.getAttribute("lang") === "zh" ? "zh" : "en";
  const locale = () => (lang === "zh" ? "zh-CN" : "en-US");
  function t(key, ...args) {
    const s = (I18N[lang] && I18N[lang][key]) || I18N.en[key] || key;
    return s.replace(/\{(\d)\}/g, (_, i) => args[i]);
  }
  function applyStaticText() {
    document.documentElement.setAttribute("lang", lang);
    document.querySelectorAll("[data-i18n]").forEach((e) => (e.textContent = t(e.dataset.i18n)));
    document.querySelectorAll("[data-i18n-title]").forEach((e) => (e.title = t(e.dataset.i18nTitle)));
    document.querySelectorAll("[data-i18n-placeholder]").forEach((e) => (e.placeholder = t(e.dataset.i18nPlaceholder)));
    const lb = document.getElementById("toggle-lang");
    if (lb) lb.textContent = t("langLabel");
  }
  applyStaticText();
  const $ = (sel, el) => (el || document).querySelector(sel);
  const main = $("#main");
  const treeEl = $("#tree");
  const outlineEl = $("#outline");
  const outlineList = $("#outline-list");

  // ---------- 工具函数 ----------

  const enc = (p) => p.split("/").map(encodeURIComponent).join("/");
  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  const dirname = (p) => (p.includes("/") ? p.slice(0, p.lastIndexOf("/")) : "");
  const basename = (p) => p.slice(p.lastIndexOf("/") + 1);

  function joinPath(base, rel) {
    const parts = base ? base.split("/") : [];
    for (const seg of rel.split("/")) {
      if (seg === "" || seg === ".") continue;
      if (seg === "..") parts.pop();
      else parts.push(seg);
    }
    return parts.join("/");
  }

  function fmtSize(n) {
    if (n < 1024) return n + " B";
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
    return (n / 1024 / 1024).toFixed(1) + " MB";
  }

  function fmtTime(s) {
    const d = new Date(s);
    const diff = (Date.now() - d) / 1000;
    if (diff < 60) return t("justNow");
    if (diff < 3600) return t("minAgo", Math.floor(diff / 60));
    if (diff < 86400) return t("hoursAgo", Math.floor(diff / 3600));
    if (diff < 86400 * 30) return t("daysAgo", Math.floor(diff / 86400));
    return d.toLocaleDateString(locale());
  }

  async function api(url) {
    const res = await fetch(url);
    const data = await res.json().catch(() => ({ error: res.statusText }));
    if (!res.ok) {
      const err = new Error(data.error || res.statusText);
      err.isDir = !!data.isDir;
      err.status = res.status;
      throw err;
    }
    return data;
  }

  const store = {
    get(k) { try { return localStorage.getItem(k); } catch (e) { return null; } },
    set(k, v) { try { localStorage.setItem(k, v); } catch (e) {} },
  };

  const ICON = {
    dir: '<svg class="ic ic-dir" viewBox="0 0 16 16" width="16" height="16"><path d="M1.75 1A1.75 1.75 0 0 0 0 2.75v10.5C0 14.216.784 15 1.75 15h12.5A1.75 1.75 0 0 0 16 13.25v-8.5A1.75 1.75 0 0 0 14.25 3H7.5a.25.25 0 0 1-.2-.1l-.9-1.2C6.07 1.26 5.55 1 5 1H1.75Z"/></svg>',
    dirOpen: '<svg class="ic ic-dir" viewBox="0 0 16 16" width="16" height="16"><path d="M.513 1.513A1.75 1.75 0 0 1 1.75 1h3.5c.55 0 1.07.26 1.4.7l.9 1.2a.25.25 0 0 0 .2.1H13a1 1 0 0 1 1 1v.5H2.75a.75.75 0 0 0 0 1.5h11.978a1 1 0 0 1 .994 1.117L15 13.25A1.75 1.75 0 0 1 13.25 15H1.75A1.75 1.75 0 0 1 0 13.25V2.75c0-.464.184-.91.513-1.237Z"/></svg>',
    file: '<svg class="ic ic-file" viewBox="0 0 16 16" width="16" height="16"><path d="M2 1.75C2 .784 2.784 0 3.75 0h6.586c.464 0 .909.184 1.237.513l2.914 2.914c.329.328.513.773.513 1.237v9.586A1.75 1.75 0 0 1 13.25 16h-9.5A1.75 1.75 0 0 1 2 14.25Zm1.75-.25a.25.25 0 0 0-.25.25v12.5c0 .138.112.25.25.25h9.5a.25.25 0 0 0 .25-.25V6h-2.75A1.75 1.75 0 0 1 9 4.25V1.5Zm6.75.062V4.25c0 .138.112.25.25.25h2.688l-.011-.013-2.914-2.914-.013-.011Z"/></svg>',
    chevron: '<svg class="chev" viewBox="0 0 16 16" width="12" height="12"><path d="M6.22 3.22a.75.75 0 0 1 1.06 0l4.25 4.25a.75.75 0 0 1 0 1.06l-4.25 4.25a.751.751 0 0 1-1.042-.018.751.751 0 0 1-.018-1.042L9.94 8 6.22 4.28a.75.75 0 0 1 0-1.06Z"/></svg>',
  };

  // 常见扩展名给文件图标上个色，方便扫一眼
  const EXT_COLOR = {
    md: "#519aba", go: "#00add8", py: "#3572a5", sh: "#89e051", bash: "#89e051", zsh: "#89e051",
    yaml: "#cb171e", yml: "#cb171e", json: "#cbcb41", js: "#f1e05a", ts: "#3178c6", tf: "#844fba",
    html: "#e34c26", css: "#563d7c", sql: "#e38c00", toml: "#9c4221", java: "#b07219", rs: "#dea584",
    png: "#a074c4", jpg: "#a074c4", jpeg: "#a074c4", svg: "#ffb13b", gif: "#a074c4", pdf: "#b30b00",
  };
  function fileIcon(name) {
    const ext = name.includes(".") ? name.split(".").pop().toLowerCase() : "";
    const color = EXT_COLOR[ext] || (/^dockerfile/i.test(name) ? "#384d54" : /^makefile$/i.test(name) ? "#427819" : "");
    return color ? ICON.file.replace('class="ic ic-file"', `class="ic ic-file" style="fill:${color}"`) : ICON.file;
  }

  // ---------- 左侧目录树 ----------

  const dirCache = new Map(); // path -> entries
  const expanded = new Set(JSON.parse(store.get("cv-expanded") || "[]"));
  let currentPath = "";

  async function listDir(p) {
    if (!dirCache.has(p)) {
      const data = await api("/api/tree?readme=0&path=" + encodeURIComponent(p));
      dirCache.set(p, data.entries);
    }
    return dirCache.get(p);
  }

  function saveExpanded() { store.set("cv-expanded", JSON.stringify([...expanded])); }

  function nodeHTML(e, depth) {
    const pad = `style="padding-left:${8 + depth * 14}px"`;
    if (e.type === "dir") {
      return `<div class="node" data-dir="${esc(e.path)}">
        <a class="row" href="/tree/${esc(enc(e.path))}" data-nav data-path="${esc(e.path)}" ${pad}>${ICON.chevron}${ICON.dir}<span>${esc(e.name)}</span></a>
        <div class="children" hidden></div></div>`;
    }
    return `<div class="node"><a class="row file" href="/blob/${esc(enc(e.path))}" data-nav data-path="${esc(e.path)}" ${pad}><span class="chev-space"></span>${fileIcon(e.name)}<span>${esc(e.name)}</span></a></div>`;
  }

  async function renderChildren(container, p, depth) {
    try {
      const entries = await listDir(p);
      container.innerHTML = entries.length ? entries.map((e) => nodeHTML(e, depth)).join("") : `<div class="empty" style="padding-left:${22 + depth * 14}px">${t("emptyDir")}</div>`;
    } catch (err) {
      container.innerHTML = `<div class="empty">${esc(err.message)}</div>`;
    }
    // 恢复之前展开的子目录
    for (const node of container.querySelectorAll(":scope > .node[data-dir]")) {
      if (expanded.has(node.dataset.dir)) await setOpen(node, true, depth + 1);
    }
    markActive();
  }

  async function setOpen(node, open, depth) {
    const children = node.querySelector(":scope > .children");
    const row = node.querySelector(":scope > .row");
    node.classList.toggle("open", open);
    row.querySelector(".ic-dir").outerHTML = open ? ICON.dirOpen : ICON.dir;
    children.hidden = !open;
    if (open) {
      expanded.add(node.dataset.dir);
      if (!children.dataset.loaded) {
        children.dataset.loaded = "1";
        await renderChildren(children, node.dataset.dir, depth);
      }
    } else {
      expanded.delete(node.dataset.dir);
    }
    saveExpanded();
    updateWatch();
  }

  function depthOf(p) { return p ? p.split("/").length : 0; }

  // 展开到目标路径的所有祖先目录
  async function revealInTree(p) {
    const parts = p ? p.split("/") : [];
    let acc = "";
    for (let i = 0; i < parts.length; i++) {
      acc = acc ? acc + "/" + parts[i] : parts[i];
      const node = treeEl.querySelector(`.node[data-dir="${CSS.escape(acc)}"]`);
      if (!node) break;
      if (i < parts.length - 1 || currentIsDir) {
        if (!node.classList.contains("open")) await setOpen(node, true, depthOf(acc));
      }
    }
    markActive();
    const active = treeEl.querySelector(".row.active");
    if (active) {
      const r = active.getBoundingClientRect(), tr = treeEl.getBoundingClientRect();
      if (r.top < tr.top || r.bottom > tr.bottom) active.scrollIntoView({ block: "center" });
    }
  }

  function markActive() {
    treeEl.querySelectorAll(".row.active").forEach((a) => a.classList.remove("active"));
    const a = treeEl.querySelector(`.row[data-path="${CSS.escape(currentPath)}"]`);
    if (a) a.classList.add("active");
  }

  treeEl.addEventListener("click", (ev) => {
    const chev = ev.target.closest(".chev");
    const row = ev.target.closest(".row");
    if (!row) return;
    const node = row.parentElement;
    // 点箭头只展开/折叠，不跳转
    if (chev && node.dataset.dir !== undefined) {
      ev.preventDefault();
      ev.stopPropagation();
      setOpen(node, !node.classList.contains("open"), depthOf(node.dataset.dir));
      return;
    }
    // 点击已打开的当前目录则折叠
    if (node.dataset.dir !== undefined && node.dataset.dir === currentPath && node.classList.contains("open")) {
      ev.preventDefault();
      ev.stopPropagation();
      setOpen(node, false, 0);
    }
  });

  $("#collapse-all").addEventListener("click", () => {
    expanded.clear();
    saveExpanded();
    treeEl.querySelectorAll(".node.open").forEach((n) => {
      n.classList.remove("open");
      n.querySelector(":scope > .children").hidden = true;
      n.querySelector(":scope > .row .ic-dir").outerHTML = ICON.dir;
    });
  });

  // ---------- 右侧大纲 ----------

  function renderOutline(items, kind) {
    if (!items || !items.length) {
      outlineEl.hidden = true;
      return;
    }
    const min = Math.min(...items.map((i) => i.level));
    outlineList.innerHTML = items.map((i) => {
      const href = kind === "markdown" ? "#" + encodeURIComponent(i.id) : "#L" + i.line;
      return `<a href="${href}" class="ol-item" style="padding-left:${8 + (i.level - min) * 12}px" title="${esc(i.text)}"><span class="ol-text">${esc(i.text)}</span>${i.line ? `<span class="ol-line">${i.line}</span>` : ""}</a>`;
    }).join("");
    outlineEl.hidden = store.get("cv-outline") === "0";
  }

  outlineList.addEventListener("click", (ev) => {
    const a = ev.target.closest("a");
    if (!a) return;
    ev.preventDefault();
    const hash = a.getAttribute("href");
    history.replaceState(history.state, "", location.pathname + hash);
    scrollToHash();
  });

  // ---------- 主内容 ----------

  let currentIsDir = true;
  let mdMode = store.get("cv-md-mode") || "preview";

  function crumbs(p, isDir) {
    const parts = p ? p.split("/") : [];
    let acc = "";
    let html = `<a href="/" data-nav class="crumb-root">${esc(SITE)}</a>`;
    parts.forEach((seg, i) => {
      acc = acc ? acc + "/" + seg : seg;
      const last = i === parts.length - 1;
      html += `<span class="sep">/</span>`;
      html += last ? `<strong>${esc(seg)}</strong>` : `<a href="/tree/${esc(enc(acc))}" data-nav>${esc(seg)}</a>`;
    });
    return `<div class="crumbs">${html}<button class="icon-btn small copy-path" data-copy="${esc(p)}" title="${t("copyPath")}">${COPY_ICON}</button></div>`;
  }

  const COPY_ICON = '<svg viewBox="0 0 16 16" width="14" height="14"><path d="M0 6.75C0 5.784.784 5 1.75 5h1.5a.75.75 0 0 1 0 1.5h-1.5a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-1.5a.75.75 0 0 1 1.5 0v1.5A1.75 1.75 0 0 1 9.25 16h-7.5A1.75 1.75 0 0 1 0 14.25Z"/><path d="M5 1.75C5 .784 5.784 0 6.75 0h7.5C15.216 0 16 .784 16 1.75v7.5A1.75 1.75 0 0 1 14.25 11h-7.5A1.75 1.75 0 0 1 5 9.25Zm1.75-.25a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-7.5a.25.25 0 0 0-.25-.25Z"/></svg>';

  async function showDir(p) {
    const data = await api("/api/tree?path=" + encodeURIComponent(p));
    dirCache.set(p, data.entries);
    currentIsDir = true;
    const rows = [];
    if (p) rows.push(`<tr><td colspan="3"><a href="/tree/${esc(enc(dirname(p)))}" data-nav class="up">..</a></td></tr>`);
    for (const e of data.entries) {
      const href = (e.type === "dir" ? "/tree/" : "/blob/") + esc(enc(e.path));
      rows.push(`<tr>
        <td class="name"><a href="${href}" data-nav>${e.type === "dir" ? ICON.dir : fileIcon(e.name)}<span>${esc(e.name)}</span></a></td>
        <td class="size">${e.type === "dir" ? "" : fmtSize(e.size)}</td>
        <td class="time" title="${esc(new Date(e.mtime).toLocaleString(locale()))}">${fmtTime(e.mtime)}</td></tr>`);
    }
    let html = crumbs(p, true);
    html += `<div class="box"><div class="box-head"><span>${t("items", data.entries.length)}</span></div>
      <table class="listing">${rows.join("") || `<tr><td class="empty">${t("emptyDir")}</td></tr>`}</table></div>`;
    if (data.readme) {
      html += `<div class="box readme"><div class="box-head">${ICON.file}<a href="/blob/${esc(enc(data.readme.path))}" data-nav>${esc(data.readme.name)}</a></div>
        <div class="markdown-body">${data.readme.kind === "markdown" ? data.readme.html : data.readme.html || ""}</div></div>`;
    }
    main.innerHTML = html;
    if (data.readme && data.readme.kind === "markdown") {
      fixMarkdown(main.querySelector(".markdown-body"), dirname(data.readme.path));
      renderOutline(data.readme.outline, "markdown");
    } else {
      renderOutline(null);
    }
  }

  async function showFile(p, live) {
    const f = await api("/api/file?path=" + encodeURIComponent(p));
    currentIsDir = false;
    // 对比基准：本次打开页面后第一次看到这个文件时的内容（刷新页面即重置）
    if (typeof f.text === "string" && !baselines.has(p)) baselines.set(p, f.text);
    const base = baselines.get(p);
    const diff = typeof f.text === "string" && base !== undefined && base !== f.text ? lineDiff(base.split("\n"), f.text.split("\n")) : null;
    const raw = "/raw/" + enc(p);
    const meta = [];
    if (f.lines) meta.push(t("lines", f.lines));
    meta.push(fmtSize(f.size));
    if (f.lang) meta.push(esc(f.lang));
    if (f.encoding && f.encoding !== "UTF-8") meta.push(esc(f.encoding));
    meta.push(`<span title="${esc(new Date(f.mtime).toLocaleString(locale()))}">${fmtTime(f.mtime)}</span>`);
    if (diff) {
      meta.push(`<span class="diff-badge" title="${t("changesTitle")}">${t("changes")} <b class="n-add">+${diff.nAdd}</b> <b class="n-mod">~${diff.nMod}</b> <b class="n-del">−${diff.nDel}</b></span>`);
    }

    const btns = [];
    if (diff) {
      btns.push(`<button class="btn ${diffOn ? "on" : ""}" id="diff-toggle" title="${t("diffToggleTitle")}">${t("diffToggle")}</button>`);
      btns.push(`<button class="btn" id="diff-reset" title="${t("diffResetTitle")}">${t("diffReset")}</button>`);
    }
    if (f.kind === "markdown") {
      btns.push(`<div class="seg"><button data-md="preview" class="${mdMode === "preview" ? "on" : ""}">${t("preview")}</button><button data-md="source" class="${mdMode === "source" ? "on" : ""}">${t("source")}</button></div>`);
    }
    if (f.kind === "code" || f.kind === "markdown") btns.push(`<button class="btn" id="copy-content" title="${t("copyContent")}">${COPY_ICON}</button>`);
    btns.push(`<a class="btn" href="${raw}" target="_blank" rel="noopener">Raw</a>`);
    btns.push(`<a class="btn" href="${raw}?download=1">${t("download")}</a>`);
    if (!outlineEl.hidden || (f.outline && f.outline.length)) {
      btns.push(`<button class="btn" id="toggle-outline" title="${t("outlineTitle")}">${t("outline")}</button>`);
    }

    let body;
    switch (f.kind) {
      case "markdown":
        body = `<div class="markdown-body md-preview" ${mdMode === "source" ? "hidden" : ""}>${f.html}</div>
                <div class="code md-source" ${mdMode === "preview" ? "hidden" : ""}>${f.source}</div>`;
        break;
      case "code":
        body = `<div class="code">${f.html}</div>`;
        break;
      case "image":
        body = `<div class="media"><img src="${raw}" alt="${esc(f.name)}"></div>`;
        break;
      case "pdf":
        body = `<iframe class="pdf" src="${raw}"></iframe>`;
        break;
      case "large":
        body = `<div class="notice">${t("tooLarge", fmtSize(f.size))} <a href="${raw}" target="_blank">${t("viewRaw")}</a></div>`;
        break;
      default:
        body = `<div class="notice">${t("binary")} <a href="${raw}?download=1">${t("download")}</a></div>`;
    }

    const keep = live ? { top: main.scrollTop, left: (main.querySelector(".code:not([hidden])") || {}).scrollLeft || 0 } : null;
    main.innerHTML = crumbs(p, false) + `<div class="box file">
      <div class="box-head"><span class="meta">${meta.join('<span class="dot">·</span>')}</span><span class="actions">${btns.join("")}</span></div>
      ${body}</div>`;

    if (f.kind === "markdown") fixMarkdown(main.querySelector(".markdown-body"), dirname(p));
    if (f.kind === "markdown" && mdMode === "source") renderOutline(null);
    else renderOutline(f.outline, f.kind);
    main._file = f;
    main._diff = diff;
    if (diff && diffOn) applyDiff(diff);
    if (keep) {
      main.scrollTop = keep.top;
      const box = main.querySelector(".code:not([hidden])");
      if (box) box.scrollLeft = keep.left;
      scrollToHash(false);
    }
  }

  // ---------- 改动对比 ----------

  const baselines = new Map(); // path -> 首次打开时的原文
  let diffOn = store.get("cv-diff") !== "0";

  // 行级对比：去掉公共前后缀后对中间部分做 LCS。
  // 返回新文件里的 added / modified 行号集合，以及 delBefore（新文件第 n 行之前删掉了几行）。
  function lineDiff(a, b) {
    let pre = 0;
    while (pre < a.length && pre < b.length && a[pre] === b[pre]) pre++;
    let suf = 0;
    while (suf < a.length - pre && suf < b.length - pre && a[a.length - 1 - suf] === b[b.length - 1 - suf]) suf++;
    const A = a.slice(pre, a.length - suf), B = b.slice(pre, b.length - suf);
    const n = A.length, m = B.length;
    const ops = [];
    if (n * m > 4e6) {
      // 改动太大就不细算了，整段视为替换
      for (let i = 0; i < n; i++) ops.push("d");
      for (let j = 0; j < m; j++) ops.push("a");
    } else {
      const w = m + 1;
      const L = new Int32Array((n + 1) * w); // L[i][j] = A[i:] 和 B[j:] 的 LCS 长度
      for (let i = n - 1; i >= 0; i--)
        for (let j = m - 1; j >= 0; j--)
          L[i * w + j] = A[i] === B[j] ? L[(i + 1) * w + j + 1] + 1 : Math.max(L[(i + 1) * w + j], L[i * w + j + 1]);
      let i = 0, j = 0;
      while (i < n || j < m) {
        if (i < n && j < m && A[i] === B[j]) { ops.push("e"); i++; j++; }
        else if (j >= m || (i < n && L[(i + 1) * w + j] >= L[i * w + j + 1])) { ops.push("d"); i++; }
        else { ops.push("a"); j++; }
      }
    }
    const added = new Set(), modified = new Set(), delBefore = new Map();
    let line = pre + 1, nDel = 0, ai = 0, bi = 0;
    for (let k = 0; k < ops.length; ) {
      if (ops[k] === "e") { line++; ai++; bi++; k++; continue; }
      // 一段连续的删除+新增（中间夹着的空行也算进来）：按相似度把删掉的行和新加的行对齐，
      // 对上的算"修改"，空行对空行算没变，其余算新增/删除
      const dels = [], adds = [];
      while (k < ops.length) {
        if (ops[k] === "e") {
          // 夹在改动中间的相同空行，并进这一段，避免 LCS 用空行把一处改动切成两半
          let k2 = k;
          while (k2 < ops.length && ops[k2] === "e" && A[ai + k2 - k] === "" ) k2++;
          if (k2 === k || k2 >= ops.length || ops[k2] === "e") break;
          for (; k < k2; k++) { dels.push(ai++); adds.push(bi++); }
          continue;
        }
        if (ops[k] === "d") dels.push(ai++);
        else adds.push(bi++);
        k++;
      }
      const pairOf = alignRun(dels.map((i) => A[i]), adds.map((j) => B[j])); // 删除下标 -> 新行下标
      const pairedAdd = new Set(pairOf.values());
      adds.forEach((bj, x) => {
        if (!pairedAdd.has(x)) added.add(line + x);
      });
      pairOf.forEach((x, y) => {
        if (A[dels[y]].trim() !== B[adds[x]].trim()) modified.add(line + x);
      });
      // 没对上的删除行：记在"它后面第一个对上的行"之前，没有就记在这段的末尾
      dels.forEach((_, y) => {
        if (pairOf.has(y)) return;
        let pos = adds.length;
        for (const [yy, xx] of pairOf) if (yy > y) { pos = xx; break; }
        delBefore.set(line + pos, (delBefore.get(line + pos) || 0) + 1);
        nDel++;
      });
      line += adds.length;
    }
    return { added, modified, delBefore, nAdd: added.size, nMod: modified.size, nDel };
  }

  // 在一段改动里把删除行和新增行按相似度对齐（保持先后顺序，总分最高），返回 Map(删除下标 -> 新增下标)
  function alignRun(D, N) {
    const pairs = new Map();
    const n = D.length, m = N.length;
    if (!n || !m || n * m > 250000) return pairs;
    const score = (i, j) => (D[i].trim() === "" && N[j].trim() === "" ? 1 : similar(D[i], N[j]) ? 2 : 0);
    const w = m + 1, S = new Int32Array((n + 1) * w);
    for (let i = n - 1; i >= 0; i--)
      for (let j = m - 1; j >= 0; j--) {
        const sc = score(i, j);
        S[i * w + j] = Math.max(S[(i + 1) * w + j], S[i * w + j + 1], sc ? S[(i + 1) * w + j + 1] + sc : 0);
      }
    let i = 0, j = 0;
    while (i < n && j < m) {
      const sc = score(i, j);
      if (sc && S[i * w + j] === S[(i + 1) * w + j + 1] + sc) { pairs.set(i, j); i++; j++; }
      else if (S[(i + 1) * w + j] >= S[i * w + j + 1]) i++;
      else j++;
    }
    return pairs;
  }

  // 两行"差不多"：公共前缀 + 公共后缀占较长那行的 40% 以上（空行不算）
  function similar(a, b) {
    a = a.trim(); b = b.trim();
    if (!a || !b) return false;
    let p = 0;
    while (p < a.length && p < b.length && a[p] === b[p]) p++;
    let q = 0;
    while (q < a.length - p && q < b.length - p && a[a.length - 1 - q] === b[b.length - 1 - q]) q++;
    return (p + q) / Math.max(a.length, b.length) >= 0.4;
  }

  function clearDiff() {
    main.querySelectorAll(".diff-add, .diff-mod, .diff-del, .diff-del-end").forEach((e) => e.classList.remove("diff-add", "diff-mod", "diff-del", "diff-del-end"));
  }

  function applyDiff(d) {
    clearDiff();
    // 代码视图（包括 Markdown 的源码视图）
    main.querySelectorAll(".file .code").forEach((box) => {
      const lines = box.querySelectorAll(".lntd:last-child .line");
      const nums = box.querySelectorAll(".lntd:first-child .lnt");
      const mark = (n, cls) => [lines[n - 1], nums[n - 1]].forEach((e) => e && e.classList.add(cls));
      d.added.forEach((n) => mark(n, "diff-add"));
      d.modified.forEach((n) => mark(n, "diff-mod"));
      d.delBefore.forEach((_, n) => (n <= lines.length ? mark(n, "diff-del") : mark(lines.length, "diff-del-end")));
    });
    // Markdown 预览：块元素带 data-sl/data-el，标出包含改动行的最内层块
    const md = main.querySelector(".md-preview");
    if (!md) return;
    const blocks = [...md.querySelectorAll("[data-sl]")].map((el) => ({ el, sl: +el.dataset.sl, end: +el.dataset.el }));
    const changedIn = (x) => {
      let add = false, mod = false;
      for (let n = x.sl; n <= x.end; n++) {
        if (d.modified.has(n)) mod = true;
        else if (d.added.has(n)) add = true;
      }
      return mod ? "diff-mod" : add ? "diff-add" : null;
    };
    for (const x of blocks) {
      const cls = changedIn(x);
      if (!cls) continue;
      // 代码块：逐行着色（第一行是 ``` 所以内容从 sl+1 开始）
      const codeLines = x.el.classList.contains("md-code") ? x.el.querySelectorAll(".line") : [];
      if (codeLines.length) {
        codeLines.forEach((ln, i) => {
          const n = x.sl + 1 + i;
          if (d.modified.has(n)) ln.classList.add("diff-mod");
          else if (d.added.has(n)) ln.classList.add("diff-add");
          if (d.delBefore.has(n)) ln.classList.add("diff-del");
        });
        continue;
      }
      // 只标最内层的块（比如列表项里的段落），避免整片列表变色
      const inner = blocks.some((y) => y.el !== x.el && x.el.contains(y.el) && changedIn(y));
      if (!inner) x.el.classList.add(cls);
    }
    // 删除：发生在某个块内部就把这个块算作修改；在块之间就标在后一个块的上沿；删在末尾就标最后一个块的下沿
    d.delBefore.forEach((_, n) => {
      const inside = blocks.filter((x) => x.sl < n && n <= x.end).sort((p, q) => (p.end - p.sl) - (q.end - q.sl))[0];
      if (inside) {
        if (inside.el.classList.contains("md-code") && inside.el.querySelector(".line")) return; // 已逐行标出
        if (!inside.el.classList.contains("diff-add")) inside.el.classList.add("diff-mod");
        return;
      }
      const after = blocks.filter((x) => x.sl >= n).sort((p, q) => p.sl - q.sl)[0];
      if (after) after.el.classList.add("diff-del");
      else if (blocks.length) blocks[blocks.length - 1].el.classList.add("diff-del-end");
    });
  }

  // Markdown 内的相对链接 / 图片改写到本站路由
  function fixMarkdown(el, baseDir) {
    if (!el) return;
    const isExternal = (u) => /^[a-z][a-z0-9+.-]*:/i.test(u) || u.startsWith("//");
    el.querySelectorAll("img[src]").forEach((img) => {
      const src = img.getAttribute("src");
      if (isExternal(src) || src.startsWith("data:")) return;
      const target = src.startsWith("/") ? src.slice(1) : joinPath(baseDir, src);
      img.src = "/raw/" + enc(decodeURIComponent(target.split(/[?#]/)[0]));
    });
    el.querySelectorAll("a[href]").forEach((a) => {
      const href = a.getAttribute("href");
      if (href.startsWith("#")) return;
      if (isExternal(href)) {
        a.target = "_blank";
        a.rel = "noopener";
        return;
      }
      const [pathPart, hash] = href.split("#");
      let target = pathPart.startsWith("/") ? pathPart.slice(1) : joinPath(baseDir, pathPart);
      try { target = decodeURIComponent(target); } catch (e) {}
      a.setAttribute("href", "/blob/" + enc(target) + (hash ? "#" + hash : ""));
      a.dataset.nav = "";
    });
    // 任务列表的勾选框只读
    el.querySelectorAll('input[type="checkbox"]').forEach((c) => (c.disabled = true));
  }

  main.addEventListener("click", async (ev) => {
    const seg = ev.target.closest("[data-md]");
    if (seg) {
      mdMode = seg.dataset.md;
      store.set("cv-md-mode", mdMode);
      main.querySelectorAll("[data-md]").forEach((b) => b.classList.toggle("on", b.dataset.md === mdMode));
      main.querySelector(".md-preview").hidden = mdMode !== "preview";
      main.querySelector(".md-source").hidden = mdMode !== "source";
      renderOutline(mdMode === "preview" ? main._file.outline : null, "markdown");
      return;
    }
    const cp = ev.target.closest(".copy-path");
    if (cp) return copyText(cp.dataset.copy, cp);
    const cc = ev.target.closest("#copy-content");
    if (cc) {
      const text = await fetch("/raw/" + enc(main._file.path)).then((r) => r.text());
      return copyText(text, cc);
    }
    if (ev.target.closest("#toggle-outline")) return toggleOutline();
    if (ev.target.closest("#diff-toggle")) return toggleDiff();
    if (ev.target.closest("#diff-reset")) {
      baselines.set(main._file.path, main._file.text);
      return showFile(main._file.path, true);
    }
    // 点击行号：高亮该行并更新地址栏
    const ln = ev.target.closest(".lnt, .ln");
    if (ln) {
      ev.preventDefault();
      const id = ln.closest("[id]") ? ln.closest("[id]").id : ln.id;
      const lineNo = (ln.textContent || "").trim();
      const hash = id && id.startsWith("L") ? "#" + id : "#L" + lineNo;
      history.replaceState(history.state, "", location.pathname + hash);
      scrollToHash(false);
    }
  });

  function toggleDiff() {
    if (!main._diff) return;
    diffOn = !diffOn;
    store.set("cv-diff", diffOn ? "1" : "0");
    const b = main.querySelector("#diff-toggle");
    if (b) b.classList.toggle("on", diffOn);
    diffOn ? applyDiff(main._diff) : clearDiff();
  }

  function toggleOutline() {
    if (!outlineList.children.length) return;
    outlineEl.hidden = !outlineEl.hidden;
    store.set("cv-outline", outlineEl.hidden ? "0" : "1");
  }

  async function copyText(text, btn) {
    try {
      await navigator.clipboard.writeText(text);
    } catch (e) {
      const ta = document.createElement("textarea");
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      ta.remove();
    }
    btn.classList.add("copied");
    setTimeout(() => btn.classList.remove("copied"), 1200);
  }

  function scrollToHash(scroll = true) {
    main.querySelectorAll(".hl-line").forEach((e) => e.classList.remove("hl-line"));
    const h = decodeURIComponent(location.hash.slice(1));
    if (!h) return;
    const m = /^L(\d+)$/.exec(h);
    let target;
    if (m) {
      // 如果在看 Markdown 预览，跳到行号时切到源码
      const src = main.querySelector(".md-source");
      if (src && src.hidden) main.querySelector('[data-md="source"]').click();
      const box = main.querySelector(".code:not([hidden])");
      if (!box) return;
      const n = m[1];
      // chroma 表格模式：行号列里 span#Ln，代码列每行是 span.line
      const lnt = box.querySelector(`#L${n}`) || box.querySelectorAll(".lnt")[n - 1];
      const line = box.querySelectorAll(".lntd:last-child .line")[n - 1];
      [lnt, line].forEach((e) => e && e.classList.add("hl-line"));
      target = lnt || line; // 用行号定位，避免长行导致代码区横向滚动
    } else {
      target = document.getElementById(h);
    }
    if (target && scroll) target.scrollIntoView({ block: m ? "center" : "start", inline: "nearest" });
  }

  // ---------- 自动更新 ----------
  // 通过 /api/watch（SSE）盯住当前文件、当前目录和左侧展开的目录，变了就只重新加载那一块。

  let es = null, watchKey = "", watchTimer;

  function updateWatch() {
    clearTimeout(watchTimer);
    watchTimer = setTimeout(() => {
      const set = new Set(["", currentPath, ...expanded]);
      if (!currentIsDir) set.add(dirname(currentPath));
      const paths = [...set].sort();
      const key = paths.join("\n");
      if (document.hidden) return; // 后台标签页不占连接，切回来再连
      if (es && key === watchKey && es.readyState !== EventSource.CLOSED) return;
      watchKey = key;
      if (es) es.close();
      const src = new EventSource("/api/watch?" + paths.map((p) => "p=" + encodeURIComponent(p)).join("&"));
      src.onmessage = (ev) => {
        try { onFsChange(JSON.parse(ev.data)); } catch (e) { console.error(e); }
      };
      // 断线重连（服务重启、网络抖动）或从后台切回来时，断开期间的改动收不到通知，连上后补刷一次
      src.onopen = () => {
        if (src._opened || resyncOnOpen) refreshAll();
        src._opened = true;
        resyncOnOpen = false;
      };
      es = src;
    }, 300);
  }

  // 浏览器对同一站点只保持约 6 个长连接：标签页切到后台就断开，切回来重连并补刷
  let resyncOnOpen = false;
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) {
      if (es) { es.close(); es = null; }
      clearTimeout(watchTimer);
    } else {
      resyncOnOpen = true;
      updateWatch();
    }
  });

  async function refreshAll() {
    for (const p of [...dirCache.keys()]) await onFsChange({ path: p, state: "changed" }, true);
    if (!currentIsDir) await onFsChange({ path: currentPath, state: "changed" }, true);
  }

  async function onFsChange({ path, state }, quiet) {
    // 1. 正在看的文件
    if (!currentIsDir && path === currentPath) {
      if (state === "deleted") {
        const head = main.querySelector(".file .box-head");
        if (head && !main.querySelector(".gone")) head.insertAdjacentHTML("afterend", `<div class="gone">${t("gone")}</div>`);
      } else {
        const before = main._file && main._file.text;
        await showFile(path, true).catch(() => {});
        main.querySelector(".gone")?.remove();
        if (!quiet || (main._file && main._file.text !== before)) toast(t("updated"));
      }
    }
    // 2. 目录：刷新左侧树里对应的节点
    if (dirCache.has(path)) {
      dirCache.delete(path);
      if (path === "") {
        await renderChildren(treeEl, "", 0);
      } else {
        const node = treeEl.querySelector(`.node[data-dir="${CSS.escape(path)}"]`);
        const children = node && node.querySelector(":scope > .children");
        if (node && node.classList.contains("open") && children) await renderChildren(children, path, depthOf(path));
        else if (children) delete children.dataset.loaded;
      }
    }
    // 3. 正在看的目录页
    if (currentIsDir && path === currentPath) {
      const top = main.scrollTop;
      if (state === "deleted") main.innerHTML = crumbs(path, true) + `<div class="notice error">${t("dirGone")}</div>`;
      else await showDir(path).catch(() => {});
      main.scrollTop = top;
      if (!quiet) toast(t("updated"));
    }
  }

  let toastTimer;
  function toast(msg) {
    let el = $("#toast");
    if (!el) {
      el = document.createElement("div");
      el.id = "toast";
      document.body.appendChild(el);
    }
    el.textContent = msg + " · " + new Date().toLocaleTimeString(locale());
    el.classList.add("show");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => el.classList.remove("show"), 1800);
  }

  // ---------- 路由 ----------

  function parseLocation() {
    const p = decodeURIComponent(location.pathname);
    if (p.startsWith("/blob/")) return { kind: "blob", path: p.slice(6).replace(/\/$/, "") };
    if (p.startsWith("/tree/")) return { kind: "tree", path: p.slice(6).replace(/\/$/, "") };
    return { kind: "tree", path: "" };
  }

  let navSeq = 0;
  async function route() {
    const seq = ++navSeq;
    const { kind, path } = parseLocation();
    currentPath = path;
    document.title = path ? basename(path) + " · " + SITE : SITE;
    try {
      if (kind === "blob") {
        try {
          await showFile(path);
        } catch (err) {
          if (!err.isDir) throw err;
          history.replaceState(null, "", "/tree/" + enc(path));
          await showDir(path);
        }
      } else {
        await showDir(path);
      }
    } catch (err) {
      if (seq !== navSeq) return;
      main.innerHTML = crumbs(path, true) + `<div class="notice error">${esc(err.message)}</div>`;
      renderOutline(null);
    }
    if (seq !== navSeq) return;
    if (location.hash) scrollToHash();
    else main.scrollTop = 0, window.scrollTo(0, 0);
    await revealInTree(path);
    updateWatch();
  }

  function navigate(url) {
    if (url === location.pathname + location.hash) return;
    history.pushState(null, "", url);
    route();
  }

  document.addEventListener("click", (ev) => {
    const a = ev.target.closest("a[data-nav]");
    if (!a || ev.defaultPrevented || ev.button !== 0 || ev.metaKey || ev.ctrlKey || ev.shiftKey || ev.altKey) return;
    ev.preventDefault();
    hideResults();
    navigate(a.getAttribute("href"));
  });
  window.addEventListener("popstate", route);
  window.addEventListener("hashchange", () => scrollToHash());

  // ---------- 搜索 ----------

  const searchInput = $("#search");
  const resultsEl = $("#search-results");
  let searchTimer, searchSeq = 0, selIdx = -1;

  function hideResults() { resultsEl.hidden = true; selIdx = -1; }

  function highlightMatch(text, q) {
    const i = text.toLowerCase().indexOf(q.toLowerCase());
    if (i < 0 || !q) return esc(text);
    return esc(text.slice(0, i)) + "<mark>" + esc(text.slice(i, i + q.length)) + "</mark>" + esc(text.slice(i + q.length));
  }

  async function doSearch() {
    const raw = searchInput.value.trim();
    if (!raw) return hideResults();
    const seq = ++searchSeq;
    const grep = raw.startsWith("/");
    const q = grep ? raw.slice(1).trim() : raw;
    if (!q) return hideResults();
    resultsEl.hidden = false;
    resultsEl.innerHTML = `<div class="sr-empty">${t("searching")}</div>`;
    try {
      if (grep) {
        const hits = await api("/api/grep?q=" + encodeURIComponent(q));
        if (seq !== searchSeq) return;
        resultsEl.innerHTML = hits.length ? hits.map((h) => `<div class="sr-group">
            <a class="sr-item sr-file" href="/blob/${esc(enc(h.path))}" data-nav>${fileIcon(basename(h.path))}<span>${esc(h.path)}</span></a>
            ${h.lines.map((l) => `<a class="sr-item sr-line" href="/blob/${esc(enc(h.path))}#L${l.line}" data-nav><span class="ln-no">${l.line}</span><span>${highlightMatch(l.text, q)}</span></a>`).join("")}
          </div>`).join("") : `<div class="sr-empty">${t("noGrep", esc(q))}</div>`;
      } else {
        const hits = await api("/api/search?q=" + encodeURIComponent(q));
        if (seq !== searchSeq) return;
        resultsEl.innerHTML = hits.length ? hits.map((h) => `<a class="sr-item" href="/${h.type === "dir" ? "tree" : "blob"}/${esc(enc(h.path))}" data-nav>
            ${h.type === "dir" ? ICON.dir : fileIcon(basename(h.path))}<span>${highlightMatch(h.path, q)}</span></a>`).join("")
          : `<div class="sr-empty">${t("noMatch", esc(q))}</div>`;
      }
      selIdx = -1;
    } catch (err) {
      resultsEl.innerHTML = `<div class="sr-empty">${esc(err.message)}</div>`;
    }
  }

  searchInput.addEventListener("input", () => {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(doSearch, searchInput.value.trim().startsWith("/") ? 300 : 120);
  });
  searchInput.addEventListener("focus", () => { if (searchInput.value.trim()) doSearch(); });
  searchInput.addEventListener("keydown", (ev) => {
    const items = [...resultsEl.querySelectorAll(".sr-item")];
    if (ev.key === "ArrowDown" || ev.key === "ArrowUp") {
      ev.preventDefault();
      if (!items.length) return;
      selIdx = (selIdx + (ev.key === "ArrowDown" ? 1 : -1) + items.length) % items.length;
      items.forEach((it, i) => it.classList.toggle("sel", i === selIdx));
      items[selIdx].scrollIntoView({ block: "nearest" });
    } else if (ev.key === "Enter") {
      const it = items[selIdx >= 0 ? selIdx : 0];
      if (it) { hideResults(); searchInput.blur(); navigate(it.getAttribute("href")); }
    } else if (ev.key === "Escape") {
      hideResults();
      searchInput.blur();
    }
  });
  document.addEventListener("mousedown", (ev) => {
    if (!ev.target.closest(".search")) hideResults();
  });

  // ---------- 快捷键、主题、侧栏 ----------

  document.addEventListener("keydown", (ev) => {
    if (ev.target.closest("input, textarea") || ev.metaKey || ev.ctrlKey || ev.altKey) return;
    if (ev.key === "t" || ev.key === "/") {
      ev.preventDefault();
      searchInput.focus();
      searchInput.select();
      if (ev.key === "/") { searchInput.value = "/"; }
    } else if (ev.key === "b") {
      toggleSidebar();
    } else if (ev.key === "o") {
      toggleOutline();
    } else if (ev.key === "d") {
      toggleDiff();
    } else if (ev.key === "Backspace" || ev.key === "u") {
      if (currentPath) navigate("/tree/" + enc(dirname(currentPath)));
    }
  });

  $("#toggle-lang").addEventListener("click", async () => {
    lang = lang === "zh" ? "en" : "zh";
    store.set("cv-lang", lang);
    applyStaticText();
    await renderChildren(treeEl, "", 0);
    route();
  });

  // 配色主题：主题列表由服务端注入（window.CV_THEMES），选择记在 localStorage
  const themeMenu = $("#theme-menu");
  function renderThemeMenu() {
    const cur = document.documentElement.getAttribute("data-theme-choice") || "auto";
    const sw = (p) => `<span class="sw"><i style="background:${p.bg}"></i><i style="background:${p["bg-subtle"]}"></i><i style="background:${p.accent}"></i><i style="background:${p.fg}"></i></span>`;
    const item = (id, name, swatch) => `<button data-theme-id="${id}" class="${cur === id ? "on" : ""}">${swatch}<span>${esc(name)}</span></button>`;
    const group = (scheme) => (window.CV_THEMES || []).filter((x) => x.scheme === scheme).map((x) => item(x.id, x.name, sw(x.palette))).join("");
    themeMenu.innerHTML = `<div class="tm-head">${t("theme")}</div>`
      + item("auto", t("themeAuto"), `<span class="sw"><i style="background:#fff"></i><i style="background:#0d1117"></i></span>`)
      + `<hr><div class="tm-head">${t("themeDark")}</div>` + group("dark")
      + `<hr><div class="tm-head">${t("themeLight")}</div>` + group("light");
  }
  $("#toggle-theme").addEventListener("click", (ev) => {
    ev.stopPropagation();
    if (themeMenu.hidden) renderThemeMenu();
    themeMenu.hidden = !themeMenu.hidden;
  });
  themeMenu.addEventListener("click", (ev) => {
    const b = ev.target.closest("[data-theme-id]");
    if (!b) return;
    window.cvApplyTheme(b.dataset.themeId);
    store.set("cv-theme", b.dataset.themeId);
    renderThemeMenu();
  });
  // 鼠标悬停时预览，移开恢复
  themeMenu.addEventListener("mouseover", (ev) => {
    const b = ev.target.closest("[data-theme-id]");
    if (b) window.cvApplyTheme(b.dataset.themeId);
  });
  themeMenu.addEventListener("mouseleave", () => window.cvApplyTheme(store.get("cv-theme") || "auto"));
  document.addEventListener("mousedown", (ev) => {
    if (!themeMenu.hidden && !ev.target.closest(".theme-picker")) themeMenu.hidden = true;
  });
  document.addEventListener("keydown", (ev) => {
    if (ev.key === "Escape" && !themeMenu.hidden) themeMenu.hidden = true;
  });

  const sidebar = $("#sidebar");
  function toggleSidebar() {
    sidebar.classList.toggle("collapsed");
    store.set("cv-side", sidebar.classList.contains("collapsed") ? "0" : "1");
  }
  $("#toggle-side").addEventListener("click", toggleSidebar);
  if (store.get("cv-side") === "0" || (store.get("cv-side") === null && window.innerWidth < 768)) sidebar.classList.add("collapsed");

  // 拖动调整侧栏宽度
  const savedW = parseInt(store.get("cv-side-w"), 10);
  if (savedW) document.documentElement.style.setProperty("--side-w", savedW + "px");
  $("#resizer").addEventListener("mousedown", (ev) => {
    ev.preventDefault();
    const move = (e) => {
      const w = Math.min(Math.max(e.clientX, 160), 600);
      document.documentElement.style.setProperty("--side-w", w + "px");
      store.set("cv-side-w", w);
    };
    const up = () => { document.removeEventListener("mousemove", move); document.removeEventListener("mouseup", up); document.body.classList.remove("resizing"); };
    document.body.classList.add("resizing");
    document.addEventListener("mousemove", move);
    document.addEventListener("mouseup", up);
  });

  // ---------- 启动 ----------

  (async function init() {
    await renderChildren(treeEl, "", 0);
    route();
  })();
})();

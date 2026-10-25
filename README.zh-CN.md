# codeview

[English](README.md) | 简体中文

一个类 GitHub 的本地代码浏览器。Go 编写，单个二进制文件，前端资源通过 `go:embed` 打包，运行时不依赖网络。

把它指向任意目录，就能在浏览器里像在 GitHub 上一样浏览目录树、查看高亮代码和渲染后的 Markdown。

## 功能

- **文件树**：左侧可展开/折叠的目录树，记住展开状态，打开文件时自动定位；可拖动调整宽度。
- **目录页**：文件列表（目录在前，自然排序）+ 自动渲染 README。
- **代码高亮**：基于 [chroma](https://github.com/alecthomas/chroma)，支持 200+ 种语言；无扩展名的脚本按 shebang 识别。行号可点击，生成 `#L20` 这样的链接，打开时自动滚动并高亮。
- **Markdown**：基于 [goldmark](https://github.com/yuin/goldmark)，支持 GFM（表格、任务列表、删除线、自动链接）、脚注、代码块高亮、YAML front matter；标题锚点与 GitHub 规则一致（支持中文）；相对链接和图片自动改写。可切换「预览 / 源码」。
- **大纲**：右侧显示 Markdown 标题目录；Go / Python / Shell / JS / TS 显示函数和类型；YAML 显示前两层 key。
- **搜索**：按 `t` 按文件名模糊搜索；输入 `/关键字` 做全文搜索，点击结果直接跳到对应行。
- **自动更新**：正在查看的文件或目录有变化时，页面只刷新变化的部分，滚动位置、预览/源码模式、文件树展开状态保持不变。标签页切到后台时断开连接，切回或断线重连后自动补刷。
- **改动着色**：以本次打开页面时第一次看到的内容为基准，之后的改动按行着色（绿色新增、蓝色修改、红线标出删除位置），并显示 `+新增 ~修改 −删除` 行数。Markdown 预览会标到对应的段落、列表项、表格行和代码行。可开关、可重置基准，刷新页面也会重置。
- 图片、PDF 直接预览；二进制和超大文件提供下载；Raw / 下载 / 复制内容 / 复制路径。
- 13 套配色主题（GitHub 浅色/深色、Dracula、Monokai、One Dark、Nord、Tokyo Night、Catppuccin、Gruvbox、Rosé Pine、Solarized 等），界面和代码高亮一起切换，也可以跟随系统深浅色；选择保存在浏览器里。适配窄屏；界面支持英文 / 中文。
- 自动识别 GBK / GB18030 编码的文本文件。
- 安全：只能访问根目录内的文件，`..` 和指向外部的符号链接会被拒绝；默认隐藏 `.` 开头的文件和 `.git`、`node_modules` 等目录；Raw 接口以纯文本 + CSP sandbox 返回，避免执行页面脚本。

## 安装

需要 Go 1.22 及以上。依赖已放在 `vendor/` 中，可离线编译。

```bash
git clone https://github.com/quxf2012/codeview.git
cd codeview
make build            # 生成 ./codeview
```

或者直接安装：

```bash
go install github.com/quxf2012/codeview@latest
```

交叉编译 Linux / Windows / macOS：

```bash
make dist             # 输出到 dist/
```

## 使用

```bash
./codeview -root /path/to/dir
# 也可以写成：./codeview /path/to/dir
```

然后在浏览器打开 http://localhost:8080 。

### 参数

| 参数 | 默认值 | 说明 |
|---|---|---|
| `-root` | `.` | 要浏览的根目录；也可作为最后一个参数传入；环境变量 `CODEVIEW_ROOT` |
| `-addr` | `127.0.0.1:8080` | 监听地址，局域网访问用 `0.0.0.0:8080`；环境变量 `CODEVIEW_ADDR` |
| `-title` | 根目录名 | 页面标题 |
| `-hidden` | `false` | 显示 `.` 开头的隐藏文件 |
| `-ignore` | `.git,node_modules,__pycache__,.venv,.idea,.vscode` | 忽略的目录 / 文件名，逗号分隔 |
| `-max-size` | `2097152` | 超过该字节数的文件不在线显示 |
| `-lang` | `en` | 界面默认语言：`en` 或 `zh`，页面右上角可随时切换；环境变量 `CODEVIEW_LANG` |
| `-version` | | 打印版本 |

### 快捷键

| 键 | 作用 |
|---|---|
| `t` | 查找文件 |
| `/` | 全文搜索 |
| `↑` `↓` `Enter` `Esc` | 在搜索结果中选择 |
| `b` | 显示 / 隐藏文件树 |
| `o` | 显示 / 隐藏大纲 |
| `d` | 开关改动着色 |
| `u` / `Backspace` | 返回上一级目录 |

### 作为服务运行

`deploy/codeview.service` 是一个 systemd 用户服务示例，按需修改其中的 `-root` 后：

```bash
mkdir -p ~/.config/systemd/user
cp deploy/codeview.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now codeview
```

## 项目结构

```
main.go        参数解析、启动 HTTP 服务
server.go      路由、路径安全检查、目录列表、文件读取、Raw 下载
render.go      代码高亮、Markdown 渲染、大纲提取、自然排序
themes.go      配色主题（界面配色 + chroma 高亮样式）
search.go      文件名模糊搜索、全文搜索
watch.go       自动更新：通过 SSE 推送文件 / 目录变化
web/           前端（index.html、app.js、style.css），原生 JS，无需构建
deploy/        systemd 服务示例
```

### HTTP 接口

| 路径 | 说明 |
|---|---|
| `GET /api/tree?path=` | 目录列表 + README |
| `GET /api/file?path=` | 文件信息、渲染后的 HTML、原文和大纲 |
| `GET /api/search?q=` | 文件名搜索 |
| `GET /api/grep?q=` | 全文搜索 |
| `GET /api/watch?p=..&p=..` | SSE，推送这些路径的变化 |
| `GET /raw/<path>` | 原始文件（`?download=1` 下载） |
| `/tree/<path>`、`/blob/<path>` | 页面路由，与 GitHub 的地址格式一致 |

### 扩展

- 某种文件没有被正确高亮：在 `render.go` 的 `lexerAlias` 中添加文件名到语言的映射。
- 增加大纲支持的语言：在 `render.go` 的 `outlineRules` 中添加正则规则。

## 致谢

- [chroma](https://github.com/alecthomas/chroma)：语法高亮
- [goldmark](https://github.com/yuin/goldmark)、[goldmark-highlighting](https://github.com/yuin/goldmark-highlighting)：Markdown 渲染
- 界面与地址格式参考 GitHub

## 许可证

[MIT](LICENSE)

# codeview

English | [简体中文](README.zh-CN.md)

A GitHub-style code browser for local directories. Written in Go and shipped as a single binary: the frontend is embedded with `go:embed`, and nothing is fetched from the network at runtime.

Point it at any directory and browse the tree, syntax-highlighted code and rendered Markdown in your browser, just like on GitHub.

## Features

- **File tree**: collapsible directory tree on the left that remembers what you expanded and follows the file you open; resizable.
- **Directory view**: file listing (directories first, natural sort) with the README rendered below.
- **Syntax highlighting**: powered by [chroma](https://github.com/alecthomas/chroma), 200+ languages; extensionless scripts are detected by shebang. Line numbers are clickable and produce `#L20` links that scroll to and highlight the line.
- **Markdown**: powered by [goldmark](https://github.com/yuin/goldmark) with GFM (tables, task lists, strikethrough, autolinks), footnotes, highlighted code blocks and YAML front matter. Heading anchors follow GitHub's rules (including CJK text); relative links and images are rewritten to work. Toggle between preview and source.
- **Outline**: a side panel with Markdown headings; functions and types for Go / Python / Shell / JS / TS; the top two levels of keys for YAML.
- **Search**: press `t` for fuzzy file-name search. Full-text search (type `/keyword`, then jump straight to the matching line) is off by default and enabled with `-grep`, since it reads every file on each query.
- **Live reload**: when the file or directory you are viewing changes, only that part of the page is refreshed. Scroll position, preview/source mode and the tree state are kept. Background tabs drop their connection and resync when they become visible again or after a reconnect.
- **Change highlighting**: the content you saw when the page first opened a file is the baseline; later edits are highlighted per line (green added, blue modified, a red rule where lines were removed) with `+added ~modified −removed` counts. In Markdown preview the marks land on the matching paragraph, list item, table row or code line. Can be toggled and reset; reloading the page resets it too.
- Images and PDFs are previewed inline; binaries and very large files get a download link. Raw / download / copy contents / copy path.
- 13 color themes (GitHub Light/Dark, Dracula, Monokai, One Dark, Nord, Tokyo Night, Catppuccin, Gruvbox, Rosé Pine, Solarized …) covering both the UI and syntax highlighting, or follow the system light/dark setting; the choice is saved in the browser. Works on narrow screens; English and Chinese UI.
- Detects GBK / GB18030 encoded text files.
- Safe by default: only files inside the root are served, `..` and symlinks pointing outside are rejected, dotfiles and directories such as `.git` and `node_modules` are hidden, and raw files are served as plain text with a CSP sandbox so they never run as pages.

## Install

Requires Go 1.22 or later. Dependencies are vendored, so it builds offline.

```bash
git clone https://github.com/quxf2012/codeview.git
cd codeview
make build            # produces ./codeview
```

Or install directly:

```bash
go install github.com/quxf2012/codeview@latest
```

Cross-compile for Linux / Windows / macOS:

```bash
make dist             # outputs to dist/
```

## Usage

```bash
./codeview -root /path/to/dir
# or: ./codeview /path/to/dir
```

Then open http://localhost:8080 in your browser.

### Flags

| Flag | Default | Description |
|---|---|---|
| `-root` | `.` | Directory to browse; can also be given as the last argument; env `CODEVIEW_ROOT` |
| `-addr` | `127.0.0.1:8080` | Listen address, use `0.0.0.0:8080` for LAN access; env `CODEVIEW_ADDR` |
| `-title` | name of the root | Page title |
| `-hidden` | `false` | Show dotfiles |
| `-ignore` | `.git,node_modules,__pycache__,.venv,.idea,.vscode` | Comma-separated file / directory names to ignore |
| `-max-size` | `2097152` | Files larger than this many bytes are not rendered |
| `-grep` | `false` | Enable full-text search; it scans files on every query, which can be slow on large trees; env `CODEVIEW_GREP=1` |
| `-lang` | `en` | Default UI language, `en` or `zh`; can be switched in the page at any time; env `CODEVIEW_LANG` |
| `-version` | | Print version and exit |

### Keyboard shortcuts

| Key | Action |
|---|---|
| `t` | Go to file |
| `/` | Full-text search (with `-grep`) |
| `↑` `↓` `Enter` `Esc` | Navigate search results |
| `b` | Show / hide file tree |
| `o` | Show / hide outline |
| `d` | Toggle change highlighting |
| `u` / `Backspace` | Go to parent directory |

### Running as a service

`deploy/codeview.service` is an example systemd user unit. Adjust `-root` in it, then:

```bash
mkdir -p ~/.config/systemd/user
cp deploy/codeview.service ~/.config/systemd/user/
systemctl --user daemon-reload
systemctl --user enable --now codeview
```

## Project layout

```
main.go        flags, HTTP server startup
server.go      routes, path safety checks, directory listing, file loading, raw downloads
render.go      syntax highlighting, Markdown rendering, outline extraction, natural sort
themes.go      color themes (UI palette + chroma style)
search.go      fuzzy file-name search, full-text search
watch.go       live reload: pushes file / directory changes over SSE
web/           frontend (index.html, app.js, style.css), plain JS, no build step
deploy/        example systemd unit
```

### HTTP API

| Path | Description |
|---|---|
| `GET /api/tree?path=` | Directory listing + README |
| `GET /api/file?path=` | File info, rendered HTML, raw text and outline |
| `GET /api/search?q=` | File-name search |
| `GET /api/grep?q=` | Full-text search (only with `-grep`) |
| `GET /api/watch?p=..&p=..` | SSE stream of changes to these paths |
| `GET /raw/<path>` | Raw file (`?download=1` to download) |
| `/tree/<path>`, `/blob/<path>` | Page routes, same URL scheme as GitHub |

### Extending

- A file type is not highlighted correctly: add a file name → language mapping to `lexerAlias` in `render.go`.
- Outline for another language: add regex rules to `outlineRules` in `render.go`.

## Acknowledgements

- [chroma](https://github.com/alecthomas/chroma): syntax highlighting
- [goldmark](https://github.com/yuin/goldmark), [goldmark-highlighting](https://github.com/yuin/goldmark-highlighting): Markdown rendering
- UI and URL scheme inspired by GitHub

## License

[MIT](LICENSE)

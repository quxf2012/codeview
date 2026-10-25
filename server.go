package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"io/fs"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

type server struct {
	cfg   *config
	mux   *http.ServeMux
	index *template.Template
	css   []byte
}

func newServer(cfg *config) *server {
	s := &server{cfg: cfg, mux: http.NewServeMux()}
	s.index = template.Must(template.ParseFS(webFS, "web/index.html"))
	s.css = themeCSS()

	s.mux.HandleFunc("/api/tree", s.handleTree)
	s.mux.HandleFunc("/api/file", s.handleFile)
	s.mux.HandleFunc("/api/search", s.handleSearch)
	if cfg.grep {
		s.mux.HandleFunc("/api/grep", s.handleGrep)
	}
	s.mux.HandleFunc("/api/watch", s.handleWatch)
	s.mux.HandleFunc("/api/theme.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write(s.css)
	})
	s.mux.HandleFunc("/raw/", s.handleRaw)
	s.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(mustSub(webFS, "web")))))
	s.mux.HandleFunc("/", s.handleIndex)
	return s
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// ---------- 路径安全 ----------

var (
	errForbidden = errors.New("forbidden")
	errIsDir     = errors.New("is a directory")
)

// resolve 把 URL 中的相对路径映射到磁盘路径，保证不会逃出根目录
// （包括通过 .. 和指向外部的符号链接）。返回规范化后的相对路径和绝对路径。
func (s *server) resolve(rel string) (string, string, error) {
	rel = strings.Trim(path.Clean("/"+rel), "/")
	if rel == "." {
		rel = ""
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg != "" && s.hiddenName(seg) {
			return "", "", errForbidden
		}
	}
	abs := filepath.Join(s.cfg.root, filepath.FromSlash(rel))
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", "", err
	}
	if real != s.cfg.root && !strings.HasPrefix(real, s.cfg.root+string(filepath.Separator)) {
		return "", "", errForbidden
	}
	return rel, real, nil
}

func (s *server) hiddenName(name string) bool {
	if s.cfg.ignore[name] {
		return true
	}
	return !s.cfg.showHidden && strings.HasPrefix(name, ".")
}

func writeErr(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	msg := err.Error()
	switch {
	case errors.Is(err, errForbidden):
		code, msg = http.StatusForbidden, "forbidden"
	case errors.Is(err, fs.ErrNotExist):
		code, msg = http.StatusNotFound, "not found"
	}
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

// ---------- 目录 ----------

type entry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Type    string    `json:"type"` // dir | file
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mtime"`
}

type treeResp struct {
	Path    string   `json:"path"`
	Entries []entry  `json:"entries"`
	Readme  *fileRes `json:"readme,omitempty"`
}

func (s *server) listDir(rel, abs string) ([]entry, error) {
	des, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	out := make([]entry, 0, len(des))
	for _, de := range des {
		name := de.Name()
		if s.hiddenName(name) {
			continue
		}
		info, err := os.Stat(filepath.Join(abs, name)) // Stat 跟随符号链接
		if err != nil {
			continue
		}
		e := entry{Name: name, Path: path.Join(rel, name), Type: "file", Size: info.Size(), ModTime: info.ModTime()}
		if info.IsDir() {
			e.Type, e.Size = "dir", 0
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type == "dir"
		}
		return naturalLess(strings.ToLower(out[i].Name), strings.ToLower(out[j].Name))
	})
	return out, nil
}

func (s *server) handleTree(w http.ResponseWriter, r *http.Request) {
	rel, abs, err := s.resolve(r.URL.Query().Get("path"))
	if err != nil {
		writeErr(w, err)
		return
	}
	entries, err := s.listDir(rel, abs)
	if err != nil {
		writeErr(w, err)
		return
	}
	resp := treeResp{Path: rel, Entries: entries}
	// 目录页像 GitHub 一样附带 README
	if r.URL.Query().Get("readme") != "0" {
		for _, e := range entries {
			if e.Type == "file" && isReadme(e.Name) {
				if fr, err := s.loadFile(e.Path); err == nil {
					resp.Readme = fr
				}
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func isReadme(name string) bool {
	n := strings.ToLower(name)
	return n == "readme.md" || n == "readme.markdown" || n == "readme" || n == "readme.txt" || n == "index.md"
}

// ---------- 文件 ----------

type fileRes struct {
	Path     string        `json:"path"`
	Name     string        `json:"name"`
	Size     int64         `json:"size"`
	ModTime  time.Time     `json:"mtime"`
	Kind     string        `json:"kind"` // markdown | code | image | pdf | binary | large
	Lang     string        `json:"lang,omitempty"`
	Lines    int           `json:"lines,omitempty"`
	Encoding string        `json:"encoding,omitempty"`
	HTML     string        `json:"html,omitempty"`   // 渲染后的 Markdown 或高亮后的代码
	Source   string        `json:"source,omitempty"` // Markdown 的高亮源码
	Text     string        `json:"text,omitempty"`   // 原文（UTF-8），前端用来和首次打开时的内容做对比
	Outline  []outlineItem `json:"outline,omitempty"`
}

func (s *server) handleFile(w http.ResponseWriter, r *http.Request) {
	fr, err := s.loadFile(r.URL.Query().Get("path"))
	if errors.Is(err, errIsDir) {
		// 前端据此改为按目录展示（Markdown 里的相对链接可能指向目录）
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "isDir": true})
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fr)
}

var imageExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true, ".bmp": true, ".ico": true, ".avif": true}

func (s *server) loadFile(p string) (*fileRes, error) {
	rel, abs, err := s.resolve(p)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errIsDir
	}
	fr := &fileRes{Path: rel, Name: path.Base(rel), Size: info.Size(), ModTime: info.ModTime()}
	ext := strings.ToLower(path.Ext(rel))
	switch {
	case imageExt[ext]:
		fr.Kind = "image"
		return fr, nil
	case ext == ".pdf":
		fr.Kind = "pdf"
		return fr, nil
	case info.Size() > s.cfg.maxView:
		fr.Kind = "large"
		return fr, nil
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	text, enc, ok := decodeText(data)
	if !ok {
		fr.Kind = "binary"
		return fr, nil
	}
	fr.Encoding = enc
	fr.Lines = countLines(text)

	if isMarkdown(rel) {
		fr.Kind = "markdown"
		fr.Lang = "Markdown"
		html, outline, err := renderMarkdown([]byte(text))
		if err != nil {
			return nil, err
		}
		fr.HTML, fr.Outline = html, outline
		fr.Source, _, _ = highlight(fr.Name, text)
		fr.Text = text
		return fr, nil
	}

	fr.Kind = "code"
	fr.Text = text
	fr.HTML, fr.Lang, err = highlight(fr.Name, text)
	if err != nil {
		return nil, err
	}
	fr.Outline = codeOutline(fr.Name, fr.Lang, text)
	return fr, nil
}

func isMarkdown(name string) bool {
	switch strings.ToLower(path.Ext(name)) {
	case ".md", ".markdown", ".mdown", ".mkd":
		return true
	}
	return false
}

// isBinary 采用和 git 类似的判断：前 8000 字节里有 NUL 就算二进制。
func isBinary(b []byte) bool {
	if len(b) > 8000 {
		b = b[:8000]
	}
	return bytes.IndexByte(b, 0) >= 0
}

// decodeText 把文件内容转成 UTF-8 文本。Windows 下写的笔记常是 GBK，
// 不是合法 UTF-8 时尝试按 GB18030 解码；都不行则视为二进制。
func decodeText(b []byte) (string, string, bool) {
	if isBinary(b) {
		return "", "", false
	}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")) // UTF-8 BOM
	if utf8.Valid(b) {
		return string(b), "UTF-8", true
	}
	out, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b)
	if err != nil || bytes.ContainsRune(out, utf8.RuneError) {
		return "", "", false
	}
	return string(out), "GB18030", true
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// ---------- 原始文件 ----------

func (s *server) handleRaw(w http.ResponseWriter, r *http.Request) {
	_, abs, err := s.resolve(strings.TrimPrefix(r.URL.Path, "/raw/"))
	if err != nil {
		writeErr(w, err)
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	ctype := mime.TypeByExtension(filepath.Ext(abs))
	if ctype == "" || strings.HasPrefix(ctype, "text/") || strings.Contains(ctype, "javascript") {
		// 文本统一按纯文本返回，避免浏览器把 .html/.js 当页面执行
		var head [512]byte
		n, _ := io.ReadFull(f, head[:])
		f.Seek(0, io.SeekStart)
		if isBinary(head[:n]) {
			ctype = "application/octet-stream"
		} else {
			ctype = "text/plain; charset=utf-8"
		}
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": info.Name()}))
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// ---------- 页面 ----------

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if p != "/" && !strings.HasPrefix(p, "/tree/") && !strings.HasPrefix(p, "/blob/") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	s.index.Execute(w, map[string]any{"Title": s.cfg.title, "Version": version, "Lang": s.cfg.lang, "Themes": themes, "Grep": s.cfg.grep})
}

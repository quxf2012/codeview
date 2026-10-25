// codeview: 一个类 GitHub 的本地代码浏览器。
//
// 单二进制，前端资源通过 embed 打包。左侧是可展开的目录树，
// 右侧显示目录列表（含 README 渲染）或文件内容（代码高亮 / Markdown 渲染）。
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed web
var webFS embed.FS

var version = "dev"

type config struct {
	root       string
	addr       string
	title      string
	showHidden bool
	ignore     map[string]bool
	maxView    int64  // 超过这个大小的文件不做高亮/渲染，只给下载链接
	lang       string // 界面默认语言，浏览器里可切换
	grep       bool   // 是否开启全文搜索（每次搜索都要读一遍文件，大目录较慢，默认关闭）
}

func main() {
	var (
		root    = flag.String("root", envOr("CODEVIEW_ROOT", "."), "root directory to browse (env CODEVIEW_ROOT)")
		addr    = flag.String("addr", envOr("CODEVIEW_ADDR", "127.0.0.1:8080"), "listen address, use 0.0.0.0:8080 for LAN access (env CODEVIEW_ADDR)")
		title   = flag.String("title", "", "page title (default: name of the root directory)")
		hidden  = flag.Bool("hidden", false, "show dotfiles")
		ignore  = flag.String("ignore", ".git,node_modules,__pycache__,.venv,.idea,.vscode", "comma-separated file/directory names to ignore")
		maxView = flag.Int64("max-size", 2<<20, "max file size in bytes to render in the browser")
		grep    = flag.Bool("grep", envOr("CODEVIEW_GREP", "") == "1", "enable full-text search (env CODEVIEW_GREP=1)")
		lang    = flag.String("lang", envOr("CODEVIEW_LANG", "en"), "default UI language: en or zh (env CODEVIEW_LANG)")
		showVer = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()
	if *showVer {
		fmt.Println("codeview", version)
		return
	}
	if flag.NArg() > 0 { // 也支持 `codeview /path/to/dir`
		*root = flag.Arg(0)
	}

	abs, err := filepath.Abs(*root)
	if err != nil {
		log.Fatal(err)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		log.Fatalf("root directory unavailable: %v", err)
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		log.Fatalf("root is not a directory: %s", abs)
	}

	cfg := &config{
		root:       abs,
		addr:       *addr,
		title:      *title,
		showHidden: *hidden,
		ignore:     map[string]bool{},
		maxView:    *maxView,
		lang:       *lang,
		grep:       *grep,
	}
	if cfg.lang != "en" && cfg.lang != "zh" {
		log.Fatalf("unsupported -lang %q (use en or zh)", cfg.lang)
	}
	if cfg.title == "" {
		cfg.title = filepath.Base(abs)
	}
	for _, s := range strings.Split(*ignore, ",") {
		if s = strings.TrimSpace(s); s != "" {
			cfg.ignore[s] = true
		}
	}

	srv := newServer(cfg)
	log.Printf("codeview %s  root %s", version, abs)
	log.Printf("listening on http://%s/", displayAddr(cfg.addr))
	hs := &http.Server{
		Addr:              cfg.addr,
		Handler:           logRequests(srv),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(hs.ListenAndServe())
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func displayAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	if strings.HasPrefix(addr, "0.0.0.0:") {
		return "localhost" + addr[len("0.0.0.0"):]
	}
	return addr
}

func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/raw/") {
			log.Printf("%s %s %s", r.Method, r.URL.RequestURI(), time.Since(start).Round(time.Millisecond))
		}
	})
}

func mustSub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err)
	}
	return sub
}

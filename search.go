package main

import (
	"bufio"
	"bytes"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	maxWalk        = 100000 // 最多遍历的条目数，防止根目录选得太大时卡死
	maxNameResults = 100
	maxGrepFiles   = 100
	maxGrepPerFile = 5
	maxGrepSize    = 1 << 20
)

// walk 遍历根目录下的文件（跳过隐藏/忽略项），fn 返回 false 时停止。
func (s *server) walk(fn func(rel, abs string, d fs.DirEntry) bool) {
	count := 0
	filepath.WalkDir(s.cfg.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p == s.cfg.root {
			return nil
		}
		if s.hiddenName(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if count++; count > maxWalk {
			return filepath.SkipAll
		}
		rel, _ := filepath.Rel(s.cfg.root, p)
		if !fn(filepath.ToSlash(rel), p, d) {
			return filepath.SkipAll
		}
		return nil
	})
}

type nameHit struct {
	Path  string `json:"path"`
	Type  string `json:"type"`
	score int
}

// handleSearch 按文件名搜索（GitHub 上按 t 的 "Go to file"）。
// 支持子序列模糊匹配，比如 "lbmd" 能匹配 "08-lb.md"。
func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	hits := []nameHit{}
	if q == "" {
		writeJSON(w, http.StatusOK, hits)
		return
	}
	s.walk(func(rel, _ string, d fs.DirEntry) bool {
		if sc := fuzzyScore(strings.ToLower(rel), strings.ToLower(d.Name()), q); sc > 0 {
			t := "file"
			if d.IsDir() {
				t = "dir"
			}
			hits = append(hits, nameHit{Path: rel, Type: t, score: sc})
		}
		return true
	})
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return len(hits[i].Path) < len(hits[j].Path)
	})
	if len(hits) > maxNameResults {
		hits = hits[:maxNameResults]
	}
	writeJSON(w, http.StatusOK, hits)
}

func fuzzyScore(path, name, q string) int {
	switch {
	case name == q:
		return 1000
	case strings.HasPrefix(name, q):
		return 800
	case strings.Contains(name, q):
		return 600
	case strings.Contains(path, q):
		return 400
	}
	// 子序列匹配
	i := 0
	for _, r := range path {
		if i < len(q) {
			qr, size := utf8.DecodeRuneInString(q[i:])
			if r == qr {
				i += size
			}
		}
	}
	if i == len(q) {
		return 100
	}
	return 0
}

type grepLine struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

type grepHit struct {
	Path  string     `json:"path"`
	Lines []grepLine `json:"lines"`
}

// handleGrep 全文搜索（不区分大小写的子串匹配）。
func (s *server) handleGrep(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	hits := []grepHit{}
	if q == "" {
		writeJSON(w, http.StatusOK, hits)
		return
	}
	needle := bytes.ToLower([]byte(q))
	s.walk(func(rel, abs string, d fs.DirEntry) bool {
		if d.IsDir() {
			return true
		}
		if info, err := d.Info(); err != nil || info.Size() > maxGrepSize || !info.Mode().IsRegular() {
			return true
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return true
		}
		text, _, ok := decodeText(data)
		if !ok || !bytes.Contains(bytes.ToLower([]byte(text)), needle) {
			return true
		}
		hit := grepHit{Path: rel}
		sc := bufio.NewScanner(strings.NewReader(text))
		sc.Buffer(make([]byte, 64*1024), maxGrepSize)
		for n := 1; sc.Scan(); n++ {
			line := sc.Text()
			if bytes.Contains(bytes.ToLower([]byte(line)), needle) {
				hit.Lines = append(hit.Lines, grepLine{Line: n, Text: truncate(line, 200)})
				if len(hit.Lines) >= maxGrepPerFile {
					break
				}
			}
		}
		hits = append(hits, hit)
		return len(hits) < maxGrepFiles
	})
	writeJSON(w, http.StatusOK, hits)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

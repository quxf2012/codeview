package main

import (
	"bytes"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// ---------- 代码高亮（chroma，和 Hugo / Gitea 同款） ----------

var codeFormatter = chromahtml.New(
	chromahtml.WithClasses(true),
	chromahtml.WithLineNumbers(true),
	chromahtml.LineNumbersInTable(true), // 表格形式，复制代码时不会带上行号
	chromahtml.WithLinkableLineNumbers(true, "L"),
	chromahtml.TabWidth(4),
)

// 一些 chroma 认不出来的常见文件名
var lexerAlias = map[string]string{
	"dockerfile":       "docker",
	"containerfile":    "docker",
	"makefile":         "make",
	"jenkinsfile":      "groovy",
	"vagrantfile":      "ruby",
	".bashrc":          "bash",
	".zshrc":           "bash",
	".profile":         "bash",
	".bash_profile":    "bash",
	".gitignore":       "plaintext",
	".env":             "bash",
	"go.mod":           "go",
	"go.sum":           "plaintext",
	"kubeconfig":       "yaml",
	"requirements.txt": "plaintext",
}

func pickLexer(name, src string) chroma.Lexer {
	lower := strings.ToLower(name)
	if alias, ok := lexerAlias[lower]; ok {
		if l := lexers.Get(alias); l != nil {
			return l
		}
	}
	if strings.HasPrefix(lower, "dockerfile") {
		return lexers.Get("docker")
	}
	if l := lexers.Match(name); l != nil {
		return l
	}
	switch strings.ToLower(path.Ext(name)) {
	case ".conf", ".cnf", ".cfg":
		return lexers.Get("ini")
	case ".tpl", ".tmpl", ".gotmpl":
		return lexers.Get("go-text-template")
	}
	// 没有扩展名时看 shebang / 内容猜
	if l := lexers.Analyse(src); l != nil {
		return l
	}
	return lexers.Fallback
}

func highlight(name, src string) (string, string, error) {
	lexer := chroma.Coalesce(pickLexer(name, src))
	it, err := lexer.Tokenise(nil, src)
	if err != nil {
		return "", "", err
	}
	var buf bytes.Buffer
	if err := codeFormatter.Format(&buf, styles.Fallback, it); err != nil {
		return "", "", err
	}
	return buf.String(), lexer.Config().Name, nil
}

// ---------- Markdown（goldmark + GFM） ----------

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM, // 表格、删除线、任务列表、自动链接
		extension.Footnote,
		highlighting.NewHighlighting(
			highlighting.WithFormatOptions(chromahtml.WithClasses(true)),
			highlighting.WithGuessLanguage(false),
			highlighting.WithWrapperRenderer(codeBlockWrapper),
		),
	),
	goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(html.WithUnsafe()), // 本地笔记，允许内嵌 HTML
)

type outlineItem struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	ID    string `json:"id,omitempty"`   // Markdown 标题锚点
	Line  int    `json:"line,omitempty"` // 代码符号所在行
}

var frontMatterRe = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n`)

func renderMarkdown(src []byte) (string, []outlineItem, error) {
	var front string
	lineOffset := 0
	if m := frontMatterRe.FindSubmatch(src); m != nil {
		front = string(m[1])
		lineOffset = bytes.Count(m[0], []byte("\n"))
		src = src[len(m[0]):]
	}

	ctx := parser.NewContext(parser.WithIDs(newGitHubIDs()))
	doc := md.Parser().Parse(text.NewReader(src), parser.WithContext(ctx))
	annotateSourceLines(doc, src, lineOffset)

	var outline []outlineItem
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && entering {
			id, _ := h.AttributeString("id")
			idStr := ""
			if b, ok := id.([]byte); ok {
				idStr = string(b)
			}
			outline = append(outline, outlineItem{Level: h.Level, Text: nodeText(h, src), ID: idStr})
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})

	var buf bytes.Buffer
	if front != "" {
		h, _, _ := highlight("front.yaml", front)
		buf.WriteString(`<div class="front-matter">` + h + `</div>`)
	}
	if err := md.Renderer().Render(&buf, src, doc); err != nil {
		return "", nil, err
	}
	return buf.String(), outline, nil
}

// annotateSourceLines 给块级元素加上 data-sl / data-el（对应源码的起止行号，从 1 开始），
// 前端据此把"相比首次打开的改动"标到渲染后的 Markdown 上。
func annotateSourceLines(doc ast.Node, src []byte, lineOffset int) {
	starts := []int{0}
	for i, c := range src {
		if c == '\n' {
			starts = append(starts, i+1)
		}
	}
	lineOf := func(off int) int { return sort.SearchInts(starts, off+1) + lineOffset }

	var span func(n ast.Node) (int, int)
	span = func(n ast.Node) (int, int) {
		lo, hi := -1, -1
		add := func(a, b int) {
			if a < 0 {
				return
			}
			if lo < 0 || a < lo {
				lo = a
			}
			if b > hi {
				hi = b
			}
		}
		if n.Type() == ast.TypeBlock {
			ls := n.Lines()
			for i := 0; i < ls.Len(); i++ {
				seg := ls.At(i)
				add(seg.Start, seg.Stop)
			}
		}
		if t, ok := n.(*ast.Text); ok {
			add(t.Segment.Start, t.Segment.Stop)
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			add(span(c))
		}
		if lo >= 0 && wantsLineAttr(n) {
			sl, el := lineOf(lo), lineOf(max(lo, hi-1))
			if n.Kind() == ast.KindFencedCodeBlock { // 内容行不含 ``` 两行
				sl, el = sl-1, el+1
			}
			n.SetAttributeString("data-sl", []byte(strconv.Itoa(sl)))
			n.SetAttributeString("data-el", []byte(strconv.Itoa(el)))
		}
		return lo, hi
	}
	span(doc)
}

func wantsLineAttr(n ast.Node) bool {
	switch n.Kind() {
	case ast.KindParagraph, ast.KindHeading, ast.KindFencedCodeBlock, ast.KindCodeBlock,
		ast.KindHTMLBlock, ast.KindListItem, extast.KindTableRow, extast.KindTableHeader:
		return true
	}
	return false
}

// codeBlockWrapper 用 div 包住代码块，带上行号属性。
func codeBlockWrapper(w util.BufWriter, c highlighting.CodeBlockContext, entering bool) {
	if entering {
		w.WriteString(`<div class="md-code"`)
		if attrs := c.Attributes(); attrs != nil {
			for _, name := range []string{"data-sl", "data-el"} {
				if v, ok := attrs.GetString(name); ok {
					if b, ok := v.([]byte); ok {
						fmt.Fprintf(w, ` %s="%s"`, name, b)
					}
				}
			}
		}
		w.WriteString(">")
		if !c.Highlighted() {
			w.WriteString("<pre><code>")
		}
		return
	}
	if !c.Highlighted() {
		w.WriteString("</code></pre>")
	}
	w.WriteString("</div>\n")
}

func nodeText(n ast.Node, src []byte) string {
	var b strings.Builder
	ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := c.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(src))
		case *ast.String:
			b.Write(t.Value)
		case *ast.CodeSpan:
			for cc := t.FirstChild(); cc != nil; cc = cc.NextSibling() {
				if tt, ok := cc.(*ast.Text); ok {
					b.Write(tt.Segment.Value(src))
				}
			}
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(b.String())
}

// githubIDs 生成和 GitHub 一致的标题锚点：保留中文等 Unicode 字母，
// 空格变 -，去掉标点；重复的加 -1、-2 后缀。goldmark 默认会丢掉中文。
type githubIDs struct{ seen map[string]int }

func newGitHubIDs() *githubIDs { return &githubIDs{seen: map[string]int{}} }

func (g *githubIDs) Generate(value []byte, kind ast.NodeKind) []byte {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(string(value))) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_':
			b.WriteRune(r)
		case r == ' ':
			b.WriteByte('-')
		}
	}
	id := b.String()
	if id == "" {
		id = "heading"
	}
	base := id
	for n := 1; g.seen[id] > 0; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	g.seen[id]++
	return []byte(id)
}

func (g *githubIDs) Put(value []byte) { g.seen[string(value)]++ }

// ---------- 代码大纲（右侧"文件结构"） ----------
// 用正则粗略提取，够浏览笔记 / 脚本用；不追求完整的语法分析。

type outlineRule struct {
	re    *regexp.Regexp
	level func(m []string) int
}

func indentLevel(s string) int { return len(strings.ReplaceAll(s, "\t", "    "))/2 + 1 }

var outlineRules = map[string][]outlineRule{
	"Go": {
		{re: regexp.MustCompile(`^func\s+(\([^)]*\)\s*)?([A-Za-z_]\w*)`)},
		{re: regexp.MustCompile(`^type\s+([A-Za-z_]\w*)\s+(struct|interface|func|\w)`)},
	},
	"Python": {
		{re: regexp.MustCompile(`^(\s*)(?:async\s+)?(?:def|class)\s+([A-Za-z_]\w*)`), level: func(m []string) int { return len(m[1])/4 + 1 }},
	},
	"Bash": {
		{re: regexp.MustCompile(`^\s*(?:function\s+)?([A-Za-z_][\w:-]*)\s*\(\)\s*\{?`)},
		{re: regexp.MustCompile(`^\s*function\s+([A-Za-z_][\w:-]*)`)},
	},
	"YAML": {
		// 只取前两层 key，k8s 清单里 apiVersion/kind/metadata/spec 一目了然
		{re: regexp.MustCompile(`^(\s{0,2})([A-Za-z_][\w.-]*):(\s|$)`), level: func(m []string) int { return len(m[1])/2 + 1 }},
		{re: regexp.MustCompile(`^(---)`)},
	},
	"JavaScript": {
		{re: regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)`)},
		{re: regexp.MustCompile(`^\s*(?:export\s+)?class\s+([A-Za-z_$][\w$]*)`)},
	},
}

func init() {
	outlineRules["TypeScript"] = outlineRules["JavaScript"]
	outlineRules["Shell"] = outlineRules["Bash"]
}

func codeOutline(name, lang, src string) []outlineItem {
	rules, ok := outlineRules[lang]
	if !ok {
		return nil
	}
	var out []outlineItem
	for i, line := range strings.Split(src, "\n") {
		for _, r := range rules {
			m := r.re.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			level := 1
			if r.level != nil {
				level = r.level(m)
			}
			label := m[len(m)-1]
			switch lang {
			case "Go":
				label = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), "{"))
				if len(label) > 80 {
					label = label[:80] + "…"
				}
			case "YAML":
				if m[1] == "---" {
					label = "---"
				} else {
					label = m[2]
				}
			case "Python", "Bash", "Shell":
				label = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line), ":"))
			}
			out = append(out, outlineItem{Level: level, Text: label, Line: i + 1})
			break
		}
		if len(out) >= 500 {
			break
		}
	}
	return out
}

// naturalLess 自然排序：file2 排在 file10 前面，01-intro 和 02-arch 按数字排。
func naturalLess(a, b string) bool {
	for a != "" && b != "" {
		ca, cb := a[0], b[0]
		if isDigit(ca) && isDigit(cb) {
			na, ra := splitNum(a)
			nb, rb := splitNum(b)
			ta, tb := strings.TrimLeft(na, "0"), strings.TrimLeft(nb, "0")
			if len(ta) != len(tb) {
				return len(ta) < len(tb)
			}
			if ta != tb {
				return ta < tb
			}
			a, b = ra, rb
			continue
		}
		if ca != cb {
			return ca < cb
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func splitNum(s string) (string, string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/styles"
)

// theme 是一套界面配色 + 对应的 chroma 高亮样式。
// 前端选中后在 <html> 上设置 data-theme=<ID> 和 data-scheme=<Scheme>：
// Scheme（light/dark）决定改动着色、阴影等通用颜色，Palette 覆盖界面主色。
type theme struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Scheme  string            `json:"scheme"`
	Chroma  string            `json:"-"`
	Palette map[string]string `json:"palette"` // CSS 变量名（不含 --）-> 颜色，也用于选择器里的色块预览
}

var themes = []theme{
	{ID: "github-light", Name: "GitHub Light", Scheme: "light", Chroma: "github", Palette: map[string]string{
		"bg": "#ffffff", "bg-subtle": "#f6f8fa", "bg-hover": "#eaeef2", "bg-active": "#ddf4ff",
		"fg": "#1f2328", "fg-muted": "#59636e", "border": "#d1d9e0", "border-muted": "#d8dee4",
		"accent": "#0969da", "dir": "#54aeff", "code-bg": "#f6f8fa", "code-fg": "#c7254e"}},
	{ID: "github-dark", Name: "GitHub Dark", Scheme: "dark", Chroma: "github-dark", Palette: map[string]string{
		"bg": "#0d1117", "bg-subtle": "#151b23", "bg-hover": "#262c36", "bg-active": "#1f3049",
		"fg": "#e6edf3", "fg-muted": "#9198a1", "border": "#3d444d", "border-muted": "#2f353d",
		"accent": "#4493f8", "dir": "#7d8590", "code-bg": "#151b23", "code-fg": "#ff7b72"}},
	{ID: "dracula", Name: "Dracula", Scheme: "dark", Chroma: "dracula", Palette: map[string]string{
		"bg": "#282a36", "bg-subtle": "#21222c", "bg-hover": "#343746", "bg-active": "#44475a",
		"fg": "#f8f8f2", "fg-muted": "#a0a8cd", "border": "#44475a", "border-muted": "#383a4a",
		"accent": "#bd93f9", "dir": "#bd93f9", "code-bg": "#21222c", "code-fg": "#ff79c6"}},
	{ID: "monokai", Name: "Monokai", Scheme: "dark", Chroma: "monokai", Palette: map[string]string{
		"bg": "#272822", "bg-subtle": "#1e1f1c", "bg-hover": "#3e3d32", "bg-active": "#49483e",
		"fg": "#f8f8f2", "fg-muted": "#a59f85", "border": "#49483e", "border-muted": "#3b3a32",
		"accent": "#66d9ef", "dir": "#e6db74", "code-bg": "#1e1f1c", "code-fg": "#f92672"}},
	{ID: "one-dark", Name: "One Dark", Scheme: "dark", Chroma: "onedark", Palette: map[string]string{
		"bg": "#282c34", "bg-subtle": "#21252b", "bg-hover": "#2c313a", "bg-active": "#3a3f4b",
		"fg": "#abb2bf", "fg-muted": "#7f848e", "border": "#3e4451", "border-muted": "#333842",
		"accent": "#61afef", "dir": "#61afef", "code-bg": "#21252b", "code-fg": "#e06c75"}},
	{ID: "nord", Name: "Nord", Scheme: "dark", Chroma: "nord", Palette: map[string]string{
		"bg": "#2e3440", "bg-subtle": "#3b4252", "bg-hover": "#434c5e", "bg-active": "#4c566a",
		"fg": "#eceff4", "fg-muted": "#a5adba", "border": "#4c566a", "border-muted": "#434c5e",
		"accent": "#88c0d0", "dir": "#81a1c1", "code-bg": "#3b4252", "code-fg": "#d08770"}},
	{ID: "tokyo-night", Name: "Tokyo Night", Scheme: "dark", Chroma: "tokyonight-night", Palette: map[string]string{
		"bg": "#1a1b26", "bg-subtle": "#16161e", "bg-hover": "#292e42", "bg-active": "#33467c",
		"fg": "#c0caf5", "fg-muted": "#8089b3", "border": "#3b4261", "border-muted": "#292e42",
		"accent": "#7aa2f7", "dir": "#7aa2f7", "code-bg": "#16161e", "code-fg": "#f7768e"}},
	{ID: "catppuccin-mocha", Name: "Catppuccin Mocha", Scheme: "dark", Chroma: "catppuccin-mocha", Palette: map[string]string{
		"bg": "#1e1e2e", "bg-subtle": "#181825", "bg-hover": "#313244", "bg-active": "#45475a",
		"fg": "#cdd6f4", "fg-muted": "#a6adc8", "border": "#45475a", "border-muted": "#313244",
		"accent": "#89b4fa", "dir": "#89b4fa", "code-bg": "#181825", "code-fg": "#f38ba8"}},
	{ID: "gruvbox-dark", Name: "Gruvbox Dark", Scheme: "dark", Chroma: "gruvbox", Palette: map[string]string{
		"bg": "#282828", "bg-subtle": "#1d2021", "bg-hover": "#3c3836", "bg-active": "#504945",
		"fg": "#ebdbb2", "fg-muted": "#a89984", "border": "#504945", "border-muted": "#3c3836",
		"accent": "#83a598", "dir": "#fabd2f", "code-bg": "#1d2021", "code-fg": "#fe8019"}},
	{ID: "rose-pine", Name: "Rosé Pine", Scheme: "dark", Chroma: "rose-pine", Palette: map[string]string{
		"bg": "#191724", "bg-subtle": "#1f1d2e", "bg-hover": "#26233a", "bg-active": "#403d52",
		"fg": "#e0def4", "fg-muted": "#908caa", "border": "#403d52", "border-muted": "#26233a",
		"accent": "#c4a7e7", "dir": "#9ccfd8", "code-bg": "#1f1d2e", "code-fg": "#ebbcba"}},
	{ID: "solarized-dark", Name: "Solarized Dark", Scheme: "dark", Chroma: "solarized-dark", Palette: map[string]string{
		"bg": "#002b36", "bg-subtle": "#073642", "bg-hover": "#0d3f4c", "bg-active": "#114a59",
		"fg": "#b3bfbf", "fg-muted": "#839496", "border": "#0f4a58", "border-muted": "#0a3c48",
		"accent": "#268bd2", "dir": "#2aa198", "code-bg": "#073642", "code-fg": "#d33682"}},
	{ID: "solarized-light", Name: "Solarized Light", Scheme: "light", Chroma: "solarized-light", Palette: map[string]string{
		"bg": "#fdf6e3", "bg-subtle": "#eee8d5", "bg-hover": "#e6dfc8", "bg-active": "#dfd8bf",
		"fg": "#586e75", "fg-muted": "#839496", "border": "#d9d2bc", "border-muted": "#e4ddc8",
		"accent": "#268bd2", "dir": "#2aa198", "code-bg": "#eee8d5", "code-fg": "#d33682"}},
	{ID: "catppuccin-latte", Name: "Catppuccin Latte", Scheme: "light", Chroma: "catppuccin-latte", Palette: map[string]string{
		"bg": "#eff1f5", "bg-subtle": "#e6e9ef", "bg-hover": "#dce0e8", "bg-active": "#ccd0da",
		"fg": "#4c4f69", "fg-muted": "#6c6f85", "border": "#ccd0da", "border-muted": "#dce0e8",
		"accent": "#1e66f5", "dir": "#1e66f5", "code-bg": "#e6e9ef", "code-fg": "#d20f39"}},
}

// paletteOrder 让生成的 CSS 稳定（map 遍历无序）。
var paletteOrder = []string{"bg", "bg-subtle", "bg-hover", "bg-active", "fg", "fg-muted", "border", "border-muted", "accent", "dir", "code-bg", "code-fg"}

// themeCSS 生成所有主题的界面变量和代码高亮样式，选择器都限定在 :root[data-theme="<id>"] 下。
func themeCSS() []byte {
	var buf bytes.Buffer
	for _, t := range themes {
		scope := fmt.Sprintf(`:root[data-theme=%q]`, t.ID)
		buf.WriteString(scope + " {")
		for _, k := range paletteOrder {
			fmt.Fprintf(&buf, " --%s: %s;", k, t.Palette[k])
		}
		fmt.Fprintf(&buf, " color-scheme: %s; }\n", t.Scheme)

		var b bytes.Buffer
		if err := chromahtml.New(chromahtml.WithClasses(true)).WriteCSS(&b, styles.Get(t.Chroma)); err != nil {
			continue
		}
		for _, line := range strings.Split(b.String(), "\n") {
			// 背景和默认文字色交给界面变量，保证代码区和页面一致
			if strings.Contains(line, "/* Background */") {
				continue
			}
			if i := strings.Index(line, ".chroma"); i >= 0 {
				line = line[:i] + scope + " " + line[i:]
			}
			buf.WriteString(line + "\n")
		}
		writeTokenFallbacks(&buf, scope, styles.Get(t.Chroma))
	}
	return buf.Bytes()
}

// 有些样式没给这些类别配颜色（比如 github-dark 没有 NameAttribute / NameBuiltin），
// shell 的参数、内置命令就会和普通文字一样。没配的借用同一样式里相近类别的颜色。
var tokenFallbacks = []struct {
	tt   chroma.TokenType
	from []chroma.TokenType
}{
	{chroma.NameFunction, []chroma.TokenType{chroma.NameClass, chroma.Keyword}},
	{chroma.NameAttribute, []chroma.TokenType{chroma.NameTag, chroma.NameVariable, chroma.NameConstant, chroma.Keyword}},
	{chroma.NameBuiltin, []chroma.TokenType{chroma.NameClass, chroma.KeywordType, chroma.NameFunction}},
	{chroma.NameVariable, []chroma.TokenType{chroma.NameConstant, chroma.LiteralStringOther, chroma.LiteralNumber}},
	{chroma.LiteralNumber, []chroma.TokenType{chroma.LiteralNumberInteger, chroma.LiteralString}},
}

func writeTokenFallbacks(buf *bytes.Buffer, scope string, style *chroma.Style) {
	plain := style.Get(chroma.Text).Colour
	colored := func(tt chroma.TokenType) (chroma.Colour, bool) {
		c := style.Get(tt).Colour
		return c, c.IsSet() && c != plain
	}
	for _, f := range tokenFallbacks {
		if _, ok := colored(f.tt); ok {
			continue
		}
		for _, from := range f.from {
			if c, ok := colored(from); ok {
				fmt.Fprintf(buf, "%s .chroma .%s { color: %s }\n", scope, chroma.StandardTypes[f.tt], c)
				break
			}
		}
	}
}

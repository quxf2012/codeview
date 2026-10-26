package main

import (
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// chroma 的 bash 词法只认关键字、字符串、变量和少数内置命令，
// 像 `kubectl get pods -A` 这样的普通命令行整行都是 Text，看起来没有颜色。
// shellLexer 包一层 bash 词法，在它的结果上再标出：
//   - 命令名（每条命令的第一个词）       -> NameFunction
//   - 参数 -x / --flag                   -> NameAttribute
//   - 占位符 <pod-name>                  -> NameVariable
//   - 数字、端口 8080:80                 -> LiteralNumber
//
// 注册时沿用 bash 的名字和别名，代码文件和 Markdown 代码块都会用到它。
type shellLexer struct {
	chroma.Lexer
}

func init() {
	if bash := lexers.Get("bash"); bash != nil {
		lexers.Register(&shellLexer{bash})
	}
}

func (l *shellLexer) Tokenise(opts *chroma.TokeniseOptions, text string) (chroma.Iterator, error) {
	it, err := l.Lexer.Tokenise(opts, text)
	if err != nil {
		return nil, err
	}
	return chroma.Literator(refineShell(it.Tokens())...), nil
}

var (
	reShellSpace  = regexp.MustCompile(`\s+|\S+`)
	reShellNumber = regexp.MustCompile(`^\d+([:.]\d+)*$`)
	reShellHole   = regexp.MustCompile(`^<[\w.:/-]+>$`)
)

// 这些词后面跟的还是一条命令，比如 `sudo apt install`、`xargs rm`
var shellPrefixCmds = map[string]bool{
	"sudo": true, "env": true, "time": true, "nohup": true, "xargs": true,
	"watch": true, "exec": true, "command": true, "builtin": true, "nice": true,
}

// 这些关键字后面是一条新命令的开头
var shellCmdKeywords = map[string]bool{
	"then": true, "do": true, "else": true, "elif": true, "if": true,
	"while": true, "until": true, "!": true, "time": true,
}

func refineShell(in []chroma.Token) []chroma.Token {
	// 先把 Text 拆成「空白 / 单词」，方便逐词判断
	toks := make([]chroma.Token, 0, len(in))
	for _, t := range in {
		if t.Type != chroma.Text || t.Value == "" {
			toks = append(toks, t)
			continue
		}
		for _, part := range reShellSpace.FindAllString(t.Value, -1) {
			toks = append(toks, chroma.Token{Type: chroma.Text, Value: part})
		}
	}
	// `<pod-name>` 会被切成 "<" 和 "pod-name>"，合成一个
	merged := toks[:0]
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		if t.Type == chroma.Text && t.Value == "<" && i+1 < len(toks) && toks[i+1].Type == chroma.Text {
			if v := "<" + toks[i+1].Value; reShellHole.MatchString(v) {
				merged = append(merged, chroma.Token{Type: chroma.NameVariable, Value: v})
				i++
				continue
			}
		}
		merged = append(merged, t)
	}
	toks = merged

	cmdPos := true     // 下一个词是不是命令名
	assignVal := false // 刚看到 `FOO=`，下一个词是值
	for i := range toks {
		t := &toks[i]
		v := t.Value
		switch {
		case strings.TrimSpace(v) == "":
			if strings.Contains(v, "\n") {
				cmdPos, assignVal = true, false
			}
			continue
		case t.Type.InCategory(chroma.Comment):
			continue
		case t.Type == chroma.Punctuation || t.Type == chroma.Operator || t.Type == chroma.Keyword:
			switch {
			case v == "=":
				if cmdPos {
					assignVal = true
				}
			case strings.ContainsAny(v, ";|&({`"):
				cmdPos, assignVal = true, false
			case t.Type == chroma.Keyword && shellCmdKeywords[v]:
				cmdPos = true
			case t.Type == chroma.Keyword:
				cmdPos = false
			}
			continue
		}

		if assignVal {
			// `FOO=bar cmd` 里的 bar：保持原样，后面还是命令
			assignVal = false
			continue
		}
		if cmdPos {
			switch {
			case t.Type == chroma.NameVariable:
				// `FOO=bar cmd` 的变量名，等后面的 `=`
				continue
			case t.Type == chroma.Text && (v == "$" || v == "#" || v == ">"):
				// 命令提示符
				t.Type = chroma.GenericPrompt
				continue
			case t.Type == chroma.Text && strings.Contains(v, "=") && !strings.HasPrefix(v, "-"):
				continue
			case t.Type == chroma.Text:
				t.Type = chroma.NameFunction
			}
			cmdPos = shellPrefixCmds[v]
			continue
		}
		switch {
		case t.Type == chroma.NameBuiltin:
			// `kubectl exec` 里的 exec 是子命令，不是 shell 内置命令
			t.Type = chroma.Text
		case t.Type != chroma.Text:
		case strings.HasPrefix(v, "-") && len(v) > 1 && v != "--":
			t.Type = chroma.NameAttribute
		case reShellNumber.MatchString(v):
			t.Type = chroma.LiteralNumber
		}
	}
	return toks
}

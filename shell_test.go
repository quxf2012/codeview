package main

import (
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestShellLexer(t *testing.T) {
	src := "$ kubectl get pods -A -n kube-system | grep Running && echo ok\n" +
		"kubectl exec -it pod -- /bin/sh\n" +
		"sudo apt install -y curl \\\n  --no-install-recommends\n" +
		"kubectl port-forward <pod-name> 8080:80\n" +
		"FOO=1 make build # comment -x\n"
	for _, name := range []string{"bash", "sh", "shell", "zsh"} {
		if _, ok := lexers.Get(name).(*shellLexer); !ok {
			t.Fatalf("%s is not using shellLexer", name)
		}
	}
	if _, ok := lexers.Match("run.sh").(*shellLexer); !ok {
		t.Fatal("*.sh is not using shellLexer")
	}
	it, err := lexers.Get("bash").Tokenise(nil, src)
	if err != nil {
		t.Fatal(err)
	}
	toks := it.Tokens()
	var joined string
	got := map[string]chroma.TokenType{}
	for _, tk := range toks {
		joined += tk.Value
		if _, seen := got[tk.Value]; !seen {
			got[tk.Value] = tk.Type
		}
	}
	want := map[string]chroma.TokenType{
		"$":                       chroma.GenericPrompt,
		"kubectl":                 chroma.NameFunction,
		"get":                     chroma.Text,
		"-A":                      chroma.NameAttribute,
		"kube-system":             chroma.Text,
		"grep":                    chroma.NameFunction,
		"Running":                 chroma.Text,
		"echo":                    chroma.NameBuiltin,
		"exec":                    chroma.Text,
		"--":                      chroma.Text,
		"sudo":                    chroma.NameFunction,
		"apt":                     chroma.NameFunction,
		"install":                 chroma.Text,
		"--no-install-recommends": chroma.NameAttribute,
		"<pod-name>":              chroma.NameVariable,
		"8080:80":                 chroma.LiteralNumber,
		"make":                    chroma.NameFunction,
		"build":                   chroma.Text,
	}
	for v, tt := range want {
		if got[v] != tt {
			t.Errorf("%q: got %v, want %v", v, got[v], tt)
		}
	}
	if joined != src {
		t.Errorf("tokens do not round-trip the source:\n%q", joined)
	}
}

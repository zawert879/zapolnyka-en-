package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriteNote(t *testing.T) {
	files := []string{"1/notes.md", "1/img/pic.png", "1/idea.md", "notes/map.png", "notes/plan.md", "2/pic.png"}
	levels := []string{"1", "2"}
	run := func(md, text, from, to string) string {
		var after []string
		for _, f := range files {
			after = append(after, mapPath(f, from, to))
		}
		return rewriteNote(text, md, mapPath(md, from, to), newLayout(files, levels), newLayout(after, levels), from, to)
	}
	cases := []struct {
		name, md, text, from, to, want string
	}{
		{"картинка", "1/notes.md", "см. ![](img/pic.png) и ![x](img/pic.png \"t\")", "1/img/pic.png", "1/img/скрин 1.png",
			"см. ![](img/скрин%201.png) и ![x](img/скрин%201.png \"t\")"},
		{"папка", "1/notes.md", "![](img/pic.png)", "1/img", "1/pics", "![](pics/pic.png)"},
		{"в notes", "1/notes.md", "![](../notes/map.png)", "notes/map.png", "notes/maps/map.png", "![](../notes/maps/map.png)"},
		{"угловые скобки", "1/notes.md", "![](<img/pic.png>)", "1/img/pic.png", "1/img/p q.png", "![](<img/p q.png>)"},
		{"вики по имени", "1/notes.md", "[[idea]] и [[idea|алиас]] и ![[pic.png|200]]", "1/idea.md", "1/мысль.md",
			"[[мысль]] и [[мысль|алиас]] и ![[pic.png|200]]"},
		{"вики в таблице", "1/notes.md", "| ![[pic.png\\|200]] |", "1/img/pic.png", "1/img/new.png", "| ![[new.png\\|200]] |"},
		{"вики картинка", "1/notes.md", "![[pic.png|200]]", "1/img/pic.png", "1/img/new.png", "![[new.png|200]]"},
		{"вики неоднозначно", "1/notes.md", "![[map.png]]", "notes/map.png", "notes/sub/pic.png", "![[../notes/sub/pic.png]]"},
		{"заметка переехала", "1/idea.md", "![](img/pic.png) [[plan]]", "1/idea.md", "1/sub/idea.md", "![](../img/pic.png) [[plan]]"},
		{"код не трогаем", "1/notes.md", "`![](img/pic.png)`\n```\n![](img/pic.png)\n```\n![](img/pic.png)", "1/img/pic.png", "1/img/b.png",
			"`![](img/pic.png)`\n```\n![](img/pic.png)\n```\n![](img/b.png)"},
		{"чужие ссылки", "1/notes.md", "[сайт](https://en.cx) ![]({{logo.png}}) [я](#top) ![](img/pic.png#x)", "1/img/pic.png", "1/img/b.png",
			"[сайт](https://en.cx) ![]({{logo.png}}) [я](#top) ![](img/b.png#x)"},
		{"несуществующая", "1/notes.md", "![](img/none.png)", "1/img/pic.png", "1/img/b.png", "![](img/none.png)"},
	}
	for _, c := range cases {
		if got := run(c.md, c.text, c.from, c.to); got != c.want {
			t.Errorf("%s:\n got  %q\n want %q", c.name, got, c.want)
		}
	}
}

func TestRenameUpdatesLinks(t *testing.T) {
	e := newEnv(t)
	gd := filepath.Dir(e.game)
	_ = os.MkdirAll(filepath.Join(gd, "1", "img"), 0o755)
	_ = os.WriteFile(filepath.Join(gd, "1", "img", "pic.png"), []byte("PNG"), 0o644)
	_ = os.WriteFile(filepath.Join(gd, "1", "notes.md"), []byte("![](img/pic.png)\n![[pic.png]]\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(gd, "notes"), 0o755)
	_ = os.WriteFile(filepath.Join(gd, "notes", "общее.md"), []byte("![](../1/img/pic.png)\n"), 0o644)
	code, body := e.do(t, "POST", "/api/ui/files/rename", map[string]any{"scope": "level", "n": 1, "path": "img/pic.png", "to": "img/схема.png"}, false)
	if code != 200 || !strings.Contains(body, "1/notes.md") || !strings.Contains(body, "notes/общее.md") {
		t.Fatalf("rename: %d %s", code, body)
	}
	if b, _ := os.ReadFile(filepath.Join(gd, "1", "notes.md")); string(b) != "![](img/схема.png)\n![[схема.png]]\n" {
		t.Fatalf("level note: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(gd, "notes", "общее.md")); string(b) != "![](../1/img/схема.png)\n" {
		t.Fatalf("game note: %q", b)
	}
}

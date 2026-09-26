package emu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRewriterContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "design.css"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "~logo.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rw := NewRewriter(dir, map[string]string{"design.css": "abc.css", "logo.png": "logo.png"})

	in := `<style>@import url("{{design.css}}?v=2");</style>` +
		`<link rel="stylesheet" href="x.css">` +
		`<img src="https://d1.endata.cx/data/games/82460/abc.css">` +
		`<img src="https://cdn.endata.cx/data/games/82460/logo.png?v=3">` +
		` {{missing.png}} <LINK href=y>tail`
	got := rw.Content(in)

	for _, want := range []string{`/assets/design.css?v=2`, `src="/assets/design.css"`, `src="/assets/logo.png?v=3"`, `{{missing.png}}`, `tail`} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in %q", want, got)
		}
	}
	if strings.Contains(strings.ToLower(got), "<link") {
		t.Errorf("<link> must be stripped: %q", got)
	}
	if m := rw.Missing(); len(m) != 1 || m[0] != "missing.png" {
		t.Errorf("missing = %v", m)
	}

	if name, ok := rw.DiskName("logo.png"); !ok || name != "~logo.png" {
		t.Errorf("DiskName(logo.png) = %q %v", name, ok)
	}
	if rw.Exists("../design.css") || rw.Exists(".manifest.json") || rw.Exists("nope.css") {
		t.Errorf("traversal / hidden / missing must not exist")
	}
}

func TestStripLinksKeepsImportAndComments(t *testing.T) {
	in := "<style>@import url(\"a.css\");</style>\n<link\n rel=\"stylesheet\" href=\"b.css\">\n<!-- движок вырезает голый <link>\n из тела --><p>ok</p>"
	got := StripLinks(in)
	if strings.Contains(got, "<link\n rel") || !strings.Contains(got, "@import") || !strings.Contains(got, "<p>ok</p>") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "голый <link>\n из тела") {
		t.Fatalf("text <link> inside a comment must stay: %q", got)
	}
}

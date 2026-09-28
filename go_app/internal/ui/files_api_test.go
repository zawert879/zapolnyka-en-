package ui

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (e *env) upload(t *testing.T, fields map[string]string, files map[string]string) (int, string) {
	t.Helper()
	var mp bytes.Buffer
	mw := multipart.NewWriter(&mp)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	for name, data := range files {
		fw, _ := mw.CreateFormFile("files", name)
		_, _ = fw.Write([]byte(data))
	}
	_ = mw.Close()
	req, _ := http.NewRequest("POST", e.ts.URL+"/api/ui/files/upload", &mp)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestNotesRaw(t *testing.T) {
	e := newEnv(t)
	notes := filepath.Join(filepath.Dir(e.game), "1", NotesFile)
	_, body := e.do(t, "GET", "/api/ui/level/1", nil, false)
	var d LevelData
	_ = json.Unmarshal([]byte(body), &d)
	if v, ok := d.Raw["notes"]; !ok || v != "" || !strings.HasSuffix(d.Files.Notes, "/1/notes.md") {
		t.Fatalf("notes before: %q %q", d.Raw["notes"], d.Files.Notes)
	}
	// Пустой текст файл не создаёт.
	if code, _ := e.do(t, "PUT", "/api/ui/level/1/raw/notes", "", true); code != 200 {
		t.Fatal("empty notes")
	}
	if _, err := os.Stat(notes); err == nil {
		t.Fatal("empty notes must not create file")
	}
	if code, body := e.do(t, "PUT", "/api/ui/level/1/raw/notes", "# Идея\n", true); code != 200 {
		t.Fatalf("save notes: %s", body)
	}
	if b, _ := os.ReadFile(notes); string(b) != "# Идея\n" {
		t.Fatalf("notes file: %q", b)
	}
}

func TestFilesAPI(t *testing.T) {
	e := newEnv(t)
	gd := filepath.Dir(e.game)

	// Дерево уровня: стандартные файлы с ролями.
	code, body := e.do(t, "GET", "/api/ui/files?scope=level&n=1", nil, false)
	var tree struct {
		Rel     string      `json:"rel"`
		Entries []FileEntry `json:"entries"`
	}
	_ = json.Unmarshal([]byte(body), &tree)
	roles := map[string]string{}
	for _, f := range tree.Entries {
		roles[f.Path] = f.Role
	}
	if code != 200 || tree.Rel != "1" || roles["conf.yml"] != "conf" || roles["task.html"] != "body" || roles["codes.yml"] != "codes" {
		t.Fatalf("level tree: %d %s", code, body)
	}
	// Глобальное хранилище ещё не создано — пусто.
	if code, body := e.do(t, "GET", "/api/ui/files?scope=game", nil, false); code != 200 || !strings.Contains(body, `"entries":[]`) {
		t.Fatalf("game tree: %d %s", code, body)
	}

	// Запись текста (с созданием папок), чтение, автосоздание notes/.
	if code, body := e.do(t, "PUT", "/api/ui/file?scope=game&path=idea/plan.md", "план", true); code != 200 {
		t.Fatalf("put: %s", body)
	}
	if b, _ := os.ReadFile(filepath.Join(gd, "notes", "idea", "plan.md")); string(b) != "план" {
		t.Fatalf("file on disk: %q", b)
	}
	if code, body := e.do(t, "GET", "/api/ui/file?scope=game&path=idea/plan.md", nil, false); code != 200 || body != "план" {
		t.Fatalf("get: %d %q", code, body)
	}
	// Стандартные файлы через обозреватель не правятся и не удаляются.
	if code, _ := e.do(t, "PUT", "/api/ui/file?scope=level&n=1&path=conf.yml", "x", true); code != 400 {
		t.Fatal("conf must be protected from put")
	}
	if code, _ := e.do(t, "DELETE", "/api/ui/file?scope=level&n=1&path=task.html", nil, false); code != 400 {
		t.Fatal("body must be protected from delete")
	}
	// Выход из корня и скрытые файлы.
	for _, bad := range []string{"../2/task.html", "a/../../x", ".hidden", "sub/.git/x", "x.tmp"} {
		if code, _ := e.do(t, "PUT", "/api/ui/file?scope=level&n=1&path="+bad, "x", true); code != 400 {
			t.Fatalf("bad path %q accepted", bad)
		}
	}
	// Симлинк наружу.
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(gd, "1", "out")); err == nil {
		if code, _ := e.do(t, "PUT", "/api/ui/file?scope=level&n=1&path=out/x.txt", "x", true); code != 400 {
			t.Fatal("symlink escape accepted")
		}
		if _, err := os.Stat(filepath.Join(outside, "x.txt")); err == nil {
			t.Fatal("file written outside")
		}
	}

	// Загрузка: одинаковое имя не перезаписывается.
	if code, body := e.upload(t, map[string]string{"scope": "level", "n": "1", "dir": "img"}, map[string]string{"pic.png": "PNG1"}); code != 200 || !strings.Contains(body, `"img/pic.png"`) {
		t.Fatalf("upload: %d %s", code, body)
	}
	if code, body := e.upload(t, map[string]string{"scope": "level", "n": "1", "dir": "img"}, map[string]string{"pic.png": "PNG2"}); code != 200 || !strings.Contains(body, `"img/pic 1.png"`) {
		t.Fatalf("upload 2: %d %s", code, body)
	}
	if b, _ := os.ReadFile(filepath.Join(gd, "1", "img", "pic.png")); string(b) != "PNG1" {
		t.Fatal("upload overwrote file")
	}
	if code, _ := e.upload(t, map[string]string{"scope": "level", "n": "1", "dir": "../2"}, map[string]string{"a.png": "x"}); code != 400 {
		t.Fatal("upload outside accepted")
	}
	// Бинарный файл в редакторе не открывается.
	_ = os.WriteFile(filepath.Join(gd, "1", "img", "bin.dat"), []byte{0, 1, 2}, 0o644)
	if code, _ := e.do(t, "GET", "/api/ui/file?scope=level&n=1&path=img/bin.dat", nil, false); code != 415 {
		t.Fatal("binary must be 415")
	}

	// /ui/fs — относительно папки игры.
	if code, body := e.do(t, "GET", "/ui/fs/1/img/pic.png", nil, false); code != 200 || body != "PNG1" {
		t.Fatalf("fs: %d %q", code, body)
	}
	if code, _ := e.do(t, "GET", "/ui/fs/notes/idea/plan.md", nil, false); code != 200 {
		t.Fatal("fs notes")
	}
	for _, bad := range []string{"/ui/fs/..%2F..%2Fetc%2Fpasswd", "/ui/fs/assets/.manifest.json"} {
		if code, _ := e.do(t, "GET", bad, nil, false); code != 404 {
			t.Fatalf("fs %s must be 404", bad)
		}
	}

	// mkdir, rename, в ассеты, удаление.
	if code, _ := e.do(t, "POST", "/api/ui/files/mkdir", map[string]any{"scope": "game", "path": "refs"}, false); code != 200 {
		t.Fatal("mkdir")
	}
	if code, _ := e.do(t, "POST", "/api/ui/files/mkdir", map[string]any{"scope": "game", "path": "refs"}, false); code != 409 {
		t.Fatal("mkdir existing must conflict")
	}
	if code, body := e.do(t, "POST", "/api/ui/files/rename", map[string]any{"scope": "level", "n": 1, "path": "img/pic 1.png", "to": "img/second.png"}, false); code != 200 {
		t.Fatalf("rename: %s", body)
	}
	if code, _ := e.do(t, "POST", "/api/ui/files/rename", map[string]any{"scope": "level", "n": 1, "path": "codes.yml", "to": "c.yml"}, false); code != 400 {
		t.Fatal("rename protected")
	}
	if code, body := e.do(t, "POST", "/api/ui/files/to-assets", map[string]any{"scope": "level", "n": 1, "path": "img/second.png"}, false); code != 200 || !strings.Contains(body, `"second.png"`) {
		t.Fatalf("to-assets: %s", body)
	}
	if b, _ := os.ReadFile(filepath.Join(gd, "assets", "second.png")); string(b) != "PNG2" {
		t.Fatal("asset not copied")
	}
	if code, _ := e.do(t, "POST", "/api/ui/files/to-assets", map[string]any{"scope": "level", "n": 1, "path": "img/second.png"}, false); code != 409 {
		t.Fatal("to-assets must not overwrite")
	}
	if code, _ := e.do(t, "DELETE", "/api/ui/file?scope=level&n=1&path=img", nil, false); code != 200 {
		t.Fatal("delete dir")
	}
	if _, err := os.Stat(filepath.Join(gd, "1", "img")); err == nil {
		t.Fatal("dir not removed")
	}
}

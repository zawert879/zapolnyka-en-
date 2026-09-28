package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"zapolnyaka/internal/assets"
)

// ---------------------------------------------------------------- файловый обозреватель
//
// Два корня (scope):
//   level — папка уровня n (где лежит его conf);
//   game  — глобальное хранилище игры: <папка игры>/notes (создаётся при первой записи).
// Ни то ни другое на en.cx не заливается: для задания нужен ассет («в ассеты»).
// Пути в запросах — относительные к корню, через «/».

// GameNotesDir — глобальное хранилище заметок и файлов автора рядом с game.yml.
const GameNotesDir = "notes"

const (
	maxTextFile = 4 << 20 // больше — не открываем в редакторе
	maxTreeSize = 5000    // защита от огромных папок
)

// FileEntry — файл или папка в обозревателе.
type FileEntry struct {
	Path  string `json:"path"` // относительно корня
	Dir   bool   `json:"dir,omitempty"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
	Kind  string `json:"kind,omitempty"` // text | image | other
	Role  string `json:"role,omitempty"` // conf | codes | body | notes — стандартные файлы уровня
}

type fileRoot struct {
	dir       string            // абсолютный путь корня
	rel       string            // корень относительно папки игры («1-sectors», «notes»)
	roles     map[string]string // абсолютный путь → роль (только level)
	protected map[string]bool   // абсолютные пути conf/codes/body: не удалять и не править через обозреватель
}

var imageExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true, ".bmp": true, ".ico": true, ".avif": true}
var textExts = map[string]bool{".md": true, ".markdown": true, ".txt": true, ".html": true, ".htm": true, ".css": true, ".js": true, ".json": true, ".yml": true, ".yaml": true, ".xml": true, ".csv": true, ".tsv": true, ".log": true}

// fileKind определяет вид файла по расширению, неизвестные — по содержимому.
func fileKind(p string) string {
	ext := strings.ToLower(filepath.Ext(p))
	if imageExts[ext] {
		return "image"
	}
	if textExts[ext] {
		return "text"
	}
	f, err := os.Open(p)
	if err != nil {
		return "other"
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := io.ReadFull(f, buf)
	if looksText(buf[:n]) {
		return "text"
	}
	return "other"
}

func looksText(b []byte) bool {
	for _, c := range b {
		if c == 0 {
			return false
		}
	}
	// обрезанный на границе многобайтовый символ не повод считать файл бинарным
	for i := 0; i < 4 && len(b) > 0 && !utf8.Valid(b); i++ {
		b = b[:len(b)-1]
	}
	return utf8.Valid(b)
}

// cleanRel нормализует относительный путь и отклоняет выход из корня и скрытые сегменты.
// Пустой путь (или «/») — сам корень.
func cleanRel(rel string) (string, error) {
	rel = strings.ReplaceAll(rel, `\`, "/")
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return "", fmt.Errorf("плохой путь %q", rel)
		}
	}
	c := strings.TrimPrefix(path.Clean("/"+rel), "/")
	if c == "" {
		return "", nil
	}
	for _, seg := range strings.Split(c, "/") {
		if strings.HasPrefix(seg, ".") || strings.ContainsAny(seg, "{}") || strings.HasSuffix(seg, ".tmp") {
			return "", fmt.Errorf("плохой путь %q", rel)
		}
	}
	return c, nil
}

// realPath раскрывает симлинки у ближайшего существующего предка p и дописывает
// несуществующий хвост.
func realPath(p string) string {
	cur, tail := p, ""
	for {
		if r, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(r, tail)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return p
		}
		tail = filepath.Join(filepath.Base(cur), tail)
		cur = parent
	}
}

// realInside проверяет, что p после раскрытия симлинков лежит внутри root.
func realInside(root, p string) bool {
	rel, err := filepath.Rel(realPath(root), realPath(p))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// scopedPath — абсолютный путь rel внутри корня root.
func scopedPath(root, rel string) (string, string, error) {
	c, err := cleanRel(rel)
	if err != nil {
		return "", "", err
	}
	p := filepath.Join(root, filepath.FromSlash(c))
	if !realInside(root, p) {
		return "", "", fmt.Errorf("путь %q вне папки", rel)
	}
	return p, c, nil
}

// root возвращает корень по параметрам scope и n.
func (u *UI) root(scope, nStr string) (*fileRoot, error) {
	gamePath := u.deps.Emu.GamePath()
	if gamePath == "" {
		return nil, fmt.Errorf("игра не выбрана — создайте её на вкладке «Команды»")
	}
	gd, _ := filepath.Abs(gameDir(gamePath))
	switch scope {
	case "game":
		return &fileRoot{dir: filepath.Join(gd, GameNotesDir), rel: GameNotesDir}, nil
	case "level":
		n, err := strconv.Atoi(nStr)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("плохой номер уровня %q", nStr)
		}
		d, err := loadLevel(gamePath, n)
		if err != nil {
			return nil, err
		}
		dir, _ := filepath.Abs(filepath.FromSlash(d.Files.Dir))
		rel, _ := filepath.Rel(gd, dir)
		fr := &fileRoot{dir: dir, rel: filepath.ToSlash(rel), roles: map[string]string{}, protected: map[string]bool{}}
		if fr.rel == "." {
			fr.rel = ""
		}
		add := func(p, role string, protect bool) {
			if p == "" {
				return
			}
			abs, _ := filepath.Abs(filepath.FromSlash(p))
			fr.roles[abs] = role
			if protect {
				fr.protected[abs] = true
			}
		}
		body := d.Files.Body
		if body == "" {
			body = d.Files.Dir + "/task.html"
		}
		add(d.Files.Conf, "conf", true)
		add(d.Files.Codes, "codes", true)
		add(body, "body", true)
		add(d.Files.Notes, "notes", false)
		return fr, nil
	}
	return nil, fmt.Errorf("неизвестный scope %q (level | game)", scope)
}

// touchesProtected — p сам защищён или папка, содержащая защищённый файл.
func (fr *fileRoot) touchesProtected(p string) bool {
	for prot := range fr.protected {
		if prot == p {
			return true
		}
		if rel, err := filepath.Rel(p, prot); err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}
	}
	return false
}

func (fr *fileRoot) list() ([]FileEntry, error) {
	out := []FileEntry{}
	if _, err := os.Stat(fr.dir); errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	err := filepath.WalkDir(fr.dir, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p == fr.dir {
			return nil
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".tmp") {
			if e.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= maxTreeSize {
			return filepath.SkipAll
		}
		rel, _ := filepath.Rel(fr.dir, p)
		fe := FileEntry{Path: filepath.ToSlash(rel), Dir: e.IsDir()}
		if info, err := e.Info(); err == nil {
			fe.Mtime = info.ModTime().Unix()
			if !e.IsDir() {
				fe.Size = info.Size()
			}
		}
		if !e.IsDir() {
			fe.Kind = fileKind(p)
			fe.Role = fr.roles[p]
		}
		out = append(out, fe)
		return nil
	})
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, err
}

// uniquePath подбирает свободное имя: «a.png», «a 1.png», «a 2.png»…
func uniquePath(p string) string {
	if _, err := os.Lstat(p); errors.Is(err, fs.ErrNotExist) {
		return p
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for i := 1; ; i++ {
		c := fmt.Sprintf("%s %d%s", base, i, ext)
		if _, err := os.Lstat(c); errors.Is(err, fs.ErrNotExist) {
			return c
		}
	}
}

// ---------------------------------------------------------------- обработчики

func (u *UI) handleFiles(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	fr, err := u.root(q.Get("scope"), q.Get("n"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	entries, err := fr.list()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"root": filepath.ToSlash(fr.dir), "rel": fr.rel, "entries": entries})
}

// scopedFromQuery — корень и путь файла из ?scope=&n=&path=.
func (u *UI) scopedFromQuery(r *http.Request) (*fileRoot, string, string, error) {
	q := r.URL.Query()
	fr, err := u.root(q.Get("scope"), q.Get("n"))
	if err != nil {
		return nil, "", "", err
	}
	p, rel, err := scopedPath(fr.dir, q.Get("path"))
	if err != nil {
		return nil, "", "", err
	}
	if rel == "" {
		return nil, "", "", fmt.Errorf("нужен path")
	}
	return fr, p, rel, nil
}

func (u *UI) handleFileGet(w http.ResponseWriter, r *http.Request) {
	_, p, _, err := u.scopedFromQuery(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		writeErr(w, http.StatusNotFound, fmt.Errorf("файл не найден"))
		return
	}
	if fi.Size() > maxTextFile {
		writeErr(w, http.StatusRequestEntityTooLarge, fmt.Errorf("файл больше 4 МБ — в редакторе не открыть"))
		return
	}
	data, err := os.ReadFile(p)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !looksText(data) {
		writeErr(w, http.StatusUnsupportedMediaType, fmt.Errorf("не текстовый файл"))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func (u *UI) handleFilePut(w http.ResponseWriter, r *http.Request) {
	fr, p, _, err := u.scopedFromQuery(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if fr.protected[p] {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("%s — стандартный файл уровня: правь его на своей вкладке", filepath.Base(p)))
		return
	}
	if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("это папка"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := writeFileAtomic(p, body); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (u *UI) handleFileDelete(w http.ResponseWriter, r *http.Request) {
	fr, p, _, err := u.scopedFromQuery(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if fr.touchesProtected(p) {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("здесь стандартные файлы уровня (conf/codes/task) — не удаляю"))
		return
	}
	if _, err := os.Lstat(p); err != nil {
		writeErr(w, http.StatusNotFound, fmt.Errorf("файл не найден"))
		return
	}
	if err := os.RemoveAll(p); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type fileOpRequest struct {
	Scope string `json:"scope"`
	N     int    `json:"n"`
	Path  string `json:"path"`
	To    string `json:"to,omitempty"`
}

func (u *UI) decodeOp(r *http.Request) (*fileOpRequest, *fileRoot, string, error) {
	var req fileOpRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		return nil, nil, "", fmt.Errorf("bad json: %w", err)
	}
	fr, err := u.root(req.Scope, strconv.Itoa(req.N))
	if err != nil {
		return nil, nil, "", err
	}
	p, rel, err := scopedPath(fr.dir, req.Path)
	if err != nil {
		return nil, nil, "", err
	}
	if rel == "" {
		return nil, nil, "", fmt.Errorf("нужен path")
	}
	return &req, fr, p, nil
}

func (u *UI) handleMkdir(w http.ResponseWriter, r *http.Request) {
	_, _, p, err := u.decodeOp(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if _, err := os.Lstat(p); err == nil {
		writeErr(w, http.StatusConflict, fmt.Errorf("%s уже есть", filepath.Base(p)))
		return
	}
	if err := os.MkdirAll(p, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (u *UI) handleRename(w http.ResponseWriter, r *http.Request) {
	req, fr, from, err := u.decodeOp(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	to, toRel, err := scopedPath(fr.dir, req.To)
	if err != nil || toRel == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("плохое новое имя %q", req.To))
		return
	}
	if fr.touchesProtected(from) {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("стандартные файлы уровня не переименовываю: на них ссылается conf"))
		return
	}
	if _, err := os.Lstat(from); err != nil {
		writeErr(w, http.StatusNotFound, fmt.Errorf("файл не найден"))
		return
	}
	if _, err := os.Lstat(to); err == nil {
		writeErr(w, http.StatusConflict, fmt.Errorf("%s уже есть", toRel))
		return
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	gd, _ := filepath.Abs(gameDir(u.deps.Emu.GamePath()))
	before := gameFiles(gd)
	if err := os.Rename(from, to); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	// ссылки в заметках игры следуют за файлом
	fromG, _ := filepath.Rel(gd, from)
	toG, _ := filepath.Rel(gd, to)
	changed, err := updateLinks(gd, before, u.levelDirs(), filepath.ToSlash(fromG), filepath.ToSlash(toG))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Errorf("файл перенесён, но ссылки обновлены не везде: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": toRel, "updated": changed})
}

// levelDirs — папки уровней игры (от папки игры; "" — уровень в самой папке игры).
func (u *UI) levelDirs() []string {
	_, levels, err := listLevels(u.deps.Emu.GamePath())
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range levels {
		d := l.Dir
		if d == "." {
			d = ""
		}
		out = append(out, d)
	}
	return out
}

// handleFilesUpload — multipart: scope, n, dir (папка внутри корня), files (несколько).
// Существующие файлы не перезаписываются: новому даётся имя «a 1.png».
func (u *UI) handleFilesUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("multipart: %w", err))
		return
	}
	fr, err := u.root(r.FormValue("scope"), r.FormValue("n"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	dir, dirRel, err := scopedPath(fr.dir, r.FormValue("dir"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	saved := []string{}
	for _, fh := range r.MultipartForm.File["files"] {
		name, err := assetName(fh.Filename)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		if fh.Size > assets.MaxFileSize {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("файл %s больше лимита 48 МБ", name))
			return
		}
		src, err := fh.Open()
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		data, err := io.ReadAll(io.LimitReader(src, assets.MaxFileSize+1))
		_ = src.Close()
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		dst := uniquePath(filepath.Join(dir, name))
		if err := writeFileAtomic(dst, data); err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		saved = append(saved, path.Join(dirRel, filepath.Base(dst)))
	}
	if len(saved) == 0 {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("нет файлов (поле files)"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "saved": saved})
}

// handleToAssets копирует файл в папку ассетов игры, чтобы его можно было
// использовать в задании как {{имя}}.
func (u *UI) handleToAssets(w http.ResponseWriter, r *http.Request) {
	_, _, p, err := u.decodeOp(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	name, err := assetName(filepath.Base(p))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		writeErr(w, http.StatusNotFound, fmt.Errorf("файл не найден"))
		return
	}
	if fi.Size() > assets.MaxFileSize {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("файл больше лимита 48 МБ"))
		return
	}
	dir, err := u.assetsDir()
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	for _, c := range []string{name, "~" + name, strings.TrimPrefix(name, "~")} {
		if _, err := os.Stat(filepath.Join(dir, c)); err == nil {
			writeErr(w, http.StatusConflict, fmt.Errorf("ассет %s уже есть — переименуй файл или удали ассет", assets.RefName(name)))
			return
		}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if err := writeFileAtomic(filepath.Join(dir, name), data); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": assets.RefName(name)})
}

// handleFS отдаёт файл из папки игры по пути относительно неё: так md-превью
// показывает картинки по обычным относительным ссылкам.
func (u *UI) handleFS(w http.ResponseWriter, r *http.Request) {
	if u.deps.Emu.GamePath() == "" {
		http.NotFound(w, r)
		return
	}
	gd, _ := filepath.Abs(gameDir(u.deps.Emu.GamePath()))
	p, rel, err := scopedPath(gd, r.PathValue("path"))
	if err != nil || rel == "" {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(p)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.EqualFold(filepath.Ext(p), ".svg") || strings.EqualFold(filepath.Ext(p), ".html") || strings.EqualFold(filepath.Ext(p), ".htm") {
		// открытый напрямую файл не должен исполнять скрипты в origin интерфейса
		w.Header().Set("Content-Security-Policy", "sandbox")
	}
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

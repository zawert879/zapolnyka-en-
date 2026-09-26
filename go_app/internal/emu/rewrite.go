package emu

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"zapolnyaka/internal/assets"
)

// Rewriter переписывает контент уровня под локальный показ:
//   - {{имя}} → /assets/имя (файл из папки ассетов игры);
//   - https://d1|cdn.endata.cx/data/games/{gid}/<файл> → /assets/<локальный файл>
//     по обратному манифесту или по совпадению имени файла;
//   - голые теги <link …> вырезаются — так же поступает движок en.cx с телом задания
//     (CSS в теле подключают только через <style>@import …</style>).
type Rewriter struct {
	assetsDir string
	reverse   map[string]string // имя на сервере → локальное имя
	missing   map[string]bool
}

// NewRewriter создаёт переписыватель для папки ассетов и манифеста (может быть пустым).
func NewRewriter(assetsDir string, manifest map[string]string) *Rewriter {
	return &Rewriter{
		assetsDir: assetsDir,
		reverse:   assets.Reverse(manifest),
		missing:   map[string]bool{},
	}
}

// AssetURLPrefix — под каким путём эмулятор раздаёт локальные ассеты.
const AssetURLPrefix = "/assets/"

var (
	d1URLRe   = regexp.MustCompile(`https?://(?:d1|cdn)\.endata\.cx/data/games/\d+/([^"'\s<>?#)]+)`)
	linkRe    = regexp.MustCompile(`(?is)<link\s+[^>]*>`) // только настоящий тег с атрибутами; «<link>» как текст движок не трогает
	commentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// Exists сообщает, есть ли такой файл в папке ассетов (имя на диске или с ведущим ~).
func (r *Rewriter) Exists(name string) bool {
	if r.assetsDir == "" || name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return false
	}
	for _, disk := range []string{name, "~" + name} {
		if fi, err := os.Stat(filepath.Join(r.assetsDir, disk)); err == nil && !fi.IsDir() {
			return true
		}
	}
	return false
}

// DiskName возвращает имя файла на диске для имени-ссылки (учитывает префикс ~).
func (r *Rewriter) DiskName(name string) (string, bool) {
	if !r.Exists(name) {
		return "", false
	}
	if fi, err := os.Stat(filepath.Join(r.assetsDir, name)); err == nil && !fi.IsDir() {
		return name, true
	}
	return "~" + name, true
}

// Resolver — резолвер плейсхолдеров для эмулятора: известные файлы → локальный URL.
func (r *Rewriter) Resolver() assets.Resolver {
	return func(name string) (string, bool) {
		if r.Exists(name) {
			return AssetURLPrefix + name, true
		}
		return "", false
	}
}

// Missing — имена плейсхолдеров, для которых не нашлось файла (накапливается).
func (r *Rewriter) Missing() []string {
	var out []string
	for n := range r.missing {
		out = append(out, n)
	}
	return out
}

// Content переписывает фрагмент HTML/текста уровня.
func (r *Rewriter) Content(s string) string {
	if s == "" {
		return s
	}
	out, missing := assets.Expand(s, r.Resolver())
	for _, m := range missing {
		r.missing[m] = true
	}
	out = d1URLRe.ReplaceAllStringFunc(out, func(u string) string {
		upload := d1URLRe.FindStringSubmatch(u)[1]
		if local, ok := r.reverse[upload]; ok && r.Exists(local) {
			return AssetURLPrefix + local
		}
		if r.Exists(upload) {
			return AssetURLPrefix + upload
		}
		return u
	})
	return StripLinks(out)
}

// StripLinks вырезает голые теги <link …> (движок en.cx удаляет их из тела задания).
// <style>@import …</style> и содержимое HTML-комментариев не затрагиваются.
func StripLinks(s string) string {
	if !strings.Contains(strings.ToLower(s), "<link") {
		return s
	}
	var b strings.Builder
	last := 0
	for _, m := range commentRe.FindAllStringIndex(s, -1) {
		b.WriteString(linkRe.ReplaceAllString(s[last:m[0]], ""))
		b.WriteString(s[m[0]:m[1]])
		last = m[1]
	}
	b.WriteString(linkRe.ReplaceAllString(s[last:], ""))
	return b.String()
}

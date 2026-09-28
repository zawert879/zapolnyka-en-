package ui

import (
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ---------------------------------------------------------------- ссылки в заметках
//
// После переименования/переноса файла или папки ссылки на него в md-заметках игры
// переписываются, как в Obsidian. «Хранилище» — вся папка игры; пути — от неё, через «/».
// Разрешение ссылок повторяет фронт (app.js: mdResolve / findWiki):
//   [текст](путь), ![](путь) — от папки заметки («/путь» — от папки игры);
//   [[цель]], ![[цель]] — рядом с заметкой, от корня её уровня, в notes/, затем по
//   имени файла (сначала свой уровень, потом notes/, потом остальное); без
//   расширения — ещё и «цель.md».

var (
	mdLinkRe = regexp.MustCompile(`(!?\[[^\]\n]*\]\()(\s*)(<[^>\n]*>|[^\s()<>]+)`)
	wikiRe   = regexp.MustCompile(`(!?\[\[)([^\[\]\n|#]+)((?:#[^\[\]\n|]*)?(?:\|[^\[\]\n]*)?\]\])`)
	fenceRe  = regexp.MustCompile("^\\s*(```|~~~)")
	schemeRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
)

// linkLayout — файлы хранилища в одном из состояний (до или после переименования).
type linkLayout struct {
	files     map[string]bool
	sorted    []string
	levelDirs []string // папки уровней (от папки игры), длинные — первыми
}

func newLayout(files []string, levelDirs []string) *linkLayout {
	l := &linkLayout{files: map[string]bool{}, levelDirs: levelDirs}
	for _, f := range files {
		l.files[f] = true
	}
	l.sorted = append([]string{}, files...)
	sort.Strings(l.sorted)
	return l
}

// levelOf — корень уровня, в котором лежит файл p ("" — вне уровней).
func (l *linkLayout) levelOf(p string) (string, bool) {
	for _, d := range l.levelDirs {
		if d == "" || p == d || strings.HasPrefix(p, d+"/") {
			return d, true
		}
	}
	return "", false
}

func joinRel(parts ...string) string {
	j := path.Join(parts...)
	if j == "." || j == "/" {
		return ""
	}
	return strings.TrimPrefix(j, "/")
}

// resolveWiki — цель вики-ссылки из заметки md.
func (l *linkLayout) resolveWiki(md, target string) string {
	find := func(t string) string {
		t = strings.TrimLeft(t, "/")
		cands := []string{joinRel(path.Dir(md), t)}
		if lv, ok := l.levelOf(md); ok {
			cands = append(cands, joinRel(lv, t))
		}
		cands = append(cands, joinRel(GameNotesDir, t))
		for _, c := range cands {
			if l.files[c] {
				return c
			}
		}
		base := path.Base(t)
		lv, inLevel := l.levelOf(md)
		var best string
		rank := 9
		for _, f := range l.sorted {
			if path.Base(f) != base {
				continue
			}
			r := 3
			if inLevel && (lv == "" || strings.HasPrefix(f, lv+"/")) {
				r = 1
			} else if strings.HasPrefix(f, GameNotesDir+"/") {
				r = 2
			}
			if r < rank {
				best, rank = f, r
			}
		}
		return best
	}
	if f := find(target); f != "" {
		return f
	}
	if path.Ext(target) == "" {
		return find(target + ".md")
	}
	return ""
}

// resolveURL — цель обычной md-ссылки (url уже без <> и раскодирован).
func resolveURL(md, u string) string {
	if strings.HasPrefix(u, "/") {
		return joinRel(u)
	}
	return joinRel(path.Dir(md), u)
}

// relFrom — путь от папки fromDir до to (как relLink во фронте).
func relFrom(fromDir, to string) string {
	var a []string
	if fromDir != "" && fromDir != "." {
		a = strings.Split(fromDir, "/")
	}
	b := strings.Split(to, "/")
	i := 0
	for i < len(a) && i < len(b)-1 && a[i] == b[i] {
		i++
	}
	return strings.Repeat("../", len(a)-i) + strings.Join(b[i:], "/")
}

func encodeMDURL(p string) string {
	return strings.NewReplacer(" ", "%20", "(", "%28", ")", "%29").Replace(p)
}

// mapPath — новое место файла после переноса from → to (папки — со всем содержимым).
func mapPath(p, from, to string) string {
	if p == from {
		return to
	}
	if strings.HasPrefix(p, from+"/") {
		return to + p[len(from):]
	}
	return p
}

// rewriteNote переписывает ссылки одной заметки. oldMD/newMD — её путь до и после.
func rewriteNote(text, oldMD, newMD string, before, after *linkLayout, from, to string) string {
	lines := strings.SplitAfter(text, "\n")
	inFence := false
	for i, line := range lines {
		if fenceRe.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		lines[i] = rewriteLine(line, oldMD, newMD, before, after, from, to)
	}
	return strings.Join(lines, "")
}

// rewriteLine — ссылки одной строки (вне `кода`).
func rewriteLine(line, oldMD, newMD string, before, after *linkLayout, from, to string) string {
	// участки инлайн-кода не трогаем
	var out strings.Builder
	for len(line) > 0 {
		i := strings.IndexByte(line, '`')
		if i < 0 {
			out.WriteString(rewriteSegment(line, oldMD, newMD, before, after, from, to))
			break
		}
		out.WriteString(rewriteSegment(line[:i], oldMD, newMD, before, after, from, to))
		n := 0
		for i+n < len(line) && line[i+n] == '`' {
			n++
		}
		ticks := line[i : i+n]
		rest := line[i+n:]
		j := strings.Index(rest, ticks)
		if j < 0 {
			out.WriteString(line[i:])
			break
		}
		out.WriteString(line[i : i+n+j+n])
		line = rest[j+n:]
	}
	return out.String()
}

func rewriteSegment(s, oldMD, newMD string, before, after *linkLayout, from, to string) string {
	s = mdLinkRe.ReplaceAllStringFunc(s, func(m string) string {
		sm := mdLinkRe.FindStringSubmatch(m)
		prefix, space, raw := sm[1], sm[2], sm[3]
		angle := strings.HasPrefix(raw, "<")
		u := strings.TrimSuffix(strings.TrimPrefix(raw, "<"), ">")
		if u == "" || strings.HasPrefix(u, "#") || strings.Contains(u, "{{") || schemeRe.MatchString(u) || strings.HasPrefix(u, "//") {
			return m
		}
		frag := ""
		if k := strings.IndexByte(u, '#'); k >= 0 {
			u, frag = u[:k], u[k:]
		}
		dec := u
		if !angle {
			if d, err := url.PathUnescape(u); err == nil {
				dec = d
			}
		}
		oldT := resolveURL(oldMD, dec)
		if !before.files[oldT] {
			return m
		}
		newT := mapPath(oldT, from, to)
		if resolveURL(newMD, dec) == newT {
			return m
		}
		np := relFrom(path.Dir(newMD), newT)
		if strings.HasPrefix(dec, "/") {
			np = "/" + newT
		}
		if angle {
			return prefix + space + "<" + np + frag + ">"
		}
		return prefix + space + encodeMDURL(np) + frag
	})
	return wikiRe.ReplaceAllStringFunc(s, func(m string) string {
		sm := wikiRe.FindStringSubmatch(m)
		open, target, tail := sm[1], sm[2], sm[3]
		if strings.HasSuffix(target, `\`) { // [[a.png\|60]] — экранированный «|» внутри таблицы
			target, tail = strings.TrimSuffix(target, `\`), `\`+tail
		}
		t := strings.TrimSpace(target)
		oldT := before.resolveWiki(oldMD, t)
		if oldT == "" {
			return m
		}
		newT := mapPath(oldT, from, to)
		if after.resolveWiki(newMD, t) == newT {
			return m
		}
		noExt := path.Ext(t) == "" && strings.HasSuffix(newT, ".md")
		trim := func(p string) string {
			if noExt {
				return strings.TrimSuffix(p, ".md")
			}
			return p
		}
		// Короче всего — имя файла, если оно однозначно приводит к цели; иначе путь от заметки.
		for _, c := range []string{trim(path.Base(newT)), trim(relFrom(path.Dir(newMD), newT))} {
			if after.resolveWiki(newMD, c) == newT {
				return open + c + tail
			}
		}
		return m
	})
}

// updateLinks переписывает md-заметки игры после переноса fromRel → toRel (пути от
// папки игры; перенос уже выполнен). filesBefore — файлы игры до переноса.
// Возвращает пути (от папки игры, после переноса) изменённых заметок.
func updateLinks(gd string, filesBefore []string, levelDirs []string, fromRel, toRel string) ([]string, error) {
	sort.Slice(levelDirs, func(i, j int) bool { return len(levelDirs[i]) > len(levelDirs[j]) })
	var filesAfter []string
	for _, f := range filesBefore {
		filesAfter = append(filesAfter, mapPath(f, fromRel, toRel))
	}
	before, after := newLayout(filesBefore, levelDirs), newLayout(filesAfter, levelDirs)
	changed := []string{}
	for _, oldMD := range filesBefore {
		if !strings.EqualFold(path.Ext(oldMD), ".md") {
			continue
		}
		newMD := mapPath(oldMD, fromRel, toRel)
		p := filepath.Join(gd, filepath.FromSlash(newMD))
		data, err := os.ReadFile(p)
		if err != nil || len(data) > maxTextFile {
			continue
		}
		text := string(data)
		upd := rewriteNote(text, oldMD, newMD, before, after, fromRel, toRel)
		if upd == text {
			continue
		}
		if err := writeFileAtomic(p, []byte(upd)); err != nil {
			return changed, err
		}
		changed = append(changed, newMD)
	}
	return changed, nil
}

// gameFiles — все файлы папки игры (от неё, через «/»), без скрытых.
func gameFiles(gd string) []string {
	var out []string
	_ = filepath.WalkDir(gd, func(p string, e fs.DirEntry, err error) error {
		if err != nil || p == gd {
			return nil
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".tmp") || name == "node_modules" {
			if e.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if len(out) >= 4*maxTreeSize {
			return filepath.SkipAll
		}
		if !e.IsDir() {
			rel, _ := filepath.Rel(gd, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

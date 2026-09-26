package emu

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"

	"zapolnyaka/internal/config"
)

// PreviewOptions — что показывать в превью контента уровня (без хрома движка).
type PreviewOptions struct {
	Task      bool // тело задания
	Sectors   bool // список секторов
	Hints     bool // все подсказки как показанные
	Penalties bool // штрафные подсказки как открытые
	Bonuses   bool // бонусы как выполненные (с help)
	Fog       bool // оставить <script> из тела (туман); иначе скрипты вырезаются
}

// DefaultPreview — всё включено, кроме скриптов.
func DefaultPreview() PreviewOptions {
	return PreviewOptions{Task: true, Sectors: true, Hints: true, Penalties: true, Bonuses: true}
}

// PreviewCheck — результат проверки контента уровня для панели превью.
type PreviewCheck struct {
	Level    string `json:"level"` // "ok" | "warn" | "error"
	Text     string `json:"text"`
}

var scriptTagRe = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)

// Preview рендерит контент уровня n в дизайне игры: те же блоки, что на play-странице,
// но без шапки/формы ответа, с движковым CSS и синтетическим состоянием
// («всё открыто»). Состояние симуляции не трогается.
func (s *Server) Preview(n int, o PreviewOptions) ([]byte, []PreviewCheck, error) {
	l, err := s.load()
	if err != nil {
		return nil, nil, err
	}
	p, ok := l.game.Level(n)
	if !ok {
		return nil, nil, fmt.Errorf("уровень %d не найден в конфиге", n)
	}
	now := s.opts.Now()
	gs := &GameState{Levels: map[int]*LevelState{}, Bonuses: map[string]Entry{}}
	ls := gs.Level(n, now)
	// Время — за последнюю подсказку, чтобы всё показалось; автопереход не считаем.
	maxT := 0
	for _, h := range p.Conf.Hints {
		if h.Time > maxT {
			maxT = h.Time
		}
	}
	for _, h := range p.Conf.PenaltyHints {
		if h.Time > maxT {
			maxT = h.Time
		}
	}
	if o.Hints || o.Penalties {
		ls.StartedAt = now.Add(-time.Duration(maxT+1) * time.Second)
	}
	if p.Conf.Autopass != nil && *p.Conf.Autopass > 0 && maxT+1 >= *p.Conf.Autopass {
		ls.StartedAt = now.Add(-time.Duration(*p.Conf.Autopass-1) * time.Second)
	}
	if o.Penalties {
		for i := range p.Conf.PenaltyHints {
			ls.OpenedPenalty[i] = 2
		}
	}
	if o.Bonuses {
		for _, b := range config.BonusesForLevel(l.game.Prepared, n) {
			ans := ""
			if len(b.Code.Answers) > 0 {
				ans = b.Code.Answers[0]
			}
			gs.Bonuses[BonusKey(b.OwnerLevel, b.Index)] = Entry{Answer: ans, Login: s.env.Login, At: now, Level: n}
		}
	}
	v, err := BuildView(l.game, n, gs, now, l.rw, s.env, Request{})
	if err != nil {
		return nil, nil, err
	}
	if !o.Hints {
		v.HelpsHTML = ""
	}
	if !o.Penalties {
		v.PenaltiesHTML = ""
	}
	if !o.Bonuses {
		v.BonusesHTML = "\r\n"
	}
	if !o.Sectors {
		v.SectorsHTML = ""
	}
	if !o.Task {
		v.TaskBlockHTML = ""
	} else if !o.Fog {
		v.TaskBlockHTML = template.HTML(scriptTagRe.ReplaceAllString(string(v.TaskBlockHTML), ""))
	}

	var b bytes.Buffer
	b.WriteString("<!DOCTYPE html>\n<html lang=\"ru\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>Превью уровня " + fmt.Sprint(n) + "</title>\n")
	b.WriteString(`<link href="` + v.EngineBase + `/css/v2/en/engines/engine.css?ver=` + EngineVer + `" type="text/css" rel="stylesheet" media="screen,projection"  />` + "\n")
	b.WriteString(`<link href="` + v.EngineBase + `/css/v2/en/engines/real.css?ver=` + EngineVer + `" type="text/css" rel="stylesheet" media="screen,projection"  />` + "\n")
	b.WriteString(`<link href="` + v.EngineBase + `/css/v2/en/engines/engine_adaptive.css?ver=` + EngineVer + `" type="text/css" rel="stylesheet" media="screen,projection"  />` + "\n")
	b.WriteString(`<script type="text/javascript" src="` + v.EngineBase + `/js/v2/jquery/jquery-1.6.2.js?ver=` + EngineVer + `"></script>` + "\n")
	b.WriteString(`<script type="text/javascript" src="` + v.EngineBase + `/js/v2/Timer.js?ver=` + EngineVer + `"></script>` + "\n")
	b.WriteString(`<script type="text/javascript" src="` + v.EngineBase + `/js/v2/common.js?ver=` + EngineVer + `"></script>` + "\n")
	b.WriteString("<style>body{min-width:0}.content{margin-left:0;padding:0 12px}.container{padding:0}</style>\n</head>\n<body>\n<div class=\"container\">\n<div class=\"content\">\r\n\t\t\r\n\r\n")
	b.WriteString("\t<h2>Уровень <span>" + fmt.Sprint(v.Number) + "</span> из " + fmt.Sprint(v.LevelsTotal))
	if v.LevelName != "" {
		b.WriteString(": " + string(v.LevelNameHTML))
	}
	b.WriteString("</h2>\r\n\r\n\r\n\r\n")
	b.WriteString(string(v.TimerHTML) + string(v.SectorsHTML) + "\r\n" + string(v.TaskBlockHTML))
	b.WriteString("<div id=\"ordinary_helps\">\t\r\n" + string(v.HelpsHTML) + "</div>\r\n\t\r\n\t \r\n<div id=\"penalty_helps\">\r\n" + string(v.PenaltiesHTML) + "</div>\r\n<br>\r\n\r\n<div id=\"bonuses\">\r\n" + string(v.BonusesHTML) + "</div>\r\n</div>\r\n</div>\n</body>\n</html>")

	return b.Bytes(), s.previewChecks(l, n, v), nil
}

// previewChecks — проверки контента: плейсхолдеры, <link>, спойлеры, размер.
func (s *Server) previewChecks(l *loaded, n int, v *View) []PreviewCheck {
	p, _ := l.game.Level(n)
	var out []PreviewCheck
	body := p.Body
	if len(v.Missing) > 0 {
		out = append(out, PreviewCheck{"error", "Нет файлов для плейсхолдеров: " + strings.Join(v.Missing, ", ")})
	} else if strings.Contains(body, "{{") {
		out = append(out, PreviewCheck{"ok", "Плейсхолдеры ассетов найдены, файлы есть в папке ассетов"})
	}
	if linkRe.MatchString(commentRe.ReplaceAllString(body, "")) {
		out = append(out, PreviewCheck{"warn", "В теле есть голый <link …> — движок его вырежет, подключай CSS через <style>@import url(…)</style>"})
	} else if strings.Contains(body, "@import") {
		out = append(out, PreviewCheck{"ok", "CSS подключён через @import"})
	}
	if strings.Contains(strings.ToLower(body), "<script") {
		out = append(out, PreviewCheck{"ok", "В теле есть <script> — в превью скрипты выключены, в эмуляторе и игре выполняются"})
	}
	plain := strings.ToLower(stripTags(scriptTagRe.ReplaceAllString(commentRe.ReplaceAllString(body, ""), "")))
	var spoilers []string
	for _, c := range p.Codes {
		for _, a := range c.Answers {
			a = strings.TrimSpace(strings.ToLower(a))
			if len([]rune(a)) < 3 {
				continue
			}
			if strings.Contains(plain, a) {
				spoilers = append(spoilers, a)
				break
			}
		}
	}
	if len(spoilers) > 0 {
		out = append(out, PreviewCheck{"warn", "Ответы встречаются в тексте задания: " + strings.Join(spoilers, ", ")})
	} else if len(p.Codes) > 0 {
		out = append(out, PreviewCheck{"ok", "Ответы секторов и бонусов в тексте задания не встречаются"})
	}
	for _, c := range p.Codes {
		name := deref(c.SectorName) + " " + deref(c.BonusName)
		for _, a := range c.Answers {
			if a != "" && strings.Contains(strings.ToLower(name), strings.ToLower(a)) {
				out = append(out, PreviewCheck{"warn", "Ответ «" + a + "» совпадает с именем сектора/бонуса — имя видно игроку до ввода"})
			}
		}
	}
	out = append(out, PreviewCheck{"ok", fmt.Sprintf("Тело %d байт, секторов %d, бонусов %d, подсказок %d+%d", len(body), len(v.Sectors), len(v.Bonuses), len(v.Hints), len(v.Penalties))})
	return out
}

package emu

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"unicode"
)

// Разметка блоков play-страницы движка en.cx (v1.88), снята побайтово с реальной
// игры (tech.en.cx, 2026-09-26). Каждый блок собирается строкой с теми же табами,
// пробелами и переводами строк, что отдаёт сервер. Менять только по новым снимкам.

// EngineVer — версия движка в ?ver= у CSS/JS.
const EngineVer = "1.88.0.0"

// PlayPath — канонический путь play-страницы игры: /gameengines/encounter/play/{gid}/.
func PlayPath(gid int) string { return fmt.Sprintf("/gameengines/encounter/play/%d/", gid) }

// randRnd — параметр ?rnd= у ссылки «Обновить» (вида 0,84232806546722).
func randRnd() string { return "0," + strconv.FormatInt(10000000000000+rand.Int63n(90000000000000), 10) }

// randTimerID — id таймера вида time8895328 (7–8 цифр).
func randTimerID() string { return "time" + strconv.Itoa(1000000+rand.Intn(99000000)) }

// HTMLEncode — как HttpUtility.HtmlEncode в .NET: <, >, &, ", ' и символы U+00A0…U+00FF
// в числовые сущности (например « → &#171;); кириллица не кодируется.
func HTMLEncode(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '<':
			b.WriteString("&lt;")
		case r == '>':
			b.WriteString("&gt;")
		case r == '&':
			b.WriteString("&amp;")
		case r == '"':
			b.WriteString("&quot;")
		case r == '\'':
			b.WriteString("&#39;")
		case r >= 0xA0 && r <= 0xFF:
			b.WriteString("&#" + strconv.Itoa(int(r)) + ";")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// wrapLetters оборачивает каждую последовательность букв в <i>…</i> — так движок
// показывает неверный ответ в истории («<i>точка</i>3», «<i>неверный</i> <i>код</i>»).
func wrapLetters(s string) string {
	var b strings.Builder
	var run []rune
	flush := func() {
		if len(run) > 0 {
			b.WriteString("<i>" + HTMLEncode(string(run)) + "</i>")
			run = run[:0]
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) {
			run = append(run, r)
			continue
		}
		flush()
		b.WriteString(HTMLEncode(string(r)))
	}
	flush()
	return b.String()
}

// plural выбирает форму по числу: forms = [1, 2–4, 5–20].
func plural(n int, one, few, many string) string {
	n = abs(n)
	if n%100 >= 11 && n%100 <= 19 {
		return many
	}
	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	}
	return many
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ruDuration — длительность словами в винительном падеже, как у движка:
// «1 минуту», «2 минуты 30 секунд», «5 минут», «30 секунд». Нулевые части
// опускаются; ноль целиком — «0 секунд».
func ruDuration(sec int) string {
	sec = abs(sec)
	d, h, m, s := sec/86400, (sec%86400)/3600, (sec%3600)/60, sec%60
	var parts []string
	if d > 0 {
		parts = append(parts, strconv.Itoa(d)+" "+plural(d, "день", "дня", "дней"))
	}
	if h > 0 {
		parts = append(parts, strconv.Itoa(h)+" "+plural(h, "час", "часа", "часов"))
	}
	if m > 0 {
		parts = append(parts, strconv.Itoa(m)+" "+plural(m, "минуту", "минуты", "минут"))
	}
	if s > 0 || len(parts) == 0 {
		parts = append(parts, strconv.Itoa(s)+" "+plural(s, "секунду", "секунды", "секунд"))
	}
	return strings.Join(parts, " ")
}

func sectorsWord(n int) string { return plural(n, "сектор", "сектора", "секторов") }
func bonusesWord(n int) string { return plural(n, "бонус", "бонуса", "бонусов") }

// timerScript — обратный отсчёт движка: <span class="bold_off" id="timeN">текст</span>
// + инлайн-скрипт new Timer(...), который по нулю перезагружает play-страницу.
// remain — секунд до события; текст = remain, StartCounter = remain+1 (как у сервера).
func timerScript(id string, remain int, playPath string) string {
	return `<span class="bold_off" id="` + id + `">` + ruDuration(remain) + `</span><script type="text/javascript">` + "\r\n" +
		"\t//<![CDATA[\t\r\n" +
		"\twindow." + id + ` = new Timer({"days":["дн","дн","дн"],"hours":["ч","ч","ч"],"minutes":["м","м","м"],"seconds":["с","с","с"],"StartCounter":` + strconv.Itoa(remain+1) + `,"TimeDirection":"Down","ShowTimeUnits":true,"TimerTextID":"` + id + `","DisplayTwoTimes":false}, function(timer, seconds) { if (seconds > 0) return true; window.location = '` + playPath + `'; });` + "\r\n" +
		"\t//]]>\r\n" +
		"\t</script>"
}

// ---------------------------------------------------------------- блоки страницы

// renderTimer — блок автоперехода (перед секторами и заданием).
func renderTimer(v *View) string {
	if v.Timeout <= 0 {
		return ""
	}
	id := randTimerID()
	s := "\t<div class=\"spacer\"></div>\r\n" +
		"\t<h3 class=\"timer\">\r\n" +
		"\t\t<strong>Автопереход</strong> на следующий уровень через&nbsp;" + timerScript(id, v.TimeoutRemain, v.PlayPath) + "\r\n"
	if v.TimeoutAward != 0 {
		s += "\t\t\t(штраф&nbsp;" + ruDuration(v.TimeoutAward) + ")\r\n"
	}
	return s + "\t</h3>\r\n"
}

// renderSectors — список секторов (только если секторов больше одного).
// До 10 секторов — одна колонка (cols w100per), больше — три колонки.
func renderSectors(v *View) string {
	n := len(v.Sectors)
	if n <= 1 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\t<div class=\"spacer\"></div>\r\n")
	b.WriteString("\t<h3>На уровне " + strconv.Itoa(n) + " " + sectorsWord(n) + " \r\n")
	if v.SectorsLeft < n {
		b.WriteString("\t\t<span class=\"color_sec\">(осталось закрыть " + strconv.Itoa(v.SectorsLeft) + ")</span>\r\n")
	}
	b.WriteString("\t</h3>\r\n")
	b.WriteString("\t<div class=\"cols-wrapper\">\r\n")

	cols := [][]SectorView{v.Sectors}
	class := "cols w100per"
	if n > 10 {
		class = "cols"
		per := (n + 2) / 3
		cols = nil
		for i := 0; i < n; i += per {
			end := i + per
			if end > n {
				end = n
			}
			cols = append(cols, v.Sectors[i:end])
		}
	}
	for _, col := range cols {
		b.WriteString("\t\t<div class=\"" + class + "\"> \t\t\r\n\t\t\r\n\r\n")
		for _, s := range col {
			b.WriteString("\t\t\t\t<p id=\"" + strconv.Itoa(s.SectorID) + "\"><span class=\"sector_name\">" + HTMLEncode(s.Name) + "</span>: ")
			if s.Answered {
				b.WriteString("<span class=\"color_correct\">" + HTMLEncode(s.Answer) + "</span> <span class=\"color_sec\">(" + s.AnsweredAt.Format("02.01.2006 15:04:05") + " <a href=\"/UserDetails.aspx?uid=" + strconv.Itoa(s.UserID) + "\">" + HTMLEncode(s.Login) + "</a>)</span></p>\r\n")
			} else {
				b.WriteString("<span class=\"color_dis\">код не введён</span></p>\r\n")
			}
		}
		b.WriteString("\t\t</div><!--end cols-->\r\n")
	}
	b.WriteString("\t</div><!--end cols-wrapper -->\r\n")
	return b.String()
}

// renderTask — блок задания (если тело не пустое).
func renderTask(v *View) string {
	if v.TaskHTML == "" {
		return ""
	}
	return "\t<div class=\"spacer\"></div>\r\n\t<h3>Задание</h3>\r\n\t<div class=\"task\">\r\n\t<p>" + string(v.TaskHTML) + "</p>\r\n\t</div>\r\n\r\n"
}

// renderHelps — содержимое #ordinary_helps.
func renderHelps(v *View) string {
	var b strings.Builder
	for _, h := range v.Hints {
		b.WriteString("\t<div class=\"spacer\"></div>\r\n")
		if h.Shown {
			b.WriteString("\t\t<h3>Подсказка " + strconv.Itoa(h.Number) + "</h3>\r\n")
			b.WriteString("\t\t<p>" + string(h.Text) + "</p>\r\n")
			continue
		}
		b.WriteString("\t\t<span class=\"color_dis\"><b>Подсказка&nbsp;" + strconv.Itoa(h.Number) + "</b>&nbsp;будет через&nbsp;" + timerScript(randTimerID(), h.Remain, v.PlayPath) + "</span>\r\n")
	}
	return b.String()
}

// renderPenalties — содержимое #penalty_helps.
// Состояния: по таймеру → отсчёт; доступна → комментарий и ссылка ?pact=2 (запрос
// подтверждения); подтверждение (ConfirmIndex) → «Согласен» (?pact=1) / «Отмена»;
// открыта → текст.
func renderPenalties(v *View) string {
	var b strings.Builder
	for _, p := range v.Penalties {
		b.WriteString("\t<div class=\"spacer\"></div>\r\n")
		switch {
		case p.State == 2:
			b.WriteString("\t\t<h3 class=\"inline\">Штрафная подсказка " + strconv.Itoa(p.Number) + "</h3>\r\n")
			b.WriteString("\t\t\t<p>" + string(p.Text) + "</p>\r\n")
		case p.Remain > 0:
			b.WriteString("\t\t<span class=\"color_dis\"><b>Штрафная подсказка&nbsp;" + strconv.Itoa(p.Number) + "</b>&nbsp;будет через&nbsp;" + timerScript(randTimerID(), p.Remain, v.PlayPath) + "</span>\r\n")
		case v.ConfirmIndex == p.Index+1:
			b.WriteString("\t\t<h3 class=\"inline\">Штрафная подсказка " + strconv.Itoa(p.Number) + "</h3>\r\n")
			b.WriteString("\t\t\t<p>За просмотр этой подсказки вам будет начислено <span>" + ruDuration(p.Penalty) + "<span> штрафного времени.&nbsp;<a href=\"" + v.PlayPath + "?pact=1&amp;pid=" + strconv.Itoa(p.HelpID) + "\">Согласен</a>&nbsp;&nbsp;<a href=\"" + v.PlayPath + "\">Отмена</a></p>\r\n")
		default:
			b.WriteString("\t\t<h3 class=\"inline\">Штрафная подсказка " + strconv.Itoa(p.Number) + "</h3>\r\n")
			comment := ""
			pact := "1"
			if p.Comment != "" {
				comment = HTMLEncode(p.Comment) + "&nbsp;"
				pact = "2"
			}
			b.WriteString("\t\t\t<p>" + comment + "<a href=\"" + v.PlayPath + "?pact=" + pact + "&amp;pid=" + strconv.Itoa(p.HelpID) + "\">Взять подсказку (штраф " + ruDuration(p.Penalty) + ")</a></p>\r\n")
		}
	}
	return b.String()
}

// renderBonuses — содержимое #bonuses.
func renderBonuses(v *View) string {
	n := len(v.Bonuses)
	if n == 0 {
		return "\r\n"
	}
	var b strings.Builder
	b.WriteString("\r\n<h3 class=\"bonus_count\">На уровне " + strconv.Itoa(n) + " " + bonusesWord(n) + " </h3>\r\n")
	b.WriteString("<span class=\"bonus_count color_sec\">(Выполненные - " + strconv.Itoa(v.PassedBonuses) + ")</span>\r\n")
	for _, bo := range v.Bonuses {
		b.WriteString("\t<div class=\"spacer\"></div>\r\n")
		title := "Бонус " + strconv.Itoa(bo.Number)
		if bo.Name != "" {
			title += ": " + HTMLEncode(bo.Name)
		}
		if !bo.Answered {
			b.WriteString("\t\t<h3 class=\"color_bonus\"> \r\n\t\t\t" + title + "\r\n\t\t</h3>\r\n")
			if bo.Task != "" {
				b.WriteString("\t\t\t\t\t\t<p>" + string(bo.Task) + "</p>\r\n")
			}
			continue
		}
		b.WriteString("\t\t<h3 class=\"color_correct\">\r\n\t\t\t" + title + "\r\n")
		if bo.Negative {
			b.WriteString("\t\t\t<span class=\"color_pen\">(выполнен, штраф " + ruDuration(bo.Award) + ")</span>\t\t\t\r\n")
		} else {
			b.WriteString("\t\t\t<span class=\"color_sec\">(выполнен, награда " + ruDuration(bo.Award) + ")</span>\t\t\t\r\n")
		}
		b.WriteString("\t\t</h3>\r\n")
		b.WriteString("\t\t<div class=\"spacer_answer\"></div>\r\n")
		b.WriteString("\t\t<span class=\"answer_bonus\">? " + bo.AnsweredAt.Format("02.01 15:04:05") + " " + HTMLEncode(bo.Login) + " [ <span class=\"color_bonus\">" + HTMLEncode(bo.Answer) + "</span> ]</span>\r\n")
		if bo.Help != "" {
			b.WriteString("\t\t\t<p>" + string(bo.Help) + "</p>\r\n")
		}
	}
	return b.String()
}

// renderHistory — содержимое <ul class="history">: пустая строка, необязательное
// уведомление о последнем вводе, пустая строка, записи (новые сверху) и сразу </ul>.
func renderHistory(v *View) string {
	var b strings.Builder
	b.WriteString("\r\n")
	if v.Notice != "" {
		if v.NoticeCorrect {
			b.WriteString("\t\t\t\t <li class=\"color_correct\">" + v.Notice + "</li> \r\n")
		} else {
			b.WriteString("\t\t\t\t <li id=\"incorrect\">" + v.Notice + "</li> \r\n")
		}
	}
	b.WriteString("\r\n")
	for _, h := range v.History {
		class := ""
		if h.Correct {
			class = "correct"
		}
		b.WriteString("\t\t\t<li class=\"" + class + "\">\r\n")
		b.WriteString(h.Time + "\r\n")
		if h.OtherLevel > 0 {
			b.WriteString("\t\t\t\t\t(" + strconv.Itoa(h.OtherLevel) + ")\r\n")
		}
		b.WriteString("\r\n")
		b.WriteString("\t\t\t\t<a href=\"/userdetails.aspx?uid=" + strconv.Itoa(h.UserID) + "\">" + HTMLEncode(h.Login) + "</a>\r\n")
		b.WriteString("\t\t\t\t\r\n")
		var span, answer, tail string
		switch {
		case !h.Correct && h.JustNow:
			span, answer = "color_incorrect", wrapLetters(h.Answer)
		case !h.Correct:
			span, answer = "incorrect", wrapLetters(h.Answer)
		case h.Kind == 2 && h.Negative:
			span, answer, tail = "color_pen", HTMLEncode(h.Answer), "\t"
		case h.Kind == 2:
			span, answer = "color_bonus", HTMLEncode(h.Answer)
		default:
			span, answer = "color_correct", HTMLEncode(h.Answer)
		}
		b.WriteString("\t\t\t\t\t<span class=\"" + span + "\">\r\n")
		b.WriteString("\t\t\t\t\t\t" + answer + "\r\n")
		b.WriteString("\t\t\t\t\t</span>" + tail + "\r\n")
		b.WriteString("\t\t\t</li>\r\n")
	}
	return b.String()
}

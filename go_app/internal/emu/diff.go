package emu

import (
	"regexp"
	"strings"
)

// Нормализация для побайтового сравнения с реальной страницей движка: маскируются
// только заведомо динамические места. Go-порт tools/emudiff.py (правила те же).
var normalizeRules = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`rnd=0,\d+`), "rnd=RND"},
	{regexp.MustCompile(`time\d{5,}`), "timeID"},
	{regexp.MustCompile(`"StartCounter":\d+`), `"StartCounter":N`},
	{regexp.MustCompile(`(id="timeID">)[^<]*(</span>)`), "${1}T${2}"},
	{regexp.MustCompile(`name="LevelId" value="\d+"`), `name="LevelId" value="LID"`},
	{regexp.MustCompile(`<p id="\d+">`), `<p id="SID">`},
	{regexp.MustCompile(`pid=\d+`), "pid=PID"},
	{regexp.MustCompile(`uid=\d+`), "uid=UID"},
	{regexp.MustCompile(`topic=\d+`), "topic=TOPIC"},
	{regexp.MustCompile(`\d\d\.\d\d\.\d{4} \d\d:\d\d:\d\d`), "DD.MM.YYYY HH:MM:SS"},
	{regexp.MustCompile(`\d\d\.\d\d \d\d:\d\d:\d\d`), "DD.MM HH:MM:SS"},
	{regexp.MustCompile(`(?m)^\d\d:\d\d:\d\d(\r?)$`), "HH:MM:SS${1}"},
	{regexp.MustCompile(`(?s)\n<!--emu-->.*$`), ""},
	{regexp.MustCompile(`https://(?:d1|cdn)\.endata\.cx/data/games/\d+/[0-9a-f-]{36}\.(\w+)`), "ASSET.${1}"},
	{regexp.MustCompile(`/assets/[A-Za-z0-9_~.-]+\.(\w+)`), "ASSET.${1}"},
}

// NormalizePage маскирует динамические места страницы.
func NormalizePage(s string) string {
	for _, r := range normalizeRules {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}

// DiffLine — строка результата сравнения.
type DiffLine struct {
	Kind string `json:"kind"` // "=" общая, "-" только в real, "+" только в emu
	Real int    `json:"real"` // номер строки в real (0 — нет)
	Emu  int    `json:"emu"`  // номер строки в emu (0 — нет)
	Text string `json:"text"`
}

// DiffResult — результат сравнения двух страниц.
type DiffResult struct {
	Equal     bool       `json:"equal"`
	RealLines int        `json:"realLines"`
	EmuLines  int        `json:"emuLines"`
	Changes   int        `json:"changes"`
	Lines     []DiffLine `json:"lines"` // изменённые строки с одной строкой контекста
}

// Diff сравнивает реальную страницу и страницу эмулятора после нормализации.
func Diff(real, emu string) DiffResult {
	a := strings.Split(NormalizePage(real), "\n")
	b := strings.Split(NormalizePage(emu), "\n")
	res := DiffResult{RealLines: len(a), EmuLines: len(b)}
	if strings.Join(a, "\n") == strings.Join(b, "\n") {
		res.Equal = true
		return res
	}
	// LCS по строкам (страницы небольшие — сотни строк).
	n, m := len(a), len(b)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var all []DiffLine
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			all = append(all, DiffLine{"=", i + 1, j + 1, a[i]})
			i++
			j++
		case j < m && (i >= n || dp[i][j+1] >= dp[i+1][j]):
			all = append(all, DiffLine{"+", 0, j + 1, b[j]})
			j++
		default:
			all = append(all, DiffLine{"-", i + 1, 0, a[i]})
			i++
		}
	}
	// Оставить изменения с одной строкой контекста вокруг.
	keep := make([]bool, len(all))
	for k, l := range all {
		if l.Kind != "=" {
			res.Changes++
			for d := -1; d <= 1; d++ {
				if k+d >= 0 && k+d < len(all) {
					keep[k+d] = true
				}
			}
		}
	}
	for k, l := range all {
		if keep[k] {
			l.Text = strings.ReplaceAll(strings.ReplaceAll(l.Text, "\t", "→"), "\r", "␍")
			res.Lines = append(res.Lines, l)
		}
	}
	return res
}

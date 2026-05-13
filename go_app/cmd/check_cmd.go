package cmd

import (
	"fmt"
	"os"
	"strings"
	"zapolnyaka/internal/config"
	"zapolnyaka/internal/zapolnyaka"

	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
)

var (
	okStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("46"))  // green
	errStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // red
)

// RunCheck opens GameScenario.aspx and compares actual server state with the game config.
func RunCheck(gamePath string) error {
	hist := LoadHistory()
	login := hist.Login
	password := hist.Password
	if login == "" || password == "" {
		_ = godotenv.Load()
		login = os.Getenv("EN_LOGIN")
		password = os.Getenv("EN_PASSWORD")
	}
	if login == "" || password == "" {
		return fmt.Errorf("учётные данные не заданы — используйте пункт «Авторизоваться» или .env")
	}

	game, prepared, err := config.LoadAll(gamePath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	w := os.Stdout

	z, err := zapolnyaka.New(login, password, game.Domain, game.GameID, config.Delays{})
	if err != nil {
		return fmt.Errorf("start browser: %w", err)
	}
	defer z.Close()

	if err := z.Auth(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	fmt.Fprintf(w, "\n🔍 Сценарий: %s  игра %d\n", game.Domain, game.GameID)
	levelMap, err := z.ScrapeScenario()
	if err != nil {
		return fmt.Errorf("scrape scenario: %w", err)
	}

	allOk := true

	for _, p := range prepared {
		levelNum := p.Conf.Level
		actual := levelMap[levelNum]

		label := fmt.Sprintf("уровень %d", levelNum)
		if p.Conf.Name != nil {
			label += fmt.Sprintf(" (%s)", *p.Conf.Name)
		}

		if actual == nil {
			fmt.Fprintf(w, "  %s: %s\n", label, errStyle.Render("✗ не найден на странице"))
			allOk = false
			continue
		}

		expectedSectors := codesWithSector(p.Codes)
		expectedBonuses := codesWithBonus(p.Codes)

		sOk := actual.SectorCount == len(expectedSectors)
		bOk := actual.BonusCount == len(expectedBonuses)

		sTag := colorTag(sOk, fmt.Sprintf("секторов %d/%d", actual.SectorCount, len(expectedSectors)))
		bTag := colorTag(bOk, fmt.Sprintf("бонусов %d/%d", actual.BonusCount, len(expectedBonuses)))
		fmt.Fprintf(w, "  %s:  %s  %s\n", label, sTag, bTag)

		// Missing sector names — only when the page actually shows names (any non-empty)
		if anyNonEmpty(actual.SectorNames) {
			for _, c := range expectedSectors {
				if c.SectorName == nil {
					continue
				}
				if !containsCI(actual.SectorNames, *c.SectorName) {
					fmt.Fprintln(w, errStyle.Render(fmt.Sprintf("    ✗ нет сектора %q", *c.SectorName)))
					allOk = false
				}
			}
		}

		// Missing bonus names — only when the page actually shows names (any non-empty)
		if anyNonEmpty(actual.BonusNames) {
			for _, c := range expectedBonuses {
				if c.BonusName == nil {
					continue
				}
				if !containsCI(actual.BonusNames, *c.BonusName) {
					fmt.Fprintln(w, errStyle.Render(fmt.Sprintf("    ✗ нет бонуса %q", *c.BonusName)))
					allOk = false
				}
			}
		}

		// First-answer positional check for sectors
		for i := 0; i < min(len(actual.SectorFirstAnswers), len(expectedSectors)); i++ {
			got := actual.SectorFirstAnswers[i]
			want := expectedSectors[i].Answers[0]
			if !strings.EqualFold(got, want) {
				fmt.Fprintln(w, errStyle.Render(fmt.Sprintf("    ✗ сектор %d: ожидалось %q, найдено %q", i+1, want, got)))
				allOk = false
			}
		}

		// First-answer positional check for bonuses
		for i := 0; i < min(len(actual.BonusFirstAnswers), len(expectedBonuses)); i++ {
			got := actual.BonusFirstAnswers[i]
			want := expectedBonuses[i].Answers[0]
			if !strings.EqualFold(got, want) {
				fmt.Fprintln(w, errStyle.Render(fmt.Sprintf("    ✗ бонус %d: ожидалось %q, найдено %q", i+1, want, got)))
				allOk = false
			}
		}

		if !sOk || !bOk {
			allOk = false
		}
	}

	fmt.Fprintln(w)
	if allOk {
		fmt.Fprintln(w, okStyle.Render("✔ Всё совпадает"))
	} else {
		fmt.Fprintln(w, errStyle.Render("✗ Есть расхождения"))
	}
	return nil
}

func colorTag(ok bool, text string) string {
	if ok {
		return okStyle.Render("✓ " + text)
	}
	return errStyle.Render("✗ " + text)
}

func codesWithSector(codes []config.Code) []config.Code {
	var out []config.Code
	for _, c := range codes {
		if c.Type.HasSector() {
			out = append(out, c)
		}
	}
	return out
}

func codesWithBonus(codes []config.Code) []config.Code {
	var out []config.Code
	for _, c := range codes {
		if c.Type.HasBonus() {
			out = append(out, c)
		}
	}
	return out
}

func containsCI(slice []string, s string) bool {
	for _, v := range slice {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

func anyNonEmpty(slice []string) bool {
	for _, v := range slice {
		if v != "" {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

package cmd

import (
	"fmt"
	"os"
	"strings"
	"zapolnyaka/internal/config"
	"zapolnyaka/internal/zapolnyaka"
	"zapolnyaka/pkg/logger"

	"github.com/joho/godotenv"
)

const (
	SelfTestGamePath = "data/selftest/game.yml"
	SelfTestMinLevel = 47
	SelfTestMaxLevel = 56
)

// RunSelfTest uploads the test config to devLevel, scrapes GameScenario.aspx, prints pass/fail.
// Returns nil even on check failures — mismatches are printed inline.
func RunSelfTest(devLevel int) error {
	hist := LoadHistory()
	login, password := hist.Login, hist.Password
	if login == "" || password == "" {
		_ = godotenv.Load()
		login = os.Getenv("EN_LOGIN")
		password = os.Getenv("EN_PASSWORD")
	}
	if login == "" || password == "" {
		return fmt.Errorf("учётные данные не заданы — используйте пункт «Авторизоваться»")
	}

	game, prepared, err := config.LoadAll(SelfTestGamePath)
	if err != nil {
		return fmt.Errorf("load selftest config: %w", err)
	}

	prepared[0].Conf.Level = devLevel

	logger.Printf("🧪 Тест: заливаем devLevel %d\n", devLevel)
	z, err := zapolnyaka.New(login, password, game.Domain, game.GameID, game.Delays)
	if err != nil {
		return fmt.Errorf("start browser: %w", err)
	}
	defer z.Close()

	if err := z.Auth(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	if err := z.ProcessLevel(prepared[0]); err != nil {
		return fmt.Errorf("upload level %d: %w", devLevel, err)
	}

	logger.Printf("🔍 Скрейпим GameScenario.aspx...\n")
	levelMap, err := z.ScrapeScenario()
	if err != nil {
		return fmt.Errorf("scrape scenario: %w", err)
	}

	w := os.Stdout
	actual := levelMap[devLevel]
	if actual == nil {
		fmt.Fprintf(w, "%s\n", errStyle.Render(fmt.Sprintf("  ✗ devLevel %d не найден на GameScenario.aspx", devLevel)))
		return nil
	}

	expectedSectors := codesWithSector(prepared[0].Codes)
	expectedBonuses := codesWithBonus(prepared[0].Codes)

	allOk := true

	sOk := actual.SectorCount == len(expectedSectors)
	bOk := actual.BonusCount == len(expectedBonuses)
	if !sOk || !bOk {
		allOk = false
	}

	fmt.Fprintf(w, "\n  devLevel %d:\n", devLevel)
	fmt.Fprintf(w, "    %s\n", colorTag(sOk, fmt.Sprintf("секторов %d/%d", actual.SectorCount, len(expectedSectors))))
	fmt.Fprintf(w, "    %s\n", colorTag(bOk, fmt.Sprintf("бонусов  %d/%d", actual.BonusCount, len(expectedBonuses))))

	if anyNonEmpty(actual.SectorNames) {
		for _, c := range expectedSectors {
			if c.SectorName == nil {
				continue
			}
			found := containsCI(actual.SectorNames, *c.SectorName)
			fmt.Fprintf(w, "    %s\n", colorTag(found, fmt.Sprintf("сектор %q", *c.SectorName)))
			if !found {
				allOk = false
			}
		}
	}

	for i := 0; i < min(len(actual.SectorFirstAnswers), len(expectedSectors)); i++ {
		got := actual.SectorFirstAnswers[i]
		want := expectedSectors[i].Answers[0]
		ok := strings.EqualFold(got, want)
		fmt.Fprintf(w, "    %s\n", colorTag(ok, fmt.Sprintf("сектор %d первый ответ: ожидалось %q, найдено %q", i+1, want, got)))
		if !ok {
			allOk = false
		}
	}

	if anyNonEmpty(actual.BonusNames) {
		for _, c := range expectedBonuses {
			if c.BonusName == nil {
				continue
			}
			found := containsCI(actual.BonusNames, *c.BonusName)
			fmt.Fprintf(w, "    %s\n", colorTag(found, fmt.Sprintf("бонус %q", *c.BonusName)))
			if !found {
				allOk = false
			}
		}
	}

	for i := 0; i < min(len(actual.BonusFirstAnswers), len(expectedBonuses)); i++ {
		got := actual.BonusFirstAnswers[i]
		want := expectedBonuses[i].Answers[0]
		ok := strings.EqualFold(got, want)
		fmt.Fprintf(w, "    %s\n", colorTag(ok, fmt.Sprintf("бонус %d первый ответ: ожидалось %q, найдено %q", i+1, want, got)))
		if !ok {
			allOk = false
		}
	}

	fmt.Fprintln(w)
	if allOk {
		fmt.Fprintln(w, okStyle.Render("  ✔ Тест пройден"))
	} else {
		fmt.Fprintln(w, errStyle.Render("  ✗ Тест НЕ пройден"))
	}
	return nil
}

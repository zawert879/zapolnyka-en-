package cmd

import (
	"fmt"
	"os"
	"strings"
	"zapolnyaka/internal/config"
)

// RunValidate loads and pretty-prints the full game config without launching a browser.
// Output goes to stdout so it's visible in the terminal.
func RunValidate(gamePath string) error {
	game, prepared, err := config.LoadAll(gamePath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	w := os.Stdout
	fmt.Fprintf(w, "\n📋 Игра: %s  ID: %d  Уровней: %d\n", game.Domain, game.GameID, len(prepared))
	if game.IsDev {
		fmt.Fprintln(w, "   режим: isDev=true")
	}

	for _, p := range prepared {
		c := p.Conf
		fmt.Fprintf(w, "\n── Уровень %d", c.Level)
		if c.Name != nil {
			fmt.Fprintf(w, "  (%s)", *c.Name)
		}
		fmt.Fprintln(w)

		parts := []string{fmt.Sprintf("clean=%v", c.Clean)}
		if p.Body != "" {
			parts = append(parts, fmt.Sprintf("body=%d б", len(p.Body)))
		} else {
			parts = append(parts, "body=—")
		}
		if c.Autopass != nil && *c.Autopass > 0 {
			parts = append(parts, fmt.Sprintf("autopass=%dс", *c.Autopass))
		}
		if len(c.Hints) > 0 {
			parts = append(parts, fmt.Sprintf("подсказок=%d", len(c.Hints)))
		}
		if len(c.PenaltyHints) > 0 {
			parts = append(parts, fmt.Sprintf("штрафных=%d", len(c.PenaltyHints)))
		}
		if c.SectorsToClose != nil {
			parts = append(parts, fmt.Sprintf("sectorsToClose=%d", *c.SectorsToClose))
		}
		fmt.Fprintf(w, "   %s\n", strings.Join(parts, "  "))

		if len(p.Codes) == 0 {
			fmt.Fprintln(w, "   Коды: (пусто)")
			continue
		}

		fmt.Fprintf(w, "   Коды (%d):\n", len(p.Codes))
		fmt.Fprintf(w, "   %-3s  %-14s  %-20s  %-20s  %7s  %s\n",
			"#", "Тип", "Сектор", "Бонус/Штраф", "Ответов", "Время")
		fmt.Fprintln(w, "   "+strings.Repeat("─", 76))
		for i, code := range p.Codes {
			sector := "—"
			if code.SectorName != nil {
				sector = *code.SectorName
			}
			bonus := "—"
			if code.BonusName != nil {
				bonus = *code.BonusName
			}
			timeStr := "—"
			if code.Time != nil {
				timeStr = fmt.Sprintf("%dс", *code.Time)
			}
			fmt.Fprintf(w, "   %-3d  %-14s  %-20s  %-20s  %7d  %s\n",
				i+1, code.Type, truncate(sector, 20), truncate(bonus, 20),
				len(code.Answers), timeStr)
		}
	}

	fmt.Fprintln(w, "\n✔ Конфиги валидны")
	return nil
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

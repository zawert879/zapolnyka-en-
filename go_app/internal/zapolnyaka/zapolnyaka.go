package zapolnyaka

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
	"zapolnyaka/encx"
	"zapolnyaka/internal/config"
	"zapolnyaka/pkg/logger"
	"zapolnyaka/pkg/utils"

	"github.com/schollz/progressbar/v3"
)

func sleep(r config.DelayRange) {
	n := r.Min
	if r.Max > r.Min {
		n = r.Min + rand.Intn(r.Max-r.Min+1)
	}
	if n > 0 {
		time.Sleep(time.Duration(n) * time.Second)
	}
}

// Zapolnyaka drives the en.cx Admin API to upload level content.
type Zapolnyaka struct {
	client     *encx.Client
	login      string
	password   string
	domain     string
	gameID     int
	delays     config.Delays
	levelDbIds map[int]int // levelNumber → DB ID, populated on Auth
}

// New creates a Zapolnyaka instance ready to authenticate and upload levels.
func New(login, password, domain string, gameID int, delays config.Delays) (*Zapolnyaka, error) {
	return &Zapolnyaka{
		client:     encx.New(domain, encx.WithLang("ru")),
		login:      login,
		password:   password,
		domain:     domain,
		gameID:     gameID,
		delays:     delays.Default(),
		levelDbIds: make(map[int]int),
	}, nil
}

// url builds an absolute HTTPS URL for the given path on the configured domain.
func (z *Zapolnyaka) url(path string) string {
	return fmt.Sprintf("https://%s/%s", z.domain, path)
}

// withSessionRetry calls fn; if it returns ErrSessionExpired, re-auths once and retries.
func (z *Zapolnyaka) withSessionRetry(fn func() error) error {
	err := fn()
	if err != nil && errors.Is(err, encx.ErrSessionExpired) {
		logger.Println("  re-auth: сессия истекла, переавторизация...")
		if authErr := z.Auth(); authErr != nil {
			return fmt.Errorf("re-auth failed: %w", authErr)
		}
		return fn()
	}
	return err
}

// ProcessLevel uploads a single prepared level to en.cx via the Admin HTTP API.
func (z *Zapolnyaka) ProcessLevel(p config.PreparedLevel) error {
	ctx := context.Background()
	level := p.Conf.Level

	steps := 0
	if p.Conf.Clean {
		steps++
	}
	if p.Conf.Name != nil || p.Conf.Comment != nil {
		steps++
	}
	if p.Conf.Autopass != nil {
		steps++
	}
	if p.Body != "" {
		steps++
	}
	steps += len(p.Conf.Hints) + len(p.Conf.PenaltyHints)
	if len(p.Codes) > 0 {
		steps++
	}
	if p.Conf.SectorsToClose != nil {
		steps++
	}

	bar := progressbar.NewOptions(steps,
		progressbar.OptionSetDescription(fmt.Sprintf("  Уровень %d", level)),
		progressbar.OptionSetWidth(28),
		progressbar.OptionShowCount(),
	)
	step := func(desc string) { bar.Describe(fmt.Sprintf("  Уровень %d — %s", level, desc)) }
	tick := func() { _ = bar.Add(1) }

	logger.Printf("\n▶ Уровень %d\n", level)

	if p.Conf.Clean {
		step("очистка")
		logger.Printf("  cleanLevel %d\n", level)
		if err := z.cleanLevel(ctx, level); err != nil {
			fmt.Println()
			return fmt.Errorf("clean level: %w", err)
		}
		tick()
	}

	if p.Conf.Name != nil || p.Conf.Comment != nil {
		step("название/комментарий")
		logger.Println("  setLevelNameAndComment")
		if err := z.setLevelNameAndComment(ctx, level, p.Conf.Name, p.Conf.Comment); err != nil {
			fmt.Println()
			return fmt.Errorf("set name/comment: %w", err)
		}
		tick()
	}

	if p.Conf.Autopass != nil {
		step(fmt.Sprintf("автопереход %s", formatDuration(*p.Conf.Autopass)))
		logger.Printf("  setAutopass %s\n", formatDuration(*p.Conf.Autopass))
		if err := z.setAutopass(ctx, level, *p.Conf.Autopass, p.Conf.AutopassPenalty); err != nil {
			fmt.Println()
			return fmt.Errorf("set autopass: %w", err)
		}
		tick()
	}

	if p.Body != "" {
		step("тело задания")
		logger.Printf("  setTaskBody (%d байт)\n", len(p.Body))
		if err := z.setTaskBody(ctx, level, p.Body); err != nil {
			fmt.Println()
			return fmt.Errorf("set task body: %w", err)
		}
		tick()
	}

	for i, h := range p.Conf.Hints {
		step(fmt.Sprintf("подсказка %d/%d", i+1, len(p.Conf.Hints)))
		logger.Printf("  addHint %d/%d time=%ds\n", i+1, len(p.Conf.Hints), h.Time)
		if err := z.addHint(ctx, level, h.Time, h.Text); err != nil {
			fmt.Println()
			return fmt.Errorf("add hint %d: %w", i+1, err)
		}
		tick()
		sleep(z.delays.BetweenHints)
	}

	for i, ph := range p.Conf.PenaltyHints {
		step(fmt.Sprintf("штрафная подсказка %d/%d", i+1, len(p.Conf.PenaltyHints)))
		logger.Printf("  addPenaltyHint %d/%d time=%ds\n", i+1, len(p.Conf.PenaltyHints), ph.Time)
		if err := z.addPenaltyHint(ctx, level, ph.Time, ph.Text, ph.Penalty, ph.Comment); err != nil {
			fmt.Println()
			return fmt.Errorf("add penalty hint %d: %w", i+1, err)
		}
		tick()
		sleep(z.delays.BetweenHints)
	}

	if len(p.Codes) > 0 {
		step(fmt.Sprintf("коды (%d)", len(p.Codes)))
		logger.Printf("  коды: %d записей\n", len(p.Codes))
		fmt.Println()

		codesBar := progressbar.NewOptions(len(p.Codes),
			progressbar.OptionSetDescription("    коды"),
			progressbar.OptionSetWidth(28),
			progressbar.OptionShowCount(),
		)

		for i, code := range p.Codes {
			logger.Printf("  код %d/%d тип=%s\n", i+1, len(p.Codes), code.Type)
			codesBar.Describe(fmt.Sprintf("    код %d/%d: %s", i+1, len(p.Codes), code.Type))
			if code.Type.HasSector() {
				name := ""
				if code.SectorName != nil {
					name = *code.SectorName
				}
				logger.Printf("    addAnswersToSector сектор=%q ответов=%d\n", name, len(code.Answers))
				sectorName, sectorAnswers := name, code.Answers
				if err := z.withSessionRetry(func() error {
					return z.addAnswersToSector(ctx, level, sectorName, sectorAnswers)
				}); err != nil {
					fmt.Println()
					return fmt.Errorf("add sector answers: %w", err)
				}
			}
			if code.Type.HasBonus() {
				bonusName := ""
				if code.BonusName != nil {
					bonusName = *code.BonusName
				}
				logger.Printf("    addBonus бонус=%q isPenalty=%v ответов=%d\n",
					bonusName, code.Type.IsPenalty(), len(code.Answers))
				codeVal := code
				if err := z.withSessionRetry(func() error {
					return z.addBonus(ctx, level, codeVal)
				}); err != nil {
					fmt.Println()
					return fmt.Errorf("add bonus: %w", err)
				}
			}
			logger.Printf("  код %d/%d — готов\n", i+1, len(p.Codes))
			_ = codesBar.Add(1)
			sleep(z.delays.BetweenCodes)
		}
		fmt.Println()
		tick()
	}

	if p.Conf.SectorsToClose != nil {
		step(fmt.Sprintf("условие: %d секторов", *p.Conf.SectorsToClose))
		logger.Printf("  setSectorsToClose %d\n", *p.Conf.SectorsToClose)
		if err := z.setSectorsToClose(ctx, level, *p.Conf.SectorsToClose); err != nil {
			fmt.Println()
			return fmt.Errorf("set sectors to close: %w", err)
		}
		tick()
	}

	fmt.Println()
	logger.Printf("  ✔ уровень %d завершён\n", level)
	sleep(z.delays.BetweenLevels)
	return nil
}

func formatDuration(seconds int) string {
	h, m, s := utils.SecondsToHMS(seconds)
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}

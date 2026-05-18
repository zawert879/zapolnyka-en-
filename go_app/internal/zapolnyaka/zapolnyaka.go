package zapolnyaka

import (
	"fmt"
	"math/rand"
	"os"
	"time"
	"zapolnyaka/internal/config"
	"zapolnyaka/pkg/logger"
	"zapolnyaka/pkg/utils"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
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

const navTimeout = 30 * time.Second

// Zapolnyaka drives the en.cx LevelEditor via a headed browser.
type Zapolnyaka struct {
	browser  *rod.Browser
	page     *rod.Page
	login    string
	password string
	domain   string
	gameID   int
	delays   config.Delays
}

// New launches a headed browser and returns a Zapolnyaka instance.
// Prefers the system-installed Chrome/Edge to avoid Windows Defender false positives
// from an auto-downloaded Chromium. Falls back to rod's managed browser if none found.
func New(login, password, domain string, gameID int, delays config.Delays) (*Zapolnyaka, error) {
	headless := os.Getenv("ZAPOLNYAKA_HEADLESS") == "1"
	l := launcher.New().Headless(headless).Leakless(false)
	if path, ok := launcher.LookPath(); ok {
		l = l.Bin(path)
		logger.Printf("🌐 Используем браузер: %s (headless=%v)\n", path, headless)
	} else {
		logger.Println("🌐 Системный браузер не найден, запускаем встроенный Chromium...")
	}
	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch browser: %w", err)
	}
	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("connect browser: %w", err)
	}
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("open page: %w", err)
	}
	return &Zapolnyaka{
		browser:  browser,
		page:     page,
		login:    login,
		password: password,
		domain:   domain,
		gameID:   gameID,
		delays:   delays.Default(),
	}, nil
}

// Close shuts down the browser.
func (z *Zapolnyaka) Close() {
	z.browser.MustClose()
}

// reconnect relaunches the browser and re-authenticates after a connection drop.
func (z *Zapolnyaka) reconnect() error {
	logger.Alert("  🔄 переподключение браузера...\n")
	_ = z.browser.Close()

	headless := os.Getenv("ZAPOLNYAKA_HEADLESS") == "1"
	l := launcher.New().Headless(headless).Leakless(false)
	if path, ok := launcher.LookPath(); ok {
		l = l.Bin(path)
	}
	u, err := l.Launch()
	if err != nil {
		return fmt.Errorf("relaunch browser: %w", err)
	}
	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("reconnect browser: %w", err)
	}
	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return fmt.Errorf("open page after reconnect: %w", err)
	}
	z.browser = browser
	z.page = page
	return z.Auth()
}

// url builds an absolute URL for the given path on the configured domain.
func (z *Zapolnyaka) url(path string) string {
	return fmt.Sprintf("https://%s/%s", z.domain, path)
}

// levelEditorURL returns the LevelEditor URL for the given level number.
func (z *Zapolnyaka) levelEditorURL(level int) string {
	return z.url(fmt.Sprintf(
		"Administration/Games/LevelEditor.aspx?gid=%d&level=%d&swanswers=1",
		z.gameID, level,
	))
}

// levelEditorFullURL returns LevelEditor URL without any swXXX filter — shows all sections.
func (z *Zapolnyaka) levelEditorFullURL(level int) string {
	return z.url(fmt.Sprintf(
		"Administration/Games/LevelEditor.aspx?gid=%d&level=%d",
		z.gameID, level,
	))
}

// openLevel navigates to LevelEditor (answers section) for the given level.
func (z *Zapolnyaka) openLevel(level int) error {
	return z.gotoSafe(z.levelEditorURL(level))
}

// openLevelFull navigates to the full LevelEditor (all sections visible) for the given level.
func (z *Zapolnyaka) openLevelFull(level int) error {
	return z.gotoSafe(z.levelEditorFullURL(level))
}

// ProcessLevel uploads a single prepared level to en.cx.
func (z *Zapolnyaka) ProcessLevel(p config.PreparedLevel) error {
	level := p.Conf.Level

	// Count total steps for the level progress bar
	steps := 1 // openLevel
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
	// step shows what's happening NOW; tick advances the bar when the step is done.
	step := func(desc string) { bar.Describe(fmt.Sprintf("  Уровень %d — %s", level, desc)) }
	tick := func() { _ = bar.Add(1) }

	logger.Printf("\n▶ Уровень %d\n", level)

	if p.Conf.Clean {
		step("очистка")
		logger.Printf("  cleanLevel %d\n", level)
		if err := z.cleanLevel(level); err != nil {
			fmt.Println()
			return fmt.Errorf("clean level: %w", err)
		}
		tick()
	}

	step("открываем уровень")
	logger.Printf("  openLevel %d\n", level)
	if err := z.openLevel(level); err != nil {
		fmt.Println()
		return fmt.Errorf("open level: %w", err)
	}
	tick()

	if p.Conf.Name != nil || p.Conf.Comment != nil {
		step("название/комментарий")
		logger.Println("  setLevelNameAndComment")
		if err := z.setLevelNameAndComment(level, p.Conf.Name, p.Conf.Comment); err != nil {
			fmt.Println()
			return fmt.Errorf("set name/comment: %w", err)
		}
		tick()
	}

	if p.Conf.Autopass != nil {
		step(fmt.Sprintf("автопереход %s", formatDuration(*p.Conf.Autopass)))
		logger.Printf("  setAutopass %s\n", formatDuration(*p.Conf.Autopass))
		if err := z.setAutopass(level, *p.Conf.Autopass, p.Conf.AutopassPenalty); err != nil {
			fmt.Println()
			return fmt.Errorf("set autopass: %w", err)
		}
		tick()
	}

	if p.Body != "" {
		step("тело задания")
		logger.Printf("  setTaskBody (%d байт)\n", len(p.Body))
		if err := z.setTaskBody(level, p.Body); err != nil {
			fmt.Println()
			return fmt.Errorf("set task body: %w", err)
		}
		tick()
	}

	for i, h := range p.Conf.Hints {
		step(fmt.Sprintf("подсказка %d/%d", i+1, len(p.Conf.Hints)))
		logger.Printf("  addHint %d/%d time=%ds\n", i+1, len(p.Conf.Hints), h.Time)
		if err := z.addHint(level, h.Time, h.Text); err != nil {
			fmt.Println()
			return fmt.Errorf("add hint %d: %w", i+1, err)
		}
		tick()
		sleep(z.delays.BetweenHints)
	}

	for i, ph := range p.Conf.PenaltyHints {
		step(fmt.Sprintf("штрафная подсказка %d/%d", i+1, len(p.Conf.PenaltyHints)))
		logger.Printf("  addPenaltyHint %d/%d time=%ds\n", i+1, len(p.Conf.PenaltyHints), ph.Time)
		if err := z.addPenaltyHint(level, ph.Time, ph.Text, ph.Penalty, ph.Comment); err != nil {
			fmt.Println()
			return fmt.Errorf("add penalty hint %d: %w", i+1, err)
		}
		tick()
		sleep(z.delays.BetweenHints)
	}

	if len(p.Codes) > 0 {
		step(fmt.Sprintf("коды (%d)", len(p.Codes)))
		logger.Printf("  коды: %d записей, открываем LevelEditor...\n", len(p.Codes))
		if err := z.openLevel(level); err != nil {
			fmt.Println()
			return fmt.Errorf("reopen for codes: %w", err)
		}
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
				if err := z.addAnswersToSector(level, name, code.Answers); err != nil {
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
				if err := z.addBonus(level, code); err != nil {
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
		if err := z.setSectorsToClose(level, *p.Conf.SectorsToClose); err != nil {
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

// setFieldsViaDom sets all named inputs in a single Eval to avoid ASP.NET AutoPostBack.
func (z *Zapolnyaka) setFieldsViaDom(fields map[string]string) error {
	_, err := z.page.Eval(`(fields) => {
		for (const [name, val] of Object.entries(fields)) {
			const el = document.querySelector('[name="' + name + '"]');
			if (el) el.value = val;
		}
	}`, fields)
	return err
}

// submitForm clicks a button by selector and waits for the next page load.
func (z *Zapolnyaka) submitForm(sel string) error {
	logger.Printf("    submitForm sel=%q\n", sel)
	wait := z.page.Timeout(navTimeout).WaitNavigation(proto.PageLifecycleEventNameLoad)
	res, err := z.page.Eval(`(sel) => {
		const el = document.querySelector(sel);
		if (!el) return 'not found: ' + sel;
		el.click();
		return 'ok';
	}`, sel)
	if err != nil {
		return fmt.Errorf("submitForm click: %w", err)
	}
	if v := res.Value.String(); v != "ok" {
		return fmt.Errorf("submitForm: кнопка не найдена: %s", v)
	}
	wait()
	logger.Println("    submitForm — навигация завершена")
	return nil
}

// submitFormAny tries selectors in order and clicks the first matching button.
func (z *Zapolnyaka) submitFormAny(sels ...string) error {
	wait := z.page.Timeout(navTimeout).WaitNavigation(proto.PageLifecycleEventNameLoad)
	res, err := z.page.Eval(`(sels) => {
		for (const sel of sels) {
			const el = document.querySelector(sel);
			if (el) { el.click(); return 'ok:' + sel; }
		}
		return 'not found';
	}`, sels)
	if err != nil {
		return fmt.Errorf("submitFormAny: %w", err)
	}
	v := res.Value.String()
	if v == "not found" {
		return fmt.Errorf("submitFormAny: ни один из селекторов не нашёл кнопку: %v", sels)
	}
	logger.Printf("    submitFormAny: %s\n", v)
	wait()
	return nil
}

// fetchURL executes a credentialed fetch inside the browser context.
func (z *Zapolnyaka) fetchURL(url string) error {
	_, err := z.page.Eval(`(url) => fetch(url, {credentials:'include'})`, url)
	return err
}

// waitForNewPage clicks something via clickFn, then waits for a new browser tab to appear.
func (z *Zapolnyaka) waitForNewPage(clickFn func() error) (*rod.Page, error) {
	pagesBefore := z.browser.MustPages()
	if err := clickFn(); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		pagesAfter := z.browser.MustPages()
		if len(pagesAfter) > len(pagesBefore) {
			newPage := pagesAfter[len(pagesAfter)-1]
			if err := newPage.Timeout(navTimeout).WaitLoad(); err != nil {
				logger.Printf("  waitForNewPage WaitLoad: %v\n", err)
			}
			return newPage, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("new page (popup) not opened within 30s")
}

func formatDuration(seconds int) string {
	h, m, s := utils.SecondsToHMS(seconds)
	if h > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
}

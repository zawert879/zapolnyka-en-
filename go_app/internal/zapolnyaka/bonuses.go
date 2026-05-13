package zapolnyaka

import (
	"fmt"
	"time"
	"zapolnyaka/internal/config"
	"zapolnyaka/pkg/logger"
	"zapolnyaka/pkg/utils"

	"github.com/go-rod/rod/lib/proto"
)

const maxAnswersPerBonus = 100

// addBonus creates a bonus/penalty by navigating directly to BonusEdit.aspx (no popup).
func (z *Zapolnyaka) addBonus(level int, code config.Code) error {
	if code.Time == nil {
		return fmt.Errorf("addBonus: time is required")
	}
	chunks := utils.ChunkSlice(code.Answers, maxAnswersPerBonus)
	if len(chunks) == 0 {
		return nil
	}

	bonusName := ""
	if code.BonusName != nil {
		bonusName = *code.BonusName
	}
	help := ""
	if code.Help != nil {
		help = *code.Help
	}

	addURL := z.url(fmt.Sprintf(
		"Administration/Games/BonusEdit.aspx?gid=%d&level=%d&action=add",
		z.gameID, level,
	))
	logger.Printf("    addBonus: GET %s\n", addURL)
	if err := z.gotoSafe(addURL); err != nil {
		return fmt.Errorf("navigate to BonusEdit: %w", err)
	}

	if err := z.fillBonusForm(level, bonusName, help, *code.Time, code.Type.IsPenalty(), chunks[0]); err != nil {
		return fmt.Errorf("create bonus: %w", err)
	}

	for i, chunk := range chunks[1:] {
		if err := z.appendBonusAnswers(chunk); err != nil {
			return fmt.Errorf("add bonus answers chunk %d: %w", i+2, err)
		}
	}
	return nil
}

// fillBonusForm fills and submits the BonusEdit add/edit form on z.page.
func (z *Zapolnyaka) fillBonusForm(level int, name, help string, timeSeconds int, isPenalty bool, answers []string) error {
	// Expand answer fields if we need more than the default 10
	if needed := len(answers); needed > 10 {
		clicks := (needed - 10 + 29) / 30
		for i := 0; i < clicks; i++ {
			if el, err := z.page.Timeout(2 * time.Second).ElementR("a", `^30$`); err == nil {
				_ = el.Click("left", 1)
				_ = z.page.WaitIdle(3 * time.Second)
			}
		}
	}

	h, m, s := utils.SecondsToHMS(timeSeconds)
	fields := map[string]string{
		"txtBonusName": name,
		"txtHelp":      help,
		"txtHours":     fmt.Sprintf("%d", h),
		"txtMinutes":   fmt.Sprintf("%d", m),
		"txtSeconds":   fmt.Sprintf("%d", s),
	}
	for i, a := range answers {
		fields[fmt.Sprintf("answer_-%d", i+1)] = a
	}

	if _, err := z.page.Eval(`(fields) => {
		for (const [name, val] of Object.entries(fields)) {
			const el = document.querySelector('[name="' + name + '"]');
			if (el) el.value = val;
		}
	}`, fields); err != nil {
		return err
	}

	// Select "на указанных" and check the level checkbox
	if _, err := z.page.Eval(`(lvl) => {
		const rb = document.querySelector('#rbCustomLevels');
		if (rb) rb.click();
		const wrappers = document.querySelectorAll('#levels_container .levelWrapper');
		for (const w of wrappers) {
			const cb = w.querySelector('input[type=checkbox]');
			const txt = (w.textContent || '').trim();
			if (cb && new RegExp('(^|\\D)' + lvl + '(\\D|$)').test(txt)) {
				cb.checked = true;
			}
		}
	}`, fmt.Sprintf("%d", level)); err != nil {
		return err
	}

	if isPenalty {
		if _, err := z.page.Eval(`() => {
			const neg = document.querySelector('[name="negative"]');
			if (neg) neg.click();
		}`); err != nil {
			return err
		}
		_ = z.page.WaitIdle(3 * time.Second)
	}

	logger.Println("    fillBonusForm: submit")
	wait := z.page.Timeout(navTimeout).WaitNavigation(proto.PageLifecycleEventNameLoad)
	if _, err := z.page.Eval(`() => {
		const btn = document.querySelector('[name="btnAdd"]');
		if (!btn) throw new Error('btnAdd not found');
		btn.click();
	}`); err != nil {
		return fmt.Errorf("click btnAdd: %w", err)
	}
	wait()
	logger.Println("    fillBonusForm: навигация завершена")
	return nil
}

// appendBonusAnswers clicks "Редактировать" on the last bonus and adds more answers.
func (z *Zapolnyaka) appendBonusAnswers(answers []string) error {
	if _, err := z.page.Eval(`() => {
		const links = Array.from(document.querySelectorAll('a')).filter(a => a.textContent.trim() === 'Редактировать');
		if (links.length === 0) throw new Error('no Редактировать links found');
		links[links.length - 1].click();
	}`); err != nil {
		return fmt.Errorf("click Редактировать: %w", err)
	}
	if err := z.page.Timeout(navTimeout).WaitLoad(); err != nil {
		logger.Printf("    appendBonusAnswers WaitLoad: %v\n", err)
	}

	if needed := len(answers); needed > 10 {
		clicks := (needed - 10 + 29) / 30
		for i := 0; i < clicks; i++ {
			if el, err := z.page.Timeout(2 * time.Second).ElementR("a", `^30$`); err == nil {
				_ = el.Click("left", 1)
				_ = z.page.WaitIdle(3 * time.Second)
			}
		}
	}

	fields := map[string]string{}
	for i, a := range answers {
		fields[fmt.Sprintf("answer_-%d", i+1)] = a
	}
	if _, err := z.page.Eval(`(fields) => {
		for (const [name, val] of Object.entries(fields)) {
			const el = document.querySelector('[name="' + name + '"]');
			if (el) el.value = val;
		}
	}`, fields); err != nil {
		return err
	}

	wait := z.page.Timeout(navTimeout).WaitNavigation(proto.PageLifecycleEventNameLoad)
	if _, err := z.page.Eval(`() => {
		const btn = document.querySelector('[name="btnUpdate"]');
		if (!btn) throw new Error('btnUpdate not found');
		btn.click();
	}`); err != nil {
		return fmt.Errorf("click btnUpdate: %w", err)
	}
	wait()
	return nil
}

package zapolnyaka

import (
	"fmt"
	"regexp"
	"time"
	"zapolnyaka/pkg/logger"
	"zapolnyaka/pkg/utils"
)

const maxAnswersPerSector = 10

// addAnswersToSector creates a new sector with the given name and adds all answers.
func (z *Zapolnyaka) addAnswersToSector(level int, sectorName string, answers []string) error {
	if err := z.openLevel(level); err != nil {
		return err
	}

	// Click "показать" if present (2s timeout — optional)
	if el, _ := z.page.Timeout(2 * time.Second).ElementR("a", "показать"); el != nil {
		logger.Println("    кликаем «показать»")
		_ = el.Click("left", 1)
		_ = z.page.WaitIdle(5 * time.Second)
	}

	chunks := utils.ChunkSlice(answers, maxAnswersPerSector)
	if len(chunks) == 0 {
		return nil
	}

	logger.Printf("    создаём сектор %q, чанк 1/%d (%d ответов)\n", sectorName, len(chunks), len(chunks[0]))
	if err := z.addAnswersToNewSector(sectorName, chunks[0]); err != nil {
		return fmt.Errorf("create sector: %w", err)
	}

	for i, chunk := range chunks[1:] {
		logger.Printf("    добавляем ответы, чанк %d/%d (%d ответов)\n", i+2, len(chunks), len(chunk))
		if err := z.addAnswersToLastSector(level, chunk); err != nil {
			return fmt.Errorf("add answers chunk %d: %w", i+2, err)
		}
	}
	return nil
}

// addAnswersToNewSector opens the sector creation form via ToggleCreateSector (extracted
// from the link's onclick), then fills name + answers via JS and submits.
func (z *Zapolnyaka) addAnswersToNewSector(sectorName string, answers []string) error {
	// Extract ToggleCreateSector args from the link's onclick and call it directly.
	// en.cx onclick = "ToggleCreateSector('divA','divB','divC');return false;"
	logger.Println("    вызываем ToggleCreateSector...")
	if _, err := z.page.Eval(`() => {
		const link = [...document.querySelectorAll('a')].find(a => a.textContent.trim().includes('Добавить сектор'));
		if (!link) return;
		const m = link.getAttribute('onclick').match(/ToggleCreateSector\(([^)]+)\)/);
		if (!m) return;
		const args = m[1].split(',').map(s => s.trim().replace(/['"]/g, ''));
		ToggleCreateSector(args[0], args[1], args[2]);
	}`); err != nil {
		return fmt.Errorf("ToggleCreateSector: %w", err)
	}

	// Wait for the sector name input to become visible (non-zero size).
	logger.Println("    ждём форму...")
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		res, err2 := z.page.Eval(`() => {
			const el = document.querySelector('input[name="txtSectorName"]');
			if (!el) return null;
			const r = el.getBoundingClientRect();
			return r.width > 0 && r.height > 0;
		}`)
		if err2 == nil && res != nil && res.Value.Bool() {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Fill sector name.
	logger.Printf("    вводим имя %q\n", sectorName)
	if _, err := z.page.Eval(`(name) => {
		const el = document.querySelector('input[name="txtSectorName"]');
		if (!el) return;
		el.focus();
		el.value = name;
		el.dispatchEvent(new Event('input', {bubbles: true}));
		el.dispatchEvent(new Event('change', {bubbles: true}));
	}`, sectorName); err != nil {
		return fmt.Errorf("set sector name: %w", err)
	}

	// Fill answer fields.
	logger.Printf("    вводим %d ответов\n", len(answers))
	for i, ans := range answers {
		sel := fmt.Sprintf(`#AnswersTable_ctl00_divSectorEdit input[name="txtAnswer_%d"]`, i)
		if _, err := z.page.Timeout(5 * time.Second).Element(sel); err != nil {
			logger.Printf("    txtAnswer_%d не найден: %v\n", i, err)
			continue
		}
		if _, err := z.page.Eval(`(sel, val) => {
			const el = document.querySelector(sel);
			if (!el) return;
			el.value = val;
			el.dispatchEvent(new Event('input', {bubbles: true}));
			el.dispatchEvent(new Event('change', {bubbles: true}));
		}`, sel, ans); err != nil {
			logger.Printf("    fill txtAnswer_%d: %v\n", i, err)
		}
	}

	return z.submitSectorForm()
}

// addAnswersToLastSector appends answers to the most recently created sector.
func (z *Zapolnyaka) addAnswersToLastSector(level int, answers []string) error {
	if err := z.openLevel(level); err != nil {
		return err
	}

	// Find the last sector's id
	containerEl, err := z.page.Timeout(15 * time.Second).Element("#AnswersTable_ctl00_divAnswersContainer")
	if err != nil {
		return fmt.Errorf("find divAnswersContainer: %w", err)
	}
	html, err := containerEl.HTML()
	if err != nil {
		return fmt.Errorf("get container html: %w", err)
	}
	re := regexp.MustCompile(`id="divAnswersEdit_(\d+)"`)
	matches := re.FindAllStringSubmatch(html, -1)
	if len(matches) == 0 {
		return fmt.Errorf("no sectors found on page")
	}
	sectorID := matches[len(matches)-1][1]
	logger.Printf("    последний сектор id=%s\n", sectorID)

	// Click "Добавить ответы"
	addLink, err := z.page.Timeout(15 * time.Second).ElementR("a", "Добавить ответы")
	if err != nil {
		return fmt.Errorf("find «Добавить ответы»: %w", err)
	}
	if err := addLink.Click("left", 1); err != nil {
		return fmt.Errorf("click Добавить ответы: %w", err)
	}
	if err := z.page.WaitIdle(15 * time.Second); err != nil {
		logger.Printf("    waitIdle: %v\n", err)
	}

	// Select sector in the dropdown and dispatch change to trigger AutoPostBack
	if _, err := z.page.Eval(`(id) => {
		const s = document.querySelector('#ddlSector');
		if (!s) return;
		s.value = id;
		s.dispatchEvent(new Event('change', {bubbles:true}));
	}`, sectorID); err != nil {
		return fmt.Errorf("select sector: %w", err)
	}
	// Wait for AutoPostBack (answer fields may load via partial postback)
	if err := z.page.WaitIdle(15 * time.Second); err != nil {
		logger.Printf("    waitIdle after select: %v\n", err)
	}

	// Fill answers via native Input() inside the answer editor
	logger.Printf("    вводим %d ответов\n", len(answers))
	for i, ans := range answers {
		sel := fmt.Sprintf(`#AnswersTable_ctl00_NewAnswerEditor input[name="txtAnswer_%d"]`, i)
		el, err := z.page.Timeout(5 * time.Second).Element(sel)
		if err != nil {
			logger.Printf("    txtAnswer_%d не найден: %v\n", i, err)
			continue
		}
		if err := el.Input(ans); err != nil {
			return fmt.Errorf("input answer %d: %w", i, err)
		}
	}

	return z.submitSectorForm()
}

// submitSectorForm clicks the sector-form Сохранить button.
// Prioritises the button inside divSectorEdit (new sector) or NewAnswerEditor (add answers),
// because the LevelEditor page has multiple Сохранить buttons and a naive page-wide
// find would click the wrong one (pnlSettings_AnswerBlockingSettings comes first in DOM).
func (z *Zapolnyaka) submitSectorForm() error {
	res, err := z.page.Eval(`() => {
		// 1. New-sector form button (name="btnSaveSector")
		const sectorBtn = document.querySelector('input[name="btnSaveSector"]');
		if (sectorBtn) { sectorBtn.click(); return 'btnSaveSector'; }
		// 2. Add-answers-to-existing-sector editor button
		const editorBtn = document.querySelector('#AnswersTable_ctl00_NewAnswerEditor input[type="image"][alt="Сохранить"]');
		if (editorBtn) { editorBtn.click(); return 'editorBtn'; }
		// 3. Last resort: any Сохранить button
		const all = [...document.querySelectorAll('button, input[type="submit"], input[type="image"]')];
		const btn = all.find(el => (el.textContent || el.value || el.getAttribute('alt') || '').includes('Сохранить'));
		if (btn) { btn.click(); return 'fallback'; }
		return 'not found';
	}`)
	if err != nil {
		return fmt.Errorf("submitSectorForm eval: %w", err)
	}
	what := res.Value.String()
	logger.Printf("    submitSectorForm: %s\n", what)
	if what == "not found" {
		return fmt.Errorf("submitSectorForm: кнопка «Сохранить» не найдена на странице")
	}
	// Wait for page to settle (works for full postback and UpdatePanel partial postback)
	if err := z.page.WaitIdle(navTimeout); err != nil {
		logger.Printf("    submitSectorForm WaitIdle: %v\n", err)
	}
	logger.Println("    submitSectorForm — готово")
	return nil
}

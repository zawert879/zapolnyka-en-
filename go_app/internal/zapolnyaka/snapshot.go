package zapolnyaka

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"zapolnyaka/encx"
	"zapolnyaka/pkg/logger"
)

func (z *Zapolnyaka) playURL() string {
	return fmt.Sprintf("https://%s/gameengines/encounter/play/%d/", z.domain, z.gameID)
}

// preparePlay выставляет cookie view-mode=desktop (как «Desktop» в шапке play-страницы).
// User-Agent после логина менять нельзя: сессия en.cx привязана к UA, и запросы
// с другим UA уходят на simplelogin.aspx.
func (z *Zapolnyaka) preparePlay() {
	_ = z.client.SetCookie("view-mode", "desktop")
}

// PlayPage возвращает HTML текущей play-страницы команды (GET).
func (z *Zapolnyaka) PlayPage() (string, error) {
	z.preparePlay()
	html, err := z.client.GetPage(context.Background(), z.playURL())
	if err != nil {
		return "", fmt.Errorf("get play page: %w", err)
	}
	return html, nil
}

// PlayModel возвращает JSON-модель текущей play-страницы (?json=1).
func (z *Zapolnyaka) PlayModel() (*encx.GameModel, error) {
	_, model, err := z.PlayModelRaw()
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, fmt.Errorf("get game model: не удалось декодировать JSON")
	}
	return model, nil
}

// PlayModelRaw возвращает сырой JSON модели и её декодированный вариант.
// Ошибка декодирования не фатальна: raw возвращается всегда, model тогда nil.
func (z *Zapolnyaka) PlayModelRaw() ([]byte, *encx.GameModel, error) {
	z.preparePlay()
	raw, err := z.client.GetGameJSON(context.Background(), z.gameID)
	if err != nil {
		return nil, nil, fmt.Errorf("get game model: %w", err)
	}
	var model encx.GameModel
	if err := json.Unmarshal(raw, &model); err != nil {
		logger.Printf("  snapshot: JSON не декодируется (%v), сохраняю сырым\n", err)
		return raw, nil, nil
	}
	return raw, &model, nil
}

// PlaySnapshot возвращает HTML текущей play-страницы команды и её JSON-модель.
func (z *Zapolnyaka) PlaySnapshot() (string, *encx.GameModel, error) {
	html, err := z.PlayPage()
	if err != nil {
		return "", nil, err
	}
	model, err := z.PlayModel()
	if err != nil {
		return html, nil, err
	}
	return html, model, nil
}

// PlaySendCode отправляет код формой ответа play-страницы (как браузер) и
// возвращает HTML страницы-ответа и HTTP-статус POST'а.
func (z *Zapolnyaka) PlaySendCode(code string) (string, int, error) {
	ctx := context.Background()
	z.preparePlay()
	model, err := z.client.GetGameModel(ctx, z.gameID)
	if err != nil {
		return "", 0, fmt.Errorf("get game model: %w", err)
	}
	if model.Level == nil {
		return "", 0, fmt.Errorf("нет текущего уровня (Event=%v)", model.Event)
	}
	form := url.Values{}
	form.Set("LevelId", strconv.Itoa(model.Level.LevelId))
	form.Set("LevelNumber", strconv.Itoa(model.Level.Number))
	form.Set("LevelAction.Answer", code)
	html, status, err := z.client.PostPage(ctx, z.playURL(), form)
	if err != nil {
		return "", status, fmt.Errorf("send code: %w", err)
	}
	return html, status, nil
}

// PlayPenaltyHint берёт штрафную подсказку (?pid=&pact=) через страницу, как
// ссылка на play-странице, и возвращает HTML ответа.
func (z *Zapolnyaka) PlayPenaltyHint(helpID, pact int) (string, error) {
	z.preparePlay()
	if pact <= 0 {
		pact = 1
	}
	u := fmt.Sprintf("%s?pid=%d&pact=%d", z.playURL(), helpID, pact)
	html, err := z.client.GetPage(context.Background(), u)
	if err != nil {
		return "", fmt.Errorf("penalty hint: %w", err)
	}
	return html, nil
}

package emu

import (
	"strings"

	"zapolnyaka/internal/config"
)

// SectorCodes возвращает записи codes уровня, создающие сектор, в порядке создания.
// Индекс в этом списке = индекс сектора в состоянии уровня.
func SectorCodes(codes []config.Code) []config.Code {
	var out []config.Code
	for _, c := range codes {
		if c.Type.HasSector() {
			out = append(out, c)
		}
	}
	return out
}

// Normalize приводит ответ к виду для сравнения: обрезка пробелов, нижний регистр.
// Движок en.cx сравнивает ответы без учёта регистра (уточняется по реальной игре).
func Normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// AnswerMatches сообщает, совпадает ли введённый ответ с одним из ответов записи.
func AnswerMatches(answer string, answers []string) bool {
	a := Normalize(answer)
	if a == "" {
		return false
	}
	for _, x := range answers {
		if Normalize(x) == a {
			return true
		}
	}
	return false
}

// MatchResult — что закрыл введённый код.
type MatchResult struct {
	Sectors        []int    // индексы секторов, закрытых этим вводом
	Bonuses        []string // ключи бонусов (BonusKey), взятых этим вводом
	AlreadySectors []int    // сектора, уже закрытые ранее, но код к ним подошёл
	AlreadyBonuses []string // бонусы, уже взятые ранее, но код к ним подошёл
}

// Correct — ввод подошёл хотя бы к чему-то (в т.ч. к уже закрытому).
func (m MatchResult) Correct() bool {
	return len(m.Sectors)+len(m.Bonuses)+len(m.AlreadySectors)+len(m.AlreadyBonuses) > 0
}

// Match проверяет ответ одновременно по всем секторам уровня и всем видимым на
// уровне бонусам — как движок без настроенной блокировки ответов: один код может
// закрыть сектор и взять бонус (секторбонус), а также несколько секторов с
// одинаковым ответом.
func Match(answer string, sectors []config.Code, bonuses []config.BonusRef, ls *LevelState, gs *GameState) MatchResult {
	var m MatchResult
	for i, s := range sectors {
		if !AnswerMatches(answer, s.Answers) {
			continue
		}
		if _, done := ls.Sectors[i]; done {
			m.AlreadySectors = append(m.AlreadySectors, i)
		} else {
			m.Sectors = append(m.Sectors, i)
		}
	}
	for _, b := range bonuses {
		if !AnswerMatches(answer, b.Code.Answers) {
			continue
		}
		key := BonusKey(b.OwnerLevel, b.Index)
		if _, done := gs.Bonuses[key]; done {
			m.AlreadyBonuses = append(m.AlreadyBonuses, key)
		} else {
			m.Bonuses = append(m.Bonuses, key)
		}
	}
	return m
}

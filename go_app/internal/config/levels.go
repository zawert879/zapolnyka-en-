package config

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// LevelFilter — разобранная спецификация уровней бонуса (поле `levels` в codes).
//
// Грамматика: термы через запятую, каждый терм — одно из:
//
//	7        — ровно уровень 7
//	1-10     — диапазон включительно
//	>10      — строго больше
//	>=10     — больше или равно
//	<5       — строго меньше
//	<=5      — меньше или равно
//	*, all, все — все уровни игры (rbAllLevels в админке)
//
// Примеры: "1-10", ">10", "2,4,6-8", "<=3, >=12".
type LevelFilter struct {
	All   bool
	terms []levelTerm
}

type levelTerm struct{ lo, hi int } // включительно

const levelMax = math.MaxInt32

// ParseLevelSpec разбирает строку спецификации уровней.
func ParseLevelSpec(spec string) (LevelFilter, error) {
	var f LevelFilter
	if strings.TrimSpace(spec) == "" {
		return f, fmt.Errorf("levels: пустая спецификация уровней")
	}
	for _, raw := range strings.Split(spec, ",") {
		t := strings.TrimSpace(raw)
		if t == "" {
			continue
		}
		switch strings.ToLower(t) {
		case "*", "all", "все":
			f.All = true
			continue
		}
		lo, hi, err := parseLevelTerm(t)
		if err != nil {
			return f, fmt.Errorf("levels %q: %w", t, err)
		}
		f.terms = append(f.terms, levelTerm{lo, hi})
	}
	if !f.All && len(f.terms) == 0 {
		return f, fmt.Errorf("levels %q: нет ни одного условия", spec)
	}
	return f, nil
}

func parseLevelTerm(t string) (lo, hi int, err error) {
	switch {
	case strings.HasPrefix(t, ">="):
		lo, err = parseLevelNum(t[2:])
		hi = levelMax
	case strings.HasPrefix(t, "<="):
		lo = 1
		hi, err = parseLevelNum(t[2:])
	case strings.HasPrefix(t, ">"):
		lo, err = parseLevelNum(t[1:])
		lo++
		hi = levelMax
	case strings.HasPrefix(t, "<"):
		lo = 1
		hi, err = parseLevelNum(t[1:])
		hi--
	case strings.Contains(t, "-"):
		parts := strings.SplitN(t, "-", 2)
		if lo, err = parseLevelNum(parts[0]); err != nil {
			return
		}
		if hi, err = parseLevelNum(parts[1]); err != nil {
			return
		}
		if hi < lo {
			err = fmt.Errorf("конец диапазона меньше начала")
		}
	default:
		lo, err = parseLevelNum(t)
		hi = lo
	}
	return
}

func parseLevelNum(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, fmt.Errorf("ожидался номер уровня")
	}
	if n < 1 {
		return 0, fmt.Errorf("номер уровня должен быть >= 1")
	}
	return n, nil
}

// Matches сообщает, попадает ли уровень с номером n под спецификацию.
func (f LevelFilter) Matches(n int) bool {
	if f.All {
		return true
	}
	for _, t := range f.terms {
		if n >= t.lo && n <= t.hi {
			return true
		}
	}
	return false
}

// Resolve возвращает отсортированный список номеров из numbers, попадающих под спецификацию.
func (f LevelFilter) Resolve(numbers []int) []int {
	var out []int
	for _, n := range numbers {
		if f.Matches(n) {
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

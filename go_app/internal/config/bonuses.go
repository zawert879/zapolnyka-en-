package config

// BonusRef — бонус, видимый на уровне: код и откуда он (уровень-владелец и индекс
// записи в его codes). Ключ OwnerLevel:Index стабилен для мультиуровневых бонусов.
type BonusRef struct {
	OwnerLevel int
	Index      int
	Code       Code
}

// BonusesForLevel возвращает бонусы, которые должны быть видны на уровне levelNum,
// в порядке создания: собственные бонусы уровня плюс бонусы других уровней, чья
// спецификация `levels` покрывает levelNum. Бонус с заданным `levels` играет только
// там, где сказано в спецификации — собственный уровень не подразумевается.
// Выключенный уровень (Disabled) не заливается, поэтому его бонусы на других
// уровнях не появляются; на нём самом (превью/эмулятор) они видны как раньше.
func BonusesForLevel(prepared []PreparedLevel, levelNum int) []BonusRef {
	var out []BonusRef
	for _, p := range prepared {
		if p.Conf == nil {
			continue
		}
		if p.Disabled && p.Conf.Level != levelNum {
			continue
		}
		for i, c := range p.Codes {
			if !c.Type.HasBonus() {
				continue
			}
			ref := BonusRef{OwnerLevel: p.Conf.Level, Index: i, Code: c}
			if c.Levels == nil {
				if p.Conf.Level == levelNum {
					out = append(out, ref)
				}
				continue
			}
			if f, err := ParseLevelSpec(*c.Levels); err == nil && f.Matches(levelNum) {
				out = append(out, ref)
			}
		}
	}
	return out
}

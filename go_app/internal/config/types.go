package config

// CodeType — тип кода на en.cx.
type CodeType string

const (
	CodeTypeSector        CodeType = "сектор"
	CodeTypeBonus         CodeType = "бонус"
	CodeTypePenalty       CodeType = "штраф"
	CodeTypeSectorBonus   CodeType = "секторбонус"
	CodeTypeSectorPenalty CodeType = "секторштраф"
)

// HasSector reports whether the code type creates a sector.
func (ct CodeType) HasSector() bool {
	return ct == CodeTypeSector || ct == CodeTypeSectorBonus || ct == CodeTypeSectorPenalty
}

// HasBonus reports whether the code type creates a bonus/penalty.
func (ct CodeType) HasBonus() bool {
	return ct == CodeTypeBonus || ct == CodeTypePenalty ||
		ct == CodeTypeSectorBonus || ct == CodeTypeSectorPenalty
}

// IsPenalty reports whether the bonus type is a penalty (adds time).
func (ct CodeType) IsPenalty() bool {
	return ct == CodeTypePenalty || ct == CodeTypeSectorPenalty
}

type Hint struct {
	Time int    `yaml:"time" json:"time"`
	Text string `yaml:"text" json:"text"`
}

type PenaltyHint struct {
	Time    int     `yaml:"time"    json:"time"`
	Text    string  `yaml:"text"    json:"text"`
	Penalty *int    `yaml:"penalty,omitempty" json:"penalty,omitempty"`
	Comment *string `yaml:"comment,omitempty" json:"comment,omitempty"`
}

type Code struct {
	Type       CodeType `yaml:"type"       json:"type"`
	SectorName *string  `yaml:"sectorName,omitempty" json:"sectorName,omitempty"`
	BonusName  *string  `yaml:"bonusName,omitempty"  json:"bonusName,omitempty"`
	Answers    []string `yaml:"answers"    json:"answers"`
	Time       *int     `yaml:"time,omitempty"       json:"time,omitempty"`
	Help       *string  `yaml:"help,omitempty"       json:"help,omitempty"`
}

type Level struct {
	Level           int           `yaml:"level"    json:"level"`
	DevLevel        *int          `yaml:"devLevel,omitempty"        json:"devLevel,omitempty"`
	Codes           *string       `yaml:"codes,omitempty"           json:"codes,omitempty"`
	Body            *string       `yaml:"body,omitempty"            json:"body,omitempty"`
	Clean           bool          `yaml:"clean"    json:"clean"`
	Autopass        *int          `yaml:"autopass,omitempty"        json:"autopass,omitempty"`
	AutopassPenalty *int          `yaml:"autopassPenalty,omitempty" json:"autopassPenalty,omitempty"`
	Hints           []Hint        `yaml:"hints,omitempty"           json:"hints,omitempty"`
	PenaltyHints    []PenaltyHint `yaml:"penaltyHints,omitempty"    json:"penaltyHints,omitempty"`
	SectorsToClose  *int          `yaml:"sectorsToClose,omitempty"  json:"sectorsToClose,omitempty"`
	Name            *string       `yaml:"name,omitempty"            json:"name,omitempty"`
	Comment         *string       `yaml:"comment,omitempty"         json:"comment,omitempty"`
}

// DelayRange defines a random delay between Min and Max seconds (inclusive).
// If Max is 0 or <= Min, the delay is exactly Min seconds.
type DelayRange struct {
	Min int `yaml:"min" json:"min"`
	Max int `yaml:"max" json:"max"`
}

// Delays holds per-game request throttling settings.
type Delays struct {
	BetweenCodes  DelayRange `yaml:"betweenCodes"  json:"betweenCodes"`  // between sector/bonus entries
	BetweenHints  DelayRange `yaml:"betweenHints"  json:"betweenHints"`  // between hint/penaltyHint requests
	BetweenLevels DelayRange `yaml:"betweenLevels" json:"betweenLevels"` // after finishing each level
}

// Default returns d with zero fields replaced by sensible defaults.
func (d Delays) Default() Delays {
	if d.BetweenCodes.Min == 0 && d.BetweenCodes.Max == 0 {
		d.BetweenCodes = DelayRange{Min: 2, Max: 4}
	}
	if d.BetweenHints.Min == 0 && d.BetweenHints.Max == 0 {
		d.BetweenHints = DelayRange{Min: 1, Max: 2}
	}
	return d
}

type Game struct {
	Domain        string   `yaml:"domain"        json:"domain"`
	GameID        int      `yaml:"gameId"        json:"gameId"`
	Levels        []string `yaml:"levels"        json:"levels"`
	DefaultFormat string   `yaml:"defaultFormat,omitempty" json:"defaultFormat,omitempty"`
	IsDev         bool     `yaml:"isDev,omitempty"         json:"isDev,omitempty"`
	Delays        Delays   `yaml:"delays,omitempty"        json:"delays,omitempty"`
}

// PreparedLevel holds all data needed to upload one level.
type PreparedLevel struct {
	Conf  *Level
	Codes []Code
	Body  string
}

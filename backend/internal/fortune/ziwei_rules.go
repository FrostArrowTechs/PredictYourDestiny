package fortune

import "errors"

const (
	ZiweiAlgorithmVersion     = "ziwei-provisional-v7"
	ZiweiRuleSetVersion       = "ziwei-provisional-rules-v7"
	ZiweiCalendarVersion      = "lunar-go-v1.4.6"
	ZiweiFourTransformRuleSet = "ziwei-quanji-four-transformations-v1"
	ZiweiLeapMonthAsNext      = "as_next_month-v1"
	ZiweiLeapMonthSplit15     = "split_at_day_15-v1"
	ZiweiFireBellRuleSet      = "ziwei-quanji-fire-bell-v1"
	ZiweiTravelHorseRuleSet   = "ziwei-quanji-travel-horse-v1"
)

var (
	ErrZiweiUnsupportedRuleSet       = errors.New("ziwei: requested rule set is not supported")
	ErrZiweiLeapMonthRuleRequired    = errors.New("ziwei: leap-month rule must be selected explicitly")
	ErrZiweiLeapMonthRuleUnsupported = errors.New("ziwei: leap-month rule is not supported")
)

type ZiweiRulePack struct {
	Version          string   `json:"version"`
	Status           string   `json:"status"`
	CalendarVersion  string   `json:"calendarVersion"`
	SupportedRules   []string `json:"supportedRules"`
	ApproximateRules []string `json:"approximateRules"`
	UnsupportedRules []string `json:"unsupportedRules"`
	Evidence         []string `json:"evidence"`
}

func activeZiweiRulePack() ZiweiRulePack {
	return ZiweiRulePack{
		Version: ZiweiRuleSetVersion, Status: "provisional", CalendarVersion: ZiweiCalendarVersion,
		SupportedRules:   []string{"solar-to-lunar conversion", ZiweiLeapMonthAsNext, ZiweiLeapMonthSplit15, "life and body palace placement", "twelve-palace counterclockwise ordering", "five-element bureau from five-tiger stems and life-palace nayin", "Ziwei placement by bureau quotient and odd/even complement", "fourteen-main-star relative placement", ZiweiFourTransformRuleSet, "left/right assistants by lunar month", "literary stars and earth void/robbery by birth hour", "noble stars and salary/sheep/dharmachakra by birth-year stem", ZiweiFireBellRuleSet, ZiweiTravelHorseRuleSet, "major-luck direction by gender and yin-yang year stem"},
		ApproximateRules: []string{"one full external golden chart passes; broader date, stem, hour and leap-month coverage is still pending"},
		UnsupportedRules: []string{"unselected leap-month convention", "complete minor auxiliary and malefic star set"},
		Evidence:         []string{"lunar-go solar/lunar calendar conversion", "traditional five-tiger stem and sixty-cycle nayin tables", "traditional Ziwei quotient/complement placement method", "Ziwei Doushu Quanji fire-bell and travel-horse formulas", "iztro v2.4.7 documentation full-chart fixture 2000-08-16", "repository-owned placement tables locked by unit and golden tests"},
	}
}

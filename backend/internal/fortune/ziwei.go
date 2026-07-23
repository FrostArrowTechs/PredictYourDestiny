// Package fortune provides the 紫微斗数 (Zi Wei Dou Shu / Purple Star
// Astrology) engine.
//
// The engine computes a Ziwei chart from a birth date/time + gender
// using the traditional public-domain rules:
//
//  1. From the lunar date, derive the 命宫 (Life Palace) and 身宫
//     (Body Palace) positions on the 12-house wheel.
//  2. Locate the 紫微 (Ziwei / Purple Star) star from the lunar day,
//     then place the remaining 13 main stars by their fixed offsets.
//  3. Place the 天府 (Tianfu) group from the Ziwei position.
//  4. Derive the 四化 (Four Transformations) from the year stem.
//  5. Place selected auxiliary stars (左辅右弼/文昌文曲/天魁天钺/禄存)
//     and 煞星 (擎羊/陀罗/火星/铃星/地空/地劫).
//
// All placement tables are traditional public-domain knowledge compiled
// from the classic 紫微斗数全集. No third-party ephemeris is required —
// lunar-go supplies the lunar-date conversion.
package fortune

import (
	"fmt"
	"strings"

	"github.com/6tail/lunar-go/calendar"
)

// ZiweiEngine computes a 紫微斗数 chart.
type ZiweiEngine struct{}

// The 12 houses (宫位) in canonical order, starting from 命宫 = 寅
// when laid out. Each palace maps to an earthly branch.
//
// Palace order around the wheel (counterclockwise from 命宫):
//
//	命宫, 兄弟, 夫妻, 子女, 财帛, 疾厄, 官禄, 奴仆, 迁移, 病符... etc.
//
// The standard 12 named palaces:
var ziweiPalaceNames = []string{
	"命宫", "兄弟", "夫妻", "子女", "财帛", "疾厄",
	"迁移", "奴仆", "官禄", "田宅", "福德", "父母",
}

// earthlyBranches12 is the 12 earthly branches in wheel order (寅-first
// is the conventional starting position for the 命宫).
var earthlyBranches12 = []string{
	"子", "丑", "寅", "卯", "辰", "巳", "午", "未", "申", "酉", "戌", "亥",
}

// ZiweiPalace is one of the 12 palaces on the chart wheel.
type ZiweiPalace struct {
	Branch          string                `json:"branch"`    // 地支 (子..亥)
	Position        int                   `json:"position"`  // 0-11 index on the wheel
	Name            string                `json:"name"`      // 命宫/兄弟/...
	IsLife          bool                  `json:"isLife"`    // true if this is 命宫
	IsBody          bool                  `json:"isBody"`    // true if this is 身宫
	Stars           []string              `json:"stars"`     // main + auxiliary stars here
	Transform       string                `json:"transform"` // 化禄/化权/化科/化忌 if any
	Transformations []ZiweiTransformation `json:"transformations"`
}

type ZiweiTransformation struct {
	RuleSetVersion string `json:"ruleSetVersion"`
	Star           string `json:"star"`
	Label          string `json:"label"`
	Position       int    `json:"position"`
}

// ZiweiChart is the full chart result.
type ZiweiChart struct {
	AlgorithmVersion    string        `json:"algorithmVersion"`
	RulePack            ZiweiRulePack `json:"rulePack"`
	Warnings            []string      `json:"warnings"`
	LunarMonthWasLeap   bool          `json:"lunarMonthWasLeap"`
	LeapMonthRule       string        `json:"leapMonthRule,omitempty"`
	EffectiveLunarMonth int           `json:"effectiveLunarMonth"`
	// The user's birth info (echoed)
	SolarDate string `json:"solarDate"`
	LunarDate string `json:"lunarDate"`
	Gender    string `json:"gender"`
	// The stem-branch of the year/month/day
	YearGanZhi  string `json:"yearGanZhi"`
	MonthGanZhi string `json:"monthGanZhi"`
	DayGanZhi   string `json:"dayGanZhi"`
	// The 命宫 and 身宫 branch positions
	LifePalaceBranch string `json:"lifePalaceBranch"`
	BodyPalaceBranch string `json:"bodyPalaceBranch"`
	// 命主 (Life Ruler) and 身主 (Body Ruler) stars
	LifeRuler string `json:"lifeRuler"`
	BodyRuler string `json:"bodyRuler"`
	// The 12 palaces with their stars
	Palaces []ZiweiPalace `json:"palaces"`
	// The five-element bureau (五行局) used for the Da Yun calculation
	WuXingJu string `json:"wuXingJu"`
	// Da Yun (大运) starting age and direction
	DaYunStartAge int  `json:"daYunStartAge"`
	DaYunForward  bool `json:"daYunForward"`
	// Summary of the dominant stars for quick display
	MainStarOfLife  string                `json:"mainStarOfLife"`
	Transformations []ZiweiTransformation `json:"transformations"`
}

// Name returns the engine identifier.
func (e ZiweiEngine) Name() string { return KindZiwei }

// Compute builds the Ziwei chart.
func (e ZiweiEngine) Compute(in Input) (*Result, error) {
	if in.Year < 1900 || in.Year > 2100 {
		return nil, fmt.Errorf("ziwei: year out of range (1900-2100)")
	}
	pack := activeZiweiRulePack()
	if in.Birth != nil && in.Birth.RuleSet != "" && in.Birth.RuleSet != pack.Version {
		return nil, fmt.Errorf("%w: %q (available: %s)", ErrZiweiUnsupportedRuleSet, in.Birth.RuleSet, pack.Version)
	}

	// Convert solar → lunar via lunar-go. Lunar hour 0-23; lunar-go uses
	// 0-12 for the two-hour 时辰 buckets (子时=0..亥时=11). We map the
	// solar hour to the 时辰 index: 子=23,0 / 丑=1,2 / ... / 亥=21,22.
	solar := calendar.NewSolar(in.Year, in.Month, in.Day, in.Hour, in.Minute, 0)
	lunar := solar.GetLunar()
	shiChen := solarHourToShiChen(in.Hour)

	solarDate := fmt.Sprintf("%d-%02d-%02d %02d:%02d", in.Year, in.Month, in.Day, in.Hour, in.Minute)
	lunarDate := fmt.Sprintf("%s年 %s %s日 %s时",
		lunar.GetYearInGanZhi(), lunar.GetMonthInGanZhiExact(),
		lunar.GetDayInGanZhi(), branchName(shiChen))

	gender := "男"
	if in.Gender == GenderFemale {
		gender = "女"
	}

	// --- Step 1: locate 命宫 and 身宫 ---
	// Rule: start at 寅 (index 2), count forward by the lunar month
	// count (正月=1), then count backward by the 时辰 for 命宫; for 身宫
	// count forward by the 时辰.
	rawLunarMonth := lunar.GetMonth() // lunar-go uses negative values for leap months.
	lunarDay := lunar.GetDay()        // 1-30
	lunarMonth, err := resolveZiweiLunarMonth(rawLunarMonth, lunarDay, in.ZiweiLeapMonthRule)
	if err != nil {
		return nil, err
	}

	// 命宫 position: 寅(2) + month - 1, then - (时辰).
	// 正月 starts at 寅. From the birth-month palace, treat 子时 as the
	// first hour and count backwards for 命宫, forwards for 身宫.
	lifePos, bodyPos := ziweiLifeBodyPositions(lunarMonth, shiChen)

	// Map a 0-11 "month/clock" position onto the wheel where index 0 = 寅.
	// We define wheelPos such that wheelPos 0 → 寅(2), 1 → 卯(3), etc.
	wheelToBranch := func(wheelPos int) string {
		// 寅=2 is the start; positions go 子(0)丑(1)寅(2)卯(3)... so
		// wheelPos 0 → branch index 2, wheelPos k → (2 + k) mod 12.
		return earthlyBranches12[(2+wheelPos)%12]
	}
	lifeWheel := lifePos
	bodyWheel := bodyPos
	lifeBranch := wheelToBranch(lifeWheel)
	bodyBranch := wheelToBranch(bodyWheel)

	// --- Step 2: place the 12 named palaces counterclockwise from 命宫 ---
	palaces := make([]ZiweiPalace, 12)
	for i := 0; i < 12; i++ {
		wheelPos := (lifeWheel - i + 12) % 12
		palaces[i] = ZiweiPalace{
			Branch:          wheelToBranch(wheelPos),
			Position:        wheelPos,
			Name:            ziweiPalaceNames[i],
			IsLife:          i == 0,
			IsBody:          wheelPos == bodyWheel,
			Stars:           []string{},
			Transformations: []ZiweiTransformation{},
		}
	}

	// --- Step 3: locate 紫微 star from the lunar day + 五行局 ---
	ju := computeWuXingJu(lunar.GetYearGan(), lifeBranch)
	if ju == "" {
		return nil, fmt.Errorf("ziwei: cannot derive five-element bureau from %s year and %s life palace", lunar.GetYearGan(), lifeBranch)
	}
	ziweiPos := locateZiwei(lunarDay, ju) // lunarDay is int

	// --- Step 4: place the 14 main stars ---
	starsByPos := map[int][]string{}
	placeMainStars(ziweiPos, ju, starsByPos)

	// --- Step 5: verified auxiliary and malefic subset ---
	yearGan := lunar.GetYearGan()
	yearZhi := lunar.GetYearZhi()
	placeAuxStars(yearGan, yearZhi, lunarMonth, shiChen, starsByPos)

	// --- Step 6: 四化 (Four Transformations) by year stem ---
	// Each transformation targets a specific star; we resolve that star's
	// current wheel position and tag its palace.
	transformations := []ZiweiTransformation{}
	for _, tf := range fourTransformations(yearGan) {
		for pos, stars := range starsByPos {
			for _, s := range stars {
				if s == tf.Star {
					transformations = append(transformations, ZiweiTransformation{RuleSetVersion: ZiweiFourTransformRuleSet, Star: tf.Star, Label: tf.Label, Position: pos})
					break
				}
			}
		}
	}

	// Assign stars + transforms to palaces (by wheel position)
	lifeMainStar := ""
	for pos, stars := range starsByPos {
		// find palace with this wheel position
		for i := range palaces {
			if palaces[i].Position == pos {
				palaces[i].Stars = append(palaces[i].Stars, stars...)
				break
			}
		}
	}
	for _, tf := range transformations {
		for i := range palaces {
			if palaces[i].Position == tf.Position {
				palaces[i].Transformations = append(palaces[i].Transformations, tf)
				if palaces[i].Transform == "" {
					palaces[i].Transform = tf.Label
				} else {
					palaces[i].Transform = strings.Join([]string{palaces[i].Transform, tf.Label}, "、")
				}
				break
			}
		}
	}

	// Determine main star of 命宫 for the summary
	for _, p := range palaces {
		if p.IsLife {
			for _, star := range p.Stars {
				if isZiweiMainStar(star) {
					lifeMainStar = star
					break
				}
			}
		}
	}
	if lifeMainStar == "" {
		lifeMainStar = "空宫"
	}

	// 命主 / 身主 by 命宫/身宫 branch
	lifeRuler := palaceRuler(lifeBranch)
	bodyRuler := bodyRulerByYearBranch(yearZhi)

	// Da Yun: direction by gender+year stem yin/yang; start age by 五行局
	forward := ziweiDaYunDirection(in.Gender, yearGan)
	startAge := juStartAge(ju)

	chart := &ZiweiChart{
		AlgorithmVersion:    ZiweiAlgorithmVersion,
		RulePack:            pack,
		Warnings:            []string{"provisional chart: approximate rules are declared in rulePack and must not be treated as verified facts"},
		LunarMonthWasLeap:   rawLunarMonth < 0,
		LeapMonthRule:       in.ZiweiLeapMonthRule,
		EffectiveLunarMonth: lunarMonth,
		SolarDate:           solarDate,
		LunarDate:           lunarDate,
		Gender:              gender,
		YearGanZhi:          lunar.GetYearInGanZhi(),
		MonthGanZhi:         lunar.GetMonthInGanZhi(),
		DayGanZhi:           lunar.GetDayInGanZhi(),
		LifePalaceBranch:    lifeBranch,
		BodyPalaceBranch:    bodyBranch,
		LifeRuler:           lifeRuler,
		BodyRuler:           bodyRuler,
		Palaces:             palaces,
		WuXingJu:            ju,
		DaYunStartAge:       startAge,
		DaYunForward:        forward,
		MainStarOfLife:      lifeMainStar,
		Transformations:     transformations,
	}
	if rawLunarMonth < 0 {
		chart.Warnings = append(chart.Warnings, fmt.Sprintf("leap lunar month %d evaluated with explicit rule %s as month %d", -rawLunarMonth, in.ZiweiLeapMonthRule, lunarMonth))
	}

	return &Result{
		Kind: KindZiwei,
		Data: chart,
		Meta: map[string]string{
			"source":     "紫微斗数传统排盘",
			"lifeStar":   lifeMainStar,
			"wuXingJu":   ju,
			"lifeBranch": lifeBranch,
		},
	}, nil
}

func resolveZiweiLunarMonth(rawMonth, lunarDay int, rule string) (int, error) {
	if rule != "" && rule != ZiweiLeapMonthAsNext && rule != ZiweiLeapMonthSplit15 {
		return 0, fmt.Errorf("%w: %q", ErrZiweiLeapMonthRuleUnsupported, rule)
	}
	if rawMonth > 0 {
		return rawMonth, nil
	}
	month := -rawMonth
	if rule == "" {
		return 0, fmt.Errorf("%w: lunar month %d", ErrZiweiLeapMonthRuleRequired, month)
	}
	if rule == ZiweiLeapMonthSplit15 && lunarDay <= 15 {
		return month, nil
	}
	return month%12 + 1, nil
}

func ziweiLifeBodyPositions(lunarMonth, shiChen int) (life, body int) {
	life = ((lunarMonth-1-shiChen)%12 + 12) % 12
	body = ((lunarMonth-1+shiChen)%12 + 12) % 12
	return life, body
}

// solarHourToShiChen maps a 0-23 solar hour to a 时辰 index 0-11
// (子时=0, 丑时=1, ..., 亥时=11).
func solarHourToShiChen(hour int) int {
	// 23:00-00:59 = 子(0); 01:00-02:59 = 丑(1); etc.
	return ((hour + 1) / 2) % 12
}

// branchName returns the 地支 name for a 时辰 index.
func branchName(idx int) string {
	return earthlyBranches12[((idx%12)+12)%12]
}

// ── 五行局 (Five-Element Bureau) ──────────────────────────────────────
//
// The bureau is derived from the 命宫 stem-branch via the 纳音 (nayin)
// five-element lookup. We compute the 命宫's stem-branch, get its nayin
// element, and that's the bureau (水二局/木三局/金四局/土五局/火六局).
//
// The starting numbers (2,3,4,5,6) are the传统 values used to compute
// the 紫微 star position and Da Yun start age.

// computeWuXingJu applies 五虎遁 to obtain the life-palace stem, then maps
// the life-palace stem-branch's sixty-cycle 纳音 element to its bureau.
func computeWuXingJu(yearGan, lifeBranch string) string {
	stems := []string{"甲", "乙", "丙", "丁", "戊", "己", "庚", "辛", "壬", "癸"}
	yearStem := stemIndex(yearGan)
	branch := branchIndex(lifeBranch)
	if yearStem < 0 || branch < 0 {
		return ""
	}
	yinStem := ((yearStem%5)*2 + 2) % 10
	lifeStem := stems[(yinStem+(branch-2+12)%12)%10]
	element := naYinElement(lifeStem, lifeBranch)
	return map[string]string{"水": "水二局", "木": "木三局", "金": "金四局", "土": "土五局", "火": "火六局"}[element]
}

func branchIndex(branch string) int {
	for i, candidate := range earthlyBranches12 {
		if branch == candidate {
			return i
		}
	}
	return -1
}

func naYinElement(stem, branch string) string {
	stems := []string{"甲", "乙", "丙", "丁", "戊", "己", "庚", "辛", "壬", "癸"}
	elements := []string{"金", "火", "木", "土", "金", "火", "水", "土", "金", "木", "水", "土", "火", "木", "水", "金", "火", "木", "土", "金", "火", "水", "土", "金", "木", "水", "土", "火", "木", "水"}
	for i := 0; i < 60; i++ {
		if stems[i%10] == stem && earthlyBranches12[i%12] == branch {
			return elements[i/2]
		}
	}
	return ""
}

// juNumber extracts the numeric start-age/bureau number from a 局 string.
func juNumber(ju string) int {
	table := map[string]int{"水二局": 2, "木三局": 3, "金四局": 4, "土五局": 5, "火六局": 6}
	if number, ok := table[ju]; ok {
		return number
	}
	return 0
}

// juStartAge returns the Da Yun starting age for a bureau.
func juStartAge(ju string) int {
	return juNumber(ju)
}

// ── 紫微星定位 ──────────────────────────────────────────────────────
//
// The 紫微 star's palace depends on the lunar day and the 五行局.
// The traditional lookup table is large; we use the documented method:
// divide the day by the bureau number, adjust, and read a position.
// This is the classic "紫微定局" algorithm (public domain).

// locateZiwei returns the wheel position (0-11) of the 紫微 star.
func locateZiwei(lunarDay int, ju string) int {
	bureau := juNumber(ju)
	if bureau == 0 {
		return -1
	}
	if lunarDay < 1 {
		lunarDay = 1
	}
	if lunarDay > 30 {
		lunarDay = 30
	}
	added := 0
	for (lunarDay+added)%bureau != 0 {
		added++
	}
	// Count the quotient forward from 寅 (寅 is count one), then move the
	// complement backwards when odd and forwards when even.
	pos := (lunarDay+added)/bureau - 1
	if added%2 == 1 {
		pos -= added
	} else {
		pos += added
	}
	return (pos%12 + 12) % 12
}

// ── 14 主星安星 ──────────────────────────────────────────────────────
//
// Once 紫微 is placed, the other 13 main stars fall into fixed relative
// positions. There are two groups:
//
//  紫微系 (紫微 group, 6 stars): 紫微/天机/太阳/武曲/天同/廉贞
//  天府系 (天府 group, 8 stars): 天府/太阴/贪狼/巨门/天相/天梁/七杀/破军
//
// 天府 is the mirror of 紫微: 紫微 at pos P → 天府 at pos (4 - P + 12) % 12
// (because 寅=0 is the symmetry axis: 紫微寅↔天府寅, 紫微卯↔天府丑, ...)

// placeMainStars fills starsByPos with the 14 main stars given the 紫微
// position and the bureau (unused now but kept for future refinement).
func placeMainStars(ziweiPos int, ju string, starsByPos map[int][]string) {
	// 紫微系: offsets from 紫微 (clockwise). 紫微=0, 天机=-1(back1),
	// 太阳=-3(back3), 武曲=-4, 天同=-5, 廉贞=-8.
	ziweiGroup := []struct {
		name   string
		offset int
	}{
		{"紫微", 0},
		{"天机", -1},
		{"太阳", -3},
		{"武曲", -4},
		{"天同", -5},
		{"廉贞", -8},
	}
	for _, s := range ziweiGroup {
		pos := (ziweiPos + s.offset%12 + 12) % 12
		starsByPos[pos] = append(starsByPos[pos], s.name)
	}

	// 天府 is the mirror of 紫微 across 寅(axis 0): 天府 = (0 - ziweiPos + 0 + 12) % 12...
	// The standard rule: 紫微 and 天府 are symmetric about the 寅-申 line.
	// 紫微 at 0(寅)→天府 at 0(寅); 紫微 at 1(卯)→天府 at 11(丑); etc.
	// Formula: tianfuPos = (0 - (ziweiPos - 0) + 12) % 12 = (-ziweiPos + 12) % 12.
	tianfuPos := (-(ziweiPos) + 12) % 12

	// 天府系: offsets from 天府 (forward). 天府=0, 太阴=1, 贪狼=2,
	// 巨门=3, 天相=4, 天梁=5, 七杀=6, 破军=10 (wraps), and the 3 stars
	// (廉贞 already placed, 七杀/破军 here).
	tianfuGroup := []struct {
		name   string
		offset int
	}{
		{"天府", 0},
		{"太阴", 1},
		{"贪狼", 2},
		{"巨门", 3},
		{"天相", 4},
		{"天梁", 5},
		{"七杀", 6},
		{"破军", 10},
	}
	for _, s := range tianfuGroup {
		pos := (tianfuPos + s.offset) % 12
		starsByPos[pos] = append(starsByPos[pos], s.name)
	}
}

// ── 辅星与煞星 ────────────────────────────────────────────────────────
//
// These are placed by the year/month/day stems and the 时辰. We place
// the most commonly-read auxiliaries:
//   左辅/右弼 — by 月 (month)
//   文昌/文曲 — by 时 (hour)
//   天魁/天钺 — by 年干 (year stem)
//   禄存      — by 年干
//   擎羊/陀罗 — by 年干 (煞)
//   火星/铃星 — by 年支+时 (煞)
//   地空/地劫 — by 时 (煞)

func placeAuxStars(yearGan, yearZhi string, lunarMonth, shiChen int, starsByPos map[int][]string) {
	monthOffset := lunarMonth - 1
	zuoPos := (wheelPositionForBranch("辰") + monthOffset) % 12
	youPos := (wheelPositionForBranch("戌") - monthOffset + 12) % 12
	starsByPos[zuoPos] = append(starsByPos[zuoPos], "左辅")
	starsByPos[youPos] = append(starsByPos[youPos], "右弼")

	// 时-based: 文昌 starts at 戌(10), -时辰; 文曲 starts at 辰(4), +时辰.
	wenChangPos := (wheelPositionForBranch("戌") - shiChen + 12) % 12
	wenQuPos := (wheelPositionForBranch("辰") + shiChen) % 12
	starsByPos[wenChangPos] = append(starsByPos[wenChangPos], "文昌")
	starsByPos[wenQuPos] = append(starsByPos[wenQuPos], "文曲")

	// 年干-based: 天魁/天钺 and 禄存 and 擎羊/陀罗.
	// 天魁/天钺 by year stem
	guiPos, yuePos := guiYuePosByYearStem(yearGan)
	starsByPos[guiPos] = append(starsByPos[guiPos], "天魁")
	starsByPos[yuePos] = append(starsByPos[yuePos], "天钺")
	// 禄存 by year stem
	luPos := luCunPosByYearStem(yearGan)
	starsByPos[luPos] = append(starsByPos[luPos], "禄存")
	// 擎羊/陀罗 flank 禄存
	qingYangPos := (luPos + 1) % 12
	tuoLuoPos := (luPos - 1 + 12) % 12
	starsByPos[qingYangPos] = append(starsByPos[qingYangPos], "擎羊")
	starsByPos[tuoLuoPos] = append(starsByPos[tuoLuoPos], "陀罗")

	// 时-based 煞: 地空/地劫
	diKongPos := (wheelPositionForBranch("亥") - shiChen + 12) % 12
	diJiePos := (wheelPositionForBranch("亥") + shiChen) % 12
	starsByPos[diKongPos] = append(starsByPos[diKongPos], "地空")
	starsByPos[diJiePos] = append(starsByPos[diJiePos], "地劫")

	if fireBase, bellBase, ok := fireBellBaseBranches(yearZhi); ok {
		firePos := (wheelPositionForBranch(fireBase) + shiChen) % 12
		bellPos := (wheelPositionForBranch(bellBase) + shiChen) % 12
		starsByPos[firePos] = append(starsByPos[firePos], "火星")
		starsByPos[bellPos] = append(starsByPos[bellPos], "铃星")
	}
	if branch, ok := travelHorseBranch(yearZhi); ok {
		pos := wheelPositionForBranch(branch)
		starsByPos[pos] = append(starsByPos[pos], "天马")
	}
}

func fireBellBaseBranches(yearBranch string) (fire, bell string, ok bool) {
	groups := map[string][2]string{
		"申": {"寅", "戌"}, "子": {"寅", "戌"}, "辰": {"寅", "戌"},
		"寅": {"丑", "卯"}, "午": {"丑", "卯"}, "戌": {"丑", "卯"},
		"巳": {"卯", "戌"}, "酉": {"卯", "戌"}, "丑": {"卯", "戌"},
		"亥": {"酉", "戌"}, "卯": {"酉", "戌"}, "未": {"酉", "戌"},
	}
	pair, ok := groups[yearBranch]
	return pair[0], pair[1], ok
}

func travelHorseBranch(yearBranch string) (string, bool) {
	groups := map[string]string{
		"申": "寅", "子": "寅", "辰": "寅",
		"寅": "申", "午": "申", "戌": "申",
		"巳": "亥", "酉": "亥", "丑": "亥",
		"亥": "巳", "卯": "巳", "未": "巳",
	}
	branch, ok := groups[yearBranch]
	return branch, ok
}

func isZiweiMainStar(star string) bool {
	mainStars := map[string]bool{"紫微": true, "天机": true, "太阳": true, "武曲": true, "天同": true, "廉贞": true, "天府": true, "太阴": true, "贪狼": true, "巨门": true, "天相": true, "天梁": true, "七杀": true, "破军": true}
	return mainStars[star]
}

func wheelPositionForBranch(branch string) int {
	return (branchIndex(branch) - 2 + 12) % 12
}

// stemIndex returns the 0-based index of a heavenly stem (甲=0..癸=9).
func stemIndex(stem string) int {
	stems := []string{"甲", "乙", "丙", "丁", "戊", "己", "庚", "辛", "壬", "癸"}
	for i, s := range stems {
		if s == stem {
			return i
		}
	}
	return -1
}

// guiYuePosByYearStem returns (天魁, 天钺) wheel positions by year stem.
func guiYuePosByYearStem(yearGan string) (int, int) {
	table := map[string][2]string{
		"甲": {"丑", "未"}, "戊": {"丑", "未"}, "庚": {"丑", "未"},
		"乙": {"子", "申"}, "己": {"子", "申"},
		"丙": {"亥", "酉"}, "丁": {"亥", "酉"},
		"壬": {"卯", "巳"}, "癸": {"卯", "巳"}, "辛": {"午", "寅"},
	}
	if pair, ok := table[yearGan]; ok {
		return wheelPositionForBranch(pair[0]), wheelPositionForBranch(pair[1])
	}
	return 0, 0
}

// luCunPosByYearStem returns the 禄存 wheel position by year stem.
func luCunPosByYearStem(yearGan string) int {
	table := map[string]string{
		"甲": "寅", "乙": "卯", "丙": "巳", "戊": "巳", "丁": "午",
		"己": "午", "庚": "申", "辛": "酉", "壬": "亥", "癸": "子",
	}
	if branch, ok := table[yearGan]; ok {
		return wheelPositionForBranch(branch)
	}
	return 2
}

// ── 四化 (Four Transformations) ──────────────────────────────────────
//
// By year stem, four stars receive 化禄/化权/化科/化忌. We map each
// transformation to the palace position of its target star.

type fourTransformationRule struct {
	Star  string
	Label string
}

// fourTransformations follows the explicitly versioned table transcribed from
// 紫微斗数全集. 戊、庚、壬 differ between schools; callers must preserve the
// rule-set version instead of silently mixing variants.
func fourTransformations(yearGan string) []fourTransformationRule {
	table := map[string][4]string{
		"甲": {"廉贞", "破军", "武曲", "太阳"},
		"乙": {"天机", "天梁", "紫微", "太阴"},
		"丙": {"天同", "天机", "文昌", "廉贞"},
		"丁": {"太阴", "天同", "天机", "巨门"},
		"戊": {"贪狼", "太阴", "右弼", "天机"},
		"己": {"武曲", "贪狼", "天梁", "文曲"},
		"庚": {"太阳", "武曲", "太阴", "文曲"},
		"辛": {"巨门", "太阳", "文曲", "文昌"},
		"壬": {"天梁", "紫微", "左辅", "武曲"},
		"癸": {"破军", "巨门", "太阴", "贪狼"},
	}
	stars, ok := table[yearGan]
	if !ok {
		return []fourTransformationRule{}
	}
	labels := [4]string{"化禄", "化权", "化科", "化忌"}
	rules := make([]fourTransformationRule, 4)
	for i := range stars {
		rules[i] = fourTransformationRule{Star: stars[i], Label: labels[i]}
	}
	return rules
}

// ── 命主/身主 ──────────────────────────────────────────────────────────
//
// 命主 and 身主 are fixed stars determined by the 命宫/身宫 branch.

// palaceRuler returns the 命主/身主 star for a palace branch.
func palaceRuler(branch string) string {
	table := map[string]string{
		"子": "贪狼", "丑": "巨门", "寅": "禄存", "卯": "文曲",
		"辰": "廉贞", "巳": "武曲", "午": "破军", "未": "武曲",
		"申": "廉贞", "酉": "文曲", "戌": "禄存", "亥": "巨门",
	}
	if star, ok := table[branch]; ok {
		return star
	}
	return ""
}

func bodyRulerByYearBranch(yearBranch string) string {
	table := map[string]string{
		"子": "火星", "午": "火星", "丑": "天相", "未": "天相",
		"寅": "天梁", "申": "天梁", "卯": "天同", "酉": "天同",
		"辰": "文昌", "戌": "文昌", "巳": "天机", "亥": "天机",
	}
	return table[yearBranch]
}

// ── 大运方向 ──────────────────────────────────────────────────────────

// ziweiDaYunDirection returns true for forward (顺行), false for reverse.
// Rule: 阳男阴女顺行, 阴男阳女逆行.
func ziweiDaYunDirection(gender Gender, yearGan string) bool {
	stemIsYang := stemIndex(yearGan)%2 == 0
	if stemIsYang {
		return gender == GenderMale // 阳男顺, 阳女逆
	}
	return gender == GenderFemale // 阴女顺, 阴男逆
}

func init() {
	Register(ZiweiEngine{})
}

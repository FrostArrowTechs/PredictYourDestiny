// Package fortune provides the 梅花易数 (Plum Blossom Numerology) engine.
//
// 梅花易数 is a divination method attributed to 邵雍 (Shao Yong, Song
// Dynasty). It uses numbers (from time or user input) to construct a
// hexagram (卦), then analyzes the 体/用 (body/use) relationship between
// the upper and lower trigrams and the changing line.
//
// All rules below are public-domain traditional content from
// 《梅花易数》. The engine is pure calculation; AI adds interpretation.
package fortune

import (
	"fmt"
	"time"

	"github.com/6tail/lunar-go/calendar"
)

// PlumFlowerEngine computes 梅花易数 hexagrams.
type PlumFlowerEngine struct{}

// PlumFlowerChart is the structured result.
type PlumFlowerChart struct {
	// Method used: "time" or "number"
	Method string `json:"method"`

	// Original hexagram (本卦)
	Original Hexagram `json:"original"`
	// Mutual hexagram (互卦)
	Mutual Hexagram `json:"mutual"`
	// Changed hexagram (变卦)
	Changed Hexagram `json:"changed"`

	// The changing line position (1-6, bottom to top)
	ChangingLine int `json:"changingLine"`

	// Body trigram (体) and Use trigram (用)
	// In the original hexagram, if changing line is in upper trigram,
	// lower trigram is 体 (body); if in lower, upper is 体.
	BodyTrigram string `json:"bodyTrigram"` // 体卦
	UseTrigram  string `json:"useTrigram"`  // 用卦

	// Body and Use five elements
	BodyWuXing string `json:"bodyWuXing"`
	UseWuXing  string `json:"useWuXing"`

	// Relationship: 体生用/用生体/体克用/用克体/体用比和
	Relationship string `json:"relationship"`
	// Fortune trend based on relationship: 吉/凶/平
	Trend string `json:"trend"`
	// Brief analysis
	Analysis string `json:"analysis"`
}

// Hexagram represents a six-line hexagram.
type Hexagram struct {
	Name      string `json:"name"`      // e.g. "乾为天"
	UpperTrig string `json:"upperTrig"` // e.g. "乾"
	LowerTrig string `json:"lowerTrig"` // e.g. "乾"
	UpperWX   string `json:"upperWX"`   // upper trigram five element
	LowerWX   string `json:"lowerWX"`   // lower trigram five element
	// Lines from bottom to top (1=yang, 0=yin)
	Lines [6]int `json:"lines"`
}

// 8 trigrams (八卦) - indexed by their binary value (0-7)
// bit2=bottom line, bit1=middle, bit0=top (乾=111=7, 坤=000=0)
var trigramNames = [8]string{"坤", "震", "坎", "兑", "艮", "离", "巽", "乾"}
var trigramWX = [8]string{"土", "木", "水", "金", "土", "火", "木", "金"}

// 64 hexagrams lookup: name indexed by [upper][lower]
var hexagramNames = [8][8]string{
	// upper = 坤(0)
	{"坤为地", "地雷复", "地水师", "地泽临", "地山谦", "地火明夷", "地风升", "地天泰"},
	// upper = 震(1)
	{"雷地豫", "震为雷", "雷水解", "雷泽归妹", "雷山小过", "雷火丰", "雷风恒", "雷天大壮"},
	// upper = 坎(2)
	{"水地比", "水雷屯", "坎为水", "水泽节", "水山蹇", "水火既济", "水风井", "水天需"},
	// upper = 兑(3)
	{"泽地萃", "泽雷随", "泽水困", "兑为泽", "泽山咸", "泽火革", "泽风大过", "泽天夬"},
	// upper = 艮(4)
	{"山地剥", "山雷颐", "山水蒙", "山泽损", "艮为山", "山火贲", "山风蛊", "山天大畜"},
	// upper = 离(5)
	{"火地晋", "火雷噬嗑", "火水未济", "火泽睽", "火山旅", "离为火", "火风鼎", "火天大有"},
	// upper = 巽(6)
	{"风地观", "风雷益", "风水涣", "风泽中孚", "风山渐", "风火家人", "巽为风", "风天小畜"},
	// upper = 乾(7)
	{"天地否", "天雷无妄", "天水讼", "天泽履", "天山遁", "天火同人", "天风姤", "乾为天"},
}

// Name returns the engine identifier.
func (e PlumFlowerEngine) Name() string { return KindPlumFlower }

// Compute constructs hexagrams from either time or user-supplied numbers.
// If Input.Question contains 3 comma-separated numbers, use number method;
// otherwise use the time method (年月日时 → 上卦下卦动爻).
func (e PlumFlowerEngine) Compute(in Input) (*Result, error) {
	if in.Year < 1900 || in.Year > 2100 {
		// For time-based divination, default to current time
		now := time.Now()
		in.Year = now.Year()
		in.Month = int(now.Month())
		in.Day = now.Day()
		in.Hour = now.Hour()
	}

	// Try to parse 3 numbers from Question (number method)
	nums, ok := parseThreeNumbers(in.Question)
	var chart *PlumFlowerChart
	if ok {
		chart = computeByNumbers(nums)
	} else {
		chart = computeByTime(in.Year, in.Month, in.Day, in.Hour)
	}

	return &Result{
		Kind: KindPlumFlower,
		Data: chart,
		Meta: map[string]string{
			"method": chart.Method,
			"source": "邵雍梅花易数",
		},
	}, nil
}

func init() {
	Register(PlumFlowerEngine{})
}

// computeByTime uses the traditional time-based casting method.
// 上卦 = (年数 + 月数 + 日数) % 8
// 下卦 = (年数 + 月数 + 日数 + 时数) % 8
// 动爻 = (年数 + 月数 + 日数 + 时数) % 6
// 年数 = 地支序号 (子=1..亥=12)
func computeByTime(year, month, day, hour int) *PlumFlowerChart {
	solar := calendar.NewSolar(year, month, day, hour, 0, 0)
	lunar := solar.GetLunar()

	yearNum := lunar.GetYearZhiIndex() + 1 // 1-12
	monthNum := month                      // 1-12
	dayNum := day                          // 1-31
	// 时辰 index (子=1..亥=12)
	hourZhi := getHourZhiIndex(hour) + 1

	upperSum := yearNum + monthNum + dayNum
	lowerSum := upperSum + hourZhi
	changeSum := lowerSum

	upperIdx := upperSum % 8
	lowerIdx := lowerSum % 8
	if upperIdx == 0 {
		upperIdx = 8
	}
	if lowerIdx == 0 {
		lowerIdx = 8
	}
	changingLine := changeSum % 6
	if changingLine == 0 {
		changingLine = 6
	}

	upperTrig := upperIdx - 1 // 0-7 index
	lowerTrig := lowerIdx - 1

	return buildChart("time", upperTrig, lowerTrig, changingLine)
}

// computeByNumbers uses the number-based casting method.
// 上卦 = num1 % 8, 下卦 = num2 % 8, 动爻 = (num1+num2+num3) % 6
func computeByNumbers(nums [3]int) *PlumFlowerChart {
	upperIdx := nums[0] % 8
	lowerIdx := nums[1] % 8
	if upperIdx == 0 {
		upperIdx = 8
	}
	if lowerIdx == 0 {
		lowerIdx = 8
	}
	changingLine := (nums[0] + nums[1] + nums[2]) % 6
	if changingLine == 0 {
		changingLine = 6
	}

	return buildChart("number", upperIdx-1, lowerIdx-1, changingLine)
}

// buildChart constructs the full chart from upper/lower trigram indices
// and the changing line position.
func buildChart(method string, upperTrig, lowerTrig, changingLine int) *PlumFlowerChart {
	// Build original hexagram lines: lower trigram = lines 1-3, upper = 4-6
	var origLines [6]int
	origLines[0] = lowerTrig & 4 >> 2
	origLines[1] = lowerTrig & 2 >> 1
	origLines[2] = lowerTrig & 1
	origLines[3] = upperTrig & 4 >> 2
	origLines[4] = upperTrig & 2 >> 1
	origLines[5] = upperTrig & 1

	original := Hexagram{
		Name:      hexagramNames[upperTrig][lowerTrig],
		UpperTrig: trigramNames[upperTrig],
		LowerTrig: trigramNames[lowerTrig],
		UpperWX:   trigramWX[upperTrig],
		LowerWX:   trigramWX[lowerTrig],
		Lines:     origLines,
	}

	// Mutual hexagram: lines 2,3,4 of original = lower trigram;
	// lines 3,4,5 of original = upper trigram
	mutualLower := origLines[1]<<2 | origLines[2]<<1 | origLines[3]
	mutualUpper := origLines[2]<<2 | origLines[3]<<1 | origLines[4]
	var mutualLines [6]int
	mutualLines[0] = mutualLower & 4 >> 2
	mutualLines[1] = mutualLower & 2 >> 1
	mutualLines[2] = mutualLower & 1
	mutualLines[3] = mutualUpper & 4 >> 2
	mutualLines[4] = mutualUpper & 2 >> 1
	mutualLines[5] = mutualUpper & 1
	mutual := Hexagram{
		Name:      hexagramNames[mutualUpper][mutualLower],
		UpperTrig: trigramNames[mutualUpper],
		LowerTrig: trigramNames[mutualLower],
		UpperWX:   trigramWX[mutualUpper],
		LowerWX:   trigramWX[mutualLower],
		Lines:     mutualLines,
	}

	// Changed hexagram: flip the changing line
	changedLines := origLines
	lineIdx := changingLine - 1
	changedLines[lineIdx] = 1 - changedLines[lineIdx]
	// Recompute trigram indices for changed hexagram
	changedLower := changedLines[0]<<2 | changedLines[1]<<1 | changedLines[2]
	changedUpper := changedLines[3]<<2 | changedLines[4]<<1 | changedLines[5]
	changed := Hexagram{
		Name:      hexagramNames[changedUpper][changedLower],
		UpperTrig: trigramNames[changedUpper],
		LowerTrig: trigramNames[changedLower],
		UpperWX:   trigramWX[changedUpper],
		LowerWX:   trigramWX[changedLower],
		Lines:     changedLines,
	}

	// Determine 体 (body) and 用 (use):
	// The trigram containing the changing line is 用 (use); the other is 体 (body).
	// Changing line 1-3 → lower trigram is 用, upper is 体
	// Changing line 4-6 → upper trigram is 用, lower is 体
	var bodyTrig, useTrig string
	var bodyWX, useWX string
	if changingLine <= 3 {
		// lower is 用, upper is 体
		bodyTrig = original.UpperTrig
		bodyWX = original.UpperWX
		useTrig = original.LowerTrig
		useWX = original.LowerWX
	} else {
		// upper is 用, lower is 体
		bodyTrig = original.LowerTrig
		bodyWX = original.LowerWX
		useTrig = original.UpperTrig
		useWX = original.UpperWX
	}

	rel, trend, analysis := analyzeBodyUse(bodyWX, useWX)

	return &PlumFlowerChart{
		Method:       method,
		Original:     original,
		Mutual:       mutual,
		Changed:      changed,
		ChangingLine: changingLine,
		BodyTrigram:  bodyTrig,
		UseTrigram:   useTrig,
		BodyWuXing:   bodyWX,
		UseWuXing:    useWX,
		Relationship: rel,
		Trend:        trend,
		Analysis:     analysis,
	}
}

// analyzeBodyUse determines the fortune based on the five-element
// relationship between 体 (body) and 用 (use).
//
// 相生 (generating): 木→火→土→金→水→木
// 相克 (overcoming): 木→土→水→火→金→木
func analyzeBodyUse(body, use string) (rel, trend, analysis string) {
	if body == use {
		return "体用比和", "吉", fmt.Sprintf("体(%s)用(%s)同属一行，比和互助，凡事顺遂。", body, use)
	}

	generating := map[string]string{"木": "火", "火": "土", "土": "金", "金": "水", "水": "木"}
	overcoming := map[string]string{"木": "土", "土": "水", "水": "火", "火": "金", "金": "木"}

	if generating[body] == use {
		return "体生用", "平", fmt.Sprintf("体(%s)生用(%s)，泄体之气，主事难成、耗损，宜守不宜进。", body, use)
	}
	if generating[use] == body {
		return "用生体", "吉", fmt.Sprintf("用(%s)生体(%s)，有扶助之意，主有贵人相助、事易成。", use, body)
	}
	if overcoming[body] == use {
		return "体克用", "吉", fmt.Sprintf("体(%s)克用(%s)，体强用弱，事可成但需费力。", body, use)
	}
	if overcoming[use] == body {
		return "用克体", "凶", fmt.Sprintf("用(%s)克体(%s)，体受克制，诸事不顺，宜谨慎防备。", use, body)
	}
	return "体用无关", "平", "体用关系中性。"
}

// parseThreeNumbers tries to parse 3 comma/space-separated numbers
// from a string. Returns ok=false if parsing fails.
func parseThreeNumbers(s string) ([3]int, bool) {
	var nums [3]int
	n, err := fmt.Sscanf(s, "%d,%d,%d", &nums[0], &nums[1], &nums[2])
	if err == nil && n == 3 {
		return nums, true
	}
	n, err = fmt.Sscanf(s, "%d %d %d", &nums[0], &nums[1], &nums[2])
	if err == nil && n == 3 {
		return nums, true
	}
	return nums, false
}

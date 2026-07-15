// Package fortune provides the zodiac (生肖运势) fortune engine.
//
// The engine uses traditional Chinese zodiac relationships to calculate
// fortune scores based on the user's zodiac animal and the current year.
// This is heuristic calculation based on public-domain traditional rules,
// combined with AI interpretation for personalized readings.
//
// Traditional relationships used:
// - 三合 (Three Harmony): mutual support, highest fortune boost
// - 六合 (Six Harmony): complementary, good fortune boost
// - 相冲 (Clash): opposition, fortune challenge
// - 相害 (Harm): minor conflict, slight fortune reduction
package fortune

import (
	"fmt"
	"time"

	"github.com/6tail/lunar-go/calendar"
)

// ZodiacEngine computes fortune based on zodiac relationships.
type ZodiacEngine struct{}

// ZodiacChart is the structured result for zodiac fortune.
type ZodiacChart struct {
	// User's zodiac animal
	Zodiac string `json:"zodiac"`
	// Current year
	Year int `json:"year"`
	// Current year's earthly branch (流年地支)
	LiuNianZhi string `json:"liuNianZhi"`
	// Current year's zodiac animal
	LiuNianZodiac string `json:"liuNianZodiac"`

	// Fortune scores (1-100)
	OverallScore  int `json:"overallScore"`
	CareerScore   int `json:"careerScore"`
	WealthScore   int `json:"wealthScore"`
	LoveScore     int `json:"loveScore"`
	HealthScore   int `json:"healthScore"`

	// Relationship with current year
	Relations []ZodiacRelation `json:"relations"`

	// Lucky elements
	LuckyColors  []string `json:"luckyColors"`
	LuckyNumbers []int    `json:"luckyNumbers"`
	LuckyDir     string   `json:"luckyDir"`

	// Tips and warnings
	Tips    []string `json:"tips"`
	Warns   []string `json:"warns"`
}

// ZodiacRelation describes the relationship between two zodiac signs.
type ZodiacRelation struct {
	Type   string `json:"type"`   // 三合/六合/相冲/相害/平
	With   string `json:"with"`   // Related zodiac animal
	Effect string `json:"effect"` // Effect description
}

// 12 zodiac animals in order
var zodiacAnimals = []string{
	"鼠", "牛", "虎", "兔", "龙", "蛇",
	"马", "羊", "猴", "鸡", "狗", "猪",
}

// 12 earthly branches
var earthlyBranches = []string{
	"子", "丑", "寅", "卯", "辰", "巳",
	"午", "未", "申", "酉", "戌", "亥",
}

// 三合 - Three Harmony groups
var threeHarmony = [][]string{
	{"鼠", "龙", "猴"}, // 子辰申 - 水局
	{"牛", "蛇", "鸡"}, // 丑巳酉 - 金局
	{"虎", "马", "狗"}, // 寅午戌 - 火局
	{"兔", "羊", "猪"}, // 卯未亥 - 木局
}

// 六合 - Six Harmony pairs
var sixHarmony = map[string]string{
	"鼠": "牛", "牛": "鼠",
	"虎": "猪", "猪": "虎",
	"兔": "狗", "狗": "兔",
	"龙": "鸡", "鸡": "龙",
	"蛇": "猴", "猴": "蛇",
	"马": "羊", "羊": "马",
}

// 相冲 - Clash pairs
var clash = map[string]string{
	"鼠": "马", "马": "鼠",
	"牛": "羊", "羊": "牛",
	"虎": "猴", "猴": "虎",
	"兔": "鸡", "鸡": "兔",
	"龙": "狗", "狗": "龙",
	"蛇": "猪", "猪": "蛇",
}

// 相害 - Harm pairs
var harm = map[string]string{
	"鼠": "羊", "羊": "鼠",
	"牛": "马", "马": "牛",
	"虎": "蛇", "蛇": "虎",
	"兔": "龙", "龙": "兔",
	"猴": "猪", "猪": "猴",
	"鸡": "狗", "狗": "鸡",
}

// Lucky colors by zodiac
var luckyColors = map[string][]string{
	"鼠": {"蓝", "黑", "白"},
	"牛": {"黄", "红", "紫"},
	"虎": {"蓝", "灰", "白"},
	"兔": {"粉", "绿", "蓝"},
	"龙": {"金", "银", "白"},
	"蛇": {"黄", "棕", "绿"},
	"马": {"红", "紫", "粉"},
	"羊": {"绿", "红", "紫"},
	"猴": {"白", "金", "银"},
	"鸡": {"黄", "棕", "金"},
	"狗": {"红", "紫", "黄"},
	"猪": {"蓝", "黑", "灰"},
}

// Lucky numbers by zodiac
var luckyNumbers = map[string][]int{
	"鼠": {1, 6, 7},
	"牛": {2, 7, 8},
	"虎": {1, 3, 9},
	"兔": {2, 6, 8},
	"龙": {1, 6, 7},
	"蛇": {2, 8, 9},
	"马": {1, 3, 7},
	"羊": {2, 6, 8},
	"猴": {1, 7, 8},
	"鸡": {2, 5, 8},
	"狗": {3, 6, 9},
	"猪": {1, 2, 9},
}

// Lucky directions by zodiac
var luckyDirections = map[string]string{
	"鼠": "东北、西南",
	"牛": "东南、南方",
	"虎": "东方、北方",
	"兔": "东方、南方",
	"龙": "西北、西方",
	"蛇": "南方、东方",
	"马": "南方、东方",
	"羊": "南方、东方",
	"猴": "西方、东南",
	"鸡": "西方、南方",
	"狗": "南方、东方",
	"猪": "西北、北方",
}

// Name returns the engine identifier.
func (e ZodiacEngine) Name() string { return KindZodiac }

// Compute calculates zodiac fortune based on birth year and current year.
func (e ZodiacEngine) Compute(in Input) (*Result, error) {
	// Validate input
	if in.Year < 1900 || in.Year > 2100 {
		return nil, errYearRange
	}

	// Get user's zodiac from birth year
	solar := calendar.NewSolar(in.Year, in.Month, in.Day, 0, 0, 0)
	lunar := solar.GetLunar()
	userZodiac := lunar.GetYearShengXiao()

	// Get current year's zodiac and earthly branch
	now := time.Now()
	currentYear := now.Year()
	currentSolar := calendar.NewSolar(currentYear, 1, 1, 0, 0, 0)
	currentLunar := currentSolar.GetLunar()
	liuNianZhi := currentLunar.GetYearZhi()
	liuNianZodiac := currentLunar.GetYearShengXiao()

	// Calculate relationships and scores
	relations := calculateRelations(userZodiac, liuNianZodiac)
	baseScore := calculateBaseScore(userZodiac, liuNianZodiac)

	// Apply relationship modifiers
	overallScore := baseScore
	for _, rel := range relations {
		switch rel.Type {
		case "三合":
			overallScore += 15
		case "六合":
			overallScore += 10
		case "相冲":
			overallScore -= 20
		case "相害":
			overallScore -= 10
		}
	}

	// Clamp score to 1-100
	if overallScore < 1 {
		overallScore = 1
	}
	if overallScore > 100 {
		overallScore = 100
	}

	// Calculate dimension scores (variations based on overall)
	careerScore := adjustScore(overallScore, 5)
	wealthScore := adjustScore(overallScore, -5)
	loveScore := adjustScore(overallScore, 10)
	healthScore := adjustScore(overallScore, 0)

	// Build chart
	chart := &ZodiacChart{
		Zodiac:         userZodiac,
		Year:           currentYear,
		LiuNianZhi:     liuNianZhi,
		LiuNianZodiac:  liuNianZodiac,
		OverallScore:   overallScore,
		CareerScore:    careerScore,
		WealthScore:    wealthScore,
		LoveScore:      loveScore,
		HealthScore:    healthScore,
		Relations:      relations,
		LuckyColors:    luckyColors[userZodiac],
		LuckyNumbers:   luckyNumbers[userZodiac],
		LuckyDir:       luckyDirections[userZodiac],
		Tips:           generateTips(userZodiac, overallScore),
		Warns:          generateWarns(userZodiac, relations),
	}

	return &Result{
		Kind: KindZodiac,
		Data: chart,
		Meta: map[string]string{
			"zodiac":  userZodiac,
			"year":    fmt.Sprintf("%d", currentYear),
			"method":  "启发式算法",
		},
	}, nil
}

func init() {
	Register(ZodiacEngine{})
}

// calculateRelations finds all relationships between user's zodiac and current year's zodiac.
func calculateRelations(userZodiac, liuNianZodiac string) []ZodiacRelation {
	var relations []ZodiacRelation

	// Check 三合
	for _, group := range threeHarmony {
		if containsStr(group, userZodiac) && containsStr(group, liuNianZodiac) {
			relations = append(relations, ZodiacRelation{
				Type:   "三合",
				With:   liuNianZodiac,
				Effect: "流年与命主生肖三合，主贵人相助、事业顺遂",
			})
			break
		}
	}

	// Check 六合
	if partner, ok := sixHarmony[userZodiac]; ok && partner == liuNianZodiac {
		relations = append(relations, ZodiacRelation{
			Type:   "六合",
			With:   liuNianZodiac,
			Effect: "流年与命主生肖六合，主感情和顺、贵人运旺",
		})
	}

	// Check 相冲
	if enemy, ok := clash[userZodiac]; ok && enemy == liuNianZodiac {
		relations = append(relations, ZodiacRelation{
			Type:   "相冲",
			With:   liuNianZodiac,
			Effect: "流年与命主生肖相冲，主变动较多、宜稳不宜动",
		})
	}

	// Check 相害
	if enemy, ok := harm[userZodiac]; ok && enemy == liuNianZodiac {
		relations = append(relations, ZodiacRelation{
			Type:   "相害",
			With:   liuNianZodiac,
			Effect: "流年与命主生肖相害，主小人口舌、宜谨言慎行",
		})
	}

	// No special relationship
	if len(relations) == 0 {
		relations = append(relations, ZodiacRelation{
			Type:   "平",
			With:   liuNianZodiac,
			Effect: "流年与命主生肖无特殊刑冲合害，运势平稳",
		})
	}

	return relations
}

// calculateBaseScore returns the base fortune score (50 = neutral).
func calculateBaseScore(userZodiac, liuNianZodiac string) int {
	return 50
}

// adjustScore applies a random variation to the score.
func adjustScore(base, offset int) int {
	score := base + offset
	if score < 1 {
		return 1
	}
	if score > 100 {
		return 100
	}
	return score
}

// generateTips returns fortune tips based on zodiac and score.
func generateTips(zodiac string, score int) []string {
	tips := []string{
		"多与人为善，贵人运自然来",
		"保持积极心态，好运自然相随",
	}

	if score >= 70 {
		tips = append(tips, "今年运势较佳，可适当把握机会")
	} else if score >= 50 {
		tips = append(tips, "运势平稳，宜稳中求进")
	} else {
		tips = append(tips, "今年宜保守行事，不宜冒进")
	}

	// Add zodiac-specific tips
	switch zodiac {
	case "鼠", "龙", "猴":
		tips = append(tips, "水局生肖，宜向北方发展")
	case "牛", "蛇", "鸡":
		tips = append(tips, "金局生肖，宜向西方发展")
	case "虎", "马", "狗":
		tips = append(tips, "火局生肖，宜向南方发展")
	case "兔", "羊", "猪":
		tips = append(tips, "木局生肖，宜向东方发展")
	}

	return tips
}

// generateWarns returns warnings based on relationships.
func generateWarns(zodiac string, relations []ZodiacRelation) []string {
	var warns []string

	for _, rel := range relations {
		if rel.Type == "相冲" {
			warns = append(warns, "本年冲太岁，凡事三思而后行")
			warns = append(warns, "避免与人争执，退一步海阔天空")
		}
		if rel.Type == "相害" {
			warns = append(warns, "本年害太岁，注意口舌是非")
			warns = append(warns, "签合同、合作需谨慎")
		}
	}

	return warns
}

// containsStr checks if a string slice contains a string.
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
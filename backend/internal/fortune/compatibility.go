// Package fortune provides the compatibility (男女配对/合盘) engine.
//
// The engine evaluates compatibility between two people based on:
// - 生肖关系 (三合/六合/相冲/相害)
// - 八字日主五行互补
// - 年柱干支关系
//
// This is heuristic calculation based on traditional public-domain rules,
// combined with AI interpretation for personalized readings.
package fortune

import (
	"fmt"

	"github.com/6tail/lunar-go/calendar"
)

// CompatibilityEngine computes compatibility between two subjects.
type CompatibilityEngine struct{}

// CompatibilityChart is the structured result for compatibility.
type CompatibilityChart struct {
	// Subject summaries
	Subject1 CompatibilitySubject `json:"subject1"`
	Subject2 CompatibilitySubject `json:"subject2"`

	// Overall score (1-100)
	OverallScore int `json:"overallScore"`

	// Dimension scores
	ChemistryScore  int `json:"chemistryScore"`  // 缘分/吸引力
	HarmonyScore    int `json:"harmonyScore"`    // 相处和谐度
	StabilityScore  int `json:"stabilityScore"`  // 关系稳定性

	// Analysis factors
	Factors []CompatibilityFactor `json:"factors"`

	// Summary
	Summary string `json:"summary"`
	Tips    string `json:"tips"`
}

// CompatibilitySubject is a summary of one person's relevant data.
type CompatibilitySubject struct {
	Zodiac    string `json:"zodiac"`
	YearGanZhi string `json:"yearGanZhi"`
	DayGan    string `json:"dayGan"`
	DayZhi    string `json:"dayZhi"`
	DayWuXing string `json:"dayWuXing"` // 日主五行
}

// CompatibilityFactor describes one compatibility dimension.
type CompatibilityFactor struct {
	Factor string `json:"factor"` // 生肖/五行/日柱
	Score  int    `json:"score"`  // -20 to +20
	Detail string `json:"detail"`
}

// Name returns the engine identifier.
func (e CompatibilityEngine) Name() string { return KindCompatibility }

// Compute evaluates compatibility between two subjects.
func (e CompatibilityEngine) Compute(in Input) (*Result, error) {
	// Validate first subject
	if in.Year < 1900 || in.Year > 2100 {
		return nil, errYearRange
	}
	// Validate second subject
	if in.Second == nil {
		return nil, fmt.Errorf("second subject is required")
	}
	if in.Second.Year < 1900 || in.Second.Year > 2100 {
		return nil, errYearRange
	}

	// Build subject summaries
	sub1 := buildSubject(in.Year, in.Month, in.Day)
	sub2 := buildSubject(in.Second.Year, in.Second.Month, in.Second.Day)

	// Calculate factors
	factors := []CompatibilityFactor{}

	// Factor 1: Zodiac relationship
	zodiacFactor := calcZodiacFactor(sub1.Zodiac, sub2.Zodiac)
	factors = append(factors, zodiacFactor)

	// Factor 2: Day master five element relationship
	wuxingFactor := calcWuXingFactor(sub1.DayWuXing, sub2.DayWuXing)
	factors = append(factors, wuxingFactor)

	// Factor 3: Year pillar ganzhi relationship
	yearFactor := calcYearPillarFactor(sub1.YearGanZhi, sub2.YearGanZhi)
	factors = append(factors, yearFactor)

	// Factor 4: Earthly branch of day pillar relationship
	dayZhiFactor := calcDayZhiFactor(sub1.DayZhi, sub2.DayZhi)
	factors = append(factors, dayZhiFactor)

	// Calculate overall score (base 60)
	overall := 60
	for _, f := range factors {
		overall += f.Score
	}
	if overall < 1 {
		overall = 1
	}
	if overall > 100 {
		overall = 100
	}

	// Calculate dimension scores
	chemistry := clampScore(60 + zodiacFactor.Score + wuxingFactor.Score/2)
	harmony := clampScore(60 + dayZhiFactor.Score + wuxingFactor.Score/2)
	stability := clampScore(60 + yearFactor.Score + dayZhiFactor.Score/2)

	summary := generateCompatSummary(overall)
	tips := generateCompatTips(overall, sub1.Zodiac, sub2.Zodiac)

	chart := &CompatibilityChart{
		Subject1:       sub1,
		Subject2:       sub2,
		OverallScore:   overall,
		ChemistryScore: chemistry,
		HarmonyScore:   harmony,
		StabilityScore: stability,
		Factors:        factors,
		Summary:        summary,
		Tips:           tips,
	}

	return &Result{
		Kind: KindCompatibility,
		Data: chart,
		Meta: map[string]string{
			"zodiac1": sub1.Zodiac,
			"zodiac2": sub2.Zodiac,
			"method":  "启发式算法",
		},
	}, nil
}

func init() {
	Register(CompatibilityEngine{})
}

// buildSubject creates a CompatibilitySubject from birth date.
func buildSubject(year, month, day int) CompatibilitySubject {
	solar := calendar.NewSolar(year, month, day, 12, 0, 0)
	lunar := solar.GetLunar()
	ec := lunar.GetEightChar()

	zodiac := lunar.GetYearShengXiao()
	yearGanZhi := lunar.GetYearInGanZhi()
	dayGan := ec.GetDayGan()
	dayZhi := ec.GetDayZhi()
	dayWuXing := ganWuXing(dayGan)

	return CompatibilitySubject{
		Zodiac:     zodiac,
		YearGanZhi: yearGanZhi,
		DayGan:     dayGan,
		DayZhi:     dayZhi,
		DayWuXing:  dayWuXing,
	}
}

// ganWuXing returns the five element of a heavenly stem.
func ganWuXing(gan string) string {
	switch gan {
	case "甲", "乙":
		return "木"
	case "丙", "丁":
		return "火"
	case "戊", "己":
		return "土"
	case "庚", "辛":
		return "金"
	case "壬", "癸":
		return "水"
	}
	return ""
}

// calcZodiacFactor evaluates zodiac compatibility.
func calcZodiacFactor(z1, z2 string) CompatibilityFactor {
	// Same zodiac
	if z1 == z2 {
		return CompatibilityFactor{
			Factor: "生肖",
			Score:  5,
			Detail: fmt.Sprintf("双方均属%s，性格相近，易理解彼此，但也可能过于相似而缺乏互补", z1),
		}
	}

	// Check 三合
	for _, group := range threeHarmony {
		if containsStr(group, z1) && containsStr(group, z2) {
			return CompatibilityFactor{
				Factor: "生肖",
				Score:  18,
				Detail: fmt.Sprintf("%s与%s三合，为最佳配对之一，缘分深厚，相处融洽", z1, z2),
			}
		}
	}

	// Check 六合
	if partner, ok := sixHarmony[z1]; ok && partner == z2 {
		return CompatibilityFactor{
			Factor: "生肖",
			Score:  15,
			Detail: fmt.Sprintf("%s与%s六合，互补性强，感情和谐", z1, z2),
		}
	}

	// Check 相冲
	if enemy, ok := clash[z1]; ok && enemy == z2 {
		return CompatibilityFactor{
			Factor: "生肖",
			Score:  -18,
			Detail: fmt.Sprintf("%s与%s相冲，性格差异较大，需更多包容与磨合", z1, z2),
		}
	}

	// Check 相害
	if enemy, ok := harm[z1]; ok && enemy == z2 {
		return CompatibilityFactor{
			Factor: "生肖",
			Score:  -10,
			Detail: fmt.Sprintf("%s与%s相害，相处中可能有摩擦，需多沟通", z1, z2),
		}
	}

	// Neutral
	return CompatibilityFactor{
		Factor: "生肖",
		Score:  0,
		Detail: fmt.Sprintf("%s与%s生肖关系平淡，无特别刑冲合害", z1, z2),
	}
}

// calcWuXingFactor evaluates day master five element compatibility.
// 五行相生 (generating cycle): 木→火→土→金→水→木
// 五行相克 (overcoming cycle): 木→土→水→火→金→木
func calcWuXingFactor(w1, w2 string) CompatibilityFactor {
	if w1 == w2 {
		return CompatibilityFactor{
			Factor: "五行",
			Score:  5,
			Detail: fmt.Sprintf("双方日主同属%s，性情相近，易产生共鸣", w1),
		}
	}

	// Check 相生 (generating)
	generating := map[string]string{
		"木": "火", "火": "土", "土": "金", "金": "水", "水": "木",
	}
	if generating[w1] == w2 {
		return CompatibilityFactor{
			Factor: "五行",
			Score:  12,
			Detail: fmt.Sprintf("%s生%s，一方滋养另一方，关系和谐互补", w1, w2),
		}
	}
	if generating[w2] == w1 {
		return CompatibilityFactor{
			Factor: "五行",
			Score:  12,
			Detail: fmt.Sprintf("%s生%s，一方滋养另一方，关系和谐互补", w2, w1),
		}
	}

	// Check 相克 (overcoming)
	overcoming := map[string]string{
		"木": "土", "土": "水", "水": "火", "火": "金", "金": "木",
	}
	if overcoming[w1] == w2 {
		return CompatibilityFactor{
			Factor: "五行",
			Score:  -8,
			Detail: fmt.Sprintf("%s克%s，可能存在性格压制，需学会互相尊重", w1, w2),
		}
	}
	if overcoming[w2] == w1 {
		return CompatibilityFactor{
			Factor: "五行",
			Score:  -8,
			Detail: fmt.Sprintf("%s克%s，可能存在性格压制，需学会互相尊重", w2, w1),
		}
	}

	return CompatibilityFactor{
		Factor: "五行",
		Score:  0,
		Detail: "日主五行关系中性",
	}
}

// calcYearPillarFactor evaluates year pillar compatibility.
func calcYearPillarFactor(y1, y2 string) CompatibilityFactor {
	if y1 == y2 {
		return CompatibilityFactor{
			Factor: "年柱",
			Score:  8,
			Detail: "双方年柱相同，年龄相仿，价值观相近",
		}
	}
	return CompatibilityFactor{
		Factor: "年柱",
		Score:  0,
		Detail: "年柱干支不同，正常情况",
	}
}

// calcDayZhiFactor evaluates day branch compatibility.
func calcDayZhiFactor(d1, d2 string) CompatibilityFactor {
	if d1 == d2 {
		return CompatibilityFactor{
			Factor: "日支",
			Score:  6,
			Detail: fmt.Sprintf("双方日支同属%s，内心世界相通", d1),
		}
	}

	// Map earthly branch to zodiac for relationship check
	z1 := branchToZodiac(d1)
	z2 := branchToZodiac(d2)
	if z1 != "" && z2 != "" {
		// Reuse zodiac relationships
		for _, group := range threeHarmony {
			if containsStr(group, z1) && containsStr(group, z2) {
				return CompatibilityFactor{
					Factor: "日支",
					Score:  10,
					Detail: fmt.Sprintf("日支%s与%s三合，夫妻宫和谐", d1, d2),
				}
			}
		}
		if partner, ok := sixHarmony[z1]; ok && partner == z2 {
			return CompatibilityFactor{
				Factor: "日支",
				Score:  8,
				Detail: fmt.Sprintf("日支%s与%s六合，夫妻宫互补", d1, d2),
			}
		}
		if enemy, ok := clash[z1]; ok && enemy == z2 {
			return CompatibilityFactor{
				Factor: "日支",
				Score:  -10,
				Detail: fmt.Sprintf("日支%s与%s相冲，夫妻宫有冲，需多包容", d1, d2),
			}
		}
	}

	return CompatibilityFactor{
		Factor: "日支",
		Score:  0,
		Detail: "日支关系中性",
	}
}

// branchToZodiac maps an earthly branch to its zodiac animal.
func branchToZhiIndex(branch string) int {
	for i, b := range earthlyBranches {
		if b == branch {
			return i
		}
	}
	return -1
}

func branchToZodiac(branch string) string {
	idx := branchToZhiIndex(branch)
	if idx < 0 {
		return ""
	}
	return zodiacAnimals[idx]
}

// clampScore clamps a score to 1-100 range.
func clampScore(score int) int {
	if score < 1 {
		return 1
	}
	if score > 100 {
		return 100
	}
	return score
}

// generateCompatSummary returns a summary based on overall score.
func generateCompatSummary(score int) string {
	switch {
	case score >= 85:
		return "天作之合！双方缘分深厚，是难得的佳配。"
	case score >= 70:
		return "佳偶天成！双方较为般配，相处融洽。"
	case score >= 55:
		return "中等姻缘。双方有缘，需用心经营。"
	case score >= 40:
		return "缘分一般。双方差异较多，需多包容理解。"
	default:
		return "情路坎坷。双方差异较大，需付出更多努力。"
	}
}

// generateCompatTips returns advice based on score and zodiacs.
func generateCompatTips(score int, z1, z2 string) string {
	if score >= 70 {
		return fmt.Sprintf("属%s与属%s的配对较为理想，珍惜缘分，共同成长。", z1, z2)
	}
	if score >= 50 {
		return fmt.Sprintf("属%s与属%s的组合需要双方共同努力，多沟通、多理解是关键。", z1, z2)
	}
	return fmt.Sprintf("属%s与属%s的差异较大，但只要互相尊重、包容，依然可以长久。建议避免正面冲突，学会换位思考。", z1, z2)
}
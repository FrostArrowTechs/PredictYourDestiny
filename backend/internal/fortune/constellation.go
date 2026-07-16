// Package fortune provides the constellation (星座运势) fortune engine.
//
// The engine computes Western sun-sign traits and a heuristic daily
// fortune based on the user's birth date (month/day → sun sign).
// Traditional astrological archetype descriptions are public-domain
// knowledge; the daily scores are a lightweight heuristic that gives
// the AI interpret layer something structured to riff on.
//
// Engine contract:
//   - Input: Year/Month/Day (birth date, solar)
//   - Output: *ConstellationChart with sign traits + today's scores
package fortune

import (
	"fmt"
	"math/rand"
	"time"
)

// ConstellationEngine computes Western zodiac (sun-sign) fortune.
type ConstellationEngine struct{}

// ConstellationChart is the structured result for constellation fortune.
type ConstellationChart struct {
	// Sun sign (e.g. 白羊座) and its Latin name (Aries)
	Sign      string `json:"sign"`
	SignLatin string `json:"signLatin"`
	// Element (火/土/风/水) and quality (基本/固定/变动)
	Element string `json:"element"`
	Quality string `json:"quality"`
	// Ruling planet
	Ruler string `json:"ruler"`
	// Date range of the sign
	DateRange string `json:"dateRange"`

	// Personality traits (public-domain archetype descriptions)
	Strengths []string `json:"strengths"`
	Weakness  []string `json:"weakness"`
	Keywords  []string `json:"keywords"`

	// Today's heuristic fortune scores (1-100). Seeded by sign + date
	// so the same user gets a stable reading for the whole day.
	OverallScore int `json:"overallScore"`
	CareerScore  int `json:"careerScore"`
	LoveScore    int `json:"loveScore"`
	WealthScore  int `json:"wealthScore"`
	HealthScore  int `json:"healthScore"`

	// Lucky elements for the day
	LuckyColors  []string `json:"luckyColors"`
	LuckyNumbers []int    `json:"luckyNumbers"`
	LuckyDir     string   `json:"luckyDir"`

	// Compatibility with other signs (best/worst matches)
	BestMatch  string `json:"bestMatch"`
	WorstMatch string `json:"worstMatch"`
}

// signDef holds the static archetype data for one sun sign.
type signDef struct {
	CN         string
	Latin      string
	Element    string
	Quality    string
	Ruler      string
	DateRange  string
	Strengths  []string
	Weakness   []string
	Keywords   []string
	LuckyColor []string
	LuckyNum   []int
	LuckyDir   string
	BestMatch  string
	WorstMatch string
}

// Order matters: indexed by getSignIndex(month, day).
var signDefs = []signDef{
	{
		CN: "白羊座", Latin: "Aries", Element: "火", Quality: "基本", Ruler: "火星",
		DateRange:  "3月21日 - 4月19日",
		Strengths:  []string{"勇敢果断", "热情直率", "行动力强", "富有冒险精神"},
		Weakness:   []string{"易冲动", "缺乏耐心", "三分钟热度"},
		Keywords:   []string{"开创", "勇气", "竞争", "先锋"},
		LuckyColor: []string{"红色", "橙色"},
		LuckyNum:   []int{1, 9},
		LuckyDir:   "东方",
		BestMatch:  "狮子座、射手座", WorstMatch: "天秤座、摩羯座",
	},
	{
		CN: "金牛座", Latin: "Taurus", Element: "土", Quality: "固定", Ruler: "金星",
		DateRange:  "4月20日 - 5月20日",
		Strengths:  []string{"踏实稳重", "有耐心", "务实可靠", "审美佳"},
		Weakness:   []string{"固执", "变化慢", "占有欲强"},
		Keywords:   []string{"稳定", "享受", "积累", "美感"},
		LuckyColor: []string{"绿色", "粉色"},
		LuckyNum:   []int{2, 6},
		LuckyDir:   "东南",
		BestMatch:  "处女座、摩羯座", WorstMatch: "狮子座、水瓶座",
	},
	{
		CN: "双子座", Latin: "Gemini", Element: "风", Quality: "变动", Ruler: "水星",
		DateRange:  "5月21日 - 6月21日",
		Strengths:  []string{"思维敏捷", "善于沟通", "适应力强", "好奇心旺盛"},
		Weakness:   []string{"善变", "注意力分散", "缺乏深度"},
		Keywords:   []string{"交流", "多元", "灵活", "学习"},
		LuckyColor: []string{"黄色", "浅蓝"},
		LuckyNum:   []int{3, 5},
		LuckyDir:   "东北",
		BestMatch:  "天秤座、水瓶座", WorstMatch: "处女座、双鱼座",
	},
	{
		CN: "巨蟹座", Latin: "Cancer", Element: "水", Quality: "基本", Ruler: "月亮",
		DateRange:  "6月22日 - 7月22日",
		Strengths:  []string{"情感丰富", "体贴顾家", "直觉敏锐", "记忆力强"},
		Weakness:   []string{"情绪化", "多愁善感", "依赖性强"},
		Keywords:   []string{"家庭", "情感", "保护", "记忆"},
		LuckyColor: []string{"银色", "白色"},
		LuckyNum:   []int{2, 7},
		LuckyDir:   "北方",
		BestMatch:  "天蝎座、双鱼座", WorstMatch: "天秤座、白羊座",
	},
	{
		CN: "狮子座", Latin: "Leo", Element: "火", Quality: "固定", Ruler: "太阳",
		DateRange:  "7月23日 - 8月22日",
		Strengths:  []string{"自信大方", "有领导力", "慷慨热情", "富有魅力"},
		Weakness:   []string{"好面子", "专横", "虚荣"},
		Keywords:   []string{"荣耀", "创造", "表达", "领袖"},
		LuckyColor: []string{"金色", "橙色"},
		LuckyNum:   []int{1, 5},
		LuckyDir:   "东北",
		BestMatch:  "白羊座、射手座", WorstMatch: "天蝎座、金牛座",
	},
	{
		CN: "处女座", Latin: "Virgo", Element: "土", Quality: "变动", Ruler: "水星",
		DateRange:  "8月23日 - 9月22日",
		Strengths:  []string{"细致严谨", "分析力强", "勤奋务实", "追求完美"},
		Weakness:   []string{"挑剔", "焦虑", "过度批判"},
		Keywords:   []string{"分析", "服务", "完善", "秩序"},
		LuckyColor: []string{"米色", "深绿"},
		LuckyNum:   []int{5, 7},
		LuckyDir:   "南方",
		BestMatch:  "金牛座、摩羯座", WorstMatch: "射手座、双子座",
	},
	{
		CN: "天秤座", Latin: "Libra", Element: "风", Quality: "基本", Ruler: "金星",
		DateRange:  "9月23日 - 10月23日",
		Strengths:  []string{"优雅公正", "善于交际", "审美出众", "追求和谐"},
		Weakness:   []string{"优柔寡断", "讨好他人", "逃避冲突"},
		Keywords:   []string{"平衡", "关系", "美感", "合作"},
		LuckyColor: []string{"粉色", "浅蓝"},
		LuckyNum:   []int{6, 9},
		LuckyDir:   "西北",
		BestMatch:  "双子座、水瓶座", WorstMatch: "巨蟹座、摩羯座",
	},
	{
		CN: "天蝎座", Latin: "Scorpio", Element: "水", Quality: "固定", Ruler: "冥王星",
		DateRange:  "10月24日 - 11月22日",
		Strengths:  []string{"洞察力强", "意志坚定", "深情专注", "神秘有魅力"},
		Weakness:   []string{"多疑", "记仇", "占有欲强"},
		Keywords:   []string{"深度", "转化", "神秘", "掌控"},
		LuckyColor: []string{"暗红", "黑色"},
		LuckyNum:   []int{4, 9},
		LuckyDir:   "北方",
		BestMatch:  "巨蟹座、双鱼座", WorstMatch: "狮子座、水瓶座",
	},
	{
		CN: "射手座", Latin: "Sagittarius", Element: "火", Quality: "变动", Ruler: "木星",
		DateRange:  "11月23日 - 12月21日",
		Strengths:  []string{"乐观豁达", "热爱自由", "富有哲思", "坦诚直率"},
		Weakness:   []string{"粗心", "不负责任", "过于直接"},
		Keywords:   []string{"自由", "探索", "智慧", "远方"},
		LuckyColor: []string{"紫色", "蓝色"},
		LuckyNum:   []int{3, 7},
		LuckyDir:   "南方",
		BestMatch:  "白羊座、狮子座", WorstMatch: "处女座、双鱼座",
	},
	{
		CN: "摩羯座", Latin: "Capricorn", Element: "土", Quality: "基本", Ruler: "土星",
		DateRange:  "12月22日 - 1月19日",
		Strengths:  []string{"坚韧务实", "有责任感", "目标明确", "自律"},
		Weakness:   []string{"悲观", "刻板", "过于现实"},
		Keywords:   []string{"成就", "责任", "结构", "野心"},
		LuckyColor: []string{"黑色", "棕色"},
		LuckyNum:   []int{8, 4},
		LuckyDir:   "南方",
		BestMatch:  "金牛座、处女座", WorstMatch: "白羊座、天秤座",
	},
	{
		CN: "水瓶座", Latin: "Aquarius", Element: "风", Quality: "固定", Ruler: "天王星",
		DateRange:  "1月20日 - 2月18日",
		Strengths:  []string{"独立创新", "思想前卫", "人道主义", "理性客观"},
		Weakness:   []string{"疏离", "固执己见", "难以亲近"},
		Keywords:   []string{"创新", "理想", "群体", "未来"},
		LuckyColor: []string{"电蓝", "银色"},
		LuckyNum:   []int{4, 8},
		LuckyDir:   "西方",
		BestMatch:  "双子座、天秤座", WorstMatch: "金牛座、天蝎座",
	},
	{
		CN: "双鱼座", Latin: "Pisces", Element: "水", Quality: "变动", Ruler: "海王星",
		DateRange:  "2月19日 - 3月20日",
		Strengths:  []string{"富有同情心", "直觉敏锐", "艺术天赋", "温柔包容"},
		Weakness:   []string{"逃避现实", "优柔寡断", "易受影响"},
		Keywords:   []string{"梦境", "慈悲", "艺术", "灵性"},
		LuckyColor: []string{"海蓝", "紫色"},
		LuckyNum:   []int{7, 3},
		LuckyDir:   "东南",
		BestMatch:  "巨蟹座、天蝎座", WorstMatch: "双子座、射手座",
	},
}

// signStartDay[month] = day-of-month on which the next sign begins.
// month is 1-12. The boundary day belongs to the NEW sign.
// Aries starts Mar 21, Taurus Apr 20, ... Capricorn Dec 22, Aquarius Jan 20, Pisces Feb 19.
var signStartDay = map[int]int{
	1: 20,  // Aquarius
	2: 19,  // Pisces
	3: 21,  // Aries
	4: 20,  // Taurus
	5: 21,  // Gemini
	6: 22,  // Cancer
	7: 23,  // Leo
	8: 23,  // Virgo
	9: 23,  // Libra
	10: 24, // Scorpio
	11: 23, // Sagittarius
	12: 22, // Capricorn
}

// getSignIndex returns the 0-based index into signDefs for a birth date.
func getSignIndex(month, day int) int {
	// The sign order in signDefs starts at Aries (month 3). We shift the
	// month index so January becomes index 9 (Capricorn is signDefs[9]).
	// monthSignStart[m] = signDefs index that begins on/after Jan 1 of month m.
	//   Jan -> Capricorn (9) until Jan 20, then Aquarius (10)
	//   Feb -> Aquarius (10) until Feb 19, then Pisces (11)
	//   Mar -> Pisces (11) until Mar 21, then Aries (0)
	// ...and so on.
	// signBeforeBoundary[m] = index of sign active at the START of month m.
	signBeforeBoundary := []int{9, 10, 11, 0, 1, 2, 3, 4, 5, 6, 7, 8}
	idx := signBeforeBoundary[month-1]
	if day >= signStartDay[month] {
		idx = (idx + 1) % 12
	}
	return idx
}

// Name returns the engine identifier.
func (e ConstellationEngine) Name() string { return KindConstellation }

// Compute returns the constellation chart for the given birth date.
func (e ConstellationEngine) Compute(in Input) (*Result, error) {
	if in.Month < 1 || in.Month > 12 {
		return nil, fmt.Errorf("constellation: month out of range (1-12)")
	}
	if in.Day < 1 || in.Day > 31 {
		return nil, fmt.Errorf("constellation: day out of range (1-31)")
	}

	idx := getSignIndex(in.Month, in.Day)
	def := signDefs[idx]

	// Deterministic daily seed: same sign + same calendar day → same scores.
	now := time.Now()
	seed := int64(idx+1)*10000 + int64(now.Year())*100 + int64(int(now.Month()))
	rnd := rand.New(rand.NewSource(seed))

	def = randomizeDaily(def, rnd)

	chart := &ConstellationChart{
		Sign:         def.CN,
		SignLatin:    def.Latin,
		Element:      def.Element,
		Quality:      def.Quality,
		Ruler:        def.Ruler,
		DateRange:    def.DateRange,
		Strengths:    def.Strengths,
		Weakness:     def.Weakness,
		Keywords:     def.Keywords,
		OverallScore: clampScore(60 + rnd.Intn(40)),
		CareerScore:  clampScore(55 + rnd.Intn(45)),
		LoveScore:    clampScore(55 + rnd.Intn(45)),
		WealthScore:  clampScore(50 + rnd.Intn(50)),
		HealthScore:  clampScore(60 + rnd.Intn(40)),
		LuckyColors:  def.LuckyColor,
		LuckyNumbers: def.LuckyNum,
		LuckyDir:     def.LuckyDir,
		BestMatch:    def.BestMatch,
		WorstMatch:   def.WorstMatch,
	}

	return &Result{
		Kind: KindConstellation,
		Data: chart,
		Meta: map[string]string{
			"sign":   def.CN,
			"date":   now.Format("2006-01-02"),
			"method": "星座特质 + 启发式日运",
		},
	}, nil
}

// randomizeDaily is a no-op placeholder kept for future per-day variation
// of traits; currently traits are static archetype data (intentional —
// a sign's core personality does not change day to day). Scores are
// randomized separately in Compute.
func randomizeDaily(def signDef, rnd *rand.Rand) signDef {
	return def
}

func init() {
	Register(ConstellationEngine{})
}

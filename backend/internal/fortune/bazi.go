package fortune

import (
	"container/list"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/6tail/lunar-go/LunarUtil"
	"github.com/6tail/lunar-go/calendar"
)

// BaziEngine implements FortuneEngine for the Four Pillars (八字)
// system. It is stateless and safe for concurrent use: every call
// builds a fresh lunar-go Lunar / EightChar chain.
//
// The engine delegates the heavy calendar lifting to lunar-go:
//   - Solar → Lunar conversion (handles 立春 year boundary, 闰月)
//   - Four pillars (年/月/日/时) with Exact (节气) month pillar
//   - 纳音, 藏干, 十神, 胎元/命宫/身宫, 旬空
//   - 大运 (DaYun) + 流年 (LiuNian) sequencing
//
// What lunar-go does NOT provide, this engine computes itself:
//   - 真太阳时 correction from longitude
//   - 神煞 (天乙贵人 / 文昌 / 驿马 / 桃花 / 华盖 …) — table-based
//   - 五行统计 + 旺衰 + 用神初判 — heuristic, not AI
//
// 旺衰/用神 are explicitly separated as versioned interpretive output. The AI
// may explain them, but must never revise the calculated chart.
type BaziEngine struct{}

// Name returns the engine identifier (also FortuneRecord.Kind).
func (BaziEngine) Name() string { return KindBazi }

// init self-registers so the handler can find us via Fortune(KindBazi).
func init() { Register(BaziEngine{}) }

// ─── output structures ────────────────────────────────────────────
// These are exported so the handler can JSON-marshal them straight
// into the API response and into FortuneRecord.ResultJSON. Field
// names use pinyin/English mix matching the rest of the codebase;
// the values themselves are Chinese characters (天干地支).

// Pillar is one of the four columns (年/月/日/时). Each pillar
// stores the gan-zhi pair plus the derived attributes lunar-go
// computes for it.
type Pillar struct {
	Position   string   `json:"position"`   // 年|月|日|时
	GanZhi     string   `json:"ganZhi"`     // e.g. "甲子"
	Gan        string   `json:"gan"`        // 天干
	Zhi        string   `json:"zhi"`        // 地支
	WuXing     string   `json:"wuXing"`     // 五行 (e.g. "木水")
	NaYin      string   `json:"naYin"`      // 纳音 (e.g. "海中金")
	HideGan    []string `json:"hideGan"`    // 地支藏干
	ShiShenGan string   `json:"shiShenGan"` // 天干十神 (日柱为"日主")
	ShiShenZhi []string `json:"shiShenZhi"` // 地支藏干十神
	DiShi      string   `json:"diShi"`      // 十二长生
	Xun        string   `json:"xun"`        // 所在旬
	XunKong    string   `json:"xunKong"`    // 旬空
}

// ShenSha is a single 神煞 entry found on a pillar.
type ShenSha struct {
	Name     string `json:"name"`     // 天乙贵人 / 文昌 / 驿马 …
	Position string `json:"position"` // 年|月|日|时
	GanZhi   string `json:"ganZhi"`   // the gan-zhi it attaches to
	Note     string `json:"note"`     // short meaning
}

// DaYun is one 10-year major luck period.
type DaYun struct {
	Index     int       `json:"index"`     // 0-based; 0 is the pre-起运 period
	GanZhi    string    `json:"ganZhi"`    // 干支 (empty for index 0)
	StartYear int       `json:"startYear"` // 公历起年 (含)
	EndYear   int       `json:"endYear"`   // 公历止年 (含)
	StartAge  int       `json:"startAge"`  // 起岁 (虚岁, 含)
	EndAge    int       `json:"endAge"`    // 止岁 (虚岁, 含)
	LiuNian   []LiuNian `json:"liuNian"`   // 逐年干支
}

// LiuNian is one year within a DaYun.
type LiuNian struct {
	Year   int    `json:"year"`   // 公历年
	Age    int    `json:"age"`    // 虚岁
	GanZhi string `json:"ganZhi"` // 流年干支
}

// WuXingStat is the count of each element across the chart.
type WuXingStat struct {
	Element string `json:"element"` // 金|木|水|火|土
	Count   int    `json:"count"`   // occurrences in the 8 chars
	Percent int    `json:"percent"` // share of total (0-100), rounded
}

// WangShuai is the 旺衰 judgement: which element is 最旺 and which is
// 最弱, plus a free-text summary. It is heuristic — derived from the
// month branch (令) and the raw element counts.
type WangShuai struct {
	Strong  string `json:"strong"`  // 最旺五行
	Weak    string `json:"weak"`    // 最弱五行
	DayWang string `json:"dayWang"` // 日主旺衰: "偏旺"|"偏弱"|"平衡"
	Summary string `json:"summary"` // short human-readable line
}

// YongYin is the preliminary 用神 hint. Confidence is low ("初判");
// the AI is expected to refine it. YinShen is the recommended 用神
// element; XiJi lists favorable vs unfavorable elements.
type YongYin struct {
	YongShen   string   `json:"yongShen"`   // 用神五行
	Xi         []string `json:"xi"`         // 喜神五行
	Ji         []string `json:"ji"`         // 忌神五行
	Confidence string   `json:"confidence"` // "初判" (always, for now)
	Reason     string   `json:"reason"`     // short heuristic rationale
}

type BaziInterpretation struct {
	RuleSetVersion string    `json:"ruleSetVersion"`
	Nature         string    `json:"nature"`
	InputFacts     []Fact    `json:"inputFacts"`
	Warnings       []string  `json:"warnings"`
	WangShuai      WangShuai `json:"wangShuai"`
	YongYin        YongYin   `json:"yongYin"`
}

// BaziChart is the complete structured output of Compute. It is what
// gets serialized to Result.Data and to FortuneRecord.ResultJSON.
type BaziChart struct {
	// Subject provenance
	Solar                     string      `json:"solar"`      // 规则校正后的排盘时间
	Lunar                     string      `json:"lunar"`      // 农历
	SolarISO                  string      `json:"solarISO"`   // ISO 8601 of corrected time
	Longitude                 float64     `json:"longitude"`  // 经度 (0 = 未校正)
	TrueSolar                 bool        `json:"trueSolar"`  // 是否做了真太阳时校正
	Correction                string      `json:"correction"` // 校正说明
	TimeZone                  string      `json:"timeZone"`
	SolarTimeMode             string      `json:"solarTimeMode"`
	LongitudeCorrectionMinute int         `json:"longitudeCorrectionMinutes"`
	EquationOfTimeMinute      float64     `json:"equationOfTimeMinutes"`
	TotalCorrectionMinute     int         `json:"totalCorrectionMinutes"`
	RuleSetVersion            string      `json:"ruleSetVersion"`
	DayBoundary               string      `json:"dayBoundary"`
	CalendarLibraryVersion    string      `json:"calendarLibraryVersion"`
	PreviousJie               JieBoundary `json:"previousJie"`
	NextJie                   JieBoundary `json:"nextJie"`
	YunMethod                 string      `json:"yunMethod"`

	// Four pillars, in 年/月/日/时 order
	Pillars []Pillar `json:"pillars"`

	// Derived columns
	TaiYuan       string `json:"taiYuan"` // 胎元
	TaiYuanNaYin  string `json:"taiYuanNaYin"`
	TaiXi         string `json:"taiXi"` // 胎息
	TaiXiNaYin    string `json:"taiXiNaYin"`
	MingGong      string `json:"mingGong"` // 命宫
	MingGongNaYin string `json:"mingGongNaYin"`
	ShenGong      string `json:"shenGong"` // 身宫
	ShenGongNaYin string `json:"shenGongNaYin"`

	ShenSha []ShenSha `json:"shenSha"`

	// Luck sequence
	StartYear  int     `json:"startYear"`  // 起运年数
	StartMonth int     `json:"startMonth"` // 起运月数
	StartDay   int     `json:"startDay"`   // 起运天数
	StartHour  int     `json:"startHour"`  // 起运小时
	Forward    bool    `json:"forward"`    // 顺排/逆排
	StartSolar string  `json:"startSolar"` // 起运公历日期
	DaYun      []DaYun `json:"daYun"`      // 大运 (8-10 轮)

	// Element analysis
	WuXingStats    []WuXingStat       `json:"wuXingStats"`
	Interpretation BaziInterpretation `json:"interpretation"`
}

type JieBoundary struct {
	Name string `json:"name"`
	Time string `json:"time"`
}

// ─── Compute ──────────────────────────────────────────────────────

// Compute builds a full bazi chart from Input. The input time is
// interpreted in Beijing time (UTC+8) as lunar-go expects; if
// Longitude is non-zero a 真太阳时 correction is applied first.
func (BaziEngine) Compute(in Input) (*Result, error) {
	if err := validateBaziInput(in); err != nil {
		return nil, err
	}
	ruleSet := ""
	if in.Birth != nil {
		ruleSet = in.Birth.RuleSet
	}
	rules, err := resolveBaziRules(ruleSet)
	if err != nil {
		return nil, err
	}
	_, timeZone, err := baziTimeZone(in.Birth)
	if err != nil {
		return nil, err
	}

	// 真太阳时: 1° east of 120°E advances local apparent solar time
	// by ~4 minutes; west delays. We adjust the wall clock by
	// (longitude − 120) × 4 minutes. lunar-go itself is timezone-free
	// and treats the supplied hour/minute as Beijing time, so this
	// correction must happen BEFORE we hand the time to it.
	longitudeCorrection := 0
	equationCorrection := 0.0
	corrMin := 0
	trueSolar := false
	correction := "未做真太阳时校正（按北京时间 UTC+8 计算）"
	year, month, day, hour, minute := in.Year, in.Month, in.Day, in.Hour, in.Minute
	hasLongitude := in.Longitude != 0
	if in.Birth != nil {
		hasLongitude = in.Birth.Longitude != nil
	}
	if hasLongitude {
		longitudeCorrection, equationCorrection, corrMin, timeZone, err = apparentSolarCorrection(
			in.Birth, in.Longitude, year, month, day, hour, minute,
		)
		if err != nil {
			return nil, err
		}
		trueSolar = true
		// advance or retard the input instant by corrMin minutes
		t := time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.UTC)
		t = t.Add(time.Duration(corrMin) * time.Minute)
		year, month, day, hour, minute = t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute()
		sign := "+"
		if corrMin < 0 {
			sign = ""
		}
		correction = fmt.Sprintf("真太阳时校正：经度差 %+d 分钟，均时差 %+.1f 分钟，总偏移 %s%d 分钟",
			longitudeCorrection, equationCorrection, sign, corrMin)
	}

	solar := calendar.NewSolar(year, month, day, hour, minute, 0)
	lunar := solar.GetLunar()
	ec := lunar.GetEightChar()
	ec.SetSect(rules.EightCharSect)
	prevJie, nextJie := lunar.GetPrevJie(), lunar.GetNextJie()

	chart := &BaziChart{
		Solar:      fmt.Sprintf("%04d-%02d-%02d %02d:%02d", year, month, day, hour, minute),
		Lunar:      lunar.ToFullString(),
		SolarISO:   fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:00+08:00", year, month, day, hour, minute),
		Longitude:  in.Longitude,
		TrueSolar:  trueSolar,
		Correction: correction,
		TimeZone:   timeZone,
		SolarTimeMode: func() string {
			if trueSolar {
				return "local_apparent_solar"
			}
			return "legal_time"
		}(),
		LongitudeCorrectionMinute: longitudeCorrection,
		EquationOfTimeMinute:      math.Round(equationCorrection*10) / 10,
		TotalCorrectionMinute:     corrMin,
		RuleSetVersion:            rules.Version,
		DayBoundary:               rules.DayBoundary,
		CalendarLibraryVersion:    BaziCalendarVersion,
		PreviousJie:               JieBoundary{Name: prevJie.GetName(), Time: prevJie.GetSolar().ToYmdHms()},
		NextJie:                   JieBoundary{Name: nextJie.GetName(), Time: nextJie.GetSolar().ToYmdHms()},
		YunMethod:                 "minute_difference_3days_per_year",
	}

	// ── four pillars ──
	chart.Pillars = buildPillars(ec)

	// ── 胎元/命宫/身宫 ──
	chart.TaiYuan = ec.GetTaiYuan()
	chart.TaiYuanNaYin = ec.GetTaiYuanNaYin()
	chart.TaiXi = ec.GetTaiXi()
	chart.TaiXiNaYin = ec.GetTaiXiNaYin()
	chart.MingGong = ec.GetMingGong()
	chart.MingGongNaYin = ec.GetMingGongNaYin()
	chart.ShenGong = ec.GetShenGong()
	chart.ShenGongNaYin = ec.GetShenGongNaYin()

	// ── 神煞 ──
	chart.ShenSha = computeShenSha(chart.Pillars)

	// ── 大运 + 流年 ──
	yun := ec.GetYunBySect(int(in.Gender), rules.YunSect)
	chart.StartYear = yun.GetStartYear()
	chart.StartMonth = yun.GetStartMonth()
	chart.StartDay = yun.GetStartDay()
	chart.StartHour = yun.GetStartHour()
	chart.Forward = yun.IsForward()
	chart.StartSolar = yun.GetStartSolar().ToFullString()
	chart.DaYun = buildDaYun(yun)

	// ── 五行 / 旺衰 / 用神 ──
	chart.WuXingStats = computeWuXingStats(chart.Pillars)
	wangShuai := computeWangShuai(chart.Pillars, chart.WuXingStats)
	chart.Interpretation = BaziInterpretation{
		RuleSetVersion: "bazi-wangshuai-heuristic-v1",
		Nature:         "interpretive_heuristic",
		InputFacts: []Fact{
			{Key: "dayMaster", Value: chart.Pillars[2].Gan},
			{Key: "monthBranch", Value: chart.Pillars[1].Zhi},
			{Key: "wuXingStats", Value: chart.WuXingStats},
		},
		Warnings:  []string{"旺衰与喜用神属于传统解释规则，不是历法基础事实，也不代表科学验证结论"},
		WangShuai: wangShuai,
		YongYin:   computeYongYin(chart.Pillars, wangShuai),
	}

	return &Result{
		Kind: KindBazi,
		Data: chart,
		Meta: map[string]string{
			"engine":      KindBazi,
			"solar":       chart.Solar,
			"lunar":       chart.Lunar,
			"trueSolar":   fmt.Sprintf("%v", chart.TrueSolar),
			"dayMaster":   chart.Pillars[2].Gan,              // 日主
			"dayMasterWX": wuxingOfGan(chart.Pillars[2].Gan), // 日主五行
		},
	}, nil
}

// ─── helpers ──────────────────────────────────────────────────────

// validateBaziInput enforces sane ranges. It does NOT check calendar
// legality (Feb 30) — lunar-go panics on those, which we convert to
// an error via the deferred recover in the handler. Here we only catch
// the cheap stuff so the error message is friendly.
func validateBaziInput(in Input) error {
	if in.Year < 1900 || in.Year > 2100 {
		return fmt.Errorf("year out of range (1900-2100): %d", in.Year)
	}
	if in.Month < 1 || in.Month > 12 {
		return fmt.Errorf("month out of range: %d", in.Month)
	}
	if in.Day < 1 || in.Day > 31 {
		return fmt.Errorf("day out of range: %d", in.Day)
	}
	if in.Hour < 0 || in.Hour > 23 {
		return fmt.Errorf("hour out of range: %d", in.Hour)
	}
	if in.Minute < 0 || in.Minute > 59 {
		return fmt.Errorf("minute out of range: %d", in.Minute)
	}
	if in.Gender != GenderMale && in.Gender != GenderFemale {
		return fmt.Errorf("gender must be 1 (male) or 0 (female)")
	}
	if in.Longitude < -180 || in.Longitude > 180 {
		return fmt.Errorf("longitude out of range (-180..180): %f", in.Longitude)
	}
	return nil
}

// listToStrings converts a *list.List of strings (lunar-go's favourite
// return type) to a []string.
func listToStrings(l *list.List) []string {
	if l == nil {
		return nil
	}
	out := make([]string, 0, l.Len())
	for e := l.Front(); e != nil; e = e.Next() {
		out = append(out, e.Value.(string))
	}
	return out
}

// buildPillars assembles the four pillars from a lunar-go EightChar.
func buildPillars(ec *calendar.EightChar) []Pillar {
	mk := func(pos string, gan, zhi, ganzhi, wuxing, nayin string,
		hideGan []string, shiShenGan string, shiShenZhi []string,
		dishi, xun, xunkong string) Pillar {
		return Pillar{
			Position: pos, GanZhi: ganzhi, Gan: gan, Zhi: zhi,
			WuXing: wuxing, NaYin: nayin, HideGan: hideGan,
			ShiShenGan: shiShenGan, ShiShenZhi: shiShenZhi,
			DiShi: dishi, Xun: xun, XunKong: xunkong,
		}
	}
	pillars := []Pillar{
		mk("年", ec.GetYearGan(), ec.GetYearZhi(), ec.GetYear(),
			ec.GetYearWuXing(), ec.GetYearNaYin(),
			ec.GetYearHideGan(), ec.GetYearShiShenGan(),
			listToStrings(ec.GetYearShiShenZhi()),
			ec.GetYearDiShi(), ec.GetYearXun(), ec.GetYearXunKong()),
		mk("月", ec.GetMonthGan(), ec.GetMonthZhi(), ec.GetMonth(),
			ec.GetMonthWuXing(), ec.GetMonthNaYin(),
			ec.GetMonthHideGan(), ec.GetMonthShiShenGan(),
			listToStrings(ec.GetMonthShiShenZhi()),
			ec.GetMonthDiShi(), ec.GetMonthXun(), ec.GetMonthXunKong()),
		mk("日", ec.GetDayGan(), ec.GetDayZhi(), ec.GetDay(),
			ec.GetDayWuXing(), ec.GetDayNaYin(),
			ec.GetDayHideGan(), ec.GetDayShiShenGan(),
			listToStrings(ec.GetDayShiShenZhi()),
			ec.GetDayDiShi(), ec.GetDayXun(), ec.GetDayXunKong()),
		mk("时", ec.GetTimeGan(), ec.GetTimeZhi(), ec.GetTime(),
			ec.GetTimeWuXing(), ec.GetTimeNaYin(),
			ec.GetTimeHideGan(), ec.GetTimeShiShenGan(),
			listToStrings(ec.GetTimeShiShenZhi()),
			ec.GetTimeDiShi(), ec.GetTimeXun(), ec.GetTimeXunKong()),
	}
	return pillars
}

// buildDaYun turns lunar-go's DaYun slice into our DaYun struct,
// pulling 10 流年 under each. We emit 8 大运 after the pre-起运
// period (index 0), which covers 80 years — enough for a full life.
func buildDaYun(yun *calendar.Yun) []DaYun {
	dayuns := yun.GetDaYunBy(9) // index 0 = pre-起运, then 8 运
	out := make([]DaYun, 0, len(dayuns))
	for _, dy := range dayuns {
		entry := DaYun{
			Index:     dy.GetIndex(),
			GanZhi:    dy.GetGanZhi(),
			StartYear: dy.GetStartYear(),
			EndYear:   dy.GetEndYear(),
			StartAge:  dy.GetStartAge(),
			EndAge:    dy.GetEndAge(),
		}
		lns := dy.GetLiuNian()
		entry.LiuNian = make([]LiuNian, 0, len(lns))
		for _, ln := range lns {
			entry.LiuNian = append(entry.LiuNian, LiuNian{
				Year:   ln.GetYear(),
				Age:    ln.GetAge(),
				GanZhi: ln.GetGanZhi(),
			})
		}
		out = append(out, entry)
	}
	return out
}

// ─── 五行统计 / 旺衰 / 用神 ───────────────────────────────────────

// wuxingOfGanZhi returns the element of a single gan or zhi char.
func wuxingOfGan(gan string) string { return LunarUtil.WU_XING_GAN[gan] }
func wuxingOfZhi(zhi string) string { return LunarUtil.WU_XING_ZHI[zhi] }

// computeWuXingStats counts the eight gan-zhi chars (4 干 + 4 支) and
// the hidden stems' element contribution, then expresses each as a
// percentage of the total. Hidden stems count at half weight to avoid
// drowning out the main stems.
func computeWuXingStats(pillars []Pillar) []WuXingStat {
	counts := map[string]float64{"金": 0, "木": 0, "水": 0, "火": 0, "土": 0}
	add := func(el string, w float64) {
		if el == "" {
			return
		}
		counts[el] += w
	}
	for _, p := range pillars {
		add(wuxingOfGan(p.Gan), 1)
		add(wuxingOfZhi(p.Zhi), 1)
		for _, hg := range p.HideGan {
			add(wuxingOfGan(hg), 0.5)
		}
	}
	total := 0.0
	for _, v := range counts {
		total += v
	}
	stats := make([]WuXingStat, 0, 5)
	order := []string{"金", "木", "水", "火", "土"}
	for _, el := range order {
		pct := 0
		if total > 0 {
			pct = int(counts[el] / total * 100)
		}
		stats = append(stats, WuXingStat{
			Element: el,
			Count:   int(counts[el]),
			Percent: pct,
		})
	}
	return stats
}

// computeWangShuai judges 日主 旺衰 from the month branch (令) plus
// the raw element counts. This is the classic 启发式:
//   - 日主同五行 + 生日主五行 = 帮派; opposite = 耗派
//   - 月令为日主同五行或生五行 → 偏旺; 克/泄/耗 → 偏弱
//
// It is intentionally simple — the AI refines it in interpretation.
func computeWangShuai(pillars []Pillar, stats []WuXingStat) WangShuai {
	dayGan := pillars[2].Gan
	dayWX := wuxingOfGan(dayGan)              // 日主五行
	monthZhiWX := wuxingOfZhi(pillars[1].Zhi) // 月令五行

	// 生克关系: who generates whom, who restrains whom
	gen := map[string]string{"金": "水", "水": "木", "木": "火", "火": "土", "土": "金"}
	ke := map[string]string{"金": "木", "木": "土", "土": "水", "水": "火", "火": "金"}

	// 帮派 = 同五行 + 生日主五行
	bang := dayWX + gen[dayWX] // e.g. 日主木 → 帮派 = 木(同) + 水(生木)
	// 耗派 = 克日主 + 日主生 + 日主克
	hao := ke[dayWX] + gen[dayWX] + ke[gen[dayWX]]

	bangCount, haoCount := 0, 0
	for _, s := range stats {
		if strings.Contains(bang, s.Element) {
			bangCount += s.Count
		}
		if strings.Contains(hao, s.Element) {
			haoCount += s.Count
		}
	}

	dayWang := "平衡"
	if bangCount > haoCount+1 {
		dayWang = "偏旺"
	} else if haoCount > bangCount+1 {
		dayWang = "偏弱"
	}

	// 最旺 / 最弱 by raw count
	strong, weak := stats[0].Element, stats[0].Element
	for _, s := range stats {
		if s.Count > countOf(stats, strong) {
			strong = s.Element
		}
		if s.Count < countOf(stats, weak) {
			weak = s.Element
		}
	}

	summary := fmt.Sprintf("日主 %s（%s），月令 %s。%s：帮派 %d / 耗派 %d。",
		dayGan, dayWX, monthZhiWX, dayWang, bangCount, haoCount)

	return WangShuai{
		Strong:  strong,
		Weak:    weak,
		DayWang: dayWang,
		Summary: summary,
	}
}

// countOf looks up the raw count for an element in the stats slice.
func countOf(stats []WuXingStat, el string) int {
	for _, s := range stats {
		if s.Element == el {
			return s.Count
		}
	}
	return 0
}

// computeYongYin emits a 用神 preliminary hint based on 旺衰:
//   - 偏旺 → 用神取克/泄/耗 (官杀/食伤/财)
//   - 偏弱 → 用神取生/帮 (印/比)
//   - 平衡 → 用神取月令所透 / 暂取最弱五行补足
//
// Confidence is always "初判": this is a heuristic, not a substitute
// for a human 命理师.
func computeYongYin(pillars []Pillar, ws WangShuai) YongYin {
	dayWX := wuxingOfGan(pillars[2].Gan)
	gen := map[string]string{"金": "水", "水": "木", "木": "火", "火": "土", "土": "金"}
	ke := map[string]string{"金": "木", "木": "土", "土": "水", "水": "火", "火": "金"}

	// 生我的 = 印, 同我的 = 比, 我生的 = 食伤, 我克的 = 财, 克我的 = 官杀
	yin := gen[ke[dayWX]]  // 生我者
	bi := dayWX            // 同我者
	shi := gen[dayWX]      // 我生者
	cai := ke[dayWX]       // 我克者
	guan := ke[gen[dayWX]] // 克我者 ... wait, 克我者 = ke[dayWX] reverse
	// correct 克我: the element that 克 dayWX is the one whose 克 target is dayWX
	// ke[?] = dayWX → ? is the 克 dayWX source. Build reverse map:
	guan = ""
	for k, v := range ke {
		if v == dayWX {
			guan = k
			break
		}
	}

	yong, xi, ji := "", []string{}, []string{}
	reason := ""
	switch ws.DayWang {
	case "偏旺":
		// 旺则宜泄宜克: 用官杀(克)或食伤(泄)或财(耗)
		yong = guan
		xi = []string{shi, cai}
		ji = []string{yin, bi}
		reason = "日主偏旺，宜用官杀克之、食伤泄之、财星耗之；忌印比生扶。"
	case "偏弱":
		// 弱则宜生宜帮: 用印(生)或比(帮)
		yong = yin
		xi = []string{bi}
		ji = []string{guan, shi, cai}
		reason = "日主偏弱，宜用印星生之、比劫帮之；忌官杀克、食伤泄、财星耗。"
	default:
		// 平衡: 补最弱
		yong = ws.Weak
		xi = []string{ws.Weak}
		ji = []string{ws.Strong}
		reason = "日主大致平衡，初判补最弱五行以调候，需结合格局细审。"
	}

	return YongYin{
		YongShen:   yong,
		Xi:         xi,
		Ji:         ji,
		Confidence: "初判",
		Reason:     reason,
	}
}

// ─── 神煞 ─────────────────────────────────────────────────────────
// 神煞 tables. These are the classical lookup rules — deterministic
// and well-documented. We compute the common, high-value set:
//   - 天乙贵人 (day-stem → branches)
//   - 文昌 (day-stem → branch)
//   - 驿马 (year/day branch → branch)
//   - 桃花 (year/day branch → branch, 子午卯酉 pivot)
//   - 华盖 (year/day branch → branch)
//
// Each is attached to whichever pillar(s) carry the trigger gan/zhi.

var tianYiGuiRen = map[string][]string{
	"甲": {"丑", "未"}, "戊": {"丑", "未"},
	"乙": {"子", "申"}, "己": {"子", "申"},
	"丙": {"酉", "亥"}, "丁": {"酉", "亥"},
	"庚": {"丑", "未"}, "辛": {"寅", "午"},
	"壬": {"卯", "巳"}, "癸": {"卯", "巳"},
}

var wenChang = map[string]string{
	"甲": "巳", "乙": "午", "丙": "申", "丁": "酉",
	"戊": "申", "己": "酉", "庚": "亥", "辛": "子",
	"壬": "寅", "癸": "卯",
}

// 驿马: 三合局第一支的冲
var yiMa = map[string]string{
	"申": "寅", "子": "寅", "辰": "寅",
	"寅": "申", "午": "申", "戌": "申",
	"巳": "亥", "酉": "亥", "丑": "亥",
	"亥": "巳", "卯": "巳", "未": "巳",
}

// 桃花: 三合局中支
var taoHua = map[string]string{
	"申": "酉", "子": "酉", "辰": "酉",
	"寅": "卯", "午": "卯", "戌": "卯",
	"巳": "午", "酉": "午", "丑": "午",
	"亥": "子", "卯": "子", "未": "子",
}

// 华盖: 三合局末支
var huaGai = map[string]string{
	"申": "辰", "子": "辰", "辰": "辰",
	"寅": "戌", "午": "戌", "戌": "戌",
	"巳": "丑", "酉": "丑", "丑": "丑",
	"亥": "未", "卯": "未", "未": "未",
}

// computeShenSha walks the four pillars and emits 神煞 entries for
// every match. The same 神煞 can appear on multiple pillars (e.g.
// 桃花 may attach to both 年支 and 日支); we keep all of them so the
// chart is faithful.
func computeShenSha(pillars []Pillar) []ShenSha {
	var out []ShenSha
	add := func(name, pos, trigger, note string) {
		out = append(out, ShenSha{
			Name: name, Position: pos, GanZhi: trigger, Note: note,
		})
	}

	dayGan := pillars[2].Gan
	for _, p := range pillars {
		// 天乙贵人 (by day stem) — only counts when found on a pillar
		// whose branch matches; the trigger is the day stem, the
		// location is the pillar carrying the lucky branch.
		if lucky, ok := tianYiGuiRen[dayGan]; ok {
			for _, b := range lucky {
				if p.Zhi == b {
					add("天乙贵人", p.Position, p.GanZhi,
						"逢凶化吉之吉神，主贵人相助")
				}
			}
		}
		// 文昌 (by day stem)
		if p.Zhi == wenChang[dayGan] {
			add("文昌", p.Position, p.GanZhi, "主聪明好学、文采出众")
		}
		// 驿马 / 桃花 / 华盖 use 年支 and 日支 as the usual triggers
		if p.Position == "年" || p.Position == "日" {
			if m, ok := yiMa[p.Zhi]; ok && p.Zhi != m {
				// 驿马 appears when the OPPOSITE branch is present
				for _, q := range pillars {
					if q.Zhi == m {
						add("驿马", q.Position, q.GanZhi, "主奔波、迁徙、外出之象")
					}
				}
			}
			if t, ok := taoHua[p.Zhi]; ok {
				for _, q := range pillars {
					if q.Zhi == t {
						add("桃花", q.Position, q.GanZhi, "主人缘、感情、异性缘")
					}
				}
			}
			if h, ok := huaGai[p.Zhi]; ok {
				for _, q := range pillars {
					if q.Zhi == h {
						add("华盖", q.Position, q.GanZhi, "主孤高、艺术、宗教缘")
					}
				}
			}
		}
	}

	// de-dup by (name, position, ganZhi)
	seen := map[string]bool{}
	dedup := out[:0]
	for _, s := range out {
		k := s.Name + "|" + s.Position + "|" + s.GanZhi
		if seen[k] {
			continue
		}
		seen[k] = true
		dedup = append(dedup, s)
	}
	sort.Slice(dedup, func(i, j int) bool {
		if dedup[i].Name != dedup[j].Name {
			return dedup[i].Name < dedup[j].Name
		}
		return dedup[i].Position < dedup[j].Position
	})
	return dedup
}

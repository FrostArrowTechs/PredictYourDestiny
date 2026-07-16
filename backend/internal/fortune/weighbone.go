// Package fortune provides the 称骨算命 (Bone Weight Fortune) engine.
//
// 称骨算命 is a traditional Chinese fortune-telling method attributed
// to 袁天罡 (Yuan Tiantang, Tang Dynasty). It assigns a "bone weight"
// (骨重) to each birth year, month, day, and hour (时辰), sums them,
// and looks up a fortune poem (称骨歌) for the total weight.
//
// All rules and poems below are public-domain traditional content.
// The engine is pure calculation; AI interpretation adds depth.
package fortune

import (
	"fmt"

	"github.com/6tail/lunar-go/calendar"
)

// WeighboneEngine computes bone-weight fortune.
type WeighboneEngine struct{}

// WeighboneChart is the structured result.
type WeighboneChart struct {
	// Per-component weights (in 钱, 10 钱 = 1 两)
	YearWeight  string `json:"yearWeight"`  // e.g. "7钱"
	MonthWeight string `json:"monthWeight"` // e.g. "6钱"
	DayWeight   string `json:"dayWeight"`   // e.g. "5钱"
	HourWeight  string `json:"hourWeight"`  // e.g. "9钱"

	// Total weight in 两钱 format, e.g. "二两三钱"
	TotalWeight string `json:"totalWeight"`
	// Total weight in numeric 钱 (10 = 1 两), e.g. 23
	TotalQian int `json:"totalQian"`

	// The fortune poem for this weight
	Poem string `json:"poem"`
	// Fortune category: 上/中上/中/中下/下
	Category string `json:"category"`
	// Brief fortune description
	Description string `json:"description"`
}

// Name returns the engine identifier.
func (e WeighboneEngine) Name() string { return KindWeighbone }

// Compute calculates bone-weight fortune from birth date/time.
func (e WeighboneEngine) Compute(in Input) (*Result, error) {
	if in.Year < 1900 || in.Year > 2100 {
		return nil, errYearRange
	}
	if in.Month < 1 || in.Month > 12 {
		return nil, errMonthRange
	}
	if in.Day < 1 || in.Day > 31 {
		return nil, errDayRange
	}

	solar := calendar.NewSolar(in.Year, in.Month, in.Day, in.Hour, in.Minute, 0)
	lunar := solar.GetLunar()

	// Get lunar year number, lunar month, lunar day, and 时辰 index
	lunarYear := lunar.GetYear()
	lunarMonth := lunar.GetMonth()
	lunarDay := lunar.GetDay()
	// 时辰: 0=子(23-1), 1=丑(1-3), ... via lunar.GetHourZhiIndex or computed
	zhiIndex := getHourZhiIndex(in.Hour)

	// Look up weights
	yearW := boneWeightYear(lunarYear)
	monthW := boneWeightMonth(lunarMonth)
	dayW := boneWeightDay(lunarDay)
	hourW := boneWeightHour(zhiIndex)

	totalQian := yearW + monthW + dayW + hourW
	totalWeightStr := qianToLiangQian(totalQian)

	poem, category, desc := bonePoem(totalQian)

	chart := &WeighboneChart{
		YearWeight:   qianToStr(yearW),
		MonthWeight:  qianToStr(monthW),
		DayWeight:    qianToStr(dayW),
		HourWeight:   qianToStr(hourW),
		TotalWeight:  totalWeightStr,
		TotalQian:    totalQian,
		Poem:         poem,
		Category:     category,
		Description:  desc,
	}

	return &Result{
		Kind: KindWeighbone,
		Data: chart,
		Meta: map[string]string{
			"lunarYear":  fmt.Sprintf("%d", lunarYear),
			"lunarMonth": fmt.Sprintf("%d", lunarMonth),
			"lunarDay":   fmt.Sprintf("%d", lunarDay),
			"method":     "袁天罡称骨算命",
		},
	}, nil
}

func init() {
	Register(WeighboneEngine{})
}

// getHourZhiIndex converts clock hour (0-23) to 时辰 index (0-11).
// 子时 = 23:00-00:59, 丑时 = 01:00-02:59, ...
// index 0 = 子, 1 = 丑, 2 = 寅, ...
func getHourZhiIndex(hour int) int {
	if hour == 23 || hour == 0 {
		return 0 // 子
	}
	return (hour + 1) / 2
}

// qianToStr formats a 钱 value as "X钱" or "X两Y钱".
// Input is in 钱 units (1 两 = 10 钱).
func qianToStr(qian int) string {
	liang := qian / 10
	yu := qian % 10
	if liang > 0 {
		return fmt.Sprintf("%s%s", numToCh(liang), liangYuToCh(yu))
	}
	return numToCh(yu) + "钱"
}

// qianToLiangQian formats total as "X两Y钱" in Chinese.
// When the remainder is 0, omit the 钱 part (e.g. "三两" not "三两零钱").
func qianToLiangQian(qian int) string {
	liang := qian / 10
	yu := qian % 10
	if yu == 0 {
		return fmt.Sprintf("%s两", numToCh(liang))
	}
	return fmt.Sprintf("%s两%s", numToCh(liang), numToCh(yu)+"钱")
}

// numToCh converts 1-9, 0 to Chinese characters.
func numToCh(n int) string {
	chs := []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}
	if n >= 0 && n <= 9 {
		return chs[n]
	}
	return "?"
}

// liangYuToCh formats the remainder 钱 part after 两.
func liangYuToCh(yu int) string {
	if yu == 0 {
		return "两"
	}
	return "两" + numToCh(yu) + "钱"
}

// ─── Bone weight lookup tables (traditional, public domain) ───────

// boneWeightYear returns the bone weight (in 钱) for a lunar year.
// Uses the 60-year sexagenary cycle (甲子).
// Returns weight in 钱 units (e.g. 7 = 7钱, 12 = 1两2钱).
func boneWeightYear(lunarYear int) int {
	// 鼠/牛/虎/兔/龙/蛇/马/羊/猴/鸡/狗/猪 × 60 cycle
	// The traditional table maps each year in the 60-cycle to a weight.
	// We use the well-known table indexed by year mod 60.
	weights := []int{
		// 甲子7 乙丑9 丙寅6 丁卯7 戊辰1 己巳5 庚午7 辛未6 壬申7 癸酉8
		// 甲戌1 乙亥8 丙子1 丁丑6 戊寅8 己卯7 庚辰8 辛巳7 壬午6 癸未7
		// 甲申5 乙酉5 丙戌6 丁亥7 戊子1 己丑5 庚寅9 辛卯8 壬辰1 癸巳7
		// 甲午1 乙未6 丙申5 丁酉1 戊戌1 己亥9 庚子7 辛丑7 壬寅9 癸卯1
		// 甲辰8 乙巳7 丙午1 丁未5 戊申1 己酉5 庚戌9 辛亥1 壬子7 癸丑7
		// 甲寅8 乙卯8 丙辰8 丁巳6 戊午1 己未6 庚申8 辛酉9 壬戌7 癸亥6
		7, 9, 6, 7, 12, 5, 7, 6, 7, 8, // 甲子..癸酉
		12, 8, 12, 6, 8, 7, 8, 7, 6, 7, // 甲戌..癸未 (戊辰=1两2钱=12)
		5, 5, 6, 7, 12, 5, 9, 8, 12, 7, // 甲申..癸巳
		12, 6, 5, 12, 12, 9, 7, 7, 9, 12, // 甲午..癸卯
		8, 7, 12, 5, 12, 5, 9, 12, 7, 7, // 甲辰..癸丑
		8, 8, 8, 6, 12, 6, 8, 9, 7, 6, // 甲寅..癸亥
	}
	idx := (lunarYear - 4) % 60 // 甲子年 = 4 (year 4 AD)
	if idx < 0 {
		idx += 60
	}
	return weights[idx]
}

// boneWeightMonth returns the bone weight for a lunar month (1-12).
func boneWeightMonth(month int) int {
	// 正月6 二月7 三月18 四月9 五月5 六月16 七月9 八月5 九月18 十月8 十一月9 十二月5
	weights := []int{0, 6, 7, 18, 9, 5, 16, 9, 5, 18, 8, 9, 5}
	if month >= 1 && month <= 12 {
		return weights[month]
	}
	return 0
}

// boneWeightDay returns the bone weight for a lunar day (1-30).
func boneWeightDay(day int) int {
	// Traditional 30-day table (in 钱; values >10 = 两+钱)
	weights := []int{
		0,
		5,  // 初一
		10, // 初二 (1两)
		8,  // 初三
		17, // 初四 (1两7钱)
		15, // 初五 (1两5钱)
		16, // 初六 (1两6钱)
		9,  // 初七
		16, // 初八 (1两6钱)
		8,  // 初九
		16, // 初十 (1两6钱)
		9,  // 十一
		17, // 十二 (1两7钱)
		8,  // 十三
		17, // 十四 (1两7钱)
		10, // 十五 (1两)
		8,  // 十六
		9,  // 十七
		18, // 十八 (1两8钱)
		5,  // 十九
		15, // 二十 (1两5钱)
		10, // 二十一 (1两)
		9,  // 二十二
		8,  // 二十三
		9,  // 二十四
		18, // 二十五 (1两8钱)
		7,  // 二十六
		7,  // 二十七
		8,  // 二十八
		16, // 二十九 (1两6钱)
		6,  // 三十
	}
	if day >= 1 && day <= 30 {
		return weights[day]
	}
	return 0
}

// boneWeightHour returns the bone weight for a 时辰 (0=子..11=亥).
func boneWeightHour(zhiIndex int) int {
	// 子时1两6 丑时6 寅时7 卯时8 辰时9 巳时1两6 午时1两 未时8 申时5 酉时9 戌时6 亥时6
	weights := []int{16, 6, 7, 8, 9, 16, 10, 8, 5, 9, 6, 6}
	if zhiIndex >= 0 && zhiIndex <= 11 {
		return weights[zhiIndex]
	}
	return 0
}

// bonePoem returns the fortune poem, category, and description for a total
// bone weight (in 钱 units). Range: 21 (二两一) to 71 (七两一).
func bonePoem(totalQian int) (poem, category, description string) {
	type entry struct {
		poem string
		cat  string
		desc string
	}
	table := map[int]entry{
		21: {"短命非业谓大空，平生灾难事重重，凡事谨防多忍耐，命中辛苦自难逢。", "下下", "此命福气不足，一生多劳碌辛苦，宜修身养性，积德行善。"},
		22: {"身寒骨冷苦伶仃，此命推来行乞人，碌碌一生无大用，到老定是难安宁。", "下下", "此命一生辛苦，凡事需要自己努力，宜踏实本分。"},
		23: {"此命推来骨格轻，求谋作事事难成，妻儿兄弟应难许，别处他乡作散人。", "下下", "此命根基较浅，宜离乡发展，凡事需多努力。"},
		24: {"此命推来福禄无，门庭困苦总难营，妻儿无靠常啼哭，独自一身烦恼多。", "下下", "此命福禄较薄，宜自立自强，中年方有转机。"},
		25: {"此命推来事不成，妻儿兄弟少六亲，别处他乡随处好，一生劳碌自主张。", "下下", "此命宜离乡发展，独立自主，中年可渐入佳境。"},
		26: {"此命推来福不轻，居家事事自然成，妻儿兄弟无依靠，只可自持衣食丰。", "中下", "此命衣食无忧，但亲缘较薄，宜靠自己打拼。"},
		27: {"一生做事少商量，难靠祖宗作主张，独马单枪空作去，早年晚岁总无长。", "中下", "此命独立性强，凡事亲力亲为，中年渐有积蓄。"},
		28: {"一身作事少商量，妻子坚牢甚不强，独马单枪空作去，早年晚岁总无长。", "中下", "此命做事需多与人商量，不宜独断，宜中年发奋。"},
		29: {"初年运限未曾享，纵有功名在后头，须过四十方可称，移居改姓始为良。", "中下", "此命早年不顺，四十岁后渐入佳境，宜中年发奋。"},
		30: {"此命生来性善良，心慈面善惹人亲，劳心劳力命不差，自当衣食福绵长。", "中", "此命为人善良，衣食不愁，一生平稳安定。"},
		31: {"忙忙碌碌苦中求，何日云开见日头，难得祖基家可立，中年衣食渐无忧。", "中", "此命中年方顺，早年辛苦，宜坚持奋斗。"},
		32: {"初年运限事难谋，渐有财源如水流，牵来时事心中想，金玉满堂命里收。", "中", "此命中年发运，财源渐丰，宜把握时机。"},
		33: {"早年做事事难成，百年勤劳枉费心，半世自如流水去，后来运到始得金。", "中", "此命半生辛苦，后半生渐顺，宜耐心等待。"},
		34: {"此命福气果如何，僧道门中衣禄多，离祖出家方得稳，终朝拜佛念弥陀。", "中", "此命宜修身养性，福禄在于清心寡欲。"},
		35: {"生平福量不周全，祖业根基觉少传，营事生涯宜守旧，时来福禄自双全。", "中", "此命宜稳中求进，不可冒进，时来运转。"},
		36: {"不须劳碌过平生，独自成家福不轻，早有福星常照命，任君行去百般成。", "中上", "此命福气不差，成家立业顺遂，宜把握机遇。"},
		37: {"此命终身运不通，劳劳作事尽皆空，苦心竭力成家计，到得那时在梦中。", "中", "此命做事多波折，宜脚踏实地，不可空想。"},
		38: {"一生骨肉最清高，早入黉门姓名标，待看年将三十六，蓝衫脱去换红袍。", "中上", "此命学业有成，中年发达，宜读书进取。"},
		39: {"此命推来福不轻，自成自立显门庭，从来富贵人钦敬，使婢差奴过一生。", "中上", "此命自成自立，富贵双全，一生受人尊敬。"},
		40: {"为名为利终日劳，中年福禄也多遭，老来稍可宽怀抱，处世从容是英豪。", "中上", "此命中年多劳，晚年安乐，宜从容处事。"},
		41: {"此命推来事不奇，劳心劳力布东西，中年还有逍遥福，不比前番目下时。", "中上", "此命中年顺遂，宜把握中年的大好时光。"},
		42: {"得宽怀处且宽怀，何必双眉皱不开，若使中年命运济，那时名利一起来。", "中上", "此命中年发运，名利双收，宜放宽心态。"},
		43: {"为人心性最聪明，作事轩昂近贵人，衣禄一生天数定，不须劳碌是丰享。", "中上", "此命聪明伶俐，衣禄丰足，近贵人。"},
		44: {"万事由天莫强求，何须苦苦怨他人，中年福禄虽云有，寿数登时五十秋。", "中上", "此命中年有福，宜顺其自然，不可强求。"},
		45: {"名利推求竭力图，朝朝役役与时违，不知否泰皆由命，只恐荣华不到头。", "中上", "此命宜知足常乐，荣华虽好，更需稳健。"},
		46: {"东西南北尽皆通，出姓移名更觉隆，衣禄无亏天数定，中年晚景一般同。", "中上", "此命衣食无忧，一生通达，宜外出发展。"},
		47: {"此命推来旺末年，妻荣子贵自怡然，平生原有滔滔福，可有财源若水流。", "中上", "此命晚年大旺，妻荣子贵，财源广进。"},
		48: {"幼年运道未曾享，墓运财源永不通，中年还有不顺事，晚景欣然便不同。", "中上", "此命早年不顺，晚年大好，宜坚持。"},
		49: {"此命推来福不轻，中年清高声誉传，有志方为人上业，无心枉自百劳神。", "中上", "此命有志者事竟成，宜立志高远。"},
		50: {"为名为利终日劳，中年福禄也多遭，老来稍可宽怀抱，处世从容是英豪。", "中上", "此命中年辛劳，晚年安乐，宜从容。"},
		51: {"一世荣华事事通，不须劳碌自亨通，平生衣禄生来好，奴仆成家自有功。", "上", "此命一世荣华，不须劳碌，富贵自来。"},
		52: {"一世享荣华，自有牙床象笏家，饥有珍馐百味，不愁唯独有荣华。", "上", "此命富贵荣华，衣食丰足，一生享福。"},
		53: {"此命推来本性纯，心慈面善贵人钦，天然富贵真根固，槐影森森绿满庭。", "上", "此命心慈面善，贵人相助，富贵双全。"},
		54: {"此格推来气象真，兴家发达在其中，一生福禄安然好，处世逍遥自在游。", "上", "此命兴家发达，福禄双全，逍遥自在。"},
		55: {"走马扬鞭争名利，少年做事费筹论，一朝福禄源源至，富贵荣华显六亲。", "上", "此命少年辛苦，中年大富大贵，显耀六亲。"},
		56: {"此格推来禄数奇，光辉宗祖耀门闾，一生豪富多康宁，半世风清自怡然。", "上", "此命富贵康宁，光宗耀祖，一生豪富。"},
		57: {"福禄丰盈万事全，一身荣耀乐天年，平生原有滔滔福，可有财源若水流。", "上", "此命福禄双全，一身荣耀，财源广进。"},
		58: {"平生福量不周全，祖业根基觉少传，营事生涯宜守旧，时来福禄自双全。", "上", "此命福禄自天，宜守成，时来运转。"},
		59: {"细推此命福非轻，富贵荣华孰与争，定有玉堂金榜客，龙楼凤阁三人行。", "上", "此命富贵荣华，定是显贵之人。"},
		60: {"一朝金榜快题名，显祖荣宗豁眼明，茅屋中生白玉柱，十年不种发青麻。", "上", "此命金榜题名，显祖荣宗，大富大贵。"},
		61: {"不作朝中金榜客，定为世上大财翁，聪明天赋经书熟，名播千秋四海中。", "上", "此命聪慧过人，定为世上大富之人。"},
		62: {"此命生来福不轻，鸿鹄之志必定成，正是人间龙凤客，飞腾六合显家声。", "上", "此命鸿鹄之志，必成大器，飞黄腾达。"},
		63: {"不愁家中受饥寒，妻贤夫贵两团圆，自是门庭喜气新，改变家风满户春。", "上", "此命妻贤夫贵，门庭兴旺，喜气满堂。"},
		64: {"此命生来福不轻，正是鹤立鸡群人，文武双全皆有分，成家立业贵人钦。", "上", "此命文武双全，鹤立鸡群，贵人钦敬。"},
		65: {"细推此命福不轻，定是恩荣作贵人，文学英才声并起，头角峥嵘三品尊。", "上", "此命文采出众，官居高位，显赫一时。"},
		66: {"此格推来福甚多，贵气恢恢出贵人，名利双全皆如意，世代荣华福禄臻。", "上", "此命福禄甚多，名利双全，世代荣华。"},
		67: {"此命生来福甚宏，诗书满腹是英雄，一生荣华财源旺，福禄绵绵万事通。", "上", "此命满腹诗书，一生荣华，财源旺盛。"},
		68: {"富贵荣华大不同，鬓眉之间显隆隆，平生福禄真无敌，富贵荣华直到终。", "上", "此命福禄无敌，富贵荣华直到终老。"},
		69: {"细推此命福无边，紫袍金带御阶前，正是人间龙凤客，一呼百诺福绵绵。", "上", "此命紫袍金带，权势显赫，福泽无边。"},
		70: {"此命生来福禄宏，定为人间栋梁材，文武双全皆有分，一呼百诺位三台。", "上", "此命文武全才，位高权重，栋梁之材。"},
		71: {"此命生来福寿长，不愁衣食在堂堂，平生福禄滔滔至，富贵荣华万代昌。", "上上", "此命福寿绵长，富贵荣华，万代昌盛。"},
	}

	if e, ok := table[totalQian]; ok {
		return e.poem, e.cat, e.desc
	}
	// Fallback for out-of-range weights
	return "命理独特，非寻常歌诀所能尽述，宜请名师详参。", "中", "此骨重超出常见范围，需结合八字详参。"
}

// Package fortune provides the huangli (黄历) calendar engine.
//
// The engine uses lunar-go to compute traditional Chinese calendar
// information for a given date, including:
//   - 农历日期
//   - 干支纪日
//   - 宜/忌
//   - 吉神/凶煞
//   - 彭祖百忌
//   - 冲/煞
//   - 五行/纳音
//   - 星座
//   - 二十八宿
//
// This is a pure calculation engine with no AI component. The AI
// interpret endpoint can provide personalized advice for choosing
// auspicious dates for specific activities.
package fortune

import (
	"container/list"
	"errors"
	"fmt"
	"strings"

	"github.com/6tail/lunar-go/calendar"
)

// Validation errors
var (
	errYearRange  = errors.New("year must be between 1900 and 2100")
	errMonthRange = errors.New("month must be between 1 and 12")
	errDayRange   = errors.New("day must be between 1 and 31")
)

// HuangliEngine computes traditional Chinese calendar data.
type HuangliEngine struct{}

// HuangliChart is the structured result for a given date.
type HuangliChart struct {
	// 公历
	Solar string `json:"solar"`
	// 农历
	Lunar string `json:"lunar"`
	// 年干支
	YearGanZhi string `json:"yearGanZhi"`
	// 月干支
	MonthGanZhi string `json:"monthGanZhi"`
	// 日干支
	DayGanZhi string `json:"dayGanZhi"`
	// 宜
	Yi []string `json:"yi"`
	// 忌
	Ji []string `json:"ji"`
	// 吉神
	JiShen []string `json:"jiShen"`
	// 凶煞
	XiongSha []string `json:"xiongSha"`
	// 彭祖百忌
	PengZu string `json:"pengZu"`
	// 冲
	Chong string `json:"chong"`
	// 煞
	Sha string `json:"sha"`
	// 五行
	WuXing string `json:"wuXing"`
	// 纳音
	NaYin string `json:"naYin"`
	// 星座
	XingZuo string `json:"xingZuo"`
	// 二十八宿
	ErShiBaXiu string `json:"erShiBaXiu"`
	// 月相
	YueXiang string `json:"yueXiang"`
	// 节气
	JieQi string `json:"jieQi"`
	// 星期
	Week string `json:"week"`
	// 今日胎神
	TaiShen string `json:"taiShen"`
}

// Name returns the engine identifier.
func (e HuangliEngine) Name() string { return KindHuangli }

// Compute returns the huangli data for the given date.
func (e HuangliEngine) Compute(in Input) (*Result, error) {
	// Validate input
	if in.Year < 1900 || in.Year > 2100 {
		return nil, errYearRange
	}
	if in.Month < 1 || in.Month > 12 {
		return nil, errMonthRange
	}
	if in.Day < 1 || in.Day > 31 {
		return nil, errDayRange
	}

	// Create solar date
	solar := calendar.NewSolar(in.Year, in.Month, in.Day, 0, 0, 0)
	lunar := solar.GetLunar()

	// Build chart - use correct API methods that return strings directly
	chart := &HuangliChart{
		Solar:       solar.ToFullString(),
		Lunar:       lunar.ToFullString(),
		YearGanZhi:  lunar.GetYearInGanZhi(),
		MonthGanZhi: lunar.GetMonthInGanZhi(),
		DayGanZhi:   lunar.GetDayInGanZhi(),
		Yi:          huangliListToStrings(lunar.GetDayYi()),
		Ji:          huangliListToStrings(lunar.GetDayJi()),
		JiShen:      huangliListToStrings(lunar.GetDayJiShen()),
		XiongSha:    huangliListToStrings(lunar.GetDayXiongSha()),
		PengZu:      lunar.GetPengZuGan() + " " + lunar.GetPengZuZhi(),
		Chong:       lunar.GetDayChong(),
		Sha:         lunar.GetDaySha(),
		WuXing:      lunar.GetDayGan() + lunar.GetDayZhi(),
		NaYin:       lunar.GetDayNaYin(),
		XingZuo:     solar.GetXingZuo(),
		ErShiBaXiu:  lunar.GetXiu(),
		YueXiang:    lunar.GetYueXiang(),
		JieQi:       lunar.GetJieQi(),
		Week:        solar.GetWeekInChinese(),
		TaiShen:     lunar.GetDayPositionTai(),
	}

	return &Result{
		Kind: KindHuangli,
		Data: chart,
		Meta: map[string]string{
			"date": solar.ToFullString(),
		},
	}, nil
}

func init() {
	Register(HuangliEngine{})
}

// huangliListToStrings converts lunar-go's *list.List to []string.
func huangliListToStrings(l *list.List) []string {
	if l == nil {
		return nil
	}
	out := make([]string, 0, l.Len())
	for e := l.Front(); e != nil; e = e.Next() {
		if s, ok := e.Value.(string); ok {
			out = append(out, strings.TrimSpace(s))
		} else {
			out = append(out, strings.TrimSpace(fmt.Sprintf("%v", e.Value)))
		}
	}
	return out
}
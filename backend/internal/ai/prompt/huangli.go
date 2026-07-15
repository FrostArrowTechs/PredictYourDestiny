// Package prompt provides the huangli (黄历) prompt builder.
//
// The prompt helps users understand the auspicious/inauspicious aspects
// of a date and provides advice for choosing dates for specific activities.
package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

// huangliLang bundles per-language strings for huangli prompts.
type huangliLang struct {
	langName   string
	persona    string
	rules      []string
	disclaimer string
	briefHint  string
	deepHint   string
}

var huangliLangs = map[string]huangliLang{
	"zh-CN": {
		langName: "简体中文",
		persona:  "你是一位精通传统黄历与择吉学的专家，熟悉宜忌、吉神凶煞、彭祖百忌等传统历法知识，擅长为用户选择吉日提供专业建议。",
		rules: []string{
			"使用简体中文回答",
			"结合黄历数据，给出客观、理性的分析",
			"避免迷信色彩，强调传统文化参考价值",
			"不输出医疗、投资等领域的专业建议",
			"结尾附简短免责声明",
		},
		disclaimer: "黄历仅供参考，请结合实际情况做出决策。",
		briefHint:  "请简要分析今日黄历特点，给出适合和不适合的活动建议（约 200-300 字）。",
		deepHint:   "请详细分析今日黄历，结合吉神凶煞、宜忌事项，为用户提供择日建议。若有特定活动需求，请给出近期更适合的日期建议。",
	},
	"zh-TW": {
		langName: "繁體中文",
		persona:  "你是一位精通傳統黃曆與擇吉學的專家，熟悉宜忌、吉神凶煞、彭祖百忌等傳統曆法知識，擅長為用戶選擇吉日提供專業建議。",
		rules: []string{
			"使用繁體中文回答",
			"結合黃曆數據，給出客觀、理性的分析",
			"避免迷信色彩，強調傳統文化參考價值",
			"不輸出醫療、投資等領域的專業建議",
			"結尾附簡短免責聲明",
		},
		disclaimer: "黃曆僅供參考，請結合實際情況做出決策。",
		briefHint:  "請簡要分析今日黃曆特點，給出適合和不適合的活動建議（約 200-300 字）。",
		deepHint:   "請詳細分析今日黃曆，結合吉神凶煞、宜忌事項，為用戶提供擇日建議。若有特定活動需求，請給出近期更適合的日期建議。",
	},
}

func resolveHuangliLang(lang string) huangliLang {
	if l, ok := huangliLangs[lang]; ok {
		return l
	}
	return huangliLangs["zh-CN"]
}

// HuangliBuild constructs the AI prompt for a huangli result.
func HuangliBuild(in fortune.Input, res *fortune.Result) (*fortune.PromptSpec, error) {
	if res == nil || res.Kind != fortune.KindHuangli {
		return nil, fmt.Errorf("prompt/huangli: expected kind %q, got %q", fortune.KindHuangli, kindOrEmpty(res))
	}
	chart, ok := res.Data.(*fortune.HuangliChart)
	if !ok {
		return nil, fmt.Errorf("prompt/huangli: result.Data is not *fortune.HuangliChart (got %T)", res.Data)
	}

	depth := in.InterpretDepth
	if depth == "" {
		depth = fortune.DepthBrief
	}
	tier := fortune.TierFree
	if depth == fortune.DepthDeep {
		tier = fortune.TierPaid
	}

	L := resolveHuangliLang(in.Lang)
	system := buildHuangliSystem(L)
	user := buildHuangliUser(L, chart, depth, in.Question)

	return &fortune.PromptSpec{
		System: system,
		User:   user,
		Tier:   tier,
	}, nil
}

func buildHuangliSystem(L huangliLang) string {
	var b strings.Builder
	b.WriteString(L.persona)
	b.WriteString("\n\n请根据用户提供的黄历数据进行解读。要求：\n")
	for i, r := range L.rules {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	fmt.Fprintf(&b, "\n免责声明：%s", L.disclaimer)
	return b.String()
}

func buildHuangliUser(L huangliLang, c *fortune.HuangliChart, depth string, activity string) string {
	var b strings.Builder
	b.WriteString("【黄历信息】\n")
	b.WriteString("公历：" + c.Solar + "\n")
	b.WriteString("农历：" + c.Lunar + "\n")
	b.WriteString("干支：" + c.YearGanZhi + "年 " + c.MonthGanZhi + "月 " + c.DayGanZhi + "日\n")
	b.WriteString("五行：" + c.WuXing + " 纳音：" + c.NaYin + "\n")
	b.WriteString("星期：" + c.Week + " 星座：" + c.XingZuo + "\n")

	if len(c.Yi) > 0 {
		b.WriteString("\n【宜】\n")
		b.WriteString(strings.Join(c.Yi, "、") + "\n")
	}
	if len(c.Ji) > 0 {
		b.WriteString("\n【忌】\n")
		b.WriteString(strings.Join(c.Ji, "、") + "\n")
	}

	if len(c.JiShen) > 0 {
		b.WriteString("\n【吉神】\n")
		b.WriteString(strings.Join(c.JiShen, "、") + "\n")
	}
	if len(c.XiongSha) > 0 {
		b.WriteString("\n【凶煞】\n")
		b.WriteString(strings.Join(c.XiongSha, "、") + "\n")
	}

	b.WriteString("\n【其他】\n")
	b.WriteString("彭祖百忌：" + c.PengZu + "\n")
	b.WriteString("冲：" + c.Chong + " 煞：" + c.Sha + "\n")
	b.WriteString("二十八宿：" + c.ErShiBaXiu + "\n")

	if activity != "" {
		b.WriteString("\n【用户需求】\n")
		b.WriteString("用户计划：" + activity + "\n")
		b.WriteString("请结合今日黄历，分析此活动是否适宜，若不适宜请建议其他日期。\n")
	}

	b.WriteString("\n【解读要求】\n")
	if depth == fortune.DepthDeep {
		b.WriteString(L.deepHint)
	} else {
		b.WriteString(L.briefHint)
	}
	return b.String()
}
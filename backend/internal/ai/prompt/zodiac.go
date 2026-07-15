// Package prompt provides the zodiac fortune prompt builder.
package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

type zodiacLang struct {
	langName   string
	persona    string
	rules      []string
	disclaimer string
	briefHint  string
	deepHint   string
}

var zodiacLangs = map[string]zodiacLang{
	"zh-CN": {
		langName: "简体中文",
		persona:  "你是一位精通中国传统生肖文化的运势分析师，擅长根据生肖特性、流年关系为用户提供运势解读和生活建议。",
		rules: []string{
			"使用简体中文回答",
			"语气积极正面，强调趋吉避凶的方法",
			"结合计算结果中的分数和关系给出合理解读",
			"避免迷信色彩，强调传统文化参考价值",
			"结尾附简短免责声明",
		},
		disclaimer: "运势仅供参考，人生掌握在自己手中。",
		briefHint:  "请简要解读今年运势，给出关键建议（约 200-300 字）。",
		deepHint:   "请详细分析今年各方面运势，结合生肖特性、流年关系，给出具体的生活、事业、感情建议（约 500-800 字）。",
	},
	"zh-TW": {
		langName: "繁體中文",
		persona:  "你是一位精通中國傳統生肖文化的運勢分析師，擅長根據生肖特性、流年關係為用戶提供運勢解讀和生活建議。",
		rules: []string{
			"使用繁體中文回答",
			"語氣積極正面，強調趨吉避凶的方法",
			"結合計算結果中的分數和關係給出合理解讀",
			"避免迷信色彩，強調傳統文化參考價值",
			"結尾附簡短免責聲明",
		},
		disclaimer: "運勢僅供參考，人生掌握在自己手中。",
		briefHint:  "請簡要解讀今年運勢，給出關鍵建議（約 200-300 字）。",
		deepHint:   "請詳細分析今年各方面運勢，結合生肖特性、流年關係，給出具體的生活、事業、感情建議（約 500-800 字）。",
	},
}

func resolveZodiacLang(lang string) zodiacLang {
	if l, ok := zodiacLangs[lang]; ok {
		return l
	}
	return zodiacLangs["zh-CN"]
}

// ZodiacBuild constructs the AI prompt for zodiac fortune.
func ZodiacBuild(in fortune.Input, res *fortune.Result) (*fortune.PromptSpec, error) {
	if res == nil || res.Kind != fortune.KindZodiac {
		return nil, fmt.Errorf("prompt/zodiac: expected kind %q, got %q", fortune.KindZodiac, kindOrEmpty(res))
	}
	chart, ok := res.Data.(*fortune.ZodiacChart)
	if !ok {
		return nil, fmt.Errorf("prompt/zodiac: result.Data is not *fortune.ZodiacChart (got %T)", res.Data)
	}

	depth := in.InterpretDepth
	if depth == "" {
		depth = fortune.DepthBrief
	}
	tier := fortune.TierFree
	if depth == fortune.DepthDeep {
		tier = fortune.TierPaid
	}

	L := resolveZodiacLang(in.Lang)
	system := buildZodiacSystem(L)
	user := buildZodiacUser(L, chart, depth)

	return &fortune.PromptSpec{
		System: system,
		User:   user,
		Tier:   tier,
	}, nil
}

func buildZodiacSystem(L zodiacLang) string {
	var b strings.Builder
	b.WriteString(L.persona)
	b.WriteString("\n\n请根据用户提供的生肖运势数据进行解读。要求：\n")
	for i, r := range L.rules {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	fmt.Fprintf(&b, "\n免责声明：%s", L.disclaimer)
	return b.String()
}

func buildZodiacUser(L zodiacLang, c *fortune.ZodiacChart, depth string) string {
	var b strings.Builder
	b.WriteString("【生肖运势】\n")
	fmt.Fprintf(&b, "生肖：%s\n", c.Zodiac)
	fmt.Fprintf(&b, "流年：%d年（%s年，生肖%s）\n", c.Year, c.LiuNianZhi, c.LiuNianZodiac)

	b.WriteString("\n【运势分数】\n")
	fmt.Fprintf(&b, "综合运势：%d分\n", c.OverallScore)
	fmt.Fprintf(&b, "事业运势：%d分\n", c.CareerScore)
	fmt.Fprintf(&b, "财运：%d分\n", c.WealthScore)
	fmt.Fprintf(&b, "感情运势：%d分\n", c.LoveScore)
	fmt.Fprintf(&b, "健康运势：%d分\n", c.HealthScore)

	b.WriteString("\n【流年关系】\n")
	for _, rel := range c.Relations {
		fmt.Fprintf(&b, "%s（与%s）：%s\n", rel.Type, rel.With, rel.Effect)
	}

	if len(c.Tips) > 0 {
		b.WriteString("\n【建议】\n")
		for _, tip := range c.Tips {
			b.WriteString("• " + tip + "\n")
		}
	}

	if len(c.Warns) > 0 {
		b.WriteString("\n【注意事项】\n")
		for _, warn := range c.Warns {
			b.WriteString("• " + warn + "\n")
		}
	}

	b.WriteString("\n【幸运元素】\n")
	fmt.Fprintf(&b, "幸运颜色：%s\n", strings.Join(c.LuckyColors, "、"))
	fmt.Fprintf(&b, "幸运数字：%v\n", c.LuckyNumbers)
	fmt.Fprintf(&b, "幸运方位：%s\n", c.LuckyDir)

	b.WriteString("\n【解读要求】\n")
	if depth == fortune.DepthDeep {
		b.WriteString(L.deepHint)
	} else {
		b.WriteString(L.briefHint)
	}
	return b.String()
}
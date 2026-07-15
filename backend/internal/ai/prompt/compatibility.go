// Package prompt provides the compatibility prompt builder.
package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

type compatibilityLang struct {
	langName   string
	persona    string
	rules      []string
	disclaimer string
	briefHint  string
	deepHint   string
}

var compatibilityLangs = map[string]compatibilityLang{
	"zh-CN": {
		langName: "简体中文",
		persona:  "你是一位精通中国传统命理的姻缘分析师，擅长根据生肖、八字五行等传统命理知识，为情侣/夫妻提供感情分析与相处建议。",
		rules: []string{
			"使用简体中文回答",
			"语气温暖积极，强调珍惜感情、互相包容",
			"结合计算结果给出客观分析",
			"即使分数不高也要给出建设性建议，避免负面断言",
			"不宣扬迷信，强调感情在于双方经营",
			"结尾附简短免责声明",
		},
		disclaimer: "命理分析仅供参考，幸福的关键在于双方共同经营。",
		briefHint:  "请简要分析双方配对情况，给出关键建议（约 300-400 字）。",
		deepHint:   "请详细分析双方各方面配对情况，包括性格互补、相处之道、注意事项等，给出具体的感情经营建议（约 600-900 字）。",
	},
	"zh-TW": {
		langName: "繁體中文",
		persona:  "你是一位精通中國傳統命理的姻緣分析師，擅長根據生肖、八字五行等傳統命理知識，為情侶/夫妻提供感情分析與相處建議。",
		rules: []string{
			"使用繁體中文回答",
			"語氣溫暖積極，強調珍惜感情、互相包容",
			"結合計算結果給出客觀分析",
			"即使分數不高也要給出建設性建議，避免負面斷言",
			"不宣揚迷信，強調感情在於雙方經營",
			"結尾附簡短免責聲明",
		},
		disclaimer: "命理分析僅供參考，幸福的關鍵在於雙方共同經營。",
		briefHint:  "請簡要分析雙方配對情況，給出關鍵建議（約 300-400 字）。",
		deepHint:   "請詳細分析雙方各方面配對情況，包括性格互補、相處之道、注意事項等，給出具體的感情經營建議（約 600-900 字）。",
	},
}

func resolveCompatibilityLang(lang string) compatibilityLang {
	if l, ok := compatibilityLangs[lang]; ok {
		return l
	}
	return compatibilityLangs["zh-CN"]
}

// CompatibilityBuild constructs the AI prompt for compatibility analysis.
func CompatibilityBuild(in fortune.Input, res *fortune.Result) (*fortune.PromptSpec, error) {
	if res == nil || res.Kind != fortune.KindCompatibility {
		return nil, fmt.Errorf("prompt/compatibility: expected kind %q, got %q", fortune.KindCompatibility, kindOrEmpty(res))
	}
	chart, ok := res.Data.(*fortune.CompatibilityChart)
	if !ok {
		return nil, fmt.Errorf("prompt/compatibility: result.Data is not *fortune.CompatibilityChart (got %T)", res.Data)
	}

	depth := in.InterpretDepth
	if depth == "" {
		depth = fortune.DepthBrief
	}
	tier := fortune.TierFree
	if depth == fortune.DepthDeep {
		tier = fortune.TierPaid
	}

	L := resolveCompatibilityLang(in.Lang)
	system := buildCompatibilitySystem(L)
	user := buildCompatibilityUser(L, chart, depth)

	return &fortune.PromptSpec{
		System: system,
		User:   user,
		Tier:   tier,
	}, nil
}

func buildCompatibilitySystem(L compatibilityLang) string {
	var b strings.Builder
	b.WriteString(L.persona)
	b.WriteString("\n\n请根据用户提供的配对分析数据进行解读。要求：\n")
	for i, r := range L.rules {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	fmt.Fprintf(&b, "\n免责声明：%s", L.disclaimer)
	return b.String()
}

func buildCompatibilityUser(L compatibilityLang, c *fortune.CompatibilityChart, depth string) string {
	var b strings.Builder
	b.WriteString("【配对信息】\n")
	fmt.Fprintf(&b, "甲方：生肖%s，年柱%s，日主%s%s\n", c.Subject1.Zodiac, c.Subject1.YearGanZhi, c.Subject1.DayGan, c.Subject1.DayZhi)
	fmt.Fprintf(&b, "乙方：生肖%s，年柱%s，日主%s%s\n", c.Subject2.Zodiac, c.Subject2.YearGanZhi, c.Subject2.DayGan, c.Subject2.DayZhi)

	b.WriteString("\n【配对分数】\n")
	fmt.Fprintf(&b, "综合配对：%d分\n", c.OverallScore)
	fmt.Fprintf(&b, "缘分吸引力：%d分\n", c.ChemistryScore)
	fmt.Fprintf(&b, "相处和谐度：%d分\n", c.HarmonyScore)
	fmt.Fprintf(&b, "关系稳定性：%d分\n", c.StabilityScore)

	b.WriteString("\n【分析要素】\n")
	for _, f := range c.Factors {
		fmt.Fprintf(&b, "%s（%+d分）：%s\n", f.Factor, f.Score, f.Detail)
	}

	b.WriteString("\n【总结】\n")
	b.WriteString(c.Summary + "\n")
	b.WriteString(c.Tips + "\n")

	b.WriteString("\n【解读要求】\n")
	if depth == fortune.DepthDeep {
		b.WriteString(L.deepHint)
	} else {
		b.WriteString(L.briefHint)
	}
	return b.String()
}
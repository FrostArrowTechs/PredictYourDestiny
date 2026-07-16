// Package prompt provides the constellation fortune prompt builder.
package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

type constellationLang struct {
	langName   string
	persona    string
	rules      []string
	disclaimer string
	briefHint  string
	deepHint   string
}

var constellationLangs = map[string]constellationLang{
	"zh-CN": {
		langName: "简体中文",
		persona:  "你是一位精通西方占星学的星座分析师，擅长根据太阳星座的特质、元素、守护星，为用户提供性格解读与当日运势建议。",
		rules: []string{
			"使用简体中文回答",
			"语气积极正面，强调发挥星座优势、规避盲点",
			"结合计算结果中的特质与分数给出合理解读",
			"避免宿命论，强调主观能动性",
			"结尾附简短免责声明",
		},
		disclaimer: "星座内容仅供参考与娱乐，个人命运由自己掌握。",
		briefHint:  "请简要解读该星座的今日运势，给出关键建议（约 200-300 字）。",
		deepHint:   "请详细分析该星座的性格特质、今日各方面运势，结合元素与守护星给出具体的事业、感情、生活建议（约 500-800 字）。",
	},
	"zh-TW": {
		langName: "繁體中文",
		persona:  "你是一位精通西方占星學的星座分析師，擅長根據太陽星座的特質、元素、守護星，為用戶提供性格解讀與當日運勢建議。",
		rules: []string{
			"使用繁體中文回答",
			"語氣積極正面，強調發揮星座優勢、規避盲點",
			"結合計算結果中的特質與分數給出合理解讀",
			"避免宿命論，強調主觀能動性",
			"結尾附簡短免責聲明",
		},
		disclaimer: "星座內容僅供參考與娛樂，個人命運由自己掌握。",
		briefHint:  "請簡要解讀該星座的今日運勢，給出關鍵建議（約 200-300 字）。",
		deepHint:   "請詳細分析該星座的性格特質、今日各方面運勢，結合元素與守護星給出具體的事業、感情、生活建議（約 500-800 字）。",
	},
}

func resolveConstellationLang(lang string) constellationLang {
	if l, ok := constellationLangs[lang]; ok {
		return l
	}
	return constellationLangs["zh-CN"]
}

// ConstellationBuild constructs the AI prompt for constellation fortune.
func ConstellationBuild(in fortune.Input, res *fortune.Result) (*fortune.PromptSpec, error) {
	if res == nil || res.Kind != fortune.KindConstellation {
		return nil, fmt.Errorf("prompt/constellation: expected kind %q, got %q", fortune.KindConstellation, kindOrEmpty(res))
	}
	chart, ok := res.Data.(*fortune.ConstellationChart)
	if !ok {
		return nil, fmt.Errorf("prompt/constellation: result.Data is not *fortune.ConstellationChart (got %T)", res.Data)
	}

	depth := in.InterpretDepth
	if depth == "" {
		depth = fortune.DepthBrief
	}
	tier := fortune.TierFree
	if depth == fortune.DepthDeep {
		tier = fortune.TierPaid
	}

	L := resolveConstellationLang(in.Lang)
	system := buildConstellationSystem(L)
	user := buildConstellationUser(L, chart, depth)

	return &fortune.PromptSpec{
		System: system,
		User:   user,
		Tier:   tier,
	}, nil
}

func buildConstellationSystem(L constellationLang) string {
	var b strings.Builder
	b.WriteString(L.persona)
	b.WriteString("\n\n请根据用户提供的星座数据进行解读。要求：\n")
	for i, r := range L.rules {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	fmt.Fprintf(&b, "\n免责声明：%s", L.disclaimer)
	return b.String()
}

func buildConstellationUser(L constellationLang, c *fortune.ConstellationChart, depth string) string {
	var b strings.Builder
	b.WriteString("【星座运势】\n")
	fmt.Fprintf(&b, "星座：%s（%s）\n", c.Sign, c.SignLatin)
	fmt.Fprintf(&b, "元素：%s  特质：%s  守护星：%s\n", c.Element, c.Quality, c.Ruler)
	fmt.Fprintf(&b, "出生日期范围：%s\n", c.DateRange)

	b.WriteString("\n【性格特质】\n")
	b.WriteString("优势：" + strings.Join(c.Strengths, "、") + "\n")
	b.WriteString("盲点：" + strings.Join(c.Weakness, "、") + "\n")
	b.WriteString("关键词：" + strings.Join(c.Keywords, "、") + "\n")

	b.WriteString("\n【今日运势分数】\n")
	fmt.Fprintf(&b, "综合运势：%d分\n", c.OverallScore)
	fmt.Fprintf(&b, "事业运势：%d分\n", c.CareerScore)
	fmt.Fprintf(&b, "感情运势：%d分\n", c.LoveScore)
	fmt.Fprintf(&b, "财运：%d分\n", c.WealthScore)
	fmt.Fprintf(&b, "健康运势：%d分\n", c.HealthScore)

	b.WriteString("\n【今日幸运元素】\n")
	fmt.Fprintf(&b, "幸运颜色：%s\n", strings.Join(c.LuckyColors, "、"))
	fmt.Fprintf(&b, "幸运数字：%v\n", c.LuckyNumbers)
	fmt.Fprintf(&b, "幸运方位：%s\n", c.LuckyDir)

	b.WriteString("\n【星座配对】\n")
	fmt.Fprintf(&b, "最佳配对：%s\n", c.BestMatch)
	fmt.Fprintf(&b, "需要注意：%s\n", c.WorstMatch)

	b.WriteString("\n【解读要求】\n")
	if depth == fortune.DepthDeep {
		b.WriteString(L.deepHint)
	} else {
		b.WriteString(L.briefHint)
	}
	return b.String()
}

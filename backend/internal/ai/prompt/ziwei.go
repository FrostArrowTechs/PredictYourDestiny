// Package prompt provides the 紫微斗数 prompt builder.
package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

type ziweiLang struct {
	langName   string
	persona    string
	rules      []string
	disclaimer string
	briefHint  string
	deepHint   string
}

var ziweiLangs = map[string]ziweiLang{
	"zh-CN": {
		langName: "简体中文",
		persona:  "你是一位精通紫微斗数的命理师，擅长根据十二宫位、主星落位、四化与煞星的组合，为用户提供命格格局与人生各层面的深度解读。",
		rules: []string{
			"使用简体中文回答",
			"结合命宫主星与三方四正的组合给出格局判断",
			"解读十二宫（命/兄/夫/子/财/疾/官/奴/迁/田/福/父）的关键信息",
			"分析四化（禄/权/科/忌）带来的吉凶与转化",
			"语气客观中正，既指出优势也提醒需注意之处",
			"结尾附简短免责声明",
		},
		disclaimer: "紫微斗数为传统命理研究，解读仅供参考娱乐。",
		briefHint:  "请简要解读此命盘的核心格局与关键宫位，给出人生大方向的建议（约 300-500 字）。",
		deepHint:   "请详细分析此命盘，先解读命宫主星格局，再逐一分析事业（官禄）、财帛、感情（夫妻）、健康（疾厄）等关键宫位，结合四化与三方四正给出具体的人生建议（约 800-1200 字）。",
	},
	"zh-TW": {
		langName: "繁體中文",
		persona:  "你是一位精通紫微斗數的命理師，擅長根據十二宮位、主星落位、四化與煞星的組合，為用戶提供命格格局與人生各層面的深度解讀。",
		rules: []string{
			"使用繁體中文回答",
			"結合命宮主星與三方四正的組合給出格局判斷",
			"解讀十二宮（命/兄/夫/子/財/疾/官/奴/遷/田/福/父）的關鍵資訊",
			"分析四化（祿/權/科/忌）帶來的吉凶與轉化",
			"語氣客觀中正，既指出優勢也提醒需注意之處",
			"結尾附簡短免責聲明",
		},
		disclaimer: "紫微斗數為傳統命理研究，解讀僅供參考娛樂。",
		briefHint:  "請簡要解讀此命盤的核心格局與關鍵宮位，給出人生大方向的建議（約 300-500 字）。",
		deepHint:   "請詳細分析此命盤，先解讀命宮主星格局，再逐一分析事業（官祿）、財帛、感情（夫妻）、健康（疾厄）等關鍵宮位，結合四化與三方四正給出具體的人生建議（約 800-1200 字）。",
	},
}

func resolveZiweiLang(lang string) ziweiLang {
	if l, ok := ziweiLangs[lang]; ok {
		return l
	}
	return ziweiLangs["zh-CN"]
}

// ZiweiBuild constructs the AI prompt for a Ziwei chart reading.
func ZiweiBuild(in fortune.Input, res *fortune.Result) (*fortune.PromptSpec, error) {
	if res == nil || res.Kind != fortune.KindZiwei {
		return nil, fmt.Errorf("prompt/ziwei: expected kind %q, got %q", fortune.KindZiwei, kindOrEmpty(res))
	}
	chart, ok := res.Data.(*fortune.ZiweiChart)
	if !ok {
		return nil, fmt.Errorf("prompt/ziwei: result.Data is not *fortune.ZiweiChart (got %T)", res.Data)
	}

	depth := in.InterpretDepth
	if depth == "" {
		depth = fortune.DepthBrief
	}
	tier := fortune.TierFree
	if depth == fortune.DepthDeep {
		tier = fortune.TierPaid
	}

	L := resolveZiweiLang(in.Lang)
	system := buildZiweiSystem(L)
	user := buildZiweiUser(L, chart, depth)

	return &fortune.PromptSpec{
		System: system,
		User:   user,
		Tier:   tier,
	}, nil
}

func buildZiweiSystem(L ziweiLang) string {
	var b strings.Builder
	b.WriteString(L.persona)
	b.WriteString("\n\n请根据用户提供的紫微斗数命盘进行解读。要求：\n")
	for i, r := range L.rules {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	fmt.Fprintf(&b, "\n免责声明：%s", L.disclaimer)
	return b.String()
}

func buildZiweiUser(L ziweiLang, c *fortune.ZiweiChart, depth string) string {
	var b strings.Builder
	b.WriteString("【紫微斗数命盘】\n")
	fmt.Fprintf(&b, "公历：%s\n", c.SolarDate)
	fmt.Fprintf(&b, "农历：%s\n", c.LunarDate)
	fmt.Fprintf(&b, "性别：%s\n", c.Gender)
	fmt.Fprintf(&b, "年柱：%s  月柱：%s  日柱：%s\n", c.YearGanZhi, c.MonthGanZhi, c.DayGanZhi)
	fmt.Fprintf(&b, "五行局：%s\n", c.WuXingJu)
	fmt.Fprintf(&b, "命宫：%s宫  身宫：%s宫\n", c.LifePalaceBranch, c.BodyPalaceBranch)
	fmt.Fprintf(&b, "命主：%s  身主：%s\n", c.LifeRuler, c.BodyRuler)
	fmt.Fprintf(&b, "命宫主星：%s\n", c.MainStarOfLife)
	fmt.Fprintf(&b, "大运：%d岁起运，%s\n", c.DaYunStartAge, daYunDirText(c.DaYunForward))

	b.WriteString("\n【十二宫星曜】\n")
	for _, p := range c.Palaces {
		b.WriteString("\n")
		marker := ""
		if p.IsLife {
			marker = "（命宫）"
		} else if p.IsBody {
			marker = "（身宫）"
		}
		fmt.Fprintf(&b, "%s%s（%s宫）\n", p.Name, marker, p.Branch)
		if len(p.Stars) > 0 {
			fmt.Fprintf(&b, "  星曜：%s\n", strings.Join(p.Stars, "、"))
		} else {
			b.WriteString("  星曜：（空宫）\n")
		}
		if p.Transform != "" {
			fmt.Fprintf(&b, "  四化：%s\n", p.Transform)
		}
	}

	b.WriteString("\n【解读要求】\n")
	if depth == fortune.DepthDeep {
		b.WriteString(L.deepHint)
	} else {
		b.WriteString(L.briefHint)
	}
	return b.String()
}

func daYunDirText(forward bool) string {
	if forward {
		return "顺行"
	}
	return "逆行"
}

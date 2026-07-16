// Package prompt provides the 称骨算命 prompt builder.
package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

type weighboneLang struct {
	langName   string
	persona    string
	rules      []string
	disclaimer string
	briefHint  string
	deepHint   string
}

var weighboneLangs = map[string]weighboneLang{
	"zh-CN": {
		langName: "简体中文",
		persona:  "你是一位精通袁天罡称骨算命与传统命理的解读师，擅长将传统称骨歌诀转化为现代人易懂的命运分析与人生建议。",
		rules: []string{
			"使用简体中文回答",
			"结合骨重、歌诀给出客观、温和的解读",
			"避免宿命论，强调'命由天定，运由己造'，给出积极建议",
			"不输出医疗、投资等专业建议",
			"结尾附简短免责声明",
		},
		disclaimer: "称骨算命仅供文化参考，人生际遇在于自身努力与抉择。",
		briefHint:  "请简要解读此骨重的命运特征，给出关键建议（约 200-300 字）。",
		deepHint:   "请详细解读此骨重的命运特征，从性格、事业、财运、感情、健康等维度展开，给出具体的人生建议（约 500-700 字）。",
	},
	"zh-TW": {
		langName: "繁體中文",
		persona:  "你是一位精通袁天罡稱骨算命與傳統命理的解讀師，擅長將傳統稱骨歌訣轉化為現代人易懂的命運分析與人生建議。",
		rules: []string{
			"使用繁體中文回答",
			"結合骨重、歌訣給出客觀、溫和的解讀",
			"避免宿命論，強調「命由天定，運由己造」，給出積極建議",
			"不輸出醫療、投資等專業建議",
			"結尾附簡短免責聲明",
		},
		disclaimer: "稱骨算命僅供文化參考，人生際遇在於自身努力與抉擇。",
		briefHint:  "請簡要解讀此骨重的命運特徵，給出關鍵建議（約 200-300 字）。",
		deepHint:   "請詳細解讀此骨重的命運特徵，從性格、事業、財運、感情、健康等維度展開，給出具體的人生建議（約 500-700 字）。",
	},
}

func resolveWeighboneLang(lang string) weighboneLang {
	if l, ok := weighboneLangs[lang]; ok {
		return l
	}
	return weighboneLangs["zh-CN"]
}

// WeighboneBuild constructs the AI prompt for bone-weight fortune.
func WeighboneBuild(in fortune.Input, res *fortune.Result) (*fortune.PromptSpec, error) {
	if res == nil || res.Kind != fortune.KindWeighbone {
		return nil, fmt.Errorf("prompt/weighbone: expected kind %q, got %q", fortune.KindWeighbone, kindOrEmpty(res))
	}
	chart, ok := res.Data.(*fortune.WeighboneChart)
	if !ok {
		return nil, fmt.Errorf("prompt/weighbone: result.Data is not *fortune.WeighboneChart (got %T)", res.Data)
	}

	depth := in.InterpretDepth
	if depth == "" {
		depth = fortune.DepthBrief
	}
	tier := fortune.TierFree
	if depth == fortune.DepthDeep {
		tier = fortune.TierPaid
	}

	L := resolveWeighboneLang(in.Lang)
	system := buildWeighboneSystem(L)
	user := buildWeighboneUser(L, chart, depth)

	return &fortune.PromptSpec{
		System: system,
		User:   user,
		Tier:   tier,
	}, nil
}

func buildWeighboneSystem(L weighboneLang) string {
	var b strings.Builder
	b.WriteString(L.persona)
	b.WriteString("\n\n请根据用户提供的称骨算命数据进行解读。要求：\n")
	for i, r := range L.rules {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	fmt.Fprintf(&b, "\n免责声明：%s", L.disclaimer)
	return b.String()
}

func buildWeighboneUser(L weighboneLang, c *fortune.WeighboneChart, depth string) string {
	var b strings.Builder
	b.WriteString("【称骨算命】\n")
	fmt.Fprintf(&b, "年骨重：%s\n", c.YearWeight)
	fmt.Fprintf(&b, "月骨重：%s\n", c.MonthWeight)
	fmt.Fprintf(&b, "日骨重：%s\n", c.DayWeight)
	fmt.Fprintf(&b, "时骨重：%s\n", c.HourWeight)
	fmt.Fprintf(&b, "总骨重：%s\n", c.TotalWeight)
	fmt.Fprintf(&b, "命格等第：%s\n", c.Category)
	b.WriteString("\n【称骨歌诀】\n")
	b.WriteString(c.Poem + "\n")
	b.WriteString("\n【简要评语】\n")
	b.WriteString(c.Description + "\n")

	b.WriteString("\n【解读要求】\n")
	if depth == fortune.DepthDeep {
		b.WriteString(L.deepHint)
	} else {
		b.WriteString(L.briefHint)
	}
	return b.String()
}

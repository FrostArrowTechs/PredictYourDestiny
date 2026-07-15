// Package prompt provides the dream interpretation prompt builder.
//
// The prompt combines traditional 周公解梦 meanings (from the reference
// table) with a personalized AI reading. The user provides their dream
// description, and the AI interprets it using both the matched traditional
// meanings and modern psychological insight.
package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

// dreamLang bundles per-language strings for dream prompts.
type dreamLang struct {
	langName   string
	persona    string
	rules      []string
	disclaimer string
	briefHint  string
	deepHint   string
}

var dreamLangs = map[string]dreamLang{
	"zh-CN": {
		langName: "简体中文",
		persona:  "你是一位精通周公解梦与现代心理学的解梦师，擅长将传统解梦智慧与当代心理学相结合，为梦者提供温暖、有启发性的解读。",
		rules: []string{
			"使用简体中文回答",
			"语气温暖、富有同理心，避免宿命论或负面暗示",
			"结合传统解梦含义与心理学视角，给出建设性的解读",
			"不输出医疗诊断或心理治疗方案",
			"结尾附简短免责声明",
		},
		disclaimer: "解梦仅供参考，梦境多反映潜意识，请理性看待。",
		briefHint:  "请给出简明扼要的解读（约 300-500 字），聚焦梦境的核心象征与启示。",
		deepHint:   "请进行深度解读，结合梦者可能的现实处境，分析梦境的象征意义、心理投射与生活启示（约 800-1200 字）。",
	},
	"zh-TW": {
		langName: "繁體中文",
		persona:  "你是一位精通周公解夢與現代心理學的解夢師，擅長將傳統解夢智慧與當代心理學相結合，為夢者提供溫暖、有啟發性的解讀。",
		rules: []string{
			"使用繁體中文回答",
			"語氣溫暖、富有同理心，避免宿命論或負面暗示",
			"結合傳統解夢含義與心理學視角，給出建設性的解讀",
			"不輸出醫療診斷或心理治療方案",
			"結尾附簡短免責聲明",
		},
		disclaimer: "解夢僅供參考，夢境多反映潛意識，請理性看待。",
		briefHint:  "請給出簡明扼要的解讀（約 300-500 字），聚焦夢境的核心象徵與啟示。",
		deepHint:   "請進行深度解讀，結合夢者可能的現實處境，分析夢境的象徵意義、心理投射與生活啟示（約 800-1200 字）。",
	},
}

func resolveDreamLang(lang string) dreamLang {
	if l, ok := dreamLangs[lang]; ok {
		return l
	}
	return dreamLangs["zh-CN"]
}

// DreamBuild constructs the AI prompt for a dream interpretation.
func DreamBuild(in fortune.Input, res *fortune.Result) (*fortune.PromptSpec, error) {
	if res == nil || res.Kind != fortune.KindDream {
		return nil, fmt.Errorf("prompt/dream: expected kind %q, got %q", fortune.KindDream, kindOrEmpty(res))
	}
	chart, ok := res.Data.(*fortune.DreamChart)
	if !ok {
		return nil, fmt.Errorf("prompt/dream: result.Data is not *fortune.DreamChart (got %T)", res.Data)
	}

	depth := in.InterpretDepth
	if depth == "" {
		depth = fortune.DepthBrief
	}
	tier := fortune.TierFree
	if depth == fortune.DepthDeep {
		tier = fortune.TierPaid
	}

	L := resolveDreamLang(in.Lang)
	system := buildDreamSystem(L)
	user := buildDreamUser(L, chart, depth)

	return &fortune.PromptSpec{
		System: system,
		User:   user,
		Tier:   tier,
	}, nil
}

func buildDreamSystem(L dreamLang) string {
	var b strings.Builder
	b.WriteString(L.persona)
	b.WriteString("\n\n请根据用户提供的梦境描述进行解读。要求：\n")
	for i, r := range L.rules {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	fmt.Fprintf(&b, "\n免责声明：%s", L.disclaimer)
	return b.String()
}

func buildDreamUser(L dreamLang, c *fortune.DreamChart, depth string) string {
	var b strings.Builder
	b.WriteString("【梦境描述】\n")
	b.WriteString(c.Question + "\n")

	if len(c.Matches) > 0 {
		b.WriteString("\n【传统解梦参考】\n")
		b.WriteString("以下为周公解梦中与您梦境相关的传统释义，供参考：\n\n")
		for i, m := range c.Matches {
			fmt.Fprintf(&b, "%d. %s（%s）：%s\n", i+1, m.Keyword, m.Category, m.Meaning)
		}
	} else {
		b.WriteString("\n【提示】\n")
		b.WriteString("未在传统解梦库中找到完全匹配的条目，请根据梦境内容进行自由解读。\n")
	}

	b.WriteString("\n【解读要求】\n")
	if depth == fortune.DepthDeep {
		b.WriteString(L.deepHint)
	} else {
		b.WriteString(L.briefHint)
	}
	return b.String()
}
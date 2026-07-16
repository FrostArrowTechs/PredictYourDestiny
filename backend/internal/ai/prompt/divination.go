// Package prompt provides the 抽签/求签 divination prompt builder.
package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

type divinationLang struct {
	langName   string
	persona    string
	rules      []string
	disclaimer string
	briefHint  string
	deepHint   string
}

var divinationLangs = map[string]divinationLang{
	"zh-CN": {
		langName: "简体中文",
		persona:  "你是一位精通观音灵签的解签师，擅长将传统签诗结合求签者的具体问题，给出温暖、有启发的解签与指引。",
		rules: []string{
			"使用简体中文回答",
			"结合签诗与求签者所问之事，给出针对性的解答",
			"语气温暖平和，给予鼓励与正向引导",
			"下签也要给出化解之道，不可一味渲染不吉",
			"结尾附简短免责声明",
		},
		disclaimer: "签诗仅供参考，吉凶在于人为，行善积德自有福报。",
		briefHint:  "请简要解签，结合所问之事给出关键指引（约 200-300 字）。",
		deepHint:   "请详细解签，逐句分析签诗含义，结合所问之事从多角度给出具体的建议与化解之道（约 400-600 字）。",
	},
	"zh-TW": {
		langName: "繁體中文",
		persona:  "你是一位精通觀音靈籤的解籤師，擅長將傳統籤詩結合求籤者的具體問題，給出溫暖、有啟發的解籤與指引。",
		rules: []string{
			"使用繁體中文回答",
			"結合籤詩與求籤者所問之事，給出針對性的解答",
			"語氣溫暖平和，給予鼓勵與正向引導",
			"下籤也要給出化解之道，不可一味渲染不吉",
			"結尾附簡短免責聲明",
		},
		disclaimer: "籤詩僅供參考，吉凶在於人為，行善積德自有福報。",
		briefHint:  "請簡要解籤，結合所問之事給出關鍵指引（約 200-300 字）。",
		deepHint:   "請詳細解籤，逐句分析籤詩含義，結合所問之事從多角度給出具體的建議與化解之道（約 400-600 字）。",
	},
}

func resolveDivinationLang(lang string) divinationLang {
	if l, ok := divinationLangs[lang]; ok {
		return l
	}
	return divinationLangs["zh-CN"]
}

// DivinationBuild constructs the AI prompt for divination interpretation.
func DivinationBuild(in fortune.Input, res *fortune.Result) (*fortune.PromptSpec, error) {
	if res == nil || res.Kind != fortune.KindDivination {
		return nil, fmt.Errorf("prompt/divination: expected kind %q, got %q", fortune.KindDivination, kindOrEmpty(res))
	}
	chart, ok := res.Data.(*fortune.DivinationChart)
	if !ok {
		return nil, fmt.Errorf("prompt/divination: result.Data is not *fortune.DivinationChart (got %T)", res.Data)
	}

	depth := in.InterpretDepth
	if depth == "" {
		depth = fortune.DepthBrief
	}
	tier := fortune.TierFree
	if depth == fortune.DepthDeep {
		tier = fortune.TierPaid
	}

	L := resolveDivinationLang(in.Lang)
	system := buildDivinationSystem(L)
	user := buildDivinationUser(L, chart, depth)

	return &fortune.PromptSpec{
		System: system,
		User:   user,
		Tier:   tier,
	}, nil
}

func buildDivinationSystem(L divinationLang) string {
	var b strings.Builder
	b.WriteString(L.persona)
	b.WriteString("\n\n请根据用户抽到的签诗进行解签。要求：\n")
	for i, r := range L.rules {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	fmt.Fprintf(&b, "\n免责声明：%s", L.disclaimer)
	return b.String()
}

func buildDivinationUser(L divinationLang, c *fortune.DivinationChart, depth string) string {
	var b strings.Builder
	b.WriteString("【抽签结果】\n")
	fmt.Fprintf(&b, "签号：%s\n", c.Title)
	fmt.Fprintf(&b, "等级：%s\n", c.Tier)
	b.WriteString("\n【签诗】\n")
	b.WriteString(c.Poem + "\n")
	b.WriteString("\n【传统解签】\n")
	b.WriteString(c.Interpret + "\n")

	if c.Question != "" {
		b.WriteString("\n【所问之事】\n")
		b.WriteString(c.Question + "\n")
	}

	b.WriteString("\n【解签要求】\n")
	if depth == fortune.DepthDeep {
		b.WriteString(L.deepHint)
	} else {
		b.WriteString(L.briefHint)
	}
	return b.String()
}

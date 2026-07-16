// Package prompt provides the 梅花易数 prompt builder.
package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

type plumflowerLang struct {
	langName   string
	persona    string
	rules      []string
	disclaimer string
	briefHint  string
	deepHint   string
}

var plumflowerLangs = map[string]plumflowerLang{
	"zh-CN": {
		langName: "简体中文",
		persona:  "你是一位精通邵雍梅花易数与易经六十四卦的占卜师，擅长根据卦象、体用关系为求测者解答疑惑，指引方向。",
		rules: []string{
			"使用简体中文回答",
			"结合本卦、互卦、变卦和体用关系进行综合解读",
			"语气平和客观，给出具体可行的建议",
			"凶卦也要指出化解之道，不可一味渲染不利",
			"结尾附简短免责声明",
		},
		disclaimer: "卦象仅供参考，吉凶在于人为，趋吉避凶方为智者。",
		briefHint:  "请简要解卦，结合体用关系给出关键指引（约 200-300 字）。",
		deepHint:   "请详细解卦，分别解读本卦、互卦、变卦的含义，结合体用生克关系，从多角度给出具体的建议（约 500-700 字）。",
	},
	"zh-TW": {
		langName: "繁體中文",
		persona:  "你是一位精通邵雍梅花易數與易經六十四卦的占卜師，擅長根據卦象、體用關係為求測者解答疑惑，指引方向。",
		rules: []string{
			"使用繁體中文回答",
			"結合本卦、互卦、變卦和體用關係進行綜合解讀",
			"語氣平和客觀，給出具體可行的建議",
			"凶卦也要指出化解之道，不可一味渲染不利",
			"結尾附簡短免責聲明",
		},
		disclaimer: "卦象僅供參考，吉凶在於人為，趨吉避凶方為智者。",
		briefHint:  "請簡要解卦，結合體用關係給出關鍵指引（約 200-300 字）。",
		deepHint:   "請詳細解卦，分別解讀本卦、互卦、變卦的含義，結合體用生剋關係，從多角度給出具體的建議（約 500-700 字）。",
	},
}

func resolvePlumFlowerLang(lang string) plumflowerLang {
	if l, ok := plumflowerLangs[lang]; ok {
		return l
	}
	return plumflowerLangs["zh-CN"]
}

// PlumFlowerBuild constructs the AI prompt for hexagram interpretation.
func PlumFlowerBuild(in fortune.Input, res *fortune.Result) (*fortune.PromptSpec, error) {
	if res == nil || res.Kind != fortune.KindPlumFlower {
		return nil, fmt.Errorf("prompt/plumflower: expected kind %q, got %q", fortune.KindPlumFlower, kindOrEmpty(res))
	}
	chart, ok := res.Data.(*fortune.PlumFlowerChart)
	if !ok {
		return nil, fmt.Errorf("prompt/plumflower: result.Data is not *fortune.PlumFlowerChart (got %T)", res.Data)
	}

	depth := in.InterpretDepth
	if depth == "" {
		depth = fortune.DepthBrief
	}
	tier := fortune.TierFree
	if depth == fortune.DepthDeep {
		tier = fortune.TierPaid
	}

	L := resolvePlumFlowerLang(in.Lang)
	system := buildPlumFlowerSystem(L)
	user := buildPlumFlowerUser(L, chart, depth, in.Question)

	return &fortune.PromptSpec{
		System: system,
		User:   user,
		Tier:   tier,
	}, nil
}

func buildPlumFlowerSystem(L plumflowerLang) string {
	var b strings.Builder
	b.WriteString(L.persona)
	b.WriteString("\n\n请根据用户提供的梅花易数卦象进行解卦。要求：\n")
	for i, r := range L.rules {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	fmt.Fprintf(&b, "\n免责声明：%s", L.disclaimer)
	return b.String()
}

func buildPlumFlowerUser(L plumflowerLang, c *fortune.PlumFlowerChart, depth string, question string) string {
	var b strings.Builder
	b.WriteString("【梅花易数卦象】\n")
	fmt.Fprintf(&b, "起卦方式：%s\n", methodLabel(c.Method))

	b.WriteString("\n【本卦】")
	b.WriteString(c.Original.Name)
	fmt.Fprintf(&b, "（上%s下%s，%s%s）\n", c.Original.UpperTrig, c.Original.LowerTrig, c.Original.UpperWX, c.Original.LowerWX)

	b.WriteString("【互卦】")
	b.WriteString(c.Mutual.Name)
	fmt.Fprintf(&b, "（上%s下%s）\n", c.Mutual.UpperTrig, c.Mutual.LowerTrig)

	b.WriteString("【变卦】")
	b.WriteString(c.Changed.Name)
	fmt.Fprintf(&b, "（上%s下%s）\n", c.Changed.UpperTrig, c.Changed.LowerTrig)

	fmt.Fprintf(&b, "动爻：第%d爻\n", c.ChangingLine)

	b.WriteString("\n【体用分析】\n")
	fmt.Fprintf(&b, "体卦：%s（%s）\n", c.BodyTrigram, c.BodyWuXing)
	fmt.Fprintf(&b, "用卦：%s（%s）\n", c.UseTrigram, c.UseWuXing)
	fmt.Fprintf(&b, "关系：%s\n", c.Relationship)
	fmt.Fprintf(&b, "趋势：%s\n", c.Trend)
	b.WriteString(c.Analysis + "\n")

	if question != "" && !isNumbers(question) {
		b.WriteString("\n【所问之事】\n")
		b.WriteString(question + "\n")
	}

	b.WriteString("\n【解卦要求】\n")
	if depth == fortune.DepthDeep {
		b.WriteString(L.deepHint)
	} else {
		b.WriteString(L.briefHint)
	}
	return b.String()
}

func methodLabel(m string) string {
	if m == "number" {
		return "数字起卦"
	}
	return "时间起卦"
}

// isNumbers checks if the question is actually number input (not a real question).
func isNumbers(s string) bool {
	var a, b, c int
	n, _ := fmt.Sscanf(s, "%d,%d,%d", &a, &b, &c)
	return n == 3
}

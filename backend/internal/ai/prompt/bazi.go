// Package prompt hosts the AI prompt templates for every fortune
// engine. Each engine gets its own file (bazi.go, tarot.go, …) that
// turns a structured fortune.Result into a ready-to-send prompt.
//
// Templates live in the ai package tree (not in fortune) so the
// fortune engines stay pure-data: they compute, the prompt layer
// phrases. The fortune.BaziEngine.BuildPrompt method delegates here.
//
// Conventions across all templates:
//   - System: role + output rules +免责声明; language-driven.
//   - User:   the structured chart as plain text + the question.
//   - Tier:   free → brief template; paid → deep template.
//   - i18n:   a Lang field on fortune.Input selects the output
//             language; unknown langs fall back to 简体中文.
package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

// baziLang bundles the per-language strings a bazi prompt uses. We
// keep it small (system persona, section headings, disclaimers) so
// adding a language is one struct literal, not a rewrite.
type baziLang struct {
	langName  string // human label for logging
	persona   string // "你是一位资深的八字命理师…"
	rules     []string
	disclaimer string
	sections  baziSections
	briefHint string // appended for free tier
	deepHint  string // appended for paid tier
}

type baziSections struct {
	pattern   string // 格局分析
	yongShen  string // 用神喜忌
	personality string // 性格特质
	career    string // 事业方向
	wealth    string // 财运
	love      string // 感情
	health    string // 健康提示
}

var baziLangs = map[string]baziLang{
	"zh-CN": {
		langName: "简体中文",
		persona:  "你是一位资深的八字命理师，精通子平法、格局用神与五行生克，擅长将传统命理转化为现代人易理解的分析。",
		rules: []string{
			"使用简体中文回答",
			"输出分章节，使用给定的章节标题",
			"语气平和客观，避免绝对化断语，强调\"趋势\"与\"可能性\"",
			"不得输出医疗诊断、投资建议、违法内容的明确指令",
			"结尾附简短免责声明",
		},
		disclaimer: "命理仅供参考，人生在于自身抉择与努力。",
		sections: baziSections{
			pattern: "一、格局分析", yongShen: "二、用神喜忌",
			personality: "三、性格特质", career: "四、事业方向",
			wealth: "五、财运", love: "六、感情", health: "七、健康提示",
		},
		briefHint: "请给出简明扼要的解读（约 400-600 字），突出最核心的几点，每章 1-3 句。",
		deepHint:  "请进行深度解读，结合大运流年走势给出十年阶段性的趋势分析与建议，每章展开论述。",
	},
	"zh-TW": {
		langName: "繁體中文",
		persona:  "你是一位資深的八字命理師，精通子平法、格局用神與五行生剋，擅長將傳統命理轉化為現代人易理解的分析。",
		rules: []string{
			"使用繁體中文回答",
			"輸出分章節，使用給定的章節標題",
			"語氣平和客觀，避免絕對化斷語，強調「趨勢」與「可能性」",
			"不得輸出醫療診斷、投資建議、違法內容的明確指令",
			"結尾附簡短免責聲明",
		},
		disclaimer: "命理僅供參考，人生在於自身抉擇與努力。",
		sections: baziSections{
			pattern: "一、格局分析", yongShen: "二、用神喜忌",
			personality: "三、性格特質", career: "四、事業方向",
			wealth: "五、財運", love: "六、感情", health: "七、健康提示",
		},
		briefHint: "請給出簡明扼要的解讀（約 400-600 字），突出最核心的幾點，每章 1-3 句。",
		deepHint:  "請進行深度解讀，結合大運流年走勢給出十年階段性的趨勢分析與建議，每章展開論述。",
	},
}

// resolveBaziLang picks the language bundle for Input.Lang, falling
// back to 简体中文. Keeping the fallback explicit (not silent) means
// an unconfigured lang never produces an empty persona.
func resolveBaziLang(lang string) baziLang {
	if l, ok := baziLangs[lang]; ok {
		return l
	}
	return baziLangs["zh-CN"]
}

// BaziBuild constructs the AI prompt for a bazi result. It is the
// single source of truth for the bazi prompt — fortune.BaziEngine
// .BuildPrompt delegates here so the fortune package never has to
// import the ai package.
//
// The chart is rendered as plain Chinese text inside User (not JSON)
// because models reason more naturally over readable prose, and the
// structured result is already in the API response for the UI to
// display directly.
func BaziBuild(in fortune.Input, res *fortune.Result) (*fortune.PromptSpec, error) {
	if res == nil || res.Kind != fortune.KindBazi {
		return nil, fmt.Errorf("prompt/bazi: expected kind %q, got %q", fortune.KindBazi, kindOrEmpty(res))
	}
	chart, ok := res.Data.(*fortune.BaziChart)
	if !ok {
		return nil, fmt.Errorf("prompt/bazi: result.Data is not *fortune.BaziChart (got %T)", res.Data)
	}

	depth := in.InterpretDepth
	if depth == "" {
		depth = fortune.DepthBrief
	}
	tier := fortune.TierFree
	if depth == fortune.DepthDeep {
		tier = fortune.TierPaid
	}

	L := resolveBaziLang(in.Lang)
	system := buildBaziSystem(L)
	user := buildBaziUser(L, chart, depth)

	return &fortune.PromptSpec{
		System: system,
		User:   user,
		Tier:   tier,
	}, nil
}

func kindOrEmpty(r *fortune.Result) string {
	if r == nil {
		return ""
	}
	return r.Kind
}

// buildBaziSystem assembles the system prompt: persona + numbered
// rules + section headings (so the model emits exactly the chapters
// the UI expects to render) + disclaimer.
func buildBaziSystem(L baziLang) string {
	var b strings.Builder
	b.WriteString(L.persona)
	b.WriteString("\n\n请根据用户提供的八字排盘数据给出专业命理解读。要求：\n")
	for i, r := range L.rules {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	b.WriteString("\n请按以下章节顺序输出：\n")
	b.WriteString(L.sections.pattern + "\n")
	b.WriteString(L.sections.yongShen + "\n")
	b.WriteString(L.sections.personality + "\n")
	b.WriteString(L.sections.career + "\n")
	b.WriteString(L.sections.wealth + "\n")
	b.WriteString(L.sections.love + "\n")
	b.WriteString(L.sections.health + "\n")
	fmt.Fprintf(&b, "\n免责声明：%s", L.disclaimer)
	return b.String()
}

// buildBaziUser lays out the chart as plain text the model reasons
// over, then appends the depth-specific instruction.
func buildBaziUser(L baziLang, c *fortune.BaziChart, depth string) string {
	var b strings.Builder
	b.WriteString("【八字排盘】\n")
	b.WriteString("公历：" + c.Solar + "\n")
	b.WriteString("农历：" + c.Lunar + "\n")
	if c.TrueSolar {
		b.WriteString("校正：" + c.Correction + "\n")
	}

	b.WriteString("\n【四柱】\n")
	for _, p := range c.Pillars {
		ganWX, zhiWX := splitWuXing(p.WuXing)
		fmt.Fprintf(&b, "%s柱：%s（%s%s）纳音 %s | 天干十神 %s | 地支十神 %s | 十二长生 %s\n",
			p.Position, p.GanZhi, ganWX, zhiWX, p.NaYin, p.ShiShenGan,
			strings.Join(p.ShiShenZhi, "/"), p.DiShi)
		fmt.Fprintf(&b, "  藏干：%s | 旬：%s | 旬空：%s\n",
			strings.Join(p.HideGan, "/"), p.Xun, p.XunKong)
	}

	b.WriteString("\n【胎元/命宫/身宫】\n")
	fmt.Fprintf(&b, "胎元 %s（%s） | 胎息 %s（%s） | 命宫 %s（%s） | 身宫 %s（%s）\n",
		c.TaiYuan, c.TaiYuanNaYin, c.TaiXi, c.TaiXiNaYin,
		c.MingGong, c.MingGongNaYin, c.ShenGong, c.ShenGongNaYin)

	if len(c.ShenSha) > 0 {
		b.WriteString("\n【神煞】\n")
		for _, s := range c.ShenSha {
			fmt.Fprintf(&b, "%s（%s柱 %s）：%s\n", s.Name, s.Position, s.GanZhi, s.Note)
		}
	}

	b.WriteString("\n【大运】\n")
	fmt.Fprintf(&b, "起运：%d年%d月%d日%d时（%s），%s\n",
		c.StartYear, c.StartMonth, c.StartDay, c.StartHour,
		c.StartSolar, dirLabel(c.Forward))
	for _, dy := range c.DaYun {
		if dy.Index == 0 {
			continue
		}
		fmt.Fprintf(&b, "第%d运 %s（%d-%d岁，%d-%d年）\n",
			dy.Index, dy.GanZhi, dy.StartAge, dy.EndAge, dy.StartYear, dy.EndYear)
	}

	b.WriteString("\n【五行统计】\n")
	for _, s := range c.WuXingStats {
		fmt.Fprintf(&b, "%s：%d次（%d%%）\n", s.Element, s.Count, s.Percent)
	}
	fmt.Fprintf(&b, "旺衰：%s\n", c.WangShuai.Summary)
	fmt.Fprintf(&b, "用神初判：%s（喜 %s / 忌 %s）——%s\n",
		c.YongYin.YongShen, strings.Join(c.YongYin.Xi, "/"),
		strings.Join(c.YongYin.Ji, "/"), c.YongYin.Reason)

	b.WriteString("\n【解读要求】\n")
	if depth == fortune.DepthDeep {
		b.WriteString(L.deepHint)
	} else {
		b.WriteString(L.briefHint)
	}
	return b.String()
}

// splitWuXing splits a pillar's "木水"-style WuXing string into the
// gan element and the zhi element. Each element is one Chinese char
// (3 UTF-8 bytes), so a clean half-split works.
func splitWuXing(wuxing string) (ganWX, zhiWX string) {
	runes := []rune(wuxing)
	if len(runes) >= 2 {
		return string(runes[0]), string(runes[1])
	}
	return wuxing, ""
}

func dirLabel(forward bool) string {
	if forward {
		return "顺排"
	}
	return "逆排"
}

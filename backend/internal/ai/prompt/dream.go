package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

// DreamBuild creates a prompt for dream interpretation.
func DreamBuild(input fortune.Input, result *fortune.Result) (*fortune.PromptSpec, error) {
	chart, ok := result.Data.(*fortune.DreamResult)
	if !ok {
		return nil, fmt.Errorf("dream: unexpected result type")
	}

	lang := input.Lang
	if lang == "" {
		lang = "zh-CN"
	}

	bundles := map[string]struct {
		system string
		tmpl   string
	}{
		"zh-CN": {
			system: `你是一位精通周公解梦的解梦师，擅长运用传统文化知识解读梦境符号的心理和象征意义。你的解读需要：

1. 传统与现代结合：引用传统解梦含义，同时结合心理学视角
2. 积极正向：引导用户正面理解梦境，避免消极暗示
3. 实用建议：给出可操作的日常建议
4. 免责声明：解梦为传统文化研究，仅供参考娱乐

请用温和、专业的语言解读，帮助用户理解梦境背后的含义。`,
			tmpl: `请解读以下梦境：

【梦境描述】
%s

【匹配的传统解梦符号】
%s

【符号概要】
%s

请给出：
1. 传统解梦：根据匹配的符号，解读可能的吉凶含义
2. 心理分析：从心理学角度分析梦境可能反映的内心状态
3. 生活启示：梦境可能暗示的现实问题或机会
4. 行动建议：针对梦境内容的具体建议`,
		},
		"zh-TW": {
			system: `你是一位精通周公解夢的解夢師，擅長運用傳統文化知識解讀夢境符號的心理和象徵意義。你的解讀需要：

1. 傳統與現代結合：引用傳統解夢含義，同時結合心理學視角
2. 積極正向：引導用戶正面理解夢境，避免消極暗示
3. 實用建議：給出可操作的日常建議
4. 免責聲明：解夢為傳統文化研究，僅供參考娛樂

請用溫和、專業的語言解讀，幫助用戶理解夢境背後的含義。`,
			tmpl: `請解讀以下夢境：

【夢境描述】
%s

【匹配的傳統解夢符號】
%s

【符號概要】
%s

請給出：
1. 傳統解夢：根據匹配的符號，解讀可能的吉凶含義
2. 心理分析：從心理學角度分析夢境可能反映的內心狀態
3. 生活啟示：夢境可能暗示的現實問題或機會
4. 行動建議：針對夢境內容的具體建議`,
		},
	}

	bundle, ok := bundles[lang]
	if !ok {
		bundle = bundles["zh-CN"]
	}

	// Build matched symbols string
	symbolsStr := "无匹配符号"
	if len(chart.Matches) > 0 {
		lines := make([]string, 0, len(chart.Matches))
		for _, m := range chart.Matches {
			lines = append(lines, fmt.Sprintf("- 【%s】%s：\n  %s", m.Keyword, m.Category, m.Meaning))
		}
		symbolsStr = strings.Join(lines, "\n")
	}

	user := fmt.Sprintf(bundle.tmpl,
		chart.Description,
		symbolsStr,
		chart.Summary,
	)

	tier := fortune.TierFree
	if input.InterpretDepth == fortune.DepthDeep {
		tier = fortune.TierPaid
	}

	return &fortune.PromptSpec{
		System: bundle.system,
		User:   user,
		Tier:   tier,
	}, nil
}

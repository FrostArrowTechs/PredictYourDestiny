package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

// AstrologyBuild creates a prompt for natal chart interpretation.
func AstrologyBuild(input fortune.Input, result *fortune.Result) (*fortune.PromptSpec, error) {
	chart, ok := result.Data.(*fortune.AstrologyResult)
	if !ok {
		return nil, fmt.Errorf("astrology: unexpected result type")
	}

	lang := input.Lang
	if lang == "" {
		lang = "zh-CN"
	}

	bundles := map[string]struct {
		system string
	}{
		"zh-CN": {
			system: `你是一位专业的西洋占星师，精通本命盘解读。你的任务是根据用户的星盘数据，提供客观、有深度的性格分析与人生指引。

【重要声明】
占星学是一种古老的象征系统，本解读仅供参考与娱乐，不构成任何决策依据。人生轨迹由个人选择与努力决定。

【数据限制】
当前数据来自娱乐性简化算法。只能解读提示中明确提供的近似行星落座和相位；不得补充或猜测上升、MC、宫位、逆行等未提供事实，也不得把近似结果描述为精确星历。

【输出格式】
请使用简体中文，结构清晰，语言优美。`,
		},
		"zh-TW": {
			system: `你是一位專業的西洋占星師，精通本命盤解讀。你的任務是根據用戶的星盤數據，提供客觀、有深度的性格分析與人生指引。

【重要聲明】
占星學是一種古老的象徵系統，本解讀僅供參考與娛樂，不構成任何決策依據。人生軌跡由個人選擇與努力決定。

【資料限制】
目前資料來自娛樂性簡化演算法。只能解讀提示中明確提供的近似行星落座和相位；不得補充或猜測上升、MC、宮位、逆行等未提供事實，也不得把近似結果描述為精確星曆。

【輸出格式】
請使用繁體中文，結構清晰，語言優美。`,
		},
	}

	bundle, ok := bundles[lang]
	if !ok {
		bundle = bundles["zh-CN"]
	}

	// Build user prompt manually (simple string replacement)
	user := buildAstrologyUserPrompt(chart, lang)

	return &fortune.PromptSpec{
		System: bundle.system,
		User:   user,
		Tier:   "free",
	}, nil
}

func buildAstrologyUserPrompt(chart *fortune.AstrologyResult, lang string) string {
	var sb strings.Builder

	if lang == "zh-TW" {
		sb.WriteString("請解讀以下娛樂性簡化占星結果。不要推斷未提供的上升、宮位或逆行：\n\n")
		sb.WriteString("【可用核心資料】\n")
		sb.WriteString(fmt.Sprintf("- 太陽星座：%s（核心自我、人生目標）\n", chart.SunSign))
		sb.WriteString(fmt.Sprintf("- 月亮星座：%s（內在情感、潛意識）\n\n", chart.MoonSign))

		sb.WriteString("【行星落座】\n")
		for _, p := range chart.Planets {
			sb.WriteString(fmt.Sprintf("- %s：%s %.1f°（近似）\n", p.Name, p.Sign, p.Degree))
		}
		sb.WriteString("\n")

		if len(chart.Aspects) > 0 {
			sb.WriteString("【主要相位】\n")
			for _, a := range chart.Aspects {
				sb.WriteString(fmt.Sprintf("- %s %s %s（誤差%.1f°）\n", a.Planet1, a.Aspect, a.Planet2, a.Orb))
			}
			sb.WriteString("\n")
		}

		sb.WriteString(fmt.Sprintf("【星盤摘要】%s\n\n", chart.ChartSummary))
		sb.WriteString("請從以下方面進行詳細解讀：\n")
		sb.WriteString("1. 核心性格特質與人生主題\n")
		sb.WriteString("2. 情感模式與內在需求\n")
		sb.WriteString("3. 事業與人生發展方向\n")
		sb.WriteString("4. 人際關係與感情運勢\n")
		sb.WriteString("5. 當前面臨的課題與建議")
	} else {
		sb.WriteString("请解读以下娱乐性简化占星结果。不要推断未提供的上升、宫位或逆行：\n\n")
		sb.WriteString("【可用核心数据】\n")
		sb.WriteString(fmt.Sprintf("- 太阳星座：%s（核心自我、人生目标）\n", chart.SunSign))
		sb.WriteString(fmt.Sprintf("- 月亮星座：%s（内在情感、潜意识）\n\n", chart.MoonSign))

		sb.WriteString("【行星落座】\n")
		for _, p := range chart.Planets {
			sb.WriteString(fmt.Sprintf("- %s：%s %.1f°（近似）\n", p.Name, p.Sign, p.Degree))
		}
		sb.WriteString("\n")

		if len(chart.Aspects) > 0 {
			sb.WriteString("【主要相位】\n")
			for _, a := range chart.Aspects {
				sb.WriteString(fmt.Sprintf("- %s %s %s（误差%.1f°）\n", a.Planet1, a.Aspect, a.Planet2, a.Orb))
			}
			sb.WriteString("\n")
		}

		sb.WriteString(fmt.Sprintf("【星盘摘要】%s\n\n", chart.ChartSummary))
		sb.WriteString("请从以下方面进行详细解读：\n")
		sb.WriteString("1. 核心性格特质与人生主题\n")
		sb.WriteString("2. 情感模式与内在需求\n")
		sb.WriteString("3. 事业与人生发展方向\n")
		sb.WriteString("4. 人际关系与感情运势\n")
		sb.WriteString("5. 当前面临的课题与建议")
	}

	return sb.String()
}

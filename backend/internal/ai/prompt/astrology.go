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
		tmpl   string
	}{
		"zh-CN": {
			system: `你是一位专业的西洋占星师，精通本命盘解读。你的任务是根据用户的星盘数据，提供客观、有深度的性格分析与人生指引。

【重要声明】
占星学是一种古老的象征系统，本解读仅供参考与娱乐，不构成任何决策依据。人生轨迹由个人选择与努力决定。

【解读原则】
1. 综合分析太阳、月亮、上升星座的核心特质
2. 解读各行星落座的性格影响
3. 说明行星所在宫位的生活领域
4. 分析主要相位对性格与命运的影响
5. 给出建设性的建议与方向

【输出格式】
请使用简体中文，结构清晰，语言优美。`,
			tmpl: `请为我解读以下本命盘：

【核心三要素】
- 太阳星座：{{.SunSign}}（核心自我、人生目标）
- 月亮星座：{{.MoonSign}}（内在情感、潜意识）
- 上升星座：{{.Ascendant}}（外在形象、人生起点）

【行星落座】
{{range .Planets}}- {{.Name}}：{{.Sign}} {{printf "%.1f" .Degree}}° 第{{.House}}宫{{if .Retrograde}}（逆行）{{end}}
{{end}}
【主要相位】
{{range .Aspects}}- {{.Planet1}} {{.Aspect}} {{.Planet2}}（误差{{printf "%.1f" .Orb}}°）
{{end}}
【星盘摘要】{{.ChartSummary}}

请从以下方面进行详细解读：
1. 核心性格特质与人生主题
2. 情感模式与内在需求
3. 事业与人生发展方向
4. 人际关系与感情运势
5. 当前面临的课题与建议`,
		},
		"zh-TW": {
			system: `你是一位專業的西洋占星師，精通本命盤解讀。你的任務是根據用戶的星盤數據，提供客觀、有深度的性格分析與人生指引。

【重要聲明】
占星學是一種古老的象徵系統，本解讀僅供參考與娛樂，不構成任何決策依據。人生軌跡由個人選擇與努力決定。

【解讀原則】
1. 綜合分析太陽、月亮、上升星座的核心特質
2. 解讀各行星落座的性格影響
3. 說明行星所在宮位的生活領域
4. 分析主要相位對性格與命運的影響
5. 給出建設性的建議與方向

【輸出格式】
請使用繁體中文，結構清晰，語言優美。`,
			tmpl: `請為我解讀以下本命盤：

【核心三要素】
- 太陽星座：{{.SunSign}}（核心自我、人生目標）
- 月亮星座：{{.MoonSign}}（內在情感、潛意識）
- 上升星座：{{.Ascendant}}（外在形象、人生起點）

【行星落座】
{{range .Planets}}- {{.Name}}：{{.Sign}} {{printf "%.1f" .Degree}}° 第{{.House}}宮{{if .Retrograde}}（逆行）{{end}}
{{end}}
【主要相位】
{{range .Aspects}}- {{.Planet1}} {{.Aspect}} {{.Planet2}}（誤差{{printf "%.1f" .Orb}}°）
{{end}}
【星盤摘要】{{.ChartSummary}}

請從以下方面進行詳細解讀：
1. 核心性格特質與人生主題
2. 情感模式與內在需求
3. 事業與人生發展方向
4. 人際關係與感情運勢
5. 當前面臨的課題與建議`,
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
		sb.WriteString("請為我解讀以下本命盤：\n\n")
		sb.WriteString("【核心三要素】\n")
		sb.WriteString(fmt.Sprintf("- 太陽星座：%s（核心自我、人生目標）\n", chart.SunSign))
		sb.WriteString(fmt.Sprintf("- 月亮星座：%s（內在情感、潛意識）\n", chart.MoonSign))
		sb.WriteString(fmt.Sprintf("- 上升星座：%s（外在形象、人生起點）\n\n", chart.Ascendant))

		sb.WriteString("【行星落座】\n")
		for _, p := range chart.Planets {
			retroStr := ""
			if p.Retrograde {
				retroStr = "（逆行）"
			}
			sb.WriteString(fmt.Sprintf("- %s：%s %.1f° 第%d宮%s\n", p.Name, p.Sign, p.Degree, p.House, retroStr))
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
		sb.WriteString("请为我解读以下本命盘：\n\n")
		sb.WriteString("【核心三要素】\n")
		sb.WriteString(fmt.Sprintf("- 太阳星座：%s（核心自我、人生目标）\n", chart.SunSign))
		sb.WriteString(fmt.Sprintf("- 月亮星座：%s（内在情感、潜意识）\n", chart.MoonSign))
		sb.WriteString(fmt.Sprintf("- 上升星座：%s（外在形象、人生起点）\n\n", chart.Ascendant))

		sb.WriteString("【行星落座】\n")
		for _, p := range chart.Planets {
			retroStr := ""
			if p.Retrograde {
				retroStr = "（逆行）"
			}
			sb.WriteString(fmt.Sprintf("- %s：%s %.1f° 第%d宫%s\n", p.Name, p.Sign, p.Degree, p.House, retroStr))
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
package prompt

import (
	"fmt"

	"predictdestiny/internal/fortune"
)

// NameBuild creates a prompt for name analysis interpretation.
func NameBuild(input fortune.Input, result *fortune.Result) (*fortune.PromptSpec, error) {
	chart, ok := result.Data.(*fortune.NameResult)
	if !ok {
		return nil, fmt.Errorf("name: unexpected result type")
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
			system: `你是一位精通姓名学的资深命理师，擅长运用五格剖象法、三才配置、81数理等传统理论分析姓名的吉凶祸福。你的解读需要：

1. 客观分析：基于五格数理和三才配置给出专业判断
2. 传统底蕴：结合数理含义解释性格、运势特点
3. 积极正向：既指出问题也给出改善建议，避免消极表述
4. 免责声明：姓名学为传统文化研究，仅供参考娱乐

请用专业但通俗的语言解读，让用户理解名字背后的数理含义。`,
			tmpl: `请分析以下姓名：

【姓名】%s

【五格分析】
- 天格：%d（%s）— 祖上运势、先天条件
- 人格：%d（%s）— 核心人格、主运中心（最重要）
- 地格：%d（%s）— 青年运、基础运势
- 外格：%d（%s）— 外在表现、人际关系
- 总格：%d（%s）— 中晚年运、总体运势

【三才配置】%s（天格-人格-地格）

【传统规则匹配分】%d分（%s）

【笔画详情】
%s

请给出：
1. 五格数理解读：分析各格数字的数理含义
2. 三才配置分析：解读五行生克关系及影响
3. 性格特点：基于人格和总格推断
4. 运势走向：事业、感情、健康等方面
5. 使用限制：不得把传统规则匹配分描述为姓名的客观质量，不得编造未提供的读音、字义或重名率结论`,
		},
		"zh-TW": {
			system: `你是一位精通姓名學的資深命理師，擅長運用五格剖象法、三才配置、81數理等傳統理論分析姓名的吉凶禍福。你的解讀需要：

1. 客觀分析：基於五格數理和三才配置給出專業判斷
2. 傳統底蘊：結合數理含義解釋性格、運勢特點
3. 積極正向：既指出問題也給出改善建議，避免消極表述
4. 免責聲明：姓名學為傳統文化研究，僅供參考娛樂

請用專業但通俗的語言解讀，讓用戶理解名字背後的數理含義。`,
			tmpl: `請分析以下姓名：

【姓名】%s

【五格分析】
- 天格：%d（%s）— 祖上運勢、先天條件
- 人格：%d（%s）— 核心人格、主運中心（最重要）
- 地格：%d（%s）— 青年運、基礎運勢
- 外格：%d（%s）— 外在表現、人際關係
- 總格：%d（%s）— 中晚年運、總體運勢

【三才配置】%s（天格-人格-地格）

【傳統規則匹配分】%d分（%s）

【筆劃詳情】
%s

請給出：
1. 五格數理解讀：分析各格數字的數理含義
2. 三才配置分析：解讀五行生克關係及影響
3. 性格特點：基於人格和總格推斷
4. 運勢走向：事業、感情、健康等方面
5. 使用限制：不得把傳統規則匹配分描述為姓名的客觀品質，不得編造未提供的讀音、字義或重名率結論`,
		},
	}

	bundle, ok := bundles[lang]
	if !ok {
		bundle = bundles["zh-CN"]
	}

	// Build stroke details string
	detailsStr := ""
	for _, d := range chart.StrokeDetails {
		wuXing := d.WuXing
		if wuXing == "" {
			wuXing = "未知"
		}
		detailsStr += fmt.Sprintf("- %s字「%s」：%d画，五行属%s\n", d.Position, d.Char, d.Strokes, wuXing)
	}

	user := fmt.Sprintf(bundle.tmpl,
		chart.FullName,
		chart.TianGe, chart.TianGeLuck,
		chart.RenGe, chart.RenGeLuck,
		chart.DiGe, chart.DiGeLuck,
		chart.WaiGe, chart.WaiGeLuck,
		chart.ZongGe, chart.ZongGeLuck,
		chart.SanCai,
		chart.TraditionalMatchScore, chart.TraditionalMatchDesc,
		detailsStr,
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

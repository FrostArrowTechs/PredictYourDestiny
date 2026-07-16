// Package prompt provides the tarot prompt builder.
package prompt

import (
	"fmt"
	"strings"

	"predictdestiny/internal/fortune"
)

type tarotLang struct {
	langName   string
	persona    string
	rules      []string
	disclaimer string
	briefHint  string
	deepHint   string
}

var tarotLangs = map[string]tarotLang{
	"zh-CN": {
		langName: "简体中文",
		persona:  "你是一位精通塔罗牌解读的占卜师，擅长结合牌阵、牌面含义、正逆位以及牌与牌之间的关系，为用户提供有洞察力且正向的解读。",
		rules: []string{
			"使用简体中文回答",
			"语气神秘但积极，强调牌面带来的启示而非宿命",
			"结合牌阵中各位置的含义与牌的正逆位给出解读",
			"注意牌与牌之间的呼应与故事线",
			"若用户提出具体问题，解读需紧扣问题",
			"结尾附简短免责声明",
		},
		disclaimer: "塔罗解读仅供参考与自我觉察，未来掌握在自己手中。",
		briefHint:  "请简要解读本次牌阵，给出核心启示与建议（约 250-400 字）。",
		deepHint:   "请详细解读本次牌阵，逐张分析每张牌在其位置上的含义，梳理牌与牌之间的故事线，并给出针对问题的具体建议（约 600-1000 字）。",
	},
	"zh-TW": {
		langName: "繁體中文",
		persona:  "你是一位精通塔羅牌解讀的占卜師，擅長結合牌陣、牌面含義、正逆位以及牌與牌之間的關係，為用戶提供有洞察力且正向的解讀。",
		rules: []string{
			"使用繁體中文回答",
			"語氣神秘但積極，強調牌面帶來的啟示而非宿命",
			"結合牌陣中各位置的含義與牌的正逆位給出解讀",
			"注意牌與牌之間的呼應與故事線",
			"若用戶提出具體問題，解讀需緊扣問題",
			"結尾附簡短免責聲明",
		},
		disclaimer: "塔羅解讀僅供參考與自我覺察，未來掌握在自己手中。",
		briefHint:  "請簡要解讀本次牌陣，給出核心啟示與建議（約 250-400 字）。",
		deepHint:   "請詳細解讀本次牌陣，逐張分析每張牌在其位置上的含義，梳理牌與牌之間的故事線，並給出針對問題的具體建議（約 600-1000 字）。",
	},
}

func resolveTarotLang(lang string) tarotLang {
	if l, ok := tarotLangs[lang]; ok {
		return l
	}
	return tarotLangs["zh-CN"]
}

// TarotBuild constructs the AI prompt for a tarot reading.
func TarotBuild(in fortune.Input, res *fortune.Result) (*fortune.PromptSpec, error) {
	if res == nil || res.Kind != fortune.KindTarot {
		return nil, fmt.Errorf("prompt/tarot: expected kind %q, got %q", fortune.KindTarot, kindOrEmpty(res))
	}
	chart, ok := res.Data.(*fortune.TarotChart)
	if !ok {
		return nil, fmt.Errorf("prompt/tarot: result.Data is not *fortune.TarotChart (got %T)", res.Data)
	}

	depth := in.InterpretDepth
	if depth == "" {
		depth = fortune.DepthBrief
	}
	tier := fortune.TierFree
	if depth == fortune.DepthDeep {
		tier = fortune.TierPaid
	}

	L := resolveTarotLang(in.Lang)
	system := buildTarotSystem(L)
	user := buildTarotUser(L, chart, depth)

	return &fortune.PromptSpec{
		System: system,
		User:   user,
		Tier:   tier,
	}, nil
}

func buildTarotSystem(L tarotLang) string {
	var b strings.Builder
	b.WriteString(L.persona)
	b.WriteString("\n\n请根据用户提供的塔罗牌阵进行解读。要求：\n")
	for i, r := range L.rules {
		fmt.Fprintf(&b, "%d. %s\n", i+1, r)
	}
	fmt.Fprintf(&b, "\n免责声明：%s", L.disclaimer)
	return b.String()
}

func buildTarotUser(L tarotLang, c *fortune.TarotChart, depth string) string {
	var b strings.Builder
	b.WriteString("【塔罗牌阵】\n")
	fmt.Fprintf(&b, "牌阵：%s（共%d张）\n", c.Spread.Name, c.Spread.Count)

	if c.Question != "" {
		fmt.Fprintf(&b, "所问之事：%s\n", c.Question)
	} else {
		b.WriteString("所问之事：综合运势\n")
	}

	b.WriteString("\n【抽到的牌】\n")
	for _, card := range c.Cards {
		orientation := "正位"
		if card.Reversed {
			orientation = "逆位"
		}
		fmt.Fprintf(&b, "\n第%d张 — %s（%s · %s）\n", card.PositionIndex+1, card.PositionLabel, card.Name, orientation)
		fmt.Fprintf(&b, "牌名：%s / %s\n", card.Name, card.NameLatin)
		if card.Suit != "" {
			fmt.Fprintf(&b, "花色：%s（%s）\n", card.Suit, card.Arcana)
		} else {
			fmt.Fprintf(&b, "类别：%s\n", card.Arcana)
		}
		fmt.Fprintf(&b, "含义：%s\n", card.Meaning)
		if card.Keywords != "" {
			fmt.Fprintf(&b, "关键词：%s\n", card.Keywords)
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

package fortune

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// DreamEngine implements 周公解梦 (dream interpretation) based on traditional Chinese dream symbols.
// Data source: Traditional Chinese dream interpretation (public domain knowledge).
type DreamEngine struct {
	DB *gorm.DB
}

// DreamResult is the output of dream interpretation.
type DreamResult struct {
	Description string        `json:"description"` // The dream description
	Matches     []DreamMatch  `json:"matches"`     // Matching symbols found
	Summary     string        `json:"summary"`     // Brief summary
}

// DreamMatch represents a matched dream symbol.
type DreamMatch struct {
	Keyword  string `json:"keyword"`  // e.g., "蛇"
	Category string `json:"category"` // e.g., "动物"
	Meaning  string `json:"meaning"`  // Traditional interpretation
}

func init() {
	Register(DreamEngine{})
}

// Name returns the engine identifier.
func (e DreamEngine) Name() string {
	return KindDream
}

// Compute searches for matching dream symbols and returns interpretations.
func (e DreamEngine) Compute(in Input) (*Result, error) {
	description := in.Question
	if description == "" {
		return nil, fmt.Errorf("dream: description required (use Question field)")
	}

	// Find matching keywords
	matches := e.findMatches(description)

	// Build summary
	summary := e.buildSummary(matches)

	result := &DreamResult{
		Description: description,
		Matches:     matches,
		Summary:     summary,
	}

	return &Result{Kind: KindDream, Data: result}, nil
}

// findMatches searches for dream symbols in the description.
func (e DreamEngine) findMatches(description string) []DreamMatch {
	if e.DB == nil {
		return e.findMatchesFallback(description)
	}

	var entries []struct {
		Keyword  string
		Category string
		Meaning  string
	}

	// Search for keywords in the description
	// Using LIKE for case-insensitive matching
	descLower := strings.ToLower(description)
	err := e.DB.Table("dream_entries").
		Where("LOWER(?) LIKE '%' || keyword || '%'", descLower).
		Or("keyword IN (?)", e.extractKeywords(description)).
		Select("keyword, category, meaning").
		Find(&entries).Error

	if err != nil || len(entries) == 0 {
		return e.findMatchesFallback(description)
	}

	matches := make([]DreamMatch, 0, len(entries))
	for _, entry := range entries {
		matches = append(matches, DreamMatch{
			Keyword:  entry.Keyword,
			Category: entry.Category,
			Meaning:  entry.Meaning,
		})
	}

	return matches
}

// findMatchesFallback provides in-memory matching when DB is unavailable.
func (e DreamEngine) findMatchesFallback(description string) []DreamMatch {
	// Hardcoded common dream symbols for fallback
	symbolDB := []DreamMatch{
		{Keyword: "蛇", Category: "动物", Meaning: "梦见蛇通常暗示变化与转机。蛇在传统文化中象征智慧与灵性，也可能预示贵人相助或桃花运。"},
		{Keyword: "龙", Category: "动物", Meaning: "梦见龙是吉兆，象征权力、地位和成功。可能预示事业腾飞、贵人相助或实现抱负。"},
		{Keyword: "虎", Category: "动物", Meaning: "梦见老虎代表力量与威严，但也可能象征挑战或困难。若虎温顺则吉，若虎凶猛则需谨慎。"},
		{Keyword: "马", Category: "动物", Meaning: "梦见马象征自由与奔放，预示旅途顺利、事业进展。骏马奔腾主成功，瘦马疲惫主劳累。"},
		{Keyword: "牛", Category: "动物", Meaning: "梦见牛代表勤劳与收获，预示辛勤劳动将得回报。牛耕田主劳碌，牛安详主富贵。"},
		{Keyword: "羊", Category: "动物", Meaning: "梦见羊象征温和与吉祥，预示平安顺遂、家庭和睦。羊群主财运，独羊主孤独。"},
		{Keyword: "猪", Category: "动物", Meaning: "梦见猪主财运亨通，预示收入增加、生活富足。猪肥壮主大吉，猪瘦弱主小利。"},
		{Keyword: "狗", Category: "动物", Meaning: "梦见狗象征忠诚与友谊，预示有贵人相助或朋友来访。狗叫主有人来访，狗咬主防小人。"},
		{Keyword: "猫", Category: "动物", Meaning: "梦见猫代表灵性与直觉，可能暗示隐秘之事或女性贵人。猫温顺主好运，猫凶狠主防是非。"},
		{Keyword: "鱼", Category: "动物", Meaning: "梦见鱼象征财富与丰收，预示财运亨通、好事连连。鱼跃水面主升迁，鱼沉水底主平稳。"},
		{Keyword: "鸟", Category: "动物", Meaning: "梦见鸟代表自由与消息，预示好消息将至或有远行人归。鸟飞高主吉祥，鸟落枝主安宁。"},
		{Keyword: "凤凰", Category: "动物", Meaning: "梦见凤凰是大吉之兆，象征重生、高贵与荣耀。预示有大事临门、贵人相助或爱情美满。"},
		{Keyword: "水", Category: "自然", Meaning: "梦见水象征财运与情感。清水主财源广进、心情舒畅；浑水主阻碍、烦心事。大水主大发。"},
		{Keyword: "火", Category: "自然", Meaning: "梦见火代表热情与变化。火焰旺盛主财运旺、事业红；火势失控主需防冲动或争执。"},
		{Keyword: "山", Category: "自然", Meaning: "梦见山象征困难与目标。登山主克服困难、步步高升；山下仰望主目标远大，需努力。"},
		{Keyword: "树", Category: "自然", Meaning: "梦见树木代表生机与成长。大树茂盛主家业兴旺；枯树主健康需注意；开花结果主收获。"},
		{Keyword: "花", Category: "自然", Meaning: "梦见花象征美好与爱情。鲜花盛开主喜事临门、桃花运；花凋零主美好短暂，需珍惜。"},
		{Keyword: "雨", Category: "自然", Meaning: "梦见雨代表洗礼与转机。细雨绵绵主心情舒畅、烦忧消散；暴雨主有变故，需谨慎应对。"},
		{Keyword: "雪", Category: "自然", Meaning: "梦见雪象征纯洁与宁静。瑞雪主丰收、好运将至；暴雪主寒冷、需防小人与阻碍。"},
		{Keyword: "太阳", Category: "自然", Meaning: "梦见太阳代表光明与希望。日出主新开始、好运临门；日落主结束、过渡期；烈日主需谦逊。"},
		{Keyword: "月亮", Category: "自然", Meaning: "梦见月亮象征情感与直觉。明月高悬主心灵宁静、贵人相助；月缺主需耐心等待时机。"},
		{Keyword: "星星", Category: "自然", Meaning: "梦见星星代表希望与指引。繁星满天主吉祥、志向远大；流星主机遇短暂，需把握。"},
		{Keyword: "父母", Category: "人物", Meaning: "梦见父母象征关爱与责任。父母健康主家庭和睦；父母叮嘱主需聆听长辈建议。"},
		{Keyword: "孩子", Category: "人物", Meaning: "梦见孩子代表纯真与希望。孩子欢笑主喜事、新机会；孩子哭泣主需关注身边人或项目。"},
		{Keyword: "朋友", Category: "人物", Meaning: "梦见朋友象征社交与支持。朋友相聚主人际和谐、贵人运；朋友离去主需珍惜友情。"},
		{Keyword: "陌生人", Category: "人物", Meaning: "梦见陌生人代表未知与机会。陌生人友善主贵人将至；陌生人冷漠主需保持警觉。"},
		{Keyword: "老人", Category: "人物", Meaning: "梦见老人象征智慧与指引。老人指点主得贵人相助；老人安详主家庭和睦、长辈健康。"},
		{Keyword: "婚嫁", Category: "事件", Meaning: "梦见婚嫁象征新的结合与契约。自己出嫁主有新机遇；参加婚礼主有人情往来、社交活跃。"},
		{Keyword: "出行", Category: "事件", Meaning: "梦见出行代表变化与探索。旅途顺利主事业进展、机会将至；旅途受阻主需克服障碍。"},
		{Keyword: "考试", Category: "事件", Meaning: "梦见考试象征考验与压力。考试顺利主信心充足、目标可成；考试困难主需加强准备。"},
		{Keyword: "发财", Category: "事件", Meaning: "梦见发财代表渴望与机会。获得财富主财运好转；失去财富主需理财、防破财。"},
		{Keyword: "建房", Category: "事件", Meaning: "梦见建房象征创立与巩固。建房顺利主事业稳固、家庭安康；建房受阻主需调整计划。"},
		{Keyword: "死亡", Category: "事件", Meaning: "梦见死亡象征结束与重生。自己死亡主旧我结束、新机会来临；他人死亡主关系变化。"},
		{Keyword: "生病", Category: "事件", Meaning: "梦见生病代表身心需要休息。自己生病主需关注健康；他人生病主需关心身边人。"},
		{Keyword: "吵架", Category: "事件", Meaning: "梦见吵架象征矛盾与宣泄。与人争执主人际需调整；争吵后和解主问题可解决。"},
		{Keyword: "哭泣", Category: "事件", Meaning: "梦见哭泣代表情感释放。自己哭泣主心情好转、烦恼消散；他人哭泣主需给予关怀。"},
		{Keyword: "大笑", Category: "事件", Meaning: "梦见大笑象征开心与释放。开怀大笑主心情舒畅、好运临门；笑中带泪主悲喜交加。"},
		{Keyword: "飞翔", Category: "事件", Meaning: "梦见飞翔代表自由与超越。自由飞翔主志向远大、有望成功；飞不高主需继续努力。"},
		{Keyword: "坠落", Category: "事件", Meaning: "梦见坠落象征不安与失控。从高处坠落主需防意外、保持警觉；安全着陆主问题可解。"},
		{Keyword: "迷路", Category: "事件", Meaning: "梦见迷路代表困惑与选择。迷路寻找主人生处于转折、需明确方向；找到出路主豁然开朗。"},
	}

	descLower := strings.ToLower(description)
	matches := []DreamMatch{}
	seenKeywords := make(map[string]bool)

	for _, symbol := range symbolDB {
		if strings.Contains(descLower, strings.ToLower(symbol.Keyword)) {
			if !seenKeywords[symbol.Keyword] {
				matches = append(matches, symbol)
				seenKeywords[symbol.Keyword] = true
			}
		}
	}

	return matches
}

// extractKeywords extracts potential keywords from description.
func (e DreamEngine) extractKeywords(description string) []string {
	// Simple extraction: split by spaces and punctuation
	words := strings.FieldsFunc(description, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == '!' || r == '?'
	})

	keywords := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) > 0 {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// buildSummary creates a brief summary of matched symbols.
func (e DreamEngine) buildSummary(matches []DreamMatch) string {
	if len(matches) == 0 {
		return "未找到明确的传统解梦符号，建议详细描述梦境内容。"
	}

	categories := make(map[string][]string)
	for _, m := range matches {
		categories[m.Category] = append(categories[m.Category], m.Keyword)
	}

	parts := []string{}
	for cat, keywords := range categories {
		parts = append(parts, fmt.Sprintf("%s类：%s", cat, strings.Join(keywords, "、")))
	}

	return "梦境中出现：" + strings.Join(parts, "；") + "。"
}

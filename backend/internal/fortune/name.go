package fortune

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

var (
	ErrStrokeDictionaryUnavailable = errors.New("name: stroke dictionary unavailable")
	ErrUnknownCharacters           = errors.New("name: characters missing from stroke dictionary")
)

const (
	NameDictionaryVersion = "kangxi-seed-2026-07-v1"
	NameStrokeStandard    = "kangxi"
	NameRuleSetVersion    = "five-grid-81-v1"
)

// UnknownCharactersError identifies every character that must be added to the
// declared stroke dictionary before a deterministic analysis can be produced.
type UnknownCharactersError struct {
	Characters []string
}

func (e *UnknownCharactersError) Error() string {
	return fmt.Sprintf("%s: %s", ErrUnknownCharacters, strings.Join(e.Characters, ", "))
}

func (e *UnknownCharactersError) Unwrap() error { return ErrUnknownCharacters }

// NameEngine implements the Five格 (Five Structures) name analysis system.
// Data sources: 康熙字典笔画（公共领域）+ 81数理吉凶（传统公共知识）
type NameEngine struct {
	DB *gorm.DB
}

// NameInput is the request for name analysis or suggestion.
type NameInput struct {
	// For analysis: the full name to analyze
	FullName string `json:"fullName"`
	// For suggestion: surname + constraints
	Surname   string `json:"surname"`   // 姓氏
	GivenName string `json:"givenName"` // 名字（可选，用于分析）
	Gender    int    `json:"gender"`    // 0=女, 1=男
	// Bazi preferences (optional)
	WuXingPrefer string `json:"wuXingPrefer"` // 喜用五行
	// Language for prompts
	Lang string `json:"lang"`
}

// NameResult is the output of name analysis.
type NameResult struct {
	FullName          string   `json:"fullName"`
	Surname           string   `json:"surname"`
	GivenName         string   `json:"givenName"`
	SurnameConfirmed  bool     `json:"surnameConfirmed"`
	InputMode         string   `json:"inputMode"`
	Script            string   `json:"script"`
	StrokeStandard    string   `json:"strokeStandard"`
	DictionaryVersion string   `json:"dictionaryVersion"`
	RuleSetVersion    string   `json:"ruleSetVersion"`
	Warnings          []string `json:"warnings"`
	// 五格
	TianGe int `json:"tianGe"` // 天格
	RenGe  int `json:"renGe"`  // 人格
	DiGe   int `json:"diGe"`   // 地格
	WaiGe  int `json:"waiGe"`  // 外格
	ZongGe int `json:"zongGe"` // 总格
	// 三才
	SanCai string `json:"sanCai"` // 三才配置 e.g. "木火土"
	// 各格吉凶
	TianGeLuck            string           `json:"tianGeLuck"` // 吉/凶/半吉
	RenGeLuck             string           `json:"renGeLuck"`
	DiGeLuck              string           `json:"diGeLuck"`
	WaiGeLuck             string           `json:"waiGeLuck"`
	ZongGeLuck            string           `json:"zongGeLuck"`
	TraditionalMatchScore int              `json:"traditionalMatchScore"`
	TraditionalMatchDesc  string           `json:"traditionalMatchDesc"`
	Evaluations           []NameEvaluation `json:"evaluations"`
	// 笔画详情
	StrokeDetails []StrokeDetail `json:"strokeDetails"`
}

type NameEvaluation struct {
	Dimension string   `json:"dimension"`
	Status    string   `json:"status"`
	Score     *int     `json:"score,omitempty"`
	Summary   string   `json:"summary"`
	Evidence  []string `json:"evidence"`
	Warnings  []string `json:"warnings"`
}

// StrokeDetail shows per-character stroke info.
type StrokeDetail struct {
	Char           string `json:"char"`
	Strokes        int    `json:"strokes"`
	WuXing         string `json:"wuXing"`
	Position       string `json:"position"` // 姓/名
	CharIndex      int    `json:"charIndex"`
	Script         string `json:"script"`
	StrokeStandard string `json:"strokeStandard"`
	DataVersion    string `json:"dataVersion"`
	ReviewStatus   string `json:"reviewStatus"`
}

// SurnameInfo contains parsed surname data.
type SurnameInfo struct {
	Chars   []string
	Strokes []int
	Total   int
}

// GivenNameInfo contains parsed given name data.
type GivenNameInfo struct {
	Chars   []string
	Strokes []int
	Total   int
}

func init() {
	Register(NameEngine{})
}

// Name returns the engine identifier.
func (e NameEngine) Name() string {
	return KindName
}

// Compute runs the Five格 calculation.
func (e NameEngine) Compute(in Input) (*Result, error) {
	var surname, givenName []rune
	inputMode := "structured"
	warnings := []string{}
	if in.Surname != "" || in.GivenName != "" {
		if strings.TrimSpace(in.Surname) == "" || strings.TrimSpace(in.GivenName) == "" {
			return nil, fmt.Errorf("name: surname and givenName must both be provided")
		}
		surname, givenName = []rune(strings.TrimSpace(in.Surname)), []rune(strings.TrimSpace(in.GivenName))
		if !in.SurnameConfirmed {
			return nil, fmt.Errorf("name: surnameConfirmed must be true for structured input")
		}
	} else {
		fullName := strings.TrimSpace(in.Question)
		chars := []rune(fullName)
		if len(chars) < 2 {
			return nil, fmt.Errorf("name: at least 2 characters required")
		}
		surname, givenName = e.splitName(chars)
		inputMode = "legacy_auto_split"
		warnings = append(warnings, "surname was inferred from fullName; confirm surname and givenName for authoritative analysis")
	}
	fullName := string(surname) + string(givenName)
	script := in.Script
	if script == "" {
		script = "zh-Hans"
	}
	if script != "zh-Hans" && script != "zh-Hant" {
		return nil, fmt.Errorf("name: unsupported script %q", script)
	}
	strokeStandard := in.StrokeStandard
	if strokeStandard == "" {
		strokeStandard = NameStrokeStandard
	}
	if strokeStandard != NameStrokeStandard {
		return nil, fmt.Errorf("name: unsupported stroke standard %q", strokeStandard)
	}

	// Get stroke counts
	surnameInfo, err := e.buildSurnameInfo(surname)
	if err != nil {
		return nil, err
	}
	givenNameInfo, err := e.buildGivenNameInfo(givenName)
	if err != nil {
		return nil, err
	}

	// Calculate 五格
	tianGe := calcTianGe(surnameInfo)
	renGe := calcRenGe(surnameInfo, givenNameInfo)
	diGe := calcDiGe(givenNameInfo)
	waiGe := calcWaiGe(surnameInfo, givenNameInfo)
	zongGe := calcZongGe(surnameInfo, givenNameInfo)

	// Calculate 三才
	sanCai := calcSanCai(tianGe, renGe, diGe)

	// Get luck for each ge
	tianGeLuck := getNumLuck(tianGe)
	renGeLuck := getNumLuck(renGe)
	diGeLuck := getNumLuck(diGe)
	waiGeLuck := getNumLuck(waiGe)
	zongGeLuck := getNumLuck(zongGe)

	// Calculate overall score
	score := calcNameScore(tianGe, renGe, diGe, zongGe, sanCai)
	scoreDesc := getScoreDesc(score)

	// Build stroke details
	strokeDetails := e.buildStrokeDetails(surname, givenName, surnameInfo, givenNameInfo)

	result := &NameResult{
		FullName:              fullName,
		Surname:               string(surname),
		GivenName:             string(givenName),
		SurnameConfirmed:      in.SurnameConfirmed,
		InputMode:             inputMode,
		Script:                script,
		StrokeStandard:        strokeStandard,
		DictionaryVersion:     NameDictionaryVersion,
		RuleSetVersion:        NameRuleSetVersion,
		Warnings:              warnings,
		TianGe:                tianGe,
		RenGe:                 renGe,
		DiGe:                  diGe,
		WaiGe:                 waiGe,
		ZongGe:                zongGe,
		SanCai:                sanCai,
		TianGeLuck:            tianGeLuck,
		RenGeLuck:             renGeLuck,
		DiGeLuck:              diGeLuck,
		WaiGeLuck:             waiGeLuck,
		ZongGeLuck:            zongGeLuck,
		TraditionalMatchScore: score,
		TraditionalMatchDesc:  scoreDesc,
		Evaluations:           buildNameEvaluations(score, scoreDesc, strokeDetails),
		StrokeDetails:         strokeDetails,
	}

	return &Result{Kind: KindName, Data: result}, nil
}

func buildNameEvaluations(score int, scoreDesc string, details []StrokeDetail) []NameEvaluation {
	return []NameEvaluation{
		{Dimension: "traditional_numerology", Status: "available", Score: &score, Summary: scoreDesc,
			Evidence: []string{NameRuleSetVersion, NameDictionaryVersion}, Warnings: []string{"traditional rule match, not a scientific quality score"}},
		{Dimension: "pronunciation", Status: "unavailable", Summary: "pronunciation dictionary is not configured",
			Evidence: []string{}, Warnings: []string{"no tone, polyphone, dialect or homophone analysis was performed"}},
		{Dimension: "meaning", Status: "unavailable", Summary: "reviewed meaning dictionary is not configured",
			Evidence: []string{}, Warnings: []string{"no semantic or cultural-association conclusion was generated"}},
		{Dimension: "writing_compatibility", Status: "basic_check", Summary: fmt.Sprintf("all %d characters exist in the configured stroke dictionary", len(details)),
			Evidence: []string{NameDictionaryVersion}, Warnings: []string{"document, font and identity-system compatibility was not independently verified"}},
	}
}

// splitName separates surname and given name.
// Heuristic: common single-char surnames + 复姓 detection.
func (e NameEngine) splitName(chars []rune) (surname, givenName []rune) {
	// Common compound surnames (复姓)
	compoundSurnames := map[string]bool{
		"欧阳": true, "上官": true, "皇甫": true, "司徒": true,
		"司马": true, "诸葛": true, "东方": true, "南宫": true,
		"令狐": true, "慕容": true, "轩辕": true, "公孙": true,
		"长孙": true, "独孤": true, "百里": true, "端木": true,
		"司空": true, "夏侯": true,
	}

	// A compound surname plus a one-character given name is a valid
	// three-character full name, so detection must not depend on len >= 4.
	if len(chars) >= 3 {
		twoChar := string(chars[:2])
		if compoundSurnames[twoChar] {
			return chars[:2], chars[2:]
		}
	}

	// Default: single-char surname
	return chars[:1], chars[1:]
}

// getStrokeCounts retrieves stroke counts for characters from DB.
func (e NameEngine) getStrokeCounts(chars []rune) ([]int, error) {
	if e.DB == nil {
		return nil, ErrStrokeDictionaryUnavailable
	}

	strokes := make([]int, len(chars))
	unknown := make([]string, 0)
	for i, char := range chars {
		var row struct {
			Strokes int
		}
		err := e.DB.Table("character_strokes").
			Where("char = ? AND stroke_standard = ? AND data_version = ?", string(char), NameStrokeStandard, NameDictionaryVersion).
			Select("strokes").Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && row.Strokes <= 0) {
			unknown = append(unknown, string(char))
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("name: query stroke dictionary for %q: %w", string(char), err)
		}
		strokes[i] = row.Strokes
	}
	if len(unknown) > 0 {
		return nil, &UnknownCharactersError{Characters: unknown}
	}
	return strokes, nil
}

// 五格计算公式（传统五格剖象法）

// calcTianGe: 天格 = 姓氏笔画之和（复姓）或 姓氏笔画+1（单姓）
func calcTianGe(surname SurnameInfo) int {
	if len(surname.Strokes) == 1 {
		return surname.Strokes[0] + 1
	}
	return sum(surname.Strokes)
}

// calcRenGe: 人格 = 姓氏末字笔画 + 名字首字笔画
func calcRenGe(surname SurnameInfo, givenName GivenNameInfo) int {
	surnameLast := surname.Strokes[len(surname.Strokes)-1]
	if len(givenName.Strokes) > 0 {
		return surnameLast + givenName.Strokes[0]
	}
	return surnameLast + 1 // 单名
}

// calcDiGe: 地格 = 名字笔画之和（双名）或 名字笔画+1（单名）
func calcDiGe(givenName GivenNameInfo) int {
	if len(givenName.Strokes) == 1 {
		return givenName.Strokes[0] + 1
	}
	return sum(givenName.Strokes)
}

// calcWaiGe: 外格 = 姓氏首字笔画 + 名字末字笔画
func calcWaiGe(surname SurnameInfo, givenName GivenNameInfo) int {
	surnameFirst := surname.Strokes[0]
	if len(givenName.Strokes) > 0 {
		givenNameLast := givenName.Strokes[len(givenName.Strokes)-1]
		return surnameFirst + givenNameLast
	}
	return surnameFirst + 1 // 单名
}

// calcZongGe: 总格 = 姓名总笔画
func calcZongGe(surname SurnameInfo, givenName GivenNameInfo) int {
	return sum(surname.Strokes) + sum(givenName.Strokes)
}

// calcSanCai: 三才配置 = 天格、人格、地格个位数对应的五行
func calcSanCai(tianGe, renGe, diGe int) string {
	return numToWuXing(tianGe) + numToWuXing(renGe) + numToWuXing(diGe)
}

// numToWuXing converts a number to its five-element attribute.
// 1-2=木, 3-4=火, 5-6=土, 7-8=金, 9-0=水
func numToWuXing(n int) string {
	d := n % 10
	switch {
	case d == 1 || d == 2:
		return "木"
	case d == 3 || d == 4:
		return "火"
	case d == 5 || d == 6:
		return "土"
	case d == 7 || d == 8:
		return "金"
	default: // 9, 0
		return "水"
	}
}

// sum returns the sum of integers.
func sum(nums []int) int {
	total := 0
	for _, n := range nums {
		total += n
	}
	return total
}

// getNumLuck returns the luck level for a number (81数理).
func getNumLuck(n int) string {
	// Simplify to 1-81 cycle
	m := n % 81
	if m == 0 {
		m = 81
	}

	// 吉数 (good)
	lucky := map[int]bool{
		1: true, 3: true, 5: true, 6: true, 7: true, 8: true,
		11: true, 13: true, 15: true, 16: true, 17: true, 18: true,
		21: true, 23: true, 24: true, 25: true, 29: true, 31: true,
		32: true, 33: true, 35: true, 37: true, 39: true, 41: true,
		45: true, 47: true, 48: true, 52: true, 57: true, 61: true,
		63: true, 65: true, 67: true, 68: true, 71: true, 73: true,
		75: true, 77: true, 81: true,
	}

	// 凶数 (bad)
	unlucky := map[int]bool{
		2: true, 4: true, 9: true, 10: true, 12: true, 14: true,
		19: true, 20: true, 22: true, 26: true, 27: true, 28: true,
		30: true, 34: true, 36: true, 38: true, 40: true, 42: true,
		43: true, 44: true, 46: true, 49: true, 50: true, 51: true,
		53: true, 54: true, 55: true, 56: true, 58: true, 59: true,
		60: true, 62: true, 64: true, 66: true, 69: true, 70: true,
		72: true, 74: true, 76: true, 78: true, 79: true, 80: true,
	}

	if lucky[m] {
		return "吉"
	}
	if unlucky[m] {
		return "凶"
	}
	return "半吉"
}

// calcNameScore computes overall score based on 五格 and 三才.
func calcNameScore(tianGe, renGe, diGe, zongGe int, sanCai string) int {
	score := 60 // base score

	// Add points for each lucky 格
	if getNumLuck(tianGe) == "吉" {
		score += 8
	} else if getNumLuck(tianGe) == "凶" {
		score -= 5
	}

	if getNumLuck(renGe) == "吉" {
		score += 10 // 人格最重要
	} else if getNumLuck(renGe) == "凶" {
		score -= 8
	}

	if getNumLuck(diGe) == "吉" {
		score += 8
	} else if getNumLuck(diGe) == "凶" {
		score -= 5
	}

	if getNumLuck(zongGe) == "吉" {
		score += 10 // 总格很重要
	} else if getNumLuck(zongGe) == "凶" {
		score -= 8
	}

	// 三才配置评分
	sanCaiScore := getSanCaiScore(sanCai)
	score += sanCaiScore

	// Clamp to 1-100
	if score < 1 {
		score = 1
	}
	if score > 100 {
		score = 100
	}

	return score
}

// getSanCaiScore returns score modifier based on 三才配置.
func getSanCaiScore(sanCai string) int {
	// Good configurations (相生)
	goodConfigs := map[string]bool{
		"木火土": true, "火土金": true, "土金水": true, "金水木": true,
		"水木火": true, "木火": true, "火土": true, "土金": true,
		"金水": true, "水木": true,
	}

	// Bad configurations (相克)
	badConfigs := map[string]bool{
		"木土水": true, "土水火": true, "水火木": true, "火金土": true,
		"金木水": true, "木土": true, "土水": true, "水火": true,
		"火金": true, "金木": true,
	}

	if goodConfigs[sanCai] {
		return 15
	}
	if badConfigs[sanCai] {
		return -10
	}

	// Mixed or neutral
	return 0
}

// getScoreDesc returns description for a score.
func getScoreDesc(score int) string {
	if score >= 90 {
		return "极佳"
	}
	if score >= 80 {
		return "优秀"
	}
	if score >= 70 {
		return "良好"
	}
	if score >= 60 {
		return "中等"
	}
	if score >= 50 {
		return "一般"
	}
	return "欠佳"
}

// buildStrokeDetails creates detailed stroke info for each character.
func (e NameEngine) buildStrokeDetails(surname, givenName []rune, surnameInfo SurnameInfo, givenNameInfo GivenNameInfo) []StrokeDetail {
	details := make([]StrokeDetail, 0, len(surname)+len(givenName))

	for i, char := range surname {
		row := e.strokeMetadata(char)
		details = append(details, StrokeDetail{
			Char:      string(char),
			Strokes:   surnameInfo.Strokes[i],
			WuXing:    row.WuXing,
			Position:  "姓",
			CharIndex: i,
			Script:    row.Script, StrokeStandard: row.StrokeStandard, DataVersion: row.DataVersion, ReviewStatus: row.ReviewStatus,
		})
	}

	for i, char := range givenName {
		row := e.strokeMetadata(char)
		details = append(details, StrokeDetail{
			Char:      string(char),
			Strokes:   givenNameInfo.Strokes[i],
			WuXing:    row.WuXing,
			Position:  "名",
			CharIndex: i,
			Script:    row.Script, StrokeStandard: row.StrokeStandard, DataVersion: row.DataVersion, ReviewStatus: row.ReviewStatus,
		})
	}

	return details
}

type strokeMetadata struct {
	WuXing         string
	Script         string
	StrokeStandard string
	DataVersion    string
	ReviewStatus   string
}

func (e NameEngine) strokeMetadata(char rune) strokeMetadata {
	var row strokeMetadata
	if e.DB != nil {
		e.DB.Table("character_strokes").
			Where("char = ? AND stroke_standard = ? AND data_version = ?", string(char), NameStrokeStandard, NameDictionaryVersion).
			Take(&row)
	}
	return row
}

// buildSurnameInfo creates SurnameInfo from rune slice.
func (e NameEngine) buildSurnameInfo(chars []rune) (SurnameInfo, error) {
	strokes, err := e.getStrokeCounts(chars)
	if err != nil {
		return SurnameInfo{}, err
	}
	return SurnameInfo{
		Chars:   toStrings(chars),
		Strokes: strokes,
		Total:   sum(strokes),
	}, nil
}

// buildGivenNameInfo creates GivenNameInfo from rune slice.
func (e NameEngine) buildGivenNameInfo(chars []rune) (GivenNameInfo, error) {
	strokes, err := e.getStrokeCounts(chars)
	if err != nil {
		return GivenNameInfo{}, err
	}
	return GivenNameInfo{
		Chars:   toStrings(chars),
		Strokes: strokes,
		Total:   sum(strokes),
	}, nil
}

// toStrings converts rune slice to string slice.
func toStrings(chars []rune) []string {
	result := make([]string, len(chars))
	for i, r := range chars {
		result[i] = string(r)
	}
	return result
}

package fortune

import (
	"errors"
	"testing"
)

func TestZiweiChartDeclaresProvisionalRulePack(t *testing.T) {
	result, err := (ZiweiEngine{}).Compute(Input{Year: 2000, Month: 1, Day: 1, Hour: 12, Gender: GenderMale})
	if err != nil {
		t.Fatal(err)
	}
	chart := result.Data.(*ZiweiChart)
	if chart.AlgorithmVersion != ZiweiAlgorithmVersion || chart.RulePack.Version != ZiweiRuleSetVersion {
		t.Fatalf("missing active versions: %+v", chart)
	}
	if chart.RulePack.Status != "provisional" || len(chart.RulePack.ApproximateRules) == 0 || len(chart.RulePack.UnsupportedRules) == 0 {
		t.Fatalf("provisional scope is not explicit: %+v", chart.RulePack)
	}
	if len(chart.Palaces) != 12 {
		t.Fatalf("got %d palaces", len(chart.Palaces))
	}
}

func TestZiweiRejectsUnknownRuleSet(t *testing.T) {
	birth := BirthContext{Year: 2000, Month: 1, Day: 1, RuleSet: "unknown-v1"}
	_, err := (ZiweiEngine{}).Compute(Input{Birth: &birth, Year: 2000, Month: 1, Day: 1, Hour: 12})
	if !errors.Is(err, ErrZiweiUnsupportedRuleSet) {
		t.Fatalf("expected unsupported rule-set error, got %v", err)
	}
}

func TestZiweiRequiresExplicitLeapMonthRule(t *testing.T) {
	_, err := (ZiweiEngine{}).Compute(Input{Year: 2023, Month: 3, Day: 22, Hour: 12})
	if !errors.Is(err, ErrZiweiLeapMonthRuleRequired) {
		t.Fatalf("expected explicit leap-month-rule error, got %v", err)
	}
}

func TestResolveZiweiLeapMonthVariantsAtDayBoundary(t *testing.T) {
	tests := []struct {
		rawMonth, day int
		rule          string
		want          int
	}{
		{-2, 1, ZiweiLeapMonthAsNext, 3},
		{-2, 15, ZiweiLeapMonthSplit15, 2},
		{-2, 16, ZiweiLeapMonthSplit15, 3},
		{-12, 30, ZiweiLeapMonthAsNext, 1},
		{2, 1, "", 2},
	}
	for _, tt := range tests {
		got, err := resolveZiweiLunarMonth(tt.rawMonth, tt.day, tt.rule)
		if err != nil || got != tt.want {
			t.Fatalf("month=%d day=%d rule=%s got=%d err=%v", tt.rawMonth, tt.day, tt.rule, got, err)
		}
	}
}

func TestZiweiLeapMonthResultRecordsSelectedRule(t *testing.T) {
	result, err := (ZiweiEngine{}).Compute(Input{Year: 2023, Month: 3, Day: 22, Hour: 12, ZiweiLeapMonthRule: ZiweiLeapMonthSplit15})
	if err != nil {
		t.Fatal(err)
	}
	chart := result.Data.(*ZiweiChart)
	if !chart.LunarMonthWasLeap || chart.LeapMonthRule != ZiweiLeapMonthSplit15 || chart.EffectiveLunarMonth != 2 {
		t.Fatalf("leap-month decision is not traceable: %+v", chart)
	}
}

func TestZiweiLifeBodyPalaceCounting(t *testing.T) {
	tests := []struct{ month, hour, life, body int }{{1, 0, 0, 0}, {1, 1, 11, 1}, {8, 6, 1, 1}}
	for _, tt := range tests {
		life, body := ziweiLifeBodyPositions(tt.month, tt.hour)
		if life != tt.life || body != tt.body {
			t.Fatalf("month=%d hour=%d got life/body %d/%d", tt.month, tt.hour, life, body)
		}
	}
}

func TestComputeWuXingJuUsesLifePalaceNaYin(t *testing.T) {
	tests := []struct{ yearStem, lifeBranch, want string }{
		{"甲", "寅", "火六局"}, // 丙寅 炉中火
		{"甲", "辰", "木三局"}, // 戊辰 大林木
		{"乙", "寅", "土五局"}, // 戊寅 城头土
		{"戊", "寅", "水二局"}, // 甲寅 大溪水
	}
	for _, tt := range tests {
		if got := computeWuXingJu(tt.yearStem, tt.lifeBranch); got != tt.want {
			t.Fatalf("%s/%s got %s want %s", tt.yearStem, tt.lifeBranch, got, tt.want)
		}
	}
}

func TestLocateZiweiMatchesWaterBureauReferenceRow(t *testing.T) {
	want := []int{11, 0, 0, 1, 1, 2, 2, 3, 3, 4}
	for day, position := range want {
		if got := locateZiwei(day+1, "水二局"); got != position {
			t.Fatalf("water bureau day %d got position %d want %d", day+1, got, position)
		}
	}
}

func TestJuNumberCoversAllChineseBureauNames(t *testing.T) {
	want := map[string]int{"水二局": 2, "木三局": 3, "金四局": 4, "土五局": 5, "火六局": 6}
	for bureau, number := range want {
		if got := juNumber(bureau); got != number {
			t.Fatalf("%s got %d want %d", bureau, got, number)
		}
	}
	if got := juNumber("unknown"); got != 0 {
		t.Fatalf("unknown bureau silently fell back to %d", got)
	}
}

func TestPlaceMainStarsProducesFourteenUniqueStars(t *testing.T) {
	starsByPosition := map[int][]string{}
	placeMainStars(0, "水二局", starsByPosition)
	want := map[string]int{
		"紫微": 0, "天机": 11, "太阳": 9, "武曲": 8, "天同": 7, "廉贞": 4,
		"天府": 0, "太阴": 1, "贪狼": 2, "巨门": 3, "天相": 4, "天梁": 5, "七杀": 6, "破军": 10,
	}
	seen := map[string]int{}
	for position, stars := range starsByPosition {
		for _, star := range stars {
			seen[star] = position
		}
	}
	if len(seen) != 14 {
		t.Fatalf("got %d unique main stars: %+v", len(seen), seen)
	}
	for star, position := range want {
		if seen[star] != position {
			t.Fatalf("%s at %d, want %d", star, seen[star], position)
		}
	}
}

func TestPlaceAuxStarsUsesLunarMonthHourAndYearStem(t *testing.T) {
	starsByPosition := map[int][]string{}
	placeAuxStars("甲", "子", 1, 0, starsByPosition)
	want := map[string]int{"左辅": 2, "右弼": 8, "文昌": 8, "文曲": 2, "天魁": 11, "天钺": 5, "禄存": 0, "擎羊": 1, "陀罗": 11, "地空": 9, "地劫": 9, "火星": 0, "铃星": 8, "天马": 0}
	seen := map[string]int{}
	for position, stars := range starsByPosition {
		for _, star := range stars {
			seen[star] = position
		}
	}
	for star, position := range want {
		if seen[star] != position {
			t.Fatalf("%s at %d, want %d", star, seen[star], position)
		}
	}
}

func TestFireBellAndTravelHorseCoverAllYearBranches(t *testing.T) {
	want := map[string][3]string{
		"子": {"寅", "戌", "寅"}, "丑": {"卯", "戌", "亥"},
		"寅": {"丑", "卯", "申"}, "卯": {"酉", "戌", "巳"},
		"辰": {"寅", "戌", "寅"}, "巳": {"卯", "戌", "亥"},
		"午": {"丑", "卯", "申"}, "未": {"酉", "戌", "巳"},
		"申": {"寅", "戌", "寅"}, "酉": {"卯", "戌", "亥"},
		"戌": {"丑", "卯", "申"}, "亥": {"酉", "戌", "巳"},
	}
	for yearBranch, branches := range want {
		fire, bell, ok := fireBellBaseBranches(yearBranch)
		horse, horseOK := travelHorseBranch(yearBranch)
		if !ok || !horseOK || fire != branches[0] || bell != branches[1] || horse != branches[2] {
			t.Fatalf("%s got fire/bell/horse %s/%s/%s", yearBranch, fire, bell, horse)
		}
	}
}

func TestFireBellAdvanceTogetherByBirthHour(t *testing.T) {
	stars := map[int][]string{}
	placeAuxStars("甲", "辰", 1, 3, stars)
	positions := map[string]int{}
	for position, names := range stars {
		for _, name := range names {
			positions[name] = position
		}
	}
	if positions["火星"] != 3 || positions["铃星"] != 11 {
		t.Fatalf("hour shift got fire/bell %d/%d", positions["火星"], positions["铃星"])
	}
}

func TestFourTransformationsAreVersionedQuanjiVariant(t *testing.T) {
	want := map[string][4]string{
		"甲": {"廉贞", "破军", "武曲", "太阳"}, "乙": {"天机", "天梁", "紫微", "太阴"},
		"丙": {"天同", "天机", "文昌", "廉贞"}, "丁": {"太阴", "天同", "天机", "巨门"},
		"戊": {"贪狼", "太阴", "右弼", "天机"}, "己": {"武曲", "贪狼", "天梁", "文曲"},
		"庚": {"太阳", "武曲", "太阴", "文曲"}, "辛": {"巨门", "太阳", "文曲", "文昌"},
		"壬": {"天梁", "紫微", "左辅", "武曲"}, "癸": {"破军", "巨门", "太阴", "贪狼"},
	}
	labels := [4]string{"化禄", "化权", "化科", "化忌"}
	for stem, stars := range want {
		rules := fourTransformations(stem)
		if len(rules) != 4 {
			t.Fatalf("%s got %d transformations", stem, len(rules))
		}
		for i := range rules {
			if rules[i].Star != stars[i] || rules[i].Label != labels[i] {
				t.Fatalf("%s transformation %d = %+v", stem, i, rules[i])
			}
		}
	}
}

func TestChartPreservesStarSpecificTransformations(t *testing.T) {
	result, err := (ZiweiEngine{}).Compute(Input{Year: 2000, Month: 1, Day: 1, Hour: 12, Gender: GenderMale})
	if err != nil {
		t.Fatal(err)
	}
	chart := result.Data.(*ZiweiChart)
	if len(chart.Transformations) != 4 {
		t.Fatalf("got %d transformations: %+v", len(chart.Transformations), chart.Transformations)
	}
	for _, transformation := range chart.Transformations {
		if transformation.RuleSetVersion != ZiweiFourTransformRuleSet || transformation.Star == "" || transformation.Label == "" {
			t.Fatalf("untraceable transformation: %+v", transformation)
		}
	}
}

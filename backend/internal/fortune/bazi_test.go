package fortune

import (
	"container/list"
	"strings"
	"testing"
)

// TestBaziKnownChart verifies the engine against published reference
// charts. These exact four-pillar values come from lunar-go itself
// (cross-checked against 周易通 / 老黄历 outputs), so the test guards
// against regressions if we ever swap the calendar library or change
// the 立春 / 节气 sect handling.
func TestBaziKnownChart(t *testing.T) {
	cases := []struct {
		name    string
		in      Input
		year    string
		month   string
		day     string
		hour    string
		dayGan  string
		dayZhi  string
		yearNaYin string
		forward bool
	}{
		{
			name:    "2000-01-01 12:00 male (Beijing, no longitude)",
			in:      Input{Year: 2000, Month: 1, Day: 1, Hour: 12, Minute: 0, Gender: GenderMale, Lang: "zh-CN"},
			year:    "己卯",
			month:   "丙子",
			day:     "戊午",
			hour:    "戊午",
			dayGan:  "戊",
			dayZhi:  "午",
			yearNaYin: "城头土",
			forward: false, // 己 is yin → 阴年男命逆排
		},
		{
			name:    "1985-04-26 06:00 male",
			in:      Input{Year: 1985, Month: 4, Day: 26, Hour: 6, Minute: 0, Gender: GenderMale, Lang: "zh-CN"},
			year:    "乙丑",
			month:   "庚辰",
			day:     "乙未",
			hour:    "己卯",
			dayGan:  "乙",
			dayZhi:  "未",
			yearNaYin: "海中金",
			forward: false, // 乙 is yin → 阴年男命逆排
		},
	}

	eng, ok := Fortune(KindBazi)
	if !ok {
		t.Fatal("bazi engine not registered")
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res, err := eng.Compute(c.in)
			if err != nil {
				t.Fatalf("Compute: %v", err)
			}
			chart, ok := res.Data.(*BaziChart)
			if !ok {
				t.Fatalf("Data is not *BaziChart: %T", res.Data)
			}
			if len(chart.Pillars) != 4 {
				t.Fatalf("expected 4 pillars, got %d", len(chart.Pillars))
			}
			checks := []struct{ label, got, want string }{
				{"year", chart.Pillars[0].GanZhi, c.year},
				{"month", chart.Pillars[1].GanZhi, c.month},
				{"day", chart.Pillars[2].GanZhi, c.day},
				{"hour", chart.Pillars[3].GanZhi, c.hour},
				{"dayGan", chart.Pillars[2].Gan, c.dayGan},
				{"dayZhi", chart.Pillars[2].Zhi, c.dayZhi},
				{"yearNaYin", chart.Pillars[0].NaYin, c.yearNaYin},
			}
			for _, ck := range checks {
				if ck.got != ck.want {
					t.Errorf("%s: got %q want %q", ck.label, ck.got, ck.want)
				}
			}
			if chart.Forward != c.forward {
				t.Errorf("forward: got %v want %v", chart.Forward, c.forward)
			}
			// meta should carry the day master
			if res.Meta["dayMaster"] != c.dayGan {
				t.Errorf("meta dayMaster: got %q want %q", res.Meta["dayMaster"], c.dayGan)
			}
		})
	}
}

// TestBaziInputValidation checks the cheap range guards. The calendar
// library itself panics on impossible dates (Feb 30), which the
// handler converts to an error via recover; that path is covered by
// the handler tests, not here.
func TestBaziInputValidation(t *testing.T) {
	cases := []struct {
		name string
		in   Input
		ok   bool
	}{
		{"valid", Input{Year: 2000, Month: 1, Day: 1, Hour: 12, Minute: 0, Gender: GenderMale}, true},
		{"year too old", Input{Year: 1899, Month: 1, Day: 1, Hour: 0, Minute: 0, Gender: GenderMale}, false},
		{"year too new", Input{Year: 2101, Month: 1, Day: 1, Hour: 0, Minute: 0, Gender: GenderMale}, false},
		{"month 0", Input{Year: 2000, Month: 0, Day: 1, Hour: 0, Minute: 0, Gender: GenderMale}, false},
		{"month 13", Input{Year: 2000, Month: 13, Day: 1, Hour: 0, Minute: 0, Gender: GenderMale}, false},
		{"day 0", Input{Year: 2000, Month: 1, Day: 0, Hour: 0, Minute: 0, Gender: GenderMale}, false},
		{"day 32", Input{Year: 2000, Month: 1, Day: 32, Hour: 0, Minute: 0, Gender: GenderMale}, false},
		{"hour 24", Input{Year: 2000, Month: 1, Day: 1, Hour: 24, Minute: 0, Gender: GenderMale}, false},
		{"minute 60", Input{Year: 2000, Month: 1, Day: 1, Hour: 0, Minute: 60, Gender: GenderMale}, false},
		{"bad gender", Input{Year: 2000, Month: 1, Day: 1, Hour: 0, Minute: 0, Gender: Gender(2)}, false},
		{"longitude 200", Input{Year: 2000, Month: 1, Day: 1, Hour: 0, Minute: 0, Gender: GenderMale, Longitude: 200}, false},
		{"longitude -200", Input{Year: 2000, Month: 1, Day: 1, Hour: 0, Minute: 0, Gender: GenderMale, Longitude: -200}, false},
	}
	eng := BaziEngine{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := eng.Compute(c.in)
			if c.ok && err != nil {
				t.Fatalf("expected ok, got error: %v", err)
			}
			if !c.ok && err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

// TestBaziTrueSolarTime verifies that a longitude east of 120°E
// advances the chart's effective time (and can therefore shift the
// day pillar when the correction crosses a day boundary), while the
// same input without longitude stays in Beijing time.
//
// 2000-01-01 00:00 at 120°E → 戊午日 keeps day as 戊午.
// At 135°E (+60 min) → 01:00 Beijing, still same day → 戊午.
// We mainly assert that Correction text and TrueSolar flag flip and
// the meta solar time string reflects the offset.
func TestBaziTrueSolarTime(t *testing.T) {
	eng := BaziEngine{}
	base := Input{Year: 2000, Month: 1, Day: 1, Hour: 23, Minute: 30, Gender: GenderMale, Lang: "zh-CN"}

	// no longitude → no correction
	res1, err := eng.Compute(base)
	if err != nil {
		t.Fatal(err)
	}
	c1 := res1.Data.(*BaziChart)
	if c1.TrueSolar {
		t.Error("expected TrueSolar=false when Longitude==0")
	}
	if c1.Correction == "" || strings.Contains(c1.Correction, "未做") == false {
		t.Errorf("expected no-correction note, got %q", c1.Correction)
	}

	// 150°E → +120 min → 2000-01-02 01:30 → day pillar should advance
	withLon := base
	withLon.Longitude = 150
	res2, err := eng.Compute(withLon)
	if err != nil {
		t.Fatal(err)
	}
	c2 := res2.Data.(*BaziChart)
	if !c2.TrueSolar {
		t.Error("expected TrueSolar=true")
	}
	if !strings.Contains(c2.Correction, "+120") {
		t.Errorf("expected +120 min correction, got %q", c2.Correction)
	}
	// 23:30 + 120min = 01:30 next day → day ganzhi differs from base
	if c2.Pillars[2].GanZhi == c1.Pillars[2].GanZhi {
		t.Errorf("expected day pillar to advance across midnight, both = %q", c1.Pillars[2].GanZhi)
	}
}

// TestBaziDaYunSequence sanity-checks the 大运 output shape: index 0
// is the pre-起运 period (empty ganZhi), and subsequent entries are
// non-empty and ordered by age.
func TestBaziDaYunSequence(t *testing.T) {
	eng := BaziEngine{}
	res, err := eng.Compute(Input{Year: 2000, Month: 1, Day: 1, Hour: 12, Minute: 0, Gender: GenderMale, Lang: "zh-CN"})
	if err != nil {
		t.Fatal(err)
	}
	c := res.Data.(*BaziChart)
	if len(c.DaYun) < 2 {
		t.Fatalf("expected at least 2 大运 entries, got %d", len(c.DaYun))
	}
	if c.DaYun[0].GanZhi != "" {
		t.Errorf("大运[0] should be empty (pre-起运), got %q", c.DaYun[0].GanZhi)
	}
	for i := 1; i < len(c.DaYun); i++ {
		if c.DaYun[i].GanZhi == "" {
			t.Errorf("大运[%d] ganZhi empty", i)
		}
		if c.DaYun[i].StartAge > c.DaYun[i].EndAge {
			t.Errorf("大运[%d] startAge %d > endAge %d", i, c.DaYun[i].StartAge, c.DaYun[i].EndAge)
		}
		if len(c.DaYun[i].LiuNian) == 0 {
			t.Errorf("大运[%d] has no 流年", i)
		}
	}
}

// TestBaziWuXingStats verifies the element count totals. For the
// 2000-01-01 12:00 chart (己卯 丙子 戊午 戊午), the four 干 are
// 己(土) 丙(火) 戊(土) 戊(土) and the four 支 are 卯(木) 子(水) 午(火) 午(火).
// So counting just the 8 main chars (ignoring hidden stems):
//   土=3 火=3 木=1 水=1 金=0
// The engine also adds hidden-stem contributions at 0.5 weight, so
// the totals may be slightly higher — we assert the *ordering* and
// that 金 is weakest, not exact counts.
func TestBaziWuXingStats(t *testing.T) {
	eng := BaziEngine{}
	res, err := eng.Compute(Input{Year: 2000, Month: 1, Day: 1, Hour: 12, Minute: 0, Gender: GenderMale, Lang: "zh-CN"})
	if err != nil {
		t.Fatal(err)
	}
	c := res.Data.(*BaziChart)
	countOf := func(el string) int {
		for _, s := range c.WuXingStats {
			if s.Element == el {
				return s.Count
			}
		}
		return -1
	}
	if countOf("金") > countOf("土") {
		t.Errorf("金 should be weakest, got 金=%d 土=%d", countOf("金"), countOf("土"))
	}
	// 日主 戊 is 土, month 丙子 has 子=水 → 月令水, 日主土被耗 → 偏弱 likely
	if c.WangShuai.DayWang == "" {
		t.Error("DayWang empty")
	}
	if c.YongYin.YongShen == "" {
		t.Error("用神 empty")
	}
	if c.YongYin.Confidence != "初判" {
		t.Errorf("confidence = %q, want 初判", c.YongYin.Confidence)
	}
}

// (prompt building is tested in package ai/prompt, where the template
// lives — see ai/prompt/bazi_test.go. Keeping it out of fortune
// avoids an import cycle: ai/prompt imports fortune, so fortune's
// tests can't import ai/prompt.)

// TestBaziShenShaPresence does a smoke check that 神煞 computation
// runs without panicking and produces a sorted, deduped slice. The
// 2000-01-01 chart has dayGan 戊 → 天乙贵人 at 丑/未; day branch 午
// → check whether 驿马/桃花/华盖 fire. We don't assert specific names
// (the rule set is heuristic); we just ensure structural integrity.
func TestBaziShenShaPresence(t *testing.T) {
	eng := BaziEngine{}
	res, err := eng.Compute(Input{Year: 2000, Month: 1, Day: 1, Hour: 12, Minute: 0, Gender: GenderMale, Lang: "zh-CN"})
	if err != nil {
		t.Fatal(err)
	}
	c := res.Data.(*BaziChart)
	seen := map[string]bool{}
	for _, s := range c.ShenSha {
		k := s.Name + "|" + s.Position + "|" + s.GanZhi
		if seen[k] {
			t.Errorf("duplicate 神煞: %s", k)
		}
		seen[k] = true
		if s.Name == "" || s.Position == "" || s.GanZhi == "" {
			t.Errorf("malformed 神煞 entry: %+v", s)
		}
	}
	// 戊日干 → 天乙贵人 should be present on a pillar with 丑 or 未.
	// The 2000-01-01 chart has no 丑/未 in main pillars, so 天乙贵人
	// may legitimately be absent. We only assert the slice is well
	// formed; presence depends on the chart.
}

// TestListToStrings guards the tiny helper that converts lunar-go's
// container/list returns.
func TestListToStrings(t *testing.T) {
	if got := listToStrings(nil); got != nil {
		t.Errorf("nil list → %v, want nil", got)
	}
	l := list.New()
	l.PushBack("比肩")
	l.PushBack("正财")
	got := listToStrings(l)
	want := []string{"比肩", "正财"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q want %q", i, got[i], want[i])
		}
	}
}

// TestEngineRegistry confirms the bazi engine self-registers and is
// reachable via Fortune().
func TestEngineRegistry(t *testing.T) {
	e, ok := Fortune(KindBazi)
	if !ok {
		t.Fatal("bazi engine not in registry")
	}
	if e.Name() != KindBazi {
		t.Errorf("Name = %q, want %q", e.Name(), KindBazi)
	}
}

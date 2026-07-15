package prompt

import (
	"strings"
	"testing"

	"predictdestiny/internal/fortune"
)

// computeBazi builds a real chart via the bazi engine so the prompt
// tests exercise the same data the handler will feed BaziBuild.
func computeBazi(t *testing.T, in fortune.Input) *fortune.Result {
	t.Helper()
	eng, ok := fortune.Fortune(fortune.KindBazi)
	if !ok {
		t.Fatal("bazi engine not registered")
	}
	res, err := eng.Compute(in)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	return res
}

// TestBaziBuildBrief verifies the free-tier (brief) prompt is
// non-empty, tagged free, localized to zh-CN, and injects the chart.
func TestBaziBuildBrief(t *testing.T) {
	in := fortune.Input{
		Year: 2000, Month: 1, Day: 1, Hour: 12, Minute: 0,
		Gender: fortune.GenderMale, Lang: "zh-CN",
	}
	res := computeBazi(t, in)

	spec, err := BaziBuild(in, res)
	if err != nil {
		t.Fatalf("BaziBuild: %v", err)
	}
	if spec.System == "" || spec.User == "" {
		t.Fatal("prompt empty")
	}
	if spec.Tier != fortune.TierFree {
		t.Errorf("tier = %q, want free", spec.Tier)
	}
	// system prompt carries the persona + chapter headings
	if !strings.Contains(spec.System, "八字命理师") {
		t.Error("system should set the 命理师 persona")
	}
	if !strings.Contains(spec.System, "格局分析") {
		t.Error("system should list the 格局分析 chapter")
	}
	// user prompt injects the actual chart (day pillar 戊午 for this input)
	if !strings.Contains(spec.User, "戊午") {
		t.Error("user prompt should mention day pillar 戊午")
	}
	// brief hint appended
	if !strings.Contains(spec.User, "简明扼要") {
		t.Error("brief prompt should carry the 简明扼要 hint")
	}
}

// TestBaziBuildDeep verifies the paid-tier (deep) prompt is tagged
// paid and carries the deep hint.
func TestBaziBuildDeep(t *testing.T) {
	in := fortune.Input{
		Year: 2000, Month: 1, Day: 1, Hour: 12, Minute: 0,
		Gender: fortune.GenderMale, Lang: "zh-CN",
		InterpretDepth: fortune.DepthDeep,
	}
	res := computeBazi(t, in)

	spec, err := BaziBuild(in, res)
	if err != nil {
		t.Fatalf("BaziBuild: %v", err)
	}
	if spec.Tier != fortune.TierPaid {
		t.Errorf("tier = %q, want paid", spec.Tier)
	}
	if !strings.Contains(spec.User, "深度") {
		t.Error("deep prompt should mention 深度")
	}
	if !strings.Contains(spec.User, "大运流年") {
		t.Error("deep prompt should reference 大运流年")
	}
}

// TestBaziBuildI18n verifies the zh-TW language bundle is selected
// when Lang=zh-TW, and an unknown lang falls back to zh-CN.
func TestBaziBuildI18n(t *testing.T) {
	res := computeBazi(t, fortune.Input{
		Year: 2000, Month: 1, Day: 1, Hour: 12, Minute: 0,
		Gender: fortune.GenderMale, Lang: "zh-CN",
	})

	zhTW, err := BaziBuild(fortune.Input{
		Year: 2000, Month: 1, Day: 1, Hour: 12, Minute: 0,
		Gender: fortune.GenderMale, Lang: "zh-TW",
	}, res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(zhTW.System, "繁體中文") {
		t.Error("zh-TW system should request 繁體中文")
	}
	if !strings.Contains(zhTW.System, "資深") {
		t.Error("zh-TW system should use traditional 資深")
	}

	// unknown lang → zh-CN fallback (should NOT panic, should be 简体)
	fallback, err := BaziBuild(fortune.Input{
		Year: 2000, Month: 1, Day: 1, Hour: 12, Minute: 0,
		Gender: fortune.GenderMale, Lang: "klingon",
	}, res)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fallback.System, "简体中文") {
		t.Error("unknown lang should fall back to zh-CN")
	}
}

// TestBaziBuildRejectsWrongKind verifies the kind/data guards.
func TestBaziBuildRejectsWrongKind(t *testing.T) {
	in := fortune.Input{Year: 2000, Month: 1, Day: 1, Hour: 12, Gender: fortune.GenderMale}

	if _, err := BaziBuild(in, nil); err == nil {
		t.Error("nil result should error")
	}
	wrongKind := &fortune.Result{Kind: "tarot"}
	if _, err := BaziBuild(in, wrongKind); err == nil {
		t.Error("wrong kind should error")
	}
	// right kind but wrong data type
	badData := &fortune.Result{Kind: fortune.KindBazi, Data: "not a chart"}
	if _, err := BaziBuild(in, badData); err == nil {
		t.Error("wrong data type should error")
	}
}

// TestResolveBaziLangFallback is a tiny unit check on the fallback.
func TestResolveBaziLangFallback(t *testing.T) {
	if l := resolveBaziLang("zh-CN"); l.langName != "简体中文" {
		t.Errorf("zh-CN: got %q", l.langName)
	}
	if l := resolveBaziLang("zh-TW"); l.langName != "繁體中文" {
		t.Errorf("zh-TW: got %q", l.langName)
	}
	if l := resolveBaziLang("nonsense"); l.langName != "简体中文" {
		t.Errorf("fallback: got %q", l.langName)
	}
}

// TestSplitWuXing guards the rune-aware splitter.
func TestSplitWuXing(t *testing.T) {
	g, z := splitWuXing("木水")
	if g != "木" || z != "水" {
		t.Errorf("got %q/%q", g, z)
	}
	if g, _ := splitWuXing(""); g != "" {
		t.Errorf("empty: got %q", g)
	}
}

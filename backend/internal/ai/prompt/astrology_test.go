package prompt

import (
	"strings"
	"testing"

	"predictdestiny/internal/fortune"
)

func TestAstrologyPromptDoesNotInventUnsupportedFacts(t *testing.T) {
	chart := &fortune.AstrologyResult{
		AccuracyLabel: "娱乐性简化版",
		SunSign:       "摩羯座",
		MoonSign:      "天蝎座",
		Planets:       []fortune.PlanetInfo{{Name: "太阳", Sign: "摩羯座", Degree: 10}},
	}
	spec, err := AstrologyBuild(fortune.Input{Lang: "zh-CN"}, &fortune.Result{Kind: fortune.KindAstrology, Data: chart})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(spec.User, "第0宫") || strings.Contains(spec.User, "上升星座：") || strings.Contains(spec.User, "逆行）") {
		t.Fatalf("prompt exposed unsupported facts: %s", spec.User)
	}
	if !strings.Contains(spec.System, "不得补充或猜测") {
		t.Fatalf("system prompt lacks fact boundary: %s", spec.System)
	}
}

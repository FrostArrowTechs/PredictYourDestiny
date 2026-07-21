package fortune

import (
	"encoding/json"
	"testing"
)

type uncertaintyTestEngine struct{}

func (uncertaintyTestEngine) Name() string { return KindWeighbone }
func (uncertaintyTestEngine) Compute(in Input) (*Result, error) {
	return &Result{Kind: KindWeighbone, Data: map[string]any{
		"date":   "stable",
		"branch": ((in.Hour + 1) / 2) % 12,
	}}, nil
}

func TestComputeWithBirthUncertaintyMergesCandidatesAndComparesFacts(t *testing.T) {
	birth := BirthContext{Year: 2000, Month: 1, Day: 1, TimePrecision: PrecisionUnknown}
	result, err := ComputeWithBirthUncertainty(uncertaintyTestEngine{}, Input{Birth: &birth, Year: 2000, Month: 1, Day: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Data != nil {
		t.Fatalf("imprecise result must not select a chart: %+v", result.Data)
	}
	meta := result.ResultMetadata
	if meta == nil || len(meta.Variants) == 0 || len(meta.StableFacts) == 0 || len(meta.VariableFacts) == 0 {
		t.Fatalf("incomplete uncertainty result: %+v", meta)
	}
	if meta.StableFacts[0].Key != "date" || meta.StableFacts[0].Value != "stable" {
		t.Fatalf("unexpected stable facts: %+v", meta.StableFacts)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" {
		t.Fatal("empty JSON")
	}
}

func TestHourPrecisionSamplesBothEnds(t *testing.T) {
	hour := 12
	birth := BirthContext{Year: 2000, Month: 1, Day: 1, Hour: &hour, TimePrecision: PrecisionHour}
	clocks, err := uncertaintyCandidateClocks(KindAstrology, birth)
	if err != nil {
		t.Fatal(err)
	}
	if len(clocks) != 2 || clocks[0].Minute != 0 || clocks[1].Minute != 59 {
		t.Fatalf("candidate clocks = %+v", clocks)
	}
}

func TestUncertaintyDoesNotExposeProbability(t *testing.T) {
	birth := BirthContext{Year: 2000, Month: 1, Day: 1, TimePrecision: PrecisionUnknown}
	result, err := ComputeWithBirthUncertainty(uncertaintyTestEngine{}, Input{Birth: &birth})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result)
	for _, forbidden := range []string{`"probability":`, `"percentage":`, `"confidence":`} {
		if contains := string(encoded); len(contains) > 0 && jsonContains(encoded, forbidden) {
			t.Fatalf("response contains unsupported %q: %s", forbidden, encoded)
		}
	}
}

func jsonContains(data []byte, needle string) bool {
	for index := 0; index+len(needle) <= len(data); index++ {
		if string(data[index:index+len(needle)]) == needle {
			return true
		}
	}
	return false
}

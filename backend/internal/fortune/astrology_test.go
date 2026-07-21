package fortune

import "testing"

func TestSimplifiedAstrologyDoesNotExposeUnsupportedPrecision(t *testing.T) {
	result, err := (AstrologyEngine{}).Compute(Input{Year: 2000, Month: 1, Day: 1, Hour: 12})
	if err != nil {
		t.Fatal(err)
	}
	chart := result.Data.(*AstrologyResult)
	if chart.AccuracyLabel == "" {
		t.Fatal("missing simplified accuracy label")
	}
	if chart.Ascendant != "" || len(chart.Houses) != 0 {
		t.Fatalf("unsupported ascendant/houses exposed: %+v", chart)
	}
	for _, planet := range chart.Planets {
		if planet.House != 0 || planet.Retrograde {
			t.Fatalf("unsupported house/retrograde exposed for %+v", planet)
		}
	}
}

package fortune

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

type failingAstrologyCalculator struct {
	err   error
	calls *int
}

func (f failingAstrologyCalculator) Calculate(AstrologyCalculationInput) (*AstrologyResult, error) {
	(*f.calls)++
	return nil, f.err
}

func TestSimplifiedAstrologyDoesNotExposeUnsupportedPrecision(t *testing.T) {
	hour, minute := 12, 0
	birth := &BirthContext{Year: 2000, Month: 1, Day: 1, Hour: &hour, Minute: &minute, TimePrecision: PrecisionMinute, TimeZone: "Asia/Shanghai"}
	result, err := (AstrologyEngine{}).Compute(Input{Birth: birth, Year: 2000, Month: 1, Day: 1, Hour: hour, Minute: minute})
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

func TestAstrologyUsesIANAHistoricalOffset(t *testing.T) {
	hour, minute := 12, 0
	birth := &BirthContext{Year: 1990, Month: 7, Day: 1, Hour: &hour, Minute: &minute, TimePrecision: PrecisionMinute, TimeZone: "Asia/Shanghai"}
	result, err := (AstrologyEngine{}).Compute(Input{Birth: birth, Year: 1990, Month: 7, Day: 1, Hour: hour, Minute: minute})
	if err != nil {
		t.Fatal(err)
	}
	chart := result.Data.(*AstrologyResult)
	// China observed daylight saving time in July 1990 (UTC+09), which a
	// hard-coded modern UTC+08 offset cannot reproduce.
	if chart.UTCInstant != "1990-07-01T03:00:00Z" || chart.TimeZone != "Asia/Shanghai" {
		t.Fatalf("historical conversion = %s (%s)", chart.UTCInstant, chart.TimeZone)
	}
}

func TestAstrologyRejectsDSTGapAndFold(t *testing.T) {
	tests := []struct {
		month, day, hour int
		want             string
	}{
		{3, 14, 2, "does not exist"},
		{11, 7, 1, "ambiguous"},
	}
	for _, test := range tests {
		_, err := strictLocalTimeUTC(2021, test.month, test.day, test.hour, 30, "America/New_York")
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("transition error = %v, want %q", err, test.want)
		}
	}
}

func TestAstrologyRequiresIANATimeZone(t *testing.T) {
	_, err := (AstrologyEngine{}).Compute(Input{Birth: &BirthContext{}, Year: 2000, Month: 1, Day: 1, Hour: 12})
	if err == nil || !strings.Contains(err.Error(), "IANA birth time zone is required") {
		t.Fatalf("missing time-zone error = %v", err)
	}
}

func TestAstrologyCalculatorFailureNeverFallsBack(t *testing.T) {
	hour, minute := 12, 0
	birth := &BirthContext{Year: 2000, Month: 1, Day: 1, Hour: &hour, Minute: &minute, TimePrecision: PrecisionMinute, TimeZone: "UTC"}
	for _, sentinel := range []error{
		fmt.Errorf("%w: Placidus cusps undefined", ErrAstrologyHighLatitude),
		fmt.Errorf("%w: ephemeris file outside configured range", ErrAstrologyCalculationFailed),
	} {
		calls := 0
		engine := AstrologyEngine{Calculator: failingAstrologyCalculator{err: sentinel, calls: &calls}}
		result, err := engine.Compute(Input{Birth: birth, Year: 2000, Month: 1, Day: 1, Hour: hour, Minute: minute})
		if result != nil || !errors.Is(err, sentinel) || calls != 1 {
			t.Fatalf("result=%v err=%v calls=%d", result, err, calls)
		}
	}
}

func TestAstrologyNilCalculatorResultIsFailure(t *testing.T) {
	hour, minute := 12, 0
	birth := &BirthContext{Year: 2000, Month: 1, Day: 1, Hour: &hour, Minute: &minute, TimePrecision: PrecisionMinute, TimeZone: "UTC"}
	calls := 0
	engine := AstrologyEngine{Calculator: failingAstrologyCalculator{calls: &calls}}
	_, err := engine.Compute(Input{Birth: birth, Year: 2000, Month: 1, Day: 1, Hour: hour, Minute: minute})
	if !errors.Is(err, ErrAstrologyCalculationFailed) {
		t.Fatalf("nil-result error = %v", err)
	}
}

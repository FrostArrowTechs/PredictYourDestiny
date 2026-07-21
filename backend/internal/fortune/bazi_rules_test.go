package fortune

import (
	"math"
	"testing"
	"time"

	"github.com/6tail/lunar-go/calendar"
)

func baziInputWithRule(year, month, day, hour, minute int, ruleSet string) Input {
	birth := BirthContext{Year: year, Month: month, Day: day, Hour: intPtr(hour), Minute: intPtr(minute), TimePrecision: PrecisionMinute, TimeZone: "Asia/Shanghai", RuleSet: ruleSet}
	return Input{Birth: &birth, Year: year, Month: month, Day: day, Hour: hour, Minute: minute, Gender: GenderMale}
}

func TestBaziDayBoundaryRuleSets(t *testing.T) {
	engine := BaziEngine{}
	standard, err := engine.Compute(baziInputWithRule(2000, 1, 1, 23, 30, BaziRuleSetStandardV1))
	if err != nil {
		t.Fatal(err)
	}
	ziChu, err := engine.Compute(baziInputWithRule(2000, 1, 1, 23, 30, BaziRuleSetZiChuV1))
	if err != nil {
		t.Fatal(err)
	}
	standardChart := standard.Data.(*BaziChart)
	ziChuChart := ziChu.Data.(*BaziChart)
	if standardChart.DayBoundary != "midnight" || ziChuChart.DayBoundary != "zi_chu_23:00" {
		t.Fatalf("day boundaries = %q / %q", standardChart.DayBoundary, ziChuChart.DayBoundary)
	}
	if standardChart.Pillars[2].GanZhi == ziChuChart.Pillars[2].GanZhi {
		t.Fatalf("23:30 day pillars should differ by rule set: %s", standardChart.Pillars[2].GanZhi)
	}
}

func TestBaziUsesExactSolarTermBoundary(t *testing.T) {
	liChun := calendar.NewSolar(2000, 2, 4, 12, 0, 0).GetLunar().GetJieQiTable()["立春"]
	boundary := time.Date(liChun.GetYear(), time.Month(liChun.GetMonth()), liChun.GetDay(), liChun.GetHour(), liChun.GetMinute(), 0, 0, time.UTC)
	beforeTime, afterTime := boundary.Add(-time.Minute), boundary.Add(time.Minute)
	before := calendar.NewSolarFromDate(beforeTime)
	after := calendar.NewSolarFromDate(afterTime)
	engine := BaziEngine{}
	beforeResult, err := engine.Compute(baziInputWithRule(before.GetYear(), before.GetMonth(), before.GetDay(), before.GetHour(), before.GetMinute(), BaziRuleSetStandardV1))
	if err != nil {
		t.Fatal(err)
	}
	afterResult, err := engine.Compute(baziInputWithRule(after.GetYear(), after.GetMonth(), after.GetDay(), after.GetHour(), after.GetMinute(), BaziRuleSetStandardV1))
	if err != nil {
		t.Fatal(err)
	}
	beforeChart := beforeResult.Data.(*BaziChart)
	afterChart := afterResult.Data.(*BaziChart)
	if beforeChart.Pillars[0].GanZhi == afterChart.Pillars[0].GanZhi || beforeChart.Pillars[1].GanZhi == afterChart.Pillars[1].GanZhi {
		t.Fatalf("year/month pillars did not cross 立春 at %s: %s/%s -> %s/%s", liChun.ToYmdHms(), beforeChart.Pillars[0].GanZhi, beforeChart.Pillars[1].GanZhi, afterChart.Pillars[0].GanZhi, afterChart.Pillars[1].GanZhi)
	}
	if beforeChart.NextJie.Name != "立春" || afterChart.PreviousJie.Name != "立春" {
		t.Fatalf("jie evidence missing around boundary: %+v / %+v", beforeChart.NextJie, afterChart.PreviousJie)
	}
}

func TestEquationOfTimeMatchesNOAAFractionalYearFormulaReference(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	got := equationOfTimeMinutes(time.Date(2000, time.January, 1, 12, 0, 0, 0, location))
	// NOAA's published approximation is about -2.9 minutes at noon on Jan 1.
	if math.Abs(got-(-2.904)) > 0.02 {
		t.Fatalf("equation of time = %.4f minutes", got)
	}
}

func TestBaziApparentSolarTimeIncludesLongitudeAndEquationOfTime(t *testing.T) {
	longitude := 150.0
	input := baziInputWithRule(2000, 1, 1, 12, 0, BaziRuleSetStandardV1)
	input.Longitude = longitude
	input.Birth.Longitude = &longitude
	result, err := (BaziEngine{}).Compute(input)
	if err != nil {
		t.Fatal(err)
	}
	chart := result.Data.(*BaziChart)
	if !chart.TrueSolar || chart.SolarTimeMode != "local_apparent_solar" {
		t.Fatalf("solar mode = %+v", chart)
	}
	if chart.LongitudeCorrectionMinute != 120 || chart.EquationOfTimeMinute == 0 || chart.TotalCorrectionMinute == chart.LongitudeCorrectionMinute {
		t.Fatalf("correction components = longitude %d, equation %.1f, total %d", chart.LongitudeCorrectionMinute, chart.EquationOfTimeMinute, chart.TotalCorrectionMinute)
	}
}

func TestBaziMinuteDifferenceYunRule(t *testing.T) {
	result, err := (BaziEngine{}).Compute(baziInputWithRule(2022, 3, 9, 20, 51, BaziRuleSetStandardV1))
	if err != nil {
		t.Fatal(err)
	}
	chart := result.Data.(*BaziChart)
	if chart.StartYear != 8 || chart.StartMonth != 9 || chart.StartDay != 2 {
		t.Fatalf("start yun = %dy %dm %dd %dh", chart.StartYear, chart.StartMonth, chart.StartDay, chart.StartHour)
	}
	if chart.YunMethod != "minute_difference_3days_per_year" || chart.CalendarLibraryVersion != BaziCalendarVersion {
		t.Fatalf("yun/version metadata missing: %+v", chart)
	}
}

func TestBaziRejectsUnknownRuleSet(t *testing.T) {
	_, err := (BaziEngine{}).Compute(baziInputWithRule(2000, 1, 1, 12, 0, "bazi-unknown"))
	if err == nil {
		t.Fatal("unknown bazi rule set was accepted")
	}
}

package fortune

import (
	"fmt"
	"math"
	"time"
)

const (
	BaziRuleSetStandardV1 = "bazi-standard-v1"
	BaziRuleSetZiChuV1    = "bazi-zi-chu-v1"
	BaziCalendarVersion   = "lunar-go-v1.4.6"
)

type baziRules struct {
	Version          string
	DayBoundary      string
	EightCharSect    int
	YunSect          int
	SolarTimeFormula string
}

func resolveBaziRules(requested string) (baziRules, error) {
	switch requested {
	case "", BaziRuleSetStandardV1:
		return baziRules{
			Version:          BaziRuleSetStandardV1,
			DayBoundary:      "midnight",
			EightCharSect:    2,
			YunSect:          2,
			SolarTimeFormula: "NOAA fractional-year equation-of-time approximation",
		}, nil
	case BaziRuleSetZiChuV1:
		return baziRules{
			Version:          BaziRuleSetZiChuV1,
			DayBoundary:      "zi_chu_23:00",
			EightCharSect:    1,
			YunSect:          2,
			SolarTimeFormula: "NOAA fractional-year equation-of-time approximation",
		}, nil
	default:
		return baziRules{}, fmt.Errorf("unsupported bazi rule set %q", requested)
	}
}

// equationOfTimeMinutes implements NOAA's published fractional-year
// approximation (https://gml.noaa.gov/grad/solcalc/solareqns.PDF). It converts
// local mean solar time to local apparent solar time; longitude/time-zone
// correction is applied separately.
func equationOfTimeMinutes(local time.Time) float64 {
	days := 365.0
	if local.Year()%400 == 0 || (local.Year()%4 == 0 && local.Year()%100 != 0) {
		days = 366
	}
	gamma := 2 * math.Pi / days * (float64(local.YearDay()-1) + (float64(local.Hour())-12)/24 + float64(local.Minute())/1440)
	return 229.18 * (0.000075 + 0.001868*math.Cos(gamma) - 0.032077*math.Sin(gamma) -
		0.014615*math.Cos(2*gamma) - 0.040849*math.Sin(2*gamma))
}

func baziTimeZone(birth *BirthContext) (*time.Location, string, error) {
	zone := "Asia/Shanghai"
	if birth != nil && birth.TimeZone != "" {
		zone = birth.TimeZone
	}
	location, err := time.LoadLocation(zone)
	if err != nil {
		return nil, "", fmt.Errorf("load birth time zone %q: %w", zone, err)
	}
	return location, zone, nil
}

func apparentSolarCorrection(birth *BirthContext, longitude float64, year, month, day, hour, minute int) (int, float64, int, string, error) {
	location, zone, err := baziTimeZone(birth)
	if err != nil {
		return 0, 0, 0, "", err
	}
	civil := time.Date(year, time.Month(month), day, hour, minute, 0, 0, location)
	_, zoneOffsetSeconds := civil.Zone()
	longitudeMinutes := int(math.Round(4*longitude - float64(zoneOffsetSeconds)/60))
	equationMinutes := equationOfTimeMinutes(civil)
	totalMinutes := int(math.Round(float64(longitudeMinutes) + equationMinutes))
	return longitudeMinutes, equationMinutes, totalMinutes, zone, nil
}

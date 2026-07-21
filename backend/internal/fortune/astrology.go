package fortune

import (
	"fmt"
	"math"
)

// AstrologyEngine implements Western natal chart calculation.
// Uses simplified astronomical algorithms for planetary positions.
// Data sources: Astronomical algorithms (public domain) + Traditional astrological interpretations
type AstrologyEngine struct{}

// AstrologyInput is the request for natal chart calculation.
type AstrologyInput struct {
	Year      int     `json:"year"`
	Month     int     `json:"month"`
	Day       int     `json:"day"`
	Hour      int     `json:"hour"`
	Minute    int     `json:"minute"`
	Longitude float64 `json:"longitude"` // Birth place longitude
	Latitude  float64 `json:"latitude"`  // Birth place latitude
	Timezone  float64 `json:"timezone"`  // Timezone offset (e.g., 8 for UTC+8)
	Lang      string  `json:"lang"`
}

// AstrologyResult contains the natal chart data.
type AstrologyResult struct {
	AccuracyLabel string       `json:"accuracyLabel"`
	SunSign       string       `json:"sunSign"`             // Sun sign
	MoonSign      string       `json:"moonSign"`            // Moon sign
	Ascendant     string       `json:"ascendant,omitempty"` // unavailable until a verified ephemeris implementation
	Planets       []PlanetInfo `json:"planets"`             // Planetary positions
	Houses        []HouseInfo  `json:"houses,omitempty"`    // unavailable in the simplified engine
	Aspects       []AspectInfo `json:"aspects"`             // Major aspects
	ChartSummary  string       `json:"chartSummary"`        // Brief summary
}

// PlanetInfo holds a planet's position.
type PlanetInfo struct {
	Name       string  `json:"name"`                 // Planet name
	Sign       string  `json:"sign"`                 // Zodiac sign
	Degree     float64 `json:"degree"`               // Degree in sign (0-30)
	House      int     `json:"house,omitempty"`      // unavailable in the simplified engine
	Retrograde bool    `json:"retrograde,omitempty"` // unavailable in the simplified engine
}

// HouseInfo holds house cusp information.
type HouseInfo struct {
	Number int     `json:"number"` // 1-12
	Sign   string  `json:"sign"`   // Sign on cusp
	Degree float64 `json:"degree"` // Degree on cusp
}

// AspectInfo holds aspect information.
type AspectInfo struct {
	Planet1 string  `json:"planet1"`
	Planet2 string  `json:"planet2"`
	Aspect  string  `json:"aspect"` // conjunction, opposition, trine, square, sextile
	Orb     float64 `json:"orb"`    // Orb in degrees
	Exact   bool    `json:"exact"`  // Is exact (within 1 degree)
}

func init() {
	Register(AstrologyEngine{})
}

func (e AstrologyEngine) Name() string {
	return KindAstrology
}

func (e AstrologyEngine) Compute(in Input) (*Result, error) {
	// Calculate Julian Day
	jd := toJulianDay(in.Year, in.Month, in.Day, in.Hour, in.Minute, 0, 8.0) // Default UTC+8

	// Calculate planetary positions
	planets := calculatePlanets(jd)

	// Calculate aspects
	aspects := calculateAspects(planets)

	// Sun sign (already in planets)
	sunSign := ""
	moonSign := ""
	for _, p := range planets {
		if p.Name == "太阳" {
			sunSign = p.Sign
		}
		if p.Name == "月亮" {
			moonSign = p.Sign
		}
	}

	result := &AstrologyResult{
		AccuracyLabel: "娱乐性简化版",
		SunSign:       sunSign,
		MoonSign:      moonSign,
		Planets:       planets,
		Houses:        []HouseInfo{},
		Aspects:       aspects,
		ChartSummary:  fmt.Sprintf("娱乐性简化结果：太阳%s，月亮%s；当前算法不提供上升、宫位或逆行结论", sunSign, moonSign),
	}

	return &Result{Kind: KindAstrology, Data: result}, nil
}

// Zodiac signs
var zodiacSigns = []string{
	"白羊座", "金牛座", "双子座", "巨蟹座", "狮子座", "处女座",
	"天秤座", "天蝎座", "射手座", "摩羯座", "水瓶座", "双鱼座",
}

// Planet names in Chinese
var planetNames = []string{
	"太阳", "月亮", "水星", "金星", "火星", "木星", "土星", "天王星", "海王星", "冥王星",
}

// toJulianDay converts date/time to Julian Day number.
func toJulianDay(year, month, day, hour, minute, second int, tz float64) float64 {
	// Adjust for timezone
	h := float64(hour) + float64(minute)/60.0 + float64(second)/3600.0 - tz

	// Julian Day calculation (simplified)
	y := year
	m := month
	d := float64(day) + h/24.0

	if m <= 2 {
		y--
		m += 12
	}

	a := y / 100
	b := 2 - a + a/4

	jd := math.Floor(365.25*float64(y+4716)) + math.Floor(30.6001*float64(m+1)) + d + float64(b) - 1524.5
	return jd
}

// calculatePlanets returns approximate planetary positions.
// Uses simplified orbital elements for demonstration.
func calculatePlanets(jd float64) []PlanetInfo {
	// Days from J2000.0
	d := jd - 2451545.0

	planets := make([]PlanetInfo, 0, 10)

	// Simplified mean longitude calculations (degrees)
	// These are approximations for demonstration

	// Sun
	sunLon := (280.460 + 0.9856474*d) / 360.0
	sunLon = sunLon - math.Floor(sunLon)
	sunDeg := sunLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:   "太阳",
		Sign:   degreeToSign(sunDeg),
		Degree: math.Mod(sunDeg, 30),
		House:  0,
	})

	// Moon
	moonLon := (218.316 + 13.176396*d) / 360.0
	moonLon = moonLon - math.Floor(moonLon)
	moonDeg := moonLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:   "月亮",
		Sign:   degreeToSign(moonDeg),
		Degree: math.Mod(moonDeg, 30),
		House:  0,
	})

	// Mercury
	mercLon := (252.251 + 4.092382*d) / 360.0
	mercLon = mercLon - math.Floor(mercLon)
	mercDeg := mercLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:   "水星",
		Sign:   degreeToSign(mercDeg),
		Degree: math.Mod(mercDeg, 30),
		House:  0,
	})

	// Venus
	venusLon := (181.980 + 1.602647*d) / 360.0
	venusLon = venusLon - math.Floor(venusLon)
	venusDeg := venusLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:   "金星",
		Sign:   degreeToSign(venusDeg),
		Degree: math.Mod(venusDeg, 30),
		House:  0,
	})

	// Mars
	marsLon := (355.453 + 0.524033*d) / 360.0
	marsLon = marsLon - math.Floor(marsLon)
	marsDeg := marsLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:   "火星",
		Sign:   degreeToSign(marsDeg),
		Degree: math.Mod(marsDeg, 30),
		House:  0,
	})

	// Jupiter
	jupLon := (34.351 + 0.083129*d) / 360.0
	jupLon = jupLon - math.Floor(jupLon)
	jupDeg := jupLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:   "木星",
		Sign:   degreeToSign(jupDeg),
		Degree: math.Mod(jupDeg, 30),
	})

	// Saturn
	satLon := (50.077 + 0.033272*d) / 360.0
	satLon = satLon - math.Floor(satLon)
	satDeg := satLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:   "土星",
		Sign:   degreeToSign(satDeg),
		Degree: math.Mod(satDeg, 30),
	})

	// Uranus
	uraLon := (314.055 + 0.011732*d) / 360.0
	uraLon = uraLon - math.Floor(uraLon)
	uraDeg := uraLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:   "天王星",
		Sign:   degreeToSign(uraDeg),
		Degree: math.Mod(uraDeg, 30),
	})

	// Neptune
	nepLon := (304.349 + 0.006012*d) / 360.0
	nepLon = nepLon - math.Floor(nepLon)
	nepDeg := nepLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:   "海王星",
		Sign:   degreeToSign(nepDeg),
		Degree: math.Mod(nepDeg, 30),
	})

	// Pluto
	pluLon := (238.929 + 0.004005*d) / 360.0
	pluLon = pluLon - math.Floor(pluLon)
	pluDeg := pluLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:   "冥王星",
		Sign:   degreeToSign(pluDeg),
		Degree: math.Mod(pluDeg, 30),
	})

	return planets
}

// degreeToSign converts ecliptic longitude to zodiac sign.
func degreeToSign(deg float64) string {
	deg = math.Mod(deg, 360)
	if deg < 0 {
		deg += 360
	}
	signIndex := int(deg / 30)
	return zodiacSigns[signIndex]
}

// calculateAspects calculates major aspects between planets.
func calculateAspects(planets []PlanetInfo) []AspectInfo {
	aspects := make([]AspectInfo, 0)

	// Define aspect angles
	aspectAngles := map[string]float64{
		"合相":  0,
		"六分相": 60,
		"四分相": 90,
		"三分相": 120,
		"对分相": 180,
	}

	// Orb tolerance
	maxOrb := 8.0

	// Check each pair of planets
	for i := 0; i < len(planets); i++ {
		for j := i + 1; j < len(planets); j++ {
			deg1 := signToDegree(planets[i].Sign) + planets[i].Degree
			deg2 := signToDegree(planets[j].Sign) + planets[j].Degree

			diff := math.Abs(deg1 - deg2)
			if diff > 180 {
				diff = 360 - diff
			}

			for aspectName, angle := range aspectAngles {
				orb := math.Abs(diff - angle)
				if orb <= maxOrb {
					aspects = append(aspects, AspectInfo{
						Planet1: planets[i].Name,
						Planet2: planets[j].Name,
						Aspect:  aspectName,
						Orb:     orb,
						Exact:   orb <= 1.0,
					})
					break
				}
			}
		}
	}

	return aspects
}

// signToDegree converts a zodiac sign to its starting degree.
func signToDegree(sign string) float64 {
	for i, s := range zodiacSigns {
		if s == sign {
			return float64(i * 30)
		}
	}
	return 0
}

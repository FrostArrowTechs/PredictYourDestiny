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
	Year     int     `json:"year"`
	Month    int     `json:"month"`
	Day      int     `json:"day"`
	Hour     int     `json:"hour"`
	Minute   int     `json:"minute"`
	Longitude float64 `json:"longitude"` // Birth place longitude
	Latitude  float64 `json:"latitude"`  // Birth place latitude
	Timezone  float64 `json:"timezone"`  // Timezone offset (e.g., 8 for UTC+8)
	Lang      string  `json:"lang"`
}

// AstrologyResult contains the natal chart data.
type AstrologyResult struct {
	SunSign      string         `json:"sunSign"`      // Sun sign
	MoonSign     string         `json:"moonSign"`     // Moon sign
	Ascendant    string         `json:"ascendant"`    // Rising sign
	Planets      []PlanetInfo   `json:"planets"`      // Planetary positions
	Houses       []HouseInfo    `json:"houses"`       // 12 houses
	Aspects      []AspectInfo   `json:"aspects"`      // Major aspects
	ChartSummary string         `json:"chartSummary"` // Brief summary
}

// PlanetInfo holds a planet's position.
type PlanetInfo struct {
	Name     string  `json:"name"`     // Planet name
	Sign     string  `json:"sign"`     // Zodiac sign
	Degree   float64 `json:"degree"`   // Degree in sign (0-30)
	House    int     `json:"house"`    // House number (1-12)
	Retrograde bool  `json:"retrograde"` // Is retrograde
}

// HouseInfo holds house cusp information.
type HouseInfo struct {
	Number  int     `json:"number"`  // 1-12
	Sign    string  `json:"sign"`    // Sign on cusp
	Degree  float64 `json:"degree"`  // Degree on cusp
}

// AspectInfo holds aspect information.
type AspectInfo struct {
	Planet1   string  `json:"planet1"`
	Planet2   string  `json:"planet2"`
	Aspect    string  `json:"aspect"`    // conjunction, opposition, trine, square, sextile
	Orb       float64 `json:"orb"`       // Orb in degrees
	Exact     bool    `json:"exact"`     // Is exact (within 1 degree)
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

	// Calculate ascendant (simplified - use longitude only)
	ascendant := calculateAscendant(jd, in.Longitude, 39.9) // Default latitude Beijing

	// Calculate houses (Placidus system approximation)
	houses := calculateHouses(ascendant)

	// Assign planets to houses
	assignPlanetsToHouses(planets, houses)

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
		SunSign:   sunSign,
		MoonSign:  moonSign,
		Ascendant: ascendant,
		Planets:   planets,
		Houses:    houses,
		Aspects:   aspects,
		ChartSummary: fmt.Sprintf("太阳%s，月亮%s，上升%s", sunSign, moonSign, ascendant),
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
		Name:     "水星",
		Sign:     degreeToSign(mercDeg),
		Degree:   math.Mod(mercDeg, 30),
		House:    0,
		Retrograde: isRetrograde("mercury", d),
	})

	// Venus
	venusLon := (181.980 + 1.602647*d) / 360.0
	venusLon = venusLon - math.Floor(venusLon)
	venusDeg := venusLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:     "金星",
		Sign:     degreeToSign(venusDeg),
		Degree:   math.Mod(venusDeg, 30),
		House:    0,
		Retrograde: isRetrograde("venus", d),
	})

	// Mars
	marsLon := (355.453 + 0.524033*d) / 360.0
	marsLon = marsLon - math.Floor(marsLon)
	marsDeg := marsLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:     "火星",
		Sign:     degreeToSign(marsDeg),
		Degree:   math.Mod(marsDeg, 30),
		House:    0,
		Retrograde: isRetrograde("mars", d),
	})

	// Jupiter
	jupLon := (34.351 + 0.083129*d) / 360.0
	jupLon = jupLon - math.Floor(jupLon)
	jupDeg := jupLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:     "木星",
		Sign:     degreeToSign(jupDeg),
		Degree:   math.Mod(jupDeg, 30),
		House:    0,
		Retrograde: isRetrograde("jupiter", d),
	})

	// Saturn
	satLon := (50.077 + 0.033272*d) / 360.0
	satLon = satLon - math.Floor(satLon)
	satDeg := satLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:     "土星",
		Sign:     degreeToSign(satDeg),
		Degree:   math.Mod(satDeg, 30),
		House:    0,
		Retrograde: isRetrograde("saturn", d),
	})

	// Uranus
	uraLon := (314.055 + 0.011732*d) / 360.0
	uraLon = uraLon - math.Floor(uraLon)
	uraDeg := uraLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:     "天王星",
		Sign:     degreeToSign(uraDeg),
		Degree:   math.Mod(uraDeg, 30),
		House:    0,
		Retrograde: isRetrograde("uranus", d),
	})

	// Neptune
	nepLon := (304.349 + 0.006012*d) / 360.0
	nepLon = nepLon - math.Floor(nepLon)
	nepDeg := nepLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:     "海王星",
		Sign:     degreeToSign(nepDeg),
		Degree:   math.Mod(nepDeg, 30),
		House:    0,
		Retrograde: isRetrograde("neptune", d),
	})

	// Pluto
	pluLon := (238.929 + 0.004005*d) / 360.0
	pluLon = pluLon - math.Floor(pluLon)
	pluDeg := pluLon * 360.0
	planets = append(planets, PlanetInfo{
		Name:     "冥王星",
		Sign:     degreeToSign(pluDeg),
		Degree:   math.Mod(pluDeg, 30),
		House:    0,
		Retrograde: isRetrograde("pluto", d),
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

// calculateAscendant returns the rising sign (simplified).
func calculateAscendant(jd float64, lon, lat float64) string {
	// Simplified: use sun's position + 6 hours offset for approximate ascendant
	d := jd - 2451545.0
	sunLon := (280.460 + 0.9856474*d) / 360.0
	sunLon = sunLon - math.Floor(sunLon)
	sunDeg := sunLon * 360.0

	// Approximate ascendant: sun position - 90 + latitude adjustment
	ascDeg := sunDeg - 90 + lat*0.5
	ascDeg = math.Mod(ascDeg, 360)
	if ascDeg < 0 {
		ascDeg += 360
	}

	return degreeToSign(ascDeg)
}

// calculateHouses returns 12 house cusps (Placidus approximation).
func calculateHouses(ascendant string) []HouseInfo {
	// Find ascendant degree
	ascIndex := 0
	for i, s := range zodiacSigns {
		if s == ascendant {
			ascIndex = i
			break
		}
	}

	houses := make([]HouseInfo, 12)
	for i := 0; i < 12; i++ {
		signIdx := (ascIndex + i) % 12
		houses[i] = HouseInfo{
			Number: i + 1,
			Sign:   zodiacSigns[signIdx],
			Degree: 0, // Simplified: cusp at 0 degrees
		}
	}
	return houses
}

// assignPlanetsToHouses assigns each planet to its house.
func assignPlanetsToHouses(planets []PlanetInfo, houses []HouseInfo) {
	// Create a map of sign to house number
	signToHouse := make(map[string]int)
	for _, h := range houses {
		signToHouse[h.Sign] = h.Number
	}

	// Assign planets to houses based on their sign
	for i := range planets {
		planets[i].House = signToHouse[planets[i].Sign]
	}
}

// isRetrograde determines if a planet is retrograde (simplified).
func isRetrograde(planet string, d float64) bool {
	// Simplified retrograde periods
	// Mercury: about 20% of the time
	// Venus: about 7% of the time
	// Mars: about 9% of the time
	// Jupiter through Pluto: about 30-40% of the time

	// Use a simple periodic function for demonstration
	switch planet {
	case "mercury":
		return math.Sin(d/115.0) > 0.3
	case "venus":
		return math.Sin(d/584.0) > 0.85
	case "mars":
		return math.Sin(d/780.0) > 0.8
	case "jupiter":
		return math.Sin(d/399.0) > 0.3
	case "saturn":
		return math.Sin(d/378.0) > 0.3
	case "uranus":
		return math.Sin(d/370.0) > 0.3
	case "neptune":
		return math.Sin(d/367.0) > 0.3
	case "pluto":
		return math.Sin(d/366.0) > 0.3
	default:
		return false
	}
}

// calculateAspects calculates major aspects between planets.
func calculateAspects(planets []PlanetInfo) []AspectInfo {
	aspects := make([]AspectInfo, 0)

	// Define aspect angles
	aspectAngles := map[string]float64{
		"合相":   0,
		"六分相":  60,
		"四分相":  90,
		"三分相":  120,
		"对分相":  180,
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
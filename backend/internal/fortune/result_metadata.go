package fortune

import "fmt"

type Fact struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type ResultVariant struct {
	Fingerprint string `json:"fingerprint"`
	Label       string `json:"label,omitempty"`
	Data        any    `json:"data,omitempty"`
}

type Evidence struct {
	Source      string `json:"source"`
	Description string `json:"description"`
}

type ResultMetadata struct {
	AlgorithmVersion       string          `json:"algorithmVersion"`
	RuleSetVersion         string          `json:"ruleSetVersion"`
	CalendarLibraryVersion string          `json:"calendarLibraryVersion,omitempty"`
	InputPrecision         TimePrecision   `json:"inputPrecision"`
	Assumptions            []string        `json:"assumptions"`
	Warnings               []string        `json:"warnings"`
	UnsupportedRules       []string        `json:"unsupportedRules"`
	StableFacts            []Fact          `json:"stableFacts"`
	VariableFacts          []Fact          `json:"variableFacts"`
	Variants               []ResultVariant `json:"variants"`
	Evidence               []Evidence      `json:"evidence"`
}

var birthAlgorithmVersions = map[string][2]string{
	KindBazi:      {"bazi-lunar-go-v2", BaziRuleSetStandardV1},
	KindZiwei:     {"ziwei-traditional-v1", "ziwei-legacy-rules-v1"},
	KindAstrology: {"astrology-simplified-v1", "astrology-simplified-rules-v1"},
	KindWeighbone: {"weighbone-table-v1", "weighbone-traditional-rules-v1"},
}

// AttachBirthMetadata adds an explicit, stable response contract without
// changing the engine-specific Data payload. Empty fact/variant arrays are
// deliberate: later uncertainty work can populate them without shape changes.
func AttachBirthMetadata(result *Result, input Input) {
	if result == nil || input.Birth == nil {
		return
	}
	birth, err := input.Birth.Normalized()
	if err != nil {
		return
	}
	versions := birthAlgorithmVersions[result.Kind]
	meta := ResultMetadata{
		AlgorithmVersion: versions[0],
		RuleSetVersion:   versions[1],
		InputPrecision:   birth.TimePrecision,
		Assumptions:      []string{},
		Warnings:         []string{},
		UnsupportedRules: []string{},
		StableFacts:      []Fact{},
		VariableFacts:    []Fact{},
		Variants:         []ResultVariant{},
		Evidence: []Evidence{{
			Source:      "user_input",
			Description: fmt.Sprintf("birth date %04d-%02d-%02d", birth.Year, birth.Month, birth.Day),
		}},
	}
	if birth.TimePrecision == PrecisionHour {
		meta.Assumptions = append(meta.Assumptions, "minute was not provided; the full stated hour was evaluated as a time range")
	}
	if birth.TimePrecision == PrecisionPeriod || birth.TimePrecision == PrecisionShichen || birth.TimePrecision == PrecisionUnknown {
		meta.Assumptions = append(meta.Assumptions, "candidate times were evaluated at declared domain boundaries; no probability distribution was assumed")
	}
	if birth.TimeZone == "" {
		meta.Assumptions = append(meta.Assumptions, "IANA time zone was not provided; legacy engine local-time rules were used")
	}
	if birth.Longitude == nil || birth.Latitude == nil {
		meta.Warnings = append(meta.Warnings, "birth location is incomplete; location-dependent conclusions may be unavailable or simplified")
	}
	if result.Kind == KindBazi && birth.Longitude == nil {
		meta.Assumptions = append(meta.Assumptions, "legal local time was used without true-solar-time correction")
		meta.UnsupportedRules = append(meta.UnsupportedRules, "true solar time")
	}
	if result.Kind != KindBazi && birth.RuleSet != "" && birth.RuleSet != meta.RuleSetVersion {
		meta.Warnings = append(meta.Warnings, "requested rule set is not available; the declared legacy rule set was used")
	}
	if result.Kind == KindBazi {
		meta.CalendarLibraryVersion = BaziCalendarVersion
		if birth.RuleSet != "" {
			meta.RuleSetVersion = birth.RuleSet
		}
		if chart, ok := result.Data.(*BaziChart); ok {
			meta.RuleSetVersion = chart.RuleSetVersion
			meta.Evidence = append(meta.Evidence,
				Evidence{Source: BaziCalendarVersion, Description: "solar/lunar conversion, exact solar-term pillars and luck-cycle sequencing"},
				Evidence{Source: chart.RuleSetVersion, Description: "declared day-boundary and luck-cycle rules"},
			)
			if chart.TrueSolar {
				meta.Evidence = append(meta.Evidence, Evidence{Source: "NOAA General Solar Position Calculations", Description: "fractional-year equation-of-time approximation used for apparent solar time"})
			}
		}
	}
	if result.Kind == KindAstrology {
		meta.Warnings = append(meta.Warnings, "entertainment-only simplified astrology; planetary positions and aspects are approximate")
		meta.UnsupportedRules = append(meta.UnsupportedRules, "ascendant", "MC", "houses", "retrograde")
	}
	if result.Kind == KindZiwei {
		meta.Warnings = append(meta.Warnings, "five-element bureau and some star placement rules still use documented legacy approximations")
		meta.UnsupportedRules = append(meta.UnsupportedRules, "verified leap-month rule pack", "verified five-element bureau rule pack")
	}
	result.ResultMetadata = &meta
}

package fortune

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type candidateClock struct {
	Hour   int
	Minute int
	Label  string
}

// ComputeWithBirthUncertainty computes an exact result for minute precision,
// or a set of deduplicated candidates for an imprecise birth time. The main
// Data field is deliberately nil for imprecise inputs: callers must consume
// stableFacts/variableFacts/variants instead of treating one candidate as true.
func ComputeWithBirthUncertainty(engine FortuneEngine, input Input) (*Result, error) {
	if input.Birth == nil {
		return nil, fmt.Errorf("birth context is required")
	}
	birth, err := input.Birth.Normalized()
	if err != nil {
		return nil, err
	}
	input.Birth = &birth
	clocks, err := uncertaintyCandidateClocks(engine.Name(), birth)
	if err != nil {
		return nil, err
	}
	if birth.TimePrecision == PrecisionMinute {
		input.Hour, input.Minute = clocks[0].Hour, clocks[0].Minute
		result, err := engine.Compute(input)
		if err != nil {
			return nil, err
		}
		AttachBirthMetadata(result, input)
		return result, nil
	}

	result := &Result{Kind: engine.Name()}
	AttachBirthMetadata(result, input)
	if result.ResultMetadata == nil {
		return nil, fmt.Errorf("uncertainty metadata unavailable")
	}

	type computedVariant struct {
		variant ResultVariant
		facts   map[string]any
		labels  []string
	}
	byFingerprint := make(map[string]*computedVariant)
	order := make([]string, 0, len(clocks))
	for _, clock := range clocks {
		candidateInput := input
		hour, minute := clock.Hour, clock.Minute
		candidateBirth := birth
		candidateBirth.Hour = &hour
		candidateBirth.Minute = &minute
		candidateBirth.TimePrecision = PrecisionMinute
		candidateBirth.TimeRange = nil
		candidateInput.Birth = &candidateBirth
		candidateInput.Hour, candidateInput.Minute = hour, minute
		candidate, err := engine.Compute(candidateInput)
		if err != nil {
			return nil, fmt.Errorf("compute candidate %s: %w", clock.Label, err)
		}
		facts, err := flattenJSON(candidate.Data)
		if err != nil {
			return nil, err
		}
		facts = projectUncertaintyFacts(engine.Name(), facts)
		encoded, err := json.Marshal(facts)
		if err != nil {
			return nil, fmt.Errorf("encode candidate %s: %w", clock.Label, err)
		}
		sum := sha256.Sum256(encoded)
		fingerprint := hex.EncodeToString(sum[:])
		if existing := byFingerprint[fingerprint]; existing != nil {
			existing.labels = append(existing.labels, clock.Label)
			continue
		}
		byFingerprint[fingerprint] = &computedVariant{
			variant: ResultVariant{Fingerprint: fingerprint, Data: facts},
			facts:   facts,
			labels:  []string{clock.Label},
		}
		order = append(order, fingerprint)
	}

	variants := make([]*computedVariant, 0, len(order))
	variantFacts := make([]map[string]any, 0, len(order))
	for _, fingerprint := range order {
		variant := byFingerprint[fingerprint]
		variant.variant.Label = strings.Join(variant.labels, ", ")
		variants = append(variants, variant)
		variantFacts = append(variantFacts, variant.facts)
		result.ResultMetadata.Variants = append(result.ResultMetadata.Variants, variant.variant)
	}
	result.ResultMetadata.StableFacts, result.ResultMetadata.VariableFacts = compareVariantFactMaps(variantFacts)
	result.ResultMetadata.Warnings = append(result.ResultMetadata.Warnings,
		"birth time is imprecise; no single candidate chart is treated as definitive")
	result.ResultMetadata.Evidence = append(result.ResultMetadata.Evidence, Evidence{
		Source:      "uncertainty_engine_v1",
		Description: fmt.Sprintf("evaluated %d boundary candidates and merged them into %d distinct results", len(clocks), len(variants)),
	})
	return result, nil
}

func projectUncertaintyFacts(kind string, facts map[string]any) map[string]any {
	projected := make(map[string]any)
	for key, value := range facts {
		allowed := true
		switch kind {
		case KindBazi:
			allowed = hasAnyPrefix(key,
				"ruleSetVersion", "dayBoundary", "calendarLibraryVersion", "previousJie", "nextJie", "yunMethod",
				"pillars", "taiYuan", "taiXi", "mingGong", "shenGong", "shenSha",
				"startYear", "startMonth", "startDay", "startHour", "forward",
				"wuXingStats", "interpretation")
			allowed = allowed && key != "interpretation.yongYin.confidence"
		case KindZiwei:
			allowed = key != "solarDate"
		case KindAstrology:
			allowed = key != "chartSummary"
		}
		if allowed {
			projected[key] = value
		}
	}
	return projected
}

func hasAnyPrefix(value string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if value == prefix || strings.HasPrefix(value, prefix+".") || strings.HasPrefix(value, prefix+"[") {
			return true
		}
	}
	return false
}

func uncertaintyCandidateClocks(kind string, birth BirthContext) ([]candidateClock, error) {
	if birth.TimePrecision == PrecisionMinute {
		return []candidateClock{{Hour: *birth.Hour, Minute: *birth.Minute, Label: fmt.Sprintf("%02d:%02d", *birth.Hour, *birth.Minute)}}, nil
	}
	start, end := 0, 23*60+59
	switch birth.TimePrecision {
	case PrecisionHour:
		start, end = *birth.Hour*60, *birth.Hour*60+59
	case PrecisionPeriod, PrecisionShichen:
		if birth.TimeRange != nil {
			start = birth.TimeRange.Start.Hour*60 + birth.TimeRange.Start.Minute
			end = birth.TimeRange.End.Hour*60 + birth.TimeRange.End.Minute
		} else {
			start, end = *birth.Hour*60, *birth.Hour*60+59
		}
	case PrecisionUnknown:
	default:
		return nil, fmt.Errorf("unsupported time precision %q", birth.TimePrecision)
	}
	if start > end {
		return nil, fmt.Errorf("time ranges crossing midnight are not yet supported; split the range at midnight")
	}

	minutes := map[int]bool{start: true, end: true}
	step := 60
	firstBoundary := ((start / step) + 1) * step
	if kind == KindBazi && birth.Longitude != nil {
		// The Bazi engine applies longitude correction before determining
		// shichen. Convert corrected-time boundaries back into input local time.
		_, _, correction, _, err := apparentSolarCorrection(&birth, *birth.Longitude, birth.Year, birth.Month, birth.Day, 12, 0)
		if err != nil {
			return nil, err
		}
		boundaries := []int{0}
		for corrected := 60; corrected < 24*60; corrected += 120 {
			boundaries = append(boundaries, corrected)
		}
		for _, corrected := range boundaries {
			local := (corrected - correction) % (24 * 60)
			if local < 0 {
				local += 24 * 60
			}
			if local > start && local <= end {
				minutes[local] = true
			}
		}
		firstBoundary = end + 1
	} else if kind != KindAstrology {
		// Traditional birth algorithms change primarily at two-hour shichen
		// boundaries (01:00, 03:00, ...), plus 23:00 and midnight.
		step = 120
		firstBoundary = 60
		for firstBoundary <= start {
			firstBoundary += step
		}
	}
	for minute := firstBoundary; minute <= end; minute += step {
		minutes[minute] = true
	}
	ordered := make([]int, 0, len(minutes))
	for minute := range minutes {
		ordered = append(ordered, minute)
	}
	sort.Ints(ordered)
	clocks := make([]candidateClock, 0, len(ordered))
	for _, minute := range ordered {
		clocks = append(clocks, candidateClock{Hour: minute / 60, Minute: minute % 60, Label: fmt.Sprintf("%02d:%02d", minute/60, minute%60)})
	}
	return clocks, nil
}

func flattenJSON(value any) (map[string]any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode facts: %w", err)
	}
	var normalized any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return nil, fmt.Errorf("normalize facts: %w", err)
	}
	facts := make(map[string]any)
	var walk func(string, any)
	walk = func(path string, current any) {
		switch typed := current.(type) {
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				next := key
				if path != "" {
					next = path + "." + key
				}
				walk(next, typed[key])
			}
		case []any:
			for index, item := range typed {
				walk(fmt.Sprintf("%s[%d]", path, index), item)
			}
		default:
			facts[path] = typed
		}
	}
	walk("", normalized)
	return facts, nil
}

// compareVariantFactMaps is kept unexported so the response surface remains
// the ResultMetadata contract rather than an implementation-specific map.
func compareVariantFactMaps(variants []map[string]any) ([]Fact, []Fact) {
	if len(variants) == 0 {
		return []Fact{}, []Fact{}
	}
	keys := make(map[string]bool)
	for _, facts := range variants {
		for key := range facts {
			keys[key] = true
		}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	stable, variable := []Fact{}, []Fact{}
	for _, key := range ordered {
		first, present := variants[0][key]
		isStable := present
		values := make([]any, 0, len(variants))
		for _, facts := range variants {
			value, ok := facts[key]
			values = append(values, value)
			if !ok || !reflect.DeepEqual(first, value) {
				isStable = false
			}
		}
		if isStable {
			stable = append(stable, Fact{Key: key, Value: first})
		} else {
			variable = append(variable, Fact{Key: key, Value: values})
		}
	}
	return stable, variable
}

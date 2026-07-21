package fortune

import (
	"errors"
	"fmt"
	"time"
)

type TimePrecision string

const (
	PrecisionMinute  TimePrecision = "minute"
	PrecisionHour    TimePrecision = "hour"
	PrecisionPeriod  TimePrecision = "period"
	PrecisionShichen TimePrecision = "shichen"
	PrecisionUnknown TimePrecision = "unknown"
)

var (
	ErrBirthTimeUnknown   = errors.New("birth time is unknown")
	ErrBirthTimeImprecise = errors.New("birth time precision is not yet supported by this calculation")
)

type LocalTime struct {
	Hour   int `json:"hour"`
	Minute int `json:"minute"`
}

type LocalTimeRange struct {
	Start LocalTime `json:"start"`
	End   LocalTime `json:"end"`
}

// BirthContext is the canonical input contract for every birth-based engine.
// Pointer clock/location fields distinguish a real zero (midnight/Greenwich)
// from missing information.
type BirthContext struct {
	Year          int             `json:"year" binding:"required,min=1"`
	Month         int             `json:"month" binding:"required,min=1,max=12"`
	Day           int             `json:"day" binding:"required,min=1,max=31"`
	Hour          *int            `json:"hour" binding:"omitempty,min=0,max=23"`
	Minute        *int            `json:"minute" binding:"omitempty,min=0,max=59"`
	TimePrecision TimePrecision   `json:"timePrecision,omitempty"`
	TimeRange     *LocalTimeRange `json:"timeRange,omitempty"`
	Longitude     *float64        `json:"longitude,omitempty" binding:"omitempty,min=-180,max=180"`
	Latitude      *float64        `json:"latitude,omitempty" binding:"omitempty,min=-90,max=90"`
	TimeZone      string          `json:"timeZone,omitempty"` // IANA name, e.g. Asia/Shanghai
	TimeSource    string          `json:"timeSource,omitempty"`
	RuleSet       string          `json:"ruleSet,omitempty"`
}

func (b BirthContext) Normalized() (BirthContext, error) {
	date := time.Date(b.Year, time.Month(b.Month), b.Day, 0, 0, 0, 0, time.UTC)
	if date.Year() != b.Year || int(date.Month()) != b.Month || date.Day() != b.Day {
		return b, fmt.Errorf("invalid birth date")
	}
	if b.TimePrecision == "" {
		switch {
		case b.Hour == nil:
			b.TimePrecision = PrecisionUnknown
		case b.Minute == nil:
			b.TimePrecision = PrecisionHour
		default:
			b.TimePrecision = PrecisionMinute
		}
	}
	if b.Hour != nil && (*b.Hour < 0 || *b.Hour > 23) {
		return b, fmt.Errorf("hour out of range: %d", *b.Hour)
	}
	if b.Minute != nil && (*b.Minute < 0 || *b.Minute > 59) {
		return b, fmt.Errorf("minute out of range: %d", *b.Minute)
	}
	switch b.TimePrecision {
	case PrecisionMinute:
		if b.Hour == nil || b.Minute == nil {
			return b, fmt.Errorf("minute precision requires hour and minute")
		}
	case PrecisionHour:
		if b.Hour == nil {
			return b, fmt.Errorf("hour precision requires hour")
		}
	case PrecisionPeriod, PrecisionShichen:
		if b.TimeRange == nil && b.Hour == nil {
			return b, fmt.Errorf("%s precision requires a time range or representative hour", b.TimePrecision)
		}
		if b.TimeRange != nil {
			for label, clock := range map[string]LocalTime{"start": b.TimeRange.Start, "end": b.TimeRange.End} {
				if clock.Hour < 0 || clock.Hour > 23 || clock.Minute < 0 || clock.Minute > 59 {
					return b, fmt.Errorf("time range %s is out of range", label)
				}
			}
		}
	case PrecisionUnknown:
		if b.Hour != nil || b.Minute != nil || b.TimeRange != nil {
			return b, fmt.Errorf("unknown time precision cannot include a clock time")
		}
	default:
		return b, fmt.Errorf("unsupported time precision %q", b.TimePrecision)
	}
	if b.TimeZone != "" {
		if _, err := time.LoadLocation(b.TimeZone); err != nil {
			return b, fmt.Errorf("invalid IANA time zone %q", b.TimeZone)
		}
	}
	return b, nil
}

// RequiredClock returns an exact clock usable by single-chart and AI paths.
// Every imprecise input must go through ComputeWithBirthUncertainty instead of
// selecting a representative time.
func (b BirthContext) RequiredClock() (BirthContext, int, int, error) {
	b, err := b.Normalized()
	if err != nil {
		return b, 0, 0, err
	}
	switch b.TimePrecision {
	case PrecisionUnknown:
		return b, 0, 0, ErrBirthTimeUnknown
	case PrecisionHour, PrecisionPeriod, PrecisionShichen:
		return b, 0, 0, ErrBirthTimeImprecise
	}
	return b, *b.Hour, *b.Minute, nil
}

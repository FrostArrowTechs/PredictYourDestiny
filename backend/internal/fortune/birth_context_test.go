package fortune

import (
	"errors"
	"testing"
)

func intPtr(value int) *int { return &value }

func TestBirthContextDistinguishesMidnightFromUnknown(t *testing.T) {
	midnight := BirthContext{Year: 2000, Month: 1, Day: 1, Hour: intPtr(0), Minute: intPtr(0)}
	normalized, hour, minute, err := midnight.RequiredClock()
	if err != nil || hour != 0 || minute != 0 || normalized.TimePrecision != PrecisionMinute {
		t.Fatalf("midnight = %+v, %d:%d, %v", normalized, hour, minute, err)
	}

	unknown := BirthContext{Year: 2000, Month: 1, Day: 1}
	normalized, _, _, err = unknown.RequiredClock()
	if !errors.Is(err, ErrBirthTimeUnknown) || normalized.TimePrecision != PrecisionUnknown {
		t.Fatalf("unknown = %+v, %v", normalized, err)
	}
}

func TestBirthContextValidatesPrecisionAndTimezone(t *testing.T) {
	if _, err := (BirthContext{Year: 2000, Month: 2, Day: 30}).Normalized(); err == nil {
		t.Fatal("invalid date was accepted")
	}
	if _, err := (BirthContext{Year: 2000, Month: 1, Day: 1, TimePrecision: PrecisionMinute}).Normalized(); err == nil {
		t.Fatal("minute precision without time was accepted")
	}
	if _, err := (BirthContext{Year: 2000, Month: 1, Day: 1, TimeZone: "Not/AZone"}).Normalized(); err == nil {
		t.Fatal("invalid time zone was accepted")
	}
}

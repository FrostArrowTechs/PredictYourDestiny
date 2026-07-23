package model

import "time"

// AIDailyCostUsage serializes a user's cost commitments for one UTC day.
// ReservedMicros covers in-flight requests; SpentMicros covers settled ones.
type AIDailyCostUsage struct {
	ID             uint   `gorm:"primaryKey"`
	UserID         uint   `gorm:"uniqueIndex:idx_ai_daily_cost_user_date;not null"`
	Date           string `gorm:"uniqueIndex:idx_ai_daily_cost_user_date;size:10;not null"`
	ReservedMicros int64  `gorm:"not null;default:0"`
	SpentMicros    int64  `gorm:"not null;default:0"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AICostReservation links the preflight hold to the metered request. RequestID
// makes settlement idempotent even when accounting is retried.
type AICostReservation struct {
	ID             uint   `gorm:"primaryKey"`
	UserID         uint   `gorm:"index;not null"`
	Date           string `gorm:"size:10;index;not null"`
	RequestID      string `gorm:"uniqueIndex;size:128;not null"`
	ReservedMicros int64  `gorm:"not null"`
	ActualMicros   int64  `gorm:"not null;default:0"`
	Status         string `gorm:"size:24;not null;default:'reserved'"` // reserved|settled
	SettlementMode string `gorm:"size:32"`                             // usage|conservative
	SettledAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

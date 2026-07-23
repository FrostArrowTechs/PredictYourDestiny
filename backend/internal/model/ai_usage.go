package model

import "time"

// AIModelPriceVersion freezes a provider/model price at a point in time so
// historical costs never change when the admin updates current pricing.
type AIModelPriceVersion struct {
	ID                            uint   `gorm:"primaryKey"`
	ProviderID                    uint   `gorm:"uniqueIndex:idx_provider_model_price_version;not null"`
	Model                         string `gorm:"uniqueIndex:idx_provider_model_price_version;size:128;not null"`
	Version                       string `gorm:"uniqueIndex:idx_provider_model_price_version;size:64;not null"`
	InputCostMicrosPerMillion     int64  `gorm:"not null;default:0"`
	OutputCostMicrosPerMillion    int64  `gorm:"not null;default:0"`
	ReasoningCostMicrosPerMillion int64  `gorm:"not null;default:0"`
	// RequestReserveMicros is the conservative amount held before generation.
	// It is configured per price version because providers/models have very
	// different maximum response costs.
	RequestReserveMicros int64     `gorm:"not null;default:0"`
	EffectiveFrom        time.Time `gorm:"index;not null"`
	CreatedAt            time.Time
}

// AIUsageLedger is append-only request accounting. Cost is stored in
// millionths of the billing currency (normally USD) to avoid floating point.
type AIUsageLedger struct {
	ID                  uint      `gorm:"primaryKey"`
	UserID              uint      `gorm:"index;not null"`
	ProviderID          *uint     `gorm:"index"`
	ProviderName        string    `gorm:"size:64"`
	Model               string    `gorm:"size:128;index;not null"`
	Feature             string    `gorm:"size:64;index"`
	RequestID           string    `gorm:"uniqueIndex;size:128;not null"`
	IdempotencyKey      string    `gorm:"size:128"`
	Stream              bool      `gorm:"not null;default:false"`
	Status              string    `gorm:"size:24;index;not null"` // succeeded|failed|cancelled
	PromptTokens        int       `gorm:"not null;default:0"`
	CompletionTokens    int       `gorm:"not null;default:0"`
	ReasoningTokens     int       `gorm:"not null;default:0"`
	TotalTokens         int       `gorm:"not null;default:0"`
	PriceVersionID      *uint     `gorm:"index"`
	PricingStatus       string    `gorm:"size:24;not null"` // priced|unpriced
	EstimatedCostMicros int64     `gorm:"not null;default:0"`
	ActualCostMicros    *int64    `gorm:"default:null"`
	ErrorCode           string    `gorm:"size:64"`
	StartedAt           time.Time `gorm:"index;not null"`
	CompletedAt         time.Time `gorm:"not null"`
	CreatedAt           time.Time
}

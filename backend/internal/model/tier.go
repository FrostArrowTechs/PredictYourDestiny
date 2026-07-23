package model

import "time"

// MembershipTier defines a user's access level and quota limits.
// Built-in tiers: free (default), basic, premium.
type MembershipTier struct {
	ID         uint   `gorm:"primaryKey"`
	Code       string `gorm:"uniqueIndex;size:32;not null"` // "free", "basic", "premium"
	Name       string `gorm:"size:64;not null"`             // "免费用户", "基础会员", "高级会员"
	DailyQuota int    `gorm:"default:5"`                    // daily AI interpretations (-1 = unlimited)
	// DailyCostBudgetMicros is an opt-in estimated provider-cost ceiling.
	// -1 keeps existing tiers unlimited until an admin explicitly configures it.
	DailyCostBudgetMicros int64  `gorm:"not null;default:-1"`
	Features              string `gorm:"type:text"`             // JSON array: ["deep_interpret","history_save"]
	IsEnabled             bool   `gorm:"default:true;not null"` // disabled tiers grant no entitlement
	PriceMonth            int    `gorm:"default:0"`             // monthly price in cents (0 = free)
	SortOrder             int    `gorm:"default:0"`             // for UI ordering
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// TableName overrides the default table name.
func (MembershipTier) TableName() string {
	return "membership_tiers"
}

// Tier constants for reference in code.
const (
	TierCodeFree    = "free"
	TierCodeBasic   = "basic"
	TierCodePremium = "premium"
)

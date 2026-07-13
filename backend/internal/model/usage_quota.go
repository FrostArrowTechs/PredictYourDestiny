package model

import "time"

// UsageQuota tracks a user's daily AI-token spend.
//
// Enforced for logged-in users on the "free" tier: anonymous visitors
// can't reach the AI layer at all, and paid users bypass this counter.
// The (UserID, Date) pair is unique so there is exactly one row per
// user per UTC day.
type UsageQuota struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_date;not null"`
	Date      string    `gorm:"uniqueIndex:idx_user_date;size:10;not null"` // YYYY-MM-DD (UTC)
	Count     int       `gorm:"default:0"`                                  // interpretations used today
	CreatedAt time.Time
	UpdatedAt time.Time
}

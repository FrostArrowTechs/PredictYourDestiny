package model

import "time"

// UserMembership tracks a user's current membership status.
// A user can have active membership (ExpiresAt in future) or
// expired membership (ExpiresAt in past or null for free tier).
type UserMembership struct {
	ID        uint       `gorm:"primaryKey"`
	UserID    uint       `gorm:"uniqueIndex;not null"`
	TierID    uint       `gorm:"not null"`
	ExpiresAt *time.Time `gorm:"index"` // null = never expires (for free tier)
	CreatedAt time.Time
	UpdatedAt time.Time

	// Relations
	User *User          `gorm:"foreignKey:UserID"`
	Tier *MembershipTier `gorm:"foreignKey:TierID"`
}

// TableName overrides the default table name.
func (UserMembership) TableName() string {
	return "user_memberships"
}
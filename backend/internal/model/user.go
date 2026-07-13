package model

import "time"

// User is an authenticated account.
//
// Anonymous visitors can use every "compute" feature (bazi chart,
// almanac, tarot draw, …) for free, since those are pure calculations
// and cost zero AI tokens. A User record is only required to:
//
//   - persist charts / divination history,
//   - consume the daily free AI-interpretation quota,
//   - unlock paid deep interpretations.
type User struct {
	ID        uint      `gorm:"primaryKey"`
	Email     string    `gorm:"uniqueIndex;size:255;not null"`
	Password  string    `gorm:"size:255;not null"` // bcrypt hash, never the plaintext
	DisplayName string  `gorm:"size:64"`
	Role      string    `gorm:"size:32;default:'user'"` // user | admin
	CreatedAt time.Time
	UpdatedAt time.Time
}

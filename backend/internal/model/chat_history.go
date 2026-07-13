package model

import "time"

// ChatHistory stores one AI interpretation turn.
//
// It links a user, the FortuneRecord it interprets (optional — a
// reading may be free-form), the model used, and the raw
// request/response payloads. Persisting the full exchange makes it
// possible to resume a conversation and to audit token spend.
type ChatHistory struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index;not null"`
	RecordID  *uint     `gorm:"index"`                     // nullable: free-form chats have no chart
	Model     string    `gorm:"size:128"`                   // e.g. deepseek-chat, gpt-4o
	Role      string    `gorm:"size:16"`                    // user | assistant
	Content   string    `gorm:"type:text"`
	TokensIn  int       `gorm:"default:0"`
	TokensOut int       `gorm:"default:0"`
	CreatedAt time.Time
}

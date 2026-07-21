package model

import "time"

// AIRequestReservation makes an interpretation idempotency key single-use for
// one user and UTC day. It is committed in the same transaction as quota
// reservation, so a duplicate request cannot call AI without being charged.
type AIRequestReservation struct {
	ID             uint   `gorm:"primaryKey"`
	UserID         uint   `gorm:"uniqueIndex:idx_ai_request_key;not null"`
	Date           string `gorm:"uniqueIndex:idx_ai_request_key;size:10;not null"`
	IdempotencyKey string `gorm:"uniqueIndex:idx_ai_request_key;size:128;not null"`
	CreatedAt      time.Time
}

func (AIRequestReservation) TableName() string {
	return "ai_request_reservations"
}

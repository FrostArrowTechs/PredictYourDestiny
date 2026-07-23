package model

import "time"

// FortuneRecord is a saved chart / divination result.
//
// One row per "reading". The Kind field discriminates the engine
// ("bazi", "tarot", "dream", "compatibility", …). The structured
// engine output lives in ResultJSON; any AI interpretation(s) are
// linked back via ChatHistory.RecordID.
//
// Stored only for logged-in users — anonymous results live solely in
// the browser session and are never persisted.
type FortuneRecord struct {
	ID              uint   `gorm:"primaryKey"`
	UserID          uint   `gorm:"index;not null"`
	Kind            string `gorm:"size:32;index;not null"` // bazi|tarot|dream|...
	Title           string `gorm:"size:255"`               // human label, e.g. "张三 · 八字命盘"
	InputJSON       string `gorm:"type:text"`              // raw input that produced the chart
	ResultJSON      string `gorm:"type:text"`              // structured engine output
	ServerGenerated bool   `gorm:"not null;default:false"` // true only when saved by a compute handler
	Note            string `gorm:"type:text"`              // optional user note
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

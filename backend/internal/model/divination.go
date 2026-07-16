package model

// DivinationPoem is a row in the 求签 (divination stick) reference table.
//
// The seed data contains traditional 观音灵签 poems (public domain).
// Each stick has a number (1-100), a title (often referencing a
// historical/mythological story), the poem itself, and an interpretation.
type DivinationPoem struct {
	ID         uint   `gorm:"primaryKey"`
	Number     int    `gorm:"uniqueIndex;not null"` // 1-100
	Title      string `gorm:"size:100;not null"`    // e.g. "第一签 钟离成仙"
	Tier       string `gorm:"size:20;not null"`     // 上签/上上签/中签/中上签/中下签/下签/下下签
	Poem       string `gorm:"type:text;not null"`   // 四句诗
	Interpret  string `gorm:"type:text;not null"`   // 解曰/诗意
	Category   string `gorm:"size:50"`              // 吉凶分类
}
package model

// DreamEntry is a row in the 周公解梦 reference table.
//
// The seed/dream.json file is the source of truth for the initial data
// and is imported once at deploy time. The engine does keyword matching
// against this table to surface traditional interpretations.
type DreamEntry struct {
	ID       uint   `gorm:"primaryKey"`
	Keyword  string `gorm:"index;size:100;not null"` // 梦见蛇
	Category string `gorm:"size:50;not null"`        // 动物/自然/人物/物品/行为/身体
	Meaning  string `gorm:"type:text;not null"`      // Traditional interpretation
}
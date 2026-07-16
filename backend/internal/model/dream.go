package model

// DreamEntry represents a traditional dream interpretation from 周公解梦.
// Data source: Traditional Chinese dream interpretation (public domain knowledge).
type DreamEntry struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	Keyword  string `json:"keyword" gorm:"index;size:100;not null"` // 关键词，如 "蛇"
	Category string `json:"category" gorm:"size:50"`                // 分类：动物/自然/人物/事件
	Meaning  string `json:"meaning" gorm:"type:text;not null"`      // 传统释义
}

// TableName specifies the table name for GORM.
func (DreamEntry) TableName() string {
	return "dream_entries"
}

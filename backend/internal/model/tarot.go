package model

// TarotCard is a row in the tarot reference table.
//
// The seed data covers all 78 cards of the Rider-Waite-Smith tarot
// (public domain, published 1909): 22 Major Arcana + 56 Minor Arcana
// (14 cards each in Wands, Cups, Swords, Pentacles).
type TarotCard struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	Number    int    `json:"number" gorm:"uniqueIndex;not null"` // 0-77 (Major 0-21, Minor 22-77)
	Name      string `json:"name" gorm:"size:50;not null"`       // e.g. "愚者", "权杖一"
	NameLatin string `json:"nameLatin" gorm:"size:50"`           // e.g. "The Fool", "Ace of Wands"
	Arcana    string `json:"arcana" gorm:"size:10;not null"`     // 大阿卡纳 / 小阿卡纳
	Suit      string `json:"suit" gorm:"size:20"`                // 权杖/圣杯/宝剑/星币 (empty for Major)
	// Upright meaning (正位)
	UprightMeaning   string `json:"uprightMeaning" gorm:"type:text;not null"`
	UprightKeywords  string `json:"uprightKeywords" gorm:"type:text"` // comma-separated
	// Reversed meaning (逆位)
	ReversedMeaning  string `json:"reversedMeaning" gorm:"type:text;not null"`
	ReversedKeywords string `json:"reversedKeywords" gorm:"type:text"`
	// Element associated with the card (for Minor Arcana suits)
	Element string `json:"element" gorm:"size:20"` // 火/水/风/土
}

// TableName specifies the table name for GORM.
func (TarotCard) TableName() string { return "tarot_cards" }

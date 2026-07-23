package model

// CharacterStroke maps a Chinese character to its Kangxi dictionary stroke count.
// This is used for Five格 (Five Structures) calculations in name analysis.
//
// Data source: 康熙字典笔画（公共领域）— traditional stroke counts used in
// Chinese name numerology, which may differ from simplified character strokes.
type CharacterStroke struct {
	ID             uint   `gorm:"primaryKey"`
	Char           string `gorm:"uniqueIndex:idx_character_stroke_version;size:1;not null"` // Single Chinese character
	Strokes        int    `gorm:"not null"`                                                 // Kangxi stroke count
	WuXing         string `gorm:"size:10"`                                                  // Five element (金木水火土), optional
	Script         string `gorm:"size:20;not null;default:mixed"`
	StrokeStandard string `gorm:"size:30;not null;default:kangxi;index;uniqueIndex:idx_character_stroke_version"`
	DataVersion    string `gorm:"size:50;not null;default:kangxi-seed-2026-07-v1;index;uniqueIndex:idx_character_stroke_version"`
	ReviewStatus   string `gorm:"size:30;not null;default:seed_unreviewed"`
}

// NameAnalysis records a name analysis result for a user (authenticated save).
// Anonymous users can still use the compute endpoint without persistence.
type NameAnalysis struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    *uint  `gorm:"index"`            // Optional, for logged-in users
	FullName  string `gorm:"size:50;not null"` // The analyzed name
	TianGe    int    `gorm:"not null"`         // 天格
	RenGe     int    `gorm:"not null"`         // 人格
	DiGe      int    `gorm:"not null"`         // 地格
	WaiGe     int    `gorm:"not null"`         // 外格
	ZongGe    int    `gorm:"not null"`         // 总格
	SanCai    string `gorm:"size:10"`          // 三才配置 (e.g., "木火土")
	Score     int    `gorm:"not null"`         // Overall score 1-100
	CreatedAt int64  `gorm:"autoCreateTime"`   // Unix timestamp
}

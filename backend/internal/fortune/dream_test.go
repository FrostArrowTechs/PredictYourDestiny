package fortune

import (
	"testing"

	"predictdestiny/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestDreamEngineEmptyQuestion verifies the engine handles empty input.
func TestDreamEngineEmptyQuestion(t *testing.T) {
	// Setup in-memory SQLite
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.DreamEntry{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	eng := DreamEngine{DB: db}
	res, err := eng.Compute(Input{Question: ""})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	if res.Kind != KindDream {
		t.Errorf("kind = %q, want %q", res.Kind, KindDream)
	}
	chart, ok := res.Data.(*DreamChart)
	if !ok {
		t.Fatalf("Data is not *DreamChart: %T", res.Data)
	}
	if chart.TotalMatches != 0 {
		t.Errorf("TotalMatches = %d, want 0", chart.TotalMatches)
	}
}

// TestDreamEngineMatching verifies keyword matching works.
func TestDreamEngineMatching(t *testing.T) {
	// Setup in-memory SQLite with seed data
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.DreamEntry{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Seed test entries
	entries := []model.DreamEntry{
		{Keyword: "梦见蛇", Category: "动物", Meaning: "蛇的释义"},
		{Keyword: "梦见龙", Category: "动物", Meaning: "龙的释义"},
		{Keyword: "梦见水", Category: "自然", Meaning: "水的释义"},
	}
	if err := db.Create(&entries).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	eng := DreamEngine{DB: db}

	// Test matching multiple keywords
	res, err := eng.Compute(Input{Question: "我梦见一条大蛇在水中游"})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	chart, ok := res.Data.(*DreamChart)
	if !ok {
		t.Fatalf("Data is not *DreamChart: %T", res.Data)
	}

	// Should match "梦见蛇" and "梦见水"
	if chart.TotalMatches < 2 {
		t.Errorf("TotalMatches = %d, want >= 2", chart.TotalMatches)
	}

	// Verify the matches contain expected keywords
	foundSnake := false
	foundWater := false
	for _, m := range chart.Matches {
		if m.Keyword == "梦见蛇" {
			foundSnake = true
		}
		if m.Keyword == "梦见水" {
			foundWater = true
		}
	}
	if !foundSnake {
		t.Error("expected to match 梦见蛇")
	}
	if !foundWater {
		t.Error("expected to match 梦见水")
	}
}

// TestDreamEngineNoMatch verifies behavior when no keywords match.
func TestDreamEngineNoMatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.DreamEntry{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	entries := []model.DreamEntry{
		{Keyword: "梦见蛇", Category: "动物", Meaning: "蛇的释义"},
	}
	if err := db.Create(&entries).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	eng := DreamEngine{DB: db}
	res, err := eng.Compute(Input{Question: "我梦见一个没有的关键词xyz"})
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	chart := res.Data.(*DreamChart)
	if chart.TotalMatches != 0 {
		t.Errorf("TotalMatches = %d, want 0", chart.TotalMatches)
	}
}

// TestKeywordMatches verifies the keyword matching helper.
func TestKeywordMatches(t *testing.T) {
	cases := []struct {
		question string
		keyword  string
		want     bool
	}{
		{"梦见蛇", "梦见蛇", true},
		{"我梦见一条大蛇", "梦见蛇", true},
		{"梦见大蛇追我", "梦见蛇", true},
		{"梦见龙", "梦见蛇", false},
		{"", "梦见蛇", false},
	}
	for _, c := range cases {
		got := keywordMatches(c.question, c.keyword)
		if got != c.want {
			t.Errorf("keywordMatches(%q, %q) = %v, want %v", c.question, c.keyword, got, c.want)
		}
	}
}
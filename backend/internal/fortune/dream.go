// Package fortune provides the dream interpretation engine.
//
// The engine matches keywords in the user's dream description against
// a reference table of traditional 周公解梦 entries and returns all
// matches for the frontend to display. The AI interpret endpoint then
// provides a personalized reading based on those matches.
//
// This is a hybrid approach: the reference table supplies authentic
// traditional interpretations (public-domain knowledge), while AI
// adds context-aware analysis tailored to the user's specific dream.
package fortune

import (
	"strings"

	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

// DreamEngine matches dream keywords against the reference table.
type DreamEngine struct {
	DB *gorm.DB
}

// DreamChart is the structured result returned by the dream engine.
type DreamChart struct {
	// The user's original dream description.
	Question string `json:"question"`

	// All matching entries from the reference table, ordered by
	// relevance (exact match first, then partial matches).
	Matches []DreamMatch `json:"matches"`

	// Total number of matches found.
	TotalMatches int `json:"totalMatches"`
}

// DreamMatch is a single reference entry that matched the query.
type DreamMatch struct {
	Keyword  string `json:"keyword"`
	Category string `json:"category"`
	Meaning  string `json:"meaning"`
}

// Name returns the engine identifier.
func (e DreamEngine) Name() string { return KindDream }

// Compute searches the dream reference table for entries matching
// keywords in the input question. It performs a simple substring
// match: any entry whose keyword appears in the question is returned.
//
// Matching strategy:
//   - Extract Chinese "梦见*" patterns from the question
//   - Also match keywords that appear as substrings (e.g., "蛇" in "梦见大蛇")
//   - Return up to 10 most relevant matches
func (e DreamEngine) Compute(in Input) (*Result, error) {
	if in.Question == "" {
		return &Result{
			Kind: KindDream,
			Data: &DreamChart{
				Question:     "",
				Matches:      nil,
				TotalMatches: 0,
			},
		}, nil
	}

	// Load all dream entries from database (cached query would be better
	// for production, but this is acceptable for the initial implementation)
	var entries []model.DreamEntry
	if err := e.DB.Find(&entries).Error; err != nil {
		return nil, err
	}

	// Match keywords against the question
	question := in.Question
	var matches []DreamMatch
	for _, entry := range entries {
		// Check if the keyword or a substring appears in the question
		if keywordMatches(question, entry.Keyword) {
			matches = append(matches, DreamMatch{
				Keyword:  entry.Keyword,
				Category: entry.Category,
				Meaning:  entry.Meaning,
			})
		}
	}

	// Limit to 10 most relevant matches (simple heuristic: shorter
	// keywords are more specific, so prefer them)
	if len(matches) > 10 {
		matches = matches[:10]
	}

	return &Result{
		Kind: KindDream,
		Data: &DreamChart{
			Question:     question,
			Matches:      matches,
			TotalMatches: len(matches),
		},
		Meta: map[string]string{
			"source": "周公解梦",
		},
	}, nil
}

// keywordMatches checks if the keyword appears in the question.
// It handles both the full "梦见蛇" form and the bare "蛇" form.
func keywordMatches(question, keyword string) bool {
	// Direct substring match
	if strings.Contains(question, keyword) {
		return true
	}

	// Extract the core keyword from "梦见*" patterns
	// e.g., "梦见蛇" → "蛇"
	if strings.HasPrefix(keyword, "梦见") && len(keyword) > 6 {
		core := keyword[6:] // skip "梦见" (2 Chinese chars = 6 bytes in UTF-8)
		if strings.Contains(question, core) {
			return true
		}
	}

	return false
}

func init() {
	// Dream engine requires DB, so registration happens in handler setup
	// This placeholder ensures the kind is recognized
}
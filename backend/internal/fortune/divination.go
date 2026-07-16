// Package fortune provides the 抽签/求签 (divination stick) engine.
//
// The engine draws a random stick from the divination_poems table and
// returns the poem + interpretation. The draw is seedable so a given
// question + timestamp produces a consistent result within a short
// window (prevents rapid re-draws for the same question).
//
// The seed data contains traditional 观音灵签 poems (public domain).
package fortune

import (
	"crypto/sha256"
	"encoding/binary"
	"time"

	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

// DivinationEngine draws divination sticks.
type DivinationEngine struct {
	DB *gorm.DB
}

// DivinationChart is the structured result.
type DivinationChart struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	Tier      string `json:"tier"`
	Poem      string `json:"poem"`
	Interpret string `json:"interpret"`
	Category  string `json:"category"`
	// The user's question (echoed back)
	Question string `json:"question"`
}

// Name returns the engine identifier.
func (e DivinationEngine) Name() string { return KindDivination }

// Compute draws a divination stick. If Input.Question is provided,
// the draw is seeded by question + current 10-minute window so rapid
// re-draws of the same question yield the same stick (feels fated).
// If no question, it's a pure random draw.
func (e DivinationEngine) Compute(in Input) (*Result, error) {
	var poems []model.DivinationPoem
	if err := e.DB.Find(&poems).Error; err != nil {
		return nil, err
	}
	if len(poems) == 0 {
		return nil, errEmptyDivination
	}

	// Determine the index
	var idx int
	if in.Question != "" {
		// Seed by question + 10-minute window (600 seconds)
		window := uint64(time.Now().Unix() / 600)
		seed := hashSeed(in.Question, window)
		idx = int(seed % uint64(len(poems)))
	} else {
		// Pure random using time
		window := uint64(time.Now().UnixNano())
		seed := hashSeed("divination", window)
		idx = int(seed % uint64(len(poems)))
	}

	p := poems[idx]
	chart := &DivinationChart{
		Number:    p.Number,
		Title:     p.Title,
		Tier:      p.Tier,
		Poem:      p.Poem,
		Interpret: p.Interpret,
		Category:  p.Category,
		Question:  in.Question,
	}

	return &Result{
		Kind: KindDivination,
		Data: chart,
		Meta: map[string]string{
			"source": "观音灵签",
		},
	}, nil
}

// hashSeed creates a deterministic uint64 from a string + numeric seed.
func hashSeed(s string, n uint64) uint64 {
	h := sha256.New()
	h.Write([]byte(s))
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], n)
	h.Write(buf[:])
	var result uint64
	for i, b := range h.Sum(nil)[:8] {
		result |= uint64(b) << (uint(i) * 8)
	}
	return result
}

// errEmptyDivination is returned when no poems are seeded.
var errEmptyDivination = &fortuneError{"divination: no poems seeded"}

type fortuneError struct{ msg string }

func (e *fortuneError) Error() string { return e.msg }

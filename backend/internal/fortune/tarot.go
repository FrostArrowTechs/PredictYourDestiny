// Package fortune provides the 塔罗 (tarot) engine.
//
// The engine draws cards from the tarot_cards reference table (78-card
// Rider-Waite-Smith deck, public domain) and supports several spreads.
// The draw is seeded by the user's question + a short time window so
// re-drawing the same question within minutes yields the same cards
// (feels fated), while a fresh question gets a fresh draw.
//
// Spreads supported:
//   - single: 1 card (quick yes/no or daily card)
//   - three:  3 cards (past / present / future)
//   - celtic: 10 cards (Celtic Cross)
package fortune

import (
	"errors"
	"math/rand"
	"time"

	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

// TarotEngine draws tarot cards for a reading.
type TarotEngine struct {
	DB *gorm.DB
}

// TarotCardDraw is one drawn card with orientation and spread position.
type TarotCardDraw struct {
	Number           int    `json:"number"`
	Name             string `json:"name"`
	NameLatin        string `json:"nameLatin"`
	Arcana           string `json:"arcana"`
	Suit             string `json:"suit"`
	Reversed         bool   `json:"reversed"`         // true = 逆位
	PositionIndex    int    `json:"positionIndex"`    // 0-based slot in the spread
	PositionLabel    string `json:"positionLabel"`    // e.g. "过去", "现在", "未来"
	Meaning          string `json:"meaning"`          // the applicable meaning (upright or reversed)
	Keywords         string `json:"keywords"`         // the applicable keywords
	Element          string `json:"element"`
}

// TarotSpread describes the positions for a spread type.
type TarotSpread struct {
	ID      string   `json:"id"`      // single / three / celtic
	Name    string   `json:"name"`    // 单张牌 / 三牌阵 / 凯尔特十字
	Count   int      `json:"count"`   // number of cards
	Labels  []string `json:"labels"`  // position labels
}

// TarotChart is the structured result of a tarot reading.
type TarotChart struct {
	Spread   TarotSpread      `json:"spread"`
	Cards    []TarotCardDraw  `json:"cards"`
	Question string           `json:"question"`
}

// Name returns the engine identifier.
func (e TarotEngine) Name() string { return KindTarot }

// spreadDefs maps a spread id to its definition.
var spreadDefs = map[string]TarotSpread{
	"single": {
		ID: "single", Name: "单张牌", Count: 1,
		Labels: []string{"提示"},
	},
	"three": {
		ID: "three", Name: "三牌阵", Count: 3,
		Labels: []string{"过去", "现在", "未来"},
	},
	"celtic": {
		ID: "celtic", Name: "凯尔特十字", Count: 10,
		Labels: []string{
			"现状", "挑战", "过去", "基础", "近期过去",
			"近期未来", "自我", "环境", "希望与恐惧", "结局",
		},
	},
}

// defaultSpreadID is used when the client doesn't specify one.
const defaultSpreadID = "three"

// SpreadIDs returns the available spread ids (for diagnostics / UI).
func SpreadIDs() []string {
	return []string{"single", "three", "celtic"}
}

// SpreadDef returns the spread definition for an id (ok=false if unknown).
func SpreadDef(id string) (TarotSpread, bool) {
	s, ok := spreadDefs[id]
	return s, ok
}

// Compute draws the requested spread for the user's question.
func (e TarotEngine) Compute(in Input) (*Result, error) {
	if e.DB == nil {
		return nil, errors.New("tarot: database not configured")
	}

	// Load all 78 cards
	var cards []model.TarotCard
	if err := e.DB.Find(&cards).Error; err != nil {
		return nil, err
	}
	if len(cards) == 0 {
		return nil, errors.New("tarot: no cards seeded")
	}

	// Resolve the spread. The handler encodes the spread id as a leading
	// "[single]"/"[three]"/"[celtic]" tag on Input.Question; if absent
	// the default spread is used.
	question := in.Question
	spreadID, question := parseSpreadPrefix(question, defaultSpreadID)
	spread := spreadDefs[spreadID]

	// Seed the RNG: question + 10-minute window → same draw for the same
	// question within a short span (feels fated). Empty question → time-only.
	seed := tarotSeed(question)
	rnd := rand.New(rand.NewSource(seed))

	// Fisher-Yates shuffle a copy of card indices.
	n := len(cards)
	order := rnd.Perm(n)

	draws := make([]TarotCardDraw, 0, spread.Count)
	for i := 0; i < spread.Count; i++ {
		card := cards[order[i]]
		reversed := rnd.Intn(2) == 0 // 50% chance reversed
		meaning := card.UprightMeaning
		keywords := card.UprightKeywords
		if reversed {
			meaning = card.ReversedMeaning
			keywords = card.ReversedKeywords
		}
		label := spread.Labels[i]
		draws = append(draws, TarotCardDraw{
			Number:        card.Number,
			Name:          card.Name,
			NameLatin:     card.NameLatin,
			Arcana:        card.Arcana,
			Suit:          card.Suit,
			Reversed:      reversed,
			PositionIndex: i,
			PositionLabel: label,
			Meaning:       meaning,
			Keywords:      keywords,
			Element:       card.Element,
		})
	}

	chart := &TarotChart{
		Spread:   spread,
		Cards:    draws,
		Question: question,
	}

	return &Result{
		Kind: KindTarot,
		Data: chart,
		Meta: map[string]string{
			"source": "Rider-Waite 塔罗",
			"spread": spread.Name,
		},
	}, nil
}

// parseSpreadPrefix extracts a leading "[spreadId]" tag from the question.
// Returns the resolved spread id (or fallback) and the stripped question.
func parseSpreadPrefix(question, fallback string) (spreadID, question2 string) {
	q := question
	if len(q) > 0 && q[0] == '[' {
		if end := indexOfByte(q, ']'); end > 0 {
			tag := q[1:end]
			if _, ok := spreadDefs[tag]; ok {
				return tag, trimLeftSpaces(q[end+1:])
			}
		}
	}
	return fallback, q
}

func indexOfByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func trimLeftSpaces(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	return s
}

// tarotSeed builds an int64 seed from the question + current 10-minute window.
func tarotSeed(question string) int64 {
	window := time.Now().Unix() / 600
	h := int64(0)
	for _, c := range question {
		h = h*31 + int64(c)
	}
	if h == 0 {
		h = 1
	}
	return h*1000003 + window
}

func init() {
	Register(TarotEngine{})
}

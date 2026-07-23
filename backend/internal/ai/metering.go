package ai

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"predictdestiny/internal/model"
)

type usageMetadataKey struct{}

type UsageMetadata struct {
	UserID         uint
	RequestID      string
	IdempotencyKey string
	Feature        string
}

func WithUsageMetadata(ctx context.Context, metadata UsageMetadata) context.Context {
	return context.WithValue(ctx, usageMetadataKey{}, metadata)
}

type MeteredGateway struct {
	DB    *gorm.DB
	Inner Gateway
	Now   func() time.Time
}

func NewMeteredGateway(db *gorm.DB, inner Gateway) *MeteredGateway {
	return &MeteredGateway{DB: db, Inner: inner, Now: time.Now}
}

func (g *MeteredGateway) ListModels() ModelCatalog { return g.Inner.ListModels() }

func (g *MeteredGateway) Chat(ctx context.Context, modelID string, msgs []Message, opts Options) (*Response, error) {
	started := g.now()
	provider := g.providerAtStart()
	response, err := g.Inner.Chat(ctx, modelID, msgs, opts)
	usage := Usage{}
	resolvedModel := modelID
	if response != nil {
		usage = response.Usage
		if response.Model != "" {
			resolvedModel = response.Model
		}
	}
	g.record(ctx, provider, resolvedModel, false, usage, started, err)
	return response, err
}

func (g *MeteredGateway) StreamChat(ctx context.Context, modelID string, msgs []Message, opts Options, onEvent func(StreamEvent)) error {
	started := g.now()
	provider := g.providerAtStart()
	usage := Usage{}
	err := g.Inner.StreamChat(ctx, modelID, msgs, opts, func(event StreamEvent) {
		if event.Usage != nil {
			usage = *event.Usage
		}
		onEvent(event)
	})
	g.record(ctx, provider, modelID, true, usage, started, err)
	return err
}

func (g *MeteredGateway) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}

func (g *MeteredGateway) providerAtStart() *model.AIProvider {
	if g.DB == nil {
		return nil
	}
	var provider model.AIProvider
	if err := g.DB.Where("is_default = ? AND is_enabled = ?", true, true).First(&provider).Error; err != nil {
		return nil
	}
	return &provider
}

func (g *MeteredGateway) record(ctx context.Context, provider *model.AIProvider, modelID string, stream bool, usage Usage, started time.Time, callErr error) {
	metadata, ok := ctx.Value(usageMetadataKey{}).(UsageMetadata)
	if !ok || metadata.UserID == 0 || metadata.RequestID == "" || g.DB == nil {
		return
	}
	completed := g.now()
	ledger := model.AIUsageLedger{
		UserID: metadata.UserID, Model: modelID, Feature: metadata.Feature,
		RequestID: metadata.RequestID, IdempotencyKey: metadata.IdempotencyKey,
		Stream: stream, Status: "succeeded", PricingStatus: "unpriced",
		PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens,
		ReasoningTokens: usage.ReasoningTokens, TotalTokens: usage.TotalTokens,
		StartedAt: started, CompletedAt: completed,
	}
	if callErr != nil {
		ledger.Status = "failed"
		ledger.ErrorCode = usageErrorCode(callErr)
		if errors.Is(callErr, context.Canceled) || errors.Is(callErr, context.DeadlineExceeded) {
			ledger.Status = "cancelled"
		}
	}
	if provider != nil {
		ledger.ProviderID = &provider.ID
		ledger.ProviderName = provider.Name
		var price model.AIModelPriceVersion
		if err := g.DB.Where("provider_id = ? AND model = ? AND effective_from <= ?", provider.ID, modelID, completed).
			Order("effective_from DESC, id DESC").First(&price).Error; err == nil {
			ledger.PriceVersionID = &price.ID
			ledger.PricingStatus = "priced"
			ledger.EstimatedCostMicros = tokenCostMicros(usage, price)
		}
	}
	// Ledger creation and cost-hold settlement share one transaction. Request
	// ID uniqueness makes the whole operation retry-safe. Accounting remains
	// non-blocking to the already completed user response.
	_ = g.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ledger)
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		return settleCostReservation(tx, metadata.RequestID, ledger.EstimatedCostMicros,
			usageReported(usage) && ledger.PricingStatus == "priced")
	})
}

func settleCostReservation(tx *gorm.DB, requestID string, estimatedCost int64, reported bool) error {
	var reservation model.AICostReservation
	err := tx.Where("request_id = ? AND status = ?", requestID, "reserved").First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil // the tier did not enable cost budgeting
	}
	if err != nil {
		return err
	}
	settledCost := estimatedCost
	mode := "usage"
	if !reported {
		// Missing usage can occur on provider errors and interrupted streams.
		// Keeping the conservative hold avoids silently making those requests
		// free; it never interrupts an answer already in progress.
		settledCost = reservation.ReservedMicros
		mode = "conservative"
	}
	daily := model.AIDailyCostUsage{}
	result := tx.Model(&daily).
		Where("user_id = ? AND date = ? AND reserved_micros >= ?", reservation.UserID, reservation.Date, reservation.ReservedMicros).
		Updates(map[string]any{
			"reserved_micros": gorm.Expr("reserved_micros - ?", reservation.ReservedMicros),
			"spent_micros":    gorm.Expr("spent_micros + ?", settledCost),
			"updated_at":      time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("AI cost reservation counter is inconsistent")
	}
	now := time.Now()
	return tx.Model(&model.AICostReservation{}).
		Where("id = ? AND status = ?", reservation.ID, "reserved").
		Updates(map[string]any{
			"actual_micros": settledCost, "status": "settled",
			"settlement_mode": mode, "settled_at": &now,
		}).Error
}

func usageReported(usage Usage) bool {
	return usage.PromptTokens > 0 || usage.CompletionTokens > 0 || usage.ReasoningTokens > 0 || usage.TotalTokens > 0
}

func tokenCostMicros(usage Usage, price model.AIModelPriceVersion) int64 {
	const million = int64(1_000_000)
	return (int64(usage.PromptTokens)*price.InputCostMicrosPerMillion +
		int64(usage.CompletionTokens)*price.OutputCostMicrosPerMillion +
		int64(usage.ReasoningTokens)*price.ReasoningCostMicrosPerMillion) / million
}

func usageErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrKeyInvalid):
		return "key_invalid"
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrInsufficient):
		return "insufficient_credit"
	case errors.Is(err, ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, ErrModelNotFound):
		return "model_not_found"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	default:
		return "upstream_error"
	}
}

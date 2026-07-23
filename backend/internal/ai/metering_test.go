package ai

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

type meteringGatewayStub struct {
	streamErr error
}

func (meteringGatewayStub) Chat(context.Context, string, []Message, Options) (*Response, error) {
	return &Response{Model: "priced-model", Usage: Usage{PromptTokens: 10, CompletionTokens: 5, ReasoningTokens: 2, TotalTokens: 15}}, nil
}

func (g meteringGatewayStub) StreamChat(_ context.Context, _ string, _ []Message, _ Options, onEvent func(StreamEvent)) error {
	onEvent(StreamEvent{Usage: &Usage{PromptTokens: 4, CompletionTokens: 3, TotalTokens: 7}})
	return g.streamErr
}

func (meteringGatewayStub) ListModels() ModelCatalog { return ModelCatalog{} }

func TestMeteredGatewayUsesVersionedPriceAndDeduplicatesRequest(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:metered_gateway?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AIProvider{}, &model.AIModelPriceVersion{}, &model.AIUsageLedger{}, &model.AIDailyCostUsage{}, &model.AICostReservation{}); err != nil {
		t.Fatal(err)
	}
	provider := model.AIProvider{Name: "provider-a", BaseURL: "https://example.com/v1", IsDefault: true, IsEnabled: true}
	db.Create(&provider)
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	price := model.AIModelPriceVersion{
		ProviderID: provider.ID, Model: "priced-model", Version: "2026-07",
		InputCostMicrosPerMillion: 1_000_000, OutputCostMicrosPerMillion: 2_000_000,
		ReasoningCostMicrosPerMillion: 3_000_000, EffectiveFrom: now.Add(-time.Hour),
	}
	db.Create(&price)
	db.Create(&model.AIDailyCostUsage{UserID: 7, Date: "2026-07-23", ReservedMicros: 100})
	db.Create(&model.AICostReservation{UserID: 7, Date: "2026-07-23", RequestID: "request-1", ReservedMicros: 100, Status: "reserved"})
	gateway := NewMeteredGateway(db, meteringGatewayStub{})
	gateway.Now = func() time.Time { return now }
	ctx := WithUsageMetadata(context.Background(), UsageMetadata{UserID: 7, RequestID: "request-1", Feature: "bazi"})
	if _, err := gateway.Chat(ctx, "priced-model", nil, Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.Chat(ctx, "priced-model", nil, Options{}); err != nil {
		t.Fatal(err)
	}
	var ledgers []model.AIUsageLedger
	if err := db.Find(&ledgers).Error; err != nil {
		t.Fatal(err)
	}
	if len(ledgers) != 1 {
		t.Fatalf("ledger rows = %d, want 1", len(ledgers))
	}
	ledger := ledgers[0]
	if ledger.PricingStatus != "priced" || ledger.PriceVersionID == nil || *ledger.PriceVersionID != price.ID || ledger.EstimatedCostMicros != 26 {
		t.Fatalf("priced ledger = %+v", ledger)
	}
	var daily model.AIDailyCostUsage
	db.Where("user_id = ?", 7).First(&daily)
	if daily.ReservedMicros != 0 || daily.SpentMicros != 26 {
		t.Fatalf("settled daily cost = %+v", daily)
	}
	var reservation model.AICostReservation
	db.Where("request_id = ?", "request-1").First(&reservation)
	if reservation.Status != "settled" || reservation.ActualMicros != 26 || reservation.SettlementMode != "usage" {
		t.Fatalf("settlement = %+v", reservation)
	}
}

func TestMeteredGatewayRecordsStreamingFailureWithoutBlocking(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:metered_gateway_stream?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AIProvider{}, &model.AIModelPriceVersion{}, &model.AIUsageLedger{}, &model.AIDailyCostUsage{}, &model.AICostReservation{}); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("stream failed")
	gateway := NewMeteredGateway(db, meteringGatewayStub{streamErr: sentinel})
	ctx := WithUsageMetadata(context.Background(), UsageMetadata{UserID: 8, RequestID: "request-stream", Feature: "dream"})
	err = gateway.StreamChat(ctx, "unpriced-model", nil, Options{}, func(StreamEvent) {})
	if !errors.Is(err, sentinel) {
		t.Fatalf("stream error = %v", err)
	}
	var ledger model.AIUsageLedger
	if err := db.First(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	if ledger.Status != "failed" || ledger.PricingStatus != "unpriced" || ledger.TotalTokens != 7 || ledger.Stream != true {
		t.Fatalf("stream ledger = %+v", ledger)
	}
}

func TestCostSettlementUsesReserveWhenUsageMissing(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:metered_missing_usage?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.AIDailyCostUsage{}, &model.AICostReservation{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&model.AIDailyCostUsage{UserID: 10, Date: "2026-07-23", ReservedMicros: 80})
	db.Create(&model.AICostReservation{UserID: 10, Date: "2026-07-23", RequestID: "missing", ReservedMicros: 80, Status: "reserved"})
	if err := db.Transaction(func(tx *gorm.DB) error {
		return settleCostReservation(tx, "missing", 0, false)
	}); err != nil {
		t.Fatal(err)
	}
	var daily model.AIDailyCostUsage
	db.Where("user_id = ?", 10).First(&daily)
	if daily.ReservedMicros != 0 || daily.SpentMicros != 80 {
		t.Fatalf("conservative daily settlement = %+v", daily)
	}
	var reservation model.AICostReservation
	db.Where("request_id = ?", "missing").First(&reservation)
	if reservation.SettlementMode != "conservative" || reservation.ActualMicros != 80 {
		t.Fatalf("conservative reservation = %+v", reservation)
	}
}

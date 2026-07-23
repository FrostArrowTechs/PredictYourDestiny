package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"predictdestiny/internal/auth"
	"predictdestiny/internal/model"
)

func accountTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:account_data_controls?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.MembershipTier{}, &model.UserMembership{},
		&model.FortuneRecord{}, &model.ChatHistory{}, &model.NameAnalysis{},
		&model.UsageQuota{}, &model.AIRequestReservation{},
		&model.AIUsageLedger{},
		&model.AIDailyCostUsage{},
		&model.AICostReservation{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func accountContext(method, body string, userID uint) (*gin.Context, *httptest.ResponseRecorder) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(method, "/account", strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set(string(auth.ClaimsKey), &auth.Claims{UserID: userID})
	return context, response
}

func TestAccountExportRetentionClearAndDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := accountTestDB(t)
	hash, err := auth.HashPassword("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{Email: "privacy@example.com", Password: hash, DisplayName: "Privacy", Role: "user"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	tier := model.MembershipTier{Code: "free", Name: "Free", IsEnabled: true}
	db.Create(&tier)
	db.Create(&model.UserMembership{UserID: user.ID, TierID: tier.ID})
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	old := model.FortuneRecord{UserID: user.ID, Kind: "bazi", InputJSON: `{}`, ResultJSON: `{}`, ServerGenerated: true, CreatedAt: now.AddDate(-2, 0, 0)}
	recent := model.FortuneRecord{UserID: user.ID, Kind: "ziwei", InputJSON: `{}`, ResultJSON: `{}`, ServerGenerated: true, CreatedAt: now.AddDate(0, 0, -1)}
	db.Create(&old)
	db.Create(&recent)
	db.Create(&model.ChatHistory{UserID: user.ID, RecordID: &old.ID, Role: "assistant", Content: "old", CreatedAt: now.AddDate(0, 0, -1)})
	db.Create(&model.ChatHistory{UserID: user.ID, RecordID: &recent.ID, Role: "assistant", Content: "recent", CreatedAt: now.AddDate(0, 0, -1)})
	db.Create(&model.NameAnalysis{UserID: &user.ID, FullName: "测试", CreatedAt: now.AddDate(-2, 0, 0).Unix()})
	db.Create(&model.UsageQuota{UserID: user.ID, Date: "2026-07-23", Count: 1})
	db.Create(&model.AIRequestReservation{UserID: user.ID, Date: "2026-06-01", IdempotencyKey: "old-key", CreatedAt: now.AddDate(0, 0, -40)})

	handler := &AccountHandler{DB: db}
	exportContext, exportResponse := accountContext(http.MethodGet, "", user.ID)
	handler.Export(exportContext)
	if exportResponse.Code != http.StatusOK || strings.Contains(exportResponse.Body.String(), hash) || !strings.Contains(exportResponse.Body.String(), "predictdestiny-account-export-v1") {
		t.Fatalf("export status=%d body=%s", exportResponse.Code, exportResponse.Body.String())
	}

	if err := PurgeExpiredUserData(db, now, 365, 30); err != nil {
		t.Fatal(err)
	}
	var recordCount, chatCount, reservationCount int64
	db.Model(&model.FortuneRecord{}).Where("user_id = ?", user.ID).Count(&recordCount)
	db.Model(&model.ChatHistory{}).Where("user_id = ?", user.ID).Count(&chatCount)
	db.Model(&model.AIRequestReservation{}).Where("user_id = ?", user.ID).Count(&reservationCount)
	if recordCount != 1 || chatCount != 1 || reservationCount != 0 {
		t.Fatalf("after retention records=%d chats=%d reservations=%d", recordCount, chatCount, reservationCount)
	}

	clearContext, clearResponse := accountContext(http.MethodDelete, "", user.ID)
	handler.ClearHistory(clearContext)
	if clearResponse.Code != http.StatusNoContent {
		t.Fatalf("clear status=%d body=%s", clearResponse.Code, clearResponse.Body.String())
	}
	db.Model(&model.FortuneRecord{}).Where("user_id = ?", user.ID).Count(&recordCount)
	if recordCount != 0 {
		t.Fatalf("history records remain: %d", recordCount)
	}

	wrongContext, wrongResponse := accountContext(http.MethodDelete, `{"password":"wrong"}`, user.ID)
	handler.Delete(wrongContext)
	if wrongResponse.Code != http.StatusForbidden {
		t.Fatalf("wrong password status=%d body=%s", wrongResponse.Code, wrongResponse.Body.String())
	}
	deleteContext, deleteResponse := accountContext(http.MethodDelete, `{"password":"correct-password"}`, user.ID)
	handler.Delete(deleteContext)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", deleteResponse.Code, deleteResponse.Body.String())
	}
	var userCount, membershipCount, quotaCount int64
	db.Model(&model.User{}).Where("id = ?", user.ID).Count(&userCount)
	db.Model(&model.UserMembership{}).Where("user_id = ?", user.ID).Count(&membershipCount)
	db.Model(&model.UsageQuota{}).Where("user_id = ?", user.ID).Count(&quotaCount)
	if userCount != 0 || membershipCount != 0 || quotaCount != 0 {
		t.Fatalf("account residue users=%d memberships=%d quotas=%d", userCount, membershipCount, quotaCount)
	}

	auth.InitJWT("account-deletion-test-secret-that-is-long-enough")
	oldToken, err := auth.SignToken(user.ID, user.Email, user.Role)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/protected", auth.AuthRequired(db), func(c *gin.Context) { c.Status(http.StatusOK) })
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+oldToken)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("deleted account token status=%d body=%s", response.Code, response.Body.String())
	}
}

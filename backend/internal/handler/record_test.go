package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"predictdestiny/internal/auth"
	"predictdestiny/internal/model"
)

func TestWriteComputedResultPersistsServerOutputForAuthenticatedUser(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:server_result_record?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.FortuneRecord{}); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPost, "/compute", nil)
	auth.OptionalAuth()(context)
	context.Set(string(auth.ClaimsKey), &auth.Claims{UserID: 42})
	input := map[string]any{"year": 2000, "question": "server accepted input"}
	result := map[string]any{"kind": "bazi", "data": map[string]any{"verified": true}}

	writeComputedResult(context, db, "bazi", input, result)
	if response.Code != http.StatusOK || response.Header().Get("X-Fortune-Record-ID") == "" {
		t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}

	var stored model.FortuneRecord
	if err := db.First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.UserID != 42 || stored.Kind != "bazi" || !stored.ServerGenerated {
		t.Fatalf("stored provenance = %+v", stored)
	}
	var storedResult map[string]any
	if err := json.Unmarshal([]byte(stored.ResultJSON), &storedResult); err != nil {
		t.Fatal(err)
	}
	if storedResult["kind"] != "bazi" {
		t.Fatalf("stored result = %s", stored.ResultJSON)
	}
}

func TestWriteComputedResultDoesNotPersistAnonymousCalculation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	writeComputedResult(context, nil, "bazi", map[string]int{"year": 2000}, map[string]string{"kind": "bazi"})
	if response.Code != http.StatusOK || response.Header().Get("X-Fortune-Record-ID") != "" {
		t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
}

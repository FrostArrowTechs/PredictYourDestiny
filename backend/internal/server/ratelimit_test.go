package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestFixedWindowLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	limiter := newFixedWindowLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	r := gin.New()
	r.GET("/limited", limiter.middleware(clientIPKey("test")), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/limited", nil)
		req.RemoteAddr = "192.0.2.10:1234"
		r.ServeHTTP(w, req)
		return w
	}

	if got := request().Code; got != http.StatusNoContent {
		t.Fatalf("first status = %d", got)
	}
	if got := request().Code; got != http.StatusNoContent {
		t.Fatalf("second status = %d", got)
	}
	blocked := request()
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("third status = %d, want 429", blocked.Code)
	}
	if blocked.Header().Get("Retry-After") == "" {
		t.Fatal("429 response is missing Retry-After")
	}

	now = now.Add(time.Minute)
	if got := request().Code; got != http.StatusNoContent {
		t.Fatalf("status after reset = %d", got)
	}
}

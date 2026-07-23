package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"predictdestiny/internal/fortune"
)

func handlerIntPtr(value int) *int { return &value }

func TestBirthHandlersDistinguishMidnightFromUnknown(t *testing.T) {
	midnight := fortune.BirthContext{
		Year: 2000, Month: 1, Day: 1,
		Hour: handlerIntPtr(0), Minute: handlerIntPtr(0),
		TimePrecision: fortune.PrecisionMinute,
	}
	unknown := fortune.BirthContext{
		Year: 2000, Month: 1, Day: 1,
		TimePrecision: fortune.PrecisionUnknown,
	}
	tests := []struct {
		name     string
		midnight func() (fortune.Input, error)
		unknown  func() (fortune.Input, error)
	}{
		{"bazi", func() (fortune.Input, error) { return (baziComputeReq{BirthContext: midnight}).toFortuneInput() }, func() (fortune.Input, error) { return (baziComputeReq{BirthContext: unknown}).toFortuneInput() }},
		{"ziwei", func() (fortune.Input, error) { return (ziweiComputeReq{BirthContext: midnight}).toFortuneInput() }, func() (fortune.Input, error) { return (ziweiComputeReq{BirthContext: unknown}).toFortuneInput() }},
		{"astrology", func() (fortune.Input, error) { return (astrologyComputeReq{BirthContext: midnight}).toFortuneInput() }, func() (fortune.Input, error) { return (astrologyComputeReq{BirthContext: unknown}).toFortuneInput() }},
		{"weighbone", func() (fortune.Input, error) { return (weighboneComputeReq{BirthContext: midnight}).toFortuneInput() }, func() (fortune.Input, error) { return (weighboneComputeReq{BirthContext: unknown}).toFortuneInput() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := tt.midnight()
			if err != nil || input.Hour != 0 || input.Minute != 0 || input.Birth == nil {
				t.Fatalf("midnight input = %+v, err=%v", input, err)
			}
			if _, err := tt.unknown(); !errors.Is(err, fortune.ErrBirthTimeUnknown) {
				t.Fatalf("unknown error = %v", err)
			}
		})
	}
}

func TestBirthComputeEndpointsReturnUncertaintyCandidates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name  string
		path  string
		mount func(*gin.Engine)
		body  string
	}{
		{"bazi", "/bazi", func(r *gin.Engine) { r.POST("/bazi", (&BaziHandler{}).Compute) }, `{"year":2000,"month":1,"day":1,"timePrecision":"unknown","gender":1}`},
		{"ziwei", "/ziwei", func(r *gin.Engine) { r.POST("/ziwei", (&ZiweiHandler{}).Compute) }, `{"year":2000,"month":1,"day":1,"timePrecision":"unknown","gender":1}`},
		{"astrology", "/astrology", func(r *gin.Engine) { r.POST("/astrology", (&AstrologyHandler{}).Compute) }, `{"year":2000,"month":1,"day":1,"timePrecision":"unknown","timeZone":"UTC"}`},
		{"weighbone", "/weighbone", func(r *gin.Engine) { r.POST("/weighbone", (&WeighboneHandler{}).Compute) }, `{"year":2000,"month":1,"day":1,"timePrecision":"unknown"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			tt.mount(router)
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"data":null`) || !strings.Contains(response.Body.String(), `"variants":[{`) {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestZiweiLeapMonthRequiresExplicitRule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/ziwei", (&ZiweiHandler{}).Compute)
	req := httptest.NewRequest(http.MethodPost, "/ziwei", strings.NewReader(`{"year":2023,"month":3,"day":22,"hour":12,"minute":0,"timePrecision":"minute","gender":1}`))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "leap-month rule") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAstrologyCalculationErrorsHaveStableHTTPResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{fortune.ErrAstrologyHighLatitude, http.StatusUnprocessableEntity, "astrology_high_latitude_unsupported"},
		{fortune.ErrAstrologyCalculationFailed, http.StatusServiceUnavailable, "astrology_calculation_failed"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(response)
		writeBirthComputeError(context, test.err)
		if response.Code != test.status || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	}
}

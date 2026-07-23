package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"predictdestiny/internal/fortune"
)

func validateBirthYear(birth fortune.BirthContext, minYear, maxYear int) error {
	if birth.Year < minYear || birth.Year > maxYear {
		return fmt.Errorf("birth year must be between %d and %d", minYear, maxYear)
	}
	return nil
}

func writeBirthComputeError(c *gin.Context, err error) {
	status := http.StatusBadRequest
	code := "invalid_birth_input"
	if errors.Is(err, fortune.ErrZiweiLeapMonthRuleRequired) {
		status = http.StatusConflict
	}
	if errors.Is(err, fortune.ErrBirthTimeUnknown) || errors.Is(err, fortune.ErrBirthTimeImprecise) ||
		errors.Is(err, fortune.ErrZiweiUnsupportedRuleSet) || errors.Is(err, fortune.ErrZiweiLeapMonthRuleUnsupported) {
		status = http.StatusUnprocessableEntity
	}
	if errors.Is(err, fortune.ErrAstrologyHighLatitude) {
		status = http.StatusUnprocessableEntity
		code = "astrology_high_latitude_unsupported"
	}
	if errors.Is(err, fortune.ErrAstrologyCalculationFailed) {
		status = http.StatusServiceUnavailable
		code = "astrology_calculation_failed"
	}
	c.JSON(status, gin.H{"error": err.Error(), "code": code})
}

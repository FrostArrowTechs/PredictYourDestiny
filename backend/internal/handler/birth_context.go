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
	if errors.Is(err, fortune.ErrBirthTimeUnknown) || errors.Is(err, fortune.ErrBirthTimeImprecise) {
		status = http.StatusUnprocessableEntity
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

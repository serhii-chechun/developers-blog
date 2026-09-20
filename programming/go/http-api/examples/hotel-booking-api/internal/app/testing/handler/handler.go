package handler

import (
	"context"
	"errors"
	"net/http"

	"hotel-booking-api/internal/app/testing/model"

	"github.com/gin-gonic/gin"
)

type (
	testingHandler struct {
		testingService
	}

	testingService interface {
		SeedTestData(ctx context.Context, p model.SeedParams) (*model.SeedResult, error)
		ResetTestData(ctx context.Context) error
	}
)

// New creates a new instance of the testing handler.
func New(ts testingService) *testingHandler {
	return &testingHandler{
		testingService: ts,
	}
}

// Seed handles PUT /v1/testing
func (h *testingHandler) Seed(c *gin.Context) {
	ctx := c.Request.Context()

	params, err := validateSeedRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.testingService.SeedTestData(ctx, params)
	if err != nil {
		c.JSON(seedErrorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Reset handles DELETE /v1/testing
func (h *testingHandler) Reset(c *gin.Context) {
	if err := h.testingService.ResetTestData(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// seedErrorStatus maps testing domain errors to HTTP status codes.
func seedErrorStatus(err error) int {
	if errors.Is(err, model.ErrNoRoomTypes) {
		return http.StatusConflict
	}

	return http.StatusInternalServerError
}

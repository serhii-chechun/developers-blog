package handler

import (
	"context"
	"net/http"

	"hotel-booking-api/internal/hotel/model"

	"github.com/gin-gonic/gin"
)

type (
	hotelHandler struct {
		hotelService
	}

	hotelService interface {
		FindAllHotels(ctx context.Context, p model.FindHotelsParams) (*model.HotelsPage, error)
	}
)

// New creates a new instance of the hotel handler.
func New(hs hotelService) *hotelHandler {
	return &hotelHandler{
		hotelService: hs,
	}
}

// GetAll handles GET /api/v1/hotels
func (h *hotelHandler) GetAll(c *gin.Context) {
	ctx := c.Request.Context()

	params, err := validateGetAllRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	page, err := h.hotelService.FindAllHotels(ctx, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, page)
}

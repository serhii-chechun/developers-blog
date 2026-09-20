package handler

import (
	"context"
	"errors"
	"net/http"

	"hotel-booking-api/internal/reservation/model"

	"github.com/gin-gonic/gin"
)

type (
	reservationHandler struct {
		reservationService
	}

	reservationService interface {
		CreateReservation(ctx context.Context, r model.ReservationItem) (*model.ReservationItem, error)
		FindReservationByReference(ctx context.Context, bookingRef string) (*model.ReservationItem, error)
	}
)

// New creates a new instance of the reservation handler.
func New(rs reservationService) *reservationHandler {
	return &reservationHandler{
		reservationService: rs,
	}
}

// Get handles GET /api/v1/reservations/:booking_ref
func (h *reservationHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()

	bookingRef := c.Param("booking_ref")
	reservation, err := h.reservationService.FindReservationByReference(ctx, bookingRef)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if reservation == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "reservation not found"})
		return
	}

	c.JSON(http.StatusOK, reservation)
}

// Create handles POST /api/v1/reservations
func (h *reservationHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	reservation, err := validateCreateRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err := h.reservationService.CreateReservation(ctx, reservation)
	if err != nil {
		c.JSON(createErrorStatus(err), gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, created)
}

// createErrorStatus maps reservation domain errors to HTTP status codes.
func createErrorStatus(err error) int {
	switch {
	case errors.Is(err, model.ErrHotelNotFound), errors.Is(err, model.ErrRoomNotFound):
		return http.StatusNotFound
	case errors.Is(err, model.ErrInsufficientCapacity), errors.Is(err, model.ErrReferenceExists), errors.Is(err, model.ErrRoomUnavailable):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

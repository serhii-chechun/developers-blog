package handler

import (
	"context"
	"net/http"

	"hotel-booking-api/internal/room/model"

	"github.com/gin-gonic/gin"
)

type (
	roomHandler struct {
		roomService
	}

	roomService interface {
		FindAllAvailableRooms(ctx context.Context, p model.FindAvailableRoomsParams) (*model.RoomsPage, error)
	}
)

// New creates a new instance of the room handler.
func New(rs roomService) *roomHandler {
	return &roomHandler{
		roomService: rs,
	}
}

// GetAll handles GET /api/v1/rooms
func (h *roomHandler) GetAll(c *gin.Context) {
	ctx := c.Request.Context()

	params, err := validateGetAllRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rooms, err := h.roomService.FindAllAvailableRooms(ctx, params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rooms)
}

package handler

import (
	"fmt"

	"hotel-booking-api/internal/app/testing/model"

	"github.com/gin-gonic/gin"
)

const (
	defaultMaxHotels        = 3
	defaultMaxRoomsPerHotel = 6

	maxHotelsLimit        = 100
	maxRoomsPerHotelLimit = 1000
)

func validateSeedRequest(c *gin.Context) (model.SeedParams, error) {
	var (
		form struct {
			MaxHotels        int `json:"max_hotels" form:"max_hotels" binding:"omitempty,gt=0"`
			MaxRoomsPerHotel int `json:"max_rooms_per_hotel" form:"max_rooms_per_hotel" binding:"omitempty,gt=0"`
		}
		result model.SeedParams
	)

	if err := c.ShouldBind(&form); err != nil {
		return result, err
	}

	if form.MaxHotels > maxHotelsLimit {
		return result, fmt.Errorf("max_hotels must not exceed %d", maxHotelsLimit)
	}

	if form.MaxRoomsPerHotel > maxRoomsPerHotelLimit {
		return result, fmt.Errorf("max_rooms_per_hotel must not exceed %d", maxRoomsPerHotelLimit)
	}

	result = model.SeedParams{
		Hotels:        form.MaxHotels,
		RoomsPerHotel: form.MaxRoomsPerHotel,
	}

	if result.Hotels == 0 {
		result.Hotels = defaultMaxHotels
	}

	if result.RoomsPerHotel == 0 {
		result.RoomsPerHotel = defaultMaxRoomsPerHotel
	}

	return result, nil
}

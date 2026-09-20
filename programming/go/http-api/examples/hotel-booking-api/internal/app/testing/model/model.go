package model

import (
	"errors"

	modelHotel "hotel-booking-api/internal/hotel/model"
	modelRoom "hotel-booking-api/internal/room/model"
)

// ErrNoRoomTypes is returned when the room_types reference table is empty.
var ErrNoRoomTypes = errors.New("room types are not seeded")

type (
	// SeedParams describes the shape of the test data to be generated.
	SeedParams struct {
		Hotels        int
		RoomsPerHotel int
	}

	// SeedResult reports the records that have been generated.
	SeedResult struct {
		Hotels    []*modelHotel.HotelItem `json:"hotels"`
		Rooms     []*modelRoom.RoomItem   `json:"rooms"`
		RoomTypes []*modelRoom.RoomType   `json:"room_types"`
	}
)

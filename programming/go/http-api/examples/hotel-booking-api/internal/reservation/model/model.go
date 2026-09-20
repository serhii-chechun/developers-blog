package model

import "errors"

const (
	ReservationStatusPending   ReservationStatus = iota + 2000 // reservation is pending
	ReservationStatusConfirmed                                 // reservation is confirmed
	ReservationStatusCompleted                                 // reservation is completed
	ReservationStatusCancelled                                 // reservation is cancelled

)

// Domain errors returned by the reservation layer.
var (
	ErrHotelNotFound        = errors.New("hotel not found")
	ErrRoomNotFound         = errors.New("room not found")
	ErrRoomUnavailable      = errors.New("room is not available for the requested dates")
	ErrInsufficientCapacity = errors.New("no combination of available rooms can accommodate the requested guests")
	ErrReferenceExists      = errors.New("booking reference already exists")
	ErrReservationNotFound  = errors.New("reservation not found")
)

type (
	// ReservationStatus represents the status of a reservation.
	ReservationStatus int

	// ReservationItem represents a booking header together with its allocated rooms.
	ReservationItem struct {
		ID string `json:"id"`

		CheckInDate  string `json:"check_in_date"`
		CheckOutDate string `json:"check_out_date"`

		GuestFullName string `json:"guest_full_name"`
		GuestEmail    string `json:"guest_email"`
		GuestsCount   int    `json:"guests_count"`

		Reference string            `json:"reference"`
		Status    ReservationStatus `json:"status"`

		HotelID                  string `json:"hotel_id"`
		*ReservationHotelDetails `json:"hotel_details,omitempty"`

		Rooms []*ReservationRoomDetails `json:"rooms,omitempty"`

		RoomIDs []string `json:"-"`
	}

	// ReservationHotelDetails represents the details of the hotel associated with a reservation.
	ReservationHotelDetails struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	}

	// ReservationRoomDetails represents a room allocated to a reservation.
	ReservationRoomDetails struct {
		RoomID           string `json:"room_id"`
		Label            string `json:"room_label"`
		RoomTypeCaption  string `json:"room_type_caption"`
		RoomTypeCapacity int    `json:"room_type_capacity"`
		GuestsCount      int    `json:"guests_count"`
	}
)

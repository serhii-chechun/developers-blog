package model

type (
	// RoomItem represents a room record with its basic information and availability.
	RoomItem struct {
		ID string `json:"id"`

		Label       string `json:"room_label"`
		IsAvailable bool   `json:"is_available"`

		TypeID    int `json:"-"`
		*RoomType `json:"room_type,omitempty"`

		HotelID               string `json:"hotel_id"`
		*RoomItemHotelDetails `json:"hotel_details,omitempty"`
	}

	// RoomItemHotelDetails represents the basic details of the hotel associated with a room.
	RoomItemHotelDetails struct {
		Name    string `json:"hotel_name"`
		Address string `json:"hotel_address"`
	}

	// RoomType represents the type of a room.
	RoomType struct {
		ID       int    `json:"-"`
		Caption  string `json:"room_type_caption"`
		Capacity int    `json:"room_type_capacity"`
	}
)

type (
	// FindAvailableRoomsParams represents the parameters required to find available rooms.
	FindAvailableRoomsParams struct {
		CheckInDate  string
		CheckOutDate string
		GuestsCount  int
		HotelID      string
		AfterID      string
		PageSize     int
	}

	// RoomsPage represents a page of available rooms together with its pagination metadata.
	RoomsPage struct {
		Rooms    []*RoomItem `json:"rooms"`
		PageSize int         `json:"page_size"`
		NextPage string      `json:"next_page,omitempty"`
	}
)

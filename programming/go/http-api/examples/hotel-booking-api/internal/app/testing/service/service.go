package service

import (
	"context"
	"fmt"

	modelTesting "hotel-booking-api/internal/app/testing/model"
	modelHotel "hotel-booking-api/internal/hotel/model"
	modelRoom "hotel-booking-api/internal/room/model"

	"github.com/google/uuid"
)

type (
	testingService struct {
		hotelService
		roomService
		reservationService
	}

	hotelService interface {
		CreateHotels(ctx context.Context, hotels []*modelHotel.HotelItem) error
		RemoveAllHotels(ctx context.Context) error
	}

	roomService interface {
		FindAllRoomTypes(ctx context.Context) ([]*modelRoom.RoomType, error)
		CreateRooms(ctx context.Context, rooms []*modelRoom.RoomItem) error
		RemoveAllRooms(ctx context.Context) error
	}

	reservationService interface {
		RemoveAllReservations(ctx context.Context) error
	}
)

// New creates a new instance of testingService wired to the domain services.
func New(hs hotelService, rs roomService, res reservationService) *testingService {
	return &testingService{
		hotelService:       hs,
		roomService:        rs,
		reservationService: res,
	}
}

// SeedTestData replaces the existing test data with hotels and rooms generated from the provided params.
func (s *testingService) SeedTestData(ctx context.Context, p modelTesting.SeedParams) (*modelTesting.SeedResult, error) {
	if err := s.ResetTestData(ctx); err != nil {
		return nil, fmt.Errorf("reset test data: %w", err)
	}

	roomTypes, err := s.roomService.FindAllRoomTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("find room types: %w", err)
	}
	if len(roomTypes) == 0 {
		return nil, modelTesting.ErrNoRoomTypes
	}

	hotels := generateHotels(p.Hotels)
	rooms := generateRooms(hotels, p.RoomsPerHotel, roomTypes)

	if err := s.hotelService.CreateHotels(ctx, hotels); err != nil {
		return nil, fmt.Errorf("create hotels: %w", err)
	}
	if err := s.roomService.CreateRooms(ctx, rooms); err != nil {
		return nil, fmt.Errorf("create rooms: %w", err)
	}

	return &modelTesting.SeedResult{
		Hotels:    hotels,
		Rooms:     rooms,
		RoomTypes: roomTypes,
	}, nil
}

// ResetTestData removes all reservations, rooms and hotels.
func (s *testingService) ResetTestData(ctx context.Context) error {
	if err := s.reservationService.RemoveAllReservations(ctx); err != nil {
		return fmt.Errorf("remove reservations: %w", err)
	}
	if err := s.roomService.RemoveAllRooms(ctx); err != nil {
		return fmt.Errorf("remove rooms: %w", err)
	}
	if err := s.hotelService.RemoveAllHotels(ctx); err != nil {
		return fmt.Errorf("remove hotels: %w", err)
	}

	return nil
}

// generateHotels builds the requested number of hotels.
func generateHotels(count int) []*modelHotel.HotelItem {
	hotels := make([]*modelHotel.HotelItem, 0, count)
	for i := 1; i <= count; i++ {
		hotels = append(hotels, &modelHotel.HotelItem{
			ID:      uuid.NewString(),
			Name:    hotelName(i),
			Address: fmt.Sprintf("Address %d, Test City", i),
			Phone:   fmt.Sprintf("+44 1234 9%05d", i),
		})
	}

	return hotels
}

// generateRooms builds rooms for every hotel, assigning room types in rotation.
func generateRooms(hotels []*modelHotel.HotelItem, roomsPerHotel int, roomTypes []*modelRoom.RoomType) []*modelRoom.RoomItem {
	rooms := make([]*modelRoom.RoomItem, 0, len(hotels)*roomsPerHotel)
	for _, hotel := range hotels {
		for i := 1; i <= roomsPerHotel; i++ {
			roomType := roomTypes[(i-1)%len(roomTypes)]
			rooms = append(rooms, &modelRoom.RoomItem{
				ID:          uuid.NewString(),
				HotelID:     hotel.ID,
				Label:       fmt.Sprintf("Room %d", i),
				IsAvailable: true,
				TypeID:      roomType.ID,
				RoomType:    roomType,
			})
		}
	}

	return rooms
}

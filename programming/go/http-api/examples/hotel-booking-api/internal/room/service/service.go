package service

import (
	"context"

	"hotel-booking-api/internal/room/model"
)

type (
	roomService struct {
		roomRepository
	}

	roomRepository interface {
		GetAvailableRooms(ctx context.Context, p model.FindAvailableRoomsParams) (*model.RoomsPage, error)
		GetRoomTypes(ctx context.Context) ([]*model.RoomType, error)
		PutRooms(ctx context.Context, rooms []*model.RoomItem) error
		DeleteAllRooms(ctx context.Context) error
	}
)

// New creates a new instance of the room service.
func New(rr roomRepository) *roomService {
	return &roomService{
		roomRepository: rr,
	}
}

// FindAvailableRooms retrieves all available rooms based on params provided.
func (s *roomService) FindAllAvailableRooms(ctx context.Context, p model.FindAvailableRoomsParams) (*model.RoomsPage, error) {
	return s.roomRepository.GetAvailableRooms(ctx, p)
}

// FindAllRoomTypes retrieves all available room types.
func (s *roomService) FindAllRoomTypes(ctx context.Context) ([]*model.RoomType, error) {
	return s.roomRepository.GetRoomTypes(ctx)
}

// CreateRooms stores the provided rooms in bulk.
func (s *roomService) CreateRooms(ctx context.Context, rooms []*model.RoomItem) error {
	return s.roomRepository.PutRooms(ctx, rooms)
}

// RemoveAllRooms removes all rooms with their dependent records.
func (s *roomService) RemoveAllRooms(ctx context.Context) error {
	return s.roomRepository.DeleteAllRooms(ctx)
}

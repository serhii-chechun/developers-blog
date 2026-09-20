package service

import (
	"context"

	"hotel-booking-api/internal/hotel/model"
)

type (
	hotelService struct {
		hotelRepository
	}

	hotelRepository interface {
		GetHotelsByName(ctx context.Context, p model.FindHotelsParams) (*model.HotelsPage, error)
		PutHotels(ctx context.Context, hotels []*model.HotelItem) error
		DeleteAllHotels(ctx context.Context) error
	}
)

// New creates and returns a new instance of hotelService.
func New(hr hotelRepository) *hotelService {
	return &hotelService{
		hotelRepository: hr,
	}
}

// FindAllHotels finds hotels by their name using the hotel repository.
func (s *hotelService) FindAllHotels(ctx context.Context, p model.FindHotelsParams) (*model.HotelsPage, error) {
	return s.hotelRepository.GetHotelsByName(ctx, p)
}

// CreateHotels stores the provided hotels in bulk.
func (s *hotelService) CreateHotels(ctx context.Context, hotels []*model.HotelItem) error {
	return s.hotelRepository.PutHotels(ctx, hotels)
}

// RemoveAllHotels removes all hotels with their dependent records.
func (s *hotelService) RemoveAllHotels(ctx context.Context) error {
	return s.hotelRepository.DeleteAllHotels(ctx)
}

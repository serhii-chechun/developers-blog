package service

import (
	"context"

	"hotel-booking-api/internal/reservation/model"
)

const maxReferenceAttempts = 5

type (
	reservationService struct {
		reservationRepository
	}

	reservationRepository interface {
		PutReservation(ctx context.Context, r model.ReservationItem) (*model.ReservationItem, error)
		GetReservationByReference(ctx context.Context, bookingRef string) (*model.ReservationItem, error)
		GetReservationByID(ctx context.Context, reservationID string) (*model.ReservationItem, error)
		GetAllReservationsByStatus(ctx context.Context, status model.ReservationStatus) ([]*model.ReservationItem, error)
		UpdateReservationStatus(ctx context.Context, reservationID string, status model.ReservationStatus) (*model.ReservationItem, error)
		DeleteAllReservations(ctx context.Context) error
	}
)

// New creates a new instance of reservationService with the provided reservationRepository.
func New(rr reservationRepository) *reservationService {
	return &reservationService{
		reservationRepository: rr,
	}
}

// CreateReservation creates a new pending reservation with a unique booking reference.
func (s *reservationService) CreateReservation(ctx context.Context, r model.ReservationItem) (*model.ReservationItem, error) {
	r.Status = model.ReservationStatusPending

	for range maxReferenceAttempts {
		r.Reference = generateReference()

		existing, err := s.reservationRepository.GetReservationByReference(ctx, r.Reference)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			continue
		}

		return s.reservationRepository.PutReservation(ctx, r)
	}

	return nil, model.ErrReferenceExists
}

// Get retrieves a reservation by its booking reference.
func (s *reservationService) FindReservationByReference(ctx context.Context, bookingRef string) (*model.ReservationItem, error) {
	return s.reservationRepository.GetReservationByReference(ctx, bookingRef)
}

// FindReservationByStatus retrieves all reservations with the specified status.
func (s *reservationService) FindReservationByStatus(ctx context.Context, status model.ReservationStatus) ([]*model.ReservationItem, error) {
	return s.reservationRepository.GetAllReservationsByStatus(ctx, status)
}

// ConfirmReservation confirms a reservation by its ID.
func (s *reservationService) ConfirmReservation(ctx context.Context, reservationID string) (*model.ReservationItem, error) {
	return s.reservationRepository.UpdateReservationStatus(ctx, reservationID, model.ReservationStatusConfirmed)
}

// RemoveAllReservations removes all reservations.
func (s *reservationService) RemoveAllReservations(ctx context.Context) error {
	return s.reservationRepository.DeleteAllReservations(ctx)
}

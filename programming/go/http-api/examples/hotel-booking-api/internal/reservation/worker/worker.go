package worker

import (
	"context"
	"log/slog"
	"time"

	"hotel-booking-api/internal/reservation/model"
)

type (
	reservationWorker struct {
		interval time.Duration
		log      *slog.Logger

		reservationService
	}

	reservationService interface {
		FindReservationByStatus(ctx context.Context, status model.ReservationStatus) ([]*model.ReservationItem, error)
		ConfirmReservation(ctx context.Context, reservationID string) (*model.ReservationItem, error)
	}
)

// New creates a new instance of reservationWorker with the provided reservationService.
func New(rs reservationService, i time.Duration, l *slog.Logger) *reservationWorker {
	return &reservationWorker{
		interval:           i,
		log:                l,
		reservationService: rs,
	}
}

// Start begins the reservation worker's processing loop.
func (w *reservationWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.log.Info("reservation worker started")
	for {
		select {
		case <-ctx.Done():
			w.log.Info("reservation worker stopped")
			return
		case <-ticker.C:
			w.log.Info("processing pending reservations")
			processed := w.processPendingReservations(ctx)
			w.log.Info("processed pending reservations", "count", processed)
		}
	}

}

func (w *reservationWorker) processPendingReservations(ctx context.Context) int {
	processedCount := 0
	defer func() {
		if r := recover(); r != nil {
			w.log.Error("panic recovered in processPendingReservations", "error", r)
			return
		}
	}()

	reservations, err := w.reservationService.FindReservationByStatus(ctx, model.ReservationStatusPending)
	if err != nil {
		w.log.Error("failed to find pending reservations", "error", err)
		return 0
	}

	for _, reservation := range reservations {
		if ctx.Err() != nil {
			w.log.Info("shutdown requested, aborting pending reservation batch")
			return processedCount
		}

		r, err := w.reservationService.ConfirmReservation(ctx, reservation.ID)
		if err != nil {
			w.log.Error("failed to confirm reservation", "error", err, "reservation_id", reservation.ID)
			continue
		}
		processedCount++
		w.log.Info("successfully confirmed reservation", "reservation_id", r.ID)

		select {
		case <-ctx.Done():
			w.log.Info("shutdown requested during email delay", "reservation_id", r.ID)
			return processedCount
		case <-time.After(2 * time.Second):
		}
		w.log.Info("email sent for reservation", "reservation_id", r.ID)
	}
	return processedCount
}
